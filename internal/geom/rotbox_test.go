package geom

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// rpBox — место в векторах rotplace.json: неповёрнутый прямоугольник и угол.
type rpBox struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	W   float64 `json:"w"`
	H   float64 `json:"h"`
	Rot float64 `json:"rot"`
}

func (b rpBox) place() RotRect { return RotRect(b) }

func boxOfPlace(pl RotRect) rpBox { return rpBox(pl) }

// rpCase — вектор для Go и frontend (geom/rotbox.ts): без prev — ClampPlaceIn (hold,
// Load, группа, поворот), с prev — перенос SetPlaces (ClampMoveIn: вылет не растёт).
type rpCase struct {
	Name string  `json:"name"`
	SW   float64 `json:"sw"`
	PH   float64 `json:"ph"`
	Prev *rpBox  `json:"prev,omitempty"`
	In   rpBox   `json:"in"`
	Out  rpBox   `json:"out"`
}

func (c rpCase) run() rpBox {
	pl := c.In.place()
	if c.Prev != nil {
		ClampMoveIn(&pl, c.Prev.place(), c.SW, c.PH)
	} else {
		ClampPlaceIn(&pl, c.SW, c.PH)
	}
	return boxOfPlace(pl)
}

func rpCases() []rpCase {
	b := func(x, y, w, h, rot float64) rpBox { return rpBox{x, y, w, h, rot} }
	pb := func(x, y, w, h, rot float64) *rpBox { v := b(x, y, w, h, rot); return &v }
	c := func(name string, prev *rpBox, in rpBox) rpCase {
		return rpCase{Name: name, SW: 562, PH: 284, Prev: prev, In: in}
	}
	return []rpCase{
		c("ровное — прежний зажим", nil, b(-10, 300, 100, 50, 0)),
		c("ровное больше разворота", nil, b(5, 5, 900, 400, 0)),
		c("ровное — шум округления", nil, b(10.004, 20.006, 100.001, 50.009, 0)),
		c("почти ноль — ровное", nil, b(-10, 10, 100, 50, 0.04)),
		c("90° у левого края: центр в r", nil, b(-60, 100, 100, 20, 90)),
		c("30° внутри не двигается", nil, b(0, 0, 100, 60, 30)),
		c("145° за правым нижним краем", nil, b(540, 270, 80, 40, 145)),
		c("угол нормируется: 400 → 40", nil, b(100, 100, 80, 40, 400)),
		c("угол нормируется: −180 → 180", nil, b(-30, 100, 80, 40, -180)),
		c("угол −179.97 → 180", nil, b(100, 100, 80, 40, -179.97)),
		c("1°: полоса у края не уходит дальше", pb(300, 100, 200, 10, 1), b(400, 100, 200, 10, 1)),
		c("1°: уже вылетающее — не дальше", pb(-50, 100, 200, 10, 1), b(-80, 100, 200, 10, 1)),
		c("1°: из вылета внутрь — свободно", pb(-50, 100, 200, 10, 1), b(-20, 100, 200, 10, 1)),
		c("1°: вниз у нижнего края", pb(100, 270, 200, 10, 1), b(100, 300, 200, 10, 1)),
		c("30°: перенос внутри", pb(100, 100, 100, 60, 30), b(150.123, 80.456, 100, 60, 30)),
		c("45°: выше листа — по высоте только hold", pb(200, 100, 100, 20, 45), b(100, -50, 500, 280, 45)),
		c("ровное: перенос — прежний зажим", pb(10, 10, 50, 50, 0), b(600, -5, 50, 50, 0)),
	}
}

// Векторы rotplace.json: Go и geom/rotbox.ts зажимают место одинаково (-update перезаписывает).
func TestRotPlaceParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "rotplace.json")
	if *update {
		cs := rpCases()
		for i := range cs {
			cs[i].Out = cs[i].run()
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
	var cs []rpCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(rpCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for _, c := range cs {
		got := c.run()
		if math.Abs(got.X-c.Out.X) > 1e-9 || math.Abs(got.Y-c.Out.Y) > 1e-9 || got.W != c.Out.W || got.H != c.Out.H || got.Rot != c.Out.Rot {
			t.Errorf("%s: %+v, ждали %+v", c.Name, got, c.Out)
		}
		// Итог любого зажима — неподвижная точка ClampPlaceIn (инвариант Load).
		pl := got.place()
		if ClampPlaceIn(&pl, c.SW, c.PH); boxOfPlace(pl) != got || !holdInside(&pl, c.SW, c.PH) {
			t.Errorf("%s: не неподвижна для ClampPlaceIn: %+v → %+v", c.Name, got, boxOfPlace(pl))
		}
	}
}

// hold: повёрнутое место держится вписанным кругом — 90° встаёт к краю вплотную,
// вылет — не больше половины длины; ClampPlaceIn идемпотентна.
func TestClampPlaceRotHold(t *testing.T) {
	pl := RotRect{X: -60, Y: 100, W: 100, H: 20, Rot: 90}
	ClampPlaceIn(&pl, 562, 284)
	if pl.X != -40 || pl.Y != 100 || math.Abs(pl.AABB().X) > 1e-9 {
		t.Fatalf("90° у края: %+v, aabb %+v", pl, pl.AABB())
	}
	pl = RotRect{X: 540, Y: 270, W: 80, H: 40, Rot: 145}
	ClampPlaceIn(&pl, 562, 284)
	if pl.X != 502 || pl.Y != 244 {
		t.Fatalf("145°: %+v", pl)
	}
	r := rand.New(rand.NewPCG(10, 2026))
	for range 2000 {
		pl := RotRect{X: r.Float64()*800 - 100, Y: r.Float64()*400 - 100, W: r.Float64() * 600, H: r.Float64() * 300,
			Rot: r.Float64()*800 - 400}
		ClampPlaceIn(&pl, 562, 284)
		again := pl
		ClampPlaceIn(&again, 562, 284)
		if again != pl || !holdInside(&pl, 562, 284) || NormDeg(pl.Rot) != pl.Rot {
			t.Fatalf("не идемпотентна или вне разворота: %+v → %+v", pl, again)
		}
	}
}

// holdInside — hold места внутри разворота sw×ph (у ровного — само место).
func holdInside(pl *RotRect, sw, ph float64) bool {
	h := pl.Hold()
	return h.X >= -1e-6 && h.Y >= -1e-6 && h.X+h.W <= sw+1e-6 && h.Y+h.H <= ph+1e-6
}
