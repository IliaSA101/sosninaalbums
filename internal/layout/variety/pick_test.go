package variety

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cand — синтетический кандидат: точные оценки и свойства без перебора деревьев.
func cand(key, look string, score float64, inRow bool) Cand {
	return Cand{
		Result: layout.Result{Key: key, Score: score, Rects: make([]layout.Rect, 4)},
		Info:   Info{Family: Dense, Look: look, LookID: lookID(look), Fill: 0.8, MinSide: 100, Big: -1},
		InRow:  inRow,
	}
}

func airCand(key string, score, fill, minSide float64) Cand {
	c := cand(key, "air-"+key, score, false)
	c.Family, c.Fill, c.MinSide = Air, fill, minSide
	return c
}

func heroCand(key string, score float64, big int, inRow bool) Cand {
	c := cand(key, "hero-"+key, score, inRow)
	c.Family, c.Big = Hero, big
	return c
}

func pickOne(lv Level, c Ctx, s Spread) int {
	return Pick([]Spread{s}, geomSq, lv, c)[0]
}

// TestPickLessIsBest: «Меньше» — всегда лучшая (0), пул не смотрится; пустой пул — −1.
func TestPickLessIsBest(t *testing.T) {
	for _, a := range corpus() {
		for i, k := range Pick(a.spreads, a.p, Less, Ctx{Prev: []string{"x"}, Next: "y", Since: 9}) {
			if k != 0 {
				t.Fatalf("%s, разворот %d: Меньше выбрал %d", a.name, i, k)
			}
		}
	}
	got := Pick([]Spread{{Pool: nil, Star: -1}, {Pool: []Cand{cand("a", "a", 1, true)}, Star: -1}}, geomSq, Less, Ctx{})
	if !slices.Equal(got, []int{-1, 0}) {
		t.Errorf("пустой пул: %v", got)
	}
	if got := Pick(nil, geomSq, Normal, Ctx{}); len(got) != 0 {
		t.Errorf("нет разворотов: %v", got)
	}
}

// TestPickDeterministic: тот же вход — тот же выбор, а пулы на холодном кэше и при параллельном
// счёте — те же; уровень вне 0…2 зажимается.
func TestPickDeterministic(t *testing.T) {
	a := corpus()[1] // серии 80/20 m8 s2: есть и воздух, и звёзды
	want := Pick(a.spreads, a.p, Normal, Ctx{})
	if got := Pick(a.spreads, a.p, Normal, Ctx{}); !slices.Equal(got, want) {
		t.Fatal("повторный Pick дал другое")
	}
	layout.ResetCache()
	fresh := make([]Spread, len(a.spreads))
	var wg sync.WaitGroup
	for i := len(a.spreads) - 1; i >= 0; i-- { // в обратном порядке и параллельно
		wg.Go(func() {
			fresh[i] = Spread{Pool: corpusPool(a.asps[i], a.p), Star: a.spreads[i].Star}
		})
	}
	wg.Wait()
	if got := Pick(fresh, a.p, Normal, Ctx{}); !slices.Equal(got, want) {
		t.Fatal("пулы на холодном кэше дали другой выбор")
	}
	if !slices.Equal(Pick(a.spreads, a.p, Level(7), Ctx{}), Pick(a.spreads, a.p, More, Ctx{})) {
		t.Error("уровень 7 = Больше")
	}
	if !slices.Equal(Pick(a.spreads, a.p, Level(-1), Ctx{}), make([]int, len(a.spreads))) {
		t.Error("уровень −1 = Меньше")
	}
}

