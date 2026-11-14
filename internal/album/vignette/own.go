package vignette

import (
	"math"
	"slices"
)

// Свой шаблон виньетки (спека 038, design §2): места, роли и шапка, сохранённые с листа
// в долях области. BuildOwn переносит их на другие числа людей и другой формат: роль,
// у которой число и формат те же, стоит как была, иначе пересчитывается сеткой в своей зоне.
// Тот же размер области в мм и те же числа — точный режим: места ровно как сохранены.

// Shape — геометрия своего шаблона без зависимости от model (G3 переводит VigShape/VigTpl).
type Shape struct {
	Boxes  []Rect   // места в долях области
	Roles  []string // по одной на Boxes; короче — недостающие ученики
	Head   *Rect    // полоса шапки в долях области; nil — шапки нет
	Aspect float64  // W/H области при сохранении
	Ratio  float64  // портрет w/h при сохранении
	W, H   float64  // размер области при сохранении, мм; 0 — неизвестен (шаблон до поля)
}

const (
	aspectTol = 0.02 // область «того же формата», если W/H отличается меньше чем на 2 %
	minPiece  = 5.0  // мм: куски зон меньше не берём
	maxPieces = 8    // кусков зоны после вырезания препятствий
	snapPart  = 0.10 // отступ рамки композиции от края области, который дотягивается до края
	tolW      = 1e-6 // допуск сравнения ширин, мм
	fitTol    = 0.02 // мм: округление мест листа до 0,01 мм не делает план «неподходящим» (placesFit)
	sizeTol   = 0.05 // мм: область «того же размера» (доли в файле — до 1e-5)
)

// ownCtx — вход BuildOwn, разобранный по ролям.
type ownCtx struct {
	in     Input
	gx, gy float64
	obs    []Rect // шапка и групповые (с полосой подписи): стоят по долям, зонам мешают
	frames []Rect // рамки композиции страниц
}

// BuildOwn — места своего шаблона sh для области area (мм: 0, 0, W, H листа или разворота)
// и входа in. Порядок слотов — как у Build: ученики → учителя → групповые; не поместившиеся —
// нулевые слоты в углу первой области.
func BuildOwn(sh Shape, area Rect, in Input) Plan {
	in = clean(in)
	if len(in.Regions) == 0 {
		return Plan{}
	}
	c := &ownCtx{in: in, gx: math.Max(in.Gap, 2), gy: math.Max(in.Gap, in.Pad)}
	roles := map[string][]Rect{}
	var plan Plan
	var all []Rect // всё, что шаблон занимает: места с полосами, шапка
	valid := area.W > 0 && area.H > 0 && !math.IsInf(area.W+area.H, 0) && !math.IsNaN(area.W+area.H)
	exact := valid && sh.exact(area, in)
	if valid {
		for i, f := range sh.Boxes {
			if !finiteBox(f) {
				continue
			}
			role := sh.role(i)
			b := toMM(area, f)
			if role == RoleGroup && !exact {
				b = c.clipGroup(b)
			}
			roles[role] = append(roles[role], b)
			all = append(all, withStrip(b, in.HS, in.HST, role))
			if role == RoleGroup {
				c.obs = append(c.obs, withStrip(b, in.HS, in.HST, role))
			}
		}
		if sh.Head != nil && finiteBox(*sh.Head) {
			h := toMM(area, *sh.Head)
			plan.Head = &h
			c.obs, all = append(c.obs, h), append(all, h)
		}
	}
	if exact { // наложения и места на шапке поставил сам пользователь: не чиним
		plan.TeacherW, plan.PupilW = firstW(roles[RoleTeacher]), firstW(roles[RolePupil])
		return finishOwn(plan, c.in, roles[RoleTeacher], roles[RolePupil], roles[RoleGroup])
	}
	c.frames = c.pageFrames(all)
	teachs, pupils := c.people(sh, area, roles, valid, &plan)
	return finishOwn(plan, c.in, teachs, pupils, roles[RoleGroup])
}

