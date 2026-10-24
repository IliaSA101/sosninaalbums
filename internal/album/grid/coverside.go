package grid

import (
	"errors"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Раскладки сторонки обложки (127): места сторонки ставятся в её безопасную зону (model.CoverSides)
// тем же applyGrid, что и сетки разворота; ряд «По фото» строит layout в режиме листа по
// пропорциям самих мест. Всё — чистые функции над *model.Project: Editor зовёт их внутри change.

const (
	// maxByPhoto — ряд «По фото» строится до 16 мест на сторонке; больше — ряда нет, форма не
	// узнаётся (0.26-127-7: полный перебор ≤ 5,2 мс).
	maxByPhoto = 16
	// sameTol — допуск совпадения форм, мм текущей зоны (0.26-127-5).
	sameTol = 0.1
	// idemTol — допуск «та же форма» при постановке: шум округления места.
	idemTol = 0.01
)

// errNoZone — у сторонки нет безопасной зоны (поля больше сторонки).
var errNoZone = errors.New("Безопасная зона сторонки пуста: проверьте поля в «Размерах для типографии»") //nolint:staticcheck // текст уходит пользователю как есть

// CoverSideIndex — номер сторонки в паре состояний и памяти формы: 0 — left (задняя), 1 — right.
func CoverSideIndex(scope string) (int, bool) {
	switch scope {
	case model.ScopeLeft:
		return 0, true
	case model.ScopeRight:
		return 1, true
	}
	return 0, false
}

// coverSide — сторонка обложки в проекте: зона, параметры layout и её места в порядке Grid.
type coverSide struct {
	s      *model.Spread
	scope  string
	box    layout.Rect
	prm    layout.Params
	places []*model.Place
}

// coverSides — сторонки проекта по индексам CoverSideIndex; nil — сторонки нет (нет обложки или
// зона пуста). Зона считается один раз на обе.
func coverSides(p *model.Project) (out [2]*coverSide) {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return out
	}
	s := p.Cover.Sheet
	for _, sd := range p.CoverSides() {
		i, _ := CoverSideIndex(sd.Scope) // CoverSides отдаёт только left и right
		// Рамка — безопасная зона, поле 0, промежуток альбома, Crop и Lock нулевые (0.26-127-3).
		c := &coverSide{s: s, scope: sd.Scope, box: sd.Box,
			prm: layout.Params{PageW: sd.Box.W / 2, PageH: sd.Box.H, Gap: p.Settings.Gap, Layflat: true, Sheet: true}}
		for _, pl := range s.Grid {
			if InArea(p, s, pl.X, pl.W, sd.Scope) {
				c.places = append(c.places, pl)
			}
		}
		out[i] = c
	}
	return out
}

// coverSideOf — одна сторонка; false — неизвестный scope или сторонки нет.
func coverSideOf(p *model.Project, scope string) (*coverSide, bool) {
	i, ok := CoverSideIndex(scope)
	if !ok {
		return nil, false
	}
	c := coverSides(p)[i]
	return c, c != nil
}

// photos — сколько мест сторонки с фото.
func (c *coverSide) photos() (n int) {
	for _, pl := range c.places {
		if pl.Photo != "" {
			n++
		}
	}
	return n
}

// aspects — пропорции мест по порядку: у места с фото — действующие пропорции ячейки «Авто»
// (свой кадр и четверть поворота, 098-18), у пустого — свои W/H.
func (c *coverSide) aspects(p *model.Project) []float64 {
	var ids []string
	for _, pl := range c.places {
		if pl.Photo != "" {
			ids = append(ids, pl.Photo)
		}
	}
	ph := p.Aspects(ids)
	out := make([]float64, len(c.places))
	for i, pl := range c.places {
		switch {
		case pl.Photo != "":
			out[i], ph = ph[0], ph[1:]
		default:
			out[i] = pl.W / max(pl.H, geom.MinPlace) // место не меньше MinPlace (ClampGrid)
		}
	}
	return out
}

// rel — места в мм от угла зоны, по порядку.
func (c *coverSide) rel() []layout.Rect {
	out := make([]layout.Rect, len(c.places))
	for i, pl := range c.places {
		out[i] = layout.Rect{X: pl.X - c.box.X, Y: pl.Y - c.box.Y, W: pl.W, H: pl.H}
	}
	return out
}

