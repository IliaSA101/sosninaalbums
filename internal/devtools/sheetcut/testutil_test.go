package sheetcut

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	white  = color.NRGBA{255, 255, 255, 255}
	navy   = color.NRGBA{20, 30, 80, 255}
	transp = color.NRGBA{}
)

// canvas — лист w×h, залитый цветом c.
func canvas(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	fillRect(img, img.Bounds(), c)
	return img
}

func fillRect(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

// ring рисует кольцо (ri = 0 — диск) с мягким краем: цвет — смесь c и фона bg по покрытию пикселя.
// Для прозрачного листа (bg = transp) покрытие уходит в альфу.
func ring(img *image.NRGBA, cx, cy int, ro, ri float64, c, bg color.NRGBA) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			d := math.Hypot(float64(x-cx), float64(y-cy))
			cov := math.Min(1, math.Max(0, ro+0.5-d))
			if ri > 0 {
				cov *= math.Min(1, math.Max(0, d-ri+0.5))
			}
			if cov == 0 {
				continue
			}
			if bg.A == 0 {
				img.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, uint8(cov*255 + 0.5)})
				continue
			}
			mix := func(a, b uint8) uint8 { return uint8(cov*float64(a) + (1-cov)*float64(b) + 0.5) }
			img.SetNRGBA(x, y, color.NRGBA{mix(c.R, bg.R), mix(c.G, bg.G), mix(c.B, bg.B), 255})
		}
	}
}

// alphaAt — альфа пикселя результата.
func alphaAt(img *image.NRGBA, x, y int) uint8 { return img.NRGBAAt(x, y).A }

func mustCut(t *testing.T, img image.Image, o Options) *Result {
	t.Helper()
	res, err := Cut(img, o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// gridSheet — прозрачный лист cols×rows ячеек по cell px: в каждой ячейке плотный квадрат 40×40
// по центру (альфа 255). Возвращает лист; ячейки нумеруются слева направо, сверху вниз.
func gridSheet(cols, rows, cell int) *image.NRGBA {
	img := canvas(cols*cell, rows*cell, transp)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			cx, cy := c*cell+cell/2, r*cell+cell/2
			fillRect(img, image.Rect(cx-20, cy-20, cx+20, cy+20), navy)
		}
	}
	return img
}

// tempDir — как t.TempDir(), но в os.TempDir(), а не в GOTMPDIR внутри модуля: иначе имя папки
// каждый раз новое и go test не кэширует пакет. testkit сюда нельзя (depguard: devtools не
// тянет internal), поэтому копия его TempDir без повтора удаления.
func tempDir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sheetcut-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// selfExe — символическая ссылка на тестовый бинарь вне модуля (приём TestHelperProcess): запуск
// самого бинаря из .gotmp иначе попадает в ключ кэша go test. Без ссылки — os.Args[0].
// Копия testkit.SelfExe (devtools не импортирует internal, depguard): ветка Windows та же —
// ссылка без .exe там не запускается. Страж TestNoOwnBinaryLink в testkit держит обе копии.
func selfExe(t testing.TB) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return os.Args[0]
	}
	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		return os.Args[0]
	}
	link := filepath.Join(tempDir(t), "selfexe")
	if os.Symlink(exe, link) != nil {
		return os.Args[0]
	}
	return link
}
