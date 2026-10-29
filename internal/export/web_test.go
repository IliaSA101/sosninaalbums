package export

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var (
	white  = color.RGBA{255, 255, 255, 255}
	red    = color.RGBA{220, 30, 30, 255}
	green  = color.RGBA{30, 200, 30, 255}
	blue   = color.RGBA{30, 30, 220, 255}
	cream  = color.RGBA{250, 235, 205, 255}
	yellow = color.RGBA{230, 220, 20, 255}
)

// webSpec100 — страница 100×60 мм, 254 dpi (10 px на мм: границы считаются в уме), обрез bx/by.
func webSpec100(bx, by float64) WebSpec {
	return WebSpec{DPI: 254, PageW: 100, PageH: 60, BleedX: bx, BleedY: by}
}

// webDir — пустая папка img, как её готовит служба.
func webDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(testkit.TempDir(t), "img")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func decodeJPEG(t *testing.T, path string) image.Image {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// near — цвет пикселя (x, y) картинки близок к want (JPEG вносит шум).
func nearRGB(img image.Image, x, y int, want color.RGBA) bool {
	r, g, b, _ := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
	d := func(a uint32, w uint8) int { return abs(int(a>>8) - int(w)) }
	return d(r, want.R) < 40 && d(g, want.G) < 40 && d(b, want.B) < 40
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func plate(x, y, w, h float64, c color.RGBA) render.Plate {
	return render.Plate{Rect: layout.Rect{X: x, Y: y, W: w, H: h}, Color: c}
}

func TestWebDPI(t *testing.T) {
	for _, mm := range []float64{200, 300, 400, 600, 150, 1200} {
		d := WebDPI(mm, mm/2, 0)
		px := mmPx(mm, d)
		// Округление dpi вверх: не меньше 1600 и не больше, чем даёт лишний 1 dpi.
		if px < ShareLongSide || px > ShareLongSide+int(mm/25.4)+1 {
			t.Errorf("%v мм: %d dpi, %d px", mm, d, px)
		}
	}
	for _, c := range []struct {
		mm   float64
		dpi  int
		want int
	}{{200, 204, 1606}, {300, 136, 1606}, {600, 68, 1606}} {
		if d := WebDPI(c.mm, 10, ShareLongSide); d != c.dpi || mmPx(c.mm, d) != c.want {
			t.Errorf("%v мм: %d dpi (ждали %d), %d px (ждали %d)", c.mm, d, c.dpi, mmPx(c.mm, d), c.want)
		}
	}
	if WebDPI(100, 300, 0) != WebDPI(300, 100, 0) {
		t.Error("длинная сторона — любая из двух")
	}
	if WebDPI(0, 0, 0) != 0 || WebDPI(nan(), nan(), 0) != 0 || WebDPI(1e-9, 1e-9, 0) != 65535 {
		t.Error("негодная страница: dpi 0; микроскопическая — потолок JFIF")
	}
	// Длинная сторона — параметр: 0 и минус — по умолчанию, свои значения масштабируют dpi.
	if WebDPI(300, 200, -5) != WebDPI(300, 200, ShareLongSide) || WebDPI(300, 200, 800) >= WebDPI(300, 200, 0) {
		t.Error("long ≤ 0 — ShareLongSide, меньший long — меньший dpi")
	}
	if d := WebDPI(300, 200, 800); mmPx(300, d) < 800 || mmPx(300, d) > 810 {
		t.Errorf("800 px: %d dpi, %d px", d, mmPx(300, d))
	}
}

func nan() float64 { z := 0.0; return z / z }

// Файл страницы по dpi: длинная сторона файла после обреза 1600…1610 px, соотношение — страницы.
func TestWebDPIFile(t *testing.T) {
	w := WebSpec{PageW: 303, PageH: 300, BleedX: 3}
	tw, th := w.Trimmed()
	w.DPI = WebDPI(tw, th, 0)
	dir := webDir(t)
	if _, err := RunWeb(context.Background(), []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}, nil, w, dir, nil); err != nil {
		t.Fatal(err)
	}
	cfg := jpegSize(t, filepath.Join(dir, "001.jpg"))
	if long := max(cfg.Width, cfg.Height); long < 1600 || long > 1610 {
		t.Fatalf("файл %d×%d px", cfg.Width, cfg.Height)
	}
}

// Обрез срезан с внешних краёв (слева у левой, справа у правой, сверху и снизу), у сгиба нет.
func TestWebTrimBleed(t *testing.T) {
	sp := render.Spread{Bg: [2]color.RGBA{cream, cream}, Plates: []render.Plate{
		plate(0, 0, 3, 60, red),       // внешний край слева: в обрезе
		plate(97, 0, 3, 60, blue),     // у сгиба слева: остаётся
		plate(100, 0, 3, 60, green),   // у сгиба справа: остаётся
		plate(197, 0, 3, 60, yellow),  // внешний край справа: в обрезе
		plate(10, 0, 20, 2.5, yellow), // верхний обрез: срезан
	}}
	dir := webDir(t)
	pages, err := RunWeb(context.Background(), []render.Spread{sp}, nil, webSpec100(3, 3), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0].Blank || pages[1].Blank {
		t.Fatalf("страницы %+v", pages)
	}
	l, r := decodeJPEG(t, filepath.Join(dir, "001.jpg")), decodeJPEG(t, filepath.Join(dir, "002.jpg"))
	// Страница 97×54 мм = 970×540 px.
	for _, im := range []image.Image{l, r} {
		if b := im.Bounds(); b.Dx() != 970 || b.Dy() != 540 {
			t.Fatalf("страница %v, ждали 970×540", b)
		}
	}
	if !nearRGB(l, 5, 270, cream) || !nearRGB(l, 20, 5, cream) {
		t.Error("левая: красный край или верхняя полоса не срезаны")
	}
	if !nearRGB(l, 965, 270, blue) {
		t.Error("левая: у сгиба полоса должна остаться")
	}
	if !nearRGB(r, 5, 270, green) {
		t.Error("правая: у сгиба полоса должна остаться")
	}
	if !nearRGB(r, 965, 270, cream) {
		t.Error("правая: жёлтый край не срезан")
	}
	// Оси обреза разные: 5 мм по бокам, 0 сверху — высота полная.
	dir2 := webDir(t)
	if _, err := RunWeb(context.Background(), []render.Spread{sp}, nil, webSpec100(5, 0), dir2, nil); err != nil {
		t.Fatal(err)
	}
	if cfg := jpegSize(t, filepath.Join(dir2, "001.jpg")); cfg.Width != 950 || cfg.Height != 600 {
		t.Fatalf("обрез 5/0: %dx%d", cfg.Width, cfg.Height)
	}
}

// Без обреза страницы — ровно половины холста.
func TestWebNoBleed(t *testing.T) {
	dir := webDir(t)
	if _, err := RunWeb(context.Background(), []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}, nil, webSpec100(0, 0), dir, nil); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"001.jpg", "002.jpg"} {
		if cfg := jpegSize(t, filepath.Join(dir, n)); cfg.Width != 1000 || cfg.Height != 600 {
			t.Errorf("%s: %dx%d", n, cfg.Width, cfg.Height)
		}
	}
}

