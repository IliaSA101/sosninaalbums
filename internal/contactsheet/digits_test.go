package contactsheet

import (
	"image"
	"image/color"
	"strconv"
	"testing"
)

// Таблица знаков: у всех семь строк по пять клеток, десять цифр попарно различны, «…» и «×» тоже.
func TestSheetDigitsTable(t *testing.T) {
	all := append(digits[:], glyphPending, glyphBroken)
	seen := map[glyph]int{}
	for i, g := range all {
		for _, row := range g {
			if len(row) != glyphW {
				t.Fatalf("знак %d: строка %q не из %d клеток", i, row, glyphW)
			}
			for _, r := range row {
				if r != '0' && r != '1' {
					t.Fatalf("знак %d: символ %q", i, r)
				}
			}
		}
		if j, dup := seen[g]; dup {
			t.Errorf("знаки %d и %d совпадают", j, i)
		}
		seen[g] = i
	}
	// Опорные знаки закреплены буквально: правка шрифта — осознанная.
	want1 := glyph{"00100", "01100", "00100", "00100", "00100", "00100", "01110"}
	if digits[1] != want1 {
		t.Errorf("«1» изменилась: %v", digits[1])
	}
}

// bitAt — клетка знака (gx, gy) нарисована? Читаем пиксель в середине клетки масштаба sc.
func bitAt(im *image.RGBA, x0, y0, sc, gx, gy int) color.RGBA {
	return im.RGBAAt(x0+gx*sc+sc/2, y0+gy*sc+sc/2)
}

// Номера рисуются своими цифрами: каждый бит знака — белый, остальное плашки — чёрное, плашка по размеру
// и масштаб max(2, клетка/70). Сверка идёт со всеми десятью цифрами и тремя масштабами.
func TestSheetDigits(t *testing.T) {
	photo := color.RGBA{90, 140, 200, 255}
	white, black := color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}
	cases := []struct {
		name       string
		grid, side int
		n          int
		sc         int
	}{
		{"масштаб падает до 1", 6, 1056, 123456789012345, 1}, // 15 цифр при масштабе 2 — 182 px, в 168 не помещаются
		{"масштаб 2, клетка 176, три цифры", 6, 1056, 907, 2},
		{"масштаб 3, клетка 256", 3, 768, 4812, 3},
		{"масштаб 7, клетка 522", 3, 1568, 1234567890, 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sheet := Compose([]Cell{{Img: solid(300, 300, photo), N: c.n}}, c.grid, c.side)
			s := strconv.Itoa(c.n)
			sc := c.sc
			x, y := pad, pad
			// Плашка: чёрная по краям, под знаками.
			wantColor(t, sheet, x, y, black, "угол плашки")
			wantColor(t, sheet, x+plateW(len(s), sc)-1, y+(glyphH+2)*sc-1, black, "противоположный угол плашки")
			wantColor(t, sheet, x+plateW(len(s), sc), y, photo, "справа от плашки — фото")
			for i := range len(s) {
				g := digits[s[i]-'0']
				gx0, gy0 := x+sc+i*advance*sc, y+sc
				for gy, row := range g {
					for gx := range len(row) {
						want := black
						if row[gx] == '1' {
							want = white
						}
						if got := bitAt(sheet, gx0, gy0, sc, gx, gy); got != want {
							t.Fatalf("цифра %q (позиция %d), клетка (%d,%d): %v, ждали %v", s[i], i, gx, gy, got, want)
						}
					}
				}
			}
		})
	}
}

// Номер у разных клеток стоит в своём углу и не задевает соседей; N ≤ 0 — плашки нет; огромный номер
// и крошечная клетка обрезаются клеткой.
func TestSheetDigitsEdges(t *testing.T) {
	photo := color.RGBA{90, 140, 200, 255}
	black := color.RGBA{0, 0, 0, 255}
	// Без номера: на месте плашки фото, а не чёрное.
	for _, n := range []int{0, -1, -1 << 40} {
		sheet := Compose([]Cell{{Img: solid(300, 300, photo), N: n}}, 3, 768)
		wantColor(t, sheet, pad+1, pad+1, photo, "без плашки")
	}
	// Вторая клетка: плашка в её углу, в первой — своя.
	cells := []Cell{{Img: solid(300, 300, photo), N: 1}, {Img: solid(300, 300, photo), N: 22}}
	sheet := Compose(cells, 3, 768)
	wantColor(t, sheet, pad, pad, black, "плашка первой")
	wantColor(t, sheet, 256+pad, pad, black, "плашка второй")
	wantColor(t, sheet, 256-1, pad, bgColor, "поле между клетками не задето")

	// Предельное число в самой малой клетке: помещается в клетку, соседа не трогает.
	huge := []Cell{{Img: solid(10, 10, photo), N: int(^uint(0) >> 1)}, {Img: solid(10, 10, photo), N: 1}}
	sheet = Compose(huge, 8, 512) // клетка 64 px
	const cell = 64
	for y := range cell {
		for x := cell; x < cell+pad; x++ { // поле между первой и второй клеткой
			if at(sheet, x, y) == black {
				t.Fatalf("плашка вышла за клетку: (%d,%d)", x, y)
			}
		}
	}
	// Все 19 цифр не помещаются в 56 px даже при масштабе 1 (115 px): правая часть плашки обрезана клеткой.
	wantColor(t, sheet, cell-pad-1, pad, black, "плашка упирается в поле клетки")
}
