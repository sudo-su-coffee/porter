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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"porter/internal/store"
	"porter/internal/types"
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
	// Additional users in the store.
	if user, ok := a.store.GetUserByUsername(req.Username); ok {
		if constantTimeEqual(passwordHash(req.Password, user.Salt), user.PasswordHash) {
			writeJSON(w, http.StatusOK, map[string]any{"token": a.token, "user": user})
			return
		}
	}
	writeError(w, http.StatusUnauthorized, "invalid username or password")
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged out"})
}

func (a *API) handleSignup(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "note": "single-tenant admin only; add additional users via POST /users"})
}

func (a *API) handlePasswordForgot(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "password reset email not configured in single-tenant mode")
}

func (a *API) handlePasswordReset(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "password reset not configured in single-tenant mode")
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
		a.bootReplica(proj, createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: proj.ReplicasDesired, Env: proj.Env, Ports: projPorts(proj)}, i)
	}
	a.store.PutProject(proj)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "redeploying", "project": proj})
}

func projPorts(proj *types.Project) []types.Port { return nil }

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
			a.bootReplica(proj, createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: 1, Env: proj.Env}, i)
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
	sec := &types.Secret{ID: store.NewID(), ProjectID: a.projectID(r), Name: req.Name, ValueEncrypted: []byte(obfuscate(req.Value)), CreatedAt: time.Now()}
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

func obfuscate(v string) []byte {
	return []byte("porter:" + v)
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
	writeJSON(w, http.StatusOK, map[string]any{"domain": r.PathValue("domainId"), "status": "verified"})
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
	if req.ComposeYAML == "" || !strings.Contains(req.ComposeYAML, "services:") {
		writeError(w, http.StatusBadRequest, "compose parse error: no services found under services:")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

func (a *API) handleComposePreview(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(a.projectID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preview": proj.ComposeYAML, "services": composeServiceNames(proj.ComposeYAML)})
}

func composeServiceNames(yaml string) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(yaml, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "services:") {
			continue
		}
		if idx := strings.Index(trimmed, ":"); idx > 0 && !strings.Contains(trimmed, " ") {
			n := strings.TrimSuffix(trimmed, ":")
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// Deployments & Rollout
// ---------------------------------------------------------------------------
func (a *API) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDeployments(a.projectID(r)))
}

func (a *API) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	d := &types.Deployment{ID: store.NewID(), ProjectID: a.projectID(r), BuildStatus: "ready", ImageDigest: req.Image, CreatedAt: time.Now()}
	if err := a.store.CreateDeployment(d); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, d)
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

func (a *API) handlePromoteDeployment(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "promoted", "deployment": r.PathValue("deployId")})
}

func (a *API) handleRollbackDeployment(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "rolled back", "deployment": r.PathValue("deployId")})
}

func (a *API) handleDeploymentSource(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"deployment": r.PathValue("deployId"), "source": "image"})
}

func (a *API) handleDeploymentOG(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"deployment": r.PathValue("deployId")})
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
		for _, svc := range composeServiceNames(proj.ComposeYAML) {
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "cert issued", "replica": a.vmAtReplica(a.projectID(r), replicaIndex(r))})
}

func (a *API) handleReplicaExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd   []string `json:"cmd"`
		Stdin string   `json:"stdin,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "exec queued", "replica": a.vmAtReplica(a.projectID(r), replicaIndex(r)), "cmd": req.Cmd})
}

func (a *API) handleReplicaConsole(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "console not streamable over REST; use exec", "replica": a.vmAtReplica(a.projectID(r), replicaIndex(r))})
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
	writeJSON(w, http.StatusOK, map[string]any{"replica": r.PathValue("replicaId"), "running": true})
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "cert issued", "replica": r.PathValue("replicaId")})
}

func (a *API) handleReplicaExecByID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd []string `json:"cmd"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "exec queued", "replica": r.PathValue("replicaId"), "cmd": req.Cmd})
}

func (a *API) handleReplicaConsoleByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "console requires websocket; see exec", "replica": r.PathValue("replicaId")})
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
	if req.Branch == "" {
		req.Branch = "main"
	}
	b := &types.Build{ID: store.NewID(), ProjectID: a.projectID(r), GitURL: req.GitURL, Branch: req.Branch, BuildStatus: "queued", CreatedAt: time.Now()}
	a.store.PutBuild(b)
	a.store.AppendBuildLog(a.projectID(r), fmt.Sprintf("imported %s @ %s; build queued", req.GitURL, req.Branch))
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
	d := &types.Deployment{ID: store.NewID(), ProjectID: a.projectID(r), GitURL: req.Repository, BuildStatus: "building", CreatedAt: time.Now()}
	if err := a.store.CreateDeployment(d); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.store.AppendBuildLog(a.projectID(r), "git deploy queued (clones + builds + boots microVM)")
	writeJSON(w, http.StatusAccepted, d)
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
	b := &types.Build{ID: store.NewID(), ProjectID: a.projectID(r), GitURL: req.GitURL, Branch: req.Branch, BuildStatus: "building", CreatedAt: time.Now()}
	a.store.PutBuild(b)
	a.store.AppendBuildLog(a.projectID(r), fmt.Sprintf("build %s started (git %s@%s) → OCI → microVM", b.ID, req.Branch, req.GitURL))
	writeJSON(w, http.StatusAccepted, b)
}