// Белая страница — nil в листалке и файла нет; страница в цвете альбома — файл. Страница с
// единственным нечистым пикселем — тоже файл.
func TestWebBlankPage(t *testing.T) {
	dot := render.Spread{Bg: [2]color.RGBA{white, white}, Plates: []render.Plate{plate(50, 30, 0.2, 0.2, red)}}
	spreads := []render.Spread{
		{Bg: [2]color.RGBA{white, white}}, // обе белые
		{Bg: [2]color.RGBA{white, cream}}, // слева белая, справа цвет альбома
		{Bg: [2]color.RGBA{cream, white}}, // наоборот
		dot,                               // точка на левой
		{Bg: [2]color.RGBA{white, white}}, // ещё белый
		{Bg: [2]color.RGBA{cream, cream}}, // обе в цвете
	}
	dir := webDir(t)
	pages, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// страницы: 0,1 белые; 2 белая, 3 цвет; 4 цвет, 5 белая; 6 точка, 7 белая; 8,9 белые; 10,11 цвет.
	wantBlank := []bool{true, true, true, false, false, true, false, true, true, true, false, false}
	if len(pages) != len(wantBlank) {
		t.Fatalf("страниц %d", len(pages))
	}
	files := names(readAll(t, dir))
	var wantFiles []string
	for k, p := range pages {
		if p.Blank != wantBlank[k] {
			t.Errorf("страница %d: Blank=%v", k, p.Blank)
		}
		if !p.Blank {
			wantFiles = append(wantFiles, p.Name)
		}
	}
	slices.Sort(wantFiles)
	if !slices.Equal(files, wantFiles) {
		t.Fatalf("файлы %v, ждали %v", files, wantFiles)
	}
	if pages[0].Name != "001.jpg" || pages[11].Name != "012.jpg" {
		t.Fatalf("имена %+v", pages)
	}
}

