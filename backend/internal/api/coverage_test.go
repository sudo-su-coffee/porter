package api

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryRegisteredRouteIsMapped tests that every route registered in
// Routes() is present in the routePerms table, so no authenticated route runs
// without a specific RBAC permission guarding it. It parses api.go statically.
func TestEveryRegisteredRouteIsMapped(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	text := string(src)

	// Registered patterns: mux.HandleFunc("METHOD /path", ...).
	handleRe := regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"`)
	registered := map[string]bool{}
	for _, m := range handleRe.FindAllStringSubmatch(text, -1) {
		registered[m[1]] = true
	}

	// Mapped patterns: inside the routePerms map literal, keys are lines like
	// "\t\"GET /path\": \"perm\"," — capture the full "METHOD /path" key.
	lineRe := regexp.MustCompile(`^\s*"((?:GET|POST|PUT|PATCH|DELETE|HEAD) [^"]+)"\s*:\s*"`)
	mnorm := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if m := lineRe.FindStringSubmatch(line); m != nil {
			mnorm[m[1]] = true
		}
	}

	var missing []string
	for pat := range registered {
		if isUnGuarded(pat) {
			continue
		}
		if !mnorm[pat] {
			missing = append(missing, pat)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("routes registered but missing from routePerms (%d):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Reverse check: every routePerms key must be an actually-registered route —
	// otherwise the permission entry is dead. Handler existence is enforced by
	// the compiler (a mux line referencing a missing handler won't build).
	var dead []string
	for pat := range mnorm {
		if !registered[pat] {
			dead = append(dead, pat)
		}
	}
	sort.Strings(dead)
	if len(dead) > 0 {
		t.Fatalf("routePerms entries with no matching registered route (%d):\n  %s",
			len(dead), strings.Join(dead, "\n  "))
	}
}

// isUnGuarded lists patterns that intentionally need no permission guard.

// isUnGuarded lists patterns that intentionally need no permission guard.
func isUnGuarded(p string) bool {
	prefixes := []string{
		"GET /csrf", "GET /health", "POST /auth/", "GET /auth/session",
		"POST /login", "POST /logout",
		"GET /users/me", "PATCH /users/me", "DELETE /users/me",
		"POST /images/custom", "GET /images/ml",
	}
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}