// role — роль места i шаблона: недостающие и чужие — ученик.
func (sh Shape) role(i int) string {
	if i < len(sh.Roles) && (sh.Roles[i] == RoleTeacher || sh.Roles[i] == RoleGroup) {
		return sh.Roles[i]
	}
	return RolePupil
}

// exact — точный режим: область того же размера в мм (у шаблона он известен), тот же портрет
// и те же числа учителей и учеников (in — после clean). Шапку и групповые задаёт шаблон.
func (sh Shape) exact(area Rect, in Input) bool {
	if !(sh.W > 0 && sh.H > 0) || math.Abs(area.W-sh.W) > sizeTol || math.Abs(area.H-sh.H) > sizeTol ||
		math.Abs(in.Ratio-sh.Ratio) >= 1e-3 {
		return false
	}
	n := map[string]int{}
	for i, f := range sh.Boxes {
		if finiteBox(f) {
			n[sh.role(i)]++
		}
	}
	return n[RoleTeacher] == in.Teachers && n[RolePupil] == in.Pupils
}

// people — места учителей и учеников (§2 п. 2, 4–8) и размеры портретов в plan.
func (c *ownCtx) people(sh Shape, area Rect, roles map[string][]Rect, valid bool, plan *Plan) (teachs, pupils []Rect) {
	in := c.in
	T, P := in.Teachers, in.Pupils
	nT, nP := len(roles[RoleTeacher]), len(roles[RolePupil])
	// Поля и полосы подписи не масштабируются с областью: при той же пропорции, но другом размере
	// места «как есть» годятся, только если с полосами лежат в областях и не пересекаются.
	same := valid && sh.Aspect > 0 && math.Abs(area.W/area.H-sh.Aspect) <= aspectTol*sh.Aspect &&
		math.Abs(in.Ratio-sh.Ratio) < 1e-3 && c.placesFit(roles, plan.Head)
	zT := c.zones(stripAll(roles[RoleTeacher], in.HST))
	zP := c.zones(stripAll(roles[RolePupil], in.HS))
	switch {
	case same && T == nT && P == nP: // точный режим: исходный план
		teachs, pupils = roles[RoleTeacher], roles[RolePupil]
		plan.TeacherW, plan.PupilW = firstW(teachs), firstW(pupils)
	case anyHit(zT, zP) || T > 0 && nT == 0 || P > 0 && nP == 0: // общая сетка
		hs := in.HS
		if T > 0 {
			hs = math.Max(hs, in.HST)
		}
		cells, pw := c.fill(T+P, c.grow(mergeZones(zT, zP), nil), hs, 0)
		if len(cells) == T+P {
			teachs, pupils = cells[:T], cells[T:]
			if T > 0 {
				plan.TeacherW = pw
			}
			if P > 0 {
				plan.PupilW = pw
			}
		}
	default:
		exP := same && P == nP && (T > 0) == (nT > 0)
		if T == 0 {
			zT = nil // роль без людей зоны не держит: другая роль может занять место
		}
		if P == 0 {
			zP = nil
		}
		if exP {
			pupils, plan.PupilW = roles[RolePupil], firstW(roles[RolePupil])
		} else {
			zP = c.grow(zP, zT) // учителя растут уже мимо выросшей зоны учеников
			pupils, plan.PupilW = c.fill(P, zP, in.HS, 0)
		}
		capW := teachMax * plan.PupilW // учитель не крупнее teachMax портретов ученика — как у каталога
		switch {
		case T == 0:
		case same && T == nT && (capW <= 0 || firstW(roles[RoleTeacher]) <= capW+tolW):
			teachs, plan.TeacherW = roles[RoleTeacher], firstW(roles[RoleTeacher])
		default:
			teachs, plan.TeacherW = c.fill(T, c.grow(zT, zP), in.HST, capW)
		}
	}
	return teachs, pupils
}