// Белая полоса обреза не делает страницу пустой, а цветная в обрезе — не делает непустой.
func TestWebBlankIgnoresTrim(t *testing.T) {
	sp := render.Spread{Bg: [2]color.RGBA{white, white}, Plates: []render.Plate{plate(0, 0, 3, 60, red), plate(197, 0, 3, 60, red)}}
	pages, err := RunWeb(context.Background(), []render.Spread{sp}, nil, webSpec100(3, 3), webDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !pages[0].Blank || !pages[1].Blank {
		t.Fatalf("цвет только в обрезе: страницы должны быть белыми, %+v", pages)
	}
}

// Имена по порядку книги: с обложкой 000…, без — 001…; ширина растёт после 999.
func TestWebPageNames(t *testing.T) {
	if got := WebPageNames(2, true); !slices.Equal(got, []string{"000.jpg", "001.jpg", "002.jpg", "003.jpg", "004.jpg", "005.jpg"}) {
		t.Errorf("с обложкой: %v", got)
	}
	if got := WebPageNames(2, false); !slices.Equal(got, []string{"001.jpg", "002.jpg", "003.jpg", "004.jpg"}) {
		t.Errorf("без обложки: %v", got)
	}
	if got := WebPageNames(0, false); len(got) != 0 {
		t.Errorf("пустой альбом: %v", got)
	}
	if got := WebPageNames(0, true); !slices.Equal(got, []string{"000.jpg", "001.jpg"}) {
		t.Errorf("только обложка: %v", got)
	}
	big := WebPageNames(500, true) // 1002 страницы: 0…1001
	if big[0] != "0000.jpg" || big[1001] != "1001.jpg" || len(big) != 1002 {
		t.Errorf("шире 999: %s … %s (%d)", big[0], big[len(big)-1], len(big))
	}
	if edge := WebPageNames(499, true); edge[999] != "999.jpg" || edge[0] != "000.jpg" { // ровно 1000 страниц: 0…999
		t.Errorf("до 999 включительно — 3 цифры: %s", edge[999])
	}
}

// Несколько разворотов: страницы в порядке книги, у каждой свой цвет; прогресс — по листам.
func TestWebSeveralSpreads(t *testing.T) {
	cols := []color.RGBA{red, green, blue, yellow, cream, white}
	var spreads []render.Spread
	for i := 0; i < len(cols); i += 2 {
		spreads = append(spreads, render.Spread{Bg: [2]color.RGBA{cols[i], cols[i+1]}})
	}
	dir := webDir(t)
	var done, totals []int
	pages, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), dir, func(d, tot int) { done, totals = append(done, d), append(totals, tot) })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []int{1, 2, 3}) || !slices.Equal(totals, []int{3, 3, 3}) {
		t.Fatalf("прогресс %v из %v", done, totals)
	}
	for k, p := range pages {
		if k == 5 {
			if !p.Blank {
				t.Error("последняя белая страница должна быть пустой")
			}
			continue
		}
		if !nearRGB(decodeJPEG(t, filepath.Join(dir, p.Name)), 100, 100, cols[k]) {
			t.Errorf("страница %d (%s) не того цвета", k, p.Name)
		}
	}
}

// Пустой альбом и один разворот: список по числу страниц, ничего лишнего в папке.
func TestWebEmptyAndSingle(t *testing.T) {
	dir := webDir(t)
	pages, err := RunWeb(context.Background(), nil, nil, webSpec100(3, 3), dir, nil)
	if err != nil || len(pages) != 0 || len(readAll(t, dir)) != 0 {
		t.Fatalf("пустой альбом: %v %v", pages, err)
	}
	pages, err = RunWeb(context.Background(), []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}, nil, webSpec100(3, 3), dir, nil)
	if err != nil || len(pages) != 2 || !slices.Equal(names(readAll(t, dir)), []string{"001.jpg", "002.jpg"}) {
		t.Fatalf("один разворот: %+v %v", pages, err)
	}
}

