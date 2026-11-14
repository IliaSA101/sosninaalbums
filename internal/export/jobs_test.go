package export

import (
	"context"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func jpegSize(t *testing.T, path string) image.Config {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// 300 мм / 300 dpi: px(600) = 7087 не делится пополам, поэтому страницы режутся из холста
// 2·px(300) = 7086 — обе ровно по 3543 px, правая не шире левой.
func TestRunPagesSplit(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "out")
	red, blue := color.RGBA{200, 20, 20, 255}, color.RGBA{20, 20, 200, 255}
	opt := render.Options{PageW: 300, PageH: 20, DPI: 300, Quality: 90}
	if w, _ := render.CanvasSize(render.Options{PageW: 300, PageH: 20, DPI: 300}); w != 7087 {
		t.Fatalf("целый разворот %d px, ждали 7087 (прежний размер файлов не меняется)", w)
	}
	if w, _ := render.CanvasSize(render.Options{PageW: 300, PageH: 20, DPI: 300, Split: true}); w != 7086 {
		t.Fatalf("холст для страниц %d px, ждали 2·px(300) = 7086", w)
	}
	spreads := []render.Spread{{Bg: [2]color.RGBA{red, blue}}, {}}
	jobs := []Job{{Index: 0, Left: true, Right: true}, {Index: 1, Whole: true, Right: true}}
	var progress []int
	if err := RunJobs(context.Background(), spreads, jobs, opt, dir, func(done, _ int) { progress = append(progress, done) }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(progress, []int{1, 2}) {
		t.Errorf("прогресс %v", progress)
	}
	l, r := jpegSize(t, filepath.Join(dir, "01-1.jpg")), jpegSize(t, filepath.Join(dir, "01-2.jpg"))
	if l.Width != 3543 || r.Width != 3543 || l.Height != r.Height {
		t.Fatalf("страницы %dx%d и %dx%d, ждали по 3543 px", l.Width, l.Height, r.Width, r.Height)
	}
	if w := jpegSize(t, filepath.Join(dir, "02.jpg")).Width; w != 7087 {
		t.Errorf("02.jpg %d px, ждали 7087", w)
	}
	if w := jpegSize(t, filepath.Join(dir, "02-2.jpg")).Width; w != 3543 {
		t.Errorf("02-2.jpg %d px", w)
	}
	for name, want := range map[string]color.RGBA{"01-1.jpg": red, "01-2.jpg": blue} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		if c := color.RGBAModel.Convert(img.At(b.Min.X+5, b.Min.Y+5)).(color.RGBA); !near(c, want, 12) {
			t.Errorf("%s: цвет страницы %v, ждали %v", name, c, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "01.jpg")); err == nil {
		t.Error("01.jpg не заказывали")
	}
}

func TestPageName(t *testing.T) {
	if got := PageName(4, 9, 1); got != "05-1.jpg" {
		t.Errorf("PageName = %q", got)
	}
	if got := PageName(4, 120, 2); got != "005-2.jpg" {
		t.Errorf("PageName = %q", got)
	}
}

// Ошибка недоступного фото доходит до тоста и лога без каталога, причина остаётся в цепочке.
func TestRunJobsMissingPhotoNoDir(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "out")
	for _, path := range []string{`C:\Users\x\photos\a.jpg`, `/home/x/photos/a.jpg`} {
		cell := layout.Rect{X: 5, Y: 5, W: 20, H: 20}
		spreads := []render.Spread{{Photos: []render.Photo{{Cell: cell, Img: cell, Path: path}}}}
		opt := render.Options{PageW: 50, PageH: 40, DPI: 100, Quality: 90}
		err := RunJobs(context.Background(), spreads, []Job{{Index: 0, Whole: true}}, opt, dir, func(int, int) {})
		if err == nil {
			t.Fatalf("%q: ждали ошибку", path)
		}
		if got := err.Error(); strings.Contains(got, "Users") || strings.Contains(got, "home") || !strings.Contains(got, "a.jpg") {
			t.Errorf("%q: текст %q", path, got)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%q: причина потеряна: %v", path, err)
		}
	}
}
