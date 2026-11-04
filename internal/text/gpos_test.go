package text

import (
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"golang.org/x/image/font/sfnt"
)

// Разбор GPOS своего шрифта (161 G1): таблицы собирает gposBuilder (gposbuild_test.go,
// gposevil_test.go), встроенные TTF читаются как есть. Строки и экран — задача G2.

func gsOf(ids ...int) []sfnt.GlyphIndex {
	out := make([]sfnt.GlyphIndex, len(ids))
	for i, v := range ids {
		out[i] = sfnt.GlyphIndex(v)
	}
	return out
}

func mustParse(t testing.TB, b []byte, upem int) *gposTable {
	t.Helper()
	tab, reads, err := parseGPOSRaw(b, upem)
	if err != nil || tab == nil {
		t.Fatalf("разбор: таблица %v, ошибка %v", tab != nil, err)
	}
	if reads <= 0 || reads > gposMaxReads {
		t.Fatalf("чтений %d вне (0, %d]", reads, gposMaxReads)
	}
	return tab
}

// kernOf — сдвиги после каждого глифа (сырой запрос, бюджет не должен кончиться).
func kernOf(t testing.TB, tab *gposTable, script string, ids ...int) []int {
	t.Helper()
	out := make([]int32, len(ids))
	steps, ok := tab.gapsRaw(gposTag(script), gsOf(ids...), out)
	if !ok || steps > gposStepsPerPair*max(len(ids)-1, 0) {
		t.Fatalf("запрос: ok=%v, шагов %d", ok, steps)
	}
	res := make([]int, len(out))
	for i, v := range out {
		res[i] = int(v)
	}
	return res
}

func wantKern(t *testing.T, got []int, want ...int) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("сдвиги %v, ждали %v", got, want)
	}
}

func pairLk(v int) *gbLookup { return gbLk(gbKern([3]int{1, 2, v})) }

func kernFeat(lks ...int) gbFeature { return gbFeature{"kern", &gbFeatBody{lookups: lks}} }

// oneLang — скрипт DFLT с DefaultLangSys: обязательная фича req, список fi.
func oneLang(feats []gbFeature, req int, fi []int, lks ...*gbLookup) *gposBuilder {
	return &gposBuilder{
		scripts:  []gbScript{{"DFLT", &gbScriptBody{langs: []gbLang{{"", &gbLangSys{req: req, feats: fi}}}}}},
		features: feats, lookups: lks,
	}
}

// Lookups фичи kern складываются; повторы индексов фич и lookups не удваивают; чужие фичи
// не в счёт; lookups скрипта идут по возрастанию номера.
func TestGPOSKernSumsLookups(t *testing.T) {
	cases := []struct {
		name string
		g    *gposBuilder
		want []int
	}{
		{"два lookups одной фичи", gbSimple("DFLT", pairLk(-10), pairLk(-10)), []int{-20, 0}},
		{"три разных", gbSimple("DFLT", pairLk(-10), pairLk(-20), pairLk(-30)), []int{-60, 0}},
		{"две фичи kern", oneLang([]gbFeature{kernFeat(0), kernFeat(1)}, 0xFFFF, []int{0, 1}, pairLk(-10), pairLk(-5)), []int{-15, 0}},
		{"обязательная фича и список", oneLang([]gbFeature{kernFeat(0), kernFeat(1)}, 1, []int{0}, pairLk(-10), pairLk(-5)), []int{-15, 0}},
		{"повторы индексов фич и lookups", oneLang([]gbFeature{kernFeat(0, 0, 1, 1)}, 0, []int{0, 0, 0}, pairLk(-10), pairLk(-10)), []int{-20, 0}},
		{"lookup в двух фичах — один раз", oneLang([]gbFeature{kernFeat(0), kernFeat(0, 1)}, 0xFFFF, []int{0, 1}, pairLk(-10), pairLk(-5)), []int{-15, 0}},
		{"чужая фича не в счёт", oneLang([]gbFeature{{"liga", &gbFeatBody{lookups: []int{0}}}, kernFeat(1)}, 0xFFFF, []int{0, 1}, pairLk(-100), pairLk(-10)), []int{-10, 0}},
		{"значение второго глифа", gbSimple("DFLT", gbLk(gbF1(4, 4, gbRec{1, 2, -5, -3})), pairLk(-10)), []int{-15, -3}},
		{"чужие поля записи пропущены", gbSimple("DFLT", gbLk(gbF1(0x3F, 0x0D, gbRec{1, 2, -5, -3}))), []int{-5, -3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tab := mustParse(t, c.g.build().b, 1000)
			wantKern(t, kernOf(t, tab, "DFLT", 1, 2), c.want...)
		})
	}
	g := oneLang([]gbFeature{kernFeat(2, 0, 1)}, 0xFFFF, []int{0}, pairLk(-1), pairLk(-2), pairLk(-4))
	tab := mustParse(t, g.build().b, 1000)
	if got := tab.scripts[tagDFLT]; !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("lookups скрипта %v, ждали по возрастанию [0 1 2]", got)
	}
}

// subMode — вид кодирования одной и той же подтаблицы: результат обязан совпасть.
type subMode struct {
	name string
	set  func(*gbSub)
}

var subModes = []subMode{
	{"список", func(*gbSub) {}},
	{"Extension", func(s *gbSub) { s.ext = true }},
	{"диапазоны", func(s *gbSub) { s.covRange, s.cdArray = true, true }},
}

// f2Cov — формат 2 с покрытием cov, одним классом (c1 × c2 = 1 × 1) и нулевой записью.
func f2Cov(cov ...int) *gbSub {
	return &gbSub{format: 2, vf1: 4, cov: cov, c1: 1, c2: 1, vals: [][2]int{{0, 0}}}
}

