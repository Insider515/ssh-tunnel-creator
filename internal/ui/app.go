// Package ui provides the Fyne interface for connection settings, logging, and tunnel control.
// Network operations run in goroutines, and UI updates are dispatched with fyne.Do.
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
	"sshtunnel/internal/i18n"
	"sshtunnel/internal/startup"
	"sshtunnel/internal/tunnel"
)

// maxLogLines is the maximum number of recent log lines kept in memory.
const maxLogLines = 1000

// authLabels maps interface labels to authentication methods.
var authLabels = map[string]tunnel.AuthMethod{
	"Password":    tunnel.AuthPassword,
	"Private key": tunnel.AuthKey,
	"ssh-agent":   tunnel.AuthAgent,
}

// authOrder defines the order of authentication methods in the selector.
var authOrder = []string{"Password", "Private key", "ssh-agent"}

// app stores application window state.
type app struct {
	fyneApp fyne.App
	win     fyne.Window

	store    *config.Store
	settings config.Settings

	profileList      *widget.List
	profileName      string
	selectedProfile  string
	showingEditor    bool
	profileNameEntry *widget.Entry
	localPort        *widget.Entry
	jumpHost         *widget.Entry
	jumpPort         *widget.Entry
	jumpUser         *widget.Entry
	authSelect       *widget.Select
	password         *widget.Entry
	rememberPass     *widget.Check
	keyPath          *widget.Entry
	keyPassphrase    *widget.Entry
	authForm         *widget.Form
	targetHost       *widget.Entry
	targetPort       *widget.Entry
	verifyHostKey    *widget.Check
	keepAlive        *widget.Entry
	autoReconnect    *widget.Check

	checkBtn             *widget.Button
	startBtn             *widget.Button
	stopBtn              *widget.Button
	settingsBtn          *widget.Button
	appSettingsBtn       *widget.Button
	languageSelect       *languagePicker
	languageSelected     string
	launchAtLoginCheck   *widget.Check
	autoConnectCheck     *widget.Check
	defaultProfileSelect *widget.Select
	settingsPage         bool

	statusLabel   *widget.Label
	statsLabel    *widget.Label
	currentStatus tunnel.Status
	statusMessage string
	stats         tunnel.Stats
	logList       *widget.List
	logLines      []string

	tun             *tunnel.Tunnel
	cancel          context.CancelFunc
	windowAnimation *fyne.Animation
}

// Run builds and runs the application window until it closes.
func Run() {
	a := &app{fyneApp: fyneapp.NewWithID("com.geekproxy.sshtunnel")}
	a.win = a.fyneApp.NewWindow("SSH Tunnel")
	a.win.Resize(fyne.NewSize(575, 700))
	a.fyneApp.Settings().SetTheme(terminalTheme{})

	store, profileLoadErr := config.Load()
	if profileLoadErr != nil {
		store = &config.Store{}
	}
	a.store = store
	settings, settingsLoadErr := config.LoadSettings()
	a.settings = settings
	if settingsLoadErr != nil {
		a.settings = config.DefaultSettings()
	}
	i18n.SetLanguage(i18n.Normalize(a.settings.Language))

	a.build()

	a.win.SetCloseIntercept(func() {
		a.stopTunnel()
		a.win.Close()
	})
	a.win.Show()
	if profileLoadErr != nil {
		dialog.ShowError(profileLoadErr, a.win)
	}
	if settingsLoadErr != nil {
		dialog.ShowError(settingsLoadErr, a.win)
	}
	if a.settings.LaunchAtLogin {
		if executable, err := os.Executable(); err != nil {
			a.showError("Could not determine the application executable: %w", err)
		} else if err := startup.Set(true, executable); err != nil {
			a.showError("Automatic startup failed: %w", err)
		}
	}
	if a.settings.AutoConnect && a.settings.DefaultProfile != "" {
		a.selectAndConnectDefaultProfile()
	}
	a.fyneApp.Run()
}

