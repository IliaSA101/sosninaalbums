package render

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Вне Render кэшей листа нет: рамочка и маска окна заводят свои (тесты и разовые вызовы).
func TestBorderWithoutSheetEnv(t *testing.T) {
	frame := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	canvas := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	paint, err := prepareBorder(BorderDraw{Box: box, Pic: true, Path: frame, Cut: [4]float64{20.0 / 120, 20.0 / 160, 20.0 / 120, 20.0 / 160}},
		nil, canvas.Bounds(), 254)
	if err != nil || paint == nil {
		t.Fatalf("prepareBorder: %v, %v", paint != nil, err)
	}
	if err := paint(canvas); err != nil {
		t.Fatal(err)
	}
	if c := canvas.RGBAAt(px(21.5, 254), px(25, 254)); !near(c, color.RGBA{20, 140, 40, 255}, 1) {
		t.Errorf("кайма: %v", c)
	}
	if s, err := winOf(item{}, 254); s != nil || err != nil {
		t.Errorf("фото без маски: %v, %v", s, err)
	}
	s, err := winOf(item{ph: Photo{Win: &WinMask{Box: box, Kind: "soft", Width: 4}}}, 254)
	if err != nil || s == nil {
		t.Fatalf("маска без кэша листа: %v, %v", s, err)
	}
	// Ошибка маски называет фото, а не рамочку.
	_, err = winOf(item{ph: Photo{Path: filepath.Join("a", "photo.jpg"), Win: &WinMask{Box: layout.Rect{W: 2100, H: 2100}, Kind: "soft", Width: 4}}}, 254)
	if err == nil || !strings.Contains(err.Error(), "photo.jpg") {
		t.Errorf("слишком большая маска: %v", err)
	}
}

// Повёрнутое фото с рамкой места: нечисловой Pivot — ошибка геометрии (файл мог быть подделан);
// место с маской окна целиком за холстом — не ошибка и не пиксель.
func TestRenderPivotAndWinOffCanvas(t *testing.T) {
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	for _, pv := range []layout.Rect{{X: math.NaN(), Y: 10, W: 40, H: 30}, {X: 20, Y: 10, W: math.Inf(1), H: 30}} {
		_, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Photos: []Photo{{Cell: box, Img: box, Path: photo, Rot: 30, Pivot: pv}}}, patOpt)
		if err == nil || !strings.Contains(err.Error(), "некорректная геометрия") || !strings.Contains(err.Error(), "solid.png") {
			t.Errorf("Pivot %v: %v", pv, err)
		}
	}
	far := layout.Rect{X: -400, Y: -400, W: 40, H: 30}
	img := renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray},
		Photos: []Photo{{Cell: far, Img: far, Path: photo, Rot: 30, Pivot: far, Win: &WinMask{Box: far, Rot: 30, Kind: "soft", Width: 4}}}})
	if !samePix(img, pageBare(t)) {
		t.Error("фото за холстом что-то нарисовало")
	}
	// Маска окна строится только для фото на холсте (079-40): за холстом её не собирают вовсе —
	// маска больше холста печати (отказ, если бы её строили) ошибки не даёт; прямо и под наклоном.
	huge := &WinMask{Box: layout.Rect{X: -3000, Y: -3000, W: 2100, H: 2100}, Kind: "soft", Width: 4}
	for _, rot := range []float64{0, 30} {
		if _, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Photos: []Photo{{Cell: far, Img: far, Path: photo, Rot: rot, Pivot: far, Win: huge}}}, patOpt); err != nil {
			t.Errorf("поворот %v: маска фото за холстом строилась: %v", rot, err)
		}
	}
}
