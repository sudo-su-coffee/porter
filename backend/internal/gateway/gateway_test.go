package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"porter/internal/types"
)

// fakeStore implements the gateway Store surface in memory.
type fakeStore struct {
	vms     []*types.VM
	domains map[string][]*types.Domain
	traffic []*types.TrafficEntry
}

func (f *fakeStore) GetVM(id string) (*types.VM, bool) {
	for _, vm := range f.vms {
		if vm.ID == id {
			return vm, true
		}
	}
	return nil, false
}
func (f *fakeStore) ListVMs() []*types.VM                    { return f.vms }
func (f *fakeStore) ListDomains(vmID string) []*types.Domain { return f.domains[vmID] }
func (f *fakeStore) AddTraffic(vmID string, e *types.TrafficEntry) {
	f.traffic = append(f.traffic, e)
}

func healthyVM(id, ip string, ports ...int) *types.VM {
	var ps []types.Port
	for _, p := range ports {
		ps = append(ps, types.Port{ContainerPort: p, HostPort: p})
	}
	return &types.VM{
		ID: id, Name: id, State: types.StateRunning,
		HealthStatus: types.HealthHealthy, IPAddress: ip, Ports: ps,
	}
}

func TestTrafficRingCapsAndOrder(t *testing.T) {
	ring := NewTrafficRing(3)
	for i := 0; i < 10; i++ {
		ring.Add("vm1", &types.TrafficEntry{Method: "GET", Path: "/"})
	}
	got := ring.List("vm1", 0)
	if len(got) != 3 {
		t.Fatalf("expected ring capped at 3, got %d", len(got))
	}
	if got[0].Path != "/" {
		t.Fatal("expected oldest-first order")
	}
}

func TestGatewayProxiesAndRecordsTraffic(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	st := &fakeStore{vms: []*types.VM{healthyVM("vm1", host, port)}}
	gw := NewGateway(st)

	req := httptest.NewRequest(http.MethodGet, "http://web.myapp.test/", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(st.traffic) != 1 {
		t.Fatalf("expected 1 store traffic entry, got %d", len(st.traffic))
	}
	if st.traffic[0].Host != "web.myapp.test" {
		t.Fatalf("unexpected traffic host %q", st.traffic[0].Host)
	}
	if got := len(gw.Ring().List("vm1", 0)); got != 1 {
		t.Fatalf("expected 1 ring entry, got %d", got)
	}
}

func TestGatewayNoBackendReturns503(t *testing.T) {
	gw := NewGateway(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "http://nobody.test/", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

type dnsResolverFunc func(context.Context, string) ([]net.IP, error)

func (f dnsResolverFunc) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return f(ctx, host)
}

func TestGatewayResolvesLocalDNS(t *testing.T) {
	st := &fakeStore{vms: []*types.VM{healthyVM("vm1", "10.0.0.5")}}
	gw := NewGateway(st)
	gw.SetDNS(dnsResolverFunc(func(_ context.Context, host string) ([]net.IP, error) {
		if host == "web.myproj.local" {
			return []net.IP{net.ParseIP("10.0.0.5")}, nil
		}
		return nil, nil
	}))

	vms := gw.backendsFor("web.myproj.local")
	if len(vms) != 1 || vms[0].ID != "vm1" {
		t.Fatalf("expected vm1 via dns, got %+v", vms)
	}
}
