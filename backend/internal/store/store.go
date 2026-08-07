// Package store persists Porter state in PostgreSQL. Only this package is
// allowed to import the pgx driver (Unified Spec §6).
//
// Model (v3): a project IS a microVM app with a replica pool. Replicas live
// inside their project. Every user auto-gets a default org; groups are folders
// inside an org. No managed volumes (persistence = project host_mount_path);
// no image catalog (image is a plain OCI ref).
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"porter/internal/types"
	"porter/migrations"
)

// Store is the single real state store: a pgx pool plus two in-memory ring
// buffers (traffic + per-VM logs) that are explicitly not persisted.
type Store struct {
	pool *pgxpool.Pool

	trafficMu sync.RWMutex
	traffic   map[string][]*types.TrafficEntry // replica_id -> ring (in-memory)

	logMu      sync.RWMutex
	logs       map[string][]string
	daemonLogs []string
}

const (
	trafficRingSize   = 200
	logRingSize       = 500
	daemonLogRingSize = 1000
)

// NewStore connects to PostgreSQL at dsn, runs pending migrations, and returns
// a ready store. Fatal on failure — the server must not start without its DB.
func NewStore(dsn string) *Store {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("store: connect postgres %q: %v", dsn, err)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("store: ping postgres %q: %v", dsn, err)
	}
	if err := Migrate(ctx, pool); err != nil {
		log.Fatalf("store: migrate: %v", err)
	}
	return &Store{
		pool:    pool,
		traffic: map[string][]*types.TrafficEntry{},
		logs:    map[string][]string{},
	}
}

// Close releases the pool.
func (s *Store) Close() error {
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}

// NewID returns a random UUID string.
func NewID() string {
	return uuid.NewString()
}

// Migrate runs every NNNN_*.up.sql in order, tracking applied versions.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		var applied bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		for _, stmt := range splitSQL(string(body)) {
			if _, err := pool.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("apply migration %s: %w\nstmt: %s", name, err, stmt)
			}
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		log.Printf("store: applied migration %s", name)
	}
	return nil
}

