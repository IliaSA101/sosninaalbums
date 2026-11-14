package colorspace

import (
	"math"
	"slices"
)

// Параметры «С фото» (спека 109-3). Порог 0,09 подобран по стоковым свадебным снимкам: при 0,06
// выдача была из 6–8 почти одинаковых серо-бежевых.
const (
	kClusters  = 16   // сколько кластеров ищем
	lloydIters = 10   // шагов Ллойда
	minPercent = 1    // кластер меньше 1 % точек не берём
	minDeltaE  = 0.09 // новый цвет отстоит от взятых не меньше (расстояние в OKLab)
)

// Dominant возвращает до limit главных цветов набора точек как "#rrggbb", по убыванию доли. Цвета
// различимы (ΔE ≥ 0,09) и занимают ≥ 1 % точек. Счёт детерминирован: k-means без случайности,
// на тех же точках ответ тот же. Нет точек или limit < 1 — пустой, не nil.
func Dominant(pts []Lab, limit int) []string {
	out := []string{}
	if len(pts) == 0 || limit < 1 {
		return out
	}
	cent := seedCenters(pts, min(kClusters, len(pts)))
	assign := make([]int, len(pts))
	for range lloydIters {
		assignAll(pts, cent, assign)
		moveCenters(pts, cent, assign)
	}
	counts := assignAll(pts, cent, assign)

	order := make([]int, len(cent))
	for i := range order {
		order[i] = i
	}
	// Равные доли — по номеру кластера, иначе порядок зависел бы от сортировки.
	slices.SortStableFunc(order, func(a, b int) int { return counts[b] - counts[a] })

	var taken []Lab
	for _, c := range order {
		if len(out) == limit {
			break
		}
		if counts[c]*100 < minPercent*len(pts) {
			break // дальше только мельче
		}
		if tooClose(cent[c], taken) {
			continue
		}
		taken = append(taken, cent[c])
		out = append(out, cent[c].Hex())
	}
	return out
}

// seedCenters — k стартовых центров: первый — среднее всех точек, каждый следующий — самая дальняя
// от уже выбранных точка (k-means++ без случайности).
func seedCenters(pts []Lab, k int) []Lab {
	var sl, sa, sb float64
	for _, p := range pts {
		sl += float64(p.L)
		sa += float64(p.A)
		sb += float64(p.B)
	}
	n := float64(len(pts))
	cent := make([]Lab, 1, k)
	cent[0] = Lab{L: float32(sl / n), A: float32(sa / n), B: float32(sb / n)}
	far := make([]float32, len(pts)) // расстояние до ближайшего выбранного
	for i := range far {
		far[i] = float32(math.Inf(1))
	}
	for len(cent) < k {
		last := cent[len(cent)-1]
		best, bi := float32(-1), 0
		for i, p := range pts {
			if d := dist2(p, last); d < far[i] {
				far[i] = d
			}
			if far[i] > best {
				best, bi = far[i], i
			}
		}
		cent = append(cent, pts[bi])
	}
	return cent
}

// assignAll относит каждую точку к ближайшему центру (при равенстве — к первому) и возвращает
// число точек в каждом кластере.
func assignAll(pts, cent []Lab, assign []int) []int {
	counts := make([]int, len(cent))
	for i, p := range pts {
		bj, bd := 0, float32(math.Inf(1))
		for j, q := range cent {
			if d := dist2(p, q); d < bd {
				bd, bj = d, j
			}
		}
		assign[i] = bj
		counts[bj]++
	}
	return counts
}

// moveCenters ставит центр в среднее его точек; пустой кластер остаётся на месте.
func moveCenters(pts, cent []Lab, assign []int) {
	type sum struct {
		l, a, b float64
		n       int
	}
	s := make([]sum, len(cent))
	for i, p := range pts {
		c := &s[assign[i]]
		c.l += float64(p.L)
		c.a += float64(p.A)
		c.b += float64(p.B)
		c.n++
	}
	for j, c := range s {
		if c.n > 0 {
			f := float64(c.n)
			cent[j] = Lab{L: float32(c.l / f), A: float32(c.a / f), B: float32(c.b / f)}
		}
	}
}

// tooClose — цвет ближе порога к одному из уже взятых.
func tooClose(c Lab, taken []Lab) bool {
	for _, t := range taken {
		if dist2(c, t) < minDeltaE*minDeltaE {
			return true
		}
	}
	return false
}
