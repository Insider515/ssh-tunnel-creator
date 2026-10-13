//go:build windows

package tunnel

import (
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
	"sshtunnel/internal/i18n"
)

// openSSHAgentPipe is the named pipe used by the built-in Windows OpenSSH agent.
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent connects to ssh-agent on Windows, trying SSH_AUTH_SOCK before the built-in agent pipe.
func dialAgent() (net.Conn, error) {
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := winio.DialPipe(sock, nil); err == nil {
			return conn, nil
		}
		if conn, err := net.Dial("unix", sock); err == nil {
			return conn, nil
		}
	}
	conn, err := winio.DialPipe(openSSHAgentPipe, nil)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Could not connect to ssh-agent (%s): %w; start the ssh-agent service"), openSSHAgentPipe, err)
	}
	return conn, nil
}