func splitSQL(body string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(strings.TrimSpace(line), ";") {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("store: marshal json: %v", err)
		return []byte("{}")
	}
	return b
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// --- Replicas (one microVM; nested inside a project) ---
// Backed by the `replicas` table (renamed from `vms` in 0010). The VM methods
// are kept so the vmmanager executor works unchanged.

func (s *Store) PutVM(vm *types.VM) {
	ctx := context.Background()
	data := mustJSON(vm)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO replicas (id, project_id, replica_index, container_id, task_id,
		                      state, health_status, ip_address, ports, started_at,
		                      crashed, data, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			project_id=EXCLUDED.project_id, replica_index=EXCLUDED.replica_index,
			container_id=EXCLUDED.container_id, task_id=EXCLUDED.task_id,
			state=EXCLUDED.state, health_status=EXCLUDED.health_status,
			ip_address=EXCLUDED.ip_address, ports=EXCLUDED.ports,
			started_at=EXCLUDED.started_at, crashed=EXCLUDED.crashed,
			data=EXCLUDED.data, updated_at=now()`,
		vm.ID, nullableStr(vm.ProjectID), vm.ReplicaIndex, nullableStr(vm.ContainerID),
		nullableStr(vm.TaskID), vm.State, vm.HealthStatus, nullableStr(vm.IPAddress),
		string(mustJSON(vm.Ports)), vm.StartedAt, vm.Crashed, string(data))
	if err != nil {
		log.Printf("store: put replica %s: %v", vm.ID, err)
	}
}

func (s *Store) GetVM(id string) (*types.VM, bool) {
	var data []byte
	if err := s.pool.QueryRow(context.Background(),
		`SELECT data FROM replicas WHERE id = $1`, id).Scan(&data); err != nil {
		return nil, false
	}
	var vm types.VM
	if err := json.Unmarshal(data, &vm); err != nil {
		log.Printf("store: unmarshal replica %s: %v", id, err)
		return nil, false
	}
	return &vm, true
}

func (s *Store) ListVMs() []*types.VM {
	rows, err := s.pool.Query(context.Background(), `SELECT data FROM replicas ORDER BY created_at`)
	if err != nil {
		log.Printf("store: list replicas: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.VM, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var vm types.VM
		if err := json.Unmarshal(data, &vm); err != nil {
			continue
		}
		out = append(out, &vm)
	}
	return out
}

// ListReplicas returns all replicas of a project.
func (s *Store) ListReplicas(projectID string) []*types.VM {
	rows, err := s.pool.Query(context.Background(),
		`SELECT data FROM replicas WHERE project_id = $1 ORDER BY replica_index`, projectID)
	if err != nil {
		log.Printf("store: list replicas for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.VM, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var vm types.VM
		if err := json.Unmarshal(data, &vm); err != nil {
			continue
		}
		out = append(out, &vm)
	}
	return out
}

func (s *Store) DeleteVM(id string) {
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `DELETE FROM replicas WHERE id = $1`, id); err != nil {
		log.Printf("store: delete replica %s: %v", id, err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM domains WHERE vm_id = $1 OR data->>'vm_id' = $1`, id); err != nil {
		log.Printf("store: delete domains for %s: %v", id, err)
	}
	s.trafficMu.Lock()
	delete(s.traffic, id)
	s.trafficMu.Unlock()
	s.logMu.Lock()
	delete(s.logs, id)
	s.logMu.Unlock()
}

// --- Projects (the microVM app; one row, owns the replica pool) ---

func (s *Store) PutProject(p *types.Project) {
	ctx := context.Background()
	data := mustJSON(p)
	kind := "single_image"
	if p.Source == "compose" {
		kind = "compose"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO projects (id, name, kind, compose_yaml, image, org_id,
		                      host_mount_path, replicas_desired, restart_policy,
		                      healthcheck, env, status, data, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, kind=EXCLUDED.kind, compose_yaml=EXCLUDED.compose_yaml,
			image=EXCLUDED.image, org_id=EXCLUDED.org_id, host_mount_path=EXCLUDED.host_mount_path,
			replicas_desired=EXCLUDED.replicas_desired, restart_policy=EXCLUDED.restart_policy,
			healthcheck=EXCLUDED.healthcheck, env=EXCLUDED.env, status=EXCLUDED.status,
			data=EXCLUDED.data, updated_at=now()`,
		p.ID, p.Name, kind, nullableStr(p.ComposeYAML), nullableStr(p.Image),
		nullableStr(p.OrgID), nullableStr(p.HostMountPath), p.ReplicasDesired,
		p.RestartPolicy, mustJSON(p.Healthcheck), string(mustJSON(p.Env)),
		statusFromProject(p), string(data))
	if err != nil {
		log.Printf("store: put project %s: %v", p.ID, err)
	}
}

func (s *Store) GetProject(id string) (*types.Project, bool) {
	var data []byte
	if err := s.pool.QueryRow(context.Background(),
		`SELECT data FROM projects WHERE id = $1`, id).Scan(&data); err != nil {
		return nil, false
	}
	var p types.Project
	if err := json.Unmarshal(data, &p); err != nil {
		log.Printf("store: unmarshal project %s: %v", id, err)
		return nil, false
	}
	return &p, true
}

func (s *Store) ListProjects() []*types.Project {
	rows, err := s.pool.Query(context.Background(), `SELECT data FROM projects ORDER BY created_at`)
	if err != nil {
		log.Printf("store: list projects: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Project, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var p types.Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out
}

func (s *Store) ListProjectsByOrg(orgID string) []*types.Project {
	rows, err := s.pool.Query(context.Background(),
		`SELECT data FROM projects WHERE org_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		log.Printf("store: list projects for org %s: %v", orgID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Project, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var p types.Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out
}

func (s *Store) DeleteProject(id string) {
	if _, err := s.pool.Exec(context.Background(),
		`DELETE FROM projects WHERE id = $1`, id); err != nil {
		log.Printf("store: delete project %s: %v", id, err)
	}
}

func (s *Store) SetProjectOrg(projectID, orgID string) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE projects SET org_id = $2 WHERE id = $1`, projectID, nullableStr(orgID))
	return err
}

func (s *Store) SetProjectTags(projectID string, tags []string) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE projects SET tags = $2 WHERE id = $1`, projectID, tags)
	return err
}

// --- Orgs (auto-created default org per user) ---

// EnsureDefaultOrg creates a default org for a user if none exists. Called on
// first admin login / signup so GET /orgs/default always resolves.
func (s *Store) EnsureDefaultOrg(ctx context.Context, ownerID, name string) (string, error) {
	var existing string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM orgs WHERE owner_id = $1 AND is_default = true`, ownerID).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	id := NewID()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO orgs (id, name, owner_id, is_default) VALUES ($1,$2,$3,true)`,
		id, name, ownerID)
	if err != nil {
		return "", err
	}
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1,$2,'owner') ON CONFLICT DO NOTHING`,
		id, ownerID)
	return id, nil
}

func (s *Store) PutOrg(o *types.Org) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO orgs (id, name, owner_id, is_default) VALUES ($1,$2,$3,false)`,
		o.ID, o.Name, nullableStr(o.OwnerID))
	return err
}

