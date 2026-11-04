package sheetcut

import "image"

// Политика замкнутых белых областей (-holes) режима seed.
const (
	HolesClean = "clean" // по умолчанию: только «чистые» (правило bgown.holes) — внутри колец и паспарту без фактуры
	HolesAll   = "all"   // все замкнутые белые области: клинья между ветками венка
	HolesNone  = "none"  // ни одной: паспарту и страницы остаются бумагой
)

// Пределы порога «почти белого» по листу. Ниже 225 фон слипается с бледными частями рисунка,
// выше 248 у Mystic остаются пятна (замер на 23 листах пилота, 0.27-129-3).
const (
	thrMin       = 225
	thrMax       = 248
	thrMargin    = 8    // порог = 1-й перцентиль краёв и центра минус запас на шум JPEG
	thrBandPct   = 3    // полоса края листа, % стороны
	holesAllMin  = 64   // px: меньшие замкнутые белые точки (блики) в режиме all не дырявим
	seedEdgePx   = 2    // мягкий край режима seed
	windowMinPct = 0.05 // окно меньше доли площади программа не принимает (gen.FrameWindow)
)

// SeedOptions — параметры снятия фона «от затравок» для рамок, уголков и разделителей.
type SeedOptions struct {
	Thr     uint8  // «почти белый»: минимум каналов не меньше; 0 — по листу (AdaptiveThr)
	Holes   string // HolesClean (пусто), HolesAll, HolesNone
	Window  bool   // окно — заливка от центра, как в программе (gen.FrameWindow)
	HoleMin int    // clean: меньше этого числа px область остаётся предметом (0 — авто, как у own)
	Edge    int    // полоса мягкого края, px (0 — 2)
}

// AdaptiveThr — порог «почти белого» по листу: 1-й перцентиль минимума каналов в полосе края (3 %
// стороны), при window ещё и в центре (47…53 %), минус запас 8, в пределах 225…248. Постоянный 245
// оставлял пятна у Mystic: белая бумага там 238–244 (замер на 23 листах пилота).
func AdaptiveThr(img *image.NRGBA, window bool) uint8 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	bx, by := max(1, w*thrBandPct/100), max(1, h*thrBandPct/100)
	var hist [256]int
	n := 0
	for y := range h {
		inY := y >= by && y < h-by
		for x := range w {
			edge := !inY || x < bx || x >= w-bx
			centre := window && x > w*47/100 && x < w*53/100 && y > h*47/100 && y < h*53/100
			if edge || centre {
				hist[minC(img.Pix, (y*w+x)*4)]++
				n++
			}
		}
	}
	p1, acc := 0, 0
	for v, c := range hist {
		acc += c
		if acc*100 >= n {
			p1 = v
			break
		}
	}
	return uint8(min(thrMax, max(thrMin, p1-thrMargin))) //nolint:gosec // зажато в 225…248
}

// RemoveBackgroundSeed снимает белый фон с листа на месте (рамки, уголки, разделители):
//  1. окно (при Window) — заливка «почти белого» от центра, 4-связно, как в программе;
//  2. фон — заливка от краёв листа;
//  3. прочие замкнутые белые области — по Holes;
//  4. мягкий край 2 px с восстановлением цвета.
//
// Порог один на лист. Статистика — в BGStats (Thr, WindowPx, Open).
func RemoveBackgroundSeed(img *image.NRGBA, o SeedOptions) BGStats {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w == 0 || h == 0 {
		return BGStats{}
	}
	if o.Thr == 0 {
		o.Thr = AdaptiveThr(img, o.Window)
	}
	if o.Edge <= 0 {
		o.Edge = seedEdgePx
	}
	if o.HoleMin <= 0 {
		o.HoleMin = max(400, w*h/2000)
	}
	b := &bgWork{pix: img.Pix, w: w, h: h, white: o.Thr, state: make([]uint8, w*h)}
	st := BGStats{Thr: o.Thr}
	if o.Window {
		st.WindowPx = b.floodFrom([]int{h/2*w + w/2})
		st.Open = st.WindowPx > 0 && b.touchesEdge()
	}
	b.floodEdges()
	switch o.Holes {
	case HolesNone:
	case HolesAll:
		st.Holes, st.HolesKept = b.holesAll(holesAllMin)
	default:
		st.Holes, st.HolesKept = b.holes(o.HoleMin)
	}
	b.softEdge(o.Edge)
	for i, s := range b.state {
		if s == stBG {
			clear(b.pix[i*4 : i*4+4])
			st.BackgroundPx++
		}
	}
	return st
}

// floodFrom — заливка «почти белого» 4-связно от затравок; возвращает число залитых px. Затравка не
// «почти белая» — не заливается (центр рамки закрашен: окна нет).
func (b *bgWork) floodFrom(seeds []int) int {
	var stack []int
	n := 0
	push := func(i int) {
		if b.state[i] == 0 && b.near(i) {
			b.state[i] = stBG
			stack = append(stack, i)
			n++
		}
	}
	for _, s := range seeds {
		push(s)
	}
	var nb [4]int
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		k := b.neighbors(i, &nb)
		for _, q := range nb[:k] {
			push(q)
		}
	}
	return n
}

// touchesEdge — залитая область достала край листа (окно открытое: уголки, завитки).
func (b *bgWork) touchesEdge() bool {
	for x := range b.w {
		if b.state[x] == stBG || b.state[(b.h-1)*b.w+x] == stBG {
			return true
		}
	}
	for y := range b.h {
		if b.state[y*b.w] == stBG || b.state[y*b.w+b.w-1] == stBG {
			return true
		}
	}
	return false
}

// holesAll — все замкнутые «почти белые» области не меньше minArea становятся фоном (клинья между
// ветками); возвращает, сколько снято и сколько мелких оставлено.
func (b *bgWork) holesAll(minArea int) (taken, kept int) {
	var comp []int
	for i := range b.state {
		if b.state[i] != 0 || !b.near(i) {
			continue
		}
		comp = b.collect(comp[:0], i)
		if len(comp) < minArea {
			kept++
			continue
		}
		for _, p := range comp {
			b.state[p] = stBG
		}
		taken++
	}
	return taken, kept
}
