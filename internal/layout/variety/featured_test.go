package variety

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Часть 2 спеки 041 (design §8, §10; critic.md §6): генератор featured и метрики разворотов
// 8–16 фото.

// featSets — наборы 8–16 фото: одинаковые горизонтальные и вертикальные, серии, смеси.
func featSets() map[string][]float64 {
	out := map[string][]float64{}
	for n := FeatMin; n < layout.ManyFrom; n++ {
		out[fmt.Sprintf("гориз%d", n)] = sameAspects(n, aspL)
		out[fmt.Sprintf("верт%d", n)] = sameAspects(n, aspP)
		out[fmt.Sprintf("серия%d", n)] = seriesAspects(n, 0.65, uint64(n))
		out[fmt.Sprintf("смесь%d", n)] = mixedAspects(n, uint64(n))
	}
	return out
}

func sortedNames(m map[string][]float64) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

var featGeoms = []struct {
	name string
	p    layout.Params
}{{"30x30", geomSq}, {"30x20", geomWide}, {"30x30-flat", geomFlat}}

// checkFeat — свой проверщик раскладки (layout не трогаем): внутри разворота, а не «в край» — и
// внутри полей; пропорции 0.1 % (кроме «в край»); без пересечений; без layflat — не через
// корешок; в порядке ленты; Build даёт то же.
func checkFeat(t *testing.T, name string, as []float64, p layout.Params, f Feat) {
	t.Helper()
	r, ok := layout.Build(f.Key, as, p)
	if !ok || !slices.Equal(r.Rects, f.Rects) || r.Score != f.Score {
		t.Fatalf("%s: %q — Build отказал или дал другое", name, f.Key)
	}
	if layout.IsRearranged(f.Key) {
		t.Fatalf("%s: %q — не в порядке ленты", name, f.Key)
	}
	bleed := -1
	if i := strings.Index(f.Key, "E("); i >= 0 {
		fmt.Sscanf(f.Key[i+2:], "%d", &bleed)
	}
	const eps = 1e-6
	for i, x := range f.Rects {
		if x.X < -eps || x.Y < -eps || x.X+x.W > 2*p.PageW+eps || x.Y+x.H > p.PageH+eps {
			t.Fatalf("%s: %q — фото %d вне разворота", name, f.Key, i)
		}
		if i != bleed {
			if x.X < p.Margin-eps || x.Y < p.Margin-eps || x.X+x.W > 2*p.PageW-p.Margin+eps || x.Y+x.H > p.PageH-p.Margin+eps {
				t.Fatalf("%s: %q — фото %d вне полей", name, f.Key, i)
			}
			if a := x.W / x.H; math.Abs(a-as[i])/as[i] > 0.001 {
				t.Fatalf("%s: %q — фото %d искажено: %.4f вместо %.4f", name, f.Key, i, a, as[i])
			}
		}
		if !p.Layflat && x.X < p.PageW-foldEps && x.X+x.W > p.PageW+foldEps {
			t.Fatalf("%s: %q — без layflat фото %d через корешок", name, f.Key, i)
		}
		for j := range i {
			y := f.Rects[j]
			if x.X+eps < y.X+y.W && y.X+eps < x.X+x.W && x.Y+eps < y.Y+y.H && y.Y+eps < x.Y+x.H {
				t.Fatalf("%s: %q — фото %d и %d пересекаются", name, f.Key, i, j)
			}
		}
	}
}

