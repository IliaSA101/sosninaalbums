package plan

// Стены (спека 107 «Пересобрать весь альбом»): якорные развороты внутри плана. Стена —
// позиция w в 1…n−1 между фото w−1 и w; ни одна группа не пересекает её, то есть
// окно [j−k, j) допустимо, только если j−k ≥ последней стены < j. Без стен (nil, пусто,
// 0 и n) расчёт — прежние функции plan.go побитово; со стенами — их варианты с массивом caps.

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// rangeLow — штраф за группу меньше «от» со стенами: rangeLow·(a−k)². Мягкий предел: отрезок
// из 1–2 фото между якорями при «от 3» иначе неразрешим. Много больше любого выигрыша
// оценки раскладок (десятые доли на фото), так что «от» уступает только там, где иначе нельзя.
const rangeLow = 10.0

// normWalls — стены в 1…n−1 по возрастанию без повторов; 0, n и вне диапазона отбрасываются.
// Вход не меняется; пусто — nil.
func normWalls(walls []int, n int) []int {
	var out []int
	for _, w := range walls {
		if w > 0 && w < n {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// wallSegments — длины отрезков между стенами (walls уже normWalls, n фото).
func wallSegments(n int, walls []int) []int {
	segs := make([]int, 0, len(walls)+1)
	prev := 0
	for _, w := range walls {
		segs = append(segs, w-prev)
		prev = w
	}
	return append(segs, n-prev)
}

// ceilSum — Σ⌈nᵢ/k⌉ по отрезкам.
func ceilSum(segs []int, k int) int {
	s := 0
	for _, l := range segs {
		s += (l + k - 1) / k
	}
	return s
}

// wallCaps — caps[j] = j − (последняя стена < j; 0 — начало): самая большая группа,
// что кончается на j. O(n).
func wallCaps(walls []int, n int) []int {
	caps := make([]int, n+1)
	wall, next := 0, 0
	for j := 1; j <= n; j++ {
		for next < len(walls) && walls[next] < j {
			wall = walls[next]
			next++
		}
		caps[j] = j - wall
	}
	return caps
}

// SpreadRangeWalls — допустимое число разворотов для n фото при не больше maxPer на
// разворот и стенах: от Σ⌈nᵢ/m⌉ по отрезкам между стенами до n. Без стен — SpreadRange.
func SpreadRangeWalls(n, maxPer int, walls []int) (lo, hi int) {
	walls = normWalls(walls, n)
	if n <= 0 || len(walls) == 0 {
		return SpreadRange(n, maxPer)
	}
	return ceilSum(wallSegments(n, walls), perSpread(maxPer)), n
}

// RangeSpreadsWalls — допустимое число разворотов для n фото по «от» minPer до «до» maxPer
// со стенами: от Σ⌈nᵢ/до⌉ до Σ max(1, ⌊nᵢ/от⌋). «От» мягкое (rangeLow), поэтому верх не
// ниже низа, как в RangeSpreads. Без стен — RangeSpreads.
func RangeSpreadsWalls(n, minPer, maxPer int, walls []int) (lo, hi int) {
	walls = normWalls(walls, n)
	if n <= 0 || len(walls) == 0 {
		return RangeSpreads(n, minPer, maxPer)
	}
	a, b := perRange(n, minPer, maxPer)
	segs := wallSegments(n, walls)
	lo = ceilSum(segs, b)
	for _, l := range segs {
		hi += max(1, l/a)
	}
	return lo, max(lo, hi)
}

// AutoPerWalls — AutoPer со стенами: N зажат в SpreadRangeWalls(n, MaxPhotos), «до» не меньше
// наименьшего K', при котором Σ⌈nᵢ/K'⌉ ≤ N (иначе PlanRange поднимет N до нижнего предела
// по «до»: длинный отрезок рядом с короткими не делится на N групп). Без стен — AutoPer.
func AutoPerWalls(n, spreads int, walls []int) (minPer, maxPer int) {
	walls = normWalls(walls, n)
	if n <= 0 || len(walls) == 0 {
		return AutoPer(n, spreads)
	}
	lo, hi := SpreadRangeWalls(n, layout.MaxPhotos, walls)
	N := max(lo, min(spreads, hi))
	minPer, maxPer = AutoPer(n, N)
	segs := wallSegments(n, walls)
	for ; maxPer < layout.MaxPhotos; maxPer++ {
		if ceilSum(segs, maxPer) <= N {
			break
		}
	}
	return minPer, maxPer
}

// wallsWidth — ширина окна «ровно» со стенами: не меньше planWidth и не меньше наименьшего
// K' ≤ m, при котором Σ⌈nᵢ/K'⌉ ≤ N — иначе длинный отрезок рядом с короткими не делится
// на N групп. N в SpreadRangeWalls, значит K' = m подходит всегда.
func wallsWidth(n, N, maxPer int, walls []int) int {
	m := perSpread(maxPer)
	k := planWidth(n, N, maxPer)
	segs := wallSegments(n, walls)
	for ; k < m; k++ {
		if ceilSum(segs, k) <= N {
			break
		}
	}
	return k
}

// planRelaxCap — planRelax с проверкой стен: caps[x] — самая большая группа, что кончается
// в клетке x (срез wallCaps, выровненный с dst); длиннее — клетка не обновляется.
func planRelaxCap(dst, src, col []float64, row []uint8, kk uint8, pk, bound float64, caps []int) {
	src, row, caps = src[:len(dst)], row[:len(dst)], caps[:len(dst)]
	if col != nil {
		col = col[:len(dst)]
	}
	for x, pv := range src {
		if caps[x] < int(kk) {
			continue
		}
		w := bound
		if col != nil {
			w = col[x]
		}
		if v := pv + w - pk; v > dst[x] {
			dst[x], row[x] = v, kk
		}
	}
}

// rangeRelaxCap — rangeRelax с проверкой стен (caps — как у planRelaxCap).
func rangeRelaxCap(dst, src, col []float64, row []uint8, kk uint8, pk float64, caps []int) {
	src, col, row, caps = src[:len(dst)], col[:len(dst)], row[:len(dst)], caps[:len(dst)]
	for x, pv := range src {
		if caps[x] < int(kk) {
			continue
		}
		w := col[x]
		if math.IsNaN(w) || math.IsInf(w, 0) {
			w = rangeBad
		}
		if v := pv + w - pk; v > dst[x] {
			dst[x], row[x] = v, kk
		}
	}
}

// planRangeWalls — PlanRange со стенами (walls уже normWalls, не пусто, MinPer ≥ 1): группы
// 1…b, меньше «от» — со штрафом rangeLow. Цели волны и серии — по общему индексу группы.
func planRangeWalls(aspects []float64, p layout.Params, o PlanOpts, walls []int) Plan {
	n := len(aspects)
	lo, hi := RangeSpreadsWalls(n, o.MinPer, o.MaxPer, walls)
	N := max(lo, min(o.Spreads, hi))
	a, b := RangePer(n, N, o.MinPer, o.MaxPer)
	cols := planMatrix(aspects, p, 1, b)
	t := rangeTargetsPhase(n, N, a, b, o.Phase)
	return planOf(rangeDPWalls(cols, n, N, a, b, t, rangeCuts(o.Cuts, n), wallCaps(walls, n)), n)
}

// rangeDPWalls — rangeDP со стенами: ровно N групп размером 1…b, штраф за отход от цели
// волны как в rangeDP плюс rangeLow·(a−k)² при k < a. Клетки, куда со стенами не дойти,
// остаются −∞ (путь из (N, n) есть при N в RangeSpreadsWalls). Ничьи — меньшая группа.
func rangeDPWalls(cols [][]float64, n, N, a, b int, t, cuts []float64, caps []int) [][2]int {
	lam := rangeLambda * 9 / (float64(n) / float64(N))
	inf := math.Inf(-1)
	prev, cur := make([]float64, n+1), make([]float64, n+1)
	plo, phi := 0, 0
	from := make([]uint8, N*(n+1)) // b ≤ layout.MaxPhotos
	for c := 1; c <= N; c++ {
		lo, hi := max(c, n-(N-c)*b), min(n-(N-c), c*b)
		for j := lo; j <= hi; j++ {
			cur[j] = inf
		}
		for k := 1; k <= b; k++ {
			j0, j1 := max(lo, plo+k), min(hi, phi+k)
			if j0 > j1 || k > math.MaxUint8 { // b ≤ MaxPhotos; вторая граница — для gosec G115 (uint8 ниже)
				continue
			}
			d := float64(k) - t[c-1]
			pk := lam * d * d
			if k < a {
				pk += rangeLow * float64((a-k)*(a-k))
			}
			rangeRelaxCap(cur[j0:j1+1], prev[j0-k:], cols[k][j0-k:], from[(c-1)*(n+1)+j0:], uint8(k), pk, caps[j0:])
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
