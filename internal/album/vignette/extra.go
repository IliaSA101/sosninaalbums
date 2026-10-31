package vignette

import (
	"math"
	"slices"
)

// Виды 049 (design §1.3): учитель в центре нижнего ряда, руководитель крупно, шапка в ряду.

// centerParts — доли нижней полосы области под ряд с учителем в центре.
func centerParts() []float64 { return []float64{0.20, 0.25, 0.30, 0.35, 0.40, 0.45} }

// centerZoneMax — зона учителей не шире этой доли полосы: по бокам должны остаться ученики.
const centerZoneMax = 0.6

// centerTry — раскладка центра при одной доле полосы.
type centerTry struct {
	teachs, pupils []Rect
	pwT, pwP       float64
}

// center — учителя в середине нижней полосы области zi, ученики сверху, по бокам (низ
// портретов — по низу учителя) и на прочих страницах. Лучшая доля — по min(pwT/1,3, pwP);
// ни одна не годится или общая сетка не хуже — inline (решение №114, §1.3.2).
func (b *builder) center(zi int, rs []Rect) {
	best, score := centerTry{}, -1.0
	for _, f := range centerParts() {
		c, ok := b.centerAt(zi, rs, f)
		if !ok {
			continue
		}
		if s := math.Min(c.pwT/teachMore, c.pwP); s > score+eps {
			best, score = c, s
		}
	}
	_, pw := b.grid(math.Max(b.in.HS, b.in.HST)).fit(b.in.Teachers+b.in.Pupils, rs)
	if score < 0 || score < pw/teachMore-eps {
		b.inline(rs)
		return
	}
	b.teachs, b.pupils = best.teachs, best.pupils
	b.plan.TeacherW, b.plan.PupilW = best.pwT, best.pwP
}

// centerAt — центр при доле f нижней полосы. Боковые прямоугольники ниже на разницу подписей:
// низ портретов учителя и боковых учеников — одна линия (полоса минус большая подпись).
func (b *builder) centerAt(zi int, rs []Rect, f float64) (centerTry, bool) {
	r := rs[zi]
	band, top := cutV(r, f*r.H, true, b.gy)
	line := math.Max(b.in.HS, b.in.HST) // от низа полосы до низа портретов
	zh := band.H - (line - b.in.HST)
	nt := float64(b.in.Teachers)
	tw := math.Max(zh-b.in.HST, 0) * b.in.Ratio
	zw := math.Min(nt*tw+(nt-1)*b.gx, centerZoneMax*band.W)
	if zw <= 0 {
		return centerTry{}, false
	}
	zone := Rect{band.X + (band.W-zw)/2, band.Y, zw, zh}
	sideW := math.Max((band.W-zw)/2-b.gx, 0)
	sideH := band.H - (line - b.in.HS)
	fl := Rect{band.X, band.Y, sideW, sideH}
	fr := Rect{zone.X + zone.W + b.gx, band.Y, sideW, sideH}
	tg, pwT := b.grid(b.in.HST).fit(b.in.Teachers, []Rect{zone})
	prs := []Rect{top, fl, fr}
	for i, x := range rs {
		if i != zi {
			prs = append(prs, x)
		}
	}
	pg, pwP := b.pgrid(b.in.HS).fit(b.in.Pupils, prs)
	if pwT <= 0 || pwP <= 0 {
		return centerTry{}, false
	}
	c := centerTry{pwT: pwT, pwP: pwP}
	c.teachs = alignBlock(tg, tg.place(b.in.Teachers, []Rect{zone}, pwT), zone, 0)
	c.pupils = pg.centerPupils(b.in.Pupils, prs, pwP)
	return c, len(c.teachs) == b.in.Teachers && len(c.pupils) == b.in.Pupils
}

