//go:build darwin

package startup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetCreatesAndRemovesLaunchAgent verifies LaunchAgent registration and removal.
func TestSetCreatesAndRemovesLaunchAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	executable := filepath.Join(home, "Applications", "SSH Tunnel.app", "Contents", "MacOS", "sshtunnel")

	if err := Set(true, executable); err != nil {
		t.Fatalf("could not enable LaunchAgent: %v", err)
	}
	agentPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentName)
	raw, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("could not read LaunchAgent: %v", err)
	}
	if !strings.Contains(string(raw), "RunAtLoad") || !strings.Contains(string(raw), "SSH Tunnel.app") {
		t.Fatalf("LaunchAgent does not contain the expected startup command: %s", raw)
	}
	if err := Set(false, executable); err != nil {
		t.Fatalf("could not disable LaunchAgent: %v", err)
	}
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Fatalf("LaunchAgent still exists or stat failed: %v", err)
	}
}
