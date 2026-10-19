package render

import (
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Рамочки мест в рендере (079 G4): встроенная — вектор gen поверх фото в окне, картинка — 9-срез,
// маска окна (мягкий край, прозрачное окно картинки) во всех трёх путях, поворот вокруг центра
// места (Pivot), прозрачность фото. Масштаб patOpt — 10 px на мм.

var (
	brInk   = color.RGBA{0xc8, 0x20, 0x1e, 255}
	brPhoto = color.RGBA{30, 60, 200, 255}
)

// bigOpt — страница 100×100 мм при 10 px/мм: повёрнутому месту хватает холста со всех сторон.
var bigOpt = Options{PageW: 100, PageH: 100, DPI: 254}

func renderBig(t *testing.T, s Spread) *image.RGBA {
	t.Helper()
	img, err := Render(s, bigOpt)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func solidPNG(t *testing.T, w, h int, c color.NRGBA) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), "solid.png"), im)
}

// ringPNG — картинка-рамка w×h: кайма rim px цвета c, внутри прозрачное окно.
func ringPNG(t *testing.T, w, h, rim int, c color.NRGBA) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < rim || y < rim || x >= w-rim || y >= h-rim {
				im.SetNRGBA(x, y, c)
			}
		}
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), "ring.png"), im)
}

// turnPt — точка (lx, ly) мм от угла места box (до поворота) на холсте при повороте места на rot
// градусов по часовой вокруг его центра.
func turnPt(box layout.Rect, rot, lx, ly float64) (x, y float64) {
	s, c := sincosDeg(rot)
	dx, dy := lx-box.W/2, ly-box.H/2
	return box.X + box.W/2 + dx*c - dy*s, box.Y + box.H/2 + dx*s + dy*c
}

// windowPhoto — фото цвета brPhoto, закрывающее окно win места box (поворот rot вокруг центра места).
func windowPhoto(path string, box, win layout.Rect, rot float64) Photo {
	return Photo{Cell: win, Img: win, Path: path, Rot: rot, Pivot: box}
}

// Середина полосы — цвет рамочки, середина окна — фото, снаружи — страница; шва нет: на пути
// от полосы к фото ни одного пикселя цвета страницы (внутренний край полосы заходит под фото).
func TestRenderBorderLine(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	win := layout.Rect{X: 22, Y: 12, W: 36, H: 26}
	bg := [2]color.RGBA{bdGray, bdGray}
	s := Spread{Bg: bg,
		Photos:  []Photo{windowPhoto(photo, box, win, 0)},
		Borders: []BorderDraw{{Box: box, Kind: "line", Inks: []string{"#c8201e"}, Width: 2}}}
	img := renderPat(t, s)
	wantExact(t, "полоса", img, 254, 21, 25, brInk)
	wantExact(t, "окно", img, 254, 40, 25, brPhoto)
	wantExact(t, "снаружи", img, 254, 15, 25, bdGray)
	for x := 200; x < 260; x++ { // слева направо через полосу и край окна
		if c := img.RGBAAt(x, 250); near(c, bdGray, 8) {
			t.Fatalf("шов: пиксель %d цвета страницы %v", x, c)
		}
	}
	for y := 100; y < 160; y++ { // сверху вниз
		if c := img.RGBAAt(400, y); near(c, bdGray, 8) {
			t.Fatalf("шов: строка %d цвета страницы %v", y, c)
		}
	}
	// Без рамочки: фото в окне то же, полосы нет.
	bare := renderPat(t, Spread{Bg: bg, Photos: []Photo{windowPhoto(photo, box, win, 0)}})
	wantExact(t, "без рамочки", bare, 254, 21, 25, bdGray)
}

