package text

import (
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
)

// gposBuilder собирает синтетическую таблицу GPOS для тестов разбора (161): скрипты, LangSys,
// фичи, lookups, подтаблицы PairPos 1/2, Extension и общие смещения. Один и тот же указатель
// (тело скрипта, LangSys, фича, lookup, подтаблица) пишется один раз, а все ссылки идут на
// него: так строятся «бомбы» из общих смещений, где пределы счётчиков не ограничивают работу.
// Смещения u16 — вперёд и в пределах 65535; выход за них — паника сборщика (ошибка теста).

type gbLangSys struct {
	req   int // 0xFFFF — обязательной фичи нет
	feats []int
}

type gbLang struct {
	tag string // "" — DefaultLangSys
	ls  *gbLangSys
}

type gbScriptBody struct{ langs []gbLang }

type gbScript struct {
	tag  string
	body *gbScriptBody
}

type gbFeatBody struct{ lookups []int }

type gbFeature struct {
	tag  string
	body *gbFeatBody
}

type gbLookup struct {
	typ, flag int
	subs      []*gbSub
}

// gbRec — пара формата 1: первый глиф, второй глиф, XAdvance первого и второго.
type gbRec struct{ g1, g2, v1, v2 int }

// gbSub — подтаблица PairPos. Формат 1 — recs (покрытие и наборы из них), либо явные cov и sets;
// формат 2 — cov, cd1/cd2 (start, end, class), c1×c2 записей vals.
type gbSub struct {
	format, vf1, vf2 int
	ext              bool // обернуть в Extension (тип lookup 9)
	covRange         bool // покрытие форматом 2 (диапазоны)
	cdArray          bool // ClassDef форматом 1 (массив)
	recs             []gbRec
	cov              []int
	sets             [][][3]int // формат 1, явно: набор на индекс покрытия, записи {g2, v1, v2}
	cd1, cd2         [][3]int
	c1, c2           int
	vals             [][2]int
}

type gposBuilder struct {
	scripts      []gbScript
	features     []gbFeature
	lookups      []*gbLookup
	featuresLast bool // FeatureList после LookupList (огромная фича не сдвигает остальное за 64 КБ)
}

// gposImage — собранная таблица и именованные места в ней для порчи байтов в тестах.
type gposImage struct {
	b   []byte
	pos map[string]int
}

func (im *gposImage) clone() []byte { return slices.Clone(im.b) }

// at — место name; нет — ошибка теста.
func (im *gposImage) at(name string) int {
	p, ok := im.pos[name]
	if !ok {
		panic("нет места " + name)
	}
	return p
}

// --- конструкторы -------------------------------------------------------------------

func gbLk(subs ...*gbSub) *gbLookup {
	lk := &gbLookup{typ: 2, subs: subs}
	if len(subs) > 0 && subs[0].ext {
		lk.typ = 9
	}
	return lk
}

// gbF1 — подтаблица формата 1 из пар.
func gbF1(vf1, vf2 int, recs ...gbRec) *gbSub {
	return &gbSub{format: 1, vf1: vf1, vf2: vf2, recs: recs}
}

// gbKern — формат 1 с XAdvance первого глифа: пары {g1, g2, v}.
func gbKern(pairs ...[3]int) *gbSub {
	recs := make([]gbRec, len(pairs))
	for i, p := range pairs {
		recs[i] = gbRec{g1: p[0], g2: p[1], v1: p[2]}
	}
	return gbF1(4, 0, recs...)
}

// gbClassKern — формат 2: первый глиф g1 (класс 1 у остальных 0), второй — g2 (класс 1), одна
// ненулевая запись {1, 1} = v; матрица 2×2.
func gbClassKern(g1, g2, v int) *gbSub {
	return &gbSub{
		format: 2, vf1: 4, cov: []int{g1},
		cd1: [][3]int{{g1, g1, 1}}, cd2: [][3]int{{g2, g2, 1}},
		c1: 2, c2: 2, vals: [][2]int{{0, 0}, {0, 0}, {0, 0}, {v, 0}},
	}
}

