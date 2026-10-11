// Package ui содержит графический интерфейс на Fyne: ввод параметров, журнал работы
// и управление туннелем. Все сетевые операции выполняются в отдельных горутинах,
// а обновления интерфейса переносятся в его поток через fyne.Do.
package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"sshtunnel/internal/config"
	"sshtunnel/internal/tunnel"
)

// maxLogLines — сколько последних строк журнала держать в окне.
const maxLogLines = 1000

// authLabels связывает подпись в интерфейсе с кодом способа аутентификации.
var authLabels = map[string]tunnel.AuthMethod{
	"Пароль":         tunnel.AuthPassword,
	"Приватный ключ": tunnel.AuthKey,
	"ssh-agent":      tunnel.AuthAgent,
}

// authOrder задаёт порядок способов аутентификации в списке.
var authOrder = []string{"Пароль", "Приватный ключ", "ssh-agent"}

// app — состояние окна приложения.
type app struct {
	fyneApp fyne.App
	win     fyne.Window

	store *config.Store

	profileSelect *widget.Select
	localPort     *widget.Entry
	jumpHost      *widget.Entry
	jumpPort      *widget.Entry
	jumpUser      *widget.Entry
	authSelect    *widget.Select
	password      *widget.Entry
	rememberPass  *widget.Check
	keyPath       *widget.Entry
	keyPassphrase *widget.Entry
	authForm      *widget.Form
	targetHost    *widget.Entry
	targetPort    *widget.Entry
	verifyHostKey *widget.Check
	keepAlive     *widget.Entry
	autoReconnect *widget.Check

	checkBtn *widget.Button
	startBtn *widget.Button
	stopBtn  *widget.Button

	statusLabel *widget.Label
	statsLabel  *widget.Label
	logList     *widget.List
	logLines    []string

	tun    *tunnel.Tunnel
	cancel context.CancelFunc

	// applying поднят, пока поля окна заполняются из профиля.
	// Нужен потому, что Select.SetSelected в Fyne вызывает OnChanged всегда,
	// в том числе при установке уже выбранного значения, а обработчик
	// списка профилей сам вызывает applyProfile — без флага вышла бы бесконечная рекурсия.
	applying bool
}

// Run собирает окно и запускает приложение. Возвращает управление после закрытия окна.
func Run() {
	a := &app{fyneApp: fyneapp.NewWithID("com.geekproxy.sshtunnel")}
	a.win = a.fyneApp.NewWindow("SSH Tunnel")
	a.win.Resize(fyne.NewSize(760, 880))

	store, err := config.Load()
	if err != nil {
		store = &config.Store{}
		defer func() { dialog.ShowError(err, a.win) }()
	}
	a.store = store

	a.build()
	a.applyProfile(a.initialProfile())

	a.win.SetCloseIntercept(func() {
		a.stopTunnel()
		a.win.Close()
	})
	a.win.ShowAndRun()
}

// initialProfile выбирает профиль, который показывается при запуске:
// последний использованный, иначе первый из списка, иначе профиль по умолчанию.
func (a *app) initialProfile() config.Profile {
	if p, ok := a.store.Get(a.store.Last); ok {
		return p
	}
	if len(a.store.Profiles) > 0 {
		return a.store.Profiles[0]
	}
	return config.NewProfile()
}

// build создаёт все элементы окна и задаёт его содержимое.
func (a *app) build() {
	a.buildProfileRow()
	a.buildFields()
	a.buildButtons()
	a.buildLog()

	form := container.NewVBox(
		widget.NewCard("Профиль", "", container.NewVBox(a.profileRow())),
		widget.NewCard("1. Локальный порт", "адрес, к которому подключаются ваши программы",
			widget.NewForm(widget.NewFormItem("Порт", a.localPort)),
		),
		widget.NewCard("2. Jump-сервер (промежуточный)", "через него открывается соединение",
			container.NewVBox(
				widget.NewForm(
					widget.NewFormItem("Адрес", a.jumpHost),
					widget.NewFormItem("Порт", a.jumpPort),
					widget.NewFormItem("Пользователь", a.jumpUser),
					widget.NewFormItem("Аутентификация", a.authSelect),
				),
				a.authForm,
			),
		),
		widget.NewCard("3. Целевой сервер (конечный)", "куда jump-сервер открывает соединение",
			widget.NewForm(
				widget.NewFormItem("Адрес", a.targetHost),
				widget.NewFormItem("Порт", a.targetPort),
			),
		),
		widget.NewCard("4. Параметры соединения", "",
			container.NewVBox(
				a.verifyHostKey,
				a.autoReconnect,
				widget.NewForm(widget.NewFormItem("Keepalive, сек", a.keepAlive)),
			),
		),
		container.NewGridWithColumns(3, a.checkBtn, a.startBtn, a.stopBtn),
		a.statusLabel,
		a.statsLabel,
	)

	logPane := container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Журнал", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewButtonWithIcon("Очистить", theme.DeleteIcon(), a.clearLog),
			widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), a.copyLog),
		),
		nil, nil, nil,
		a.logList,
	)

	split := container.NewVSplit(container.NewVScroll(form), logPane)
	split.SetOffset(0.68)
	a.win.SetContent(split)
}

