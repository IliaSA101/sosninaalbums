package gen

import (
	"image"
	"math"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/image/math/f64"
)

// Запрос и рисование /media/gen/frame (079 design §5): накладка встроенной рамочки на место
// w×h мм, её маска (m=1, только Mask-вид) и плитка каталога (t=1). Адрес строит Go (View и
// каталог), разбор — здесь же, чтобы они не разошлись; UI дописывает &px=. Ответ неизменен:
// вид заморожен эталоном (TestFrameGolden).

// FramePath — путь накладки встроенной рамочки.
const FramePath = "/media/gen/frame"

// Плитка каталога: рамочка вокруг серого силуэта 3:4 в квадрате. Место — 60×80 мм, чтобы толщина
// в мм читалась так же, как на странице; занимает 84 % стороны квадрата.
const (
	tilePlaceW = 60.0
	tilePlaceH = 80.0
	tileFill   = 0.84
)

// FrameRequest — разобранный запрос /media/gen/frame. Spec — после ClampFrame; у плитки размер
// места и форма не используются.
type FrameRequest struct {
	Spec FrameSpec
	Px   int // ширина накладки; у плитки — сторона квадрата
	Mask bool
	Tile bool
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// inkList — цвета через запятую, без «#».
func inkList(inks []string) string {
	hex := make([]string, len(inks))
	for i, v := range inks {
		hex[i] = strings.TrimPrefix(v, "#")
	}
	return strings.Join(hex, ",")
}

// FrameQuery — адрес накладки без px: рамочка s на месте s.W×s.H; mask — маска окна (soft).
// Вид должен быть известен (иначе разбор откажет). Пустое и по умолчанию не пишется.
func FrameQuery(s FrameSpec, mask bool) string {
	s = ClampFrame(s)
	q := []string{"k=" + s.Kind}
	if len(s.Inks) > 0 {
		q = append(q, "i="+inkList(s.Inks))
	}
	if s.Width > 0 {
		q = append(q, "bw="+num(s.Width))
	}
	q = append(q, "w="+num(round3(s.W)), "h="+num(round3(s.H)))
	switch {
	case s.Oval:
		q = append(q, "o=1")
	case s.Radius > 0:
		q = append(q, "rd="+num(round3(s.Radius)))
	}
	if mask {
		q = append(q, "m=1")
	}
	q = append(q, "v="+strconv.Itoa(Version))
	return FramePath + "?" + strings.Join(q, "&")
}

// TileQuery — адрес плитки каталога без px: вид, цвета и толщина.
func TileQuery(kind string, inks []string, width float64) string {
	s := ClampFrame(FrameSpec{Kind: kind, Inks: inks, Width: width})
	q := []string{"k=" + s.Kind}
	if len(s.Inks) > 0 {
		q = append(q, "i="+inkList(s.Inks))
	}
	if s.Width > 0 {
		q = append(q, "bw="+num(s.Width))
	}
	q = append(q, "t=1", "v="+strconv.Itoa(Version))
	return FramePath + "?" + strings.Join(q, "&")
}

// oneValues — значения запроса по одному; повтор, лишний ключ и неразборный запрос — false.
func oneValues(raw string, allowed map[string]bool) (map[string]string, bool) {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return nil, false
	}
	out := make(map[string]string, len(vals))
	for k, v := range vals {
		if !allowed[k] || len(v) != 1 {
			return nil, false
		}
		out[k] = v[0]
	}
	return out, true
}

// numIn — число из строки: конечное, в lo…hi.
func numIn(s string, lo, hi float64) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil && !math.IsNaN(f) && f >= lo && f <= hi
}

// pxV — обязательные px (16…4096) и v (uint16).
func pxV(m map[string]string) (int, bool) {
	n, err := strconv.Atoi(m["px"])
	_, errV := strconv.ParseUint(m["v"], 10, 16)
	return n, err == nil && errV == nil && n >= MinPx && n <= MaxPx
}

