package gen

import (
	"bytes"
	"image"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"golang.org/x/image/math/f64"
)

// artKinds — набор стикеров research §2 (30 ключей); выпущенный ключ из набора не пропадает.
var artKinds = []string{
	"star", "burst", "sparkle", "heart", "hearts-pair", "circle", "blob", "plate", "banner", "wave-band",
	"rainbow", "arches", "corner", "brush-stroke", "arrow", "confetti-pile", "flower", "daisy", "leaf",
	"branch", "sun", "cloud", "balloon", "balloons", "cap", "pencil", "book", "apple", "bell", "rings",
}

// TestGenArtSet — набор стикеров на месте, у каждого пропорция и цвета; растяжимые — ровно
// плашка и волны (решение №136); IsArt и Known согласованы, узор стикером не считается.
func TestGenArtSet(t *testing.T) {
	arts := Arts()
	if len(arts) != len(artKinds) {
		t.Fatalf("стикеров %d, ждали %d", len(arts), len(artKinds))
	}
	for i, m := range arts {
		if m.Kind != artKinds[i] || !IsArt(m.Kind) || !Known(m.Kind) || m.Aspect < MinAspect || m.Aspect > MaxAspect {
			t.Errorf("%d: %+v", i, m)
		}
		if len(m.Inks) < 1 || len(m.Inks) > 4 || m.Paper {
			t.Errorf("%s: цветов %d", m.Kind, len(m.Inks))
		}
		if m.Stretch != (m.Kind == "plate" || m.Kind == "wave-band") {
			t.Errorf("%s: Stretch = %v", m.Kind, m.Stretch)
		}
	}
	for _, p := range Patterns() {
		if IsArt(p.Kind) {
			t.Errorf("узор %s назван стикером", p.Kind)
		}
	}
}

// artImage рисует стикер в рамку w×h px, окружённую полем pad.
func artImage(kind string, seed uint32, aspect float64, w, h, pad int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	DrawArt(img, f64.Aff3{float64(w), 0, float64(pad), 0, float64(h), float64(pad)}, Spec{Kind: kind, Seed: seed, Aspect: aspect}, 0)
	return img
}

// TestGenArtInFrame — каждый стикер в своей и в крайних рамках (растяжимый — тонкой, квадратной,
// длинной): рисунок не пуст и не выходит за рамку.
func TestGenArtInFrame(t *testing.T) {
	const pad = 12
	for _, m := range Arts() {
		frames := [][2]int{{int(100 * m.Aspect), 100}}
		if m.Aspect < 1 {
			frames = [][2]int{{100, int(100 / m.Aspect)}}
		}
		if m.Stretch {
			frames = append(frames, [2]int{300, 30}, [2]int{100, 100}, [2]int{40, 200})
		}
		for _, f := range frames {
			img := artImage(m.Kind, 5, float64(f[0])/float64(f[1]), f[0], f[1], pad)
			inner := image.Rect(pad, pad, pad+f[0], pad+f[1])
			opaque := 0
			for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
				for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
					if img.RGBAAt(x, y).A == 0 {
						continue
					}
					if !(image.Point{X: x, Y: y}.In(inner)) {
						t.Fatalf("%s %v: пиксель (%d,%d) вне рамки", m.Kind, f, x, y)
					}
					opaque++
				}
			}
			if opaque < f[0]*f[1]/40 {
				t.Errorf("%s %v: закрашено %d px из %d — пусто", m.Kind, f, opaque, f[0]*f[1])
			}
		}
	}
}

// TestGenArtSeed — у «Перемешать» другой сид — другая картинка, у остальных сид зажим обнуляет.
func TestGenArtSeed(t *testing.T) {
	for _, m := range Arts() {
		a, b := artImage(m.Kind, 1, 0, 120, 120, 0), artImage(m.Kind, 2, 0, 120, 120, 0)
		same := bytes.Equal(a.Pix, b.Pix)
		if m.Random == same {
			t.Errorf("%s: Random = %v, картинки совпадают = %v", m.Kind, m.Random, same)
		}
		if !m.Random && Clamp(Spec{Kind: m.Kind, Seed: 9}).Seed != 0 {
			t.Errorf("%s: сид не обнулён", m.Kind)
		}
		if !bytes.Equal(a.Pix, artImage(m.Kind, 1, 0, 120, 120, 0).Pix) {
			t.Errorf("%s: рисунок недетерминирован", m.Kind)
		}
	}
}

