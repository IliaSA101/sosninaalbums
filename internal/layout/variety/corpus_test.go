package variety

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
)

// Корпус спеки 041 (design §6): геометрии, наборы «серии» / «все горизонтальные» / «смесь»,
// альбомы, сиды 1–3, звёзды. Входы детерминированы своим генератором — не зависят от версии Go.

var (
	// Layflat — в «любые фото» (FoldAny): прежнее поведение, эталоны прежние (064 G1).
	geomSq   = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}
	geomWide = layout.Params{PageW: 300, PageH: 200, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}
	geomFlat = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: false}
)

// wideFold — p в положении «только широкие» (layflat-геометрии тестов выше — «любые фото», 064).
func wideFold(p layout.Params) layout.Params {
	p.FoldAny = false
	return p
}

const (
	aspL = 1.5       // горизонтальное 3:2
	aspP = 2.0 / 3.0 // вертикальное 2:3
)

type rng uint64

func (s *rng) unit() float64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return float64((z^z>>31)>>11) / (1 << 53)
}

// seriesAspects — серии: ориентация держится с вероятностью 0.6, горизонтальных долей pLand.
func seriesAspects(n int, pLand float64, seed uint64) []float64 {
	r := rng(seed)
	out := make([]float64, n)
	cur := aspL
	for i := range out {
		if r.unit() > 0.6 {
			cur = aspP
			if r.unit() < pLand {
				cur = aspL
			}
		}
		out[i] = cur
	}
	return out
}

// mixedAspects — смесь 62 % горизонтальных, 33 % вертикальных, 5 % квадратов.
func mixedAspects(n int, seed uint64) []float64 {
	r := rng(seed)
	out := make([]float64, n)
	for i := range out {
		switch x := r.unit(); {
		case x < 0.62:
			out[i] = aspL
		case x < 0.95:
			out[i] = aspP
		default:
			out[i] = 1
		}
	}
	return out
}

// sameAspects — n одинаковых фото.
func sameAspects(n int, a float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = a
	}
	return out
}

// ratingsOf — оценки: 5 % — 5★, 10 % — 4★.
func ratingsOf(n int, seed uint64) []int {
	r := rng(seed * 7)
	out := make([]int, n)
	for i := range out {
		switch x := r.unit(); {
		case x < 0.05:
			out[i] = 5
		case x < 0.15:
			out[i] = 4
		}
	}
	return out
}

// rowLen — длина ряда «Авто» для n фото, как model.RowLen.
func rowLen(n int) int {
	if n >= layout.ManyFrom {
		return layout.ManyBatch
	}
	return 24
}

// corpusPool — пул разворота, как у программы (spreads.poolOf): ряд «Авто» и «Другие варианты»
// (Extra: ≤ ExtraLimit лучших по оценке), а не весь пул Pool; с ManyFrom — только ряд.
func corpusPool(as []float64, p layout.Params) []Cand {
	if len(as) >= layout.ManyFrom {
		return Pool(as, p, rowLen(len(as)), Less)
	}
	extra := Extra(as, p, rowLen(len(as)))
	return slices.DeleteFunc(Pool(as, p, rowLen(len(as)), Normal), func(c Cand) bool {
		return !c.InRow && !slices.ContainsFunc(extra, func(x layout.Result) bool { return x.Key == c.Key })
	})
}

// corpusAlbum — альбом корпуса после группировки: пулы и звёзды разворотов.
type corpusAlbum struct {
	name    string
	p       layout.Params
	sizes   []int // фото на развороте
	asps    [][]float64
	spreads []Spread
}

// build считает PlanGroups и пулы (уровень Normal: тот же пул у Больше; Меньше берёт [0]).
func build(name string, asp []float64, n, maxPer int, p layout.Params, seed uint64) corpusAlbum {
	pl := plan.PlanGroups(asp, p, n, maxPer)
	rt := ratingsOf(len(asp), seed)
	a := corpusAlbum{name: name, p: p}
	for _, g := range pl.Groups {
		as := asp[g[0]:g[1]]
		a.sizes = append(a.sizes, len(as))
		a.asps = append(a.asps, as)
		a.spreads = append(a.spreads, Spread{Pool: corpusPool(as, p), Star: Star(rt[g[0]:g[1]])})
	}
	return a
}

