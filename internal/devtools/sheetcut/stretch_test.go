package sheetcut

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Общий страж меры «тянется» (0.27-129-103): те же файлы читает internal/app/libverify_test.go —
// копия меры в scripts/library-verify-sheets.go. Пиксели на границе допуска unevenTol = 20:
// отличие 15 и 20 не считается («> допуска»), 21 и 25 считается. Мутация допуска в любой копии
// ломает одну из сторон.
var updateStretch = flag.Bool("update-stretch", false, "перезаписать testdata/stretch-*.png")

// stretchTolWant — допуск, каким его держит страж; число записано здесь отдельно от unevenTol: тест,
// читающий константу из кода, не заметил бы её мутации.
const stretchTolWant = 20

// stretchDiffs — отличие пятна от планки, единиц канала (из 255); имя файла — stretch-<d>.png.
var stretchDiffs = []int{15, 20, 21, 25}

// StretchGuardPct — «тянется» у рамки с пятнами сверх допуска: 4 пятна 120×150 из 270 000 px кольца.
const stretchGuardPct = 100.0 * 4 * 120 * 150 / (600*600 - 300*300)

// stretchFixture — рамка 600×600, кольцо 150 (окно 300×300, срез 0,25), по пятну 120×150 в середине
// каждой планки; пятно отличается от планки на d: сверху по R, справа по альфе, снизу по G, слева по B
// — так мутация любого канала в мере тоже ловится.
func stretchFixture(d int) *image.NRGBA {
	img := canvas(600, 600, color.NRGBA{0x90, 0x90, 0x96, 255})
	fillRect(img, image.Rect(150, 150, 450, 450), transp)
	fillRect(img, image.Rect(240, 0, 360, 150), color.NRGBA{uint8(0x90 + d), 0x90, 0x96, 255})
	fillRect(img, image.Rect(450, 240, 600, 360), color.NRGBA{0x90, 0x90, 0x96, uint8(255 - d)})
	fillRect(img, image.Rect(240, 450, 360, 600), color.NRGBA{0x90, uint8(0x90 + d), 0x96, 255})
	fillRect(img, image.Rect(0, 240, 150, 360), color.NRGBA{0x90, 0x90, uint8(0x96 + d), 255})
	return img
}

func stretchPath(d int) string { return filepath.Join("testdata", fmt.Sprintf("stretch-%d.png", d)) }

func TestStretchFixturesInSync(t *testing.T) {
	for _, d := range stretchDiffs {
		var buf bytes.Buffer
		if err := png.Encode(&buf, stretchFixture(d)); err != nil {
			t.Fatal(err)
		}
		if *updateStretch {
			if err := os.WriteFile(stretchPath(d), buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !bytes.Equal(readStretch(t, d).Pix, stretchFixture(d).Pix) {
			t.Errorf("%s не совпадает с генератором (-update-stretch перепишет)", stretchPath(d))
		}
	}
}

// readStretch читает файл рамки как NRGBA (PNG 8 бит без предумножения декодируется в него же).
func readStretch(t *testing.T, d int) *image.NRGBA {
	t.Helper()
	raw, err := os.ReadFile(stretchPath(d))
	if err != nil {
		t.Fatalf("нет файла (go test ./internal/devtools/sheetcut -run Stretch -update-stretch): %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	n, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("%s: не NRGBA, а %T", stretchPath(d), img)
	}
	return n
}

// Точное «тянется» на границе допуска: до 20 включительно — 0, с 21 — все четыре пятна.
func TestStretchToleranceBoundary(t *testing.T) {
	for _, d := range stretchDiffs {
		want := 0.0
		if d > stretchTolWant {
			want = stretchGuardPct
		}
		img := readStretch(t, d)
		fi := analyzeFrame(img, [4]int{})
		if !fi.Window || fi.Open || fi.Cut != [4]float64{0.25, 0.25, 0.25, 0.25} {
			t.Fatalf("stretch-%d: разбор рамки %+v", d, fi)
		}
		if math.Abs(fi.Stretch-want) > 0.05 {
			t.Errorf("stretch-%d: тянется %.2f %%, нужно %.2f %%", d, fi.Stretch, want)
		}
		// Через Cut: плотную альфу (≥ Dense, 224) нарезка ужимает до 255 — пятно по альфе 255-d (≥ 230)
		// пропадает, остаются три пятна; в отчёт идёт то же число (ровно 20 % — не больше порога, без флага).
		res := mustCut(t, img, frameOpts())
		wantCut := 0.0
		if d > stretchTolWant {
			wantCut = stretchGuardPct * 3 / 4
		}
		if got := res.Stickers[0].Frame.Stretch; math.Abs(got-wantCut) > 0.05 {
			t.Errorf("stretch-%d через Cut: %.2f %%, нужно %.2f %%", d, got, wantCut)
		}
		if text := res.Text(); !strings.Contains(text, fmt.Sprintf("тянется %.0f %%", wantCut)) {
			t.Errorf("stretch-%d: отчёт без «тянется %.0f %%»:\n%s", d, wantCut, text)
		}
	}
}
