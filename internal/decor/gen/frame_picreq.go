package gen

import (
	"image"
	"math"
	"strconv"
	"strings"
)

// Запрос накладки и маски рамочки-картинки (079 design §5, 079-22): путь /media/f/<id> ведёт
// маршрут App, а строку запроса строит Go (View) и разбирает decor.ServeFramePic — формат один,
// здесь. Срез и open — из альбома, как их зажал ClampBorder; UI дописывает &px=.

// Размер картинки-рамочки (Border.Scale): 0,5…2 с шагом 0,01; в запросе не указан — 1.
const (
	MinPicScale = 0.5
	MaxPicScale = 2.0
)

// FramePicRequest — разобранный запрос. W, H — размер места, мм; Scale — размер картинки; Cut —
// срез l, t, r, b (доли); Open — рамочка без окна (маски нет); Mask — маска окна вместо накладки.
type FramePicRequest struct {
	W, H  float64
	Scale float64
	Cut   [4]float64
	Open  bool
	Mask  bool
	Px    int
}

// ClampPicScale — размер картинки: NaN и 0 — 1, иначе 0,5…2 с шагом 0,01.
func ClampPicScale(v float64) float64 {
	if math.IsNaN(v) || v == 0 {
		return 1
	}
	return math.Round(min(max(v, MinPicScale), MaxPicScale)*100) / 100
}

// FramePicQuery — строка запроса без px (и без «?»): место w×h мм, размер, срез, open, маска.
func FramePicQuery(r FramePicRequest) string {
	q := []string{"w=" + num(round3(r.W)), "h=" + num(round3(r.H))}
	if s := ClampPicScale(r.Scale); s != 1 {
		q = append(q, "s="+num(s))
	}
	c := ClampCut(r.Cut)
	q = append(q, "c="+num(c[0])+","+num(c[1])+","+num(c[2])+","+num(c[3]))
	if r.Open {
		q = append(q, "o=1")
	}
	if r.Mask {
		q = append(q, "m=1")
	}
	return strings.Join(append(q, "v="+strconv.Itoa(Version)), "&")
}

// ParseFramePic разбирает строку запроса. ok = false (404): лишний, повторённый или недостающий
// (w, h, px, v) параметр, число не читается или не конечное, o/m не «1», маска у рамочки без окна
// (o=1), px вне 16…4096, место вне 0,01…5000 мм, ответ выше 4096 px. Размер и срез в пределах
// зажимаются (ClampPicScale, ClampCut).
func ParseFramePic(raw string) (FramePicRequest, bool) {
	req := FramePicRequest{Scale: 1}
	m, ok := oneValues(raw, map[string]bool{"w": true, "h": true, "s": true, "c": true, "o": true, "m": true, "px": true, "v": true})
	if !ok {
		return req, false
	}
	if req.Px, ok = pxV(m); !ok {
		return req, false
	}
	if req.W, ok = numIn(m["w"], 0.01, maxFrameMM); !ok {
		return req, false
	}
	if req.H, ok = numIn(m["h"], 0.01, maxFrameMM); !ok {
		return req, false
	}
	if v, has := m["s"]; has {
		s, good := numIn(v, 0, 1e6)
		if !good {
			return req, false
		}
		req.Scale = ClampPicScale(s)
	}
	if v, has := m["c"]; has {
		parts := strings.Split(v, ",")
		if len(parts) != 4 {
			return req, false
		}
		for i, p := range parts {
			f, err := strconv.ParseFloat(p, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return req, false
			}
			req.Cut[i] = f
		}
		req.Cut = ClampCut(req.Cut)
	}
	var badO, badM bool
	req.Open, badO = onFlag(m, "o")
	req.Mask, badM = onFlag(m, "m")
	if badO || badM || req.Open && req.Mask {
		return req, false
	}
	_, h := req.Size()
	return req, h >= 1 && h <= MaxPx
}

// Size — размер ответа, px: Px × Px·H/W.
func (r FramePicRequest) Size() (int, int) {
	if !(r.W > 0) {
		return 0, 0
	}
	return r.Px, max(1, int(math.Round(float64(r.Px)*r.H/r.W)))
}

// ClampInsets — отступы окна l, t, r, b, мм: пара оси не больше 0,7 стороны, иначе сжимается
// пропорционально (geom.ClampInset; gen его не импортирует, TestFrameInsParity держит равенство).
func ClampInsets(ins [4]float64, w, h float64) [4]float64 { return clampIns(ins, w, h) }

// FrameInsets — отступы окна встроенной рамочки s, мм (накладка и неизвестный вид — нули).
func FrameInsets(s FrameSpec) [4]float64 {
	if f, ok := newFrame(s); ok {
		return f.ins
	}
	return [4]float64{}
}

// StretchWindow растягивает центральную часть маски окна m (WindowMask; срез cut) на win холста
// size и отдаёт чёрный NRGBA с альфой маски: вне win — 0. Билинейно, как задумано для печати
// (design §3.1). Кайма в 1 px, которой WindowMask заводит окно под непрозрачную рамочку, едет
// вместе с центром и ложится за край win (без неё между фото и рамочкой просвечивала бы страница).
// Нет окна, нет маски или пустое win — пустой холст.
func StretchWindow(m *image.Alpha, cut [4]float64, size image.Point, win image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, max(size.X, 0), max(size.Y, 0))) // чёрный: пишется только альфа
	stretchWin(alphaDst{pix: out.Pix, stride: out.Stride, bpp: 4, off: 3, rect: out.Rect}, m, cut, win)
	return out
}

