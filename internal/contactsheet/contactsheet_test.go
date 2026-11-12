package contactsheet

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
	"testing"
)

// solid — картинка одного цвета w × h.
func solid(w, h int, c color.RGBA) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(im, im.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return im
}

// at — пиксель листа.
func at(im *image.RGBA, x, y int) color.RGBA {
	return im.RGBAAt(x, y)
}

// near — цвета равны с допуском на округление масштаба.
func near(a, b color.RGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= 2 && d(a.G, b.G) <= 2 && d(a.B, b.B) <= 2 && d(a.A, b.A) <= 2
}

func wantColor(t *testing.T, im *image.RGBA, x, y int, want color.RGBA, what string) {
	t.Helper()
	if got := at(im, x, y); !near(got, want) {
		t.Errorf("%s: пиксель (%d,%d) = %v, ждали %v", what, x, y, got, want)
	}
}

// Лист 6 колонок на 1056 px: размер, фото вписаны целиком по центру клетки, поля и остаток — фон.
func TestSheetCompose(t *testing.T) {
	const grid, side, cell, n = 6, 1056, 176, 28
	red := color.RGBA{200, 30, 30, 255}
	cells := make([]Cell, 0, n)
	for i := range n { // 28 из 30 (5 строк): хвост последней строки остаётся фоном
		var im *image.RGBA
		switch i % 3 {
		case 0:
			im = solid(400, 200, red) // широкое
		case 1:
			im = solid(200, 400, red) // высокое
		default:
			im = solid(300, 300, red) // квадрат
		}
		cells = append(cells, Cell{Img: im, N: i + 1})
	}
	sheet := Compose(cells, grid, side)
	if b := sheet.Bounds(); b.Dx() != side || b.Dy() != 5*cell {
		t.Fatalf("размер листа %v, ждали %d×%d", b, side, 5*cell)
	}
	for i := range n {
		x0, y0 := i%grid*cell, i/grid*cell
		cx, cy := x0+cell/2, y0+cell/2
		wantColor(t, sheet, cx, cy, red, "центр фото")
		wantColor(t, sheet, x0+cell-1, y0+cell-1, bgColor, "правый нижний угол клетки (поле)")
		switch i % 3 {
		case 0: // 168 × 84 по центру box: сверху и снизу 42 px фона
			wantColor(t, sheet, cx, y0+pad+42-3, bgColor, "над широким")
			wantColor(t, sheet, cx, y0+pad+42+3, red, "верх широкого")
			wantColor(t, sheet, cx, y0+pad+42+84+3, bgColor, "под широким")
			wantColor(t, sheet, x0+pad+3, cy, red, "ширина широкого — вся клетка")
		case 1: // 84 × 168: слева и справа по 42 px
			wantColor(t, sheet, x0+pad+42-3, cy, bgColor, "слева от высокого")
			wantColor(t, sheet, x0+pad+42+3, cy, red, "левый край высокого")
			wantColor(t, sheet, cx, y0+pad+3, red, "высота высокого — вся клетка")
		}
	}
	for i := n; i < 30; i++ {
		x0, y0 := i%grid*cell, i/grid*cell
		wantColor(t, sheet, x0+cell/2, y0+cell/2, bgColor, "пустая клетка")
	}
}

