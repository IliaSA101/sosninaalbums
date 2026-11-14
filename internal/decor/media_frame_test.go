package decor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// picGet — ServeFramePic без разбора URL httptest (мусор в строке запроса должен дойти).
func picGet(method, path, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r := &http.Request{Method: method, URL: &url.URL{Path: "/media/f/x", RawQuery: query}, Header: http.Header{}}
	ServeFramePic(rec, r, path)
	return rec
}

func decodeBody(t testing.TB, rec *httptest.ResponseRecorder) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("ответ не PNG: %v", err)
	}
	return img
}

func alphaOf(img image.Image, x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a >> 8 }

// ringFile — рамочка 120×160 с каймой 24/32/24/32 в файле (срез 0,2 / 0,2 / 0,2 / 0,2).
func ringFile(t testing.TB, dir string) (string, *image.NRGBA) {
	t.Helper()
	img := ringImage(120, 160, 24, 32, 24, 32)
	p := filepath.Join(dir, "ring.png")
	if err := os.WriteFile(p, pngBytes(t, img), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, img
}

// TestMediaFrame — /media/gen/frame (накладка, маска, плитка) через Handler и /media/f через
// ServeFramePic: размеры, прозрачность и краска, immutable у генератора и no-cache у картинки,
// кэш, HEAD, мусор → 404, паника → 404 и слот свободен.
func TestMediaFrame(t *testing.T) {
	s, _ := openTemp(t)
	h := s.Handler(nil)
	over := strings.TrimPrefix(gen.FrameQuery(gen.FrameSpec{Kind: "line", Inks: []string{"#ff0000"}, Width: 4, W: 60, H: 80}, false), gen.FramePath+"?")
	rec := get(h, http.MethodGet, gen.FramePath, over+"&px=120")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("накладка: %d %v", rec.Code, rec.Header())
	}
	img := decodeBody(t, rec)
	if b := img.Bounds(); b.Dx() != 120 || b.Dy() != 160 {
		t.Errorf("накладка %v", b)
	}
	if r, g, b, a := img.At(4, 80).RGBA(); a>>8 != 255 || r>>8 != 255 || g != 0 || b != 0 { // 4 мм × 2 px/мм: полоса красная
		t.Errorf("полоса: %v %v %v %v", r, g, b, a)
	}
	if alphaOf(img, 60, 80) != 0 || alphaOf(img, 20, 80) != 0 {
		t.Error("окно закрашено")
	}
	if _, hit := s.media.get(gen.FramePath + "?" + over + "&px=120"); !hit {
		t.Error("нет в кэше")
	}
	if rec := get(h, http.MethodHead, gen.FramePath, over+"&px=120"); rec.Code != http.StatusOK {
		t.Errorf("HEAD: %d", rec.Code)
	}
	mask := get(h, http.MethodGet, gen.FramePath, "k=soft&w=60&h=80&v=1&px=120&m=1")
	if mask.Code != http.StatusOK || alphaOf(decodeBody(t, mask), 60, 80) != 255 || alphaOf(decodeBody(t, mask), 0, 80) > 10 {
		t.Errorf("маска: %d", mask.Code)
	}
	tile := get(h, http.MethodGet, gen.FramePath, strings.TrimPrefix(s.tileURL(t, "sa:fr-mat"), gen.FramePath+"?")+"&px=128")
	if b := decodeBody(t, tile).Bounds(); tile.Code != http.StatusOK || b.Dx() != 128 || b.Dy() != 128 {
		t.Errorf("плитка: %d %v", tile.Code, b)
	}
	for _, c := range []struct{ method, query string }{
		{http.MethodPost, over + "&px=120"}, {http.MethodGet, over}, {http.MethodGet, over + "&px=15"}, {http.MethodGet, over + "&px=4097"},
		{http.MethodGet, over + "&px=120&path=C:/x"}, {http.MethodGet, strings.Replace(over, "k=line", "k=nope", 1) + "&px=120"},
		{http.MethodGet, "k=line&w=1&h=5000&v=1&px=4096"}, {http.MethodGet, over + "&m=1&px=120"}, {http.MethodGet, "%zz"},
	} {
		if rec := get(h, c.method, gen.FramePath, c.query); rec.Code != http.StatusNotFound {
			t.Errorf("%s %q: %d, ждали 404", c.method, c.query, rec.Code)
		}
	}
	// Паника рисовальщика: чистый 404 и слот genSem освобождён (иначе после стольких паник, сколько слотов, всё встало бы).
	orig := drawFrame
	t.Cleanup(func() { drawFrame = orig })
	drawFrame = func(gen.FrameRequest) image.Image { panic("сбой рамочки") }
	for range cap(genSem) + 1 {
		if rec := get(h, http.MethodGet, gen.FramePath, over+"&px=121"); rec.Code != http.StatusNotFound {
			t.Fatalf("паника: %d", rec.Code)
		}
	}
	drawFrame = orig
	if rec := get(h, http.MethodGet, gen.FramePath, over+"&px=122"); rec.Code != http.StatusOK {
		t.Errorf("после паник: %d", rec.Code)
	}
}

