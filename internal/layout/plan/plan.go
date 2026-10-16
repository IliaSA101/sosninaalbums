package plan

// Группировка по числу разворотов (спека 027 §5): ровно N групп подряд, DP по префиксам
// над матрицей оценок окон (счёт — planfast.go). Матрица дороже DP в сотни раз,
// поэтому матрица кэшируется, а ползунок окна автосборки дёргает только DP.

import (
	"math"
	"slices"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Plan — разбиение фото на развороты: Groups — пары [начало, конец), Avg — фото на разворот.
type Plan struct {
	Groups   [][2]int
	Spreads  int
	Min, Max int
	Avg      float64
}

// planLambda — штраф за отход размера группы от среднего: без него DP дробит
// на неравные развороты ради мелких выигрышей оценки.
const planLambda = 0.12

// planWorkers — горутин на матрицу (как у прежнего Group).
const planWorkers = 8

// planKeep — сколько матриц помним: окно автосборки открывают на одном-двух наборах.
const planKeep = 4

// perSpread зажимает «не больше m фото на разворот» в 1…layout.MaxPhotos.
func perSpread(maxPer int) int { return max(1, min(maxPer, layout.MaxPhotos)) }

// SpreadRange — допустимое число разворотов для n фото при не больше maxPer на разворот:
// от ⌈n/m⌉ до n. Для n = 0 — (0, 0).
func SpreadRange(n, maxPer int) (lo, hi int) {
	if n <= 0 {
		return 0, 0
	}
	m := perSpread(maxPer)
	return (n + m - 1) / m, n
}

// planWidth — самая большая группа, которую рассматриваем при N разворотах: вдвое больше
// среднего с запасом. Не меньше ⌈n/N⌉, так что ровно N групп всегда достижимо.
func planWidth(n, spreads, maxPer int) int {
	return min(perSpread(maxPer), 2*((n+spreads-1)/spreads)+2)
}

// PlanGroups делит фото (по порядку) ровно на spreads разворотов (зажимается в SpreadRange),
// в каждом не больше maxPer фото. Ищет лучшую сумму оценок раскладок с весом по числу фото
// и штрафом за отход от среднего. Один и тот же вход — один и тот же план: подсказка окна
// и сама сборка совпадают.
func PlanGroups(aspects []float64, p layout.Params, spreads, maxPer int) Plan {
	n := len(aspects)
	if n == 0 {
		return Plan{}
	}
	lo, hi := SpreadRange(n, maxPer)
	spreads = max(lo, min(spreads, hi))
	k := planWidth(n, spreads, maxPer)
	// Сначала без групп меньше половины среднего: на больших альбомах это почти все окна
	// до layout.MaxFullTree, самые дорогие. Вместо них — верхняя граница; если лучший путь всё
	// равно прошёл через такую группу, досчитываются и они (план тот же, см. planDP).
	lo, ub := 1, planBound(aspects, p)
	if !math.IsInf(ub, 1) {
		lo = max(1, int(float64(n)/float64(spreads)/2))
	}
	groups, ok := planDP(planMatrix(aspects, p, lo, k), n, spreads, k, ub)
	if !ok {
		groups, _ = planDP(planMatrix(aspects, p, 1, k), n, spreads, k, ub)
	}
	return planOf(groups, n)
}

// planOf — план по готовым группам: Min, Max, Avg.
func planOf(groups [][2]int, n int) Plan {
	pl := Plan{Groups: groups, Spreads: len(groups), Min: n, Avg: float64(n) / float64(len(groups))}
	for _, g := range groups {
		pl.Min = min(pl.Min, g[1]-g[0])
		pl.Max = max(pl.Max, g[1]-g[0])
	}
	return pl
}

// PlanOpts — группировка с разбросом (спека 051 §3): число фото на разворотах нарочно
// разное в пределах «от … до …», граница разворота тянется к границе серии съёмки.
type PlanOpts struct {
	Spreads        int       // ровно N (зажим — RangeSpreads)
	MinPer, MaxPer int       // k ∈ [MinPer, MaxPer]; MinPer 0 — прежний PlanGroups побитово
	Cuts           []float64 // len n+1 или nil: сила границы серии перед фото j, 0…1
}

// Числа подобраны по TestPlanRangeMetrics (-v); правка — записью в decisions 0.13.
const (
	// rangeLambda — штраф за отход группы от цели волны при среднем 9 фото; на других
	// средних масштаб 9/avg: отход в фото растёт вместе со средним, волна не должна гаснуть.
	rangeLambda = 0.12
	// rangeBeta — награда за разрез на границе серии силы 1: тянет разрез на несколько фото.
	// Спека начинала с 3; 4,5 на синтетике 600/50 — 25–29 границ серий из 36–41 вместо 23
	// при том же разбросе (6 уже убавляет скачки).
	rangeBeta = 4.5
)

// rangeWave — доли волны целей по кругу: минус — к «от», плюс — к «до». Первый разворот
// редкий (как у SmartAlbums), соседи резко разные, период 8 — без заметного повтора.
var rangeWave = [...]float64{-1, 0.8, -0.4, 1, -0.8, 0.4, -0.2, 0}

// perRange — «от» и «до» в 1…layout.MaxPhotos, «от» не больше «до» и не больше n
// (фото меньше, чем «от», — один разворот из всех).
func perRange(n, minPer, maxPer int) (a, b int) {
	b = perSpread(maxPer)
	a = max(1, min(minPer, b, n))
	return a, b
}

// RangeSpreads — допустимое число разворотов для n фото по «от» minPer до «до» maxPer:
// [⌈n/до⌉, ⌊n/от⌋]. Если n не делится так ни при каком N (10 фото по 6–7), верх = низу:
// «от» тогда уступает (PlanRange). Для n = 0 — (0, 0).
func RangeSpreads(n, minPer, maxPer int) (lo, hi int) {
	if n <= 0 {
		return 0, 0
	}
	a, b := perRange(n, minPer, maxPer)
	lo = (n + b - 1) / b
	return lo, max(lo, n/a)
}

// RangePer — действующие «от» и «до» для n фото на spreads разворотах (spreads уже в
// RangeSpreads): зажим perRange, и «от» не выше n/spreads — N·a ≤ n не всегда (см.
// RangeSpreads), тогда «от» уступает. Подсказка окна (AutoPlan.From/To) — отсюда же.
func RangePer(n, spreads, minPer, maxPer int) (a, b int) {
	a, b = perRange(n, minPer, maxPer)
	return max(1, min(a, n/max(1, spreads))), b
}

// AutoPer — «Авто»: диапазон от среднего n/N (N зажат в SpreadRange(n, layout.MaxPhotos)):
// от round(ср./3) до round(1,75·ср.), но не уже трёх значений. 100 фото на 11 — 3…16.
func AutoPer(n, spreads int) (minPer, maxPer int) {
	if n <= 0 {
		return 0, 0
	}
	lo, hi := SpreadRange(n, layout.MaxPhotos)
	avg := float64(n) / float64(max(lo, min(spreads, hi)))
	minPer = max(1, int(math.Round(avg/3)))
	return minPer, min(layout.MaxPhotos, max(minPer+2, int(math.Round(1.75*avg))))
}

// PlanRange делит фото (по порядку) ровно на o.Spreads разворотов (зажим — RangeSpreads),
// в каждом от MinPer до MaxPer фото. DP как у PlanGroups, но размер группы тянется не к
// среднему, а к цели волны (rangeTargets), и разрез на границе серии (o.Cuts) награждается.
// MinPer ≤ 0 — прежний PlanGroups (Cuts не действуют). Один вход — один план.
func PlanRange(aspects []float64, p layout.Params, o PlanOpts) Plan {
	if o.MinPer <= 0 {
		return PlanGroups(aspects, p, o.Spreads, o.MaxPer)
	}
	n := len(aspects)
	if n == 0 {
		return Plan{}
	}
	lo, hi := RangeSpreads(n, o.MinPer, o.MaxPer)
	N := max(lo, min(o.Spreads, hi))
	a, b := RangePer(n, N, o.MinPer, o.MaxPer)
	cols := planMatrix(aspects, p, a, b)
	return planOf(rangeDP(cols, n, N, a, b, rangeTargets(n, N, a, b), rangeCuts(o.Cuts, n)), n)
}

// rangeTargets — цели размеров групп: среднее, сдвинутое волной к «от» или «до» на долю
// rangeWave, затем нормировка на сумму n. Узкий диапазон сжимает волну — почти ровно.
func rangeTargets(n, N, a, b int) []float64 {
	avg := float64(n) / float64(N)
	t := make([]float64, N)
	sum := 0.0
	for c := range t {
		w := rangeWave[c%len(rangeWave)]
		if w >= 0 {
			t[c] = avg + w*(float64(b)-avg)
		} else {
			t[c] = avg + w*(avg-float64(a))
		}
		sum += t[c]
	}
	for c := range t {
		t[c] *= float64(n) / sum
	}
	return t
}

// rangeCuts — силы границ в 0…1 (NaN — 0); длина не n+1 — серий нет.
func rangeCuts(cuts []float64, n int) []float64 {
	if len(cuts) != n+1 {
		return nil
	}
	out := make([]float64, n+1)
	for j, v := range cuts {
		if v > 0 {
			out[j] = min(v, 1)
		}
	}
	return out
}

// rangeBad — вместо оценки окна NaN/±∞ (испорченные пропорции): путь через такое окно
// остаётся (план есть всегда, каждая клетка полосы достижима), но хуже любого настоящего.
const rangeBad = -1e9

// rangeDP — ровно N групп размером a…b: best[c][j] = max_k best[c−1][j−k] + cols[k][j−k]
// − λs·(9/avg)·(k − t_c)² + β·cuts[j] (у последней группы разрез — конец ленты, без
// награды). Ничьи — меньшая группа (k по возрастанию, строгое >), как planDP. Нужно
// N·a ≤ n ≤ N·b — тогда достижима каждая клетка полосы [lo, hi] строки.
func rangeDP(cols [][]float64, n, N, a, b int, t, cuts []float64) [][2]int {
	lam := rangeLambda * 9 / (float64(n) / float64(N))
	inf := math.Inf(-1)
	prev, cur := make([]float64, n+1), make([]float64, n+1)
	plo, phi := 0, 0
	from := make([]uint8, N*(n+1)) // b ≤ layout.MaxPhotos
	for c := 1; c <= N; c++ {
		lo, hi := max(c*a, n-(N-c)*b), min(n-(N-c)*a, c*b)
		for j := lo; j <= hi; j++ {
			cur[j] = inf
		}
		for k := a; k <= b; k++ {
			j0, j1 := max(lo, plo+k), min(hi, phi+k)
			if j0 > j1 || k < 0 || k > math.MaxUint8 { // k ≤ layout.MaxPhotos (perRange); проверка — для uint8
				continue
			}
			d := float64(k) - t[c-1]
			pk := lam * d * d
			rangeRelax(cur[j0:j1+1], prev[j0-k:], cols[k][j0-k:], from[(c-1)*(n+1)+j0:], uint8(k), pk)
		}
		if cuts != nil && c < N {
			for j := lo; j <= hi; j++ {
				cur[j] += rangeBeta * cuts[j]
			}
		}
		prev, cur = cur, prev
		plo, phi = lo, hi
	}
	groups := make([][2]int, N)
	for c, j := N, n; c > 0; c-- {
		k := int(from[(c-1)*(n+1)+j])
		groups[c-1] = [2]int{j - k, j}
		j -= k
	}
	return groups
}

// rangeRelax — клетки dst строки через группу из kk фото (как planRelax; окно NaN/±∞ —
// rangeBad).
func rangeRelax(dst, src, col []float64, row []uint8, kk uint8, pk float64) {
	src, col, row = src[:len(dst)], col[:len(dst)], row[:len(dst)]
	for x, pv := range src {
		w := col[x]
		if math.IsNaN(w) || math.IsInf(w, 0) {
			w = rangeBad
		}
		if v := pv + w - pk; v > dst[x] {
			dst[x], row[x] = v, kk
		}
	}
}

// planDP — ровно N групп размером 1…K: best[c][j] = max_k best[c−1][j−k] + val − λ·(k − n/N)².
// cols[k][i] — оценка окна [i, i+k). При равенстве берётся меньшая группа (строгое >).
//
// cols[k] = nil — столбец не посчитан, вместо окна k·ub (ub не меньше любой оценки окна).
// Если лучший путь обошёлся без таких столбцов, он тот же, что при всех посчитанных:
// сложение и max монотонны, так что при настоящих столбцах каждая клетка не больше, чем
// при границе, а клетки пути те же; в клетке пути при границе меньшие k строго хуже,
// большие — не лучше, при настоящих тем более. Иначе (или при неконечных значениях,
// где монотонность ломается) — false: нужен счёт со всеми столбцами.
func planDP(cols [][]float64, n, N, K int, ub float64) ([][2]int, bool) {
	virtual := false
	for k := 1; k <= K; k++ {
		virtual = virtual || cols[k] == nil
	}
	if virtual && !planFinite(cols[1:K+1]) {
		return nil, false
	}
	avg := float64(n) / float64(N)
	inf := math.Inf(-1)
	pen := make([]float64, K+1)
	for k := 1; k <= K; k++ {
		d := float64(k) - avg
		pen[k] = planLambda * d * d
	}
	prev, cur := make([]float64, n+1), make([]float64, n+1)
	prev[0] = 0
	plo, phi := 0, 0               // клетки предыдущей строки; вне их пути нет (−∞)
	from := make([]uint8, N*(n+1)) // размер последней группы; K ≤ layout.MaxPhotos
	for c := 1; c <= N; c++ {
		// Остальным N−c группам нужно от 1 до K фото каждой: клетки вне [lo, hi] в путь
		// до (N, n) не входят и читают только такие же.
		lo, hi := max(c, n-(N-c)*K), min(n-(N-c), c*K)
		for j := lo; j <= hi; j++ {
			cur[j] = inf
		}
		// k снаружи, но у каждой клетки k по-прежнему по возрастанию — при равенстве
		// остаётся меньшая группа. −∞ из prev даёт −∞ (или NaN), строгое > его не берёт.
		for k := 1; k <= K; k++ {
			if j0, j1 := max(lo, plo+k), min(hi, phi+k); j0 <= j1 {
				var col []float64
				if cols[k] != nil {
					col = cols[k][j0-k:]
				}
				planRelax(cur[j0:j1+1], prev[j0-k:], col, from[(c-1)*(n+1)+j0:], uint8(k), pen[k], float64(k)*ub)
			}
		}
		prev, cur = cur, prev
		plo, phi = lo, hi
	}
	groups := make([][2]int, N)
	for c, j := N, n; c > 0; c-- {
		k := int(from[(c-1)*(n+1)+j])
		if virtual && cols[k] == nil { // cols[0] тоже nil: пути нет
			return nil, false
		}
		groups[c-1] = [2]int{j - k, j}
		j -= k
	}
	return groups, true
}

// planRelax — клетки dst строки через группу из kk фото: src и col — с тех же сдвигов,
// col = nil — вместо окна граница bound = k·ub.
func planRelax(dst, src, col []float64, row []uint8, kk uint8, pk, bound float64) {
	src, row = src[:len(dst)], row[:len(dst)]
	if col == nil {
		for x, pv := range src {
			if v := pv + bound - pk; v > dst[x] {
				dst[x], row[x] = v, kk
			}
		}
		return
	}
	col = col[:len(dst)]
	for x, pv := range src {
		if v := pv + col[x] - pk; v > dst[x] {
			dst[x], row[x] = v, kk
		}
	}
}

// planFinite — все посчитанные оценки конечны.
func planFinite(cols [][]float64) bool {
	for _, col := range cols {
		for _, v := range col {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
		}
	}
	return true
}

// planBound — верхняя граница оценки окна (score и pairScore): фото внутри разворота без
// полей занимают не больше его площади, «в край» (одно-два фото) — не больше площади
// всего разворота при штрафе bleedPenalty ≥ 0.1; остальные слагаемые только вычитают.
// Запас — на погрешность сумм. +∞ — граница не годится (пропорции или поля не те).
func planBound(aspects []float64, p layout.Params) float64 {
	for _, a := range aspects {
		if !(a > 0) || math.IsInf(a, 0) {
			return math.Inf(1)
		}
	}
	g := layout.NewGeom(p)
	area := g.Spread.W * g.Spread.H
	if !(g.Spread.W > 0 && g.Spread.H > 0) || math.IsInf(area, 0) {
		return math.Inf(1)
	}
	return max(1, g.Full.W*g.Full.H/area-0.1) * (1 + 1e-6)
}

// planEntry — матрица одного набора: cols[k] — окна по k фото, nil — ещё не считали.
// mu держится на время досчёта: второй запрос того же набора ждёт и не считает то же самое.
type planEntry struct {
	mu   sync.Mutex
	cols [layout.MaxPhotos + 1][]float64
}

var plans = struct {
	sync.Mutex
	keys []string // от давних к недавним
	m    map[string]*planEntry
}{m: map[string]*planEntry{}}

// planMatrix — cols[k][i] = k · оценка окна [i, i+k) для from ≤ k ≤ K (ниже from — что уже
// есть, иначе nil). Недостающие столбцы досчитываются (ползунок ушёл к меньшему N — группы
// крупнее), посчитанные не меняются.
func planMatrix(aspects []float64, p layout.Params, from, K int) [][]float64 {
	key := "G" + layout.CacheKey(aspects, p, 0)
	plans.Lock()
	e, ok := plans.m[key]
	if !ok {
		e = &planEntry{}
		plans.m[key] = e
	}
	for i, k := range plans.keys {
		if k == key {
			plans.keys = append(plans.keys[:i], plans.keys[i+1:]...)
			break
		}
	}
	plans.keys = append(plans.keys, key)
	if len(plans.keys) > planKeep {
		delete(plans.m, plans.keys[0])
		plans.keys = plans.keys[1:]
	}
	plans.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	for k := from; k <= K; k++ {
		if e.cols[k] != nil {
			continue
		}
		to := k
		for to < K && e.cols[to+1] == nil {
			to++
		}
		// Новые столбцы ставятся целиком после счёта: при панике запись прежняя.
		for x, col := range windowCols(aspects, p, k, to) {
			e.cols[k+x] = col
		}
		k = to
	}
	return slices.Clone(e.cols[:K+1])
}

// Матрицы сбрасываются вместе с общим кэшем: layout.ResetCache зовёт resetPlans.
func init() { layout.OnReset(resetPlans) }

// resetPlans — для бенчмарков и тестов на холодном кэше.
func resetPlans() {
	plans.Lock()
	plans.keys, plans.m = nil, map[string]*planEntry{}
	plans.Unlock()
}
