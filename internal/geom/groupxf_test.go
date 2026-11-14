package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// gxCase — вектор для Go и frontend (geom/groupxf.ts, groupxf.test.ts): вход и итог GroupXf.
type gxCase struct {
	Name string `json:"name"`
	In   GxIn   `json:"in"`
	Out  GxOut  `json:"out"`
}

func gxCases() []gxCase {
	r := func(x, y, w, h float64) RotRect { return RotRect{X: x, Y: y, W: w, H: h} }
	in := func(k, ax, ay, dx, dy float64, pl []RotRect, tx []GxText) GxIn {
		pl, tx = append([]RotRect{}, pl...), append([]GxText{}, tx...) // [] вместо null в JSON
		return GxIn{SW: 562, PH: 284, Places: pl, Texts: tx, K: k, AX: ax, AY: ay, DX: dx, DY: dy}
	}
	two := []RotRect{r(10, 10, 100, 100), r(120, 10, 100, 100)}
	txt := []GxText{{X: 300, Y: 60, W: 120, H: 20, Size: 12}}
	return []gxCase{
		{Name: "перенос мест", In: in(1, 0, 0, 12.345, -3, two, nil)},
		{Name: "масштаб от угла", In: in(0.5, 10, 10, 0, 0, two, nil)},
		{Name: "масштаб с текстом", In: in(0.7, 10, 10, 4, 4, two, txt)},
		{Name: "текст с поворотом", In: in(1.3, 20, 20, 0, 0, nil, []GxText{{X: 200, Y: 100, W: 80, H: 20, Rot: 30, Size: 14}})},
		{Name: "k выше предела разворота", In: in(50, 10, 10, 0, 0, two, nil)},
		{Name: "k ниже предела места", In: in(0.001, 0, 0, 0, 0, two, nil)},
		{Name: "k выше предела по высоте", In: in(50, 10, 10, 0, 0, []RotRect{r(10, 10, 40, 120)}, nil)},
		{Name: "нижний предел кегля", In: in(0.1, 300, 60, 0, 0, nil, txt)},
		{Name: "верхний предел кегля", In: in(40, 300, 60, 0, 0, nil, []GxText{{X: 300, Y: 60, W: 20, H: 10, Size: 100}})},
		{Name: "d прижимает к краям", In: in(1, 0, 0, 9999, -9999, two, txt)},
		{Name: "d прижимает влево", In: in(0.6, 100, 100, -9999, 9999, two, nil)},
		{Name: "округление кегля", In: in(1.0371, 0, 0, 0, 0, nil, []GxText{{X: 50, Y: 50, W: 60, H: 10, Size: 13.3}})},
		// Повёрнутое место входит в границу группы своим hold (квадрат min(W, H) вокруг центра).
		{Name: "повёрнутое место влево до hold", In: in(1, 0, 0, -9999, 0, []RotRect{{X: 200, Y: 100, W: 100, H: 20, Rot: 90}}, nil)},
		{Name: "наклон 45 вниз до hold", In: in(1, 0, 0, 0, 9999, []RotRect{{X: 100, Y: 200, W: 80, H: 40, Rot: 45}}, nil)},
		{Name: "повёрнутое и ровное, масштаб", In: in(1.5, 10, 10, 0, 0, []RotRect{r(10, 10, 100, 60), {X: 150, Y: 40, W: 120, H: 50, Rot: -30}}, txt)},
		// Поле «Ш» карточки группы (спека 014): якорь — левый верхний угол aabb, k = v/w; у края d сдвигает внутрь.
		{Name: "поле Ш: якорь в углу описанной рамки, у правого края — сдвиг", In: in(1.3, 51.7, 19.02, 0, 0,
			[]RotRect{{X: 60, Y: 40, W: 100, H: 60, Rot: 30}, r(400, 30, 80, 120)}, nil)},
	}
}

func TestGroupXfParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "groupxf.json")
	if *update {
		cs := gxCases()
		for i := range cs {
			cs[i].Out = GroupXf(cs[i].In)
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
	var cs []gxCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(gxCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for _, c := range cs {
		got := GroupXf(c.In)
		if math.Abs(got.K-c.Out.K) > 1e-9 || math.Abs(got.DX-c.Out.DX) > 1e-9 || math.Abs(got.DY-c.Out.DY) > 1e-9 ||
			!slices.EqualFunc(got.Places, c.Out.Places, gxPlaceNear) || !slices.EqualFunc(got.Texts, c.Out.Texts, gxNear) {
			t.Errorf("%s: %+v, ждали %+v", c.Name, got, c.Out)
		}
		// Итог внутри разворота (hold мест: у ровного — само место).
		for _, g := range got.Places {
			pl := g.Hold()
			if pl.X < -0.01 || pl.Y < -0.01 || pl.X+pl.W > c.In.SW+0.01 || pl.Y+pl.H > c.In.PH+0.01 {
				t.Errorf("%s: место вне разворота %+v", c.Name, pl)
			}
		}
	}
}

func gxPlaceNear(a, b RotRect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 &&
		math.Abs(a.H-b.H) < 1e-9 && a.Rot == b.Rot
}

func gxNear(a, b GxText) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 &&
		math.Abs(a.H-b.H) < 1e-9 && a.Rot == b.Rot && a.Size == b.Size
}
