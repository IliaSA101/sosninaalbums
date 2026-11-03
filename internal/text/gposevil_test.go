package text

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"slices"

	"golang.org/x/image/font/sfnt"
)

// Генераторы злых и предельных таблиц GPOS для TestGPOSLimits, TestGPOSWorstCase и фаззов
// (аудит 161: произведение счётчиков на общих смещениях, худший запрос форматов 1 и 2).

// gbEmptyLookups — n пустых lookups одного общего указателя (разных по номеру).
func gbEmptyLookups(n int) []*gbLookup {
	return slices.Repeat([]*gbLookup{{typ: 2}}, n)
}

func gbNums(from, to int) []int {
	out := make([]int, 0, max(to-from, 0))
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

// gbBody — тело скрипта с одним DefaultLangSys на фичи feats.
func gbBody(feats ...int) *gbScriptBody {
	return &gbScriptBody{langs: []gbLang{{"", &gbLangSys{req: 0xFFFF, feats: feats}}}}
}

func gbTagN(i int) string { return fmt.Sprintf("s%03d", i) }

// limScripts — n скриптов, все на одно тело.
func limScripts(n int) *gposBuilder {
	body := gbBody(0)
	g := gbSimple("DFLT", gbEmptyLookups(1)...)
	g.scripts = g.scripts[:0]
	for i := range n {
		g.scripts = append(g.scripts, gbScript{gbTagN(i), body})
	}
	return g
}

// limLangSys — скрипт с n записями LangSys (разные теги, один общий LangSys).
func limLangSys(n int) *gposBuilder {
	g := gbSimple("DFLT", gbEmptyLookups(1)...)
	ls := g.scripts[0].body.langs[0].ls
	for i := range n {
		g.scripts[0].body.langs = append(g.scripts[0].body.langs, gbLang{fmt.Sprintf("L%03d", i), ls})
	}
	return g
}

// limFeatIdx — DefaultLangSys с n повторами индекса одной фичи kern.
func limFeatIdx(n int) *gposBuilder {
	g := gbSimple("DFLT", gbEmptyLookups(1)...)
	g.scripts[0].body.langs[0].ls.feats = slices.Repeat([]int{0}, n)
	return g
}

// limFeatLks — фича kern с n разными lookups.
func limFeatLks(n int) *gposBuilder {
	return gbSimple("DFLT", gbEmptyLookups(n)...)
}

// limScriptLks — n разных lookups у скрипта через две фичи kern (каждая не больше 64).
func limScriptLks(n int) *gposBuilder {
	g := gbSimple("DFLT", gbEmptyLookups(n)...)
	h := n / 2
	g.features = []gbFeature{{"kern", &gbFeatBody{lookups: gbNums(0, h)}}, {"kern", &gbFeatBody{lookups: gbNums(h, n)}}}
	g.scripts[0].body.langs[0].ls.feats = []int{0, 1}
	return g
}

// limLookups — n разных lookups: скрипты по 64 lookups, у каждого своя фича.
func limLookups(n int) *gposBuilder {
	g := &gposBuilder{lookups: gbEmptyLookups(n)}
	for from, i := 0, 0; from < n; from, i = from+64, i+1 {
		g.features = append(g.features, gbFeature{"kern", &gbFeatBody{lookups: gbNums(from, min(from+64, n))}})
		g.scripts = append(g.scripts, gbScript{gbTagN(i), gbBody(i)})
	}
	return g
}

// gbSkipSub — подтаблица с ValueFormat > 0xFF: разбор её пропускает, принятых нет.
func gbSkipSub() *gbSub { return &gbSub{format: 1, vf1: 0x100} }

// limSubsInLookup — один lookup с n записями подтаблиц (пропускаемых).
func limSubsInLookup(n int) *gposBuilder {
	return gbSimple("DFLT", gbLk(slices.Repeat([]*gbSub{gbSkipSub()}, n)...))
}

// subsLookups — lookups по 64 принятых подтаблицы sub, всего n.
func subsLookups(n int, sub *gbSub) []*gbLookup {
	var out []*gbLookup
	for ; n > 0; n -= 64 {
		out = append(out, gbLk(slices.Repeat([]*gbSub{sub}, min(n, 64))...))
	}
	return out
}

// limSubsScript — один скрипт, у него n принятых подтаблиц.
func limSubsScript(n int) *gposBuilder {
	return gbSimple("DFLT", subsLookups(n, gbKern([3]int{1, 2, -1}))...)
}

// limSubsTotal — скрипты по 512 подтаблиц у каждого (разные lookups), всего n.
func limSubsTotal(n int) *gposBuilder {
	sub := gbKern([3]int{1, 2, -1})
	g := &gposBuilder{}
	for i := 0; n > 0; i, n = i+1, n-512 {
		lks := subsLookups(min(n, 512), sub)
		from := len(g.lookups)
		g.lookups = append(g.lookups, lks...)
		g.features = append(g.features, gbFeature{"kern", &gbFeatBody{lookups: gbNums(from, from+len(lks))}})
		g.scripts = append(g.scripts, gbScript{gbTagN(i), gbBody(i)})
	}
	return g
}

// limAll — все измерения на пределе сразу, на общих смещениях: 64 скрипта на одно тело,
// 128 LangSys, 512 индексов фич, фича с 64 lookups, у скрипта 512 подтаблиц (64 lookups
// по 8), в одном lookup 1024 записи. over — измерение, которое поднято на единицу:
// scripts, langSys, featIdx, featLks, scriptLks, subs, subsInLookup ("" — ни одного).
func limAll(over string) *gposBuilder {
	plus := func(dim string, n int) int {
		if dim == over {
			return n + 1
		}
		return n
	}
	sub := gbKern([3]int{1, 2, -1})
	var lks []*gbLookup
	for range 64 {
		lks = append(lks, gbLk(slices.Repeat([]*gbSub{sub}, 8)...))
	}
	lks[0] = gbLk(append(slices.Repeat([]*gbSub{sub}, 8), slices.Repeat([]*gbSub{gbSkipSub()}, plus("subsInLookup", 1016))...)...)
	if over == "subs" {
		lks[1] = gbLk(slices.Repeat([]*gbSub{sub}, 9)...)
	}
	lks = append(lks, gbLk()) // 65-й lookup нужен только для +1 по lookups
	g := &gposBuilder{lookups: lks}
	g.features = []gbFeature{
		{"kern", &gbFeatBody{lookups: gbNums(0, plus("featLks", 64))}},
		{"kern", &gbFeatBody{lookups: []int{64}}}, // с ней у скрипта 65 разных lookups
	}
	feats := slices.Repeat([]int{0}, plus("featIdx", 512))
	if over == "scriptLks" {
		feats[1] = 1
	}
	body := gbBody(feats...)
	for i := range plus("langSys", 128) {
		body.langs = append(body.langs, gbLang{fmt.Sprintf("L%03d", i), body.langs[0].ls})
	}
	for i := range plus("scripts", 64) {
		g.scripts = append(g.scripts, gbScript{gbTagN(i), body})
	}
	return g
}

// bombFeatures — «featBomb» в обход пропуска повторов: k скриптов на одно тело, у него
// 512 разных индексов фич, и все они — одна фича kern с 64 lookups. Работа каждого скрипта
// ≈ 512 × 67 чтений; пределы счётчиков соблюдены, останавливает только бюджет чтений.
func bombFeatures(k int) *gposBuilder {
	g := &gposBuilder{lookups: gbEmptyLookups(64)}
	shared := &gbFeatBody{lookups: gbNums(0, 64)}
	for range 512 {
		g.features = append(g.features, gbFeature{"kern", shared})
	}
	body := gbBody(gbNums(0, 512)...)
	for i := range k {
		g.scripts = append(g.scripts, gbScript{gbTagN(i), body})
	}
	return g
}

// bombRaw — прежний featBomb аудита: 64 скрипта на одно тело, 512 индексов фичи 0, у фичи
// nl записей lookup (все = li). Сам по себе упирается в предел lookups у фичи.
func bombRaw(nl int) *gposBuilder {
	g := &gposBuilder{lookups: gbEmptyLookups(1), featuresLast: true}
	g.features = []gbFeature{{"kern", &gbFeatBody{lookups: slices.Repeat([]int{0}, nl)}}}
	body := gbBody(slices.Repeat([]int{0}, 512)...)
	for i := range 64 {
		g.scripts = append(g.scripts, gbScript{gbTagN(i), body})
	}
	return g
}

// evilFmt1 — nl lookups по ns общих подтаблиц формата 1: покрытие из 30 000 глифов
// (нечётные), один набор из 6000 пар. Пара (1, 1): первый глиф покрыт, второй не найден —
// самый дорогой промах формата 1.
func evilFmt1(nl, ns int) *gposBuilder {
	cov := make([]int, 30000)
	for i := range cov {
		cov[i] = 2*i + 1
	}
	set := make([][3]int, 6000)
	for i := range set {
		set[i] = [3]int{2 * i, -1, 0}
	}
	sub := &gbSub{format: 1, vf1: 4, cov: cov, sets: [][][3]int{set}}
	lk := gbLk(slices.Repeat([]*gbSub{sub}, ns)...)
	return gbSimple("DFLT", slices.Repeat([]*gbLookup{lk}, nl)...)
}

// evilFmt2 — nl lookups по ns общих подтаблиц формата 2: глиф 1 в покрытии (32 000 глифов),
// класс первого 0 < c1, второго 5 ≥ c2 — «следующая подтаблица», три поиска на пробу.
func evilFmt2(nl, ns int) *gposBuilder {
	cov := make([]int, 32000)
	for i := range cov {
		cov[i] = 2*i + 1
	}
	cd2 := make([][3]int, 21845)
	for r := range cd2 {
		cd2[r] = [3]int{3 * r, 3*r + 2, 5}
	}
	sub := &gbSub{
		format: 2, vf1: 4, cov: cov, cd1: [][3]int{{0, 1, 0}}, cd2: cd2,
		c1: 1, c2: 1, vals: [][2]int{{-1, 0}},
	}
	lk := gbLk(slices.Repeat([]*gbSub{sub}, ns)...)
	return gbSimple("DFLT", slices.Repeat([]*gbLookup{lk}, nl)...)
}

// extFar — Extension, чья внутренняя подтаблица лежит дальше 64 КБ от обёртки (смещение u32).
func extFar() *gposImage {
	g := gbSimple("DFLT", gbLk(&gbSub{format: 1, vf1: 4, ext: true, recs: []gbRec{{1, 2, -3, 0}}}))
	im := g.build()
	wrap, inner := im.at("sub.0.0"), im.at("inner.0.0")
	pad := make([]byte, 70000)
	b := slices.Concat(im.b[:inner], pad, im.b[inner:])
	binary.BigEndian.PutUint32(b[wrap+4:], uint32(inner-wrap+len(pad)))
	return &gposImage{b: b, pos: im.pos}
}

// cacheCovered — покрытие evilCache: все глифы u16, кроме кратных 11 (≈ 59 600 из 65 536).
func cacheCovered() []int {
	var cov []int
	for g := 0; g < 1<<16; g++ {
		if g%11 != 0 {
			cov = append(cov, g)
		}
	}
	return cov
}

// evilCache — один lookup Extension из ns подтаблиц формата 1 с покрытием на ≈ 59 600 глифов
// (≈ 119 КБ на подтаблицу). distinct — у каждой своё покрытие в памяти (подтаблицы не помещаются
// в кэш процессора), иначе одно общее. Глиф кратный 11 не покрыт: промах после полного
// двоичного поиска (≈ 16 шагов), а путь поиска у каждого глифа свой. Смещения u16 лежали бы за
// 64 КБ, поэтому таблица собрана вручную: lookup ссылается на обёртки Extension (u32).
func evilCache(ns int, distinct bool) []byte {
	cov := cacheCovered()
	subSize := 14 + 2*len(cov)
	extBase := 48 + 6 + 2*ns
	subBase := extBase + 8*ns
	nsub := 1
	if distinct {
		nsub = ns
	}
	b := make([]byte, subBase+nsub*subSize)
	put := func(at, v int) { binary.BigEndian.PutUint16(b[at:], uint16(v)) }
	put(0, 1)  // версия 1.0
	put(4, 10) // ScriptList
	put(6, 30) // FeatureList
	put(8, 44) // LookupList
	put(10, 1) // скрипт DFLT: Script в 18, DefaultLangSys в 22
	copy(b[12:], "DFLT")
	put(16, 8)
	put(18, 4)
	put(24, 0xFFFF) // обязательной фичи нет
	put(26, 1)      // фича 0
	put(30, 1)      // фича kern: Feature в 38, lookup 0
	copy(b[32:], "kern")
	put(36, 8)
	put(40, 1)
	put(44, 1) // LookupList: Lookup в 48
	put(46, 4)
	put(48, 9)  // Extension
	put(52, ns) // подтаблиц в lookup
	for i := range ns {
		e := extBase + 8*i
		j := 0
		if distinct {
			j = i
		}
		put(54+2*i, e-48)
		put(e, 1)
		put(e+2, 2)                                                      // внутри PairPos
		binary.BigEndian.PutUint32(b[e+4:], uint32(subBase+j*subSize-e)) //nolint:gosec // таблица ≤ 32 МБ
	}
	for j := range nsub {
		s := subBase + j*subSize
		put(s, 1)
		put(s+2, 10) // покрытие сразу за заголовком
		put(s+4, 4)  // XAdvance
		put(s+10, 1)
		put(s+12, len(cov))
		for i, g := range cov {
			put(s+14+2*i, g)
		}
	}
	return b
}

// cacheText — абзац из n знаков и distinct разных глифов (все кратны 11, то есть вне покрытия).
func cacheText(n, distinct int) []sfnt.GlyphIndex {
	r := rand.New(rand.NewPCG(1, 2))
	set := make([]sfnt.GlyphIndex, distinct)
	for i := range set {
		set[i] = sfnt.GlyphIndex(11 * (1 + r.IntN(5900)))
	}
	gs := make([]sfnt.GlyphIndex, n)
	for i := range gs {
		gs[i] = set[i%distinct] // каждый из distinct встречается: иначе «разных» было бы меньше
		if i >= distinct {
			gs[i] = set[r.IntN(distinct)]
		}
	}
	return gs
}