// Порядок подтаблиц как в HarfBuzz: ф. 1 без пары — дальше, ф. 1 с парой и ф. 2 с покрытием
// закрывают позицию (ф. 2 — даже нулевой записью), ValueFormat2 ≠ 0 пропускает второй глиф.
func TestGPOSSubtableOrder(t *testing.T) {
	f1 := func(g1, g2, v int) *gbSub { return gbKern([3]int{g1, g2, v}) }
	f2Class := func() *gbSub { // класс первого 1 ≥ c1 = 1 — подтаблица не применяется
		s := f2Cov(1)
		s.cd1, s.vals = [][3]int{{1, 1, 1}}, [][2]int{{-4, 0}}
		return s
	}
	cases := []struct {
		name string
		subs func() []*gbSub
		gs   []int
		want []int
	}{
		{"ф. 1 без пары — дальше", func() []*gbSub { return []*gbSub{f1(1, 9, -1), f1(1, 2, -7)} }, []int{1, 2}, []int{-7, 0}},
		{"ф. 1 с парой — конец", func() []*gbSub { return []*gbSub{f1(1, 2, -3), f1(1, 2, -9)} }, []int{1, 2}, []int{-3, 0}},
		{"ф. 2 с нулевой записью — конец lookup", func() []*gbSub { return []*gbSub{f2Cov(1), f1(1, 2, -9)} }, []int{1, 2}, []int{0, 0}},
		{"ф. 2 класс вне числа классов — дальше", func() []*gbSub { return []*gbSub{f2Class(), f1(1, 2, -9)} }, []int{1, 2}, []int{-9, 0}},
		{"ф. 2 глиф вне покрытия — дальше", func() []*gbSub { return []*gbSub{f2Cov(5), f1(1, 2, -9)} }, []int{1, 2}, []int{-9, 0}},
		{"ф. 2 применяется", func() []*gbSub { return []*gbSub{gbClassKern(1, 2, -6), f1(1, 2, -9)} }, []int{1, 2}, []int{-6, 0}},
		{"ф. 2 нулевой класс второго — запись {1,0}", func() []*gbSub { return []*gbSub{gbClassKern(1, 2, -6), f1(1, 3, -9)} }, []int{1, 3}, []int{0, 0}},
		{"ValueFormat2 ≠ 0: второй глиф пропущен", func() []*gbSub { return []*gbSub{gbF1(4, 4, gbRec{1, 2, -5, -6}), f1(2, 3, -9)} }, []int{1, 2, 3}, []int{-5, -6, 0}},
		{"ValueFormat2 = 0: пара второго глифа тоже", func() []*gbSub { return []*gbSub{f1(1, 2, -5), f1(2, 3, -9)} }, []int{1, 2, 3}, []int{-5, -9, 0}},
		{"после ф. 2 идём на следующую позицию", func() []*gbSub { return []*gbSub{gbClassKern(1, 2, -6), f1(2, 3, -9)} }, []int{1, 2, 3}, []int{-6, -9, 0}},
	}
	for _, c := range cases {
		for _, m := range subModes {
			t.Run(c.name+"/"+m.name, func(t *testing.T) {
				subs := c.subs()
				for _, s := range subs {
					m.set(s)
				}
				tab := mustParse(t, gbSimple("DFLT", gbLk(subs...)).build().b, 1000)
				wantKern(t, kernOf(t, tab, "DFLT", c.gs...), c.want...)
			})
		}
	}
}

// LangSys «RUS » выбирается раньше DefaultLangSys; скрипта нет — DFLT, dflt, latn;
// скрипт есть, но без своей kern — кернинга нет (не DFLT): браузер делает так же.
func TestGPOSLangSysRUS(t *testing.T) {
	lang := func(tag string, feat int) gbLang { return gbLang{tag, &gbLangSys{req: 0xFFFF, feats: []int{feat}}} }
	script := func(tag string, langs ...gbLang) gbScript { return gbScript{tag, &gbScriptBody{langs: langs}} }
	g := &gposBuilder{
		features: []gbFeature{kernFeat(0), kernFeat(1), kernFeat(2), kernFeat(3)},
		lookups:  []*gbLookup{pairLk(-1), pairLk(-2), pairLk(-4), pairLk(-8)},
		scripts: []gbScript{
			script("cyrl", lang("", 0), lang("RUS ", 1), lang("SRB ", 2)),
			script("grek", lang("", 0)),
			script("armn", lang("RUS ", 1)),
			script("geor", lang("SRB ", 2)),
			script("hebr"),
			script("DFLT", lang("", 3)),
		},
	}
	tab := mustParse(t, g.build().b, 1000)
	for _, c := range []struct {
		script string
		want   int
	}{
		{"cyrl", -2}, {"grek", -1}, {"armn", -2}, {"geor", 0}, {"hebr", 0},
		{"xxxx", -8}, // скрипта нет — DFLT
	} {
		if got := kernOf(t, tab, c.script, 1, 2)[0]; got != c.want {
			t.Errorf("%s: %d, ждали %d", c.script, got, c.want)
		}
	}
	chain := func(tags ...string) *gposBuilder {
		g := &gposBuilder{features: []gbFeature{kernFeat(0), kernFeat(1), kernFeat(2)}, lookups: []*gbLookup{pairLk(-1), pairLk(-2), pairLk(-4)}}
		for _, tag := range tags {
			f := map[string]int{"DFLT": 0, "dflt": 1, "latn": 2}[tag]
			g.scripts = append(g.scripts, script(tag, lang("", f)))
		}
		return g
	}
	for _, c := range []struct {
		have []string
		want int
	}{
		{[]string{"latn", "dflt", "DFLT"}, -1}, {[]string{"latn", "dflt"}, -2}, {[]string{"latn"}, -4},
	} {
		tab := mustParse(t, chain(c.have...).build().b, 1000)
		if got := kernOf(t, tab, "xxxx", 1, 2)[0]; got != c.want {
			t.Errorf("есть %v: %d, ждали %d", c.have, got, c.want)
		}
	}
}