// onFlag — необязательный ключ-флаг: нет — false, «1» — true; другое значение — bad.
func onFlag(m map[string]string, key string) (on, bad bool) {
	v, has := m[key]
	return has && v == "1", has && v != "1"
}

// optNum — необязательное число ключа key в lo…hi; нет ключа — dst не трогается, не читается — false.
func optNum(m map[string]string, key string, dst *float64, lo, hi float64) bool {
	v, has := m[key]
	if !has {
		return true
	}
	f, ok := numIn(v, lo, hi)
	*dst = f
	return ok
}

// ParseFrame разбирает строку запроса /media/gen/frame. ok = false (404): лишний, повторённый
// или недостающий параметр, число не читается, вид неизвестен, o/m/t не «1», маска у вида без
// маски, px вне 16…4096, накладка выше 4096 px. Значения в пределах зажимает ClampFrame.
func ParseFrame(raw string) (FrameRequest, bool) {
	var req FrameRequest
	m, ok := oneValues(raw, map[string]bool{"k": true, "i": true, "bw": true, "w": true, "h": true, "o": true,
		"rd": true, "m": true, "t": true, "px": true, "v": true})
	if !ok {
		return req, false
	}
	px, ok := pxV(m)
	if !ok || !FrameKnown(m["k"]) {
		return req, false
	}
	req.Px, req.Spec.Kind = px, m["k"]
	var badM, badT bool
	req.Mask, badM = onFlag(m, "m")
	req.Tile, badT = onFlag(m, "t")
	if badM || badT || !req.parseInks(m) || !optNum(m, "bw", &req.Spec.Width, 0, 1e6) {
		return req, false
	}
	if req.Tile {
		return req.finishTile(m)
	}
	return req.finishOverlay(m)
}

// parseInks читает i — до MaxFrameInks цветов #rrggbb без «#».
func (r *FrameRequest) parseInks(m map[string]string) bool {
	v, has := m["i"]
	if !has {
		return true
	}
	for c := range strings.SplitSeq(v, ",") {
		ink, good := normInk("#" + c)
		if !good || len(r.Spec.Inks) == MaxFrameInks {
			return false
		}
		r.Spec.Inks = append(r.Spec.Inks, ink)
	}
	return true
}

// finishTile — у плитки нет размера места, формы и маски (soft в ней — сам силуэт).
func (r FrameRequest) finishTile(m map[string]string) (FrameRequest, bool) {
	for _, k := range [...]string{"w", "h", "o", "rd"} {
		if _, has := m[k]; has {
			return r, false
		}
	}
	r.Spec = ClampFrame(r.Spec)
	return r, !r.Mask
}

// finishOverlay — размер места и форма накладки или маски; маска — только у вида с маской.
func (r FrameRequest) finishOverlay(m map[string]string) (FrameRequest, bool) {
	oval, badO := onFlag(m, "o")
	r.Spec.Oval = oval
	if badO || !optNum(m, "rd", &r.Spec.Radius, 0, 1e6) {
		return r, false
	}
	var ok bool
	if r.Spec.W, ok = numIn(m["w"], 0.01, maxFrameMM); !ok {
		return r, false
	}
	if r.Spec.H, ok = numIn(m["h"], 0.01, maxFrameMM); !ok {
		return r, false
	}
	r.Spec = ClampFrame(r.Spec)
	if meta, _ := FrameLookup(r.Spec.Kind); r.Mask && !meta.Mask {
		return r, false
	}
	_, h := r.Size()
	return r, h >= 1 && h <= MaxPx
}

// Size — размер картинки ответа, px: накладка и маска — Px × Px·H/W, плитка — квадрат.
func (r FrameRequest) Size() (int, int) {
	if r.Tile {
		return r.Px, r.Px
	}
	if !(r.Spec.W > 0) {
		return 0, 0
	}
	return r.Px, max(1, int(math.Round(float64(r.Px)*r.Spec.H/r.Spec.W)))
}

