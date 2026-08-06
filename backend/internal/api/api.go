// Package api implements the Porter control-plane HTTP API on Go 1.22+
// net/http.ServeMux pattern routing (no external router dependency).
package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"porter/internal/compose"
	"porter/internal/event"
	"porter/internal/imagecatalog"
	netmgr "porter/internal/net"
	"porter/internal/runtime"
	"porter/internal/store"
	"porter/internal/types"
)

type API struct {
	store      *store.Store
	hub        *event.Hub
	vmm        *runtime.VMManager
	net        *netmgr.NetManager
	catalog    *imagecatalog.Catalog
	token      string
	baseDomain string
	version    string
	startedAt  time.Time

	// Single prototype admin account (from porter.toml [admin], no user
	// database). /login checks these and, on success, hands back the same
	// token every other endpoint expects on the Authorization header.
	adminUser string
	adminPass string
}

func NewAPI(store *store.Store, hub *event.Hub, vmm *runtime.VMManager, net *netmgr.NetManager, catalog *imagecatalog.Catalog, token, baseDomain, adminUser, adminPass, version string) *API {
	return &API{
		store: store, hub: hub, vmm: vmm, net: net, catalog: catalog,
		token: token, baseDomain: baseDomain,
		adminUser: adminUser, adminPass: adminPass, version: version,
		startedAt: time.Now().UTC(),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Routes registers every endpoint on a stdlib net/http.ServeMux.
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("POST /login", a.handleLogin)

	mux.Handle("GET /vms", a.auth(a.handleListVMs))
	mux.Handle("POST /vms", a.auth(a.handleCreateVM))
	mux.Handle("GET /vms/{id}", a.auth(a.handleGetVM))
	mux.Handle("POST /vms/{id}/stop", a.auth(a.handleStopVM))
	mux.Handle("POST /vms/{id}/start", a.auth(a.handleStartVM))
	mux.Handle("POST /vms/{id}/restart", a.auth(a.handleRestartVM))
	mux.Handle("DELETE /vms/{id}", a.auth(a.handleDeleteVM))

	mux.Handle("GET /vms/{id}/domains", a.auth(a.handleListDomains))
	mux.Handle("POST /vms/{id}/domains", a.auth(a.handleAddDomain))
	mux.Handle("DELETE /vms/{id}/domains/{domain}", a.auth(a.handleRemoveDomain))

	mux.Handle("GET /vms/{id}/traffic", a.auth(a.handleTraffic))
	mux.Handle("GET /vms/{id}/logs", a.auth(a.handleLogs))
	mux.Handle("GET /vms/{id}/ssh-info", a.auth(a.handleSSHInfo))

	mux.Handle("GET /images", a.auth(a.handleImages))
	mux.Handle("GET /overview", a.auth(a.handleOverview))

	mux.Handle("GET /users", a.auth(a.handleListUsers))
	mux.Handle("POST /users", a.auth(a.handleCreateUser))
	mux.Handle("DELETE /users/{id}", a.auth(a.handleDeleteUser))

	mux.Handle("GET /projects", a.auth(a.handleListProjects))
	mux.Handle("POST /projects/compose", a.auth(a.handleCreateComposeProject))
	mux.Handle("GET /projects/{id}", a.auth(a.handleGetProject))
	mux.Handle("PATCH /projects/{id}/services/{service}/scale", a.auth(a.handleScale))
	mux.Handle("DELETE /projects/{id}/services/{service}", a.auth(a.handleRemoveService))
	mux.Handle("DELETE /projects/{id}", a.auth(a.handleDeleteProject))

	mux.HandleFunc("GET /events", a.hub.ServeHTTP)
}

func (a *API) auth(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") || strings.TrimPrefix(hdr, "Bearer ") != a.token {
			writeErr(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		h(w, r)
	})
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": a.version})
}

// --- Login ---

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// 1) The bootstrap admin from porter.toml [admin].
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(a.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(a.adminPass)) == 1
	if userOK && passOK {
		writeJSON(w, http.StatusOK, map[string]string{"token": a.token})
		return
	}

	// 2) Any additional user stored in SQLite (see /users).
	if dbUser, ok := a.store.GetUserByUsername(req.Username); ok {
		if verifyPassword(req.Password, dbUser.PasswordHash, dbUser.Salt) {
			writeJSON(w, http.StatusOK, map[string]string{"token": a.token})
			return
		}
	}

	time.Sleep(300 * time.Millisecond) // a small, cheap brake on brute-forcing
	writeErr(w, http.StatusUnauthorized, "invalid username or password")
}

