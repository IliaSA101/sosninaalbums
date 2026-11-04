package forms

// Главное фото раскладки по геометрии (переехало из variety: variety зовёт его отсюда, иначе
// цикл forms ↔ variety, design §5) и сравнение в квантах.

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

const (
	// HeroRatio — во сколько раз фото больше второго по площади, чтобы стать главным.
	HeroRatio = 1.6
	// HeroMinPhotos — главное фото бывает от трёх фото (у пары «главное» — просто неровная пара).
	HeroMinPhotos = 3
	// PairMinPhotos — две крупных (оба ≥ 1.6× третьего) — от четырёх фото: «две крупных + лента».
	PairMinPhotos = 4
)

// tieEps — как в layout и variety: пороги сравниваются в квантах, чтобы FMA на arm64 не менял
// решение у границы.
const tieEps = 1e-9

func quanta(x float64) float64 { return math.Round(x / tieEps) }

// HeroInfo — главные фото раскладки по геометрии.
type HeroInfo struct {
	N   int        // 0, 1 или 2 (пара крупных)
	Idx [2]int     // индексы двух наибольших по площади (ничья — раньше), −1 — нет
	Top [3]float64 // три наибольшие площади по убыванию
}

// Heroes — главное: от трёх фото, ≥ 1.6× второго; иначе пара — от четырёх фото, обе ≥ 1.6×
// третьего («две крупных + лента»).
func Heroes(rs []layout.Rect) HeroInfo {
	h := HeroInfo{Idx: [2]int{-1, -1}}
	for i, x := range rs {
		switch s := x.W * x.H; {
		case s > h.Top[0]:
			h.Top, h.Idx = [3]float64{s, h.Top[0], h.Top[1]}, [2]int{i, h.Idx[0]}
		case s > h.Top[1]:
			h.Top[1], h.Top[2], h.Idx[1] = s, h.Top[1], i
		case s > h.Top[2]:
			h.Top[2] = s
		}
	}
	switch k := len(rs); {
	case k >= HeroMinPhotos && h.Top[0] >= HeroRatio*h.Top[1]:
		h.N = 1
	case k >= PairMinPhotos && h.Top[1] >= HeroRatio*h.Top[2]:
		h.N = 2
	}
	return h
}

// isHero — фото i — главное (у одного главного второй индекс не в счёт).
func (h HeroInfo) isHero(i int) bool {
	return h.N >= 1 && h.Idx[0] == i || h.N == 2 && h.Idx[1] == i
}
