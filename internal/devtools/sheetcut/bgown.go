package sheetcut

import "image"

// BGOptions — параметры снятия белого фона своим алгоритмом.
type BGOptions struct {
	White   uint8 // «почти белый»: минимум каналов не меньше (по умолчанию 250)
	HoleMin int   // замкнутая белая область меньше этого числа пикселей остаётся предметом
	Edge    int   // ширина полосы мягкого края у границы фона, px (по умолчанию 3)
}

// BGStats — что сделал алгоритм (для отчёта).
type BGStats struct {
	Holes        int // замкнутых «чистых» белых областей снято как фон (внутри колец и т. п.)
	HolesKept    int // крупных замкнутых белых областей оставлено (фактура или мягкая граница)
	BackgroundPx int // пикселей фона всего
}

// Пороги «чистой» замкнутой области (замер на листах Magnific: белая середина колец с шумом апскейла).
const (
	maxDeficit  = 1.5  // средняя недобелённость (255 − мин. канал) внутри области; кольца 0,5–1,1, крылья и бумага ≥ 2
	softDeficit = 3.0  // для области с лёгким градиентом (тень в кольце) граница должна быть явно тёмной:
	softDark    = 0.2  // доля тёмных на границе не меньше этого
	minInside   = 0.5  // доля «глубоких» пикселей (окрестность 5×5 белая) в области: узкие щели — не дыры
	rayLen      = 6    // на сколько px от границы области смотреть наружу
	rayDark     = 220  // пиксель луча «тёмный»: за границей настоящего предмета цвет уходит ниже
	darkShare   = 0.15 // доля граничных точек с тёмным лучом; у белых частей предмета граница светлая
)

const (
	stBG   = 1 // пиксель — фон
	stSeen = 2 // «почти белый» пиксель замкнутой области, уже разобранной (оставлена)
)

// bgWork — рабочее состояние одного прохода.
type bgWork struct {
	pix   []uint8
	w, h  int
	white uint8
	state []uint8 // по пикселю: 0, stBG, stSeen
	depth []uint8 // глубина в полосе мягкого края (только на время softEdge); 0 — вне полосы
}

func (b *bgWork) near(i int) bool { return minC(b.pix, i*4) >= b.white }

// RemoveBackground снимает белый фон с листа на месте: фон → альфа 0, у границы — мягкий край
// с восстановлением цвета, внутри предметов — альфа 255.
//
// Фон: (1) заливка от краёв по «почти белому»; (2) замкнутые белые области, не дотянувшиеся до
// края (внутри колец, между лапками), — тоже фон, если они крупнее HoleMin и «чистые»: внутри нет
// фактуры, граница контрастная. Белые части предметов с фактурой (крылья, бумага) остаются.
func RemoveBackground(img *image.NRGBA, o BGOptions) BGStats {
	if o.White == 0 {
		o.White = 250
	}
	if o.Edge <= 0 {
		o.Edge = 3
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if o.HoleMin <= 0 {
		o.HoleMin = max(400, w*h/2000)
	}
	b := &bgWork{pix: img.Pix, w: w, h: h, white: o.White, state: make([]uint8, w*h)}
	var st BGStats
	b.floodEdges()
	st.Holes, st.HolesKept = b.holes(o.HoleMin)
	b.softEdge(o.Edge)
	for i, s := range b.state {
		if s == stBG {
			copy(b.pix[i*4:i*4+4], []uint8{0, 0, 0, 0})
			st.BackgroundPx++
		}
	}
	return st
}

// floodEdges — заливка фона от краёв листа по «почти белому» (4-связность).
func (b *bgWork) floodEdges() {
	var stack []int
	push := func(i int) {
		if b.state[i] == 0 && b.near(i) {
			b.state[i] = stBG
			stack = append(stack, i)
		}
	}
	for x := 0; x < b.w; x++ {
		push(x)
		push((b.h-1)*b.w + x)
	}
	for y := 0; y < b.h; y++ {
		push(y * b.w)
		push(y*b.w + b.w - 1)
	}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%b.w, i/b.w
		if x > 0 {
			push(i - 1)
		}
		if x < b.w-1 {
			push(i + 1)
		}
		if y > 0 {
			push(i - b.w)
		}
		if y < b.h-1 {
			push(i + b.w)
		}
	}
}

