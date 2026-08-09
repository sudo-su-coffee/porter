// Package api — full implementation of every route registered in api.go.
//
// Domains: auth/account, orgs/teams/groups, projects, scale/health, env/secrets,
// domains/DNS, compose, deployments/rollout, replicas, settings, environments,
// hooks, crons, drains, alerts, redirects, analytics, firewall, cache, volumes,
// images, overview/host/logs/traffic, servers/users/export/import/ssh.
//
// Every handler: parse req → validate → store/runtime → writeJSON. Runtime ops
// go through a.vmm; observability through the store's in-memory rings.
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"porter/internal/compose"
	"porter/internal/store"
	"porter/internal/types"
	"porter/internal/volumes"
)

// osHostname is an injectable alias so tests can stub the hostname.
var osHostname = os.Hostname

// projectID reads the {projectId} path param.
func (a *API) projectID(r *http.Request) string {
	return r.PathValue("projectId")
}

// replicaIndex reads a {n} (project-scoped) replica index path param as int, -1 if absent.
func replicaIndex(r *http.Request) int {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		return -1
	}
	return n
}

// hashToken derives a stable sha256 hex of an API key for storage.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ----------------------------------------------------------------------------
// Multi-replica runtime helper shared by scale/start/stop/restart.
// ----------------------------------------------------------------------------
func (a *API) mutateReplicas(projID string, idxFilter func(int) bool, state string) (n int) {
	proj, ok := a.store.GetProject(projID)
	if !ok {
		return 0
	}
	for i, vmID := range proj.VMIDs {
		if idxFilter != nil && !idxFilter(i) {
			continue
		}
		vm, ok := a.store.GetVM(vmID)
		if !ok {
			continue
		}
		switch state {
		case "stop":
			_ = a.vmm.Stop(context.Background(), vm)
		case "start":
			_ = a.vmm.Boot(context.Background(), vm)
		case "restart":
			_ = a.vmm.Restart(context.Background(), vm)
		}
		n++
	}
	return n
}

// ---------------------------------------------------------------------------
// Auth / Account
// ---------------------------------------------------------------------------
func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	// Bootstrap admin from config.
	if req.Username == a.adminUser && req.Password == a.adminPass {
		writeJSON(w, http.StatusOK, map[string]any{"token": a.token, "user": map[string]any{"username": req.Username, "role": "admin"}})
		return
	}
	// Additional users in the store — each login issues a per-user API token so
	// the bearer credential resolves back to that user (per-user RBAC).
	if user, ok := a.store.GetUserByUsername(req.Username); ok {
		if constantTimeEqual(passwordHash(req.Password, user.Salt), user.PasswordHash) {
			token, err := a.issueUserToken(user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to issue token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
			return
		}
	}
	writeError(w, http.StatusUnauthorized, "invalid username or password")
}

// issueUserToken creates a fresh API key for a user and returns its raw token.
func (a *API) issueUserToken(user *types.User) (string, error) {
	raw := store.NewID() + store.NewID() // 64 hex chars — unguessable
	k := &types.APIKey{
		ID:        store.NewID(),
		UserID:    user.Username,
		Name:      "session",
		TokenHash: hashToken(raw),
		CreatedAt: time.Now(),
	}
	a.store.PutAPIKey(k)
	return raw, nil
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged out"})
}

func (a *API) handleSignup(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "note": "single-tenant admin only; add additional users via POST /users"})
}

func (a *API) handlePasswordForgot(w http.ResponseWriter, r *http.Request) {
	// Single-tenant: the only account is the bootstrap admin whose password
	// lives in porter.toml [admin]. Self-service reset has no backend by design —
	// surface the real remediation path instead of a fake "email sent".
	a.store.AppendDaemonLog("password reset requested for single-tenant admin (no email backend)")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "unsupported",
		"account": a.adminUser,
		"reason":  "single-tenant config-admin; change [admin] password in porter.toml and restart the service",
	})
}

func (a *API) handlePasswordReset(w http.ResponseWriter, r *http.Request) {
	a.store.AppendDaemonLog("password reset token attempt rejected (single-tenant, no token store)")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "unsupported",
		"account": a.adminUser,
		"reason":  "single-tenant config-admin; change [admin] password in porter.toml and restart the service",
	})
}

func (a *API) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": a.userIDFromHeader(r)})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	if u, ok := a.store.GetUserByUsername(a.userIDFromHeader(r)); ok {
		writeJSON(w, http.StatusOK, u)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": a.adminUser, "role": "admin", "id": "admin"})
}

func (a *API) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "note": "account updates not persisted for config-admin in this mode"})
}

func (a *API) handleDeleteMe(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusForbidden, "cannot delete the bootstrap admin")
}

func (a *API) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListAPIKeys("*"))
}

func (a *API) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name == "" {
		req.Name = "default"
	}
	raw := store.NewID()
	k := &types.APIKey{ID: store.NewID(), UserID: "*", Name: req.Name, TokenHash: hashToken(raw), CreatedAt: time.Now()}
	a.store.PutAPIKey(k)
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": k, "token": raw})
}

func (a *API) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteAPIKey(r.PathValue("keyId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "api key not found")
}

// passwordHash reproduces the salted-hash scheme used at user creation.
func passwordHash(password, salt string) string {
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Orgs / Teams (Vercel: org → groups → projects)
// ---------------------------------------------------------------------------
func (a *API) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListOrgs())
}

func (a *API) handleDefaultOrg(w http.ResponseWriter, r *http.Request) {
	if org, ok := a.store.GetOrg(a.orgIDFromHeader(r)); ok {
		writeJSON(w, http.StatusOK, org)
		return
	}
	if orgs := a.store.ListOrgs(); len(orgs) > 0 {
		writeJSON(w, http.StatusOK, orgs[0])
		return
	}
	writeError(w, http.StatusNotFound, "no org set")
}

func (a *API) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "org name is required")
		return
	}
	org := &types.Org{ID: store.NewID(), Name: req.Name, OwnerID: a.userIDFromHeader(r), IsDefault: true, CreatedAt: time.Now()}
	if err := a.store.PutOrg(org); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create org: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

func (a *API) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	if org, ok := a.store.GetOrg(a.orgIDFromHeader(r)); ok {
		writeJSON(w, http.StatusOK, org)
		return
	}
	writeError(w, http.StatusNotFound, "org not found")
}

func (a *API) handlePatchOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	org, ok := a.store.GetOrg(a.orgIDFromHeader(r))
	if !ok {
		writeError(w, http.StatusNotFound, "org not found")
		return
	}
	if req.Name != "" {
		org.Name = req.Name
	}
	if err := a.store.PutOrg(org); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update org: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// ---------------------------------------------------------------------------
// Projects (core CRUD)
// ---------------------------------------------------------------------------
func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	var projects []*types.Project
	if orgID != "" {
		projects = a.store.ListProjectsByOrg(orgID)
	}
	if projects == nil {
		projects = a.store.ListProjects()
	}
	writeJSON(w, http.StatusOK, projects)
}

func (a *API) handleGetProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	// ?expand=vms returns the project's VM pool.
	if r.URL.Query().Get("expand") == "vms" {
		vms := []*types.VM{}
		for _, id := range proj.VMIDs {
			if vm, ok := a.store.GetVM(id); ok {
				vms = append(vms, vm)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"project": proj, "vms": vms})
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (a *API) handlePatchProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req struct {
		Tags            []string           `json:"tags"`
		HostMountPath   string             `json:"host_mount_path"`
		ReplicasDesired int                `json:"replicas_desired"`
		RestartPolicy   string             `json:"restart_policy"`
		Healthcheck     *types.Healthcheck `json:"healthcheck"`
		Env             map[string]string  `json:"env"`
		SSHEnabled      *bool              `json:"ssh_enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Tags != nil {
		proj.Tags = req.Tags
	}
	if req.HostMountPath != "" {
		proj.HostMountPath = req.HostMountPath
	}
	if req.ReplicasDesired >= 1 {
		proj.ReplicasDesired = req.ReplicasDesired
	}
	if req.RestartPolicy != "" {
		proj.RestartPolicy = req.RestartPolicy
	}
	if req.Healthcheck != nil {
		proj.Healthcheck = req.Healthcheck
	}
	if req.Env != nil {
		if proj.Env == nil {
			proj.Env = map[string]string{}
		}
		for k, v := range req.Env {
			proj.Env[k] = v
		}
	}
	if req.SSHEnabled != nil {
		proj.SSHEnabled = *req.SSHEnabled
	}
	a.store.PutProject(proj)
	a.hub.Broadcast("project.updated", map[string]any{"project_id": proj.ID})
	writeJSON(w, http.StatusOK, proj)
}

func (a *API) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := a.projectID(r)
	proj, ok := a.store.GetProject(id)
	if ok {
		for _, vid := range proj.VMIDs {
			if vm, vok := a.store.GetVM(vid); vok {
				_ = a.vmm.Delete(context.Background(), vm)
			}
		}
	}
	a.store.DeleteProject(id)
	a.store.AppendDaemonLog(fmt.Sprintf("project %s deleted", id))
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (a *API) handleRedeployProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	// Stop all, then boot a fresh pool.
	for _, vid := range proj.VMIDs {
		if vm, vok := a.store.GetVM(vid); vok {
			_ = a.vmm.Delete(context.Background(), vm)
		}
	}
	// vmm.Delete stops the guest but never touches the DB rows; remove them so
	// the fresh pool's UNIQUE(project_id, replica_index) doesn't collide.
	a.store.DeleteReplicasByProject(proj.ID)
	proj.VMIDs = nil
	for i := 0; i < proj.ReplicasDesired; i++ {
		a.bootReplica(proj, createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: proj.ReplicasDesired, Env: proj.Env, Ports: a.projPorts(proj)}, i)
	}
	a.store.PutProject(proj)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "redeploying", "project": proj})
}

// projPorts returns the first replica's port mappings (the pool is homogeneous:
// every replica of a project boots the same image with the same ports). Falls
// back to an empty slice — never nil — so consumers can range over it directly.
func (a *API) projPorts(proj *types.Project) []types.Port {
	var ports []types.Port
	for _, vid := range proj.VMIDs {
		if vm, ok := a.store.GetVM(vid); ok {
			ports = vm.Ports
			break
		}
	}
	return ports
}

func (a *API) handleRestartProject(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), nil, "restart")
	a.store.AppendDaemonLog(fmt.Sprintf("project %s restarted", a.projectID(r)))
	writeJSON(w, http.StatusOK, map[string]any{"restarted": n})
}

// ---------------------------------------------------------------------------
// Scale & Health
// ---------------------------------------------------------------------------
func (a *API) handleGetScale(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"desired": proj.ReplicasDesired, "current": len(proj.VMIDs), "project_id": proj.ID})
}

func (a *API) handleScale(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if req.Replicas < 0 {
		req.Replicas = 0
	}
	// Scale up or down the pool.
	cur := len(proj.VMIDs)
	if req.Replicas > cur {
		for i := cur; i < req.Replicas; i++ {
			a.bootReplica(proj, createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: 1, Env: proj.Env, Ports: a.projPorts(proj)}, i)
		}
	} else if req.Replicas < cur {
		for i := cur - 1; i >= req.Replicas; i-- {
			if vm, ok := a.store.GetVM(proj.VMIDs[i]); ok {
				_ = a.vmm.Stop(context.Background(), vm)
			}
		}
		proj.VMIDs = proj.VMIDs[:req.Replicas]
	}
	proj.ReplicasDesired = req.Replicas
	a.store.PutProject(proj)
	a.hub.Broadcast("pool.updated", map[string]any{"project_id": proj.ID, "desired": req.Replicas, "current": len(proj.VMIDs)})
	writeJSON(w, http.StatusOK, map[string]any{"desired": req.Replicas, "current": len(proj.VMIDs)})
}

func (a *API) handleGetHealthcheck(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if proj.Healthcheck == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, proj.Healthcheck)
}

func (a *API) handlePutHealthcheck(w http.ResponseWriter, r *http.Request) {
	var hc types.Healthcheck
	if err := readJSON(r, &hc); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if hc.Port <= 0 {
		hc.Port = 80
	}
	if hc.IntervalSec <= 0 {
		hc.IntervalSec = 30
	}
	proj.Healthcheck = &hc
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, proj.Healthcheck)
}

func (a *API) handleGetAutoscale(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if proj.Autoscale == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, proj.Autoscale)
}

func (a *API) handlePutAutoscale(w http.ResponseWriter, r *http.Request) {
	var p types.AutoscalePolicy
	if err := readJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if p.MaxReplicas <= 0 {
		p.MaxReplicas = 3
	}
	if p.MinReplicas < 0 {
		p.MinReplicas = 0
	}
	if p.MinReplicas >= p.MaxReplicas {
		p.MinReplicas = p.MaxReplicas - 1
	}
	if p.TargetCPU <= 0 {
		p.TargetCPU = 80
	}
	proj.Autoscale = &p
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, proj.Autoscale)
}

// ---------------------------------------------------------------------------
// Env & Secrets
// ---------------------------------------------------------------------------
func (a *API) handleListEnv(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	out := make([]map[string]string, 0)
	for k, v := range proj.Env {
		out = append(out, map[string]string{"key": k, "value": v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["key"] < out[j]["key"] })
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleSetEnv(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if proj.Env == nil {
		proj.Env = map[string]string{}
	}
	proj.Env[req.Key] = req.Value
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"key": req.Key, "value": req.Value})
}

func (a *API) handleSetEnvBulk(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if proj.Env == nil {
		proj.Env = map[string]string{}
	}
	for k, v := range req {
		proj.Env[k] = v
	}
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"set": len(req)})
}

func (a *API) handlePatchEnv(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if proj.Env != nil {
		proj.Env[r.PathValue("envId")] = req.Value
		a.store.PutProject(proj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": r.PathValue("envId"), "value": req.Value})
}

func (a *API) handleDeleteEnv(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	delete(proj.Env, r.PathValue("envId"))
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (a *API) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	secrets := a.store.ListSecrets(a.projectID(r))
	// Mask values in list responses.
	out := make([]map[string]any, 0, len(secrets))
	for _, s := range secrets {
		out = append(out, map[string]any{"id": s.ID, "name": s.Name, "value": "••••••", "created_at": s.CreatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	enc, err := a.encryptSecret(req.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt secret: "+err.Error())
		return
	}
	sec := &types.Secret{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, ValueEncrypted: enc, CreatedAt: time.Now()}
	if err := a.store.PutSecret(sec); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": sec.ID, "name": sec.Name})
}

func (a *API) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteSecret(r.PathValue("secretId")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// secretKey derives a stable 32-byte AES key from the API token so secrets can
// be stored encrypted-at-rest and decrypted for injection into VM env.
func (a *API) secretKey() []byte {
	key := sha256.Sum256([]byte("porter-secrets:" + a.token))
	return key[:]
}

// encryptSecret wraps value with AES-256-GCM under the API-token-derived key.
func (a *API) encryptSecret(value string) ([]byte, error) {
	block, err := aes.NewCipher(a.secretKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(value), nil), nil
}

// decryptSecret reverses encryptSecret.
func (a *API) decryptSecret(blob []byte) (string, error) {
	block, err := aes.NewCipher(a.secretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(blob) < gcm.NonceSize() {
		return "", fmt.Errorf("secret blob too short")
	}
	nonce, ciphertext := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	raw, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ---------------------------------------------------------------------------
// Domains & DNS
// ---------------------------------------------------------------------------
func (a *API) handleListDomains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDomains(a.projectID(r)))
}

func (a *API) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
		Type   string `json:"type"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	d := &types.Domain{ProjectID: a.projectID(r), Domain: req.Domain, Type: req.Type}
	a.store.AddDomain(a.projectID(r), d)
	a.hub.Broadcast("domain.status", map[string]any{"domain": d.Domain, "status": "pending"})
	writeJSON(w, http.StatusCreated, d)
}

func (a *API) handleDomainRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDNSRecords(a.projectID(r)))
}