// boxes — доли зоны → места в мм листа обложки (рамочка — копия на место, 079-23). Место не меньше
// geom.MinPlace (меньшее ClampPlace раздул бы за край зоны) и целиком в зоне; у повёрнутого в зоне
// его описанный прямоугольник (0.26-127-28).
func (c *coverSide) boxes(frac []model.Box) []model.Box {
	z := c.box
	out := make([]model.Box, len(frac))
	for i, f := range frac {
		b := model.Box{X: z.X + f.X*z.W, Y: z.Y + f.Y*z.H, W: f.W * z.W, H: f.H * z.H,
			Rot: f.Rot, Shape: f.Shape, Round: f.Round, Border: f.Border.Clone()}
		if b.Rot == 0 {
			b.W, b.H = min(max(b.W, geom.MinPlace), z.W), min(max(b.H, geom.MinPlace), z.H)
			b.X, b.Y = geom.ClampF(b.X, z.X, z.X+z.W-b.W), geom.ClampF(b.Y, z.Y, z.Y+z.H-b.H)
		} else {
			b = holdRotated(b, z)
		}
		out[i] = b
	}
	return out
}

// holdRotated — повёрнутое место b целиком в зоне z: габарит больше зоны сжимается вокруг центра
// (угол и пропорции те же), затем сдвигается внутрь. Меньше MinPlace не сжимается: ClampPlace
// всё равно поднял бы сторону, а такая зона (< 5 мм) уже отсеяна как пустая.
func holdRotated(b model.Box, z layout.Rect) model.Box {
	r := geom.RotRect{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}
	bb := r.AABB()
	if k := min(1, z.W/bb.W, z.H/bb.H); k < 1 { // AABB растёт с размером места линейно
		cx, cy := b.X+b.W/2, b.Y+b.H/2
		b.W, b.H = max(b.W*k, geom.MinPlace), max(b.H*k, geom.MinPlace)
		b.X, b.Y = cx-b.W/2, cy-b.H/2
		r = geom.RotRect{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}
		bb = r.AABB()
	}
	b.X += geom.ClampF(bb.X, z.X, z.X+z.W-bb.W) - bb.X
	b.Y += geom.ClampF(bb.Y, z.Y, z.Y+z.H-bb.H) - bb.Y
	return b
}

// fracOf — прямоугольники в мм от угла зоны → доли зоны (зажим как у встроенных сеток).
func fracOf(rs []layout.Rect, box layout.Rect) []model.Box {
	out := make([]model.Box, len(rs))
	for i, r := range rs {
		out[i] = model.Box{X: r.X / box.W, Y: r.Y / box.H, W: r.W / box.W, H: r.H / box.H}
	}
	return model.ClampBoxes(out)
}

// rectsOf — доли зоны → прямоугольники в мм от угла зоны.
func rectsOf(frac []model.Box, box layout.Rect) []layout.Rect {
	out := make([]layout.Rect, len(frac))
	for i, f := range frac {
		out[i] = layout.Rect{X: f.X * box.W, Y: f.Y * box.H, W: f.W * box.W, H: f.H * box.H}
	}
	return out
}

// sameRects — формы совпали по порядку мест с допуском tol мм по каждой из x, y, w, h.
func sameRects(a, b []layout.Rect, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	near := func(x, y float64) bool { return math.Abs(x-y) <= tol }
	for i := range a {
		if !near(a[i].X, b[i].X) || !near(a[i].Y, b[i].Y) || !near(a[i].W, b[i].W) || !near(a[i].H, b[i].H) {
			return false
		}
	}
	return true
}

// byPhotoRow — ряд «По фото»: раскладки layout по пропорциям мест, порядок мест не меняется,
// по оценке; формы, совпавшие в допуске, оставлены раз. Нет мест или больше maxByPhoto — ряда нет.
func byPhotoRow(asp []float64, prm layout.Params) []layout.Result {
	if len(asp) == 0 || len(asp) > maxByPhoto || prm.PageW <= 0 || prm.PageH <= 0 {
		return nil
	}
	var out []layout.Result
	for _, r := range safeLayouts(asp, prm) {
		if len(r.Rects) == len(asp) && !slices.ContainsFunc(out, func(o layout.Result) bool { return sameRects(o.Rects, r.Rects, sameTol) }) {
			out = append(out, r)
		}
	}
	return out
}