// Обложка: лицевая — зона front, задняя — back; каждая в своём файле, без корешка и заворота.
func TestWebCoverSides(t *testing.T) {
	front := layout.Rect{X: 120, Y: 10, W: 90, H: 70}
	back := layout.Rect{X: 5, Y: 10, W: 90, H: 70}
	w := WebSpec{DPI: 254, PageW: 100, PageH: 60, BleedX: 3, BleedY: 3, Front: front, Back: back}
	sheet := render.Spread{Bg: [2]color.RGBA{cream, cream}, Plates: []render.Plate{
		plate(back.X, back.Y, back.W, back.H, blue),
		plate(front.X, front.Y, front.W, front.H, red),
		plate(95, 0, 25, 90, green), // корешок между сторонками
	}}
	job := &CoverJob{Spread: sheet, Opt: render.Options{PageW: 107.5, PageH: 90, DPI: 20, Quality: 95}}
	dir := webDir(t)
	var done, totals []int
	pages, err := RunWeb(context.Background(), []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}, job, w, dir, func(d, tot int) { done, totals = append(done, d), append(totals, tot) })
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 4 || pages[0].Name != "000.jpg" || pages[3].Name != "003.jpg" || pages[0].Blank || pages[3].Blank {
		t.Fatalf("страницы %+v", pages)
	}
	if !slices.Equal(done, []int{1, 2}) || totals[0] != 2 {
		t.Errorf("прогресс %v из %v: обложка — один лист", done, totals)
	}
	f, b := decodeJPEG(t, filepath.Join(dir, "000.jpg")), decodeJPEG(t, filepath.Join(dir, "003.jpg"))
	if fb := f.Bounds(); fb.Dx() != 900 || fb.Dy() != 700 {
		t.Errorf("лицевая %v: размер зоны, а не страницы", fb)
	}
	if bb := b.Bounds(); bb.Dx() != 900 || bb.Dy() != 700 {
		t.Errorf("задняя %v", bb)
	}
	for _, p := range [][2]int{{5, 5}, {450, 350}, {895, 695}, {5, 695}, {895, 5}} {
		if !nearRGB(f, p[0], p[1], red) {
			t.Errorf("лицевая в %v не красная: корешок или заворот попал в кадр", p)
		}
		if !nearRGB(b, p[0], p[1], blue) {
			t.Errorf("задняя в %v не синяя", p)
		}
	}
	// Рендер обложки идёт на dpi листалки, а не на dpi задания: те же пиксели, что и для страниц.
	if jpegSize(t, filepath.Join(dir, "001.jpg")).Width != 970 {
		t.Error("страница разворота рядом с обложкой — 970 px")
	}
}