// Рамочка над своим фото при равных Z и под тем, что выше места.
func TestRenderBorderZOrder(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	// Фото закрывает всё место (окно = место), рамочка-накладка «строчка» — поверх него.
	for _, z := range []int{1, 5} {
		s := Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Photos:  []Photo{{Cell: box, Img: box, Path: photo, Z: z}},
			Borders: []BorderDraw{{Box: box, Kind: "corners", Inks: []string{"#c8201e"}, Width: 7, Z: z}}}
		img := renderPat(t, s)
		wantExact(t, "уголок над фото", img, 254, 22, 12, brInk)
		wantExact(t, "середина фото", img, 254, 40, 25, brPhoto)
	}
	// Plate выше (Z больше) закрывает рамочку.
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray},
		Photos:  []Photo{{Cell: box, Img: box, Path: photo, Z: 1}},
		Borders: []BorderDraw{{Box: box, Kind: "corners", Inks: []string{"#c8201e"}, Width: 7, Z: 1}},
		Slots:   []Plate{{Rect: layout.Rect{X: 18, Y: 8, W: 10, H: 10}, Color: color.RGBA{0, 255, 0, 255}, Z: 2}}}
	wantExact(t, "плашка выше рамочки", renderPat(t, s), 254, 22, 12, color.RGBA{0, 255, 0, 255})
}

// Прозрачность фото места: окно — среднее фото и страницы, рамочка не прозрачнеет; полная — фото
// не читается вовсе (пропавший файл не ошибка), а рамочка на месте.
func TestRenderPhotoFadeWithBorder(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	win := layout.Rect{X: 22, Y: 12, W: 36, H: 26}
	line := BorderDraw{Box: box, Kind: "line", Inks: []string{"#c8201e"}, Width: 2}
	ph := windowPhoto(photo, box, win, 0)
	ph.Fade = 0.5
	img := renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph}, Borders: []BorderDraw{line}})
	mid := color.RGBA{uint8((int(brPhoto.R) + int(bdGray.R) + 1) / 2), uint8((int(brPhoto.G) + int(bdGray.G) + 1) / 2),
		uint8((int(brPhoto.B) + int(bdGray.B) + 1) / 2), 255}
	wantExact(t, "окно при 0,5", img, 254, 40, 25, mid)
	wantExact(t, "рамочка не прозрачнеет", img, 254, 21, 25, brInk)
	for _, rot := range []float64{90, 25} { // и в повёрнутых путях
		ph.Rot = rot
		b := line
		b.Rot = rot
		img = renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph}, Borders: []BorderDraw{b}})
		wantExact(t, "окно при 0,5, поворот", img, 254, 40, 25, mid)
	}
	ph.Rot, ph.Fade, ph.Path = 0, 1, filepath.Join(t.TempDir(), "нет.png")
	img = renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph}, Borders: []BorderDraw{line}})
	wantExact(t, "полностью прозрачное", img, 254, 40, 25, bdGray)
	wantExact(t, "рамочка при полной прозрачности", img, 254, 21, 25, brInk)
}

// Окно со смещённым центром (полароид) при повороте места: окно и полосы поворачиваются вокруг
// центра места, а не окна — в четвертях (путь 1 после fold), под наклоном (путь 3) и в нуле.
func TestRenderBorderPivot(t *testing.T) {
	photo := solidPNG(t, 30, 22, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 25, W: 40, H: 45}
	win := layout.Rect{X: 35, Y: 30, W: 30, H: 22.5} // полароид 5 мм: отступы 5, 5, 5, 17,5
	for _, rot := range []float64{0, 90, 180, 270, 30, -75} {
		s := Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Photos:  []Photo{windowPhoto(photo, box, win, rot)},
			Borders: []BorderDraw{{Box: box, Rot: rot, Kind: "polaroid", Inks: []string{"#c8201e"}, Width: 5}}}
		img := renderBig(t, s)
		at := func(what string, lx, ly float64, want color.RGBA) {
			t.Helper()
			x, y := turnPt(box, rot, lx, ly)
			wantExact(t, what, img, 254, x, y, want)
		}
		at("центр окна", 20, 16.25, brPhoto)
		at("нижняя полоса", 20, 41.25, brInk)
		at("левая полоса", 2.5, 20, brInk)
		at("снаружи места", -2, -2, bdGray)
		at("под местом", 20, 47, bdGray)
	}
}

