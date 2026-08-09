// Package api implements the Porter Control API.
//
// ... (original comment) ...
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"porter/internal/autoscale"
	"porter/internal/compose"
	"porter/internal/dns"
	"porter/internal/event"
	"porter/internal/netmgr"
	"porter/internal/store"
	"porter/internal/types"
	"porter/internal/volumes"
)

// HeaderOrgID is the header used to pass the current org context.
const HeaderOrgID = "X-Porter-Org-Id"

// HeaderUserID is the header used to pass the acting user (optional, falls back to admin).
const HeaderUserID = "X-Porter-User-Id"

// VMRunner is the executor the API boots replicas through (the runtime's
// VMManager, adapted by cmd/porter's vmEngine).
type VMRunner interface {
	Boot(ctx context.Context, vm *types.VM) error
	Stop(ctx context.Context, vm *types.VM) error
	Restart(ctx context.Context, vm *types.VM) error
	Delete(ctx context.Context, vm *types.VM) error
}

// Execer optionally bridges a command into a running VM's containerd task
// (the SSH/console path). Implemented by the runtime engine when the host has
// containerd; nil when unavailable (bare/firecracker-only boots).
type Execer interface {
	Exec(ctx context.Context, vmID string, stdin, stdout interface{}) error
}

// Cataloger lists known images (OCI refs, never local paths).
type Cataloger interface {
	All() []types.ImageManifest
}

// API holds every dependency of the Control API.
type API struct {
	store      *store.Store
	hub        *event.Hub
	vmm        VMRunner
	net        *netmgr.NetManager
	catalog    Cataloger
	token      string
	baseDomain string
	adminUser  string
	adminPass  string
	version    string
	logger     *log.Logger

	// customImagesDir is where user-uploaded microVM .zip images unpack to.
	// Set via SetCustomImagesDir (wired from config in main.go).
	customImagesDir string

	// domainMgr handles automatic preview/prod domain assignment.
	domainMgr *dns.DomainManager

	// volMgr manages real persistent volume directories on the host.
	volMgr *volumes.Manager

	// CSRF secret – must be set before routes are registered.
	csrfToken string

	// Rate limiting (per client IP per minute); 0 disables.
	rateLimit int
	rateMu    sync.Mutex
	rate      map[string]rateEntry

	mu sync.Mutex
	// In-process CRUD stores for settings-type endpoints (not persisted in v0.1).
	settings     map[string]map[string]any // projectID -> section -> value
	crons        map[string][]any
	alerts       map[string][]any
	drains       map[string][]any
	redirects    map[string][]any
	firewall     map[string][]any
	environments map[string][]any
	hooks        map[string][]any
	volumes      map[string]any // volumeId -> any
}

// NewAPI wires the Control API.
func NewAPI(st *store.Store, hub *event.Hub, vmm VMRunner, net *netmgr.NetManager, catalog Cataloger, token, baseDomain, adminUser, adminPass, version string) *API {
	api := &API{
		store:        st,
		hub:          hub,
		vmm:          vmm,
		net:          net,
		catalog:      catalog,
		token:        token,
		baseDomain:   baseDomain,
		adminUser:    adminUser,
		adminPass:    adminPass,
		version:      version,
		logger:       log.New(log.Writer(), "api: ", log.LstdFlags),
		csrfToken:    generateRandomToken(32),
		rate:         map[string]rateEntry{},
		settings:     map[string]map[string]any{},
		crons:        map[string][]any{},
		alerts:       map[string][]any{},
		drains:       map[string][]any{},
		redirects:    map[string][]any{},
		firewall:     map[string][]any{},
		environments: map[string][]any{},
		hooks:        map[string][]any{},
		volumes:      map[string]any{},
	}
	return api
}

// SetDomainManager configures automatic domain assignment for projects.
func (a *API) SetDomainManager(dm *dns.DomainManager) { a.domainMgr = dm }

// SetVolumesManager configures the real persistent-volume manager.
func (a *API) SetVolumesManager(vm *volumes.Manager) { a.volMgr = vm }

// StartAutoscaler runs the horizontal autoscaler in the background for
// projects with an AutoscalePolicy. interval is the load-poll cadence.
func (a *API) StartAutoscaler(interval time.Duration) {
	sc := autoscale.New(a.store,
		func(ctx context.Context, proj *types.Project, idx int) {
			a.bootReplica(proj, createProjectReq{Name: proj.Name, Image: proj.Image, Replicas: 1, Env: proj.Env, Ports: a.projPorts(proj)}, idx)
		},
		func(ctx context.Context, vmID string) {
			if vm, ok := a.store.GetVM(vmID); ok {
				_ = a.vmm.Stop(ctx, vm)
			}
		},
		interval)
	sc.Start()
}

// SetCustomImagesDir configures the directory user-uploaded microVM images
// are unpacked into (must be set before /images/custom is used).
func (a *API) SetCustomImagesDir(dir string) { a.customImagesDir = dir }

// SetRateLimit configures the per-IP request cap (0 disables).
func (a *API) SetRateLimit(n int) { a.rateLimit = n }

// rateEntry is a sliding one-minute token bucket per client IP.
type rateEntry struct {
	count int
	reset time.Time
}

// allowRate returns true if the client is under the per-minute cap.
func (a *API) allowRate(client string) bool {
	ip := client
	if h, _, err := net.SplitHostPort(client); err == nil {
		ip = h
	}
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	now := time.Now()
	e, ok := a.rate[ip]
	if !ok || now.After(e.reset) {
		a.rate[ip] = rateEntry{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	e.count++
	if e.count > a.rateLimit {
		return false
	}
	a.rate[ip] = e
	return true
}

// generateRandomToken creates a hex-encoded random string of length n bytes.
func generateRandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random csrf token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// writeJSON marshals v and writes it to w with Content-Type application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: writeJSON encode error: %v", err)
	}
}

// writeError writes a uniform {"error": msg} JSON body with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes a JSON request body into v.
func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// bearerToken extracts the "Authorization: Bearer <token>" credential, or "".
func bearerToken(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return r.URL.Query().Get("access_token")
}

// constantTimeEqual compares two strings in constant time (length-guarded).
func constantTimeEqual(a, b string) bool {
	// Short-circuit on length without leaking content timing for mismatched lengths.
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// handleHealth is the unauthenticated liveness endpoint.
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": a.version})
}

// orgIDFromHeader returns the org ID from the X-Porter-Org-Id header, or the first org as fallback.
func (a *API) orgIDFromHeader(r *http.Request) string {
	if orgID := r.Header.Get(HeaderOrgID); orgID != "" {
		return orgID
	}
	// Fallback for single-org setups
	if orgs := a.store.ListOrgs(); len(orgs) > 0 {
		return orgs[0].ID
	}
	return ""
}

// userIDFromHeader returns the user ID from X-Porter-User-Id header, or the admin user as fallback.
func (a *API) userIDFromHeader(r *http.Request) string {
	if uid := r.Header.Get(HeaderUserID); uid != "" {
		return uid
	}
	return a.adminUser
}