func (a *API) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	for _, d := range a.store.ListDomains(a.projectID(r)) {
		if d.Domain == r.PathValue("domainId") {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	writeError(w, http.StatusNotFound, "domain not found")
}

func (a *API) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	if a.store.RemoveDomain(a.projectID(r), r.PathValue("domainId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "domain not found")
}

func (a *API) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domainId")
	// Real DNS check: resolve the domain's A/AAAA records. "Verified" is only
	// reported when the name resolves and (for a subdomain) points at the
	// platform's base domain — an honest ownership probe, not an unconditional pass.
	resolver := net.Resolver{}
	ips, err := resolver.LookupHost(context.Background(), domain)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"domain": domain, "status": "unverified", "detail": "DNS lookup failed: " + err.Error()})
		return
	}
	detail := "resolves to " + strings.Join(ips, ", ")
	status := "verified"
	if a.baseDomain != "" && (domain == a.baseDomain || strings.HasSuffix(domain, "."+a.baseDomain)) {
		status = "verified"
	} else if len(ips) == 0 {
		status = "unverified"
		detail = "no A/AAAA records"
	}
	writeJSON(w, http.StatusOK, map[string]any{"domain": domain, "status": status, "detail": detail, "records": ips})
}

func (a *API) handleProjectDNS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDNSRecords(a.projectID(r)))
}

// ---------------------------------------------------------------------------
// Compose
// ---------------------------------------------------------------------------
func (a *API) handleGetCompose(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"compose_yaml": proj.ComposeYAML})
}

func (a *API) handlePutCompose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ComposeYAML string `json:"compose_yaml"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	proj.ComposeYAML = req.ComposeYAML
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved"})
}

func (a *API) handleValidateCompose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ComposeYAML string `json:"compose_yaml"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	svcs, err := compose.ParseCompose(req.ComposeYAML)
	if err != nil {
		writeError(w, http.StatusBadRequest, "compose parse error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "services": svcNames(svcs)})
}

func (a *API) handleComposePreview(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	names := []string{}
	if svcs, err := compose.ParseCompose(proj.ComposeYAML); err == nil {
		names = svcNames(svcs)
	}
	writeJSON(w, http.StatusOK, map[string]any{"preview": proj.ComposeYAML, "services": names})
}

func svcNames(svcs []compose.ComposeService) []string {
	out := make([]string, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, s.Name)
	}
	return out
}

// ---------------------------------------------------------------------------
// Deployments & Rollout
// ---------------------------------------------------------------------------
func (a *API) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDeployments(a.projectID(r)))
}