// fill — n портретов в кусках зон rs; capW > 0 — верхняя граница ширины портрета.
func (c *ownCtx) fill(n int, rs []Rect, hs, capW float64) ([]Rect, float64) {
	g, pw := grid{gx: c.gx, gy: c.gy, a: c.in.Ratio, hs: hs}.fit(n, rs)
	if capW > 0 && pw > capW {
		pw = capW
	}
	return g.place(n, rs, pw), pw
}

// pageFrames — рамка композиции страницы: все занятые шаблоном места, полосы и шапка внутри
// области страницы; край, отстоящий от края области меньше чем на snapPart, дотягивается до него.
func (c *ownCtx) pageFrames(all []Rect) []Rect {
	out := c.zones(all)
	for i, f := range out {
		for _, r := range c.in.Regions {
			if hit(f, r) {
				out[i] = snap(f, r)
				break
			}
		}
	}
	return out
}

// zones — по странице объединение рамок boxes, обрезанное областью страницы.
func (c *ownCtx) zones(boxes []Rect) []Rect {
	un := make([]Rect, len(c.in.Regions))
	has := make([]bool, len(un))
	for _, b := range boxes {
		i := c.pageOf(b)
		if has[i] {
			un[i] = union(un[i], b)
		} else {
			un[i], has[i] = b, true
		}
	}
	var out []Rect
	for i, u := range un {
		if !has[i] {
			continue
		}
		if z, ok := inter(u, c.in.Regions[i]); ok {
			out = append(out, z)
		}
	}
	return out
}

// clipGroup — групповое с полосой подписи не выходит за область своей страницы: оно стоит по
// долям, а полоса не масштабируется. Вне всех областей — как есть.
func (c *ownCtx) clipGroup(g Rect) Rect {
	r := c.in.Regions[c.pageOf(g)]
	full := withStrip(g, c.in.HS, c.in.HST, RoleGroup)
	z, ok := inter(full, r)
	if !ok || contains(Rect{r.X - fitTol, r.Y - fitTol, r.W + 2*fitTol, r.H + 2*fitTol}, full) { // внутри — без обрезки (дрейфа дробей и округления места)
		return g
	}
	return Rect{z.X, z.Y, z.W, math.Max(z.H-c.in.HS, 0)}
}

// placesFit — места всех ролей с полосами лежат внутри областей и не пересекаются между собой и с шапкой.
// Места листа округлены до 0,01 мм (зажим места), поэтому план, вплотную прижатый к краю области,
// после сохранения в шаблон выступает за неё на сотые доли: ячейки сжимаются на fitTol с края.
func (c *ownCtx) placesFit(roles map[string][]Rect, head *Rect) bool {
	var cells []Rect
	for role, bs := range roles {
		for _, b := range bs {
			s := withStrip(b, c.in.HS, c.in.HST, role)
			cells = append(cells, Rect{s.X + fitTol, s.Y + fitTol, s.W - 2*fitTol, s.H - 2*fitTol})
		}
	}
	for i, x := range cells {
		if !slices.ContainsFunc(c.in.Regions, func(r Rect) bool { return contains(r, x) }) ||
			head != nil && hit(*head, x) {
			return false
		}
		for _, o := range cells[:i] {
			if hit(o, x) {
				return false
			}
		}
	}
	return true
}

// contains — a целиком лежит в r (с допуском).
func contains(r, a Rect) bool {
	return a.X >= r.X-tolW && a.Y >= r.Y-tolW && a.X+a.W <= r.X+r.W+tolW && a.Y+a.H <= r.Y+r.H+tolW
}

// pageOf — страница, где центр рамки b; вне всех — последняя.
func (c *ownCtx) pageOf(b Rect) int {
	cx, cy := b.X+b.W/2, b.Y+b.H/2
	for i, r := range c.in.Regions {
		if cx >= r.X && cx <= r.X+r.W && cy >= r.Y && cy <= r.Y+r.H {
			return i
		}
	}
	return len(c.in.Regions) - 1
}