func put16(b []byte, at, v int) { binary.BigEndian.PutUint16(b[at:], uint16(v)) }

// refuseBase — таблица для порчи: два скрипта, фича kern, lookup 0 с Extension, lookup 1 формата 2.
func refuseBase() *gposImage {
	ext := &gbSub{format: 1, vf1: 4, ext: true, recs: []gbRec{{1, 2, -3, 0}}}
	g := gbSimple("cyrl", gbLk(ext), gbLk(gbClassKern(1, 2, -5)))
	g.scripts = append(g.scripts, gbScript{"DFLT", g.scripts[0].body})
	return g.build()
}

type refuseCase struct {
	name string
	mut  func(b []byte, im *gposImage) []byte
	upem int
	want error // nil — таблица принята
}

func patch16(name string, delta int, v int) func([]byte, *gposImage) []byte {
	return func(b []byte, im *gposImage) []byte {
		put16(b, im.at(name)+delta, v)
		return b
	}
}

func refuseCases() []refuseCase {
	far := 0xFFF0
	return []refuseCase{
		{"пусто", func([]byte, *gposImage) []byte { return nil }, 1000, errGPOS},
		{"шапка обрезана", func(b []byte, _ *gposImage) []byte { return b[:9] }, 1000, errGPOS},
		{"версия 2", func(b []byte, _ *gposImage) []byte { put16(b, 0, 2); return b }, 1000, errGPOS},
		{"ScriptList вне таблицы", func(b []byte, _ *gposImage) []byte { put16(b, 4, len(b)+10); return b }, 1000, errGPOS},
		{"FeatureList вне таблицы", func(b []byte, _ *gposImage) []byte { put16(b, 6, len(b)+10); return b }, 1000, errGPOS},
		{"LookupList вне таблицы", func(b []byte, _ *gposImage) []byte { put16(b, 8, len(b)+10); return b }, 1000, errGPOS},
		{"скриптов 65535 при коротком теле", func(b []byte, im *gposImage) []byte { put16(b, im.at("scriptList"), 0xFFFF); return b }, 1000, errGPOSLimit},
		{"смещение Script вне", func(b []byte, im *gposImage) []byte { put16(b, im.at("scriptList")+2+4, far); return b }, 1000, errGPOS},
		{"смещение LangSys вне", patch16("script.0", 0, far), 1000, errGPOS},
		{"LangSys 65535 записей", patch16("script.0", 2, 0xFFFF), 1000, errGPOSLimit},
		{"индексов фич 65535", patch16("lang.0", 4, 0xFFFF), 1000, errGPOSLimit},
		{"lookups у фичи 65535", patch16("feature.0", 2, 0xFFFF), 1000, errGPOSLimit},
		{"смещение Feature вне", func(b []byte, im *gposImage) []byte { put16(b, im.at("featureList")+2+4, far); return b }, 1000, errGPOS},
		{"смещение lookup вне", func(b []byte, im *gposImage) []byte { put16(b, im.at("lookupList")+2, far); return b }, 1000, errGPOS},
		{"подтаблиц в lookup 65535", patch16("lookup.1", 4, 0xFFFF), 1000, errGPOSLimit},
		{"смещение подтаблицы вне", patch16("lookup.1", 6, far), 1000, errGPOS},
		{"Extension → Extension", patch16("sub.0.0", 2, 9), 1000, errGPOS},
		{"Extension формата 2", patch16("sub.0.0", 0, 2), 1000, errGPOS},
		{"Extension со смещением вне", func(b []byte, im *gposImage) []byte {
			binary.BigEndian.PutUint32(b[im.at("sub.0.0")+4:], 0x7FFFFFF0)
			return b
		}, 1000, errGPOS},
		{"Extension со смещением 0xFFFFFFFF", func(b []byte, im *gposImage) []byte {
			binary.BigEndian.PutUint32(b[im.at("sub.0.0")+4:], 0xFFFFFFFF)
			return b
		}, 1000, errGPOS},
		{"матрица формата 2 вне (c1 = c2 = 65535)", func(b []byte, im *gposImage) []byte {
			put16(b, im.at("sub.1.0")+12, 0xFFFF)
			put16(b, im.at("sub.1.0")+14, 0xFFFF)
			return b
		}, 1000, errGPOS},
		{"заголовок формата 2 обрезан", func(b []byte, im *gposImage) []byte { return b[:im.at("sub.1.0")+9] }, 1000, errGPOS},
		{"upem 0", func(b []byte, _ *gposImage) []byte { return b }, 0, errGPOS},
		{"upem 20000", func(b []byte, _ *gposImage) []byte { return b }, 20000, errGPOS},
		{"без порчи", func(b []byte, _ *gposImage) []byte { return b }, 1000, nil},
		{"ValueFormat > 0xFF — подтаблица пропущена, не отказ", patch16("sub.1.0", 4, 0x100), 1000, nil},
		{"формат подтаблицы 3 — пропущена", patch16("sub.1.0", 0, 3), 1000, nil},
		{"смещение покрытия 0 — подтаблица пропущена", patch16("sub.1.0", 2, 0), 1000, nil},
	}
}

// Испорченные таблицы: отказ с нужной причиной, без паники (сырой разбор — без recover),
// чтений не больше бюджета; обёртка отдаёт то же самое.
func TestGPOSRefuse(t *testing.T) {
	cases := refuseCases()
	if len(cases) < 12 {
		t.Fatalf("входов %d, нужно не меньше 12", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := c.mut(refuseBase().clone(), refuseBase())
			tab, reads, err := parseGPOSRaw(b, c.upem)
			if !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
				t.Fatalf("ошибка %v, ждали %v", err, c.want)
			}
			if (err == nil) != (tab != nil) || reads > gposMaxReads {
				t.Fatalf("таблица %v, чтений %d", tab != nil, reads)
			}
			if _, werr := parseGPOS(b, c.upem); !errors.Is(werr, c.want) || (c.want == nil) != (werr == nil) {
				t.Fatalf("обёртка: %v, ждали %v", werr, c.want)
			}
		})
	}
}

