package text

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"slices"
)

// Разбор кернинга GPOS своего шрифта (161): lookups фичи kern по скриптам, как их
// выбирает HarfBuzz. Вход недоверенный (таблица из файла фотографа), поэтому у каждого
// счётчика есть предел, а у всего разбора — бюджет чтений: пределы по отдельности не
// ограничивают их произведение на общих смещениях. Запрос пар — gpos_pairs.go.

var (
	errGPOS       = errors.New("GPOS: таблица не читается")
	errGPOSLimit  = errors.New("GPOS: превышен предел размера")
	errGPOSBudget = errors.New("GPOS: превышен бюджет чтений")
	errGPOSSteps  = errors.New("GPOS: превышен бюджет шагов запроса")
)

// Пределы (design §5): в 370 прочитанных шрифтах максимум 26 lookups и 42 подтаблицы.
const (
	gposMaxScripts      = 64
	gposMaxLangSys      = 128
	gposMaxFeatIdx      = 512  // индексов фич в LangSys
	gposMaxFeatLks      = 64   // lookups у одной фичи
	gposMaxScriptLks    = 64   // разных lookups у скрипта
	gposMaxLookups      = 256  // разных lookups в таблице
	gposMaxSubsInLookup = 1024 // подтаблиц в заголовке lookup
	gposMaxSubsScript   = 512  // принятых подтаблиц у скрипта
	gposMaxSubsTotal    = 2048 // принятых подтаблиц всего
	gposMaxValueFormat  = 0xFF
	gposMaxReads        = 1_000_000 // чтений разбора (u16/u32/tag — по единице)
	gposMinUPEM         = 16        // как у sfnt
	gposMaxUPEM         = 16384
)

func gposTag(s string) uint32 { return binary.BigEndian.Uint32([]byte(s)) }

var (
	tagKern = gposTag("kern")
	tagRUS  = gposTag("RUS ")
)

// gposTable — кернинг GPOS одного начертания: срез таблицы (без копии) и разобранные
// lookups фичи kern. После разбора только читается — из разных горутин безопасно.
type gposTable struct {
	b       []byte
	scripts map[uint32][]int // тег скрипта → номера lookups kern (по возрастанию, без повторов)
	lookups map[int]*kernLookup
	upem    int
}

// kernLookup — подтаблицы PairPos (тип 2 и 9→2) в порядке файла.
type kernLookup struct{ subs []pairSub }

// pairSub — заголовок подтаблицы PairPos; смещения абсолютные (от начала GPOS), 0 — нет.
type pairSub struct {
	format, off, cov, vf1, vf2 int
	n1, n2                     int // размер ValueRecord первого и второго глифа, байт
	cd1, cd2, c1, c2           int // формат 2: ClassDef и число классов
}

// hasKern — есть ли хоть у одного скрипта lookups kern (иначе кернит sfnt, как до 161).
func (t *gposTable) hasKern() bool {
	for _, lks := range t.scripts {
		if len(lks) > 0 {
			return true
		}
	}
	return false
}

// parseGPOSRaw разбирает таблицу GPOS без recover: выход за срез здесь — паника, которую
// увидят тесты и фаззы (в обёртке она стала бы «нет кернинга»). reads — число чтений,
// всегда ≤ gposMaxReads. (nil, n, nil) — фичи kern нет: кернит sfnt.
func parseGPOSRaw(table []byte, upem int) (t *gposTable, reads int, err error) {
	if upem < gposMinUPEM || upem > gposMaxUPEM {
		return nil, 0, errGPOS
	}
	p := &gposParser{b: table, t: &gposTable{
		b: table, upem: upem, scripts: map[uint32][]int{}, lookups: map[int]*kernLookup{},
	}}
	p.run()
	switch {
	case p.err != nil:
		return nil, p.n, p.err
	case !p.t.hasKern():
		return nil, p.n, nil
	}
	return p.t, p.n, nil
}

// parseGPOS — обёртка для продукта: паника сырого тела (но не sfnt) — отказ таблицы; текст
// паники (runtime error без путей и имён файлов) идёт в ошибку для журнала.
func parseGPOS(table []byte, upem int) (t *gposTable, err error) {
	if v := panicked(func() { t, _, err = parseGPOSRaw(table, upem) }); v != nil {
		return nil, fmt.Errorf("%w: %v", errGPOS, v)
	}
	return t, err
}

// panicked вызывает fn и возвращает значение паники (nil — паники не было; panic(nil) с Go 1.21
// тоже даёт значение). recover только вокруг сырых тел GPOS.
func panicked(fn func()) (v any) {
	defer func() { v = recover() }()
	fn()
	return nil
}

// idxSet — множество u16-индексов (повторы индексов фич и lookups пропускаются).
type idxSet [1 << 10]uint64