// centerPupils — раздача учеников центра: по бокам (prs[1], prs[2]) до вместимости поровну,
// нечётный лишний — влево; остаток — сверху (prs[0]) и на прочие страницы по share. Своя
// раздача, а не share на всех: иначе верхний блок перестраивается в неровные ряды (7-7-6).
// Боковые блоки прижаты к учителю и вниз. Порядок: верх по рядам, левый бок, правый, прочие.
func (g grid) centerPupils(n int, prs []Rect, pw float64) []Rect {
	caps := make([]int, len(prs))
	cols := make([]int, len(prs))
	for i, r := range prs {
		c, k := g.dims(r, pw)
		caps[i], cols[i] = g.capOf(c, k), c
	}
	sides := min(n, caps[1]+caps[2])
	nl := min(caps[1], (sides+1)/2)
	nr := min(caps[2], sides-nl)
	nl = sides - nr
	restCaps := append([]int{caps[0]}, caps[3:]...)
	ks := share(n-sides, restCaps)
	out := make([]Rect, 0, n)
	out = append(out, g.block(ks[0], cols[0], prs[0], pw)...)
	out = append(out, alignBlock(g, g.block(nl, cols[1], prs[1], pw), prs[1], 1)...)
	out = append(out, alignBlock(g, g.block(nr, cols[2], prs[2], pw), prs[2], -1)...)
	for i := 3; i < len(prs); i++ {
		out = append(out, g.block(ks[i-2], cols[i], prs[i], pw)...)
	}
	return out
}

// alignBlock сдвигает блок мест (с полосой подписи g.hs) вниз области r и по горизонтали:
// side 1 — к правому краю, −1 — к левому, 0 — как стоит.
func alignBlock(g grid, cells []Rect, r Rect, side int) []Rect {
	if len(cells) == 0 {
		return cells
	}
	left, right, bottom := math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range cells {
		left, right, bottom = math.Min(left, c.X), math.Max(right, c.X+c.W), math.Max(bottom, c.Y+c.H+g.hs)
	}
	dx, dy := 0.0, r.Y+r.H-bottom
	switch side {
	case 1:
		dx = r.X + r.W - right
	case -1:
		dx = r.X - left
	}
	for i := range cells {
		cells[i].X += dx
		cells[i].Y += dy
	}
	return cells
}

// cellSplit — сколько из n людей ставить в последнюю область, чтобы в хвосте её последнего
// ряда встала шапка (headGrid: c×rn), а остальные поместились в прежних областях. Ближе всего к доле
// по вместимости (как share на n + 2 места); нельзя — ok false. Не перебор резерва k = 2…6
// (design §1.3.4): тот оставлял откаты на полосу сверху или дыры в предпоследнем ряду.
func (g grid) cellSplit(n int, rs []Rect, pw float64) (nl, c, rn int, ok bool) {
	if n <= 0 || pw <= 0 || len(rs) == 0 {
		return 0, 0, 0, false
	}
	caps := make([]int, len(rs))
	others := 0
	for i, r := range rs {
		c, k := g.dims(r, pw)
		caps[i] = c * k
		others += caps[i]
	}
	last := len(rs) - 1
	cm, rm := g.dims(rs[last], pw)
	others -= caps[last]
	lo, hi := max(n-others, 0), min(n, cm*rm-cellsMin)
	if lo > hi {
		return 0, 0, 0, false
	}
	want := min(max(share(n+cellsMin, caps)[last]-cellsMin, lo), hi)
	for d := 0; d <= hi-lo; d++ {
		for _, v := range [2]int{want - d, want + d} {
			if v < lo || v > hi {
				continue
			}
			if c, rn := headGrid(v, cm, rm); c > 0 {
				return v, c, rn, true
			}
		}
	}
	return 0, 0, 0, false
}

