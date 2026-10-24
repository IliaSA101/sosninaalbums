package grid

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Готовые развороты (спека 067, design §2): снять места и оформление области в шаблон и
// поставить шаблон на область — одной правкой (места и оформление — один шаг отмены).

// ErrNothing — в области нет ни мест, ни оформления: шаблон не из чего снять.
var ErrNothing = errors.New("сохранять нечего: в области нет ни мест, ни оформления")

// tplPart — часть области цели, куда встаёт шаблон: область листа и зеркало (страница на весь
// разворот: правая — зеркально, решение 24).
type tplPart struct {
	scope  string
	a      layout.Rect
	mirror bool
}

// tplParts — куда встаёт шаблон t на области scope листа s. Шаблон разворота — только на весь
// разворот из двух страниц (ErrTplScope); страница на весь разворот — левая как есть, правая
// зеркально; лист 031 — одна область. Неизвестная область — ErrScope.
func tplParts(p *model.Project, s *model.Spread, scope string, t model.SpreadTpl) ([]tplPart, error) {
	a, ok := pageArea(p, s, scope)
	if !ok || !model.ValidScope(scope) {
		return nil, model.ErrScope
	}
	l, two := p.AreaOf(s, model.ScopeLeft)
	switch {
	case t.Pages == 2 && (scope != model.ScopeSpread || !two):
		return nil, ErrTplScope
	case t.Pages == 1 && scope == model.ScopeSpread && two:
		r, _ := p.AreaOf(s, model.ScopeRight)
		return []tplPart{{scope: model.ScopeLeft, a: l}, {scope: model.ScopeRight, a: r, mirror: true}}, nil
	}
	return []tplPart{{scope: scope, a: a}}, nil
}

// ApplySpreadTpl ставит шаблон t на область scope разворота spreadID: места заменяются (фото
// по порядку, лишние — в ленту, «Авто» закрепляется; на виньетке и листе 031 места не
// ставятся), оформление области — как у «Копировать оформление» (надписи — если они есть в
// шаблоне; шапка виньетки стоит). Свои из библиотеки берутся из libs (ключ → сведения о файле
// на этом ПК); ключа нет — слой пропускается, missing — сколько таких ключей. Область уже такая
// же — холостая правка (повтор не даёт шага). Обложка — ErrScope; неизвестный разворот —
// холостая; пустой шаблон — ErrTplEmpty.
func ApplySpreadTpl(p *model.Project, spreadID, scope string, t model.SpreadTpl, libs map[string]model.PhotoInfo) (plan model.GridPlan, missing int, err error) {
	if spreadID == model.CoverID {
		return plan, 0, model.ErrScope
	}
	_, s := p.Find(spreadID)
	if s == nil {
		return plan, 0, nil
	}
	t, ok := model.ClampSpreadTpl(t)
	if !ok {
		return plan, 0, model.ErrTplEmpty
	}
	parts, err := tplParts(p, s, scope, t)
	if err != nil {
		return plan, 0, err
	}
	r := &tplRes{p: p, s: s, libs: libs, gone: map[string]bool{}}
	sd, seq := r.design(t, scope, parts)
	_, two := p.AreaOf(s, model.ScopeLeft)
	var boxes []model.Box
	places := len(t.Places) > 0 && two && !s.IsVignette()
	if places {
		boxes = tplBoxes(p, s, parts, t)
	}
	o := putOpt{decor: true, texts: len(t.Texts) > 0}
	if (!places || samePlaces(p, s, scope, boxes)) && sameDecor(p, s, scope, sd, o) {
		return plan, len(r.gone), nil
	}
	if places {
		auto := len(s.Grid) == 0
		if plan.Left, err = applyGrid(p, s, scope, boxes); err != nil {
			return model.GridPlan{}, 0, err
		}
		if auto && len(s.Grid) > 0 {
			plan.Frozen = 1
		}
	}
	old := s.Kinds()
	if plan.Texts, plan.Stickers, err = putDesign(p, s, scope, sd, o); err != nil {
		return model.GridPlan{}, 0, err
	}
	decorOrder(s, old, seq)
	return plan, len(r.gone), nil
}

// PlanSpreadTpl — сухой прогон ApplySpreadTpl на копии проекта c (её делает обёртка под
// замком): вопрос до правки (фото в ленту, заменятся стикеры и надписи). Ошибка — та же, что
// даст правка.
func PlanSpreadTpl(c *model.Project, spreadID, scope string, t model.SpreadTpl, libs map[string]model.PhotoInfo) (model.GridPlan, error) {
	plan, _, err := ApplySpreadTpl(c, spreadID, scope, t, libs)
	if err != nil {
		return model.GridPlan{}, err
	}
	return plan, nil
}

