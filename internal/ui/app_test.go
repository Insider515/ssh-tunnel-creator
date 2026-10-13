package ui

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"sshtunnel/internal/config"
)

// newTestApp creates the application window with Fyne's test implementation.
func newTestApp(t *testing.T, store *config.Store, settings ...config.Settings) *app {
	t.Helper()
	a := &app{fyneApp: test.NewApp()}
	a.win = a.fyneApp.NewWindow("test")
	a.store = store
	if len(settings) > 0 {
		a.settings = settings[0]
	}
	a.build()
	return a
}

// sampleProfile returns a profile without secrets for UI tests.
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

// TestOpenProfileLoadsFieldsAndShowsEditor verifies loading a profile and opening its editor.
func TestOpenProfileLoadsFieldsAndShowsEditor(t *testing.T) {
	p := sampleProfile("work")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}, Last: p.Name})

	a.openProfile(p.Name)

	if a.jumpHost.Text != p.JumpHost {
		t.Fatalf("jump host is %q, want %q", a.jumpHost.Text, p.JumpHost)
	}
	if a.jumpPort.Text != "2222" {
		t.Fatalf("jump port is %q, want 2222", a.jumpPort.Text)
	}
	if a.profileName != p.Name {
		t.Fatalf("opened profile %q, want %q", a.profileName, p.Name)
	}
	if !a.verifyHostKey.Checked {
		t.Fatal("host-key verification was not loaded from the profile")
	}
	if !a.showingEditor {
		t.Fatal("profile settings screen is not shown")
	}
	if a.profileNameEntry.Text != p.Name {
		t.Fatalf("profile name field is %q, want %q", a.profileNameEntry.Text, p.Name)
	}
}

// TestTerminalThemeUsesDarkNeonPalette verifies the main terminal theme colors.
func TestTerminalThemeUsesDarkNeonPalette(t *testing.T) {
	background := terminalTheme{}.Color(theme.ColorNameBackground, theme.VariantDark)
	backgroundColor, ok := background.(color.NRGBA)
	if !ok || backgroundColor.R > 20 || backgroundColor.G > 50 || backgroundColor.B > 55 {
		t.Fatalf("background is not dark: %#v", background)
	}

	primary := terminalTheme{}.Color(theme.ColorNamePrimary, theme.VariantDark)
	primaryColor, ok := primary.(color.NRGBA)
	if !ok || primaryColor.G < 200 || primaryColor.R != 0 {
		t.Fatalf("primary color is not neon green: %#v", primary)
	}

	foregroundOnPrimary := terminalTheme{}.Color(theme.ColorNameForegroundOnPrimary, theme.VariantDark)
	if foregroundOnPrimary != (color.NRGBA{A: 255}) {
		t.Fatalf("foreground on selected controls is %#v, want black", foregroundOnPrimary)
	}

	gotFont := terminalTheme{}.Font(fyne.TextStyle{})
	wantFont := theme.DefaultTheme().Font(fyne.TextStyle{Monospace: true})
	if gotFont.Name() != wantFont.Name() {
		t.Fatalf("theme font is %q, want monospace font %q", gotFont.Name(), wantFont.Name())
	}
}

// TestOutlinedButtonUsesRequestedAccent verifies the button border color.
func TestOutlinedButtonUsesRequestedAccent(t *testing.T) {
	outlined := outlinedButton(widget.NewButton("test", func() {})).(*fyne.Container)
	if len(outlined.Objects) != 2 {
		t.Fatalf("outlined button contains %d objects, want button and border", len(outlined.Objects))
	}
	border, ok := outlined.Objects[1].(*canvas.Rectangle)
	if !ok {
		t.Fatalf("outlined button border has type %T, want *canvas.Rectangle", outlined.Objects[1])
	}
	if border.StrokeColor != (color.NRGBA{R: 182, G: 255, B: 100, A: 255}) {
		t.Fatalf("button border color is %#v, want #B6FF64", border.StrokeColor)
	}
	if border.CornerRadius != accentBorderRadius {
		t.Fatalf("button border corner radius is %v, want %v", border.CornerRadius, accentBorderRadius)
	}
}

