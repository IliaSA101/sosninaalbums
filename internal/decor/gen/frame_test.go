package gen

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// newCanvas — холст рамочки s на img (для заливки «фото» под рамочкой в тестах).
func newCanvas(img *image.RGBA, m f64.Aff3, s FrameSpec) *canvas {
	return paint.New(img, img.Rect, m, s.W, s.H, 0)
}

const fk = 4.0 // px на мм в тестах геометрии

// alphaAt — пиксель рамочки в точке (x, y) мм места при масштабе fk.
func alphaAt(img *image.RGBA, x, y float64) color.RGBA {
	return img.RGBAAt(int(x*fk), int(y*fk))
}

func frameImg(s FrameSpec) *image.RGBA { return renderFrame(s, fk) }

func spec(kind string, w float64) FrameSpec {
	return FrameSpec{Kind: kind, Width: w, W: 100, H: 150}
}

func opaque(p color.RGBA) bool { return p.A == 255 }
func clear0(p color.RGBA) bool { return p.A == 0 }

// probe — проверка пикселя рамочки в точке (x, y) мм места: want < 0 — пусто, 0 — любой
// непрозрачный, иначе красный канал цвета.
type probe struct {
	x, y float64
	want int
}

// TestFrameBands — полосы видов лежат там, где их считает окно фото: середина полосы — цвет
// рамочки, окно — пусто, у полароида низ шире, у овала полоса ровная.
func TestFrameBands(t *testing.T) {
	oval := func(s FrameSpec) FrameSpec { s.Oval = true; return s }
	for _, c := range []struct {
		name   string
		s      FrameSpec
		probes []probe
	}{
		{"line", spec("line", 2), []probe{{1, 75, 255}, {3, 75, -1}, {50, 148.5, 0}, {50, 75, -1}}},
		{"polaroid", spec("polaroid", 5), []probe{{50, 141, 0}, {50, 131, -1}, {2.5, 75, 0}, {7, 75, -1}}}, // низ 17,5
		{"polaroidOval", oval(spec("polaroid", 2)), []probe{{50, 147, 0}, {50, 142, -1}, {3, 75, 0}, {8, 75, -1}, {50, 75, -1}}},
		{"double", spec("double", 3), []probe{{0.6, 75, 0}, {1.65, 75, -1}, {2.55, 75, 0}, {3.8, 75, -1}}}, // 0…1,2 · просвет · 2,1…3
		{"mat", spec("mat", 8), []probe{{4, 75, 0xfb}, {7.9, 75, 0xc8}, {9, 75, -1}}},                      // цвет 1 и линия 0,4 мм цвета 2
		{"gold", spec("gold", 4), []probe{{0.6, 75, 0xec}, {2, 75, 0x8f}, {3.4, 75, 0xec}, {5, 75, -1}}},   // светлая–тёмная–светлая
		{"corners", spec("corners", 0), []probe{{1, 1, 0}, {99, 149, 0}, {99, 1, 0}, {1, 149, 0}, {6.5, 6.5, -1}, {50, 1, -1}, {50, 75, -1}}},
	} {
		img := frameImg(c.s)
		for _, p := range c.probes {
			got := alphaAt(img, p.x, p.y)
			switch {
			case p.want < 0 && got.A != 0, p.want >= 0 && got.A != 255, p.want > 0 && int(got.R) != p.want:
				t.Errorf("%s: (%v, %v) = %v, ждали %d", c.name, p.x, p.y, got, p.want)
			}
		}
	}
}

// TestFrameScallop — выемка 0,4·w = 1,6 мм: глубже полоса сплошная, край волнистый, окно открыто.
func TestFrameScallop(t *testing.T) {
	img := frameImg(spec("scallop", 4))
	var edge, gaps int
	for y := 20.0; y < 130; y += 0.25 {
		if !opaque(alphaAt(img, 2.4, y)) {
			t.Fatalf("фестоны: дыра на y = %v", y)
		}
		edge++
		if clear0(alphaAt(img, 0.5, y)) {
			gaps++
		}
	}
	if gaps == 0 || gaps == edge {
		t.Errorf("фестоны: край не волнистый (%d из %d)", gaps, edge)
	}
	if !clear0(alphaAt(img, 5, 75)) {
		t.Error("фестоны: окно закрыто")
	}
}

