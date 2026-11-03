package export

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тон фото (068): одна формула в двух местах — строка CSS (model.Tone.CSS, экран) и пиксели
// (render.applyTone, экспорт). Здесь они сводятся: строку из View разбираем как браузер и
// сравниваем с пикселями экспорта; вектор testdata/parity/tone.json читают vitest и проба QA.

// toneColors — 10 эталонных цветов: края, середина, основные, кожа, тени и света.
func toneColors() [][3]uint8 {
	return [][3]uint8{{0, 0, 0}, {255, 255, 255}, {128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255},
		{224, 172, 105}, {30, 20, 10}, {240, 230, 220}, {90, 150, 210}}
}

// toneTable — тоны сверки: шаги по одному, края шкалы, сочетания с зажимом на белом.
func toneTable() []model.Tone {
	return []model.Tone{
		{Gray: true}, {Bright: 20}, {Bright: -50}, {Contrast: 50}, {Contrast: -50},
		{Gray: true, Bright: 50, Contrast: 50}, {Bright: 50, Contrast: -50}, {Gray: true, Bright: -30, Contrast: 20},
		{Bright: 7, Contrast: -3}, {Gray: true, Contrast: -50},
	}
}

// cssFilter — строка filter, применённая к цвету как в Chromium: функции по порядку, зажим 0…1
// после каждой, sRGB, округление в конце. Неизвестная функция — ошибка теста.
func cssFilter(t *testing.T, css string, c [3]uint8) [3]uint8 {
	t.Helper()
	v := [3]float64{float64(c[0]) / 255, float64(c[1]) / 255, float64(c[2]) / 255}
	for _, f := range strings.Fields(css) {
		name, arg, ok := strings.Cut(strings.TrimSuffix(f, ")"), "(")
		k, err := strconv.ParseFloat(strings.TrimSuffix(arg, "%"), 64)
		if !ok || err != nil {
			t.Fatalf("не разобрать %q в %q", f, css)
		}
		if strings.HasSuffix(arg, "%") {
			k /= 100
		}
		switch name {
		case "grayscale":
			if k != 1 {
				t.Fatalf("grayscale(%v): ждали 1", k)
			}
			l := 0.2126*v[0] + 0.7152*v[1] + 0.0722*v[2]
			v = [3]float64{l, l, l}
		case "brightness":
			v = [3]float64{v[0] * k, v[1] * k, v[2] * k}
		case "contrast":
			v = [3]float64{(v[0]-0.5)*k + 0.5, (v[1]-0.5)*k + 0.5, (v[2]-0.5)*k + 0.5}
		default:
			t.Fatalf("функция %q в %q", name, css)
		}
		for i := range v {
			v[i] = min(max(v[i], 0), 1)
		}
	}
	return [3]uint8{uint8(math.Round(v[0] * 255)), uint8(math.Round(v[1] * 255)), uint8(math.Round(v[2] * 255))}
}

func solidPNG(t *testing.T, dir, name string, c [3]uint8) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 30, 20))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c[0], c[1], c[2], 255
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Через весь путь: Editor (SetGray, SetTone) → View.Filter → fromSheet → render. Пиксель в центре
// каждой из 10 ячеек = строка CSS из вида, применённая к цвету фото, ±1 уровень; фото без тона
// на том же развороте — исходный цвет.
func TestToneCSSMatchesRender(t *testing.T) {
	dir := testkit.TempDir(t)
	cols := toneColors()
	p := model.New()
	p.Settings.Bg = "#ff00ff"
	s := &model.Spread{ID: "s1"}
	for i, c := range append(cols, [3]uint8{10, 200, 90}) {
		id := fmt.Sprintf("p%02d", i)
		p.Photos[id] = model.PhotoRef{Path: solidPNG(t, dir, id+".png", c), W: 30, H: 20}
		s.Photos = append(s.Photos, id)
	}
	p.Spreads = []*model.Spread{s}
	e := mustEditor(t, p, "")
	toned := s.Photos[:len(cols)] // последнее фото — без тона
	for _, tn := range toneTable() {
		if r, _ := e.Ref(toned[0]); (r.Adj != nil && r.Adj.Gray) != tn.Gray {
			e.SetGray(toned)
		}
		for _, id := range toned {
			e.SetTone(id, tn.Bright, tn.Contrast)
		}
		pp, v := e.Snapshot()
		sv := v.Spreads[0]
		const dpi = 40
		img, err := render.Render(fromSheet(pp, sv), render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: dpi})
		if err != nil {
			t.Fatal(err)
		}
		for i, id := range sv.Photos {
			n, _ := strconv.Atoi(id[1:])
			src := [3]uint8{10, 200, 90}
			if n < len(cols) {
				src = cols[n]
			}
			fv, r := sv.Frames[i], sv.Rects[i]
			if n < len(cols) && fv.Filter != tn.CSS() {
				t.Fatalf("%+v: в виде %q, ждали %q", tn, fv.Filter, tn.CSS())
			}
			want := cssFilter(t, fv.Filter, src)
			got := img.RGBAAt(int((r.X+r.W/2)/25.4*dpi), int((r.Y+r.H/2)/25.4*dpi))
			for k, g := range [3]uint8{got.R, got.G, got.B} {
				if d := math.Abs(float64(g) - float64(want[k])); d > 1 {
					t.Errorf("%q, цвет %v: экспорт %v, CSS %v", fv.Filter, src, got, want)
					break
				}
			}
		}
	}
}

// toneVector — пример паритета: тон, строка CSS, цвета и ожидаемое после фильтра (float CSS).
type toneVector struct {
	Tone   model.Tone `json:"tone"`
	CSS    string     `json:"css"`
	Pixels [][3]uint8 `json:"pixels"`
	Want   [][3]uint8 `json:"want"`
}

// Вектор testdata/parity/tone.json (vitest tone.test.ts: toneCss = css; проба QA: canvas с
// filter = css даёт want ±2). Пишется с -update; проверяется строка и формула по каждому цвету.
func TestToneParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "tone.json")
	if *updateGolden {
		var vs []toneVector
		for _, tn := range append(toneTable(), model.Tone{}) {
			v := toneVector{Tone: tn, CSS: (&tn).CSS(), Pixels: toneColors()}
			for _, c := range v.Pixels {
				v.Want = append(v.Want, cssFilter(t, v.CSS, c))
			}
			vs = append(vs, v)
		}
		data, err := json.MarshalIndent(vs, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vs []toneVector
	if err := json.Unmarshal(data, &vs); err != nil {
		t.Fatal(err)
	}
	if len(vs) < 10 {
		t.Fatalf("в tone.json всего %d примеров", len(vs))
	}
	for _, v := range vs {
		if got := (&v.Tone).CSS(); got != v.CSS {
			t.Errorf("%+v: CSS %q, в векторе %q", v.Tone, got, v.CSS)
		}
		if len(v.Pixels) != 10 || len(v.Want) != len(v.Pixels) {
			t.Fatalf("%q: %d цветов, %d ожиданий", v.CSS, len(v.Pixels), len(v.Want))
		}
		for i, c := range v.Pixels {
			if got := cssFilter(t, v.CSS, c); got != v.Want[i] {
				t.Errorf("%q, %v: %v, в векторе %v", v.CSS, c, got, v.Want[i])
			}
		}
	}
}
