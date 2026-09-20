package store

import (
	"os"
	"testing"

	"porter/internal/types"
)

// testDSN returns a PostgreSQL DSN from the environment, or "" when no test DB
// is configured. Postgres-backed tests skip when unset.
func testDSN() string { return os.Getenv("PORTER_TEST_DATABASE_URL") }

func TestTrafficRing(t *testing.T) {
	s := &Store{traffic: map[string][]*types.TrafficEntry{}}
	defer s.Close()
	for i := 0; i < trafficRingSize+10; i++ {
		s.AddTraffic("vm1", &types.TrafficEntry{Method: "GET", Path: "/"})
	}
	got := s.ListTraffic("vm1", 0)
	if len(got) != trafficRingSize {
		t.Fatalf("expected ring capped at %d, got %d", trafficRingSize, len(got))
	}
}

func TestLogRing(t *testing.T) {
	s := &Store{logs: map[string][]string{}}
	defer s.Close()
	for i := 0; i < logRingSize+5; i++ {
		s.AppendLog("vm1", "line")
	}
	got := s.TailLogs("vm1", 0)
	if len(got) != logRingSize {
		t.Fatalf("expected log ring capped at %d, got %d", logRingSize, len(got))
	}
}

// TestVMCRUD exercises Postgres-backed persistence; requires a live DB.
func TestVMCRUD(t *testing.T) {
	if testDSN() == "" {
		t.Skip("PORTER_TEST_DATABASE_URL not set; skipping Postgres CRUD test")
	}
	s := NewStore(testDSN())
	defer s.Close()
	vm := &types.VM{ID: "abc", Name: "cache", State: types.StateRunning}
	s.PutVM(vm)

	got, ok := s.GetVM("abc")
	if !ok {
		t.Fatal("expected vm to exist after PutVM")
	}
	if got.Name != "cache" || got.State != types.StateRunning {
		t.Fatalf("unexpected vm: %+v", got)
	}

	list := s.ListVMs()
	if len(list) != 1 {
		t.Fatalf("expected 1 vm, got %d", len(list))
	}

	s.DeleteVM("abc")
	if _, ok := s.GetVM("abc"); ok {
		t.Fatal("expected vm to be deleted")
	}
}