// Каждый обрезок и каждый испорченный байт: сырой разбор и запрос не паникуют.
func TestGPOSDamageNoPanic(t *testing.T) {
	base := refuseBase().b
	probe := func(b []byte) {
		tab, reads, err := parseGPOSRaw(b, 1000)
		if reads > gposMaxReads {
			t.Fatalf("чтений %d", reads)
		}
		if err != nil || tab == nil {
			return
		}
		for _, s := range []string{"cyrl", "DFLT", "latn"} {
			kernOf(t, tab, s, 1, 2, 1, 3, 2, 1, 1, 2)
		}
	}
	for cut := range base {
		probe(base[:cut])
	}
	for at := range base {
		for _, v := range []byte{0x00, 0x7F, 0x80, 0xFF} {
			b := slices.Clone(base)
			b[at] = v
			probe(b)
		}
	}
}

// Терпимое: индекс фичи и lookup вне списков, чужой тип lookup, UseMarkFilteringSet,
// повтор тега скрипта, нулевые смещения списков — не отказ, лишнее просто пропущено.
func TestGPOSTolerates(t *testing.T) {
	good := func() *gbLookup { return pairLk(-3) }
	skipFirst := gbLk(gbSkipSub(), gbKern([3]int{1, 2, -3}), &gbSub{format: 3})
	marks := gbLk(gbKern([3]int{1, 2, -7}))
	marks.flag = 0x10
	other := gbLk(gbKern([3]int{1, 2, -7}))
	other.typ = 1
	cases := []struct {
		name string
		g    *gposBuilder
		want int
	}{
		{"индекс фичи вне списка", oneLang([]gbFeature{kernFeat(0)}, 0xFFFF, []int{0, 99}, good()), -3},
		{"обязательная фича вне списка", oneLang([]gbFeature{kernFeat(0)}, 99, []int{0}, good()), -3},
		{"номер lookup вне списка", oneLang([]gbFeature{kernFeat(0, 77)}, 0xFFFF, []int{0}, good()), -3},
		{"ValueFormat > 0xFF и формат 3 пропущены", gbSimple("DFLT", skipFirst), -3},
		{"UseMarkFilteringSet — lookup пустой", gbSimple("DFLT", marks), 0},
		{"чужой тип lookup — пустой", gbSimple("DFLT", other), 0},
		{"повтор тега скрипта — первый", func() *gposBuilder {
			g := oneLang([]gbFeature{kernFeat(0), kernFeat(1)}, 0xFFFF, []int{0}, good(), pairLk(-100))
			g.scripts = append(g.scripts, gbScript{"DFLT", gbBody(1)})
			return g
		}(), -3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tab, _, err := parseGPOSRaw(c.g.build().b, 1000)
			if err != nil {
				t.Fatal(err)
			}
			if tab == nil {
				t.Fatal("таблицы нет")
			}
			wantKern(t, kernOf(t, tab, "DFLT", 1, 2), c.want, 0)
		})
	}
}

// Нет фичи kern (или ни один скрипт её не берёт) — таблицы нет, кернит sfnt, как до 161.
func TestGPOSNoKernIsNil(t *testing.T) {
	noKern := oneLang([]gbFeature{{"liga", &gbFeatBody{lookups: []int{0}}}}, 0xFFFF, []int{0}, pairLk(-3))
	unused := oneLang([]gbFeature{kernFeat(0)}, 0xFFFF, nil, pairLk(-3))
	zeroed := gbSimple("DFLT", pairLk(-3)).build().clone()
	put16(zeroed, 4, 0)
	for name, b := range map[string][]byte{
		"только liga":               noKern.build().b,
		"фича не у скриптов":        unused.build().b,
		"ScriptList со смещением 0": zeroed,
	} {
		tab, _, err := parseGPOSRaw(b, 1000)
		if tab != nil || err != nil {
			t.Errorf("%s: таблица %v, ошибка %v, ждали (nil, nil)", name, tab != nil, err)
		}
	}
}

// Предел каждого счётчика: ровно на нём — разбор, на единицу больше — отказ. Числа записаны
// литералами (design §5), а не константами: сдвиг константы должен ронять тест.
func TestGPOSLimits(t *testing.T) {
	cases := []struct {
		name  string
		gen   func(n int) *gposBuilder
		limit int
	}{
		{"скриптов", limScripts, 64},
		{"LangSys", limLangSys, 128},
		{"индексов фич", limFeatIdx, 512},
		{"lookups у фичи", limFeatLks, 64},
		{"lookups у скрипта", limScriptLks, 64},
		{"разных lookups", limLookups, 256},
		{"подтаблиц в lookup", limSubsInLookup, 1024},
		{"подтаблиц у скрипта", limSubsScript, 512},
		{"подтаблиц всего", limSubsTotal, 2048},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustParse(t, c.gen(c.limit).build().b, 1000)
			if _, _, err := parseGPOSRaw(c.gen(c.limit+1).build().b, 1000); !errors.Is(err, errGPOSLimit) {
				t.Fatalf("предел+1: ошибка %v, ждали %v", err, errGPOSLimit)
			}
		})
	}
	if gposMaxValueFormat != 0xFF {
		t.Fatalf("предел ValueFormat %#x", gposMaxValueFormat)
	}
}