func (s *Store) ListOrgs() []*types.Org {
	rows, err := s.pool.Query(context.Background(), `SELECT id, name FROM orgs ORDER BY name`)
	if err != nil {
		log.Printf("store: list orgs: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Org, 0)
	for rows.Next() {
		var o types.Org
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			continue
		}
		out = append(out, &o)
	}
	return out
}

func (s *Store) GetOrg(id string) (*types.Org, bool) {
	var o types.Org
	if err := s.pool.QueryRow(context.Background(),
		`SELECT id, name, owner_id, is_default FROM orgs WHERE id = $1`, id).
		Scan(&o.ID, &o.Name, &o.OwnerID, &o.IsDefault); err != nil {
		return nil, false
	}
	return &o, true
}

// --- Groups (folders for related projects, scoped to an org) ---

func (s *Store) PutGroup(g *types.Group) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO groups (id, org_id, name) VALUES ($1,$2,$3)`,
		g.ID, nullableStr(g.OrgID), g.Name)
	return err
}

func (s *Store) ListGroups(orgID string) []*types.Group {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, org_id, name FROM groups WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		log.Printf("store: list groups for %s: %v", orgID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Group, 0)
	for rows.Next() {
		var g types.Group
		if err := rows.Scan(&g.ID, &g.OrgID, &g.Name); err != nil {
			continue
		}
		out = append(out, &g)
	}
	return out
}

func (s *Store) AddProjectToGroup(groupID, projectID string) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO group_projects (group_id, project_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		groupID, projectID)
	return err
}

func (s *Store) RemoveProjectFromGroup(groupID, projectID string) error {
	_, err := s.pool.Exec(context.Background(), `
		DELETE FROM group_projects WHERE group_id = $1 AND project_id = $2`,
		groupID, projectID)
	return err
}

func (s *Store) ListProjectsInGroup(groupID string) []*types.Project {
	rows, err := s.pool.Query(context.Background(), `
		SELECT p.data FROM group_projects gp JOIN projects p ON p.id = gp.project_id
		WHERE gp.group_id = $1 ORDER BY p.created_at`, groupID)
	if err != nil {
		log.Printf("store: list projects in group %s: %v", groupID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Project, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var p types.Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out
}

// --- Domains (attached to a project; routes to its replica pool) ---

func (s *Store) AddDomain(projectID string, d *types.Domain) {
	data := mustJSON(d)
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO domains (project_id, domain, kind, status, data, created_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (domain) DO UPDATE SET
			project_id=EXCLUDED.project_id, kind=EXCLUDED.kind, status=EXCLUDED.status, data=EXCLUDED.data`,
		nullableStr(projectID), d.Domain, d.Type, d.Status, string(data))
	if err != nil {
		log.Printf("store: add domain %s for project %s: %v", d.Domain, projectID, err)
	}
}