// TestPickBounds: каждый выбор на корпусе — в своём допуске: ряд в пределах δ, звезда в пределах δ★
// (на 8–16 фото — по Fair от потолка), воздух по ритму и со своими порогами, featured (часть 2) —
// звезда, по ритму главного не после главного или «даром». Допуски переписаны здесь независимо
// от кода выбора.
func TestPickBounds(t *testing.T) {
	feats := 0
	for _, a := range slices.Concat(corpus(), featCorpus()) {
		for _, lv := range []Level{Normal, More} {
			r := rhythm{}
			for i, k := range Pick(a.spreads, a.p, lv, Ctx{}) {
				s := a.spreads[i]
				x := s.Pool[k]
				if x.Kind != NoKind {
					feats++
				}
				if !r.admitted(tunings[lv], s, &x, a.p) {
					t.Fatalf("%s, уровень %d, разворот %d: выбор %q вне допусков (потеря %.3f, по Fair %.3f)",
						a.name, lv, i, x.Key, s.Pool[0].Score-x.Score, min(s.Pool[0].Score, 0.72)-x.Fair)
				}
				r.push(&x)
			}
		}
	}
	if feats == 0 {
		t.Error("ни один разворот не взял раскладку с главным фото")
	}
}

// rhythm — ритмы цепочки для TestPickBounds, независимо от кода выбора.
type rhythm struct {
	since, sinceHero int
	prevHero         bool
}

func (r *rhythm) push(x *Cand) {
	r.since++
	if x.Family == Air {
		r.since = 0
	}
	r.sinceHero++
	if r.prevHero = x.Heroes > 0; r.prevHero {
		r.sinceHero = 0
	}
}

// admitted — выбор x в своём допуске: ряд в пределах δ, звезда-главное в пределах δ★ (на 8–16 фото —
// по Fair от потолка 0.72), воздух по ритму, featured — звезда, ритм главного не после главного
// или «даром» (сырая потеря ≤ 0.02).
func (r *rhythm) admitted(tn tuning, s Spread, x *Cand, p layout.Params) bool {
	const eps = 1e-8
	best := s.Pool[0]
	loss, fairLoss := best.Score-x.Score, min(best.Score, 0.72)-x.Fair
	if x.Kind != NoKind {
		if s.Star >= 0 {
			return x.Big == s.Star && fairLoss <= tn.deltaStar+eps
		}
		return !r.prevHero && (r.sinceHero+1 >= tn.heroPeriod && fairLoss <= tn.deltaHero+eps || loss <= 0.02+eps)
	}
	starLoss := loss
	if len(x.Rects) >= 8 {
		starLoss = min(loss, fairLoss)
	}
	due := r.since+1 >= tn.period && s.Star < 0 && len(best.Rects) <= tn.airMaxK
	return x.InRow && loss <= tn.delta+eps ||
		s.Star >= 0 && x.Big == s.Star && starLoss <= tn.deltaStar+eps ||
		due && x.Family == Air && x.MinSide >= tn.airMinSide*p.PageH && x.Fill >= tn.airFloor &&
			(best.Score-best.Fill)-(x.Score-x.Fill) <= tn.airLoss+eps
}

