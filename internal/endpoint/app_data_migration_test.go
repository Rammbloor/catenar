package endpoint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigratedAppDataBaseDirMovesLegacyData(t *testing.T) {
	configDir := t.TempDir()
	legacyDir := filepath.Join(configDir, "tether")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("create legacy app data directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "history.db"), []byte("history"), 0o600); err != nil {
		t.Fatalf("write legacy app data: %v", err)
	}

	currentDir := migratedAppDataBaseDir(configDir)
	if currentDir != filepath.Join(configDir, "catenar") {
		t.Fatalf("expected catenar app data directory, got %q", currentDir)
	}
	if _, err := os.Stat(filepath.Join(currentDir, "history.db")); err != nil {
		t.Fatalf("expected legacy app data to be retained after migration: %v", err)
	}
}

func TestMigratedAppDataBaseDirPrefersCurrentData(t *testing.T) {
	configDir := t.TempDir()
	currentDir := filepath.Join(configDir, "catenar")
	legacyDir := filepath.Join(configDir, "tether")
	for _, directory := range []string{currentDir, legacyDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create app data directory: %v", err)
		}
	}

	if got := migratedAppDataBaseDir(configDir); got != currentDir {
		t.Fatalf("expected current app data directory %q, got %q", currentDir, got)
	}
}
