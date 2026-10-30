package gen

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

func render(kind string, seed uint32, wmm, hmm, k, fade float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, int(math.Round(wmm*k)), int(math.Round(hmm*k))))
	DrawPattern(img, img.Rect, Spec{Kind: kind, Seed: seed}, k, fade)
	return img
}

func opaqueCount(img *image.RGBA) int {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			n++
		}
	}
	return n
}

// TestGenDeterministic — тот же Spec → те же байты; у случайных другой сид — другой рисунок,
// у неслучайных сид не влияет; каждый узор что-то рисует.
func TestGenDeterministic(t *testing.T) {
	for _, m := range Patterns() {
		a := render(m.Kind, 3, 120, 80, 2, 0)
		b := render(m.Kind, 3, 120, 80, 2, 0)
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Errorf("%s: два прогона разошлись", m.Kind)
		}
		if opaqueCount(a) == 0 {
			t.Errorf("%s: пусто", m.Kind)
		}
		c := render(m.Kind, 4, 120, 80, 2, 0)
		if bytes.Equal(a.Pix, c.Pix) != !m.Random {
			t.Errorf("%s: сид влияет = %v, Random = %v", m.Kind, !bytes.Equal(a.Pix, c.Pix), m.Random)
		}
	}
}

// TestGenPreviewMatchesPrint — мотив по мм: плитка 1 px/мм и печать 5 px/мм, уменьшенная
// усреднением 5×5, совпадают (research §3: период, округлённый до px, сдвинул бы мотив на 6 %).
func TestGenPreviewMatchesPrint(t *testing.T) {
	const f = 5
	for _, m := range Patterns() {
		lo := render(m.Kind, 11, 160, 96, 1, 0)
		hi := render(m.Kind, 11, 160, 96, f, 0)
		var sum, n float64
		for y := range lo.Rect.Dy() {
			for x := range lo.Rect.Dx() {
				for ch := range 4 {
					var acc float64
					for dy := range f {
						for dx := range f {
							acc += float64(hi.Pix[hi.PixOffset(f*x+dx, f*y+dy)+ch])
						}
					}
					sum += math.Abs(acc/(f*f) - float64(lo.Pix[lo.PixOffset(x, y)+ch]))
					n++
				}
			}
		}
		if d := sum / n; d > 2 {
			t.Errorf("%s: средняя разница превью и печати %.2f из 255", m.Kind, d)
		}
	}
}

// TestGenAreaAnchor — узор привязан к углу области: та же область в другом месте холста даёт
// те же пиксели (± 1 от float32), за областью холст не тронут.
func TestGenAreaAnchor(t *testing.T) {
	for _, m := range Patterns() {
		alone := render(m.Kind, 2, 100, 60, 2, 0)
		big := image.NewRGBA(image.Rect(-10, -5, 290, 200))
		area := alone.Rect.Add(image.Pt(37, 21))
		DrawPattern(big, area, Spec{Kind: m.Kind, Seed: 2}, 2, 0)
		for y := big.Rect.Min.Y; y < big.Rect.Max.Y; y++ {
			for x := big.Rect.Min.X; x < big.Rect.Max.X; x++ {
				got := big.RGBAAt(x, y)
				if !(image.Point{x, y}.In(area)) {
					if got != (color.RGBA{}) {
						t.Fatalf("%s: рисунок за областью в (%d, %d)", m.Kind, x, y)
					}
					continue
				}
				want := alone.RGBAAt(x-area.Min.X, y-area.Min.Y)
				if d := maxDiff(got, want); d > 1 {
					t.Fatalf("%s: (%d, %d) %v ≠ %v", m.Kind, x, y, got, want)
				}
			}
		}
	}
}

func maxDiff(a, b color.RGBA) int {
	d := 0
	for _, p := range [4][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
		d = max(d, int(p[0])-int(p[1]), int(p[1])-int(p[0]))
	}
	return d
}

