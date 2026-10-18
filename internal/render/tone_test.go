package render

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тон фото в экспорте (068): формула CSS-фильтров в float против таблицы applyTone, пути 1 и 3,
// нулевой тон — байт-в-байт прежнее, цена.

// toneFloat — формула design §5 в float: grayscale(1) Rec.709, brightness, contrast, зажим
// после каждого шага (как у Chromium), округление в конце.
func toneFloat(c color.RGBA, t Tone) color.RGBA {
	v := [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
	if t.Gray {
		l := 0.2126*v[0] + 0.7152*v[1] + 0.0722*v[2]
		v = [3]float64{l, l, l}
	}
	kb, kc := float64(100+t.Bright)/100, float64(100+t.Contrast)/100
	var out [3]uint8
	for i, x := range v {
		x = min(x*kb, 1)
		x = min(max((x-0.5)*kc+0.5, 0), 1)
		out[i] = uint8(math.Round(x * 255))
	}
	return color.RGBA{out[0], out[1], out[2], c.A}
}

// toneCases — тоны для сверки: каждый шаг отдельно, края шкалы и сочетания с зажимом.
func toneCases() []Tone {
	return []Tone{
		{Gray: true}, {Bright: 20}, {Bright: -50}, {Bright: 50}, {Contrast: 50}, {Contrast: -50}, {Contrast: 1},
		{Gray: true, Bright: 50, Contrast: 50}, {Gray: true, Bright: -50, Contrast: -50}, {Bright: 50, Contrast: -50},
		{Gray: true, Bright: 12, Contrast: -7}, {Bright: -1, Contrast: 33},
	}
}

func diff(a, b color.RGBA) int {
	d := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
		d = max(d, int(math.Abs(float64(p[0])-float64(p[1]))))
	}
	return d
}

// Таблица applyTone против float: ≤ 1 уровня на канал для всех значений канала и сетки цветов;
// у ч/б R = G = B; альфа стоит.
func TestApplyToneMatchesFormula(t *testing.T) {
	var cols []color.RGBA
	for v := range 256 {
		cols = append(cols, color.RGBA{uint8(v), uint8(255 - v), uint8(v * 7), uint8(v)})
	}
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				cols = append(cols, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
			}
		}
	}
	im := image.NewNRGBA(image.Rect(0, 0, len(cols), 1))
	for _, tn := range toneCases() {
		for i, c := range cols {
			im.SetNRGBA(i, 0, color.NRGBA(c))
		}
		applyTone(im, tn)
		worst := 0
		for i, c := range cols {
			got := color.RGBA(im.NRGBAAt(i, 0))
			if d := diff(got, toneFloat(c, tn)); d > worst {
				worst = d
			}
			if got.A != c.A {
				t.Fatalf("%+v: альфа %d → %d", tn, c.A, got.A)
			}
			if tn.Gray && (got.R != got.G || got.G != got.B) {
				t.Fatalf("%+v: ч/б не серое: %v", tn, got)
			}
		}
		if worst > 1 {
			t.Errorf("%+v: расхождение с формулой %d уровней", tn, worst)
		}
	}
}

// Нулевой тон и nil — пиксели не трогаются.
func TestRenderNoToneUnchanged(t *testing.T) {
	applyTone(nil, Tone{Gray: true})
	im := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := range im.Pix {
		im.Pix[i] = uint8(i * 37)
	}
	before := append([]byte(nil), im.Pix...)
	applyTone(im, Tone{})
	if string(before) != string(im.Pix) {
		t.Fatal("нулевой тон изменил пиксели")
	}
}

// tonePNG — однотонное фото без потерь (PNG): цвет на холсте сверяется с формулой точно.
func tonePNG(t *testing.T, c color.RGBA) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 60, 40))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, 255
	}
	return writePNG(t, filepath.Join(testkit.TempDir(t), "tone.png"), im)
}

