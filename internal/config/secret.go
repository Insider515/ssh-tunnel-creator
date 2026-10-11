package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService — имя службы, под которым приложение хранит пароли
// в системном хранилище секретов: Keychain в macOS, Credential Manager в Windows,
// Secret Service в Linux.
const keyringService = "sshtunnel"

// secretKey строит идентификатор записи в хранилище секретов из параметров подключения.
func secretKey(p Profile) string {
	return fmt.Sprintf("%s@%s:%d", p.JumpUser, p.JumpHost, p.JumpPort)
}

// SavePassword кладёт пароль в системное хранилище секретов ОС.
func SavePassword(p Profile, password string) error {
	if err := keyring.Set(keyringService, secretKey(p), password); err != nil {
		return fmt.Errorf("не удалось сохранить пароль в системном хранилище: %w", err)
	}
	return nil
}

// LoadPassword достаёт пароль из системного хранилища секретов ОС.
// Отсутствие записи не считается ошибкой: возвращается пустая строка.
func LoadPassword(p Profile) (string, error) {
	password, err := keyring.Get(keyringService, secretKey(p))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать пароль из системного хранилища: %w", err)
	}
	return password, nil
}

// DeletePassword убирает пароль из системного хранилища секретов ОС.
// Отсутствие записи не считается ошибкой.
func DeletePassword(p Profile) error {
	err := keyring.Delete(keyringService, secretKey(p))
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("не удалось удалить пароль из системного хранилища: %w", err)
}