// gbSimple — один скрипт tag: DefaultLangSys, фича kern на lookups 0..n-1.
func gbSimple(tag string, lks ...*gbLookup) *gposBuilder {
	nums := make([]int, len(lks))
	for i := range lks {
		nums[i] = i
	}
	return &gposBuilder{
		scripts:  []gbScript{{tag, &gbScriptBody{langs: []gbLang{{"", &gbLangSys{req: 0xFFFF, feats: []int{0}}}}}}},
		features: []gbFeature{{"kern", &gbFeatBody{lookups: nums}}},
		lookups:  lks,
	}
}

// --- сборка -------------------------------------------------------------------------

type gw struct{ b []byte }

func (w *gw) u16(v int) {
	if v < 0 || v > 0xFFFF {
		panic(fmt.Sprintf("u16 вне диапазона: %d", v))
	}
	w.b = binary.BigEndian.AppendUint16(w.b, uint16(v))
}
func (w *gw) u32(v int)    { w.b = binary.BigEndian.AppendUint32(w.b, uint32(v)) }
func (w *gw) tag(s string) { w.b = append(w.b, s...) }
func (w *gw) at() int      { return len(w.b) }
func (w *gw) set16(at, v int) {
	if v < 0 || v > 0xFFFF {
		panic(fmt.Sprintf("смещение u16 вне диапазона: %d", v))
	}
	binary.BigEndian.PutUint16(w.b[at:], uint16(v))
}

// gbRef — слот u16 по адресу at, значение — адрес key минус base.
type gbRef struct {
	at, base int
	key      any
}

type gbEmit struct {
	w    gw
	addr map[any]int
	refs []gbRef
	pos  map[string]int
}

// place пишет объект key один раз; false — он уже есть.
func (e *gbEmit) place(key any, name string) bool {
	if _, ok := e.addr[key]; ok {
		return false
	}
	e.addr[key] = e.w.at()
	if name != "" {
		e.pos[name] = e.w.at()
	}
	return true
}

// ref пишет слот-ссылку u16 на key; значение подставит resolve.
func (e *gbEmit) ref(base int, key any) {
	e.refs = append(e.refs, gbRef{e.w.at(), base, key})
	e.w.u16(0)
}

func (e *gbEmit) resolve() {
	for _, r := range e.refs {
		a, ok := e.addr[r.key]
		if !ok {
			panic("ссылка на неразмещённый объект")
		}
		e.w.set16(r.at, a-r.base)
	}
}

// build собирает таблицу: заголовок, ScriptList, Script, LangSys, FeatureList, Feature,
// LookupList, Lookup, подтаблицы — каждый слой после предыдущего (смещения только вперёд).
func (g *gposBuilder) build() *gposImage {
	e := &gbEmit{addr: map[any]int{}, pos: map[string]int{}}
	e.w.u16(1)
	e.w.u16(0)
	for range 3 {
		e.w.u16(0)
	}
	e.w.set16(4, e.w.at())
	e.pos["scriptList"] = e.w.at()
	langs := g.emitScripts(e)
	g.emitLangSys(e, langs)
	if g.featuresLast {
		g.emitLookupList(e)
		g.emitFeatureList(e)
	} else {
		g.emitFeatureList(e)
		g.emitLookupList(e)
	}
	e.resolve()
	return &gposImage{b: e.w.b, pos: e.pos}
}

func (g *gposBuilder) emitScripts(e *gbEmit) []*gbLangSys {
	base := e.w.at()
	e.w.u16(len(g.scripts))
	for _, s := range g.scripts {
		e.w.tag(s.tag)
		e.ref(base, s.body)
	}
	var out []*gbLangSys
	for i, s := range g.scripts {
		if !e.place(s.body, fmt.Sprintf("script.%d", i)) {
			continue
		}
		base := e.w.at()
		dflt := slices.IndexFunc(s.body.langs, func(l gbLang) bool { return l.tag == "" })
		if dflt < 0 {
			e.w.u16(0)
		} else {
			e.ref(base, s.body.langs[dflt].ls)
		}
		named := slices.DeleteFunc(slices.Clone(s.body.langs), func(l gbLang) bool { return l.tag == "" })
		e.w.u16(len(named))
		for _, l := range named {
			e.w.tag(l.tag)
			e.ref(base, l.ls)
		}
		for _, l := range s.body.langs {
			out = append(out, l.ls)
		}
	}
	return out
}

