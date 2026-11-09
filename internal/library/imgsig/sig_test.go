package imgsig

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// sceneCount — сколько разных сцен в проверке (рисование сцены — основная цена теста).
func sceneCount() int {
	if testing.Short() {
		return 8
	}
	return 16
}

// Почти дубли одного кадра близки, разные кадры далеки — по порогам уровней (078-3, research §2).
func TestDistSeparates(t *testing.T) {
	scenes := sceneCount()
	sigs := make([]Sig, scenes)
	var worst [3]int
	for i := range sigs {
		s := newScene(uint64(i + 1))
		a := Of(thumbSize(t, s, full, 1, 240, 180))
		sigs[i] = a
		if !a.Ok {
			t.Fatalf("сцена %d: подписи нет", i)
		}
		// экспозиция −15 % и пересжатие: уровень «Дубли»
		dark := Dist(a, Of(thumb(t, s.render(full, 240, 180, 0.85, false), 70)))
		near := Dist(a, Of(thumbSize(t, s, shift, 1, 240, 180)))
		zoom := Dist(a, Of(thumbSize(t, s, zoom5, 1.1, 240, 180)))
		worst[0], worst[1], worst[2] = max(worst[0], dark), max(worst[1], near), max(worst[2], zoom)
		if dark > DistDup {
			t.Errorf("сцена %d, темнее на 15 %%: Dist %d, ждали ≤ %d", i, dark, DistDup)
		}
		if near > DistNear {
			t.Errorf("сцена %d, сдвиг 3 %%: Dist %d, ждали ≤ %d", i, near, DistNear)
		}
		if zoom > DistSimilar {
			t.Errorf("сцена %d, ближе на 5 %% и ярче: Dist %d, ждали ≤ %d", i, zoom, DistSimilar)
		}
	}
	lowest := 128
	for i := range sigs {
		for j := i + 1; j < len(sigs); j++ {
			lowest = min(lowest, Dist(sigs[i], sigs[j]))
		}
	}
	if lowest <= DistSimilar {
		t.Errorf("разные сцены сошлись на Dist %d (≤ %d): уровень «Похожие» убрал бы разные кадры", lowest, DistSimilar)
	}
	t.Logf("разные кадры: минимум Dist %d из %d пар; почти дубли, худший: темнее %d, сдвиг %d, зум %d",
		lowest, scenes*(scenes-1)/2, worst[0], worst[1], worst[2])
}

// Подпись детерминирована: тот же кадр даёт ту же подпись, сколько ни считай.
func TestOfDeterministic(t *testing.T) {
	img := thumbOf(t, newScene(7), full, 1)
	want := Of(img)
	for range 5 {
		if got := Of(img); got != want {
			t.Fatalf("подпись изменилась: %+v → %+v", want, got)
		}
	}
	if again := Of(thumbOf(t, newScene(7), full, 1)); again != want {
		t.Fatalf("тот же кадр, заново закодированный: %+v ≠ %+v", again, want)
	}
}

// Один и тот же кадр в разных типах картинки (быстрые пути и общий через At) даёт одну подпись.
type plain struct{ image.Image }

