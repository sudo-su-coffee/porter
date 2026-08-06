package store

import (
	"path/filepath"
	"testing"

	"porter/internal/types"
)

func TestVMCRUD(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "test.db"))
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

func TestTrafficRing(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "test.db"))
	defer s.Close()
	for i := 0; i < trafficRingSize+10; i++ {
		s.AddTraffic("vm1", &types.TrafficEntry{Method: "GET", Path: "/"})
	}
	got := s.ListTraffic("vm1", 0)
	if len(got) != trafficRingSize {
		t.Fatalf("expected ring capped at %d, got %d", trafficRingSize, len(got))
	}
}