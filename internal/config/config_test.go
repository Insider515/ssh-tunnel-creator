package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// isolateHome redirects home-directory lookups to a temporary directory.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir+"/.config")
	t.Setenv("APPDATA", dir+"/AppData")
	return dir
}

// TestLoadOnEmptyDir verifies that a missing profile file is not an error.
func TestLoadOnEmptyDir(t *testing.T) {
	isolateHome(t)

	store, err := Load()
	if err != nil {
		t.Fatalf("loading an empty directory returned an error: %v", err)
	}
	if len(store.Profiles) != 0 {
		t.Fatalf("profile count is %d, want 0", len(store.Profiles))
	}
}

// TestSaveLoadRoundTrip verifies that a saved profile is loaded without changes.
func TestSaveLoadRoundTrip(t *testing.T) {
	isolateHome(t)

	p := NewProfile()
	p.Name = "production"
	p.JumpHost = "jump.example"
	p.JumpPort = 2222
	p.JumpUser = "operator"
	p.TargetHost = "target.example"
	p.LocalPort = 3307

	store := &Store{}
	store.Upsert(p)
	store.Last = p.Name
	if err := store.Save(); err != nil {
		t.Fatalf("could not save profiles: %v", err)
	}

	again, err := Load()
	if err != nil {
		t.Fatalf("could not load profiles: %v", err)
	}
	got, ok := again.Get("production")
	if !ok {
		t.Fatal("profile was not found after loading")
	}
	if got != p {
		t.Fatalf("loaded profile %+v, want %+v", got, p)
	}
	if again.Last != "production" {
		t.Fatalf("last profile is %q, want production", again.Last)
	}
}

// TestSaveStoresNoSecrets verifies that profile files do not contain passwords or passphrases.
func TestSaveStoresNoSecrets(t *testing.T) {
	isolateHome(t)

	p := NewProfile()
	p.Name = "production"
	p.JumpHost = "jump.example"
	p.JumpUser = "operator"
	p.RememberPassword = true

	store := &Store{}
	store.Upsert(p)
	if err := store.Save(); err != nil {
		t.Fatalf("could not save profiles: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("could not get profile file path: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read profile file: %v", err)
	}
	// Check field names: "password" as auth_method is a method identifier, not a secret.
	var parsed struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("could not parse profile file: %v", err)
	}
	if len(parsed.Profiles) != 1 {
		t.Fatalf("profile file contains %d profiles, want 1", len(parsed.Profiles))
	}
	for key := range parsed.Profiles[0] {
		lower := strings.ToLower(key)
		if key == "remember_password" {
			continue
		}
		if strings.Contains(lower, "pass") || strings.Contains(lower, "secret") {
			t.Fatalf("profile contains a secret field %q\n%s", key, raw)
		}
	}
	if _, ok := parsed.Profiles[0]["remember_password"]; !ok {
		t.Fatal("password-storage preference was not saved")
	}
}

// TestRemoveDeletesProfile verifies profile removal.
func TestRemoveDeletesProfile(t *testing.T) {
	store := &Store{}
	store.Upsert(Profile{Name: "one"})
	store.Upsert(Profile{Name: "two"})

	store.Remove("one")
	if _, ok := store.Get("one"); ok {
		t.Fatal("profile remains after removal")
	}
	if _, ok := store.Get("two"); !ok {
		t.Fatal("the wrong profile was removed")
	}
	if len(store.Names()) != 1 {
		t.Fatalf("profile name count is %d, want 1", len(store.Names()))
	}
}

// TestUpsertReplacesByName verifies that saving an existing name replaces its profile.
func TestUpsertReplacesByName(t *testing.T) {
	store := &Store{}
	store.Upsert(Profile{Name: "one", LocalPort: 1000})
	store.Upsert(Profile{Name: "one", LocalPort: 2000})

	if len(store.Profiles) != 1 {
		t.Fatalf("profile count is %d, want 1", len(store.Profiles))
	}
	p, _ := store.Get("one")
	if p.LocalPort != 2000 {
		t.Fatalf("port is %d, want 2000", p.LocalPort)
	}
}

// TestKnownHostsPathInsideAppDir verifies that known hosts are stored in the application directory.
func TestKnownHostsPathInsideAppDir(t *testing.T) {
	isolateHome(t)

	path, err := KnownHostsPath()
	if err != nil {
		t.Fatalf("could not get known-hosts path: %v", err)
	}
	if !strings.Contains(path, appDir) {
		t.Fatalf("path %q is not inside the application directory", path)
	}
	if strings.Contains(path, ".ssh") {
		t.Fatalf("path %q points to the user's .ssh directory", path)
	}
}

// TestSettingsRoundTripPreservesPreferences verifies that application settings persist.
func TestSettingsRoundTripPreservesPreferences(t *testing.T) {
	isolateHome(t)

	want := Settings{
		Language:       "uk",
		LaunchAtLogin:  true,
		AutoConnect:    true,
		DefaultProfile: "work",
	}
	if err := want.Save(); err != nil {
		t.Fatalf("could not save settings: %v", err)
	}
	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("could not load settings: %v", err)
	}
	if got != want {
		t.Fatalf("loaded settings %+v, want %+v", got, want)
	}
}

// TestLoadSettingsDefaultsToEnglish verifies first-launch language and disabled startup options.
func TestLoadSettingsDefaultsToEnglish(t *testing.T) {
	isolateHome(t)

	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("could not load default settings: %v", err)
	}
	if got != DefaultSettings() {
		t.Fatalf("default settings %+v, want %+v", got, DefaultSettings())
	}
}
