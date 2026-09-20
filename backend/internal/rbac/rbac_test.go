package rbac

import (
	"context"
	"testing"
)

// fakeResolver is an in-memory Resolver for engine tests.
type fakeResolver struct {
	assignments []Assignment
	caps        map[string][]RoleCapability // roleID → entries
	ancestors   map[Scope][]Scope
}

func (f *fakeResolver) AssignmentsForPrincipal(_ context.Context, ptype, pid string) ([]Assignment, error) {
	var out []Assignment
	for _, a := range f.assignments {
		if a.PrincipalType == ptype && a.PrincipalID == pid {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeResolver) RoleCapabilities(_ context.Context, roleIDs []string) ([]RoleCapability, error) {
	var out []RoleCapability
	for _, id := range roleIDs {
		out = append(out, f.caps[id]...)
	}
	return out, nil
}

func (f *fakeResolver) ScopeAncestors(_ context.Context, s Scope) ([]Scope, error) {
	return f.ancestors[s], nil
}

func testResolver() *fakeResolver {
	return &fakeResolver{
		assignments: []Assignment{
			{PrincipalType: "user", PrincipalID: "alice", Scope: Scope{Type: "org", ID: "org-1"}, RoleID: "member"},
			{PrincipalType: "user", PrincipalID: "alice", Scope: Scope{Type: "project", ID: "proj-9"}, RoleID: "viewer"},
			{PrincipalType: "user", PrincipalID: "root", Scope: Scope{Type: "platform", ID: ""}, RoleID: "owner"},
		},
		caps: map[string][]RoleCapability{
			"member": {{Capability: "project.read", Effect: EffectAllow}, {Capability: "project.scale", Effect: EffectAllow}},
			"viewer": {{Capability: "project.read", Effect: EffectAllow}, {Capability: "project.scale", Effect: EffectDeny}},
			"owner":  {{Capability: "project.read", Effect: EffectAllow}, {Capability: "project.delete", Effect: EffectAllow}},
		},
		ancestors: map[Scope][]Scope{
			{Type: "project", ID: "proj-9"}: {{Type: "org", ID: "org-1"}, {Type: "platform", ID: ""}},
			{Type: "project", ID: "other"}:  {{Type: "org", ID: "org-2"}, {Type: "platform", ID: ""}},
		},
	}
}

func TestHasCapabilityDirectGrant(t *testing.T) {
	ok, err := HasCapability(context.Background(), testResolver(), "user", "alice", "project.scale", Scope{Type: "org", ID: "org-1"})
	if err != nil || !ok {
		t.Fatalf("direct org grant: ok=%v err=%v", ok, err)
	}
}

func TestHasCapabilityInheritedGrant(t *testing.T) {
	// alice's org-1 member grant inherits down to proj-9 (org-1 child).
	ok, err := HasCapability(context.Background(), testResolver(), "user", "alice", "project.read", Scope{Type: "project", ID: "proj-9"})
	if err != nil || !ok {
		t.Fatalf("inherited grant: ok=%v err=%v", ok, err)
	}
}

func TestDenyOverridesAllow(t *testing.T) {
	// alice is viewer (deny project.scale) directly on proj-9 while inheriting
	// member (allow) from org-1: deny must win.
	ok, err := HasCapability(context.Background(), testResolver(), "user", "alice", "project.scale", Scope{Type: "project", ID: "proj-9"})
	if err != nil || ok {
		t.Fatalf("deny should win: ok=%v err=%v", ok, err)
	}
}

func TestPlatformGrantAppliesEverywhere(t *testing.T) {
	ok, err := HasCapability(context.Background(), testResolver(), "user", "root", "project.delete", Scope{Type: "project", ID: "other"})
	if err != nil || !ok {
		t.Fatalf("platform grant: ok=%v err=%v", ok, err)
	}
}

func TestUnknownPrincipalDenied(t *testing.T) {
	ok, err := HasCapability(context.Background(), testResolver(), "user", "nobody", "project.read", Scope{Type: "org", ID: "org-1"})
	if err != nil || ok {
		t.Fatalf("unknown principal must be denied: ok=%v err=%v", ok, err)
	}
}

func TestOutOfScopeDenied(t *testing.T) {
	// alice's grants live under org-1; org-2 subtree must deny.
	ok, err := HasCapability(context.Background(), testResolver(), "user", "alice", "project.read", Scope{Type: "project", ID: "other"})
	if err != nil || ok {
		t.Fatalf("out-of-scope must be denied: ok=%v err=%v", ok, err)
	}
}

func TestEmptyInputDenied(t *testing.T) {
	if _, err := HasCapability(context.Background(), testResolver(), "", "alice", "project.read", Scope{Type: "org", ID: "org-1"}); err == nil {
		t.Fatal("empty principal type must error")
	}
}
