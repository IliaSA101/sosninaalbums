package layout

// Узкий вход для layout/plan (группировка по разворотам): оценка и разрез рядов — те же
// функции, что у раскладки, без копий. Экспортировано только то, что plan читает сам;
// логика остаётся здесь, поэтому числа совпадают с прежним путём до бита. В конце — прежний
// путь расчёта (Ref*): эталон для тестов plan, в программе не вызывается.

import "sync"

// MaxFullTree — до скольких фото окно считается полным перебором деревьев (дальше — ряды).
const MaxFullTree = maxFullTree

// Geom — области разворота при заданных Params (то, что plan читает из geom).
type Geom struct {
	Spread Rect    // весь разворот без полей
	Pages  [2]Rect // левая и правая страница без полей
	Full   Rect    // весь разворот в край
	Spine  float64 // x корешка, у листа — 0
	g      geom    // целиком: pairScore читает и остальные поля
}

// NewGeom — геометрия по параметрам.
func NewGeom(p Params) Geom {
	g := newGeom(p)
	return Geom{Spread: g.spread, Pages: g.pages, Full: g.full, Spine: g.spine, g: g}
}

// Score — оценка раскладки rects в box (как у результата Layouts).
func Score(rects []Rect, box Rect, spine float64, depth int) float64 {
	return score(rects, box, spine, depth)
}

// PairScore — оценка пары страниц (hl, hr — высоты блоков левой и правой).
func PairScore(rects []Rect, g Geom, depth int, hl, hr float64) float64 {
	return pairScore(rects, g.g, depth, hl, hr, false)
}

// BreakTable — DP разреза на ряды (breakDP) над фото [lo, lo+M): From[k*(M+1)+j] — начало
// ряда k, кончающегося на j.
type BreakTable struct {
	M    int
	From []int
}

// BreakDP — таблица разреза [lo, hi) на r рядов (cols — столбцов); false — нет места.
// Значение, не указатель: иначе таблица уходит в кучу на каждый вызов.
func BreakDP(asp []float64, lo, hi int, box Rect, gap float64, r int, cols bool) (BreakTable, bool) {
	t := breakDP(asp, lo, hi, box, gap, r, cols)
	if t == nil {
		return BreakTable{}, false
	}
	return BreakTable{M: t.m, From: t.from}, true
}

var (
	resetMu    sync.Mutex
	resetHooks []func()
)

// OnReset — подписка на ResetCache: кэши пакетов-соседей (матрица группировки plan)
// сбрасываются вместе с общим, как до выноса plan из layout.
func OnReset(f func()) {
	resetMu.Lock()
	resetHooks = append(resetHooks, f)
	resetMu.Unlock()
}

func runResetHooks() {
	resetMu.Lock()
	hooks := resetHooks
	resetMu.Unlock()
	for _, f := range hooks {
		f()
	}
}

// Прежний путь расчёта окон группировки (до ускорения 027): быстрая матрица plan обязана
// совпадать с ним до бита. Здесь, а не в plan, потому что опирается на неэкспортные деревья и manyGen.

// RefQuickScore — прежний quickScore окна asp: деревья lineNodes, manyGen.pairs и spread.
func RefQuickScore(asp []float64, p Params) float64 {
	n := len(asp)
	m := newManyGen(asp, p)
	g := m.g
	rows := func(lo, hi int, box Rect) []*mblock {
		return m.fitAll(m.e.lineNodes(asp, lo, hi, box, 0, 1, false), lo, hi, box)
	}
	m.pairs(rows(0, n/2, g.pages[0]), rows(n/2, n, g.pages[1]))
	if p.Layflat {
		m.spread(rows(0, n, g.spread))
	}
	best := -1.0
	found := false
	for _, c := range m.cands {
		if !found || c.s > best {
			best, found = c.s, true
		}
	}
	return best
}

// RefFit — прежняя раскладка одного дерева рядов: высота блока, глубина, Rects от начала отрезка.
type RefFit struct {
	H     float64
	Depth int
	Ok    bool
	Rects []Rect
}

// RefRows — прежний путь рядов: для отрезка [s, s+m) деревья lineNodes(…, 0, 1, false),
// вписанные fitTree в box. Перечислитель один на набор asp (как у прежнего счёта).
func RefRows(asp []float64, gap float64) func(s, m int, box Rect) []RefFit {
	e := newEnum(asp, gap)
	return func(s, m int, box Rect) []RefFit {
		var out []RefFit
		for _, tr := range e.lineNodes(asp, s, s+m, box, 0, 1, false) {
			rects := make([]Rect, s+m) // fitTree пишет по номерам фото набора
			h, ok := fitTree(tr, box, gap, rects)
			out = append(out, RefFit{H: h, Depth: tr.depth, Ok: ok, Rects: rects[s:]})
		}
		return out
	}
}
