package store

import (
	"context"
	"testing"
)

func TestZZDepDebug(t *testing.T) {
	if testDSN() == "" {
		t.Skip("no db")
	}
	s := NewStore(testDSN())
	defer s.Close()
	// Raw query identical to ListDeployments.
	rows, err := s.pool.Query(context.Background(), `
		SELECT id FROM deployments WHERE project_id = $1 ORDER BY revision DESC`,
		"7770c0a9-3eb1-4efd-bc6f-5fa947266f65")
	if err != nil {
		t.Fatalf("raw: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	t.Logf("raw rows=%d err=%v", n, rows.Err())
	got := s.ListDeployments("7770c0a9-3eb1-4efd-bc6f-5fa947266f65")
	if len(got) != 1 {
		t.Fatalf("list=%d, want 1", len(got))
	}
	t.Logf("row ok id=%s digest=%s", got[0].ID, got[0].ImageDigest)
}