// TestFrameStitch — накладка: окно не занимает, штрихи лежат только на линии отступа 2,5 мм.
func TestFrameStitch(t *testing.T) {
	img := frameImg(spec("stitch", 0))
	var dash, gap int
	for x := 5.0; x < 95; x += 0.25 {
		if opaque(alphaAt(img, x, 2.5)) {
			dash++
		} else {
			gap++
		}
	}
	if dash == 0 || gap == 0 {
		t.Errorf("строчка: штрихов %d, промежутков %d", dash, gap)
	}
	for y := 10.0; y < 140; y++ {
		if !clear0(alphaAt(img, 0.4, y)) || !clear0(alphaAt(img, 50, y)) {
			t.Fatal("строчка рисует вне линии отступа")
		}
	}
}

// TestFrameDefaultWidth — Width = 0 даёт тот же рисунок, что явная толщина по умолчанию вида.
func TestFrameDefaultWidth(t *testing.T) {
	for _, m := range Frames() {
		for _, sh := range frameGoldenShapes {
			a := renderFrame(frameGoldenSpec(m.Kind, 0, sh.oval, sh.radius), 2)
			b := renderFrame(frameGoldenSpec(m.Kind, m.Width, sh.oval, sh.radius), 2)
			if goldenHash(a) != goldenHash(b) {
				t.Errorf("%s/%s: w = 0 и w = %v различаются", m.Kind, sh.name, m.Width)
			}
		}
	}
}

// TestFrameNoSeam — на границе окна покрытие «фото + рамочка» ≥ 0,98: край фото не просвечивает
// цветом страницы (шов 0,15 мм). Фото — сплошная заливка окна, нарисованная тем же холстом.
func TestFrameNoSeam(t *testing.T) {
	const k = 6.0
	shapes := []struct {
		name   string
		oval   bool
		radius float64
	}{{"rect", false, 0}, {"round", false, 8}, {"oval", true, 0}}
	for _, m := range Frames() {
		if m.Ins == ([4]float64{}) {
			continue
		}
		for _, sh := range shapes {
			s := ClampFrame(FrameSpec{Kind: m.Kind, W: 80, H: 100, Oval: sh.oval, Radius: sh.radius})
			f, _ := newFrame(s)
			win, ok := f.shape().off(f.ins)
			if !ok {
				t.Fatal("нет окна")
			}
			img := image.NewRGBA(image.Rect(0, 0, int(s.W*k), int(s.H*k)))
			mm := f64.Aff3{k, 0, 0, 0, k, 0}
			fillShape(img, mm, s, win, color.NRGBA{200, 0, 0, 255})
			DrawFrame(img, mm, s)
			// Вдоль границы окна (шаг 1°, по эллипсу или по периметру) покрытие ≥ 0,98.
			minA := 255
			for i := range 720 {
				x, y := windowEdge(win, float64(i)/2*math.Pi/180)
				if win.r > 0 && !win.oval && nearCorner(win, x, y) {
					continue // скругление: край — дуга, точка периметра прямоугольника рядом, но не на ней
				}
				a := int(img.RGBAAt(int(x*k), int(y*k)).A)
				minA = min(minA, a)
			}
			if minA < 250 {
				t.Errorf("%s/%s: на границе окна покрытие %d/255", m.Kind, sh.name, minA)
			}
		}
	}
}

// windowEdge — точка границы окна под углом a от центра: эллипс или пересечение луча с рамкой.
func windowEdge(win fshape, a float64) (float64, float64) {
	cx, cy := (win.x0+win.x1)/2, (win.y0+win.y1)/2
	hw, hh := (win.x1-win.x0)/2, (win.y1-win.y0)/2
	sn, cs := math.Sincos(a)
	if win.oval {
		return cx + hw*cs, cy + hh*sn
	}
	t := math.Min(hw/math.Max(math.Abs(cs), 1e-9), hh/math.Max(math.Abs(sn), 1e-9))
	return cx + t*cs, cy + t*sn
}