func (s *Store) ListDomains(projectID string) []*types.Domain {
	rows, err := s.pool.Query(context.Background(), `
		SELECT data FROM domains WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		log.Printf("store: list domains for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Domain, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var d types.Domain
		if err := json.Unmarshal(data, &d); err != nil {
			continue
		}
		out = append(out, &d)
	}
	return out
}

func (s *Store) RemoveDomain(projectID, domain string) bool {
	res, err := s.pool.Exec(context.Background(),
		`DELETE FROM domains WHERE domain = $2 AND project_id = $1`, projectID, domain)
	if err != nil {
		log.Printf("store: remove domain %s for %s: %v", domain, projectID, err)
		return false
	}
	return res.RowsAffected() > 0
}

// --- DNS records (per project) ---

func (s *Store) AddDNSRecord(r *types.DNSRecord) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO dns_records (id, project_id, name, type, value, ttl, created_at)
		VALUES ($1,$2,$3,$4,$5,$6, now())
		ON CONFLICT (project_id, name) DO UPDATE SET type=EXCLUDED.type, value=EXCLUDED.value, ttl=EXCLUDED.ttl`,
		r.ID, nullableStr(r.ProjectID), r.Name, r.Type, r.Value, r.TTL)
	return err
}

func (s *Store) ListDNSRecords(projectID string) []*types.DNSRecord {
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, project_id, name, type, value, ttl FROM dns_records
		WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		log.Printf("store: list dns for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.DNSRecord, 0)
	for rows.Next() {
		var r types.DNSRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Type, &r.Value, &r.TTL); err != nil {
			continue
		}
		out = append(out, &r)
	}
	return out
}

func (s *Store) RemoveDNSRecord(projectID, name string) error {
	_, err := s.pool.Exec(context.Background(),
		`DELETE FROM dns_records WHERE project_id = $1 AND name = $2`, projectID, name)
	return err
}

// --- Build logs (rolling, per project) ---

func (s *Store) AppendBuildLog(projectID, line string) {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO build_logs (project_id, line) VALUES ($1,$2)`, projectID, line)
	if err != nil {
		log.Printf("store: append build log for %s: %v", projectID, err)
	}
}

func (s *Store) TailBuildLogs(projectID string, n int) []string {
	if n <= 0 {
		n = 200
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT line FROM (SELECT id, line FROM build_logs WHERE project_id = $1 ORDER BY id DESC LIMIT $2) t
		ORDER BY id ASC`, projectID, n)
	if err != nil {
		log.Printf("store: tail build logs for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			continue
		}
		out = append(out, line)
	}
	return out
}

// --- Deployments (version history / rollback) ---

func (s *Store) CreateDeployment(d *types.Deployment) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO deployments (id, project_id, build_status, image_digest, rollback_to, created_at)
		VALUES ($1,$2,$3,$4,$5, now())`,
		d.ID, nullableStr(d.ProjectID), d.BuildStatus, nullableStr(d.ImageDigest), nullableStr(d.RollbackTo))
	return err
}

func (s *Store) ListDeployments(projectID string) []*types.Deployment {
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, project_id, build_status, image_digest, created_at FROM deployments
		WHERE project_id = $1 ORDER BY revision DESC`, projectID)
	if err != nil {
		log.Printf("store: list deployments for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Deployment, 0)
	for rows.Next() {
		var d types.Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.BuildStatus, &d.ImageDigest, &d.CreatedAt); err != nil {
			continue
		}
		out = append(out, &d)
	}
	return out
}

// --- Secrets (per project, stored encrypted) ---

