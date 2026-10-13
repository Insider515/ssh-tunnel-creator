//go:build !darwin && !windows && !linux

package startup

import (
	"errors"

	"sshtunnel/internal/i18n"
)

// Set reports that automatic startup is unsupported on this operating system.
func Set(bool, string) error {
	return errors.New(i18n.T("Automatic startup is not supported on this operating system"))
}
