package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// isolateHome подменяет домашний каталог на временный, чтобы тесты
// не трогали настоящие настройки пользователя.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir+"/.config")
	t.Setenv("APPDATA", dir+"/AppData")
	return dir
}

// TestLoadOnEmptyDir проверяет, что отсутствующий файл профилей не считается ошибкой.
func TestLoadOnEmptyDir(t *testing.T) {
	isolateHome(t)

	store, err := Load()
	if err != nil {
		t.Fatalf("чтение пустого каталога вернуло ошибку: %v", err)
	}
	if len(store.Profiles) != 0 {
		t.Fatalf("профилей %d, ожидалось 0", len(store.Profiles))
	}
}

// TestSaveLoadRoundTrip проверяет, что сохранённый профиль читается обратно без изменений.
func TestSaveLoadRoundTrip(t *testing.T) {
	isolateHome(t)

	p := NewProfile()
	p.Name = "боевой"
	p.JumpHost = "jump.example"
	p.JumpPort = 2222
	p.JumpUser = "operator"
	p.TargetHost = "target.example"
	p.LocalPort = 3307

	store := &Store{}
	store.Upsert(p)
	store.Last = p.Name
	if err := store.Save(); err != nil {
		t.Fatalf("сохранение не удалось: %v", err)
	}

	again, err := Load()
	if err != nil {
		t.Fatalf("чтение не удалось: %v", err)
	}
	got, ok := again.Get("боевой")
	if !ok {
		t.Fatal("профиль не найден после чтения")
	}
	if got != p {
		t.Fatalf("прочитан профиль %+v, сохранялся %+v", got, p)
	}
	if again.Last != "боевой" {
		t.Fatalf("последний профиль %q, ожидался \"боевой\"", again.Last)
	}
}

// TestSaveStoresNoSecrets проверяет, что в файле профилей нет ни пароля, ни passphrase.
// В Electron-версии пароль jump-сервера лежал прямо в index.html и попадал в сборку.
func TestSaveStoresNoSecrets(t *testing.T) {
	isolateHome(t)

	p := NewProfile()
	p.Name = "боевой"
	p.JumpHost = "jump.example"
	p.JumpUser = "operator"
	p.RememberPassword = true

	store := &Store{}
	store.Upsert(p)
	if err := store.Save(); err != nil {
		t.Fatalf("сохранение не удалось: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("путь к файлу не получен: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("файл профилей не прочитан: %v", err)
	}
	// Проверяются именно имена полей: значение "password" у auth_method —
	// это название способа аутентификации, а не секрет.
	var parsed struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("файл профилей не разобран: %v", err)
	}
	if len(parsed.Profiles) != 1 {
		t.Fatalf("профилей в файле %d, ожидался 1", len(parsed.Profiles))
	}
	for key := range parsed.Profiles[0] {
		lower := strings.ToLower(key)
		if key == "remember_password" {
			continue
		}
		if strings.Contains(lower, "pass") || strings.Contains(lower, "secret") {
			t.Fatalf("в профиле есть поле с секретом: %q\n%s", key, raw)
		}
	}
	if _, ok := parsed.Profiles[0]["remember_password"]; !ok {
		t.Fatal("признак хранения пароля не записан")
	}
}

// TestRemoveDeletesProfile проверяет удаление профиля из списка.
func TestRemoveDeletesProfile(t *testing.T) {
	store := &Store{}
	store.Upsert(Profile{Name: "один"})
	store.Upsert(Profile{Name: "два"})

	store.Remove("один")
	if _, ok := store.Get("один"); ok {
		t.Fatal("профиль остался после удаления")
	}
	if _, ok := store.Get("два"); !ok {
		t.Fatal("удалён не тот профиль")
	}
	if len(store.Names()) != 1 {
		t.Fatalf("имён %d, ожидалось 1", len(store.Names()))
	}
}

// TestUpsertReplacesByName проверяет, что повторное сохранение под тем же именем
// заменяет профиль, а не добавляет второй с таким же именем.
func TestUpsertReplacesByName(t *testing.T) {
	store := &Store{}
	store.Upsert(Profile{Name: "один", LocalPort: 1000})
	store.Upsert(Profile{Name: "один", LocalPort: 2000})

	if len(store.Profiles) != 1 {
		t.Fatalf("профилей %d, ожидался 1", len(store.Profiles))
	}
	p, _ := store.Get("один")
	if p.LocalPort != 2000 {
		t.Fatalf("порт %d, ожидался 2000", p.LocalPort)
	}
}

// TestKnownHostsPathInsideAppDir проверяет, что файл известных хостов лежит
// в каталоге приложения, а не в ~/.ssh пользователя.
func TestKnownHostsPathInsideAppDir(t *testing.T) {
	isolateHome(t)

	path, err := KnownHostsPath()
	if err != nil {
		t.Fatalf("путь не получен: %v", err)
	}
	if !strings.Contains(path, appDir) {
		t.Fatalf("путь %q не внутри каталога приложения", path)
	}
	if strings.Contains(path, ".ssh") {
		t.Fatalf("путь %q указывает в каталог .ssh пользователя", path)
	}
}