// TestGenArtFade — прозрачность кладёт рисунок одним слоем: перекрытия фигур (взрыв, цветок)
// не темнеют, альфа везде ≤ 1 − fade; fade 1 — ничего.
func TestGenArtFade(t *testing.T) {
	for _, kind := range []string{"burst", "flower", "balloons", "confetti-pile"} {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		DrawArt(img, f64.Aff3{100, 0, 0, 0, 100, 0}, Spec{Kind: kind}, 0.5)
		mx := uint8(0)
		for i := 3; i < len(img.Pix); i += 4 {
			mx = max(mx, img.Pix[i])
		}
		if mx < 120 || mx > 128 {
			t.Errorf("%s: наибольшая альфа %d, ждали ≈ 127", kind, mx)
		}
		full := image.NewRGBA(image.Rect(0, 0, 100, 100))
		DrawArt(full, f64.Aff3{100, 0, 0, 0, 100, 0}, Spec{Kind: kind}, 1)
		if !bytes.Equal(full.Pix, make([]byte, len(full.Pix))) {
			t.Errorf("%s: fade 1 что-то нарисовал", kind)
		}
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestGenArtMirror — зеркальная матрица даёт зеркальную картинку (↔ в просмотре и экспорте).
func TestGenArtMirror(t *testing.T) {
	const w = 120
	for _, m := range Arts() {
		h := int(float64(w) / m.Aspect)
		if h < 1 || h > 400 {
			continue
		}
		spec := Spec{Kind: m.Kind, Seed: 3}
		a, b := image.NewRGBA(image.Rect(0, 0, w, h)), image.NewRGBA(image.Rect(0, 0, w, h))
		DrawArt(a, f64.Aff3{w, 0, 0, 0, float64(h), 0}, spec, 0)
		DrawArt(b, f64.Aff3{-w, 0, w, 0, float64(h), 0}, spec, 0)
		bad := 0
		for y := range h {
			for x := range w {
				p, q := a.RGBAAt(x, y), b.RGBAAt(w-1-x, y)
				if absDiff(p.A, q.A) > 4 || absDiff(p.R, q.R) > 4 || absDiff(p.G, q.G) > 4 || absDiff(p.B, q.B) > 4 {
					bad++
				}
			}
		}
		if bad > w*h/200 {
			t.Errorf("%s: зеркало расходится в %d px", m.Kind, bad)
		}
	}
}

// TestGenArtStretch — растяжимые рисуются заново в рамке, а не растягиваются (спека 049):
// скругление плашки и размах волны — от короткой стороны, поэтому левая часть рамок 3:1 и 6:1
// одной высоты совпадает.
func TestGenArtStretch(t *testing.T) {
	for _, kind := range []string{"plate", "wave-band"} {
		a := Request{Art: true, Spec: Spec{Kind: kind, Aspect: 3}, Px: 300}.Draw()
		b := Request{Art: true, Spec: Spec{Kind: kind, Aspect: 6}, Px: 600}.Draw()
		if a.Rect.Dy() != 100 || b.Rect.Dy() != 100 {
			t.Fatalf("%s: высоты %d и %d", kind, a.Rect.Dy(), b.Rect.Dy())
		}
		bad := 0
		for y := range 100 {
			for x := range 90 {
				if p, q := a.RGBAAt(x, y), b.RGBAAt(x, y); absDiff(p.A, q.A) > 3 || absDiff(p.R, q.R) > 3 {
					bad++
				}
			}
		}
		if bad > 20 {
			t.Errorf("%s: левая часть 3:1 и 6:1 расходится в %d px — рисунок растянут", kind, bad)
		}
	}
	// Пропорция по умолчанию — из Meta.
	m, _ := Lookup("plate")
	w, h := Request{Art: true, Spec: Spec{Kind: "plate"}, Px: 300}.Size()
	if r := float64(w) / float64(h); r < m.Aspect-0.05 || r > m.Aspect+0.05 {
		t.Errorf("plate по умолчанию %d×%d", w, h)
	}
}

// TestGenArtQuery — круг Query → Parse у каждого стикера (растяжимый — с пропорцией «a»);
// крайние пропорции растяжимого не дают пустой картинки.
func TestGenArtQuery(t *testing.T) {
	for _, m := range Arts() {
		s := Spec{Kind: m.Kind, Inks: []string{"#102030"}, Seed: 42}
		if m.Stretch {
			s.Aspect = 2.5
		}
		path, raw, _ := strings.Cut(Query(s, 0, 0), "?")
		if path != artPath {
			t.Fatalf("%s: путь %s", m.Kind, path)
		}
		req, ok := Parse(path, raw+"&px=256")
		if !ok || !req.Art || !reflect.DeepEqual(req.Spec, Clamp(s)) {
			t.Errorf("%s: %s → %+v %v", m.Kind, raw, req, ok)
			continue
		}
		if m.Stretch && !strings.Contains(raw, "a=2.5") {
			t.Errorf("%s: нет пропорции: %s", m.Kind, raw)
		}
		w, h := req.Size()
		if max(w, h) != 256 || w < 1 || h < 1 {
			t.Errorf("%s: размер %d×%d", m.Kind, w, h)
		}
	}
	for _, a := range []float64{MinAspect, MaxAspect} {
		req := Request{Art: true, Spec: Spec{Kind: "wave-band", Aspect: a}, Px: 4096}
		if w, h := req.Size(); w < 1 || h < 1 || max(w, h) != 4096 {
			t.Errorf("a=%v: %d×%d", a, w, h)
		}
	}
}

// TestGenArtFast — плитка стикера 160 px — Limit(10 ms); стикер 100 мм при 300 dpi (1181 px) —
// Limit(150 ms) (замер аналитика: 6–21 мс).
func TestGenArtFast(t *testing.T) {
	if testing.Short() || testkit.Race {
		t.Skip("замер времени: обычный прогон без -short и -race")
	}
	for _, m := range Arts() {
		tile := image.NewRGBA(image.Rect(0, 0, 160, 160))
		m2 := f64.Aff3{160, 0, 0, 0, 160, 0}
		if best := testkit.Fastest(5, func() { DrawArt(tile, m2, Spec{Kind: m.Kind}, 0) }); best > testkit.Limit(10*time.Millisecond) {
			t.Errorf("%s: плитка %v", m.Kind, best)
		}
		big := image.NewRGBA(image.Rect(0, 0, 1181, 1181))
		m3 := f64.Aff3{1181, 0, 0, 0, 1181, 0}
		if best := testkit.Fastest(3, func() { DrawArt(big, m3, Spec{Kind: m.Kind}, 0) }); best > testkit.Limit(150*time.Millisecond) {
			t.Errorf("%s: 100 мм при 300 dpi %v", m.Kind, best)
		}
	}
}
