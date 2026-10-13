package tunnel

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"sshtunnel/internal/i18n"
)

// AuthMethod identifies an authentication method for the jump server.
type AuthMethod string

const (
	// AuthPassword uses password and keyboard-interactive authentication.
	AuthPassword AuthMethod = "password"
	// AuthKey uses a private-key file for authentication.
	AuthKey AuthMethod = "key"
	// AuthAgent uses keys from a running ssh-agent.
	AuthAgent AuthMethod = "agent"
)

// AuthConfig contains the operator-provided authentication parameters.
// Passwords and passphrases stay in process memory and are never written to the configuration file.
type AuthConfig struct {
	Method        AuthMethod
	Password      string
	KeyPath       string
	KeyPassphrase string
}

// buildAuth creates SSH authentication methods and returns any resource to close after connecting.
// It checks that the selected method is usable before starting the SSH connection.
func buildAuth(cfg AuthConfig) ([]ssh.AuthMethod, io.Closer, error) {
	switch cfg.Method {
	case AuthPassword:
		if cfg.Password == "" {
			return nil, nil, errors.New(i18n.T("Password is not set"))
		}
		// Some servers disable "password" and accept only "keyboard-interactive".
		// Offer both methods and answer each interactive prompt with the same password.
		kbd := ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = cfg.Password
			}
			return answers, nil
		})
		return []ssh.AuthMethod{ssh.Password(cfg.Password), kbd}, nil, nil

	case AuthKey:
		signer, err := loadKey(cfg.KeyPath, cfg.KeyPassphrase)
		if err != nil {
			return nil, nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil, nil

	case AuthAgent:
		conn, err := dialAgent()
		if err != nil {
			return nil, nil, err
		}
		client := agent.NewClient(conn)
		keys, err := client.List()
		if err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf(i18n.T("Agent is not responding: %w"), err)
		}
		if len(keys) == 0 {
			_ = conn.Close()
			return nil, nil, errors.New(i18n.T("No key found in ssh-agent; add a key with ssh-add"))
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(client.Signers)}, conn, nil

	default:
		return nil, nil, fmt.Errorf(i18n.T("Unknown authentication method: %q"), cfg.Method)
	}
}

// loadKey reads a private-key file and returns its signer.
func loadKey(path, passphrase string) (ssh.Signer, error) {
	if path == "" {
		return nil, errors.New(i18n.T("Key file path is not set"))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Key file %s could not be read: %w"), path, err)
	}
	if passphrase == "" {
		signer, err := ssh.ParsePrivateKey(raw)
		if err == nil {
			return signer, nil
		}
		var need *ssh.PassphraseMissingError
		if errors.As(err, &need) {
			return nil, errors.New(i18n.T("Key file is encrypted; a passphrase is required"))
		}
		return nil, fmt.Errorf(i18n.T("Key file %s could not be parsed: %w"), path, err)
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(raw, []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Key file %s could not be decrypted: %w"), path, err)
	}
	return signer, nil
}
