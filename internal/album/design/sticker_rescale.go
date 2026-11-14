package design

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// edgeEps — зазор до края листа, мм, не больше которого стикер считается прижатым к нему
// (и «целиком на листе» с допуском на округление до сотых).
const edgeEps = 0.05

// rescaleSticker — стикер s при смене формата листа ow×oh → nw×nh: центр ×kx/×ky, стороны
// ×kx/×ky, затем итоговая пропорция (FitSticker — та же, что вернёт FitStickers). Стоявший
// ДО смены целиком на листе (по описанному прямоугольнику, поворот учтён) остаётся на нём
// ПОСЛЕ: сдвиг внутрь, прижатый к краю (зазор ≤ edgeEps) — к тому же краю; не влезает —
// уменьшается до листа с пропорцией. Выходивший за лист раньше (вынес сам) — как есть:
// без этого при kx > ky уголок у края становился выше места до края (0.27-129-107).
func rescaleSticker(p *model.Project, s *model.Spread, st *model.Sticker, ow, oh, nw, nh float64) {
	kx, ky := nw/ow, nh/oh
	was := st.Rect().AABB()
	gl, gr, gt, gb := was.X, ow-was.X-was.W, was.Y, oh-was.Y-was.H
	inside := min(gl, gr, gt, gb) >= -edgeEps
	cx, cy := (st.X+st.W/2)*kx, (st.Y+st.H/2)*ky
	st.W, st.H = geom.Round2(st.W*kx), geom.Round2(st.H*ky)
	st.X, st.Y = geom.Round2(cx-st.W/2), geom.Round2(cy-st.H/2)
	if !inside {
		return
	}
	// Итоговый размер — по пропорции; влезает ли описанный прямоугольник — считаем по нему.
	p.FitSticker(s, st)
	for range 5 {
		st.X, st.Y = cx-st.W/2, cy-st.H/2
		b := st.Rect().AABB()
		if b.W <= nw && b.H <= nh {
			break
		}
		f := min(nw/b.W, nh/b.H)
		st.W, st.H = geom.Floor2(st.W*f), geom.Floor2(st.H*f)
		p.FitSticker(s, st)
	}
	// Размер после цикла мог смениться (не сошёлся за заходы): box — от итогового размера.
	st.X, st.Y = cx-st.W/2, cy-st.H/2
	b := st.Rect().AABB()
	box := struct{ w, h float64 }{b.W, b.H}
	// Положение описанного прямоугольника: прижатое остаётся прижатым, иное — внутрь.
	left := edgePos(cx-box.w/2, box.w, nw, gl, gr)
	top := edgePos(cy-box.h/2, box.h, nh, gt, gb)
	// От описанного прямоугольника к рамке: центр один.
	st.X, st.Y = geom.Round2(left+box.w/2-st.W/2), geom.Round2(top+box.h/2-st.H/2)
}

// edgePos — левый (верхний) край описанного прямоугольника размером size на оси длиной side:
// был прижат к краю (зазор lo/hi ≤ edgeEps) — прижат; иначе pos, зажатая внутрь.
func edgePos(pos, size, side, lo, hi float64) float64 {
	top := geom.Floor2(side - size)
	switch {
	case lo <= edgeEps && hi > edgeEps:
		return 0
	case hi <= edgeEps && lo > edgeEps:
		return max(top, 0)
	}
	return geom.ClampF(geom.Round2(pos), 0, max(top, 0))
}