func (s *idxSet) reset() { clear(s[:]) }

// add — true, если индекса ещё не было.
func (s *idxSet) add(i int) bool {
	w, m := i>>6, uint64(1)<<(i&63)
	if s[w]&m != 0 {
		return false
	}
	s[w] |= m
	return true
}

// gposParser читает таблицу с «липкой» ошибкой: после первой неудачи все чтения отдают 0,
// циклы вызывающих выходят по p.err.
type gposParser struct {
	b          []byte
	t          *gposTable
	n          int // чтений
	err        error
	flo, llo   int // FeatureList, LookupList
	nfeat, nlk int
	subs       int // принятых подтаблиц у разных lookups
	feats, lks idxSet
}

func (p *gposParser) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

// read — одно чтение размера sz по смещению off: граница среза и бюджет.
func (p *gposParser) read(off, sz int) []byte {
	switch {
	case p.err != nil:
		return nil
	case p.n >= gposMaxReads:
		p.fail(errGPOSBudget)
		return nil
	case off < 0 || off > len(p.b)-sz:
		p.fail(errGPOS)
		return nil
	}
	p.n++
	return p.b[off : off+sz]
}

func (p *gposParser) u16(off int) int {
	if v := p.read(off, 2); v != nil {
		return int(binary.BigEndian.Uint16(v))
	}
	return 0
}

func (p *gposParser) u32(off int) int {
	if v := p.read(off, 4); v != nil {
		return int(binary.BigEndian.Uint32(v))
	}
	return 0
}

func (p *gposParser) tag(off int) uint32 {
	if v := p.read(off, 4); v != nil {
		return binary.BigEndian.Uint32(v)
	}
	return 0
}

// run: шапка и скрипты. Смещение 0 у списка — NULL, то есть пусто.
func (p *gposParser) run() {
	if p.u16(0) != 1 { // major; minor 0 и 1 читаются одинаково (FeatureVariations не нужны)
		p.fail(errGPOS)
		return
	}
	slo, flo, llo := p.u16(4), p.u16(6), p.u16(8)
	if p.err != nil || slo == 0 || flo == 0 || llo == 0 {
		return
	}
	p.flo, p.llo = flo, llo
	p.nfeat, p.nlk = p.u16(flo), p.u16(llo)
	nsc := p.u16(slo)
	if nsc > gposMaxScripts {
		p.fail(errGPOSLimit)
	}
	for i := 0; i < nsc && p.err == nil; i++ {
		p.script(slo, i)
	}
}

// script разбирает запись скрипта i: lookups kern его LangSys и сами lookups.
func (p *gposParser) script(slo, i int) {
	rec := slo + 2 + 6*i
	tag, so := p.tag(rec), p.u16(rec+4)
	if _, dup := p.t.scripts[tag]; dup || p.err != nil {
		return
	}
	var lks []int
	if so != 0 {
		lks = p.scriptLookups(slo + so)
	}
	if p.err != nil {
		return
	}
	p.t.scripts[tag] = lks
	p.addLookups(lks)
}

// scriptLookups — номера lookups kern: LangSys «RUS » (документ lang="ru"), иначе
// DefaultLangSys; нет ни того ни другого — пусто.
func (p *gposParser) scriptLookups(sc int) []int {
	ls, nls := p.u16(sc), p.u16(sc+2)
	if nls > gposMaxLangSys {
		p.fail(errGPOSLimit)
	}
	for j := 0; j < nls && p.err == nil; j++ {
		rec := sc + 4 + 6*j
		if p.tag(rec) == tagRUS {
			ls = p.u16(rec + 4)
			break
		}
	}
	if ls == 0 || p.err != nil {
		return nil
	}
	return p.langLookups(sc + ls)
}

// langLookups собирает lookups фич kern LangSys: без повторов, по возрастанию номера
// (HarfBuzz применяет lookups стадии в этом порядке).
func (p *gposParser) langLookups(lso int) []int {
	req, nfi := p.u16(lso+2), p.u16(lso+4)
	if nfi > gposMaxFeatIdx {
		p.fail(errGPOSLimit)
	}
	p.feats.reset()
	p.lks.reset()
	var lks []int
	if req != 0xFFFF {
		lks = p.featureLookups(req, lks)
	}
	for f := 0; f < nfi && p.err == nil; f++ {
		lks = p.featureLookups(p.u16(lso+6+2*f), lks)
	}
	slices.Sort(lks)
	return lks
}

