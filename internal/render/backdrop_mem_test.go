package render

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Повёрнутый фон с кадром из .salbum, не совпадающим с файлом (узкая полоса 4000×10 при
// рамке 100×2500 мм): окно оригинала нельзя растягивать до размера печати (~140 МБ), иначе
// подделанный файл просит гигабайты. Заодно ничего не пропадает: полоса остаётся на холсте.
func TestBackdropRotNeverUpscales(t *testing.T) {
	dir := testkit.TempDir(t)
	src := image.NewRGBA(image.Rect(0, 0, 4000, 10))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	path := filepath.Join(dir, "wide.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, src, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	opt := Options{PageW: 100, PageH: 300, DPI: 300}
	bd := Backdrop{Area: layout.Rect{W: 200, H: 300}, Box: layout.Rect{W: 100, H: 2500}, Rot: 1,
		Img: layout.Rect{W: 100, H: 2500}, Path: path}

	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	img, err := Render(Spread{Backdrops: []Backdrop{bd}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&b)
	// Холст (~33 МБ) + декод полосы; растянутое окно добавило бы ещё ~140 МБ.
	if got := (b.TotalAlloc - a.TotalAlloc) >> 20; got > 100 {
		t.Fatalf("выделено %d МБ: окно фона растянуто сверх исходника", got)
	}
	if img.Bounds().Empty() {
		t.Fatal("пустой холст")
	}
}