// Мягкий край (Mask-вид): у края места фото растворено, на глубине толщины — целое; в прямом,
// четвертном (blitMasked после fold) и наклонном (путь 3 с маской) положении.
func TestRenderBorderSoftMask(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 30, W: 40, H: 30}
	for _, rot := range []float64{0, 90, 30} {
		ph := Photo{Cell: box, Img: box, Path: photo, Rot: rot, Pivot: box,
			Win: &WinMask{Box: box, Rot: rot, Kind: "soft", Width: 4}}
		img := renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph}})
		at := func(lx, ly float64) color.RGBA {
			x, y := turnPt(box, rot, lx, ly)
			return img.RGBAAt(px(x, 254), px(y, 254))
		}
		if c := at(0.15, 15); !near(c, bdGray, 14) {
			t.Errorf("поворот %v: у края фото не растворено: %v", rot, c)
		}
		if c := at(2, 15); math.Abs(float64(c.R)-(float64(bdGray.R)+float64(brPhoto.R))/2) > 22 {
			t.Errorf("поворот %v: на половине толщины %v, ждали середину между страницей и фото", rot, c)
		}
		if c := at(6, 15); !near(c, brPhoto, 3) {
			t.Errorf("поворот %v: глубже толщины %v, ждали фото", rot, c)
		}
		if c := at(20, 15); !near(c, brPhoto, 1) {
			t.Errorf("поворот %v: в середине %v", rot, c)
		}
	}
}

// Рамочка-картинка: углы 1:1, стороны тянутся, прозрачное окно показывает фото; в прямом и
// повёрнутом месте; фото в окне — без шва с рамкой; вне повёрнутого места — страница.
func TestRenderBorderPicture(t *testing.T) {
	green := color.NRGBA{20, 140, 40, 255}
	frame := ringPNG(t, 120, 160, 20, green) // кайма 20 px = 1/6 ширины и 1/8 высоты
	photo := solidPNG(t, 30, 22, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 30, W: 40, H: 30}
	cut := [4]float64{20.0 / 120, 20.0 / 160, 20.0 / 120, 20.0 / 160}
	// k = min(40/120, 30/160) = 0,1875 мм/px: кайма 3,75 мм со всех сторон.
	win := layout.Rect{X: 33.75, Y: 33.75, W: 32.5, H: 22.5}
	gr := color.RGBA{green.R, green.G, green.B, 255}
	for _, rot := range []float64{0, 90, 30} {
		ph := windowPhoto(photo, box, win, rot)
		ph.Win = &WinMask{Box: box, Rot: rot, Path: frame, Cut: cut}
		s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph},
			Borders: []BorderDraw{{Box: box, Rot: rot, Pic: true, Path: frame, Cut: cut}}}
		img := renderBig(t, s)
		at := func(what string, lx, ly float64, want color.RGBA) {
			t.Helper()
			x, y := turnPt(box, rot, lx, ly)
			wantExact(t, what, img, 254, x, y, want)
		}
		at("центр: фото", 20, 15, brPhoto)
		at("левая кайма", 1.9, 15, gr)
		at("верхняя кайма", 20, 1.9, gr)
		at("угол", 0.8, 0.8, gr)
		at("вне места", -2, -2, bdGray)
		// Через край окна нет щели цвета страницы.
		for lx := 3.0; lx < 4.6; lx += 0.1 {
			x, y := turnPt(box, rot, lx, 15)
			if c := img.RGBAAt(px(x, 254), px(y, 254)); near(c, bdGray, 10) {
				t.Fatalf("поворот %v: шов на lx=%.1f: %v", rot, lx, c)
			}
		}
	}
}

// Прозрачное окно картинки режет фото: рамка с полупрозрачной каймой (альфа 200 — это не окно)
// смешивается со страницей, а не с фото, которое по ячейке заходит под кайму. В прямом, четвертном
// и наклонном положении места.
func TestRenderBorderPictureMask(t *testing.T) {
	frame := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 200})
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 30, W: 40, H: 30}
	cut := [4]float64{20.0 / 120, 20.0 / 160, 20.0 / 120, 20.0 / 160}
	// Кайма над страницей: зелёный при альфе 200/255 поверх серого.
	over := func(below color.RGBA) color.RGBA {
		mix := func(g, b uint8) uint8 { return uint8((int(g)*200 + int(b)*55 + 127) / 255) }
		return color.RGBA{mix(20, below.R), mix(140, below.G), mix(40, below.B), 255}
	}
	for _, rot := range []float64{0, 90, 30} {
		ph := Photo{Cell: box, Img: box, Path: photo, Rot: rot, Pivot: box,
			Win: &WinMask{Box: box, Rot: rot, Path: frame, Cut: cut}}
		img := renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph},
			Borders: []BorderDraw{{Box: box, Rot: rot, Pic: true, Path: frame, Cut: cut}}})
		x, y := turnPt(box, rot, 1.9, 15)
		wantExact(t, "кайма над страницей", img, 254, x, y, over(bdGray))
		x, y = turnPt(box, rot, 20, 15)
		wantExact(t, "окно", img, 254, x, y, brPhoto)
	}
}