// coverForm — что известно о форме мест сторонки.
type coverForm struct {
	row   []layout.Result // ряд «По фото» для текущих мест
	idx   int             // номер текущей формы в ряду; -1 — нет в нём
	known bool            // форма узнана (3.3): ‹ › листают
	alone bool            // узнана, в ряду одна раскладка, и она стоит
}

// recognize — узнавание формы мест сторонки: ни одно место не повёрнуто, 1…16 мест, и места по порядку
// совпали (± sameTol мм зоны) с раскладкой «По фото», с последней формой last (доли зоны) или с
// любой встроенной на эти n мест (n ≤ 7).
func (c *coverSide) recognize(p *model.Project, last []model.Box) coverForm {
	f := coverForm{idx: -1}
	n := len(c.places)
	if n == 0 || n > maxByPhoto || slices.ContainsFunc(c.places, func(pl *model.Place) bool { return pl.Rot != 0 }) {
		return f
	}
	if f.row = byPhotoRow(c.aspects(p), c.prm); len(f.row) == 0 {
		return f
	}
	cur := c.rel()
	f.idx = slices.IndexFunc(f.row, func(r layout.Result) bool { return sameRects(cur, r.Rects, sameTol) })
	f.known = f.idx >= 0 || sameRects(cur, rectsOf(last, c.box), sameTol) || n <= maxBuiltinN && c.matchBuiltin(cur)
	f.alone = f.idx == 0 && len(f.row) == 1
	return f
}

// matchBuiltin — места совпали с одной из раскладок четырёх наборов встроенных сеток на len(cur).
// Это надмножество показанных в галерее (там ряд обрезан по BuiltinLimit): узнать лишнюю не вредно.
func (c *coverSide) matchBuiltin(cur []layout.Rect) bool {
	for _, set := range builtinSets(nil, len(cur)) {
		for _, r := range safeLayouts(set, c.prm) {
			if sameRects(cur, r.Rects, sameTol) {
				return true
			}
		}
	}
	return false
}

// CoverSideStates — состояние сторонок для вида: места, узнана ли форма, одна ли раскладка в ряду.
// last — память последней формы Editor по CoverSideIndex (доли зоны). Нет сторонки — нули.
func CoverSideStates(p *model.Project, last [2][]model.Box) (out [2]model.CoverSideState) {
	for i, c := range coverSides(p) {
		if c == nil {
			continue
		}
		f := c.recognize(p, last[i])
		out[i] = model.CoverSideState{Places: len(c.places), Known: f.known, Alone: f.alone}
	}
	return out
}

// ApplyCoverGrid заменяет места сторонки scope раскладкой frac (доли безопасной зоны, порядок мест):
// фото садятся по порядку, лишние уходят в ленту (left). Другая сторонка, корешок, тексты, стикеры и
// фон не трогаются. Нет обложки или та же форма — холостая; не left/right — ErrScope; зоны нет —
// ошибка; пустые и нечисловые доли — отказ (ClampBoxesStrict).
func ApplyCoverGrid(p *model.Project, scope string, frac []model.Box) (left int, err error) {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return 0, nil
	}
	if _, ok := CoverSideIndex(scope); !ok {
		return 0, model.ErrScope
	}
	c, ok := coverSideOf(p, scope)
	if !ok {
		return 0, errNoZone
	}
	if frac, err = model.ClampBoxesStrict(frac); err != nil {
		return 0, err
	}
	boxes := c.boxes(frac)
	if c.sameAs(boxes) {
		return 0, nil
	}
	return applyGrid(p, c.s, scope, boxes)
}

// sameAs — места сторонки уже такие: тот же порядок, мм листа в пределах idemTol, угол и форма.
func (c *coverSide) sameAs(boxes []model.Box) bool {
	if len(c.places) != len(boxes) {
		return false
	}
	near := func(a, b float64) bool { return math.Abs(a-b) <= idemTol }
	for i, pl := range c.places {
		b := boxes[i]
		if !near(pl.X, b.X) || !near(pl.Y, b.Y) || !near(pl.W, b.W) || !near(pl.H, b.H) ||
			pl.Rot != b.Rot || pl.Shape != b.Shape || pl.Round != b.Round {
			return false
		}
	}
	return true
}

