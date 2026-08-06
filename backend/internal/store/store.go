// Package store persists Porter state (VMs, projects, domains) in a
// single-file SQLite database, plus keeps an in-memory ring buffer of
// recent traffic per VM.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"porter/internal/types"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo — registers as "sqlite"
)

// Store persists VMs, projects, and domains in a local SQLite database
// (one file, no server, no admin/user tables — a single prototype admin
// account lives in Config instead).
//
// Each table is a simple id -> JSON-blob mapping rather than a fully
// normalized schema: the domain structs already carry JSON tags used
// throughout the API and dashboard, so storing them as JSON keeps this
// small and keeps "add a field to VM" a one-place change. SQLite still
// buys atomic single-file writes and safe concurrent reads.
type Store struct {
	db *sql.DB

	trafficMu sync.RWMutex
	traffic   map[string][]*types.TrafficEntry // vm_id -> ring buffer (in-memory only)

	logMu sync.RWMutex
	logs  map[string][]string // vm_id -> recent log lines ring buffer (in-memory only)
}

const schema = `
CREATE TABLE IF NOT EXISTS vms (
	id   TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
	id   TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS domains (
	vm_id  TEXT NOT NULL,
	domain TEXT NOT NULL,
	data   TEXT NOT NULL,
	PRIMARY KEY (vm_id, domain)
);
`

// NewStore opens (creating if needed) the SQLite database at path and
// ensures its schema exists. Fatal on failure.
func NewStore(path string) *Store {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("open sqlite store %q: %v", path, err)
	}
	// modernc.org/sqlite serializes access per-connection; a single
	// connection avoids SQLITE_BUSY errors from this process racing
	// itself.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("init sqlite schema in %q: %v", path, err)
	}
	return &Store{db: db, traffic: map[string][]*types.TrafficEntry{}, logs: map[string][]string{}}
}

// Close releases the underlying SQLite handle. The server process never
// calls this (the DB lives for the process lifetime); tests use it so a
// temp DB can be removed cleanly on Windows without a file-lock error.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// NewID returns a short random hex ID for a VM or project.
func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// --- VM operations ---

func (s *Store) PutVM(vm *types.VM) {
	data, err := json.Marshal(vm)
	if err != nil {
		log.Printf("store: marshal vm %s: %v", vm.ID, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO vms (id, data) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, vm.ID, data); err != nil {
		log.Printf("store: put vm %s: %v", vm.ID, err)
	}
}

func (s *Store) GetVM(id string) (*types.VM, bool) {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM vms WHERE id = ?`, id).Scan(&data)
	if err != nil {
		return nil, false
	}
	var vm types.VM
	if err := json.Unmarshal(data, &vm); err != nil {
		log.Printf("store: unmarshal vm %s: %v", id, err)
		return nil, false
	}
	return &vm, true
}

func (s *Store) ListVMs() []*types.VM {
	rows, err := s.db.Query(`SELECT data FROM vms`)
	if err != nil {
		log.Printf("store: list vms: %v", err)
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
	if _, err := s.db.Exec(`DELETE FROM vms WHERE id = ?`, id); err != nil {
		log.Printf("store: delete vm %s: %v", id, err)
	}
	if _, err := s.db.Exec(`DELETE FROM domains WHERE vm_id = ?`, id); err != nil {
		log.Printf("store: delete domains for vm %s: %v", id, err)
	}
	s.trafficMu.Lock()
	delete(s.traffic, id)
	s.trafficMu.Unlock()
	s.logMu.Lock()
	delete(s.logs, id)
	s.logMu.Unlock()
}

// --- Project operations ---

func (s *Store) PutProject(p *types.Project) {
	data, err := json.Marshal(p)
	if err != nil {
		log.Printf("store: marshal project %s: %v", p.ID, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO projects (id, data) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, p.ID, data); err != nil {
		log.Printf("store: put project %s: %v", p.ID, err)
	}
}

func (s *Store) GetProject(id string) (*types.Project, bool) {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM projects WHERE id = ?`, id).Scan(&data)
	if err != nil {
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
	rows, err := s.db.Query(`SELECT data FROM projects`)
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

func (s *Store) DeleteProject(id string) {
	if _, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id); err != nil {
		log.Printf("store: delete project %s: %v", id, err)
	}
}

// --- Domains ---

func (s *Store) AddDomain(vmID string, d *types.Domain) {
	d.VMID = vmID
	data, err := json.Marshal(d)
	if err != nil {
		log.Printf("store: marshal domain %s: %v", d.Domain, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO domains (vm_id, domain, data) VALUES (?, ?, ?)
		ON CONFLICT(vm_id, domain) DO UPDATE SET data = excluded.data`, vmID, d.Domain, data); err != nil {
		log.Printf("store: add domain %s for vm %s: %v", d.Domain, vmID, err)
	}
}

func (s *Store) ListDomains(vmID string) []*types.Domain {
	rows, err := s.db.Query(`SELECT data FROM domains WHERE vm_id = ? ORDER BY rowid`, vmID)
	if err != nil {
		log.Printf("store: list domains for vm %s: %v", vmID, err)
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

func (s *Store) RemoveDomain(vmID, domain string) bool {
	res, err := s.db.Exec(`DELETE FROM domains WHERE vm_id = ? AND domain = ?`, vmID, domain)
	if err != nil {
		log.Printf("store: remove domain %s for vm %s: %v", domain, vmID, err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// --- Traffic ring buffer (bounded, in-memory only) ---

const trafficRingSize = 200

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

// --- Log ring buffer (bounded, in-memory only) ---

const logRingSize = 500

// AppendLog records one VM log line in the ring buffer. In v0.1.0 the
// containerd VM manager calls this as guest output streams in.
func (s *Store) AppendLog(vmID, line string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	buf := append(s.logs[vmID], line)
	if len(buf) > logRingSize {
		buf = buf[len(buf)-logRingSize:]
	}
	s.logs[vmID] = buf
}

// TailLogs returns the last n log lines for a VM, most recent last.
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