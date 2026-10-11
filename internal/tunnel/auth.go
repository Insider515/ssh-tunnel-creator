package tunnel

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// AuthMethod — способ аутентификации на jump-сервере.
type AuthMethod string

const (
	// AuthPassword — аутентификация паролем (password и keyboard-interactive).
	AuthPassword AuthMethod = "password"
	// AuthKey — аутентификация файлом приватного ключа.
	AuthKey AuthMethod = "key"
	// AuthAgent — аутентификация ключами из запущенного ssh-agent.
	AuthAgent AuthMethod = "agent"
)

// AuthConfig — параметры аутентификации, заданные оператором.
// Пароль и passphrase живут только в памяти процесса и никогда не пишутся в файл конфигурации.
type AuthConfig struct {
	Method        AuthMethod
	Password      string
	KeyPath       string
	KeyPassphrase string
}

// ErrKeyPassphraseRequired возвращается, когда файл ключа зашифрован,
// а passphrase не задан. UI по этой ошибке просит оператора ввести passphrase.
var ErrKeyPassphraseRequired = errors.New("файл ключа зашифрован: требуется passphrase")

// buildAuth собирает список ssh.AuthMethod для выбранного способа аутентификации.
// Проверяет выполнимость способа до начала соединения (есть ли файл ключа, запущен ли агент),
// чтобы ошибка была конкретной, а не общим "ssh: handshake failed".
func buildAuth(cfg AuthConfig) ([]ssh.AuthMethod, error) {
	switch cfg.Method {
	case AuthPassword:
		if cfg.Password == "" {
			return nil, errors.New("пароль не задан")
		}
		// Часть серверов отключает метод "password" и принимает только
		// "keyboard-interactive". Отдаём оба, на каждый вопрос отвечая тем же паролем.
		kbd := ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = cfg.Password
			}
			return answers, nil
		})
		return []ssh.AuthMethod{ssh.Password(cfg.Password), kbd}, nil

	case AuthKey:
		signer, err := loadKey(cfg.KeyPath, cfg.KeyPassphrase)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil

	case AuthAgent:
		conn, err := dialAgent()
		if err != nil {
			return nil, err
		}
		client := agent.NewClient(conn)
		keys, err := client.List()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("ssh-agent не отвечает: %w", err)
		}
		if len(keys) == 0 {
			conn.Close()
			return nil, errors.New("в ssh-agent нет ни одного ключа (добавьте ключ командой ssh-add)")
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(client.Signers)}, nil

	default:
		return nil, fmt.Errorf("неизвестный способ аутентификации: %q", cfg.Method)
	}
}

// loadKey читает файл приватного ключа и возвращает подписывающего.
// Зашифрованный ключ без passphrase отдаёт ErrKeyPassphraseRequired,
// чтобы UI мог отличить этот случай от повреждённого файла.
func loadKey(path, passphrase string) (ssh.Signer, error) {
	if path == "" {
		return nil, errors.New("путь к файлу приватного ключа не задан")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл ключа %s: %w", path, err)
	}
	if passphrase == "" {
		signer, err := ssh.ParsePrivateKey(raw)
		if err == nil {
			return signer, nil
		}
		var need *ssh.PassphraseMissingError
		if errors.As(err, &need) {
			return nil, ErrKeyPassphraseRequired
		}
		return nil, fmt.Errorf("не удалось разобрать файл ключа %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(raw, []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("не удалось расшифровать файл ключа %s: %w", path, err)
	}
	return signer, nil
}