// build creates the application widgets and sets the initial content.
func (a *app) build() {
	a.buildFields()
	a.buildButtons()
	a.buildLog()

	a.profileList = widget.NewList(
		func() int { return len(a.store.Names()) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			names := a.store.Names()
			if int(id) < len(names) {
				obj.(*widget.Label).SetText(names[id])
			}
		},
	)
	a.profileList.OnSelected = func(id widget.ListItemID) {
		names := a.store.Names()
		if int(id) < len(names) {
			a.selectedProfile = names[id]
			if p, ok := a.store.Get(a.selectedProfile); ok {
				a.applyProfile(p)
			}
			a.refreshProfileList()
		}
	}
	a.showProfileList()
	a.selectInitialProfile()
}

// selectInitialProfile selects the configured default profile or the first profile by name.
func (a *app) selectInitialProfile() {
	names := a.store.Names()
	if len(names) == 0 {
		return
	}
	selectedIndex := 0
	for index, name := range names {
		if name == a.settings.DefaultProfile {
			selectedIndex = index
			break
		}
	}
	a.profileList.Select(widget.ListItemID(selectedIndex))
}

// profileForm creates the connection settings form.
func (a *app) profileForm() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewCard("1. "+i18n.T("Local port"), i18n.T("Address used by local applications"),
			widget.NewForm(widget.NewFormItem(i18n.T("Port"), a.localPort)),
		),
		widget.NewCard("2. "+i18n.T("Jump server"), i18n.T("The connection is established through this server"),
			container.NewVBox(
				widget.NewForm(
					widget.NewFormItem(i18n.T("Address"), a.jumpHost),
					widget.NewFormItem(i18n.T("Port"), a.jumpPort),
					widget.NewFormItem(i18n.T("Username"), a.jumpUser),
					widget.NewFormItem(i18n.T("Authentication"), a.authSelect),
				),
				a.authForm,
			),
		),
		widget.NewCard("3. "+i18n.T("Target server"), i18n.T("The jump server connects to this destination"),
			widget.NewForm(
				widget.NewFormItem(i18n.T("Address"), a.targetHost),
				widget.NewFormItem(i18n.T("Port"), a.targetPort),
			),
		),
		widget.NewCard("4. "+i18n.T("Connection settings"), "",
			container.NewVBox(
				a.verifyHostKey,
				a.autoReconnect,
				widget.NewForm(widget.NewFormItem(i18n.T("Keepalive, sec"), a.keepAlive)),
			),
		),
	)
}

// profileEditor creates the selected profile editor and log view.
func (a *app) profileEditor() fyne.CanvasObject {
	back := widget.NewButton(i18n.T("Profiles"), a.showProfileList)
	save := widget.NewButtonWithIcon(i18n.T("Save"), theme.DocumentSaveIcon(), a.saveProfile)
	remove := widget.NewButtonWithIcon(i18n.T("Delete"), theme.DeleteIcon(), a.removeProfile)
	a.profileNameEntry.SetPlaceHolder(i18n.T("Edit the current profile name"))
	a.profileNameEntry.SetText(a.profileName)
	header := container.NewBorder(nil, nil, outlinedButton(back), container.NewHBox(outlinedButton(save), outlinedButton(remove)), a.profileNameEntry)
	logPane := a.logPanel()
	settings := container.NewVScroll(a.profileForm())
	split := container.NewHSplit(settings, logPane)
	split.SetOffset(0.62)
	return terminalSurface(container.NewBorder(header, nil, nil, nil, split))
}

// logPanel creates the log view with management, export, and connection-check controls.
func (a *app) logPanel() fyne.CanvasObject {
	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle(i18n.T("Log"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			outlinedButton(widget.NewButtonWithIcon(i18n.T("Clear"), theme.DeleteIcon(), a.clearLog)),
			outlinedButton(widget.NewButtonWithIcon(i18n.T("Copy"), theme.ContentCopyIcon(), a.copyLog)),
			outlinedButton(widget.NewButtonWithIcon(i18n.T("Export"), theme.DocumentSaveIcon(), a.exportLog)),
		),
		outlinedButton(a.checkBtn), nil, nil,
		a.logList,
	)
}

