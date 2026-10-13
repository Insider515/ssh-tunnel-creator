package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"sshtunnel/internal/i18n"
)

// HostKeyPrompt asks whether to trust a host key that has not been seen before.
type HostKeyPrompt func(host, keyType, fingerprint string) bool

// newHostKeyCallback builds host-key verification for the jump server.
// It reads known keys from ~/.ssh/known_hosts and the application's own file.
// A mismatch with a saved key is rejected; a new key requires operator confirmation.
// Confirmed keys are appended to the application file, never the user's known_hosts.
func newHostKeyCallback(verify bool, appFile string, prompt HostKeyPrompt, logf func(string, ...any)) (ssh.HostKeyCallback, error) {
	if !verify {
		logf(i18n.T("Host key checking is disabled; the connection is vulnerable to server impersonation"))
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if err := ensureFile(appFile); err != nil {
		return nil, err
	}
	files := []string{appFile}
	if home, err := os.UserHomeDir(); err == nil {
		userFile := filepath.Join(home, ".ssh", "known_hosts")
		if _, err := os.Stat(userFile); err == nil {
			files = append(files, userFile)
		}
	}

	base, err := knownhosts.New(files...)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Could not read the known-hosts list: %w"), err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}

		fp := ssh.FingerprintSHA256(key)

		// Want is non-empty when a different key is already saved for this host.
		if len(keyErr.Want) > 0 {
			return fmt.Errorf(
				i18n.T("Host key for %s differs from the previously saved key (received %s %s). The server may have been reinstalled or the connection may be under attack. The connection will not be established until this mismatch is explained."),
				hostname, key.Type(), fp)
		}

		// Want is empty when this host has not been seen before.
		if prompt == nil || !prompt(hostname, key.Type(), fp) {
			return fmt.Errorf(i18n.T("Host key %s (%s %s) was not trusted"), hostname, key.Type(), fp)
		}
		if err := appendKnownHost(appFile, hostname, key); err != nil {
			logf(i18n.T("Could not remember host key %s: %v"), hostname, err)
		} else {
			logf(i18n.T("Host key %s (%s %s) was saved to %s"), hostname, key.Type(), fp, appFile)
		}
		return nil
	}, nil
}

// ensureFile creates the file and its parent directory when needed.
func ensureFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf(i18n.T("Could not create directory %s: %w"), filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf(i18n.T("Could not create file %s: %w"), path, err)
	}
	return f.Close()
}

// appendKnownHost appends a confirmed host key to the application file in known_hosts format.
func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	_, err = f.WriteString(line + "\n")
	return err
}