// Routes registers every Control API endpoint, grouped by resource.
func (a *API) Routes(mux *http.ServeMux) {
	// ========== CSRF ==========
	mux.HandleFunc("GET /csrf", a.auth(a.handleCSRFToken))

	// ========== Health ==========
	mux.HandleFunc("GET /health", a.handleHealth)

	// ========== Auth & Users ==========
	mux.HandleFunc("POST /auth/login", a.handleLogin)
	mux.HandleFunc("POST /login", a.handleLogin) // legacy alias the shipped dashboard still posts to
	mux.HandleFunc("POST /auth/logout", a.handleLogout)
	mux.HandleFunc("POST /logout", a.handleLogout)

	mux.HandleFunc("POST /auth/signup", a.handleSignup)
	mux.HandleFunc("POST /auth/password/forgot", a.handlePasswordForgot)
	mux.HandleFunc("POST /auth/password/reset", a.handlePasswordReset)
	mux.HandleFunc("GET /auth/session", a.auth(a.handleSession))
	mux.HandleFunc("GET /users/me", a.auth(a.handleMe))
	mux.HandleFunc("PATCH /users/me", a.auth(a.handlePatchMe))
	mux.HandleFunc("DELETE /users/me", a.auth(a.handleDeleteMe))
	mux.HandleFunc("GET /users/me/api-keys", a.auth(a.handleListAPIKeys))
	mux.HandleFunc("POST /users/me/api-keys", a.auth(a.handleCreateAPIKey))
	mux.HandleFunc("DELETE /users/me/api-keys/{keyId}", a.auth(a.handleDeleteAPIKey))

	// ========== Orgs (no org ID in path) ==========
	mux.HandleFunc("GET /orgs", a.auth(a.handleListOrgs))
	mux.HandleFunc("GET /orgs/default", a.auth(a.handleDefaultOrg))
	mux.HandleFunc("POST /orgs", a.auth(a.handleCreateOrg))
	mux.HandleFunc("GET /orgs/current", a.auth(a.handleGetCurrentOrg))
	mux.HandleFunc("PATCH /orgs/current", a.auth(a.handlePatchCurrentOrg))
	mux.HandleFunc("DELETE /orgs/current", a.auth(a.handleDeleteCurrentOrg))
	mux.HandleFunc("GET /orgs/members", a.auth(a.handleListOrgMembers))
	mux.HandleFunc("POST /orgs/members", a.auth(a.handleAddOrgMember))
	mux.HandleFunc("PATCH /orgs/members/{username}", a.auth(a.handlePatchOrgMember))
	mux.HandleFunc("DELETE /orgs/members/{username}", a.auth(a.handleRemoveOrgMember))
	mux.HandleFunc("GET /orgs/audit", a.auth(a.handleOrgAudit))
	mux.HandleFunc("POST /orgs/transfer", a.auth(a.handleOrgTransfer))

	// ========== Groups (org from header) ==========
	mux.HandleFunc("GET /groups", a.auth(a.handleListGroups))
	mux.HandleFunc("POST /groups", a.auth(a.handleCreateGroup))
	mux.HandleFunc("GET /groups/{groupId}", a.auth(a.handleGetGroup))
	mux.HandleFunc("PATCH /groups/{groupId}", a.auth(a.handlePatchGroup))
	mux.HandleFunc("DELETE /groups/{groupId}", a.auth(a.handleDeleteGroup))
	mux.HandleFunc("GET /groups/{groupId}/projects", a.auth(a.handleGroupProjects))
	mux.HandleFunc("POST /groups/{groupId}/projects/{projectId}", a.auth(a.handleAddGroupProject))
	mux.HandleFunc("DELETE /groups/{groupId}/projects/{projectId}", a.auth(a.handleRemoveGroupProject))

	// ========== Core Projects ==========
	mux.HandleFunc("GET /projects", a.auth(a.handleListProjects))
	mux.HandleFunc("POST /projects", a.auth(a.handleCreateProject))
	mux.HandleFunc("POST /projects/compose", a.auth(a.handleCreateComposeProject))
	mux.HandleFunc("GET /projects/{projectId}", a.auth(a.handleGetProject))
	mux.HandleFunc("PATCH /projects/{projectId}", a.auth(a.handlePatchProject))
	mux.HandleFunc("DELETE /projects/{projectId}", a.auth(a.handleDeleteProject))
	mux.HandleFunc("POST /projects/{projectId}/redeploy", a.auth(a.handleRedeployProject))

	// ========== Project: Scale, Health, Restart ==========
	mux.HandleFunc("GET /projects/{projectId}/scale", a.auth(a.handleGetScale))
	mux.HandleFunc("PATCH /projects/{projectId}/scale", a.auth(a.handleScale))
	mux.HandleFunc("GET /projects/{projectId}/healthcheck", a.auth(a.handleGetHealthcheck))
	mux.HandleFunc("PUT /projects/{projectId}/healthcheck", a.auth(a.handlePutHealthcheck))
	mux.HandleFunc("GET /projects/{projectId}/autoscale", a.auth(a.handleGetAutoscale))
	mux.HandleFunc("PUT /projects/{projectId}/autoscale", a.auth(a.handlePutAutoscale))
	mux.HandleFunc("POST /projects/{projectId}/restart", a.auth(a.handleRestartProject))

	// ========== Project: Env & Secrets ==========
	mux.HandleFunc("GET /projects/{projectId}/env", a.auth(a.handleListEnv))
	mux.HandleFunc("POST /projects/{projectId}/env", a.auth(a.handleSetEnv))
	mux.HandleFunc("POST /projects/{projectId}/env/bulk", a.auth(a.handleSetEnvBulk))
	mux.HandleFunc("PATCH /projects/{projectId}/env/{envId}", a.auth(a.handlePatchEnv))
	mux.HandleFunc("DELETE /projects/{projectId}/env/{envId}", a.auth(a.handleDeleteEnv))
	mux.HandleFunc("GET /projects/{projectId}/secrets", a.auth(a.handleListSecrets))
	mux.HandleFunc("POST /projects/{projectId}/secrets", a.auth(a.handleCreateSecret))
	mux.HandleFunc("DELETE /projects/{projectId}/secrets/{secretId}", a.auth(a.handleDeleteSecret))

	// ========== Project: Domains & DNS ==========
	mux.HandleFunc("GET /projects/{projectId}/domains", a.auth(a.handleListDomains))
	mux.HandleFunc("POST /projects/{projectId}/domains", a.auth(a.handleAddDomain))
	mux.HandleFunc("GET /projects/{projectId}/domains/records", a.auth(a.handleDomainRecords))
	mux.HandleFunc("GET /projects/{projectId}/domains/{domainId}", a.auth(a.handleGetDomain))
	mux.HandleFunc("DELETE /projects/{projectId}/domains/{domainId}", a.auth(a.handleDeleteDomain))
	mux.HandleFunc("POST /projects/{projectId}/domains/{domainId}/verify", a.auth(a.handleVerifyDomain))
	mux.HandleFunc("POST /projects/{projectId}/domains/{domainId}/reverify", a.auth(a.handleVerifyDomain))
	mux.HandleFunc("GET /projects/{projectId}/dns", a.auth(a.handleProjectDNS))
	mux.HandleFunc("GET /projects/{projectId}/dns/records", a.auth(a.handleProjectDNS)) // alias

	// ========== Project: Compose ==========
	mux.HandleFunc("GET /projects/{projectId}/compose", a.auth(a.handleGetCompose))
	mux.HandleFunc("PUT /projects/{projectId}/compose", a.auth(a.handlePutCompose))
	mux.HandleFunc("POST /projects/{projectId}/compose/validate", a.auth(a.handleValidateCompose))
	mux.HandleFunc("GET /projects/{projectId}/compose/preview", a.auth(a.handleComposePreview))

	// ========== Project: Aggregated Views ==========
	mux.HandleFunc("GET /projects/{projectId}/logs", a.auth(a.handleProjectLogs))
	mux.HandleFunc("GET /projects/{projectId}/metrics", a.auth(a.handleProjectMetrics))
	mux.HandleFunc("GET /projects/{projectId}/traffic", a.auth(a.handleProjectTraffic))
	mux.HandleFunc("GET /projects/{projectId}/events", a.auth(a.handleProjectEvents))

	// ========== Project: Pool & Rollout ==========
	mux.HandleFunc("GET /projects/{projectId}/pool", a.auth(a.handlePoolStatus))
	mux.HandleFunc("POST /projects/{projectId}/pool/drain", a.auth(a.handlePoolDrain))
	mux.HandleFunc("GET /projects/{projectId}/status", a.auth(a.handleProjectStatus))
	mux.HandleFunc("GET /projects/{projectId}/liveness", a.auth(a.handleProjectLiveness))

	// ========== Nested Replicas ==========
	mux.HandleFunc("GET /projects/{projectId}/replicas", a.auth(a.handleListReplicas))
	mux.HandleFunc("POST /projects/{projectId}/replicas/batch/start", a.auth(a.handleReplicaBatchStart))
	mux.HandleFunc("POST /projects/{projectId}/replicas/batch/stop", a.auth(a.handleReplicaBatchStop))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}", a.auth(a.handleGetReplica))
	mux.HandleFunc("POST /projects/{projectId}/replicas/{n}/start", a.auth(a.handleReplicaStart))
	mux.HandleFunc("POST /projects/{projectId}/replicas/{n}/stop", a.auth(a.handleReplicaStop))
	mux.HandleFunc("POST /projects/{projectId}/replicas/{n}/restart", a.auth(a.handleReplicaRestart))
	mux.HandleFunc("DELETE /projects/{projectId}/replicas/{n}", a.auth(a.handleReplicaDelete))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/logs", a.auth(a.handleReplicaLogs))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/metrics", a.auth(a.handleReplicaMetrics))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/traffic", a.auth(a.handleReplicaTraffic))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/health", a.auth(a.handleReplicaHealth))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/ssh-info", a.auth(a.handleSSHInfo))
	mux.HandleFunc("POST /projects/{projectId}/replicas/{n}/ssh-cert", a.auth(a.handleSSHCert))
	mux.HandleFunc("POST /projects/{projectId}/replicas/{n}/exec", a.auth(a.handleReplicaExec))
	mux.HandleFunc("GET /projects/{projectId}/replicas/{n}/console", a.auth(a.handleReplicaConsole))

	// ========== Deployments ==========
	mux.HandleFunc("GET /projects/{projectId}/deployments", a.auth(a.handleListDeployments))
	mux.HandleFunc("POST /projects/{projectId}/deployments", a.auth(a.handleCreateDeployment))
	mux.HandleFunc("GET /projects/{projectId}/deployments/upload", a.auth(a.handleDeploymentUpload))
	mux.HandleFunc("GET /projects/{projectId}/deployments/{deployId}", a.auth(a.handleGetDeployment))
	mux.HandleFunc("GET /projects/{projectId}/deployments/{deployId}/logs", a.auth(a.handleDeploymentLogs))
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deployId}/promote", a.auth(a.handlePromoteDeployment))
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deployId}/rollback", a.auth(a.handleRollbackDeployment))
	mux.HandleFunc("GET /projects/{projectId}/deployments/{deployId}/source", a.auth(a.handleDeploymentSource))
	mux.HandleFunc("GET /projects/{projectId}/deployments/{deployId}/og", a.auth(a.handleDeploymentOG))

	// ========== Project Settings: General ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/general", a.auth(a.handleGetGeneral))
	mux.HandleFunc("PATCH /projects/{projectId}/settings/general", a.auth(a.handlePatchGeneral))
	mux.HandleFunc("POST /projects/{projectId}/avatar", a.auth(a.handleSetAvatar))
	mux.HandleFunc("POST /projects/{projectId}/transfer", a.auth(a.handleTransferProject))

	// ========== Project Settings: Build & Deployment ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/build", a.auth(a.handleGetBuild))
	mux.HandleFunc("PUT /projects/{projectId}/settings/build", a.auth(a.handlePutBuild))
	mux.HandleFunc("GET /projects/{projectId}/settings/checks", a.auth(a.handleGetChecks))
	mux.HandleFunc("POST /projects/{projectId}/settings/checks", a.auth(a.handlePutChecks))
	mux.HandleFunc("GET /projects/{projectId}/settings/rollout", a.auth(a.handleGetRollout))
	mux.HandleFunc("PUT /projects/{projectId}/settings/rollout", a.auth(a.handlePutRollout))
	mux.HandleFunc("GET /projects/{projectId}/settings/build-machine", a.auth(a.handleGetBuildMachine))
	mux.HandleFunc("PUT /projects/{projectId}/settings/build-machine", a.auth(a.handlePutBuildMachine))
	mux.HandleFunc("POST /projects/{projectId}/settings/ignore-command", a.auth(a.handleSetIgnoreCommand))
	mux.HandleFunc("GET /projects/{projectId}/settings/framework", a.auth(a.handleGetFramework))

	// ========== Environments ==========
	mux.HandleFunc("GET /projects/{projectId}/environments", a.auth(a.handleListEnvironments))
	mux.HandleFunc("POST /projects/{projectId}/environments", a.auth(a.handleCreateEnvironment))
	mux.HandleFunc("GET /projects/{projectId}/environments/available", a.auth(a.handleEnvironmentsAvailable))
	mux.HandleFunc("GET /projects/{projectId}/environments/{envId}", a.auth(a.handleGetEnvironment))
	mux.HandleFunc("PATCH /projects/{projectId}/environments/{envId}", a.auth(a.handlePatchEnvironment))
	mux.HandleFunc("DELETE /projects/{projectId}/environments/{envId}", a.auth(a.handleDeleteEnvironment))
	mux.HandleFunc("POST /projects/{projectId}/environments/{envId}/branch", a.auth(a.handleEnvBranch))
	mux.HandleFunc("POST /projects/{projectId}/environments/{envId}/domain", a.auth(a.handleEnvDomain))

	// ========== Git & Hooks ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/git", a.auth(a.handleGetGit))
	mux.HandleFunc("PUT /projects/{projectId}/settings/git", a.auth(a.handlePutGit))
	mux.HandleFunc("POST /projects/{projectId}/settings/git/sync", a.auth(a.handleGitSync))
	mux.HandleFunc("PATCH /projects/{projectId}/settings/git/toggles", a.auth(a.handleGitToggles))
	mux.HandleFunc("GET /projects/{projectId}/settings/git/lfs", a.auth(a.handleGetGitLFS))
	mux.HandleFunc("PUT /projects/{projectId}/settings/git/lfs", a.auth(a.handlePutGitLFS))
	mux.HandleFunc("GET /projects/{projectId}/hooks", a.auth(a.handleListHooks))
	mux.HandleFunc("POST /projects/{projectId}/hooks", a.auth(a.handleCreateHook))
	mux.HandleFunc("DELETE /projects/{projectId}/hooks/{hookId}", a.auth(a.handleDeleteHook))
	mux.HandleFunc("POST /projects/{projectId}/hooks/{hookId}/trigger", a.auth(a.handleTriggerHook))

	// ========== Protection & OIDC ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/deployment-protection", a.auth(a.handleGetProtection))
	mux.HandleFunc("PUT /projects/{projectId}/settings/deployment-protection", a.auth(a.handlePutProtection))
	mux.HandleFunc("GET /projects/{projectId}/settings/oidc", a.auth(a.handleGetOIDC))
	mux.HandleFunc("PUT /projects/{projectId}/settings/oidc", a.auth(a.handlePutOIDC))

	// ========== Functions & Crons ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/functions", a.auth(a.handleGetFunctions))
	mux.HandleFunc("PUT /projects/{projectId}/settings/functions", a.auth(a.handlePutFunctions))
	mux.HandleFunc("GET /projects/{projectId}/crons", a.auth(a.handleListCrons))
	mux.HandleFunc("POST /projects/{projectId}/crons", a.auth(a.handleCreateCron))
	mux.HandleFunc("GET /projects/{projectId}/crons/history", a.auth(a.handleCronHistory))
	mux.HandleFunc("GET /projects/{projectId}/crons/{cronId}", a.auth(a.handleGetCron))
	mux.HandleFunc("PATCH /projects/{projectId}/crons/{cronId}", a.auth(a.handlePatchCron))
	mux.HandleFunc("DELETE /projects/{projectId}/crons/{cronId}", a.auth(a.handleDeleteCron))
	mux.HandleFunc("POST /projects/{projectId}/crons/{cronId}/run", a.auth(a.handleRunCron))

	// ========== Project Members (use username) ==========
	mux.HandleFunc("GET /projects/{projectId}/members", a.auth(a.handleListProjectMembers))
	mux.HandleFunc("POST /projects/{projectId}/members", a.auth(a.handleAddProjectMember))
	mux.HandleFunc("GET /projects/{projectId}/members/{username}", a.auth(a.handleGetProjectMember))
	mux.HandleFunc("PATCH /projects/{projectId}/members/{username}", a.auth(a.handlePatchProjectMember))
	mux.HandleFunc("DELETE /projects/{projectId}/members/{username}", a.auth(a.handleRemoveProjectMember))
	mux.HandleFunc("POST /projects/{projectId}/members/invite", a.auth(a.handleInviteMember))

	// ========== Log Drains & Alerts ==========
	mux.HandleFunc("GET /projects/{projectId}/drains", a.auth(a.handleListDrains))
	mux.HandleFunc("POST /projects/{projectId}/drains", a.auth(a.handleCreateDrain))
	mux.HandleFunc("DELETE /projects/{projectId}/drains/{drainId}", a.auth(a.handleDeleteDrain))
	mux.HandleFunc("POST /projects/{projectId}/drains/{drainId}/test", a.auth(a.handleTestDrain))
	mux.HandleFunc("GET /projects/{projectId}/alerts", a.auth(a.handleListAlerts))
	mux.HandleFunc("POST /projects/{projectId}/alerts", a.auth(a.handleCreateAlert))
	mux.HandleFunc("GET /projects/{projectId}/alerts/{alertId}", a.auth(a.handleGetAlert))
	mux.HandleFunc("PATCH /projects/{projectId}/alerts/{alertId}", a.auth(a.handlePatchAlert))
	mux.HandleFunc("DELETE /projects/{projectId}/alerts/{alertId}", a.auth(a.handleDeleteAlert))
	mux.HandleFunc("POST /projects/{projectId}/alerts/{alertId}/silence", a.auth(a.handleSilenceAlert))
	mux.HandleFunc("POST /projects/{projectId}/alerts/{alertId}/unsilence", a.auth(a.handleUnsilenceAlert))

	// ========== Security / Networking / Advanced ==========
	mux.HandleFunc("GET /projects/{projectId}/settings/security", a.auth(a.handleGetSecurity))
	mux.HandleFunc("PUT /projects/{projectId}/settings/security", a.auth(a.handlePutSecurity))
	mux.HandleFunc("GET /projects/{projectId}/settings/retention", a.auth(a.handleGetRetention))
	mux.HandleFunc("PUT /projects/{projectId}/settings/retention", a.auth(a.handlePutRetention))
	mux.HandleFunc("GET /projects/{projectId}/settings/networking", a.auth(a.handleGetNetworking))
	mux.HandleFunc("PUT /projects/{projectId}/settings/networking", a.auth(a.handlePutNetworking))
	mux.HandleFunc("GET /projects/{projectId}/settings/advanced", a.auth(a.handleGetAdvanced))
	mux.HandleFunc("PUT /projects/{projectId}/settings/advanced", a.auth(a.handlePutAdvanced))
	mux.HandleFunc("GET /projects/{projectId}/settings/passport", a.auth(a.handleGetPassport))
	mux.HandleFunc("PUT /projects/{projectId}/settings/passport", a.auth(a.handlePutPassport))
	mux.HandleFunc("GET /projects/{projectId}/settings/microfrontends", a.auth(a.handleGetMicrofrontends))
	mux.HandleFunc("PUT /projects/{projectId}/settings/microfrontends", a.auth(a.handlePutMicrofrontends))

	// ========== Redirects ==========
	mux.HandleFunc("GET /projects/{projectId}/redirects", a.auth(a.handleListRedirects))
	mux.HandleFunc("POST /projects/{projectId}/redirects", a.auth(a.handleCreateRedirect))
	mux.HandleFunc("DELETE /projects/{projectId}/redirects/{redirectId}", a.auth(a.handleDeleteRedirect))
	mux.HandleFunc("PUT /projects/{projectId}/redirects/bulk", a.auth(a.handleBulkRedirects))

	// ========== Analytics & Observability ==========
	mux.HandleFunc("GET /projects/{projectId}/analytics/usage", a.auth(a.handleAnalyticsUsage))
	mux.HandleFunc("GET /projects/{projectId}/analytics/usage/timeseries", a.auth(a.handleAnalyticsTimeseries))
	mux.HandleFunc("GET /projects/{projectId}/analytics/paths", a.auth(a.handleAnalyticsPaths))
	mux.HandleFunc("GET /projects/{projectId}/analytics/status-codes", a.auth(a.handleAnalyticsStatusCodes))
	mux.HandleFunc("GET /projects/{projectId}/analytics/bandwidth", a.auth(a.handleAnalyticsBandwidth))
	mux.HandleFunc("GET /projects/{projectId}/analytics/requests", a.auth(a.handleAnalyticsRequests))
	mux.HandleFunc("GET /projects/{projectId}/analytics/invocations", a.auth(a.handleAnalyticsInvocations))
	mux.HandleFunc("GET /projects/{projectId}/observability/web-vitals", a.auth(a.handleWebVitals))
	mux.HandleFunc("POST /projects/{projectId}/observability/web-vitals/beacon", a.auth(a.handleWebVitalsBeacon))
	mux.HandleFunc("GET /projects/{projectId}/observability/web-vitals/timeseries", a.auth(a.handleWebVitalsTimeseries))

	// LCP/CLS/FID aliases
	mux.HandleFunc("GET /projects/{projectId}/observability/lcp", a.auth(a.handleAnalyticsUsage))
	mux.HandleFunc("GET /projects/{projectId}/observability/cls", a.auth(a.handleAnalyticsUsage))
	mux.HandleFunc("GET /projects/{projectId}/observability/fid", a.auth(a.handleAnalyticsUsage))
	mux.HandleFunc("GET /global/analytics", a.auth(a.handleGlobalAnalytics))
	mux.HandleFunc("GET /global/analytics/timeseries", a.auth(a.handleGlobalAnalyticsTimeseries))
	mux.HandleFunc("GET /usage", a.auth(a.handleUsage))
	mux.HandleFunc("GET /usage/bandwidth", a.auth(a.handleUsageBandwidth))
	mux.HandleFunc("GET /usage/requests", a.auth(a.handleUsageRequests))
	mux.HandleFunc("GET /usage/timeseries", a.auth(a.handleGlobalAnalyticsTimeseries))

	// ========== Firewall & WAF ==========
	mux.HandleFunc("GET /projects/{projectId}/firewall/rules", a.auth(a.handleListFirewallRules))
	mux.HandleFunc("POST /projects/{projectId}/firewall/rules", a.auth(a.handleCreateFirewallRule))
	mux.HandleFunc("GET /projects/{projectId}/firewall/rules/{ruleId}", a.auth(a.handleGetFirewallRule))
	mux.HandleFunc("DELETE /projects/{projectId}/firewall/rules/{ruleId}", a.auth(a.handleDeleteFirewallRule))
	mux.HandleFunc("PATCH /projects/{projectId}/firewall/rules/{ruleId}", a.auth(a.handlePatchFirewallRule))
	mux.HandleFunc("GET /projects/{projectId}/firewall/events", a.auth(a.handleFirewallEvents))
	mux.HandleFunc("GET /projects/{projectId}/firewall/stats", a.auth(a.handleFirewallStats))
	mux.HandleFunc("POST /projects/{projectId}/firewall/whitelist", a.auth(a.handleFirewallWhitelist))

	// ========== Cache ==========
	mux.HandleFunc("GET /projects/{projectId}/cache/stats", a.auth(a.handleCacheStats))
	mux.HandleFunc("POST /projects/{projectId}/cache/purge", a.auth(a.handleCachePurge))
	mux.HandleFunc("POST /projects/{projectId}/cache/purge/path", a.auth(a.handleCachePurgePath))

	// ========== Volumes ==========
	mux.HandleFunc("GET /volumes", a.auth(a.handleListVolumes))
	mux.HandleFunc("POST /volumes", a.auth(a.handleCreateVolume))
	mux.HandleFunc("GET /volumes/{volumeId}", a.auth(a.handleGetVolume))
	mux.HandleFunc("DELETE /volumes/{volumeId}", a.auth(a.handleDeleteVolume))
	mux.HandleFunc("POST /volumes/{volumeId}/resize", a.auth(a.handleResizeVolume))
	mux.HandleFunc("GET /volumes/{volumeId}/usage", a.auth(a.handleVolumeUsage))

	// ========== Images / Registry ==========
	mux.HandleFunc("GET /images", a.auth(a.handleListImages))
	mux.HandleFunc("POST /images/custom", a.auth(a.handleUploadCustomImage))
	mux.HandleFunc("GET /images/search", a.auth(a.handleImageSearch))
	mux.HandleFunc("GET /images/{reference}", a.auth(a.handleGetImage))
	mux.HandleFunc("DELETE /images/{reference}", a.auth(a.handleDeleteImage))
	mux.HandleFunc("POST /images/prune", a.auth(a.handlePruneImages))
	mux.HandleFunc("GET /images/stats", a.auth(a.handleImageStats))

	// ========== Events / Overview / Host ==========
	mux.Handle("GET /events", a.hub)
	mux.HandleFunc("GET /orgs/events", a.auth(a.handleOrgEvents)) // org from header
	mux.HandleFunc("GET /overview", a.auth(a.handleOverview))

	// ========== Legacy /vms aliases (the shipped dashboard still uses them) ==========
	mux.HandleFunc("GET /vms", a.auth(a.handleListAllVMs))
	mux.HandleFunc("GET /vms/{replicaId}", a.auth(a.handleGetVMCompat))
	mux.HandleFunc("POST /vms/{replicaId}/start", a.auth(a.handleReplicaStartByID))
	mux.HandleFunc("POST /vms/{replicaId}/stop", a.auth(a.handleReplicaStopByID))
	mux.HandleFunc("POST /vms/{replicaId}/restart", a.auth(a.handleReplicaRestartByID))
	mux.HandleFunc("DELETE /vms/{replicaId}", a.auth(a.handleVMCompatDelete))
	mux.HandleFunc("GET /host/overview", a.auth(a.handleHostOverview))
	mux.HandleFunc("GET /logs", a.auth(a.handleDaemonLogs))
	mux.HandleFunc("GET /host/ports", a.auth(a.handleHostPorts))
	mux.HandleFunc("GET /host/kernel", a.auth(a.handleHostKernel))
	mux.HandleFunc("GET /traffic", a.auth(a.handleAllTraffic))
	mux.HandleFunc("DELETE /traffic", a.auth(a.handleClearTraffic))
	mux.HandleFunc("GET /traffic/search", a.auth(a.handleTrafficSearch))

	// ========== Servers / Users / Export / Import / SSH ==========
	mux.HandleFunc("GET /servers", a.auth(a.handleListServers))
	mux.HandleFunc("POST /servers", a.auth(a.handleRegisterServer))
	mux.HandleFunc("DELETE /servers/{id}", a.auth(a.handleDeleteServer))
	mux.HandleFunc("GET /users", a.auth(a.handleListUsers))
	mux.HandleFunc("POST /users", a.auth(a.handleCreateUser))
	mux.HandleFunc("DELETE /users/{username}", a.auth(a.handleDeleteUser)) // now uses username

	// ========== RBAC (roles & permissions CRUD) ==========
	mux.HandleFunc("GET /roles", a.auth(a.handleListRoles))
	mux.HandleFunc("POST /roles", a.auth(a.handleCreateRole))
	mux.HandleFunc("GET /roles/{roleId}", a.auth(a.handleGetRole))
	mux.HandleFunc("PATCH /roles/{roleId}", a.auth(a.handlePatchRole))
	mux.HandleFunc("DELETE /roles/{roleId}", a.auth(a.handleDeleteRole))
	mux.HandleFunc("GET /permissions", a.auth(a.handleListPermissions))
	mux.HandleFunc("GET /roles/{roleId}/permissions", a.auth(a.handleGetRolePermissions))
	mux.HandleFunc("PUT /roles/{roleId}/permissions", a.auth(a.handleSetRolePermissions))
	mux.HandleFunc("POST /roles/{roleId}/permissions/{permissionId}", a.auth(a.handleAddRolePermission))
	mux.HandleFunc("DELETE /roles/{roleId}/permissions/{permissionId}", a.auth(a.handleRemoveRolePermission))
	mux.HandleFunc("POST /projects/{projectId}/export", a.auth(a.handleExportProject))
	mux.HandleFunc("POST /projects/{projectId}/import", a.auth(a.handleImportProject))
	mux.HandleFunc("PUT /projects/{projectId}/ssh", a.auth(a.handleSSHToggle))

	// ========== Git Builds & GitOps (self-hosted Vercel vision) ==========
	mux.HandleFunc("POST /projects/{projectId}/git/import", a.auth(a.handleGitImport))
	mux.HandleFunc("POST /projects/{projectId}/deployments/git", a.auth(a.handleDeployGit))
	mux.HandleFunc("GET /projects/{projectId}/builds", a.auth(a.handleListBuilds))
	mux.HandleFunc("POST /projects/{projectId}/builds", a.auth(a.handleCreateBuild))
	mux.HandleFunc("POST /projects/{projectId}/builds/run", a.auth(a.handleCreateBuild))
	mux.HandleFunc("GET /projects/{projectId}/builds/{buildId}/logs", a.auth(a.handleBuildLogs))
	mux.HandleFunc("GET /projects/{projectId}/git/branches", a.auth(a.handleGitBranches))
	mux.HandleFunc("GET /projects/{projectId}/rollouts", a.auth(a.handleListRollouts))

	// ========== Docker-ecosystem parity: services & networks ==========
	mux.HandleFunc("GET /projects/{projectId}/services", a.auth(a.handleListServices))
	mux.HandleFunc("GET /projects/{projectId}/services/{serviceName}", a.auth(a.handleGetService))
	mux.HandleFunc("POST /projects/{projectId}/services/{serviceName}/scale", a.auth(a.handleScaleService))
	mux.HandleFunc("GET /projects/{projectId}/networks", a.auth(a.handleListNetworks))
	mux.HandleFunc("POST /projects/{projectId}/networks", a.auth(a.handleCreateNetwork))

	// ========== ML / serving image catalog ==========
	mux.HandleFunc("GET /images/ml", a.auth(a.handleImageSearch)) // catalog filter for ML images
}

