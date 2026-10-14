package variety

import (
	"slices"
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// TestPoolKeysBuild: каждый ключ пула строится layout.Build на тех же пропорциях и даёт те же
// Rects и оценку — в файл можно писать любой кандидат (design §4).
func TestPoolKeysBuild(t *testing.T) {
	n, extra := 0, 0
	for _, a := range corpus() {
		for i, s := range a.spreads {
			for _, c := range s.Pool {
				r, ok := layout.Build(c.Key, a.asps[i], a.p)
				if !ok {
					t.Fatalf("%s, разворот %d: Build(%q) отказал", a.name, i, c.Key)
				}
				if !slices.Equal(r.Rects, c.Rects) || r.Score != c.Score {
					t.Fatalf("%s, разворот %d: %q: Build даёт другое", a.name, i, c.Key)
				}
				n++
				if !c.InRow {
					extra++
				}
			}
		}
	}
	if extra == 0 {
		t.Error("в корпусе нет ни одного воздушного варианта вне ряда")
	}
	t.Logf("ключей %d, из них вне ряда %d", n, extra)
}

// TestPoolShape: ряд «Авто» — префикс пула как есть; дальше без повторов ключей ровно «Другие
// варианты» в их порядке: страницы с модификатором S/O/I и раскладки с главным фото (семейство
// Kind — только у них, у 8–16 фото); Меньше — только ряд; уровень вне 0…2 зажимается.
func TestPoolShape(t *testing.T) {
	for _, asp := range [][]float64{
		sameAspects(6, aspL), sameAspects(12, aspL), mixedAspects(8, 1), mixedAspects(10, 2), {aspL},
	} {
		row := layout.Cached(asp, geomSq, 24)
		less := Pool(asp, geomSq, 24, Less)
		if len(less) != len(row) {
			t.Fatalf("%d фото: Меньше — %d, ряд — %d", len(asp), len(less), len(row))
		}
		pool := Pool(asp, geomSq, 24, Normal)
		if len(pool) < len(row) {
			t.Fatalf("пул короче ряда")
		}
		extra := Extra(asp, geomSq, 24)
		if len(pool) != len(row)+len(extra) {
			t.Fatalf("%d фото: пул %d, ряд %d + других %d", len(asp), len(pool), len(row), len(extra))
		}
		feat := checkPoolOrder(t, asp, pool, row, extra)
		if want := len(asp) >= FeatMin; feat != want {
			t.Errorf("%d фото: featured в пуле %v, ждали %v", len(asp), feat, want)
		}
		if !slices.EqualFunc(Pool(asp, geomSq, 24, Level(9)), pool, func(a, b Cand) bool { return a.Key == b.Key }) {
			t.Error("уровень 9 = Больше (тот же пул)")
		}
		if got := Pool(asp, geomSq, 24, Level(-3)); len(got) != len(row) {
			t.Error("уровень −3 = Меньше")
		}
	}
}

// checkPoolOrder — пул = ряд, затем «Другие варианты» в их порядке, ключи без повторов; featured
// (Kind) — вне ряда, без модификатора страницы и с главным; остальное вне ряда — воздух S/O/I.
// Возвращает, есть ли featured.
func checkPoolOrder(t *testing.T, asp []float64, pool []Cand, row, extra []layout.Result) (feat bool) {
	t.Helper()
	seen := map[string]bool{}
	for i, c := range pool {
		if seen[c.Key] {
			t.Errorf("%d фото: повтор ключа %q", len(asp), c.Key)
		}
		seen[c.Key] = true
		want := ""
		if i < len(row) {
			want = row[i].Key
		} else {
			want = extra[i-len(row)].Key
		}
		if c.InRow != (i < len(row)) || c.Key != want {
			t.Errorf("%d фото: на %d %q (InRow %v), ждали %q", len(asp), i, c.Key, c.InRow, want)
		}
		switch {
		case c.Kind != NoKind:
			feat = true
			if c.InRow || pageMod.MatchString(c.Key) || c.Heroes == 0 {
				t.Errorf("%d фото: featured %q в ряду «Авто», с модификатором или без главного", len(asp), c.Key)
			}
		case !c.InRow && (!layout.IsPaged(c.Key) || !pageMod.MatchString(c.Key)):
			t.Errorf("лишний вариант не страницы с модификатором: %q", c.Key)
		}
	}
	return feat
}

// TestPoolEdges: пустой вход и одно фото не падают.
func TestPoolEdges(t *testing.T) {
	if got := Pool(nil, geomSq, 24, More); len(got) != 0 {
		t.Errorf("без фото пул пуст, получили %d", len(got))
	}
	if got := Pool([]float64{aspL}, geomSq, 24, Normal); len(got) == 0 || !got[0].InRow {
		t.Error("одно фото — ряд не пуст")
	}
}

// checkExtraRow — ряд «Других вариантов» got набора asp: без повторов и ключей ряда «Авто», строится,
// по убыванию оценки в квантах (ничья — по ключу); каждый — воздух или featured. Возвращает, сколько
// в нём featured.
func checkExtraRow(t *testing.T, asp []float64, got []layout.Result) (feat int) {
	t.Helper()
	row := layout.Cached(asp, geomSq, 24)
	seen := map[string]bool{}
	kinds := extraOf(asp, geomSq, 24).kind
	for i, x := range got {
		if kinds[x.Key] != NoKind {
			feat++
		} else if !pageMod.MatchString(x.Key) {
			t.Errorf("%d фото: %q — не воздух и не featured", len(asp), x.Key)
		}
		if seen[x.Key] || slices.ContainsFunc(row, func(r layout.Result) bool { return r.Key == x.Key }) {
			t.Errorf("%d фото: ключ %q повторён или есть в ряду «Авто»", len(asp), x.Key)
		}
		seen[x.Key] = true
		if r, ok := layout.Build(x.Key, asp, geomSq); !ok || !slices.Equal(r.Rects, x.Rects) {
			t.Errorf("%d фото: %q не строится", len(asp), x.Key)
		}
		if i == 0 {
			continue
		}
		prev := got[i-1]
		if qa, qb := quanta(prev.Score), quanta(x.Score); qa < qb || qa == qb && prev.Key > x.Key {
			t.Errorf("%d фото: порядок нарушен на %d (%q, %q)", len(asp), i, prev.Key, x.Key)
		}
	}
	return feat
}

// TestExtra: «Другие варианты» — вне ряда «Авто», строятся, по убыванию оценки (ничья — по ключу),
// воздуха не больше airLimit, featured не больше featLimit (на богатых наборах оба предела
// достигаются), featured только у 8–16 фото; второй вызов отдаёт тот же ряд из кэша; холодный
// кэш считает то же; пустой вход и одно фото не падают.
func TestExtra(t *testing.T) {
	sameKeys := func(a, b []layout.Result) bool {
		return slices.EqualFunc(a, b, func(x, y layout.Result) bool { return x.Key == y.Key })
	}
	longestAir, longestFeat := 0, 0
	sets := [][]float64{{aspL}, nil}
	for n := 3; n <= 16; n++ {
		sets = append(sets, sameAspects(n, aspL), mixedAspects(n, uint64(n)))
	}
	for _, asp := range sets {
		ResetCache()
		if Cached(asp, geomSq, 24) {
			t.Fatalf("%d фото: кэш не холодный после ResetCache", len(asp))
		}
		got := Extra(asp, geomSq, 24)
		if !Cached(asp, geomSq, 24) {
			t.Errorf("%d фото: после Extra ряд не в кэше", len(asp))
		}
		feat := checkExtraRow(t, asp, got)
		longestAir, longestFeat = max(longestAir, len(got)-feat), max(longestFeat, feat)
		if len(got)-feat > airLimit || feat > featLimit || len(got) > ExtraLimit {
			t.Fatalf("%d фото: «Других вариантов» %d (featured %d)", len(asp), len(got), feat)
		}
		if feat > 0 && len(asp) < FeatMin {
			t.Fatalf("%d фото: featured %d у меньше %d фото", len(asp), feat, FeatMin)
		}
		if !sameKeys(Extra(asp, geomSq, 24), got) {
			t.Errorf("%d фото: второй вызов дал другой ряд", len(asp))
		}
		ResetCache()
		layout.ResetCache()
		if !sameKeys(Extra(asp, geomSq, 24), got) {
			t.Errorf("%d фото: холодный кэш дал другой ряд", len(asp))
		}
	}
	if longestAir != airLimit || longestFeat != featLimit {
		t.Errorf("пределы не достигнуты: воздух %d из %d, featured %d из %d", longestAir, airLimit, longestFeat, featLimit)
	}
}

// TestExtraCacheGen: на extraGen записях кэш «Других вариантов» начинается заново (память не
// растёт), новая запись в нём; прежние — нет.
func TestExtraCacheGen(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)
	old := sameAspects(4, aspL)
	Extra(old, geomSq, 24)
	extraCache.Lock()
	for i := len(extraCache.m); i < extraGen; i++ {
		extraCache.m[strconv.Itoa(i)] = extraEntry{}
	}
	extraCache.Unlock()
	as := sameAspects(3, aspL)
	if Cached(as, geomSq, 24) {
		t.Fatal("заготовка: ряд уже в кэше")
	}
	Extra(as, geomSq, 24)
	extraCache.Lock()
	n := len(extraCache.m)
	extraCache.Unlock()
	if n != 1 || !Cached(as, geomSq, 24) || Cached(old, geomSq, 24) {
		t.Fatalf("после предела записей %d, новая в кэше %v", n, Cached(as, geomSq, 24))
	}
}