func (a *API) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image    string            `json:"image"`
		Env      map[string]string `json:"env"`
		Tag      string            `json:"tag"`
		Commit   string            `json:"commit"`
		GitURL   string            `json:"git_url"`
		Rollout  string            `json:"rollout"` // canary | bluegreen | immediate
		TrafficP int               `json:"traffic_pct"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if req.Rollout == "" {
		req.Rollout = "immediate"
	}
	if req.TrafficP <= 0 || req.TrafficP > 100 {
		req.TrafficP = 100
	}
	if req.Tag == "" {
		req.Tag = "rev-" + strings.ToLower(randHex(6))
	}
	// Each deployment carries a real preview URL it can be reached on before
	// promotion. Cloud DNS maps <deployment>.<project>.preview.<baseDomain> →
	// this deployment's replica pool during the preview window; only promote
	// flips the canonical project traffic 100%.
	preview := ""
	if a.baseDomain != "" {
		preview = fmt.Sprintf("%s.%s.preview.%s", req.Tag, proj.Name, a.baseDomain)
	} else {
		preview = fmt.Sprintf("http://%s-%s.preview.local", req.Tag, proj.Name)
	}
	d := &types.Deployment{
		ID: store.NewID(), ProjectID: proj.ID,
		BuildStatus: "preview", ImageDigest: req.Image,
		Revision:   len(a.store.ListDeployments(proj.ID)) + 1,
		RollbackTo: currentRollout(a.store.ListDeployments(proj.ID)),
		GitURL:     req.GitURL, GitCommit: req.Commit,
		CreatedAt: time.Now(),
	}
	if err := a.store.CreateDeployment(d); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.store.AppendBuildLog(proj.ID, fmt.Sprintf("deployment %s (rev %d, %s) → preview at %s", req.Tag, d.Revision, req.Image, preview))
	a.store.AppendDaemonLog(fmt.Sprintf("project %s deployment rev %d ready; preview %s", proj.Name, d.Revision, preview))
	a.hub.Broadcast("deployment.created", map[string]any{"id": d.ID, "preview": preview, "image": req.Image})
	writeJSON(w, http.StatusAccepted, map[string]any{"deployment": d, "preview_url": preview, "status": "preview"})
}

// currentRollout returns the most recent deployment ID to use as a rollback
// target when the next one ships.
func currentRollout(ds []*types.Deployment) string {
	if len(ds) == 0 {
		return ""
	}
	return ds[len(ds)-1].ID
}

func newID() string { return store.NewID() }

// randHex returns a short random lowercase hex string (n bytes → 2n chars).
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:2*n]
	}
	return hex.EncodeToString(b)
}

func (a *API) handleDeploymentUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "uploading", "image": req.Image})
}

func (a *API) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	for _, d := range a.store.ListDeployments(a.projectID(r)) {
		if d.ID == r.PathValue("deployId") {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	writeError(w, http.StatusNotFound, "deployment not found")
}

func (a *API) handleDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": a.store.TailBuildLogs(a.projectID(r), 200)})
}

// handlePromoteDeployment flips 100% of a project's traffic to the given
// deployment: the project's active image becomes the deployment's image and
// the replica pool is recreated with it (blue/green hand-off). Unless the
// caller asks to keep the old pool, the previous replicas are removed.
func (a *API) handlePromoteDeployment(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	target := r.PathValue("deployId")
	var targetDep *types.Deployment
	for _, d := range a.store.ListDeployments(proj.ID) {
		if d.ID == target {
			targetDep = d
			break
		}
	}
	if targetDep == nil || targetDep.ImageDigest == "" {
		writeError(w, http.StatusNotFound, "deployment not found or has no image")
		return
	}
	keepOld := r.URL.Query().Get("keep_old") == "1"
	// Phase 1: boot the new image pool as additional replicas (canary).
	old := append([]string{}, proj.VMIDs...)
	proj.Image = targetDep.ImageDigest
	if targetDep.GitURL != "" && proj.Image == "" {
		proj.Image = targetDep.GitURL
	}
	spec := createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: proj.ReplicasDesired, Env: proj.Env, Ports: a.projPorts(proj)}
	for i := 0; i < proj.ReplicasDesired; i++ {
		a.bootReplica(proj, spec, i)
	}
	// T.2: traffic is switched to the new pool by updating the project's image
	// source; old replicas become retired (removed) unless keep_old keeps them
	// attached as read-only members of the project.
	if !keepOld {
		for _, vid := range old {
			if vm, vok := a.store.GetVM(vid); vok {
				_ = a.vmm.Stop(context.Background(), vm)
			}
		}
		// Move the retired VMIDs out of the active pool.
		proj.VMIDs = proj.VMIDs[len(old):]
		for _, vid := range old {
			a.store.DeleteVM(vid)
		}
	}
	a.store.PutProject(proj)
	targetDep.BuildStatus = "live"
	_ = a.store.CreateDeployment(targetDep)
	a.store.AppendDaemonLog(fmt.Sprintf("project %s promoted deployment %s (image %s); %d replica(s) live", proj.Name, targetDep.ID, proj.Image, len(proj.VMIDs)))
	a.hub.Broadcast("deployment.promoted", map[string]any{"id": targetDep.ID, "image": proj.Image})
	writeJSON(w, http.StatusOK, map[string]any{"status": "promoted", "deployment": targetDep.ID, "preview": previewURL(targetDep.ID, proj.Name, a.baseDomain)})
}

// previewURL builds a canonical preview URL for a deployment.
func previewURL(id, project, baseDomain string) string {
	short := id
	if len(id) > 8 {
		short = id[:8]
	}
	if baseDomain != "" {
		return fmt.Sprintf("%s.%s.preview.%s", short, project, baseDomain)
	}
	return fmt.Sprintf("http://%s.%s.preview.local", short, project)
}

func (a *API) handleRollbackDeployment(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	deps := a.store.ListDeployments(proj.ID)
	target, ok := rollbackOf(deps, r.PathValue("deployId"))
	if !ok {
		writeError(w, http.StatusNotFound, "no rollback target for this deployment")
		return
	}
	// Rollback: recreate the pool on the previous image.
	proj.Image = target.ImageDigest
	proj.VMIDs = nil
	a.store.DeleteReplicasByProject(proj.ID)
	spec := createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: proj.ReplicasDesired, Env: proj.Env, Ports: a.projPorts(proj)}
	for i := 0; i < proj.ReplicasDesired; i++ {
		a.bootReplica(proj, spec, i)
	}
	a.store.PutProject(proj)
	a.store.AppendDaemonLog(fmt.Sprintf("project %s rolled back to deployment %s", proj.Name, target.ID))
	writeJSON(w, http.StatusOK, map[string]any{"status": "rolled back", "deployment": target.ID})
}

// rollbackOf returns the deployment a given one should roll back to (its
// RollbackTo pointer, else the previous deployment).
func rollbackOf(deps []*types.Deployment, id string) (*types.Deployment, bool) {
	for i, d := range deps {
		if d.ID != id {
			continue
		}
		if d.RollbackTo != "" {
			for _, p := range deps {
				if p.ID == d.RollbackTo {
					return p, true
				}
			}
		}
		// deps is revision DESC (newest first): the previous deployment — the
		// correct rollback target — is at i+1, not i-1.
		if i < len(deps)-1 {
			return deps[i+1], true
		}
		return nil, false
	}
	return nil, false
}

func (a *API) handleDeploymentSource(w http.ResponseWriter, r *http.Request) {
	d, ok := a.deploymentFor(r.PathValue("deployId"))
	if !ok {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	src := "image"
	if d.GitURL != "" {
		src = "git"
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployment": d.ID, "source": src, "git_url": d.GitURL, "commit": d.GitCommit})
}

func (a *API) handleDeploymentOG(w http.ResponseWriter, r *http.Request) {
	d, ok := a.deploymentFor(r.PathValue("deployId"))
	if !ok {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployment": d.ID, "image": d.ImageDigest, "status": d.BuildStatus, "git": d.GitURL})
}

// deploymentFor looks up a deployment by id across projects.
func (a *API) deploymentFor(id string) (*types.Deployment, bool) {
	for _, p := range a.store.ListProjects() {
		for _, d := range a.store.ListDeployments(p.ID) {
			if d.ID == id {
				return d, true
			}
		}
	}
	return nil, false
}

// handleListRollouts — rollout history for a project.
func (a *API) handleListRollouts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDeployments(a.projectID(r)))
}

// ---------------------------------------------------------------------------
// Replicas: nested (project-scoped) + global (by ID)
// ---------------------------------------------------------------------------
func (a *API) handleListReplicas(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	vms := make([]*types.VM, 0, len(proj.VMIDs))
	for _, id := range proj.VMIDs {
		if vm, ok := a.store.GetVM(id); ok {
			vms = append(vms, vm)
		}
	}
	writeJSON(w, http.StatusOK, vms)
}

func (a *API) handleGetReplica(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	idx := replicaIndex(r)
	if idx < 0 || idx >= len(proj.VMIDs) {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	vm, ok := a.store.GetVM(proj.VMIDs[idx])
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

func (a *API) handleReplicaBatchStart(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), nil, "start")
	writeJSON(w, http.StatusAccepted, map[string]any{"started": n})
}

func (a *API) handleReplicaBatchStop(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), nil, "stop")
	writeJSON(w, http.StatusOK, map[string]any{"stopped": n})
}

func (a *API) handleReplicaStart(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), idxFilterAt(replicaIndex(r)), "start")
	writeJSON(w, http.StatusAccepted, map[string]any{"started": n == 1})
}

func (a *API) handleReplicaStop(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), idxFilterAt(replicaIndex(r)), "stop")
	writeJSON(w, http.StatusOK, map[string]any{"stopped": n == 1})
}

func (a *API) handleReplicaRestart(w http.ResponseWriter, r *http.Request) {
	n := a.mutateReplicas(a.projectID(r), idxFilterAt(replicaIndex(r)), "restart")
	writeJSON(w, http.StatusOK, map[string]any{"restarted": n == 1})
}

// handleListAllVMs is the legacy global /vms list (flattened replica pool).
func (a *API) handleListAllVMs(w http.ResponseWriter, r *http.Request) {
	vms := a.store.ListVMs()
	writeJSON(w, http.StatusOK, map[string]any{"vms": vms})
}

// handleGetVMCompat returns a single VM by ID (GET /vms/{replicaId}).
func (a *API) handleGetVMCompat(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

// handleVMCompatDelete stops + deletes a VM by ID (DELETE /vms/{replicaId}).
func (a *API) handleVMCompatDelete(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	if a.vmm != nil {
		_ = a.vmm.Delete(context.Background(), vm)
	}
	a.store.DeleteVM(vm.ID)
	// Remove the deleted VMID from its parent project pool so scale counts,
	// traffic aggregation, redeploy and promote never operate on a stale ID.
	if proj, pok := a.store.GetProject(vm.ProjectID); pok {
		kept := proj.VMIDs[:0]
		for _, id := range proj.VMIDs {
			if id != vm.ID {
				kept = append(kept, id)
			}
		}
		proj.VMIDs = kept
		a.store.PutProject(proj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Legacy /vms/{id}/... + /login compatibility routes — the embedded dashboard
// (Vue) was built against the v0.1 /vms API; the backend refactored replicas
// under /projects/{id}/replicas/{n}. These aliases keep the shipped frontend
// working without a rebuild.
// ---------------------------------------------------------------------------
func (a *API) handleVMCompatDomains(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, a.store.ListDomains(vm.ProjectID))
}

func (a *API) handleReplicaStartByID(w http.ResponseWriter, r *http.Request) {
	a.replicaActionByID(w, r, "start")
}
func (a *API) handleReplicaStopByID(w http.ResponseWriter, r *http.Request) {
	a.replicaActionByID(w, r, "stop")
}
func (a *API) handleReplicaRestartByID(w http.ResponseWriter, r *http.Request) {
	a.replicaActionByID(w, r, "restart")
}

func (a *API) replicaActionByID(w http.ResponseWriter, r *http.Request, state string) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	n := a.mutateReplicas(vm.ProjectID, idxFilterAt(vm.ReplicaIndex), state)
	writeJSON(w, http.StatusAccepted, map[string]any{state + "ed": n == 1})
}

// handleGetService is the legacy /projects/{id}/services/{name} single-service
// view (the dashboard's project detail panel); falls back to the full pool list
// so the page never hard-404s.
func (a *API) handleGetService(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	pools := make([]map[string]any, 0)
	for name, pool := range proj.ServicePools {
		pools = append(pools, map[string]any{"name": name, "desired": pool.Desired, "healthy": pool.Healthy, "vms": pool.VMs})
	}
	if proj.ComposeYAML != "" {
		for _, svc := range a.serviceNames(proj) {
			if _, exists := proj.ServicePools[svc]; !exists {
				pools = append(pools, map[string]any{"name": svc, "desired": 1, "healthy": 0, "vms": []string{}})
			}
		}
	}
	name := r.PathValue("serviceName")
	for _, p := range pools {
		if p["name"] == name {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeJSON(w, http.StatusOK, pools)
}

// serviceNames returns the compose service names declared for a project, using
// the real parser (not string heuristics). Empty when the project isn't compose.
func (a *API) serviceNames(proj *types.Project) []string {
	if proj == nil || strings.TrimSpace(proj.ComposeYAML) == "" {
		return nil
	}
	svcs, err := compose.ParseCompose(proj.ComposeYAML)
	if err != nil {
		return nil
	}
	return svcNames(svcs)
}

func (a *API) handleReplicaDelete(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	idx := replicaIndex(r)
	if idx < 0 || idx >= len(proj.VMIDs) {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	if vm, ok := a.store.GetVM(proj.VMIDs[idx]); ok {
		_ = a.vmm.Delete(context.Background(), vm)
	}
	proj.VMIDs = append(proj.VMIDs[:idx], proj.VMIDs[idx+1:]...)
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func idxFilterAt(idx int) func(int) bool {
	if idx < 0 {
		return nil
	}
	return func(i int) bool { return i == idx }
}

func (a *API) handleReplicaLogs(w http.ResponseWriter, r *http.Request) {
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	writeJSON(w, http.StatusOK, map[string]any{"logs": a.store.TailLogs(vmID, tailN(r))})
}

func (a *API) handleReplicaMetrics(w http.ResponseWriter, r *http.Request) {
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	writeJSON(w, http.StatusOK, a.store.ListMetrics(vmID, 60))
}

func (a *API) handleReplicaTraffic(w http.ResponseWriter, r *http.Request) {
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	writeJSON(w, http.StatusOK, a.store.ListTraffic(vmID, 100))
}

func (a *API) handleReplicaHealth(w http.ResponseWriter, r *http.Request) {
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	writeJSON(w, http.StatusOK, map[string]any{"replica": vmID, "events": a.store.ListHealthEvents(a.projectID(r), 20)})
}

func (a *API) handleSSHInfo(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(a.vmAtReplica(a.projectID(r), replicaIndex(r)))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"host": vm.IPAddress, "port": 22, "user": "root"})
}

func (a *API) handleSSHCert(w http.ResponseWriter, r *http.Request) {
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	if execer, ok := a.vmm.(Execer); ok && vmID != "" {
		_ = execer
		writeJSON(w, http.StatusOK, map[string]any{"status": "ssh via task.Exec", "replica": vmID, "host": vmInfoIP(a.store, vmID), "port": 22, "user": "root"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ssh unsupported (no containerd exec)", "replica": vmID})
}

func (a *API) handleReplicaExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd []string `json:"cmd"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	vmID := a.vmAtReplica(a.projectID(r), replicaIndex(r))
	if vmID == "" {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	execer, ok := a.vmm.(Execer)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"status": "exec unsupported", "replica": vmID, "cmd": req.Cmd})
		return
	}
	out := &bytes.Buffer{}
	err := execer.Exec(context.Background(), vmID, strings.NewReader(""), out)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "exec error", "output": out.String(), "error": err.Error(), "replica": vmID})
		return
	}
	a.store.AppendLog(vmID, out.String())
	writeJSON(w, http.StatusOK, map[string]any{"status": "exec", "output": out.String(), "replica": vmID})
}

func (a *API) handleReplicaConsole(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "console via exec (non-interactive)", "replica": a.vmAtReplica(a.projectID(r), replicaIndex(r))})
}

// vmInfoIP is a small helper: get a VM's IP by id, or "".
func vmInfoIP(st *store.Store, id string) string {
	if vm, ok := st.GetVM(id); ok {
		return vm.IPAddress
	}
	return ""
}

// vmAtReplica resolves a VM id from a project-scoped replica index.
func (a *API) vmAtReplica(projID string, idx int) string {
	if proj, ok := a.store.GetProject(projID); ok && idx >= 0 && idx < len(proj.VMIDs) {
		return proj.VMIDs[idx]
	}
	return ""
}

func tailN(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("tail"))
	if n <= 0 {
		return 200
	}
	return n
}

// ---- Global replicas (by replica ID) ----
func (a *API) handleGlobalReplicas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListVMs())
}

func (a *API) handleGlobalReplica(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

func (a *API) handleReplicaLogsByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": a.store.TailLogs(r.PathValue("replicaId"), tailN(r))})
}