// StepCoverGrid листает ряд «По фото» сторонки scope на delta позиций по кругу и ставит раскладку
// как ApplyCoverGrid; shape — её доли зоны (Editor кладёт в память). Форма в ряду под номером i —
// (i + delta) mod len; узнана по памяти last (доли зоны) или встроенной — первая при delta > 0,
// последняя при delta < 0, как StepLayout. Нет сторонки, форма не узнана, в ряду одна раскладка
// или delta = 0 — nil и правки нет (UI решает по Known и Alone, открывать ли ▦). При err shape не берут:
// try откатывает правку.
func StepCoverGrid(p *model.Project, scope string, delta int, last [2][]model.Box) (shape []model.Box, err error) {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return nil, nil
	}
	i, ok := CoverSideIndex(scope)
	if !ok {
		return nil, model.ErrScope
	}
	c, ok := coverSideOf(p, scope)
	if !ok || delta == 0 {
		return nil, nil
	}
	f := c.recognize(p, last[i])
	if !f.known || f.alone {
		return nil, nil
	}
	m := len(f.row)
	next := 0
	switch {
	case f.idx >= 0:
		next = ((f.idx+delta)%m + m) % m
	case delta < 0:
		next = m - 1
	}
	if shape = fracOf(f.row[next].Rects, c.box); len(shape) != len(c.places) {
		return nil, nil // зажим выбросил место: раскладку не ставим
	}
	_, err = applyGrid(p, c.s, scope, c.boxes(shape)) // число мест то же: сверх MaxPlaces не выйдет
	return shape, err
}

// CoverSource — всё, что нужно CoverGridChoicesOf от проекта: снимается под замком (CoverInput),
// дальше layout считается уже вне него.
type CoverSource struct {
	found  bool
	scope  string
	box    layout.Rect
	prm    layout.Params
	asp    []float64
	places int
	photos int
}

// CoverInput читает из проекта всё для CoverGridChoicesOf; нет сторонки — источник без found.
func CoverInput(p *model.Project, scope string) CoverSource {
	c, ok := coverSideOf(p, scope)
	if !ok {
		return CoverSource{scope: scope}
	}
	return CoverSource{found: true, scope: scope, box: c.box, prm: c.prm, asp: c.aspects(p), places: len(c.places), photos: c.photos()}
}

// CoverGridChoicesOf считает галерею сторонки для вкладки n (зажимается в 1..7, меньше 1 — число
// мест сторонки, 0 мест — 1; в 7 входят и 8+ мест). «По фото» есть на вкладке, где сторонка
// сейчас (1…16 мест); «Встроенные» — сетки на n мест под пропорции зоны без форм «По фото»; «Мои» —
// шаблоны страницы на n мест (7 — от 7), шаблоны разворота и виньетки не берутся (0.26-127-16).
// Drop у «По фото» 0, у остальных — фото сторонки минус места. Только чтение; нет сторонки — пустые ряды.
func CoverGridChoicesOf(in CoverSource, n int, tpls []model.Template) model.CoverGridChoices {
	if n < 1 {
		n = in.places
	}
	n = max(1, min(n, maxBuiltinN))
	c := model.CoverGridChoices{Scope: in.scope, N: n, ByPhoto: []model.GridChoice{}, Builtin: []model.GridChoice{}, Mine: []model.GridChoice{}}
	if !in.found {
		return c
	}
	c.Box, c.Places, c.Photos = in.box, in.places, in.photos
	seen := map[string]bool{}
	if in.places >= 1 && in.places <= maxByPhoto && min(in.places, maxBuiltinN) == n {
		for _, r := range byPhotoRow(in.asp, in.prm) {
			seen[ShapeKey(r.Rects)] = true
			c.ByPhoto = append(c.ByPhoto, model.GridChoice{ID: r.Key, Scope: model.TplPage, Boxes: fracOf(r.Rects, in.box)})
		}
	}
	for _, g := range builtinGrids(in.prm, nil, n) {
		if seen[ShapeKey(rectsOf(g.Boxes, in.box))] {
			continue // форма уже в «По фото»
		}
		g.Scope, g.Drop = model.TplPage, dropCount(in.photos, len(g.Boxes), model.TplPage, in.scope)
		c.Builtin = append(c.Builtin, g)
	}
	for _, t := range tpls {
		if t.Scope != model.TplPage || t.Kind == model.TplVignette {
			continue
		}
		if len(t.Boxes) == n || (n == maxBuiltinN && len(t.Boxes) > n) {
			c.Mine = append(c.Mine, model.GridChoice{ID: t.ID, Name: t.Name, Scope: t.Scope, Boxes: t.Boxes,
				Drop: dropCount(in.photos, len(t.Boxes), t.Scope, in.scope)})
		}
	}
	return c
}
