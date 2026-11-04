package geom

import "math"

// Решение сдвига кадра по лицу (решение 0.27-130-72): строгий приоритет линии глаз (Y листа)
// при наклоне давал скачок. Y листа зависит от X кадра на долю sin(R) хода: при R = 1° лицо
// у низа кадра, чья линия глаз недостижима на доли миллиметра, уезжало по X на ±25 мм ради
// этих долей. Теперь Y держится в полосе faceYBand от лучшего достижимого, а внутри полосы
// центр по X ставится как можно ближе к цели (допуск faceXTol не двигает Y).

// faceYBand — во сколько мм линия глаз может уйти от лучшей достижимой ради центра по X.
// Тратится только если центр иначе недостижим: центр достижим — берётся точка полосы с самой
// близкой к цели линией глаз. 2 мм = 0,2 см на отпечатке при лице ≈ 22 мм; при ходе кадра
// 60 мм полоса возвращает центр до наклона ≈ 2°, 1 мм — до ≈ 1°.
const faceYBand = 2 * FaceShiftTol

// shiftPt — точка (X, Y) в квадрате сдвигов кадра.
type shiftPt struct{ x, y float64 }

// faceXTol — центр по X в пределах этого допуска (мм) считается достигнутым: ради долей
// миллиметра по X линию глаз не двигаем.
const faceXTol = 0.5 * FaceShiftTol

// solveShift — (X, Y) ∈ [−1, 1]², при которых точка лица на листе
// (xs, ys) = (x0 + e·X + f·Y, y0 + a·X + b·Y) ближе к цели (tx, ty). Порядок: ys в полосе
// faceYBand от ближайшего достижимого к ty; в полосе — xs в допуске faceXTol от tx (нет —
// ближайшая к tx); среди таких точек — ys ближайшая к ty, из равных — xs ближайшая к tx.
// Нет хода по Y листа — свободен весь квадрат, двигаем вдоль x. Решение точное (выпуклый
// многоугольник и его сечения), не перебор.
func solveShift(y0, a, b, x0, e, f, ty, tx float64) (X, Y float64) {
	const eps = 1e-9
	ys := func(p shiftPt) float64 { return y0 + a*p.x + b*p.y }
	xs := func(p shiftPt) float64 { return x0 + e*p.x + f*p.y }
	reach := math.Abs(a) + math.Abs(b)
	if reach < eps {
		switch {
		case math.Abs(e) >= math.Abs(f) && math.Abs(e) >= eps:
			return ClampF((tx-x0)/e, -1, 1), 0
		case math.Abs(f) >= eps:
			return 0, ClampF((tx-x0)/f, -1, 1)
		}
		return 0, 0
	}
	yk := ClampF(ty, y0-reach, y0+reach) // ближайшая к ty достижимая линия глаз
	poly := []shiftPt{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	poly = clipHalf(poly, func(p shiftPt) float64 { return ys(p) - (yk - faceYBand) }) // ys ≥ yk − полоса
	poly = clipHalf(poly, func(p shiftPt) float64 { return (yk + faceYBand) - ys(p) }) // ys ≤ yk + полоса
	if len(poly) == 0 {
		return 0, 0 // полоса пуста: не бывает (yk достижима), страховка от округления
	}
	// Допустимое множество по X: полоса допуска вокруг tx; пуста — линия ближайшей достижимой xs.
	cand := clipHalf(poly, func(p shiftPt) float64 { return xs(p) - (tx - faceXTol) })
	cand = clipHalf(cand, func(p shiftPt) float64 { return (tx + faceXTol) - xs(p) })
	if len(cand) == 0 {
		xmin, xmax := math.Inf(1), math.Inf(-1)
		for _, p := range poly {
			xmin, xmax = math.Min(xmin, xs(p)), math.Max(xmax, xs(p))
		}
		xt := ClampF(tx, xmin, xmax)
		if cand = cutLine(poly, func(p shiftPt) float64 { return xs(p) - xt }); len(cand) == 0 {
			cand = poly // xs от кадра не зависит
		}
	}
	// В допустимом множестве — линия глаз как можно ближе к ty, на ней — центр ближе к tx.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range cand {
		lo, hi = math.Min(lo, ys(p)), math.Max(hi, ys(p))
	}
	yt := ClampF(ty, lo, hi)
	// yt зажата в [lo, hi] по вершинам cand, поэтому линия ys = yt непременно пересекает cand
	// (вершина на ней или две по разные стороны): seg не пуст.
	seg := cutLine(cand, func(p shiftPt) float64 { return ys(p) - yt })
	pick := closestOnSegment(seg, xs, tx)
	return ClampF(pick.x, -1, 1), ClampF(pick.y, -1, 1)
}