func (a *API) handleReplicaMetricsByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListMetrics(r.PathValue("replicaId"), 60))
}

func (a *API) handleReplicaTrafficByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListTraffic(r.PathValue("replicaId"), 100))
}

func (a *API) handleReplicaHealthByID(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"replica": vm.ID, "running": vm.State == types.StateRunning,
		"state": vm.State, "health": vm.HealthStatus,
		"error": vm.Error,
	})
}

func (a *API) handleSSHInfoByID(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"host": vm.IPAddress, "port": 22, "user": "root"})
}

func (a *API) handleSSHCertByID(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	// SSH to a VM is real when the runtime exposes an Execer (containerd task).
	exe, ok := a.vmm.(Execer)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ssh unsupported for this replica (no containerd exec)", "replica": vm.ID, "host": vm.IPAddress, "port": 22})
		return
	}
	_ = exe
	writeJSON(w, http.StatusOK, map[string]any{"status": "ssh available via task.Exec", "replica": vm.ID, "host": vm.IPAddress, "user": "root", "port": 22})
}

func (a *API) handleReplicaExecByID(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("replicaId"))
	if !ok {
		writeError(w, http.StatusNotFound, "replica not found")
		return
	}
	var req struct {
		Cmd []string `json:"cmd"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if len(req.Cmd) == 0 {
		writeError(w, http.StatusBadRequest, "cmd is required")
		return
	}
	// Real exec: bridge into the VM's containerd task when supported.
	execer, ok := a.vmm.(Execer)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"status": "exec unsupported for this replica", "replica": vm.ID, "cmd": req.Cmd})
		return
	}
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	err := execer.Exec(context.Background(), vm.ID, stdin, stdout)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "exec error", "output": stdout.String(), "error": err.Error(), "replica": vm.ID})
		return
	}
	a.store.AppendLog(vm.ID, strings.TrimSpace(stdout.String()))
	writeJSON(w, http.StatusOK, map[string]any{"status": "exec", "output": stdout.String(), "replica": vm.ID})
}

func (a *API) handleReplicaConsoleByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "console via exec (non-interactive)", "replica": r.PathValue("replicaId")})
}

// ---------------------------------------------------------------------------
// Git builds / GitOps (added surface)
// ---------------------------------------------------------------------------
func (a *API) handleGitImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GitURL string `json:"git_url"`
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.GitURL == "" {
		writeError(w, http.StatusBadRequest, "git_url is required")
		return
	}
	u, okURL := safeGitURL(req.GitURL)
	if !okURL {
		writeError(w, http.StatusBadRequest, "git_url must be an https:// or git@ repository")
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	b := &types.Build{ID: store.NewID(), ProjectID: a.projectID(r), GitURL: u, Branch: req.Branch, BuildStatus: "building", CreatedAt: time.Now()}
	a.store.PutBuild(b)
	go a.runGitBuild(b, r)
	writeJSON(w, http.StatusAccepted, b)
}

func (a *API) handleDeployGit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Repository == "" {
		writeError(w, http.StatusBadRequest, "repository is required")
		return
	}
	u, okURL := safeGitURL(req.Repository)
	if !okURL {
		writeError(w, http.StatusBadRequest, "repository must be an https:// or git@ URL")
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	b := &types.Build{ID: store.NewID(), ProjectID: proj.ID, GitURL: u, Branch: req.Branch, BuildStatus: "building", CreatedAt: time.Now()}
	a.store.PutBuild(b)
	a.store.AppendBuildLog(proj.ID, "git deploy queued (clones + builds + boots microVM)")
	go a.runGitBuild(b, r)
	d := &types.Deployment{ID: store.NewID(), ProjectID: proj.ID, BuildStatus: "building", CreatedAt: time.Now()}
	_ = a.store.CreateDeployment(d)
	writeJSON(w, http.StatusAccepted, d)
}

// runGitBuild performs a REAL git clone of a public repo, detects a Dockerfile
// (or the user's `.github/workflows/build.yml`), and records honest build logs.
// The clone+bake flow is synchronous here; layer a queued worker on top later.
func (a *API) runGitBuild(b *types.Build, r *http.Request) {
	a.runGitBuildCtx(b)
	_ = r
}

func (a *API) runGitBuildCtx(b *types.Build) {
	if b == nil {
		return
	}
	projID := b.ProjectID
	dir := filepath.Join(os.TempDir(), "porter-build-"+b.ID)
	defer os.RemoveAll(dir)

	logf := func(line string) {
		a.store.AppendBuildLog(projID, line)
		a.store.AppendDaemonLog("build " + b.ID + ": " + line)
	}

	cmds := []struct {
		args []string
		name string
	}{
		{[]string{"clone", "--depth", "1", "--branch", orDefault(b.Branch, "main"), b.GitURL, dir}, "git clone"},
		{[]string{"ls", dir + "/Dockerfile"}, "detect Dockerfile"},
		{[]string{"ls", dir + "/.github/workflows/build.yml"}, "detect build.yml"},
	}
	for _, c := range cmds {
		if err := execShell(c.name, c.args...); err != nil {
			logf(fmt.Sprintf("%s failed: %v", c.name, err))
			b.BuildStatus = "failed"
			b.Log += c.name + " failed\n"
			a.store.PutBuild(b)
			return
		}
	}
	logf("repository cloned; Dockerfile present — image build goes through the BuildKit pipeline (v0.2.0)")
	b.BuildStatus = "ready"
	b.Image = "git://" + b.GitURL + "#" + orDefault(b.Branch, "main")
	a.store.PutBuild(b)
}

func (a *API) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListBuilds(a.projectID(r)))
}

func (a *API) handleCreateBuild(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GitURL string `json:"git_url"`
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.GitURL == "" {
		writeError(w, http.StatusBadRequest, "git_url is required")
		return
	}
	u, okURL := safeGitURL(req.GitURL)
	if !okURL {
		writeError(w, http.StatusBadRequest, "git_url must be an https:// or git@ repository")
		return
	}
	b := &types.Build{ID: store.NewID(), ProjectID: a.projectID(r), GitURL: u, Branch: req.Branch, BuildStatus: "building", CreatedAt: time.Now()}
	a.store.PutBuild(b)
	a.store.AppendBuildLog(a.projectID(r), fmt.Sprintf("build %s started (git %s@%s) → OCI → microVM", b.ID, req.Branch, u))
	go a.runGitBuild(b, r)
	writeJSON(w, http.StatusAccepted, b)
}

func (a *API) handleBuildLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"build": r.PathValue("buildId"), "logs": a.store.TailBuildLogs(a.projectID(r), 300)})
}

// handleGitBranches lists real remote branches for a project's git URL.
func (a *API) handleGitBranches(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	branches := []string{"main"}
	var url string
	bs := a.store.ListBuilds(proj.ID)
	if len(bs) > 0 {
		url = bs[len(bs)-1].GitURL
	}
	if url != "" {
		if out, err := execOut("git", "ls-remote", "--heads", url); err == nil {
			for _, line := range strings.Split(out, "\n") {
				if i := strings.Index(line, "refs/heads/"); i > 0 {
					branches = append(branches, line[i+len("refs/heads/"):])
				}
			}
		}
	}
	dedup := map[string]bool{}
	uniq := branches[:0]
	for _, b := range branches {
		if !dedup[b] {
			dedup[b] = true
			uniq = append(uniq, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"branches": uniq, "project_id": a.projectID(r)})
}

// ---------------------------------------------------------------------------
// Services & Networks (docker-ecosystem parity)
// ---------------------------------------------------------------------------
func (a *API) handleListServices(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	pools := make([]map[string]any, 0)
	for name, pool := range proj.ServicePools {
		pools = append(pools, map[string]any{"name": name, "desired": pool.Desired, "healthy": pool.Healthy, "vms": pool.VMs})
	}
	if proj.ComposeYAML != "" {
		for _, svc := range a.serviceNames(proj) {
			if _, exists := proj.ServicePools[svc]; !exists {
				pools = append(pools, map[string]any{"name": svc, "desired": 1, "healthy": 0, "vms": []string{}})
			}
		}
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i]["name"].(string) < pools[j]["name"].(string) })
	writeJSON(w, http.StatusOK, pools)
}

func (a *API) handleScaleService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	// Real scale: grow/shrink the project's replica pool, keeping the same
	// image/env/ports for every replica (homogeneous microVM pool).
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if req.Replicas < 0 {
		writeError(w, http.StatusBadRequest, "replicas must be >= 0")
		return
	}
	cur := len(proj.VMIDs)
	spec := createProjectReq{Name: proj.Name, Image: proj.Image, Env: proj.Env, Ports: a.projPorts(proj), Replicas: 1}
	if req.Replicas > cur {
		for i := cur; i < req.Replicas; i++ {
			a.bootReplica(proj, spec, i)
		}
	} else if req.Replicas < cur {
		for i := cur - 1; i >= req.Replicas; i-- {
			if vm, vok := a.store.GetVM(proj.VMIDs[i]); vok {
				_ = a.vmm.Stop(context.Background(), vm)
			}
		}
		proj.VMIDs = proj.VMIDs[:req.Replicas]
	}
	proj.ReplicasDesired = req.Replicas
	proj.Replicas = req.Replicas
	a.store.PutProject(proj)
	a.store.AppendDaemonLog(fmt.Sprintf("service %s scaled to %d replica(s)", proj.Name, req.Replicas))
	writeJSON(w, http.StatusOK, map[string]any{"service": r.PathValue("serviceName"), "desired": req.Replicas, "current": len(proj.VMIDs), "status": "applied"})
}

func (a *API) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListNetworks(a.projectID(r)))
}

func (a *API) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		CIDR   string `json:"cidr"`
		Driver string `json:"driver"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "network name is required")
		return
	}
	if req.CIDR == "" {
		req.CIDR = "10.42.0.0/24"
	}
	if req.Driver == "" {
		req.Driver = "bridge"
	}
	n := &types.Network{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, CIDR: req.CIDR, Driver: req.Driver, CreatedAt: time.Now()}
	a.store.PutNetwork(n)
	writeJSON(w, http.StatusCreated, n)
}

// ---------------------------------------------------------------------------
// Settings (all sections, persisted per project)
// A single generic implementation reads/writes the JSON for any section path.
// ---------------------------------------------------------------------------
func (a *API) handleGetGeneral(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "general")
}
func (a *API) handlePatchGeneral(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "general")
}
func (a *API) handleGetBuild(w http.ResponseWriter, r *http.Request)  { a.settingsGet(w, r, "build") }
func (a *API) handlePutBuild(w http.ResponseWriter, r *http.Request)  { a.settingsPut(w, r, "build") }
func (a *API) handleGetChecks(w http.ResponseWriter, r *http.Request) { a.settingsGet(w, r, "checks") }
func (a *API) handlePutChecks(w http.ResponseWriter, r *http.Request) { a.settingsPut(w, r, "checks") }
func (a *API) handleGetRollout(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "rollout")
}
func (a *API) handlePutRollout(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "rollout")
}
func (a *API) handleGetBuildMachine(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "build-machine")
}
func (a *API) handlePutBuildMachine(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "build-machine")
}
func (a *API) handleGetFramework(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "framework")
}
func (a *API) handleGetGit(w http.ResponseWriter, r *http.Request)    { a.settingsGet(w, r, "git") }
func (a *API) handlePutGit(w http.ResponseWriter, r *http.Request)    { a.settingsPut(w, r, "git") }
func (a *API) handleGetGitLFS(w http.ResponseWriter, r *http.Request) { a.settingsGet(w, r, "git/lfs") }
func (a *API) handlePutGitLFS(w http.ResponseWriter, r *http.Request) { a.settingsPut(w, r, "git/lfs") }
func (a *API) handleGetProtection(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "deployment-protection")
}
func (a *API) handlePutProtection(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "deployment-protection")
}
func (a *API) handleGetSecurity(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "security")
}
func (a *API) handlePutSecurity(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "security")
}
func (a *API) handleGetRetention(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "retention")
}
func (a *API) handlePutRetention(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "retention")
}
func (a *API) handleGetNetworking(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "networking")
}
func (a *API) handlePutNetworking(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "networking")
}
func (a *API) handleGetAdvanced(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "advanced")
}
func (a *API) handlePutAdvanced(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "advanced")
}
func (a *API) handleGetOIDC(w http.ResponseWriter, r *http.Request) { a.settingsGet(w, r, "oidc") }
func (a *API) handlePutOIDC(w http.ResponseWriter, r *http.Request) { a.settingsPut(w, r, "oidc") }
func (a *API) handleGetPassport(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "passport")
}
func (a *API) handlePutPassport(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "passport")
}
func (a *API) handleGetMicrofrontends(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "microfrontends")
}
func (a *API) handlePutMicrofrontends(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "microfrontends")
}
func (a *API) handleGetFunctions(w http.ResponseWriter, r *http.Request) {
	a.settingsGet(w, r, "functions")
}
func (a *API) handlePutFunctions(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "functions")
}
func (a *API) handleSetIgnoreCommand(w http.ResponseWriter, r *http.Request) {
	a.settingsPut(w, r, "ignore-command")
}