func (s *Store) PutSecret(sec *types.Secret) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO secrets (id, project_id, name, value_encrypted, created_at, updated_at)
		VALUES ($1,$2,$3,$4, now(), now())
		ON CONFLICT (project_id, name) DO UPDATE SET value_encrypted=EXCLUDED.value_encrypted, updated_at=now()`,
		sec.ID, nullableStr(sec.ProjectID), sec.Name, sec.ValueEncrypted)
	return err
}

func (s *Store) ListSecrets(projectID string) []*types.Secret {
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, project_id, name, value_encrypted FROM secrets WHERE project_id = $1 ORDER BY name`,
		projectID)
	if err != nil {
		log.Printf("store: list secrets for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Secret, 0)
	for rows.Next() {
		var sec types.Secret
		if err := rows.Scan(&sec.ID, &sec.ProjectID, &sec.Name, &sec.ValueEncrypted); err != nil {
			continue
		}
		out = append(out, &sec)
	}
	return out
}

func (s *Store) DeleteSecret(id string) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM secrets WHERE id = $1`, id)
	return err
}

// --- Golden images (image library; image is an OCI ref/URL) ---

func (s *Store) PutGoldenImage(gi *types.GoldenImage) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO golden_images (id, name, image, description, vcpus, mem_mib, ports, env, tags, logo, version, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
		ON CONFLICT (name) DO UPDATE SET image=EXCLUDED.image, description=EXCLUDED.description,
			vcpus=EXCLUDED.vcpus, mem_mib=EXCLUDED.mem_mib, ports=EXCLUDED.ports,
			env=EXCLUDED.env, tags=EXCLUDED.tags, logo=EXCLUDED.logo, version=EXCLUDED.version`,
		gi.ID, gi.Name, gi.Image, gi.Description, gi.VCPUs, gi.MemMiB,
		string(mustJSON(gi.Ports)), string(mustJSON(gi.Env)), gi.Tags, gi.Logo, gi.Version)
	return err
}

func (s *Store) ListGoldenImages() []*types.GoldenImage {
	rows, err := s.pool.Query(context.Background(), `SELECT data FROM golden_images ORDER BY name`)
	if err != nil {
		log.Printf("store: list golden images: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.GoldenImage, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var gi types.GoldenImage
		if err := json.Unmarshal(data, &gi); err != nil {
			continue
		}
		out = append(out, &gi)
	}
	return out
}

func (s *Store) DeleteGoldenImage(id string) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM golden_images WHERE id = $1`, id)
	return err
}

// --- Metrics + health events ---

func (s *Store) AddMetric(m *types.MetricSample) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO metrics_samples (vm_id, metric, value, ts) VALUES ($1,$2,$3,$4)`,
		nullableStr(m.VMID), m.Metric, m.Value, m.TS)
	return err
}

func (s *Store) ListMetrics(vmID string, limit int) []*types.MetricSample {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT vm_id, metric, value, ts FROM metrics_samples WHERE vm_id = $1 ORDER BY ts DESC LIMIT $2`,
		vmID, limit)
	if err != nil {
		log.Printf("store: list metrics for %s: %v", vmID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.MetricSample, 0)
	for rows.Next() {
		var m types.MetricSample
		if err := rows.Scan(&m.VMID, &m.Metric, &m.Value, &m.TS); err != nil {
			continue
		}
		out = append(out, &m)
	}
	return out
}

func (s *Store) AddHealthEvent(e *types.HealthEvent) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO health_events (vm_id, project_id, status, detail, ts) VALUES ($1,$2,$3,$4,$5)`,
		nullableStr(e.VMID), nullableStr(e.ProjectID), e.Status, e.Detail, e.TS)
	return err
}

func (s *Store) ListHealthEvents(projectID string, limit int) []*types.HealthEvent {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT vm_id, project_id, status, detail, ts FROM health_events
		WHERE project_id = $1 ORDER BY ts DESC LIMIT $2`, projectID, limit)
	if err != nil {
		log.Printf("store: list health events for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.HealthEvent, 0)
	for rows.Next() {
		var e types.HealthEvent
		if err := rows.Scan(&e.VMID, &e.ProjectID, &e.Status, &e.Detail, &e.TS); err != nil {
			continue
		}
		out = append(out, &e)
	}
	return out
}

// --- Users ---

func (s *Store) PutUser(u *types.User) {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO users (id, username, role, password_hash, salt, created_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (username) DO UPDATE SET role=EXCLUDED.role,
			password_hash=EXCLUDED.password_hash, salt=EXCLUDED.salt`,
		u.ID, u.Username, u.Role, u.PasswordHash, u.Salt)
	if err != nil {
		log.Printf("store: put user %s: %v", u.Username, err)
	}
}

