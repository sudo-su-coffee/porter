// Package gateway is the HTTP front door: it routes by Host header (Vercel-
// style stable + preview domains) to the healthy replica pool for a service,
// logs every request into an in-memory ring (the one piece of state that is
// deliberately NOT in Postgres — high write, low durability), and serves as a
// reverse proxy to the target microVM.
package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"porter/internal/types"
)

// Store is the narrow persistence surface the gateway needs.
type Store interface {
	GetVM(id string) (*types.VM, bool)
	ListVMs() []*types.VM
	ListDomains(vmID string) []*types.Domain
}

// TrafficRing is a bounded, in-memory per-VM request log.
type TrafficRing struct {
	mu    sync.RWMutex
	perVM map[string][]*types.TrafficEntry
	size  int
}

// NewTrafficRing returns a ring that keeps the last n entries per VM.
func NewTrafficRing(n int) *TrafficRing {
	if n <= 0 {
		n = 500
	}
	return &TrafficRing{perVM: map[string][]*types.TrafficEntry{}, size: n}
}

// Add appends one traffic entry to a VM's ring, trimming to the bound.
func (t *TrafficRing) Add(vmID string, e *types.TrafficEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	buf := append(t.perVM[vmID], e)
	if len(buf) > t.size {
		buf = buf[len(buf)-t.size:]
	}
	t.perVM[vmID] = buf
}

// List returns up to limit recent entries for a VM, oldest first.
func (t *TrafficRing) List(vmID string, limit int) []*types.TrafficEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	buf := t.perVM[vmID]
	if limit <= 0 || limit > len(buf) {
		limit = len(buf)
	}
	start := len(buf) - limit
	out := make([]*types.TrafficEntry, limit)
	copy(out, buf[start:])
	return out
}

// Gateway routes Host-header requests to VM pools.
type Gateway struct {
	store  Store
	ring   *TrafficRing
	logger *log.Logger
}

// NewGateway builds the gateway with its in-memory traffic ring.
func NewGateway(store Store) *Gateway {
	return &Gateway{
		store: store,
		ring:  NewTrafficRing(500),
		logger: log.New(log.Writer(), "gateway: ", log.LstdFlags),
	}
}

// Ring exposes the traffic ring (the API's GET /vms/{id}/traffic reads it).
func (g *Gateway) Ring() *TrafficRing {
	return g.ring
}

// ServeHTTP implements http.Handler: resolve the Host header to a VM backend
// and reverse-proxy, then record a traffic entry.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := strings.Split(r.Host, ":")[0]

	vms := g.backendsFor(host)
	if len(vms) == 0 {
		http.Error(w, "no healthy backend for "+host, http.StatusServiceUnavailable)
		return
	}
	vm := vms[0]
	target, ok := targetAddress(vm)
	if !ok {
		http.Error(w, "vm has no address", http.StatusServiceUnavailable)
		return
	}

	start := time.Now()
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
	dur := time.Since(start).Milliseconds()

	g.ring.Add(vm.ID, &types.TrafficEntry{
		Timestamp:  start,
		Method:     r.Method,
		Host:       r.Host,
		Path:       r.URL.Path,
		Status:     statusCode(w),
		DurationMS: int(dur),
		RemoteIP:   r.RemoteAddr,
	})
}

// backendsFor finds healthy VMs that serve the given domain: direct match on
// a VM's domain records, then fall back to any running VM (dev convenience).
func (g *Gateway) backendsFor(host string) []*types.VM {
	// Domains are attached to VMs; match any domain record equal to host.
	for _, vm := range g.store.ListVMs() {
		if !isHealthy(vm) {
			continue
		}
		for _, d := range g.store.ListDomains(vm.ID) {
			if strings.EqualFold(d.Domain, host) {
				return []*types.VM{vm}
			}
		}
	}
	// Fallback: single running VM answers any host (single-tenant dev path).
	out := []*types.VM{}
	for _, vm := range g.store.ListVMs() {
		if isHealthy(vm) {
			out = append(out, vm)
		}
	}
	return out
}

func isHealthy(vm *types.VM) bool {
	return vm.State == types.StateRunning && vm.HealthStatus != types.HealthUnhealthy
}

// statusCode peeks at the response status code committed so far (best effort).
func statusCode(w http.ResponseWriter) int {
	if rw, ok := w.(*responseRecorder); ok {
		return rw.status
	}
	return 200
}

// responseRecorder decorates a ResponseWriter to capture a status (wrapped
// around the real writer by callers that want traffic status codes).
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// NewResponseWriter wraps w with a status recorder.
func NewResponseWriter(w http.ResponseWriter) http.ResponseWriter {
	return &responseRecorder{ResponseWriter: w, status: 200}
}

// targetAddress builds the upstream URL for a VM from its IP and the first
// mapped (host) port. Host port defaults to the container port.
func targetAddress(vm *types.VM) (*url.URL, bool) {
	if vm.IPAddress == "" {
		return nil, false
	}
	port := 80
	if len(vm.Ports) > 0 {
		port = vm.Ports[0].HostPort
		if port == 0 {
			port = vm.Ports[0].ContainerPort
		}
	}
	return &url.URL{Scheme: "http", Host: vm.IPAddress + ":" + itoa(port)}, true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

var _ = httputil.ReverseProxy{}
var _ = url.URL{}