//go:build windows

package startup

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
	"sshtunnel/internal/i18n"
)

const windowsRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Set enables or disables the current user's Windows startup entry.
func Set(enabled bool, executable string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf(i18n.T("Could not open the current user's startup settings: %w"), err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue("SSHTunnel"); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf(i18n.T("Could not remove the startup entry: %w"), err)
		}
		return nil
	}
	if strings.TrimSpace(executable) == "" {
		return errors.New(i18n.T("Application executable path is empty"))
	}
	if err := key.SetStringValue("SSHTunnel", `"`+executable+`"`); err != nil {
		return fmt.Errorf(i18n.T("Could not register the application for startup: %w"), err)
	}
	return nil
}
