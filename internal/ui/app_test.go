package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"sshtunnel/internal/config"
)

// newTestApp собирает окно приложения на тестовой реализации Fyne.
func newTestApp(t *testing.T, store *config.Store) *app {
	t.Helper()
	a := &app{fyneApp: test.NewApp()}
	a.win = a.fyneApp.NewWindow("тест")
	a.store = store
	a.build()
	return a
}

// sampleProfile — профиль без секретов для тестов интерфейса.
func sampleProfile(name string) config.Profile {
	return config.Profile{
		Name:          name,
		LocalPort:     8080,
		JumpHost:      "jump.example",
		JumpPort:      2222,
		JumpUser:      "operator",
		AuthMethod:    "password",
		TargetHost:    "target.example",
		TargetPort:    22,
		VerifyHostKey: true,
		KeepAliveSec:  20,
		AutoReconnect: true,
	}
}

// TestApplyProfileDoesNotRecurse проверяет, что заполнение полей из профиля
// не вызывает себя повторно через обработчик списка профилей.
// Select.SetSelected в Fyne вызывает OnChanged всегда, поэтому без защиты
// этот вызов уходил бы в бесконечную рекурсию и ронял приложение.
func TestApplyProfileDoesNotRecurse(t *testing.T) {
	p := sampleProfile("рабочий")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}, Last: p.Name})

	a.applyProfile(p)

	if a.jumpHost.Text != p.JumpHost {
		t.Fatalf("адрес jump-сервера %q, ожидался %q", a.jumpHost.Text, p.JumpHost)
	}
	if a.jumpPort.Text != "2222" {
		t.Fatalf("порт jump-сервера %q, ожидался 2222", a.jumpPort.Text)
	}
	if a.profileSelect.Selected != p.Name {
		t.Fatalf("выбран профиль %q, ожидался %q", a.profileSelect.Selected, p.Name)
	}
	if !a.verifyHostKey.Checked {
		t.Fatal("проверка ключа хоста не включилась из профиля")
	}
	if a.applying {
		t.Fatal("флаг заполнения полей остался поднятым")
	}
}

// TestCollectProfileReadsFields проверяет, что собранный из полей профиль
// повторяет то, что было в него загружено.
func TestCollectProfileReadsFields(t *testing.T) {
	p := sampleProfile("рабочий")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}})
	a.applyProfile(p)

	got, err := a.collectProfile()
	if err != nil {
		t.Fatalf("сбор параметров не удался: %v", err)
	}
	if got.JumpHost != p.JumpHost || got.JumpPort != p.JumpPort || got.TargetPort != p.TargetPort {
		t.Fatalf("собранный профиль %+v не совпадает с исходным %+v", got, p)
	}
	if got.AuthMethod != "password" {
		t.Fatalf("способ аутентификации %q, ожидался password", got.AuthMethod)
	}
	if got.KeepAliveSec != 20 {
		t.Fatalf("keepalive %d, ожидалось 20", got.KeepAliveSec)
	}
}

// TestCollectProfileRejectsBadPort проверяет, что нечисловой и пустой порт
// отвергаются с внятным сообщением, а не превращаются в случайный порт.
// В Electron-версии parseInt от пустого поля давал NaN, и слушатель поднимался
// на произвольном порту.
func TestCollectProfileRejectsBadPort(t *testing.T) {
	p := sampleProfile("рабочий")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}})
	a.applyProfile(p)

	a.localPort.SetText("")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "не заполнен") {
		t.Fatalf("пустой порт не отвергнут: %v", err)
	}

	a.localPort.SetText("не число")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "должен быть числом") {
		t.Fatalf("нечисловой порт не отвергнут: %v", err)
	}

	a.localPort.SetText("70000")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "1..65535") {
		t.Fatalf("порт вне диапазона не отвергнут: %v", err)
	}

	a.localPort.SetText("8080")
	a.keepAlive.SetText("0")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "keepalive") {
		t.Fatalf("нулевой keepalive не отвергнут: %v", err)
	}
}

// TestAuthFormSwitchesWithMethod проверяет, что при смене способа аутентификации
// в окне остаются только относящиеся к нему поля.
func TestAuthFormSwitchesWithMethod(t *testing.T) {
	a := newTestApp(t, &config.Store{})
	a.applyProfile(config.NewProfile())

	a.authSelect.SetSelected("Пароль")
	if len(a.authForm.Items) != 2 || a.authForm.Items[0].Widget != a.password {
		t.Fatalf("для пароля показаны не те поля: %d шт.", len(a.authForm.Items))
	}

	a.authSelect.SetSelected("Приватный ключ")
	if len(a.authForm.Items) != 2 || a.authForm.Items[1].Widget != a.keyPassphrase {
		t.Fatalf("для ключа показаны не те поля: %d шт.", len(a.authForm.Items))
	}

	a.authSelect.SetSelected("ssh-agent")
	if len(a.authForm.Items) != 1 {
		t.Fatalf("для ssh-agent показаны не те поля: %d шт.", len(a.authForm.Items))
	}
}

// TestLogIsTrimmed проверяет, что журнал не растёт без предела.
func TestLogIsTrimmed(t *testing.T) {
	a := newTestApp(t, &config.Store{})
	for i := 0; i < maxLogLines+50; i++ {
		a.appendLog("строка")
	}
	if len(a.logLines) != maxLogLines {
		t.Fatalf("в журнале %d строк, ожидалось %d", len(a.logLines), maxLogLines)
	}
}
