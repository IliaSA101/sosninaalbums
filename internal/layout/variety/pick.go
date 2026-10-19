package variety

import (
	"math"
	"slices"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// Spread — разворот для выбора: пул кандидатов (Pool; [0] — лучшая раскладка «Авто») и звезда.
type Spread struct {
	Pool []Cand
	Star int // индекс фото-звезды группы; −1 — нет (см. Star)
}

// Ctx — соседи цепочки разворотов. Виды — строки Look.
type Ctx struct {
	Prev []string // виды 1–2 разворотов перед первым (последний — ближний)
	// PrevKey — ключ разворота перед первым: тот же ключ на чуть других пропорциях — другой вид,
	// но та же схема, и рядом она читается повтором (QA 0.11 на 127 живых фото).
	PrevKey string
	Family  Family   // семейство разворота перед первым ("" — нет): штраф семейства и у первого цепочки
	Next    string   // вид разворота после последнего ("" — нет)
	Cur     []string // VaryLayouts: текущий вид каждого разворота цепочки (nil — нет); тоже считается повтором
	Since   int      // разворотов без воздуха перед первым
	// Hero — у разворота перед первым главное фото (Info.Heroes > 0); SinceHero — разворотов без
	// главного перед первым (ритм главного, решение 0.11 №34).
	Hero      bool
	SinceHero int
	// Forms — формы разворотов перед первым (последняя — ближняя, учитываются FormsWindow); NextForm —
	// форма разворота после последнего ("" — нет). Avg — среднее фото на разворот альбома (0 — без
	// бонуса «под размер», 051 design §7).
	Forms    []forms.Form
	NextForm forms.Form
	Avg      float64
	// Ladder — направление ближайшей лесенки перед первым ('L', 'A', 0 — нет; forms.LadderDir):
	// «Разнообразить» части альбома продолжает чередование вниз/вверх (055).
	Ladder byte
	// Seed — номер варианта сборки (078): 0 и 1 — выбор как в 0.19 побитово; 2… — к штрафу
	// кандидата прибавляется детерминированный «шум» (seedNoise) и сдвигается ритм воздуха и
	// главного. Генератора случайных чисел нет: тот же номер на том же входе — тот же выбор.
	Seed int
}

// tuning — пороги уровня (спека 041, design §5, §10). Меняются только с прогоном
// TestVarietyMetrics -v и записью в decisions: подобраны на синтетическом корпусе.
type tuning struct {
	delta      float64 // допуск потери у ряда «Авто»: ничьи и почти-ничьи, дальше — заметно хуже
	deltaStar  float64 // допуск потери у раскладки, где звезда — главное (замер: теряет 0.06–0.21)
	period     int     // каждый period-й разворот — воздух (15–20 % разворотов; у SmartAlbums чаще)
	airMaxK    int     // воздух только до стольких фото: у 13+ он делает фото мелкими
	airMinSide float64 // мельчайшая сторона воздуха / высота страницы (51 / 42 мм на 30 см)
	airFloor   float64 // пол заполнения воздуха: «Обычное» — одна страница поменьше, «Больше» — и обе
	airLoss    float64 // допуск воздуха по оценке без заполнения (rest): по общей он проигрывает 0.3–0.6
	heroPeriod int     // каждый heroPeriod-й разворот — с главным фото из featured, если допустимо (№34)
	deltaHero  float64 // допуск featured по Fair от min(лучшая, heroCap) (решение №30)
	deltaForm  float64 // допуск формы ряда «Авто» по Q от Q лучшей (051 design §7)
	mosaicRun  int     // мозаик подряд не больше: дальше мозаика недопустима, если есть другая
}

var tunings = [...]tuning{
	Less: {},
	Normal: {delta: 0.04, deltaStar: 0.10, period: 5, airMaxK: 12, airMinSide: 0.17, airFloor: 0.40, airLoss: 0.08,
		heroPeriod: 4, deltaHero: 0.08, deltaForm: 0.12, mosaicRun: 3},
	More: {delta: 0.08, deltaStar: 0.15, period: 3, airMaxK: 16, airMinSide: 0.14, airFloor: 0.28, airLoss: 0.12,
		heroPeriod: 3, deltaHero: 0.12, deltaForm: 0.25, mosaicRun: 2},
}

// Бонусы и штрафы выбора (меньше — лучше). Звезда перекрывает любые штрафы соседей.
const (
	bonusStar     = -10.0
	bonusAir      = -3.0
	bonusHero     = -2.0 // слабее воздуха: оба по ритму — воздух, главное достанется следующему…
	bonusHeroLate = -4.0 // …если главное ждёт не дольше воздуха; иначе главное (периоды «Больше» равны)
	penSame1      = 2.0  // вид как у предыдущего (и у следующего, и у текущего)
	penSame2      = 1.0  // вид как у позапрошлого
	penSameFamily = 0.5
	penAirOffBeat = 1.0  // воздух не по ритму
	penSameKind   = 0.5  // семейство featured как у прошлого разворота с главным: семейства чередуются
	penSeen       = 0.25 // вид уже был в цепочке раньше позапрошлого: из равных берётся невиданный
	penSameKey    = 1.0  // ключ как у предыдущего при другом виде: та же схема на других пропорциях
	penForm1      = 1.5  // форма как у предыдущего (и у следующего), 051
	penForm2      = 0.75 // форма как у позапрошлого
	penFormOften  = 0.25 // форма уже ≥ formsOften раз в последних FormsWindow
	bonusSize     = -0.5 // форма под размер группы (sizeForm)
	// penHeroAgain — форма «С главным», вставленная в ряд «Авто» (Cand.Formed), сразу после разворота
	// с главным (051, №40): featured там запрещены (limits.feat), а форма — штраф крупнее обычных
	// штрафов соседей, но не запрет: на 3–4 фото «с главным» почти всё, и запрет оставил бы только
	// лучшую. Лучшая раскладка (Layout "") и ключи ряда layout штрафа не получают: ритм воздуха 041
	// остаётся как был (метрики корпуса №40). Звезда (bonusStar) штраф перекрывает.
	penHeroAgain = 6.0
	// penLadderDir — лесенка не того направления: ждём противоположное прошлой, без прошлой — L
	// (по ходу чтения, 055). L и A различает только он, так что лесенок в целом не меньше и не больше.
	penLadderDir = 0.25
	// seedAmp — размах «шума» варианта (078, design §7): меньше penSameKey (1.0), больше penSeen
	// (0.25) — перемешивает почти равные раскладки, но штрафы соседей (penSame1 2.0) не ломает.
	// Подобран по TestPickSeedDiffers / TestPickSeedKeepsRules; правка — записью в decisions.
	seedAmp = 0.6
	// seedLossK — вес потери оценки в варианте. Без него шум становится последним доводом вместо
	// «меньше потеря», и средняя потеря растёт на 30 % (замер на корпусе: размах шума 0,02…0,6 на это
	// не влияет); с весом 16 потеря на уровне варианта 1, а варианты отличаются у 42–45 % разворотов.
	seedLossK = 16.0
	// seedLossCap — потеря выше не штрафуется: воздух и главное допускаются своими правилами с большой потерей.
	seedLossCap = 0.04
	// seedLessLoss — «Меньше» с вариантом берёт любую раскладку ряда «Авто» с потерей не больше этой
	// (иначе у «Меньше» номер варианта ничего бы не менял).
	seedLessLoss = 0.02
)

const (
	// FormsWindow — сколько форм перед цепочкой знает выбор («форма ≥ 2 раз в последних 6», 051);
	// экспорт — spreads собирает Ctx.Forms по нему же.
	FormsWindow = 6
	formsOften  = 2
	// smallK, bigK — группа «редкая» при ≤ smallK·среднего фото, «плотная» при ≥ bigK·среднего.
	smallK = 0.7
	bigK   = 1.3
)

// sizeForm — форма под размер группы (design §7): редкому развороту — ленты, лесенка, мелкое —
// крупное, воздух; плотному — мозаика, ряды, разные страницы, крупное; сетка — обоим.
func sizeForm(f forms.Form, small, big bool) bool {
	switch f {
	case forms.Grid:
		return small || big
	case forms.Strip, forms.Ladder, forms.Contrast, forms.Air:
		return small
	case forms.Mosaic, forms.Rows, forms.Asym, forms.Hero:
		return big
	}
	return false
}

// heroFree — featured вне ритма берётся «даром», только если сырая оценка не хуже лучшей на
// столько (решение №34: по Fair «через корешок» бывает выше лучшей, и ритм перестал бы управлять).
const heroFree = 0.02

// tieEps — как в layout: оценки сравниваются в квантах, чтобы FMA на arm64 не менял выбор у порогов.
const tieEps = 1e-9

func quanta(x float64) float64 { return math.Round(x / tieEps) }

// Star — индекс единственной звезды: фото с наибольшей оценкой, если она ≥ 4 и такое одно
// (два 5★ — звезды нет). ratings — действующие оценки группы по порядку фото.
func Star(ratings []int) int {
	best, at, n := 3, -1, 0
	for i, r := range ratings {
		switch {
		case r > best:
			best, at, n = r, i, 1
		case r == best && at >= 0:
			n++
		}
	}
	if n != 1 {
		return -1
	}
	return at
}

// Pick — индекс выбранного кандидата в Pool каждого разворота (−1 — пул пуст, разворот
// пропускается и историю не меняет). Less — везде лучшая (0). Жадно слева направо,
// детерминированно: ряд «Авто» в пределах δ, звезда-главное, воздух по ритму; среди допустимых
// минимум штрафа за повтор вида соседей, затем потери, затем ключа.
func Pick(in []Spread, p layout.Params, lv Level, c Ctx) []int {
	out := make([]int, len(in))
	lv = lv.clamp()
	if lv == Less {
		for i, s := range in {
			if len(s.Pool) == 0 {
				out[i] = -1
			} else if c.Seed >= 2 {
				out[i] = lessSeeded(s, c.Seed, i)
			}
		}
		return out
	}
	t := tunings[lv]
	st := newChain(c)
	if c.Seed >= 2 { // ритм воздуха и главного сдвигается на номер варианта
		st.since += (c.Seed - 1) % t.period
		st.sinceHero += (c.Seed - 1) % t.heroPeriod
	}
	for i, s := range in {
		if len(s.Pool) == 0 {
			out[i] = -1
			continue
		}
		n := st.neighbors(t, s)
		if i == len(in)-1 {
			n.next, n.nextForm = c.Next, c.NextForm
		}
		if i < len(c.Cur) {
			n.cur = c.Cur[i]
		}
		n.seed, n.idx = c.Seed, i
		out[i] = t.one(s, p, &n)
		st.push(&s.Pool[out[i]])
	}
	return out
}

// chain — история цепочки слева направо: виды, ритмы воздуха и главного, прошлые ключ и семейства.
type chain struct {
	hist      []string        // виды выбранных (и Ctx.Prev), последний — ближний
	histID    []uint64        // их LookID
	older     map[uint64]bool // виды раньше позапрошлого
	prevKey   string
	prevFam   Family
	since     int // разворотов без воздуха
	sinceHero int // разворотов без главного
	prevHero  bool
	prevKind  Kind         // семейство featured последнего разворота с главным
	forms     []forms.Form // формы выбранных (и Ctx.Forms), последняя — ближняя
	avg       float64
	ladder    byte // направление последней лесенки (и Ctx.Ladder), 0 — не было
}

func newChain(c Ctx) *chain {
	st := &chain{hist: slices.Clone(c.Prev[max(0, len(c.Prev)-2):]), older: map[uint64]bool{}, prevKey: c.PrevKey,
		prevFam: c.Family, since: c.Since, sinceHero: c.SinceHero, prevHero: c.Hero,
		forms: slices.Clone(c.Forms[max(0, len(c.Forms)-FormsWindow):]), avg: c.Avg, ladder: c.Ladder}
	for _, l := range st.hist {
		st.histID = append(st.histID, lookID(l))
	}
	return st
}

// neighbors — соседи и ритмы для очередного разворота s (next и cur заполняет Pick).
func (st *chain) neighbors(t tuning, s Spread) neighbors {
	n := neighbors{prevFam: st.prevFam, prevHero: st.prevHero, prevKind: st.prevKind, older: st.older, prevKey: st.prevKey,
		heroDue: st.sinceHero+1 >= t.heroPeriod && s.Star < 0,
		airDue:  st.since+1 >= t.period && s.Star < 0 && len(s.Pool[0].Rects) <= t.airMaxK}
	n.heroLate = n.heroDue && st.sinceHero > st.since
	n.ladder = st.ladder
	n.recent = st.forms[max(0, len(st.forms)-FormsWindow):]
	if f := len(st.forms); f >= 1 {
		n.form1 = st.forms[f-1]
		if f >= 2 {
			n.form2 = st.forms[f-2]
		}
	}
	for i := len(st.forms) - 1; i >= 0 && st.forms[i] == forms.Mosaic; i-- {
		n.mosaics++
	}
	if k := float64(len(s.Pool[0].Rects)); st.avg > 0 {
		n.small, n.big = quanta(k) <= quanta(smallK*st.avg), quanta(k) >= quanta(bigK*st.avg)
	}
	if h := len(st.hist); h >= 1 {
		n.prev1 = st.hist[h-1]
		if h >= 2 {
			n.prev2 = st.hist[h-2]
		}
	}
	return n
}

// push — выбран x: он становится ближним соседом, ритмы сдвигаются.
func (st *chain) push(x *Cand) {
	if h := len(st.histID); h >= 2 {
		st.older[st.histID[h-2]] = true // уходит из окна соседей — в «уже были»
	}
	st.hist, st.histID = append(st.hist, x.Look), append(st.histID, x.LookID)
	st.prevFam, st.prevKey = x.Family, x.Key
	st.forms = append(st.forms, x.Form)
	if d := ladderDir(x); d != 0 {
		st.ladder = d
	}
	if x.Family == Air {
		st.since = 0
	} else {
		st.since++
	}
	if st.prevHero = x.Heroes > 0; st.prevHero {
		st.sinceHero = 0
		if x.Kind != NoKind {
			st.prevKind = x.Kind
		}
	} else {
		st.sinceHero++
	}
}

// neighbors — что учитывает выбор одного разворота.
type neighbors struct {
	prev1, prev2, next, cur string
	prevFam                 Family
	airDue                  bool
	prevHero, heroDue       bool            // у предыдущего главное; пора главного (звезды нет)
	heroLate                bool            // главное ждёт дольше воздуха: при обоих по ритму — оно
	older                   map[uint64]bool // виды цепочки раньше позапрошлого (LookID)
	prevKey                 string          // ключ предыдущего
	prevKind                Kind            // семейство featured последнего разворота с главным
	form1, form2, nextForm  forms.Form      // формы предыдущего, позапрошлого и следующего ("" — нет)
	recent                  []forms.Form    // формы последних FormsWindow разворотов
	mosaics                 int             // мозаик подряд перед этим разворотом
	small, big              bool            // группа редкая / плотная против среднего альбома
	ladder                  byte            // направление последней лесенки, 0 — не было
	seed, idx               int             // номер варианта и место разворота в цепочке (шум seedNoise)
}

// ladderDir — направление лесенки кандидата ('L', 'A'), 0 — не лесенка.
func ladderDir(x *Cand) byte {
	if x.Form != forms.Ladder {
		return 0
	}
	return forms.LadderDir(x.Key)
}

// one — выбор одного разворота: допустимые кандидаты, среди них минимум (штраф, потеря, ключ).
func (t tuning) one(s Spread, p layout.Params, n *neighbors) int {
	best := &s.Pool[0]
	bestQ := quanta(best.Score)
	lim := limits{
		bestRest: quanta(best.Score - best.Fill), capQ: quanta(min(best.Score, heroCap)),
		delta: quanta(t.delta), star: quanta(t.deltaStar), air: quanta(t.airLoss), hero: quanta(t.deltaHero),
		free: quanta(heroFree), form: quanta(best.Q - t.deltaForm),
	}
	// Мозаик подряд уже предел — мозаика берётся, только если другого допустимого нет.
	capped := n.mosaics >= t.mosaicRun
	at, atPen, atLoss := -1, 0.0, 0.0
	mo, moPen, moLoss := -1, 0.0, 0.0
	for i := range s.Pool {
		x := &s.Pool[i]
		loss := bestQ - quanta(x.Score)
		var bonus float64
		var ok bool
		if x.Kind != NoKind {
			bonus, ok = lim.feat(x, s.Star, loss, n)
		} else {
			bonus, ok = t.admit(x, s.Star, loss, &lim, p, n)
		}
		if !ok {
			continue
		}
		pen := bonus + n.penalty(x) + n.noise(x, loss*tieEps)
		if capped && x.Form == forms.Mosaic {
			if better(s.Pool, mo, moPen, moLoss, i, pen, loss) {
				mo, moPen, moLoss = i, pen, loss
			}
			continue
		}
		if better(s.Pool, at, atPen, atLoss, i, pen, loss) {
			at, atPen, atLoss = i, pen, loss
		}
	}
	if at < 0 {
		at = mo
	}
	return max(at, 0) // [0] допустим всегда: ряд, потеря 0
}

// better — кандидат i лучше стоящего at: меньше штраф, затем потеря, затем ключ.
func better(pool []Cand, at int, atPen, atLoss float64, i int, pen, loss float64) bool {
	return at < 0 || pen < atPen || pen == atPen && (loss < atLoss || loss == atLoss && pool[i].Key < pool[at].Key)
}

// limits — пороги разворота в квантах tieEps (считаются раз на разворот, а не на кандидата).
type limits struct {
	bestRest, capQ               float64 // лучшая без заполнения; min(лучшая, heroCap)
	delta, star, air, hero, free float64 // δ, δ★, δ_air, δ_hero, heroFree
	form                         float64 // пол Q формы: Q лучшей − δ_form
}

// admit — допустим ли кандидат (не featured) и какой у него бонус. loss — сырая потеря в квантах.
// На 8–16 фото звезда-главное судится по Fair, как featured (решение №30): штраф разброса за
// главное там и есть то, что мешает ему быть крупным. Fair ≥ Score, потолок ≤ лучшей — допуск
// только шире прежнего.
func (t tuning) admit(x *Cand, star int, loss float64, lim *limits, p layout.Params, n *neighbors) (bonus float64, ok bool) {
	if x.InRow && loss <= lim.delta || x.Formed && quanta(x.Q) >= lim.form {
		ok = true
	}
	if star >= 0 && x.Big == star {
		starLoss := loss
		if len(x.Rects) >= FeatMin {
			starLoss = min(loss, lim.capQ-quanta(x.Fair))
		}
		if starLoss <= lim.star {
			ok, bonus = true, bonusStar
		}
	}
	if n.airDue && x.Family == Air && x.MinSide >= t.airMinSide*p.PageH && x.Fill >= t.airFloor &&
		lim.bestRest-quanta(x.Score-x.Fill) <= lim.air {
		ok, bonus = true, min(bonus, bonusAir)
	}
	return bonus, ok
}

// feat — допуск раскладки с главным фото (featured, решения №30, №34): потеря судится по Fair
// от min(лучшая, heroCap). Звезда — только если главное она, и тогда без оглядки на соседа; без
// звезды — не после разворота с главным: по ритму с бонусом или «даром» (сырая потеря ≤ heroFree).
// Вид соседа не повторяет никогда: два одинаковых разворота с главным подряд (одна звезда на
// одинаковых сериях) заметнее, чем звезда не крупно.
func (lim *limits) feat(x *Cand, star int, loss float64, n *neighbors) (bonus float64, ok bool) {
	fairLoss := lim.capQ - quanta(x.Fair)
	switch {
	case x.Look == n.prev1 || x.Look == n.next:
	case star >= 0:
		if x.Big == star && fairLoss <= lim.star {
			return bonusStar, true
		}
	case n.prevHero:
	case n.heroDue && fairLoss <= lim.hero:
		if n.heroLate {
			return bonusHeroLate, true
		}
		return bonusHero, true
	case loss <= lim.free:
		return 0, true
	}
	return 0, false
}

// penalty — штраф за повтор вида и семейства соседей, за воздух не по ритму и за направление лесенки.
func (n *neighbors) penalty(x *Cand) float64 {
	pen := 0.0
	for _, v := range [...]struct {
		look string
		pen  float64
	}{{n.prev1, penSame1}, {n.prev2, penSame2}, {n.next, penSame1}, {n.cur, penSame1}} {
		if v.look != "" && x.Look == v.look {
			pen += v.pen
		}
	}
	if n.prevFam != "" && x.Family == n.prevFam {
		pen += penSameFamily
	}
	if len(n.older) > 0 && n.older[x.LookID] {
		pen += penSeen
	}
	if n.prevKey != "" && x.Key == n.prevKey && x.Look != n.prev1 {
		pen += penSameKey
	}
	if x.Family == Air && !n.airDue {
		pen += penAirOffBeat
	}
	if x.Kind != NoKind && x.Kind == n.prevKind {
		pen += penSameKind
	}
	if n.prevHero && x.Formed && FamilyOf(x.Form) == Hero {
		pen += penHeroAgain
	}
	if d := ladderDir(x); d != 0 && d != n.wantLadder() {
		pen += penLadderDir
	}
	return pen + n.formPenalty(x.Form)
}

// wantLadder — ожидаемое направление лесенки: противоположное прошлой, без прошлой — L.
func (n *neighbors) wantLadder() byte {
	if n.ladder == 'L' {
		return 'A'
	}
	return 'L'
}

// formPenalty — 051: форма как у соседей и частая в последних разворотах, бонус формы под размер.
func (n *neighbors) formPenalty(f forms.Form) float64 {
	if f == "" {
		return 0
	}
	pen := 0.0
	if f == n.form1 || f == n.nextForm {
		pen += penForm1
	}
	if f == n.form2 {
		pen += penForm2
	}
	seen := 0
	for _, g := range n.recent {
		if g == f {
			seen++
		}
	}
	if seen >= formsOften {
		pen += penFormOften
	}
	if sizeForm(f, n.small, n.big) {
		pen += bonusSize
	}
	return pen
}

// seedNoise — «шум» варианта в [0, 1): FNV-1a 64 от (номер варианта, место в цепочке, ключ).
// Не зависит от порядка вызовов и параллелизма — один вход даёт один выбор.
func seedNoise(seed, idx int, key string) float64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	mix := func(b byte) { h = (h ^ uint64(b)) * prime }
	var buf [48]byte
	b := strconv.AppendInt(buf[:0], int64(seed), 10)
	b = append(b, '/')
	b = strconv.AppendInt(b, int64(idx), 10)
	b = append(b, '/')
	for _, c := range b {
		mix(c)
	}
	for i := range len(key) {
		mix(key[i])
	}
	return float64(h>>11) / (1 << 53)
}

// noise — добавка к штрафу кандидата; у вариантов 0 и 1 её нет.
func (n *neighbors) noise(x *Cand, loss float64) float64 {
	if n.seed < 2 {
		return 0
	}
	return seedAmp*seedNoise(n.seed, n.idx, x.Key) + seedLossK*min(max(loss, 0), seedLossCap)
}

// lessSeeded — «Меньше» с вариантом: из ряда «Авто» с потерей ≤ seedLessLoss берётся минимум
// шума (ничья — меньший индекс); лучшая раскладка [0] подходит всегда.
func lessSeeded(s Spread, seed, idx int) int {
	bestQ := quanta(s.Pool[0].Score)
	at, atU := 0, math.Inf(1)
	for i := range s.Pool {
		x := &s.Pool[i]
		if !x.InRow && i != 0 || bestQ-quanta(x.Score) > quanta(seedLessLoss) {
			continue
		}
		if u := seedNoise(seed, idx, x.Key); u < atU {
			at, atU = i, u
		}
	}
	return at
}