// Высота листа — по числу строк ⌈n / grid⌉, ширина и клетка прежние (QA 151: 12 фото на сетке 6 × 6
// давали пустые две трети). Края: нет клеток — одна строка, клеток больше сетки — квадрат.
func TestSheetRows(t *testing.T) {
	green := color.RGBA{20, 160, 40, 255}
	cases := []struct {
		name         string
		n, grid      int
		wantRows     int
		wantW, wantH int
	}{
		{"12 фото на 6 → 2 строки", 12, 6, 2, 1056, 2 * 176},
		{"36 фото на 6 → 6 строк", 36, 6, 6, 1056, 6 * 176},
		{"1 фото → 1 строка", 1, 6, 1, 1056, 176},
		{"7 фото на 6 → 2 строки", 7, 6, 2, 1056, 2 * 176},
		{"6 фото на 6 → 1 строка", 6, 6, 1, 1056, 176},
		{"0 клеток → 1 строка", 0, 6, 1, 1056, 176},
		{"100 клеток → не выше сетки", 100, 6, 6, 1056, 6 * 176},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cells := make([]Cell, c.n)
			for i := range cells {
				cells[i] = Cell{Img: solid(10, 10, green), N: i + 1}
			}
			if got := Rows(c.n, c.grid); got != c.wantRows {
				t.Errorf("Rows(%d, %d) = %d, ждали %d", c.n, c.grid, got, c.wantRows)
			}
			sheet := Compose(cells, c.grid, 1056)
			if b := sheet.Bounds(); b.Dx() != c.wantW || b.Dy() != c.wantH {
				t.Fatalf("размер %v, ждали %d×%d", b, c.wantW, c.wantH)
			}
			if c.n > 0 { // последнее фото в своей клетке, а не вне листа
				i := min(c.n, c.grid*c.grid) - 1
				wantColor(t, sheet, i%c.grid*176+88, i/c.grid*176+88, green, "последнее фото")
			}
		})
	}
	// Render отдаёт тот же размер, что и лист.
	cells := make([]Cell, 12)
	for i := range cells {
		cells[i] = Cell{Img: solid(300, 200, green), N: i + 1}
	}
	if _, w, h, err := Render(cells, 6, 1056); err != nil || w != 1056 || h != 2*176 {
		t.Fatalf("Render: %d×%d, %v", w, h, err)
	}
}

// Края: сетка и сторона зажимаются, лишние и недостающие клетки, нет клеток, детерминизм.
func TestSheetComposeEdges(t *testing.T) {
	green := color.RGBA{20, 160, 40, 255}
	cases := []struct {
		name       string
		grid, side int
		wantSide   int
	}{
		{"сетка 0 → 3", 0, 1568, 522 * 3},
		{"сетка отрицательная → 3", -5, 1568, 522 * 3},
		{"сетка 2 → 3", 2, 1568, 522 * 3},
		{"сетка 99 → 8", 99, 1568, 196 * 8},
		{"сторона 0 → низ", 6, 0, 64 * 6},
		{"сторона мала для 8 клеток", 8, 100, 64 * 8},
		{"сторона огромна → 1568", 4, 1 << 30, 392 * 4},
		{"сторона не кратна", 7, 1000, 142 * 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			many := make([]Cell, 100) // больше, чем клеток в любой сетке
			for i := range many {
				many[i] = Cell{Img: solid(10, 10, green), N: i + 1}
			}
			sheet := Compose(many, c.grid, c.side)
			if b := sheet.Bounds(); b.Dx() != c.wantSide || b.Dy() != c.wantSide {
				t.Fatalf("размер %v, ждали %d", b, c.wantSide)
			}
		})
	}

	// Ни одной клетки: лист целиком фоновый.
	for _, cells := range [][]Cell{nil, {}} {
		sheet := Compose(cells, 4, 768)
		for _, p := range []image.Point{{0, 0}, {383, 100}, {767, 191}, {5, 150}} {
			wantColor(t, sheet, p.X, p.Y, bgColor, "пустой лист")
		}
	}
	// Одна клетка: остальные — фон, соседняя не задета.
	one := Compose([]Cell{{Img: solid(50, 50, green), N: 1}}, 4, 768)
	wantColor(t, one, 96, 96, green, "единственное фото")
	wantColor(t, one, 192+96, 96, bgColor, "соседняя клетка")

	// Одно и то же на входе — побайтно одно и то же на выходе.
	in := []Cell{{Img: solid(40, 30, green), N: 5}, {State: Pending, N: 6}, {State: Broken, N: 7}}
	a, b := Compose(in, 3, 768), Compose(in, 3, 768)
	if string(a.Pix) != string(b.Pix) {
		t.Fatal("Compose недетерминирован")
	}
}