type albumSpec struct {
	name        string
	n, sp, maxP int
	p           layout.Params
	set         func(n int, seed uint64) []float64
}

var corpusSpecs = []albumSpec{
	{"серии 120/15 m12", 120, 15, 12, geomSq, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"серии 80/20 m8", 80, 20, 8, geomSq, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"гориз. 96/12 m12", 96, 12, 12, geomSq, func(n int, _ uint64) []float64 { return sameAspects(n, aspL) }},
	{"серии 120/30 30x20 m8", 120, 30, 8, geomWide, func(n int, s uint64) []float64 { return seriesAspects(n, 0.65, s) }},
	{"смесь 80/10 m12", 80, 10, 12, geomSq, mixedAspects},
	{"смесь 60/20 m6", 60, 20, 6, geomSq, mixedAspects},
	{"смесь 40/20 m4", 40, 20, 4, geomSq, mixedAspects},
	{"смесь 80/10 m12 без layflat", 80, 10, 12, geomFlat, mixedAspects},
}

const corpusSeeds = 3

var (
	corpusOnce, corpusWideOnce sync.Once
	corpusData, corpusWideData []corpusAlbum
)

// corpus — 8 альбомов × 3 сида = 24; строится один раз параллельно (пулы на холодном кэше).
// Layflat-альбомы — «любые фото» (прежнее поведение, эталон variety.sha).
func corpus() []corpusAlbum {
	corpusOnce.Do(func() { corpusData = buildCorpus(corpusSpecs) })
	return corpusData
}

// corpusWide — layflat-альбомы корпуса при «только широкие» (064; без layflat — тот же, не
// повторяется): 7 × 3 = 21, эталон variety-wide.sha.
func corpusWide() []corpusAlbum {
	corpusWideOnce.Do(func() {
		var specs []albumSpec
		for _, sp := range corpusSpecs {
			if sp.p.Layflat {
				sp.name, sp.p = sp.name+" широкие", wideFold(sp.p)
				specs = append(specs, sp)
			}
		}
		corpusWideData = buildCorpus(specs)
	})
	return corpusWideData
}

func buildCorpus(specs []albumSpec) []corpusAlbum {
	out := make([]corpusAlbum, len(specs)*corpusSeeds)
	var wg sync.WaitGroup
	for i, sp := range specs {
		for s := range corpusSeeds {
			wg.Go(func() {
				seed := uint64(s + 1)
				out[i*corpusSeeds+s] = build(fmt.Sprintf("%s s%d", sp.name, seed), sp.set(sp.n, seed), sp.sp, sp.maxP, sp.p, seed)
			})
		}
	}
	wg.Wait()
	return out
}

// metrics — показатели одного альбома на одном уровне.
type metrics struct {
	pairs, repeats  int
	sameKeys        int // соседних пар с одним ключом (та же схема, хоть вид и другой)
	looks           int
	sigma           float64
	hero, air       int
	stars, starsBig int // развороты ≤ 7 фото со звездой / где звезда — главное
	loss, maxLoss   float64
	lossN, spreadsN int
	picked          []int
	// развороты 8–16 фото (часть 2): всего, с одним главным (Big ≥ 0), с главным или парой крупных
	// (Heroes > 0), со звездой / звезда главная
	big, bigOne, bigHero, bigStars, bigStarsBig int
}

func measure(a corpusAlbum, lv Level) metrics { return measureCtx(a, lv, Ctx{}) }

