//go:build windows

package tunnel

import (
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
)

// openSSHAgentPipe — именованный канал, который слушает штатный агент OpenSSH в Windows.
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent открывает соединение с ssh-agent в Windows.
// Сначала пробует адрес из SSH_AUTH_SOCK (его задают сторонние агенты),
// затем именованный канал штатного агента OpenSSH.
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
		return nil, fmt.Errorf("не удалось подключиться к ssh-agent (%s): %w; запустите службу ssh-agent", openSSHAgentPipe, err)
	}
	return conn, nil
}