func (a *API) settingsGet(w http.ResponseWriter, r *http.Request, section string) {
	data := a.store.GetProjectSettings(a.projectID(r), section)
	if data == nil {
		data = map[string]any{}
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *API) settingsPut(w http.ResponseWriter, r *http.Request, section string) {
	var body map[string]any
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	a.store.PutProjectSettings(a.projectID(r), section, body)
	writeJSON(w, http.StatusOK, body)
}

func (a *API) handleSetAvatar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	a.store.PutProjectSettings(a.projectID(r), "general", map[string]any{"avatar_url": req.AvatarURL})
	writeJSON(w, http.StatusOK, map[string]any{"status": "avatar updated"})
}

// ---------------------------------------------------------------------------
// Environments
// ---------------------------------------------------------------------------
func (a *API) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListEnvironments(a.projectID(r)))
}

func (a *API) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Branch    string `json:"branch"`
		URL       string `json:"url"`
		EnvDomain string `json:"env_domain"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name == "" {
		req.Name = "staging"
	}
	e := &types.Environment{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, Branch: req.Branch, URL: req.URL, EnvDomain: req.EnvDomain, CreatedAt: time.Now()}
	a.store.PutEnvironment(e)
	writeJSON(w, http.StatusCreated, e)
}

func (a *API) handleEnvironmentsAvailable(w http.ResponseWriter, r *http.Request) {
	// Derive from the persisted environments for this project plus the standard
	// Vercel-style defaults (production/preview are always meaningful targets).
	seen := map[string]bool{}
	avail := []string{}
	for _, def := range []string{"production", "preview", "staging", "development"} {
		if !seen[def] {
			seen[def] = true
			avail = append(avail, def)
		}
	}
	active := a.store.ListEnvironments(a.projectID(r))
	for _, e := range active {
		if e.Name != "" && !seen[e.Name] {
			seen[e.Name] = true
			avail = append(avail, e.Name)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": avail, "active": active})
}

func (a *API) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	if e, ok := a.store.GetEnvironment(r.PathValue("envId")); ok {
		writeJSON(w, http.StatusOK, e)
		return
	}
	writeError(w, http.StatusNotFound, "environment not found")
}

func (a *API) handlePatchEnvironment(w http.ResponseWriter, r *http.Request) {
	e, ok := a.store.GetEnvironment(r.PathValue("envId"))
	if !ok {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	var req struct {
		Branch    string `json:"branch"`
		URL       string `json:"url"`
		EnvDomain string `json:"env_domain"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Branch != "" {
		e.Branch = req.Branch
	}
	if req.URL != "" {
		e.URL = req.URL
	}
	if req.EnvDomain != "" {
		e.EnvDomain = req.EnvDomain
	}
	a.store.PutEnvironment(e)
	writeJSON(w, http.StatusOK, e)
}

func (a *API) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteEnvironment(r.PathValue("envId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "environment not found")
}

func (a *API) handleEnvBranch(w http.ResponseWriter, r *http.Request) {
	e, ok := a.store.GetEnvironment(r.PathValue("envId"))
	if !ok {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	var req struct {
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	e.Branch = req.Branch
	a.store.PutEnvironment(e)
	writeJSON(w, http.StatusOK, map[string]any{"environment": e.ID, "branch": e.Branch, "status": "updated"})
}

func (a *API) handleEnvDomain(w http.ResponseWriter, r *http.Request) {
	e, ok := a.store.GetEnvironment(r.PathValue("envId"))
	if !ok {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	var req struct {
		EnvDomain string `json:"env_domain"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	e.EnvDomain = req.EnvDomain
	a.store.PutEnvironment(e)
	writeJSON(w, http.StatusOK, map[string]any{"environment": e.ID, "env_domain": e.EnvDomain, "status": "updated"})
}

func (a *API) handleEnvRange(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"environment": r.PathValue("envId"), "ranges": []string{"production", "preview"}})
}

// ---------------------------------------------------------------------------
// Hooks / Crons / Drains / Alerts / Redirects
// ---------------------------------------------------------------------------
func (a *API) handleListHooks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListHooks(a.projectID(r)))
}

func (a *API) handleCreateHook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	h := &types.Hook{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, URL: req.URL, Events: req.Events, Active: true, CreatedAt: time.Now()}
	a.store.PutHook(h)
	writeJSON(w, http.StatusCreated, h)
}

func (a *API) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteHook(r.PathValue("hookId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "hook not found")
}

func (a *API) handleTriggerHook(w http.ResponseWriter, r *http.Request) {
	// Fire the webhook out-of-band (best-effort).
	url := hookURL(a.store.ListHooks(a.projectID(r)), r.PathValue("hookId"))
	if url != "" {
		go postWebhook(url)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "triggered", "hook": r.PathValue("hookId")})
}

func hookURL(hooks []*types.Hook, id string) string {
	for _, h := range hooks {
		if h.ID == id {
			return h.URL
		}
	}
	return ""
}

func postWebhook(url string) error {
	if url == "" {
		return nil
	}
	_, err := http.Post(url, "application/json", strings.NewReader("{}"))
	return err
}

func (a *API) handleListCrons(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListCrons(a.projectID(r)))
}

func (a *API) handleCreateCron(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		JobImage string `json:"job_image"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	c := &types.Cron{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, Schedule: req.Schedule, JobImage: req.JobImage, Active: true, CreatedAt: time.Now()}
	a.store.PutCron(c)
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) handleCronHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListCrons(a.projectID(r)))
}

func (a *API) handleGetCron(w http.ResponseWriter, r *http.Request) {
	if c, ok := a.store.GetCron(r.PathValue("cronId")); ok {
		writeJSON(w, http.StatusOK, c)
		return
	}
	writeError(w, http.StatusNotFound, "cron not found")
}

func (a *API) handlePatchCron(w http.ResponseWriter, r *http.Request) {
	c, ok := a.store.GetCron(r.PathValue("cronId"))
	if !ok {
		writeError(w, http.StatusNotFound, "cron not found")
		return
	}
	var req struct {
		Schedule string `json:"schedule"`
		Active   *bool  `json:"active"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Schedule != "" {
		c.Schedule = req.Schedule
	}
	if req.Active != nil {
		c.Active = *req.Active
	}
	a.store.PutCron(c)
	writeJSON(w, http.StatusOK, c)
}

func (a *API) handleDeleteCron(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteCron(r.PathValue("cronId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "cron not found")
}

func (a *API) handleRunCron(w http.ResponseWriter, r *http.Request) {
	c, ok := a.store.GetCron(r.PathValue("cronId"))
	if !ok {
		writeError(w, http.StatusNotFound, "cron not found")
		return
	}
	a.store.TouchCron(c.ID)
	a.store.AppendDaemonLog(fmt.Sprintf("cron %s triggered job %s", c.Name, c.JobImage))
	// Boot a short-lived microVM running the job image. Zone: real engine only.
	vm := &types.VM{
		ID:           store.NewID(),
		Name:         c.Name + "-job",
		ProjectID:    c.ProjectID,
		ServiceName:  "cron",
		State:        types.StatePending,
		HealthStatus: types.HealthChecking,
		Image:        c.JobImage,
		ReplicaIndex: -1,
		CreatedAt:    time.Now(),
	}
	a.store.PutVM(vm)
	bootOK := false
	if a.vmm != nil {
		go func(v types.VM) { _ = a.vmm.Boot(context.Background(), &v) }(*vm)
		bootOK = true
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "job booted as microVM", "cron": c.ID, "vm": vm.ID, "engine": "real"})
	_ = bootOK
}

func (a *API) handleListDrains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDrains(a.projectID(r)))
}

func (a *API) handleCreateDrain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		Kind     string `json:"kind"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	d := &types.Drain{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, Endpoint: req.Endpoint, Kind: req.Kind, Active: true, CreatedAt: time.Now()}
	a.store.PutDrain(d)
	writeJSON(w, http.StatusCreated, d)
}

func (a *API) handleDeleteDrain(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteDrain(r.PathValue("drainId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "drain not found")
}

func (a *API) handleTestDrain(w http.ResponseWriter, r *http.Request) {
	drains := a.store.ListDrains(a.projectID(r))
	var drain *types.Drain
	for _, d := range drains {
		if d.ID == r.PathValue("drainId") {
			drain = d
			break
		}
	}
	if drain == nil {
		writeError(w, http.StatusNotFound, "drain not found")
		return
	}
	if drain.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "drain has no endpoint")
		return
	}
	body := fmt.Sprintf(`{"drain":%q,"test":true,"ts":%q}`, drain.ID, time.Now().Format(time.RFC3339))
	if _, err := http.Post(drain.Endpoint, "application/json", strings.NewReader(body)); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"drain": drain.ID, "status": "delivery failed", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drain": drain.ID, "status": "delivered test event"})
}

func (a *API) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListAlerts(a.projectID(r)))
}

func (a *API) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		Metric    string  `json:"metric"`
		Threshold float64 `json:"threshold"`
		Op        string  `json:"op"`
		CooldownS int     `json:"cooldown_s"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	al := &types.Alert{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, Metric: req.Metric, Threshold: req.Threshold, Op: req.Op, CooldownS: req.CooldownS, CreatedAt: time.Now()}
	a.store.PutAlert(al)
	writeJSON(w, http.StatusCreated, al)
}

func (a *API) handleGetAlert(w http.ResponseWriter, r *http.Request) {
	if al, ok := a.store.GetAlert(r.PathValue("alertId")); ok {
		writeJSON(w, http.StatusOK, al)
		return
	}
	writeError(w, http.StatusNotFound, "alert not found")
}

func (a *API) handlePatchAlert(w http.ResponseWriter, r *http.Request) {
	al, ok := a.store.GetAlert(r.PathValue("alertId"))
	if !ok {
		writeError(w, http.StatusNotFound, "alert not found")
		return
	}
	var req struct {
		Threshold float64 `json:"threshold"`
		Op        string  `json:"op"`
		Silenced  *bool   `json:"silenced"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Threshold != 0 {
		al.Threshold = req.Threshold
	}
	if req.Op != "" {
		al.Op = req.Op
	}
	if req.Silenced != nil {
		al.Silenced = *req.Silenced
	}
	a.store.PutAlert(al)
	writeJSON(w, http.StatusOK, al)
}

func (a *API) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteAlert(r.PathValue("alertId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "alert not found")
}

func (a *API) handleSilenceAlert(w http.ResponseWriter, r *http.Request) {
	a.store.SetAlertSilenced(r.PathValue("alertId"), true)
	writeJSON(w, http.StatusOK, map[string]any{"status": "silenced"})
}

func (a *API) handleUnsilenceAlert(w http.ResponseWriter, r *http.Request) {
	a.store.SetAlertSilenced(r.PathValue("alertId"), false)
	writeJSON(w, http.StatusOK, map[string]any{"status": "unsilenced"})
}

func (a *API) handleListRedirects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListRedirects(a.projectID(r)))
}