// --- Users (accounts beyond the bootstrap [admin]) ---

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListUsers())
}

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "\"username\" and \"password\" are required")
		return
	}
	if _, ok := a.store.GetUserByUsername(req.Username); ok {
		writeErr(w, http.StatusConflict, "user already exists")
		return
	}
	if req.Role == "" {
		req.Role = "admin"
	}
	hash, salt := hashPassword(req.Password)
	u := &types.User{
		ID:           store.NewID(),
		Username:     req.Username,
		Role:         req.Role,
		PasswordHash: hash,
		Salt:         salt,
		CreatedAt:    time.Now().UTC(),
	}
	a.store.PutUser(u)
	writeJSON(w, http.StatusCreated, u)
}

func (a *API) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	a.store.DeleteUser(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// hashPassword returns a salted SHA-256 hash of password. Placeholder KDF
// for v0.1.0 (stdlib-only); swap for bcrypt/argon2 in a hardening pass.
func hashPassword(password string) (hash, salt string) {
	sb := make([]byte, 16)
	_, _ = rand.Read(sb)
	salt = hex.EncodeToString(sb)
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:]), salt
}

func verifyPassword(password, hash, salt string) bool {
	h := sha256.Sum256([]byte(salt + password))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(hash)) == 1
}

// --- VMs ---

func (a *API) handleListVMs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListVMs())
}

type createVMReq struct {
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	Rootfs string            `json:"rootfs"`
	VCPUs  int               `json:"vcpus"`
	MemMiB int               `json:"mem_mib"`
	Env    map[string]string `json:"env"`
	Ports  []types.Port      `json:"ports"`
}

func (a *API) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var req createVMReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Image == "" {
		writeErr(w, http.StatusBadRequest, "\"image\" is required")
		return
	}
	if req.Name == "" {
		req.Name = strings.SplitN(req.Image, ":", 2)[0]
	}
	if req.VCPUs == 0 {
		req.VCPUs = 1
	}
	if req.MemMiB == 0 {
		req.MemMiB = 256
	}

	vm := &types.VM{
		ID:           store.NewID(),
		Name:         req.Name,
		State:        types.StatePending,
		HealthStatus: types.HealthChecking,
		ReplicaIndex: 0,
		Image:        req.Image,
		VCPUs:        req.VCPUs,
		MemMiB:       req.MemMiB,
		Env:          req.Env,
		Ports:        req.Ports,
		CreatedAt:    time.Now().UTC(),
	}
	a.store.PutVM(vm)

	subnet := a.net.AllocateProjectSubnet()
	spec := a.net.AllocateVMNetwork(subnet, 0, vm.ID)
	a.vmm.Boot(vm, spec)
	a.registerStableDomain(vm)

	writeJSON(w, http.StatusAccepted, vm)
}

func (a *API) handleGetVM(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

func (a *API) handleStopVM(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	a.vmm.Stop(vm)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (a *API) handleStartVM(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	subnet := a.net.AllocateProjectSubnet()
	spec := a.net.AllocateVMNetwork(subnet, vm.ReplicaIndex, vm.ID)
	a.vmm.Boot(vm, spec)
	writeJSON(w, http.StatusAccepted, vm)
}

func (a *API) handleRestartVM(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	a.vmm.Stop(vm)
	subnet := a.net.AllocateProjectSubnet()
	spec := a.net.AllocateVMNetwork(subnet, vm.ReplicaIndex, vm.ID)
	a.vmm.Boot(vm, spec)
	writeJSON(w, http.StatusAccepted, vm)
}

func (a *API) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vm, ok := a.store.GetVM(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	if vm.State == types.StateRunning || vm.State == types.StateBooting {
		a.vmm.Stop(vm)
	}
	a.store.DeleteVM(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Domains ---

func (a *API) registerStableDomain(vm *types.VM) {
	if a.baseDomain == "" {
		return
	}
	host := vm.Name
	if vm.ServiceName != "" {
		host = vm.ServiceName
	}
	a.store.AddDomain(vm.ID, &types.Domain{
		Domain: fmt.Sprintf("%s.%s", host, a.baseDomain),
		Type:   "stable",
		Status: "verified",
	})
}

func (a *API) handleListDomains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListDomains(r.PathValue("id")))
}

func (a *API) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := a.store.GetVM(id); !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Domain == "" {
		writeErr(w, http.StatusBadRequest, "\"domain\" is required")
		return
	}
	d := &types.Domain{Domain: req.Domain, Type: "custom", Status: "pending"}
	a.store.AddDomain(id, d)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"domain": d.Domain, "type": d.Type, "status": d.Status,
		"required_record": map[string]string{
			"type": "CNAME", "name": d.Domain, "value": "gateway." + a.baseDomain,
		},
	})
}