// nearCorner — точка в квадрате скругления угла окна.
func nearCorner(win fshape, x, y float64) bool {
	return (x < win.x0+win.r || x > win.x1-win.r) && (y < win.y0+win.r || y > win.y1-win.r)
}

// fillShape заливает окно win цветом (фото под рамочкой).
func fillShape(img *image.RGBA, m f64.Aff3, s FrameSpec, win fshape, ink color.NRGBA) {
	c := newCanvas(img, m, s)
	win.trace(c, false)
	c.Fill(ink, 1)
}

// TestFrameInsParity — свой зажим отступов совпадает с geom.ClampInset (векторы паритета).
func TestFrameInsParity(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/parity/inset.json")
	if err != nil {
		t.Fatal(err)
	}
	var vs []struct {
		Cell    struct{ W, H float64 }
		Ins     [4]float64
		Clamped [4]float64
	}
	if err := json.Unmarshal(data, &vs); err != nil || len(vs) == 0 {
		t.Fatalf("векторы: %v (%d)", err, len(vs))
	}
	for i, v := range vs {
		if got := clampIns(v.Ins, v.Cell.W, v.Cell.H); got != v.Clamped {
			t.Errorf("вектор %d: %v, ждали %v", i, got, v.Clamped)
		}
	}
}

// TestFrameWindowMatchesBand — окно, которое занимает рамочка (clampIns × Ins), совпадает с
// полосой на экране: линия 20 мм на месте 40×40 сжата до 14 мм по оси (окно 12 мм).
func TestFrameWindowMatchesBand(t *testing.T) {
	s := FrameSpec{Kind: "line", Width: 20, W: 40, H: 40}
	img := renderFrame(s, fk)
	if !opaque(alphaAt(img, 13, 20)) || !clear0(alphaAt(img, 15, 20)) || !clear0(alphaAt(img, 20, 20)) {
		t.Error("пара отступов не сжата до 0,7 стороны")
	}
}

// TestFrameClamp — зажим: мусор, пустое, пределы; идемпотентность.
func TestFrameClamp(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := []FrameSpec{
		{Kind: "line", W: 100, H: 100},
		{Kind: "mat", Inks: []string{"#ABCDEF"}, Width: 99, W: 1e9, H: nan},
		{Kind: "mat", Inks: []string{"плохо", "#112233", "#445566"}, Width: nan, W: -5, H: inf},
		{Kind: "soft", Inks: []string{"#ffffff"}, Width: 0.01, W: 50, H: 50, Radius: 1e9},
		{Kind: "future-kind", Inks: []string{"#fff", "#123456", "#654321"}, Width: 3, W: 10, H: 10, Oval: true, Radius: 4},
		{Kind: "", W: 0, H: 0},
	}
	for _, c := range cases {
		g := ClampFrame(c)
		if again := ClampFrame(g); !reflect.DeepEqual(again, g) {
			t.Errorf("%+v: не идемпотентна: %+v", c, again)
		}
		if g.W < 0 || g.W > maxFrameMM || math.IsNaN(g.H) || g.Radius > min(g.W, g.H)/2 || len(g.Inks) > MaxFrameInks {
			t.Errorf("%+v → %+v: вне пределов", c, g)
		}
	}
	g := ClampFrame(FrameSpec{Kind: "mat", Inks: []string{"#ABCDEF"}, W: 50, H: 50})
	if !reflect.DeepEqual(g.Inks, []string{"#abcdef", "#c8a24a"}) || g.Width != 8 {
		t.Errorf("цвета достроены не по виду: %+v", g)
	}
	if g := ClampFrame(FrameSpec{Kind: "line", W: 40, H: 60, Oval: true, Radius: 5}); g.Radius != 0 {
		t.Error("у овала скругление не обнулено")
	}
	if g := ClampFrame(FrameSpec{Kind: "line", Width: 99, W: 40, H: 60, Radius: 99}); g.Width != MaxFrameWidth || g.Radius != 20 {
		t.Errorf("пределы: %+v", g)
	}
	if g := ClampFrame(FrameSpec{Kind: "line", Width: -3, W: 40, H: 60}); g.Width != 1.5 {
		t.Errorf("толщина ≤ 0 — по умолчанию вида: %v", g.Width)
	}
}