// TestGenFade — прозрачность 1 и больше — пусто; 0,5 — альфа сплошного узора вдвое меньше;
// NaN — как 0.
func TestGenFade(t *testing.T) {
	full := render("grad-h", 0, 50, 30, 2, 0)
	half := render("grad-h", 0, 50, 30, 2, 0.5)
	if a := half.Pix[3]; a < 126 || a > 129 || full.Pix[3] != 255 {
		t.Errorf("альфа при 0,5 = %d, при 0 = %d", a, full.Pix[3])
	}
	for _, f := range []float64{1, 2, math.Inf(1)} {
		if n := opaqueCount(render("dots", 0, 50, 30, 2, f)); n != 0 {
			t.Errorf("fade %v: нарисовано %d px", f, n)
		}
	}
	if !bytes.Equal(render("dots", 0, 50, 30, 2, math.NaN()).Pix, render("dots", 0, 50, 30, 2, 0).Pix) {
		t.Error("NaN — не как 0")
	}
	// Мягкий слой (6 px/мм > softK) с прозрачностью — тоже вдвое прозрачнее.
	a, b := render("watercolor", 1, 120, 80, 6, 0), render("watercolor", 1, 120, 80, 6, 0.5)
	var sa, sb int
	for i := 3; i < len(a.Pix); i += 4 {
		sa += int(a.Pix[i])
		sb += int(b.Pix[i])
	}
	if r := float64(sb) / float64(sa); r < 0.45 || r > 0.55 {
		t.Errorf("акварель при 0,5: доля альфы %.3f", r)
	}
}

// TestGenEdges — пустая область, нулевой и мусорный масштаб, nil-холст, огромная область —
// без паники и без рисунка.
func TestGenEdges(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for _, k := range []float64{0, -1, math.NaN(), math.Inf(1), 1e-6} {
		DrawPattern(dst, dst.Rect, Spec{Kind: "dots"}, k, 0)
	}
	DrawPattern(dst, image.Rectangle{}, Spec{Kind: "dots"}, 1, 0)
	DrawPattern(dst, image.Rect(100, 100, 200, 200), Spec{Kind: "dots"}, 1, 0)
	DrawPattern(nil, dst.Rect, Spec{Kind: "dots"}, 1, 0)
	DrawArt(nil, f64.Aff3{1, 0, 0, 0, 1, 0}, Spec{Kind: "dots"}, 0)
	if n := opaqueCount(dst); n != 0 {
		t.Errorf("нарисовано %d px", n)
	}
	// Наименьший масштаб на обложке — конечное время (клеток не миллионы).
	DrawPattern(dst, dst.Rect, Spec{Kind: "confetti", Scale: 0.25}, 20.0/2400, 0)
}

// TestPaperTiles — плитки текстуры делят область без щелей и без перекрытия: ровная маска даёт
// ровную альфу на стыках, в том числе при дробном размере плитки.
func TestPaperTiles(t *testing.T) {
	flat := image.NewAlpha(image.Rect(0, 0, 64, 64))
	for i := range flat.Pix {
		flat.Pix[i] = 100
	}
	tx := &texture{file: "нет.jpg"}
	tx.once.Do(func() {})
	tx.mips = mipChain(flat)
	draw1 := paper(tx)
	for _, k := range []float64{0.3, 1.37, 2} {
		dst := image.NewRGBA(image.Rect(0, 0, int(400*k), int(310*k)))
		area := dst.Rect.Inset(1)
		c := paint.New(dst, area, f64.Aff3{k, 0, float64(area.Min.X), 0, k, float64(area.Min.Y)}, 400, 310, 0)
		draw1(c, Spec{})
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if a := dst.RGBAAt(x, y).A; a != 100 {
					t.Fatalf("k=%v: (%d, %d) альфа %d — щель или двойная плитка", k, x, y, a)
				}
			}
		}
		if dst.RGBAAt(0, 0).A != 0 {
			t.Fatalf("k=%v: рисунок за областью", k)
		}
	}
	if got := (&texture{file: "нет.jpg"}).mask(10); got != nil {
		t.Error("нет файла — маска не nil")
	}
}