// buildProfileRow создаёт список профилей и кнопки работы с ним.
func (a *app) buildProfileRow() {
	a.profileSelect = widget.NewSelect(a.store.Names(), func(name string) {
		if a.applying {
			return
		}
		p, ok := a.store.Get(name)
		if !ok {
			return
		}
		a.applyProfile(p)
	})
	a.profileSelect.PlaceHolder = "профиль не выбран"
}

// profileRow возвращает строку с выбором профиля и кнопками сохранения, удаления и сброса.
func (a *app) profileRow() fyne.CanvasObject {
	save := widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), a.saveProfile)
	remove := widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), a.removeProfile)
	fresh := widget.NewButtonWithIcon("Новый", theme.ContentAddIcon(), func() {
		a.applyProfile(config.NewProfile())
		a.profileSelect.ClearSelected()
	})
	return container.NewBorder(nil, nil, nil, container.NewHBox(save, remove, fresh), a.profileSelect)
}

// buildFields создаёт поля ввода параметров подключения.
func (a *app) buildFields() {
	a.localPort = widget.NewEntry()
	a.jumpHost = widget.NewEntry()
	a.jumpHost.SetPlaceHolder("адрес или имя хоста")
	a.jumpPort = widget.NewEntry()
	a.jumpUser = widget.NewEntry()

	a.password = widget.NewPasswordEntry()
	a.rememberPass = widget.NewCheck("Запомнить пароль в системном хранилище ОС", nil)
	a.keyPath = widget.NewEntry()
	a.keyPath.SetPlaceHolder("путь к файлу приватного ключа")
	a.keyPassphrase = widget.NewPasswordEntry()
	a.keyPassphrase.SetPlaceHolder("если ключ зашифрован")

	a.authForm = widget.NewForm()
	a.authSelect = widget.NewSelect(authOrder, func(string) { a.rebuildAuthForm() })

	a.targetHost = widget.NewEntry()
	a.targetHost.SetPlaceHolder("адрес или имя хоста")
	a.targetPort = widget.NewEntry()

	a.verifyHostKey = widget.NewCheck("Проверять ключ хоста jump-сервера (защита от подмены сервера)", nil)
	a.autoReconnect = widget.NewCheck("Переподключаться автоматически при обрыве связи", nil)
	a.keepAlive = widget.NewEntry()
}

// rebuildAuthForm показывает поля, относящиеся только к выбранному способу аутентификации.
func (a *app) rebuildAuthForm() {
	browse := widget.NewButtonWithIcon("Выбрать файл", theme.FolderOpenIcon(), a.pickKeyFile)

	switch authLabels[a.authSelect.Selected] {
	case tunnel.AuthPassword:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem("Пароль", a.password),
			widget.NewFormItem("", a.rememberPass),
		}
	case tunnel.AuthKey:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem("Файл ключа", container.NewBorder(nil, nil, nil, browse, a.keyPath)),
			widget.NewFormItem("Passphrase", a.keyPassphrase),
		}
	case tunnel.AuthAgent:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem("", widget.NewLabel("Используются ключи из запущенного ssh-agent")),
		}
	default:
		a.authForm.Items = nil
	}
	a.authForm.Refresh()
}

// pickKeyFile открывает диалог выбора файла приватного ключа.
func (a *app) pickKeyFile() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		a.keyPath.SetText(r.URI().Path())
	}, a.win)
	if dir := sshDir(); dir != "" {
		if uri, err := storage.ParseURI("file://" + dir); err == nil {
			if list, err := storage.ListerForURI(uri); err == nil {
				d.SetLocation(list)
			}
		}
	}
	d.Show()
}

