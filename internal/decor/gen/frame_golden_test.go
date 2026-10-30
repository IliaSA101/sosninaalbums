package gen

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"

	"golang.org/x/image/math/f64"
)

// Эталон встроенных рамочек (079-28, решение №90): хэш пикселей каждого вида на трёх формах
// (прямоугольник, скругление, овал), двух толщинах (0 — по умолчанию вида и явная) и двух
// масштабах (плитка и экран). Толщина по умолчанию заморожена вместе с Ins вида — альбомы с
// Width = 0 держат окно фото. Перезапись — только для нового вида: -update (или
// SA_GOLDEN_UPDATE=1); SA_GEN_DUMP=<папка> — PNG для просмотра глазами.
const frameGoldenPath = "testdata/frame_golden.json"

var (
	frameGoldenMM = [2]float64{100, 150}
	frameGoldenPx = [2]float64{1.6, 6} // px на мм
)

// frameGoldenShapes — формы места эталона.
var frameGoldenShapes = []struct {
	name   string
	oval   bool
	radius float64
}{{"rect", false, 0}, {"round", false, 10}, {"oval", true, 0}}

// frameGoldenSpec — рамочка эталона: цвета вида, толщина w (0 — по умолчанию).
func frameGoldenSpec(kind string, w float64, oval bool, radius float64) FrameSpec {
	return FrameSpec{Kind: kind, Width: w, W: frameGoldenMM[0], H: frameGoldenMM[1], Oval: oval, Radius: radius}
}

// renderFrame рисует рамочку (или маску soft) на место в k px/мм без полей; результат — RGBA
// (маска — серой картинкой в красном канале и альфе, чтобы хэш и PNG были одного вида).
func renderFrame(s FrameSpec, k float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, int(math.Round(s.W*k)), int(math.Round(s.H*k))))
	m := f64.Aff3{k, 0, 0, 0, k, 0}
	if mt, _ := FrameLookup(s.Kind); mt.Mask {
		a := image.NewAlpha(img.Rect)
		FrameMask(a, m, s)
		for i, v := range a.Pix {
			img.Pix[4*i], img.Pix[4*i+3] = v, v
		}
		return img
	}
	DrawFrame(img, m, s)
	return img
}

func frameGolden(t *testing.T, dump string) map[string][2]string {
	t.Helper()
	got := map[string][2]string{}
	for _, mt := range Frames() {
		for _, sh := range frameGoldenShapes {
			for _, w := range []float64{0, 2.5} {
				key := fmt.Sprintf("%s/%s/w=%s", mt.Kind, sh.name, strconv.FormatFloat(w, 'f', -1, 64))
				var hs [2]string
				for i, k := range frameGoldenPx {
					img := renderFrame(frameGoldenSpec(mt.Kind, w, sh.oval, sh.radius), k)
					hs[i] = goldenHash(img)
					if dump != "" {
						page := image.NewRGBA(img.Rect) // серая страница: белая рамка на прозрачном не видна
						draw.Draw(page, page.Rect, image.NewUniform(color.RGBA{110, 110, 120, 255}), image.Point{}, draw.Src)
						draw.Draw(page, page.Rect, img, image.Point{}, draw.Over)
						writePNG(t, filepath.Join(dump, "frame", fmt.Sprintf("%s-%s-w%v-%d.png", mt.Kind, sh.name, w, int(k))), page)
					}
				}
				got[key] = hs
			}
		}
	}
	// Поворот и сдвиг в большем холсте: вектор без пересэмплирования.
	for _, kind := range []string{"line", "scallop", "stitch"} {
		img := image.NewRGBA(image.Rect(0, 0, 360, 420))
		DrawFrame(img, rotMatrix(60, 40, 3, 20), frameGoldenSpec(kind, 0, false, 5))
		got[kind+"/rot20"] = [2]string{goldenHash(img), ""}
	}
	return got
}

// rotMatrix — мм места → px: масштаб k, поворот deg вокруг угла места, сдвиг (x, y).
func rotMatrix(x, y, k, deg float64) f64.Aff3 {
	s, c := math.Sincos(deg * math.Pi / 180)
	return f64.Aff3{k * c, -k * s, x, k * s, k * c, y}
}

// TestFrameGolden — каждый вид сборки совпадает с эталоном побитово; ключ эталона без вида —
// тоже провал (выпущенный вид не удаляют).
func TestFrameGolden(t *testing.T) {
	update := *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1"
	if runtime.GOARCH != "amd64" && os.Getenv("SA_GOLDEN_ANYARCH") != "1" && !update {
		t.Skip("эталон снят на amd64")
	}
	got := frameGolden(t, os.Getenv("SA_GEN_DUMP"))
	if update {
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(frameGoldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	var want map[string][2]string
	data, err := os.ReadFile(frameGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("%s: %v", frameGoldenPath, err)
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		w, ok := want[k]
		if !ok {
			t.Errorf("%s: нет эталона — новый вид: -update", k)
		} else if w != got[k] {
			t.Errorf("%s: рисунок изменился (%v, эталон %v) — выпущенный вид не меняют, правка — новый ключ", k, got[k], w)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("%s: эталон есть, вида нет — выпущенный вид удалён", k)
		}
	}
}
