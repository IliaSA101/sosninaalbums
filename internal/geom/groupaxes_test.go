package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// gaCase — вектор для Go и frontend (geom/groupaxes.ts): вход и итог GroupAxes.
type gaCase struct {
	Name string    `json:"name"`
	In   GxAxesIn  `json:"in"`
	Out  GxAxesOut `json:"out"`
}

// tilted — места в осях рамки под phi: локальные (u, v, w, h) от точки o.
func tilted(phi, ox, oy float64, loc [][4]float64) []RotRect {
	var out []RotRect
	for _, l := range loc {
		cx, cy := Rotate(phi, l[0]+l[2]/2, l[1]+l[3]/2)
		out = append(out, RotRect{X: ox + cx - l[2]/2, Y: oy + cy - l[3]/2, W: l[2], H: l[3], Rot: phi})
	}
	return out
}

// frame20 — три места под 20°, рамка 260×120 в своих осях, a — середина левой стороны.
func frame20() ([]RotRect, float64, float64) {
	pl := tilted(20, 100, 20, [][4]float64{{0, 0, 100, 60}, {110, 0, 80, 120}, {200, 10, 60, 60}})
	ax, ay := Rotate(20, 0, 60)
	return pl, 100 + ax, 20 + ay
}

// mixAxes — места 0°, 30°, 90° (по 100×60) в рамке 0°, a = (10, 10).
func mixAxes() []RotRect {
	return []RotRect{{X: 10, Y: 10, W: 100, H: 60}, {X: 130, Y: 100, W: 100, H: 60, Rot: 30}, {X: 250, Y: 180, W: 100, H: 60, Rot: 90}}
}

func gaCases() []gaCase {
	in := func(kx, ky, phi, ax, ay float64, pl []RotRect, tx []GxText) GxAxesIn {
		pl, tx = append([]RotRect{}, pl...), append([]GxText{}, tx...) // [] вместо null в JSON
		return GxAxesIn{SW: 562, PH: 284, Places: pl, Texts: tx, KX: kx, KY: ky, Phi: phi, AX: ax, AY: ay}
	}
	f20, ax, ay := frame20()
	center := []GxText{{X: 60, Y: 200, W: 120, H: 20, Size: 14, Al: 0.5}}
	return []gaCase{
		{Name: "рамка 20°, kx 1.5 от левой стороны", In: in(1.5, 1, 20, ax, ay, f20, nil)},
		{Name: "несоосные в рамке 0°, kx 1.5", In: in(1.5, 1, 0, 10, 10, mixAxes(), nil)},
		{Name: "несоосные, ky 0.7", In: in(1, 0.7, 0, 10, 10, mixAxes(), nil)},
		{Name: "текст по центру, kx 1.5", In: in(1.5, 1, 0, 10, 10, mixAxes()[:1], center)},
		{Name: "текст по центру, ky 0.5", In: in(1, 0.5, 0, 10, 10, mixAxes()[:1], center)},
		{Name: "текст под 90°, kx 0.5", In: in(0.5, 1, 0, 0, 0, nil, []GxText{{X: 300, Y: 40, W: 150, H: 30, Rot: 90, Size: 20, Al: 1}})},
		{Name: "kx 5 — бисекция до разворота", In: in(5, 1, 20, ax, ay, f20, nil)},
		{Name: "kx 0.01 — нижние пределы", In: in(0.01, 1, 0, 10, 10, mixAxes(), center)},
		{Name: "у правого края — сдвиг внутрь", In: in(1.4, 1, 0, 350, 100, []RotRect{{X: 350, Y: 100, W: 200, H: 80}}, nil)},
		{Name: "пусто", In: in(2, 1, 0, 0, 0, nil, nil)},
		{Name: "текст под 45°, kx 2 — кегль стоит", In: in(2, 1, 0, 20, 20, nil, []GxText{tilt45})},
		{Name: "кегль 150, kx 2, ky 1.5 — упор 200", In: in(2, 1.5, 0, 20, 20, nil, []GxText{big150})},
	}
}

// tilt45 — текст под 45° в рамке 0°: при kx 2 образ его оси ≈ 1.58, кегль стоит.
var tilt45 = GxText{X: 100, Y: 60, W: 100, H: 30, Rot: 45, Size: 20, Al: 0.5}

// big150 — текст 150 пт: кегль упрётся в 200 при меньшем множителе 4/3.
var big150 = GxText{X: 30, Y: 40, W: 100, H: 50, Size: 150}

func TestGroupAxesParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "groupaxes.json")
	if *update {
		cs := gaCases()
		for i := range cs {
			cs[i].Out = GroupAxes(cs[i].In)
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
	var cs []gaCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(gaCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for _, c := range cs {
		got := GroupAxes(c.In)
		if math.Abs(got.KX-c.Out.KX) > 1e-9 || math.Abs(got.KY-c.Out.KY) > 1e-9 || math.Abs(got.DX-c.Out.DX) > 1e-9 ||
			math.Abs(got.DY-c.Out.DY) > 1e-9 || !slices.EqualFunc(got.Places, c.Out.Places, gxPlaceNear) ||
			!slices.EqualFunc(got.Texts, c.Out.Texts, func(a, b GxText) bool { return gxNear(a, b) && a.Al == b.Al }) {
			t.Errorf("%s: %+v, ждали %+v", c.Name, got, c.Out)
		}
		inSpread(t, c.Name, got.Places, nil, c.In.SW, c.In.PH)
	}
}

// local — центр места в осях рамки phi от точки o.
func local(r RotRect, phi, ox, oy float64) (float64, float64) {
	return Rotate(-phi, r.X+r.W/2-ox, r.Y+r.H/2-oy)
}

func TestGroupAxesAligned(t *testing.T) {
	pl, ax, ay := frame20()
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Places: pl, KX: 1.5, KY: 1, Phi: 20, AX: ax, AY: ay})
	if out.KX != 1.5 || out.KY != 1 || out.DX != 0 || out.DY != 0 {
		t.Fatalf("k (%v, %v), d (%v, %v)", out.KX, out.KY, out.DX, out.DY)
	}
	for i, r := range out.Places {
		if r.Rot != 20 || r.W != Round2(pl[i].W*1.5) || r.H != pl[i].H {
			t.Errorf("место %d: %+v", i, r)
		}
		// В осях рамки центр уехал от a в 1.5 раза по u, по v стоит: перекоса нет.
		u0, v0 := local(pl[i], 20, ax, ay)
		u1, v1 := local(r, 20, ax, ay)
		if math.Abs(u1-1.5*u0) > 0.02 || math.Abs(v1-v0) > 0.02 {
			t.Errorf("место %d: (%.3f, %.3f), было (%.3f, %.3f)", i, u1, v1, u0, v0)
		}
	}
	// Рамка в осях 20°: ширина ×1.5, высота и левая сторона стоят.
	_, _, w0, h0 := obb(pl, nil, 20)
	_, _, w1, h1 := obb(out.Places, nil, 20)
	if math.Abs(w1-1.5*w0) > 0.03 || math.Abs(h1-h0) > 0.02 {
		t.Fatalf("рамка %.3f×%.3f, была %.3f×%.3f", w1, h1, w0, h0)
	}
}

func TestGroupAxesMisaligned(t *testing.T) {
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Places: mixAxes(), KX: 1.5, KY: 1, AX: 10, AY: 10})
	want := []RotRect{{W: 150, H: 60}, {W: 139.19, H: 68.74, Rot: 30}, {W: 100, H: 90, Rot: 90}}
	for i, r := range out.Places {
		if r.W != want[i].W || r.H != want[i].H || r.Rot != want[i].Rot {
			t.Errorf("место %d: %+v, ждали %v×%v под %v", i, r, want[i].W, want[i].H, want[i].Rot)
		}
		cx0, cy0 := mixAxes()[i].X+mixAxes()[i].W/2, mixAxes()[i].Y+mixAxes()[i].H/2
		if cx, cy := r.X+r.W/2, r.Y+r.H/2; math.Abs(cx-(10+(cx0-10)*1.5)) > 0.01 || math.Abs(cy-cy0) > 0.01 {
			t.Errorf("место %d: центр (%.2f, %.2f)", i, cx, cy)
		}
	}
}