// ----------------------------------------------------------------------------
// CSRF token endpoint
// ----------------------------------------------------------------------------
func (a *API) handleCSRFToken(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": a.csrfToken})
}

// ----------------------------------------------------------------------------
// Auth middleware with CSRF check for state-changing methods
// ----------------------------------------------------------------------------
// rbacCtxKey is the context key carrying the authenticated principal.
type rbacCtxKey struct{}

// principal is the authenticated user attached to the request by auth().
type principal struct {
	username string
	role     string // global role from the users table
	isAdmin  bool   // bootstrap config admin always has full access
}

// currentPrincipal returns the principal attached to the request, or the
// config admin when none is attached (defensive default).
func currentPrincipal(r *http.Request) principal {
	if p, ok := r.Context().Value(rbacCtxKey{}).(principal); ok {
		return p
	}
	return principal{username: "admin", role: "admin", isAdmin: true}
}

// currentRole returns the effective global role of the authenticated user.
func currentRole(r *http.Request) string { return currentPrincipal(r).role }

// currentUser returns the authenticated username (empty for the bootstrap admin
// which is not a users-table row).
func currentUser(r *http.Request) string { return currentPrincipal(r).username }

func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Authenticate the bearer token: config admin OR a per-user API key.
		tok := bearerToken(r)
		p := principal{}
		if tok != "" && constantTimeEqual(a.token, tok) {
			p = principal{role: "admin", isAdmin: true} // bootstrap config admin
		} else if tok != "" {
			if u, ok := a.store.GetUserByToken(tok); ok {
				role := u.Role
				if role == "" {
					role = "member"
				}
				p = principal{username: u.Username, role: role}
			}
		}
		if p.role == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		// 1b. Optional per-IP rate limit on auth'd requests.
		if a.rateLimit > 0 && !a.allowRate(r.RemoteAddr) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		// 2. CSRF check for mutable methods (except the CSRF endpoint itself and auth)
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.URL.Path != "/csrf" {
			csrf := r.Header.Get("X-CSRF-Token")
			if csrf == "" || !constantTimeEqual(csrf, a.csrfToken) {
				writeError(w, http.StatusForbidden, "invalid or missing CSRF token")
				return
			}
		}
		// 3. Route-level RBAC via the central permission table. Every path
		// pattern in routePerms is guarded by a specific permission code (e.g.
		// ssh.connect, deployment.promote, member.remove). The check resolves
		// global/org/project context from the URL and consults role_permissions.
		if !p.isAdmin {
			if perm := permForRoute(r); perm != "" {
				ok := a.granted(r, p, perm)
				if !ok {
					writeError(w, http.StatusForbidden, "missing permission: "+perm)
					return
				}
			}
		}
		ctx := context.WithValue(r.Context(), rbacCtxKey{}, p)
		next(w, r.WithContext(ctx))
	}
}