// Без окна в картинке (Open): рамочка рисуется, маски у фото нет — фото на всё место под ней.
func TestRenderBorderPictureOpen(t *testing.T) {
	green := color.NRGBA{20, 140, 40, 255}
	frame := ringPNG(t, 120, 160, 20, green)
	photo := solidPNG(t, 30, 22, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 10, W: 40, H: 30}
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray},
		Photos:  []Photo{{Cell: box, Img: box, Path: photo}},
		Borders: []BorderDraw{{Box: box, Pic: true, Path: frame, Cut: [4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}}}}
	img := renderPat(t, s)
	wantExact(t, "центр", img, 254, 50, 25, brPhoto)
	wantExact(t, "угол места", img, 254, 30.8, 10.8, color.RGBA{20, 140, 40, 255})
}

// Размеры картинки рамочки — по заголовку, до декода (079-35): PNG 15000² из 33 байт IHDR не
// разворачивается в гигабайты; больше 64 Мп (и узкая) — отказ, 4096 — можно (сторона больше 4096
// уменьшается — TestRenderBorderShrinksBig, 079-40). Имя файла в тексте без каталога.
func TestRenderBorderHeaderLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	header := func(w, h uint32) string {
		var b []byte
		b = append(b, 0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a)
		chunk := func(name string, data []byte) {
			b = binary.BigEndian.AppendUint32(b, uint32(len(data)))
			b = append(b, name...)
			b = append(b, data...)
			b = binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(append([]byte(name), data...)))
		}
		ihdr := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, w), h)
		chunk("IHDR", append(ihdr, 8, 6, 0, 0, 0))
		chunk("IEND", nil)
		p := filepath.Join(dir, "h.png")
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	run := func(path string) error {
		_, err := Render(Spread{Borders: []BorderDraw{{Box: layout.Rect{X: 10, Y: 10, W: 40, H: 30}, Pic: true, Path: path,
			Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}}}, patOpt)
		return err
	}
	for _, sz := range [][2]uint32{{15000, 15000}, {8001, 8000}, {64001, 1000}, {1000, 64001}} {
		start := time.Now()
		err := run(header(sz[0], sz[1]))
		if err == nil || !strings.Contains(err.Error(), "слишком велика") || strings.Contains(err.Error(), dir) ||
			!strings.Contains(err.Error(), "h.png") {
			t.Errorf("%v: ошибка %v", sz, err)
		}
		if d := time.Since(start); d > testkit.Limit(500*time.Millisecond) {
			t.Errorf("%v: отказ занял %v", sz, d)
		}
	}
	// Ровно 4096 по стороне допустимо (картинка узкая: кадр 4096×16 — настоящий файл).
	im := image.NewNRGBA(image.Rect(0, 0, 4096, 16))
	if err := run(writePNG(t, filepath.Join(dir, "wide.png"), im)); err != nil {
		t.Errorf("4096×16: %v", err)
	}
	// Пропавший файл — ошибка с именем, а не паника; в тексте нет каталога.
	err := run(filepath.Join(dir, "нет.png"))
	if err == nil || !strings.Contains(err.Error(), "нет.png") || strings.Contains(err.Error(), dir) {
		t.Errorf("пропавший файл: %v", err)
	}
}

// Кэши листа: декод и масштаб один раз на путь и размер; ошибка и паника сборки не роняют кэш;
// третья запись вытесняет самую старую.
func TestBorderEnvMemo(t *testing.T) {
	green := color.NRGBA{20, 140, 40, 255}
	frame := ringPNG(t, 60, 80, 10, green)
	var e borderEnv
	a, err := e.picture(frame)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := e.picture(frame); b != a {
		t.Error("второй вызов декодировал заново")
	}
	s1, _ := e.scaled(a, 30, 40)
	if s2, _ := e.scaled(a, 30, 40); s2 != s1 {
		t.Error("второй вызов масштабировал заново")
	}
	if s0, _ := e.scaled(a, 60, 80); s0 != a.img {
		t.Error("тот же размер должен отдавать оригинал")
	}
	var m memo[int]
	calls := 0
	build := func() (int, error) { calls++; return calls, nil }
	for _, k := range []string{"a", "b", "a", "c", "a"} { // «c» вытесняет «a» (самую старую)
		_, _ = m.get(k, build)
	}
	if calls != 4 {
		t.Errorf("сборок %d, ждали 4 (a, b, c, снова a)", calls)
	}
	if _, err := m.get("boom", func() (int, error) { panic("декодер") }); err == nil || !strings.Contains(err.Error(), "декодер") {
		t.Errorf("паника сборки: %v", err)
	}
}

