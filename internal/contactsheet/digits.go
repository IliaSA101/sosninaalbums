package contactsheet

import (
	"image"
	"image/color"
	"strconv"
)

// Размер знака в «пикселях» шрифта.
const (
	glyphW = 5
	glyphH = 7
)

// glyph — знак 5 × 7: строки сверху вниз, «1» — закрашено.
type glyph [glyphH]string

// Шрифт свой, а не internal/text: десять цифр не стоят шрифта sfnt и раскладки, а побайтный тест
// пакета без зависимостей проще (0.34-151-4).
var digits = [10]glyph{
	{"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	{"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	{"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	{"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	{"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	{"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	{"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	{"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	{"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	{"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
}

var (
	// «…» посередине строки, а не на базовой линии: знак по центру клетки остаётся на виду.
	glyphPending = glyph{"00000", "00000", "00000", "10101", "00000", "00000", "00000"}
	glyphBroken  = glyph{"00000", "10001", "01010", "00100", "01010", "10001", "00000"}
)

// advance — шаг знака в «пикселях»: ширина и пробел.
const advance = glyphW + 1

// number рисует номер n белым на чёрной плашке в левом верхнем углу клетки cell. Масштаб
// max(2, клетка/70); не помещается по ширине — уменьшается до 1, остальное обрезается клеткой.
func number(sheet *image.RGBA, cell image.Rectangle, n int) {
	s := strconv.Itoa(n)
	sc := max(2, cell.Dx()/70)
	for sc > 1 && plateW(len(s), sc) > cell.Dx()-2*pad {
		sc--
	}
	x, y := cell.Min.X+pad, cell.Min.Y+pad
	plate := image.Rect(x, y, x+plateW(len(s), sc), y+(glyphH+2)*sc).Intersect(cell)
	fill(sheet, plate, plateColor)
	for i := range len(s) {
		drawGlyph(sheet, plate, digits[s[i]-'0'], x+sc+i*advance*sc, y+sc, sc, textColor)
	}
}

// plateW — ширина плашки: знаки с пробелами и поле в один «пиксель» с краёв.
func plateW(n, sc int) int {
	return n*advance*sc + sc
}

// drawGlyph рисует знак с левым верхним углом (x, y) и масштабом sc; clip ограничивает область.
func drawGlyph(sheet *image.RGBA, clip image.Rectangle, g glyph, x, y, sc int, c color.RGBA) {
	for gy, row := range g {
		for gx := range len(row) {
			if row[gx] != '1' {
				continue
			}
			px, py := x+gx*sc, y+gy*sc
			fill(sheet, image.Rect(px, py, px+sc, py+sc).Intersect(clip), c)
		}
	}
}