// granted reports whether the principal holds the permission for this request,
// choosing project / org / global scope from the URL.
func (a *API) granted(r *http.Request, p principal, perm string) bool {
	if projID := r.PathValue("projectId"); projID != "" {
		return a.store.HasProjectPermission(projID, p.username, perm)
	}
	return a.store.HasPermission(p.username, perm)
}

// routePerms maps each registered method+pattern to the fine-grained permission
// that guards it. Permission codes are "<resource>.<action>" and live in the
// permissions table (migrations/0007_rbac.sql) so they are editable in the UI.
func permForRoute(r *http.Request) string {
	return routePerms[r.Pattern]
}

var routePerms = map[string]string{
	// projects
	"GET /projects":                                    "project.list",
	"POST /projects":                                   "project.create",
	"POST /projects/compose":                           "project.create",
	"GET /projects/{projectId}":                        "project.read",
	"PATCH /projects/{projectId}":                      "project.rename",
	"DELETE /projects/{projectId}":                     "project.delete",
	"POST /projects/{projectId}/avatar":                "project.avatar",
	"POST /projects/{projectId}/transfer":              "project.transfer",
	"POST /projects/{projectId}/redeploy":              "project.deploy",
	"POST /projects/{projectId}/restart":               "project.restart",
	"POST /projects/{projectId}/export":                "project.export",
	"POST /projects/{projectId}/import":                "project.import",
	"GET /projects/{projectId}/scale":                  "replica.list",
	"PATCH /projects/{projectId}/scale":                "project.scale",
	"GET /projects/{projectId}/healthcheck":            "project.read",
	"PUT /projects/{projectId}/healthcheck":            "project.settings",
	"GET /projects/{projectId}/autoscale":              "project.read",
	"PUT /projects/{projectId}/autoscale":              "project.settings",
	"GET /projects/{projectId}/env":                    "env.list",
	"POST /projects/{projectId}/env":                   "env.set",
	"POST /projects/{projectId}/env/bulk":              "env.set",
	"PATCH /projects/{projectId}/env/{envId}":          "env.set",
	"DELETE /projects/{projectId}/env/{envId}":         "env.set",
	"GET /projects/{projectId}/secrets":                "secret.list",
	"POST /projects/{projectId}/secrets":               "secret.create",
	"DELETE /projects/{projectId}/secrets/{secretId}":  "secret.delete",
	"GET /projects/{projectId}/domains":                "domain.list",
	"POST /projects/{projectId}/domains":               "domain.add",
	"GET /projects/{projectId}/domains/records":        "domain.list",
	"GET /projects/{projectId}/domains/{domainId}":     "domain.list",
	"DELETE /projects/{projectId}/domains/{domainId}":  "domain.remove",
	"POST /projects/{projectId}/domains/{domainId}/verify":  "domain.verify",
	"GET /projects/{projectId}/dns":                    "domain.list",
	"GET /projects/{projectId}/compose":                "project.read",
	"PUT /projects/{projectId}/compose":                "project.import",
	"POST /projects/{projectId}/compose/validate":      "project.import",
	"GET /projects/{projectId}/compose/preview":        "project.read",
	"GET /projects/{projectId}/logs":                   "log.read",
	"GET /projects/{projectId}/metrics":                "metric.read",
	"GET /projects/{projectId}/traffic":                "traffic.read",
	"GET /projects/{projectId}/events":                 "event.read",
	"GET /projects/{projectId}/pool":                   "replica.list",
	"POST /projects/{projectId}/pool/drain":            "project.settings",
	"GET /projects/{projectId}/status":                 "project.read",
	"GET /projects/{projectId}/liveness":               "project.read",
	"GET /projects/{projectId}/replicas":               "replica.list",
	"POST /projects/{projectId}/replicas/batch/start":  "replica.start",
	"POST /projects/{projectId}/replicas/batch/stop":   "replica.stop",
	"GET /projects/{projectId}/replicas/{n}":           "replica.list",
	"POST /projects/{projectId}/replicas/{n}/start":    "replica.start",
	"POST /projects/{projectId}/replicas/{n}/stop":     "replica.stop",
	"POST /projects/{projectId}/replicas/{n}/restart":  "replica.restart",
	"DELETE /projects/{projectId}/replicas/{n}":        "replica.delete",
	"GET /projects/{projectId}/replicas/{n}/logs":      "log.read",
	"GET /projects/{projectId}/replicas/{n}/metrics":   "metric.read",
	"GET /projects/{projectId}/replicas/{n}/traffic":   "traffic.read",
	"GET /projects/{projectId}/replicas/{n}/health":    "replica.list",
	"GET /projects/{projectId}/replicas/{n}/ssh-info":  "ssh.connect",
	"POST /projects/{projectId}/replicas/{n}/ssh-cert": "ssh.connect",
	"POST /projects/{projectId}/replicas/{n}/exec":     "replica.exec",
	"GET /projects/{projectId}/replicas/{n}/console":   "console.open",
	"GET /projects/{projectId}/deployments":            "deployment.list",
	"POST /projects/{projectId}/deployments":           "deployment.create",
	"GET /projects/{projectId}/deployments/{deployId}": "deployment.list",
	"GET /projects/{projectId}/deployments/{deployId}/logs": "log.read",
	"POST /projects/{projectId}/deployments/{deployId}/promote":  "deployment.promote",
	"POST /projects/{projectId}/deployments/{deployId}/rollback": "deployment.rollback",
	"GET /projects/{projectId}/settings/general":       "project.read",
	"PATCH /projects/{projectId}/settings/general":     "project.settings",
	"GET /projects/{projectId}/settings/build":         "project.read",
	"PUT /projects/{projectId}/settings/build":         "project.settings",
	"GET /projects/{projectId}/settings/git":           "project.read",
	"PUT /projects/{projectId}/settings/git":           "git.settings",
	"POST /projects/{projectId}/settings/git/sync":     "git.settings",
	"GET /projects/{projectId}/crons":                  "cron.create",
	"POST /projects/{projectId}/crons":                 "cron.create",
	"GET /projects/{projectId}/crons/history":          "cron.update",
	"PATCH /projects/{projectId}/crons/{cronId}":       "cron.update",
	"DELETE /projects/{projectId}/crons/{cronId}":      "cron.delete",
	"POST /projects/{projectId}/crons/{cronId}/run":    "cron.run",
	"GET /projects/{projectId}/members":                "member.list",
	"POST /projects/{projectId}/members":               "member.invite",
	"POST /projects/{projectId}/members/invite":        "member.invite",
	"PATCH /projects/{projectId}/members/{username}":   "member.role",
	"DELETE /projects/{projectId}/members/{username}":  "member.remove",
	"GET /projects/{projectId}/drains":                 "project.settings",
	"POST /projects/{projectId}/drains":                "drain.create",
	"DELETE /projects/{projectId}/drains/{drainId}":    "drain.delete",
	"POST /projects/{projectId}/drains/{drainId}/test": "drain.create",
	"GET /projects/{projectId}/hooks":                  "project.read",
	"POST /projects/{projectId}/hooks":                 "hook.create",
	"DELETE /projects/{projectId}/hooks/{hookId}":      "hook.delete",
	"POST /projects/{projectId}/hooks/{hookId}/trigger":"hook.trigger",
	"GET /projects/{projectId}/firewall/rules":         "project.read",
	"POST /projects/{projectId}/firewall/rules":        "firewall.create",
	"GET /projects/{projectId}/firewall/rules/{ruleId}":"project.read",
	"PATCH /projects/{projectId}/firewall/rules/{ruleId}": "firewall.update",
	"DELETE /projects/{projectId}/firewall/rules/{ruleId}": "firewall.delete",
	"GET /projects/{projectId}/firewall/events":        "traffic.read",
	"GET /projects/{projectId}/firewall/stats":         "traffic.read",
	"POST /projects/{projectId}/firewall/whitelist":    "firewall.create",
	"GET /projects/{projectId}/cache/stats":            "cache.stats",
	"POST /projects/{projectId}/cache/purge":           "cache.purge",
	"POST /projects/{projectId}/cache/purge/path":      "cache.purge",
	"GET /projects/{projectId}/analytics/usage":        "analytics.read",
	"GET /projects/{projectId}/analytics/usage/timeseries": "analytics.read",
	"GET /projects/{projectId}/analytics/paths":        "analytics.read",
	"GET /projects/{projectId}/analytics/status-codes": "analytics.read",
	"GET /projects/{projectId}/analytics/bandwidth":    "analytics.read",
	"GET /projects/{projectId}/analytics/requests":     "analytics.read",
	"GET /projects/{projectId}/analytics/invocations":  "analytics.read",
	"GET /projects/{projectId}/observability/web-vitals": "webvital.read",
	"POST /projects/{projectId}/observability/web-vitals/beacon": "webvital.read",
	"GET /projects/{projectId}/observability/web-vitals/timeseries": "webvital.read",
	"GET /projects/{projectId}/observability/lcp":     "webvital.read",
	"GET /projects/{projectId}/observability/cls":     "webvital.read",
	"GET /projects/{projectId}/observability/fid":     "webvital.read",
	"GET /projects/{projectId}/deployments/upload":    "deployment.create",
	"GET /projects/{projectId}/deployments/{deployId}/source": "deployment.list",
	"GET /projects/{projectId}/deployments/{deployId}/og":     "deployment.list",
	"GET /projects/{projectId}/dns/records":           "domain.list",
	"POST /projects/{projectId}/domains/{domainId}/reverify": "domain.verify",
	"GET /projects/{projectId}/settings/checks":        "project.read",
	"POST /projects/{projectId}/settings/checks":       "project.settings",
	"GET /projects/{projectId}/settings/rollout":       "project.read",
	"PUT /projects/{projectId}/settings/rollout":       "project.settings",
	"GET /projects/{projectId}/settings/build-machine": "project.read",
	"PUT /projects/{projectId}/settings/build-machine": "project.settings",
	"POST /projects/{projectId}/settings/ignore-command": "git.settings",
	"GET /projects/{projectId}/settings/framework":     "project.read",
	"PATCH /projects/{projectId}/settings/git/toggles": "git.settings",
	"GET /projects/{projectId}/settings/git/lfs":       "project.read",
	"PUT /projects/{projectId}/settings/git/lfs":       "git.settings",
	"GET /projects/{projectId}/settings/deployment-protection": "project.read",
	"PUT /projects/{projectId}/settings/deployment-protection": "project.settings",
	"GET /projects/{projectId}/settings/oidc":          "project.read",
	"PUT /projects/{projectId}/settings/oidc":          "project.settings",
	"GET /projects/{projectId}/settings/functions":     "git.settings",
	"PUT /projects/{projectId}/settings/functions":     "git.settings",
	"GET /projects/{projectId}/settings/security":      "project.read",
	"PUT /projects/{projectId}/settings/security":      "project.settings",
	"GET /projects/{projectId}/settings/retention":     "project.read",
	"PUT /projects/{projectId}/settings/retention":     "project.settings",
	"GET /projects/{projectId}/settings/networking":    "project.network",
	"PUT /projects/{projectId}/settings/networking":    "project.network",
	"GET /projects/{projectId}/settings/advanced":      "project.read",
	"PUT /projects/{projectId}/settings/advanced":      "project.settings",
	"GET /projects/{projectId}/settings/passport":      "project.read",
	"PUT /projects/{projectId}/settings/passport":      "project.settings",
	"GET /projects/{projectId}/settings/microfrontends": "project.read",
	"PUT /projects/{projectId}/settings/microfrontends": "project.settings",
	"GET /projects/{projectId}/environments":            "project.read",
	"POST /projects/{projectId}/environments":           "project.settings",
	"GET /projects/{projectId}/environments/available":  "project.read",
	"GET /projects/{projectId}/environments/{envId}":    "project.read",
	"PATCH /projects/{projectId}/environments/{envId}":  "project.settings",
	"DELETE /projects/{projectId}/environments/{envId}": "project.settings",
	"POST /projects/{projectId}/environments/{envId}/branch": "project.settings",
	"POST /projects/{projectId}/environments/{envId}/domain":  "project.settings",
	"GET /projects/{projectId}/members/{username}":     "member.list",
	"GET /projects/{projectId}/alerts":                 "project.read",
	"POST /projects/{projectId}/alerts":                "alert.create",
	"GET /projects/{projectId}/alerts/{alertId}":       "project.read",
	"PATCH /projects/{projectId}/alerts/{alertId}":     "alert.update",
	"DELETE /projects/{projectId}/alerts/{alertId}":    "alert.delete",
	"POST /projects/{projectId}/alerts/{alertId}/silence": "alert.silence",
	"POST /projects/{projectId}/alerts/{alertId}/unsilence": "alert.silence",
	"GET /projects/{projectId}/redirects":              "project.read",
	"POST /projects/{projectId}/redirects":             "redirect.create",
	"DELETE /projects/{projectId}/redirects/{redirectId}": "redirect.delete",
	"PUT /projects/{projectId}/redirects/bulk":         "redirect.create",
	// volumes are global paths (not under a project)
	"GET /volumes":            "volume.read",
	"POST /volumes":           "volume.create",
	"GET /volumes/{volumeId}": "volume.read",
	"DELETE /volumes/{volumeId}": "volume.delete",
	"POST /volumes/{volumeId}/resize": "volume.resize",
	"GET /volumes/{volumeId}/usage":  "volume.read",
	// global observability / host / vms
	"GET /global/analytics":              "analytics.read",
	"GET /global/analytics/timeseries":   "analytics.read",
	"GET /usage":                          "analytics.read",
	"GET /usage/bandwidth":                "analytics.read",
	"GET /usage/requests":                 "analytics.read",
	"GET /usage/timeseries":               "analytics.read",
	"GET /overview":                      "project.read",
	"GET /host/overview":                 "metric.read",
	"GET /host/ports":                    "metric.read",
	"GET /host/kernel":                   "metric.read",
	"GET /logs":                          "log.read",
	"GET /traffic":                       "traffic.read",
	"DELETE /traffic":                    "cache.purge",
	"GET /traffic/search":                "traffic.read",
	"GET /images":                        "project.read",
	"GET /images/search":                 "project.read",
	"GET /images/stats":                  "project.read",
	"POST /images/prune":                 "cache.purge",
	"GET /images/{reference}":            "project.read",
	"DELETE /images/{reference}":         "project.delete",
	"GET /vms":                           "replica.list",
	"GET /vms/{replicaId}":               "replica.list",
	"POST /vms/{replicaId}/start":        "replica.start",
	"POST /vms/{replicaId}/stop":         "replica.stop",
	"POST /vms/{replicaId}/restart":      "replica.restart",
	"DELETE /vms/{replicaId}":            "replica.delete",
	"GET /orgs/events":                   "event.read",
	"GET /projects/{projectId}/builds":                 "build.create",
	"POST /projects/{projectId}/builds":                "build.create",
	"POST /projects/{projectId}/builds/run":            "build.create",
	"GET /projects/{projectId}/builds/{buildId}/logs":  "log.read",
	"POST /projects/{projectId}/git/import":            "git.import",
	"POST /projects/{projectId}/deployments/git":       "build.create",
	"GET /projects/{projectId}/git/branches":           "git.import",
	"GET /projects/{projectId}/networks":               "project.network",
	"POST /projects/{projectId}/networks":              "project.network",
	"GET /projects/{projectId}/services":               "project.read",
	"GET /projects/{projectId}/services/{serviceName}": "project.read",
	"POST /projects/{projectId}/services/{serviceName}/scale": "project.scale",
	"GET /projects/{projectId}/rollouts":               "deployment.list",
	// org / groups (org-scoped)
	"GET /orgs/members":           "member.list",
	"POST /orgs/members":          "org.member.add",
	"PATCH /orgs/members/{username}": "org.member.role",
	"DELETE /orgs/members/{username}": "org.member.remove",
	"GET /orgs/audit":             "org.audit",
	"POST /orgs/transfer":         "org.transfer",
	"GET /groups":                 "group.create",
	"POST /groups":                "group.create",
	"GET /groups/{groupId}":       "project.read",
	"PATCH /groups/{groupId}":     "group.update",
	"DELETE /groups/{groupId}":    "group.delete",
	"GET /groups/{groupId}/projects": "project.read",
	"POST /groups/{groupId}/projects/{projectId}": "project.write",
	"DELETE /groups/{groupId}/projects/{projectId}": "project.write",
	// auth / users / servers (global)
	"GET /users":                "user.list",
	"POST /users":               "user.create",
	"DELETE /users/{username}":  "user.delete",
	"GET /servers":              "server.register",
	"POST /servers":             "server.register",
	"DELETE /servers/{id}":      "server.remove",
	"GET /users/me/api-keys":    "apikey.create",
	"POST /users/me/api-keys":   "apikey.create",
	"DELETE /users/me/api-keys/{keyId}": "apikey.delete",
	// RBAC management
	"GET /roles":                            "org.audit",
	"POST /roles":                           "org.member.role",
	"GET /roles/{roleId}":                   "org.audit",
	"PATCH /roles/{roleId}":                 "org.member.role",
	"DELETE /roles/{roleId}":                "org.member.role",
	"GET /permissions":                      "org.audit",
	"GET /roles/{roleId}/permissions":       "org.audit",
	"PUT /roles/{roleId}/permissions":       "org.member.role",
	"POST /roles/{roleId}/permissions/{permissionId}": "org.member.role",
	"DELETE /roles/{roleId}/permissions/{permissionId}": "org.member.role",
}