// grow — зоны роли растут в свободное место рамки композиции (мимо шапки, групповых и
// зон other), затем препятствия внутри зоны вырезаются (п. 6–7).
func (c *ownCtx) grow(z, other []Rect) []Rect {
	block := append(slices.Clone(c.obs), other...)
	out := make([]Rect, 0, len(z))
	for _, r := range z {
		for _, f := range c.frames {
			if hit(f, r) {
				r = growRect(r, f, block, c.gx, c.gy)
				break
			}
		}
		out = append(out, r)
	}
	return carve(out, c.obs, c.gx)
}

// finishOwn собирает слоты: ученики → учителя → групповые; недостающие — нулевые в углу.
func finishOwn(plan Plan, in Input, teachs, pupils, groups []Rect) Plan {
	stub := Rect{in.Regions[0].X, in.Regions[0].Y, 0, 0}
	plan.Slots = make([]Slot, 0, in.Pupils+in.Teachers+len(groups))
	add := func(cells []Rect, n int, role string) {
		for i := range n {
			b := stub
			if i < len(cells) {
				b = cells[i]
			}
			plan.Slots = append(plan.Slots, Slot{b, role})
		}
	}
	add(pupils, in.Pupils, RolePupil)
	add(teachs, in.Teachers, RoleTeacher)
	add(groups, len(groups), RoleGroup)
	return plan
}

// growRect — r растёт вверх, вниз, влево и вправо до рамки f или до ближайшего из block с
// промежутком; каждая сторона — по пролёту r на тот момент, поэтому пересечений не появляется.
func growRect(r, f Rect, block []Rect, gx, gy float64) Rect {
	span := func(a0, a1, b0, b1 float64) bool { return a0 < b1-eps && b0 < a1-eps }
	top := f.Y
	for _, b := range block {
		if span(r.X, r.X+r.W, b.X, b.X+b.W) && b.Y+b.H <= r.Y+eps {
			top = math.Max(top, b.Y+b.H+gy)
		}
	}
	if top < r.Y {
		r.H += r.Y - top
		r.Y = top
	}
	bot := f.Y + f.H
	for _, b := range block {
		if span(r.X, r.X+r.W, b.X, b.X+b.W) && b.Y >= r.Y+r.H-eps {
			bot = math.Min(bot, b.Y-gy)
		}
	}
	if bot > r.Y+r.H {
		r.H = bot - r.Y
	}
	left := f.X
	for _, b := range block {
		if span(r.Y, r.Y+r.H, b.Y, b.Y+b.H) && b.X+b.W <= r.X+eps {
			left = math.Max(left, b.X+b.W+gx)
		}
	}
	if left < r.X {
		r.W += r.X - left
		r.X = left
	}
	right := f.X + f.W
	for _, b := range block {
		if span(r.Y, r.Y+r.H, b.Y, b.Y+b.H) && b.X >= r.X+r.W-eps {
			right = math.Min(right, b.X-gx)
		}
	}
	if right > r.X+r.W {
		r.W = right - r.X
	}
	return r
}