// buildButtons создаёт кнопки проверки, запуска и остановки.
func (a *app) buildButtons() {
	a.checkBtn = widget.NewButtonWithIcon("Проверить соединение", theme.SearchIcon(), a.checkTunnel)
	a.startBtn = widget.NewButtonWithIcon("Запустить", theme.MediaPlayIcon(), a.startTunnel)
	a.startBtn.Importance = widget.HighImportance
	a.stopBtn = widget.NewButtonWithIcon("Остановить", theme.MediaStopIcon(), a.stopTunnel)
	a.stopBtn.Disable()

	a.statusLabel = widget.NewLabel("Состояние: неактивен")
	a.statusLabel.Wrapping = fyne.TextWrapWord
	a.statsLabel = widget.NewLabel("Соединений: активных 0, всего 0")
}

// buildLog создаёт панель журнала.
// Журнал построен на widget.List, а не на текстовом поле: список рисует только
// видимые строки, поэтому добавление записи не зависит от длины всей истории.
func (a *app) buildLog() {
	a.logList = widget.NewList(
		func() int { return len(a.logLines) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.TextStyle = fyne.TextStyle{Monospace: true}
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(a.logLines) {
				o.(*widget.Label).SetText(a.logLines[i])
			}
		},
	)
}

// applyProfile заполняет поля окна значениями из профиля.
// Пароль подставляется из системного хранилища, если профиль помечен как запоминающий пароль.
func (a *app) applyProfile(p config.Profile) {
	a.applying = true
	defer func() { a.applying = false }()

	a.localPort.SetText(strconv.Itoa(p.LocalPort))
	a.jumpHost.SetText(p.JumpHost)
	a.jumpPort.SetText(strconv.Itoa(p.JumpPort))
	a.jumpUser.SetText(p.JumpUser)
	a.keyPath.SetText(p.KeyPath)
	a.targetHost.SetText(p.TargetHost)
	a.targetPort.SetText(strconv.Itoa(p.TargetPort))
	a.verifyHostKey.SetChecked(p.VerifyHostKey)
	a.autoReconnect.SetChecked(p.AutoReconnect)
	if p.KeepAliveSec <= 0 {
		p.KeepAliveSec = 15
	}
	a.keepAlive.SetText(strconv.Itoa(p.KeepAliveSec))
	a.rememberPass.SetChecked(p.RememberPassword)

	label := "Пароль"
	for name, code := range authLabels {
		if string(code) == p.AuthMethod {
			label = name
		}
	}
	a.authSelect.SetSelected(label)
	a.rebuildAuthForm()

	a.password.SetText("")
	if p.RememberPassword && p.JumpHost != "" && p.JumpUser != "" {
		if pass, err := config.LoadPassword(p); err != nil {
			a.logf("пароль из системного хранилища не прочитан: %v", err)
		} else if pass != "" {
			a.password.SetText(pass)
			a.logf("пароль профиля %q взят из системного хранилища ОС", p.Name)
		}
	}
	if p.Name != "" && p.Name != "Новый профиль" {
		a.profileSelect.SetSelected(p.Name)
	}
}

// collectProfile собирает профиль из полей окна и проверяет числовые поля.
func (a *app) collectProfile() (config.Profile, error) {
	localPort, err := parsePort(a.localPort.Text, "локальный порт")
	if err != nil {
		return config.Profile{}, err
	}
	jumpPort, err := parsePort(a.jumpPort.Text, "порт jump-сервера")
	if err != nil {
		return config.Profile{}, err
	}
	targetPort, err := parsePort(a.targetPort.Text, "порт целевого сервера")
	if err != nil {
		return config.Profile{}, err
	}
	keepAlive, err := strconv.Atoi(strings.TrimSpace(a.keepAlive.Text))
	if err != nil || keepAlive < 1 {
		return config.Profile{}, fmt.Errorf("keepalive должен быть целым числом секунд больше нуля, указано %q", a.keepAlive.Text)
	}

	name := a.profileSelect.Selected
	if name == "" {
		name = "Новый профиль"
	}
	return config.Profile{
		Name:             name,
		LocalPort:        localPort,
		JumpHost:         strings.TrimSpace(a.jumpHost.Text),
		JumpPort:         jumpPort,
		JumpUser:         strings.TrimSpace(a.jumpUser.Text),
		AuthMethod:       string(authLabels[a.authSelect.Selected]),
		KeyPath:          strings.TrimSpace(a.keyPath.Text),
		TargetHost:       strings.TrimSpace(a.targetHost.Text),
		TargetPort:       targetPort,
		VerifyHostKey:    a.verifyHostKey.Checked,
		KeepAliveSec:     keepAlive,
		AutoReconnect:    a.autoReconnect.Checked,
		RememberPassword: a.rememberPass.Checked,
	}, nil
}