// showProfileList displays the profile list and its actions.
func (a *app) showProfileList() {
	a.showingEditor = false
	a.settingsPage = false
	a.resizeWindow(fyne.NewSize(575, 700))
	a.settingsBtn = widget.NewButtonWithIcon(i18n.T("Edit profile"), theme.SettingsIcon(), func() {
		if a.selectedProfile != "" {
			a.openProfile(a.selectedProfile)
		}
	})
	a.appSettingsBtn = widget.NewButtonWithIcon(i18n.T("App settings"), theme.SettingsIcon(), a.showAppSettings)
	a.refreshProfileList()
}

// refreshProfileList updates the main window and shows the log when a profile is selected.
func (a *app) refreshProfileList() {
	a.startBtn.Disable()
	if a.selectedProfile != "" && !a.isRunning() {
		a.startBtn.Enable()
	}
	a.settingsBtn.Disable()
	if a.selectedProfile != "" {
		a.settingsBtn.Enable()
	}
	add := widget.NewButtonWithIcon(i18n.T("Add profile"), theme.ContentAddIcon(), a.newProfile)
	toolbar := container.NewVBox(
		container.NewHBox(outlinedButton(add), outlinedButton(a.settingsBtn), outlinedButton(a.startBtn), outlinedButton(a.stopBtn), outlinedButton(a.appSettingsBtn)),
		container.NewVBox(a.statusLabel, a.statsLabel),
	)
	profileArea := container.NewBorder(toolbar, nil, nil, nil, a.profileList)
	if a.selectedProfile == "" {
		a.win.SetContent(terminalSurface(profileArea))
		return
	}
	split := container.NewVSplit(profileArea, a.logPanel())
	split.SetOffset(0.78)
	a.win.SetContent(terminalSurface(split))
}