// TestPaperFlatDraw — чёрный с альфой: на белом холсте светлая плитка затемняет ровно на
// (1 − яркость); прозрачность ослабляет.
func TestPaperFlatDraw(t *testing.T) {
	flat := image.NewAlpha(image.Rect(0, 0, 32, 32))
	for i := range flat.Pix {
		flat.Pix[i] = 51 // яркость 0,8
	}
	tx := &texture{}
	tx.once.Do(func() {})
	tx.mips = mipChain(flat)
	dst := image.NewRGBA(image.Rect(0, 0, 40, 40))
	draw.Draw(dst, dst.Rect, image.White, image.Point{}, draw.Src)
	c := paint.New(dst, dst.Rect, f64.Aff3{1, 0, 0, 0, 1, 0}, 40, 40, 0.5)
	paper(tx)(c, Spec{})
	if got := dst.RGBAAt(20, 20); got.R < 228 || got.R > 231 || got.A != 255 {
		t.Errorf("белый под плиткой 0,8 при прозрачности 0,5: %v, ждали ≈ 229", got)
	}
}

// TestGenFadeMatchesPreview — печать с прозрачностью 0,5 = превью (fade 0) с opacity 0,5 поверх
// страницы, как на холсте UI: перекрытия фигур не темнеют. Плотность 2 и 6 px/мм (мягкий слой).
func TestGenFadeMatchesPreview(t *testing.T) {
	for _, kind := range []string{"waves", "confetti", "watercolor", "brush", "stars", "dots-scatter"} {
		for _, k := range []float64{2, 6} {
			pre := render(kind, 5, 120, 80, k, 0)
			want := image.NewRGBA(pre.Rect)
			draw.Draw(want, want.Rect, image.White, image.Point{}, draw.Src)
			draw.DrawMask(want, want.Rect, pre, image.Point{}, image.NewUniform(color.Alpha{A: 128}), image.Point{}, draw.Over)
			got := image.NewRGBA(pre.Rect)
			draw.Draw(got, got.Rect, image.White, image.Point{}, draw.Src)
			DrawPattern(got, got.Rect, Spec{Kind: kind, Seed: 5}, k, 0.5)
			worst := 0
			for y := range pre.Rect.Dy() {
				for x := range pre.Rect.Dx() {
					worst = max(worst, maxDiff(got.RGBAAt(x, y), want.RGBAAt(x, y)))
				}
			}
			if worst > 2 {
				t.Errorf("%s, %v px/мм: печать с прозрачностью разошлась с превью на %d", kind, k, worst)
			}
		}
	}
}

// TestPaperHugeScale — плотность превью 100 px/мм: маска плитки не больше maxMaskPx (иначе
// сотни МБ), а текстура в прежнем масштабе — плитка 150 мм, а не обрезанная маска.
func TestPaperHugeScale(t *testing.T) {
	ramp := image.NewAlpha(image.Rect(0, 0, 64, 64)) // альфа растёт слева направо: 4·x
	for i := range ramp.Pix {
		ramp.Pix[i] = uint8(4 * (i % 64))
	}
	tx := &texture{}
	tx.once.Do(func() {})
	tx.mips = mipChain(ramp)
	const k = 100
	dst := image.NewRGBA(image.Rect(0, 0, 4096, 4))
	c := paint.New(dst, dst.Rect, f64.Aff3{k, 0, 0, 0, k, 0}, 40.96, 0.04, 0)
	paper(tx)(c, Spec{})
	if tx.lastN > maxMaskPx {
		t.Fatalf("маска %d px — больше %d", tx.lastN, maxMaskPx)
	}
	for _, x := range []int{1000, 2000, 4000} {
		want := 4 * ((float64(x)+0.5)*64/(paperMM*k) - 0.5)
		if a := float64(dst.RGBAAt(x, 2).A); math.Abs(a-want) > 6 {
			t.Errorf("x=%d: альфа %v, ждали ≈ %.0f — масштаб текстуры сбит", x, a, want)
		}
	}
}
