package geom

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Наклон держит приближение (спека 057, решения 9, 10, 13).

// centerPt — точка фото в центре места: центр фото от центра места в осях фото, доли W и H.
func centerPt(cell, img layout.Rect, r float64) (float64, float64) {
	u, v := Rotate(-r, img.X+img.W/2-(cell.X+cell.W/2), img.Y+img.H/2-(cell.Y+cell.H/2))
	return u / img.W, v / img.H
}

// Таблица примеров спеки: место 150×100, фото 3:2, прямоугольник — если не сказано иное.
func TestTiltCropRule(t *testing.T) {
	wide := layout.Rect{X: 7, Y: 9, W: 150, H: 100}
	sq := layout.Rect{W: 100, H: 100}
	oval := layout.Rect{W: 80, H: 100}
	for _, c := range []struct {
		name   string
		cell   layout.Rect
		a      float64
		o      Outline
		base   Crop
		r      float64
		want   Crop
		w0, w1 float64 // ширина фото на экране до и после, мм
	}{
		{"100 % растёт", wide, 1.5, Outline{}, Crop{S: 1}, 3, Crop{S: 1, R: 3}, 150, 161.57},
		{"150 % не растёт", wide, 1.5, Outline{}, Crop{S: 1.5}, 3, Crop{S: 1.393, R: 3}, 225, 225},
		{"обратно, новая сессия", wide, 1.5, Outline{}, Crop{S: 1.393, R: 3}, 0, Crop{S: 1.5}, 225, 225},
		{"упёрлось в 1", wide, 1.5, Outline{}, Crop{S: 1.5}, 25, Crop{S: 1, R: 25}, 225, 231},
		{"сдвиг", wide, 1.5, Outline{}, Crop{S: 1.5, X: 0.5}, 3, Crop{S: 1.393, X: 0.536, R: 3}, 225, 225},
		{"круг", sq, 1.5, Outline{Oval: true}, Crop{S: 1.5}, 10, Crop{S: 1.5, R: 10}, 225, 225},
		{"овал, фото 2:3", oval, 2.0 / 3, Outline{Oval: true}, Crop{S: 1.3}, 8, Crop{S: 1.293, R: 8}, 104, 104},
		{"скругление 10 мм", wide, 1.5, Outline{Radius: 10}, Crop{S: 1.5}, 3, Crop{S: 1.406, R: 3}, 225, 225},
		{"предел 4", wide, 1.5, Outline{}, Crop{S: 4, R: 20}, 0, Crop{S: 4}, 872, 600},
		{"отрицательный наклон", wide, 1.5, Outline{}, Crop{S: 1.5}, -3, Crop{S: 1.393, R: -3}, 225, 225},
		{"четверть 90", layout.Rect{W: 100, H: 150}, 1.5, Outline{}, Crop{S: 1.5, R: 90}, 93, Crop{S: 1.393, R: 93}, 225, 225},
	} {
		got := TiltCrop(c.cell, c.a, c.base, c.r, c.o)
		if math.Abs(got.S-c.want.S) > 5e-4 || math.Abs(got.X-c.want.X) > 5e-4 || math.Abs(got.Y-c.want.Y) > 5e-4 || got.R != c.want.R {
			t.Errorf("%s: %+v, ждали %+v", c.name, got, c.want)
		}
		w0, w1 := PlaceImageIn(c.cell, c.a, c.base, c.o).W, PlaceImageIn(c.cell, c.a, got, c.o).W
		if math.Abs(w0-c.w0) > 0.5 || math.Abs(w1-c.w1) > 0.5 {
			t.Errorf("%s: ширина %.2f → %.2f, ждали %v → %v", c.name, w0, w1, c.w0, c.w1)
		}
	}
	// Тот же угол — кадр побитно; S = 1 и сдвиг 0 не дают «-0».
	b := Crop{S: 1.737, X: -0.413, Y: 0.271, R: 12.3}
	if got := TiltCrop(wide, 1.5, b, 12.3, Outline{Radius: 5}); got != b {
		t.Fatalf("тот же угол: %+v", got)
	}
	if got := TiltCrop(wide, 1.5, Crop{S: 1}, -7, Outline{}); math.Signbit(got.X) || math.Signbit(got.Y) {
		t.Fatalf("минус ноль: %+v", got)
	}
	// Края: пропорции не число, вырожденное место — S и ход конечны, кадр в пределах.
	for _, c := range []struct {
		cell layout.Rect
		a    float64
	}{{wide, math.NaN()}, {wide, 0}, {wide, math.Inf(1)}, {layout.Rect{}, 1.5}, {layout.Rect{W: 100}, 1.5}} {
		got := TiltCrop(c.cell, c.a, Crop{S: 2, X: 1, Y: -1}, 5, Outline{})
		if !(got.S >= 1 && got.S <= MaxScale) || !(math.Abs(got.X) <= 1) || !(math.Abs(got.Y) <= 1) {
			t.Fatalf("края %+v a=%v: %+v", c.cell, c.a, got)
		}
	}
}