func (g *gposBuilder) emitLangSys(e *gbEmit, all []*gbLangSys) {
	n := 0
	for _, ls := range all {
		if !e.place(ls, fmt.Sprintf("lang.%d", n)) {
			continue
		}
		n++
		e.w.u16(0)
		e.w.u16(ls.req)
		e.w.u16(len(ls.feats))
		for _, f := range ls.feats {
			e.w.u16(f)
		}
	}
}

func (g *gposBuilder) emitFeatureList(e *gbEmit) {
	e.w.set16(6, e.w.at())
	e.pos["featureList"] = e.w.at()
	g.emitFeatures(e)
}

func (g *gposBuilder) emitLookupList(e *gbEmit) {
	e.w.set16(8, e.w.at())
	e.pos["lookupList"] = e.w.at()
	g.emitLookups(e)
}

func (g *gposBuilder) emitFeatures(e *gbEmit) {
	base := e.w.at()
	e.w.u16(len(g.features))
	for _, f := range g.features {
		e.w.tag(f.tag)
		e.ref(base, f.body)
	}
	for i, f := range g.features {
		if !e.place(f.body, fmt.Sprintf("feature.%d", i)) {
			continue
		}
		e.w.u16(0)
		e.w.u16(len(f.body.lookups))
		for _, l := range f.body.lookups {
			e.w.u16(l)
		}
	}
}

func (g *gposBuilder) emitLookups(e *gbEmit) {
	base := e.w.at()
	e.w.u16(len(g.lookups))
	for _, lk := range g.lookups {
		e.ref(base, lk)
	}
	var subs []*gbSub
	seen := map[*gbSub]bool{}
	for i, lk := range g.lookups {
		if !e.place(lk, fmt.Sprintf("lookup.%d", i)) {
			continue
		}
		base := e.w.at()
		e.w.u16(lk.typ)
		e.w.u16(lk.flag)
		e.w.u16(len(lk.subs))
		for _, s := range lk.subs {
			e.ref(base, s)
			if !seen[s] {
				seen[s] = true
				subs = append(subs, s)
			}
		}
		if lk.flag&0x10 != 0 {
			e.w.u16(0) // markFilteringSet
		}
	}
	for _, s := range subs {
		s.emit(e, g.subName(s))
	}
}

// subName — имя sub.<lookup>.<номер> первой ссылки на подтаблицу.
func (g *gposBuilder) subName(s *gbSub) string {
	for i, lk := range g.lookups {
		if j := slices.Index(lk.subs, s); j >= 0 {
			return fmt.Sprintf("sub.%d.%d", i, j)
		}
	}
	return ""
}

// emit пишет подтаблицу (с обёрткой Extension, если нужна). Имя: sub.* — начало записи
// в lookup (обёртка или тело), inner.* — тело PairPos.
func (s *gbSub) emit(e *gbEmit, name string) {
	e.place(s, name)
	if s.ext {
		e.w.u16(1)
		e.w.u16(2)
		e.w.u32(8)
		e.pos["inner"+name[len("sub"):]] = e.w.at()
	}
	e.w.b = append(e.w.b, s.body()...)
}

type gbLeaf struct {
	slot int // место слота смещения в теле
	b    []byte
}