func TestOfSameAcrossTypes(t *testing.T) {
	rgba := newScene(3).render(full, 200, 150, 1, false)
	nrgba := image.NewNRGBA(rgba.Bounds())
	for y := range 150 {
		for x := range 200 {
			c := rgba.RGBAAt(x, y)
			nrgba.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, 255})
		}
	}
	want := Of(rgba)
	if got := Of(nrgba); got != want {
		t.Errorf("NRGBA: %+v, RGBA: %+v", got, want)
	}
	if got := Of(plain{rgba}); got != want {
		t.Errorf("общий путь через At: %+v, RGBA: %+v", got, want)
	}
	// Gray: яркость та же, что у цветной картинки с R=G=B
	g := image.NewGray(rgba.Bounds())
	flat := image.NewRGBA(rgba.Bounds())
	for y := range 150 {
		for x := range 200 {
			v := rgba.RGBAAt(x, y).G
			g.SetGray(x, y, color.Gray{v})
			flat.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	gs, fs := Of(g), Of(flat)
	if gs.D != fs.D || gs.P != fs.P {
		t.Errorf("Gray: хеши %x/%x, цветная серая: %x/%x", gs.D, gs.P, fs.D, fs.P)
	}
	if gs.Color > 0.01 || fs.Color > 0.01 {
		t.Errorf("серые кадры: цветность %v и %v, ждали 0", gs.Color, fs.Color)
	}
}

// Края: пустая картинка, 1 пиксель, меньше сетки, не нулевой Min (под-картинка), однотонная.
func TestOfEdges(t *testing.T) {
	if s := Of(image.NewRGBA(image.Rect(0, 0, 0, 0))); s.Ok || s != (Sig{}) {
		t.Errorf("пустая картинка: %+v, ждали пустую подпись", s)
	}
	if s := Of(image.NewRGBA(image.Rect(5, 5, 5, 20))); s.Ok {
		t.Errorf("нулевая ширина: Ok = true")
	}
	for _, sz := range [][2]int{{1, 1}, {3, 2}, {8, 8}, {31, 5}, {9, 400}, {1000, 1}} {
		s := Of(newScene(2).render(full, sz[0], sz[1], 1, false))
		if !s.Ok || math.IsNaN(float64(s.Color)) || math.IsNaN(float64(s.Sharp)) {
			t.Errorf("%d×%d: %+v", sz[0], sz[1], s)
		}
	}
	// под-картинка со сдвигом Min считается как её копия с нулевым Min
	big := newScene(5).render(full, 120, 90, 1, false)
	r := image.Rect(20, 10, 100, 70)
	cp := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := range r.Dy() {
		for x := range r.Dx() {
			cp.SetRGBA(x, y, big.RGBAAt(r.Min.X+x, r.Min.Y+y))
		}
	}
	if got, want := Of(big.SubImage(r)), Of(cp); got != want {
		t.Errorf("под-картинка: %+v, копия: %+v", got, want)
	}
	// под-картинка JPEG-плоскостей с чётным сдвигом: просто не падает и считается
	yc := thumbOf(t, newScene(5), full, 1).(*image.YCbCr)
	if s := Of(yc.SubImage(image.Rect(40, 30, 300, 250))); !s.Ok {
		t.Error("под-картинка YCbCr: подписи нет")
	}
	// однотонная: хеши нулевые и стабильные, резкости нет
	flat := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for i := range flat.Pix {
		flat.Pix[i] = 200
	}
	f := Of(flat)
	if f.D != 0 || f.P != 0 || f.Sharp != 0 || f.Color > 0.01 {
		t.Errorf("однотонная: %+v", f)
	}
}

// Цветность различает цветной и чёрно-белый кадр (ColorGray), в том числе после JPEG.
func TestOfColor(t *testing.T) {
	colorful, gray := 0, 0
	for i := range 12 {
		s := newScene(uint64(100 + i))
		c := Of(thumb(t, s.render(full, 400, 300, 1, false), 82))
		g := Of(thumb(t, s.render(full, 400, 300, 1, true), 82))
		if c.Color >= ColorGray {
			colorful++
		}
		if g.Color < ColorGray {
			gray++
		}
		if g.Color >= c.Color {
			t.Errorf("сцена %d: ч/б цветность %v ≥ цветной %v", i, g.Color, c.Color)
		}
	}
	if colorful < 12 || gray < 12 {
		t.Errorf("цветных кадров определено %d из 12, ч/б — %d из 12", colorful, gray)
	}
}

// Резкость растёт с мелкими деталями.
func TestOfSharp(t *testing.T) {
	s := newScene(9)
	smooth := s.render(full, 400, 300, 1, false)
	noisy := image.NewRGBA(smooth.Bounds())
	copy(noisy.Pix, smooth.Pix)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < len(noisy.Pix); i += 4 {
		d := rng.IntN(120) - 60
		for c := range 3 {
			noisy.Pix[i+c] = clamp8(float64(int(noisy.Pix[i+c]) + d))
		}
	}
	if a, b := Of(smooth).Sharp, Of(noisy).Sharp; !(b > a*3) {
		t.Errorf("резкость гладкого %v, с шумом %v: ждали заметно больше", a, b)
	}
}

func TestDist(t *testing.T) {
	a := Sig{D: 0xF0F0F0F0F0F0F0F0, P: 0x123456789ABCDEF0, Ok: true}
	b := Sig{D: 0x0F0F0F0F0F0F0F0F, P: ^a.P, Ok: true}
	if Dist(a, a) != 0 {
		t.Errorf("Dist(a, a) = %d", Dist(a, a))
	}
	if Dist(a, b) != Dist(b, a) || Dist(a, b) != 128 {
		t.Errorf("противоположные: %d и %d, ждали 128", Dist(a, b), Dist(b, a))
	}
	if d := Dist(Sig{D: 0b1011}, Sig{P: 0b1}); d != 4 {
		t.Errorf("Dist = %d, ждали 4", d)
	}
}

// Запись подписи: круг Bytes → Parse без потерь, включая крайние значения.
func TestBytesParseRoundTrip(t *testing.T) {
	for _, s := range []Sig{
		{Ok: true},
		{D: math.MaxUint64, P: math.MaxUint64, Color: math.MaxFloat32, Sharp: math.SmallestNonzeroFloat32, Ok: true},
		{D: 1, P: 1 << 63, Color: 3.5, Sharp: 1234.5, Ok: true},
		Of(thumbOf(t, newScene(11), full, 1)),
	} {
		b := s.Bytes()
		if len(b) != FileSize {
			t.Fatalf("размер записи %d, ждали %d", len(b), FileSize)
		}
		got, ok := Parse(b)
		if !ok || got != s {
			t.Errorf("круг: %+v → %+v (ok=%v)", s, got, ok)
		}
	}
}

// Повреждённая запись — подписи нет, не паника.
func TestParseRejects(t *testing.T) {
	good := Sig{D: 5, P: 9, Color: 10, Sharp: 20, Ok: true}.Bytes()
	cases := map[string][]byte{
		"пусто":        nil,
		"короче":       good[:FileSize-1],
		"длиннее":      append(slices.Clone(good), 0),
		"версия 0":     withByte(good, 0, 0),
		"версия 2":     withByte(good, 0, 2),
		"NaN цвет":     withFloat(good, 17, float32(math.NaN())),
		"Inf резкость": withFloat(good, 21, float32(math.Inf(1))),
		"минус цвет":   withFloat(good, 17, -1),
		"минус резк.":  withFloat(good, 21, -0.5),
	}
	for name, b := range cases {
		if s, ok := Parse(b); ok || s != (Sig{}) {
			t.Errorf("%s: принято %+v", name, s)
		}
	}
}

func withByte(b []byte, i int, v byte) []byte {
	c := slices.Clone(b)
	c[i] = v
	return c
}

func withFloat(b []byte, i int, f float32) []byte {
	c := slices.Clone(b)
	bits := math.Float32bits(f)
	for k := range 4 {
		c[i+k] = byte(bits >> (8 * k))
	}
	return c
}

// Любые байты файла подписи: принятое переписывается обратно в те же байты.
func FuzzSigFile(f *testing.F) {
	f.Add(Sig{D: 1, P: 2, Color: 3, Sharp: 4, Ok: true}.Bytes())
	f.Add([]byte{})
	f.Add(make([]byte, FileSize))
	f.Add(withFloat(Sig{Ok: true}.Bytes(), 17, float32(math.NaN())))
	f.Fuzz(func(t *testing.T, b []byte) {
		s, ok := Parse(b)
		if !ok {
			if s != (Sig{}) {
				t.Fatalf("отказ вернул %+v", s)
			}
			return
		}
		if !s.Ok || !okFloat(s.Color) || !okFloat(s.Sharp) {
			t.Fatalf("принята негодная подпись %+v", s)
		}
		if got, ok := Parse(s.Bytes()); !ok || got != s {
			t.Fatalf("круг: %+v → %+v", s, got)
		}
	})
}

// BenchmarkSigThumb — подпись миниатюры 400×300 JPEG (без декода): цель — единицы миллисекунд.
func BenchmarkSigThumb(b *testing.B) {
	img := thumbOf(b, newScene(1), full, 1)
	b.ResetTimer()
	for range b.N {
		if !Of(img).Ok {
			b.Fatal("нет подписи")
		}
	}
}
