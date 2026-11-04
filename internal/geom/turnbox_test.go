package geom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func near1(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestTurnBox — смена сторон при смене ориентации, центр стоит всегда.
func TestTurnBox(t *testing.T) {
	b := TextBox{X: 10, Y: 20, W: 200, H: 80}
	cases := []struct {
		name         string
		from         float64
		to           float64
		wantW, wantH float64
	}{
		{"0 → 90: стороны меняются", 0, 90, 80, 200},
		{"0 → 30: нет", 0, 30, 200, 80},
		{"80 → 100: обе вертикальные — нет", 80, 100, 200, 80},
		{"−90 → 0: меняются", -90, 0, 80, 200},
		{"0 → 180: нет", 0, 180, 200, 80},
		{"180 → 270: меняются", 180, 270, 80, 200},
		{"0 → 45: ровно на границе — нет", 0, 45, 200, 80},
		{"45 → 90: меняются", 45, 90, 80, 200},
		{"0 → 0: на месте", 0, 0, 200, 80},
		{"0 → 450 (=90): меняются", 0, 450, 80, 200},
		{"90 → −90: нет", 90, -90, 200, 80},
	}
	for _, c := range cases {
		from := b
		from.Rot = c.from
		cx, cy := from.Center()
		got := TurnBox(from, c.to)
		if got.W != c.wantW || got.H != c.wantH || got.Rot != c.to {
			t.Errorf("%s: %+v", c.name, got)
		}
		gx, gy := got.Center()
		if !near1(gx, cx) || !near1(gy, cy) {
			t.Errorf("%s: центр (%v, %v) → (%v, %v)", c.name, cx, cy, gx, gy)
		}
	}
	// Пустая и нулевая рамка не ломают расчёт.
	if got := TurnBox(TextBox{}, 90); got.W != 0 || got.H != 0 || got.Rot != 90 {
		t.Errorf("нулевая рамка: %+v", got)
	}
	// Туда и обратно — исходная рамка.
	back := TurnBox(TurnBox(b, 90), 0)
	if !near1(back.X, b.X) || !near1(back.Y, b.Y) || back.W != b.W || back.H != b.H || back.Rot != 0 {
		t.Errorf("0 → 90 → 0: %+v, ждали %+v", back, b)
	}
}

// turnCase — вектор testdata/parity/bdturn.json (backdrop.ts turnBackdropBox).
type turnCase struct {
	Box  rpBox   `json:"box"`
	Rot  float64 `json:"rot"`
	Want rpBox   `json:"want"`
}

func turnCases() []turnCase {
	b := func(x, y, w, h, rot float64) rpBox { return rpBox{x, y, w, h, rot} }
	return []turnCase{
		{Box: b(10, 20, 200, 80, 0), Rot: 90},
		{Box: b(10, 20, 200, 80, 0), Rot: 30},
		{Box: b(10, 20, 200, 80, 80), Rot: 100},
		{Box: b(10, 20, 200, 80, -90), Rot: 0},
		{Box: b(0, 0, 600, 300, 0), Rot: 270},
		{Box: b(-30, 15.5, 123.45, 67.89, 12.3), Rot: 102.3},
		{Box: b(300, 0, 6, 284, 0), Rot: 90},
		{Box: b(300, 0, 284, 6, 90), Rot: 180},
		{Box: b(5, 5, 100, 100, 45), Rot: 90},
		{Box: b(5, 5, 0, 0, 0), Rot: 90},
	}
}

func TestTurnBoxParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "bdturn.json")
	run := func(c turnCase) rpBox {
		r := TurnBox(TextBox{X: c.Box.X, Y: c.Box.Y, W: c.Box.W, H: c.Box.H, Rot: c.Box.Rot}, c.Rot)
		return rpBox(r)
	}
	if *update {
		cs := turnCases()
		for i := range cs {
			cs[i].Want = run(cs[i])
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
	var cs []turnCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(turnCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		got := run(c)
		w := c.Want
		if !near1(got.X, w.X) || !near1(got.Y, w.Y) || !near1(got.W, w.W) || !near1(got.H, w.H) || got.Rot != w.Rot {
			t.Errorf("вектор %d: %+v, ждали %+v", i, got, w)
		}
	}
}
