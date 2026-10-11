// Команда genicon рисует иконку приложения в файл assets/icon.png.
// Иконка генерируется кодом, чтобы её не приходилось держать бинарником в репозитории
// и чтобы размер и цвета можно было поменять в одном месте.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// size — сторона квадратной иконки в пикселях.
const size = 512

// samples — кратность суперсэмплинга по каждой оси для сглаживания краёв.
const samples = 4

// цвета иконки.
var (
	bgTop   = color.NRGBA{R: 0x1d, G: 0x30, B: 0x57, A: 0xff}
	bgDown  = color.NRGBA{R: 0x0b, G: 0x12, B: 0x22, A: 0xff}
	ringCol = color.NRGBA{R: 0x4d, G: 0x9f, B: 0xff, A: 0xff}
	arrow   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// main рисует иконку и записывает её в assets/icon.png.
func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, pixel(float64(x), float64(y)))
		}
	}
	f, err := os.Create("assets/icon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

// pixel считает итоговый цвет одного пикселя, усредняя несколько подпикселей.
// Усреднение даёт сглаженные края скруглённого квадрата, кольца и стрелки.
func pixel(px, py float64) color.NRGBA {
	var r, g, b, a float64
	step := 1.0 / float64(samples)
	for sy := 0; sy < samples; sy++ {
		for sx := 0; sx < samples; sx++ {
			c := sample(px+(float64(sx)+0.5)*step, py+(float64(sy)+0.5)*step)
			r += float64(c.R)
			g += float64(c.G)
			b += float64(c.B)
			a += float64(c.A)
		}
	}
	n := float64(samples * samples)
	return color.NRGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n)}
}

// sample возвращает цвет иконки в точке с дробными координатами.
// Слои накладываются снизу вверх: фон, кольцо-«тоннель», стрелка.
func sample(x, y float64) color.NRGBA {
	if !insideRoundedSquare(x, y, 24, 110) {
		return color.NRGBA{}
	}
	c := gradient(y)
	if insideRing(x, y, 256, 256, 152, 20) {
		c = ringCol
	}
	if insideArrow(x, y) {
		c = arrow
	}
	return c
}

// gradient задаёт вертикальную заливку фона от светлого верха к тёмному низу.
func gradient(y float64) color.NRGBA {
	t := y / size
	lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	return color.NRGBA{R: lerp(bgTop.R, bgDown.R), G: lerp(bgTop.G, bgDown.G), B: lerp(bgTop.B, bgDown.B), A: 0xff}
}

// insideRoundedSquare сообщает, попадает ли точка внутрь скруглённого квадрата
// с отступом margin от края изображения и радиусом скругления radius.
func insideRoundedSquare(x, y, margin, radius float64) bool {
	minX, minY := margin, margin
	maxX, maxY := size-margin, size-margin
	if x < minX || x > maxX || y < minY || y > maxY {
		return false
	}
	cx := math.Min(math.Max(x, minX+radius), maxX-radius)
	cy := math.Min(math.Max(y, minY+radius), maxY-radius)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

// insideRing сообщает, попадает ли точка в кольцо заданного радиуса и толщины.
func insideRing(x, y, cx, cy, radius, thickness float64) bool {
	d := math.Hypot(x-cx, y-cy)
	return d >= radius-thickness/2 && d <= radius+thickness/2
}

// insideArrow сообщает, попадает ли точка в горизонтальную стрелку,
// проходящую через центр иконки слева направо.
func insideArrow(x, y float64) bool {
	const (
		shaftLeft  = 120.0
		shaftRight = 320.0
		shaftHalf  = 26.0
		headRight  = 400.0
		headHalf   = 74.0
		centerY    = 256.0
	)
	if x >= shaftLeft && x <= shaftRight && math.Abs(y-centerY) <= shaftHalf {
		return true
	}
	if x > shaftRight && x <= headRight {
		t := (headRight - x) / (headRight - shaftRight)
		return math.Abs(y-centerY) <= headHalf*t
	}
	return false
}
