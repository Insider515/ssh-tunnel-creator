package ui

import (
	"os"
	"path/filepath"
)

// sshDir returns ~/.ssh when it exists, for use as the private-key file dialog's initial directory.
func sshDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".ssh")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
