package render

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Накладка встроенной рамочки на экране (/media/gen/frame) и кусок печати того же места — одно и то
// же ±2/255, на крае сглаживания до 4 (спека 079; 080 G3): каждый вид каталога на альбомном и
// портретном (дробный размер) месте, овальном и скруглённом. Повёрнутое место не входит: печать
// пересэмплирует его BiLinear. Накладка кладётся на цвет страницы так же, как браузер.
func TestFrameOverlayMatchesPrint(t *testing.T) {
	places := []struct {
		name         string
		box          layout.Rect
		oval, radius bool
	}{
		{"альбомное", layout.Rect{X: 20, Y: 10, W: 40, H: 30}, false, false},
		{"портретное", layout.Rect{X: 20, Y: 1, W: 33.3, H: 47.7}, false, false},
		{"овал", layout.Rect{X: 20, Y: 10, W: 40, H: 30}, true, false},
		{"скруглённое", layout.Rect{X: 20, Y: 10, W: 40, H: 30}, false, true},
	}
	for _, meta := range gen.Frames() {
		for _, pl := range places {
			spec := gen.FrameSpec{Kind: meta.Kind, W: pl.box.W, H: pl.box.H, Oval: pl.oval}
			if pl.radius {
				spec.Radius = 4
			}
			worst, bad, painted := overlayVsPrint(t, spec, pl.box)
			if painted == 0 && meta.Kind != "soft" { // мягкий край — только маска, накладка пуста
				t.Errorf("%s/%s: накладка пуста", meta.Kind, pl.name)
			}
			if bad > 0 {
				t.Errorf("%s/%s: экран и печать расходятся: наибольшее %d/255, сверх допуска — %d px", meta.Kind, pl.name, worst, bad)
			}
		}
	}
}

// overlayVsPrint рисует накладку места (как /media/gen/frame, 10 px на мм) и печать страницы с той же
// рамочкой, сравнивает накладку на цвете страницы с куском печати: наибольшее расхождение, число px
// сверх допуска и сколько px накладки не пусты.
func overlayVsPrint(t *testing.T, spec gen.FrameSpec, box layout.Rect) (worst, bad, painted int) {
	t.Helper()
	bg := color.RGBA{236, 228, 214, 255}
	_, q, _ := strings.Cut(gen.FrameQuery(spec, false), "?")
	req, ok := gen.ParseFrame(q + fmt.Sprintf("&px=%d", int(box.W*10+0.5)))
	if !ok {
		t.Fatalf("%s: запрос не читается: %s", spec.Kind, q)
	}
	over, ok := req.Draw().(*image.RGBA)
	if !ok {
		t.Fatalf("%s: накладка не RGBA", spec.Kind)
	}
	printed, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Borders: []BorderDraw{{
		Box: box, Kind: spec.Kind, Oval: spec.Oval, Radius: spec.Radius}}}, patOpt)
	if err != nil {
		t.Fatal(err)
	}
	ox, oy := int(box.X*10), int(box.Y*10)
	for y := over.Rect.Min.Y; y < over.Rect.Max.Y; y++ {
		for x := over.Rect.Min.X; x < over.Rect.Max.X; x++ {
			o := over.RGBAAt(x, y)
			if o.A > 0 {
				painted++
			}
			want, got := compositeOver(o, bg), printed.RGBAAt(x+ox, y+oy)
			d := max(absDiff(want.R, got.R), absDiff(want.G, got.G), absDiff(want.B, got.B))
			worst = max(worst, d)
			if d > edgeTol(o.A) {
				bad++
			}
		}
	}
	return worst, bad, painted
}

// edgeTol — допуск, /255: ±2 у сплошных и пустых px; на полупрозрачном крае сглаживания (у «паспарту»
// в овале — два слоя) накладка хранится с предумноженной альфой 8 бит, а печать смешивает цвет
// напрямую — расхождение до 4/255 на крае шириной в 1 px, глазом не видно.
func edgeTol(a uint8) int {
	if a > 0 && a < 255 {
		return 4
	}
	return 2
}

// compositeOver — накладка с предумноженной альфой на непрозрачный фон.
func compositeOver(o, bg color.RGBA) color.RGBA {
	k := 255 - int(o.A)
	return color.RGBA{uint8(int(o.R) + int(bg.R)*k/255), uint8(int(o.G) + int(bg.G)*k/255), uint8(int(o.B) + int(bg.B)*k/255), 255}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