func (s *Store) ListUsers() []*types.User {
	rows, err := s.pool.Query(context.Background(), `SELECT id, username, role FROM users ORDER BY created_at`)
	if err != nil {
		log.Printf("store: list users: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.User, 0)
	for rows.Next() {
		var u types.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			continue
		}
		out = append(out, &u)
	}
	return out
}

func (s *Store) GetUserByUsername(username string) (*types.User, bool) {
	var u types.User
	if err := s.pool.QueryRow(context.Background(),
		`SELECT id, username, role, password_hash, salt FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.Salt); err != nil {
		return nil, false
	}
	return &u, true
}

func (s *Store) DeleteUser(id string) {
	if _, err := s.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
		log.Printf("store: delete user %s: %v", id, err)
	}
}

// --- Servers (Phase-8 scaffold) ---

func (s *Store) PutServer(srv *types.Server) {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO servers (id, hostname, registered_at) VALUES ($1,$2, now())
		ON CONFLICT (id) DO UPDATE SET hostname=EXCLUDED.hostname`,
		srv.ID, srv.Name)
	if err != nil {
		log.Printf("store: put server %s: %v", srv.ID, err)
	}
}

func (s *Store) ListServers() []*types.Server {
	rows, err := s.pool.Query(context.Background(), `SELECT id, hostname FROM servers ORDER BY registered_at`)
	if err != nil {
		log.Printf("store: list servers: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*types.Server, 0)
	for rows.Next() {
		var srv types.Server
		if err := rows.Scan(&srv.ID, &srv.Name); err != nil {
			continue
		}
		out = append(out, &srv)
	}
	return out
}

func (s *Store) DeleteServer(id string) bool {
	res, err := s.pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, id)
	if err != nil {
		log.Printf("store: delete server %s: %v", id, err)
		return false
	}
	return res.RowsAffected() > 0
}

// --- Traffic ring buffer (in-memory only) ---

func (s *Store) AddTraffic(vmID string, e *types.TrafficEntry) {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	buf := append(s.traffic[vmID], e)
	if len(buf) > trafficRingSize {
		buf = buf[len(buf)-trafficRingSize:]
	}
	s.traffic[vmID] = buf
}

func (s *Store) ListTraffic(vmID string, limit int) []*types.TrafficEntry {
	s.trafficMu.RLock()
	defer s.trafficMu.RUnlock()
	buf := s.traffic[vmID]
	if limit <= 0 || limit > len(buf) {
		limit = len(buf)
	}
	start := len(buf) - limit
	out := make([]*types.TrafficEntry, limit)
	copy(out, buf[start:])
	return out
}

// --- Log ring buffer (in-memory only) ---

func (s *Store) AppendLog(vmID, line string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	buf := append(s.logs[vmID], line)
	if len(buf) > logRingSize {
		buf = buf[len(buf)-logRingSize:]
	}
	s.logs[vmID] = buf
}

func (s *Store) TailLogs(vmID string, n int) []string {
	s.logMu.RLock()
	defer s.logMu.RUnlock()
	buf := s.logs[vmID]
	if n <= 0 || n > len(buf) {
		n = len(buf)
	}
	start := len(buf) - n
	out := make([]string, n)
	copy(out, buf[start:])
	return out
}

// --- Global daemon (audit) log ring ---

func (s *Store) AppendDaemonLog(line string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.daemonLogs = append(s.daemonLogs, line)
	if len(s.daemonLogs) > daemonLogRingSize {
		s.daemonLogs = s.daemonLogs[len(s.daemonLogs)-daemonLogRingSize:]
	}
}

func (s *Store) TailDaemonLogs(n int) []string {
	s.logMu.RLock()
	defer s.logMu.RUnlock()
	if n <= 0 || n > len(s.daemonLogs) {
		n = len(s.daemonLogs)
	}
	return append([]string(nil), s.daemonLogs[len(s.daemonLogs)-n:]...)
}

func statusFromProject(p *types.Project) string {
	switch {
	case len(p.VMIDs) == 0:
		return "pending"
	case p.Source == "compose":
		return "deploying"
	default:
		return "running"
	}
}

var _ = pgx.ErrNoRows // keep pgx imported for future raw queries
var _ = time.Now      // keep time imported