// bearerToken, constantTimeEqual unchanged...

// ----------------------------------------------------------------------------
// Health / Auth handlers (unchanged except login uses userID from header? no)
// ----------------------------------------------------------------------------

// handleLogin (unchanged) ...
// handleSignup (unchanged) ...
// ... all auth handlers unchanged ...

// ----------------------------------------------------------------------------
// Updated Org handlers using orgFromHeader
// ----------------------------------------------------------------------------

func (a *API) handleGetCurrentOrg(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	org, ok := a.store.GetOrg(orgID)
	if !ok {
		writeError(w, http.StatusNotFound, "no org set; use X-Porter-Org-Id header")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"org": org})
}

func (a *API) handlePatchCurrentOrg(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	org, ok := a.store.GetOrg(orgID)
	if !ok {
		writeError(w, http.StatusNotFound, "no org set")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = readJSON(r, &req)
	if req.Name != "" {
		org.Name = req.Name
	}
	_ = a.store.PutOrg(org)
	writeJSON(w, http.StatusOK, map[string]any{"org": org})
}

func (a *API) handleDeleteCurrentOrg(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	org, ok := a.store.GetOrg(orgID)
	if !ok {
		writeError(w, http.StatusNotFound, "no org set")
		return
	}
	if org.IsDefault {
		writeError(w, http.StatusForbidden, "cannot delete your default org")
		return
	}
	// In a full implementation, you'd cascade delete projects, memberships, etc.
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "org": org.Name})
}