// showAppSettings displays language, startup, and default-profile preferences.
func (a *app) showAppSettings() {
	a.settingsPage = true
	if a.settings.Language == string(i18n.Ukrainian) {
		a.languageSelected = i18n.T("Ukrainian")
	} else {
		a.languageSelected = i18n.T("English")
	}
	a.languageSelect = newLanguagePicker(
		a.win.Canvas(),
		a.languageSelected,
		[]string{i18n.T("English"), i18n.T("Ukrainian")},
		func(selected string) {
			a.languageSelected = selected
		},
	)
	a.launchAtLoginCheck = widget.NewCheck(i18n.T("Launch at system login"), nil)
	a.launchAtLoginCheck.SetChecked(a.settings.LaunchAtLogin)
	a.autoConnectCheck = widget.NewCheck(i18n.T("Connect to the default profile when the application starts"), nil)
	a.autoConnectCheck.SetChecked(a.settings.AutoConnect)
	a.defaultProfileSelect = widget.NewSelect(a.store.Names(), nil)
	a.defaultProfileSelect.PlaceHolder = i18n.T("Not selected")
	if a.settings.DefaultProfile != "" {
		a.defaultProfileSelect.SetSelected(a.settings.DefaultProfile)
	}

	form := widget.NewForm(
		widget.NewFormItem(i18n.T("Language"), a.languageSelect),
		widget.NewFormItem("", a.launchAtLoginCheck),
		widget.NewFormItem("", a.autoConnectCheck),
		widget.NewFormItem(i18n.T("Default connection profile"), a.defaultProfileSelect),
	)
	header := container.NewHBox(
		outlinedButton(widget.NewButton(i18n.T("Back"), a.showProfileList)),
		widget.NewLabelWithStyle(i18n.T("Application settings"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	save := widget.NewButtonWithIcon(i18n.T("Save"), theme.DocumentSaveIcon(), a.saveAppSettings)
	content := container.NewBorder(header, outlinedButton(save), nil, nil, container.NewVScroll(form))
	a.win.SetContent(terminalSurface(content))
}

// saveAppSettings validates and persists application-wide preferences.
func (a *app) saveAppSettings() {
	language := string(i18n.English)
	if a.languageSelected == i18n.T("Ukrainian") {
		language = string(i18n.Ukrainian)
	}
	defaultProfile := a.defaultProfileSelect.Selected
	if a.autoConnectCheck.Checked {
		if defaultProfile == "" {
			a.showError("Select a default profile before enabling auto-connect", nil)
			return
		}
		if _, ok := a.store.Get(defaultProfile); !ok {
			a.showError("The selected default profile no longer exists", nil)
			return
		}
	}
	next := config.Settings{
		Language:       language,
		LaunchAtLogin:  a.launchAtLoginCheck.Checked,
		AutoConnect:    a.autoConnectCheck.Checked,
		DefaultProfile: defaultProfile,
	}

	executable, err := os.Executable()
	if err != nil {
		a.showError("Could not determine the application executable: %w", err)
		return
	}
	if err := startup.Set(next.LaunchAtLogin, executable); err != nil {
		a.showError("Automatic startup failed: %w", err)
		return
	}
	if err := next.Save(); err != nil {
		if rollbackErr := startup.Set(a.settings.LaunchAtLogin, executable); rollbackErr != nil {
			a.appendLog(fmt.Sprintf(i18n.T("Could not restore the previous startup setting: %w"), rollbackErr))
		}
		a.showError("Settings could not be saved: %w", err)
		return
	}

	a.settings = next
	i18n.SetLanguage(i18n.Normalize(language))
	a.refreshLocalizedWidgets()
	a.showProfileList()
	a.appendLog(i18n.T("Settings saved"))
}

// selectAndConnectDefaultProfile selects and starts the configured profile on application launch.
func (a *app) selectAndConnectDefaultProfile() {
	names := a.store.Names()
	for index, name := range names {
		if name == a.settings.DefaultProfile {
			a.profileList.Select(widget.ListItemID(index))
			a.activateSelectedProfile()
			return
		}
	}
	a.appendLog(i18n.T("The selected default profile no longer exists"))
}

// showError displays an error using the active application language.
func (a *app) showError(format string, err error) {
	if err == nil {
		dialog.ShowError(errors.New(i18n.T(format)), a.win)
		return
	}
	dialog.ShowError(fmt.Errorf(i18n.T(format), err), a.win)
}

// showProfileEditor opens the current profile editor.
func (a *app) showProfileEditor() {
	a.showingEditor = true
	a.resizeWindow(fyne.NewSize(850, 845))
	a.win.SetContent(a.profileEditor())
}

// resizeWindow animates window resizing when switching between the list and editor.
func (a *app) resizeWindow(target fyne.Size) {
	if a.windowAnimation != nil {
		a.windowAnimation.Stop()
		a.windowAnimation = nil
	}
	start := a.win.Canvas().Size()
	if start.Width <= 0 || start.Height <= 0 || start == target {
		a.win.Resize(target)
		return
	}

	var animation *fyne.Animation
	animation = fyne.NewAnimation(240*time.Millisecond, func(progress float32) {
		eased := 1 - (1-progress)*(1-progress)
		width := start.Width + (target.Width-start.Width)*eased
		height := start.Height + (target.Height-start.Height)*eased
		a.win.Resize(fyne.NewSize(width, height))
		if progress >= 1 {
			a.windowAnimation = nil
		}
	})
	a.windowAnimation = animation
	animation.Start()
}

// openProfile loads a saved profile and opens its editor.
func (a *app) openProfile(name string) {
	p, ok := a.store.Get(name)
	if !ok {
		return
	}
	a.selectedProfile = name
	a.profileList.UnselectAll()
	a.profileName = name
	a.applyProfile(p)
	a.showProfileEditor()
}

// newProfile opens an empty form with default settings.
func (a *app) newProfile() {
	a.selectedProfile = ""
	a.applyProfile(config.NewProfile())
	a.profileName = ""
	a.profileNameEntry.SetText("")
	a.showProfileEditor()
}

// buildFields creates the connection input widgets.
func (a *app) buildFields() {
	a.profileNameEntry = widget.NewEntry()
	a.localPort = widget.NewEntry()
	a.jumpHost = widget.NewEntry()
	a.jumpHost.SetPlaceHolder(i18n.T("Address or hostname"))
	a.jumpPort = widget.NewEntry()
	a.jumpUser = widget.NewEntry()

	a.password = widget.NewPasswordEntry()
	a.rememberPass = widget.NewCheck(i18n.T("Keep the password in the OS credential store"), nil)
	a.keyPath = widget.NewEntry()
	a.keyPath.SetPlaceHolder(i18n.T("Path to the private key file"))
	a.keyPassphrase = widget.NewPasswordEntry()
	a.keyPassphrase.SetPlaceHolder(i18n.T("Required if the key is encrypted"))

	a.authForm = widget.NewForm()
	a.authSelect = widget.NewSelect(translatedAuthOrder(), func(string) { a.rebuildAuthForm() })

	a.targetHost = widget.NewEntry()
	a.targetHost.SetPlaceHolder(i18n.T("Address or hostname"))
	a.targetPort = widget.NewEntry()

	a.verifyHostKey = widget.NewCheck(i18n.T("Verify the jump server host key (protects against server impersonation)"), nil)
	a.autoReconnect = widget.NewCheck(i18n.T("Automatic reconnect after connection loss"), nil)
	a.keepAlive = widget.NewEntry()
}

// translatedAuthOrder returns authentication labels in the active language.
func translatedAuthOrder() []string {
	options := make([]string, len(authOrder))
	for index, label := range authOrder {
		options[index] = i18n.T(label)
	}
	return options
}

// authMethodForLabel resolves a translated authentication label to its protocol value.
func authMethodForLabel(label string) tunnel.AuthMethod {
	for english, method := range authLabels {
		if label == english || label == i18n.T(english) {
			return method
		}
	}
	return ""
}

// rebuildAuthForm displays fields for the selected authentication method.
func (a *app) rebuildAuthForm() {
	browse := widget.NewButtonWithIcon(i18n.T("Browse"), theme.FolderOpenIcon(), a.pickKeyFile)

	switch authMethodForLabel(a.authSelect.Selected) {
	case tunnel.AuthPassword:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem(i18n.T("Password"), a.password),
			widget.NewFormItem("", a.rememberPass),
		}
	case tunnel.AuthKey:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem(i18n.T("Key file"), container.NewBorder(nil, nil, nil, outlinedButton(browse), a.keyPath)),
			widget.NewFormItem(i18n.T("Passphrase"), a.keyPassphrase),
		}
	case tunnel.AuthAgent:
		a.authForm.Items = []*widget.FormItem{
			widget.NewFormItem("", widget.NewLabel(i18n.T("Open SSH-agent keys are used"))),
		}
	default:
		a.authForm.Items = nil
	}
	a.authForm.Refresh()
}

// pickKeyFile opens a file dialog for selecting a private key.
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

// buildButtons creates the check, connect, and stop controls.
func (a *app) buildButtons() {
	a.checkBtn = widget.NewButtonWithIcon(i18n.T("Check connection"), theme.SearchIcon(), a.checkTunnel)
	a.startBtn = widget.NewButtonWithIcon(i18n.T("Connect"), theme.MediaPlayIcon(), a.activateSelectedProfile)
	a.startBtn.Importance = widget.HighImportance
	a.startBtn.Disable()
	a.stopBtn = widget.NewButtonWithIcon(i18n.T("Stop"), theme.MediaStopIcon(), a.stopTunnel)
	a.stopBtn.Disable()

	a.statusLabel = widget.NewLabel(i18n.T("Status: idle"))
	a.statusLabel.Wrapping = fyne.TextWrapWord
	a.statsLabel = widget.NewLabel(fmt.Sprintf(i18n.T("Connections: active %d, total %d"), 0, 0))
	a.currentStatus = tunnel.StatusIdle
	a.statusMessage = i18n.T("Tunnel stopped")
}

// buildLog creates the log list. widget.List renders visible rows only, keeping
// append cost independent of the total log length.
func (a *app) buildLog() {
	a.logList = widget.NewList(
		func() int { return len(a.logLines) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.TextStyle = fyne.TextStyle{Monospace: true}
			l.Truncation = fyne.TextTruncateEllipsis
			l.Importance = widget.WarningImportance
			return l
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(a.logLines) {
				o.(*widget.Label).SetText(a.logLines[i])
			}
		},
	)
}

// applyProfile fills the form with profile values and loads a saved password when enabled.
func (a *app) applyProfile(p config.Profile) {
	a.profileName = p.Name
	if a.profileNameEntry != nil {
		a.profileNameEntry.SetText(p.Name)
	}
	a.localPort.SetText(strconv.Itoa(p.LocalPort))
	a.jumpHost.SetText(p.JumpHost)
	a.jumpPort.SetText(strconv.Itoa(p.JumpPort))
	a.jumpUser.SetText(p.JumpUser)
	a.keyPath.SetText(p.KeyPath)
	a.keyPassphrase.SetText("")
	a.targetHost.SetText(p.TargetHost)
	a.targetPort.SetText(strconv.Itoa(p.TargetPort))
	a.verifyHostKey.SetChecked(p.VerifyHostKey)
	a.autoReconnect.SetChecked(p.AutoReconnect)
	if p.KeepAliveSec <= 0 {
		p.KeepAliveSec = 15
	}
	a.keepAlive.SetText(strconv.Itoa(p.KeepAliveSec))
	a.rememberPass.SetChecked(p.RememberPassword)

	label := "Password"
	for name, code := range authLabels {
		if string(code) == p.AuthMethod {
			label = name
		}
	}
	a.authSelect.SetSelected(i18n.T(label))
	a.rebuildAuthForm()

	a.password.SetText("")
	if p.RememberPassword && p.JumpHost != "" && p.JumpUser != "" {
		if pass, err := config.LoadPassword(p); err != nil {
			a.logf(i18n.T("Could not read a password from the OS credential store: %v"), err)
		} else if pass != "" {
			a.password.SetText(pass)
			a.logf(i18n.T("Password for profile %q was loaded from the OS credential store"), p.Name)
		}
	}
}

// collectProfile reads and validates profile values from the form.
func (a *app) collectProfile() (config.Profile, error) {
	localPort, err := parsePort(a.localPort.Text, i18n.T("Local port"))
	if err != nil {
		return config.Profile{}, err
	}
	jumpPort, err := parsePort(a.jumpPort.Text, i18n.T("Jump server port"))
	if err != nil {
		return config.Profile{}, err
	}
	targetPort, err := parsePort(a.targetPort.Text, i18n.T("Target server port"))
	if err != nil {
		return config.Profile{}, err
	}
	keepAlive, err := strconv.Atoi(strings.TrimSpace(a.keepAlive.Text))
	if err != nil || keepAlive < 1 {
		return config.Profile{}, fmt.Errorf(i18n.T("Keepalive must be a positive integer number of seconds, got %q"), a.keepAlive.Text)
	}

	name := a.profileName
	if name == "" {
		name = i18n.T("New profile")
	}
	return config.Profile{
		Name:             name,
		LocalPort:        localPort,
		JumpHost:         strings.TrimSpace(a.jumpHost.Text),
		JumpPort:         jumpPort,
		JumpUser:         strings.TrimSpace(a.jumpUser.Text),
		AuthMethod:       string(authMethodForLabel(a.authSelect.Selected)),
		KeyPath:          strings.TrimSpace(a.keyPath.Text),
		TargetHost:       strings.TrimSpace(a.targetHost.Text),
		TargetPort:       targetPort,
		VerifyHostKey:    a.verifyHostKey.Checked,
		KeepAliveSec:     keepAlive,
		AutoReconnect:    a.autoReconnect.Checked,
		RememberPassword: a.rememberPass.Checked,
	}, nil
}

// parsePort parses a port field and returns a specific validation error.
func parsePort(text, name string) (int, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, fmt.Errorf(i18n.T("%s is not set"), name)
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf(i18n.T("%s must be a number, got %q"), name, text)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf(i18n.T("%s must be in the range 1..65535, got %d"), name, port)
	}
	return port, nil
}