func (a *API) handleRemoveDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	domain := r.PathValue("domain")
	if !a.store.RemoveDomain(id, domain) {
		writeErr(w, http.StatusNotFound, "domain not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// --- Traffic ---

func (a *API) handleTraffic(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, a.store.ListTraffic(r.PathValue("id"), limit))
}

// --- Logs ---

func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			tail = n
		}
	}
	id := r.PathValue("id")
	if _, ok := a.store.GetVM(id); !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vm_id": id, "lines": a.store.TailLogs(id, tail)})
}

// --- Images & Overview ---

func (a *API) handleImages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.catalog.All())
}

func (a *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	vms := a.store.ListVMs()
	running := 0
	failed := 0
	for _, vm := range vms {
		switch vm.State {
		case types.StateRunning:
			running++
		case types.StateFailed:
			failed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host":         host,
		"version":      a.version,
		"vm_total":     len(vms),
		"vm_running":   running,
		"vm_failed":    failed,
		"projects":     len(a.store.ListProjects()),
		"images":       len(a.catalog.All()),
		"started_at":   a.startedAt,
	})
}

// --- SSH ---

func (a *API) handleSSHInfo(w http.ResponseWriter, r *http.Request) {
	vm, ok := a.store.GetVM(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "vm not found")
		return
	}
	if vm.State != types.StateRunning {
		writeErr(w, http.StatusConflict, fmt.Sprintf("vm is %s, not running — no task to exec into", vm.State))
		return
	}
	target := vm.Name
	if vm.ProjectID != "" {
		target = fmt.Sprintf("%s-%s-%d", vm.ProjectID[:8], vm.ServiceName, vm.ReplicaIndex)
	}
	gwHost := "gateway." + a.baseDomain
	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_host": gwHost,
		"gateway_port": 2222,
		"target_name":  target,
		"command":      fmt.Sprintf("ssh %s@%s -p 2222", target, gwHost),
	})
}

// --- Projects / Compose ---

func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.ListProjects())
}