// TestFeaturedInvariants: все ключи генератора на наборах 8–16 фото × 3 геометрии строятся и
// проходят инварианты; генератор что-то даёт на каждой геометрии и в каждом семействе; бюджет
// Build ≤ 150 на разворот (design §8); вне 5–40 фото, у листа и на плохом входе — пусто
// (5–7 и 17–40 — TestFeaturedWide).
func TestFeaturedInvariants(t *testing.T) {
	sets := featSets()
	for _, g := range featGeoms {
		kinds := map[Kind]int{}
		for _, name := range sortedNames(sets) {
			as := sets[name]
			fs := featured(as, g.p)
			for _, f := range fs {
				checkFeat(t, g.name+"/"+name, as, g.p, f)
				kinds[f.Kind]++
			}
			if b := builds(as, g.p); b > 150 {
				t.Errorf("%s/%s: Build %d > 150", g.name, name, b)
			}
		}
		for _, k := range []Kind{KindPage, KindJoint, KindProps, KindPair} {
			if kinds[k] == 0 {
				t.Errorf("%s: нет ни одной раскладки семейства %d", g.name, k)
			}
		}
		if g.p.Layflat != (kinds[KindSpine] > 0) {
			t.Errorf("%s: «через корешок» %d при layflat=%v", g.name, kinds[KindSpine], g.p.Layflat)
		}
	}
	sheet := geomSq
	sheet.Sheet = true
	for name, c := range map[string]struct {
		as []float64
		p  layout.Params
	}{
		"4 фото":        {sameAspects(4, aspL), geomSq},
		"41 фото":       {sameAspects(41, aspL), geomSq},
		"пусто":         {nil, geomSq},
		"лист":          {sameAspects(12, aspL), sheet},
		"нулевой поля":  {sameAspects(12, aspL), layout.Params{PageW: 20, PageH: 300, Margin: 12, Layflat: true}},
		"плохой аспект": {append(sameAspects(11, aspL), math.NaN()), geomSq},
		"нулевой":       {append(sameAspects(11, aspL), 0), geomSq},
	} {
		if fs := featured(c.as, c.p); len(fs) != 0 {
			t.Errorf("%s: ждали пусто, получили %d", name, len(fs))
		}
	}
}

// builds — сколько раз генератор вызывает Build на наборе.
func builds(as []float64, p layout.Params) int {
	g := newGen(as, p)
	g.run()
	return g.builds
}

