//go:build !windows

package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"
)

// dialAgent открывает соединение с ssh-agent через unix-сокет из SSH_AUTH_SOCK.
// Так агент адресуется на macOS и Linux.
func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("ssh-agent не найден: переменная SSH_AUTH_SOCK не задана")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к ssh-agent по адресу %s: %w", sock, err)
	}
	return conn, nil
}
