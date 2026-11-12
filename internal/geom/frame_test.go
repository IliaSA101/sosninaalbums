package geom

import (
	"encoding/json"
	"flag"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

var update = flag.Bool("update", false, "перезаписать testdata/parity/frames.json")

// frameCase — общий вектор кадра для Go и frontend (geom/frame.ts, frame.test.ts).
type frameCase struct {
	Cell   layout.Rect `json:"cell"`
	Aspect float64     `json:"aspect"`
	Frame  Crop        `json:"frame"`
	// Outline — форма места (044); нет поля — прямоугольник.
	Outline Outline     `json:"outline,omitzero"`
	Px      int         `json:"px"` // ширина фото в пикселях (PhotoRef.W); 0 — неизвестна
	Img     layout.Rect `json:"img"`
	DPI     int         `json:"dpi"`
}

func frameCases() []frameCase {
	sq := layout.Rect{X: 10, Y: 20, W: 100, H: 100}
	return []frameCase{
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 2, X: -1, Y: -1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 2, X: 1, Y: 1}, Px: 6000},
		{Cell: layout.Rect{W: 80, H: 120}, Aspect: 1.5, Frame: Crop{S: 1, X: 0.5}, Px: 6000},
		{Cell: layout.Rect{X: 300, Y: 12, W: 150, H: 100}, Aspect: 1.5, Frame: Crop{S: 1, X: 1, Y: -1}, Px: 6000},
		{Cell: layout.Rect{X: 312, Y: 12, W: 100, H: 100}, Aspect: 4000.0 / 6000, Frame: Crop{S: 1.35, X: -0.4, Y: 0.25}, Px: 4000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 4, X: 0.123, Y: -0.5}, Px: 0},
		// Поворот фото в рамке (010): четверти, наклон, ход по осям фото.
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 90}, Px: 6000},
		{Cell: layout.Rect{X: 10, Y: 20, W: 80, H: 120}, Aspect: 1.5, Frame: Crop{S: 1, R: -90, X: 0.5}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 180, X: 1, Y: -1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 30}, Px: 6000},
		{Cell: layout.Rect{X: 300, Y: 12, W: 150, H: 100}, Aspect: 1.5, Frame: Crop{S: 1.5, X: 0.5, Y: -0.75, R: -12.3}, Px: 6000},
		{Cell: layout.Rect{X: 312, Y: 12, W: 100, H: 100}, Aspect: 4000.0 / 6000, Frame: Crop{S: 2.25, X: -1, Y: 1, R: 44.9}, Px: 4000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 3, X: 0.3, Y: 0.6, R: 135.1}, Px: 0},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, X: 1, Y: 1, R: -135.1}, Px: 6000},
		// Форма места (044): габарит формы в осях фото; без наклона — как прямоугольник.
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 30}, Px: 6000, Outline: Outline{Oval: true}},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1.2, X: 1, Y: -1}, Px: 6000, Outline: Outline{Oval: true}},
		{Cell: layout.Rect{X: 40, Y: 15, W: 60, H: 80}, Aspect: 0.75, Frame: Crop{S: 1, R: 12}, Px: 3000, Outline: Outline{Oval: true}},
		{Cell: layout.Rect{X: 40, Y: 15, W: 80, H: 60}, Aspect: 1.5, Frame: Crop{S: 1.7, X: -0.6, Y: 0.4, R: -44.9}, Px: 6000, Outline: Outline{Oval: true}},
		{Cell: layout.Rect{X: 5, Y: 5, W: 100, H: 60}, Aspect: 1.5, Frame: Crop{S: 1, R: 20}, Px: 6000, Outline: Outline{Radius: 10}},
		{Cell: layout.Rect{X: 5, Y: 5, W: 100, H: 60}, Aspect: 1.5, Frame: Crop{S: 1.5, X: 0.5, Y: 1, R: 135.1}, Px: 6000, Outline: Outline{Radius: 30}},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 90}, Px: 6000, Outline: Outline{Radius: 25}},
	}
}