// Все измерения на пределе сразу, на общих смещениях: разбор укладывается в бюджет;
// любое измерение +1 — отказ по пределу.
func TestGPOSLimitsAllAtOnce(t *testing.T) {
	_, reads, err := parseGPOSRaw(limAll("").build().b, 1000)
	if err != nil {
		t.Fatalf("всё на пределе: %v", err)
	}
	t.Logf("чтений на пределе всех измерений: %d из %d", reads, gposMaxReads)
	if reads > gposMaxReads/4 {
		t.Fatalf("чтений %d: общие смещения съедают бюджет", reads)
	}
	for _, dim := range []string{"scripts", "langSys", "featIdx", "featLks", "scriptLks", "subs", "subsInLookup"} {
		t.Run(dim, func(t *testing.T) {
			if _, _, err := parseGPOSRaw(limAll(dim).build().b, 1000); !errors.Is(err, errGPOSLimit) {
				t.Fatalf("%s +1: ошибка %v", dim, err)
			}
		})
	}
}

// Бомба аудита: пределы счётчиков соблюдены, а чтений — миллионы. Останавливает бюджет.
func TestGPOSReadBudget(t *testing.T) {
	_, r1, err := parseGPOSRaw(bombFeatures(1).build().b, 1000)
	if err != nil {
		t.Fatalf("один скрипт: %v", err)
	}
	if r1 < 20000 {
		t.Fatalf("бомба слишком лёгкая: %d чтений на скрипт", r1)
	}
	ok := (gposMaxReads - 200) / r1
	if ok >= gposMaxScripts {
		t.Fatalf("бомба не доходит до бюджета: %d скриптов по %d чтений", ok, r1)
	}
	if _, _, err := parseGPOSRaw(bombFeatures(ok).build().b, 1000); err != nil {
		t.Fatalf("%d скриптов (в бюджете): %v", ok, err)
	}
	var reads int
	var berr error
	elapsed := testkit.Fastest(3, func() {
		_, reads, berr = parseGPOSRaw(bombFeatures(gposMaxScripts).build().b, 1000)
	})
	if !errors.Is(berr, errGPOSBudget) || reads != gposMaxReads {
		t.Fatalf("бомба: ошибка %v, чтений %d (ждали отказ по бюджету на %d)", berr, reads, gposMaxReads)
	}
	if limit := testkit.Limit(300 * time.Millisecond); elapsed > limit {
		t.Fatalf("отказ по бюджету шёл %s (> %s)", elapsed, limit)
	}
	// Прежний featBomb аудита: 65535 lookups у фичи — отказ по пределу раньше бюджета.
	if _, _, err := parseGPOSRaw(bombRaw(65535).build().b, 1000); !errors.Is(err, errGPOSLimit) {
		t.Fatalf("featBomb: ошибка %v", err)
	}
	mustParse(t, bombRaw(64).build().b, 1000)
}

// stepsPerPair — шагов запроса на пару для таблицы g и 5000 одинаковых глифов (пара {1,1}).
func stepsPerPair(t *testing.T, g *gposBuilder, n int) (perPair int, ok bool) {
	t.Helper()
	tab := mustParse(t, g.build().b, 1000)
	out := make([]int32, n)
	steps, ok := tab.gapsRaw(gposTag("DFLT"), slices.Repeat(gsOf(1), n), out)
	if limit := gposStepsPerPair * (n - 1); steps > limit || (!ok && steps != limit) {
		t.Fatalf("шагов %d при бюджете %d (ok=%v)", steps, limit, ok)
	}
	return steps / (n - 1), ok
}

// Худший запрос: промах каждой подтаблицы форматов 1 и 2 на 5000 знаках. У самого бюджета —
// ok, чуть выше — отказ; везде шагов ≤ 4096 × (n−1) и время в пределах.
func TestGPOSWorstCase(t *testing.T) {
	const n = 5000
	formats := []struct {
		name string
		gen  func(nl, ns int) *gposBuilder
	}{{"ф. 1", evilFmt1}, {"ф. 2", evilFmt2}}
	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			c1, ok := stepsPerPair(t, f.gen(1, 1), n)
			if !ok || c1 < 8 {
				t.Fatalf("одна подтаблица: %d шагов на пару, ok=%v", c1, ok)
			}
			near := gposStepsPerPair / c1 // подтаблиц до самого бюджета
			if per, ok := stepsPerPair(t, f.gen(1, near), n); !ok || per < gposStepsPerPair-2*c1 {
				t.Fatalf("у бюджета (%d подтаблиц): %d шагов на пару, ok=%v", near, per, ok)
			}
			if _, ok := stepsPerPair(t, f.gen(1, near+1), n); ok {
				t.Fatalf("%d подтаблиц дали ok при бюджете %d", near+1, gposStepsPerPair)
			}
			for _, shape := range [][2]int{{64, 8}, {1, 512}, {8, 64}} {
				if _, ok := stepsPerPair(t, f.gen(shape[0], shape[1]), n); ok {
					t.Errorf("злая таблица %v не остановлена бюджетом", shape)
				}
			}
			tab := mustParse(t, f.gen(64, 8).build().b, 1000)
			out := make([]int32, n)
			gs := slices.Repeat(gsOf(1), n)
			elapsed := testkit.Fastest(3, func() { tab.gapsRaw(gposTag("DFLT"), gs, out) })
			if limit := testkit.Limit(300 * time.Millisecond); elapsed > limit {
				t.Fatalf("запрос шёл %s (> %s)", elapsed, limit)
			}
		})
	}
}

// cacheQuery — запрос по таблице evilCache(ns): шагов, ok и лучшее из трёх времён.
func cacheQuery(t *testing.T, ns int, distinct bool, gs []sfnt.GlyphIndex) (steps int, ok bool, el time.Duration) {
	t.Helper()
	b := evilCache(ns, distinct)
	if len(b) > MaxFontBytes {
		t.Fatalf("таблица %d Б больше предела файла шрифта %d", len(b), MaxFontBytes)
	}
	tab := mustParse(t, b, 1000)
	out := make([]int32, len(gs))
	el = testkit.Fastest(3, func() { steps, ok = tab.gapsRaw(tagDFLT, gs, out) })
	if limit := gposStepsPerPair * (len(gs) - 1); steps > limit || (!ok && steps != limit) {
		t.Fatalf("шагов %d при бюджете %d (ok=%v)", steps, limit, ok)
	}
	return steps, ok, el
}