func TestGroupAxesText(t *testing.T) {
	tx := GxText{X: 60, Y: 200, W: 120, H: 20, Size: 14, Al: 0.5}
	anchor := func(x GxText) (float64, float64) {
		dx, dy := Rotate(x.Rot, x.Al*x.W, 0)
		return x.X + dx, x.Y + dy
	}
	ax0, ay0 := anchor(tx)
	// Вправо ×1.5: кегль прежний, точка выравнивания уехала по x от a.
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{tx}, KX: 1.5, KY: 1, AX: 10, AY: 10})
	x := out.Texts[0]
	if ax, ay := anchor(x); x.Size != 14 || x.W != 180 || math.Abs(ax-(10+(ax0-10)*1.5)) > 0.01 || math.Abs(ay-ay0) > 0.01 {
		t.Fatalf("kx 1.5: %+v, точка (%.2f, %.2f)", x, ax, ay)
	}
	// Вниз ×0.5: кегль ×0.5, ширина стоит.
	out = GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{tx}, KX: 1, KY: 0.5, AX: 10, AY: 10})
	if x := out.Texts[0]; x.Size != 7 || x.W != 120 || math.Abs(x.H-10) > 1e-9 {
		t.Fatalf("ky 0.5: %+v", x)
	}
	// Повёрнутый на 90°: ось x рамки — его ось v. kx 1.5 — кегль стоит (sx 1), kx 0.5 — ×0.5.
	rt := GxText{X: 300, Y: 40, W: 150, H: 30, Rot: 90, Size: 20, Al: 1}
	for _, c := range []struct{ kx, size, w float64 }{{1.5, 20, 150}, {0.5, 10, 150}} {
		out = GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{rt}, KX: c.kx, KY: 1})
		if x := out.Texts[0]; x.Size != c.size || x.W != c.w || x.Rot != 90 {
			t.Errorf("90°, kx %v: %+v", c.kx, x)
		}
	}
}

func TestGroupAxesClamp(t *testing.T) {
	pl, ax, ay := frame20()
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Places: pl, KX: 5, KY: 1, Phi: 20, AX: ax, AY: ay})
	if out.KX >= 5 || out.KX <= 1.5 || out.KY != 1 {
		t.Fatalf("kx 5 → (%v, %v)", out.KX, out.KY)
	}
	x0, _, x1, _, _ := groupBound(out.Places, nil)
	if x1-x0 < 562-0.05 || x1-x0 > 562+0.01 {
		t.Fatalf("ширина группы %.3f, ждали во весь разворот", x1-x0)
	}
	inSpread(t, "kx 5", out.Places, nil, 562, 284)
	tx := []GxText{{X: 60, Y: 200, W: 120, H: 20, Size: 14, Al: 0.5}}
	out = GroupAxes(GxAxesIn{SW: 562, PH: 284, Places: mixAxes(), Texts: tx, KX: 0.01, KY: 1, AX: 10, AY: 10})
	for _, r := range out.Places {
		if r.W < MinPlace || r.H < MinPlace {
			t.Errorf("место меньше предела: %+v", r)
		}
	}
	if out.Texts[0].Size < text.MinSize {
		t.Errorf("кегль %v", out.Texts[0].Size)
	}
}

// TestGroupAxesTextTilted — решение 3: кегль по min(kx, ky), а не по осям текста.
// Под 45° при kx 2 прежняя формула давала 20 → 31.6.
func TestGroupAxesTextTilted(t *testing.T) {
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{tilt45}, KX: 2, KY: 1, AX: 20, AY: 20})
	sx := math.Hypot(2*math.Cos(math.Pi/4), math.Sin(math.Pi/4))
	if x := out.Texts[0]; x.Size != 20 || x.Rot != 45 || x.W != Round2(100*sx) || math.Abs(x.H-30) > 1e-9 {
		t.Fatalf("45°, kx 2: %+v", x)
	}
	// Сжатие по одной оси кегль уменьшает и под 45°.
	out = GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{tilt45}, KX: 1, KY: 0.5, AX: 20, AY: 20})
	if x := out.Texts[0]; x.Size != 10 {
		t.Fatalf("45°, ky 0.5: %+v", x)
	}
}

// TestGroupAxesMaxSize — меньший множитель зажат по MaxSize/Size до бисекции:
// кегль ровно Size·min(kx, ky), больший множитель тянет ширину как задан.
func TestGroupAxesMaxSize(t *testing.T) {
	out := GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{big150}, KX: 2, KY: 1.5, AX: 20, AY: 20})
	x := out.Texts[0]
	if out.KX != 2 || math.Abs(out.KY-text.MaxSize/150) > 1e-12 || x.Size != text.MaxSize || x.W != 200 ||
		math.Abs(x.H-50*text.MaxSize/150) > 1e-9 {
		t.Fatalf("k (%v, %v), %+v", out.KX, out.KY, x)
	}
	// Равные множители — оба до 4/3, как k у GroupXf.
	out = GroupAxes(GxAxesIn{SW: 562, PH: 284, Texts: []GxText{big150}, KX: 1.5, KY: 1.5, AX: 20, AY: 20})
	if g := GroupXf(GxIn{SW: 562, PH: 284, Texts: []GxText{big150}, K: 1.5, AX: 20, AY: 20}); out.KX != g.K || out.KY != g.K {
		t.Fatalf("k (%v, %v), у GroupXf %v", out.KX, out.KY, g.K)
	}
}