// measureCtx — measure с заданным Ctx (варианты сборки: Ctx.Seed).
func measureCtx(a corpusAlbum, lv Level, c Ctx) metrics {
	picked := Pick(a.spreads, a.p, lv, c)
	m := metrics{picked: picked, spreadsN: len(picked)}
	looks := map[string]bool{}
	var fills []float64
	prev, prevKey := "", ""
	for i, k := range picked {
		s, x := a.spreads[i], a.spreads[i].Pool[k]
		looks[x.Look] = true
		if i > 0 {
			m.pairs++
			if x.Look == prev {
				m.repeats++
			}
			if x.Key == prevKey {
				m.sameKeys++
			}
		}
		prev, prevKey = x.Look, x.Key
		fills = append(fills, x.Fill)
		if x.Family == Air {
			m.air++
		} else {
			l := s.Pool[0].Score - x.Score
			// featured и звезда-главное на 8–16 фото судятся по Fair от потолка (решение 0.11 №30)
			if x.Kind != NoKind || len(x.Rects) >= FeatMin && s.Star >= 0 && x.Big == s.Star {
				l = max(0, min(s.Pool[0].Score, heroCap)-x.Fair)
			}
			// форма ряда «Авто» (051) судится по Q формы от Q лучшей: оценка layout штрафует её воздух
			if x.Formed {
				l = min(l, max(0, s.Pool[0].Q-x.Q))
			}
			m.loss, m.lossN, m.maxLoss = m.loss+l, m.lossN+1, math.Max(m.maxLoss, l)
		}
		if x.Heroes > 0 {
			m.hero++
		}
		if k := a.sizes[i]; k >= FeatMin && k < layout.ManyFrom {
			m.addBig(s.Star, &x)
		}
		if s.Star >= 0 && a.sizes[i] <= 7 {
			m.stars++
			if x.Big == s.Star {
				m.starsBig++
			}
		}
	}
	m.looks, m.sigma = len(looks), stddev(fills)
	return m
}

// addBig — разворот 8–16 фото (часть 2): главное, пара крупных, звезда.
func (m *metrics) addBig(star int, x *Cand) {
	m.big++
	if x.Big >= 0 {
		m.bigOne++
	}
	if x.Heroes > 0 {
		m.bigHero++
	}
	if star >= 0 {
		m.bigStars++
		if x.Big == star {
			m.bigStarsBig++
		}
	}
}

func stddev(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	mu := 0.0
	for _, x := range v {
		mu += x
	}
	mu /= float64(len(v))
	s := 0.0
	for _, x := range v {
		s += (x - mu) * (x - mu)
	}
	return math.Sqrt(s / float64(len(v)))
}

// total — сумма по набору альбомов.
type total struct {
	pairs, repeats, stars, starsBig, spreads, hero, air int
	sameKeys                                            int
	loss                                                float64
	lossN                                               int
	maxLoss                                             float64
	sigmaSum                                            float64
	albums                                              int
	big, bigOne, bigHero, bigStars, bigStarsBig         int
}

func (t *total) add(m metrics) {
	t.pairs += m.pairs
	t.repeats += m.repeats
	t.sameKeys += m.sameKeys
	t.stars += m.stars
	t.starsBig += m.starsBig
	t.spreads += m.spreadsN
	t.hero += m.hero
	t.air += m.air
	t.loss += m.loss
	t.lossN += m.lossN
	t.maxLoss = math.Max(t.maxLoss, m.maxLoss)
	t.sigmaSum += m.sigma
	t.albums++
	t.big += m.big
	t.bigOne += m.bigOne
	t.bigHero += m.bigHero
	t.bigStars += m.bigStars
	t.bigStarsBig += m.bigStarsBig
}