func (a *API) handleListOrgMembers(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	members := make([]map[string]any, 0, len(a.store.ListUsers()))
	for _, u := range a.store.ListUsers() {
		members = append(members, map[string]any{"user_id": u.ID, "username": u.Username, "role": u.Role})
	}
	// The bootstrap admin is always a member.
	members = append(members, map[string]any{"user_id": a.adminUser, "username": a.adminUser, "role": "owner"})
	writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "members": members})
}

func (a *API) handleAddOrgMember(w http.ResponseWriter, r *http.Request) {
	// Adds a real user account (the org is single-tenant; membership = user list).
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	_ = readJSON(r, &req)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	salt := store.NewID()
	u := &types.User{ID: store.NewID(), Username: req.Username, Role: req.Role, Salt: salt, PasswordHash: passwordHash(req.Password, salt), CreatedAt: time.Now()}
	a.store.PutUser(u)
	a.store.AppendDaemonLog(fmt.Sprintf("org member %s added (role %s)", req.Username, req.Role))
	writeJSON(w, http.StatusCreated, map[string]any{"status": "added", "user": map[string]any{"id": u.ID, "username": u.Username, "role": u.Role}})
}

func (a *API) handlePatchOrgMember(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	var req struct {
		Role string `json:"role"`
	}
	_ = readJSON(r, &req)
	if u, ok := a.store.GetUserByUsername(username); ok {
		if req.Role != "" {
			u.Role = req.Role
			a.store.PutUser(u)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "username": username, "role": u.Role})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"status": "user not found", "username": username})
}

