package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthSetsRequestID verifies every request through auth (even rejected
// ones) carries an X-Request-ID for log/audit correlation (task T10b).
func TestAuthSetsRequestID(t *testing.T) {
	a := NewAPI(nil, nil, nil, nil, nil, "secret-material", "example.com", "v1.0.0-beta-dev")
	next := a.auth(func(w http.ResponseWriter, r *http.Request) {})

	// Generated when absent (401 path still carries it).
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rr := httptest.NewRecorder()
	next(rr, req)
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("expected generated X-Request-ID header")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rr.Code)
	}

	// Echoed when supplied.
	req2 := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req2.Header.Set("X-Request-ID", "test-123")
	rr2 := httptest.NewRecorder()
	next(rr2, req2)
	if got := rr2.Header().Get("X-Request-ID"); got != "test-123" {
		t.Errorf("expected echoed request ID, got %q", got)
	}
}

// TestRequestIDContextRoundTrip verifies withRequestID/currentRequestID agree.
func TestRequestIDContextRoundTrip(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	req2 := withRequestID(rr, req)
	if currentRequestID(req2) == "" {
		t.Error("expected request ID in context")
	}
	if currentRequestID(req) != "" {
		t.Error("original request must stay untouched")
	}
}

// TestPaginateSlicesAndHeaders verifies limit/cursor paging (task T10b).
func TestPaginateSlicesAndHeaders(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	req := httptest.NewRequest(http.MethodGet, "/x?limit=2", nil)
	rr := httptest.NewRecorder()
	page := paginate(rr, req, items)
	if len(page) != 2 || page[0] != 1 {
		t.Fatalf("first page wrong: %v", page)
	}
	if rr.Header().Get("X-Total-Count") != "5" || rr.Header().Get("X-Next-Cursor") != "2" {
		t.Fatalf("paging headers wrong: %v", rr.Header())
	}
	req2 := httptest.NewRequest(http.MethodGet, "/x?limit=2&cursor=4", nil)
	rr2 := httptest.NewRecorder()
	last := paginate(rr2, req2, items)
	if len(last) != 1 || rr2.Header().Get("X-Next-Cursor") != "" {
		t.Fatalf("last page wrong: %v headers %v", last, rr2.Header())
	}
}

// TestJWKSUnavailableWithoutKey verifies the JWKS endpoint 503s when JWT is
// off instead of leaking an empty key set.
func TestJWKSUnavailableWithoutKey(t *testing.T) {
	a := NewAPI(nil, nil, nil, nil, nil, "secret-material", "example.com", "v1.0.0-beta-dev")
	req := httptest.NewRequest(http.MethodGet, "/auth/jwks", nil)
	rr := httptest.NewRecorder()
	a.handleJWKS(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without JWT key, got %d", rr.Code)
	}
}