// 30 мест 30×40 мм с одной рамочкой-картинкой 3000×4000 при 300 dpi: декод и масштаб один раз,
// остальное — срез на каждое место (design §12).
func TestRenderBorderFast(t *testing.T) {
	if testkit.Lite() {
		t.Skip("большая картинка рамочки: полный прогон")
	}
	const iw, ih = 3000, 4000
	im := image.NewNRGBA(image.Rect(0, 0, iw, ih))
	rim := 420 // кайма 14 %
	for y := range ih {
		for x := range iw {
			if x < rim || y < rim || x >= iw-rim || y >= ih-rim {
				im.SetNRGBA(x, y, color.NRGBA{20, 140, 40, 255})
			}
		}
	}
	frame := writePNG(t, filepath.Join(testkit.TempDir(t), "big.png"), im)
	photo := solidPNG(t, 30, 40, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	cut := [4]float64{0.14, 0.14, 0.14, 0.14}
	k := 30.0 / iw // мм на px картинки: 30×40 мм, k = 0,01
	ins := k * 0.14 * iw
	var s Spread
	s.Bg = [2]color.RGBA{bdGray, bdGray}
	for i := range 30 {
		box := layout.Rect{X: float64(i%6) * 32, Y: float64(i/6) * 42, W: 30, H: 40}
		win := layout.Rect{X: box.X + ins, Y: box.Y + k*0.14*ih, W: 30 - 2*ins, H: 40 - 2*k*0.14*ih}
		ph := windowPhoto(photo, box, win, 0)
		ph.Win = &WinMask{Box: box, Path: frame, Cut: cut}
		s.Photos = append(s.Photos, ph)
		s.Borders = append(s.Borders, BorderDraw{Box: box, Pic: true, Path: frame, Cut: cut})
	}
	opt := Options{PageW: 100, PageH: 210, DPI: 300}
	start := time.Now()
	out, err := Render(s, opt)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > testkit.Limit(1500*time.Millisecond) {
		t.Errorf("30 рамочек-картинок: %v", d)
	}
	// Контроль: рамочка нарисована (кайма — зелёная), фото в окне — синее.
	if c := out.RGBAAt(px(1, 300), px(20, 300)); !near(c, color.RGBA{20, 140, 40, 255}, 2) {
		t.Errorf("кайма первой: %v", c)
	}
	if c := out.RGBAAt(px(15, 300), px(20, 300)); !near(c, brPhoto, 2) {
		t.Errorf("окно первой: %v", c)
	}
}

// Рамка места со смещённым центром и сторона > 0,7: отступы зажаты, окно из модели — маска
// строится по зажатому (gen.StretchWindow, 079-34), рамка закрывает край фото.
func TestRenderBorderInsetClamp(t *testing.T) {
	green := color.NRGBA{20, 140, 40, 255}
	frame := ringPNG(t, 100, 100, 40, green) // кайма 40 % с каждой стороны — пара 80 % > 70 %
	photo := solidPNG(t, 30, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 10, W: 30, H: 30}
	cut := [4]float64{0.4, 0.4, 0.4, 0.4}
	// ins = 12 мм, пара 24 > 21 → ×0,875: 10,5 мм; окно 9 мм по центру.
	win := layout.Rect{X: 40.5, Y: 20.5, W: 9, H: 9}
	ph := windowPhoto(photo, box, win, 0)
	ph.Win = &WinMask{Box: box, Path: frame, Cut: cut}
	img := renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph},
		Borders: []BorderDraw{{Box: box, Pic: true, Path: frame, Cut: cut}}})
	wantExact(t, "окно", img, 254, 45, 25, brPhoto)
	wantExact(t, "рамка рядом с окном", img, 254, 38, 25, color.RGBA{20, 140, 40, 255})
	wantExact(t, "рамка закрывает край фото", img, 254, 41.5, 25, color.RGBA{20, 140, 40, 255})
}

