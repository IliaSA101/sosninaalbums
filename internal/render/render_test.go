package render

import (
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// coverPhoto — фото в ячейке при S=1: кадр закрывает ячейку и центрирован
// (то же, что считает geom.PlaceImage).
func coverPhoto(cell layout.Rect, aspect float64, path string) Photo {
	return framedPhoto(cell, aspect, 1, 0, 0, path)
}

func framedPhoto(cell layout.Rect, aspect, s, fx, fy float64, path string) Photo {
	w, h := cell.W, cell.W/aspect
	if h < cell.H {
		h, w = cell.H, cell.H*aspect
	}
	w, h = w*s, h*s
	img := layout.Rect{X: cell.X - (w-cell.W)*(1+fx)/2, Y: cell.Y - (h-cell.H)*(1+fy)/2, W: w, H: h}
	return Photo{Cell: cell, Img: img, Path: path}
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// S=1: окно кадра — прежний Fill(Center), для обычной ячейки и для ячейки «в край».
func TestRenderFrameDefaultMatchesFill(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	src, release, err := imgfile.OpenOriented(ph)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	opt := Options{PageW: 100, PageH: 60, DPI: 100}
	for _, cell := range []layout.Rect{{X: 10, Y: 10, W: 45, H: 30}, {X: 120, Y: 5, W: 60, H: 20}} {
		img, err := Render(Spread{Photos: []Photo{coverPhoto(cell, 1.5, ph)}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		x0, y0 := px(cell.X, 100), px(cell.Y, 100)
		x1, y1 := px(cell.X+cell.W, 100), px(cell.Y+cell.H, 100)
		want := imaging.Fill(src, x1-x0, y1-y0, imaging.Center, imaging.Lanczos)
		var sum, n int
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				a, b := img.RGBAAt(x, y), want.NRGBAAt(x-x0, y-y0)
				for _, d := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
					sum += max(d, -d)
					n++
				}
			}
		}
		if mean := float64(sum) / float64(n); mean >= 1 {
			t.Fatalf("ячейка %v: средняя разница с Fill %.2f/255", cell, mean)
		}
	}
}

// quadrants пишет JPEG из четырёх плоских цветных квадрантов.
func quadrants(t *testing.T, dir string, cols [4]color.RGBA) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			im.SetRGBA(x, y, cols[(y/100)*2+x/100])
		}
	}
	path := filepath.Join(dir, "q.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, im, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// S=2 и сдвиг в угол: в ячейке виден один квадрант.
func TestRenderFrameZoom(t *testing.T) {
	red, grn := color.RGBA{220, 20, 20, 255}, color.RGBA{20, 200, 20, 255}
	blu, yel := color.RGBA{20, 20, 220, 255}, color.RGBA{220, 220, 20, 255}
	ph := quadrants(t, testkit.TempDir(t), [4]color.RGBA{red, grn, blu, yel})
	opt := Options{PageW: 100, PageH: 100, DPI: 100}
	cell := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	for _, c := range []struct {
		fx, fy float64
		want   color.RGBA
	}{{-1, -1, red}, {1, -1, grn}, {-1, 1, blu}, {1, 1, yel}} {
		img, err := Render(Spread{Photos: []Photo{framedPhoto(cell, 1, 2, c.fx, c.fy, ph)}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		x0, y0, x1, y1 := px(20, 100), px(20, 100), px(60, 100), px(60, 100)
		for _, pt := range [][2]int{{x0 + 2, y0 + 2}, {x1 - 3, y1 - 3}, {(x0 + x1) / 2, (y0 + y1) / 2}} {
			if got := img.RGBAAt(pt[0], pt[1]); !near(got, c.want, 12) {
				t.Fatalf("кадр (%v,%v), точка %v: цвет %v, ждали %v", c.fx, c.fy, pt, got, c.want)
			}
		}
	}
}

func TestRenderBackgroundPages(t *testing.T) {
	red, blu := color.RGBA{200, 0, 0, 255}, color.RGBA{0, 0, 200, 255}
	opt := Options{PageW: 50, PageH: 40, DPI: 100}
	img, err := Render(Spread{Bg: [2]color.RGBA{red, blu}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	mid := px(50, 100)
	for _, c := range []struct {
		x    int
		want color.RGBA
	}{{0, red}, {mid - 1, red}, {mid, blu}, {img.Bounds().Dx() - 1, blu}} {
		if got := img.RGBAAt(c.x, 5); got != c.want {
			t.Fatalf("x=%d: %v, ждали %v", c.x, got, c.want)
		}
	}
	// Цвет не задан — страница белая.
	img, err = Render(Spread{}, opt)
	if err != nil || img.RGBAAt(3, 3) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("пустой фон: %v, %v", img.RGBAAt(3, 3), err)
	}
}

// Прозрачный PNG ложится на фон страницы (а не чёрным): прозрачная половина показывает цвет
// страницы, закрашенная — свой цвет.
func TestRenderAlphaOnBg(t *testing.T) {
	dir := testkit.TempDir(t)
	logo := testkit.PNG(t, dir, "logo.png", 100, 60, color.NRGBA{200, 30, 30, 255})
	bg := color.RGBA{0xf6, 0xf1, 0xe7, 255}
	opt := Options{PageW: 100, PageH: 60, DPI: 100}
	cell := layout.Rect{X: 10, Y: 10, W: 60, H: 36}
	img, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Photos: []Photo{coverPhoto(cell, 100.0/60, logo)}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	x0, y0 := px(cell.X, 100), px(cell.Y, 100)
	x1, y1 := px(cell.X+cell.W, 100), px(cell.Y+cell.H, 100)
	cy := (y0 + y1) / 2
	if got := img.RGBAAt(x0+3, cy); !near(got, color.RGBA{200, 30, 30, 255}, 3) {
		t.Errorf("закрашенная половина %v", got)
	}
	if got := img.RGBAAt(x1-3, cy); got != bg {
		t.Errorf("прозрачная половина %v, ждали фон %v (не чёрный)", got, bg)
	}
	if got := img.RGBAAt(x1+5, cy); got != bg {
		t.Errorf("вне ячейки %v, ждали фон", got)
	}
}

// contain: фото вписано в ячейку по центру, поля — цвет страницы, а не растянутая или чёрная заливка.
func TestRenderContain(t *testing.T) {
	dir := testkit.TempDir(t)
	logo := testkit.PNG(t, dir, "logo.png", 100, 60, color.NRGBA{200, 30, 30, 255})
	bg := color.RGBA{0xf6, 0xf1, 0xe7, 255}
	opt := Options{PageW: 100, PageH: 80, DPI: 100}
	cell := layout.Rect{X: 10, Y: 10, W: 60, H: 60} // квадратное место, фото 100:60 — 60x36 мм
	ph := Photo{Cell: cell, Img: layout.Rect{X: 10, Y: 22, W: 60, H: 36}, Path: logo}
	img, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Photos: []Photo{ph}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	cx := px(20, 100) // PNG из testkit — красная левая половина, правая прозрачная
	for _, c := range []struct {
		name string
		y    float64
		want color.RGBA
	}{
		{"поле сверху", 12, bg}, {"фото", 40, color.RGBA{200, 30, 30, 255}}, {"поле снизу", 68, bg},
	} {
		if got := img.RGBAAt(cx, px(c.y, 100)); !near(got, c.want, 3) {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
	// По ширине фото занимает всю ячейку: у самого края уже красное.
	if got := img.RGBAAt(px(10, 100)+2, px(40, 100)); !near(got, color.RGBA{200, 30, 30, 255}, 3) {
		t.Errorf("левый край фото %v", got)
	}
}

// Места перекрываются: сверху то, что дальше в списке (как z-index на экране), при любом числе
// фото (больше слотов семафора) и без гонки. Ошибка фото в середине не вешает остальных.
func TestRenderOverlapOrder(t *testing.T) {
	dir := testkit.TempDir(t)
	const n = 8
	opt := Options{PageW: 100, PageH: 60, DPI: 100}
	photos := make([]Photo, n)
	want := make([]color.RGBA, n)
	for i := range photos {
		c := color.NRGBA{uint8(20 + 28*i), uint8(250 - 28*i), 90, 255}
		want[i] = color.RGBA{c.R, c.G, c.B, 255}
		path := testkit.PNG(t, dir, "p"+string(rune('a'+i))+".png", 100, 60, c)
		photos[i] = coverPhoto(layout.Rect{X: float64(10 + 5*i), Y: 10, W: 40, H: 24}, 100.0/60, path)
	}
	y := px(20, 100)
	for range 10 {
		img, err := Render(Spread{Photos: photos}, opt)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.RGBAAt(px(47, 100), y); !near(got, want[n-1], 2) {
			t.Fatalf("в общем углу %v, ждали верхнее (последнее) %v", got, want[n-1])
		}
		if got := img.RGBAAt(px(12, 100), y); !near(got, want[0], 2) {
			t.Fatalf("только первое: %v, ждали %v", got, want[0])
		}
	}
	bad := slices.Clone(photos)
	bad[3].Path = filepath.Join(dir, "нет.png")
	if _, err := Render(Spread{Photos: bad}, opt); err == nil {
		t.Fatal("ждали ошибку про отсутствующий файл")
	}
}

func textBlock(s string, x, y, w float64, st text.Style) TextDraw {
	return TextDraw{X: x, Y: y, Box: text.Layout(s, st, w), Style: st}
}

// TestTextRelayoutOnFontGen — набор шрифтов сменился после снимка (Gen запомнен и не совпал):
// строки разбиваются заново из Text; совпал или не запомнен (0) — Box рисуется как есть (082).
func TestTextRelayoutOnFontGen(t *testing.T) {
	opt := Options{PageW: 80, PageH: 40, DPI: 100}
	st := text.Style{Font: "montserrat", Size: 18, Color: "#000000", Align: "left"}
	const s = "Анна и Павел"
	stale := textBlock("Ж", 5, 5, 60, st) // строки «старого» шрифта: не те, что дал бы Text
	stale.Text = s
	fresh := textBlock(s, 5, 5, 60, st)
	draw := func(td TextDraw) []byte {
		img, err := Render(Spread{Texts: []TextDraw{td}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		return img.Pix
	}
	asIs, want := draw(textBlock("Ж", 5, 5, 60, st)), draw(fresh)
	if slices.Equal(asIs, want) {
		t.Fatal("старые и новые строки рисуются одинаково — тест ничего не различает")
	}
	for _, c := range []struct {
		name string
		gen  uint64
		same []byte
	}{
		{"не запомнено", 0, asIs},
		{"набор тот же", text.Gen(), asIs},
		{"набор сменился", text.Gen() + 1, want},
	} {
		td := stale
		td.Gen = c.gen
		if got := draw(td); !slices.Equal(got, c.same) {
			t.Errorf("%s: рисунок не тот", c.name)
		}
	}
}

// inkBounds — рамка пикселей, отличных от фона bg.
func inkBounds(img *image.RGBA, bg color.RGBA) (r image.Rectangle) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != bg {
				p := image.Rect(x, y, x+1, y+1)
				if r.Empty() {
					r = p
				} else {
					r = r.Union(p)
				}
			}
		}
	}
	return r
}

// Текст лежит поверх фото и фона, чернила — внутри рамки блока.
func TestRenderText(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	opt := Options{PageW: 100, PageH: 60, DPI: 100}
	st := text.Style{Font: "montserrat", Size: 20, Color: "#ff0000", Align: "center"}
	blk := textBlock("Иван и Мария", 110, 20, 80, st)

	img, err := Render(Spread{Texts: []TextDraw{blk}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	ink := inkBounds(img, color.RGBA{255, 255, 255, 255})
	// Низ рамки с запасом на выносные элементы: базовая линия последней строки + спуск.
	box := image.Rect(px(110, 100)-1, px(20, 100)-1, px(190, 100)+1, px(20+blk.Box.H, 100)+px(blk.Box.Size, 100)/2)
	if ink.Empty() || !ink.In(box) {
		t.Fatalf("чернила %v вне рамки блока %v", ink, box)
	}

	// Поверх фото: текст рисуется после фото, красные пиксели есть и внутри ячейки.
	cell := layout.Rect{X: 105, Y: 10, W: 90, H: 40}
	img, err = Render(Spread{Photos: []Photo{coverPhoto(cell, 1.5, ph)}, Texts: []TextDraw{blk}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	red := 0
	for y := px(20, 100); y < px(20+blk.Box.H, 100); y++ {
		for x := px(110, 100); x < px(190, 100); x++ {
			if c := img.RGBAAt(x, y); c.R > 200 && c.G < 60 && c.B < 60 {
				red++
			}
		}
	}
	if red < 50 {
		t.Fatalf("красных пикселей текста поверх фото: %d", red)
	}
}

// Разворот без фото, только с текстом, рисуется (запись в файлы — TestRunTextOnlySpreads в export).
func TestRenderTextOnlySpread(t *testing.T) {
	opt := Options{PageW: 60, PageH: 40, DPI: 100}
	st := text.Style{Font: "cormorant", Size: 24, Color: "#000000", Align: "left"}
	bg := color.RGBA{240, 240, 230, 255}
	sp := Spread{Bg: [2]color.RGBA{bg, bg}, Texts: []TextDraw{textBlock("Заголовок", 10, 10, 40, st)}}
	img, err := Render(sp, opt)
	if err != nil || inkBounds(img, bg).Empty() {
		t.Fatalf("текст не нарисован: %v", err)
	}
}

func TestRenderRejectsPhotoWithoutFrame(t *testing.T) {
	ph := testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 30, 20)
	sp := Spread{Photos: []Photo{{Cell: layout.Rect{W: 10, H: 10}, Path: ph}}}
	if _, err := Render(sp, Options{PageW: 50, PageH: 40, DPI: 100}); err == nil {
		t.Fatal("ждали отказ: у фото нет кадра")
	}
}

func TestCropWindow(t *testing.T) {
	img := layout.Rect{X: 0, Y: 0, W: 80, H: 80}
	if got := cropWindow(layout.Rect{X: 0, Y: 0, W: 40, H: 40}, img, 200, 200); got != image.Rect(0, 0, 100, 100) {
		t.Fatalf("левый верх: %v", got)
	}
	if got := cropWindow(layout.Rect{X: 40, Y: 40, W: 40, H: 40}, img, 200, 200); got != image.Rect(100, 100, 200, 200) {
		t.Fatalf("правый низ: %v", got)
	}
	// Выход за кадр и вырожденная ячейка зажимаются, окно не пустое.
	if got := cropWindow(layout.Rect{X: 79.99, Y: -5, W: 0.001, H: 200}, img, 200, 200); got.Empty() || !got.In(image.Rect(0, 0, 200, 200)) {
		t.Fatalf("зажим: %v", got)
	}
}

func TestParseHex(t *testing.T) {
	if got := ParseHex("#1c2b3a"); got != (color.RGBA{0x1c, 0x2b, 0x3a, 255}) {
		t.Fatal(got)
	}
	for _, bad := range []string{"", "red", "#12", "#gggggg", "1c2b3a"} {
		if got := ParseHex(bad); got != (color.RGBA{255, 255, 255, 255}) {
			t.Fatalf("%q: %v", bad, got)
		}
	}
}

// Паника при подготовке фото — ошибка рендера, а не падение процесса.
func TestDrawItemsPanicBecomesError(t *testing.T) {
	old := prepareFn
	t.Cleanup(func() { prepareFn = old })
	prepareFn = func(context.Context, Photo, int, image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
		panic("декодер")
	}
	err := drawItems(context.Background(), image.NewRGBA(image.Rect(0, 0, 10, 10)), []item{{ph: Photo{Path: "a.jpg"}}}, 100)
	if err == nil || !strings.Contains(err.Error(), "декодер") {
		t.Fatalf("ждали ошибку с причиной паники, получили %v", err)
	}
}

// После первой ошибки новые фото не запускаются: максимум те, кто уже получил слот (их photoSlots()).
func TestDrawItemsStopsAfterError(t *testing.T) {
	old := prepareFn
	t.Cleanup(func() { prepareFn = old })
	var calls atomic.Int32
	prepareFn = func(_ context.Context, ph Photo, _ int, _ image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
		calls.Add(1)
		if ph.Path == "0" {
			return nil, image.Rectangle{}, errors.New("не открылось")
		}
		return nil, image.Rectangle{}, nil
	}
	items := make([]item, 30)
	for i := range items {
		items[i].ph.Path = strconv.Itoa(i)
	}
	if err := drawItems(context.Background(), image.NewRGBA(image.Rect(0, 0, 10, 10)), items, 100); err == nil {
		t.Fatal("ждали ошибку")
	}
	if n, lim := calls.Load(), photoSlots(); int(n) > lim {
		t.Fatalf("запущено %d фото из 30, ждали не больше %d", n, lim)
	}
}

func TestRenderRejectsHugeImage(t *testing.T) {
	dir := testkit.TempDir(t)
	bomb := testkit.HugeJPEG(t, dir, "bomb.jpg", 30000, 30000)
	sp := Spread{Photos: []Photo{coverPhoto(layout.Rect{X: 0, Y: 0, W: 10, H: 10}, 1, bomb)}}
	if _, err := Render(sp, Options{PageW: 50, PageH: 40, DPI: 100}); err == nil || !strings.Contains(err.Error(), "слишком") {
		t.Fatal("ждали отказ для 900 Мп")
	}
}

func TestPxFsfera(t *testing.T) {
	if got := px(562, 300); got != 6638 {
		t.Fatalf("px(562,300) = %d", got)
	}
	if got := px(284, 300); got != 3354 {
		t.Fatalf("px(284,300) = %d", got)
	}
}

// Результат рисования с поворотом не зависит от числа потоков: полосы фиксированной
// высоты, потоки только раздают их (маска vector строится в координатах полосы).
func TestRotatedSameForAnyProcs(t *testing.T) {
	path := quadrants(t, testkit.TempDir(t), [4]color.RGBA{
		{220, 40, 40, 255}, {40, 200, 60, 255}, {40, 60, 220, 255}, {230, 210, 40, 255},
	})
	bg := color.RGBA{250, 250, 245, 255}
	cell := layout.Rect{X: 20, Y: 15, W: 160, H: 120}
	// Путь 2: наклон фото в прямой ячейке с прозрачностью.
	p2 := framedPhoto(cell, 1.5, 1.6, 0.3, -0.2, path)
	p2.ImgRot, p2.Fade = 7, 0.25
	// Путь 3: повёрнутая ячейка (маска многоугольника).
	p3 := framedPhoto(cell, 1.5, 1.6, 0.3, -0.2, path)
	p3.Rot = 23
	// Путь 3: фото целиком в рамке под наклоном, поворот ячейки и прозрачность.
	p4 := Photo{Cell: cell, Img: containImg(cell, 1.5, 11), Path: path, Rot: -17, ImgRot: 11, Fade: 0.4}

	cases := []struct {
		name string
		ph   Photo
	}{{"путь 2", p2}, {"маска", p3}, {"маска, прозрачность", p4}}
	opt := Options{PageW: 100, PageH: 150, DPI: 150}
	render := func(ph Photo) [32]byte {
		img, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Photos: []Photo{ph}}, opt)
		if err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(img.Pix)
	}

	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })
	for _, c := range cases {
		runtime.GOMAXPROCS(1)
		want := render(c.ph)
		for _, n := range []int{2, 4, 8} {
			runtime.GOMAXPROCS(n)
			if got := render(c.ph); got != want {
				t.Errorf("%s: GOMAXPROCS=%d даёт другие пиксели, чем 1", c.name, n)
			}
		}
	}
}