// parsePort превращает текст поля в номер порта и объясняет, что именно не так.
func parsePort(text, name string) (int, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, fmt.Errorf("%s не заполнен", name)
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s должен быть числом, указано %q", name, text)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s должен быть в диапазоне 1..65535, указано %d", name, port)
	}
	return port, nil
}

// buildTunnelConfig собирает параметры туннеля из профиля и полей с секретами.
func (a *app) buildTunnelConfig(p config.Profile) (tunnel.Config, error) {
	knownHosts, err := config.KnownHostsPath()
	if err != nil {
		return tunnel.Config{}, err
	}
	return tunnel.Config{
		LocalPort:  p.LocalPort,
		JumpHost:   p.JumpHost,
		JumpPort:   p.JumpPort,
		JumpUser:   p.JumpUser,
		TargetHost: p.TargetHost,
		TargetPort: p.TargetPort,
		Auth: tunnel.AuthConfig{
			Method:        tunnel.AuthMethod(p.AuthMethod),
			Password:      a.password.Text,
			KeyPath:       p.KeyPath,
			KeyPassphrase: a.keyPassphrase.Text,
		},
		VerifyHostKey:  p.VerifyHostKey,
		KnownHostsFile: knownHosts,
		HostKeyPrompt:  a.hostKeyPrompt,
		ConnectTimeout: 15 * time.Second,
		KeepAlive:      time.Duration(p.KeepAliveSec) * time.Second,
		AutoReconnect:  p.AutoReconnect,
	}, nil
}

// newTunnel создаёт туннель с привязкой обновлений к потоку интерфейса.
func (a *app) newTunnel(cfg tunnel.Config) *tunnel.Tunnel {
	return tunnel.New(cfg, tunnel.Callbacks{
		OnStatus: func(s tunnel.Status, msg string) {
			fyne.Do(func() { a.setStatus(s, msg) })
		},
		OnLog: func(line string) {
			fyne.Do(func() { a.appendLog(line) })
		},
		OnStats: func(st tunnel.Stats) {
			fyne.Do(func() {
				a.statsLabel.SetText(fmt.Sprintf("Соединений: активных %d, всего %d", st.Active, st.Total))
			})
		},
	})
}

// checkTunnel выполняет проверку соединения, ничего не запуская.
func (a *app) checkTunnel() {
	p, err := a.collectProfile()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	cfg, err := a.buildTunnelConfig(p)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}

	a.checkBtn.Disable()
	a.startBtn.Disable()
	tun := a.newTunnel(cfg)
	go func() {
		err := tun.Check(context.Background())
		fyne.Do(func() {
			a.checkBtn.Enable()
			if !a.isRunning() {
				a.startBtn.Enable()
			}
			if err != nil {
				dialog.ShowError(err, a.win)
			}
		})
	}()
}

