package ui

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const accentBorderRadius float32 = 8

// terminalTheme defines a dark teal and green palette for the terminal-style interface.
type terminalTheme struct{}

// Color returns the palette color for a theme role or the default dark-theme color.
func (terminalTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 4, G: 27, B: 31, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 8, G: 67, B: 57, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 91, G: 132, B: 124, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 26, G: 58, B: 55, A: 255}
	case theme.ColorNameError:
		return color.NRGBA{R: 255, G: 94, B: 112, A: 255}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0, G: 255, B: 164, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 204, G: 231, B: 220, A: 255}
	case theme.ColorNameForegroundOnWarning:
		return color.NRGBA{R: 255, G: 221, B: 104, A: 255}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 255, G: 221, B: 104, A: 255}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 7, G: 45, B: 43, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 17, G: 81, B: 68, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 7, G: 43, B: 45, A: 255}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0, G: 172, B: 117, A: 255}
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 5, G: 34, B: 36, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 109, G: 153, B: 143, A: 255}
	case theme.ColorNamePressed, theme.ColorNameSelection:
		return color.NRGBA{R: 12, G: 112, B: 82, A: 255}
	case theme.ColorNamePrimary, theme.ColorNameSuccess:
		return color.NRGBA{R: 0, G: 238, B: 145, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0, G: 133, B: 94, A: 255}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 7, G: 36, B: 38, A: 255}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 15, G: 99, B: 79, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 12, B: 15, A: 210}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 255, G: 225, B: 90, A: 255}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

// Font preserves the requested style and uses a monospace font.
func (terminalTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Monospace = true
	return theme.DefaultTheme().Font(style)
}

// Icon returns an icon from the default Fyne theme.
func (terminalTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size returns element sizes from the default Fyne theme.
func (terminalTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// outlinedButton adds a persistent accent border around a button without changing its behavior.
func outlinedButton(button *widget.Button) fyne.CanvasObject {
	outline := canvas.NewRectangle(color.NRGBA{A: 0})
	outline.StrokeColor = color.NRGBA{R: 182, G: 255, B: 100, A: 255}
	outline.StrokeWidth = 1
	outline.CornerRadius = accentBorderRadius
	return container.NewStack(button, outline)
}

// terminalSurface decorates content with a gradient, scanline texture, and border.
func terminalSurface(content fyne.CanvasObject) fyne.CanvasObject {
	background := canvas.NewVerticalGradient(
		color.NRGBA{R: 5, G: 42, B: 39, A: 255},
		color.NRGBA{R: 4, G: 25, B: 35, A: 255},
	)
	texture := canvas.NewImageFromImage(scanlineTexture())
	texture.FillMode = canvas.ImageFillStretch
	texture.ScaleMode = canvas.ImageScaleSmooth
	frame := canvas.NewRectangle(color.NRGBA{A: 0})
	frame.StrokeColor = color.NRGBA{R: 0, G: 255, B: 153, A: 180}
	frame.StrokeWidth = 1.5
	frame.CornerRadius = accentBorderRadius
	return container.NewStack(background, content, texture, frame)
}

// scanlineTexture creates a repeating translucent pattern of horizontal lines.
func scanlineTexture() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 850, 845))
	for y := 0; y < 845; y += 6 {
		for x := 0; x < 850; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 0, G: 255, B: 150, A: 12})
		}
	}
	return img
}