// Худший запрос с промахами кэша процессора (аудит S2): подтаблицы формата 1 с покрытием на
// ≈ 59 600 глифов у каждой своё, 66 разных глифов в абзаце на 5000 знаков — каждый шаг поиска
// читает холодную память (11–16 нс против 1,7 нс в кэше). Бюджет шагов на пару ограничивает и
// это: у самого бюджета — ok, чуть выше — отказ, время в пределах.
func TestGPOSWorstCaseCache(t *testing.T) {
	const n, distinct = 5000, 66
	gs := cacheText(n, distinct)
	if len(slices.Compact(slices.Sorted(slices.Values(gs)))) != distinct {
		t.Fatal("в тексте не 66 разных глифов")
	}
	one, ok, _ := cacheQuery(t, 1, true, gs)
	c1 := float64(one) / (n - 1) // шагов на пару одной подтаблицей
	if !ok || c1 < 8 {
		t.Fatalf("одна подтаблица: %.1f шагов на пару, ok=%v", c1, ok)
	}
	near := int(gposStepsPerPair/c1) - 1 // подтаблиц чуть ниже бюджета
	limit := testkit.Limit(300 * time.Millisecond)
	for _, c := range []struct {
		ns     int
		wantOK bool
	}{{near, true}, {2 * near, false}, {240, false}} {
		steps, ok, el := cacheQuery(t, c.ns, true, gs)
		t.Logf("подтаблиц %d: шагов %d, ok=%v, %s (%.1f нс/шаг)", c.ns, steps, ok, el, float64(el)/float64(steps))
		if ok != c.wantOK {
			t.Errorf("%d подтаблиц: ok=%v, ждали %v (бюджет %d, на подтаблицу %.1f)", c.ns, ok, c.wantOK, gposStepsPerPair, c1)
		}
		if el > limit {
			t.Errorf("%d подтаблиц: запрос шёл %s (> %s)", c.ns, el, limit)
		}
	}
	if _, ok, _ := cacheQuery(t, 240, false, gs); ok {
		t.Error("240 общих подтаблиц не остановлены бюджетом")
	}
}

// Сумма по lookups зажата в ±upem; значение вне ±upem не учитывается, ровно ±upem — учитывается.
func TestGPOSSumClamped(t *testing.T) {
	for _, upem := range []int{1000, gposMaxUPEM} {
		for _, sign := range []int{-1, 1} {
			v := sign * upem
			lks := make([]*gbLookup, gposMaxFeatLks)
			for i := range lks {
				lks[i] = gbLk(gbF1(4, 4, gbRec{1, 2, v, v}))
			}
			tab := mustParse(t, gbSimple("DFLT", lks...).build().b, upem)
			wantKern(t, kernOf(t, tab, "DFLT", 1, 2), v, v)
			// длинная цепочка 1 2 1 2 …: каждый знак получает вклады соседних пар, итог тот же
			chain := slices.Repeat([]int{1, 2}, 2500)
			out := make([]int32, len(chain))
			if _, ok := tab.gapsRaw(gposTag("DFLT"), gsOf(chain...), out); !ok {
				t.Fatal("бюджет")
			}
			for i, o := range out[:len(out)-1] {
				if int(o) < -upem || int(o) > upem {
					t.Fatalf("upem %d: знак %d сдвинут на %d", upem, i, o)
				}
			}
		}
	}
	small := gbSimple("DFLT", pairLk(-100), pairLk(-100), pairLk(-100))
	wantKern(t, kernOf(t, mustParse(t, small.build().b, 1000), "DFLT", 1, 2), -300, 0)
	for _, c := range []struct{ v, want int }{
		{1000, 1000}, {-1000, -1000}, {1001, 0}, {-1001, 0}, {32767, 0}, {-32768, 0},
	} {
		tab := mustParse(t, gbSimple("DFLT", pairLk(c.v)).build().b, 1000)
		wantKern(t, kernOf(t, tab, "DFLT", 1, 2), c.want, 0)
	}
}

// Обёртки ловят панику сырых тел и отдают её текст; сырое тело на порче (lookup есть в списке
// скрипта, а в разборе его нет) паникует — иначе тесты и фаззы не увидели бы выход за границы.
func TestGPOSWrappers(t *testing.T) {
	if v := panicked(func() { panic("x") }); v != "x" || panicked(func() {}) != nil {
		t.Fatal("panicked ошибается")
	}
	if panicked(func() { panic(nil) }) == nil {
		t.Fatal("panic(nil) принят за норму")
	}
	tab := mustParse(t, gbSimple("DFLT", pairLk(-5)).build().b, 1000)
	out := make([]int32, 2)
	if err := tab.gaps(tagDFLT, gsOf(1, 2), out); err != nil || out[0] != -5 {
		t.Fatalf("gaps: %v, %v", err, out)
	}
	wrapped, err := parseGPOS(gbSimple("DFLT", pairLk(-5)).build().b, 1000)
	if err != nil || wrapped == nil {
		t.Fatalf("parseGPOS: %v", err)
	}
	broken := &gposTable{b: tab.b, upem: 1000, scripts: map[uint32][]int{tagDFLT: {7}}, lookups: map[int]*kernLookup{}}
	if panicked(func() { broken.gapsRaw(tagDFLT, gsOf(1, 2), out) }) == nil {
		t.Fatal("сырое тело не паникует на порче: тесты её не увидят")
	}
	// паника обёртки — errGPOS с текстом runtime error: он идёт в журнал, без путей и имён файлов
	err = broken.gaps(tagDFLT, gsOf(1, 2), out)
	if !errors.Is(err, errGPOS) || !strings.Contains(err.Error(), "runtime error") || strings.Contains(err.Error(), "/") {
		t.Fatalf("обёртка не поймала панику: %v", err)
	}
	// бюджет шагов — свой отказ
	evil := mustParse(t, evilFmt1(1, 64).build().b, 1000)
	if err := evil.gaps(tagDFLT, slices.Repeat(gsOf(1), 100), make([]int32, 100)); !errors.Is(err, errGPOSSteps) {
		t.Fatalf("бюджет шагов: %v", err)
	}
	// короткий out — ошибка вызывающего, а не порча шрифта: паника мимо recover, не отказ GPOS
	short := func() (v any) {
		defer func() { v = recover() }()
		_ = tab.gaps(tagDFLT, gsOf(1, 2, 3), out)
		return nil
	}()
	if s, _ := short.(string); !strings.Contains(s, "out короче gs") {
		t.Fatalf("короткий out: паника %v", short)
	}
}

