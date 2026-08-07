package config

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigEnvCarriesRequiredSecrets(t *testing.T) {
	t.Setenv("PORTER_API_TOKEN", "tok")
	t.Setenv("PORTER_ADMIN_PASSWORD", "pw")
	t.Setenv("PORTER_DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("PORTER_GATEWAY_ENABLED", "true")
	t.Setenv("PORTER_GATEWAY_LISTEN_ADDR", ":8081")

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "porter.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen addr %q", cfg.ListenAddr)
	}
	if !cfg.GatewayEnabled {
		t.Fatal("gateway should be enabled via env")
	}
	if cfg.GatewayListenAddr != ":8081" {
		t.Fatalf("unexpected gateway addr %q", cfg.GatewayListenAddr)
	}
}

func TestLoadConfigMissingAPIToken(t *testing.T) {
	t.Setenv("PORTER_API_TOKEN", "")
	t.Setenv("PORTER_ADMIN_PASSWORD", "pw")
	t.Setenv("PORTER_DATABASE_URL", "postgres://u:p@localhost:5432/db")
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "porter.toml")); err == nil {
		t.Fatal("expected error when api_token is missing")
	}
}

func TestParseTOMLBasic(t *testing.T) {
	text := "[server]\nlisten_addr = \":9999\"\nbase_domain = \"example.com\"\n[admin]\npassword = \"secret\"\n[gateway]\nenabled = true\n"
	sections, err := ParseTOML(text)
	if err != nil {
		t.Fatal(err)
	}
	if sections["server"]["listen_addr"] != ":9999" {
		t.Fatalf("unexpected listen_addr %q", sections["server"]["listen_addr"])
	}
	if sections["admin"]["password"] != "secret" {
		t.Fatal("password not parsed")
	}
	if sections["gateway"]["enabled"] != "true" {
		t.Fatal("gateway enabled not parsed")
	}
}

func TestParseTOMLRejectsBadLine(t *testing.T) {
	if _, err := ParseTOML("[server]\njust-a-key\n"); err == nil {
		t.Fatal("expected parse error for bare line")
	}
}
