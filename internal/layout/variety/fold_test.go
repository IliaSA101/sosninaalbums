package variety

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// foldPaths — всё, что подбор показывает или выбирает для разворота as: ряд layout («Авто» и
// «С перестановкой»), формы Gen, featured, ряд «Авто» по формам (Row), пул подборки (Pool:
// ряд, воздух, featured «Других вариантов») и «Другие варианты» (Extra).
func foldPaths(as []float64, p layout.Params) map[string][]layout.Result {
	n := len(as)
	out := map[string][]layout.Result{"layout": layout.Cached(as, p, rowLen(n)), "row": RowResults(as, p, rowLen(n)),
		"extra": Extra(as, p, rowLen(n))}
	out["rearranged"], _ = layout.CachedRearranged(as, p, rowLen(n))
	for _, c := range forms.Gen(as, p) {
		out["forms"] = append(out["forms"], c.Result)
	}
	for _, f := range featured(as, p) {
		out["featured"] = append(out["featured"], f.Result)
	}
	for _, c := range Pool(as, p, rowLen(n), More) {
		out["pool"] = append(out["pool"], c.Result)
	}
	return out
}

// TestFoldRuleAllPaths — сторож правила сгиба во всех путях подбора, «владелец 2026-10-17»
// (правило одно для всех: формы, в т. ч. мозаика и виды с главным, featured, ряд «Авто»,
// «Другие варианты», пул подборки). На 2–40 фото четырёх геометрий при «только широкие» каждый
// результат проходит layout.FoldOK по кадрам, без layflat ни одно фото не пересекает корешок;
// подборка корпуса «широких» альбомов — тоже. Чтобы сторож не был пустым, при «любые фото» на
// том же корпусе есть результаты форм и ряда вне правила — их и отсёк фильтр.
func TestFoldRuleAllPaths(t *testing.T) {
	// 30×30, 30×20, книжная 20×30 с промежутком 0 (разворот 4:3), 40×30.
	geoms := []layout.Params{geomSq, geomWide, {PageW: 200, PageH: 300, Margin: 5, Layflat: true, FoldAny: true},
		{PageW: 400, PageH: 300, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}}
	ns := []int{2, 3, 4, 5, 6, 7, 8, 10, 12, 16, 17, 24, 40}
	if testkit.Lite() {
		ns = []int{3, 4, 5, 8, 17}
	}
	var outside atomic.Int64 // результатов форм и ряда при «любые» вне правила «широкие»
	var wg sync.WaitGroup
	for gi, pAny := range geoms {
		for _, n := range ns {
			sets := map[string][]float64{"верт": sameAspects(n, aspP), "смесь": mixedAspects(n, uint64(n)),
				"серия": seriesAspects(n, 0.65, uint64(gi+n))}
			for name, as := range sets {
				wg.Go(func() {
					outside.Add(int64(checkFoldSet(t, fmt.Sprintf("геометрия %d, %s %d", gi, name, n), as, pAny)))
				})
			}
		}
	}
	wg.Wait()
	if outside.Load() == 0 {
		t.Error("при «любые» нет ни одного результата вне правила: сторож ничего не проверяет")
	}
	if !testkit.Lite() {
		checkFoldCorpus(t, corpusWide())
	}
}

// checkFoldSet — шаг TestFoldRuleAllPaths на одном наборе: при «только широкие» (wideFold(pAny))
// каждый путь в правиле и ряд «Авто» не пуст, без layflat — ничего через корешок. Возвращает,
// сколько результатов форм и ряда при «любые» (pAny) вне правила «широкие».
func checkFoldSet(t *testing.T, where string, as []float64, pAny layout.Params) int {
	t.Helper()
	wide, none := wideFold(pAny), pAny
	none.Layflat, none.FoldAny = false, false
	for path, rs := range foldPaths(as, wide) {
		if path == "row" && len(rs) == 0 {
			t.Errorf("%s: ряд «Авто» пуст", where)
		}
		for _, r := range rs {
			if !layout.FoldOK(r.Rects, as, wide) {
				t.Errorf("%s, %s: %s — фото на сгибе вне правила «широкие»", where, path, r.Key)
			}
		}
	}
	for path, rs := range foldPaths(as, none) {
		for _, r := range rs {
			if !layout.FoldOK(r.Rects, nil, none) {
				t.Errorf("%s, %s: %s — фото через корешок без layflat", where, path, r.Key)
			}
		}
	}
	outside := 0
	all := foldPaths(as, pAny)
	for _, r := range slices.Concat(all["forms"], all["row"]) {
		if !layout.FoldOK(r.Rects, as, wide) {
			outside++
		}
	}
	return outside
}

// checkFoldCorpus — пул подборки каждого разворота альбомов в правиле их геометрии.
func checkFoldCorpus(t *testing.T, albums []corpusAlbum) {
	t.Helper()
	for _, a := range albums {
		for i, s := range a.spreads {
			for _, c := range s.Pool {
				if !layout.FoldOK(c.Rects, a.asps[i], a.p) {
					t.Errorf("%s, разворот %d: в пуле %s вне правила «широкие»", a.name, i, c.Key)
				}
			}
		}
	}
}

// TestFoldAnyLadderOnSpread — три вертикальных 2:3 на 30×30, «владелец 2026-10-17»: при «любые
// фото» в ряду «Вариантов» есть лесенка на весь разворот L(0,1,2) и столбец по сгибу V(0,1,2)
// (до 0.15 формы отрезали их своим полом корешка); при «только широкие» обоих нет, а «Авто»
// — постраничная раскладка (прежняя H(0,1,2) режет сгибом портреты).
func TestFoldAnyLadderOnSpread(t *testing.T) {
	ppp := sameAspects(3, aspP)
	keys := func(p layout.Params) []string {
		var out []string
		for _, r := range RowResults(ppp, p, rowLen(3)) {
			out = append(out, r.Key)
		}
		for _, r := range Extra(ppp, p, rowLen(3)) {
			out = append(out, r.Key)
		}
		return out
	}
	anyKeys, wideKeys := keys(geomSq), keys(wideFold(geomSq))
	for _, k := range []string{"L(0,1,2)", "V(0,1,2)"} {
		if !slices.Contains(anyKeys, k) {
			t.Errorf("«любые»: %s нет в «Вариантах» %v", k, anyKeys)
		}
		if slices.Contains(wideKeys, k) {
			t.Errorf("«только широкие»: %s в «Вариантах»", k)
		}
	}
	if anyKeys[0] != "H(0,1,2)" {
		t.Errorf("«любые»: «Авто» %s, ждали H(0,1,2) — как до 0.15", anyKeys[0])
	}
	if len(wideKeys) == 0 || !layout.IsPaged(wideKeys[0]) {
		t.Errorf("«только широкие»: «Авто» %v, ждали постраничную", wideKeys[:min(1, len(wideKeys))])
	}
}