func (a *API) handleCreateComposeProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		ComposeYAML string `json:"compose_yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	svcs, err := compose.ParseCompose(req.ComposeYAML)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	proj := &types.Project{
		ID:           store.NewID(),
		Name:         req.Name,
		Source:       "compose",
		Network:      a.net.AllocateProjectSubnet(),
		ComposeYAML:  req.ComposeYAML,
		ServicePools: map[string]*types.ServicePool{},
		CreatedAt:    time.Now().UTC(),
	}
	a.store.PutProject(proj)

	go func() {
		for _, svc := range svcs {
			a.bootService(proj, svc)
			a.hub.Broadcast("project.progress", map[string]any{
				"project_id": proj.ID,
			})
		}
	}()

	writeJSON(w, http.StatusAccepted, proj)
}

func (a *API) bootService(proj *types.Project, svc compose.ComposeService) {
	pool := &types.ServicePool{Desired: svc.Replicas}
	proj.ServicePools[svc.Name] = pool
	a.store.PutProject(proj)

	for i := 0; i < svc.Replicas; i++ {
		vm := &types.VM{
			ID:           store.NewID(),
			Name:         fmt.Sprintf("%s-%s-%d", proj.Name, svc.Name, i),
			ProjectID:    proj.ID,
			ServiceName:  svc.Name,
			State:        types.StatePending,
			HealthStatus: types.HealthChecking,
			ReplicaIndex: i,
			Image:        svc.Image,
			VCPUs:        1,
			MemMiB:       256,
			Env:          svc.Env,
			Ports:        svc.Ports,
			Healthcheck:  svc.Healthcheck,
			Restart:      svc.Restart,
			CreatedAt:    time.Now().UTC(),
		}
		a.store.PutVM(vm)
		proj.VMIDs = append(proj.VMIDs, vm.ID)
		pool.VMs = append(pool.VMs, vm.ID)
		a.store.PutProject(proj)

		spec := a.net.AllocateVMNetwork(proj.Network, i, vm.ID)
		a.vmm.Boot(vm, spec)
		if i == 0 {
			a.registerStableDomain(vm)
		}
	}
}

func (a *API) handleGetProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	if r.URL.Query().Get("expand") == "vms" {
		vms := make([]*types.VM, 0, len(proj.VMIDs))
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

func (a *API) handleScale(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	service := r.PathValue("service")
	pool, ok := proj.ServicePools[service]
	if !ok {
		writeErr(w, http.StatusNotFound, "service not found in project")
		return
	}
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Replicas < 0 {
		writeErr(w, http.StatusBadRequest, "\"replicas\" must be a non-negative integer")
		return
	}

	current := len(pool.VMs)
	pool.Desired = req.Replicas
	a.store.PutProject(proj)

	go func() {
		if req.Replicas > current {
			var sampleImage string
			var sampleEnv map[string]string
			var samplePorts []types.Port
			var sampleHC *types.Healthcheck
			if len(pool.VMs) > 0 {
				if vm, ok := a.store.GetVM(pool.VMs[0]); ok {
					sampleImage, sampleEnv, samplePorts, sampleHC = vm.Image, vm.Env, vm.Ports, vm.Healthcheck
				}
			}
			for i := current; i < req.Replicas; i++ {
				vm := &types.VM{
					ID: store.NewID(), Name: fmt.Sprintf("%s-%s-%d", proj.Name, service, i),
					ProjectID: proj.ID, ServiceName: service, State: types.StatePending,
					HealthStatus: types.HealthChecking, ReplicaIndex: i, Image: sampleImage,
					VCPUs: 1, MemMiB: 256, Env: sampleEnv, Ports: samplePorts,
					Healthcheck: sampleHC, CreatedAt: time.Now().UTC(),
				}
				a.store.PutVM(vm)
				proj.VMIDs = append(proj.VMIDs, vm.ID)
				pool.VMs = append(pool.VMs, vm.ID)
				a.store.PutProject(proj)
				spec := a.net.AllocateVMNetwork(proj.Network, i, vm.ID)
				a.vmm.Boot(vm, spec)
			}
		} else if req.Replicas < current {
			for len(pool.VMs) > req.Replicas {
				last := pool.VMs[len(pool.VMs)-1]
				pool.VMs = pool.VMs[:len(pool.VMs)-1]
				a.store.PutProject(proj)
				if vm, ok := a.store.GetVM(last); ok {
					a.vmm.Stop(vm)
				}
			}
		}
		a.hub.Broadcast("pool.updated", map[string]any{
			"project_id": proj.ID, "service": service,
			"desired": pool.Desired, "healthy": len(pool.VMs),
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"service": service, "desired": req.Replicas, "current": current, "vms": pool.VMs,
	})
}

func (a *API) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	proj, ok := a.store.GetProject(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	service := r.PathValue("service")
	pool, ok := proj.ServicePools[service]
	if !ok {
		writeErr(w, http.StatusNotFound, "service not found in project")
		return
	}
	for _, id := range pool.VMs {
		if vm, ok := a.store.GetVM(id); ok {
			a.vmm.Stop(vm)
			a.store.DeleteVM(id)
		}
	}
	delete(proj.ServicePools, service)
	a.store.PutProject(proj)
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (a *API) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, ok := a.store.GetProject(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	for _, vmID := range proj.VMIDs {
		if vm, ok := a.store.GetVM(vmID); ok {
			a.vmm.Stop(vm)
			a.store.DeleteVM(vmID)
		}
	}
	a.store.DeleteProject(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}