// Draw рисует ответ: накладка (прозрачный холст), маска (альфа окна) или плитка.
func (r FrameRequest) Draw() image.Image {
	switch {
	case r.Tile:
		return r.drawTile()
	case r.Mask:
		return r.drawMask()
	}
	w, h := r.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	DrawFrame(img, f64.Aff3{float64(w) / r.Spec.W, 0, 0, 0, float64(h) / r.Spec.H, 0}, r.Spec)
	return img
}

// drawMask — альфа окна Mask-вида как PNG: цвет чёрный, прозрачность — маска (CSS mask-image).
func (r FrameRequest) drawMask() *image.NRGBA {
	w, h := r.Size()
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	FrameMask(a, f64.Aff3{float64(w) / r.Spec.W, 0, 0, 0, float64(h) / r.Spec.H, 0}, r.Spec)
	out := image.NewNRGBA(a.Rect)
	for i, v := range a.Pix { // оба буфера плотные: у NewAlpha шаг равен ширине
		out.Pix[4*i+3] = v
	}
	return out
}

// drawTile — плитка: серый силуэт 3:4 в окне рамочки и сама рамочка поверх; у Mask-вида край
// силуэта растворяется.
func (r FrameRequest) drawTile() *image.RGBA {
	n := r.Px
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	s := r.Spec
	s.W, s.H, s.Oval, s.Radius = tilePlaceW, tilePlaceH, false, 0
	f, ok := newFrame(s)
	if !ok {
		return img
	}
	k := tileFill * float64(n) / tilePlaceH
	ox, oy := (float64(n)-tilePlaceW*k)/2, (float64(n)-tilePlaceH*k)/2
	win := image.Rect(
		int(math.Round(ox+f.ins[0]*k)), int(math.Round(oy+f.ins[1]*k)),
		int(math.Round(ox+(tilePlaceW-f.ins[2])*k)), int(math.Round(oy+(tilePlaceH-f.ins[3])*k)))
	silhouette(img, win)
	m := f64.Aff3{k, 0, ox, 0, k, oy}
	if f.meta.Mask {
		mask := image.NewAlpha(img.Rect)
		FrameMask(mask, m, s)
		for i, a := range mask.Pix {
			if a == 255 {
				continue
			}
			p := img.Pix[4*i : 4*i+4]
			for j := range p {
				p[j] = uint8((uint16(p[j])*uint16(a) + 127) / 255) //nolint:gosec // (≤255·255+127)/255 ≤ 255
			}
		}
		return img
	}
	DrawFrame(img, m, s)
	return img
}

// Тона силуэта плитки: фон и голова с плечами.
var (
	silBack = [3]uint8{0xd6, 0xd6, 0xd6}
	silFore = [3]uint8{0xae, 0xae, 0xae}
)

// silhouette — серый прямоугольник win с головой и плечами тоном темнее (покрытие по расстоянию:
// край без лесенки).
func silhouette(img *image.RGBA, win image.Rectangle) {
	win = win.Intersect(img.Rect)
	if win.Empty() {
		return
	}
	w, h := float64(win.Dx()), float64(win.Dy())
	hx, hy, hr := float64(win.Min.X)+w/2, float64(win.Min.Y)+h*0.38, min(w*0.2, h*0.15)
	sx, sy, rx, ry := hx, float64(win.Max.Y), w*0.4, h*0.3
	for y := win.Min.Y; y < win.Max.Y; y++ {
		for x := win.Min.X; x < win.Max.X; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			head := 0.5 - (math.Hypot(px-hx, py-hy) - hr)
			body := 0.5 - (math.Hypot((px-sx)/rx, (py-sy)/ry)-1)*min(rx, ry)
			c := min(max(max(head, body), 0), 1)
			o := img.PixOffset(x, y)
			for j := range 3 {
				back, fore := float64(silBack[j]), float64(silFore[j])
				img.Pix[o+j] = uint8(math.Round(back + (fore-back)*c))
			}
			img.Pix[o+3] = 255
		}
	}
}
