package similar

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/series"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// Что сравнивается (решение 078-3): кадры, стоящие рядом в порядке ленты, внутри одной серии
// съёмки и не слишком далеко по времени; всё остальное — разные моменты, даже если похожи.
const (
	// Window — на сколько позиций ленты друг от друга сравниваются кадры.
	Window = 12
	// MaxGap — не дальше по времени съёмки, секунды (если время есть у обоих).
	MaxGap = 600
	// wallCut — сила границы серии (series.Cuts), через которую не сравнивают.
	wallCut = 0.5
	// starKeep — кадры с такой оценкой и выше не убираются никогда (решение 078-5).
	starKeep = 4
	// sharpQ — резкость сравнивается с такой точностью: пустяки не решают, кого оставить.
	sharpQ = 1e3
)

// thresholds — порог imgsig.Dist по уровню: 0 — выключено, 1 «Дубли», 2 «Очень похожие», 3 «Похожие».
var thresholds = [...]int{0, imgsig.DistDup, imgsig.DistNear, imgsig.DistSimilar}

// MaxLevel — старший уровень отбора.
const MaxLevel = len(thresholds) - 1

// Drop — кадр, который автосборка не берёт: I — индекс во входе, Like — индекс оставленного кадра,
// на который он похож, Dist — расстояние между подписями.
type Drop struct{ I, Like, Dist int }

// Marks — что известно о кадрах кроме самих подписей; поля — по индексам входа, короткие или
// пустые (nil) читаются как «нет».
type Marks struct {
	Rate    []int     // действующие звёзды 0…5
	Pick    []bool    // флажок «беру»
	InAlbum []bool    // уже стоит в альбоме (выбор человека при сборке из выделенных)
	Asp     []float64 // действующие пропорции (с поворотом); нет — по W/H фото
}

func (m Marks) rate(i int) int {
	if i < len(m.Rate) {
		return m.Rate[i]
	}
	return 0
}

func (m Marks) pick(i int) bool    { return i < len(m.Pick) && m.Pick[i] }
func (m Marks) inAlbum(i int) bool { return i < len(m.InAlbum) && m.InAlbum[i] }

// wide — кадр шире высоты по действующим пропорциям.
func (m Marks) wide(infos []model.PhotoInfo, i int) bool {
	if i < len(m.Asp) && m.Asp[i] > 0 {
		return m.Asp[i] > 1
	}
	return infos[i].W > infos[i].H
}

// Drops — кадры, которые не берём, по порядку ленты. Кадры идут от лучшего к худшему (лучшего
// не убрать никогда): флажок «беру» → больше звёзд → уже стоит в альбоме → резче → раньше. Кадр
// с флажком и с оценкой ≥ 4 остаётся всегда; остальной уходит, если среди уже оставленных рядом
// есть кадр с Dist ≤ порога уровня (из таких — ближайший, при ничьей — раньше). Убранные не
// сравниваются: цепочек A ~ B ~ C нет — если A ≁ C, уходит один B. Кадр без подписи
// (Sig.Ok = false), свой из библиотеки (Lib) ни с чем не сравнивается и не убирается.
// Возвращённые пользователем (keep по ID) просто вычёркиваются из списка и в отборе не участвуют
// (решение 078-19): иначе возвращённый становился бы образцом и выбивал прежде взятые — «Вернуть N»
// давало бы не +N фото. lv вне 1…3 зажимается; 0 и меньше — отбора нет (nil).
func Drops(infos []model.PhotoInfo, lv int, keep map[string]bool, m Marks) []Drop {
	if lv <= 0 || len(infos) < 2 {
		return nil
	}
	T := thresholds[min(lv, MaxLevel)]
	n := len(infos)
	walls := wallCount(infos)
	ok := make([]bool, n)
	var order []int
	for i, ph := range infos {
		if ok[i] = ph.Sig.Ok && ph.Lib == ""; ok[i] {
			order = append(order, i)
		}
	}
	slices.SortFunc(order, func(a, b int) int { return quality(infos, m, a, b) })

	kept := make([]bool, n)
	var out []Drop
	for _, i := range order {
		if m.pick(i) || m.rate(i) >= starKeep {
			kept[i] = true
			continue
		}
		at, atD := -1, 0
		for j := max(0, i-Window); j <= min(n-1, i+Window); j++ {
			if j == i || !kept[j] || !canPair(infos, walls, m, i, j) {
				continue
			}
			if d := imgsig.Dist(infos[i].Sig, infos[j].Sig); d <= T && (at < 0 || d < atD || d == atD && j < at) {
				at, atD = j, d
			}
		}
		if at < 0 {
			kept[i] = true
			continue
		}
		if !keep[infos[i].ID] {
			out = append(out, Drop{I: i, Like: at, Dist: atD})
		}
	}
	slices.SortFunc(out, func(a, b Drop) int { return a.I - b.I })
	return out
}

// quality — порядок «лучший раньше» (меньше нуля — a лучше b).
func quality(infos []model.PhotoInfo, m Marks, a, b int) int {
	if pa, pb := m.pick(a), m.pick(b); pa != pb {
		return flip(pa)
	}
	if ra, rb := m.rate(a), m.rate(b); ra != rb {
		return rb - ra
	}
	if ia, ib := m.inAlbum(a), m.inAlbum(b); ia != ib {
		return flip(ia)
	}
	if sa, sb := sharpOf(infos[a]), sharpOf(infos[b]); sa != sb {
		if sa > sb {
			return -1
		}
		return 1
	}
	return a - b
}

// flip — a лучше b, если у a признак есть.
func flip(aHas bool) int {
	if aHas {
		return -1
	}
	return 1
}

// sharpOf — резкость, квантованная до 1e-3 (NaN и минус — 0).
func sharpOf(ph model.PhotoInfo) int64 {
	s := float64(ph.Sig.Sharp)
	if math.IsNaN(s) || s <= 0 {
		return 0
	}
	return int64(math.Round(math.Min(s, 1e12) * sharpQ))
}

// wallCount — walls[k] = число сильных границ серии перед фото 1…k: между i < j стена есть,
// если walls[j] > walls[i] (разрез j стоит перед фото j). nil — серий нет.
func wallCount(infos []model.PhotoInfo) []int {
	cuts := series.Cuts(infos)
	if cuts == nil {
		return nil
	}
	w := make([]int, len(infos))
	for k := 1; k < len(w); k++ {
		w[k] = w[k-1]
		if cuts[k] >= wallCut {
			w[k]++
		}
	}
	return w
}

// canPair — пару кадров вообще можно сравнивать: одна серия, не дальше MaxGap по времени,
// одна ориентация. Подпись у обоих и окно по позициям проверяет вызывающий (оставленные — только с подписью).
func canPair(infos []model.PhotoInfo, walls []int, m Marks, i, j int) bool {
	lo, hi := min(i, j), max(i, j)
	if walls != nil && walls[hi] > walls[lo] {
		return false
	}
	if a, b := infos[i].Taken, infos[j].Taken; a > 0 && b > 0 && max(a-b, b-a) > MaxGap {
		return false
	}
	return m.wide(infos, i) == m.wide(infos, j)
}
