package gen

import "math"

// Срез картинки-рамочки (079 design §1, §3.1): доли l, t, r, b ширины и высоты картинки, где
// кончается рамка и начинается окно. Один зажим на разбор PNG (FrameWindow) и модель
// (model.ClampBorder, 079-25): импорт и Load дают одну геометрию.

// MaxCut — предел стороны среза: окно картинки не уже 10 % её стороны.
const MaxCut = 0.45

// cutPrec — шаг среза 1e-4 доли (меньше пикселя у картинки до 4096 px); делитель, а не множитель:
// v·1e4/1e4 даёт ближайшее к десятичному число, без хвоста двоичной дроби в файле.
const cutPrec = 1e4

// ClampCut — каждая сторона 0…MaxCut с шагом 1e-4; NaN и −Inf — 0, +Inf — MaxCut. Идемпотентна.
func ClampCut(c [4]float64) [4]float64 {
	var out [4]float64
	for i, v := range c {
		if !(v > 0) { // и NaN
			continue
		}
		v = math.Round(min(v, MaxCut)*cutPrec) / cutPrec
		out[i] = min(v, MaxCut) // округление не выводит за предел
	}
	return out
}