// gposTables — встроенные TTF с таблицей GPOS: (файл, байты, GPOS, шрифт).
type gposFont struct {
	file string
	gpos []byte
	sf   *sfnt.Font
}

func builtinGPOSFonts(tb testing.TB) (fonts []gposFont, noGPOS []string) {
	tb.Helper()
	for _, f := range families {
		for _, fc := range f.Faces {
			b, err := fontFS.ReadFile("fonts/" + fc.File)
			if err != nil {
				tb.Fatal(err)
			}
			tabs, err := readTables(b)
			if err != nil {
				tb.Fatalf("%s: %v", fc.File, err)
			}
			sf, err := sfnt.Parse(b)
			if err != nil {
				tb.Fatalf("%s: %v", fc.File, err)
			}
			if g := tabs.bytes(b, "GPOS"); g != nil {
				fonts = append(fonts, gposFont{fc.File, g, sf})
			} else {
				noGPOS = append(noGPOS, fc.File)
			}
		}
	}
	return fonts, noGPOS
}

// hasKernFeature — независимая проверка: в FeatureList есть запись kern.
func hasKernFeature(gpos []byte) bool {
	flo := int(binary.BigEndian.Uint16(gpos[6:]))
	n := int(binary.BigEndian.Uint16(gpos[flo:]))
	for i := range n {
		if string(gpos[flo+2+6*i:][:4]) == "kern" {
			return true
		}
	}
	return false
}

// glyphsOf — индексы глифов строки в шрифте.
func glyphsOf(tb testing.TB, sf *sfnt.Font, s string) []int {
	tb.Helper()
	var buf sfnt.Buffer
	var out []int
	for _, r := range s {
		g, err := sf.GlyphIndex(&buf, r)
		if err != nil {
			tb.Fatal(err)
		}
		out = append(out, int(g))
	}
	return out
}

// pairChars — знаки для перебора пар: кириллица, латиница, цифры, знаки.
const pairChars = "абвгдеёжзийклмнопрстуфхцчшщъыьэюяАБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ" +
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,:;!?'\"()-—«»"

// maxStepsPerPair — наибольшее число шагов запроса на одну пару знаков pairChars (скрипты
// cyrl, latn, DFLT): запас бюджета на пару считается от него.
func maxStepsPerPair(t testing.TB, tab *gposTable, sf *sfnt.Font) int {
	t.Helper()
	var gl []sfnt.GlyphIndex
	for _, g := range glyphsOf(t, sf, pairChars) {
		if g != 0 {
			gl = append(gl, sfnt.GlyphIndex(g))
		}
	}
	best, out := 0, make([]int32, 2)
	for _, script := range []string{"cyrl", "latn", "DFLT"} {
		for _, a := range gl {
			for _, b := range gl {
				steps, ok := tab.gapsRaw(gposTag(script), []sfnt.GlyphIndex{a, b}, out)
				if !ok {
					t.Fatalf("пара %d %d (%s): бюджет", a, b, script)
				}
				best = max(best, steps)
			}
		}
	}
	return best
}

// builtinKernFonts — сколько из 62 встроенных TTF несёт фичу kern в GPOS (research §2: у
// Ruslan Display GPOS нет совсем); число сверено выводом теста при вводе.
const builtinKernFonts = 61

// Встроенные TTF: GPOS читается без отказа, таблица не пуста у шрифтов с kern, запросы
// укладываются в бюджет и в ±upem, «шрифт сломан» (kernOff) не включается.
func TestGPOSBuiltinsParse(t *testing.T) {
	fonts, noGPOS := builtinGPOSFonts(t)
	if !slices.Equal(noGPOS, []string{"RuslanDisplay-Regular.ttf"}) {
		t.Fatalf("без GPOS: %v, ждали только Ruslan Display", noGPOS)
	}
	const sample = "ТАТЬЯНА и Павел AVATAR 1941. То 'А .у 7. Ёж"
	kerned, maxReads, maxLk, maxSub, maxSteps, maxStepsFont := 0, 0, 0, 0, 0, ""
	for _, f := range fonts {
		upem := f.sf.UnitsPerEm()
		tab, reads, err := parseGPOSRaw(f.gpos, int(upem))
		if err != nil {
			t.Fatalf("%s: %v", f.file, err)
		}
		maxReads = max(maxReads, reads)
		if has := hasKernFeature(f.gpos); has != (tab != nil) {
			t.Fatalf("%s: фича kern %v, таблица %v", f.file, has, tab != nil)
		}
		if tab == nil {
			continue
		}
		kerned++
		subs := 0
		for _, lk := range tab.lookups {
			subs += len(lk.subs)
		}
		maxLk, maxSub = max(maxLk, len(tab.lookups)), max(maxSub, subs)
		if m := maxStepsPerPair(t, tab, f.sf); m > maxSteps {
			maxSteps, maxStepsFont = m, f.file
		}
		ids := glyphsOf(t, f.sf, sample)
		for _, script := range []string{"cyrl", "latn", "DFLT", "grek"} {
			out := make([]int32, len(ids))
			if err := tab.gaps(gposTag(script), gsOf(ids...), out); err != nil {
				t.Fatalf("%s/%s: запрос отказал (kernOff включился бы): %v", f.file, script, err)
			}
			for _, o := range out {
				if int(o) < -int(upem) || int(o) > int(upem) {
					t.Fatalf("%s: сдвиг %d вне ±%d", f.file, o, upem)
				}
			}
		}
	}
	t.Logf("с kern: %d; максимум чтений %d, lookups %d, подтаблиц %d, шагов на пару %d (%s)",
		kerned, maxReads, maxLk, maxSub, maxSteps, maxStepsFont)
	if maxSteps*2 > gposStepsPerPair {
		t.Fatalf("шагов на пару до %d (%s): запас до бюджета %d меньше ×2", maxSteps, maxStepsFont, gposStepsPerPair)
	}
	if kerned != builtinKernFonts {
		t.Fatalf("таблица не пуста у %d шрифтов, ждали %d", kerned, builtinKernFonts)
	}
	if maxReads > gposMaxReads/20 {
		t.Fatalf("максимум чтений %d: запас до бюджета %d меньше 20×", maxReads, gposMaxReads)
	}
	if maxLk > gposMaxLookups/4 || maxSub > gposMaxSubsTotal/4 {
		t.Fatalf("lookups %d, подтаблиц %d: пределы не в 4 раза выше встроенных", maxLk, maxSub)
	}
}

