package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// pt — точка в мм разворота.
type pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// textboxCase — общий вектор геометрии блока для Go и frontend (geom/textbox.ts,
// textbox.test.ts). Вход: рамка x, y, w, h, угол rot, выравнивание, новый угол deg.
// Выход: aabb рамки, rotated — угол P после поворота на norm вокруг центра,
// anchor — точка выравнивания (при rot), norm = NormDeg(deg).
type textboxCase struct {
	X       float64     `json:"x"`
	Y       float64     `json:"y"`
	W       float64     `json:"w"`
	H       float64     `json:"h"`
	Rot     float64     `json:"rot"`
	Align   string      `json:"align"`
	Deg     float64     `json:"deg"`
	AABB    layout.Rect `json:"aabb"`
	Rotated pt          `json:"rotated"`
	Anchor  pt          `json:"anchor"`
	Norm    float64     `json:"norm"`
}

func textboxCases() []textboxCase {
	return []textboxCase{
		{X: 100, Y: 50, W: 80, H: 20, Rot: 0, Align: "left", Deg: 30},
		{X: 100, Y: 50, W: 80, H: 20, Rot: 30, Align: "center", Deg: 0},
		{X: 100, Y: 50, W: 80, H: 20, Rot: 90, Align: "right", Deg: -90},
		{X: 10, Y: 200, W: 120.5, H: 33.3, Rot: -135, Align: "center", Deg: 190},
		{X: 0, Y: 0, W: 10, H: 5, Rot: 180, Align: "left", Deg: -180},
		{X: 300, Y: 150, W: 60, H: 12, Rot: -45.5, Align: "right", Deg: 360.04},
		{X: 42, Y: 17, W: 284, H: 100, Rot: 12.3, Align: "center", Deg: -540},
		{X: 5, Y: 5, W: 30, H: 8, Rot: 179.9, Align: "left", Deg: -179.96},
		{X: 250, Y: 120, W: 77.7, H: 9.9, Rot: -0.1, Align: "right", Deg: 0.04},
	}
}

func (c textboxCase) box() TextBox { return TextBox{c.X, c.Y, c.W, c.H, c.Rot} }

func nearPt(ax, ay float64, b pt) bool { return math.Abs(ax-b.X) < 1e-9 && math.Abs(ay-b.Y) < 1e-9 }

func TestTextGeomVectors(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "textbox.json")
	if *update {
		cs := textboxCases()
		for i := range cs {
			b := cs[i].box()
			cs[i].Norm = NormDeg(cs[i].Deg)
			cs[i].AABB = b.AABB()
			cs[i].Rotated.X, cs[i].Rotated.Y = b.RotateAbout(cs[i].Norm)
			cs[i].Anchor.X, cs[i].Anchor.Y = b.Anchor(cs[i].Align)
		}
		data, _ := json.MarshalIndent(cs, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cs []textboxCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(textboxCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		b := c.box()
		if NormDeg(c.Deg) != c.Norm || !near(b.AABB(), c.AABB) {
			t.Errorf("вектор %d: norm %v aabb %+v", i, NormDeg(c.Deg), b.AABB())
		}
		if x, y := b.RotateAbout(c.Norm); !nearPt(x, y, c.Rotated) {
			t.Errorf("вектор %d: поворот (%v, %v), ждали %+v", i, x, y, c.Rotated)
		}
		if x, y := b.Anchor(c.Align); !nearPt(x, y, c.Anchor) {
			t.Errorf("вектор %d: якорь (%v, %v), ждали %+v", i, x, y, c.Anchor)
		}
		// Поворот вокруг центра центр не двигает.
		r := b
		r.X, r.Y = b.RotateAbout(c.Norm)
		r.Rot = c.Norm
		cx, cy := b.Center()
		if x, y := r.Center(); math.Abs(x-cx) > 1e-9 || math.Abs(y-cy) > 1e-9 {
			t.Errorf("вектор %d: центр сдвинулся", i)
		}
	}
	// Ручная сверка: 90° — рамка 80×20 стоит вертикально левее угла P.
	if !near(cs[2].AABB, layout.Rect{X: 80, Y: 50, W: 20, H: 80}) {
		t.Fatalf("90°: %+v", cs[2].AABB)
	}
}

func TestNormDeg(t *testing.T) {
	for _, c := range [][2]float64{{190, -170}, {-180, 180}, {180, 180}, {360.04, 0}, {-0.04, 0}, {-540, 180},
		{30.04, 30}, {-179.96, 180}, {1e308, NormDeg(math.Mod(1e308, 360))}, {math.NaN(), 0}, {math.Inf(-1), 0}} {
		if got := NormDeg(c[0]); got != c[1] || math.Signbit(got) && got == 0 {
			t.Errorf("NormDeg(%v) = %v, ждали %v", c[0], got, c[1])
		}
	}
}

func TestFitAxis(t *testing.T) {
	cases := []struct{ x, lo, hi, want float64 }{
		{5, 0, 10, 5},           // внутри — как есть
		{-3.123, 0, 10, 0},      // за левым краем — к нему
		{12, 0.004, 10.006, 10}, // за правым — внутрь (вниз до сотых)
		{-1, 0.004, 10, 0.01},   // внутрь — вверх до сотых
		{3, 2, 1, 2},            // шире — к левому краю
		{0.001, 0.0005, 0.009, 0.01},
	}
	for _, c := range cases {
		got := FitAxis(c.x, c.lo, c.hi)
		if math.Abs(got-c.want) > 1e-9 || FitAxis(got, c.lo, c.hi) != got {
			t.Errorf("FitAxis(%v, %v, %v) = %v, ждали %v", c.x, c.lo, c.hi, got, c.want)
		}
	}
}

func TestFitAxisNaN(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range [][3]float64{{0, nan, 5}, {0, 0, nan}, {nan, 0, 5}, {0, -inf, 5}, {0, 0, inf}} {
		if got := FitAxis(c[0], c[1], c[2]); got != 0 {
			t.Fatalf("FitAxis(%v) = %v, ждали 0", c, got)
		}
	}
	if got := FitAxis(3, 0, 5); got != 3 {
		t.Fatalf("обычный случай изменён: %v", got)
	}
}