// Обложка без фото: страницы белые — nil, файлов нет, а список по-прежнему 2N+2.
func TestWebCoverWithoutPhotos(t *testing.T) {
	p, v := coverEditor(t)
	w, err := NewWebSpec(p.Settings, v)
	if err != nil {
		t.Fatal(err)
	}
	v.Cover.SpineBg = "" // плашки корешка нет: сторонки пустые
	job := NewCoverJob(p, v, w.DPI, 90)
	dir := webDir(t)
	pages, err := RunWeb(context.Background(), FromView(p, v), job, w, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 8 {
		t.Fatalf("3 разворота и обложка: %d страниц", len(pages))
	}
	for k, pg := range pages {
		if !pg.Blank {
			t.Errorf("страница %d (%s): пустой альбом белый, ждали nil", k, pg.Name)
		}
	}
	if len(readAll(t, dir)) != 0 {
		t.Errorf("файлы пустых страниц: %v", names(readAll(t, dir)))
	}
}

// NewWebSpec: обрез и сторонки — из тех же данных, что у вида (View.Bleed и CoverZones).
func TestNewWebSpec(t *testing.T) {
	p, _ := coverEditor(t)
	p.Settings.PageW, p.Settings.PageH, p.Settings.Bleed = 123, 90, 3
	e := mustEditor(t, p, "")
	p, v := e.Snapshot()
	w, err := NewWebSpec(p.Settings, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Bleed) != 4 || v.Bleed[0].H != w.BleedY || v.Bleed[2].W != w.BleedX {
		t.Fatalf("обрез спецификации %v/%v, розовые полосы %+v", w.BleedX, w.BleedY, v.Bleed)
	}
	tw, th := w.Trimmed()
	if tw != 120 || th != 84 || w.DPI != WebDPI(tw, th, ShareLongSide) || w.Quality != WebQuality || w.LongSide != ShareLongSide {
		t.Fatalf("после обреза %v×%v, dpi %d", tw, th, w.DPI)
	}
	var front, back layout.Rect
	for _, z := range v.Cover.Zones {
		switch z.Kind {
		case "front":
			front = z.Rect
		case "back":
			back = z.Rect
		}
	}
	if front.W == 0 || w.Front != front || w.Back != back {
		t.Fatalf("сторонки %+v / %+v, зоны %+v / %+v", w.Front, w.Back, front, back)
	}
	// Без обложки сторонки не нужны.
	if w2, err := NewWebSpec(p.Settings, model.View{}); err != nil || w2.Front != (layout.Rect{}) {
		t.Fatalf("без обложки: %+v %v", w2, err)
	}
	// Обложка без сторонок и страница, съеденная обрезом, — ошибка.
	if _, err := NewWebSpec(p.Settings, model.View{Cover: &model.CoverView{}}); err == nil {
		t.Error("обложка без сторонок принята")
	}
	if _, err := NewWebSpec(model.Settings{PageW: 5, PageH: 5, Bleed: 3}, model.View{}); err == nil {
		t.Error("страница, съеденная обрезом, принята")
	}
}

// В файле только JFIF, COM-метка программы и ICC sRGB: ни EXIF, ни XMP, ни GPS исходников.
func TestWebJPEGMeta(t *testing.T) {
	photo := testkit.JPEGOriented(t, testkit.TempDir(t), "src.jpg", 400, 300, 6)
	r := layout.Rect{X: 20, Y: 10, W: 60, H: 40}
	sp := render.Spread{Photos: []render.Photo{{Cell: r, Img: r, Path: photo}}}
	dir := webDir(t)
	if _, err := RunWeb(context.Background(), []render.Spread{sp}, nil, webSpec100(3, 3), dir, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "001.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for i := 2; i+4 <= len(b) && b[i] == 0xFF; { // маркеры до SOS (0xDA)
		m := b[i+1]
		if m == 0xDA {
			break
		}
		got = append(got, m)
		i += 2 + int(b[i+2])<<8 + int(b[i+3])
	}
	if len(got) < 3 || got[0] != 0xE0 || got[1] != 0xFE || got[2] != 0xE2 {
		t.Fatalf("маркеры до данных: % x, ждали E0 (JFIF), FE (COM), E2 (ICC) первыми", got)
	}
	for _, m := range got {
		if m == 0xE1 || m == 0xED || m == 0xEB { // EXIF/XMP, Photoshop IRB, APP11
			t.Errorf("лишний сегмент метаданных %02X", m)
		}
	}
	if !IsOurs(filepath.Join(dir, "001.jpg")) {
		t.Error("нет COM-метки программы")
	}
	if bytes.Contains(b, []byte("Exif")) || bytes.Contains(b, []byte("GPS")) {
		t.Error("в файле есть Exif/GPS")
	}
	if !bytes.Equal(b[6:11], []byte("JFIF\x00")) {
		t.Errorf("JFIF: % x", b[6:11])
	}
	dpi := int(b[14])<<8 | int(b[15])
	if dpi != 254 {
		t.Errorf("dpi в JFIF %d, ждали 254", dpi)
	}
}

// Отмена: готовые листы остаются, недорисованный не появляется, ошибка — context.Canceled.
func TestWebStop(t *testing.T) {
	spreads := []render.Spread{{Bg: [2]color.RGBA{cream, cream}}, {Bg: [2]color.RGBA{red, red}}, {Bg: [2]color.RGBA{blue, blue}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := webDir(t)
	pages, err := RunWeb(ctx, spreads, nil, webSpec100(3, 3), dir, func(d, _ int) {
		if d == 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || pages != nil {
		t.Fatalf("отмена: %v %v", pages, err)
	}
	if got := names(readAll(t, dir)); !slices.Equal(got, []string{"001.jpg", "002.jpg"}) {
		t.Fatalf("после отмены на первом листе: %v", got)
	}
	// Отмена на последнем листе тоже не отдаёт книгу: index.html не писать.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := RunWeb(ctx2, spreads[:1], nil, webSpec100(3, 3), webDir(t), func(int, int) { cancel2() }); !errors.Is(err, context.Canceled) {
		t.Fatalf("отмена после последнего: %v", err)
	}
	// Уже отменённый — ничего не пишется.
	d3 := webDir(t)
	if _, err := RunWeb(ctx, spreads, nil, webSpec100(3, 3), d3, nil); !errors.Is(err, context.Canceled) || len(readAll(t, d3)) != 0 {
		t.Fatalf("отменённый заранее: %v", err)
	}
}

// Сбой записи второй страницы листа убирает и первую; паника рендера и кодера — ошибка.
func TestWebFailures(t *testing.T) {
	spreads := []render.Spread{{Bg: [2]color.RGBA{cream, cream}}, {Bg: [2]color.RGBA{red, red}}}
	orig := saveSheet
	t.Cleanup(func() { saveSheet = orig })
	n := 0
	saveSheet = func(img image.Image, path string, opt render.Options) error {
		if n++; n == 4 { // вторая страница второго листа
			return errors.New("диск полон")
		}
		return orig(img, path, opt)
	}
	dir := webDir(t)
	_, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), dir, nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("разворот 2")) {
		t.Fatalf("сбой записи: %v", err)
	}
	if got := names(readAll(t, dir)); !slices.Equal(got, []string{"001.jpg", "002.jpg"}) {
		t.Fatalf("недописанный лист не убран: %v", got)
	}
	saveSheet = func(image.Image, string, render.Options) error { panic("кодер") }
	if _, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), webDir(t), nil); err == nil {
		t.Fatal("паника кодера не стала ошибкой")
	}
	saveSheet = orig
	origR := renderSheet
	t.Cleanup(func() { renderSheet = origR })
	renderSheet = func(context.Context, render.Spread, render.Options) (*image.RGBA, error) { panic("рендер") }
	if _, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), webDir(t), nil); err == nil {
		t.Fatal("паника рендера не стала ошибкой")
	}
}