// snap — рамка f дотягивается до края области r, если отстоит от него меньше snapPart размера.
func snap(f, r Rect) Rect {
	x0, y0, x1, y1 := f.X, f.Y, f.X+f.W, f.Y+f.H
	if x0-r.X < snapPart*r.W {
		x0 = r.X
	}
	if r.X+r.W-x1 < snapPart*r.W {
		x1 = r.X + r.W
	}
	if y0-r.Y < snapPart*r.H {
		y0 = r.Y
	}
	if r.Y+r.H-y1 < snapPart*r.H {
		y1 = r.Y + r.H
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// subtract — r без препятствия o с промежутком gap: до 4 кусков (сверху, снизу, слева, справа).
func subtract(r, o Rect, gap float64) []Rect {
	e := Rect{o.X - gap, o.Y - gap, o.W + 2*gap, o.H + 2*gap}
	if !hit(r, e) {
		return []Rect{r}
	}
	var out []Rect
	add := func(x Rect) {
		if x.W >= minPiece && x.H >= minPiece {
			out = append(out, x)
		}
	}
	add(Rect{r.X, r.Y, r.W, e.Y - r.Y})
	add(Rect{r.X, e.Y + e.H, r.W, r.Y + r.H - (e.Y + e.H)})
	y0, y1 := math.Max(r.Y, e.Y), math.Min(r.Y+r.H, e.Y+e.H)
	add(Rect{r.X, y0, e.X - r.X, y1 - y0})
	add(Rect{e.X + e.W, y0, r.X + r.W - (e.X + e.W), y1 - y0})
	return out
}

// carve вырезает препятствия из зон; после каждого остаются maxPieces самых больших кусков.
func carve(rs, obs []Rect, gap float64) []Rect {
	for _, o := range obs {
		var next []Rect
		for _, r := range rs {
			next = append(next, subtract(r, o, gap)...)
		}
		slices.SortStableFunc(next, func(a, b Rect) int {
			switch {
			case a.W*a.H > b.W*b.H:
				return -1
			case a.W*a.H < b.W*b.H:
				return 1
			}
			return 0
		})
		rs = next[:min(len(next), maxPieces)]
	}
	return rs
}

// mergeZones — зоны двух ролей; пересекающиеся сливаются в общую рамку.
func mergeZones(a, b []Rect) []Rect {
	out := slices.Clone(a)
	for _, z := range b {
		merged := false
		for i := range out {
			if hit(out[i], z) {
				out[i], merged = union(out[i], z), true
			}
		}
		if !merged {
			out = append(out, z)
		}
	}
	return out
}

func toMM(a, f Rect) Rect { return Rect{a.X + f.X*a.W, a.Y + f.Y*a.H, f.W * a.W, f.H * a.H} }

// finiteBox — место из файла: доли конечные, размер положительный.
func finiteBox(f Rect) bool {
	for _, v := range []float64{f.X, f.Y, f.W, f.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return f.W > 0 && f.H > 0
}

// withStrip — место вместе с полосой подписи своей роли.
func withStrip(b Rect, hs, hst float64, role string) Rect {
	if role == RoleTeacher {
		b.H += hst
	} else {
		b.H += hs
	}
	return b
}

func stripAll(bs []Rect, hs float64) []Rect {
	out := make([]Rect, len(bs))
	for i, b := range bs {
		out[i] = Rect{b.X, b.Y, b.W, b.H + hs}
	}
	return out
}

func firstW(bs []Rect) float64 {
	if len(bs) == 0 {
		return 0
	}
	return bs[0].W
}

// hit — прямоугольники пересекаются по площади.
func hit(a, b Rect) bool {
	return a.X < b.X+b.W-eps && b.X < a.X+a.W-eps && a.Y < b.Y+b.H-eps && b.Y < a.Y+a.H-eps
}

func anyHit(a, b []Rect) bool {
	for _, x := range a {
		for _, y := range b {
			if hit(x, y) {
				return true
			}
		}
	}
	return false
}

func inter(a, b Rect) (Rect, bool) {
	x0, y0 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x1, y1 := math.Min(a.X+a.W, b.X+b.W), math.Min(a.Y+a.H, b.Y+b.H)
	if x1-x0 <= 0 || y1-y0 <= 0 {
		return Rect{}, false
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}, true
}

func union(a, b Rect) Rect {
	x0, y0 := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	x1, y1 := math.Max(a.X+a.W, b.X+b.W), math.Max(a.Y+a.H, b.Y+b.H)
	return Rect{x0, y0, x1 - x0, y1 - y0}
}