// buildTunnelConfig combines profile values and in-memory credentials into a tunnel config.
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

// newTunnel creates a tunnel whose callbacks dispatch updates to the UI thread.
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
				a.stats = st
				a.refreshStatsLabel()
			})
		},
	})
}

// checkTunnel validates the connection without leaving a tunnel running.
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
				if a.showingEditor || a.selectedProfile != "" {
					a.startBtn.Enable()
				} else {
					a.startBtn.Disable()
				}
			}
			if err != nil {
				dialog.ShowError(err, a.win)
			}
		})
	}()
}

// startTunnel verifies the connection before binding the local port.
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
	// Keep a local reference so the goroutine cannot observe a later replacement of a.tun.
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
				if a.showingEditor || a.selectedProfile != "" {
					a.startBtn.Enable()
				} else {
					a.startBtn.Disable()
				}
				dialog.ShowError(err, a.win)
				return
			}
			a.stopBtn.Enable()
			if !a.showingEditor {
				a.showProfileList()
			}
		})
	}()
}

// activateSelectedProfile loads and starts the selected profile.
func (a *app) activateSelectedProfile() {
	if !a.showingEditor {
		p, ok := a.store.Get(a.selectedProfile)
		if !ok {
			return
		}
		a.applyProfile(p)
	}
	a.startTunnel()
}