func near(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

func TestFramesParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "frames.json")
	if *update {
		cs := frameCases()
		for i := range cs {
			cs[i].Img = PlaceImageIn(cs[i].Cell, cs[i].Aspect, cs[i].Frame, cs[i].Outline)
			cs[i].DPI = FrameDPI(cs[i].Px, cs[i].Img)
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
	var cs []frameCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(frameCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		img := PlaceImageIn(c.Cell, c.Aspect, c.Frame, c.Outline)
		if !near(img, c.Img) || FrameDPI(c.Px, img) != c.DPI {
			t.Errorf("вектор %d: %+v dpi %d, ждали %+v dpi %d", i, img, FrameDPI(c.Px, img), c.Img, c.DPI)
		}
		// Фото всегда закрывает ячейку целиком (у формы — саму форму).
		if c.Outline != (Outline{}) {
			if !coversShape(img, c.Cell, c.Frame.R, c.Outline) {
				t.Errorf("вектор %d: форма открыта", i)
			}
		} else if !coversCell(img, c.Cell, c.Frame.R) {
			t.Errorf("вектор %d: ячейка открыта", i)
		}
	}
	// Ручная сверка: 3:2 в квадрате 100 мм, S=1 — 150×100 по центру; 6000 px на 150 мм ≈ 1016 dpi.
	if !near(cs[0].Img, layout.Rect{X: -15, Y: 20, W: 150, H: 100}) || cs[0].DPI != 1016 {
		t.Fatalf("вектор 0: %+v %d", cs[0].Img, cs[0].DPI)
	}
}

// coversCell — фото img, повёрнутое на r вокруг своего центра, закрывает ячейку cell:
// все её углы, переведённые в оси фото, лежат внутри img.
func coversCell(img, cell layout.Rect, r float64) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	const eps = 1e-7
	for _, c := range [4][2]float64{{0, 0}, {cell.W, 0}, {0, cell.H}, {cell.W, cell.H}} {
		u, v := Rotate(-r, cell.X+c[0]-cx, cell.Y+c[1]-cy)
		if math.Abs(u) > img.W/2+eps || math.Abs(v) > img.H/2+eps {
			return false
		}
	}
	return true
}

// Закрытие точное при любых θ, S, X, Y; ход сдвига до краёв.
func TestPlaceImageRotCovers(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for range 3000 {
		cell := layout.Rect{X: rng.Float64() * 300, Y: rng.Float64() * 300, W: 10 + rng.Float64()*200, H: 10 + rng.Float64()*200}
		f := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1, R: NormDeg(rng.Float64()*360 - 180)}
		switch rng.Intn(4) { // края хода и четверти — самые острые случаи
		case 0:
			f.X, f.Y = math.Round(f.X), math.Round(f.Y)
		case 1:
			f.R = float64(90 * (rng.Intn(4) - 1))
		}
		a := 0.3 + rng.Float64()*3
		if img := PlaceImage(cell, a, f); !coversCell(img, cell, f.R) {
			t.Fatalf("ячейка открыта: cell %+v, a %.3f, кадр %+v, Img %+v", cell, a, f, img)
		}
		// Вписанное фото (contain): углы фото в рамке, пропорции фото.
		if c := ContainImage(cell, a, f.R); !inside(c, f.R, cell) || math.Abs(c.W/c.H-a) > 1e-9 {
			t.Fatalf("contain %+v вне рамки %+v (a %.3f, R %v)", c, cell, a, f.R)
		}
	}
}

// inside — img, повёрнутое на r вокруг своего центра, лежит в рамке cell.
func inside(img layout.Rect, r float64, cell layout.Rect) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	for _, uv := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		dx, dy := Rotate(r, uv[0]*img.W/2, uv[1]*img.H/2)
		x, y := cx+dx, cy+dy
		if x < cell.X-1e-9 || x > cell.X+cell.W+1e-9 || y < cell.Y-1e-9 || y > cell.Y+cell.H+1e-9 {
			return false
		}
	}
	return true
}

