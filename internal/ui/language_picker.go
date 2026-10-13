package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// languagePicker displays the selected language and opens a full-width option list.
type languagePicker struct {
	widget.BaseWidget
	canvas   fyne.Canvas
	options  []string
	selected string
	onChange func(string)
	popup    *widget.PopUp
}

// newLanguagePicker creates a language control with a Select-style appearance.
func newLanguagePicker(canvas fyne.Canvas, selected string, options []string, onChange func(string)) *languagePicker {
	picker := &languagePicker{
		canvas:   canvas,
		options:  options,
		selected: selected,
		onChange: onChange,
	}
	picker.ExtendBaseWidget(picker)
	return picker
}

// CreateRenderer creates the language control's background, selected label, and dropdown icon.
func (p *languagePicker) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(p.Theme().Color(theme.ColorNamePrimary, fyne.CurrentApp().Settings().ThemeVariant()))
	background.CornerRadius = p.Theme().Size(theme.SizeNameInputRadius)
	label := canvas.NewText(p.selected, color.Black)
	label.TextStyle = fyne.TextStyle{Monospace: true}
	icon := widget.NewIcon(theme.MenuDropDownIcon())
	return &languagePickerRenderer{
		picker:     p,
		background: background,
		label:      label,
		icon:       icon,
		objects:    []fyne.CanvasObject{background, label, icon},
	}
}

// Tapped opens a dropdown whose width matches the language control.
func (p *languagePicker) Tapped(*fyne.PointEvent) {
	if p.popup != nil {
		p.popup.Hide()
		p.popup = nil
		return
	}
	content := newLanguageOptionsPopup(p, p.Size().Width)
	p.popup = widget.NewPopUp(content, p.canvas)
	p.popup.Resize(p.popup.MinSize())
	p.popup.ShowAtRelativePosition(fyne.NewPos(0, p.Size().Height), p)
}

// choose updates the selected language and closes its option list.
func (p *languagePicker) choose(language string) {
	p.selected = language
	p.Refresh()
	if p.onChange != nil {
		p.onChange(language)
	}
	if p.popup != nil {
		p.popup.Hide()
		p.popup = nil
	}
}

type languagePickerRenderer struct {
	picker     *languagePicker
	background *canvas.Rectangle
	label      *canvas.Text
	icon       *widget.Icon
	objects    []fyne.CanvasObject
}

// Destroy releases renderer resources.
func (r *languagePickerRenderer) Destroy() {}

// Layout positions the selected language and dropdown icon.
func (r *languagePickerRenderer) Layout(size fyne.Size) {
	th := r.picker.Theme()
	padding := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	r.background.Resize(size)
	r.label.TextSize = th.Size(theme.SizeNameText)
	r.label.Text = r.picker.selected
	textHeight := fyne.MeasureText("Ag", r.label.TextSize, r.label.TextStyle).Height
	r.label.Resize(fyne.NewSize(size.Width-iconSize-3*padding, textHeight))
	r.label.Move(fyne.NewPos(padding, (size.Height-textHeight)/2))
	r.icon.Resize(fyne.NewSquareSize(iconSize))
	r.icon.Move(fyne.NewPos(size.Width-iconSize-padding, (size.Height-iconSize)/2))
}

// MinSize returns enough width to display the widest available language.
func (r *languagePickerRenderer) MinSize() fyne.Size {
	th := r.picker.Theme()
	width := float32(0)
	for _, option := range r.picker.options {
		size := fyne.MeasureText(option, th.Size(theme.SizeNameText), fyne.TextStyle{Monospace: true})
		if size.Width > width {
			width = size.Width
		}
	}
	padding := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	textHeight := fyne.MeasureText("Ag", th.Size(theme.SizeNameText), fyne.TextStyle{Monospace: true}).Height
	return fyne.NewSize(width+iconSize+3*padding, textHeight+2*padding)
}

// Objects returns the language control's canvas objects.
func (r *languagePickerRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Refresh updates the displayed language and themed background.
func (r *languagePickerRenderer) Refresh() {
	th := r.picker.Theme()
	r.background.FillColor = th.Color(theme.ColorNamePrimary, fyne.CurrentApp().Settings().ThemeVariant())
	r.background.CornerRadius = th.Size(theme.SizeNameInputRadius)
	r.label.Text = r.picker.selected
	r.label.Color = color.Black
	r.label.Refresh()
	r.icon.Refresh()
	r.Layout(r.picker.Size())
	canvas.Refresh(r.picker)
}

type languageOptionsPopup struct {
	widget.BaseWidget
	picker  *languagePicker
	width   float32
	options []*languageOption
}

// newLanguageOptionsPopup creates menu rows while preserving the control's full width.
func newLanguageOptionsPopup(picker *languagePicker, width float32) *languageOptionsPopup {
	popup := &languageOptionsPopup{picker: picker, width: width}
	for _, language := range picker.options {
		popup.options = append(popup.options, newLanguageOption(language, language == picker.selected, picker.choose))
	}
	popup.ExtendBaseWidget(popup)
	return popup
}

// CreateRenderer creates a menu background and full-width language options.
func (p *languageOptionsPopup) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(p.Theme().Color(theme.ColorNameOverlayBackground, fyne.CurrentApp().Settings().ThemeVariant()))
	objects := []fyne.CanvasObject{background}
	for _, option := range p.options {
		objects = append(objects, option)
	}
	return &languageOptionsPopupRenderer{popup: p, background: background, objects: objects}
}

type languageOptionsPopupRenderer struct {
	popup      *languageOptionsPopup
	background *canvas.Rectangle
	objects    []fyne.CanvasObject
}

