package grid

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгонка шага по страницам (165, design §2): при SpreadTpl.Fit = "page" композиция делится
// на две половины, каждая одним масштабом вписывается в свою страницу (looktpl.PageFrame). Места,
// предметы и надписи считаются по кадру своей страницы; предметы в Anchor разворота не
// перекодируются (clampAnchor режет сдвиг до ±2 S, а центр правой страницы 40×30 — 2,67 S).

// pageFit — кадр половины композиции в странице части (tplPart.a — вся страница).
type pageFit struct {
	frame layout.Rect // кадр половины в странице, мм
	k     float64     // мм на единицу высоты композиции
	ratio float64     // Ratio композиции
	half  int         // 0 — левая половина композиции, 1 — правая
}

// fitParts — две части «страница + кадр» для шага t с ByPage.
func fitParts(t looktpl.SpreadTpl, l, r layout.Rect) []tplPart {
	mk := func(scope string, a layout.Rect, half int) tplPart {
		fr, k := looktpl.PageFrame(t, a)
		return tplPart{scope: scope, a: a, fit: &pageFit{frame: fr, k: k, ratio: t.Ratio, half: half}}
	}
	return []tplPart{mk(model.ScopeLeft, l, 0), mk(model.ScopeRight, r, 1)}
}

// fitted — части шага подогнаны по страницам.
func fitted(parts []tplPart) bool { return len(parts) == 2 && parts[0].fit != nil }

// compPoint — центр предмета в композиции: страница (0|1) и точка в долях страницы (растяжимый:
// u, v — доли ширины и высоты композиции, как в anchorPoint) либо в единицах композиции от угла
// её половины (обычный: S композиции = min(Ratio, 1)).
func compPoint(ratio float64, an looktpl.Anchor, stretch bool) (half int, x, y float64) {
	if stretch {
		u, v := float64(an.AX+1)/2+an.DX, float64(an.AY+1)/2+an.DY
		if u >= 0.5 {
			half = 1
		}
		return half, (u - 0.5*float64(half)) * 2, v
	}
	s := min(ratio, 1)
	cx, cy := float64(an.AX+1)/2*ratio+an.DX*s, float64(an.AY+1)/2+an.DY*s
	if cx >= ratio/2 {
		half = 1
	}
	return half, cx - float64(half)*ratio/2, cy
}

// owns — предмет с якорем an лежит на странице части (без подгонки — любая часть).
func (pt tplPart) owns(an looktpl.Anchor, stretch bool) bool {
	if pt.fit == nil {
		return true
	}
	h, _, _ := compPoint(pt.fit.ratio, an, stretch)
	return h == pt.fit.half
}

// scale — S части в мм: размер предмета и надписи = доля × S.
func (pt tplPart) scale() float64 {
	if pt.fit != nil {
		return min(pt.fit.ratio, 1) * pt.fit.k
	}
	return min(pt.a.W, pt.a.H)
}

// size — размер предмета в мм по долям w, h; растяжимый — по осям области (при подгонке — по
// осям всей своей страницы: доля ширины композиции — две доли ширины половины).
func (pt tplPart) size(w, h float64, stretch bool) (float64, float64) {
	switch {
	case stretch && pt.fit != nil:
		return w * 2 * pt.a.W, h * pt.a.H
	case stretch:
		return w * pt.a.W, h * pt.a.H
	}
	s := pt.scale()
	return w * s, h * s
}

// center — центр предмета по якорю, мм листа.
func (pt tplPart) center(an looktpl.Anchor, stretch bool) (float64, float64) {
	if pt.fit == nil {
		return anchorPoint(pt.a, an, stretch)
	}
	_, x, y := compPoint(pt.fit.ratio, an, stretch)
	if stretch {
		return pt.a.X + x*pt.a.W, pt.a.Y + y*pt.a.H
	}
	return pt.fit.frame.X + x*pt.fit.k, pt.fit.frame.Y + y*pt.fit.k
}

// fitBoxes — места шага t (доли композиции) в мм листа: каждое по кадру своей страницы
// (страница — по центру места), порядок мест сохраняется. Сторона на краю композиции или на
// сгибе прилипает к краю страницы (фото на весь лист остаётся навылет на любом формате); у
// наклонённого места прилипания нет.
func fitBoxes(t looktpl.SpreadTpl, parts []tplPart) []model.Box {
	out := make([]model.Box, 0, len(t.Places))
	for _, f := range t.Places {
		half := 0
		if f.X+f.W/2 >= 0.5 {
			half = 1
		}
		pt := parts[half]
		fr, k, rt := pt.fit.frame, pt.fit.k, pt.fit.ratio
		x0 := float64(half) * rt / 2
		l, r := fr.X+(f.X*rt-x0)*k, fr.X+((f.X+f.W)*rt-x0)*k
		tp, b := fr.Y+f.Y*k, fr.Y+(f.Y+f.H)*k
		if f.Rot == 0 {
			l, r = stickX(f.X, l, pt.a, half), stickX(f.X+f.W, r, pt.a, half)
			tp, b = stickY(f.Y, tp, pt.a), stickY(f.Y+f.H, b, pt.a)
		}
		out = append(out, model.Box{X: l, Y: tp, W: r - l, H: b - tp, Rot: f.Rot, Shape: f.Shape, Round: f.Round,
			Border: f.Border.Clone()})
	}
	return out
}

// stickX — x в мм (cur) стороны места с долей v композиции: у края композиции или сгиба —
// край страницы.
func stickX(v, cur float64, pg layout.Rect, half int) float64 {
	switch {
	case math.Abs(v) <= looktpl.FoldEps && half == 0, math.Abs(v-0.5) <= looktpl.FoldEps && half == 1:
		return pg.X
	case math.Abs(v-0.5) <= looktpl.FoldEps && half == 0, math.Abs(v-1) <= looktpl.FoldEps && half == 1:
		return pg.X + pg.W
	}
	return cur
}

// stickY — то же по вертикали: верх композиции — верх страницы, низ — низ.
func stickY(v, cur float64, pg layout.Rect) float64 {
	switch {
	case math.Abs(v) <= looktpl.FoldEps:
		return pg.Y
	case math.Abs(v-1) <= looktpl.FoldEps:
		return pg.Y + pg.H
	}
	return cur
}
