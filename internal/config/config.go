// Package config хранит профили подключений в каталоге конфигурации пользователя.
// Пароли и passphrase в файл профилей не пишутся никогда: в Electron-версии пароль
// jump-сервера был вписан прямо в index.html и попадал в собранный .app.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// appDir — имя каталога приложения внутри каталога конфигурации пользователя.
const appDir = "sshtunnel"

// Profile — сохранённый набор параметров одного подключения без секретов.
type Profile struct {
	Name          string `json:"name"`
	LocalPort     int    `json:"local_port"`
	JumpHost      string `json:"jump_host"`
	JumpPort      int    `json:"jump_port"`
	JumpUser      string `json:"jump_user"`
	AuthMethod    string `json:"auth_method"`
	KeyPath       string `json:"key_path,omitempty"`
	TargetHost    string `json:"target_host"`
	TargetPort    int    `json:"target_port"`
	VerifyHostKey bool   `json:"verify_host_key"`
	KeepAliveSec  int    `json:"keepalive_sec"`
	AutoReconnect bool   `json:"auto_reconnect"`
	// RememberPassword — хранить ли пароль в системном хранилище секретов ОС.
	// Сам пароль в этом файле не лежит, здесь только признак.
	RememberPassword bool `json:"remember_password"`
}

// NewProfile возвращает профиль со значениями по умолчанию.
// Никаких реальных адресов и паролей по умолчанию нет.
func NewProfile() Profile {
	return Profile{
		Name:          "Новый профиль",
		LocalPort:     8080,
		JumpPort:      22,
		TargetPort:    22,
		AuthMethod:    "password",
		VerifyHostKey: true,
		KeepAliveSec:  15,
		AutoReconnect: true,
	}
}

// Store — содержимое файла профилей.
type Store struct {
	Profiles []Profile `json:"profiles"`
	Last     string    `json:"last_profile"`
}

// Dir возвращает каталог приложения в каталоге конфигурации пользователя
// и создаёт его, если он отсутствует.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить каталог конфигурации пользователя: %w", err)
	}
	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("не удалось создать каталог %s: %w", dir, err)
	}
	return dir, nil
}

// Path возвращает полный путь к файлу профилей.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

// KnownHostsPath возвращает путь к собственному файлу известных хостов приложения.
// Отдельный файл нужен, чтобы приложение не правило ~/.ssh/known_hosts пользователя.
func KnownHostsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// Load читает файл профилей. Отсутствующий файл — не ошибка, возвращается пустой список.
func Load() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать %s: %w", path, err)
	}
	var s Store
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("файл %s повреждён: %w", path, err)
		}
	}
	return &s, nil
}

// Save записывает файл профилей. Запись идёт через временный файл с переименованием,
// чтобы сбой на половине записи не оставил повреждённый конфиг.
func (s *Store) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("не удалось записать %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("не удалось заменить %s: %w", path, err)
	}
	return nil
}

// Names возвращает имена профилей в алфавитном порядке.
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for _, p := range s.Profiles {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// Get находит профиль по имени.
func (s *Store) Get(name string) (Profile, bool) {
	for _, p := range s.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Upsert добавляет профиль или заменяет существующий с тем же именем.
func (s *Store) Upsert(p Profile) {
	for i := range s.Profiles {
		if s.Profiles[i].Name == p.Name {
			s.Profiles[i] = p
			return
		}
	}
	s.Profiles = append(s.Profiles, p)
}

// Remove удаляет профиль по имени.
func (s *Store) Remove(name string) {
	out := s.Profiles[:0]
	for _, p := range s.Profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	s.Profiles = out
}