// TestPickThresholds: границы допусков на точных числах (δ, δ★, воздух) — ровно на пороге берём,
// на шаг дальше — нет.
func TestPickThresholds(t *testing.T) {
	best := cand("best", "same", 1.0, true)
	ctx := Ctx{Prev: []string{"same"}} // лучший повторяет соседа: любой допустимый другой выигрывает
	for _, c := range []struct {
		name string
		lv   Level
		alt  Cand
		want int
	}{
		{"Обычное: потеря ровно δ", Normal, cand("alt", "other", 0.96, true), 1},
		{"Обычное: потеря чуть больше δ", Normal, cand("alt", "other", 0.9599, true), 0},
		{"Больше: потеря ровно δ", More, cand("alt", "other", 0.92, true), 1},
		{"Больше: потеря чуть больше δ", More, cand("alt", "other", 0.9199, true), 0},
		{"вне ряда без звезды и воздуха не берём", Normal, cand("alt", "other", 1.0, false), 0},
		{"тот же вид — не лучше", Normal, cand("z", "same", 1.0, true), 0},
	} {
		if got := pickOne(c.lv, ctx, Spread{Pool: []Cand{best, c.alt}, Star: -1}); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
	star := func(loss float64, big int) Cand { return heroCand("star", 1.0-loss, big, false) }
	for _, c := range []struct {
		name string
		lv   Level
		alt  Cand
		want int
	}{
		{"звезда: потеря ровно δ★", Normal, star(0.10, 2), 1},
		{"звезда: потеря чуть больше δ★", Normal, star(0.1001, 2), 0},
		{"звезда: Больше — ровно δ★", More, star(0.15, 2), 1},
		{"главное — не звезда", Normal, star(0.01, 1), 0},
		{"главного нет (−1)", Normal, star(0.01, -1), 0},
	} {
		// у лучшей тот же вид, что у соседа, и звезды нет — штраф «звезду перекрывает» проверяет следующий тест
		if got := pickOne(c.lv, ctx, Spread{Pool: []Cand{best, c.alt}, Star: 2}); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
}

// TestPickAir: воздух по ритму и пороги допуска.
func TestPickAir(t *testing.T) {
	best := cand("best", "b", 1.0, true) // rest = 1.0 − 0.8 = 0.2
	page := geomSq.PageH
	air := func(score, fill, minSide float64) Cand { return airCand("a", score, fill, minSide) }
	for _, c := range []struct {
		name  string
		lv    Level
		since int
		alt   Cand
		star  int
		want  int
	}{
		{"Обычное: 5-й — пора", Normal, 4, air(0.6, 0.45, 0.17*page), -1, 1},
		{"Обычное: 4-й — рано", Normal, 3, air(0.6, 0.45, 0.17*page), -1, 0},
		{"Больше: 3-й — пора", More, 2, air(0.6, 0.30, 0.14*page), -1, 1},
		{"Больше: 2-й — рано", More, 1, air(0.6, 0.30, 0.14*page), -1, 0},
		{"пол заполнения: ровно", Normal, 4, air(0.6, 0.40, 0.17*page), -1, 1},
		{"пол заполнения: ниже", Normal, 4, air(0.6, 0.3999, 0.17*page), -1, 0},
		{"мелкое фото: ниже порога", Normal, 4, air(0.6, 0.45, 0.17*page-0.01), -1, 0},
		{"rest-потеря ровно δ_air", Normal, 4, air(0.57, 0.45, 0.17*page), -1, 1}, // rest 0.12 → потеря 0.08
		{"rest-потеря больше δ_air", Normal, 4, air(0.5699, 0.45, 0.17*page), -1, 0},
		{"есть звезда — воздуха нет", Normal, 4, air(0.6, 0.45, 0.17*page), 3, 0},
	} {
		if got := pickOne(c.lv, Ctx{Since: c.since}, Spread{Pool: []Cand{best, c.alt}, Star: c.star}); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
	// у 13 фото воздуха нет на «Обычном» (k_air = 12) и есть на «Больше» (16)
	big := best
	big.Rects = make([]layout.Rect, 13)
	spread := Spread{Pool: []Cand{big, air(0.6, 0.45, 0.17*page+5)}, Star: -1}
	if pickOne(Normal, Ctx{Since: 9}, spread) != 0 {
		t.Error("13 фото: на «Обычном» воздуха нет")
	}
	spread.Pool[1] = air(0.6, 0.30, 0.14*page+5)
	if pickOne(More, Ctx{Since: 9}, spread) != 1 {
		t.Error("13 фото: на «Больше» воздух возможен")
	}
}

// TestPickStarWins: звезда перекрывает любые штрафы соседей; но воздух не по ритму — штраф.
func TestPickStarWins(t *testing.T) {
	best := cand("best", "b", 1.0, true)
	hero := heroCand("hero", 0.95, 2, false)
	ctx := Ctx{Prev: []string{"hero-hero", "hero-hero"}, Next: "hero-hero", Cur: []string{"hero-hero"}}
	if got := pickOne(Normal, ctx, Spread{Pool: []Cand{best, hero}, Star: 2}); got != 1 {
		t.Errorf("звезда должна перекрыть штрафы соседей, выбран %d", got)
	}
	// без звезды тот же кандидат вне ряда не допустим вовсе
	if got := pickOne(Normal, Ctx{}, Spread{Pool: []Cand{best, hero}, Star: -1}); got != 0 {
		t.Errorf("без звезды выбран %d", got)
	}
	// воздух не по ритму (в ряду, fill < 0.55): штраф, лучший выигрывает при равных видах
	offBeat := cand("row-air", "row-air", 0.99, true)
	offBeat.Family, offBeat.Fill = Air, 0.4
	if got := pickOne(Normal, Ctx{Since: 0}, Spread{Pool: []Cand{best, offBeat}, Star: -1}); got != 0 {
		t.Errorf("воздух не по ритму выбран %d", got)
	}
	// штраф +1 слабее повтора вида +2: воздух не запрещён, а лишь дороже
	if got := pickOne(Normal, Ctx{Prev: []string{"b"}}, Spread{Pool: []Cand{best, offBeat}, Star: -1}); got != 1 {
		t.Errorf("воздух из ряда вместо повтора вида: выбран %d", got)
	}
}

// TestPickFamilyPenalty: то же семейство, что у предыдущего, штрафуется — смена характера.
func TestPickFamilyPenalty(t *testing.T) {
	mk := func(a, b string) Spread {
		return Spread{Pool: []Cand{cand(a, a, 1.0, true), heroCand(b, 0.99, 1, true)}, Star: -1}
	}
	got := Pick([]Spread{mk("d1", "h1"), mk("d2", "h2")}, geomSq, Normal, Ctx{})
	if !slices.Equal(got, []int{0, 1}) {
		t.Errorf("плотный, затем с главным: %v", got)
	}
	// семейство соседа слева (Ctx.Family) штрафует уже первый разворот цепочки
	if got := pickOne(Normal, Ctx{Family: Dense}, mk("d1", "h1")); got != 1 {
		t.Errorf("слева плотный: выбран %d, ждали с главным", got)
	}
	if got := pickOne(Normal, Ctx{Family: Hero}, mk("d1", "h1")); got != 0 {
		t.Errorf("слева с главным: выбран %d, ждали плотный", got)
	}
}

// TestPickNeighbors: вид не повторяет предыдущего и позапрошлого, следующего и текущего.
func TestPickNeighbors(t *testing.T) {
	mk := func() Spread {
		return Spread{Pool: []Cand{cand("a", "A", 1.0, true), cand("b", "B", 0.99, true), cand("c", "C", 0.98, true)}, Star: -1}
	}
	for _, c := range []struct {
		name string
		ctx  Ctx
		want int
	}{
		{"нет соседей — лучшая", Ctx{}, 0},
		{"как предыдущий", Ctx{Prev: []string{"A"}}, 1},
		{"предыдущий и позапрошлый", Ctx{Prev: []string{"B", "A"}}, 2},
		{"позапрошлый штрафуется слабее предыдущего", Ctx{Prev: []string{"A", "B"}}, 2}, // A +1, B +2, C 0
		{"следующий", Ctx{Next: "A"}, 1},
		{"текущий вид", Ctx{Cur: []string{"A"}}, 1},
		{"из истории берутся два последних", Ctx{Prev: []string{"A", "A", "A", "C"}}, 1}, // A +1, C +2, B 0
	} {
		if got := pickOne(Normal, c.ctx, mk()); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
	// у цепочки из трёх: Next относится только к последнему, Cur — к своему развороту
	chain := []Spread{mk(), mk(), mk()}
	got := Pick(chain, geomSq, Normal, Ctx{Next: "A"})
	looks := make([]string, len(got))
	for i, k := range got {
		looks[i] = chain[i].Pool[k].Look
	}
	if want := []string{"A", "B", "C"}; !slices.Equal(looks, want) {
		// A, затем B (не A), затем C (не B, не A позапрошлый) и Next = A тоже не мешает
		t.Errorf("цепочка: %v, ждали %v", looks, want)
	}
	got = Pick(chain[:2], geomSq, Normal, Ctx{Cur: []string{"A", "B"}})
	if got[0] == 0 || chain[1].Pool[got[1]].Look == "B" {
		t.Errorf("текущий вид считается повтором: %v", got)
	}
	// пропущенный разворот (пустой пул) историю не меняет
	skip := []Spread{mk(), {Star: -1}, mk()}
	got = Pick(skip, geomSq, Normal, Ctx{})
	if got[1] != -1 || got[0] != 0 || got[2] != 1 {
		t.Errorf("пропуск пустого пула: %v", got)
	}
}

// TestPickNeighborsCorpus (спека, критерии; решение 0.11 №41 — после G3 видов ≥ 6): 96
// горизонтальных на 12 разворотов, «Обычное» — соседних повторов вида 0 (у «Меньше» 6), видов ≥ 6
// (у «Меньше» 2).
func TestPickNeighborsCorpus(t *testing.T) {
	for _, a := range corpus() {
		if !strings.HasPrefix(a.name, "гориз.") {
			continue
		}
		less, normal := measure(a, Less), measure(a, Normal)
		if less.repeats != 6 || less.looks != 2 {
			t.Errorf("%s: «Меньше» — повторов %d, видов %d (ждали 6 и 2): корпус другой", a.name, less.repeats, less.looks)
		}
		if normal.repeats != 0 || normal.looks < 6 {
			t.Errorf("%s: «Обычное» — повторов %d, видов %d", a.name, normal.repeats, normal.looks)
		}
	}
}

// TestPickChainEdges: соседи цепочки на настоящих пулах — VaryLayouts (Cur) меняет вид там, где
// есть допустимый другой; Prev/Next не дают повторить.
func TestPickChainEdges(t *testing.T) {
	var a corpusAlbum
	for _, x := range corpus() {
		if strings.HasPrefix(x.name, "гориз.") {
			a = x
			break
		}
	}
	cur := make([]string, len(a.spreads))
	for i, k := range Pick(a.spreads, a.p, Less, Ctx{}) {
		cur[i] = a.spreads[i].Pool[k].Look
	}
	// текущий вид — штраф, а не запрет: у соседей вид тоже штрафуется, и бывает, что оставить
	// текущий дешевле всех; но подавляющее большинство разворотов меняется
	got := Pick(a.spreads, a.p, Normal, Ctx{Cur: cur})
	kept := 0
	for i, k := range got {
		if a.spreads[i].Pool[k].Look == cur[i] {
			kept++
		}
	}
	if kept*4 > len(got) {
		t.Errorf("текущий вид остался у %d из %d разворотов", kept, len(got))
	}
	t.Logf("текущий вид остался у %d из %d", kept, len(got))
	prev := cur[0]
	if k := pickOne(Normal, Ctx{Prev: []string{prev}}, a.spreads[0]); a.spreads[0].Pool[k].Look == prev {
		t.Error("первый разворот повторил вид перед собой")
	}
	if k := pickOne(Normal, Ctx{Next: prev}, a.spreads[0]); a.spreads[0].Pool[k].Look == prev {
		t.Error("последний разворот повторил вид после себя")
	}
}

// TestStarOf: единственная высшая оценка ≥ 4★; края — пусто, нули, ничья, два 5★.
func TestStarOf(t *testing.T) {
	for i, c := range []struct {
		r    []int
		want int
	}{
		{nil, -1}, {[]int{4}, 0}, {[]int{0, 0, 0}, -1}, {[]int{3, 3, 3}, -1}, {[]int{4, 3, 3}, 0},
		{[]int{3, 3, 4}, 2}, {[]int{4, 4, 3}, -1}, {[]int{5, 4, 4}, 0}, {[]int{5, 5, 4}, -1}, {[]int{1, 2, 3, 2}, -1},
	} {
		if got := Star(c.r); got != c.want {
			t.Errorf("Star #%d %v = %d, ждали %d", i, c.r, got, c.want)
		}
	}
}

// TestPickStar: звезда — главное везде на корпусе, где пул это позволяет (в пределах δ★).
func TestPickStar(t *testing.T) {
	possible, got := 0, 0
	for _, a := range corpus() {
		for _, lv := range []Level{Normal, More} {
			tn := tunings[lv]
			for i, k := range Pick(a.spreads, a.p, lv, Ctx{}) {
				s := a.spreads[i]
				if s.Star < 0 {
					continue
				}
				ok := false
				for _, c := range s.Pool {
					ok = ok || c.Big == s.Star && s.Pool[0].Score-c.Score <= tn.deltaStar-1e-8
				}
				if !ok {
					continue
				}
				possible++
				if s.Pool[k].Big == s.Star {
					got++
				} else {
					t.Errorf("%s, уровень %d, разворот %d: звезда не главная, хотя пул позволяет", a.name, lv, i)
				}
			}
		}
	}
	if possible == 0 {
		t.Error("в корпусе не нашлось ни одного разворота, где звезда может быть главной")
	}
	t.Logf("звезда главная: %d из %d возможных", got, possible)
}

// TestPickStarFour (design §10): 5★ у первого из 4 горизонтальных — главное (≥ 1.6× второго).
func TestPickStarFour(t *testing.T) {
	asp := sameAspects(4, aspL)
	s := Spread{Pool: Pool(asp, geomSq, 24, Normal), Star: Star([]int{5, 0, 0, 0})}
	x := s.Pool[pickOne(Normal, Ctx{}, s)]
	if x.Big != 0 {
		t.Fatalf("5★ у первого из 4 горизонтальных: главное %d (%s)", x.Big, x.Key)
	}
	area := func(i int) float64 { return x.Rects[i].W * x.Rects[i].H }
	for i := 1; i < 4; i++ {
		if area(0) < 1.6*area(i) {
			t.Errorf("главное меньше 1.6× фото %d", i)
		}
	}
}

// TestPickSpeed: Pick на 100 разворотах (тёплые пулы) — design §10, ≤ 50 мс.
func TestPickSpeed(t *testing.T) {
	in := bigAlbum(100)
	d := testkit.Fastest(3, func() { Pick(in, geomSq, Normal, Ctx{}) })
	t.Logf("Pick на %d разворотах: %v", len(in), d)
	if lim := testkit.Limit(50 * time.Millisecond); d > lim {
		t.Fatalf("Pick %v > %v", d, lim)
	}
}

var (
	bigOnce sync.Once
	bigData []Spread
)

// bigAlbum — n разворотов по 12–16 фото (смесь), пулы посчитаны и закэшированы.
func bigAlbum(n int) []Spread {
	bigOnce.Do(func() {
		asp := mixedAspects(1500, 1)
		pl := layout.PlanGroups(asp, geomSq, 100, 16)
		bigData = make([]Spread, len(pl.Groups))
		var wg sync.WaitGroup
		for i, g := range pl.Groups {
			wg.Go(func() { bigData[i] = Spread{Pool: Pool(asp[g[0]:g[1]], geomSq, 24, Normal), Star: -1} })
		}
		wg.Wait()
	})
	return bigData[:min(n, len(bigData))]
}

// BenchmarkPick — 100 разворотов, тёплый кэш (цель ≤ 50 мс).
func BenchmarkPick(b *testing.B) {
	in := bigAlbum(100)
	b.ResetTimer()
	for range b.N {
		Pick(in, geomSq, Normal, Ctx{})
	}
}

// BenchmarkPoolAlbum — пулы 100 разворотов (1500 фото, 12–16 на развороте) на холодном кэше,
// последовательно: цена «Обычного» сверх «Меньше» (design §10: ≤ 1.3× холодный кэш).
func BenchmarkPoolAlbum(b *testing.B) {
	asp := mixedAspects(1500, 1)
	pl := layout.PlanGroups(asp, geomSq, 100, 16)
	for _, lv := range []Level{Less, Normal} {
		b.Run([]string{"less", "normal"}[lv], func(b *testing.B) {
			for range b.N {
				layout.ResetCache()
				ResetCache() // пул берёт «Другие варианты» из своего кэша
				for _, g := range pl.Groups {
					Pool(asp[g[0]:g[1]], geomSq, 24, lv)
				}
			}
		})
	}
}

// BenchmarkPool16 — пул одного разворота из 16 фото на холодном кэше: Меньше (только ряд) и Обычное.
func BenchmarkPool16(b *testing.B) {
	asp := mixedAspects(16, 3)
	for _, lv := range []Level{Less, Normal} {
		b.Run([]string{"less", "normal"}[lv], func(b *testing.B) {
			for range b.N {
				layout.ResetCache()
				ResetCache() // пул берёт «Другие варианты» из своего кэша
				Pool(asp, geomSq, 24, lv)
			}
		})
	}
}