// StretchWindowAlpha — StretchWindow сразу в Alpha (печать, 079-40): без NRGBA вчетверо больше
// и без копии в Alpha; байты альфы те же (TestStretchWindowAlpha).
func StretchWindowAlpha(m *image.Alpha, cut [4]float64, size image.Point, win image.Rectangle) *image.Alpha {
	out := image.NewAlpha(image.Rect(0, 0, max(size.X, 0), max(size.Y, 0)))
	stretchWin(alphaDst{pix: out.Pix, stride: out.Stride, bpp: 1, rect: out.Rect}, m, cut, win)
	return out
}

// alphaDst — альфа приёмника с нуля: байт пикселя (x, y) — pix[y·stride + x·bpp + off].
type alphaDst struct {
	pix              []uint8
	stride, bpp, off int
	rect             image.Rectangle
}

func (d alphaDst) at(x, y int) int { return y*d.stride + x*d.bpp + d.off }

// stretchWin — общее тело StretchWindow и StretchWindowAlpha.
func stretchWin(dst alphaDst, m *image.Alpha, cut [4]float64, win image.Rectangle) {
	win = win.Intersect(dst.rect)
	if m == nil || win.Empty() {
		return
	}
	cut = ClampCut(cut)
	mw, mh := m.Rect.Dx(), m.Rect.Dy()
	keep := image.Rect(int(cut[0]*float64(mw)+0.5), int(cut[1]*float64(mh)+0.5), mw-int(cut[2]*float64(mw)+0.5), mh-int(cut[3]*float64(mh)+0.5)).
		Intersect(image.Rect(0, 0, mw, mh))
	if keep.Empty() {
		return
	}
	// Источник — центр с каймой; приёмник — win, расширенный на ту же кайму в масштабе растяжения.
	sr := keep.Inset(-1).Intersect(image.Rect(0, 0, mw, mh))
	kx, ky := float64(win.Dx())/float64(keep.Dx()), float64(win.Dy())/float64(keep.Dy())
	dr := image.Rect(win.Min.X-int(math.Round(kx*float64(keep.Min.X-sr.Min.X))), win.Min.Y-int(math.Round(ky*float64(keep.Min.Y-sr.Min.Y))),
		win.Max.X+int(math.Round(kx*float64(sr.Max.X-keep.Max.X))), win.Max.Y+int(math.Round(ky*float64(sr.Max.Y-keep.Max.Y)))).Intersect(dst.rect)
	scaleAlpha(dst, dr, m, sr.Add(m.Rect.Min))
}

// scaleAlpha — альфа m в прямоугольнике sr (координаты m) на dr приёмника: формулы
// xdraw.ApproxBiLinear.Scale с op Src, по одному каналу (прежде маска шла через RGBA (0, 0, 0, a);
// сверка — TestStretchWindowAlpha). Равные размеры — копия, как у Scale.
func scaleAlpha(dst alphaDst, dr image.Rectangle, m *image.Alpha, sr image.Rectangle) {
	if dr.Empty() || sr.Empty() {
		return
	}
	if dr.Size() == sr.Size() {
		for y := range dr.Dy() {
			row := m.Pix[m.PixOffset(sr.Min.X, sr.Min.Y+y):][:sr.Dx()]
			for x, a := range row {
				dst.pix[dst.at(dr.Min.X+x, dr.Min.Y+y)] = a
			}
		}
		return
	}
	sw, sh := sr.Dx(), sr.Dy()
	xscale, yscale := float64(sw)/float64(dr.Dx()), float64(sh)/float64(dr.Dy())
	for dy := range dr.Dy() {
		sy0, sy1, yFrac0, yFrac1 := ablTap(dy, yscale, sh)
		r0 := m.Pix[m.PixOffset(sr.Min.X, sr.Min.Y+sy0):][:sw]
		r1 := m.Pix[m.PixOffset(sr.Min.X, sr.Min.Y+sy1):][:sw]
		d := dst.at(dr.Min.X, dr.Min.Y+dy)
		for dx := range dr.Dx() {
			sx0, sx1, xFrac0, xFrac1 := ablTap(dx, xscale, sw)
			s00, s10 := float64(uint32(r0[sx0])*0x101), float64(uint32(r0[sx1])*0x101)
			s01, s11 := float64(uint32(r1[sx0])*0x101), float64(uint32(r1[sx1])*0x101)
			// float64(…) у каждого произведения — как в x/image: без слияния в FMA результат тот же.
			s10 = float64(xFrac1*s00) + float64(xFrac0*s10)
			s11 = float64(xFrac1*s01) + float64(xFrac0*s11)
			s11 = float64(yFrac1*s10) + float64(yFrac0*s11)
			dst.pix[d] = hiByte(uint32(s11))
			d += dst.bpp
		}
	}
}

// hiByte — старший байт 16-битной альфы (v ≤ 0xffff), как uint8(v >> 8) в x/image.
func hiByte(v uint32) uint8 { return uint8(min(v>>8, 0xff)) }

// ablTap — два соседних отсчёта источника и их веса для пикселя i приёмника (ApproxBiLinear;
// там int32 — у маски стороны далеко от предела, результат тот же).
func ablTap(i int, scale float64, n int) (s0, s1 int, f0, f1 float64) {
	s := float64((float64(i)+0.5)*scale) - 0.5
	s0 = int(s) // s < 0 зажимается ниже, усечение вместо Floor не важно
	f0 = s - float64(s0)
	f1 = 1 - f0
	s1 = s0 + 1
	if s < 0 {
		s0, s1, f0, f1 = 0, 0, 0, 1
	} else if s1 > n-1 {
		s0, s1, f0, f1 = n-1, n-1, 1, 0
	}
	return s0, s1, f0, f1
}