// featureLookups дописывает lookups фичи fi, если это kern. Индекс фичи вне списка и
// повтор индекса пропускаются (как в HarfBuzz); пределы — отказ.
func (p *gposParser) featureLookups(fi int, lks []int) []int {
	if p.err != nil || fi >= p.nfeat || !p.feats.add(fi) {
		return lks
	}
	rec := p.flo + 2 + 6*fi
	tag, fo := p.tag(rec), p.u16(rec+4)
	if p.err != nil || tag != tagKern || fo == 0 {
		return lks
	}
	fe := p.flo + fo
	nl := p.u16(fe + 2)
	if nl > gposMaxFeatLks {
		p.fail(errGPOSLimit)
	}
	for l := 0; l < nl && p.err == nil; l++ {
		li := p.u16(fe + 4 + 2*l)
		if li >= p.nlk || !p.lks.add(li) {
			continue
		}
		if lks = append(lks, li); len(lks) > gposMaxScriptLks {
			p.fail(errGPOSLimit)
		}
	}
	return lks
}

// addLookups разбирает ещё не виденные lookups скрипта и сверяет суммы подтаблиц.
func (p *gposParser) addLookups(lks []int) {
	sum := 0
	for _, li := range lks {
		lk, done := p.t.lookups[li]
		if !done {
			if len(p.t.lookups) >= gposMaxLookups {
				p.fail(errGPOSLimit)
				return
			}
			if lk = p.lookup(li); p.err != nil {
				return
			}
			p.t.lookups[li] = lk
			p.subs += len(lk.subs)
		}
		sum += len(lk.subs)
		if sum > gposMaxSubsScript || p.subs > gposMaxSubsTotal {
			p.fail(errGPOSLimit)
			return
		}
	}
}

// lookup разбирает lookup li. Не PairPos и UseMarkFilteringSet (0x10) — пустой, как у sfnt.
func (p *gposParser) lookup(li int) *kernLookup {
	lk := &kernLookup{}
	lo := p.u16(p.llo + 2 + 2*li)
	if lo == 0 {
		return lk
	}
	lt := p.llo + lo
	typ, flag, ns := p.u16(lt), p.u16(lt+2), p.u16(lt+4)
	if ns > gposMaxSubsInLookup {
		p.fail(errGPOSLimit)
	}
	if p.err != nil || flag&0x10 != 0 || (typ != 2 && typ != 9) {
		return lk
	}
	for i := 0; i < ns && p.err == nil; i++ {
		so := p.u16(lt + 6 + 2*i)
		if so == 0 {
			continue
		}
		if sub, ok := p.subtable(typ, lt+so); ok {
			lk.subs = append(lk.subs, sub)
		}
	}
	return lk
}

// subtable читает подтаблицу s lookup типа typ; Extension (9) разворачивается один раз.
// ok=false — чужая подтаблица, её пропускаем; нечитаемая — отказ (p.err).
func (p *gposParser) subtable(typ, s int) (pairSub, bool) {
	if typ == 9 {
		format, inner, off := p.u16(s), p.u16(s+2), p.u32(s+4)
		if p.err != nil {
			return pairSub{}, false
		}
		if format != 1 || inner == 9 {
			p.fail(errGPOS)
			return pairSub{}, false
		}
		if inner != 2 {
			return pairSub{}, false
		}
		s += off // u32 < 2³², int 64 бит — не переполняется
	}
	return p.pairSub(s)
}

// vrSize — размер ValueRecord в байтах по ValueFormat (по 2 байта на бит младшего байта).
func vrSize(vf int) int { return 2 * bits.OnesCount8(uint8(vf&gposMaxValueFormat)) }

func (p *gposParser) pairSub(s int) (pairSub, bool) {
	x := pairSub{off: s}
	x.format, x.cov = p.u16(s), p.u16(s+2)
	x.vf1, x.vf2 = p.u16(s+4), p.u16(s+6)
	if p.err != nil || x.cov == 0 || x.vf1 > gposMaxValueFormat || x.vf2 > gposMaxValueFormat {
		return x, false
	}
	x.cov += s
	x.n1, x.n2 = vrSize(x.vf1), vrSize(x.vf2)
	switch x.format {
	case 1:
		return x, true
	case 2:
		return x, p.classes(&x)
	}
	return x, false
}

// classes дочитывает заголовок формата 2: матрица c1×c2 записей должна лежать в таблице
// целиком (сравнение в int64: 65535² × 32 не переполняется).
func (p *gposParser) classes(x *pairSub) bool {
	x.cd1, x.cd2 = p.u16(x.off+8), p.u16(x.off+10)
	x.c1, x.c2 = p.u16(x.off+12), p.u16(x.off+14)
	if p.err != nil {
		return false
	}
	if x.cd1 != 0 {
		x.cd1 += x.off
	}
	if x.cd2 != 0 {
		x.cd2 += x.off
	}
	if int64(x.off)+16+int64(x.c1)*int64(x.c2)*int64(x.n1+x.n2) > int64(len(p.b)) {
		p.fail(errGPOS)
		return false
	}
	return true
}