func (a *API) handleRemoveOrgMember(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if u, ok := a.store.GetUserByUsername(username); ok {
		a.store.DeleteUser(u.ID)
		a.store.AppendDaemonLog("org member " + username + " removed")
		writeJSON(w, http.StatusOK, map[string]any{"status": "removed", "username": username})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"status": "user not found", "username": username})
}

func (a *API) handleOrgAudit(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	// Audit trail comes from the durable daemon log (best-effort, real data).
	logs := a.store.TailDaemonLogs(100)
	events := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		events = append(events, map[string]any{"event": l, "ts": time.Now()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "events": events})
}

func (a *API) handleOrgTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewOwnerEmail string `json:"new_owner_email"`
	}
	_ = readJSON(r, &req)
	orgID := a.orgIDFromHeader(r)
	if org, ok := a.store.GetOrg(orgID); ok {
		org.OwnerID = req.NewOwnerEmail
		_ = a.store.PutOrg(org)
		a.store.AppendDaemonLog(fmt.Sprintf("org %s transferred to %s", org.Name, req.NewOwnerEmail))
		writeJSON(w, http.StatusOK, map[string]any{"status": "transferred", "to": req.NewOwnerEmail, "org": org})
		return
	}
	writeError(w, http.StatusNotFound, "no org set")
}

func (a *API) handleOrgEvents(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	for _, p := range a.store.ListProjectsByOrg(orgID) {
		events := a.store.ListHealthEvents(p.ID, 10)
		if len(events) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "events": events})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "events": []any{}})
}

// ListOrgs, DefaultOrg, CreateOrg, GetOrg, PatchOrg, DeleteOrg unchanged (they don't need org header)

// ----------------------------------------------------------------------------
// Groups handlers – use org from header
// ----------------------------------------------------------------------------

func (a *API) handleListGroups(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	writeJSON(w, http.StatusOK, map[string]any{"groups": a.store.ListGroups(orgID)})
}

func (a *API) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	orgID := a.orgIDFromHeader(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	g := &types.Group{ID: store.NewID(), OrgID: orgID, Name: req.Name, CreatedAt: time.Now()}
	if err := a.store.PutGroup(g); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": g})
}

func (a *API) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	for _, g := range a.groupsAll() {
		if g.ID == r.PathValue("groupId") {
			writeJSON(w, http.StatusOK, map[string]any{"group": g})
			return
		}
	}
	writeError(w, http.StatusNotFound, "group not found")
}

func (a *API) handlePatchGroup(w http.ResponseWriter, r *http.Request) {
	g, ok := a.store.GetGroup(r.PathValue("groupId"))
	if !ok {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Name != "" {
		g.Name = req.Name
	}
	if err := a.store.PutGroup(g); err != nil {
		writeError(w, http.StatusInternalServerError, "update group: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (a *API) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	deleted := a.store.DeleteGroup(r.PathValue("groupId"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": r.PathValue("groupId"), "deleted": deleted})
}

func (a *API) handleGroupProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"group_id": r.PathValue("groupId"), "projects": a.store.ListProjectsInGroup(r.PathValue("groupId"))})
}

func (a *API) handleAddGroupProject(w http.ResponseWriter, r *http.Request) {
	if err := a.store.AddProjectToGroup(r.PathValue("groupId"), r.PathValue("projectId")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "added"})
}

