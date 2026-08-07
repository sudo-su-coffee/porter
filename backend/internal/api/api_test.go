package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRoutesRegistersPaths ensures Routes() wires every handler without
// panicking. A nil store is safe here: handlers never run, only registration.
func TestRoutesRegistersPaths(t *testing.T) {
	a := NewAPI(nil, nil, nil, nil, nil, "tok", "example.com", "admin", "pw", "v0.1.0")
	mux := http.NewServeMux()
	a.Routes(mux)

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/overview", http.StatusUnauthorized}, // auth gate runs first
		{http.MethodGet, "/traffic", http.StatusUnauthorized},
		{http.MethodGet, "/projects/abc/replicas", http.StatusUnauthorized},
		{http.MethodGet, "/definitely-not-a-route", http.StatusNotFound},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("%s %s: expected %d, got %d", tc.method, tc.path, tc.want, rr.Code)
		}
	}
}
