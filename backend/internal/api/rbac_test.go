package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCurrentPrincipalDefaultsToAdmin verifies unauthenticated context resolves
// to the bootstrap admin (the defensive default).
func TestCurrentPrincipalDefaultsToAdmin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/projects/x", nil)
	p := currentPrincipal(req)
	if !p.isAdmin || p.role != "admin" {
		t.Fatalf("expected admin default, got %+v", p)
	}
}

// TestPermForRoute verifies the central route→permission table maps known
// patterns to specific <resource>.<action> codes.
func TestPermForRoute(t *testing.T) {
	tests := []struct{ pattern, want string }{
		{"POST /projects/{projectId}/replicas/{n}/exec", "replica.exec"},
		{"POST /projects/{projectId}/replicas/{n}/ssh-cert", "ssh.connect"},
		{"GET /projects/{projectId}/replicas/{n}/ssh-info", "ssh.connect"},
		{"POST /projects/{projectId}/deployments/{deployId}/promote", "deployment.promote"},
		{"POST /projects/{projectId}/deployments/{deployId}/rollback", "deployment.rollback"},
		{"DELETE /projects/{projectId}/members/{username}", "member.remove"},
		{"POST /projects/{projectId}/members/invite", "member.invite"},
		{"DELETE /projects/{projectId}", "project.delete"},
		{"POST /projects/{projectId}/transfer", "project.transfer"},
		{"POST /projects/{projectId}/crons/{cronId}/run", "cron.run"},
		{"DELETE /volumes/{volumeId}", "volume.delete"},
		{"POST /projects/{projectId}/cache/purge", "cache.purge"},
		{"GET /projects/{projectId}/analytics/bandwidth", "analytics.read"},
		{"POST /orgs/transfer", "org.transfer"},
		{"DELETE /orgs/members/{username}", "org.member.remove"},
		{"POST /users", "user.create"},
		{"DELETE /servers/{id}", "server.remove"},
	}
	for _, c := range tests {
		req := httptest.NewRequest(http.MethodGet, "/irrelevant", nil)
		req.Pattern = c.pattern
		if got := permForRoute(req); got != c.want {
			t.Errorf("permForRoute(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

// TestPermForRouteUnknownPattern confirms unlisted routes return "" (auth-only).
func TestPermForRouteUnknownPattern(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/irrelevant", nil)
	req.Pattern = "GET /some/unlisted/thing"
	if got := permForRoute(req); got != "" {
		t.Fatalf("expected empty permission for unlisted route, got %q", got)
	}
}