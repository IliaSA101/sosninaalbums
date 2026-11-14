package geom

import "math"

// Силуэт под лицо (спека 130, silhouette.md, решения 0.27-130-22, 26): направляющая при
// подгонке кадра. Чистая геометрия: строится по норме листа в мм окна места, фото и детектор
// не нужны. Числа сняты с 23 мест двух листов класса; окончательно — по ответу Марины.

// Единица силуэта — F, высота рамки лица по норме; начало — середина линии глаз, y вниз.
const (
	bustHeadCY    = -0.07 // центр эллипса головы
	bustHeadRX    = 0.50  // полуось головы по X
	bustHeadRY    = 0.68  // полуось головы по Y: макушка −0,75, подбородок +0,61
	bustNeckHalf  = 0.26  // полуширина шеи
	bustNeckY     = 0.74  // где шея переходит в плечи
	bustShoulderX = 1.45  // плечо по X
	bustShoulderY = 1.10  // плечо по Y; ниже — прямо вниз до низа окна

	bustHeadSegs     = 48 // отрезков дуги головы
	bustShoulderSegs = 16 // отрезков четверть-кривой плеча
)

// Bust — силуэт: EyeY — линия глаз от верха окна, Path — замкнутый контур (последняя точка не
// равна первой, замыкает вызывающий). Точки в мм окна, начало — левый верхний угол; плечи могут
// выходить за окно по бокам — обрезает вызывающий. Пустой Path — силуэта нет.
type Bust struct {
	EyeY float64
	Path [][2]float64
}

// FaceBust — силуэт под лицо по норме n в окне w × h мм: центр по X — середина окна, линия
// глаз — n.Eye·h, F = n.Size·h. Контур идёт от шеи справа вокруг головы против часовой стрелки
// на экране (y вниз), вниз по левому плечу, по низу окна и вверх по правому плечу. Негодный вход (размер окна ≤ 0,
// не число, n.Size ≤ 0) и результат не из конечных чисел — пустой Bust.
func FaceBust(w, h float64, n FaceNorm) Bust {
	if !finite(w, h) || w <= 0 || h <= 0 || !validNorm(n) {
		return Bust{}
	}
	f, cx, ey := n.Size*h, w/2, n.Eye*h
	pt := func(x, y float64) [2]float64 { return [2]float64{cx + x*f, ey + y*f} }
	// Низ контура — низ окна, но не выше конца плеча: иначе линия пошла бы вверх.
	bottom := max((h-ey)/f, bustShoulderY)

	path := make([][2]float64, 0, bustHeadSegs+2*bustShoulderSegs+5)
	// Дуга головы: от шеи справа (низ эллипса) через макушку к шее слева.
	a0 := math.Asin(bustNeckHalf / bustHeadRX)
	for k := 0; k <= bustHeadSegs; k++ {
		t := a0 + (2*math.Pi-2*a0)*float64(k)/bustHeadSegs
		path = append(path, pt(bustHeadRX*math.Sin(t), bustHeadCY+bustHeadRY*math.Cos(t)))
	}
	// Плечо — четверть эллипса: у шеи идёт вдоль, у края уходит вниз.
	var sh [bustShoulderSegs][2]float64
	for k := range sh {
		u := float64(k+1) / bustShoulderSegs * math.Pi / 2
		sh[k] = [2]float64{
			bustNeckHalf + (bustShoulderX-bustNeckHalf)*math.Sin(u),
			bustNeckY + (bustShoulderY-bustNeckY)*(1-math.Cos(u)),
		}
	}
	path = append(path, pt(-bustNeckHalf, bustNeckY))
	for _, s := range sh {
		path = append(path, pt(-s[0], s[1]))
	}
	path = append(path, pt(-bustShoulderX, bottom), pt(bustShoulderX, bottom))
	for k := len(sh) - 1; k >= 0; k-- {
		path = append(path, pt(sh[k][0], sh[k][1]))
	}
	path = append(path, pt(bustNeckHalf, bustNeckY))

	for _, p := range path {
		if !finite(p[0], p[1]) {
			return Bust{}
		}
	}
	return Bust{EyeY: ey, Path: path}
}
