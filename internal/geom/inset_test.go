package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// insetCase — общий вектор окна рамочки для Go и фронта (живое изменение размера места, 079 §9).
type insetCase struct {
	Cell    layout.Rect `json:"cell"`
	Outline Outline     `json:"outline,omitzero"`
	Ins     [4]float64  `json:"ins"`     // как пришли (до зажима)
	Clamped [4]float64  `json:"clamped"` // ClampInset
	Win     layout.Rect `json:"win"`
	WinOut  Outline     `json:"winOutline,omitzero"`
}

func insetCases() []insetCase {
	return []insetCase{
		{Cell: layout.Rect{W: 100, H: 150}, Ins: [4]float64{2, 2, 2, 2}},
		{Cell: layout.Rect{X: 10, Y: 20, W: 100, H: 150}, Ins: [4]float64{5, 5, 5, 17.5}},
		{Cell: layout.Rect{X: 5, Y: 5, W: 80, H: 100}, Outline: Outline{Oval: true}, Ins: [4]float64{2, 2, 2, 2}},
		{Cell: layout.Rect{W: 100, H: 60}, Outline: Outline{Radius: 10}, Ins: [4]float64{3, 3, 3, 3}},
		{Cell: layout.Rect{W: 100, H: 60}, Outline: Outline{Radius: 2}, Ins: [4]float64{3, 3, 3, 10.5}},
		{Cell: layout.Rect{W: 40, H: 40}, Ins: [4]float64{20, 20, 20, 20}},
		{Cell: layout.Rect{W: 30, H: 40}, Ins: [4]float64{4.2, 5.6, 4.2, 5.6}},
		{Cell: layout.Rect{X: 1, Y: 1, W: 50, H: 50}, Ins: [4]float64{}},
		{Cell: layout.Rect{X: 1, Y: 1, W: 50, H: 50}, Outline: Outline{Radius: 5}, Ins: [4]float64{-1, 0, 30, 0}},
	}
}

// TestInsetBox — числа design §12: линия 2 мм у 100×150 — окно (2, 2, 96, 146); полароид 5 мм —
// низ 17,5; овал 80×100 с линией 2 — овал 76×96; скругление 10 + линия 3 — 7; линия 20 у 40×40 —
// пара 28 мм (окно 12 = 30 %); без отступов — само место и форма.
func TestInsetBox(t *testing.T) {
	cs := insetCases()
	want := []struct {
		win layout.Rect
		wo  Outline
	}{
		{layout.Rect{X: 2, Y: 2, W: 96, H: 146}, Outline{}},
		{layout.Rect{X: 15, Y: 25, W: 90, H: 127.5}, Outline{}},
		{layout.Rect{X: 7, Y: 7, W: 76, H: 96}, Outline{Oval: true}},
		{layout.Rect{X: 3, Y: 3, W: 94, H: 54}, Outline{Radius: 7}},
		{layout.Rect{X: 3, Y: 3, W: 94, H: 46.5}, Outline{}},
		{layout.Rect{X: 14, Y: 14, W: 12, H: 12}, Outline{}},
		{layout.Rect{X: 4.2, Y: 5.6, W: 21.6, H: 28.8}, Outline{}},
		{layout.Rect{X: 1, Y: 1, W: 50, H: 50}, Outline{}},
		{layout.Rect{X: 1, Y: 1, W: 20, H: 50}, Outline{Radius: 5}}, // минус — 0, 30 ≤ 35: без сжатия
	}
	for i, c := range cs {
		win, wo := InsetBox(c.Cell, c.Outline, c.Ins)
		if !nearTol(win, want[i].win, 1e-9) || math.Abs(wo.Radius-want[i].wo.Radius) > 1e-9 || wo.Oval != want[i].wo.Oval {
			t.Errorf("случай %d: окно %+v %+v, ждали %+v %+v", i, win, wo, want[i].win, want[i].wo)
		}
		if win.W < (1-MaxInsetPair)*c.Cell.W-1e-9 || win.H < (1-MaxInsetPair)*c.Cell.H-1e-9 {
			t.Errorf("случай %d: окно %+v меньше 30 %% места", i, win)
		}
		cl := ClampInset(c.Ins, c.Cell.W, c.Cell.H)
		if ClampInset(cl, c.Cell.W, c.Cell.H) != cl {
			t.Errorf("случай %d: ClampInset не идемпотентна", i)
		}
	}
	// Без отступов форма места — дословно (тот же путь, что без рамочки).
	o := Outline{Radius: 12.345}
	if win, wo := InsetBox(layout.Rect{W: 10, H: 10}, o, [4]float64{}); wo != o || win != (layout.Rect{W: 10, H: 10}) {
		t.Fatalf("без отступов: %+v %+v", win, wo)
	}
	// NaN, Inf, нулевое место — окно без NaN.
	for _, ins := range [][4]float64{{math.NaN(), math.Inf(1), math.Inf(-1), -5}, {1e308, 1e308, 1e308, 1e308}} {
		win, _ := InsetBox(layout.Rect{W: 40, H: 0}, Outline{}, ins)
		for _, v := range []float64{win.X, win.Y, win.W, win.H} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("отступы %v: окно %+v", ins, win)
			}
		}
	}
}

func nearTol(a, b layout.Rect, eps float64) bool {
	return math.Abs(a.X-b.X) < eps && math.Abs(a.Y-b.Y) < eps && math.Abs(a.W-b.W) < eps && math.Abs(a.H-b.H) < eps
}

// TestInsetParity — векторы окна для фронта (testdata/parity/inset.json, -update перезаписывает).
func TestInsetParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "inset.json")
	if *update {
		cs := insetCases()
		for i := range cs {
			cs[i].Clamped = ClampInset(cs[i].Ins, cs[i].Cell.W, cs[i].Cell.H)
			cs[i].Win, cs[i].WinOut = InsetBox(cs[i].Cell, cs[i].Outline, cs[i].Ins)
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
	var cs []insetCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(insetCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		win, wo := InsetBox(c.Cell, c.Outline, c.Ins)
		if !near(win, c.Win) || math.Abs(wo.Radius-c.WinOut.Radius) > 1e-9 || wo.Oval != c.WinOut.Oval ||
			ClampInset(c.Ins, c.Cell.W, c.Cell.H) != c.Clamped {
			t.Errorf("вектор %d: %+v %+v, ждали %+v %+v", i, win, wo, c.Win, c.WinOut)
		}
	}
}