// tileURL — адрес плитки встроенной из каталога.
func (s *Store) tileURL(t testing.TB, id string) string {
	t.Helper()
	cat, err := s.Catalog(KindFrame, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat.Items {
		if it.ID == id {
			return it.URL
		}
	}
	t.Fatalf("нет плитки %s", id)
	return ""
}

// TestMediaFramePic — /media/f через ServeFramePic: той же пропорции — файл как есть (углы 1:1,
// середина прозрачна); размер 1,5 — кайма толще; маска окна по срезу; заменённый файл — новый
// ответ; нет файла, каталог, мусор → 404; HEAD и no-cache.
func TestMediaFramePic(t *testing.T) {
	dir := testkit.TempDir(t)
	path, src := ringFile(t, dir)
	q := "w=60&h=80&c=0.2,0.2,0.2,0.2&v=1&px=120"
	rec := picGet(http.MethodGet, path, q)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("Last-Modified") == "" {
		t.Fatalf("накладка: %d %v", rec.Code, rec.Header())
	}
	img := decodeBody(t, rec)
	if img.Bounds() != image.Rect(0, 0, 120, 160) {
		t.Fatalf("размер %v", img.Bounds())
	}
	for _, p := range [][2]int{{0, 0}, {10, 10}, {23, 80}, {24, 80}, {60, 80}, {100, 150}, {119, 159}, {60, 5}} {
		wr, wg, wb, wa := src.At(p[0], p[1]).RGBA()
		gr, gg, gb, ga := img.At(p[0], p[1]).RGBA()
		if wr != gr || wg != gg || wb != gb || wa != ga {
			t.Errorf("(%d,%d): %v, ждали %v", p[0], p[1], img.At(p[0], p[1]), src.At(p[0], p[1]))
		}
	}
	if rec := picGet(http.MethodHead, path, q); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD: %d, тело %d", rec.Code, rec.Body.Len())
	}
	// Размер 1,5: кайма слева 24 px × 0,75 мм/px × 2 px/мм = 36 px.
	big := decodeBody(t, picGet(http.MethodGet, path, "w=60&h=80&s=1.5&c=0.2,0.2,0.2,0.2&v=1&px=120"))
	if alphaOf(big, 30, 80) != 255 || alphaOf(big, 41, 80) != 0 || alphaOf(src, 30, 80) != 0 { // в файле на этом месте уже окно
		t.Errorf("кайма при размере 1,5: %d / %d", alphaOf(big, 30, 80), alphaOf(big, 41, 80))
	}
	// Место шире картинки: углы 1:1, стороны растянуты — угол остаётся, середина шире.
	wide := decodeBody(t, picGet(http.MethodGet, path, "w=120&h=80&c=0.2,0.2,0.2,0.2&v=1&px=240"))
	if wide.Bounds() != image.Rect(0, 0, 240, 160) || alphaOf(wide, 10, 10) != 255 || alphaOf(wide, 120, 80) != 0 || alphaOf(wide, 120, 10) != 255 {
		t.Errorf("широкое место: %v", wide.Bounds())
	}
	// Маска окна: середина 255, кайма 0; окно = место минус отступы (12 / 16 мм = 24 / 32 px).
	m := decodeBody(t, picGet(http.MethodGet, path, q+"&m=1"))
	if m.Bounds() != image.Rect(0, 0, 120, 160) || alphaOf(m, 60, 80) != 255 || alphaOf(m, 10, 80) != 0 || alphaOf(m, 60, 10) != 0 ||
		alphaOf(m, 30, 40) != 255 || alphaOf(m, 20, 80) != 0 {
		t.Errorf("маска: %v", m.Bounds())
	}
	// Файл заменили (другой размер и время): ответ новый, не из кэша.
	other := pngBytes(t, ringImage(60, 80, 6, 8, 6, 8))
	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if again := decodeBody(t, picGet(http.MethodGet, path, "w=60&h=80&c=0.1,0.1,0.1,0.1&v=1&px=120")); alphaOf(again, 5, 80) != 255 || alphaOf(again, 18, 80) != 0 {
		t.Errorf("после замены файла: кайма %d / %d", alphaOf(again, 5, 80), alphaOf(again, 18, 80))
	}
	for _, c := range []struct{ method, path, query string }{
		{http.MethodPost, path, q}, {http.MethodGet, path, ""}, {http.MethodGet, path, q + "&x=1"}, {http.MethodGet, path, q + "&px=64"},
		{http.MethodGet, path, strings.Replace(q, "px=120", "px=4097", 1)}, {http.MethodGet, path, q + "&o=1&m=1"}, {http.MethodGet, path, "%zz"},
		{http.MethodGet, filepath.Join(dir, "нет.png"), q}, {http.MethodGet, dir, q}, {http.MethodGet, "", q},
	} {
		if rec := picGet(c.method, c.path, c.query); rec.Code != http.StatusNotFound {
			t.Errorf("%s %q %q: %d, ждали 404", c.method, filepath.Base(c.path), c.query, rec.Code)
		}
	}
	bad := filepath.Join(dir, "битый.png")
	if err := os.WriteFile(bad, []byte("не картинка"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := picGet(http.MethodGet, bad, q); rec.Code != http.StatusNotFound {
		t.Errorf("битый файл: %d", rec.Code)
	}
	if len(genSem) != 0 {
		t.Errorf("слоты не освобождены: %d", len(genSem))
	}
}

// TestNineVsResize — 9-срез той же пропорции ≈ imaging.Resize (± 2 уровня; решение 079-32): место
// размером картинки в масштабе k — один Draw (точно), на пиксель больше — девять частей (мягкое
// содержимое); углы при растяжении места остаются 1:1.
func TestNineVsResize(t *testing.T) {
	full := image.NewNRGBA(image.Rect(0, 0, 300, 400)) // плавный градиент с мягкой каймой
	for y := range 400 {
		for x := range 300 {
			edge := min(x, y, 299-x, 399-y)
			a := uint8(min(255, edge*255/60)) // кайма 60 px: альфа 0 → 255 к середине
			full.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / 299), uint8(y * 255 / 399), 120, 255 - a})
		}
	}
	want := imaging.Resize(full, 120, 160, imaging.Lanczos)
	cut := [4]int{24, 32, 24, 32}
	maxDiff := func(a, b image.Image) int {
		d := 0
		for y := 0; y < 160; y++ {
			for x := 0; x < 120; x++ {
				ar, ag, ab, aa := a.At(x, y).RGBA()
				br, bg, bb, ba := b.At(x, y).RGBA()
				for _, p := range [][2]uint32{{ar, br}, {ag, bg}, {ab, bb}, {aa, ba}} {
					d = max(d, absInt(int(p[0]>>8)-int(p[1]>>8)))
				}
			}
		}
		return d
	}
	for _, sz := range [][2]int{{120, 160}, {121, 161}, {119, 159}} {
		scaled := imaging.Resize(full, sz[0], sz[1], imaging.Lanczos)
		dst := image.NewRGBA(image.Rect(0, 0, 120, 160))
		gen.Nine(dst, dst.Bounds(), scaled, [4]int{cut[0] * sz[0] / 120, cut[1] * sz[1] / 160, cut[2] * sz[0] / 120, cut[3] * sz[1] / 160})
		// Премультипликация RGBA сдвигает цвет полупрозрачных на единицу-две: сравнивается по ±2 после неё же.
		tol := 2 // той же пропорции и размера — один Draw; на пиксель мимо — девять частей, билинейно
		if sz != [2]int{120, 160} {
			tol = 4
		}
		if d := maxDiff(dst, premul(want)); d > tol {
			t.Errorf("размер %v: расхождение с imaging.Resize %d > %d", sz, d, tol)
		}
	}
	// Углы 1:1 при растяжении места: левый верхний угол в 24×32 px совпадает с картинкой масштаба k.
	scaled := imaging.Resize(full, 120, 160, imaging.Lanczos)
	dst := image.NewRGBA(image.Rect(0, 0, 200, 300))
	gen.Nine(dst, dst.Bounds(), scaled, cut)
	pm := premul(scaled)
	for y := range 32 {
		for x := range 24 {
			if dst.At(x, y) != pm.At(x, y) || dst.At(199-x, 299-y) != pm.At(119-x, 159-y) {
				t.Fatalf("угол не 1:1 в (%d,%d)", x, y)
			}
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// premul — картинка как RGBA (премультипликация, как у холста печати).
func premul(src image.Image) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			out.Set(x, y, src.At(x, y))
		}
	}
	return out
}

