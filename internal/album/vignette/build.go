package vignette

import (
	"math"
	"slices"
)

// Доли зон от высоты (ширины) области зон, 031 design §4.
const (
	headPart  = 0.11 // полоса шапки, зажата в headMin…headMax мм
	headMin   = 14
	headMax   = 40
	groupPart = 0.38 // полоса группового фото
	groupA    = 1.5  // групповое — 3:2
	teachMore = 1.3  // учитель желательно в 1,3 раза крупнее ученика
	teachMax  = 2.0  // учителя на своей странице — не больше 2 портретов ученика
)

// teachParts — перебор доли зоны учителей; выбор — по min(pwT/1,3, pwP).
func teachParts() []float64 { return []float64{0.18, 0.22, 0.26, 0.30, 0.35} }

// builder собирает Plan; gx и gy — промежутки по горизонтали и вертикали.
type builder struct {
	in             Input
	gx, gy         float64
	pupils, teachs []Rect
	group          *Rect
	plan           Plan
}

// Build — места виньетки по шаблону t для входа in. Число слотов всегда
// Pupils + Teachers (+ 1 групповое у шаблона с ним): обёртке не нужно сверять.
func Build(t Tpl, in Input) Plan {
	in = clean(in)
	if len(in.Regions) == 0 {
		return Plan{}
	}
	t = adapt(t, len(in.Regions))
	b := &builder{in: in, gx: math.Max(in.Gap, 2), gy: math.Max(in.Gap, in.Pad)}
	b.build(t)
	return b.finish(t)
}

func (b *builder) grid(hs float64) grid {
	return grid{gx: b.gx, gy: b.gy, a: b.in.Ratio, hs: hs}
}

func (b *builder) build(t Tpl) {
	rs := slices.Clone(b.in.Regions)
	zi := 0 // область зон: у листа единственная, у разворота левая (учителя справа — правая)
	if t.Teach == PosRight && len(rs) > 1 {
		zi = 1
	}
	z, full := rs[zi], rs[zi].H
	if t.Head != PosNone {
		h := math.Min(math.Max(headPart*full, headMin), headMax)
		var head Rect
		head, z = cutV(z, h, t.Head == PosBottom, b.gy)
		b.plan.Head = &head
	}
	if t.Split {
		b.split(t, z, full, rs)
		return
	}
	if t.Group == PosTop || t.Group == PosBottom {
		var band Rect
		band, z = cutV(z, groupPart*full, t.Group == PosBottom, b.gy)
		b.groupIn(band)
	}
	rs[zi] = z
	b.people(t.Teach, zi, rs)
}

// people ставит учителей в их зону области zi и учеников во все области (без зоны).
func (b *builder) people(teach string, zi int, rs []Rect) {
	if b.in.Teachers == 0 || b.in.Pupils == 0 || teach == PosInline {
		b.inline(rs)
		return
	}
	type try struct {
		zone     []Rect
		prs      []Rect
		tg, pg   grid
		pwT, pwP float64
	}
	best, score := try{}, -1.0
	for _, f := range teachParts() {
		zone, rest := cutZone(rs[zi], teach, f, b.gx, b.gy)
		c := try{zone: []Rect{zone}, prs: slices.Clone(rs)}
		c.prs[zi] = rest
		c.tg, c.pwT = b.grid(b.in.HST).fit(b.in.Teachers, c.zone)
		c.pg, c.pwP = b.grid(b.in.HS).fit(b.in.Pupils, c.prs)
		if s := math.Min(c.pwT/teachMore, c.pwP); s > score+eps {
			best, score = c, s
		}
	}
	b.teachs = best.tg.place(b.in.Teachers, best.zone, best.pwT)
	b.pupils = best.pg.place(b.in.Pupils, best.prs, best.pwP)
	b.plan.TeacherW, b.plan.PupilW = best.pwT, best.pwP
}

// inline — учителя первыми в общей сетке того же размера; полоса — большая из двух.
func (b *builder) inline(rs []Rect) {
	t, p := b.in.Teachers, b.in.Pupils
	hs := b.in.HS
	if t > 0 {
		hs = math.Max(hs, b.in.HST)
	}
	g, pw := b.grid(hs).fit(t+p, rs)
	cells := g.place(t+p, rs, pw)
	if len(cells) < t+p {
		return // не поместились (пустые области) — finish поставит заглушки
	}
	b.teachs, b.pupils = cells[:t], cells[t:]
	if t > 0 {
		b.plan.TeacherW = pw
	}
	if p > 0 {
		b.plan.PupilW = pw
	}
}

