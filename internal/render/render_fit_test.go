package render

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Подгонка файла (123 §5, Options.Fit). Образец — тот же лист без подгонки: пиксель холста
// подгонки (x, y) равен пикселю образца в (x, y), прижатому к его краю (повтор крайнего ряда),
// то есть сцена не сдвинулась и не растянулась.

// fitOpt — 254 dpi: 10 px/мм; страница 60×40 мм — разворот 1200×400 px; допуск подгонки 5 px.
var fitOpt = Options{PageW: 60, PageH: 40, DPI: 254}

// fitScene — лист со всеми видами элементов; у краёв холста лежат плашка, фото и текст, чтобы
// повтор крайнего ряда был не однотонным.
func fitScene(t *testing.T) Spread {
	t.Helper()
	pic := blocksPNG(t)
	st := text.Style{Font: "montserrat", Size: 14, Color: "#102030", Align: "left"}
	rot := coverPhoto(layout.Rect{X: 70, Y: 8, W: 30, H: 20}, 16.0/12, pic)
	rot.Rot = 17
	return Spread{
		Bg:       [2]color.RGBA{bdGray, bdCream},
		Patterns: []PatternDraw{{Area: layout.Rect{X: 62, W: 58, H: 40}, Kind: "stripes", Inks: []string{"#203040"}, Scale: 1}},
		Plates:   []Plate{{Rect: layout.Rect{X: 0, Y: 0, W: 8, H: 40}, Color: color.RGBA{200, 40, 40, 255}}},
		Photos: []Photo{
			coverPhoto(layout.Rect{X: 10, Y: 0, W: 24, H: 18}, 16.0/12, pic),   // у верхнего края
			coverPhoto(layout.Rect{X: 100, Y: 22, W: 20, H: 18}, 16.0/12, pic), // у правого и нижнего
			rot,
		},
		Arts:    []ArtDraw{{Box: layout.Rect{X: 36, Y: 20, W: 16, H: 16}, Kind: "heart", Inks: []string{"#203040"}, Z: 5}},
		Borders: []BorderDraw{{Box: layout.Rect{X: 12, Y: 20, W: 20, H: 16}, Kind: "line", Inks: []string{"#c8201e"}, Width: 2, Z: 6}},
		Texts: []TextDraw{
			textBlock("Анна и Павел", 1, 1, 50, st),
			{X: 90, Y: 30, Rot: 8, Box: text.Layout("Свадьба", st, 28), Style: st, Z: 7},
		},
	}
}

func clampInt(v, lo, hi int) int { return min(max(v, lo), hi-1) }

// TestRenderFitExact: подгонка на ±N px по осям — сцена на месте пиксель в пиксель, полосы —
// повтор крайнего ряда; образец без подгонки не меняется от самого Fit.
func TestRenderFitExact(t *testing.T) {
	s := fitScene(t)
	base, err := Render(s, fitOpt)
	if err != nil {
		t.Fatal(err)
	}
	W, H := base.Rect.Dx(), base.Rect.Dy()
	if W != 1200 || H != 400 {
		t.Fatalf("образец %d×%d, ждали 1200×400", W, H)
	}
	for _, d := range [][2]int{{1, 1}, {-1, -1}, {5, 5}, {-5, -5}, {3, -4}, {-2, 5}, {5, 0}, {0, -5}, {1, 0}} {
		opt := fitOpt
		opt.FitW, opt.FitH = W+d[0], H+d[1]
		img, err := Render(s, opt)
		if err != nil {
			t.Fatalf("подгонка %v: %v", d, err)
		}
		if w, h := img.Rect.Dx(), img.Rect.Dy(); w != opt.FitW || h != opt.FitH {
			t.Fatalf("подгонка %v: холст %d×%d, ждали %d×%d", d, w, h, opt.FitW, opt.FitH)
		}
		if w, h := CanvasSize(opt); w != opt.FitW || h != opt.FitH {
			t.Fatalf("подгонка %v: CanvasSize %d×%d", d, w, h)
		}
		bad := 0
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
				if got, want := img.RGBAAt(x, y), base.RGBAAt(clampInt(x, 0, W), clampInt(y, 0, H)); got != want {
					if bad++; bad <= 3 {
						t.Errorf("подгонка %v: (%d, %d) %v, образец %v", d, x, y, got, want)
					}
				}
			}
		}
		if bad > 0 {
			t.Fatalf("подгонка %v: расхождений %d", d, bad)
		}
	}
}

// Файл подгонки — обычный JPEG нужного размера: кодер пишет Bounds холста, а не сцены.
func TestRenderFitJPEGSize(t *testing.T) {
	opt := fitOpt
	opt.FitW, opt.FitH = 1197, 403
	img, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdCream}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(&buf)
	if err != nil || cfg.Width != 1197 || cfg.Height != 403 {
		t.Fatalf("JPEG %dx%d, %v", cfg.Width, cfg.Height, err)
	}
}

// Расхождение больше допуска — ошибка до рисования; нули и Split — прежний путь.
func TestRenderFitLimits(t *testing.T) {
	s := Spread{Bg: [2]color.RGBA{bdGray, bdCream}}
	for _, d := range [][2]int{{6, 0}, {0, -6}, {-6, 6}, {24, 24}} {
		opt := fitOpt
		opt.FitW, opt.FitH = 1200+d[0], 400+d[1]
		if _, err := Render(s, opt); err == nil || !strings.Contains(err.Error(), "допуска") {
			t.Errorf("подгонка %v: ошибка %v", d, err)
		}
	}
	opt := fitOpt
	opt.FitW, opt.FitH = 1, 1
	if _, err := Render(s, opt); err == nil {
		t.Error("подгонка до 1×1 принята")
	}
	opt.FitW, opt.FitH = 1203, 0 // одна ось — не подгонка
	img, err := Render(s, opt)
	if err != nil || img.Rect != image.Rect(0, 0, 1200, 400) {
		t.Errorf("FitH=0: %v %v", img.Rect, err)
	}
	opt.FitW, opt.FitH, opt.Split = 1203, 403, true
	if w, h := CanvasSize(opt); w != 1200 || h != 400 {
		t.Errorf("Split с Fit: холст %d×%d", w, h)
	}
	if got := FitTolPx(300); got != 6 {
		t.Errorf("FitTolPx(300) = %d, ждали 6", got)
	}
}
