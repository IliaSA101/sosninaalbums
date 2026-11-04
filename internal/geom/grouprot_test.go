package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// grCase — вектор для Go и frontend (geom/grouprot.ts): вход и итог GroupRot.
type grCase struct {
	Name string   `json:"name"`
	In   GxRotIn  `json:"in"`
	Out  GxRotOut `json:"out"`
}

// mixGroup — место 0°, место 30°, текст −15° (центр рамки ≈ (230, 140)).
func mixGroup() ([]RotRect, []GxText) {
	return []RotRect{{X: 150, Y: 90, W: 100, H: 60}, {X: 270, Y: 110, W: 80, H: 50, Rot: 30}},
		[]GxText{{X: 170, Y: 170, W: 120, H: 20, Rot: -15, Size: 14, Al: 0.5}}
}

// wideGroup — два места 250×250 почти во всю ширину разворота 562×284 (design §5).
func wideGroup() []RotRect {
	return []RotRect{{X: 21, Y: 17, W: 250, H: 250}, {X: 291, Y: 17, W: 250, H: 250}}
}

func grCases() []grCase {
	in := func(deg, cx, cy float64, pl []RotRect, tx []GxText) GxRotIn {
		pl, tx = append([]RotRect{}, pl...), append([]GxText{}, tx...) // [] вместо null в JSON
		return GxRotIn{SW: 562, PH: 284, Places: pl, Texts: tx, Deg: deg, CX: cx, CY: cy}
	}
	pl, tx := mixGroup()
	edge := []RotRect{{X: 505, Y: 35, W: 50, H: 50}, {X: 505, Y: 195, W: 50, H: 50}}
	return []grCase{
		{Name: "холостая 0°", In: in(0, 230, 140, pl, tx)},
		{Name: "холостая 360°", In: in(360, 230, 140, pl, tx)},
		{Name: "поворот 25° вокруг центра", In: in(25, 230, 140, pl, tx)},
		{Name: "поворот −30° и обратно к 0° у места 30°", In: in(-30, 230, 140, pl, tx)},
		{Name: "у правого края — сдвиг внутрь", In: in(30, 530, 140, edge, nil)},
		{Name: "широкая на 45° — не упирается, по центру вылета", In: in(45, 281, 142, wideGroup(), nil)},
		{Name: "широкая на 90°", In: in(90, 281, 142, wideGroup(), nil)},
		{Name: "широкая на 180° — помещается", In: in(180, 281, 142, wideGroup(), nil)},
		{Name: "текст у края — целиком внутрь", In: in(60, 40, 40, nil, []GxText{{X: 10, Y: 10, W: 200, H: 30, Size: 20}, {X: 10, Y: 60, W: 50, H: 10, Size: 8}})},
		{Name: "угол шумом нормализуется", In: in(-200.04, 230, 140, pl, tx)},
		{Name: "пустая группа, 30°", In: in(30, 230, 140, nil, nil)},
		{Name: "одно место", In: in(30, 200, 120, pl[1:], nil)},
		{Name: "405° — это 45°", In: in(405, 230, 140, pl, tx)},
		{Name: "−720,05° — шум у полного круга", In: in(-720.05, 230, 140, pl, tx)},
	}
}

// TestGroupRotDeg — угол на выходе: нормализован, у пустой группы сдвиг 0.
func TestGroupRotDeg(t *testing.T) {
	for _, c := range []struct {
		deg, want float64
		pl        []RotRect
	}{{30, 30, nil}, {405, 45, wideGroup()}, {-720.05, 0, wideGroup()}, {-719.9, 0.1, wideGroup()}} {
		out := GroupRot(GxRotIn{SW: 562, PH: 284, Places: c.pl, Deg: c.deg, CX: 230, CY: 140})
		if out.Deg != c.want || out.DX != 0 && c.pl == nil || out.DY != 0 && c.pl == nil ||
			math.IsNaN(out.DX) || math.IsNaN(out.DY) || len(out.Places) != len(c.pl) {
			t.Errorf("%v°: %+v", c.deg, out)
		}
	}
}

func TestGroupRotParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "grouprot.json")
	if *update {
		cs := grCases()
		for i := range cs {
			cs[i].Out = GroupRot(cs[i].In)
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
	var cs []grCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(grCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for _, c := range cs {
		got := GroupRot(c.In)
		if got.Deg != c.Out.Deg || math.Abs(got.DX-c.Out.DX) > 1e-9 || math.Abs(got.DY-c.Out.DY) > 1e-9 ||
			!slices.EqualFunc(got.Places, c.Out.Places, gxPlaceNear) || !slices.EqualFunc(got.Texts, c.Out.Texts, gxNear) {
			t.Errorf("%s: %+v, ждали %+v", c.Name, got, c.Out)
		}
		inSpread(t, c.Name, got.Places, got.Texts, c.In.SW, c.In.PH)
	}
}

// inSpread — hold мест и aabb текстов внутри разворота.
func inSpread(t *testing.T, name string, pls []RotRect, txs []GxText, sw, ph float64) {
	t.Helper()
	for _, r := range pls {
		h := r.Hold()
		if h.X < -0.01 || h.Y < -0.01 || h.X+h.W > sw+0.01 || h.Y+h.H > ph+0.01 {
			t.Errorf("%s: место вне разворота %+v", name, r)
		}
	}
	for _, x := range txs {
		a := TextBox{x.X, x.Y, x.W, x.H, x.Rot}.AABB()
		if a.X < -0.01 || a.Y < -0.01 || a.X+a.W > sw+0.01 || a.Y+a.H > ph+0.01 {
			t.Errorf("%s: текст вне разворота %+v", name, x)
		}
	}
}

// obb — рамка группы в осях phi по углам членов: центр и стороны (как groupOBB в UI).
func obb(pls []RotRect, txs []GxText, phi float64) (cx, cy, w, h float64) {
	u0, v0, u1, v1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	var cs [][4][2]float64
	for _, r := range pls {
		cs = append(cs, r.Box().corners())
	}
	for _, x := range txs {
		cs = append(cs, TextBox{x.X, x.Y, x.W, x.H, x.Rot}.corners())
	}
	for _, c := range cs {
		for _, p := range c {
			u, v := Rotate(-phi, p[0], p[1])
			u0, v0, u1, v1 = math.Min(u0, u), math.Min(v0, v), math.Max(u1, u), math.Max(v1, v)
		}
	}
	cx, cy = Rotate(phi, (u0+u1)/2, (v0+v1)/2)
	return cx, cy, u1 - u0, v1 - v0
}

// centers — центры мест и угол P текстов: по ним видно взаимное положение.
func centers(pls []RotRect, txs []GxText) [][2]float64 {
	var out [][2]float64
	for _, r := range pls {
		out = append(out, [2]float64{r.X + r.W/2, r.Y + r.H/2})
	}
	for _, x := range txs {
		out = append(out, [2]float64{x.X, x.Y})
	}
	return out
}

// rigid — after = before, повёрнутое на deg и сдвинутое (взаимное положение то же).
func rigid(t *testing.T, before, after [][2]float64, deg float64) {
	t.Helper()
	for i := 1; i < len(before); i++ {
		ux, uy := Rotate(deg, before[i][0]-before[0][0], before[i][1]-before[0][1])
		gx, gy := after[i][0]-after[0][0], after[i][1]-after[0][1]
		if math.Abs(ux-gx) > 0.02 || math.Abs(uy-gy) > 0.02 {
			t.Errorf("член %d: смещение (%.3f, %.3f), ждали (%.3f, %.3f)", i, gx, gy, ux, uy)
		}
	}
}

func TestGroupRotRigid(t *testing.T) {
	pl, tx := mixGroup()
	cx, cy, w, h := obb(pl, tx, 0)
	out := GroupRot(GxRotIn{SW: 562, PH: 284, Places: pl, Texts: tx, Deg: 25, CX: cx, CY: cy})
	if out.Deg != 25 || out.DX != 0 || out.DY != 0 {
		t.Fatalf("угол %v, сдвиг (%v, %v)", out.Deg, out.DX, out.DY)
	}
	rigid(t, centers(pl, tx), centers(out.Places, out.Texts), 25)
	for i, r := range out.Places {
		if r.Rot != NormDeg(pl[i].Rot+25) || r.W != pl[i].W || r.H != pl[i].H {
			t.Errorf("место %d: %+v", i, r)
		}
	}
	if x := out.Texts[0]; x.Rot != 10 || x.W != tx[0].W || x.Size != tx[0].Size {
		t.Errorf("текст: %+v", x)
	}
	// Рамка в осях 25° — та же рамка, повёрнутая вокруг своего центра: не «дышит».
	gx, gy, gw, gh := obb(out.Places, out.Texts, 25)
	for _, d := range []float64{gx - cx, gy - cy, gw - w, gh - h} {
		if math.Abs(d) > 0.02 {
			t.Fatalf("рамка (%.3f, %.3f, %.3f×%.3f), была (%.3f, %.3f, %.3f×%.3f)", gx, gy, gw, gh, cx, cy, w, h)
		}
	}
	// Туда и обратно — на место.
	back := GroupRot(GxRotIn{SW: 562, PH: 284, Places: out.Places, Texts: out.Texts, Deg: -25, CX: cx, CY: cy})
	if !slices.EqualFunc(back.Places, pl, func(a, b RotRect) bool { return math.Abs(a.X-b.X) < 0.02 && math.Abs(a.Y-b.Y) < 0.02 && a.Rot == b.Rot }) {
		t.Errorf("обратно: %+v", back.Places)
	}
}

func TestGroupRotShiftInside(t *testing.T) {
	pl := []RotRect{{X: 505, Y: 35, W: 50, H: 50}, {X: 505, Y: 195, W: 50, H: 50}}
	out := GroupRot(GxRotIn{SW: 562, PH: 284, Places: pl, Deg: 30, CX: 530, CY: 140})
	if out.DX >= 0 {
		t.Fatalf("группа не сдвинулась влево: dx %v", out.DX)
	}
	inSpread(t, "у края", out.Places, nil, 562, 284)
	rigid(t, centers(pl, nil), centers(out.Places, nil), 30)
}

// TestGroupRotNoLimit — угол не упирается (решение 2026-10-09): широкая группа
// доворачивается, крайние поджаты до hold, вылезшее обрезает край.
func TestGroupRotNoLimit(t *testing.T) {
	for _, deg := range []float64{45, 90, -45, 7, 180} {
		out := GroupRot(GxRotIn{SW: 562, PH: 284, Places: wideGroup(), Deg: deg, CX: 281, CY: 142})
		if out.Deg != deg {
			t.Errorf("%v°: угол %v", deg, out.Deg)
		}
		for _, r := range out.Places {
			if r.Rot != deg || r.W != 250 || r.H != 250 {
				t.Errorf("%v°: место %+v", deg, r)
			}
		}
		inSpread(t, "широкая", out.Places, nil, 562, 284)
	}
	// На 180° помещается: группа целая, места поменялись местами.
	out := GroupRot(GxRotIn{SW: 562, PH: 284, Places: wideGroup(), Deg: 180, CX: 281, CY: 142})
	rigid(t, centers(wideGroup(), nil), centers(out.Places, nil), 180)
}

// TestGroupRotTextFit — текст после поворота держится внутри так же, как его держит fitTexts.
func TestGroupRotTextFit(t *testing.T) {
	tx := []GxText{{X: 10, Y: 10, W: 200, H: 30, Size: 20}}
	out := GroupRot(GxRotIn{SW: 562, PH: 284, Texts: tx, Deg: 60, CX: 10, CY: 10})
	x := out.Texts[0]
	if fx, fy := (TextBox{x.X, x.Y, x.W, x.H, x.Rot}).FitIn(562, 284); fx != x.X || fy != x.Y {
		t.Fatalf("повторный зажим сдвинул: (%v, %v) → (%v, %v)", x.X, x.Y, fx, fy)
	}
	inSpread(t, "текст", nil, out.Texts, 562, 284)
}
