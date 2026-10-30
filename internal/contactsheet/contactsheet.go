// Package contactsheet собирает лист-сетку из миниатюр для агента (151, design §2): N×N клеток с
// номерами своими цифрами 5 × 7, заглушки для не успевших и битых фото, JPEG без метаданных в пределе
// размера. Пакет чистый: только image и x/image/draw, ни файлов, ни проекта — миниатюры приносит вызывающий.
package contactsheet

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// Пределы листа: сетка и сторона зажимаются здесь же, как последний рубеж за проверкой инструмента.
const (
	MinGrid = 3    // меньше — клетка больше миниатюры 400 px, лист зря велик
	MaxGrid = 8    // больше — клетка мала для глаз
	MaxSide = 1568 // выше этой стороны клиент всё равно уменьшает картинку
	minCell = 64   // клетка меньше не читается: номер и заглушка не помещаются
	pad     = 4    // поле клетки, px
)

// State — что лежит в клетке.
type State uint8

const (
	Ready   State = iota // фото готово: Cell.Img
	Pending              // миниатюра не успела к сроку — «…», повторный вызов возьмёт готовое
	Broken               // файла нет или он не читается — «×»
)

// String — имя состояния в ответе инструмента (`ok | pending | broken`).
func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case Broken:
		return "broken"
	}
	return "ok"
}

// Cell — одна клетка листа. N — номер на плашке (≤ 0 — без плашки).
type Cell struct {
	Img   image.Image
	N     int
	State State
}

var (
	bgColor    = color.RGBA{0x28, 0x28, 0x28, 0xff} // фон листа #282828
	holeColor  = color.RGBA{0x33, 0x33, 0x33, 0xff} // подложка заглушки
	signColor  = color.RGBA{0x9a, 0x9a, 0x9a, 0xff} // знак заглушки
	plateColor = color.RGBA{0, 0, 0, 0xff}
	textColor  = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

// Clamp зажимает сетку в MinGrid…MaxGrid и сторону в grid·minCell…MaxSide.
func Clamp(grid, side int) (int, int) {
	grid = min(max(grid, MinGrid), MaxGrid)
	return grid, min(max(side, grid*minCell), MaxSide)
}

// CellBox — сторона квадрата, в который вписывается фото клетки (клетка минус поля): вызывающий
// уменьшает миниатюру до него сразу и не держит полноразмерные картинки до сборки листа.
func CellBox(grid, side int) int {
	grid, side = Clamp(grid, side)
	return side/grid - 2*pad
}

// Rows — строк в листе из n клеток при сетке grid: ⌈n / grid⌉ в пределах 1…grid.
func Rows(n, grid int) int {
	grid, _ = Clamp(grid, 0)
	return min(max((n+grid-1)/grid, 1), grid)
}

// Compose рисует лист: клетка side/grid, фото вписано целиком (без обрезки), номер — в левом верхнем
// углу. Клеток больше grid² отбрасываются. Ширина листа — клетка × grid (до grid-1 px меньше side от
// целочисленного деления), высота — по числу строк ⌈клеток / grid⌉ (не меньше одной): на малом числе
// фото нет пустой нижней части, а размер клетки и номера те же.
func Compose(cells []Cell, grid, side int) *image.RGBA {
	grid, side = Clamp(grid, side)
	cell := side / grid
	sheet := image.NewRGBA(image.Rect(0, 0, cell*grid, cell*Rows(len(cells), grid)))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)
	for i := 0; i < len(cells) && i < grid*grid; i++ {
		x, y := i%grid*cell, i/grid*cell
		drawCell(sheet, image.Rect(x, y, x+cell, y+cell), cells[i])
	}
	return sheet
}

func drawCell(sheet *image.RGBA, r image.Rectangle, c Cell) {
	box := r.Inset(pad)
	switch {
	case c.State == Ready && usable(c.Img):
		fitPhoto(sheet, box, c.Img)
	case c.State == Pending:
		placeholder(sheet, box, glyphPending)
	default: // Broken и «готово» без картинки: лучше «×», чем пустая клетка
		placeholder(sheet, box, glyphBroken)
	}
	if c.N > 0 {
		number(sheet, r, c.N)
	}
}

// maxSrc — предел стороны исходной картинки клетки: это миниатюра (400 px), а не оригинал. Больше —
// не наше (бесконечный Uniform, ошибка вызывающего): заглушка вместо долгого или переполняющего масштаба.
const maxSrc = 8192

// usable — у картинки есть пиксели разумного размера (nil, пустые и огромные границы — не фото).
func usable(img image.Image) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	return !b.Empty() && b.Dx() <= maxSrc && b.Dy() <= maxSrc
}

// fitPhoto вписывает фото в box по центру, пропорции сохраняются. Прозрачные места (PNG) видят фон.
func fitPhoto(sheet *image.RGBA, box image.Rectangle, img image.Image) {
	sb := img.Bounds()
	w, h := box.Dx(), box.Dy()
	if sb.Dx()*h > sb.Dy()*w { // шире клетки: по ширине
		h = max(1, sb.Dy()*w/sb.Dx())
	} else {
		w = max(1, sb.Dx()*h/sb.Dy())
	}
	x := box.Min.X + (box.Dx()-w)/2
	y := box.Min.Y + (box.Dy()-h)/2
	xdraw.ApproxBiLinear.Scale(sheet, image.Rect(x, y, x+w, y+h), img, sb, xdraw.Over, nil)
}

// placeholder — подложка клетки и знак по центру.
func placeholder(sheet *image.RGBA, box image.Rectangle, g glyph) {
	fill(sheet, box, holeColor)
	sc := max(2, box.Dx()/12)
	w, h := glyphW*sc, glyphH*sc
	x := box.Min.X + (box.Dx()-w)/2
	y := box.Min.Y + (box.Dy()-h)/2
	drawGlyph(sheet, box, g, x, y, sc, signColor)
}

// fill заливает прямоугольник цветом (за пределами листа draw обрезает сам).
func fill(sheet *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(sheet, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}