// Destroy releases renderer resources.
func (r *languageOptionsPopupRenderer) Destroy() {}

// Layout places each option in a full-width row.
func (r *languageOptionsPopupRenderer) Layout(size fyne.Size) {
	th := r.popup.Theme()
	padding := th.Size(theme.SizeNameInnerPadding)
	rowHeight := th.Size(theme.SizeNameText) + 2*padding
	r.background.Resize(size)
	for index, option := range r.popup.options {
		option.Move(fyne.NewPos(0, float32(index)*rowHeight))
		option.Resize(fyne.NewSize(size.Width, rowHeight))
	}
}

// MinSize makes the dropdown as wide as its language control.
func (r *languageOptionsPopupRenderer) MinSize() fyne.Size {
	th := r.popup.Theme()
	padding := th.Size(theme.SizeNameInnerPadding)
	rowHeight := th.Size(theme.SizeNameText) + 2*padding
	return fyne.NewSize(r.popup.width, float32(len(r.popup.options))*rowHeight)
}

// Objects returns the dropdown background and options.
func (r *languageOptionsPopupRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Refresh updates the popup background and option rows.
func (r *languageOptionsPopupRenderer) Refresh() {
	r.background.FillColor = r.popup.Theme().Color(theme.ColorNameOverlayBackground, fyne.CurrentApp().Settings().ThemeVariant())
	r.background.Refresh()
	for _, option := range r.popup.options {
		option.Refresh()
	}
	r.Layout(r.popup.Size())
	canvas.Refresh(r.popup)
}

type languageOption struct {
	widget.BaseWidget
	language string
	selected bool
	onSelect func(string)
	hovered  bool
}

// newLanguageOption creates one selectable language row.
func newLanguageOption(language string, selected bool, onSelect func(string)) *languageOption {
	option := &languageOption{language: language, selected: selected, onSelect: onSelect}
	option.ExtendBaseWidget(option)
	return option
}

// CreateRenderer creates a language row with a selected-state background and label.
func (o *languageOption) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(o.language, o.textColor())
	label.TextStyle = fyne.TextStyle{Monospace: true}
	return &languageOptionRenderer{option: o, background: background, label: label}
}

// Tapped selects this language.
func (o *languageOption) Tapped(*fyne.PointEvent) {
	if o.onSelect != nil {
		o.onSelect(o.language)
	}
}

// MouseIn highlights this option while hovered.
func (o *languageOption) MouseIn(*desktop.MouseEvent) {
	o.hovered = true
	o.Refresh()
}

// MouseMoved keeps the desktop hover interface satisfied without changing row state.
func (o *languageOption) MouseMoved(*desktop.MouseEvent) {}

// MouseOut removes the hover highlight.
func (o *languageOption) MouseOut() {
	o.hovered = false
	o.Refresh()
}

// textColor returns black for the selected or hovered row and the normal foreground otherwise.
func (o *languageOption) textColor() color.Color {
	th := o.Theme()
	if o.selected || o.hovered {
		return color.Black
	}
	return th.Color(theme.ColorNameForeground, fyne.CurrentApp().Settings().ThemeVariant())
}

type languageOptionRenderer struct {
	option     *languageOption
	background *canvas.Rectangle
	label      *canvas.Text
}

// Destroy releases renderer resources.
func (r *languageOptionRenderer) Destroy() {}

// Layout positions the option label inside its row.
func (r *languageOptionRenderer) Layout(size fyne.Size) {
	th := r.option.Theme()
	padding := th.Size(theme.SizeNameInnerPadding)
	textHeight := fyne.MeasureText(r.label.Text, th.Size(theme.SizeNameText), r.label.TextStyle).Height
	r.background.Resize(size)
	r.background.CornerRadius = th.Size(theme.SizeNameMenuRadius)
	r.label.TextSize = th.Size(theme.SizeNameText)
	r.label.Text = r.option.language
	r.label.Resize(fyne.NewSize(size.Width-2*padding, textHeight))
	r.label.Move(fyne.NewPos(padding, (size.Height-textHeight)/2))
}

// MinSize returns the row's minimum size.
func (r *languageOptionRenderer) MinSize() fyne.Size {
	th := r.option.Theme()
	padding := th.Size(theme.SizeNameInnerPadding)
	textSize := fyne.MeasureText(r.option.language, th.Size(theme.SizeNameText), r.label.TextStyle)
	return textSize.AddWidthHeight(2*padding, 2*padding)
}

// Objects returns the row background and label.
func (r *languageOptionRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.label}
}

// Refresh updates row colors for the selected and hovered states.
func (r *languageOptionRenderer) Refresh() {
	th := r.option.Theme()
	if r.option.selected || r.option.hovered {
		r.background.FillColor = th.Color(theme.ColorNameFocus, fyne.CurrentApp().Settings().ThemeVariant())
	} else {
		r.background.FillColor = color.Transparent
	}
	r.label.Color = r.option.textColor()
	r.label.Refresh()
	r.background.Refresh()
	r.Layout(r.option.Size())
	canvas.Refresh(r.option)
}

var _ fyne.Tappable = (*languagePicker)(nil)
var _ fyne.Widget = (*languagePicker)(nil)
var _ fyne.WidgetRenderer = (*languagePickerRenderer)(nil)
var _ fyne.Widget = (*languageOptionsPopup)(nil)
var _ fyne.WidgetRenderer = (*languageOptionsPopupRenderer)(nil)
var _ fyne.Tappable = (*languageOption)(nil)
var _ desktop.Hoverable = (*languageOption)(nil)
var _ fyne.WidgetRenderer = (*languageOptionRenderer)(nil)