// Негодные входы — ошибка до записи: параметры, папка (нет, файл, ссылка), огромный холст.
func TestWebBadInput(t *testing.T) {
	sp := []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}
	good := webSpec100(3, 3)
	for name, w := range map[string]WebSpec{
		"dpi 0":         {PageW: 100, PageH: 60},
		"страница 0":    {DPI: 100},
		"обрез съел":    {DPI: 100, PageW: 100, PageH: 60, BleedX: 100},
		"обрез минус":   {DPI: 100, PageW: 100, PageH: 60, BleedX: -1},
		"высоту съел":   {DPI: 100, PageW: 100, PageH: 60, BleedY: 30},
		"NaN":           {DPI: 100, PageW: nan(), PageH: 60},
		"холст огромен": {DPI: 65535, PageW: 100000, PageH: 100000},
	} {
		dir := webDir(t)
		if _, err := RunWeb(context.Background(), sp, nil, w, dir, nil); err == nil {
			t.Errorf("%s: принято", name)
		}
		if len(readAll(t, dir)) != 0 {
			t.Errorf("%s: что-то записано", name)
		}
	}
	if _, err := RunWeb(context.Background(), sp, &CoverJob{}, good, webDir(t), nil); err == nil {
		t.Error("обложка без сторонок принята")
	}
	base := testkit.TempDir(t)
	file := filepath.Join(base, "img")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"нет папки": filepath.Join(base, "нет"), "файл": file} {
		if _, err := RunWeb(context.Background(), sp, nil, good, dir, nil); err == nil {
			t.Errorf("%s: принято", name)
		}
	}
	target := webDir(t)
	link := filepath.Join(testkit.TempDir(t), "img")
	if err := os.Symlink(target, link); err == nil {
		if _, err := RunWeb(context.Background(), sp, nil, good, link, nil); err == nil || len(readAll(t, target)) != 0 {
			t.Errorf("ссылка вместо img принята: %v", err)
		}
	}
}

// Файлов страниц столько, сколько непустых; прогресс nil допустим; оставшиеся чужие файлы не трогаем.
func TestWebKeepsForeign(t *testing.T) {
	dir := webDir(t)
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("моё"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWeb(context.Background(), []render.Spread{{Bg: [2]color.RGBA{cream, white}}}, nil, webSpec100(3, 3), dir, nil); err != nil {
		t.Fatal(err)
	}
	if got := names(readAll(t, dir)); !slices.Equal(got, []string{"001.jpg", "notes.txt"}) {
		t.Fatalf("файлы %v", got)
	}
}
