package geom

import "github.com/IliaSA101/sosninaalbums/internal/layout"

// Поворот места (спека 010, design-rotation §2.1). Опора — центр места: код мест
// читает X…X+W как протяжённость и X+W/2 как центр. Зеркало — frontend geom/rotbox.ts,
// паритет testdata/parity/rotplace.json.

// MinPlace — меньшая сторона места, мм: меньше не попасть мышью и не разглядеть.
const MinPlace = 5.0

// RotRect — место (и место группы в groupxf.json): неповёрнутый прямоугольник и угол вокруг центра.
// Угол переносится как есть: аффинное преобразование угла рамки переносит и центр.
type RotRect struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	W   float64 `json:"w"`
	H   float64 `json:"h"`
	Rot float64 `json:"rot,omitempty"`
}

// ClampPlaceIn — model.ClampPlace в развороте sw×ph (чистая, паритет testdata/parity/rotplace.json).
func ClampPlaceIn(pl *RotRect, sw, ph float64) {
	pl.W = ClampF(Round2(pl.W), MinPlace, Floor2(sw))
	pl.H = ClampF(Round2(pl.H), MinPlace, Floor2(ph))
	pl.Rot = NormDeg(pl.Rot)
	if pl.Rot == 0 {
		pl.X = ClampF(Round2(pl.X), 0, Floor2(sw-pl.W))
		pl.Y = ClampF(Round2(pl.Y), 0, Floor2(ph-pl.H))
		return
	}
	r := min(pl.W, pl.H) / 2
	pl.X = clampIn01(Round2(pl.X), r-pl.W/2, sw-r-pl.W/2)
	pl.Y = clampIn01(Round2(pl.Y), r-pl.H/2, ph-r-pl.H/2)
}

// ClampMoveIn — зажим переноса повёрнутого места (SetPlaces): вылет описанного
// прямоугольника ни с одной стороны не растёт относительно prev — что было внутри,
// остаётся внутри. Иначе узкая полоса под 1° уезжала бы за лист почти целиком, а
// поворот к 0° потом прыгал бы. Сверх того — hold (инвариант Load). Без поворота —
// ClampPlaceIn. prev — то же место до правки (зажатое).
func ClampMoveIn(pl *RotRect, prev RotRect, sw, ph float64) {
	pl.W = ClampF(Round2(pl.W), MinPlace, Floor2(sw))
	pl.H = ClampF(Round2(pl.H), MinPlace, Floor2(ph))
	pl.Rot = NormDeg(pl.Rot)
	if pl.Rot == 0 {
		ClampPlaceIn(pl, sw, ph)
		return
	}
	was := prev.Box().AABB()
	half := pl.Box().AABB() // от положения зависит только сдвиг, размер — нет
	ax, ay := half.W/2, half.H/2
	r := min(pl.W, pl.H) / 2
	axis := func(v, size, a, o0, o1, side float64) float64 {
		lo, hi := max(a-o0, r)-size/2, min(side+o1-a, side-r)-size/2
		if ceil01(lo) > floor01(hi) { // размер вырос сильнее вылета — только hold
			lo, hi = r-size/2, side-r-size/2
		}
		return clampIn01(Round2(v), lo, hi)
	}
	pl.X = axis(pl.X, pl.W, ax, max(0, -was.X), max(0, was.X+was.W-sw), sw)
	pl.Y = axis(pl.Y, pl.H, ay, max(0, -was.Y), max(0, was.Y+was.H-ph), ph)
}

// Box — место как повёрнутая рамка: угол до поворота = центр + Rotate(Rot, −W/2, −H/2).
// Дальше corners, AABB — как у текста.
func (pl RotRect) Box() TextBox {
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	dx, dy := Rotate(pl.Rot, -pl.W/2, -pl.H/2)
	return TextBox{cx + dx, cy + dy, pl.W, pl.H, pl.Rot}
}

// AABB — описанный прямоугольник места: привязка, слои, корешок.
func (pl RotRect) AABB() layout.Rect {
	if pl.Rot == 0 {
		return layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}
	}
	return pl.Box().AABB()
}

// Hold — прямоугольник удержания: без поворота — само место, иначе квадрат
// min(W, H) вокруг центра (вписанный круг не зависит от угла).
func (pl RotRect) Hold() layout.Rect {
	if pl.Rot == 0 {
		return layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}
	}
	d := min(pl.W, pl.H)
	return layout.Rect{X: pl.X + (pl.W-d)/2, Y: pl.Y + (pl.H-d)/2, W: d, H: d}
}
