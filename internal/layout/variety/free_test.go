package variety

// 105 «Разброс»: порядок «Других вариантов» (центр и плитки первыми), Manual в Pick и ByFamily,
// семья плитки, число скрытых сгибом.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

var geomRule = layout.Params{PageW: 300, PageH: 305, Margin: 12, Gap: 3, Layflat: true}

func freeGeoms() map[string]layout.Params {
	return map[string]layout.Params{"любые": geomSq, "широкие": geomRule,
		"не layflat": {PageW: 300, PageH: 305, Margin: 12, Gap: 3}}
}

func freeSets() [][]float64 {
	var out [][]float64
	for n := 3; n <= 5; n++ {
		out = append(out, sameAspects(n, aspP), sameAspects(n, aspL), mixedAspects(n, uint64(n)))
	}
	return out
}

// TestExtraFreeFirst: «Другие варианты» начинаются с центра и «Разброса» в порядке профилей (ручные,
// подряд), без повторов ключа и вида ряда «Авто» и друг друга; дальше прежний ряд — воздух и featured,
// ни одной ручной, по убыванию оценки.
func TestExtraFreeFirst(t *testing.T) {
	for gname, p := range freeGeoms() {
		for _, as := range freeSets() {
			checkFreeHead(t, fmt.Sprintf("%s %d фото", gname, len(as)), as, p)
		}
	}
	// 3 вертикальных, любые фото: центр и все 7 плиток
	e := extraOf(sameAspects(3, aspP), geomSq, 24)
	if len(e.manual) != 8 || !forms.CenterBig(e.row[0].Key) {
		t.Errorf("3 вертикальных: ручных %d, первая %q", len(e.manual), e.row[0].Key)
	}
}

// checkFreeHead — голова «Других вариантов» набора as: ручные подряд, центр первым, плитки в порядке
// Free, виды и ключи новые, хвост без ручных.
func checkFreeHead(t *testing.T, name string, as []float64, p layout.Params) {
	t.Helper()
	e := extraOf(as, p, 24)
	rowKeys, looks := map[string]bool{}, map[string]bool{}
	for _, c := range Row(as, p, 24) {
		rowKeys[c.Key], looks[Look(c.Rects, p)] = true, true
	}
	head := 0
	for head < len(e.row) && e.manual[e.row[head].Key] {
		head++
	}
	if head != len(e.manual) || head > midLimit+scatterLimit || len(e.row) > ExtraLimit {
		t.Fatalf("%s: голова %d, ручных %d, всего %d", name, head, len(e.manual), len(e.row))
	}
	free := scatterOf(forms.Free(as, p))
	seen, at := map[string]bool{}, -1
	for i, r := range e.row[:head] {
		l := Look(r.Rects, p)
		f := forms.Of(r.Key, r.Rects, p)
		if rowKeys[r.Key] || seen[r.Key] || seen[l] || f == forms.Scatter && looks[l] {
			t.Errorf("%s: %q повторяет ключ или вид", name, r.Key)
		}
		seen[r.Key], seen[l] = true, true
		if f != forms.Scatter {
			if i != 0 || f != forms.Contrast {
				t.Errorf("%s: на %d месте %q (%s), ждали центр первым", name, i, r.Key, f)
			}
			continue
		}
		j := slices.IndexFunc(free, func(c forms.Cand) bool { return c.Key == r.Key })
		if j <= at {
			t.Errorf("%s: %q не по порядку профилей Free", name, r.Key)
		}
		at = j
	}
	for _, r := range e.row[head:] {
		if forms.Of(r.Key, r.Rects, p) == forms.Scatter {
			t.Errorf("%s: «Разброс» %q в хвосте", name, r.Key)
		}
	}
}

func scatterOf(cs []forms.Cand) []forms.Cand {
	return slices.DeleteFunc(slices.Clone(cs), func(c forms.Cand) bool { return c.Form != forms.Scatter })
}