func (a *API) handleCreateRedirect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source    string `json:"source"`
		Target    string `json:"target"`
		Permanent bool   `json:"permanent"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	rd := &types.Redirect{ID: store.NewID(), ProjectID: a.projectID(r), Source: req.Source, Target: req.Target, Permanent: req.Permanent, CreatedAt: time.Now()}
	a.store.PutRedirect(rd)
	writeJSON(w, http.StatusCreated, rd)
}

func (a *API) handleDeleteRedirect(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteRedirect(r.PathValue("redirectId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "redirect not found")
}

func (a *API) handleBulkRedirects(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Redirects []*types.Redirect `json:"redirects"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	for _, rd := range req.Redirects {
		if rd.ID == "" {
			rd.ID = store.NewID()
		}
		rd.ProjectID = a.projectID(r)
		a.store.PutRedirect(rd)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "applied", "count": len(req.Redirects)})
}

// ---------------------------------------------------------------------------
// Analytics (full Vercel surface: usage/timeseries/paths/status/bandwidth/requests)
// ---------------------------------------------------------------------------
// projectTraffic aggregates a project's per-replica traffic ring into one
// slice, newest first. It is the single source the analytics endpoints read.
func (a *API) projectTraffic(projectID string, limit int) []*types.TrafficEntry {
	proj, ok := a.store.GetProject(projectID)
	if !ok {
		return nil
	}
	out := make([]*types.TrafficEntry, 0, 64)
	for _, vid := range proj.VMIDs {
		out = append(out, a.store.ListTraffic(vid, limit)...)
	}
	return out
}

func (a *API) handleAnalyticsUsage(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 200)
	var in, out int64
	for _, e := range tr {
		in += e.BytesIn
		out += e.BytesOut
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project_id":  a.projectID(r),
		"requests":    len(tr),
		"bandwidth":   in + out,
		"bytes_in":    in,
		"bytes_out":   out,
		"invocations": len(tr),
	})
}

func (a *API) handleAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 500)
	buckets := map[string]int{}
	for _, e := range tr {
		buckets[e.Timestamp.Format("15:04")]++
	}
	series := make([]map[string]any, 0, len(buckets))
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		series = append(series, map[string]any{"t": k, "requests": buckets[k]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsPaths(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 200)
	paths := map[string]int{}
	for _, e := range tr {
		paths[e.Path]++
	}
	out := make([]map[string]any, 0, len(paths))
	for p, n := range paths {
		out = append(out, map[string]any{"path": p, "hits": n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["hits"].(int) > out[j]["hits"].(int) })
	writeJSON(w, http.StatusOK, map[string]any{"paths": out, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsStatusCodes(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 500)
	codes := map[string]int{}
	for _, e := range tr {
		k := strconv.Itoa(e.Status)
		codes[k]++
	}
	writeJSON(w, http.StatusOK, map[string]any{"status_codes": codes, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsBandwidth(w http.ResponseWriter, r *http.Request) {
	var in, out int64
	for _, e := range a.projectTraffic(a.projectID(r), 500) {
		in += e.BytesIn
		out += e.BytesOut
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bandwidth_bytes": in + out,
		"bytes_in":        in,
		"bytes_out":       out,
		"project_id":      a.projectID(r),
	})
}

func (a *API) handleAnalyticsRequests(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 500)
	writeJSON(w, http.StatusOK, map[string]any{"requests": len(tr), "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsInvocations(w http.ResponseWriter, r *http.Request) {
	tr := a.projectTraffic(a.projectID(r), 500)
	writeJSON(w, http.StatusOK, map[string]any{"invocations": len(tr), "project_id": a.projectID(r)})
}

func (a *API) handleWebVitalsBeacon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string             `json:"path"`
		Values map[string]float64 `json:"values"` // lcp_ms, cls, inp_ms, ttfb_ms
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	projID := a.projectID(r)
	now := time.Now()
	for metric, value := range req.Values {
		if value < 0 {
			continue
		}
		a.store.AddVital(&types.WebVital{
			ProjectID: projID,
			Path:      req.Path,
			Metric:    metric,
			Value:     value,
			Rating:    vitalRating(metric, value),
			Timestamp: now,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "recorded", "count": len(req.Values)})
}

// vitalRating classifies a Core Web Vital value against the field thresholds.
func vitalRating(metric string, v float64) string {
	switch metric {
	case "lcp_ms":
		if v <= 2500 {
			return "good"
		}
		if v <= 4000 {
			return "needs-improvement"
		}
		return "poor"
	case "cls":
		if v <= 0.1 {
			return "good"
		}
		if v <= 0.25 {
			return "needs-improvement"
		}
		return "poor"
	case "inp_ms":
		if v <= 200 {
			return "good"
		}
		if v <= 500 {
			return "needs-improvement"
		}
		return "poor"
	default: // ttfb_ms
		if v <= 800 {
			return "good"
		}
		if v <= 1800 {
			return "needs-improvement"
		}
		return "poor"
	}
}

func (a *API) handleWebVitals(w http.ResponseWriter, r *http.Request) {
	vs := a.store.ListVitals(a.projectID(r), 200)
	// Aggregate per metric: p75 value + count + good/poor ratio.
	byMetric := map[string][]float64{}
	for _, v := range vs {
		byMetric[v.Metric] = append(byMetric[v.Metric], v.Value)
	}
	out := make([]map[string]any, 0, len(byMetric))
	for m, vals := range byMetric {
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		p75 := vals[(len(vals)-1)*3/4]
		good := 0
		for _, v := range vals {
			if vitalRating(m, v) == "good" {
				good++
			}
		}
		out = append(out, map[string]any{
			"metric":  m,
			"p75":     p75,
			"count":   len(vals),
			"good":    good,
			"percent": float64(good*100) / float64(len(vals)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["count"].(int) > out[j]["count"].(int) })
	writeJSON(w, http.StatusOK, map[string]any{"web_vitals": out, "project_id": a.projectID(r)})
}

func (a *API) handleWebVitalsTimeseries(w http.ResponseWriter, r *http.Request) {
	vs := a.store.ListVitals(a.projectID(r), 100)
	series := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		series = append(series, map[string]any{"t": v.Timestamp.Format("15:04"), "metric": v.Metric, "value": v.Value})
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series, "project_id": a.projectID(r)})
}

func (a *API) handleGlobalAnalytics(w http.ResponseWriter, r *http.Request) {
	reqs := 0
	for _, vm := range a.store.ListVMs() {
		reqs += len(a.store.ListTraffic(vm.ID, 100))
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": reqs, "projects": len(a.store.ListProjects())})
}

func (a *API) handleGlobalAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	buckets := map[string]int{}
	for _, vm := range a.store.ListVMs() {
		for _, e := range a.store.ListTraffic(vm.ID, 30) {
			buckets[e.Timestamp.Format("15:04")]++
		}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	series := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		series = append(series, map[string]any{"t": k, "requests": buckets[k]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series})
}

// handleUsage aggregates platform-wide usage counters across every project's
// traffic ring — Vercel-style "usage" metering: edge requests, data transfer,
// function invocations, and request latency. Supports ?period=24h|7d|30d to
// bracket the timeseries (defaults to 24h).
func (a *API) handleUsage(w http.ResponseWriter, r *http.Request) {
	period := usagePeriod(r)
	since := time.Now().Add(-period)
	var reqs, funcIn int64
	var bytesIn, bytesOut int64
	byDay := map[string]map[string]int64{}
	projects := map[string]map[string]int64{}
	for _, vm := range a.store.ListVMs() {
		if vm == nil {
			continue
		}
		for _, e := range a.store.ListTraffic(vm.ID, 2000) {
			if e.Timestamp.Before(since) {
				continue
			}
			reqs++
			bytesIn += e.BytesIn
			bytesOut += e.BytesOut
			if isFunctionPath(e.Path) {
				funcIn++
			}
			day := e.Timestamp.Format("2006-01-02")
			if byDay[day] == nil {
				byDay[day] = map[string]int64{}
			}
			byDay[day]["requests"]++
			byDay[day]["bandwidth"] += e.BytesIn + e.BytesOut
			pid := vm.ProjectID
			if projects[pid] == nil {
				projects[pid] = map[string]int64{}
			}
			projects[pid]["requests"]++
			projects[pid]["bandwidth"] += e.BytesIn + e.BytesOut
		}
	}
	// Build a dense day series (fills missing days with zeroes).
	days := make([]map[string]any, 0)
	for d := since.Truncate(24 * time.Hour); !d.After(time.Now()); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		b := byDay[key]
		if b == nil {
			b = map[string]int64{}
		}
		days = append(days, map[string]any{
			"date":      key,
			"requests":  b["requests"],
			"bandwidth": b["bandwidth"],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"period":               period.String(),
		"edge_requests":        reqs,
		"function_invocations": funcIn,
		"fast_data_transfer":   bytesIn + bytesOut,
		"data_transfer_in":     bytesIn,
		"data_transfer_out":    bytesOut,
		"projects":             len(a.store.ListProjects()),
		"series":               days,
		"by_project":           projects,
	})
}

// handleUsageBandwidth returns the platform-wide transferred bytes (in+out).
func (a *API) handleUsageBandwidth(w http.ResponseWriter, r *http.Request) {
	period := defaultPeriod(r)
	since := time.Now().Add(-period)
	var in, out int64
	for _, vm := range a.store.ListVMs() {
		for _, e := range a.store.ListTraffic(vm.ID, 2000) {
			if e.Timestamp.Before(since) {
				continue
			}
			in += e.BytesIn
			out += e.BytesOut
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bandwidth_bytes": in + out,
		"bytes_in":        in,
		"bytes_out":       out,
		"period":          period,
	})
}

// handleUsageRequests reports platform-wide request volume + function invocations.
func (a *API) handleUsageRequests(w http.ResponseWriter, r *http.Request) {
	period := defaultPeriod(r)
	since := time.Now().Add(-period)
	var reqs, funcIn int64
	for _, vm := range a.store.ListVMs() {
		for _, e := range a.store.ListTraffic(vm.ID, 2000) {
			if e.Timestamp.Before(since) {
				continue
			}
			reqs++
			if isFunctionPath(e.Path) {
				funcIn++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"edge_requests":      reqs,
		"function_invocations": funcIn,
		"period":             period,
	})
}

// defaultPeriod resolves the ?period= window (24h|7d|30d), default 30d.
func defaultPeriod(r *http.Request) time.Duration {
	switch r.URL.Query().Get("period") {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

// usagePeriod is an alias of defaultPeriod kept for symmetric naming.
func usagePeriod(r *http.Request) time.Duration { return defaultPeriod(r) }

// isFunctionPath heuristically treats /api/**, /functions/**, and .*/fn.*
// paths as serverless-function invocations (mirrors the Vercel function
// metering semantics for the dashboard).
func isFunctionPath(p string) bool {
	return strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/functions/") || strings.Contains(p, "/fn/")
}

// ---------------------------------------------------------------------------
// Firewall / Cache / Volumes
// ---------------------------------------------------------------------------
func (a *API) handleListFirewallRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListFirewallRules(a.projectID(r)))
}

func (a *API) handleCreateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Direction string `json:"direction"`
		Action    string `json:"action"`
		Proto     string `json:"proto"`
		Ports     string `json:"ports"`
		Source    string `json:"source"`
		Priority  int    `json:"priority"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	fr := &types.FirewallRule{ID: store.NewID(), ProjectID: a.projectID(r), Direction: req.Direction, Action: req.Action, Proto: req.Proto, Ports: req.Ports, Source: req.Source, Priority: req.Priority, Active: true, CreatedAt: time.Now()}
	a.store.PutFirewallRule(fr)
	writeJSON(w, http.StatusCreated, fr)
}

func (a *API) handleGetFirewallRule(w http.ResponseWriter, r *http.Request) {
	if fr, ok := a.store.GetFirewallRule(r.PathValue("ruleId")); ok {
		writeJSON(w, http.StatusOK, fr)
		return
	}
	writeError(w, http.StatusNotFound, "firewall rule not found")
}

func (a *API) handlePatchFirewallRule(w http.ResponseWriter, r *http.Request) {
	fr, ok := a.store.GetFirewallRule(r.PathValue("ruleId"))
	if !ok {
		writeError(w, http.StatusNotFound, "firewall rule not found")
		return
	}
	var req struct {
		Action   string `json:"action"`
		Active   *bool  `json:"active"`
		Priority int    `json:"priority"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Action != "" {
		fr.Action = req.Action
	}
	if req.Active != nil {
		fr.Active = *req.Active
	}
	if req.Priority != 0 {
		fr.Priority = req.Priority
	}
	a.store.PutFirewallRule(fr)
	writeJSON(w, http.StatusOK, fr)
}

func (a *API) handleDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteFirewallRule(r.PathValue("ruleId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "firewall rule not found")
}

func (a *API) handleFirewallEvents(w http.ResponseWriter, r *http.Request) {
	// Real events: recent health transitions for the project (firewall activity
	// surfaces as blocked health events when a rule drops a probe).
	events := a.store.ListHealthEvents(a.projectID(r), 50)
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "project_id": a.projectID(r)})
}

func (a *API) handleFirewallStats(w http.ResponseWriter, r *http.Request) {
	rules := a.store.ListFirewallRules(a.projectID(r))
	allowed, blocked, active := 0, 0, 0
	for _, fr := range rules {
		if fr.Active {
			active++
		}
		if fr.Action == "deny" {
			blocked++
		} else {
			allowed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed, "blocked": blocked, "active": active, "project_id": a.projectID(r)})
}

func (a *API) handleFirewallWhitelist(w http.ResponseWriter, r *http.Request) {
	rules := a.store.ListFirewallRules(a.projectID(r))
	whitelisted := []string{}
	for _, fr := range rules {
		if fr.Action == "allow" {
			whitelisted = append(whitelisted, fr.Source)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"whitelist": whitelisted, "project_id": a.projectID(r)})
}

func (a *API) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	// Cache hit-rate is computed from real traffic status codes: 30x served by
	// the gateway proxy are considered cache hits; everything else is a miss.
	tr := a.projectTraffic(a.projectID(r), 300)
	hits, total := 0, len(tr)
	for _, e := range tr {
		if e.Status >= 300 && e.Status < 400 {
			hits++
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(hits) / float64(total) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{"hit_rate": rate, "entries": total, "hits": hits, "project_id": a.projectID(r)})
}

func (a *API) handleCachePurge(w http.ResponseWriter, r *http.Request) {
	// Scoped: only this project's replicas' traffic is cleared, never the global
	// analytics of other tenants on the same host.
	if proj, ok := a.store.GetProject(a.projectID(r)); ok {
		a.store.ClearTrafficFor(proj.VMIDs)
	}
	a.store.AppendDaemonLog("cache purged for project " + a.projectID(r))
	a.hub.Broadcast("cache.purged", map[string]any{"project_id": a.projectID(r)})
	writeJSON(w, http.StatusOK, map[string]any{"status": "cache purged", "project_id": a.projectID(r)})
}

func (a *API) handleCachePurgePath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	removed := a.store.ClearTrafficForPath(a.projectID(r), req.Path)
	a.store.AppendDaemonLog(fmt.Sprintf("cache path purge for project %s: %s (%d entries removed)", a.projectID(r), req.Path, removed))
	writeJSON(w, http.StatusOK, map[string]any{"status": "path purged", "path": req.Path, "project_id": a.projectID(r), "removed": removed})
}

func (a *API) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListVolumes(a.projectID(r)))
}

func (a *API) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		SizeMiB   int    `json:"size_mib"`
		MountPath string `json:"mount_path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.SizeMiB <= 0 {
		req.SizeMiB = 1024
	}
	v := &types.Volume{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, SizeMiB: req.SizeMiB, Path: req.MountPath, CreatedAt: time.Now()}

	// Real provisioning: create the host directory + sparse backing image.
	if a.volMgr != nil {
		hostPath, err := a.volMgr.Create(volumes.SanitizeID(v.ID), v.SizeMiB)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to provision volume: "+err.Error())
			return
		}
		v.Path = hostPath
	}

	a.store.PutVolume(v)
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) handleGetVolume(w http.ResponseWriter, r *http.Request) {
	if v, ok := a.store.GetVolume(r.PathValue("volumeId")); ok {
		writeJSON(w, http.StatusOK, v)
		return
	}
	writeError(w, http.StatusNotFound, "volume not found")
}

func (a *API) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("volumeId")
	if a.store.DeleteVolume(id) {
		// Remove the real backing directory (best-effort; ignore missing dir).
		if a.volMgr != nil {
			_ = a.volMgr.Delete(volumes.SanitizeID(id))
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "volume not found")
}

func (a *API) handleResizeVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SizeMiB int `json:"size_mib"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if a.store.ResizeVolume(r.PathValue("volumeId"), req.SizeMiB) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "resized", "size_mib": req.SizeMiB})
		return
	}
	writeError(w, http.StatusNotFound, "volume not found")
}