// cellPlace — n мест по областям rs при портрете pw и шапка в свободных ячейках последнего
// ряда последней области (§1.3.4): на развороте — правая страница, шапка не тянется через
// корешок. В последней области люди идут построчно с левого края блока, хвост последнего
// ряда (≥ 2 ячеек) — шапка до правого края блока, высота — портрет и полоса подписи; прочие
// области — как place. Пустых рядов и дыр нет; не вышло — head nil.
func (g grid) cellPlace(n int, rs []Rect, pw float64) (cells []Rect, head *Rect) {
	g.stagger = false // шапка в ряду — только у ровной сетки
	nl, c, rn, ok := g.cellSplit(n, rs, pw)
	if !ok {
		return nil, nil
	}
	last := len(rs) - 1
	cells = g.place(n-nl, rs[:last], pw)
	r, ph, step := rs[last], pw/g.a, g.cellH(pw)+g.gy
	blockW := float64(c)*pw + float64(c-1)*g.gx
	blockH := float64(rn)*step - g.gy
	x0, y0 := r.X+(r.W-blockW)/2, r.Y+(r.H-blockH)/2
	for i := range nl {
		cells = append(cells, Rect{x0 + float64(i%c)*(pw+g.gx), y0 + float64(i/c)*step, pw, ph})
	}
	used := float64(nl-(rn-1)*c) * (pw + g.gx) // занято людьми в последнем ряду
	return cells, &Rect{x0 + used, y0 + float64(rn-1)*step, blockW - used, g.cellH(pw)}
}

// headGrid — сетка c×rn для n людей построчно и шапки в хвосте последнего ряда: наименьшее
// число рядов, затем столбцов (блок узкий, как у block), людей хватает на все ряды, кроме
// последнего, в хвосте ≥ cellsMin ячеек. Нет такой в cmax×rmax — c = 0: при c столбцах
// годится любое n ≤ rn·c − 2, кроме n ≡ −1 (mod c) — тогда в хвосте одна ячейка.
func headGrid(n, cmax, rmax int) (c, rn int) {
	for rn = 1; rn <= rmax; rn++ {
		for c = cellsMin; c <= cmax; c++ {
			if (rn-1)*c <= n && rn*c-n >= cellsMin {
				return c, rn
			}
		}
	}
	return 0, 0
}

// lead — первый учитель крупно в зоне слева (на развороте Split — вся левая страница под
// шапкой), остальные учителя и ученики — одной сеткой в прочих областях с полосой большей
// подписи; учителя в ней первыми (очередь учителей 043 ставит руководителю первую строку).
// Выбор доли — по min(pwLead/2,2, pwRest), без остальных — pwLead (§1.3.3).
func (b *builder) lead(zi int, rs []Rect) {
	rest := b.in.Teachers - 1 + b.in.Pupils
	hs := math.Max(b.in.HS, b.in.HST)
	type try struct {
		zone, prs []Rect
		tg, pg    grid
		pwT, pwP  float64
	}
	var tries []try
	if b.t.Split && len(rs) > 1 {
		tries = append(tries, try{zone: []Rect{rs[0]}, prs: slices.Clone(rs[1:])})
	} else {
		for _, f := range bigParts() {
			zone, r := cutZone(rs[zi], PosLeft, f, b.gx, b.gy)
			c := try{zone: []Rect{zone}, prs: slices.Clone(rs)}
			c.prs[zi] = r
			tries = append(tries, c)
		}
	}
	best, score := try{}, -1.0
	for _, c := range tries {
		c.tg, c.pwT = b.grid(b.in.HST).fit(1, c.zone)
		c.pg, c.pwP = b.grid(hs).fit(rest, c.prs)
		s := math.Min(c.pwT/teachBig, c.pwP)
		if rest == 0 {
			s = c.pwT
		}
		if c.pwT > 0 && (rest == 0 || c.pwP > 0) && s > score+eps {
			best, score = c, s
		}
	}
	lead := best.tg.place(1, best.zone, best.pwT)
	cells := best.pg.place(rest, best.prs, best.pwP)
	if score < 0 || len(lead) < 1 || len(cells) < rest {
		b.inline(rs) // руководителю места нет — все общей сеткой
		return
	}
	nt := b.in.Teachers - 1
	b.teachs = slices.Concat(lead, cells[:nt])
	b.pupils = cells[nt:]
	b.plan.TeacherW, b.plan.LeadW = best.pwT, best.pwT
	if b.in.Pupils > 0 {
		b.plan.PupilW = best.pwP
	}
}
