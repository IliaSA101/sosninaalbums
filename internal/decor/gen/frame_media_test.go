package gen

import (
	"bytes"
	"image"
	"math"
	"math/rand/v2"
	"net/url"
	"strings"
	"testing"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// rawQuery — строка запроса из адреса Query: после «?».
func rawQuery(u string) string { _, q, _ := strings.Cut(u, "?"); return q }

// TestFrameQueryRoundTrip — адрес накладки: что построил FrameQuery, ParseFrame читает без потерь
// (каждый вид, овал, скругление, маска), с px; нулевое не пишется.
func TestFrameQueryRoundTrip(t *testing.T) {
	for _, m := range Frames() {
		for _, sh := range []FrameSpec{
			{W: 100, H: 150}, {W: 80.123456, H: 40, Oval: true}, {W: 100, H: 50, Radius: 12.5},
		} {
			s := sh
			s.Kind, s.Width = m.Kind, 3.2
			if len(m.Inks) > 0 {
				s.Inks = []string{"#AABBCC"}
			}
			want := ClampFrame(s)
			want.W, want.H = round3(want.W), round3(want.H)
			want.Radius = round3(want.Radius)
			for _, mask := range []bool{false, m.Mask} {
				u := FrameQuery(s, mask)
				path, q, _ := strings.Cut(u, "?")
				if path != FramePath || strings.Contains(q, "px=") {
					t.Fatalf("%s: адрес %s", m.Kind, u)
				}
				req, ok := ParseFrame(q + "&px=300")
				if !ok || req.Px != 300 || req.Mask != mask || req.Tile {
					t.Fatalf("%s: разбор %s: %+v %v", m.Kind, u, req, ok)
				}
				if got := req.Spec; got.Kind != want.Kind || got.W != want.W || got.H != want.H || got.Oval != want.Oval ||
					got.Radius != want.Radius || got.Width != want.Width || strings.Join(got.Inks, ",") != strings.Join(want.Inks, ",") {
					t.Errorf("%s: %+v, ждали %+v", m.Kind, got, want)
				}
			}
		}
	}
	u := FrameQuery(FrameSpec{Kind: "line", W: 10, H: 10}, false)
	if strings.Contains(u, "o=") || strings.Contains(u, "rd=") || strings.Contains(u, "m=") {
		t.Errorf("лишнее в адресе: %s", u)
	}
}

// TestParseFrameBad — мусор и лишнее: 404 (ok = false).
func TestParseFrameBad(t *testing.T) {
	good := "k=line&i=ffffff&bw=1.5&w=100&h=50&v=1&px=64"
	if _, ok := ParseFrame(good); !ok {
		t.Fatal("образец не читается")
	}
	for _, q := range []string{
		"", "%zz", good + "&px=64", good + "&path=C:/x", good + "&t=1", // повтор, лишний ключ, плитка с размером
		strings.Replace(good, "k=line", "k=nope", 1), strings.Replace(good, "k=line", "k=", 1),
		strings.Replace(good, "&px=64", "", 1), strings.Replace(good, "&v=1", "", 1), strings.Replace(good, "&w=100", "", 1),
		strings.Replace(good, "px=64", "px=15", 1), strings.Replace(good, "px=64", "px=4097", 1), strings.Replace(good, "v=1", "v=x", 1),
		strings.Replace(good, "w=100", "w=NaN", 1), strings.Replace(good, "w=100", "w=Inf", 1), strings.Replace(good, "w=100", "w=0", 1),
		strings.Replace(good, "w=100", "w=5001", 1), strings.Replace(good, "bw=1.5", "bw=-1", 1),
		strings.Replace(good, "i=ffffff", "i=fff", 1), strings.Replace(good, "i=ffffff", "i=ffffff,000000,111111", 1),
		good + "&o=0", good + "&o=2", good + "&rd=-3", good + "&m=1", // m у вида без маски
		"k=soft&w=100&h=50&v=1&px=64&m=0", "k=soft&w=1&h=5000&v=1&px=4096&m=1", // выше 4096 px
		"k=soft&w=100&h=50&v=1&px=64&t=1", // плитка с размером места
		"k=line&t=1&v=1&px=64&m=1",        // плитка с маской
		"k=line&t=1&v=1&px=64&o=1", "k=line&t=0&v=1&px=64",
	} {
		if _, ok := ParseFrame(q); ok {
			t.Errorf("принят: %q", q)
		}
	}
	if _, ok := ParseFrame("k=soft&w=100&h=50&v=1&px=64&m=1"); !ok {
		t.Error("маска soft не читается")
	}
	if req, ok := ParseFrame("k=line&t=1&v=1&px=64"); !ok || !req.Tile || req.Spec.Width != 1.5 || len(req.Spec.Inks) != 1 {
		t.Errorf("плитка: %+v %v", req, ok)
	}
}

// TestFrameDrawKinds — накладка: размер по месту, прозрачный центр; плитка — квадрат, силуэт на
// месте, не пусто и угол свободен; у каждого вида.
func TestFrameDrawKinds(t *testing.T) {
	for _, m := range Frames() {
		req, ok := ParseFrame(rawQuery(FrameQuery(FrameSpec{Kind: m.Kind, W: 60, H: 80}, false)) + "&px=120")
		if !ok {
			t.Fatalf("%s: не читается", m.Kind)
		}
		img := req.Draw()
		if b := img.Bounds(); b.Dx() != 120 || b.Dy() != 160 {
			t.Errorf("%s: накладка %v", m.Kind, b)
		}
		if _, _, _, a := img.At(60, 80).RGBA(); a != 0 {
			t.Errorf("%s: центр накладки закрашен", m.Kind)
		}
		tile, ok := ParseFrame(rawQuery(TileQuery(m.Kind, nil, 0)) + "&px=96")
		if !ok {
			t.Fatalf("%s: плитка не читается", m.Kind)
		}
		checkTile(t, m.Kind, tile.Draw())
	}
}

func checkTile(t *testing.T, kind string, ti image.Image) {
	t.Helper()
	if b := ti.Bounds(); b.Dx() != 96 || b.Dy() != 96 {
		t.Errorf("%s: плитка %v", kind, b)
	}
	painted := 0
	for y := range 96 {
		for x := range 96 {
			if _, _, _, a := ti.At(x, y).RGBA(); a != 0 {
				painted++
			}
		}
	}
	if painted < 96*96/4 {
		t.Errorf("%s: плитка почти пуста: %d px", kind, painted)
	}
	if _, _, _, a := ti.At(1, 1).RGBA(); a != 0 {
		t.Errorf("%s: угол плитки закрашен", kind)
	}
}

// TestFrameDrawMask — маска soft: 0 у края, 255 в середине, чёрная; совпадает с FrameMask.
func TestFrameDrawMask(t *testing.T) {
	req, _ := ParseFrame("k=soft&w=60&h=80&v=1&px=120&m=1")
	mask, ok := req.Draw().(*image.NRGBA)
	if !ok {
		t.Fatal("маска не NRGBA")
	}
	if a := mask.NRGBAAt(0, 80).A; a > 10 { // центр пикселя в 0,25 мм от края: 8/255
		t.Errorf("край маски %d", a)
	}
	if c := mask.NRGBAAt(60, 80); c.A != 255 || c.R+c.G+c.B != 0 {
		t.Errorf("середина маски %+v", c)
	}
	direct := image.NewAlpha(image.Rect(0, 0, 120, 160))
	FrameMask(direct, f64.Aff3{2, 0, 0, 0, 2, 0}, req.Spec)
	for _, p := range [][2]int{{3, 3}, {10, 80}, {60, 80}, {16, 40}} {
		if mask.NRGBAAt(p[0], p[1]).A != direct.AlphaAt(p[0], p[1]).A {
			t.Errorf("маска в %v расходится с FrameMask", p)
		}
	}
}

// TestFramePicQuery — запрос рамочки-картинки: круг строитель → разбор, зажим размера и среза,
// отказы на мусор.
func TestFramePicQuery(t *testing.T) {
	r := FramePicRequest{W: 100.12345, H: 150, Scale: 1.234, Cut: [4]float64{0.14, 0, 0.5, math.NaN()}, Open: false, Mask: true}
	q := FramePicQuery(r)
	if strings.Contains(q, "px=") || strings.Contains(q, "?") {
		t.Fatalf("запрос %q", q)
	}
	got, ok := ParseFramePic(q + "&px=200")
	want := FramePicRequest{W: 100.123, H: 150, Scale: 1.23, Cut: [4]float64{0.14, 0, 0.45, 0}, Mask: true, Px: 200}
	if !ok || got != want {
		t.Fatalf("круг: %+v %v, ждали %+v", got, ok, want)
	}
	if w, h := got.Size(); w != 200 || h != 300 {
		t.Errorf("размер %d×%d", w, h)
	}
	if def, ok := ParseFramePic("w=30&h=40&v=1&px=64"); !ok || def.Scale != 1 || def.Cut != [4]float64{} || def.Open || def.Mask {
		t.Errorf("по умолчанию: %+v %v", def, ok)
	}
	if q := FramePicQuery(FramePicRequest{W: 30, H: 40, Scale: 1}); strings.Contains(q, "s=") || strings.Contains(q, "o=") || strings.Contains(q, "m=") {
		t.Errorf("лишнее: %s", q)
	}
	if z, ok := ParseFramePic("w=30&h=40&s=0&v=1&px=64"); !ok || z.Scale != 1 {
		t.Errorf("s=0: %+v %v", z, ok)
	}
	if z, ok := ParseFramePic("w=30&h=40&s=9&c=1e308,-5,0.1,0.2&v=1&px=64"); !ok || z.Scale != 2 || z.Cut != [4]float64{0.45, 0, 0.1, 0.2} {
		t.Errorf("зажим: %+v %v", z, ok)
	}
	good := "w=30&h=40&c=0.1,0.1,0.1,0.1&v=1&px=64"
	for _, bad := range []string{
		"", "%zz", good + "&px=64", good + "&x=1", good + "&o=0", good + "&m=2", good + "&o=1&m=1",
		"h=40&v=1&px=64", "w=30&v=1&px=64", "w=30&h=40&px=64", "w=30&h=40&v=1",
		"w=30&h=40&v=1&px=15", "w=30&h=40&v=1&px=4097", "w=NaN&h=40&v=1&px=64", "w=30&h=Inf&v=1&px=64",
		"w=0&h=40&v=1&px=64", "w=5001&h=40&v=1&px=64", "w=30&h=40&s=NaN&v=1&px=64", "w=30&h=40&s=-1&v=1&px=64",
		"w=30&h=40&c=1,2,3&v=1&px=64", "w=30&h=40&c=1,2,3,4,5&v=1&px=64", "w=30&h=40&c=NaN,0,0,0&v=1&px=64",
		"w=30&h=40&c=Inf,0,0,0&v=1&px=64", "w=30&h=40&c=a,b,c,d&v=1&px=64",
		"w=1&h=5000&v=1&px=4096", // выше 4096 px
	} {
		if _, ok := ParseFramePic(bad); ok {
			t.Errorf("принят: %q", bad)
		}
	}
	if _, err := url.ParseQuery(q); err != nil {
		t.Errorf("запрос не разбирается: %v", err)
	}
}

// TestStretchWindow — центральная часть маски растянута на окно: внутри 255, снаружи 0; пусто —
// пустой холст.
func TestStretchWindow(t *testing.T) {
	img := frameImage(200, 300, rectHole(40, 60, 160, 240)) // срез 0,2 / 0,2 / 0,2 / 0,2
	cut, _, ok := FrameWindow(img)
	if !ok {
		t.Fatal("окна нет")
	}
	m := WindowMask(img, cut)
	win := image.Rect(10, 20, 90, 120)
	out := StretchWindow(m, cut, image.Pt(100, 150), win)
	if out.Bounds() != image.Rect(0, 0, 100, 150) {
		t.Fatalf("холст %v", out.Bounds())
	}
	for _, c := range []struct {
		x, y int
		want uint8
	}{{50, 70, 255}, {12, 22, 255}, {88, 118, 255}, {9, 70, 255}, {90, 70, 255}, {8, 70, 0}, {5, 70, 0}, {50, 5, 0}, {95, 70, 0}, {50, 130, 0}} { // 9 и 90 — кайма 1 px под рамочку
		if a := out.NRGBAAt(c.x, c.y).A; a != c.want {
			t.Errorf("(%d,%d): %d, ждали %d", c.x, c.y, a, c.want)
		}
	}
	for _, empty := range []*image.NRGBA{
		StretchWindow(nil, cut, image.Pt(10, 10), image.Rect(0, 0, 5, 5)),
		StretchWindow(m, cut, image.Pt(10, 10), image.Rectangle{}),
	} {
		for _, v := range empty.Pix {
			if v != 0 {
				t.Fatal("пустое окно закрашено")
			}
		}
	}
}

// stretchWindowRef — StretchWindow до 079-40 дословно (маска через RGBA и xdraw.ApproxBiLinear):
// эталон для сверки нового тела без промежуточного RGBA.
func stretchWindowRef(m *image.Alpha, cut [4]float64, size image.Point, win image.Rectangle) *image.NRGBA {
	dst := image.NewRGBA(image.Rect(0, 0, max(size.X, 0), max(size.Y, 0)))
	out := &image.NRGBA{Pix: dst.Pix, Stride: dst.Stride, Rect: dst.Rect}
	win = win.Intersect(dst.Rect)
	if m == nil || win.Empty() {
		return out
	}
	cut = ClampCut(cut)
	mw, mh := m.Rect.Dx(), m.Rect.Dy()
	keep := image.Rect(int(cut[0]*float64(mw)+0.5), int(cut[1]*float64(mh)+0.5), mw-int(cut[2]*float64(mw)+0.5), mh-int(cut[3]*float64(mh)+0.5)).
		Intersect(image.Rect(0, 0, mw, mh))
	if keep.Empty() {
		return out
	}
	sr := keep.Inset(-1).Intersect(image.Rect(0, 0, mw, mh))
	kx, ky := float64(win.Dx())/float64(keep.Dx()), float64(win.Dy())/float64(keep.Dy())
	dr := image.Rect(win.Min.X-int(math.Round(kx*float64(keep.Min.X-sr.Min.X))), win.Min.Y-int(math.Round(ky*float64(keep.Min.Y-sr.Min.Y))),
		win.Max.X+int(math.Round(kx*float64(sr.Max.X-keep.Max.X))), win.Max.Y+int(math.Round(ky*float64(sr.Max.Y-keep.Max.Y)))).Intersect(dst.Rect)
	src := image.NewRGBA(image.Rect(0, 0, sr.Dx(), sr.Dy()))
	for y := range sr.Dy() {
		row := m.Pix[m.PixOffset(m.Rect.Min.X+sr.Min.X, m.Rect.Min.Y+sr.Min.Y+y):][:sr.Dx()]
		for x, a := range row {
			src.Pix[y*src.Stride+4*x+3] = a
		}
	}
	xdraw.ApproxBiLinear.Scale(dst, dr, src, src.Bounds(), xdraw.Src, nil)
	return out
}

// TestStretchWindowAlpha — новое тело (079-40) байт в байт как прежнее через RGBA: и NRGBA для
// /media/f, и Alpha для печати. Мягкая маска со случайной альфой, срезы, растяжение и сжатие, окно
// за краем холста, равные размеры (путь копии), маска-подкартинка с ненулевым началом, пустое.
func TestStretchWindowAlpha(t *testing.T) {
	rng := rand.New(rand.NewPCG(79, 40))
	type tc struct {
		m    *image.Alpha
		cut  [4]float64
		size image.Point
		win  image.Rectangle
	}
	soft := func(w, h int) *image.Alpha {
		m := image.NewAlpha(image.Rect(0, 0, w, h))
		for i := range m.Pix {
			m.Pix[i] = uint8(rng.IntN(256))
		}
		return m
	}
	big := soft(90, 70)
	cases := []tc{
		{soft(50, 40), [4]float64{0.2, 0.2, 0.2, 0.2}, image.Pt(40, 34), image.Rect(5, 5, 35, 29)}, // равные размеры
		{big.SubImage(image.Rect(13, 9, 77, 61)).(*image.Alpha), [4]float64{0.1, 0.15, 0.2, 0.05}, image.Pt(300, 200), image.Rect(20, 10, 280, 190)},
		{soft(40, 40), [4]float64{0.3, 0.3, 0.3, 0.3}, image.Pt(60, 60), image.Rect(-10, -5, 70, 50)}, // за краем
		{soft(200, 160), [4]float64{0.1, 0.1, 0.1, 0.1}, image.Pt(30, 25), image.Rect(3, 3, 27, 22)},  // сжатие
		{soft(1, 1), [4]float64{}, image.Pt(10, 7), image.Rect(0, 0, 10, 7)},
		{nil, [4]float64{0.1, 0.1, 0.1, 0.1}, image.Pt(10, 10), image.Rect(0, 0, 5, 5)},
		{soft(20, 20), [4]float64{0.1, 0.1, 0.1, 0.1}, image.Pt(10, 10), image.Rectangle{}},
		{soft(20, 20), [4]float64{0.1, 0.1, 0.1, 0.1}, image.Pt(0, 0), image.Rect(0, 0, 5, 5)},
	}
	for range 40 {
		var cut [4]float64
		for i := range cut {
			cut[i] = rng.Float64() * 0.4
		}
		size := image.Pt(1+rng.IntN(160), 1+rng.IntN(160))
		x0, y0 := rng.IntN(size.X+10)-10, rng.IntN(size.Y+10)-10
		cases = append(cases, tc{soft(1+rng.IntN(120), 1+rng.IntN(120)), cut, size,
			image.Rect(x0, y0, x0+1+rng.IntN(size.X+20), y0+1+rng.IntN(size.Y+20))})
	}
	for i, c := range cases {
		want := stretchWindowRef(c.m, c.cut, c.size, c.win)
		got := StretchWindow(c.m, c.cut, c.size, c.win)
		if got.Rect != want.Rect || !bytes.Equal(got.Pix, want.Pix) {
			t.Errorf("%d: NRGBA разошлась с прежней", i)
		}
		a := StretchWindowAlpha(c.m, c.cut, c.size, c.win)
		if a.Rect != want.Rect {
			t.Fatalf("%d: Alpha %v, ждали %v", i, a.Rect, want.Rect)
		}
		for j, v := range a.Pix {
			if v != want.Pix[4*j+3] {
				t.Fatalf("%d: альфа пикселя %d = %d, ждали %d", i, j, v, want.Pix[4*j+3])
			}
		}
	}
}

// TestFrameInsetsHelpers — отступы встроенной (полароид ×3,5 внизу) и зажим пары.
func TestFrameInsetsHelpers(t *testing.T) {
	ins := FrameInsets(FrameSpec{Kind: "polaroid", W: 100, H: 150, Width: 5})
	if ins != [4]float64{5, 5, 5, 17.5} {
		t.Errorf("полароид: %v", ins)
	}
	if got := FrameInsets(FrameSpec{Kind: "stitch", W: 100, H: 150}); got != [4]float64{} {
		t.Errorf("накладка: %v", got)
	}
	if got := FrameInsets(FrameSpec{Kind: "nope", W: 100, H: 150}); got != [4]float64{} {
		t.Errorf("неизвестный вид: %v", got)
	}
	if got := ClampInsets([4]float64{20, 5, 20, 5}, 40, 40); !near(got[0], 14, 1e-9) || !near(got[1], 5, 1e-9) {
		t.Errorf("зажим пары: %v", got)
	}
}