// R = 0 — прежняя формула дословно (до последнего бита).
func TestPlaceImageRotZeroSame(t *testing.T) {
	old := func(cell layout.Rect, aspect float64, f Crop) layout.Rect {
		w, h := cell.W, cell.W/aspect
		if h < cell.H {
			h, w = cell.H, cell.H*aspect
		}
		w, h = w*f.S, h*f.S
		fx, fy := w-cell.W, h-cell.H
		return layout.Rect{X: cell.X - fx*(1+f.X)/2, Y: cell.Y - fy*(1+f.Y)/2, W: w, H: h}
	}
	rng := rand.New(rand.NewSource(3))
	for range 500 {
		cell := layout.Rect{X: rng.Float64() * 600, Y: rng.Float64() * 300, W: 10 + rng.Float64()*280, H: 10 + rng.Float64()*280}
		f := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1}
		a := 0.3 + rng.Float64()*3
		want := old(cell, a, f)
		if got := PlaceImage(cell, a, f); got != want {
			t.Fatalf("%+v != %+v", got, want)
		}
		if r := placeImageRot(cell, a, f); !near6(r, want) {
			t.Fatalf("ветка с поворотом при 0 — %+v, ждали %+v", r, want)
		}
	}
}

func near6(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6 && math.Abs(a.W-b.W) < 1e-6 && math.Abs(a.H-b.H) < 1e-6
}

// contain с поворотом: повёрнутое фото целиком внутри ячейки и упирается хотя бы в одну сторону.
func TestContainImageRot(t *testing.T) {
	cell := layout.Rect{X: 10, Y: 20, W: 120, H: 80}
	for _, r := range []float64{0, 90, -90, 180, 30, -44.9, 135.1} {
		img := ContainImage(cell, 1.5, r)
		sn, cs := sinCosDeg(r)
		c, s := math.Abs(cs), math.Abs(sn)
		bw, bh := img.W*c+img.H*s, img.W*s+img.H*c
		if bw > cell.W+1e-9 || bh > cell.H+1e-9 || math.Abs(bw-cell.W) > 1e-9 && math.Abs(bh-cell.H) > 1e-9 {
			t.Errorf("R=%v: описанный %.3f×%.3f в ячейке %v×%v", r, bw, bh, cell.W, cell.H)
		}
		if math.Abs(img.X+img.W/2-(cell.X+cell.W/2)) > 1e-9 || math.Abs(img.Y+img.H/2-(cell.Y+cell.H/2)) > 1e-9 {
			t.Errorf("R=%v: не по центру: %+v", r, img)
		}
	}
	if ContainImage(cell, 1.5, 0) != (layout.Rect{X: 10, Y: 20, W: 120, H: 80}) {
		t.Fatal("R = 0: прежняя формула")
	}
}

// lineAt — линия длиной l мм под углом a° к краю листа (по часовой, y вниз).
func lineAt(a, l float64) (float64, float64) {
	s, c := math.Sincos(a * math.Pi / 180)
	return l * c, l * s
}

// Выпрямить по линии (045, design §2.2): угол линии к краю рамки → поворот фото.
func TestStraightenDelta(t *testing.T) {
	cases := []struct {
		name        string
		angle, want float64
	}{
		{"ровно", 0, 0},
		{"+4,5", 4.5, -4.5},
		{"−4,5", -4.5, 4.5},
		{"справа налево +4,5", 184.5, -4.5},
		{"справа налево −4,5", 175.5, 4.5},
		{"вертикаль 93", 93, -3},
		{"вертикаль −93", -93, 3},
		{"вертикаль снизу вверх 267", 267, 3},
		{"45 — горизонталь", 45, -45},
		{"−45 — горизонталь", -45, 45},
		{"135 — горизонталь", 135, 45},
		{"45,1 — вертикаль", 45.1, 44.9},
		{"90", 90, 0},
		{"180", 180, 0},
	}
	for _, c := range cases {
		dx, dy := lineAt(c.angle, 37)
		got, ok := StraightenDelta(dx, dy, 0)
		if !ok || math.Abs(got-c.want) > 1e-9 || got == 0 && math.Signbit(got) {
			t.Errorf("%s: %v %v, ждали %v", c.name, got, ok, c.want)
		}
	}
	// Ровно 45° по точным координатам — горизонталь, хвост atan2 не решает.
	for _, v := range [][3]float64{{1, 1, -45}, {-1, -1, -45}, {1, -1, 45}, {-1, 1, 45}} {
		if got, ok := StraightenDelta(v[0], v[1], 0); !ok || got != v[2] {
			t.Errorf("(%v, %v): %v, ждали %v", v[0], v[1], got, v[2])
		}
	}
	for _, bad := range [][3]float64{{0, 0, 0}, {0.005, 0.005, 0}, {math.NaN(), 1, 0}, {1, math.Inf(1), 0}, {1, 1, math.NaN()}, {1, 1, math.Inf(-1)}} {
		if got, ok := StraightenDelta(bad[0], bad[1], bad[2]); ok || got != 0 {
			t.Errorf("%v: %v %v, ждали холостую", bad, got, ok)
		}
	}
	// Огромная линия — конечный ответ.
	if got, ok := StraightenDelta(1e9, 1e9*math.Tan(4.5*math.Pi/180), 0); !ok || math.Abs(got+4.5) > 1e-6 {
		t.Errorf("1e9: %v %v", got, ok)
	}
}