// closestOnSegment — точка отрезка seg (одна или две точки), где аффинная g ближе всего к
// цели c; g вдоль отрезка не меняется — точка, ближайшая к середине кадра.
func closestOnSegment(seg []shiftPt, g func(shiftPt) float64, c float64) shiftPt {
	if len(seg) == 1 {
		return seg[0]
	}
	p, q := seg[0], seg[len(seg)-1]
	vp, vq := g(p)-c, g(q)-c
	switch {
	case math.Abs(vp-vq) < 1e-9:
		return nearestToOrigin(seg)
	case vp*vq <= 0: // c между концами: ровно в c
		t := vp / (vp - vq)
		return shiftPt{p.x + t*(q.x-p.x), p.y + t*(q.y-p.y)}
	case math.Abs(vq) < math.Abs(vp):
		return q
	}
	return p
}

// clipHalf — выпуклый многоугольник, обрезанный до полуплоскости g ≥ 0 (Сазерленд — Ходжман).
func clipHalf(poly []shiftPt, g func(shiftPt) float64) []shiftPt {
	const eps = 1e-12
	var out []shiftPt
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		gp, gq := g(p), g(q)
		if gp >= -eps {
			out = append(out, p)
		}
		if (gp < -eps && gq > eps) || (gp > eps && gq < -eps) {
			t := gp / (gp - gq)
			out = append(out, shiftPt{p.x + t*(q.x-p.x), p.y + t*(q.y-p.y)})
		}
	}
	return out
}

// cutLine — точки, где граница выпуклого многоугольника пересекает линию g = 0 (вершины на
// линии тоже); у выпуклого их две крайние, остальные лежат между ними. Нет пересечения — nil.
func cutLine(poly []shiftPt, g func(shiftPt) float64) []shiftPt {
	const eps = 1e-12
	var out []shiftPt
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		gp, gq := g(p), g(q)
		switch {
		case math.Abs(gp) <= eps:
			out = append(out, p)
		case (gp < 0 && gq > eps) || (gp > 0 && gq < -eps):
			t := gp / (gp - gq)
			out = append(out, shiftPt{p.x + t*(q.x-p.x), p.y + t*(q.y-p.y)})
		}
	}
	if len(out) > 2 { // линия вдоль ребра: крайние точки по обходу — первая и самая далёкая от неё
		far := 1
		for k := 2; k < len(out); k++ {
			if dist(out[0], out[k]) > dist(out[0], out[far]) {
				far = k
			}
		}
		out = []shiftPt{out[0], out[far]}
	}
	return out
}

func dist(p, q shiftPt) float64 { return math.Hypot(p.x-q.x, p.y-q.y) }

// nearestToOrigin — точка отрезка seg (ровно две точки: одну closestOnSegment разбирает
// раньше) ближайшая к (0, 0).
func nearestToOrigin(seg []shiftPt) shiftPt {
	p, q := seg[0], seg[1]
	dx, dy := q.x-p.x, q.y-p.y
	l2 := dx*dx + dy*dy
	if l2 < 1e-18 {
		return p
	}
	t := ClampF(-(p.x*dx+p.y*dy)/l2, 0, 1)
	return shiftPt{p.x + t*dx, p.y + t*dy}
}