func (t total) repeatShare() float64 { return ratio(t.repeats, t.pairs) }
func (t total) starShare() float64   { return ratio(t.starsBig, t.stars) }
func (t total) sigma() float64       { return ratio2(t.sigmaSum, t.albums) }
func (t total) avgLoss() float64     { return ratio2(t.loss, t.lossN) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func ratio2(a float64, b int) float64 {
	if b == 0 {
		return 0
	}
	return a / float64(b)
}

// TestVarietyMetrics — пороги design §10 на корпусе: повторы вида, σ заполнения, звезда
// главная, средняя потеря. Если реализация даёт лучше порога — порог не поднимать без решения.
func TestVarietyMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("метрики по всему корпусу: полный прогон (check.ps1) и CI")
	}
	var all, small [3]total // весь корпус; альбомы со средним ≤ 6 фото на развороте
	var fs [3]formStats     // формы (051): не мозаика, соседи одной формы
	for _, a := range corpus() {
		for lv := Less; lv <= More; lv++ {
			m := measure(a, lv)
			f := formsOf(a, m.picked)
			fs[lv].spreads, fs[lv].nonMosaic, fs[lv].sameNeighbors = fs[lv].spreads+f.spreads, fs[lv].nonMosaic+f.nonMosaic, fs[lv].sameNeighbors+f.sameNeighbors
			fs[lv].mosaicRun = max(fs[lv].mosaicRun, f.mosaicRun)
			all[lv].add(m)
			if sum(a.sizes) <= 6*len(a.sizes) {
				small[lv].add(m)
			}
			if testing.Verbose() {
				t.Logf("%-34s %-7d видов %2d повторов %2d/%2d σ %.3f воздух %d главное %d звезда %d/%d потеря %.3f/%.3f",
					a.name, lv, m.looks, m.repeats, m.pairs, m.sigma, m.air, m.hero, m.starsBig, m.stars,
					ratio2(m.loss, m.lossN), m.maxLoss)
			}
		}
	}
	for lv := Less; lv <= More; lv++ {
		t.Logf("итого %d: повторов %.1f%% (ключей %d), σ %.3f (≤6 фото %.3f), звезда главная %.0f%% (%d/%d), воздух %d, потеря %.4f/%.3f",
			lv, 100*all[lv].repeatShare(), all[lv].sameKeys, all[lv].sigma(), small[lv].sigma(), 100*all[lv].starShare(),
			all[lv].starsBig, all[lv].stars, all[lv].air, all[lv].avgLoss(), all[lv].maxLoss)
		t.Logf("  8–16 фото: с главным %.0f%% (%d/%d; с парой крупных %.0f%%), звезда главная %.0f%% (%d/%d)",
			100*ratio(all[lv].bigOne, all[lv].big), all[lv].bigOne, all[lv].big, 100*ratio(all[lv].bigHero, all[lv].big),
			100*ratio(all[lv].bigStarsBig, all[lv].bigStars), all[lv].bigStarsBig, all[lv].bigStars)
	}
	for lv := Less; lv <= More; lv++ {
		t.Logf("формы %d: не мозаика %.0f%% (%d/%d), соседей одной формы %d, мозаик подряд до %d",
			lv, 100*ratio(fs[lv].nonMosaic, fs[lv].spreads), fs[lv].nonMosaic, fs[lv].spreads, fs[lv].sameNeighbors, fs[lv].mosaicRun)
	}
	check := func(ok bool, format string, args ...any) {
		t.Helper()
		if !ok {
			t.Errorf(format, args...)
		}
	}
	check(all[Less].loss == 0 && all[Less].air == 0 || all[Less].avgLoss() == 0, "«Меньше» теряет оценку: %v", all[Less].avgLoss())
	check(all[Normal].repeatShare() <= 0.10, "«Обычное»: повторов вида %.1f%% > 10%%", 100*all[Normal].repeatShare())
	check(all[Normal].repeatShare() < all[Less].repeatShare(), "«Обычное» повторяет не реже «Меньше»")
	check(all[Normal].sameKeys < all[Less].sameKeys && all[More].sameKeys < all[Less].sameKeys,
		"соседей с одним ключом: «Меньше» %d, «Обычное» %d, «Больше» %d", all[Less].sameKeys, all[Normal].sameKeys, all[More].sameKeys)
	check(all[Normal].avgLoss() <= 0.02, "«Обычное»: средняя потеря %.4f > 0.02", all[Normal].avgLoss())
	check(small[Normal].sigma() >= 0.10, "«Обычное», ≤ 6 фото: σ заполнения %.3f < 0.10", small[Normal].sigma())
	check(all[More].sigma() >= 0.12, "«Больше»: σ заполнения %.3f < 0.12", all[More].sigma())
	check(fs[Normal].nonMosaic > fs[Less].nonMosaic && fs[More].nonMosaic >= fs[Normal].nonMosaic,
		"не мозаика: «Меньше» %d, «Обычное» %d, «Больше» %d", fs[Less].nonMosaic, fs[Normal].nonMosaic, fs[More].nonMosaic)
	check(fs[Normal].sameNeighbors < fs[Less].sameNeighbors && fs[More].sameNeighbors <= fs[Normal].sameNeighbors,
		"соседей одной формы: «Меньше» %d, «Обычное» %d, «Больше» %d", fs[Less].sameNeighbors, fs[Normal].sameNeighbors, fs[More].sameNeighbors)
	check(all[Normal].starShare() >= 2*all[Less].starShare(), "звезда главная: «Обычное» %.2f < 2× «Меньше» %.2f",
		all[Normal].starShare(), all[Less].starShare())
}