// Рамка повёрнута: линия ровняется по её краю, а не по краю листа.
func TestStraightenDeltaPlaceRot(t *testing.T) {
	cases := []struct{ line, rot, want float64 }{
		{14.5, 10, -4.5},
		{10, 10, 0},
		{5.5, 10, 4.5},
		{103, 10, -3}, // вертикаль рамки под 10° — это 100°
		{-170, 10, 0}, // та же линия, проведённая наоборот
		{0, 90, 0},    // четверть: край рамки снова вдоль листа
		{4.5, -90, -4.5},
		{-175.5, 180, -4.5},
		{40, -10, 40}, // 50° к краю — вертикаль: 50 − 90 = −40
	}
	for _, c := range cases {
		dx, dy := lineAt(c.line, 50)
		got, ok := StraightenDelta(dx, dy, c.rot)
		if !ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("линия %v, рамка %v: %v %v, ждали %v", c.line, c.rot, got, ok, c.want)
		}
	}
}

// shapePoints — точки контура формы o рамки cell (мм листа): эллипс — n точек, скругление —
// дуги углов (при ρ = 0 — углы рамки); отрезки между ними выпуклое фото покрывает само.
func shapePoints(cell layout.Rect, o Outline, n int) [][2]float64 {
	cx, cy := cell.X+cell.W/2, cell.Y+cell.H/2
	var out [][2]float64
	if o.Oval {
		for k := range n {
			t := 2 * math.Pi * float64(k) / float64(n)
			out = append(out, [2]float64{cx + cell.W/2*math.Cos(t), cy + cell.H/2*math.Sin(t)})
		}
		return out
	}
	r := o.radius(cell.W, cell.H)
	for _, q := range [4][2]float64{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
		ox, oy := cx+q[0]*(cell.W/2-r), cy+q[1]*(cell.H/2-r)
		for k := 0; k <= n/4; k++ {
			t := math.Pi / 2 * float64(k) / float64(n/4)
			out = append(out, [2]float64{ox + q[0]*r*math.Cos(t), oy + q[1]*r*math.Sin(t)})
		}
	}
	return out
}

// coversShape — фото img, повёрнутое на r вокруг своего центра, закрывает форму o рамки cell.
func coversShape(img, cell layout.Rect, r float64, o Outline) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	for _, p := range shapePoints(cell, o, 4000) {
		u, v := Rotate(-r, p[0]-cx, p[1]-cy)
		if math.Abs(u) > img.W/2+1e-7 || math.Abs(v) > img.H/2+1e-7 {
			return false
		}
	}
	return true
}

