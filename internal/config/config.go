// Package config stores connection profiles in the user's configuration directory.
// Passwords and passphrases are never written to the profile file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sshtunnel/internal/i18n"
)

// appDir is the application directory within the user's configuration directory.
const appDir = "sshtunnel"

// Profile stores one connection configuration without secrets.
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
	// RememberPassword indicates whether the OS credential store should retain the password.
	RememberPassword bool `json:"remember_password"`
}

// NewProfile returns a profile with safe default values and no credentials.
func NewProfile() Profile {
	return Profile{
		Name:          "New profile",
		LocalPort:     8080,
		JumpPort:      22,
		TargetPort:    22,
		AuthMethod:    "password",
		VerifyHostKey: true,
		KeepAliveSec:  15,
		AutoReconnect: true,
	}
}

// Store contains saved connection profiles.
type Store struct {
	Profiles []Profile `json:"profiles"`
	Last     string    `json:"last_profile"`
}

// Settings stores application-wide preferences independently from connection profiles.
type Settings struct {
	Language       string `json:"language"`
	LaunchAtLogin  bool   `json:"launch_at_login"`
	AutoConnect    bool   `json:"auto_connect"`
	DefaultProfile string `json:"default_profile"`
}

// DefaultSettings returns the preferences used on first launch.
func DefaultSettings() Settings {
	return Settings{Language: "en"}
}

// Dir returns the application directory and creates it when it does not exist.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf(i18n.T("Could not determine the user configuration directory: %w"), err)
	}
	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf(i18n.T("Could not create directory %s: %w"), dir, err)
	}
	return dir, nil
}

// Path returns the full path to the profile file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

// SettingsPath returns the full path to the application settings file.
func SettingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// KnownHostsPath returns the path to the application's known-hosts file.
func KnownHostsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// Load reads the profile file. A missing file returns an empty store.
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
		return nil, fmt.Errorf(i18n.T("Could not read %s: %w"), path, err)
	}
	var s Store
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf(i18n.T("File %s is corrupted: %w"), path, err)
		}
	}
	return &s, nil
}

// LoadSettings reads application preferences or returns defaults when no file exists.
func LoadSettings() (Settings, error) {
	path, err := SettingsPath()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf(i18n.T("Could not read %s: %w"), path, err)
	}

	settings := DefaultSettings()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return Settings{}, fmt.Errorf(i18n.T("File %s is corrupted: %w"), path, err)
		}
	}
	if settings.Language != "en" && settings.Language != "uk" {
		settings.Language = "en"
	}
	return settings, nil
}

// Save writes application preferences atomically.
func (s Settings) Save() error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf(i18n.T("Could not write %s: %w"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf(i18n.T("Could not replace %s: %w"), path, err)
	}
	return nil
}

// Save writes the profile store through a temporary file and rename.
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
		return fmt.Errorf(i18n.T("Could not write %s: %w"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf(i18n.T("Could not replace %s: %w"), path, err)
	}
	return nil
}

// Names returns profile names in alphabetical order.
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for _, p := range s.Profiles {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// Get finds a profile by name.
func (s *Store) Get(name string) (Profile, bool) {
	for _, p := range s.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Upsert adds a profile or replaces the profile with the same name.
func (s *Store) Upsert(p Profile) {
	for i := range s.Profiles {
		if s.Profiles[i].Name == p.Name {
			s.Profiles[i] = p
			return
		}
	}
	s.Profiles = append(s.Profiles, p)
}

// Remove deletes a profile by name.
func (s *Store) Remove(name string) {
	out := s.Profiles[:0]
	for _, p := range s.Profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	s.Profiles = out
}