// tplBoxes — места шаблона в мм частей области (зеркальная часть — mirrorFrac). Области и
// вид шаблона уже проверил tplParts, а мест не бывает только на листе 031: отказа FracBoxes
// здесь нет.
func tplBoxes(p *model.Project, s *model.Spread, parts []tplPart, t model.SpreadTpl) []model.Box {
	tpl := model.TplPage
	if t.Pages == 2 {
		tpl = model.TplSpread
	}
	var out []model.Box
	for _, pt := range parts {
		f := t.Places
		if pt.mirror {
			f = mirrorFrac(f)
		}
		b, _ := FracBoxes(p, s, tpl, pt.scope, f)
		out = append(out, b...)
	}
	return out
}

// samePlaces — места области scope уже те, что поставит шаблон (boxes после зажима, тот же
// порядок, без «Вписать», те же рамочки): повтор шаблона — холостой. У «Авто» мест нет — не те
// же. Рамочка сравнивается: иначе снимет ли шаблон рамочку, зависело бы от того, менялся ли фон.
func samePlaces(p *model.Project, s *model.Spread, scope string, boxes []model.Box) bool {
	var cur []*model.Place
	for _, pl := range s.Grid {
		if InArea(p, s, pl.X, pl.W, scope) {
			cur = append(cur, pl)
		}
	}
	if len(cur) != len(boxes) {
		return false
	}
	for i, b := range boxes {
		n := model.Place{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot, Shape: b.Shape, Round: b.Round, Border: b.Border}
		p.ClampPlace(s, &n)
		c := cur[i]
		if cents(c.X) != cents(n.X) || cents(c.Y) != cents(n.Y) || cents(c.W) != cents(n.W) || cents(c.H) != cents(n.H) ||
			c.Rot != n.Rot || c.Shape != n.Shape || c.Round != n.Round || c.Fit != "" || !sameBorder(c.Border, n.Border) {
			return false
		}
	}
	return true
}

// tplRes — предметы шаблона в мм листа s; ключи библиотеки → записи Photos.
type tplRes struct {
	p    *model.Project
	s    *model.Spread
	libs map[string]model.PhotoInfo
	gone map[string]bool // ключи без файла на этом ПК
}

// photo — ID записи Photos своего из библиотеки key; файла на этом ПК нет — "". Запись, уже
// стоящая в альбоме, не переписывается: повтор шаблона остаётся холостым.
func (r *tplRes) photo(key string) string {
	ph, ok := r.libs[key]
	if !ok {
		r.gone[key] = true
		return ""
	}
	id := model.LibPhotoID(key)
	if _, have := r.p.Photos[id]; !have {
		ph.ID, ph.Lib = id, key
		model.Remember(r.p, []model.PhotoInfo{ph})
	}
	return id
}

// bg — фото фона шаблона рамкой по области a (как «На фон» 048); нет ключа или файла — nil.
func (r *tplRes) bg(b model.TplBg, a layout.Rect) *model.Backdrop {
	if b.Lib == "" {
		return nil
	}
	id := r.photo(b.Lib)
	if id == "" {
		return nil
	}
	bd := &model.Backdrop{Photo: id, Fade: b.Fade}
	bd.FitArea(a)
	return bd
}

// design — оформление шаблона в мм области scope (srcDesign с x0 области: putDesign его не
// сдвигает) и виды слоёв seq для decorOrder: стикеры «под местами» — до места, остальные и
// надписи — после.
func (r *tplRes) design(t model.SpreadTpl, scope string, parts []tplPart) (srcDesign, []string) {
	p, s := r.p, r.s
	a := p.BdArea(s, scope)
	sd := srcDesign{x0: a.X}
	pg := t.Page[0]
	switch {
	case t.Pages == 2:
		l, rr := p.BdArea(s, model.ScopeLeft), p.BdArea(s, model.ScopeRight)
		sd.bgL, sd.bgR = t.Page[0].Color, t.Page[1].Color
		sd.patL, sd.patR = t.Page[0].Pat.Clone(), t.Page[1].Pat.Clone()
		sd.photoL, sd.photoR = r.bg(t.Page[0], l), r.bg(t.Page[1], rr)
		if t.Whole != nil {
			sd.pat, sd.photo = t.Whole.Pat.Clone(), r.bg(*t.Whole, a)
		}
	case len(parts) == 2: // страница на обе страницы разворота
		sd.bgL, sd.bgR = pg.Color, pg.Color
		sd.patL, sd.patR = pg.Pat.Clone(), pg.Pat.Clone()
		sd.photoL, sd.photoR = r.bg(pg, parts[0].a), r.bg(pg, parts[1].a)
	case scope == model.ScopeSpread: // лист 031: страниц нет, цвет листа — обе половины (FoldPages)
		sd.bgL, sd.bgR, sd.pat, sd.photo = pg.Color, pg.Color, pg.Pat.Clone(), r.bg(pg, a)
	default:
		sd.bgL, sd.patL, sd.photoL = pg.Color, pg.Pat.Clone(), r.bg(pg, a)
	}
	var under, above []*model.Sticker
	for _, pt := range parts {
		for _, it := range t.Stickers {
			st := r.sticker(it, pt)
			switch {
			case st == nil:
			case it.Under:
				under = append(under, st)
			default:
				above = append(above, st)
			}
		}
	}
	// decorOrder сопоставляет стикеры по порядку: сначала все «под местами» — со всех частей.
	sd.stickers = slices.Concat(under, above)
	k := tplScale(t)
	for _, pt := range parts {
		for _, tx := range t.Texts {
			sd.texts = append(sd.texts, r.text(tx, pt, k))
		}
	}
	seq := slices.Repeat([]string{model.LayerSticker}, len(under))
	seq = append(seq, model.LayerPlace)
	seq = append(seq, slices.Repeat([]string{model.LayerSticker}, len(above))...)
	seq = append(seq, slices.Repeat([]string{model.LayerText}, len(sd.texts))...)
	return sd, seq
}

