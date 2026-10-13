package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
	"sshtunnel/internal/i18n"
)

// keyringService is the service name used to store passwords in the OS credential store.
const keyringService = "sshtunnel"

// secretKey builds a credential identifier from connection parameters.
func secretKey(p Profile) string {
	return fmt.Sprintf("%s@%s:%d", p.JumpUser, p.JumpHost, p.JumpPort)
}

// SavePassword stores a password in the operating system credential store.
func SavePassword(p Profile, password string) error {
	if err := keyring.Set(keyringService, secretKey(p), password); err != nil {
		return fmt.Errorf(i18n.T("Could not save a password to the OS credential store: %w"), err)
	}
	return nil
}

// LoadPassword retrieves a password from the operating system credential store.
// A missing entry is not an error and returns an empty string.
func LoadPassword(p Profile) (string, error) {
	password, err := keyring.Get(keyringService, secretKey(p))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf(i18n.T("Could not read a password from the OS credential store: %w"), err)
	}
	return password, nil
}

// DeletePassword removes a password from the operating system credential store.
// A missing entry is not an error.
func DeletePassword(p Profile) error {
	err := keyring.Delete(keyringService, secretKey(p))
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf(i18n.T("Could not delete a password from the OS credential store: %w"), err)
}