// TestFrameDegenerate — мусор на входе: ничего не рисуется и не падает.
func TestFrameDegenerate(t *testing.T) {
	nan := math.NaN()
	ok := f64.Aff3{4, 0, 0, 0, 4, 0}
	blank := func(img *image.RGBA) bool {
		for _, v := range img.Pix {
			if v != 0 {
				return false
			}
		}
		return true
	}
	for name, c := range map[string]struct {
		m f64.Aff3
		s FrameSpec
	}{
		"nan":      {f64.Aff3{nan, 0, 0, 0, 4, 0}, spec("line", 0)},
		"singular": {f64.Aff3{0, 0, 0, 0, 0, 0}, spec("line", 0)},
		"unknown":  {ok, spec("future", 0)},
		"empty":    {ok, FrameSpec{Kind: "line"}},
		"soft":     {ok, spec("soft", 0)}, // маска — через FrameMask, не DrawFrame
		"nanSize":  {ok, FrameSpec{Kind: "line", W: nan, H: 10}},
	} {
		img := image.NewRGBA(image.Rect(0, 0, 40, 60))
		DrawFrame(img, c.m, c.s)
		if !blank(img) {
			t.Errorf("%s: что-то нарисовано", name)
		}
	}
	DrawFrame(nil, ok, spec("line", 0))
	FrameMask(nil, ok, spec("soft", 0))
	// Рамочка вне холста и холст нулевого размера.
	DrawFrame(image.NewRGBA(image.Rect(0, 0, 0, 0)), ok, spec("line", 0))
	img := image.NewRGBA(image.Rect(500, 500, 540, 560))
	DrawFrame(img, ok, spec("line", 0))
	if !blank(img) {
		t.Error("рамочка вне холста рисует")
	}
}

// TestFrameSeveral — несколько рамочек на одном холсте не влияют друг на друга: рисунок равен
// сумме отдельных (области не пересекаются).
func TestFrameSeveral(t *testing.T) {
	kinds := []string{"line", "double", "mat", "polaroid", "stitch", "corners", "gold", "scallop"}
	all := image.NewRGBA(image.Rect(0, 0, 8*130, 200))
	var solo []*image.RGBA
	for i, kind := range kinds {
		m := f64.Aff3{2, 0, float64(i*130 + 5), 0, 2, 5}
		s := FrameSpec{Kind: kind, W: 55, H: 90, Oval: i%3 == 2, Radius: float64(i % 2 * 6)}
		DrawFrame(all, m, s)
		one := image.NewRGBA(all.Rect)
		DrawFrame(one, m, s)
		solo = append(solo, one)
	}
	sum := image.NewRGBA(all.Rect)
	for _, one := range solo {
		for i, v := range one.Pix {
			sum.Pix[i] += v
		}
	}
	if !reflect.DeepEqual(sum.Pix, all.Pix) {
		t.Fatal("рамочки на одном холсте влияют друг на друга")
	}
	if goldenHash(all) == goldenHash(image.NewRGBA(all.Rect)) {
		t.Fatal("холст пуст")
	}
}

// TestFrameRotated — поворот на 90° квадратного места даёт тот же рисунок (вектор, без
// пересэмплирования), сдвиг и масштаб — в матрице.
func TestFrameRotated(t *testing.T) {
	s := FrameSpec{Kind: "gold", Width: 3, W: 60, H: 60, Radius: 5}
	a := image.NewRGBA(image.Rect(0, 0, 120, 120))
	DrawFrame(a, f64.Aff3{2, 0, 0, 0, 2, 0}, s)
	b := image.NewRGBA(image.Rect(0, 0, 120, 120))
	DrawFrame(b, f64.Aff3{0, -2, 120, 2, 0, 0}, s) // поворот на 90° вокруг центра 60 мм
	var maxd int
	for i := range a.Pix {
		maxd = max(maxd, int(math.Abs(float64(a.Pix[i])-float64(b.Pix[i]))))
	}
	if maxd > 3 {
		t.Errorf("поворот на 90° расходится на %d", maxd)
	}
	if reflect.DeepEqual(a.Pix, make([]uint8, len(a.Pix))) {
		t.Fatal("пусто")
	}
	// Зеркальная матрица (det < 0) рисует ту же рамочку.
	c := image.NewRGBA(image.Rect(0, 0, 120, 120))
	DrawFrame(c, f64.Aff3{-2, 0, 120, 0, 2, 0}, s)
	if goldenHash(c) == goldenHash(image.NewRGBA(c.Rect)) {
		t.Error("зеркальная матрица: пусто")
	}
}