// FuzzMediaFrameQuery — любая строка запроса к /media/f и /media/gen/frame: ответ 200 с PNG ожидаемого
// размера или 404, паника наружу не выходит, слоты свободны. Большие ответы (> 1 Мп) не рисуются —
// проверяется только разбор.
func FuzzMediaFrameQuery(f *testing.F) {
	dir := testkit.TempDir(f)
	path, _ := ringFile(f, dir)
	s, _ := openTemp2(f)
	h := s.Handler(nil)
	for _, q := range []string{
		"w=60&h=80&c=0.2,0.2,0.2,0.2&v=1&px=64", "w=60&h=80&s=1.5&c=0.2,0.2,0.2,0.2&o=1&v=1&px=64", "w=60&h=80&c=0.2,0.2,0.2,0.2&m=1&v=1&px=64",
		"w=30&h=40&s=0.5&c=0.45,0.45,0.45,0.45&v=1&px=32", "w=30&h=40&c=1e308,-1,NaN,Inf&v=1&px=64", "w=0.01&h=5000&v=1&px=16",
		"k=line&w=60&h=80&v=1&px=64", "k=soft&w=60&h=80&v=1&px=64&m=1", "k=double&i=c8a24a&bw=2.5&w=60&h=80&o=1&v=1&px=64",
		"k=mat&i=fbf8f1,c8a24a&bw=8&w=60&h=80&rd=10&v=1&px=64", "k=polaroid&t=1&v=1&px=64", "k=gold&i=ecd27a,8f6b22&t=1&v=1&px=96",
		"", "%zz", "px=64", "w=1&h=1&v=1&px=99999999999999999999", "k=line&w=NaN&h=1&v=1&px=64",
	} {
		f.Add(q)
	}
	f.Fuzz(func(t *testing.T, q string) {
		check := func(code int, rec *httptest.ResponseRecorder, w, hh int) {
			if code != http.StatusOK && code != http.StatusNotFound {
				t.Fatalf("код %d для %q", code, q)
			}
			if code == http.StatusOK && rec.Body.Len() > 0 {
				if b := decodeBody(t, rec).Bounds(); b.Dx() != w || b.Dy() != hh {
					t.Fatalf("размер ответа %v, ждали %d×%d для %q", b, w, hh, q)
				}
			}
		}
		if req, ok := gen.ParseFramePic(q); ok {
			if w, hh := req.Size(); w*hh <= 1<<20 {
				rec := picGet(http.MethodGet, path, q)
				check(rec.Code, rec, w, hh)
			}
		} else if rec := picGet(http.MethodGet, path, q); rec.Code != http.StatusNotFound {
			t.Fatalf("мусор принят /media/f: %q → %d", q, rec.Code)
		}
		if req, ok := gen.ParseFrame(q); ok {
			if w, hh := req.Size(); w*hh <= 1<<20 {
				rec := get(h, http.MethodGet, gen.FramePath, q)
				check(rec.Code, rec, w, hh)
			}
		} else if rec := get(h, http.MethodGet, gen.FramePath, q); rec.Code != http.StatusNotFound {
			t.Fatalf("мусор принят /media/gen/frame: %q → %d", q, rec.Code)
		}
		if len(genSem) != 0 {
			t.Fatalf("слот не освобождён после %q", q)
		}
	})
}