// Овальное место под 30° с рамочкой «линия» (079, спека: TestRenderBorderOvalRot): полоса идёт по
// овалу, за овалом в углах габарита места — страница, в окне — фото; на пути от полосы к фото и
// сверху, и сбоку нет пикселя цвета страницы (шва нет и под поворотом).
func TestRenderBorderOvalRot(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 30, Y: 30, W: 40, H: 30}
	win := layout.Rect{X: 32, Y: 32, W: 36, H: 26}
	const rot = 30
	ph := windowPhoto(photo, box, win, rot)
	ph.Outline = geom.Outline{Oval: true}
	img := renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Photos: []Photo{ph},
		Borders: []BorderDraw{{Box: box, Rot: rot, Kind: "line", Inks: []string{"#c8201e"}, Width: 2, Oval: true}}})
	at := func(lx, ly float64) color.RGBA {
		x, y := turnPt(box, rot, lx, ly)
		return img.RGBAAt(px(x, 254), px(y, 254))
	}
	for _, c := range []struct {
		what   string
		lx, ly float64
		want   color.RGBA
	}{
		{"центр", 20, 15, brPhoto}, {"полоса сверху", 20, 1, brInk}, {"полоса слева", 1, 15, brInk},
		{"полоса снизу", 20, 29, brInk}, {"угол габарита", 1.5, 1.5, bdGray}, {"угол габарита справа снизу", 38.5, 28.5, bdGray},
	} {
		if got := at(c.lx, c.ly); !near(got, c.want, 3) {
			t.Errorf("%s: %v, ждали %v", c.what, got, c.want)
		}
	}
	for d := 0.6; d < 8; d += 0.05 { // от полосы внутрь: сверху и слева
		if c := at(20, d); near(c, bdGray, 8) {
			t.Fatalf("шов сверху на глубине %.2f мм: %v", d, c)
		}
		if c := at(d, 15); near(c, bdGray, 8) {
			t.Fatalf("шов слева на глубине %.2f мм: %v", d, c)
		}
	}
}

// Картинка рамочки на месте 600×300 мм при 300 dpi уменьшается пределом площади (borderArea), а
// толщины углов на месте — k·срез, как окно фото (079-40): от края места до окна — рамка, потом
// фото, кольца страницы нет ни по горизонтали, ни по вертикали.
func TestRenderBorderAreaCapNoRing(t *testing.T) {
	ring := ringPNG(t, 400, 200, 40, color.NRGBA{G: 140, A: 255}) // кайма 40 px: по вертикали срез 0,2
	photo := solidPNG(t, 40, 20, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	cut := [4]float64{0.1, 0.2, 0.1, 0.2}
	box := layout.Rect{W: 600, H: 300}
	win := layout.Rect{X: 60, Y: 60, W: 480, H: 180} // k = 1,5 мм/px: отступы 1,5·срез·сторона картинки
	if g := picGeometry(box, 1, cut, 400, 200, px(600, 300), px(300, 300)); g.tw*g.th > borderArea || g.dsl == g.sl {
		t.Fatalf("предел площади не сработал: %+v", g)
	}
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray},
		Photos:  []Photo{{Cell: win, Img: win, Path: photo, Pivot: box, Win: &WinMask{Box: box, Path: ring, Cut: cut}}},
		Borders: []BorderDraw{{Box: box, Pic: true, Path: ring, Cut: cut, Z: 1}}}
	img, err := Render(s, Options{PageW: 300, PageH: 300, DPI: 300})
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		if c := img.RGBAAt(x, b.Dy()/2); near(c, bdGray, 8) {
			t.Fatalf("кольцо по горизонтали: x %d (%.1f мм) %v", x, float64(x)*25.4/300, c)
		}
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if c := img.RGBAAt(b.Dx()/2, y); near(c, bdGray, 8) {
			t.Fatalf("кольцо по вертикали: y %d (%.1f мм) %v", y, float64(y)*25.4/300, c)
		}
	}
	wantExact(t, "рамка у края", img, 300, 30, 150, color.RGBA{G: 140, A: 255})
	wantExact(t, "фото в окне", img, 300, 300, 150, brPhoto)
}
