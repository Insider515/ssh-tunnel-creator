//go:build linux

package startup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetCreatesAndRemovesAutostartEntry verifies XDG autostart registration and removal.
func TestSetCreatesAndRemovesAutostartEntry(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	executable := filepath.Join(configHome, "SSH Tunnel", "sshtunnel")

	if err := Set(true, executable); err != nil {
		t.Fatalf("could not enable autostart: %v", err)
	}
	entryPath := filepath.Join(configHome, "autostart", desktopEntryName)
	content, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("could not read autostart entry: %v", err)
	}
	if !strings.Contains(string(content), `Exec="`+executable+`"`) {
		t.Fatalf("autostart entry does not contain the expected executable path: %s", content)
	}
	if err := Set(false, executable); err != nil {
		t.Fatalf("could not disable autostart: %v", err)
	}
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Fatalf("autostart entry still exists or stat failed: %v", err)
	}
}

// TestSetRejectsRelativeExecutable verifies that autostart requires an absolute executable path.
func TestSetRejectsRelativeExecutable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Set(true, "sshtunnel"); err == nil {
		t.Fatal("relative executable path was accepted")
	}
}
