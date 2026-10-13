//go:build darwin

package startup

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sshtunnel/internal/i18n"
)

const launchAgentName = "com.geekproxy.sshtunnel.plist"

// Set enables or disables the current user's macOS LaunchAgent.
func Set(enabled bool, executable string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf(i18n.T("Could not determine the home directory: %w"), err)
	}
	agentPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentName)
	if !enabled {
		if err := os.Remove(agentPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(i18n.T("Could not remove the LaunchAgent: %w"), err)
		}
		return nil
	}

	if executable == "" {
		return errors.New(i18n.T("Application executable path is empty"))
	}
	var plist strings.Builder
	plist.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	plist.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	plist.WriteString(`<plist version="1.0"><dict>`)
	plist.WriteString(`<key>Label</key><string>com.geekproxy.sshtunnel</string>`)
	plist.WriteString(`<key>ProgramArguments</key><array><string>`)
	escaped, err := xmlEscape(executable)
	if err != nil {
		return fmt.Errorf(i18n.T("Could not encode the executable path: %w"), err)
	}
	plist.WriteString(escaped)
	plist.WriteString(`</string></array><key>RunAtLoad</key><true/>`)
	plist.WriteString(`</dict></plist>` + "\n")

	if err := os.MkdirAll(filepath.Dir(agentPath), 0o700); err != nil {
		return fmt.Errorf(i18n.T("Could not create the LaunchAgents directory: %w"), err)
	}
	if err := os.WriteFile(agentPath, []byte(plist.String()), 0o644); err != nil {
		return fmt.Errorf(i18n.T("Could not write the LaunchAgent: %w"), err)
	}
	return nil
}

// xmlEscape escapes a value for safe inclusion in plist XML text.
func xmlEscape(value string) (string, error) {
	var out strings.Builder
	if err := xml.EscapeText(&out, []byte(value)); err != nil {
		return "", err
	}
	return out.String(), nil
}
