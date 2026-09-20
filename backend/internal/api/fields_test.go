package api

import (
	"net/http/httptest"
	"testing"
)

func TestSelectFieldsFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?fields=id,name", nil)
	v := map[string]any{"id": "1", "name": "n", "secret": "drop"}
	out, ok := selectFields(req, v).(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", selectFields(req, v))
	}
	if len(out) != 2 || out["id"] != "1" {
		t.Fatalf("unexpected projection: %v", out)
	}
	if _, bad := out["secret"]; bad {
		t.Fatalf("secret must be dropped: %v", out)
	}
}

func TestSelectFieldsEmptyPassthrough(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	v := map[string]any{"a": 1}
	if selectFields(req, v).(map[string]any)["a"] != 1 {
		t.Fatal("empty fields must passthrough")
	}
}
