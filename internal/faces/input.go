package faces

import (
	"image"
	"image/color"
	"math"
)

const (
	// inputLong — длинная сторона входа сети. На 320 теряется лицо в рост (stock-73), на 640 — нет.
	inputLong = 640
	// maxSourcePixels — выше этого картинку не берём: превью библиотеки — 2048 px, а перебор
	// десятков миллионов пикселей ради поиска лица — признак не того файла.
	maxSourcePixels = 1 << 27
)

// turned — картинка, повёрнутая на q четвертей по часовой. Пикселей не копирует: Detect
// сначала уменьшает исходник и поворачивает уже маленький вход.
type turned struct {
	src image.Image
	q   int
}

func (t *turned) ColorModel() color.Model { return t.src.ColorModel() }

func (t *turned) Bounds() image.Rectangle {
	b := t.src.Bounds()
	if t.q%2 == 1 {
		return image.Rect(0, 0, b.Dy(), b.Dx())
	}
	return image.Rect(0, 0, b.Dx(), b.Dy())
}

func (t *turned) At(x, y int) color.Color {
	b := t.src.Bounds()
	w, h := b.Dx(), b.Dy()
	switch t.q {
	case 1:
		return t.src.At(b.Min.X+y, b.Min.Y+h-1-x)
	case 2:
		return t.src.At(b.Min.X+w-1-x, b.Min.Y+h-1-y)
	case 3:
		return t.src.At(b.Min.X+w-1-y, b.Min.Y+x)
	}
	return t.src.At(b.Min.X+x, b.Min.Y+y)
}

// Turn поворачивает картинку на q четвертей по часовой (как поворот фото в проекте) до поиска
// лица: у снимка с поворотом EXIF или рукой лицо лежит боком, и сеть его не видит. q любой:
// берётся по модулю 4. Копии пикселей нет.
func Turn(img image.Image, q int) image.Image {
	if img == nil {
		return nil
	}
	q = ((q % 4) + 4) % 4
	if t, ok := img.(*turned); ok {
		img, q = t.src, (t.q+q)%4
	}
	if q == 0 {
		return img
	}
	return &turned{src: img, q: q}
}

// netInput — подготовленный вход сети: BGR 0…255, CHW, картинка в левом верхнем углу, остальное нули.
type netInput struct {
	d    []float32
	w, h int // размер картинки в входе (после уменьшения и поворота)
	W, H int // размер входа, кратный 32
}

