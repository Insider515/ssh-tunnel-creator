//go:build !windows

package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"

	"sshtunnel/internal/i18n"
)

// dialAgent connects to ssh-agent through the Unix socket in SSH_AUTH_SOCK.
func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New(i18n.T("SSH-agent was not found: SSH_AUTH_SOCK is not set"))
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Could not open a connection to ssh-agent at %s: %w"), sock, err)
	}
	return conn, nil
}