// TestLanguagePickerUsesBlackSelectedText verifies the selected value's text color.
func TestLanguagePickerUsesBlackSelectedText(t *testing.T) {
	a := test.NewApp()
	picker := newLanguagePicker(a.NewWindow("test").Canvas(), "English", []string{"English", "Ukrainian"}, nil)
	renderer := picker.CreateRenderer().(*languagePickerRenderer)
	if renderer.label.Color != color.Black {
		t.Fatalf("selected language text color is %#v, want black", renderer.label.Color)
	}

	picker.Resize(fyne.NewSize(640, 48))
	popup := newLanguageOptionsPopup(picker, picker.Size().Width)
	popupRenderer := popup.CreateRenderer().(*languageOptionsPopupRenderer)
	if got := popupRenderer.MinSize().Width; got != picker.Size().Width {
		t.Fatalf("dropdown width is %v, want selector width %v", got, picker.Size().Width)
	}
	selectedOption := popup.options[0].CreateRenderer().(*languageOptionRenderer)
	if selectedOption.label.Color != color.Black {
		t.Fatalf("selected dropdown option text color is %#v, want black", selectedOption.label.Color)
	}
}

// TestBuildStartsOnProfileList verifies that the first profile is selected on startup.
func TestBuildStartsOnProfileList(t *testing.T) {
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{sampleProfile("work")}})

	if a.showingEditor {
		t.Fatal("editor is shown instead of the profile list on startup")
	}
	if a.profileList == nil {
		t.Fatal("profile list was not created")
	}
	if a.selectedProfile != "work" {
		t.Fatalf("selected profile is %q, want work", a.selectedProfile)
	}
	if a.startBtn.Disabled() {
		t.Fatal("connect button is disabled after a profile was selected")
	}
	if a.settingsBtn.Disabled() {
		t.Fatal("profile settings button is disabled after a profile was selected")
	}
}

// TestBuildPrefersDefaultProfile verifies that the configured default wins over the first profile.
func TestBuildPrefersDefaultProfile(t *testing.T) {
	store := &config.Store{Profiles: []config.Profile{sampleProfile("alpha"), sampleProfile("zeta")}}
	settings := config.Settings{DefaultProfile: "zeta"}
	a := newTestApp(t, store, settings)

	if a.selectedProfile != "zeta" {
		t.Fatalf("selected profile is %q, want default profile zeta", a.selectedProfile)
	}
}

// TestBuildFallsBackToFirstProfileWhenDefaultIsMissing verifies fallback to the first listed profile.
func TestBuildFallsBackToFirstProfileWhenDefaultIsMissing(t *testing.T) {
	store := &config.Store{Profiles: []config.Profile{sampleProfile("zeta"), sampleProfile("alpha")}}
	settings := config.Settings{DefaultProfile: "missing"}
	a := newTestApp(t, store, settings)

	if a.selectedProfile != "alpha" {
		t.Fatalf("selected profile is %q, want first sorted profile alpha", a.selectedProfile)
	}
}

// TestBuildLeavesSelectionEmptyWithoutProfiles verifies that startup does not select a missing profile.
func TestBuildLeavesSelectionEmptyWithoutProfiles(t *testing.T) {
	a := newTestApp(t, &config.Store{})

	if a.selectedProfile != "" {
		t.Fatalf("selected profile is %q, want no selection", a.selectedProfile)
	}
	if !a.startBtn.Disabled() {
		t.Fatal("connect button is enabled without a profile")
	}
}

// TestSelectingProfileDoesNotOpenEditor verifies that selecting a profile keeps the list visible.
func TestSelectingProfileDoesNotOpenEditor(t *testing.T) {
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{sampleProfile("work")}})

	a.profileList.Select(0)

	if a.showingEditor {
		t.Fatal("selecting a profile unexpectedly opened the editor")
	}
	if a.selectedProfile != "work" {
		t.Fatalf("selected profile is %q, want work", a.selectedProfile)
	}
	if a.jumpHost.Text != "jump.example" {
		t.Fatalf("loaded jump host is %q after profile selection, want jump.example", a.jumpHost.Text)
	}
	if a.startBtn.Disabled() {
		t.Fatal("connect button remains disabled after profile selection")
	}
	if a.settingsBtn.Disabled() {
		t.Fatal("profile settings button remains disabled after profile selection")
	}
	root, ok := a.win.Content().(*fyne.Container)
	if !ok {
		t.Fatalf("root container has type %T, want *fyne.Container", a.win.Content())
	}
	hasJournalSplit := false
	for _, object := range root.Objects {
		if _, ok := object.(*container.Split); ok {
			hasJournalSplit = true
			break
		}
	}
	if !hasJournalSplit {
		t.Fatal("main screen does not show the log panel after profile selection")
	}
}