// Свойства на случайных кадрах: фото закрывает место; если S не упёрлось в предел и сдвиг
// не в упоре, размер на экране и точка фото в центре места те же.
func TestTiltCropHolds(t *testing.T) {
	rng := rand.New(rand.NewSource(57))
	for range 1000 {
		cell := layout.Rect{X: rng.Float64() * 200, Y: rng.Float64() * 200, W: 20 + rng.Float64()*200, H: 20 + rng.Float64()*200}
		a := 0.4 + rng.Float64()*2.5
		var o Outline
		switch rng.Intn(3) {
		case 1:
			o = Outline{Oval: true}
		case 2:
			o = Outline{Radius: rng.Float64() * min(cell.W, cell.H) / 2}
		}
		turn := float64(90 * rng.Intn(4))
		base := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1, R: turn + math.Round(rng.Float64()*898-449)/10}
		r := turn + math.Round(rng.Float64()*898-449)/10
		got := TiltCrop(cell, a, base, r, o)
		img0, img1 := PlaceImageIn(cell, a, base, o), PlaceImageIn(cell, a, got, o)
		if !coversShape(img1, cell, r, o) {
			t.Fatalf("открыто: cell %+v a %.3f %+v → %+v", cell, a, base, got)
		}
		if got.S <= 1 || got.S >= MaxScale {
			continue
		}
		if math.Abs(img1.W-img0.W) > 1e-9*img0.W {
			t.Fatalf("размер %v → %v: %+v → %+v", img0.W, img1.W, base, got)
		}
		x0, y0 := centerPt(cell, img0, base.R)
		x1, y1 := centerPt(cell, img1, r)
		if math.Abs(got.X) < 1 && math.Abs(x1-x0) > 1e-9 || math.Abs(got.Y) < 1 && math.Abs(y1-y0) > 1e-9 {
			t.Fatalf("центр (%v, %v) → (%v, %v): %+v → %+v", x0, y0, x1, y1, base, got)
		}
	}
}

// tiltCase — вектор testdata/parity/tilt.json (frame.ts tiltFrame), до округления.
type tiltCase struct {
	Cell    layout.Rect `json:"cell"`
	Aspect  float64     `json:"aspect"`
	Outline Outline     `json:"outline,omitzero"`
	Base    Crop        `json:"base"`
	R       float64     `json:"r"`
	Want    Crop        `json:"want"`
}