// holes разбирает замкнутые «почти белые» области; чистые и крупные становятся фоном.
func (b *bgWork) holes(minArea int) (taken, kept int) {
	var comp []int
	for i := range b.state {
		if b.state[i] != 0 || !b.near(i) {
			continue
		}
		comp = b.collect(comp[:0], i)
		if len(comp) < minArea {
			continue
		}
		if b.cleanHole(comp) {
			for _, p := range comp {
				b.state[p] = stBG
			}
			taken++
		} else {
			kept++
		}
	}
	return taken, kept
}

// collect собирает 4-связную компоненту «почти белого» от пикселя start, помечая её stSeen.
func (b *bgWork) collect(comp []int, start int) []int {
	b.state[start] = stSeen
	comp = append(comp, start)
	var nb [4]int
	for k := 0; k < len(comp); k++ {
		n := b.neighbors(comp[k], &nb)
		for _, q := range nb[:n] {
			if b.state[q] == 0 && b.near(q) {
				b.state[q] = stSeen
				comp = append(comp, q)
			}
		}
	}
	return comp
}

// cleanHole — область «чистая»: внутри нет фактуры (почти идеальный белый), снаружи за границей
// цвет предмета заметно темнее белого (иначе это светлая часть предмета, а не дырка).
func (b *bgWork) cleanHole(comp []int) bool {
	interior, deficit := 0, 0
	for _, p := range comp {
		if b.deepInside(p) {
			interior++
			deficit += 255 - int(minC(b.pix, p*4))
		}
	}
	if interior == 0 || float64(interior) < minInside*float64(len(comp)) || float64(deficit) > softDeficit*float64(interior) {
		return false
	}
	border, dark := 0, 0
	var nb [4]int
	for _, p := range comp {
		n := b.neighbors(p, &nb)
		for _, q := range nb[:n] {
			if b.near(q) {
				continue
			}
			border++
			if b.darkAhead(p, q) {
				dark++
			}
		}
	}
	if border == 0 {
		return false
	}
	share := float64(dark) / float64(border)
	if float64(deficit) > maxDeficit*float64(interior) {
		return share >= softDark
	}
	return share >= darkShare
}

// darkAhead — на луче от p через q (соседний пиксель) в пределах rayLen px есть тёмный.
func (b *bgWork) darkAhead(p, q int) bool {
	step := q - p
	x, y := q%b.w, q/b.w
	dx, dy := 0, 0
	switch step {
	case 1:
		dx = 1
	case -1:
		dx = -1
	case b.w:
		dy = 1
	default:
		dy = -1
	}
	for k := 0; k < rayLen && x >= 0 && y >= 0 && x < b.w && y < b.h; k++ {
		if minC(b.pix, (y*b.w+x)*4) < rayDark {
			return true
		}
		x, y = x+dx, y+dy
	}
	return false
}

// deepInside — вся окрестность 5×5 пикселя «почти белая» (не ближе 2 px к границе области).
func (b *bgWork) deepInside(i int) bool {
	x, y := i%b.w, i/b.w
	if x < 2 || y < 2 || x >= b.w-2 || y >= b.h-2 {
		return false
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if !b.near(i + dy*b.w + dx) {
				return false
			}
		}
	}
	return true
}

// neighbors пишет в out 4-соседей пикселя внутри листа и возвращает их число.
func (b *bgWork) neighbors(i int, out *[4]int) int {
	n := 0
	x, y := i%b.w, i/b.w
	if x > 0 {
		out[n] = i - 1
		n++
	}
	if x < b.w-1 {
		out[n] = i + 1
		n++
	}
	if y > 0 {
		out[n] = i - b.w
		n++
	}
	if y < b.h-1 {
		out[n] = i + b.w
		n++
	}
	return n
}