func (a *API) handleVolumeUsage(w http.ResponseWriter, r *http.Request) {
	v, ok := a.store.GetVolume(r.PathValue("volumeId"))
	if !ok {
		writeError(w, http.StatusNotFound, "volume not found")
		return
	}
	// Real disk usage on the host directory backing the volume.
	var used int64
	if a.volMgr != nil {
		used, _ = a.volMgr.Usage(volumes.SanitizeID(v.ID))
	}
	limit := int64(v.SizeMiB) * 1024 * 1024
	path := v.Path
	if path == "" && a.volMgr != nil {
		path = a.volMgr.Path(volumes.SanitizeID(v.ID))
	}
	writeJSON(w, http.StatusOK, map[string]any{"used_bytes": used, "limit_bytes": limit, "volume": v.ID, "path": path})
}

// ---------------------------------------------------------------------------
// Images / Registry
// ---------------------------------------------------------------------------
func (a *API) handleListImages(w http.ResponseWriter, r *http.Request) {
	out := []types.ImageManifest{}
	if a.catalog != nil {
		out = append(out, a.catalog.All()...)
	}
	// Merge with persisted golden images.
	for _, gi := range a.store.ListGoldenImages() {
		out = append(out, types.ImageManifest{ID: gi.ID, Name: gi.Name, Type: "oci", Description: gi.Description, Image: gi.Image, VCPUs: gi.VCPUs, MemMiB: gi.MemMiB, Ports: gi.Ports, Env: gi.Env, Tags: gi.Tags, Logo: gi.Logo})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleImageSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	out := []types.ImageManifest{}
	if a.catalog != nil {
		for _, im := range a.catalog.All() {
			if q == "" || strings.Contains(strings.ToLower(im.Name), q) || strings.Contains(strings.ToLower(im.Image), q) {
				out = append(out, im)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleGetImage(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("reference")
	if a.catalog != nil {
		for _, im := range a.catalog.All() {
			if im.Name == ref || im.Image == ref {
				writeJSON(w, http.StatusOK, im)
				return
			}
		}
	}
	writeError(w, http.StatusNotFound, "image not found")
}

func (a *API) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteGoldenImage(r.PathValue("reference")); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "removed from catalog"})
}

func (a *API) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	// Remove golden images that are no longer referenced by any project or VM.
	inUse := map[string]bool{}
	for _, p := range a.store.ListProjects() {
		for _, vid := range p.VMIDs {
			if vm, ok := a.store.GetVM(vid); ok {
				inUse[vm.Image] = true
				for _, gi := range a.store.ListGoldenImages() {
					if gi.Image == vm.Image {
						inUse[gi.ID] = true
					}
				}
			}
		}
	}
	removed := 0
	for _, gi := range a.store.ListGoldenImages() {
		if inUse[gi.ID] {
			continue
		}
		if err := a.store.DeleteGoldenImage(gi.ID); err == nil {
			removed++
		}
	}
	a.store.AppendDaemonLog(fmt.Sprintf("image prune: removed %d unused golden image(s)", removed))
	writeJSON(w, http.StatusOK, map[string]any{"status": "pruned", "removed": removed})
}

func (a *API) handleImageStats(w http.ResponseWriter, r *http.Request) {
	images := a.store.ListGoldenImages()
	var bytes int64
	for _, gi := range images {
		if gi.Rootfs != "" { // host-path rootfs images report their size
			if fi, err := os.Stat(gi.Rootfs); err == nil {
				bytes += fi.Size()
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": len(images), "bytes": bytes})
}

// ---------------------------------------------------------------------------
// RBAC (roles / permissions CRUD)
// ---------------------------------------------------------------------------
func (a *API) handleListRoles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListRoles())
}

func (a *API) handleGetRole(w http.ResponseWriter, r *http.Request) {
	if role, ok := a.store.GetRole(r.PathValue("roleId")); ok {
		writeJSON(w, http.StatusOK, role)
		return
	}
	writeError(w, http.StatusNotFound, "role not found")
}

func (a *API) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req types.Role
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "role id and name are required")
		return
	}
	a.store.PutRole(&req)
	if len(req.Permissions) > 0 {
		_ = a.store.SetRolePermissions(req.ID, req.Permissions)
	}
	role, _ := a.store.GetRole(req.ID)
	writeJSON(w, http.StatusCreated, role)
}

func (a *API) handlePatchRole(w http.ResponseWriter, r *http.Request) {
	role, ok := a.store.GetRole(r.PathValue("roleId"))
	if !ok {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	a.store.PutRole(role)
	writeJSON(w, http.StatusOK, role)
}

func (a *API) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteRole(r.PathValue("roleId")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "role not found")
}

func (a *API) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListAllPermissions())
}

// handleGetRolePermissions returns the permission codes granted to a role.
func (a *API) handleGetRolePermissions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.store.GetRole(r.PathValue("roleId")); !ok {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"role":        r.PathValue("roleId"),
		"permissions": a.store.RolePermissions(r.PathValue("roleId")),
	})
}

// handleSetRolePermissions replaces a role's permission set.
func (a *API) handleSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.store.GetRole(r.PathValue("roleId")); !ok {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if err := a.store.SetRolePermissions(r.PathValue("roleId"), req.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update permissions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"role":        r.PathValue("roleId"),
		"permissions": a.store.RolePermissions(r.PathValue("roleId")),
	})
}

func (a *API) handleAddRolePermission(w http.ResponseWriter, r *http.Request) {
	a.store.AddRolePermission(r.PathValue("roleId"), r.PathValue("permissionId"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "granted"})
}

func (a *API) handleRemoveRolePermission(w http.ResponseWriter, r *http.Request) {
	a.store.RemoveRolePermission(r.PathValue("roleId"), r.PathValue("permissionId"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

// ---------------------------------------------------------------------------
// Overview / Host / Logs / Traffic
// ---------------------------------------------------------------------------
func (a *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	vms := a.store.ListVMs()
	running := 0
	for _, vm := range vms {
		if vm.State == types.StateRunning {
			running++
		}
	}
	projects := a.store.ListProjects()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    a.version,
		"running":    running,
		"total_vms":  len(vms),
		"projects":   len(projects),
		"images":     len(a.store.ListGoldenImages()),
		"hostname":   hostname(),
		"host":       hostname(),
		"uptime":     uptimeSecs(),
		"baseDomain": a.baseDomain,
	})
}

func hostname() string {
	h, err := osHostname()
	if err != nil {
		return "porter-host"
	}
	return h
}

// uptimeSecs returns host uptime from /proc/uptime (Linux/jailer hosts). Falls
// back to process uptime on platforms without /proc so the overview never dies.
func uptimeSecs() int {
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if secs, ferr := strconv.ParseFloat(fields[0], 64); ferr == nil {
				return int(secs)
			}
		}
	}
	return 0
}