func tiltCases() []tiltCase {
	wide := layout.Rect{X: 10, Y: 20, W: 150, H: 100}
	sq := layout.Rect{X: 5, Y: 5, W: 100, H: 100}
	return []tiltCase{
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1}, R: 3},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1.5}, R: 3},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1.393, R: 3}, R: 0},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1.5}, R: 25},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1.5, X: 0.5, Y: -0.3}, R: -3},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 4, R: 20}, R: 0},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 4, X: -1, Y: 1}, R: 10},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1, X: 1, Y: -1}, R: -44.9},
		{Cell: sq, Aspect: 1.5, Outline: Outline{Oval: true}, Base: Crop{S: 1.5, X: 0.2}, R: 10},
		{Cell: layout.Rect{W: 80, H: 100}, Aspect: 2.0 / 3, Outline: Outline{Oval: true}, Base: Crop{S: 1.3, Y: 0.4}, R: 8},
		{Cell: wide, Aspect: 1.5, Outline: Outline{Radius: 10}, Base: Crop{S: 1.5}, R: 3},
		{Cell: layout.Rect{W: 100, H: 60}, Aspect: 1.5, Outline: Outline{Radius: 30}, Base: Crop{S: 2.2, X: -0.7, Y: 0.6, R: 12.5}, R: -20},
		{Cell: layout.Rect{W: 100, H: 150}, Aspect: 1.5, Base: Crop{S: 1.5, R: 90}, R: 93},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 2, X: 0.3, Y: 0.3, R: 180}, R: 175.5},
		{Cell: sq, Aspect: 0.75, Base: Crop{S: 1.8, X: -0.5, R: 270}, R: 254.1},
		{Cell: wide, Aspect: 1.5, Base: Crop{S: 1.7, X: 0.1, R: 7}, R: 7},
	}
}

func TestTiltParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "tilt.json")
	if *update {
		cs := tiltCases()
		for i := range cs {
			cs[i].Want = TiltCrop(cs[i].Cell, cs[i].Aspect, cs[i].Base, cs[i].R, cs[i].Outline)
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
	var cs []tiltCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(tiltCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		got := TiltCrop(c.Cell, c.Aspect, c.Base, c.R, c.Outline)
		if math.Abs(got.S-c.Want.S) > 1e-9 || math.Abs(got.X-c.Want.X) > 1e-9 || math.Abs(got.Y-c.Want.Y) > 1e-9 || got.R != c.Want.R {
			t.Errorf("вектор %d: %+v, ждали %+v", i, got, c.Want)
		}
	}
	// Ручная сверка: 150 % при 3° — 1,393.
	if math.Abs(cs[1].Want.S-1.3926) > 1e-4 {
		t.Fatalf("вектор 1: %+v", cs[1].Want)
	}
}

// containCase — вектор testdata/parity/contain.json (frame.ts containImage).
type containCase struct {
	Cell    layout.Rect `json:"cell"`
	Aspect  float64     `json:"aspect"`
	R       float64     `json:"r"`
	Outline Outline     `json:"outline,omitzero"`
	Img     layout.Rect `json:"img"`
}

func containCases() []containCase {
	cell := layout.Rect{X: 12, Y: 30, W: 120, H: 80}
	var cs []containCase
	for _, o := range []Outline{{}, {Oval: true}, {Radius: 15}} {
		for _, r := range []float64{0, 30, -44.9, 90} {
			cs = append(cs, containCase{Cell: cell, Aspect: 1.5, R: r, Outline: o})
		}
	}
	return append(cs,
		containCase{Cell: layout.Rect{W: 60, H: 90}, Aspect: 0.75, R: 12.5, Outline: Outline{Oval: true}},
		containCase{Cell: layout.Rect{W: 100, H: 100}, Aspect: 2, R: -17, Outline: Outline{Radius: 50}},
	)
}

func TestContainParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "contain.json")
	if *update {
		cs := containCases()
		for i := range cs {
			cs[i].Img = ContainImageIn(cs[i].Cell, cs[i].Aspect, cs[i].R, cs[i].Outline)
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
	var cs []containCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(containCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		if img := ContainImageIn(c.Cell, c.Aspect, c.R, c.Outline); !near(img, c.Img) {
			t.Errorf("вектор %d: %+v, ждали %+v", i, img, c.Img)
		}
	}
	// Ручная сверка: 3:2 в 120×80 без наклона — во всё место.
	if !near(cs[0].Img, layout.Rect{X: 12, Y: 30, W: 120, H: 80}) {
		t.Fatalf("вектор 0: %+v", cs[0].Img)
	}
}