// stopTunnel stops the tunnel and restores the control states.
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
	if a.showingEditor || a.selectedProfile != "" {
		a.startBtn.Enable()
	} else {
		a.startBtn.Disable()
	}
}

// isRunning reports whether the tunnel is running.
func (a *app) isRunning() bool {
	return a.tun != nil && a.tun.Running()
}

// setStatus displays the tunnel state and updates the controls.
func (a *app) setStatus(s tunnel.Status, msg string) {
	a.currentStatus = s
	a.statusMessage = msg
	a.refreshStatusLabel()
	switch s {
	case tunnel.StatusActive, tunnel.StatusReconnecting:
		a.stopBtn.Enable()
		a.startBtn.Disable()
		a.checkBtn.Disable()
	case tunnel.StatusIdle, tunnel.StatusError:
		if !a.isRunning() {
			a.stopBtn.Disable()
			if a.showingEditor || a.selectedProfile != "" {
				a.startBtn.Enable()
			} else {
				a.startBtn.Disable()
			}
			a.checkBtn.Enable()
		}
	}
}

// refreshStatusLabel renders the latest tunnel state using the active language.
func (a *app) refreshStatusLabel() {
	a.statusLabel.SetText(fmt.Sprintf(i18n.T("Status: %s — %s"), i18n.T(string(a.currentStatus)), a.statusMessage))
}