// Габарит формы в осях фото (044 design §2.1): таблица замеров, сверка с контуром,
// R = 0 — PlaceImage побитно, фото закрывает форму при любом кадре.
func TestPlaceImageOutline(t *testing.T) {
	circle := Outline{Oval: true}
	cases := []struct {
		name   string
		w, h   float64
		o      Outline
		deg    float64
		bw, bh float64
	}{
		{"круг 100 под 30°", 100, 100, circle, 30, 100, 100},
		{"квадрат 100 под 30°", 100, 100, Outline{}, 30, 136.603, 136.603},
		{"овал 60×80 под 12°", 60, 80, circle, 12, 61.000, 79.240},
		{"скругление 10 у 100×60 под 20°", 100, 60, Outline{Radius: 10}, 20, 108.856, 84.949},
		{"скругление больше половины — полукруги", 100, 60, Outline{Radius: 99}, 0, 100, 60},
	}
	for _, c := range cases {
		sn, cs := sinCosDeg(c.deg)
		bw, bh := c.o.bound(c.w, c.h, math.Abs(cs), math.Abs(sn))
		if math.Abs(bw-c.bw) > 5e-4 || math.Abs(bh-c.bh) > 5e-4 {
			t.Errorf("%s: %.4f×%.4f, ждали %.3f×%.3f", c.name, bw, bh, c.bw, c.bh)
		}
		// Сверка с контуром: наибольшие |u|, |v| точек формы в осях фото.
		var mu, mv float64
		for _, p := range shapePoints(layout.Rect{W: c.w, H: c.h}, c.o, 40000) {
			u, v := Rotate(-c.deg, p[0]-c.w/2, p[1]-c.h/2)
			mu, mv = max(mu, math.Abs(u)), max(mv, math.Abs(v))
		}
		if math.Abs(2*mu-bw) > 1e-3 || math.Abs(2*mv-bh) > 1e-3 {
			t.Errorf("%s: контур %.4f×%.4f, формула %.4f×%.4f", c.name, 2*mu, 2*mv, bw, bh)
		}
	}
	// Круг: наклон не увеличивает фото (k = 1).
	if img := PlaceImageIn(layout.Rect{W: 100, H: 100}, 1.5, Crop{S: 1, R: 30}, circle); math.Abs(img.W-150) > 1e-9 || math.Abs(img.H-100) > 1e-9 {
		t.Errorf("круг под 30°: %+v, ждали 150×100", img)
	}
	rng := rand.New(rand.NewSource(44))
	for range 1500 {
		cell := layout.Rect{X: rng.Float64() * 300, Y: rng.Float64() * 300, W: 10 + rng.Float64()*200, H: 10 + rng.Float64()*200}
		f := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1, R: NormDeg(rng.Float64()*360 - 180)}
		if rng.Intn(3) == 0 {
			f.X, f.Y = math.Round(f.X), math.Round(f.Y)
		}
		o := circle
		if rng.Intn(2) == 0 {
			o = Outline{Radius: rng.Float64() * min(cell.W, cell.H) * 0.6}
		}
		a := 0.3 + rng.Float64()*3
		img := PlaceImageIn(cell, a, f, o)
		if !coversShape(img, cell, f.R, o) {
			t.Fatalf("форма открыта: cell %+v, a %.3f, кадр %+v, форма %+v, Img %+v", cell, a, f, o, img)
		}
		// Форма внутри рамки: фото не крупнее, чем у прямоугольника.
		if rect := PlaceImage(cell, a, f); img.W > rect.W+1e-9 {
			t.Fatalf("форма увеличила фото: %+v > %+v", img, rect)
		}
		f.R = 0
		if got, want := PlaceImageIn(cell, a, f, o), PlaceImage(cell, a, f); got != want {
			t.Fatalf("R = 0: %+v != %+v", got, want)
		}
	}
}