// Путь 1 (без поворота), место «круг»: внутри — цвет по формуле, в углах описанного
// квадрата — страница без тона. Соседнее фото без тона — исходный цвет. «Целиком» (сц. 8): поле
// ячейки над и под фото — страница без тона, само фото — по формуле.
func TestRenderToneMatchesFormula(t *testing.T) {
	src := color.RGBA{200, 120, 40, 255}
	path := tonePNG(t, src)
	tn := Tone{Gray: true, Bright: 20, Contrast: -30}
	o := geom.Outline{Oval: true}
	cell := layout.Rect{X: 10, Y: 10, W: 40, H: 40}
	plain := layout.Rect{X: 55, Y: 55, W: 30, H: 20}
	whole := layout.Rect{X: 10, Y: 55, W: 40, H: 40} // фото 3:2 — 40×26,7 мм, поля по 6,7 мм
	img, err := Render(Spread{Bg: [2]color.RGBA{magenta, magenta}, Photos: []Photo{
		{Cell: cell, Img: geom.PlaceImageIn(cell, 1.5, geom.Crop{S: 1}, o), Path: path, Outline: o, Tone: tn},
		{Cell: plain, Img: geom.PlaceImage(plain, 1.5, geom.Crop{S: 1}), Path: path},
		{Cell: whole, Img: geom.ContainImage(whole, 1.5, 0), Path: path, Tone: tn},
	}}, Options{PageW: 100, PageH: 100, DPI: shapeDPI})
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y float64) color.RGBA { return img.RGBAAt(px(x, shapeDPI), px(y, shapeDPI)) }
	if got, want := at(30, 30), toneFloat(src, tn); diff(got, want) > 1 {
		t.Errorf("фото с тоном: %v, ждали %v ±1", got, want)
	}
	if got := at(11, 11); got != magenta {
		t.Errorf("угол круга: %v — тон задел страницу", got)
	}
	if got := at(70, 65); diff(got, src) > 1 {
		t.Errorf("фото без тона: %v, ждали %v", got, src)
	}
	if got, want := at(30, 75), toneFloat(src, tn); diff(got, want) > 1 {
		t.Errorf("«Целиком» с тоном: %v, ждали %v ±1", got, want)
	}
	for _, y := range []float64{57, 93} {
		if got := at(30, y); got != magenta {
			t.Errorf("«Целиком», поле ячейки y=%v: %v — тон задел страницу", y, got)
		}
	}
}

// Пути 2 и 3 (наклон фото и поворот места на произвольный угол): тон тот же, что на пути 1.
func TestRenderToneRotatedPath(t *testing.T) {
	src := color.RGBA{30, 160, 220, 255}
	path := tonePNG(t, src)
	cell := layout.Rect{X: 30, Y: 30, W: 40, H: 40}
	for _, tn := range []Tone{{Gray: true}, {Bright: -40, Contrast: 45}} {
		for _, a := range [][2]float64{{30, 0}, {0, 12}, {-135, 7}} {
			ph := Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1, 0, 0, a[1]), Rot: a[0], ImgRot: a[1], Path: path, Tone: tn}
			img := renderOn(t, ph, magenta, shapeDPI)
			if got, want := img.RGBAAt(px(50, shapeDPI), px(50, shapeDPI)), toneFloat(src, tn); diff(got, want) > 1 {
				t.Errorf("%+v, поворот %v: %v, ждали %v ±1", tn, a, got, want)
			}
			if got := img.RGBAAt(px(2, shapeDPI), px(2, shapeDPI)); got != magenta {
				t.Errorf("%+v, поворот %v: страница %v", tn, a, got)
			}
		}
	}
}

// Цена тона: страж «не стало квадратичным/медленным в разы». Критерий 068 (2 Мп ≤ 10 мс, 24 Мп
// ≤ 80 мс) — по BenchmarkRenderTone на спокойной машине (3,3 мс); здесь порог с запасом ×5:
// под параллельными прогонами пакетов 2 Мп доходит до 11 мс, а порог 10 мс ронял бы тест зря.
func TestToneCost(t *testing.T) {
	sizes := []struct {
		w, h int
		lim  time.Duration
	}{{1772, 1181, 50 * time.Millisecond}, {6000, 4000, 400 * time.Millisecond}}
	if testkit.Lite() {
		sizes = sizes[:1]
	}
	for _, s := range sizes {
		im := image.NewNRGBA(image.Rect(0, 0, s.w, s.h))
		d := testkit.Fastest(3, func() { applyTone(im, Tone{Gray: true, Bright: 10, Contrast: 10}) })
		if lim := testkit.Limit(s.lim); d > lim {
			t.Errorf("%d×%d: %v, предел %v", s.w, s.h, d, lim)
		}
	}
}

func BenchmarkRenderTone(b *testing.B) {
	im := image.NewNRGBA(image.Rect(0, 0, 1772, 1181))
	for b.Loop() {
		applyTone(im, Tone{Gray: true, Bright: 10, Contrast: 10})
	}
}