// refreshStatsLabel renders connection counters using the active language.
func (a *app) refreshStatsLabel() {
	a.statsLabel.SetText(fmt.Sprintf(i18n.T("Connections: active %d, total %d"), a.stats.Active, a.stats.Total))
}

// refreshLocalizedWidgets updates controls that persist across a language change.
func (a *app) refreshLocalizedWidgets() {
	a.checkBtn.SetText(i18n.T("Check connection"))
	a.startBtn.SetText(i18n.T("Connect"))
	a.stopBtn.SetText(i18n.T("Stop"))
	a.rememberPass.SetText(i18n.T("Keep the password in the OS credential store"))
	a.verifyHostKey.SetText(i18n.T("Verify the jump server host key (protects against server impersonation)"))
	a.autoReconnect.SetText(i18n.T("Automatic reconnect after connection loss"))
	a.jumpHost.SetPlaceHolder(i18n.T("Address or hostname"))
	a.targetHost.SetPlaceHolder(i18n.T("Address or hostname"))
	a.keyPath.SetPlaceHolder(i18n.T("Path to the private key file"))
	a.keyPassphrase.SetPlaceHolder(i18n.T("Required if the key is encrypted"))
	a.authSelect.SetOptions(translatedAuthOrder())

	if p, ok := a.store.Get(a.selectedProfile); ok {
		for label, method := range authLabels {
			if string(method) == p.AuthMethod {
				a.authSelect.SetSelected(i18n.T(label))
				break
			}
		}
		a.applyProfile(p)
	} else {
		a.authSelect.SetSelected(i18n.T("Password"))
	}
	a.rebuildAuthForm()
	a.refreshStatusLabel()
	a.refreshStatsLabel()
}