// prepareInput уменьшает картинку (с усреднением, Box) до длинной стороны long и кладёт в тензор.
// ok == false — картинка пуста или слишком велика. Тензор потом учитывает сеть (run берёт его во владение).
func prepareInput(img image.Image, long int) (netInput, bool) {
	q := 0
	if t, is := img.(*turned); is {
		img, q = t.src, t.q
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 || (sw > maxSourcePixels/sh) {
		return netInput{}, false
	}
	rw, rh := sw, sh // размеры после поворота
	if q%2 == 1 {
		rw, rh = sh, sw
	}
	k := float64(long) / float64(max(rw, rh))
	w, h := max(1, int(math.Round(float64(rw)*k))), max(1, int(math.Round(float64(rh)*k)))
	in := netInput{w: w, h: h, W: (w + 31) / 32 * 32, H: (h + 31) / 32 * 32}
	in.d = make([]float32, 3*in.W*in.H)
	dw, dh := w, h // размеры уменьшенного исходника до поворота
	if q%2 == 1 {
		dw, dh = h, w
	}
	plane := in.W * in.H
	boxResample(img, dw, dh, func(x, y int, r, g, bl float32) {
		x, y = rotatePoint(x, y, dw, dh, q)
		i := y*in.W + x
		in.d[i], in.d[plane+i], in.d[2*plane+i] = bl, g, r // порядок BGR, как у модели
	})
	return in, true
}

// rotatePoint переносит пиксель (x, y) картинки w×h в повёрнутую на q четвертей по часовой.
func rotatePoint(x, y, w, h, q int) (int, int) {
	switch q {
	case 1:
		return h - 1 - y, x
	case 2:
		return w - 1 - x, h - 1 - y
	case 3:
		return y, w - 1 - x
	}
	return x, y
}

// span — область источника, из которой собирается один выходной пиксель вдоль оси: целые
// отсчёты [i0, i1), вес первого и последнего (середина — единицы), общий вес для нормировки.
type span struct {
	i0, i1 int
	wl, wr float64
	norm   float64
}

// spans делит ось из n отсчётов на m равных частей; части берут отсчёты с долями на стыках.
func spans(n, m int) []span {
	out := make([]span, m)
	step := float64(n) / float64(m)
	for o := range out {
		lo, hi := float64(o)*step, min(float64(n), float64(o+1)*step)
		i0, i1 := int(lo), min(n, int(math.Ceil(hi)))
		i1 = max(i1, i0+1)
		s := span{i0: i0, i1: i1, wl: float64(i0+1) - lo, wr: hi - float64(i1-1), norm: hi - lo}
		if i1-i0 == 1 { // один отсчёт: вес — вся длина части
			s.wl, s.wr = hi-lo, 0
		}
		out[o] = s
	}
	return out
}

// boxResample усредняет картинку по площади до dw×dh и отдаёт пиксели по строкам. Увеличение
// даёт то же, что ближайший сосед. Память — две строки, не картинка.
func boxResample(img image.Image, dw, dh int, put func(x, y int, r, g, b float32)) {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	xs, ys := spans(sw, dw), spans(sh, dh)
	row := make([]uint8, 3*sw)
	hrow := make([]float32, 3*dw)
	acc := make([]float32, 3*dw)
	cached := -1
	for oy, ysp := range ys {
		clear(acc)
		for y := ysp.i0; y < ysp.i1; y++ {
			if y != cached {
				readRow(img, b, y, row)
				hreduce(row, xs, hrow)
				cached = y
			}
			wy := float32(1)
			switch {
			case ysp.i1-ysp.i0 == 1:
				wy = float32(ysp.norm)
			case y == ysp.i0:
				wy = float32(ysp.wl)
			case y == ysp.i1-1:
				wy = float32(ysp.wr)
			}
			for i, v := range hrow {
				acc[i] += wy * v
			}
		}
		inv := float32(1 / ysp.norm)
		for ox := range dw {
			put(ox, oy, acc[3*ox]*inv, acc[3*ox+1]*inv, acc[3*ox+2]*inv)
		}
	}
}

// hreduce усредняет строку RGB по частям xs: результат — среднее (0…255), по три значения на часть.
func hreduce(row []uint8, xs []span, out []float32) {
	for ox, s := range xs {
		var r, g, b float32
		for x := s.i0; x < s.i1; x++ {
			w := float32(1)
			switch {
			case s.i1-s.i0 == 1:
				w = float32(s.wl)
			case x == s.i0:
				w = float32(s.wl)
			case x == s.i1-1:
				w = float32(s.wr)
			}
			r += w * float32(row[3*x])
			g += w * float32(row[3*x+1])
			b += w * float32(row[3*x+2])
		}
		inv := float32(1 / s.norm)
		out[3*ox], out[3*ox+1], out[3*ox+2] = r*inv, g*inv, b*inv
	}
}

// readRow читает строку y (от верха картинки) в RGB по 8 бит. Быстрые пути — для того, что отдают
// декодеры: JPEG (YCbCr), RGBA/NRGBA, Gray; остальное (CMYK, 16 бит, палитра) — через At.
func readRow(img image.Image, b image.Rectangle, y int, row []uint8) {
	ay := b.Min.Y + y
	switch m := img.(type) {
	case *image.YCbCr:
		for x := range b.Dx() {
			ax := b.Min.X + x
			r, g, bl := color.YCbCrToRGB(m.Y[m.YOffset(ax, ay)], m.Cb[m.COffset(ax, ay)], m.Cr[m.COffset(ax, ay)])
			row[3*x], row[3*x+1], row[3*x+2] = r, g, bl
		}
	case *image.RGBA:
		o := m.PixOffset(b.Min.X, ay)
		for x := range b.Dx() {
			row[3*x], row[3*x+1], row[3*x+2] = m.Pix[o+4*x], m.Pix[o+4*x+1], m.Pix[o+4*x+2]
		}
	case *image.Gray:
		o := m.PixOffset(b.Min.X, ay)
		for x := range b.Dx() {
			v := m.Pix[o+x]
			row[3*x], row[3*x+1], row[3*x+2] = v, v, v
		}
	default:
		for x := range b.Dx() {
			r, g, bl, _ := img.At(b.Min.X+x, ay).RGBA()
			row[3*x], row[3*x+1], row[3*x+2] = to8(r), to8(g), to8(bl)
		}
	}
}

// to8 — 16-битный канал (0…65535) в 8 бит.
func to8(v uint32) uint8 {
	if v > 0xffff {
		return 255
	}
	return uint8(v >> 8)
}