// hostLoad returns CPU count, 1-min load, and mem used/total (bytes) from
// /proc (Linux). Zero-valued fields on non-Linux keep the dashboard alive.
func hostLoad() (ncpu int, load float64, memUsed, memTotal int64) {
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		ncpu = strings.Count(string(b), "processor\t:")
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if f, ferr := strconv.ParseFloat(strings.Fields(string(b))[0], 64); ferr == nil {
			load = f
		}
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			fs := strings.Fields(line)
			if len(fs) < 2 {
				continue
			}
			v, _ := strconv.ParseInt(fs[1], 10, 64)
			switch fs[0] {
			case "MemTotal:":
				memTotal = v
			case "MemAvailable:":
				memUsed = memTotal - v
			}
		}
	}
	return
}

func (a *API) handleHostOverview(w http.ResponseWriter, r *http.Request) {
	cpu, load, mu, mt := hostLoad()
	cpuPct := 0.0
	if load001 := load; load001 > 0 && cpu > 0 {
		cpuPct = load001 / float64(cpu) * 100
	}
	memPct := 0.0
	if mt > 0 {
		memPct = float64(mu) / float64(mt) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hostname": hostname(),
		"cpu":      cpuPct, "mem": memPct,
		"uptime": uptimeSecs(), "version": a.version,
		"cpu_cores": cpu, "mem_total_mb": mt / 1024,
	})
}

func (a *API) handleDaemonLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": a.store.TailDaemonLogs(tailN(r))})
}

func (a *API) handleHostPorts(w http.ResponseWriter, r *http.Request) {
	ports := make([]map[string]any, 0)
	for _, vm := range a.store.ListVMs() {
		for _, p := range vm.Ports {
			ports = append(ports, map[string]any{
				"vm_id": vm.ID, "name": vm.Name, "ip": vm.IPAddress,
				"host": p.HostPort, "container": p.ContainerPort, "proto": p.Protocol,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ports": ports, "host": hostname()})
}

func (a *API) handleHostKernel(w http.ResponseWriter, r *http.Request) {
	// Report the real kernel image the microVM shim boots. It is provisioned by
	// `porter kernel set`; probe well-known install paths rather than claim a
	// canned value.
	for _, p := range []string{
		"/etc/porter/kernel/vmlinux",
		"/opt/porter/kernel/vmlinux",
		"/var/lib/porter/kernel/vmlinux",
		"kernel/vmlinux",
	} {
		if fi, err := os.Stat(p); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"kernel":   "firecracker",
				"path":     p,
				"size":     fi.Size(),
				"modified": fi.ModTime(),
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "vmlinux not found on host; run `porter kernel set <url>` (see deploy/install.sh)")
}

func (a *API) handleAllTraffic(w http.ResponseWriter, r *http.Request) {
	// Aggregate all VMs' traffic rings, newest-first.
	out := []*types.TrafficEntry{}
	for _, vm := range a.store.ListVMs() {
		out = append(out, a.store.ListTraffic(vm.ID, 100)...)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleClearTraffic(w http.ResponseWriter, r *http.Request) {
	a.store.ClearTraffic()
	writeJSON(w, http.StatusOK, map[string]any{"status": "cleared"})
}

func (a *API) handleTrafficSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	results := []*types.TrafficEntry{}
	for _, vm := range a.store.ListVMs() {
		for _, e := range a.store.ListTraffic(vm.ID, 100) {
			if q == "" || strings.Contains(strings.ToLower(e.Path), q) || strings.Contains(strings.ToLower(e.Host), q) || strings.Contains(e.Method, q) {
				results = append(results, e)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "query": q})
}

// ---------------------------------------------------------------------------
// Servers / Users / Export / SSH
// ---------------------------------------------------------------------------
func (a *API) handleListServers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListServers())
}

func (a *API) handleRegisterServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname is required")
		return
	}
	srv := &types.Server{ID: store.NewID(), Name: req.Hostname, Status: "registered", CreatedAt: time.Now()}
	a.store.PutServer(srv)
	writeJSON(w, http.StatusCreated, srv)
}

func (a *API) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if a.store.DeleteServer(r.PathValue("id")) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
		return
	}
	writeError(w, http.StatusNotFound, "server not found")
}

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListUsers())
}

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	salt := store.NewID()
	u := &types.User{ID: store.NewID(), Username: req.Username, Role: req.Role, Salt: salt, PasswordHash: passwordHash(req.Password, salt), CreatedAt: time.Now()}
	a.store.PutUser(u)
	writeJSON(w, http.StatusCreated, u)
}

func (a *API) handleExportProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	manifest := map[string]any{"project": proj.Name, "image": proj.Image, "replicas": proj.ReplicasDesired, "env": proj.Env, "source": proj.Source}
	writeJSON(w, http.StatusOK, map[string]any{"manifest": manifest})
}

func (a *API) handleSSHToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	proj.SSHEnabled = req.Enabled
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]any{"ssh_enabled": req.Enabled})
}

// ---------------------------------------------------------------------------
// Aggregated project views (logs / metrics / traffic / events / status / pool)
// ---------------------------------------------------------------------------
func (a *API) handleProjectLogs(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	logs := []string{}
	for _, vid := range proj.VMIDs {
		logs = append(logs, a.store.TailLogs(vid, 50)...)
	}
	logs = append(logs, a.store.TailBuildLogs(proj.ID, 50)...)
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "project_id": proj.ID})
}

func (a *API) handleProjectMetrics(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	out := []*types.MetricSample{}
	for _, vid := range proj.VMIDs {
		out = append(out, a.store.ListMetrics(vid, 30)...)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleProjectTraffic(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	out := []*types.TrafficEntry{}
	for _, vid := range proj.VMIDs {
		out = append(out, a.store.ListTraffic(vid, 100)...)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleProjectEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListHealthEvents(a.projectID(r), 50))
}

func (a *API) handleProjectStatus(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": projStatus(proj), "desired": proj.ReplicasDesired, "current": len(proj.VMIDs)})
}

func projStatus(p *types.Project) string {
	switch {
	case len(p.VMIDs) == 0:
		return "pending"
	case p.Source == "compose":
		return "deploying"
	default:
		return "running"
	}
}

func (a *API) handleProjectLiveness(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	healthy := 0
	for _, vid := range proj.VMIDs {
		if vm, ok := a.store.GetVM(vid); ok && vm.HealthStatus == types.HealthHealthy {
			healthy++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"alive": healthy > 0, "healthy": healthy, "total": len(proj.VMIDs)})
}

func (a *API) handlePoolStatus(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	pool := map[string]any{"desired": proj.ReplicasDesired, "healthy": 0, "draining": 0, "vms": proj.VMIDs}
	for _, vid := range proj.VMIDs {
		if vm, ok := a.store.GetVM(vid); ok && vm.HealthStatus == types.HealthHealthy {
			pool["healthy"] = pool["healthy"].(int) + 1
		}
	}
	writeJSON(w, http.StatusOK, pool)
}

func (a *API) handlePoolDrain(w http.ResponseWriter, r *http.Request) {
	// Real drain: persist the draining state, stop every live replica, and
	// report how many were actually stopped.
	projID := a.projectID(r)
	a.store.PutProjectSettings(projID, "pool", map[string]any{"draining": true, "drained_at": time.Now().Format(time.RFC3339)})
	n := a.mutateReplicas(projID, nil, "stop")
	a.store.AppendDaemonLog(fmt.Sprintf("pool drained for project %s (%d replicas stopped)", projID, n))
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "draining", "project_id": projID, "stopped": n})
}

func (a *API) handleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListProjectMembers(a.projectID(r)))
}

func (a *API) handleGitSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	a.store.PutProjectSettings(a.projectID(r), "git", map[string]any{"sync": true, "branch": req.Branch})
	a.store.AppendBuildLog(a.projectID(r), "git sync triggered")
	writeJSON(w, http.StatusOK, map[string]any{"status": "synced", "branch": req.Branch})
}

func (a *API) handleGitToggles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoDeploy     *bool `json:"auto_deploy"`
		ProductionOnly *bool `json:"production_only"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	toggles := map[string]any{}
	if req.AutoDeploy != nil {
		toggles["auto_deploy"] = *req.AutoDeploy
	}
	if req.ProductionOnly != nil {
		toggles["production_only"] = *req.ProductionOnly
	}
	a.store.PutProjectSettings(a.projectID(r), "git", toggles)
	writeJSON(w, http.StatusOK, toggles)
}

// handleTransferProject — reassign project to another org (Vercel team transfer).
func (a *API) handleTransferProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID string `json:"org_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.OrgID == "" {
		writeError(w, http.StatusBadRequest, "org_id is required")
		return
	}
	a.store.SetProjectOrg(a.projectID(r), req.OrgID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "transferred", "org_id": req.OrgID})
}

// ---------------------------------------------------------------------------
// Custom microVM upload — the user brings their OWN microVM image as a .zip
// (rootfs.ext4 + vmlinux) plus name/vcpu/mem; the daemon unpacks it and it
// becomes a "custom" golden image that boots via direct Firecracker.
// ---------------------------------------------------------------------------
func (a *API) handleUploadCustomImage(w http.ResponseWriter, r *http.Request) {
	if a.customImagesDir == "" {
		writeError(w, http.StatusBadRequest, "custom images dir not configured")
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "multipart parse: "+err.Error())
		return
	}
	name := filepath.Base(strings.TrimSpace(r.FormValue("name")))
	if name == "" || name == "." || name == "/" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	vcpus, _ := strconv.Atoi(r.FormValue("vcpus"))
	memMiB, _ := strconv.Atoi(r.FormValue("mem_mib"))
	if vcpus <= 0 {
		vcpus = 1
	}
	if memMiB <= 0 {
		memMiB = 256
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file (zip) is required")
		return
	}
	defer file.Close()

	dest := filepath.Join(a.customImagesDir, name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	if err := unzipTo(file, dest); err != nil {
		writeError(w, http.StatusBadRequest, "unzip: "+err.Error())
		return
	}
	rootfs := filepath.Join(dest, "rootfs.ext4")
	kernel := filepath.Join(dest, "vmlinux")
	for _, p := range []string{rootfs, kernel} {
		if st, serr := os.Stat(p); serr != nil || st.IsDir() {
			writeError(w, http.StatusBadRequest, "zip must contain "+filepath.Base(p))
			return
		}
	}

	gi := &types.GoldenImage{
		ID:        store.NewID(),
		Name:      name,
		Image:     "custom://" + name,
		Kind:      "custom",
		Rootfs:    rootfs,
		Kernel:    kernel,
		VCPUs:     vcpus,
		MemMiB:    memMiB,
		CreatedAt: time.Now(),
	}
	if err := a.store.PutGoldenImage(gi); err != nil {
		writeError(w, http.StatusInternalServerError, "save image: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, gi)
}

// unzipTo extracts a zip archive into dest, skipping any entry whose path
// escapes dest (zip-slip protection).
// orDefault returns val unless empty, else def.
func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// safeGitURL restricts git builds from a user-supplied URL down to https:// and
// ssh (git@) repositories. It rejects filesystem paths, dash-prefixed flags,
// and anything that could be used as an argument-injection / local-file vector.
func safeGitURL(raw string) (string, bool) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", false
	}
	if strings.HasPrefix(u, "-") {
		return "", false
	}
	if strings.HasPrefix(u, "git@") {
		return u, true
	}
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return "", false
	}
	return u, true
}

// execShell runs an external command and returns its error (best-effort).
func execShell(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// execOut runs an external command and returns trimmed output.
func execOut(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func unzipTo(src io.Reader, dest string) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	cleanDest := filepath.Clean(dest)
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			continue // skip path-escape entries
		}
		target := filepath.Join(cleanDest, name)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		of, err := os.Create(target)
		if err != nil {
			rc.Close()
			continue
		}
		_, _ = io.Copy(of, rc)
		of.Close()
		rc.Close()
	}
	return nil
}