// hostKeyPrompt asks whether to trust an unknown host key and waits for the response.
func (a *app) hostKeyPrompt(host, keyType, fingerprint string) bool {
	answer := make(chan bool, 1)
	fyne.Do(func() {
		text := fmt.Sprintf(
			i18n.T("Server %s presented a host key that has not been seen before.\n\nKey type: %s\nFingerprint: %s\n\nDo you trust this key and want to remember it?"),
			host, keyType, fingerprint)
		d := dialog.NewConfirm(i18n.T("Unknown host key"), text, func(ok bool) { answer <- ok }, a.win)
		d.SetConfirmText(i18n.T("Trust"))
		d.SetDismissText(i18n.T("Cancel"))
		d.Show()
	})
	return <-answer
}

// saveProfile saves the current form under the name in the header.
func (a *app) saveProfile() {
	p, err := a.collectProfile()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	name := strings.TrimSpace(a.profileNameEntry.Text)
	if name == "" {
		dialog.ShowError(errors.New(i18n.T("Enter a profile name")), a.win)
		return
	}
	if name != a.profileName {
		if _, exists := a.store.Get(name); exists {
			dialog.ShowError(fmt.Errorf(i18n.T("Profile %q already exists"), name), a.win)
			return
		}
	}

	p.Name = name
	updated := &config.Store{
		Profiles: append([]config.Profile(nil), a.store.Profiles...),
		Last:     name,
	}
	if a.profileName != "" && a.profileName != name {
		updated.Remove(a.profileName)
	}
	updated.Upsert(p)
	if err := updated.Save(); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.store = updated

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
	a.profileName = p.Name
	a.selectedProfile = p.Name
	a.profileList.Refresh()
	a.showProfileEditor()
	a.appendLog(fmt.Sprintf(i18n.T("Profile %q saved (password is not written to the file)"), p.Name))
}

// removeProfile deletes the selected profile and its stored password.
func (a *app) removeProfile() {
	name := a.profileName
	if name == "" {
		return
	}
	p, ok := a.store.Get(name)
	if !ok {
		return
	}
	dialog.NewConfirm(i18n.T("Delete profile"), fmt.Sprintf(i18n.T("Delete profile %q?"), name), func(yes bool) {
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
		a.profileList.Refresh()
		a.profileName = ""
		a.selectedProfile = ""
		a.showProfileList()
		a.appendLog(fmt.Sprintf(i18n.T("Profile %q deleted"), name))
	}, a.win).Show()
}

// appendLog adds a timestamped line to the log and trims excess history.
func (a *app) appendLog(line string) {
	stamp := time.Now().Format("15:04:05")
	a.logLines = append(a.logLines, stamp+"  "+line)
	if len(a.logLines) > maxLogLines {
		a.logLines = a.logLines[len(a.logLines)-maxLogLines:]
	}
	a.logList.Refresh()
	a.logList.ScrollToBottom()
}

// logf adds a formatted line to the log.
func (a *app) logf(format string, args ...any) {
	a.appendLog(fmt.Sprintf(format, args...))
}

// clearLog removes all log lines.
func (a *app) clearLog() {
	a.logLines = nil
	a.logList.Refresh()
}

// copyLog copies the log to the clipboard.
func (a *app) copyLog() {
	a.fyneApp.Clipboard().SetContent(strings.Join(a.logLines, "\n"))
}

// exportLog saves the log lines to a text file selected by the user.
func (a *app) exportLog() {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if writer == nil {
			return
		}
		_, writeErr := io.WriteString(writer, strings.Join(a.logLines, "\n"))
		if err := errors.Join(writeErr, writer.Close()); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	d.SetFileName("ssh-tunnel-log.txt")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".txt"}))
	d.Show()
}