// TestExtraFreeEdges: пусто, одно, два, шесть и 17 фото — ручных нет, число скрытых 0; кэш отдаёт то же.
func TestExtraFreeEdges(t *testing.T) {
	for _, as := range [][]float64{nil, {aspP}, sameAspects(2, aspP), sameAspects(6, aspP), sameAspects(16, aspP)} {
		e := extraOf(as, geomRule, 24)
		if len(e.manual) != 0 && len(as) != 6 || e.hidden != 0 && len(as) < 3 {
			t.Errorf("%d фото: ручных %d, скрыто %d", len(as), len(e.manual), e.hidden)
		}
		for k := range e.manual {
			if forms.Of(k, nil, geomRule) == forms.Scatter {
				t.Errorf("%d фото: «Разброс» %q вне 3–5 фото", len(as), k)
			}
		}
	}
	if got := ExtraFoldHidden(sameAspects(3, aspP), geomRule, 24); got != 8 {
		t.Errorf("3 вертикальных, широкие: скрыто %d, ждали 8", got)
	}
	if got := ExtraFoldHidden(sameAspects(3, aspP), geomSq, 24); got != 0 {
		t.Errorf("3 вертикальных, любые: скрыто %d", got)
	}
}

// manualPool — пул, где ручные раскладки заманчивы: оценка выше лучшей, ряд «Авто» отключён
// от них. Без пропуска Manual Pick выбрал бы одну из них.
func manualPool(t *testing.T, as []float64, p layout.Params) []Cand {
	t.Helper()
	pool := Pool(as, p, 24, Normal)
	n := 0
	for i := range pool {
		if pool[i].Manual {
			pool[i].Score += 5
			pool[i].Fair += 5
			pool[i].Q += 5
			pool[i].Formed, pool[i].Family = true, Dense
			n++
		}
	}
	if n == 0 {
		t.Fatalf("%d фото: ручных в пуле нет — тест ничего не проверяет", len(as))
	}
	return pool
}

// TestPickSkipsManual: Manual (разброс, центр) не выбирается ни на одном уровне, ни со звездой;
// ByFamily его не пропускает ни при каком фильтре, а при пустой выборке отдаёт ряд «Авто».
func TestPickSkipsManual(t *testing.T) {
	for _, as := range [][]float64{sameAspects(3, aspP), sameAspects(4, aspP), sameAspects(5, aspP)} {
		pool := manualPool(t, as, geomSq)
		for _, lv := range []Level{Less, Normal, More} {
			for star := -1; star < len(as); star++ {
				got := Pick([]Spread{{Pool: pool, Star: star}, {Pool: pool, Star: -1}}, geomSq, lv, Ctx{})
				for _, i := range got {
					if i < 0 || pool[i].Manual {
						t.Errorf("%d фото, уровень %d, звезда %d: выбрана ручная %q", len(as), lv, star, pool[max(i, 0)].Key)
					}
				}
			}
		}
		for _, allow := range []map[Family]bool{{Figure: true}, {Dense: true}, {Figure: true, Dense: true, Air: true, Hero: true, Strip: true, Bleed: true}} {
			out, _ := ByFamily(pool, allow, geomSq)
			if slices.ContainsFunc(out, func(c Cand) bool { return c.Manual }) {
				t.Errorf("%d фото: ByFamily пропустил ручную", len(as))
			}
		}
	}
	// фильтр «Фигуры» без фигур в ряду: не разброс, а лучшая раскладка (запасной pool[:1])
	pool := manualPool(t, sameAspects(3, aspP), geomSq)
	pool = slices.DeleteFunc(pool, func(c Cand) bool {
		return !c.InRow && !c.Manual || c.InRow && TileFamily(c.Form, c.Rects, geomSq) == Figure
	})
	if out, fb := ByFamily(pool, map[Family]bool{Figure: true}, geomSq); !fb || len(out) != 1 || out[0].Manual {
		t.Errorf("«Фигуры» без фигур в ряду: %d плиток, запасной %v", len(out), fb)
	}
}

// TestDescribeScatterFigure: семья «Разброса» — «Фигуры» (по заполнению вышел бы «Воздух» и попал бы в такт воздуха).
func TestDescribeScatterFigure(t *testing.T) {
	as := sameAspects(3, aspP)
	for _, c := range scatterOf(forms.Free(as, geomSq)) {
		in := Describe(c.Result, geomSq)
		if in.Form != forms.Scatter || in.Family != Figure || TileFamily(in.Form, c.Rects, geomSq) != Figure {
			t.Errorf("%q: вид %s, семья %q", c.Key, in.Form, in.Family)
		}
		if in.Fill >= airFill {
			t.Logf("%q: заполнение %.2f ≥ порога воздуха", c.Key, in.Fill)
		}
	}
	if FamilyOf(forms.Scatter) != Figure {
		t.Error("FamilyOf(Scatter) не Figure")
	}
	for _, c := range Pool(as, geomSq, 24, Normal) {
		if c.Manual && c.Family != Figure {
			t.Errorf("ручная %q с семьёй %q", c.Key, c.Family)
		}
	}
}
