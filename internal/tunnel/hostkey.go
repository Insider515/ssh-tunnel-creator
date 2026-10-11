package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// HostKeyPrompt спрашивает оператора, доверять ли ранее не встречавшемуся ключу хоста.
// Возвращает true, если оператор подтвердил ключ.
type HostKeyPrompt func(host, keyType, fingerprint string) bool

// newHostKeyCallback строит проверку ключа jump-сервера.
// Известные ключи читаются из ~/.ssh/known_hosts и из собственного файла приложения.
// Несовпадение с сохранённым ключом — жёсткий отказ (признак подмены сервера).
// Ранее не встречавшийся ключ выносится на подтверждение оператору и после согласия
// дописывается в файл приложения; файл known_hosts пользователя приложение не меняет.
func newHostKeyCallback(verify bool, appFile string, prompt HostKeyPrompt, logf func(string, ...any)) (ssh.HostKeyCallback, error) {
	if !verify {
		logf("ВНИМАНИЕ: проверка ключа хоста отключена, соединение уязвимо к подмене сервера")
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
		return nil, fmt.Errorf("не удалось прочитать список известных хостов: %w", err)
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

		// Want не пуст — для этого хоста уже сохранён другой ключ.
		if len(keyErr.Want) > 0 {
			return fmt.Errorf(
				"ключ хоста %s не совпадает с сохранённым ранее (получен %s %s). "+
					"Это либо переустановка сервера, либо подмена соединения. "+
					"Пока расхождение не объяснено, соединение не устанавливается",
				hostname, key.Type(), fp)
		}

		// Want пуст — хост встречается впервые.
		if prompt == nil || !prompt(hostname, key.Type(), fp) {
			return fmt.Errorf("ключ хоста %s (%s %s) не подтверждён оператором", hostname, key.Type(), fp)
		}
		if err := appendKnownHost(appFile, hostname, key); err != nil {
			logf("не удалось запомнить ключ хоста %s: %v", hostname, err)
		} else {
			logf("ключ хоста %s (%s %s) сохранён в %s", hostname, key.Type(), fp, appFile)
		}
		return nil
	}, nil
}

// ensureFile создаёт файл и его каталог, если их нет.
// Нужно потому, что knownhosts.New возвращает ошибку на отсутствующий файл.
func ensureFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("не удалось создать каталог %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("не удалось создать файл %s: %w", path, err)
	}
	return f.Close()
}

// appendKnownHost дописывает подтверждённый ключ хоста в файл приложения
// в формате known_hosts.
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
