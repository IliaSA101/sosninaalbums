package variety

// Ряд «Авто» по формам (спека 051, design §6): первая — прежняя лучшая (Layout "" и «Меньше» не
// меняются), дальше через одну — лучшая ещё не показанной формы (forms.Gen и featured), между
// ними — следующие из ряда layout. Подборка, счётчик «i / N» и листание берут ряд отсюда.
// Лесенка в ряду бывает дважды (055): вторая — другого направления, встаёт после прочих форм.

import (
	"cmp"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// rowQSlack — форма встаёт в ряд, если её Q не хуже Q лучшей раскладки на столько: дальше она
// заметно слабее мозаики, и в начале ряда её показывать незачем.
const rowQSlack = 0.25

// centerQSlack — «центр крупнее» (forms.CenterBig) идёт в «Другие варианты», если его Q не хуже Q
// лучшей раскладки на столько: центр на сгибе штрафуется spineTerm (≈0,12 на 3 вертикальных 2:3),
// и при rowQSlack он едва не проходил (0,6056 против пола 0,6069).
const centerQSlack = 0.5

// manySideSlack — «не мельче мозаики» на многих фото (072-2): мельчайшая сторона формы не меньше
// этой доли той же у Row[0]. Мозаики 2–8-й плиток того же ряда уже бывают на 10–15 % мельче первой,
// а при 1,0 на 30 фото смеси в трети наборов оставалась одна мозаика.
const manySideSlack = 0.9

// rowEntry — ряд «Авто» и то, что о нём нужно пулу и «Другим вариантам».
type rowEntry struct {
	row   []forms.Cand
	res   []layout.Result // те же раскладки без формы: model работает с []layout.Result
	kind  map[string]Kind // featured в ряду — их семейство (Pick судит их по Fair, как 041)
	gen   map[string]bool // вставлены формой (Gen, featured), а не из ряда layout: допуск по Q
	feats []Feat          // featured вне ряда для «Других вариантов» (bestFeatured), до ManyFrom фото
	mid   *forms.Cand     // «центр крупнее» (contrast) вне ряда для «Других вариантов» (midBig)
}

// rowCache — ряды по пропорциям и геометрии: View берёт ряд у каждого разворота на каждый запрос,
// а без кэша это Gen, featured и Quality на каждую раскладку. Два поколения, как у extraCache.
var rowCache genCache[rowEntry]

// Row — ряд «Авто» разворота длиной не больше rowLen: Row[0] = layout.Cached[0] всегда; на
// позициях 1, 3, 5, … — лучшая по Q ещё не показанной формы (Q ≥ Q[0] − rowQSlack; ключ и вид не
// повторяются; форма — кроме пары лесенки, ladderPair), между ними — следующие из layout.Cached;
// кончился один источник — берётся другой.
// С ManyFrom фото формы только в первой порции (ManyBatch), дальше — ряд layout без показанных:
// Row(…, 24)[:12] == Row(…, 12). Результат общий: не менять.
func Row(aspects []float64, p layout.Params, rowLen int) []forms.Cand {
	return rowOf(aspects, p, rowLen).row
}

// RowResults — тот же ряд как []layout.Result (без копирования: общий, не менять).
func RowResults(aspects []float64, p layout.Params, rowLen int) []layout.Result {
	return rowOf(aspects, p, rowLen).res
}

func rowOf(aspects []float64, p layout.Params, rowLen int) rowEntry {
	key := layout.CacheKey(aspects, p, rowLen)
	e, ok := rowCache.get(key)
	if ok {
		return e
	}
	base := layout.Cached(aspects, p, rowLen)
	if len(aspects) >= layout.ManyFrom && rowLen > layout.ManyBatch {
		e = tailRow(rowOf(aspects, p, layout.ManyBatch), base, rowLen, p)
	} else {
		e = buildRow(aspects, p, rowLen, base)
	}
	e.res = make([]layout.Result, len(e.row))
	for i, c := range e.row {
		e.res[i] = c.Result
	}
	rowCache.put(key, e)
	return e
}

// candOf — раскладка ряда layout с её формой и Q (пол формы здесь не судит: ряд берётся как есть).
func candOf(r layout.Result, p layout.Params) forms.Cand {
	f := forms.Of(r.Key, r.Rects, p)
	q, _ := forms.Quality(r, f, p)
	return forms.Cand{Result: r, Form: f, Q: q}
}

// rowBuild — сборка ряда: что уже показано, кандидаты форм и позиция в ряду layout.
type rowBuild struct {
	e     rowEntry
	p     layout.Params
	keys  map[string]bool
	looks map[string]bool
	shown map[forms.Form]bool

	base  []layout.Result
	bi    int // следующий из base
	cands []forms.Cand
	kinds map[string]Kind
	used  []bool
	clook []string // вид кандидата — по требованию: до порога доходят немногие
	floor float64  // пол Q формы в квантах
	as    []float64
	pair  *forms.Cand // вторая лесенка: ждёт, пока кончатся прочие формы (055)
}

func (b *rowBuild) add(c forms.Cand, formed bool) {
	b.keys[c.Key], b.looks[Look(c.Rects, b.p)], b.shown[c.Form] = true, true, true
	b.e.row = append(b.e.row, c)
	if formed {
		b.e.gen[c.Key] = true
	}
}

// nextForm — лучшая по Q ещё не показанной формы с новым ключом и видом; таких нет — вторая
// лесенка (takePair); false — нет и её.
func (b *rowBuild) nextForm() bool {
	for i, c := range b.cands {
		if quanta(c.Q) < b.floor {
			break
		}
		if b.used[i] || b.shown[c.Form] || b.keys[c.Key] {
			continue
		}
		if b.clook[i] == "" {
			b.clook[i] = Look(c.Rects, b.p)
		}
		if b.looks[b.clook[i]] {
			continue
		}
		b.used[i] = true
		b.add(c, true)
		if k := b.kinds[c.Key]; k != NoKind {
			b.e.kind[c.Key] = k
		}
		if c.Form == forms.Ladder {
			b.pair = b.ladderPair(c)
		}
		return true
	}
	return b.takePair()
}

// ladderPair — пара первой лесенки ряда c: лучший по Q кандидат-лесенка другого направления и
// другого вида (разворот ↔ страница) не ниже пола ряда; нет такого — зеркало c (L ↔ A). Так в
// ряду есть и вниз, и вверх; первая остаётся по (Q, ключ) — ключи рядов альбомов 0.13 на месте.
func (b *rowBuild) ladderPair(c forms.Cand) *forms.Cand {
	dir, paged := forms.LadderDir(c.Key), layout.IsPaged(c.Key)
	for i, d := range b.cands {
		if quanta(d.Q) < b.floor {
			break
		}
		if !b.used[i] && d.Form == forms.Ladder && forms.LadderDir(d.Key) != dir && layout.IsPaged(d.Key) != paged {
			b.used[i] = true
			return &d
		}
	}
	r, ok := layout.Build(forms.MirrorLadder(c.Key), b.as, b.p)
	if !ok {
		return nil
	}
	if q, ok := forms.Quality(r, forms.Ladder, b.p); ok && quanta(q) >= b.floor {
		return &forms.Cand{Result: r, Form: forms.Ladder, Q: q}
	}
	return nil
}

// takePair — вторая лесенка, если она есть и её ключ и вид в ряду новые; ставится один раз.
func (b *rowBuild) takePair() bool {
	c := b.pair
	b.pair = nil
	if c == nil || b.keys[c.Key] || b.looks[Look(c.Rects, b.p)] {
		return false
	}
	b.add(*c, true)
	return true
}

// midBig — лучший по Q «центр крупнее» (contrast) вне ряда, не ниже centerQSlack от лучшей
// раскладки. В ряду форма одна — левое крупное или Q[…|…] выигрывает по Q, и центр не показывался
// совсем (0.23-q6); в ряд его не ставим: ряд — это выбор автосборки, он зафиксирован (вариант 1 =
// 0.19.0, variantDigest19), а «Другие варианты» сборка не берёт (ни InRow, ни Formed, ни Air).
func (b *rowBuild) midBig(first float64) *forms.Cand {
	for _, c := range b.cands {
		if quanta(c.Q) < quanta(first-centerQSlack) {
			break
		}
		if c.Form == forms.Contrast && forms.CenterBig(c.Key) && !b.keys[c.Key] {
			return &c
		}
	}
	return nil
}

// nextBase — следующий ещё не показанный из ряда layout; false — ряд кончился.
func (b *rowBuild) nextBase() bool {
	for ; b.bi < len(b.base); b.bi++ {
		if r := b.base[b.bi]; !b.keys[r.Key] {
			b.add(candOf(r, b.p), false)
			b.bi++
			return true
		}
	}
	return false
}

// buildRow — ряд из base (layout.Cached) и форм; первые head позиций чередуются с формами.
func buildRow(as []float64, p layout.Params, rowLen int, base []layout.Result) rowEntry {
	b := &rowBuild{e: rowEntry{kind: map[string]Kind{}, gen: map[string]bool{}}, p: p,
		keys: map[string]bool{}, looks: map[string]bool{}, shown: map[forms.Form]bool{}, base: base, bi: 1, as: as}
	if len(base) == 0 || rowLen <= 0 {
		return b.e
	}
	head := rowLen
	if len(as) >= layout.ManyFrom {
		head = min(rowLen, layout.ManyBatch)
	}
	all := featured(as, p)
	b.cands, b.kinds = formCands(as, p, all, formRelax(as, base[0]))
	b.used, b.clook = make([]bool, len(b.cands)), make([]string, len(b.cands))
	first := candOf(base[0], p)
	b.floor = quanta(first.Q - rowQSlack)
	b.add(first, false)
	for len(b.e.row) < head {
		// позиции 1, 3, 5… — форма, остальные — ряд layout; кончился один источник — другой
		if len(b.e.row)%2 == 1 && b.nextForm() || b.nextBase() || b.nextForm() {
			continue
		}
		break
	}
	for len(b.e.row) < rowLen {
		if !b.nextBase() {
			break
		}
	}
	if len(as) < layout.ManyFrom {
		b.e.feats = bestFeatured(all, base, cloneSet(b.keys))
		b.e.mid = b.midBig(first.Q)
	}
	return b.e
}

// cloneSet — копия множества ключей (bestFeatured его пополняет).
func cloneSet(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// tailRow — ряд rowLen на ManyFrom+ фото: первая порция head как есть, дальше base без показанных.
func tailRow(head rowEntry, base []layout.Result, rowLen int, p layout.Params) rowEntry {
	e := rowEntry{row: slices.Clone(head.row), kind: head.kind, gen: head.gen}
	seen := make(map[string]bool, len(e.row))
	for _, c := range e.row {
		seen[c.Key] = true
	}
	for _, r := range base {
		if len(e.row) >= rowLen {
			break
		}
		if !seen[r.Key] {
			e.row = append(e.row, candOf(r, p))
		}
	}
	return e
}

// formRelax — с ManyFrom фото пол мельчайшей стороны форм Gen не строже manySideSlack той же у
// лучшей раскладки (мм): на 25+ фото полы видов отсекали почти все формы, хотя мозаика там не
// крупнее (решение 0.13 №40, 072-2). До ManyFrom — 0: полы вида как есть.
func formRelax(as []float64, best layout.Result) float64 {
	if len(as) < layout.ManyFrom || len(best.Rects) == 0 {
		return 0
	}
	m := math.Inf(1)
	for _, r := range best.Rects {
		m = math.Min(m, math.Min(r.W, r.H))
	}
	return manySideSlack * m
}

// formCands — кандидаты форм: Gen (пол мельчайшей стороны не строже relax мм, formRelax) и
// featured (главное, через корешок, пара — их Of по геометрии), прошедшие полы формы; без мозаики
// (она — ряд layout), без повторов ключа; по убыванию Q в квантах, ничья — по ключу. kinds —
// семейство featured по ключу.
func formCands(as []float64, p layout.Params, feats []Feat, relax float64) ([]forms.Cand, map[string]Kind) {
	out := slices.DeleteFunc(forms.GenMin(as, p, relax), func(c forms.Cand) bool { return c.Form == forms.Mosaic })
	seen := make(map[string]bool, len(out)+len(feats))
	for _, c := range out {
		seen[c.Key] = true
	}
	kinds := make(map[string]Kind, len(feats))
	for _, f := range feats {
		if seen[f.Key] {
			continue
		}
		form := forms.Of(f.Key, f.Rects, p)
		if q, ok := forms.Quality(f.Result, form, p); ok && form != forms.Mosaic {
			seen[f.Key] = true
			kinds[f.Key] = f.Kind
			out = append(out, forms.Cand{Result: f.Result, Form: form, Q: q})
		}
	}
	slices.SortStableFunc(out, func(a, b forms.Cand) int {
		return cmp.Or(cmp.Compare(quanta(b.Q), quanta(a.Q)), cmp.Compare(a.Key, b.Key))
	})
	return out, kinds
}

// RowCached — ряд «Авто» уже посчитан (проверка прогрева).
func RowCached(aspects []float64, p layout.Params, rowLen int) bool {
	return rowCache.has(layout.CacheKey(aspects, p, rowLen))
}