// TestFrameMask — мягкий край: 0 у края, линейно до 255 на толщине, 255 глубже; у овала то же.
func TestFrameMask(t *testing.T) {
	for _, oval := range []bool{false, true} {
		s := FrameSpec{Kind: "soft", Width: 8, W: 100, H: 150, Oval: oval}
		a := image.NewAlpha(image.Rect(0, 0, 400, 600))
		FrameMask(a, f64.Aff3{4, 0, 0, 0, 4, 0}, s)
		at := func(x, y float64) int { return int(a.AlphaAt(int(x*4), int(y*4)).A) }
		if at(50, 0.1) > 4 || at(50, 4) < 120 || at(50, 4) > 136 || at(50, 9) != 255 || at(50, 75) != 255 {
			t.Errorf("oval=%v: края %d, середина ширины %d, глубже %d", oval, at(50, 0.1), at(50, 4), at(50, 9))
		}
		if at(0.1, 75) > 4 || at(4, 75) < 120 || at(4, 75) > 136 {
			t.Errorf("oval=%v: боковой край %d / %d", oval, at(0.1, 75), at(4, 75))
		}
		if oval && (at(1, 1) != 0 || at(2, 3) != 0) {
			t.Error("овал: угол места не пуст")
		}
	}
	// Скруглённый угол: вне дуги — 0.
	s := FrameSpec{Kind: "soft", W: 100, H: 100, Radius: 20}
	a := image.NewAlpha(image.Rect(0, 0, 100, 100))
	FrameMask(a, f64.Aff3{1, 0, 0, 0, 1, 0}, s)
	if a.AlphaAt(1, 1).A != 0 || a.AlphaAt(50, 50).A != 255 {
		t.Error("скругление: угол или центр")
	}
	// Не Mask-вид: dst не трогается.
	b := image.NewAlpha(image.Rect(0, 0, 10, 10))
	b.Pix[5] = 77
	FrameMask(b, f64.Aff3{1, 0, 0, 0, 1, 0}, FrameSpec{Kind: "line", W: 10, H: 10})
	if b.Pix[5] != 77 {
		t.Error("FrameMask тронул dst для вида без маски")
	}
}

// TestFrameFast — плитка 256 px любого вида ≤ Limit(30 ms); экран 6 px/мм на 100×150 — Limit(300 ms).
func TestFrameFast(t *testing.T) {
	if testing.Short() || testkit.Race {
		t.Skip("замер времени: обычный прогон без -short и -race")
	}
	for _, m := range Frames() {
		s := frameGoldenSpec(m.Kind, 0, false, 0)
		best := testkit.Fastest(5, func() { renderFrame(s, 256.0/100) })
		if best > testkit.Limit(30*time.Millisecond) {
			t.Errorf("%s: плитка 256 px %v", m.Kind, best)
		}
		best = testkit.Fastest(3, func() { renderFrame(s, 6) })
		if best > testkit.Limit(300*time.Millisecond) {
			t.Errorf("%s: экран 6 px/мм %v", m.Kind, best)
		}
	}
}

func BenchmarkDrawFrame(b *testing.B) {
	for _, m := range Frames() {
		s := frameGoldenSpec(m.Kind, 0, false, 5)
		b.Run(m.Kind+"/300dpi", func(b *testing.B) {
			k := 300 / 25.4
			img := image.NewRGBA(image.Rect(0, 0, int(s.W*k), int(s.H*k)))
			mm := f64.Aff3{k, 0, 0, 0, k, 0}
			a := image.NewAlpha(img.Rect)
			b.ResetTimer()
			for range b.N {
				if m.Mask {
					FrameMask(a, mm, s)
				} else {
					clear(img.Pix)
					DrawFrame(img, mm, s)
				}
			}
		})
	}
}