// body — тело PairPos с относительными смещениями; листья (покрытие, ClassDef, наборы пар)
// лежат после заголовка по возрастанию размера, чтобы большие не выходили за 64 КБ.
func (s *gbSub) body() []byte {
	var w gw
	var leaves []gbLeaf
	slot := func(b []byte) {
		leaves = append(leaves, gbLeaf{w.at(), b})
		w.u16(0)
	}
	w.u16(s.format)
	covSlot := w.at()
	w.u16(0)
	w.u16(s.vf1)
	w.u16(s.vf2)
	cov := s.cov
	switch s.format {
	case 1:
		var sets [][][3]int
		sets, cov = s.pairSets()
		w.u16(len(sets))
		for _, set := range sets {
			slot(s.pairSet(set))
		}
	case 2:
		slot(s.classDef(s.cd1))
		slot(s.classDef(s.cd2))
		w.u16(s.c1)
		w.u16(s.c2)
		for _, v := range s.vals {
			w.b = append(w.b, vrec(s.vf1, v[0])...)
			w.b = append(w.b, vrec(s.vf2, v[1])...)
		}
	}
	leaves = append(leaves, gbLeaf{covSlot, s.coverage(cov)})
	sort.SliceStable(leaves, func(a, b int) bool { return len(leaves[a].b) < len(leaves[b].b) })
	for _, l := range leaves {
		w.set16(l.slot, w.at())
		w.b = append(w.b, l.b...)
	}
	return w.b
}

// pairSets группирует recs по первому глифу: наборы по возрастанию первого, покрытие — они же.
func (s *gbSub) pairSets() ([][][3]int, []int) {
	if s.sets != nil || s.recs == nil {
		return s.sets, s.cov
	}
	by := map[int][][3]int{}
	for _, r := range s.recs {
		by[r.g1] = append(by[r.g1], [3]int{r.g2, r.v1, r.v2})
	}
	firsts := make([]int, 0, len(by))
	for g := range by {
		firsts = append(firsts, g)
	}
	slices.Sort(firsts)
	sets := make([][][3]int, len(firsts))
	for i, g := range firsts {
		set := by[g]
		sort.SliceStable(set, func(a, b int) bool { return set[a][0] < set[b][0] })
		sets[i] = set
	}
	return sets, firsts
}

func (s *gbSub) pairSet(set [][3]int) []byte {
	var w gw
	w.u16(len(set))
	for _, p := range set {
		w.u16(p[0])
		w.b = append(w.b, vrec(s.vf1, p[1])...)
		w.b = append(w.b, vrec(s.vf2, p[2])...)
	}
	return w.b
}

// vrec — ValueRecord формата vf: в слоте XAdvance значение v, в остальных 7 (чужое поле,
// которое разбор обязан пропустить).
func vrec(vf, v int) []byte {
	var w gw
	for bit := 1; bit <= 0x80; bit <<= 1 {
		switch {
		case vf&bit == 0:
		case bit == 4:
			w.b = binary.BigEndian.AppendUint16(w.b, uint16(int16(v)))
		default:
			w.u16(7)
		}
	}
	return w.b
}

// coverage кодирует покрытие: список глифов (формат 1) или диапазоны (формат 2).
func (s *gbSub) coverage(gs []int) []byte {
	var w gw
	if !s.covRange {
		w.u16(1)
		w.u16(len(gs))
		for _, g := range gs {
			w.u16(g)
		}
		return w.b
	}
	type rng struct{ start, end, idx int }
	var rs []rng
	for i, g := range gs {
		if n := len(rs); n > 0 && rs[n-1].end+1 == g {
			rs[n-1].end = g
			continue
		}
		rs = append(rs, rng{g, g, i})
	}
	w.u16(2)
	w.u16(len(rs))
	for _, r := range rs {
		w.u16(r.start)
		w.u16(r.end)
		w.u16(r.idx)
	}
	return w.b
}

// classDef кодирует ClassDef: диапазоны (формат 2) или массив (формат 1).
func (s *gbSub) classDef(rs [][3]int) []byte {
	var w gw
	if s.cdArray && len(rs) > 0 {
		start, end := rs[0][0], rs[len(rs)-1][1]
		classes := make([]int, end-start+1)
		for _, r := range rs {
			for g := r[0]; g <= r[1]; g++ {
				classes[g-start] = r[2]
			}
		}
		w.u16(1)
		w.u16(start)
		w.u16(len(classes))
		for _, c := range classes {
			w.u16(c)
		}
		return w.b
	}
	w.u16(2)
	w.u16(len(rs))
	for _, r := range rs {
		w.u16(r[0])
		w.u16(r[1])
		w.u16(r[2])
	}
	return w.b
}
