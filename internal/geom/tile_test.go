package geom

import (
	"fmt"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// tileSet — пропорции k фото: L — все 3:2, P — все 2:3, mix — каждое третье 2:3.
func tileSet(kind string, k int) []float64 {
	out := make([]float64, k)
	for i := range out {
		switch {
		case kind == "P", kind == "mix" && i%3 == 2:
			out[i] = 2.0 / 3
		default:
			out[i] = 1.5
		}
	}
	return out
}

// tilePage — область страницы side×side минус поле 12 и предел «Места» (1/3 стороны).
func tilePage(side float64) (layout.Rect, float64) {
	return layout.Rect{X: 12, Y: 12, W: side - 24, H: side - 24}, side / 3
}

func overlap(a, b layout.Rect) bool {
	const eps = 1e-9
	return a.X < b.X+b.W-eps && b.X < a.X+a.W-eps && a.Y < b.Y+b.H-eps && b.Y < a.Y+a.H-eps
}

func TestTileFitsArea(t *testing.T) {
	for _, side := range []float64{50, 80, 200, 300, 1000} {
		area, limit := tilePage(side)
		for _, kind := range []string{"L", "P", "mix"} {
			for k := 1; k <= 64; k++ {
				asp := tileSet(kind, k)
				checkTile(t, fmt.Sprintf("%v мм, %s, k=%d", side, kind, k), area, asp, Tile(area, asp, 3, limit))
			}
		}
	}
}

// checkTile — места rs внутри area, не пустые, в пропорциях фото asp и не налезают друг на друга.
func checkTile(t *testing.T, name string, area layout.Rect, asp []float64, rs []layout.Rect) {
	t.Helper()
	if len(rs) != len(asp) {
		t.Fatalf("%s: мест %d", name, len(rs))
	}
	for i, r := range rs {
		if r.X < area.X-1e-9 || r.Y < area.Y-1e-9 || r.X+r.W > area.X+area.W+1e-9 || r.Y+r.H > area.Y+area.H+1e-9 {
			t.Fatalf("%s: место %d %+v вне области %+v", name, i, r, area)
		}
		if !(r.W > 0) || !(r.H > 0) {
			t.Fatalf("%s: место %d пустое %+v", name, i, r)
		}
		// ±0,01 и ошибка округления краёв до 0,01 мм (у мелких мест заметна).
		if math.Abs(r.W/r.H-asp[i]) > 0.01+0.01*(1+asp[i])/r.H {
			t.Fatalf("%s: место %d пропорции %.4f, фото %.4f", name, i, r.W/r.H, asp[i])
		}
		for j := range i {
			if overlap(r, rs[j]) {
				t.Fatalf("%s: места %d и %d налезают: %+v %+v", name, i, j, r, rs[j])
			}
		}
	}
}

func TestTileCapSide(t *testing.T) {
	area, limit := tilePage(300)
	for _, k := range []int{1, 2, 4} {
		for _, r := range Tile(area, tileSet("mix", k), 3, limit) {
			if max(r.W, r.H) > limit+0.01 {
				t.Fatalf("k=%d: место %+v крупнее «Места» %.2f", k, r, limit)
			}
		}
	}
	// Замер design §3: одно фото 3:2 на 300×300 — 100×67, четыре — 2×2 того же размера.
	one := Tile(area, tileSet("L", 1), 3, limit)[0]
	if math.Abs(one.W-100) > 0.01 || math.Abs(one.H-66.67) > 0.02 {
		t.Fatalf("одно фото: %+v, ждали 100×66,67", one)
	}
	four := Tile(area, tileSet("L", 4), 3, limit)
	if four[0].Y != four[1].Y || four[2].Y == four[0].Y || four[0].W != one.W {
		t.Fatalf("четыре фото не сеткой 2×2 размера «Места»: %+v", four)
	}
	// 64 фото 3:2 на 300×300 — около 37×25 мм (design §3).
	many := Tile(area, tileSet("L", 64), 3, limit)
	if w := many[0].W; w < 35 || w > 39 {
		t.Fatalf("64 фото: место %+v, ждали около 37×25", many[0])
	}
	// Без предела — блок занимает область по одной стороне.
	free := Tile(area, tileSet("L", 1), 3, math.Inf(1))
	if math.Abs(free[0].W-area.W) > 0.01 {
		t.Fatalf("без предела ширина %.2f, ждали %.2f", free[0].W, area.W)
	}
}

func TestTileOrder(t *testing.T) {
	area, limit := tilePage(300)
	rs := Tile(area, tileSet("L", 7), 3, limit)
	// По рядам слева направо: в ряду X растёт, следующий ряд ниже.
	for i := 1; i < len(rs); i++ {
		a, b := rs[i-1], rs[i]
		if !(b.Y == a.Y && b.X > a.X || b.Y > a.Y) {
			t.Fatalf("порядок мест нарушен у %d: %+v → %+v", i, a, b)
		}
	}
	// Неполный последний ряд — по центру области, как и блок целиком.
	last := rs[len(rs)-1]
	var row []layout.Rect
	for _, r := range rs {
		if r.Y == last.Y {
			row = append(row, r)
		}
	}
	mid := area.X + area.W/2
	if c := (row[0].X + row[len(row)-1].X + row[len(row)-1].W) / 2; math.Abs(c-mid) > 0.02 {
		t.Fatalf("последний ряд не по центру: %.2f, центр %.2f", c, mid)
	}
	top, bot := rs[0].Y, last.Y+last.H
	if c := (top + bot) / 2; math.Abs(c-(area.Y+area.H/2)) > 0.02 {
		t.Fatalf("блок не по центру по высоте: %.2f", c)
	}
	// Место — фото по центру ячейки: у 2:3 среди 3:2 ячейка шире, место уже её.
	mix := Tile(area, []float64{1.5, 2.0 / 3, 1.5}, 3, limit)
	if !(mix[1].W < mix[0].W) || math.Abs(mix[1].H-mix[0].H) > 0.01 {
		t.Fatalf("2:3 в ячейке 3:2: %+v", mix)
	}
}

func TestTileEdge(t *testing.T) {
	area, limit := tilePage(300)
	cases := map[string][]layout.Rect{
		"k=0":        Tile(area, nil, 3, limit),
		"пустая":     Tile(layout.Rect{W: 0, H: 100}, tileSet("L", 2), 3, limit),
		"NaN":        Tile(layout.Rect{W: math.NaN(), H: 100}, tileSet("L", 2), 3, limit),
		"Inf":        Tile(layout.Rect{W: math.Inf(1), H: 100}, tileSet("L", 2), 3, limit),
		"промежуток": Tile(layout.Rect{W: 10, H: 10}, tileSet("L", 2), 20, limit), // ни в ряд, ни в столбец
	}
	for name, rs := range cases {
		if rs != nil {
			t.Fatalf("%s: ждали пусто, %+v", name, rs)
		}
	}
	// Плохие пропорции и параметры — как 1:1, без NaN в ответе.
	rs := Tile(area, []float64{0, math.NaN(), math.Inf(1), -2}, math.NaN(), math.NaN())
	for _, r := range rs {
		if r.W != r.H || !finite(r.X, r.Y, r.W, r.H) {
			t.Fatalf("плохие пропорции: %+v", r)
		}
	}
}