// split — разворот: на левой странице шапка, учителя и (или) групповое, ученики — на правой.
// Слева мест нет (учителей 0 и группового нет) — ученики идут на обе страницы.
func (b *builder) split(t Tpl, left Rect, full float64, rs []Rect) {
	T := b.in.Teachers
	teachLeft := t.Teach == PosPage && T > 0
	if !teachLeft && t.Group != PosPage {
		rs[0] = left
		b.inline(rs)
		return
	}
	right := rs[1:]
	if teachLeft {
		g, pw := b.grid(b.in.HS).fit(b.in.Pupils, right)
		b.pupils, b.plan.PupilW = g.place(b.in.Pupils, right, pw), pw
	} else {
		b.inline(right) // учителя «в сетке» — с классом на правой странице
	}
	zone := left
	if t.Group == PosPage {
		if teachLeft {
			var band Rect
			band, zone = cutV(left, groupPart*full, true, b.gy)
			b.groupIn(band)
		} else {
			b.groupIn(left)
		}
	}
	if teachLeft {
		g, pw := b.grid(b.in.HST).fit(T, []Rect{zone})
		if b.plan.PupilW > 0 {
			pw = math.Min(pw, teachMax*b.plan.PupilW)
		}
		b.teachs, b.plan.TeacherW = g.place(T, []Rect{zone}, pw), pw
	}
}

// groupIn — групповое 3:2 наибольшее в полосе, с полосой подписи под ним, по центру.
func (b *builder) groupIn(band Rect) {
	hs := b.in.HS
	if band.H-hs <= 0 {
		hs = 0
	}
	w := math.Min(band.W, (band.H-hs)*groupA)
	h := w / groupA
	r := Rect{band.X + (band.W-w)/2, band.Y + (band.H-h-hs)/2, w, h}
	b.group = &r
}

// finish собирает слоты в порядке ученики → учителя → групповое и добивает недостающие
// нулевыми местами в углу первой области: число слотов обёртка не сверяет.
func (b *builder) finish(t Tpl) Plan {
	r0 := b.in.Regions[0]
	stub := Rect{r0.X, r0.Y, 0, 0}
	add := func(cells []Rect, n int, role string) {
		for i := range n {
			box := stub
			if i < len(cells) {
				box = cells[i]
			}
			b.plan.Slots = append(b.plan.Slots, Slot{box, role})
		}
	}
	b.plan.Slots = make([]Slot, 0, b.in.Pupils+b.in.Teachers+1)
	add(b.pupils, b.in.Pupils, RolePupil)
	add(b.teachs, b.in.Teachers, RoleTeacher)
	if t.Group != PosNone {
		var cells []Rect
		if b.group != nil {
			cells = []Rect{*b.group}
		}
		add(cells, 1, RoleGroup)
	}
	return b.plan
}

// cutV отрезает от r полосу высотой h сверху (или снизу) и промежуток gap; rest — остаток.
func cutV(r Rect, h float64, bottom bool, gap float64) (band, rest Rect) {
	h = math.Min(math.Max(h, 0), r.H)
	rh := math.Max(r.H-h-gap, 0)
	if bottom {
		return Rect{r.X, r.Y + r.H - h, r.W, h}, Rect{r.X, r.Y, r.W, rh}
	}
	return Rect{r.X, r.Y, r.W, h}, Rect{r.X, r.Y + r.H - rh, r.W, rh}
}

// cutH — то же по ширине: столбец слева (или справа).
func cutH(r Rect, w float64, right bool, gap float64) (band, rest Rect) {
	w = math.Min(math.Max(w, 0), r.W)
	rw := math.Max(r.W-w-gap, 0)
	if right {
		return Rect{r.X + r.W - w, r.Y, w, r.H}, Rect{r.X, r.Y, rw, r.H}
	}
	return Rect{r.X, r.Y, w, r.H}, Rect{r.X + r.W - rw, r.Y, rw, r.H}
}

// cutZone — зона учителей долей f области r по стороне pos.
func cutZone(r Rect, pos string, f, gx, gy float64) (zone, rest Rect) {
	switch pos {
	case PosRight:
		return cutH(r, f*r.W, true, gx)
	case PosTop:
		return cutV(r, f*r.H, false, gy)
	case PosBottom:
		return cutV(r, f*r.H, true, gy)
	}
	return cutH(r, f*r.W, false, gx)
}