func sum(v []int) int {
	s := 0
	for _, x := range v {
		s += x
	}
	return s
}

// Эталон выбора (design §11): sha256 ключей выбранных раскладок по альбому корпуса и уровню.
// Ловит любую смену порогов, штрафов, пула и самого корпуса; -update — только осознанно, с записью
// в decisions. Как many.sha — на amd64 (оценки с FMA на arm64 различаются в последних битах).
// variety.sha — layflat «любые фото», variety-wide.sha — «только широкие» (064).
var updateGolden = flag.Bool("update", false, "перезаписать testdata/golden/variety*.sha")

const (
	goldenPath     = "testdata/golden/variety.sha"
	goldenWidePath = "testdata/golden/variety-wide.sha"
)

// keysHash — первые 8 байт sha256 ключей по строке на ключ (строка эталона).
func keysHash(keys []string) string {
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintln(h, k)
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func goldenLines(albums []corpusAlbum) []string {
	var out []string
	for _, a := range albums {
		for lv := Less; lv <= More; lv++ {
			var keys []string
			for i, k := range Pick(a.spreads, a.p, lv, Ctx{}) {
				keys = append(keys, a.spreads[i].Pool[k].Key)
			}
			name := strings.ReplaceAll(a.name, " ", "_")
			out = append(out, fmt.Sprintf("%s/%d %s", name, lv, keysHash(keys)))
		}
	}
	return out
}

// goldenArch пропускает эталон вне amd64 (решение 0.11 №70): эталоны пакета сняты на amd64 и на
// arm64 не подтверждены. SA_GOLDEN_ANYARCH=1 включает их на любой архитектуре — так их гоняет
// наблюдение на arm64 в release.yml; подтвердится — пропуск снять, как у first.sha.
func goldenArch(t *testing.T) {
	t.Helper()
	if runtime.GOARCH != "amd64" && os.Getenv("SA_GOLDEN_ANYARCH") != "1" {
		t.Skip("эталон снят на amd64 (SA_GOLDEN_ANYARCH=1 — гонять и здесь)")
	}
}

func TestGoldenVariety(t *testing.T) { checkGoldenVariety(t, goldenPath, goldenLines(corpus())) }

// TestGoldenVarietyWide — выбор на layflat-альбомах корпуса при «только широкие» (064).
func TestGoldenVarietyWide(t *testing.T) {
	checkGoldenVariety(t, goldenWidePath, goldenLines(corpusWide()))
}

func checkGoldenVariety(t *testing.T, path string, got []string) {
	t.Helper()
	goldenArch(t)
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", path, len(got))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/layout/variety -run Golden -update", err)
	}
	want := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(want) != len(got) {
		t.Fatalf("строк в эталоне %d, получили %d", len(want), len(got))
	}
	bad := 0
	for i := range got {
		if got[i] != want[i] {
			bad++
			if bad <= 5 {
				t.Errorf("расхождение:\n  есть   %s\n  эталон %s", got[i], want[i])
			}
		}
	}
	if bad > 0 {
		t.Errorf("разошлось %d из %d строк", bad, len(got))
	}
}
