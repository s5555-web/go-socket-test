package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfig = `
server:
  client_addr: "127.0.0.1:1802"
  admin_addr: "127.0.0.1:1801"
database:
  dsn: ""
auth:
  secret: ""
socket:
  read_buffer_size: 1024
  write_buffer_size: 1024
  pong_wait_sec: 60
  ping_period_sec: 54
`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(testConfig), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRequiresServerSecrets(t *testing.T) {
	t.Setenv("SIGNAL_DATABASE_DSN", "")
	t.Setenv("SIGNAL_AUTH_SECRET", "")
	_, err := Load(writeTestConfig(t))
	if err == nil || !strings.Contains(err.Error(), "SIGNAL_DATABASE_DSN") {
		t.Fatalf("expected missing DSN error, got %v", err)
	}
}

func TestLoadUsesEnvironmentSecrets(t *testing.T) {
	t.Setenv("SIGNAL_DATABASE_DSN", "app:secret@tcp(127.0.0.1:3306)/signal_web")
	t.Setenv("SIGNAL_AUTH_SECRET", "01234567890123456789012345678901")
	cfg, err := Load(writeTestConfig(t))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Database.DSN != "app:secret@tcp(127.0.0.1:3306)/signal_web" {
		t.Fatalf("environment DSN was not applied: %q", cfg.Database.DSN)
	}
	if cfg.Auth.Secret != "01234567890123456789012345678901" {
		t.Fatal("environment auth secret was not applied")
	}
}
