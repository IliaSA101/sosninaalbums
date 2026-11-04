package packprep

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/devtest"
)

var (
	navy   = color.NRGBA{20, 30, 80, 255}
	coral  = color.NRGBA{220, 90, 70, 255}
	white  = color.NRGBA{255, 255, 255, 255}
	transp = color.NRGBA{}
)

// tempDir — папка теста вне GOTMPDIR (кэш go test), см. devtest.TempDir.
func tempDir(t testing.TB) string {
	t.Helper()
	return devtest.TempDir(t, "packprep")
}

func canvas(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

func fillRect(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// disc — диск радиуса r с мягким краем в 1 px: покрытие уходит в альфу.
func disc(img *image.NRGBA, cx, cy int, r float64, c color.NRGBA) {
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			cov := math.Min(1, math.Max(0, r+0.5-math.Hypot(float64(x-cx), float64(y-cy))))
			if cov > 0 {
				img.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, uint8(cov*float64(c.A) + 0.5)})
			}
		}
	}
}

// texture — детерминированный непрозрачный «фон»: диагональные полосы и точки, чтобы Lanczos было что считать.
func texture(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8((x*3 + y*5) % 200)
			img.SetNRGBA(x, y, color.NRGBA{v, uint8(255 - int(v)), uint8((x ^ y) & 0xff), 255})
		}
	}
	return img
}

func writePNG(t testing.TB, file string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, buf.Bytes())
}

func writeJPEG(t testing.TB, file string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, buf.Bytes())
}

func writeFile(t testing.TB, file string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// readOut читает файл папки набора.
func readOut(t testing.TB, out, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodePNG(t testing.TB, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// baseRecipe — рецепт набора с заданными предметами; шапка проходит проверку.
func baseRecipe(items ...Item) *Recipe {
	return &Recipe{ID: "test-pack", Version: 1, MinApp: "0.26.0", Title: "Тест", Theme: "school", Author: "Автор",
		License: "CC0-1.0", Source: "example.test", About: "Нарисовано ИИ (Magnific), отобрано и обработано вручную.", Items: items}
}

// run запускает Run над src и новой папкой выхода; возвращает отчёт и папку.
func run(t testing.TB, src string, rec *Recipe) (*Report, string) {
	t.Helper()
	out := filepath.Join(tempDir(t), "pack")
	rep, err := Run(Config{Src: src, Out: out}, rec)
	if err != nil {
		t.Fatal(err)
	}
	return rep, out
}

// hasError — в отчёте есть ошибка с подстрокой.
func hasError(rep *Report, sub string) bool {
	for _, e := range rep.Errors {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// hasFlag — у предмета есть флаг с подстрокой.
func hasFlag(o *Output, sub string) bool {
	for _, f := range o.Flags {
		if strings.Contains(f, sub) {
			return true
		}
	}
	return false
}

// byFile — предмет отчёта по имени файла.
func byFile(t testing.TB, rep *Report, file string) *Output {
	t.Helper()
	for _, o := range rep.Items {
		if o.File == file {
			return o
		}
	}
	t.Fatalf("в отчёте нет %s: %+v", file, rep.Errors)
	return nil
}

// stickerSheet — прозрачный лист 2×1 ячейки: диск и квадрат разных цветов 80 px.
func stickerSheet() *image.NRGBA {
	img := canvas(400, 200, transp)
	disc(img, 100, 100, 40, navy)
	fillRect(img, image.Rect(260, 60, 340, 140), coral)
	return img
}