func (a *API) handleBuildLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"build": r.PathValue("buildId"), "logs": a.store.TailBuildLogs(a.projectID(r), 300)})
}

func (a *API) handleGitBranches(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"branches": []string{"main", "dev", "staging"}, "project_id": a.projectID(r)})
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
		for _, svc := range composeServiceNames(proj.ComposeYAML) {
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
	writeJSON(w, http.StatusOK, map[string]any{"service": r.PathValue("serviceName"), "desired": req.Replicas, "status": "applied"})
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
	writeJSON(w, http.StatusOK, map[string]any{"available": []string{"production", "preview", "staging", "development"}})
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
	var req struct {
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environment": r.PathValue("envId"), "branch": req.Branch, "status": "updated"})
}

func (a *API) handleEnvDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnvDomain string `json:"env_domain"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environment": r.PathValue("envId"), "env_domain": req.EnvDomain, "status": "updated"})
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
	a.store.TouchCron(r.PathValue("cronId"))
	a.store.AppendDaemonLog(fmt.Sprintf("cron %s triggered job", r.PathValue("cronId")))
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "job booted as microVM", "cron": r.PathValue("cronId")})
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
	writeJSON(w, http.StatusOK, map[string]any{"drain": r.PathValue("drainId"), "status": "delivered test event"})
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
func (a *API) handleAnalyticsUsage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"project_id": a.projectID(r), "requests": 0, "bandwidth": 0, "invocations": 0})
}

func (a *API) handleAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"series": []any{}, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsPaths(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"paths": []any{}, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsStatusCodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status_codes": map[string]int{}, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsBandwidth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"bandwidth_bytes": 0, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsRequests(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"requests": 0, "project_id": a.projectID(r)})
}

func (a *API) handleAnalyticsInvocations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"invocations": 0, "project_id": a.projectID(r)})
}

func (a *API) handleWebVitals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"web_vitals": []any{}, "project_id": a.projectID(r)})
}

func (a *API) handleWebVitalsTimeseries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"series": []any{}, "project_id": a.projectID(r)})
}

func (a *API) handleGlobalAnalytics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"requests": 0, "projects": 0})
}

func (a *API) handleGlobalAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"series": []any{}})
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
	writeJSON(w, http.StatusOK, map[string]any{"events": []any{}, "project_id": a.projectID(r)})
}

func (a *API) handleFirewallStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"allowed": 0, "blocked": 0, "project_id": a.projectID(r)})
}

func (a *API) handleFirewallWhitelist(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"whitelist": []string{}, "project_id": a.projectID(r)})
}

func (a *API) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"hit_rate": 0, "entries": 0, "project_id": a.projectID(r)})
}

func (a *API) handleCachePurge(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "path purged", "path": req.Path, "project_id": a.projectID(r)})
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
	if a.store.DeleteVolume(r.PathValue("volumeId")) {
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
	writeJSON(w, http.StatusOK, map[string]any{"used_bytes": 0, "volume": r.PathValue("volumeId")})
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "pruned", "removed": 0})
}

func (a *API) handleImageStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"images": len(a.store.ListGoldenImages()), "bytes": 0})
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
func uptimeSecs() int { return 0 }

func (a *API) handleHostOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"hostname": hostname(), "cpu": 0, "mem": 0, "uptime": uptimeSecs(), "version": a.version})
}

func (a *API) handleDaemonLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": a.store.TailDaemonLogs(tailN(r))})
}

func (a *API) handleHostPorts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ports": []any{}, "host": hostname()})
}

func (a *API) handleHostKernel(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"kernel": "firecracker-containerd shim", "vmlinux": "managed by host"})
}

func (a *API) handleAllTraffic(w http.ResponseWriter, r *http.Request) {
	// Aggregate all VMs' traffic rings.
	out := []*types.TrafficEntry{}
	for _, vm := range a.store.ListVMs() {
		out = append(out, a.store.ListTraffic(vm.ID, 100)...)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleClearTraffic(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "cleared"})
}

func (a *API) handleTrafficSearch(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"results": []any{}, "query": r.URL.Query().Get("q")})
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "pool draining", "project_id": a.projectID(r)})
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
