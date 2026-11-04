package gen

import (
	"image"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

// Картинка-рамочка (079 design §3.1): окно ищется заливкой прозрачного от центра на уменьшенной
// копии, 9-срез кладёт готовую (уже уменьшенную вызывающим) картинку в рамку места. Только
// x/image: Lanczos-уменьшение (imaging) — у вызывающих.

const (
	winAnalyzePx = 1024 // длинная сторона копии для разбора
	winAlphaMin  = 128  // прозрачное — альфа копии меньше
	winMinArea   = 0.05 // окно меньше этой доли площади — отказ
)

// winRegion — область окна на копии: прозрачная связная часть от центра.
type winRegion struct {
	w, h int
	fill []uint8         // 1 — пиксель области
	n    int             // пикселей в области
	box  image.Rectangle // габарит области (Max исключён)
}

// alphaCopy — альфа картинки на копии ≤ winAnalyzePx по длинной стороне: среднее по блоку исходных
// пикселей (при сильном уменьшении тонкий просвет не теряется, как при выборке билинейной).
// Один проход по исходнику; быстрые пути *image.NRGBA/RGBA, остальное — через At.
func alphaCopy(img image.Image) (a []uint8, w, h int) {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw <= 0 || ih <= 0 {
		return nil, 0, 0
	}
	long := max(iw, ih)
	w, h = iw, ih
	if long > winAnalyzePx {
		w = max((iw*winAnalyzePx+long/2)/long, 1)
		h = max((ih*winAnalyzePx+long/2)/long, 1)
	}
	if w == iw && h == ih {
		a = make([]uint8, w*h)
		rowAlpha(img, b, func(y int, row []uint8) { copy(a[y*w:], row) })
		return a, w, h
	}
	sum := make([]uint32, w*h)
	cx, cy := make([]uint32, w), make([]uint32, h)
	xmap := make([]int, iw)
	for x := range iw {
		xmap[x] = x * w / iw
		cx[xmap[x]]++
	}
	for y := range ih {
		cy[y*h/ih]++
	}
	rowAlpha(img, b, func(y int, row []uint8) {
		s := sum[y*h/ih*w:][:w]
		for x, v := range row {
			s[xmap[x]] += uint32(v)
		}
	})
	a = make([]uint8, w*h)
	for y := range h {
		for x := range w {
			n := cx[x] * cy[y]
			a[y*w+x] = uint8((sum[y*w+x] + n/2) / n) //nolint:gosec // среднее восьмибитных значений ≤ 255
		}
	}
	return a, w, h
}

// rowAlpha зовёт fn с альфой каждой строки картинки (y от 0, слайс переиспользуется).
func rowAlpha(img image.Image, b image.Rectangle, fn func(y int, row []uint8)) {
	row := make([]uint8, b.Dx())
	for y := range b.Dy() {
		py := b.Min.Y + y
		switch m := img.(type) {
		case *image.NRGBA:
			off := m.PixOffset(b.Min.X, py)
			for x := range row {
				row[x] = m.Pix[off+4*x+3]
			}
		case *image.RGBA:
			off := m.PixOffset(b.Min.X, py)
			for x := range row {
				row[x] = m.Pix[off+4*x+3]
			}
		default:
			for x := range row {
				_, _, _, al := img.At(b.Min.X+x, py).RGBA()
				row[x] = uint8(al >> 8) //nolint:gosec // альфа 16 бит → старший байт ≤ 255
			}
		}
		fn(y, row)
	}
}

// openWindow — область окна: заливка 4-связная построчная (отрезками) прозрачного от центра;
// память — маска 1 байт на пиксель и стек отрезков. Центр непрозрачен — n = 0.
func openWindow(img image.Image) winRegion {
	a, w, h := alphaCopy(img)
	rg := winRegion{w: w, h: h}
	if w == 0 {
		return rg
	}
	tr := a // маска на месте альфы: 1 — прозрачное и не залито, 2 — залито
	for i, v := range a {
		if v < winAlphaMin {
			tr[i] = 1
		} else {
			tr[i] = 0
		}
	}
	cx, cy := w/2, h/2
	if tr[cy*w+cx] != 1 {
		return rg
	}
	rg.box = image.Rect(cx, cy, cx+1, cy+1)
	stack := []image.Point{{X: cx, Y: cy}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if tr[p.Y*w+p.X] != 1 {
			continue
		}
		l, r := fillSpan(tr, w, p)
		rg.n += r - l + 1
		rg.box = rg.box.Union(image.Rect(l, p.Y, r+1, p.Y+1))
		for _, y := range [2]int{p.Y - 1, p.Y + 1} {
			if y >= 0 && y < h {
				stack = seedRow(stack, tr, w, y, l, r)
			}
		}
	}
	rg.fill = tr
	return rg
}

// fillSpan заливает (2) отрезок строки вокруг p по прозрачным (1); возвращает его концы.
func fillSpan(tr []uint8, w int, p image.Point) (l, r int) {
	row := tr[p.Y*w : p.Y*w+w]
	l, r = p.X, p.X
	for l > 0 && row[l-1] == 1 {
		l--
	}
	for r < w-1 && row[r+1] == 1 {
		r++
	}
	for x := l; x <= r; x++ {
		row[x] = 2
	}
	return l, r
}

// seedRow кладёт в стек начало каждого прозрачного отрезка строки y под или над отрезком l…r.
func seedRow(stack []image.Point, tr []uint8, w, y, l, r int) []image.Point {
	row := tr[y*w : y*w+w]
	for x := l; x <= r; x++ {
		if row[x] == 1 && (x == l || row[x-1] != 1) {
			stack = append(stack, image.Point{X: x, Y: y})
		}
	}
	return stack
}

// FrameWindow — окно картинки-рамочки: cut — доли l, t, r, b ширины и высоты, где кончается рамка
// и начинается окно; open — область касается края (уголки, завитки: окно = всё место, срез —
// трети); ok = false — окна нет (центр непрозрачен) или оно меньше 5 % площади. Срез отдаётся
// уже через ClampCut (079-25): импорт и Load дают одну геометрию; окно у края шире прозрачной
// области, видимое режет WindowMask.
func FrameWindow(img image.Image) (cut [4]float64, open, ok bool) {
	rg := openWindow(img)
	if rg.n == 0 || float64(rg.n) < winMinArea*float64(rg.w*rg.h) {
		return cut, false, false
	}
	b := rg.box
	if b.Min.X == 0 || b.Min.Y == 0 || b.Max.X == rg.w || b.Max.Y == rg.h {
		return ClampCut([4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}), true, true
	}
	w, h := float64(rg.w), float64(rg.h)
	return ClampCut([4]float64{
		float64(b.Min.X) / w, float64(b.Min.Y) / h,
		1 - float64(b.Max.X)/w, 1 - float64(b.Max.Y)/h,
	}), false, true
}

// WindowMask — маска окна на той же копии ≤ 1024 px: область окна, обрезанная прямоугольником
// среза (центральной частью) и расширенная на 1 px, чтобы край окна ушёл под непрозрачную рамку без
// шва; 255 — окно. На печать растягивается вызывающим билинейно. Нет окна — пустая маска; нет
// картинки — nil.
func WindowMask(img image.Image, cut [4]float64) *image.Alpha {
	rg := openWindow(img)
	if rg.w == 0 {
		return nil
	}
	cut = ClampCut(cut)
	m := image.NewAlpha(image.Rect(0, 0, rg.w, rg.h))
	if rg.n == 0 {
		return m
	}
	w, h := float64(rg.w), float64(rg.h)
	keep := image.Rect(int(cut[0]*w+0.5), int(cut[1]*h+0.5), rg.w-int(cut[2]*w+0.5), rg.h-int(cut[3]*h+0.5))
	for y := max(keep.Min.Y, 0); y < min(keep.Max.Y, rg.h); y++ {
		for x := max(keep.Min.X, 0); x < min(keep.Max.X, rg.w); x++ {
			if rg.fill[y*rg.w+x] != 2 {
				continue
			}
			m.Pix[y*m.Stride+x] = 255
			for _, d := range [4]image.Point{{X: -1}, {X: 1}, {Y: -1}, {Y: 1}} {
				if q := (image.Point{X: x + d.X, Y: y + d.Y}); q.In(m.Rect) {
					m.Pix[q.Y*m.Stride+q.X] = 255
				}
			}
		}
	}
	return m
}

// Nine кладёт src в dr холста dst 9-срезом: sl — толщины l, t, r, b рамки в px src. Углы идут 1:1
// (draw.Draw), стороны и середина тянутся по своей оси билинейно; src уже в нужном масштабе
// (уменьшает вызывающий). Размер dr равен размеру src — один draw.Draw (та же пропорция = масштаб).
// Срез не больше src; если dr меньше суммы толщин оси, углы сжимаются пропорционально.
func Nine(dst *image.RGBA, dr image.Rectangle, src image.Image, sl [4]int) {
	NineTo(dst, dr, src, sl, sl)
}

// NineTo — Nine, но толщины рамки в dr заданы отдельно: dsl, px dst. Нужно, когда src уменьшен
// сильнее масштаба места (предел площади копии, 079-40): углы растягиваются до dsl, и рамка
// доходит до окна фото, которое считает модель (k·срез), без кольца страницы между ними.
func NineTo(dst *image.RGBA, dr image.Rectangle, src image.Image, sl, dsl [4]int) {
	sb := src.Bounds()
	if dst == nil || dr.Empty() || sb.Empty() {
		return
	}
	if dr.Size() == sb.Size() && sl == dsl {
		draw.Draw(dst, dr, src, sb.Min, draw.Over)
		return
	}
	sw, sh := sb.Dx(), sb.Dy()
	l, r := fitSlice(sw, sl[0], sl[2])
	t, bt := fitSlice(sh, sl[1], sl[3])
	if sl == dsl {
		dsl = [4]int{l, t, r, bt}
	}
	dl, drr := fitSlice(dr.Dx(), dsl[0], dsl[2])
	dt, dbt := fitSlice(dr.Dy(), dsl[1], dsl[3])
	sx := [4]int{sb.Min.X, sb.Min.X + l, sb.Max.X - r, sb.Max.X}
	sy := [4]int{sb.Min.Y, sb.Min.Y + t, sb.Max.Y - bt, sb.Max.Y}
	dx := [4]int{dr.Min.X, dr.Min.X + dl, dr.Max.X - drr, dr.Max.X}
	dy := [4]int{dr.Min.Y, dr.Min.Y + dt, dr.Max.Y - dbt, dr.Max.Y}
	for j := range 3 {
		for i := range 3 {
			s := image.Rect(sx[i], sy[j], sx[i+1], sy[j+1])
			d := image.Rect(dx[i], dy[j], dx[i+1], dy[j+1])
			switch {
			case s.Empty() || d.Empty():
			case s.Size() == d.Size():
				draw.Draw(dst, d, src, s.Min, draw.Over)
			default:
				xdraw.ApproxBiLinear.Scale(dst, d, src, s, xdraw.Over, nil)
			}
		}
	}
}

// SlicePx — толщины рамки l, t, r, b в px у картинки w×h px со срезом cut (доли сторон).
func SlicePx(cut [4]float64, w, h float64) [4]int {
	return [4]int{int(math.Round(cut[0] * w)), int(math.Round(cut[1] * h)), int(math.Round(cut[2] * w)), int(math.Round(cut[3] * h))}
}

// fitSlice — две толщины оси длиной total: неотрицательные, в сумме не больше total (иначе
// сжимаются пропорционально).
func fitSlice(total, a, b int) (int, int) {
	a, b = max(a, 0), max(b, 0)
	if a+b <= total {
		return a, b
	}
	a = total * a / (a + b)
	return a, total - a
}
