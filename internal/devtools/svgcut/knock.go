package svgcut

import (
	"errors"
	"fmt"
	"image/color"
	"math"
)

// hitScale — масштаб служебного растра: 1:1 к viewBox, но не больше MaxRenderSide на сторону и
// maxRenderPx по площади (иначе лист 7000² дал бы 49 Мп, а Render отказал бы по пределу).
func hitScale(doc *Doc) (sx, sy float64, pw, ph int) {
	k := math.Min(1, math.Min(MaxRenderSide/math.Max(doc.W, doc.H), math.Sqrt(maxRenderPx/(doc.W*doc.H))))
	pw, ph = max(1, int(math.Round(doc.W*k))), max(1, int(math.Round(doc.H*k)))
	for pw*ph > maxRenderPx { // округление вверх на обеих сторонах может чуть перешагнуть площадь
		if pw >= ph {
			pw--
		} else {
			ph--
		}
	}
	return float64(pw) / doc.W, float64(ph) / doc.H, pw, ph
}

// Knock — результат попадания одной точки выреза.
type Knock struct {
	Point [2]float64 `json:"point"`
	White int        `json:"white"` // номер белого пути wN
	Path  int        `json:"path"`  // номер пути в файле, с 1
}

// whiteIndex нумерует белые пути (цвета листа, кроме самого листа): wN в отчёте и на подсветке.
func whiteIndex(doc *Doc, sheet int) (isWhite func(i int) bool, order []int, num map[int]int) {
	num = map[int]int{}
	if sheet < 0 {
		return func(int) bool { return false }, nil, num
	}
	c := doc.Paths[sheet].RGB
	isWhite = func(i int) bool { return i > sheet && doc.Paths[i].Flat() && doc.Paths[i].RGB == c }
	for i := range doc.Paths {
		if isWhite(i) {
			order = append(order, i)
			num[i] = len(order)
		}
	}
	return isWhite, order, num
}

// hitKnocks находит белый путь под каждой точкой. Попадание определяет браузер: служебный растр листа
// с белыми путями цветов-номеров; своего Безье нет. Под точкой не белый путь (чужой цвет, пустой лист
// или вне листа) — ошибка со всеми промахами сразу, а не по одному на прогон.
func hitKnocks(doc *Doc, sheet int, isWhite func(int) bool, points [][2]float64, render RenderFunc) ([]Knock, error) {
	if len(points) == 0 {
		return nil, nil
	}
	if sheet < 0 {
		return nil, errors.New("svgcut: knock задан, а лист не найден (нет прямоугольника во весь лист с каналами ≥ 245): цвет пустот неизвестен")
	}
	if render == nil {
		return nil, errors.New("svgcut: для knock нужен браузер (PACKPREP_CHROME)")
	}
	sx, sy, pw, ph := hitScale(doc)
	img, err := render(hitSVG(doc, sheet, isWhite, pw, ph), pw, ph)
	if err != nil {
		return nil, err
	}
	var hits []Knock
	var misses []error
	for _, pt := range points {
		x, y := int(math.Floor(pt[0]*sx)), int(math.Floor(pt[1]*sy))
		if pt[0] < 0 || pt[1] < 0 || pt[0] > doc.W || pt[1] > doc.H {
			misses = append(misses, fmt.Errorf("svgcut: knock (%g, %g): точка вне листа %g×%g", pt[0], pt[1], doc.W, doc.H))
			continue
		}
		x, y = min(x, pw-1), min(y, ph-1)
		pi, ok := decodeHit(img.NRGBAAt(x, y), len(doc.Paths))
		if !ok || !isWhite(pi) {
			misses = append(misses, fmt.Errorf("svgcut: knock (%g, %g): под точкой нет пути цвета листа", pt[0], pt[1]))
			continue
		}
		hits = append(hits, Knock{Point: pt, Path: pi + 1})
	}
	return hits, errors.Join(misses...)
}

// decodeHit возвращает номер пути из цвета служебного растра; прозрачный и чёрный — не белый путь.
func decodeHit(c color.NRGBA, n int) (int, bool) {
	if c.A != 255 {
		return 0, false
	}
	id := int(c.R)<<16 | int(c.G)<<8 | int(c.B)
	if id < 1 || id > n {
		return 0, false
	}
	return id - 1, true
}
