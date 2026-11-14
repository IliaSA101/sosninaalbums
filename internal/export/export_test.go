package export

import (
	"bytes"
	"context"
	"image"
	"image/color"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

func TestRunCanvasSizeAndDPI(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	opt := render.Options{PageW: 50, PageH: 40, DPI: 150}
	sp := render.Spread{Photos: []render.Photo{coverPhoto(layout.Rect{X: 10, Y: 10, W: 60, H: 20}, 1.5, ph)}}
	out := filepath.Join(dir, "out")
	if err := Run(context.Background(), []render.Spread{sp}, opt, out, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "01.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// 100×40 мм при 150 dpi = 591×236 px
	if cfg.Width != 591 || cfg.Height != 236 {
		t.Fatalf("холст %dx%d, ждали 591x236", cfg.Width, cfg.Height)
	}
	// JFIF сразу после SOI: units=1 (dpi), плотность по X и Y
	if string(data[6:10]) != "JFIF" || data[13] != 1 ||
		int(data[14])<<8|int(data[15]) != 150 || int(data[16])<<8|int(data[17]) != 150 {
		t.Fatalf("нет отметки DPI: % x", data[:20])
	}
}

func TestRunRejectsHugeCanvas(t *testing.T) {
	dir := testkit.TempDir(t)
	out := filepath.Join(dir, "out")
	// 600×400 мм при 4000 dpi: 189 Мп по ширине — сторона > 65535 px
	opt := render.Options{PageW: 600, PageH: 400, DPI: 4000}
	err := Run(context.Background(), []render.Spread{{}}, opt, out, func(int, int) {})
	if err == nil || !strings.Contains(err.Error(), "DPI") {
		t.Fatalf("ждали отказ с подсказкой про DPI, получили %v", err)
	}
	if _, e := os.Stat(out); e == nil {
		t.Fatal("папка не должна создаваться при отказе")
	}
	// Стороны в пределах, но площадь > 400 Мп.
	if err := render.CheckSize(40000, 11000); err == nil {
		t.Fatal("ждали отказ по площади")
	}
	if err := render.CheckSize(7000, 3500); err != nil {
		t.Fatal(err)
	}
	// Публичная проверка по настройкам страницы.
	if err := CheckCanvas(opt); err == nil || !strings.Contains(err.Error(), "DPI") {
		t.Fatalf("CheckCanvas: ждали отказ, получили %v", err)
	}
	if err := CheckCanvas(render.Options{PageW: 300, PageH: 200, DPI: 300}); err != nil {
		t.Fatal(err)
	}
}

func TestFileName(t *testing.T) {
	for _, c := range []struct {
		i, total int
		want     string
	}{{0, 5, "01.jpg"}, {0, 120, "001.jpg"}, {119, 120, "120.jpg"}, {98, 99, "99.jpg"}, {99, 100, "100.jpg"}} {
		if got := FileName(c.i, c.total); got != c.want {
			t.Errorf("FileName(%d,%d) = %q, ждали %q", c.i, c.total, got, c.want)
		}
	}
}

// segments разбирает сегменты JPEG до SOS: маркер и данные без поля длины.
func segments(t *testing.T, data []byte) [][2][]byte {
	t.Helper()
	var out [][2][]byte
	for i := 2; i+4 <= len(data) && data[i] == 0xFF && data[i+1] != 0xDA; {
		n := int(data[i+2])<<8 | int(data[i+3])
		if i+2+n > len(data) {
			t.Fatal("сегмент за краем файла")
		}
		out = append(out, [2][]byte{data[i : i+2], data[i+4 : i+2+n]})
		i += 2 + n
	}
	return out
}

func TestRunNamesMarkerICC(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	opt := render.Options{PageW: 50, PageH: 40, DPI: 150}
	sp := render.Spread{Photos: []render.Photo{coverPhoto(layout.Rect{X: 10, Y: 10, W: 60, H: 20}, 1.5, ph)}}
	out := filepath.Join(dir, "out")
	if err := Run(context.Background(), []render.Spread{sp, sp}, opt, out, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"01.jpg", "02.jpg"} {
		path := filepath.Join(out, n)
		if !IsOurs(path) {
			t.Fatalf("%s: нет метки", n)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var haveICC, haveCOM bool
		for i, sg := range segments(t, data) {
			switch string(sg[0]) {
			case "\xFF\xE0":
				if i != 0 || int(sg[1][10])<<8|int(sg[1][11]) != 150 {
					t.Fatalf("%s: JFIF не первым или не 150 dpi", n)
				}
			case "\xFF\xFE":
				haveCOM = string(sg[1]) == Marker
			case "\xFF\xE2":
				if bytes.HasPrefix(sg[1], []byte("ICC_PROFILE\x00\x01\x01")) {
					haveICC = bytes.Equal(sg[1][14:], colorspace.SRGBProfile())
				}
			}
		}
		if !haveCOM || !haveICC {
			t.Fatalf("%s: COM=%v ICC=%v", n, haveCOM, haveICC)
		}
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("%s не декодируется: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "spread_01.jpg")); err == nil {
		t.Fatal("старое имя spread_01.jpg не должно появляться")
	}
}

func TestIsOurs(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := testkit.JPEG(t, dir, "plain.jpg", 40, 30) // чужой JPEG без метки
	if IsOurs(ph) {
		t.Fatal("чужой JPEG принят за наш")
	}
	data, _ := os.ReadFile(ph)
	cut := filepath.Join(dir, "cut.jpg")
	_ = os.WriteFile(cut, data[:10], 0o644)
	empty := filepath.Join(dir, "empty.jpg")
	_ = os.WriteFile(empty, nil, 0o644)
	for _, p := range []string{cut, empty, filepath.Join(dir, "нет.jpg")} {
		if IsOurs(p) {
			t.Fatalf("%s: ждали false", p)
		}
	}
	ours := filepath.Join(dir, "ours.jpg")
	if err := saveJPEG(image.NewRGBA(image.Rect(0, 0, 8, 8)), ours, render.Options{DPI: 300}); err != nil {
		t.Fatal(err)
	}
	if !IsOurs(ours) {
		t.Fatal("свой файл не опознан")
	}
}

// coverPhoto и near — копии помощников render_test: фото «во всю ячейку» и допуск цвета.
func coverPhoto(cell layout.Rect, aspect float64, path string) render.Photo {
	w, h := cell.W, cell.W/aspect
	if h < cell.H {
		h, w = cell.H, cell.H*aspect
	}
	img := layout.Rect{X: cell.X - (w-cell.W)/2, Y: cell.Y - (h-cell.H)/2, W: w, H: h}
	return render.Photo{Cell: cell, Img: img, Path: path}
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// Развороты без фото, только с текстом, сохраняются подряд.
func TestRunTextOnlySpreads(t *testing.T) {
	dir := testkit.TempDir(t)
	opt := render.Options{PageW: 60, PageH: 40, DPI: 100}
	st := text.Style{Font: "cormorant", Size: 24, Color: "#000000", Align: "left"}
	bg := color.RGBA{240, 240, 230, 255}
	sp := render.Spread{Bg: [2]color.RGBA{bg, bg},
		Texts: []render.TextDraw{{X: 10, Y: 10, Box: text.Layout("Заголовок", st, 40), Style: st}}}
	out := filepath.Join(dir, "out")
	if err := Run(context.Background(), []render.Spread{sp, sp}, opt, out, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"01.jpg", "02.jpg"} {
		if !IsOurs(filepath.Join(out, n)) {
			t.Fatalf("%s не записан", n)
		}
	}
}

func TestExportOptionsClamp(t *testing.T) {
	for _, c := range []struct{ inQ, inDPI, wantQ, wantDPI int }{
		{0, 0, 95, 300},
		{100, 150, 100, 150},
		{80, 72, 80, 72},
		{60, 5, 95, 72},
		{-1, 9999, 95, 1200},
	} {
		if q, d := ClampOptions(c.inQ, c.inDPI, 300); q != c.wantQ || d != c.wantDPI {
			t.Errorf("ClampOptions(%d, %d) = %d, %d, ждали %d, %d", c.inQ, c.inDPI, q, d, c.wantQ, c.wantDPI)
		}
	}
	if _, d := ClampOptions(0, 0, 200); d != 200 {
		t.Errorf("dpi 0 — из альбома: %d", d)
	}
}
