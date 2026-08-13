package runtime

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestFCClientUsesUnixSocketAndOfficialPayload(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "firecracker.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seen := make(chan map[string]any, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		seen <- body
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	client := newFCClient(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.SetMachineConfig(ctx, 2, 512); err != nil {
		t.Fatal(err)
	}
	select {
	case body := <-seen:
		if body["vcpu_count"] != float64(2) || body["mem_size_mib"] != float64(512) {
			t.Fatalf("unexpected Firecracker payload: %#v", body)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for Unix-socket request")
	}
}

func TestWaitForSocketHonorsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	if err := waitForSocket(ctx, filepath.Join(t.TempDir(), "missing.sock")); err == nil {
		t.Fatal("waitForSocket returned nil for a missing socket")
	}
}
