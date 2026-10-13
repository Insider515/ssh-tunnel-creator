//go:build linux

package startup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sshtunnel/internal/i18n"
)

const desktopEntryName = "sshtunnel.desktop"

// Set enables or disables the current user's XDG autostart desktop entry.
func Set(enabled bool, executable string) error {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf(i18n.T("Could not determine the home directory: %w"), err)
		}
		configHome = filepath.Join(home, ".config")
	}
	entryPath := filepath.Join(configHome, "autostart", desktopEntryName)
	if !enabled {
		if err := os.Remove(entryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(i18n.T("Could not remove the autostart entry: %w"), err)
		}
		return nil
	}
	if strings.TrimSpace(executable) == "" {
		return errors.New(i18n.T("Application executable path is empty"))
	}
	if !filepath.IsAbs(executable) {
		return errors.New(i18n.T("Application executable path must be absolute"))
	}

	content := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=SSH Tunnel",
		"Exec=" + desktopQuote(executable),
		"X-GNOME-Autostart-enabled=true",
		"",
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		return fmt.Errorf(i18n.T("Could not create the autostart directory: %w"), err)
	}
	if err := os.WriteFile(entryPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf(i18n.T("Could not write the autostart entry: %w"), err)
	}
	return nil
}

// desktopQuote escapes an executable path for a desktop-entry Exec argument.
func desktopQuote(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
		"$", "\\$",
		"%", "%%",
	)
	return `"` + replacer.Replace(value) + `"`
}