// «Целиком» в форме (044 design §2.2): замеры эллипса, углы фото внутри формы, по центру.
func TestContainImageOutline(t *testing.T) {
	circle := Outline{Oval: true}
	cases := []struct {
		name string
		cell layout.Rect
		a, r float64
		o    Outline
		w, h float64
	}{
		{"круг 100, фото 1:1", layout.Rect{W: 100, H: 100}, 1, 0, circle, 70.711, 70.711},
		{"круг 100, фото 3:2", layout.Rect{X: 3, Y: 4, W: 100, H: 100}, 1.5, 0, circle, 83.205, 55.470},
		{"овал 60×80, фото 3:4, 10°", layout.Rect{W: 60, H: 80}, 0.75, 10, circle, 40.362, 53.817},
		{"круг, фото 1:1 под 45° — та же диагональ", layout.Rect{W: 100, H: 100}, 1, 45, circle, 70.711, 70.711},
		{"малое скругление не трогает", layout.Rect{W: 100, H: 60}, 1.5, 0, Outline{Radius: 5}, 90, 60},
	}
	for _, c := range cases {
		img := ContainImageIn(c.cell, c.a, c.r, c.o)
		if math.Abs(img.W-c.w) > 5e-4 || math.Abs(img.H-c.h) > 5e-4 {
			t.Errorf("%s: %.4f×%.4f, ждали %.3f×%.3f", c.name, img.W, img.H, c.w, c.h)
		}
		if math.Abs(img.X+img.W/2-(c.cell.X+c.cell.W/2)) > 1e-9 || math.Abs(img.Y+img.H/2-(c.cell.Y+c.cell.H/2)) > 1e-9 {
			t.Errorf("%s: не по центру %+v", c.name, img)
		}
	}
	if got, want := ContainImageIn(layout.Rect{W: 120, H: 80}, 1.5, 30, Outline{}), ContainImage(layout.Rect{W: 120, H: 80}, 1.5, 30); got != want {
		t.Fatalf("без формы: %+v != %+v", got, want)
	}
	rng := rand.New(rand.NewSource(45))
	for range 1500 {
		cell := layout.Rect{X: rng.Float64() * 300, Y: rng.Float64() * 300, W: 10 + rng.Float64()*200, H: 10 + rng.Float64()*200}
		r := NormDeg(rng.Float64()*360 - 180)
		o := circle
		if rng.Intn(2) == 0 {
			o = Outline{Radius: rng.Float64() * min(cell.W, cell.H) * 0.6}
		}
		a := 0.3 + rng.Float64()*3
		img := ContainImageIn(cell, a, r, o)
		if math.Abs(img.W/img.H-a) > 1e-9 {
			t.Fatalf("пропорции %v, ждали %v", img.W/img.H, a)
		}
		// Все четыре угла внутри формы, и хотя бы один — на её краю (вписано, а не мельче).
		cx, cy := cell.X+cell.W/2, cell.Y+cell.H/2
		touch := false
		for _, uv := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			x, y := Rotate(r, uv[0]*img.W/2, uv[1]*img.H/2)
			x += img.X + img.W/2 - cx
			y += img.Y + img.H/2 - cy
			if !o.contains(cell.W, cell.H, x, y, 1e-7) {
				t.Fatalf("угол фото вне формы: cell %+v, a %.3f, r %v, форма %+v, img %+v", cell, a, r, o, img)
			}
			touch = touch || !o.contains(cell.W, cell.H, x*1.0001, y*1.0001, 0)
		}
		if !touch {
			t.Fatalf("фото не вписано: cell %+v, a %.3f, r %v, форма %+v, img %+v", cell, a, r, o, img)
		}
	}
}

// Подпись «на фото» — внутри вписанного прямоугольника (044 design §2.3).
func TestInnerBox(t *testing.T) {
	cell := layout.Rect{X: 10, Y: 20, W: 100, H: 60}
	if InnerBox(cell, Outline{}) != cell {
		t.Fatal("прямоугольник — сама рамка")
	}
	ov := InnerBox(cell, Outline{Oval: true})
	if math.Abs(ov.W-100/math.Sqrt2) > 1e-9 || math.Abs(ov.H-60/math.Sqrt2) > 1e-9 ||
		math.Abs(ov.X+ov.W/2-60) > 1e-9 || math.Abs(ov.Y+ov.H/2-50) > 1e-9 {
		t.Errorf("эллипс: %+v", ov)
	}
	// Углы вписанного — на эллипсе.
	if x, y := ov.W/2, ov.H/2; math.Abs(x*x/2500+y*y/900-1) > 1e-9 {
		t.Errorf("угол не на эллипсе: %v %v", x, y)
	}
	d := 20 * (1 - math.Sqrt2/2)
	if rr := InnerBox(cell, Outline{Radius: 20}); !near(rr, layout.Rect{X: 10 + d, Y: 20 + d, W: 100 - 2*d, H: 60 - 2*d}) {
		t.Errorf("скругление: %+v", rr)
	}
	// Радиус сверх половины меньшей стороны зажат: 30, а не 99.
	if big := InnerBox(cell, Outline{Radius: 99}); math.Abs(big.X-(10+30*(1-math.Sqrt2/2))) > 1e-9 {
		t.Errorf("скругление 99: %+v", big)
	}
	for _, o := range []Outline{{Oval: true}, {Radius: 20}, {Radius: 30}} {
		b := InnerBox(cell, o)
		for _, c := range [4][2]float64{{b.X, b.Y}, {b.X + b.W, b.Y}, {b.X, b.Y + b.H}, {b.X + b.W, b.Y + b.H}} {
			if !o.contains(cell.W, cell.H, c[0]-60, c[1]-50, 1e-9) {
				t.Errorf("%+v: угол %v вне формы", o, c)
			}
		}
	}
}