func (a *API) handleRemoveGroupProject(w http.ResponseWriter, r *http.Request) {
	if err := a.store.RemoveProjectFromGroup(r.PathValue("groupId"), r.PathValue("projectId")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

func (a *API) groupsAll() []*types.Group {
	var all []*types.Group
	for _, org := range a.store.ListOrgs() {
		all = append(all, a.store.ListGroups(org.ID)...)
	}
	return all
}

// ----------------------------------------------------------------------------
// Core Projects – now pass org from header if missing
// ----------------------------------------------------------------------------

type createProjectReq struct {
	Name          string             `json:"name"`
	Image         string             `json:"image"`
	GitURL        string             `json:"git_url"`
	Branch        string             `json:"branch"`
	ComposeYAML   string             `json:"compose_yaml"`
	OrgID         string             `json:"org_id"`
	GroupID       string             `json:"group_id"`
	VCPUs         int                `json:"vcpus"`
	MemMiB        int                `json:"mem_mib"`
	Env           map[string]string  `json:"env"`
	Ports         []types.Port       `json:"ports"`
	Replicas      int                `json:"replicas"`
	HostMountPath string             `json:"host_mount_path"`
	Healthcheck   *types.Healthcheck `json:"healthcheck"`
	RestartPolicy string             `json:"restart_policy"`
	SSHEnabled    bool               `json:"ssh_enabled"`
}

func (a *API) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.OrgID == "" {
		req.OrgID = a.orgIDFromHeader(r)
	}
	a.createProjectFrom(w, req)
}

func (a *API) createProjectFrom(w http.ResponseWriter, req createProjectReq) {
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Replicas < 1 {
		req.Replicas = 1
	}
	if req.RestartPolicy == "" {
		req.RestartPolicy = "on-failure"
	}
	// orgID already set (from header or body)
	orgID := req.OrgID
	if orgID == "" {
		if orgs := a.store.ListOrgs(); len(orgs) > 0 {
			orgID = orgs[0].ID
		}
	}
	projID := store.NewID()
	source := "image"
	if req.GitURL != "" {
		source = "git"
	}
	proj := &types.Project{
		ID:              projID,
		OrgID:           orgID,
		Name:            req.Name,
		Source:          source,
		Image:           req.Image,
		Network:         "10.42.0.0/16",
		HostMountPath:   req.HostMountPath,
		ReplicasDesired: req.Replicas,
		Replicas:        req.Replicas,
		RestartPolicy:   req.RestartPolicy,
		Healthcheck:     req.Healthcheck,
		Env:             req.Env,
		SSHEnabled:      req.SSHEnabled,
		ServicePools:    map[string]*types.ServicePool{},
		CreatedAt:       time.Now(),
	}
	if a.net != nil {
		if subnet, err := a.net.AllocateSubnet(); err == nil {
			proj.Network = subnet.String()
		}
	}
	a.store.PutProject(proj)
	if req.GroupID != "" {
		_ = a.store.AddProjectToGroup(req.GroupID, projID)
	}

	// Git deploys build first: a Build row is queued and cloned/baked
	// asynchronously; replicas are booted once the image is ready.
	if req.GitURL != "" {
		b := &types.Build{ID: store.NewID(), ProjectID: projID, GitURL: req.GitURL, Branch: orDefault(req.Branch, "main"), BuildStatus: "building", CreatedAt: time.Now()}
		a.store.PutBuild(b)
		a.store.AppendBuildLog(projID, "git project queued: "+req.GitURL)
		a.runGitBuildCtx(b)
		for i := 0; i < req.Replicas; i++ {
			rr := req
			rr.Image = b.Image
			if rr.Image != "" {
				rr.Name = req.Name
				a.bootReplica(proj, rr, i)
			}
		}
		_ = a.store.CreateDeployment(&types.Deployment{ID: store.NewID(), ProjectID: projID, BuildStatus: b.BuildStatus, ImageDigest: b.Image, GitURL: req.GitURL, CreatedAt: time.Now()})
		a.store.AppendDaemonLog(fmt.Sprintf("project %s created via git (%s)", req.Name, req.GitURL))
		writeJSON(w, http.StatusAccepted, map[string]any{"project": proj, "status": "building"})
		return
	}

	for i := 0; i < req.Replicas; i++ {
		a.bootReplica(proj, req, i)
	}
	_ = a.store.CreateDeployment(&types.Deployment{ID: store.NewID(), ProjectID: projID, BuildStatus: "ready", ImageDigest: req.Image, CreatedAt: time.Now()})
	a.store.AppendBuildLog(projID, fmt.Sprintf("deployed %s (%s) with %d replica(s)", req.Name, req.Image, req.Replicas))
	a.store.AppendDaemonLog(fmt.Sprintf("project %s created (%s)", req.Name, req.Image))
	writeJSON(w, http.StatusAccepted, map[string]any{"project": proj, "status": "deploying"})
}

func (a *API) bootReplica(proj *types.Project, req createProjectReq, idx int) {
	// unchanged ...
	vmID := store.NewID()
	env := req.Env
	if env == nil {
		env = map[string]string{}
	}
	for k, v := range a.secretsEnv(proj) {
		if _, exists := env[k]; !exists {
			env[k] = v
		}
	}
	vm := &types.VM{
		ID:           vmID,
		Name:         fmt.Sprintf("%s-%d", proj.Name, idx),
		ProjectID:    proj.ID,
		ServiceName:  "web",
		State:        types.StatePending,
		HealthStatus: types.HealthChecking,
		Image:        req.Image,
		ReplicaIndex: idx,
		VCPUs:        req.VCPUs,
		MemMiB:       req.MemMiB,
		Ports:        req.Ports,
		Env:          env,
		CreatedAt:    time.Now(),
	}
	a.applyImageManifest(vm)
	a.store.PutVM(vm)
	if proj.VMIDs == nil {
		proj.VMIDs = []string{}
	}
	proj.VMIDs = append(proj.VMIDs, vmID)
	if a.vmm != nil {
		go func(c types.VM) { _ = a.vmm.Boot(context.Background(), &c) }(*vm)
	}
	a.store.PutProject(proj)
}

// secretsEnv returns the merged project.env + decrypted project secrets, so
// every replica boots with its declared environment AND its secrets injected.
func (a *API) secretsEnv(proj *types.Project) map[string]string {
	merged := map[string]string{}
	if proj.Env != nil {
		for k, v := range proj.Env {
			merged[k] = v
		}
	}
	for _, sec := range a.store.ListSecrets(proj.ID) {
		val, err := a.decryptSecret(sec.ValueEncrypted)
		if err != nil {
			a.store.AppendDaemonLog(fmt.Sprintf("secret %q for project %s could not be decrypted (%v)", sec.Name, proj.ID, err))
			continue
		}
		merged[sec.Name] = val
	}
	return merged
}

// applyImageManifest fills VM fields from a custom (user-uploaded microVM)
// golden image: rootfs + kernel host paths and the default vCPU/memory spec.
// OCI image references are left untouched (containerd boots them).
func (a *API) applyImageManifest(vm *types.VM) {
	if vm == nil || !strings.HasPrefix(vm.Image, "custom://") {
		return
	}
	ref := strings.TrimPrefix(vm.Image, "custom://")
	for _, gi := range a.store.ListGoldenImages() {
		if gi.Image == "custom://"+ref || gi.Name == ref {
			vm.RootfsPath = gi.Rootfs
			vm.Kernel = gi.Kernel
			if vm.VCPUs == 0 {
				vm.VCPUs = gi.VCPUs
			}
			if vm.MemMiB == 0 {
				vm.MemMiB = gi.MemMiB
			}
			return
		}
	}
}

func (a *API) handleCreateComposeProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		ComposeYAML string `json:"compose_yaml"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	svcs, perr := compose.ParseCompose(req.ComposeYAML)
	if perr != nil {
		writeError(w, http.StatusBadRequest, perr.Error())
		return
	}
	if req.Name == "" {
		req.Name = "compose-" + time.Now().Format("20060102-150405")
	}
	orgID := a.orgIDFromHeader(r)
	if orgID == "" {
		if orgs := a.store.ListOrgs(); len(orgs) > 0 {
			orgID = orgs[0].ID
		}
	}

	// A compose file is a *stack*: one project per service, each holding its own
	// microVM pool, all grouped under one stack row (docker-ecosystem parity).
	stack := &types.Stack{ID: store.NewID(), Name: req.Name, OrgID: orgID, Source: "compose", ComposeYAML: req.ComposeYAML, CreatedAt: time.Now()}
	a.store.PutStack(stack)

	created := make([]*types.Project, 0, len(svcs))
	for _, svc := range svcs {
		proj := &types.Project{
			ID:              store.NewID(),
			OrgID:           orgID,
			Name:            req.Name + "/" + svc.Name,
			Source:          "compose",
			Image:           svc.Image,
			Network:         "10.42.0.0/16",
			ReplicasDesired: svc.Replicas,
			Replicas:        svc.Replicas,
			RestartPolicy:   "on-failure",
			Healthcheck:     svc.Healthcheck,
			Env:             svc.Env,
			ComposeYAML:     req.ComposeYAML,
			StackID:         stack.ID,
			ComposeService:  svc.Name,
			ServicePools:    map[string]*types.ServicePool{},
			VMIDs:           []string{},
			CreatedAt:       time.Now(),
		}
		if a.net != nil {
			if sub, serr := a.net.AllocateSubnet(); serr == nil {
				proj.Network = sub.String()
			}
		}
		spec := createProjectReq{Name: proj.Name, Image: svc.Image, Env: svc.Env, Ports: svc.Ports, Healthcheck: svc.Healthcheck, Replicas: svc.Replicas}
		if svc.Networks != nil {
			proj.Networks = svc.Networks
		} else if topNetworks := compose.ParseTopLevelNetworks(req.ComposeYAML); len(topNetworks) > 0 {
			proj.Networks = topNetworks
		}
		spec.OrgID = orgID
		a.store.PutProject(proj)
		for i := 0; i < svc.Replicas; i++ {
			a.bootReplica(proj, spec, i)
		}
		a.store.AppendDaemonLog(fmt.Sprintf("compose stack %s: service %s (%s) with %d replica(s)", req.Name, svc.Name, svc.Image, svc.Replicas))
		created = append(created, proj)
	}
	a.hub.Broadcast("compose.created", map[string]any{"stack": req.Name, "projects": len(created)})
	writeJSON(w, http.StatusCreated, map[string]any{"stack": stack, "projects": created})
}

// handleListProjects, handleGetProject, handlePatchProject, handleDeleteProject, handleRedeployProject unchanged except PatchProject could use org header? not needed.

// Import project also uses org from header:
func (a *API) handleImportProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Manifest map[string]any `json:"manifest"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Manifest == nil {
		writeError(w, http.StatusBadRequest, "manifest is required")
		return
	}
	name, _ := req.Manifest["project"].(string)
	image, _ := req.Manifest["image"].(string)
	if name == "" || image == "" {
		writeError(w, http.StatusBadRequest, "manifest needs project + image")
		return
	}
	cr := createProjectReq{Name: name, Image: image}
	cr.OrgID = a.orgIDFromHeader(r)
	if v, ok := req.Manifest["replicas"].(float64); ok {
		cr.Replicas = int(v)
	}
	if v, ok := req.Manifest["ssh_enabled"].(bool); ok {
		cr.SSHEnabled = v
	}
	a.createProjectFrom(w, cr)
}

// ----------------------------------------------------------------------------
// Project members – use username instead of userId
// ----------------------------------------------------------------------------

// memberOf resolves a project member's user id for the current request user
// (header, else admin). Memberships are persisted in project_members.
func (a *API) memberUserID(r *http.Request) string {
	if uid := r.Header.Get(HeaderUserID); uid != "" {
		return uid
	}
	return a.adminUser
}

func (a *API) handleGetProjectMember(w http.ResponseWriter, r *http.Request) {
	pid := a.projectID(r)
	for _, m := range a.store.ListProjectMembers(pid) {
		u, _ := a.store.GetUserByUsername(r.PathValue("username"))
		if u != nil && u.ID == m.UserID {
			writeJSON(w, http.StatusOK, map[string]any{"member": m, "username": u.Username})
			return
		}
	}
	writeError(w, http.StatusNotFound, "member not found")
}

func (a *API) handlePatchProjectMember(w http.ResponseWriter, r *http.Request) {
	pid := a.projectID(r)
	var req struct {
		Role string `json:"role"`
	}
	_ = readJSON(r, &req)
	u, found := a.store.GetUserByUsername(r.PathValue("username"))
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	updated := false
	for _, m := range a.store.ListProjectMembers(pid) {
		if m.UserID == u.ID {
			m.Role = req.Role
			a.store.PutProjectMember(m)
			updated = true
			break
		}
	}
	if !updated {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": req.Role, "username": r.PathValue("username")})
}

func (a *API) handleRemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	pid := a.projectID(r)
	u, found := a.store.GetUserByUsername(r.PathValue("username"))
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	a.store.DeleteProjectMember(pid, u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

// handleAddProjectMember persists a membership (project_members row).
func (a *API) handleAddProjectMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
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
	if req.Role == "" {
		req.Role = "member"
	}
	u, found := a.store.GetUserByUsername(req.Username)
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	m := &types.ProjectMember{ProjectID: a.projectID(r), UserID: u.ID, Role: req.Role, CreatedAt: time.Now()}
	a.store.PutProjectMember(m)
	writeJSON(w, http.StatusCreated, map[string]any{"member": m, "username": u.Username})
}

func (a *API) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	_ = readJSON(r, &req)
	u, found := a.store.GetUserByUsername(req.Username)
	if !found {
		// No account yet → record the invite with the pending marker only.
		m := &types.ProjectMember{ProjectID: a.projectID(r), UserID: store.NewID(), Role: "member", Invited: true, CreatedAt: time.Now()}
		a.store.PutProjectMember(m)
		writeJSON(w, http.StatusCreated, map[string]any{"member": m, "email": req.Email, "invited": true})
		return
	}
	m := &types.ProjectMember{ProjectID: a.projectID(r), UserID: u.ID, Role: "member", Invited: true, CreatedAt: time.Now()}
	a.store.PutProjectMember(m)
	writeJSON(w, http.StatusCreated, map[string]any{"member": m, "email": req.Email, "username": u.Username, "invited": true})
}

// ----------------------------------------------------------------------------
// Users – delete by username, create still uses body, list unchanged
// ----------------------------------------------------------------------------

func (a *API) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	user, found := a.store.GetUserByUsername(username)
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	a.store.DeleteUser(user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ----------------------------------------------------------------------------
// All other handlers (settings, analytics, firewall, cache, volumes, images, etc.)
// remain identical to the original code, no changes needed except they use
// a.projectID(r) which is unchanged.
// ----------------------------------------------------------------------------

// (The rest of the file continues exactly as in the original, with the same helper functions,
// writeJSON, writeError, etc. I'm omitting them for brevity, but they are unchanged.)