// Кириллица Lora и сумма lookups Roboto Condensed — значения из браузера (research §4).
func TestGPOSBuiltinPairs(t *testing.T) {
	fonts, _ := builtinGPOSFonts(t)
	byFile := map[string]gposFont{}
	for _, f := range fonts {
		byFile[f.file] = f
	}
	for _, c := range []struct {
		file, pair string
		want, upem int
	}{
		{"Lora-Regular.ttf", "То", -49, 1000},
		{"Lora-Regular.ttf", "'А", -49, 1000},
		{"Lora-Regular.ttf", ".у", -46, 1000},
		{"RobotoCondensed-Regular.ttf", "'А", -120, 2048},
	} {
		f := byFile[c.file]
		if int(f.sf.UnitsPerEm()) != c.upem {
			t.Fatalf("%s: upem %d", c.file, f.sf.UnitsPerEm())
		}
		tab := mustParse(t, f.gpos, c.upem)
		if got := kernOf(t, tab, "cyrl", glyphsOf(t, f.sf, c.pair)...)[0]; got != c.want {
			t.Errorf("%s «%s»: %d, ждали %d", c.file, c.pair, got, c.want)
		}
	}
	for _, file := range []string{"Lora-Regular.ttf", "Lora-Bold.ttf", "Lora-Italic.ttf", "Lora-BoldItalic.ttf"} {
		tab := mustParse(t, byFile[file].gpos, 1000)
		if len(tab.scripts[gposTag("cyrl")]) == 0 {
			t.Errorf("%s: у cyrl нет lookups kern", file)
		}
	}
}

// checkGPOSInput — общая проверка входа для фаззера: чтений ≤ бюджета, шагов ≤ бюджета,
// сдвиги в ±upem, сырые тела (паника — находка).
func checkGPOSInput(t *testing.T, data []byte, upemSeed uint16) {
	upem := gposMinUPEM + int(upemSeed)%(gposMaxUPEM-gposMinUPEM+1)
	tab, reads, err := parseGPOSRaw(data, upem)
	if reads > gposMaxReads {
		t.Fatalf("чтений %d", reads)
	}
	if err != nil || tab == nil {
		return
	}
	inputs := [][]sfnt.GlyphIndex{make([]sfnt.GlyphIndex, 512), make([]sfnt.GlyphIndex, 512)}
	for i := range inputs[0] {
		inputs[0][i] = sfnt.GlyphIndex(i % 40) // малые номера: попадают в покрытия синтетики
		if len(data) >= 2 {
			at := (7 * i) % (len(data) - 1)
			inputs[1][i] = sfnt.GlyphIndex(binary.BigEndian.Uint16(data[at:]))
		}
	}
	for _, script := range []string{"cyrl", "latn", "DFLT", "xxxx"} {
		for _, gs := range inputs {
			out := make([]int32, len(gs))
			steps, ok := tab.gapsRaw(gposTag(script), gs, out)
			if limit := gposStepsPerPair * (len(gs) - 1); steps > limit || (!ok && steps != limit) {
				t.Fatalf("шагов %d при бюджете %d (ok=%v)", steps, limit, ok)
			}
			if !ok {
				continue
			}
			for _, o := range out {
				if int(o) < -upem || int(o) > upem {
					t.Fatalf("сдвиг %d вне ±%d", o, upem)
				}
			}
		}
	}
}

func FuzzGPOS(f *testing.F) {
	fonts, _ := builtinGPOSFonts(f)
	for _, f1 := range fonts {
		if strings.HasPrefix(f1.file, "Lora-Regular") || strings.HasPrefix(f1.file, "RobotoCondensed-Regular") ||
			strings.HasPrefix(f1.file, "CormorantGaramond-Italic") || strings.HasPrefix(f1.file, "Montserrat-Regular") {
			f.Add(f1.gpos, uint16(f1.sf.UnitsPerEm()-gposMinUPEM))
		}
	}
	seeds := []*gposImage{
		refuseBase(), extFar(), gbSimple("DFLT", pairLk(-10), pairLk(-10)).build(),
		bombFeatures(8).build(), bombRaw(70).build(),
		evilFmt1(2, 4).build(), evilFmt2(2, 4).build(), limAll("").build(),
	}
	for _, s := range seeds {
		f.Add(s.b, uint16(1000-gposMinUPEM))
	}
	f.Fuzz(func(t *testing.T, data []byte, upem uint16) { checkGPOSInput(t, data, upem) })
}