// tplScale — S шаблона в мм (кегль надписей × S'/S); мм не записаны — 0 (кегль как есть).
func tplScale(t model.SpreadTpl) float64 {
	if t.W > 0 && t.H > 0 {
		return min(t.W, t.H)
	}
	return 0
}

// anchorPoint — центр предмета по якорю в области a: сдвиг в долях S = min(W, H) области, у
// растяжимого (stretch) — в долях ширины и высоты области, как и его размер.
func anchorPoint(a layout.Rect, an model.Anchor, stretch bool) (float64, float64) {
	sx, sy := min(a.W, a.H), min(a.W, a.H)
	if stretch {
		sx, sy = a.W, a.H
	}
	return a.X + float64(an.AX+1)/2*a.W + an.DX*sx, a.Y + float64(an.AY+1)/2*a.H + an.DY*sy
}

// sticker — стикер шаблона в части pt: размер по S (растяжимый — по осям области), центр по
// якорю, у зеркальной части — отражён (центр, FlipH, −Rot). Рамка сразу зажата и подогнана
// под пропорцию, как её оставит конвейер. Нет файла своего — nil.
func (r *tplRes) sticker(it model.TplItem, pt tplPart) *model.Sticker {
	st := &model.Sticker{Rot: it.Rot, FlipH: it.FlipH, FlipV: it.FlipV, Fade: it.Fade}
	if it.Art != nil {
		st.Art = &model.Art{Kind: it.Art.Kind, Inks: slices.Clone(it.Art.Inks), Seed: it.Art.Seed}
	} else if st.Photo = r.photo(it.Lib); st.Photo == "" {
		return nil
	}
	a, s := pt.a, min(pt.a.W, pt.a.H)
	w, h := it.W*s, it.H*s
	stretch := model.StretchArt(it.Art)
	if stretch {
		w, h = it.W*a.W, it.H*a.H
	}
	cx, cy := anchorPoint(a, it.Anchor, stretch)
	if pt.mirror {
		cx, st.FlipH, st.Rot = 2*a.X+a.W-cx, !st.FlipH, geom.NormDeg(-st.Rot)
	}
	st.X, st.Y, st.W, st.H = cx-w/2, cy-h/2, w, h
	c := r.p.ClampSticker(r.s, st)
	if c != nil {
		r.p.FitSticker(r.s, c)
	}
	return c
}

// text — надпись шаблона в части pt: ширина и кегль × S'/S (k = S шаблона; 0 — как есть),
// «по тексту» — сразу своя ширина, центр по якорю (зеркала у надписей нет — сдвиг), рамка
// внутри листа (как FitTexts).
func (r *tplRes) text(tx model.TplText, pt tplPart, k float64) model.TextBlock {
	s := min(pt.a.W, pt.a.H)
	st := tx.Style
	if k > 0 {
		st.Size = model.Round1(st.Size * s / k) // шаг кегля 0,1, как у ручки размера
	}
	t := model.TextBlock{W: tx.W * s, Rot: tx.Rot, Auto: tx.Auto, Text: tx.Text, Style: model.ClampStyle(st)}
	g := r.p.Geo(r.s)
	if t.Auto {
		t.W = r.p.SheetAutoW(r.s, &t)
	} else {
		t.W = geom.Round2(geom.ClampF(t.W, model.MinTextW, g.W))
	}
	cx, cy := anchorPoint(pt.a, tx.Anchor, false)
	dx, dy := geom.Rotate(t.Rot, t.W/2, model.BoxOf(&t).H/2)
	t.X, t.Y = geom.Round2(cx-dx), geom.Round2(cy-dy)
	t.X, t.Y = model.BoxOf(&t).FitIn(g.W, g.H)
	return t
}