// TestFeaturedFold (решение №31): у каждой раскладки генератора главное по геометрии есть и
// занимает свою долю площади; не-главные не на сгибе и не мельче 0.12 высоты страницы; через
// сгиб — только главное шире 1.2. Проверка независимая: по Rects, а не через allowed.
func TestFeaturedFold(t *testing.T) {
	sets := featSets()
	n := 0
	for _, g := range featGeoms {
		for _, name := range sortedNames(sets) {
			for _, f := range featured(sets[name], g.p) {
				n++
				if msg := foldFault(f.Rects, g.p); msg != "" {
					t.Fatalf("%s/%s %q: %s", g.name, name, f.Key, msg)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("генератор ничего не дал")
	}
}

// foldFault — нарушение запретов №31 у раскладки (пусто — нет): главных (одно ≥ 1.6× второго или
// пара ≥ 1.6× третьего) нет, доля главного меньше 0.30 (у пары — 0.15), портрет через сгиб,
// не-главное на сгибе или мельче 0.12 высоты страницы.
func foldFault(rs []layout.Rect, p layout.Params) string {
	areas, order, sum := make([]float64, len(rs)), make([]int, len(rs)), 0.0
	for i, x := range rs {
		areas[i], order[i] = x.W*x.H, i
		sum += areas[i]
	}
	slices.SortStableFunc(order, func(a, b int) int { return -cmpF(areas[a], areas[b]) })
	heroes, share := 1, 0.30
	if areas[order[0]] < 1.6*areas[order[1]] {
		heroes, share = 2, 0.15
		if areas[order[1]] < 1.6*areas[order[2]] {
			return "главного нет"
		}
	}
	// Допуск 1e-9 доли — квант allowed: сторона, легшая на пол точно, может выйти на бит меньше.
	const tol = 1e-9
	for rank, i := range order {
		x := rs[i]
		fold := p.Layflat && x.X < p.PageW-0.01 && x.X+x.W > p.PageW+0.01
		switch {
		case rank < heroes && (areas[i]/sum < share-tol || fold && x.W/x.H < 1.2-tol):
			return fmt.Sprintf("главное %d — доля %.2f, через сгиб %v", i, areas[i]/sum, fold)
		case rank >= heroes && (fold || math.Min(x.W, x.H)/p.PageH < 0.12-tol):
			return fmt.Sprintf("не-главное %d на сгибе (%v) или мельче пола", i, fold)
		}
	}
	return ""
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// TestFeaturedUniform (critic §4): 12 одинаковых фото (3:2 и 2:3) на квадратном развороте —
// у генератора есть раскладка с главным в допуске «Обычного» от потолка (иначе главного на
// одинаковых сериях не бывает — жалоба владельца), и она попадает в «Другие варианты».
func TestFeaturedUniform(t *testing.T) {
	for _, as := range [][]float64{sameAspects(12, aspL), sameAspects(12, aspP)} {
		row := layout.Cached(as, geomSq, 24)
		lim := quanta(min(row[0].Score, heroCap) - tunings[Normal].deltaHero)
		e := extraOf(as, geomSq, 24)
		ok := false
		for _, r := range e.row {
			if e.kind[r.Key] != NoKind && quanta(Describe(r, geomSq).Fair) >= lim {
				ok = true
			}
		}
		if !ok {
			t.Errorf("аспект %.2f: нет раскладки с главным в допуске", as[0])
		}
	}
}

// TestFeaturedExtra: в «Других вариантах» featured — не больше featLimit, сначала по одному на
// главное фото: позиций главного не меньше, чем у генератора (до предела); пул «Обычного» берёт
// их с семейством (и featured ряда «Авто» — с семейством ряда), «Меньше» — только ряд.
func TestFeaturedExtra(t *testing.T) {
	as := sameAspects(12, aspL)
	e := extraOf(as, geomSq, 24)
	var mine []Feat
	for _, r := range e.row {
		if k := e.kind[r.Key]; k != NoKind {
			mine = append(mine, Feat{Result: r, Info: Describe(r, geomSq), Kind: k})
		}
	}
	if len(mine) == 0 || len(mine) > featLimit {
		t.Fatalf("featured в «Других вариантах» %d", len(mine))
	}
	floor := quanta(min(layout.Cached(as, geomSq, 24)[0].Score, heroCap) - featSlack)
	gen, have := map[int]bool{}, map[int]bool{}
	for _, f := range featured(as, geomSq) {
		if f.Big >= 0 && quanta(f.Fair) >= floor {
			gen[f.Big] = true
		}
	}
	for _, f := range mine {
		have[f.Big] = true
	}
	for _, c := range rowOf(as, geomSq, 24).row { // featured ряда «Авто» (051) тоже дают позицию
		if c.Form == forms.Hero || c.Form == forms.Spine {
			have[Describe(c.Result, geomSq).Big] = true
		}
	}
	for b := range gen {
		if !have[b] && len(mine) < featLimit {
			t.Errorf("главное %d есть у генератора, но не в ряду и не в «Других вариантах»", b)
		}
	}
	kinds := 0
	re := rowOf(as, geomSq, 24)
	for _, c := range Pool(as, geomSq, 24, Normal) {
		if c.Kind == NoKind {
			continue
		}
		want := e.kind[c.Key]
		if c.InRow {
			want = re.kind[c.Key] // с 051 лучшие featured по формам — в ряду «Авто»
		} else {
			kinds++
		}
		if want != c.Kind {
			t.Errorf("%q: семейство в пуле %d, в ряду или подборке %d", c.Key, c.Kind, want)
		}
	}
	if kinds != len(mine) {
		t.Errorf("в пуле featured вне ряда %d, в подборке %d", kinds, len(mine))
	}
	for _, c := range Pool(as, geomSq, 24, Less) {
		if !c.InRow {
			t.Fatalf("«Меньше»: %q вне ряда «Авто» в пуле", c.Key)
		}
	}
}

// TestFeaturedDeterministic: тот же вход — те же ключи в том же порядке (map внутри не влияет).
func TestFeaturedDeterministic(t *testing.T) {
	as := mixedAspects(14, 5)
	keys := func() []string {
		var out []string
		for _, f := range featured(as, geomSq) {
			out = append(out, f.Key)
		}
		return out
	}
	first := keys()
	for range 5 {
		if !slices.Equal(keys(), first) {
			t.Fatal("генератор недетерминирован")
		}
	}
}

// Корпус части 2: альбомы 8–16 фото на развороте (critic.md: серии, одинаковые, смесь).
var featSpecs = []albumSpec{
	{"серии 300/25 m16", 300, 25, 16, geomSq, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"серии 300/25 30x20 m16", 300, 25, 16, geomWide, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"серии 300/25 m16 без layflat", 300, 25, 16, geomFlat, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"гориз. 120/10 m16", 120, 10, 16, geomSq, func(n int, _ uint64) []float64 { return sameAspects(n, aspL) }},
	{"верт. 96/8 m16", 96, 8, 16, geomSq, func(n int, _ uint64) []float64 { return sameAspects(n, aspP) }},
	{"смесь 160/14 m16", 160, 14, 16, geomSq, mixedAspects},
}

var (
	featOnce sync.Once
	featData []corpusAlbum
)

func featCorpus() []corpusAlbum {
	featOnce.Do(func() {
		const seeds = 2
		featData = make([]corpusAlbum, len(featSpecs)*seeds)
		var wg sync.WaitGroup
		for i, sp := range featSpecs {
			for s := range seeds {
				wg.Go(func() {
					seed := uint64(s + 1)
					featData[i*seeds+s] = build(fmt.Sprintf("%s s%d", sp.name, seed), sp.set(sp.n, seed), sp.sp, sp.maxP, sp.p, seed)
				})
			}
		}
		wg.Wait()
	})
	return featData
}

// TestFeaturedMetrics — пороги части 2 (design §10, решения №30–34) на корпусе 8–16 фото и на
// разворотах 8–16 основного корпуса: с главным фото «Обычное» ≥ 15 %, «Больше» ≥ 25 %; звезда
// главная ≥ 40 %; средняя потеря «Обычного» ≤ 0.02; на одинаковых сериях главное не реже раза
// на период ритма; «Меньше» без главного по выбору (как 0.10).
func TestFeaturedMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("метрики по всему корпусу: полный прогон (check.ps1) и CI")
	}
	var all [3]total
	for _, a := range slices.Concat(featCorpus(), corpus()) {
		for lv := Less; lv <= More; lv++ {
			m := measure(a, lv)
			all[lv].add(m)
			if testing.Verbose() && m.big > 0 {
				t.Logf("%-34s %d видов %2d повторов %2d/%2d главное %2d/%2d (пары %2d) звезда %d/%d потеря %.3f/%.3f",
					a.name, lv, m.looks, m.repeats, m.pairs, m.bigOne, m.big, m.bigHero, m.bigStarsBig, m.bigStars,
					ratio2(m.loss, m.lossN), m.maxLoss)
			}
			uniform := strings.HasPrefix(a.name, "гориз. 120") || strings.HasPrefix(a.name, "верт.")
			if lv > Less && uniform {
				if per := tunings[lv].heroPeriod; m.bigHero < m.spreadsN/per {
					t.Errorf("%s, уровень %d: главное на %d из %d разворотов — реже раза на %d", a.name, lv, m.bigHero, m.spreadsN, per)
				}
			}
		}
	}
	share := func(lv Level) float64 { return ratio(all[lv].bigOne, all[lv].big) }
	star := func(lv Level) float64 { return ratio(all[lv].bigStarsBig, all[lv].bigStars) }
	for lv := Less; lv <= More; lv++ {
		t.Logf("итого %d (8–16 фото): с главным %.0f%% (%d/%d; с парой %.0f%%), звезда главная %.0f%% (%d/%d), повторов %.1f%%, потеря %.4f/%.3f",
			lv, 100*share(lv), all[lv].bigOne, all[lv].big, 100*ratio(all[lv].bigHero, all[lv].big), 100*star(lv),
			all[lv].bigStarsBig, all[lv].bigStars, 100*all[lv].repeatShare(), all[lv].avgLoss(), all[lv].maxLoss)
	}
	if share(Normal) < 0.15 || share(More) < 0.25 || share(Normal) <= share(Less) {
		t.Errorf("с главным: «Меньше» %.2f, «Обычное» %.2f (≥ 0.15), «Больше» %.2f (≥ 0.25)", share(Less), share(Normal), share(More))
	}
	for _, lv := range []Level{Normal, More} {
		if star(lv) < 0.40 {
			t.Errorf("уровень %d: звезда главная %.2f < 0.40", lv, star(lv))
		}
	}
	if l := all[Normal].avgLoss(); l > 0.02 {
		t.Errorf("«Обычное»: средняя потеря %.4f > 0.02", l)
	}
	if all[Less].avgLoss() != 0 {
		t.Errorf("«Меньше» теряет оценку: %.4f", all[Less].avgLoss())
	}
}

// TestFeaturedTiesNoise — шум 1e-12 в пропорциях (как FMA на arm64 в оценках сеток) не меняет
// ключи генератора: ничьи симметричных разбиений решает порядок перебора, а не последний бит.
func TestFeaturedTiesNoise(t *testing.T) {
	sets := featSets()
	r := rng(41)
	for _, g := range featGeoms {
		for _, name := range sortedNames(sets) {
			as := sets[name]
			noisy := slices.Clone(as)
			for i := range noisy {
				noisy[i] *= 1 + (r.unit()*2-1)*1e-12
			}
			a, b := featured(as, g.p), featured(noisy, g.p)
			if !slices.EqualFunc(a, b, func(x, y Feat) bool { return x.Key == y.Key }) {
				t.Errorf("%s/%s: шум 1e-12 поменял ключи генератора (%d / %d)", g.name, name, len(a), len(b))
			}
		}
	}
}

// TestFeaturedSpeed: генератор на 16 фото (холодный кэш layout ему не нужен) — страж от взрыва
// перебора. Цель design §10 (≤ 2 мс) — у BenchmarkFeatured16: обычно 1–1,6 мс, и предел 2 мс в тесте
// падал от шума (2,1 мс; под -race 63 мс), поэтому здесь 40 мс.
func TestFeaturedSpeed(t *testing.T) {
	as := mixedAspects(16, 3)
	d := testkit.Fastest(5, func() { featured(as, geomSq) })
	t.Logf("featured 16: %v", d)
	if lim := testkit.Limit(40 * time.Millisecond); d > lim {
		t.Fatalf("featured 16 фото %v > %v", d, lim)
	}
}

// BenchmarkFeatured16 — генератор на 16 фото (design §10: ≤ 2 мс).
func BenchmarkFeatured16(b *testing.B) {
	as := mixedAspects(16, 3)
	for b.Loop() {
		featured(as, geomSq)
	}
}

// Эталон генератора (critic §4): ключи featured по наборам 8–16 фото × 3 геометрии, отсортированные
// по ключу, — sha256. Только ключи и порядок по ключу. Ничьи сеток и пороги запретов — в квантах
// (решение 0.11 №68), шум 1e-12 ключей не меняет (TestFeaturedTiesNoise); на arm64 — наблюдение (№70).
const featGoldenPath = "testdata/golden/featured.sha"

func featGoldenLines() []string {
	sets := featSets()
	var out []string
	for _, g := range featGeoms {
		for _, name := range sortedNames(sets) {
			var keys []string
			for _, f := range featured(sets[name], g.p) {
				keys = append(keys, f.Key)
			}
			slices.Sort(keys)
			h := sha256.Sum256([]byte(strings.Join(keys, "\n")))
			out = append(out, fmt.Sprintf("%s/%s %d %s", g.name, name, len(keys), hex.EncodeToString(h[:8])))
		}
	}
	return out
}

func TestGoldenFeatured(t *testing.T) {
	goldenArch(t)
	got := featGoldenLines()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(featGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(featGoldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", featGoldenPath, len(got))
		return
	}
	data, err := os.ReadFile(featGoldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/layout/variety -run GoldenFeatured -update", err)
	}
	want := strings.Split(strings.TrimSpace(string(data)), "\n")
	if !slices.Equal(want, got) {
		bad := 0
		for i := range min(len(got), len(want)) {
			if got[i] != want[i] {
				if bad++; bad <= 5 {
					t.Errorf("расхождение:\n  есть   %s\n  эталон %s", got[i], want[i])
				}
			}
		}
		t.Errorf("строк %d / %d, разошлось %d", len(got), len(want), bad)
	}
}