// Особые картинки: подкартинка со смещением, прозрачный PNG, поле Min не в нуле.
func TestSheetComposeImages(t *testing.T) {
	blue := color.RGBA{10, 30, 220, 255}
	// Подкартинка: Bounds().Min не нулевой.
	big := solid(400, 400, blue)
	sub := big.SubImage(image.Rect(100, 100, 300, 300))
	sheet := Compose([]Cell{{Img: sub, N: 1}}, 3, 768)
	wantColor(t, sheet, 128, 128, blue, "подкартинка")

	// Прозрачный NRGBA: прозрачные места показывают фон листа, а не чёрный.
	nr := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	sheet = Compose([]Cell{{Img: nr, N: 1}}, 3, 768)
	wantColor(t, sheet, 128, 128, bgColor, "прозрачное фото")

	// Серое одноканальное и YCbCr (то, что отдают декодеры JPEG) — без паники, цвет на месте.
	g := image.NewGray(image.Rect(0, 0, 80, 60))
	for i := range g.Pix {
		g.Pix[i] = 200
	}
	sheet = Compose([]Cell{{Img: g, N: 1}}, 3, 768)
	wantColor(t, sheet, 128, 128, color.RGBA{200, 200, 200, 255}, "серое")
	yc := image.NewYCbCr(image.Rect(0, 0, 80, 60), image.YCbCrSubsampleRatio420)
	for i := range yc.Y {
		yc.Y[i] = 128
	}
	for i := range yc.Cb {
		yc.Cb[i], yc.Cr[i] = 128, 128
	}
	sheet = Compose([]Cell{{Img: yc, N: 1}}, 3, 768)
	wantColor(t, sheet, 128, 128, color.RGBA{128, 128, 128, 255}, "YCbCr")

	// Очень узкое и очень высокое: хоть один пиксель, без деления на ноль и паники.
	// box 248 px: полоска в один пиксель стоит на 4+(248-1)/2 = 127.
	sheet = Compose([]Cell{{Img: solid(4000, 1, blue), N: 1}}, 3, 768)
	wantColor(t, sheet, 128, 127, blue, "горизонтальная полоска")
	sheet = Compose([]Cell{{Img: solid(1, 4000, blue), N: 1}}, 3, 768)
	wantColor(t, sheet, 127, 128, blue, "вертикальная полоска")
}

// Случайные размеры, сетки, состояния, номера: Compose не паникует, размер в пределах.
func FuzzCompose(f *testing.F) {
	f.Add(6, 1056, 400, 300, 12, uint8(0))
	f.Add(0, 0, 0, 0, 0, uint8(1))
	f.Add(99, 1<<30, 1, 4000, 1<<30, uint8(2))
	f.Add(-1, -1, 4000, 1, -7, uint8(7))
	f.Fuzz(func(t *testing.T, grid, side, w, h, n int, st uint8) {
		w, h = min(max(w, 0), 600), min(max(h, 0), 600)
		var img image.Image
		if w > 0 && h > 0 { // nil и пустая — тоже входы, но не для NewRGBA
			img = image.NewRGBA(image.Rect(0, 0, w, h))
		}
		cells := make([]Cell, int(st%80))
		for i := range cells {
			cells[i] = Cell{Img: img, N: n + i, State: State(st >> 4 % 3)}
		}
		sheet := Compose(cells, grid, side)
		if b := sheet.Bounds(); b.Dy() > b.Dx() || b.Dy() < b.Dx()/MaxGrid || b.Dx() < MinGrid*minCell || b.Dx() > MaxSide {
			t.Fatalf("размер %v", b)
		}
	})
}

// noisy — шум (худший случай размера JPEG); seed делает тест воспроизводимым.
func noisy(w, h int, seed uint64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, seed))
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255
	}
	return im
}

// TestCellBox: сторона вписываемого фото — клетка без полей; зажим сетки и стороны общий с Compose.
func TestCellBox(t *testing.T) {
	for _, c := range []struct{ grid, side, want int }{{6, 1056, 176 - 8}, {8, 1568, 196 - 8}, {3, 768, 256 - 8}, {1, 10, 64 - 8}, {99, 9999, 196 - 8}} {
		if got := CellBox(c.grid, c.side); got != c.want {
			t.Errorf("CellBox(%d, %d) = %d, ждали %d", c.grid, c.side, got, c.want)
		}
	}
}
