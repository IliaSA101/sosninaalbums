package grid

import (
	"cmp"
	"errors"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Операции над местами на уровне Project (спека 007, design §3): их зовут методы
// Editor внутри try, поэтому откат и undo получаются даром.

var (
	// ErrTplScope — шаблон разворота на страницу не ставится: доли разворота на
	// странице исказили бы места вдвое.
	ErrTplScope = errors.New("раскладку разворота можно поставить только на разворот")
)

// rectBox — прямоугольник раскладки без угла («Авто», встроенные сетки).
func rectBox(r layout.Rect) model.Box { return model.Box{X: r.X, Y: r.Y, W: r.W, H: r.H} }

// InArea — место листа s относится к области: у разворота — любое, у страницы — центр
// места на ней (applyGrid, копирование, тексты с design). У обложки центр на корешке
// не относится ни к одной сторонке.
func InArea(p *model.Project, s *model.Spread, x, w float64, scope string) bool {
	c := x + w/2
	a, ok := p.AreaOf(s, scope)
	switch {
	case !ok:
		return false
	case scope == model.ScopeSpread:
		return true
	case scope == model.ScopeLeft:
		return c < a.X+a.W
	}
	return c >= a.X
}

// applyGrid — общая постановка сетки (копирование, шаблон, встроенные; design §3):
// «Авто» сначала закрепляется, места области scope снимаются, новые места boxes
// (мм разворота, зажимаются) встают на позицию первого снятого в Grid — места
// другой страницы и их слои не двигаются; фото снятых мест по порядку садятся в
// новые, лишние уходят из альбома. left — сколько фото ушло. Тексты и фон не
// трогаются. Больше MaxPlaces в итоге — ошибка (try откатывает).
func applyGrid(p *model.Project, s *model.Spread, scope string, boxes []model.Box) (left int, err error) {
	if _, ok := p.AreaOf(s, scope); !ok {
		return 0, model.ErrScope
	}
	p.Freeze(s)
	had := len(s.Photos)
	at, kept := -1, make([]*model.Place, 0, len(s.Grid)+len(boxes))
	var q []string
	for _, pl := range s.Grid {
		if !InArea(p, s, pl.X, pl.W, scope) {
			kept = append(kept, pl)
			continue
		}
		if at < 0 {
			at = len(kept)
		}
		if pl.Photo != "" {
			q = append(q, pl.Photo)
		}
	}
	if at < 0 {
		at = len(kept)
	}
	if len(kept)+len(boxes) > model.MaxPlaces {
		return 0, model.ErrTooManyPlaces()
	}
	taken := p.PlaceIDs()
	fresh := make([]*model.Place, len(boxes))
	for i, b := range boxes {
		fresh[i] = &model.Place{ID: model.NewPlaceID(taken), X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}
		p.ClampPlace(s, fresh[i])
	}
	s.Grid = slices.Insert(kept, at, fresh...)
	for i, id := range q[:min(len(q), len(fresh))] {
		p.Put(s, fresh[i], id)
	}
	s.Photos = model.GridPhotos(s) // фото без нового места покидают разворот
	return had - len(s.Photos), nil
}

// FracBoxes переводит доли области шаблона tplScope (TplPage | TplSpread) в мм
// области scope листа s. Шаблон страницы на разворот ставится на обе страницы.
func FracBoxes(p *model.Project, s *model.Spread, tplScope, scope string, frac []model.Box) ([]model.Box, error) {
	conv := func(out []model.Box, a layout.Rect) []model.Box {
		for _, f := range frac {
			out = append(out, model.Box{X: a.X + f.X*a.W, Y: a.Y + f.Y*a.H, W: f.W * a.W, H: f.H * a.H, Rot: f.Rot})
		}
		return out
	}
	area := func(sc string) layout.Rect { a, _ := p.AreaOf(s, sc); return a }
	if _, ok := p.AreaOf(s, model.ScopeLeft); !ok && (tplScope == model.TplPage || scope != model.ScopeSpread) {
		return nil, model.ErrScope // лист 031: ни страниц, ни шаблонов страницы
	}
	switch {
	case tplScope == model.TplSpread && scope == model.ScopeSpread:
		return conv(nil, area(scope)), nil
	case tplScope == model.TplPage && (scope == model.ScopeLeft || scope == model.ScopeRight):
		return conv(nil, area(scope)), nil
	case tplScope == model.TplPage && scope == model.ScopeSpread:
		return conv(conv(nil, area(model.ScopeLeft)), area(model.ScopeRight)), nil
	case tplScope == model.TplSpread:
		return nil, ErrTplScope
	}
	return nil, model.ErrScope
}

// AreaFrac — места области scope листа s в долях этой области (порядок мест).
func AreaFrac(p *model.Project, s *model.Spread, scope string) []model.Box {
	a, ok := p.AreaOf(s, scope)
	if !ok {
		return nil
	}
	var out []model.Box
	for _, pl := range s.Grid {
		if InArea(p, s, pl.X, pl.W, scope) {
			out = append(out, model.Box{X: (pl.X - a.X) / a.W, Y: (pl.Y - a.Y) / a.H, W: pl.W / a.W, H: pl.H / a.H, Rot: pl.Rot})
		}
	}
	return model.ClampBoxes(out) // то же правило, что у PlaceBoxes и шаблонов
}

// mirrorFrac — отражение долей области по горизонтали (копия «зеркально»): центр
// отражается, угол — −Rot (как у mirrorGrid).
func mirrorFrac(frac []model.Box) []model.Box {
	out := make([]model.Box, len(frac))
	for i, f := range frac {
		out[i] = f
		out[i].X, out[i].Rot = 1-f.X-f.W, geom.NormDeg(-f.Rot)
	}
	return out
}

// mirrorSet — места, которые отражает «Отразить» в области [a0, a1]: при
// развороте все, при странице — лежащие на ней целиком (повёрнутое — своим hold:
// его и держит зажим). Место через корешок при отражении страницы не трогается:
// отражённое, оно вышло бы за разворот.
func mirrorSet(grid []*model.Place, scope string, a0, a1 float64) []*model.Place {
	var out []*model.Place
	for _, pl := range grid {
		h := model.Hold(pl)
		if scope == model.ScopeSpread || a0-model.SpineEps <= h.X && h.X+h.W <= a1+model.SpineEps {
			out = append(out, pl)
		}
	}
	return out
}

// mirrorGrid отражает места области scope разворота-сетки s: x' = A0 + A1 − x − w,
// угол −Rot (центр отражается, описанный прямоугольник — тоже); y, w, h, ID, фото,
// Fit, слой и кадр фото (Frame.R) те же: зеркалится раскладка, а не фото; кадр, тексты (кроме шапки виньетки — mirrorHead) и фон не трогаются (design §3
// «Отражение»). false — отражать нечего: неизвестная область, нет мест или
// сетка симметрична (иначе в симметричной паре фото поменялись бы местами).
func mirrorGrid(p *model.Project, s *model.Spread, scope string) bool {
	a, ok := p.AreaOf(s, scope)
	if !ok {
		return false
	}
	a0, a1 := a.X, a.X+a.W
	set := mirrorSet(s.Grid, scope, a0, a1)
	if len(set) == 0 || symmetric(set, a0+a1) {
		return false
	}
	for _, pl := range set {
		pl.X, pl.Rot = geom.Round2(a0+a1-pl.X-pl.W), geom.NormDeg(-pl.Rot)
		p.ClampPlace(s, pl) // область отражается в себя; зажим — только от шума округления
	}
	return true
}

// mirrorHead отражает надпись-шапку виньетки s в области scope (при странице — шапку, целиком
// лежащую на ней): без неё «Отразить» разворота уводил учеников под шапку, а она оставалась.
// Как у мест: центр отражается, угол −Rot; текст и стиль те же. false — шапки нет, она в
// другой области или симметрична.
func mirrorHead(p *model.Project, s *model.Spread, scope string) bool {
	a, ok := p.AreaOf(s, scope)
	if !ok || !s.IsVignette() {
		return false
	}
	h := vigHead(s, s.Vig.Head)
	if h == nil {
		return false
	}
	b := model.BoxOf(h)
	if bb := b.AABB(); scope != model.ScopeSpread && (bb.X < a.X-model.SpineEps || bb.X+bb.W > a.X+a.W+model.SpineEps) {
		return false
	}
	cx, cy := b.Center()
	rot := geom.NormDeg(-b.Rot)
	dx, dy := geom.Rotate(rot, b.W/2, b.H/2)
	x, y := geom.Round2(2*a.X+a.W-cx-dx), geom.Round2(cy-dy)
	if cents(x) == cents(h.X) && cents(y) == cents(h.Y) && rot == geom.NormDeg(h.Rot) {
		return false
	}
	h.X, h.Y, h.Rot = x, y, rot
	return true
}

// cents — мм в сотых для сравнения без шума double.
func cents(v float64) int64 { return int64(math.Round(v * 100)) }

// symmetric — отражённые места (x, y, w, h, угол −rot, fit) совпадают с исходными
// как мультимножество, с точностью 0.01 мм; слои не учитываются.
func symmetric(set []*model.Place, axis2 float64) bool {
	type key struct {
		x, y, w, h int64
		rot        int64 // десятые градуса, geom.NormDeg
		fit        string
	}
	deci := func(d float64) int64 { return int64(math.Round(geom.NormDeg(d) * 10)) }
	orig, mir := make([]key, len(set)), make([]key, len(set))
	for i, pl := range set {
		orig[i] = key{cents(pl.X), cents(pl.Y), cents(pl.W), cents(pl.H), deci(pl.Rot), pl.Fit}
		mir[i] = orig[i]
		mir[i].x, mir[i].rot = cents(axis2-pl.X-pl.W), deci(-pl.Rot)
	}
	byKey := func(a, b key) int {
		return cmp.Or(cmp.Compare(a.x, b.x), cmp.Compare(a.y, b.y), cmp.Compare(a.w, b.w),
			cmp.Compare(a.h, b.h), cmp.Compare(a.rot, b.rot), cmp.Compare(a.fit, b.fit))
	}
	slices.SortFunc(orig, byKey)
	slices.SortFunc(mir, byKey)
	near := func(a, b int64) bool { return a-b <= 1 && b-a <= 1 }
	for i := range orig {
		o, m := orig[i], mir[i]
		if !near(o.x, m.x) || !near(o.y, m.y) || !near(o.w, m.w) || !near(o.h, m.h) || o.rot != m.rot || o.fit != m.fit {
			return false
		}
	}
	return true
}

// Rescale — поменялся формат страницы: места остаются на том же месте
// страницы пропорционально (x, w — по ширине, y, h — по высоте), затем зажим.
// У повёрнутого центр — по kx, ky, угол стоит, стороны — длины образов осей места
// (иначе под 90° при kx ≠ ky ширина места тянулась бы по высоте страницы).
// Поля и промежуток места не трогают. Только развороты: обложку пересчитывает свой
// Reflow (032 §3).
func Rescale(p *model.Project, before *model.Project) {
	og, ng := before.Geo(nil), p.Geo(nil)
	ow, oh, nw, nh := og.W, og.H, ng.W, ng.H
	if ow == nw && oh == nh || !(ow > 0) || !(oh > 0) {
		return
	}
	kx, ky := nw/ow, nh/oh
	for _, s := range p.Spreads {
		for _, pl := range s.Grid {
			if pl.Rot == 0 {
				pl.X, pl.W, pl.Y, pl.H = pl.X*kx, pl.W*kx, pl.Y*ky, pl.H*ky
			} else {
				sn, c := math.Sincos(pl.Rot * math.Pi / 180)
				cx, cy := (pl.X+pl.W/2)*kx, (pl.Y+pl.H/2)*ky
				pl.W, pl.H = pl.W*math.Hypot(kx*c, ky*sn), pl.H*math.Hypot(kx*sn, ky*c)
				pl.X, pl.Y = cx-pl.W/2, cy-pl.H/2
			}
			p.ClampPlace(s, pl)
		}
	}
}