// startTunnel запускает туннель: сначала полная проверка соединения,
// и только при её успехе занимается локальный порт.
func (a *app) startTunnel() {
	p, err := a.collectProfile()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	cfg, err := a.buildTunnelConfig(p)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}

	if p.RememberPassword && tunnel.AuthMethod(p.AuthMethod) == tunnel.AuthPassword && a.password.Text != "" {
		if err := config.SavePassword(p, a.password.Text); err != nil {
			a.appendLog(err.Error())
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	// Туннель держится и в локальной переменной: горутина работает с ней,
	// а не с полем структуры, которое поток интерфейса может перезаписать.
	tun := a.newTunnel(cfg)
	a.tun = tun

	a.checkBtn.Disable()
	a.startBtn.Disable()
	go func() {
		err := tun.Start(ctx)
		fyne.Do(func() {
			if err != nil {
				cancel()
				a.tun = nil
				a.checkBtn.Enable()
				a.startBtn.Enable()
				dialog.ShowError(err, a.win)
				return
			}
			a.stopBtn.Enable()
		})
	}()
}

// stopTunnel останавливает туннель и возвращает кнопки в исходное состояние.
func (a *app) stopTunnel() {
	if a.tun != nil {
		a.tun.Stop()
		a.tun = nil
	}
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.stopBtn.Disable()
	a.checkBtn.Enable()
	a.startBtn.Enable()
}

// isRunning сообщает, работает ли туннель сейчас.
func (a *app) isRunning() bool {
	return a.tun != nil && a.tun.Running()
}

// setStatus показывает состояние туннеля и приводит кнопки в соответствие с ним.
func (a *app) setStatus(s tunnel.Status, msg string) {
	a.statusLabel.SetText(fmt.Sprintf("Состояние: %s — %s", s, msg))
	switch s {
	case tunnel.StatusActive, tunnel.StatusReconnecting:
		a.stopBtn.Enable()
		a.startBtn.Disable()
		a.checkBtn.Disable()
	case tunnel.StatusIdle, tunnel.StatusError:
		if !a.isRunning() {
			a.stopBtn.Disable()
			a.startBtn.Enable()
			a.checkBtn.Enable()
		}
	}
}

// hostKeyPrompt спрашивает оператора про незнакомый ключ хоста и ждёт его решения.
// Вызывается из горутины туннеля, поэтому блокирующее ожидание ответа безопасно.
func (a *app) hostKeyPrompt(host, keyType, fingerprint string) bool {
	answer := make(chan bool, 1)
	fyne.Do(func() {
		text := fmt.Sprintf(
			"Сервер %s предъявил ключ, который раньше не встречался.\n\nТип ключа: %s\nОтпечаток: %s\n\nДоверять этому ключу и запомнить его?",
			host, keyType, fingerprint)
		d := dialog.NewConfirm("Неизвестный ключ хоста", text, func(ok bool) { answer <- ok }, a.win)
		d.SetConfirmText("Доверять")
		d.SetDismissText("Отменить")
		d.Show()
	})
	return <-answer
}

// saveProfile записывает текущие поля как профиль под введённым именем.
func (a *app) saveProfile() {
	p, err := a.collectProfile()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	name := widget.NewEntry()
	name.SetText(p.Name)
	form := dialog.NewForm("Сохранить профиль", "Сохранить", "Отмена",
		[]*widget.FormItem{widget.NewFormItem("Имя профиля", name)},
		func(ok bool) {
			if !ok || strings.TrimSpace(name.Text) == "" {
				return
			}
			p.Name = strings.TrimSpace(name.Text)
			a.store.Upsert(p)
			a.store.Last = p.Name
			if err := a.store.Save(); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			if p.RememberPassword && tunnel.AuthMethod(p.AuthMethod) == tunnel.AuthPassword && a.password.Text != "" {
				if err := config.SavePassword(p, a.password.Text); err != nil {
					a.appendLog(err.Error())
				}
			}
			if !p.RememberPassword {
				if err := config.DeletePassword(p); err != nil {
					a.appendLog(err.Error())
				}
			}
			a.profileSelect.Options = a.store.Names()
			a.profileSelect.Refresh()
			a.profileSelect.SetSelected(p.Name)
			a.appendLog(fmt.Sprintf("профиль %q сохранён (пароль в файл не записывается)", p.Name))
		}, a.win)
	form.Show()
}

// removeProfile удаляет выбранный профиль и его пароль из системного хранилища.
func (a *app) removeProfile() {
	name := a.profileSelect.Selected
	if name == "" {
		return
	}
	p, ok := a.store.Get(name)
	if !ok {
		return
	}
	dialog.NewConfirm("Удалить профиль", fmt.Sprintf("Удалить профиль %q?", name), func(yes bool) {
		if !yes {
			return
		}
		a.store.Remove(name)
		if a.store.Last == name {
			a.store.Last = ""
		}
		if err := a.store.Save(); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if err := config.DeletePassword(p); err != nil {
			a.appendLog(err.Error())
		}
		a.profileSelect.Options = a.store.Names()
		a.profileSelect.ClearSelected()
		a.profileSelect.Refresh()
		a.appendLog(fmt.Sprintf("профиль %q удалён", name))
	}, a.win).Show()
}

// appendLog добавляет строку в журнал с отметкой времени и подрезает историю.
func (a *app) appendLog(line string) {
	stamp := time.Now().Format("15:04:05")
	a.logLines = append(a.logLines, stamp+"  "+line)
	if len(a.logLines) > maxLogLines {
		a.logLines = a.logLines[len(a.logLines)-maxLogLines:]
	}
	a.logList.Refresh()
	a.logList.ScrollToBottom()
}

// logf добавляет в журнал форматированную строку.
func (a *app) logf(format string, args ...any) {
	a.appendLog(fmt.Sprintf(format, args...))
}

// clearLog очищает журнал.
func (a *app) clearLog() {
	a.logLines = nil
	a.logList.Refresh()
}

// copyLog копирует журнал в буфер обмена.
func (a *app) copyLog() {
	a.fyneApp.Clipboard().SetContent(strings.Join(a.logLines, "\n"))
}
