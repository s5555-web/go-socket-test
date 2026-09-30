package api

import (
	"os"
	"path/filepath"
	"sket/internal/api/socket"
	"sket/internal/auth"
	"sket/internal/config"
	"testing"
)

func TestFirstExistingDir(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	existing := filepath.Join(root, "admin")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if got := firstExistingDir(missing, existing); got != existing {
		t.Fatalf("firstExistingDir() = %q, want %q", got, existing)
	}
}

func TestFirstExistingDirFallsBackToFirstPath(t *testing.T) {
	if got := firstExistingDir("primary", "secondary"); got != "primary" {
		t.Fatalf("firstExistingDir() = %q, want primary", got)
	}
}

func TestClientEngineRegistersSupportRoutes(t *testing.T) {
	app := &API{
		auth:      auth.New("test-secret"),
		hub:       socket.NewHub(),
		clientDir: filepath.Join("..", "..", "..", "web", "client"),
	}
	engine := app.ClientEngine(&config.Config{})
	found := map[string]bool{}
	for _, route := range engine.Routes() {
		found[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{"GET /api/widget/config", "POST /api/widget/sessions", "GET /api/support/threads", "GET /api/conversation-clears/pending", "POST /api/conversation-clears/:event/ack"} {
		if !found[route] {
			t.Fatalf("support route %q was not registered", route)
		}
	}
}