// openTemp2 — openTemp для fuzz (testing.F): Open в чистой папке.
func openTemp2(f *testing.F) (*Store, string) {
	f.Helper()
	cfg := testkit.TempDir(f)
	return Open(cfg), cfg
}

// TestMediaFrameFast — плитка встроенной 256 px у каждого вида быстрее Limit(30 ms); картинка
// (источник в памяти) на месте 30×40 мм, 256 px — быстрее Limit(60 ms) (design §12).
func TestMediaFrameFast(t *testing.T) {
	testkit.CostTest(t)
	s, _ := openTemp(t)
	for _, p := range framePresets {
		_, q, _ := strings.Cut(s.tileURL(t, p.id), "?")
		req, ok := gen.ParseFrame(q + "&px=256")
		if !ok {
			t.Fatalf("%s: плитка не читается", p.id)
		}
		var data []byte
		d := testkit.Fastest(5, func() { data, _ = encodePNG(req.Draw()) })
		if limit := testkit.Limit(30 * time.Millisecond); d > limit {
			t.Errorf("%s: плитка %v, предел %v", p.id, d, limit)
		}
		if len(data) == 0 {
			t.Errorf("%s: пусто", p.id)
		}
	}
	dir := testkit.TempDir(t)
	big := ringImage(3000, 4000, 420, 560, 420, 560)
	path := filepath.Join(dir, "big.png")
	if err := os.WriteFile(path, pngBytes(t, big), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := picGet(http.MethodGet, path, "w=30&h=40&c=0.14,0.14,0.14,0.14&v=1&px=256"); rec.Code != http.StatusOK { // греет источник
		t.Fatalf("прогрев: %d", rec.Code)
	}
	i := 0
	d := testkit.Fastest(5, func() {
		i++ // другой адрес: ответ не из кэша, источник — из памяти
		if rec := picGet(http.MethodGet, path, fmt.Sprintf("w=30&h=40&c=0.14,0.14,0.14,0.14&v=%d&px=256", i)); rec.Code != http.StatusOK {
			t.Errorf("код %d", rec.Code)
		}
	})
	if limit := testkit.Limit(60 * time.Millisecond); d > limit {
		t.Errorf("картинка 256 px (тёплый источник) %v, предел %v", d, limit)
	}
}

// TestFramePicAreaCapNoRing — /media/f на месте 600×300 мм при размере 2 и px 4096: копия больше
// framePicArea уменьшается, а толщины рамки — k·срез, как окно маски (079-40). По средней строке и
// столбцу нет пикселя, где прозрачны и накладка, и маска (кольцо страницы между рамкой и фото).
func TestFramePicAreaCapNoRing(t *testing.T) {
	src := &picSource{img: ringImage(400, 200, 40, 40, 40, 40), w: 400, h: 200}
	req := gen.FramePicRequest{W: 600, H: 300, Scale: 2, Cut: [4]float64{0.1, 0.2, 0.1, 0.2}, Px: 4096}
	if pw, ph := req.Size(); float64(pw*ph)*req.Scale*req.Scale <= framePicArea {
		t.Fatalf("предел площади не сработает: %d×%d", pw, ph)
	}
	ov := picOverlay(src, req)
	req.Mask = true
	mask := picMask(src, req)
	b := ov.Bounds()
	gap := func(x, y int) bool { return ov.RGBAAt(x, y).A < 128 && mask.NRGBAAt(x, y).A < 128 }
	for x := range b.Dx() {
		if gap(x, b.Dy()/2) {
			t.Fatalf("кольцо по горизонтали: x %d", x)
		}
	}
	for y := range b.Dy() {
		if gap(b.Dx()/2, y) {
			t.Fatalf("кольцо по вертикали: y %d", y)
		}
	}
}
