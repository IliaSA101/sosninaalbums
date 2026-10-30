package geom

import "math"

// Поворот рамки фона через поле «Поворот, °» (QA 061-3, decisions 0.14 №33; 070 Г).
// Зеркало — frontend editor/backdrop/backdrop.ts turnBackdropBox, паритет testdata/parity/bdturn.json.

// uprightAngle — рамка лежит вдоль вертикали (около ±90°): угол в 45° от оси y.
func uprightAngle(deg float64) bool {
	return math.Abs(math.Mod(math.Mod(deg, 180)+180, 180)-90) < 45
}

// TurnBox — рамка b, повёрнутая на rot вокруг своего центра. Смена ориентации (около 0/180° ↔ ±90°)
// меняет W и H местами: горизонтальное фото на 90° встаёт вертикальной рамкой того же центра,
// иначе оно лежало бы поперёк узкой области (корешок) и не заполняло её. Без смены — поворот на месте.
// Угол rot возвращается как есть (нормирует зажим фона).
func TurnBox(b TextBox, rot float64) TextBox {
	cx, cy := b.Center()
	w, h := b.W, b.H
	if uprightAngle(b.Rot) != uprightAngle(rot) {
		w, h = h, w
	}
	dx, dy := Rotate(rot, w/2, h/2)
	return TextBox{X: cx - dx, Y: cy - dy, W: w, H: h, Rot: rot}
}
