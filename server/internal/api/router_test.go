package api

import (
	"os"
	"path/filepath"
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