// TestNewProfileOpensDefaultSettings verifies that adding a profile opens its default settings.
func TestNewProfileOpensDefaultSettings(t *testing.T) {
	a := newTestApp(t, &config.Store{})

	a.newProfile()

	if !a.showingEditor {
		t.Fatal("profile settings did not open after adding a profile")
	}
	if a.profileName != "" {
		t.Fatalf("new profile unexpectedly has a saved name %q", a.profileName)
	}
	if a.profileNameEntry.Text != "" {
		t.Fatalf("new profile name field is unexpectedly populated: %q", a.profileNameEntry.Text)
	}
	if a.localPort.Text != "8080" {
		t.Fatalf("local port is %q, want 8080", a.localPort.Text)
	}
	if a.startBtn.Text != "Connect" {
		t.Fatalf("connect button label is %q, want Connect", a.startBtn.Text)
	}
}

// TestApplyProfileClearsPreviousKeyPassphrase verifies that secrets are cleared between profiles.
func TestApplyProfileClearsPreviousKeyPassphrase(t *testing.T) {
	a := newTestApp(t, &config.Store{})
	a.keyPassphrase.SetText("previous profile secret")

	a.applyProfile(sampleProfile("other profile"))

	if a.keyPassphrase.Text != "" {
		t.Fatal("the previous profile passphrase was left in the field")
	}
}

// TestCollectProfileReadsFields verifies that profile fields round-trip correctly.
func TestCollectProfileReadsFields(t *testing.T) {
	p := sampleProfile("work")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}})
	a.applyProfile(p)

	got, err := a.collectProfile()
	if err != nil {
		t.Fatalf("could not collect profile fields: %v", err)
	}
	if got.JumpHost != p.JumpHost || got.JumpPort != p.JumpPort || got.TargetPort != p.TargetPort {
		t.Fatalf("collected profile %+v differs from source %+v", got, p)
	}
	if got.AuthMethod != "password" {
		t.Fatalf("authentication method is %q, want password", got.AuthMethod)
	}
	if got.KeepAliveSec != 20 {
		t.Fatalf("keepalive is %d, want 20", got.KeepAliveSec)
	}
}

// TestCollectProfileRejectsBadPort verifies that invalid ports return clear errors.
func TestCollectProfileRejectsBadPort(t *testing.T) {
	p := sampleProfile("work")
	a := newTestApp(t, &config.Store{Profiles: []config.Profile{p}})
	a.applyProfile(p)

	a.localPort.SetText("")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "is not set") {
		t.Fatalf("empty port was not rejected: %v", err)
	}

	a.localPort.SetText("not a number")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "must be a number") {
		t.Fatalf("non-numeric port was not rejected: %v", err)
	}

	a.localPort.SetText("70000")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "1..65535") {
		t.Fatalf("out-of-range port was not rejected: %v", err)
	}

	a.localPort.SetText("8080")
	a.keepAlive.SetText("0")
	if _, err := a.collectProfile(); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("zero keepalive was not rejected: %v", err)
	}
}

// TestAuthFormSwitchesWithMethod verifies that the form follows the selected authentication method.
func TestAuthFormSwitchesWithMethod(t *testing.T) {
	a := newTestApp(t, &config.Store{})
	a.applyProfile(config.NewProfile())

	a.authSelect.SetSelected("Password")
	if len(a.authForm.Items) != 2 || a.authForm.Items[0].Widget != a.password {
		t.Fatalf("password authentication shows %d fields", len(a.authForm.Items))
	}

	a.authSelect.SetSelected("Private key")
	if len(a.authForm.Items) != 2 || a.authForm.Items[1].Widget != a.keyPassphrase {
		t.Fatalf("private-key authentication shows %d fields", len(a.authForm.Items))
	}

	a.authSelect.SetSelected("ssh-agent")
	if len(a.authForm.Items) != 1 {
		t.Fatalf("ssh-agent authentication shows %d fields", len(a.authForm.Items))
	}
}

// TestLogIsTrimmed verifies that the log has a fixed maximum size.
func TestLogIsTrimmed(t *testing.T) {
	a := newTestApp(t, &config.Store{})
	for i := 0; i < maxLogLines+50; i++ {
		a.appendLog("line")
	}
	if len(a.logLines) != maxLogLines {
		t.Fatalf("log contains %d lines, want %d", len(a.logLines), maxLogLines)
	}
}
