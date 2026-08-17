package guestagent

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestClientStatusRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go func() {
		var req Request
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			return
		}
		_ = json.NewEncoder(server).Encode(Response{Protocol: ProtocolVersion, ID: req.ID, OK: true, Data: map[string]any{"boot_id": "boot-1", "state": "ready", "ready": true, "healthy": true, "updated_at": time.Now()}})
	}()
	status, err := (Client{Dial: func(context.Context) (net.Conn, error) { return client, nil }}).Status(context.Background())
	if err != nil || status.BootID != "boot-1" || !status.Ready || !status.Healthy {
		t.Fatalf("unexpected status: %#v, %v", status, err)
	}
}

func TestClientPropagatesAgentError(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go func() {
		var req Request
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			return
		}
		_ = json.NewEncoder(server).Encode(Response{Protocol: ProtocolVersion, ID: req.ID, Error: "not ready"})
	}()
	if err := (Client{Dial: func(context.Context) (net.Conn, error) { return client, nil }}).Health(context.Background()); err == nil {
		t.Fatal("expected guest-agent error")
	}
}
