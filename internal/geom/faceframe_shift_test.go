package geom

import (
	"math"
	"testing"
)

// near — сравнение с допуском для значений сдвигов.
func eqf(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// Y листа не зависит от кадра: двигаем вдоль большей по модулю оси X листа, зажим в [−1, 1].
func TestSolveShiftNoYTravel(t *testing.T) {
	cases := []struct {
		name                    string
		e, f, x0, tx, wantX, wY float64
	}{
		{"ось X листа, ход внутри", 4, 1, 10, 12, 0.5, 0},
		{"ось X листа, зажим сверху", 4, 1, 10, 100, 1, 0},
		{"ось X листа, зажим снизу", -4, 1, 10, 100, -1, 0},
		{"равные оси: берём X кадра", 2, 2, 0, 1, 0.5, 0},
		{"только Y кадра, ход внутри", 0, 2, 10, 11, 0, 0.5},
		{"только Y кадра, зажим", 0, -2, 10, 100, 0, -1},
		{"ось Y кадра сильнее", 1, 3, 0, 1.5, 0, 0.5},
		{"нет хода по X листа вовсе", 0, 0, 5, 9, 0, 0},
		{"ход ниже eps", 1e-12, -1e-12, 5, 9, 0, 0},
	}
	for _, c := range cases {
		X, Y := solveShift(50, 0, 0, c.x0, c.e, c.f, 70, c.tx)
		if !eqf(X, c.wantX) || !eqf(Y, c.wY) {
			t.Errorf("%s: (%v, %v), хотел (%v, %v)", c.name, X, Y, c.wantX, c.wY)
		}
	}
}

// Нечисловая цель по Y: полоса пуста, вместо паники — нулевой сдвиг; бесконечная цель
// зажимается до достижимой, нечисловая по X — не даёт NaN в ответе.
func TestSolveShiftNaNTarget(t *testing.T) {
	if X, Y := solveShift(50, 1, 0.5, 10, 1, 0, math.NaN(), 12); X != 0 || Y != 0 {
		t.Errorf("ty=NaN: (%v, %v), хотел (0, 0)", X, Y)
	}
	// ys = 50 + X + 0,5·Y: предел достижимости на (1, 1); xs = 10 + X — центр 11 при X = 1.
	if X, Y := solveShift(50, 1, 0.5, 10, 1, 0, math.Inf(1), 11); !eqf(X, 1) || !eqf(Y, 1) {
		t.Errorf("ty=+Inf: (%v, %v), хотел (1, 1)", X, Y)
	}
	if X, Y := solveShift(50, 1, 0.5, 10, 1, 0, 50, math.NaN()); math.IsNaN(X) || math.IsNaN(Y) {
		t.Errorf("tx=NaN: (%v, %v), хотел числа", X, Y)
	}
}

// Центр по X недостижим в полосе Y: берём ближайшую к нему достижимую xs, внутри — линию
// глаз ближе к цели. ys = y0 + X, xs = x0 + Y.
func TestSolveShiftXUnreachableInBand(t *testing.T) {
	X, Y := solveShift(50, 1, 0, 10, 0, 1, 50, 15)
	if !eqf(X, 0) || !eqf(Y, 1) {
		t.Errorf("tx выше предела: (%v, %v), хотел (0, 1)", X, Y)
	}
	X, Y = solveShift(50, 1, 0, 10, 0, 1, 50.4, 5)
	if !eqf(X, 0.4) || !eqf(Y, -1) {
		t.Errorf("tx ниже предела: (%v, %v), хотел (0.4, -1)", X, Y)
	}
}

// xs от кадра не зависит: цель по X не влияет, а линия глаз всё равно ближе всего к цели.
func TestSolveShiftXIndependent(t *testing.T) {
	X, Y := solveShift(50, 1, 0, 10, 0, 0, 50.3, 99)
	if !eqf(X, 0.3) || Y < -1 || Y > 1 {
		t.Errorf("(%v, %v), хотел X=0.3 и Y в квадрате", X, Y)
	}
}

// Отрезок решений вырождается в точку и вдоль него xs постоянна: closestOnSegment и
// nearestToOrigin берут точку, ближайшую к середине кадра.
func TestClosestOnSegmentFlat(t *testing.T) {
	g := func(p shiftPt) float64 { return 7 } // g вдоль отрезка не меняется
	cases := []struct {
		name string
		seg  []shiftPt
		want shiftPt
	}{
		{"одна точка", []shiftPt{{0.3, -0.2}}, shiftPt{0.3, -0.2}},
		{"отрезок мимо нуля", []shiftPt{{1, 1}, {1, -0.5}}, shiftPt{1, 0}},
		{"отрезок через ноль", []shiftPt{{-1, 0}, {1, 0}}, shiftPt{0, 0}},
		{"конец ближе нуля", []shiftPt{{0.5, 0.5}, {1, 1}}, shiftPt{0.5, 0.5}},
		{"концы совпали", []shiftPt{{0.2, 0.2}, {0.2, 0.2}}, shiftPt{0.2, 0.2}},
	}
	for _, c := range cases {
		got := closestOnSegment(c.seg, g, 1)
		if !eqf(got.x, c.want.x) || !eqf(got.y, c.want.y) {
			t.Errorf("%s: %+v, хотел %+v", c.name, got, c.want)
		}
	}
}

// g меняется вдоль отрезка: цель между концами — точно в ней; вне — ближайший конец.
func TestClosestOnSegmentSlope(t *testing.T) {
	g := func(p shiftPt) float64 { return p.x }
	seg := []shiftPt{{-1, 0}, {1, 2}}
	if got := closestOnSegment(seg, g, 0); !eqf(got.x, 0) || !eqf(got.y, 1) {
		t.Errorf("цель между концами: %+v, хотел {0 1}", got)
	}
	if got := closestOnSegment(seg, g, 5); !eqf(got.x, 1) || !eqf(got.y, 2) {
		t.Errorf("цель за вторым концом: %+v, хотел {1 2}", got)
	}
	if got := closestOnSegment(seg, g, -5); !eqf(got.x, -1) || !eqf(got.y, 0) {
		t.Errorf("цель за первым концом: %+v, хотел {-1 0}", got)
	}
}

// Пустой многоугольник и вырожденное сечение: обрезка и разрез не падают.
func TestClipCutDegenerate(t *testing.T) {
	g := func(p shiftPt) float64 { return p.x - 5 }
	sq := []shiftPt{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	if got := clipHalf(nil, g); len(got) != 0 {
		t.Errorf("clipHalf(nil) = %v", got)
	}
	if got := clipHalf(sq, g); len(got) != 0 {
		t.Errorf("полуплоскость мимо квадрата: %v", got)
	}
	if got := cutLine(nil, g); len(got) != 0 {
		t.Errorf("cutLine(nil) = %v", got)
	}
	if got := cutLine(sq, g); len(got) != 0 {
		t.Errorf("линия мимо квадрата: %v", got)
	}
	// Линия вдоль ребра x = 1: остаются две крайние точки ребра.
	if got := cutLine(sq, func(p shiftPt) float64 { return p.x - 1 }); len(got) != 2 {
		t.Errorf("линия вдоль ребра: %v, хотел 2 точки", got)
	}
	// Многоугольник из одной точки на линии: сама точка.
	if got := cutLine([]shiftPt{{1, 2}}, func(p shiftPt) float64 { return p.x - 1 }); len(got) != 1 {
		t.Errorf("одна точка на линии: %v", got)
	}
}
