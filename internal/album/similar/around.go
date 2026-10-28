package similar

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// Around — похожие на кадр at соседи для окна сравнения (138): те же правила пары, что у Drops
// (одна серия, MaxGap, одна ориентация, окно ±Window), порог «Похожие» (DistSimilar).
// Возвращает индексы: at и до limit−1 ближайших по Dist (ничья — ближе по ленте, затем раньше),
// отсортированные по ленте; more — сколько похожих не вошло. Похожих нет — только at. Кадр без
// подписи, свой (Lib), at вне входа или limit < 1 — nil. Цепочек нет: сравнивается только с at.
func Around(infos []model.PhotoInfo, at, limit int) (idx []int, more int) {
	if at < 0 || at >= len(infos) || limit < 1 || !infos[at].Sig.Ok || infos[at].Lib != "" {
		return nil, 0
	}
	walls := wallCount(infos)
	type cand struct{ j, d int }
	var cs []cand
	for j := max(0, at-Window); j <= min(len(infos)-1, at+Window); j++ {
		if j == at || !infos[j].Sig.Ok || infos[j].Lib != "" || !canPair(infos, walls, Marks{}, at, j) {
			continue
		}
		if d := imgsig.Dist(infos[at].Sig, infos[j].Sig); d <= imgsig.DistSimilar {
			cs = append(cs, cand{j, d})
		}
	}
	slices.SortFunc(cs, func(a, b cand) int {
		if a.d != b.d {
			return a.d - b.d
		}
		if da, db := absInt(a.j-at), absInt(b.j-at); da != db {
			return da - db
		}
		return a.j - b.j
	})
	keep := min(len(cs), limit-1)
	idx = []int{at}
	for _, c := range cs[:keep] {
		idx = append(idx, c.j)
	}
	slices.Sort(idx)
	return idx, len(cs) - keep
}

func absInt(x int) int { return max(x, -x) }
