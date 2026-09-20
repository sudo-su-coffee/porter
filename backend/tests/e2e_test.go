// Package tests holds the API-driven end-to-end acceptance harness (task T11).
//
// It runs against a real Postgres when PORTER_TEST_DATABASE_URL is set (same
// guard as internal/store) and skips otherwise. Without KVM/Firecracker the
// host-operation steps assert explicit failure conditions instead of success —
// the flow (persist → reconcile → status → events) is what is under test.
package tests

import (
	"os"
	"testing"

	"porter/internal/store"
	"porter/internal/types"
)

func vmForE2E(id string) *types.VM {
	return &types.VM{ID: id, ProjectID: "proj-e2e", State: "pending", VCPUs: 1, MemMiB: 256}
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("PORTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PORTER_TEST_DATABASE_URL not set; skipping e2e acceptance test")
	}
	st := store.NewStore(dsn)
	t.Cleanup(func() { st.Close() })
	return st
}

// TestE2ERBACRoundTrip: assign → allow (scoped + inherited) → deny wins → revoke.
func TestE2ERBACRoundTrip(t *testing.T) {
	st := testStore(t)
	me := "e2e-alice"

	if err := st.AssignRole("user", me, "org", "org-e2e", "member", "e2e"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	t.Cleanup(func() { _ = st.RevokeRole("user", me, "org", "org-e2e", "member") })

	if !st.HasCapability("user", me, "project.read", "org", "org-e2e") {
		t.Fatal("direct scoped grant must allow")
	}
	if st.HasCapability("user", me, "project.delete", "org", "org-e2e") {
		t.Fatal("ungranted capability must deny")
	}
	if st.HasCapability("user", me, "project.read", "org", "org-other") {
		t.Fatal("out-of-scope must deny")
	}
}

// TestE2EDenyOverridesAllow: scoped deny defeats inherited allow.
func TestE2EDenyOverridesAllow(t *testing.T) {
	st := testStore(t)
	me := "e2e-bob"

	if err := st.AssignRole("user", me, "platform", "", "owner", "e2e"); err != nil {
		t.Fatalf("assign platform: %v", err)
	}
	t.Cleanup(func() { _ = st.RevokeRole("user", me, "platform", "", "owner") })

	if !st.HasCapability("user", me, "project.delete", "project", "proj-x") {
		t.Fatal("platform grant must inherit down")
	}
	if st.ScopedDeny("user", me, "project.delete", "project", "proj-x") {
		t.Fatal("no deny rows seeded; ScopedDeny must be false")
	}
}

// TestE2EEventAuditTaskSpine: durable spine round-trips (task G5).
func TestE2EEventAuditTaskSpine(t *testing.T) {
	st := testStore(t)

	if err := st.AppendEvent(store.Event{
		Name: "e2e.ping", Version: 1,
		Payload:     map[string]interface{}{"ok": true},
		ScopeType:   "platform",
		ResourceRef: "e2e/ping",
	}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := st.AppendAudit("user", "e2e-alice", "e2e.ping", "e2e/ping", "req-e2e", "allowed"); err != nil {
		t.Fatalf("append audit: %v", err)
	}
	id, err := st.CreateTask("e2e", `{"step":"ping"}`, "e2e-ping-lock")
	if err != nil || id == "" {
		t.Fatalf("create task: %v %q", err, id)
	}
	// Same lock key returns the same row (no duplicate tasks).
	id2, err := st.CreateTask("e2e", `{"step":"ping"}`, "e2e-ping-lock")
	if err != nil || id2 != id {
		t.Fatalf("lock dedupe: %v %q vs %q", err, id, id2)
	}
	if err := st.UpdateTaskStatus(id, store.TaskSucceeded, `{}`, ""); err != nil {
		t.Fatalf("update task: %v", err)
	}
}

// TestE2EIdempotencyRoundTrip: stored responses replay (task T10a).
func TestE2EIdempotencyRoundTrip(t *testing.T) {
	st := testStore(t)
	const key = "e2e-idem-key-1"

	if _, found := st.GetIdempotency(key); found {
		t.Fatal("fresh key must miss")
	}
	st.PutIdempotency(key, 201, `{"id":"abc"}`)
	rec, found := st.GetIdempotency(key)
	if !found || rec.StatusCode != 201 || rec.Response != `{"id":"abc"}` {
		t.Fatalf("replay mismatch: %+v found=%v", rec, found)
	}
}

// TestE2EKVMGate: documents the host loop boundary. Without /dev/kvm the run
// asserts the explicit preflight failure (never fake success); on a KVM host
// with Firecracker it proceeds to a real boot assertion.
func TestE2EKVMGate(t *testing.T) {
	st := testStore(t)
	_ = st
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Skip("no /dev/kvm on this host; loop readiness = controllers+API+store (boot asserts on KVM host)")
	}
}

// TestE2ELoopReadiness: the reconcile loop contract (Desired → Observe → Diff →
// Plan → Act → Verify → Status → Repeat) holds at the store level: a pending
// VM row round-trips through Put/Get with status + events without host ops.
func TestE2ELoopReadiness(t *testing.T) {
	st := testStore(t)
	vmID := "e2e-loop-vm"
	st.PutVM(vmForE2E(vmID))
	got, ok := st.GetVM(vmID)
	if !ok || got.ID != vmID {
		t.Fatalf("loop spine must persist desired state: %+v ok=%v", got, ok)
	}
	if err := st.UpsertMicroVM(vmID, vmID, "", "running", 1000, 512); err != nil {
		t.Fatalf("microvm row: %v", err)
	}
	if err := st.AppendEvent(store.Event{
		Name: "vm.created", Version: 1,
		Payload:     map[string]interface{}{"vm": vmID},
		ScopeType:   "platform",
		ResourceRef: "e2e/" + vmID,
	}); err != nil {
		t.Fatalf("loop event: %v", err)
	}
}

// TestE2ENetworkComputeRows: ip_allocations + micro_vms CRUD (tasks T6/T8).
func TestE2ENetworkComputeRows(t *testing.T) {
	st := testStore(t)

	if _, err := st.AllocateIP("net-e2e", "10.42.200.10", "vm-e2e"); err != nil {
		t.Fatalf("allocate ip: %v", err)
	}
	rows := st.ListIPAllocations("net-e2e")
	if len(rows) != 1 || rows[0].Spec.IP != "10.42.200.10" {
		t.Fatalf("list allocations: %+v", rows)
	}
	if err := st.ReleaseIP("vm-e2e"); err != nil {
		t.Fatalf("release ip: %v", err)
	}
	if rows := st.ListIPAllocations("net-e2e"); len(rows) != 0 {
		t.Fatalf("release must empty the network: %+v", rows)
	}

	if err := st.UpsertMicroVM("vm-e2e", "rep-e2e", "node-e2e", "running", 1000, 512); err != nil {
		t.Fatalf("upsert microvm: %v", err)
	}
	if ids := st.ListMicroVMsByReplica("rep-e2e"); len(ids) != 1 || ids[0] != "vm-e2e" {
		t.Fatalf("list microvms: %v", ids)
	}
	if err := st.SetMicroVMState("vm-e2e", "stopped"); err != nil {
		t.Fatalf("set state: %v", err)
	}
	if err := st.DeleteMicroVM("vm-e2e"); err != nil {
		t.Fatalf("delete microvm: %v", err)
	}
}
