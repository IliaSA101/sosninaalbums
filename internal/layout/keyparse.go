package layout

// Грамматика ключа раскладки: дерево, разбор, каноническая запись и преобразования
// позиций (mapLeaves, relabel, shapeKey). Геометрия по ключу — key.go и shapes.go.

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Ключ однозначно описывает раскладку с точностью до размеров страницы: это и
// идентификатор варианта, и то, что хранится в файле. Листья — позиции фото во
// входе (0…n−1), сквозные по всему развороту (EBNF и разбор — спека 051, c0-grammar.md):
//
//	ключ     = разворот                                      только layflat
//	         | "P[" стр "|" стр "]"                          постранично
//	         | "Q" ["U"|"D"] "[" корень "|" корень "]"       страницы одной высоты (051)
//	         | "B[full:" поз "]" | "B[right:" поз "]" | "B[pages:" поз "," поз "]"   в край
//	разворот = корень | ("S"|"U"|"D") "(" корень ")"         поменьше / вверху / внизу (051)
//	стр      = "-"                                           пустая страница
//	         | корень                                        вписано в поля по центру
//	         | ("S"|"O"|"I"|"U"|"D") "(" корень ")"          поменьше / к внешнему краю /
//	                                                         к корешку / вверх / вниз
//	         | "E(" поз ")"                                  одно фото в край на всю страницу
//	корень   = поз | "V(" дерево ("," дерево)+ ")"
//	         | "H(" hдет ("," hдет)+ ")"                     не все hдет — m
//	         | "G(" ряд ("," ряд)+ ")"                       ряды одной высоты (051)
//	         | ("L"|"A") "(" поз "," поз "," поз ["," поз ["," поз]] ")"   лесенка (051)
//	         | "Z(" зв {"," зв} ")"                          разброс (105): 1–5 фото одной высоты
//	hдет     = дерево | "m" поз | "n" поз                    m, n — фото поменьше в ряду (051, 105)
//	зв       = ("a"|"b"|"c"|"d"|"e"|"f"|"g") поз             уровень: доля свободной высоты 0, ¼, ⅓, ½, ⅔, ¾, 1
//	ряд      = поз | "H(" поз ("," поз)+ ")"
//	дерево   = поз | ("H"|"V") "(" дерево ("," дерево)+ ")"
//	поз      = "0" | ненуль {цифра}                          не больше 4 цифр
//
// Первые символы альтернатив различаются — разбор без возвратов, а новые ветки 051
// начинаются там, где прежний разбор отказывал. Цифры стоят только в позициях (на этом
// стоит mapLeaves), «[ ] | :» — только на верхнем уровне.
//
// Ключ не зависит от Params: после смены настроек Build либо строит то же
// дерево в новой геометрии, либо отказывает. У одной раскладки один ключ: пишет его
// только parsedKey.String, и Build отвергает запись, отличную от неё (P[-|E(i)] —
// это B[right:i], P[E(i)|E(j)] — B[pages:i,j]).

// MaxKeyLen — ключ длиннее не разбирается: ключ из файла — недоверенный ввод, а глубину
// рекурсии разбора ограничивает только длина (переполнение стека в Go не перехватить).
// Самый длинный ключ, который Build строит на MaxPhotos фото, — меньше 500 байт.
const MaxKeyLen = 2048

type keyTree struct {
	leaf  int  // позиция фото; -1 у составного узла
	op    byte // составной узел: 'H', 'V', 'G' — ряды одной высоты, 'L'/'A' — лесенка, 'Z' — разброс
	small bool // лист m<i> или n<i> — поменьше в ряду корня
	big   bool // лист n<i>: поменьше с долей kappaN (105), только вместе с small
	lvl   int8 // ребёнок Z: уровень a…g = 1…7; 0 — не ребёнок Z
	kids  []*keyTree
}

func (t *keyTree) String() string {
	if t == nil {
		return "-"
	}
	if t.leaf >= 0 {
		switch {
		case t.lvl > 0:
			return string(rune('a'+t.lvl-1)) + strconv.Itoa(t.leaf)
		case t.big:
			return "n" + strconv.Itoa(t.leaf)
		case t.small:
			return "m" + strconv.Itoa(t.leaf)
		}
		return strconv.Itoa(t.leaf)
	}
	parts := make([]string, len(t.kids))
	for i, k := range t.kids {
		parts[i] = k.String()
	}
	return string(t.op) + "(" + strings.Join(parts, ",") + ")"
}

func (t *keyTree) leaves(out []int) []int {
	if t == nil {
		return out
	}
	if t.leaf >= 0 {
		return append(out, t.leaf)
	}
	for _, k := range t.kids {
		out = k.leaves(out)
	}
	return out
}

// node собирает дерево H/V для расчёта; листья ссылаются на позиции во входе.
// Лист m (n) — ячейка высоты h шириной kappa·h·a (kappaN), фото в ней kappa·h по центру (place).
func (t *keyTree) node(aspects []float64, gap float64) *node {
	if t.leaf >= 0 {
		if t.small {
			kp := kappa
			if t.big {
				kp = kappaN
			}
			return &node{leaf: t.leaf, a: 1 / (kp * aspects[t.leaf]), small: true, big: t.big, key: t.String()}
		}
		return &node{leaf: t.leaf, a: 1 / aspects[t.leaf], key: strconv.Itoa(t.leaf)}
	}
	kids := make([]*node, len(t.kids))
	for i, k := range t.kids {
		kids[i] = k.node(aspects, gap)
	}
	return compose(t.op == 'H', kids, gap)
}

type parsedKey struct {
	kind  byte        // 'T' — на весь разворот, 'P' — постранично, 'Q' — страницы одной высоты, 'B' — в край
	mod   byte        // 'T': S/U/D разворота, 'Q': U/D; 0 — по центру
	tree  *keyTree    // 'T'
	pages [2]*keyTree // 'P', 'Q'; nil — пустая страница (только у 'P')
	mods  [2]byte     // 'P': модификатор страницы (pageWrap или E), 0 — вписано по центру
	bleed string      // 'B': full | right | pages
	idx   []int       // 'B': позиции фото по ячейкам
}

// pageMods — модификаторы, которые variants пробует у страницы в ряду PageChoices:
// E — в край, S — поменьше по центру, O — прижато к внешнему краю, I — к корешку.
// Не расширять: U/D в ряду сменили бы PageChoices и эталоны layouts.sha и first16.sha
// (такие ключи строит только генератор форм). Разбор принимает pageWrap и E.
const pageMods = "ESOI"

// pageWrap — модификаторы страницы вокруг корня; U/D — блок вверх / вниз (051).
const pageWrap = "SOIUD"

// spreadMods — модификаторы разворота: S — поменьше по центру, U/D — вверху / внизу (051).
const spreadMods = "SUD"

// pageString — запись страницы в ключе.
func pageString(mod byte, t *keyTree) string {
	if t == nil || mod == 0 {
		return t.String()
	}
	return string(mod) + "(" + t.String() + ")"
}

// joinPages собирает постраничный ключ; страницы «в край» без второй страницы
// или с такой же второй получают ключ B[…], который был у них до модификаторов.
func joinPages(l, r string) string {
	inner := func(s string) (string, bool) {
		if strings.HasPrefix(s, "E(") {
			return s[2 : len(s)-1], true
		}
		return "", false
	}
	if ri, ok := inner(r); ok {
		if l == "-" {
			return "B[right:" + ri + "]"
		}
		if li, ok := inner(l); ok {
			return "B[pages:" + li + "," + ri + "]"
		}
	}
	return "P[" + l + "|" + r + "]"
}

// String — каноническая запись ключа, единственный писатель: Build отвергает другую.
func (k parsedKey) String() string {
	switch k.kind {
	case 'T':
		if k.mod != 0 {
			return string(k.mod) + "(" + k.tree.String() + ")"
		}
		return k.tree.String()
	case 'P':
		return joinPages(pageString(k.mods[0], k.pages[0]), pageString(k.mods[1], k.pages[1]))
	case 'Q':
		q := "Q"
		if k.mod != 0 {
			q += string(k.mod)
		}
		return q + "[" + k.pages[0].String() + "|" + k.pages[1].String() + "]"
	}
	parts := make([]string, len(k.idx))
	for i, n := range k.idx {
		parts[i] = strconv.Itoa(n)
	}
	return "B[" + k.bleed + ":" + strings.Join(parts, ",") + "]"
}

// pageForm — ключ как две страницы: P[…], а также B[right:i] и B[pages:i,j]
// (это страницы «в край»). false — у раскладки нет страниц (Q[…] не листается).
func pageForm(key string) (parsedKey, bool) {
	k, ok := parseKey(key)
	if !ok {
		return k, false
	}
	switch {
	case k.kind == 'P':
		return k, true
	case k.bleed == "right":
		return parsedKey{kind: 'P', pages: [2]*keyTree{nil, {leaf: k.idx[0]}}, mods: [2]byte{0, 'E'}}, true
	case k.bleed == "pages":
		return parsedKey{kind: 'P', pages: [2]*keyTree{{leaf: k.idx[0]}, {leaf: k.idx[1]}}, mods: [2]byte{'E', 'E'}}, true
	}
	return k, false
}

// IsPaged — раскладку можно листать по страницам: P[…] или страницы «в край».
func IsPaged(key string) bool {
	_, ok := pageForm(key)
	return ok
}

func (k parsedKey) leaves() []int {
	switch k.kind {
	case 'T':
		return k.tree.leaves(nil)
	case 'P', 'Q':
		return k.pages[1].leaves(k.pages[0].leaves(nil))
	}
	return k.idx
}

type keyParser struct {
	s string
	i int
}

func (k *keyParser) eat(c byte) bool {
	if k.i < len(k.s) && k.s[k.i] == c {
		k.i++
		return true
	}
	return false
}

func (k *keyParser) peek() byte {
	if k.i < len(k.s) {
		return k.s[k.i]
	}
	return 0
}

// open съедает «c(», если узел начинается здесь.
func (k *keyParser) open(c byte) bool {
	if k.i+1 < len(k.s) && k.s[k.i] == c && k.s[k.i+1] == '(' {
		k.i += 2
		return true
	}
	return false
}

func (k *keyParser) num() (int, bool) {
	start := k.i
	for k.i < len(k.s) && k.s[k.i] >= '0' && k.s[k.i] <= '9' && k.i-start < 4 {
		k.i++
	}
	s := k.s[start:k.i]
	n, err := strconv.Atoi(s)
	// Только каноническая запись: без ведущих нулей, иначе у раскладки два ключа.
	if err != nil || strconv.Itoa(n) != s {
		return 0, false
	}
	return n, true
}

func (k *keyParser) leaf() (*keyTree, bool) {
	n, ok := k.num()
	return &keyTree{leaf: n}, ok
}

// list разбирает детей узла до «)»: не меньше lo, не больше hi (0 — без предела).
func (k *keyParser) list(lo, hi int, kid func() (*keyTree, bool)) ([]*keyTree, bool) {
	var kids []*keyTree
	for {
		t, ok := kid()
		if !ok || hi > 0 && len(kids) >= hi {
			return nil, false
		}
		kids = append(kids, t)
		if k.eat(',') {
			continue
		}
		return kids, k.eat(')') && len(kids) >= lo
	}
}

// node разбирает детей составного узла op (после «op(»).
func (k *keyParser) node(op byte, lo, hi int, kid func() (*keyTree, bool)) (*keyTree, bool) {
	kids, ok := k.list(lo, hi, kid)
	return &keyTree{leaf: -1, op: op, kids: kids}, ok
}

func (k *keyParser) tree() (*keyTree, bool) {
	c := k.peek()
	if c != 'H' && c != 'V' {
		return k.leaf()
	}
	k.i++
	if !k.eat('(') {
		return nil, false
	}
	return k.node(c, 2, 0, k.tree)
}

// zkid — ребёнок Z: уровень a…g и позиция.
func (k *keyParser) zkid() (*keyTree, bool) {
	c := k.peek()
	if c < 'a' || c > 'g' {
		return nil, false
	}
	k.i++
	t, ok := k.leaf()
	t.lvl = int8(c-'a') + 1
	return t, ok
}

// root — корень блока: дерево, ряд с листьями m и n, ряды G, лесенка L/A или разброс Z.
func (k *keyParser) root() (*keyTree, bool) {
	switch c := k.peek(); {
	case k.open('G'):
		return k.node('G', 2, 0, k.row)
	case k.open('Z'):
		return k.node('Z', 1, 5, k.zkid)
	case (c == 'L' || c == 'A') && k.open(c):
		return k.node(c, 3, 5, k.leaf)
	case k.open('H'):
		t, ok := k.node('H', 2, 0, k.hkid)
		// Ряд из одних m и n — тот же ряд, только мельче: у вида было бы два ключа.
		return t, ok && slices.ContainsFunc(t.kids, func(kid *keyTree) bool { return !kid.small })
	}
	return k.tree()
}

// hkid — ребёнок корневого H: дерево или лист m / n.
func (k *keyParser) hkid() (*keyTree, bool) {
	if c := k.peek(); c == 'm' || c == 'n' {
		k.i++
		t, ok := k.leaf()
		t.small, t.big = true, c == 'n'
		return t, ok
	}
	return k.tree()
}

// row — ряд G: одно фото или H из фото.
func (k *keyParser) row() (*keyTree, bool) {
	if k.open('H') {
		return k.node('H', 2, 0, k.leaf)
	}
	return k.leaf()
}

func (k *keyParser) page() (*keyTree, byte, bool) {
	if k.eat('-') {
		return nil, 0, true
	}
	// В край ставится только одно фото.
	if k.open('E') {
		t, ok := k.leaf()
		return t, 'E', ok && k.eat(')')
	}
	if m := k.peek(); strings.IndexByte(pageWrap, m) >= 0 && k.open(m) {
		t, ok := k.root()
		return t, m, ok && k.eat(')')
	}
	t, ok := k.root()
	return t, 0, ok
}

// paged — «стр|стр]» после «P[».
func (k *keyParser) paged(r *parsedKey) bool {
	var ok bool
	r.pages[0], r.mods[0], ok = k.page()
	if !ok || !k.eat('|') {
		return false
	}
	r.pages[1], r.mods[1], ok = k.page()
	return ok && k.eat(']')
}

// sides — «[U|D]» и «[корень|корень]» после «Q».
func (k *keyParser) sides(r *parsedKey) bool {
	if c := k.peek(); c == 'U' || c == 'D' {
		r.mod = c
		k.i++
	}
	ok := k.eat('[')
	for side, sep := range []byte("|]") {
		if !ok {
			return false
		}
		r.pages[side], ok = k.root()
		ok = ok && k.eat(sep)
	}
	return ok
}

// cells — «вид:позиции]» после «B[».
func (k *keyParser) cells(r *parsedKey) bool {
	rest := k.s[2:]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return false
	}
	r.bleed = rest[:colon]
	k.i = 2 + colon + 1
	cells := map[string]int{"full": 1, "right": 1, "pages": 2}[r.bleed]
	if cells == 0 {
		return false
	}
	for c := range cells {
		if c > 0 && !k.eat(',') {
			return false
		}
		n, ok := k.num()
		if !ok {
			return false
		}
		r.idx = append(r.idx, n)
	}
	return k.eat(']')
}

func parseKey(key string) (parsedKey, bool) {
	var r parsedKey
	// До разбора: длина ограничивает глубину рекурсии.
	if len(key) > MaxKeyLen {
		return r, false
	}
	k := &keyParser{s: key}
	var ok bool
	switch {
	case strings.HasPrefix(key, "P["):
		k.i = 2
		r.kind = 'P'
		ok = k.paged(&r)
	case strings.HasPrefix(key, "Q"):
		k.i = 1
		r.kind = 'Q'
		ok = k.sides(&r)
	case strings.HasPrefix(key, "B["):
		r.kind = 'B'
		ok = k.cells(&r)
	case strings.IndexByte(spreadMods, k.peek()) >= 0 && k.open(k.peek()):
		r.kind = 'T'
		r.mod = key[0]
		r.tree, ok = k.root()
		ok = ok && k.eat(')')
	default:
		r.kind = 'T'
		r.tree, ok = k.root()
	}
	return r, ok && k.i == len(key)
}

// leavesOK: каждая позиция 0…n−1 встречается в ключе ровно один раз.
func leavesOK(ls []int, n int) bool {
	if len(ls) != n {
		return false
	}
	seen := make([]bool, n)
	for _, i := range ls {
		if i < 0 || i >= n || seen[i] {
			return false
		}
		seen[i] = true
	}
	return true
}

// mapLeaves заменяет каждую позицию фото в ключе на f(позиция). Цифры в ключе — только
// позиции (параметры узлов — буквы), поэтому замена по цепочкам цифр верна.
// Только для ключей, которые принял разбор: relabel вне idx паникует.
func mapLeaves(key string, f func(int) string) string {
	var b strings.Builder
	for i := 0; i < len(key); {
		if c := key[i]; c < '0' || c > '9' {
			b.WriteByte(c)
			i++
			continue
		}
		j := i
		for j < len(key) && key[j] >= '0' && key[j] <= '9' {
			j++
		}
		n, _ := strconv.Atoi(key[i:j])
		b.WriteString(f(n))
		i = j
	}
	return b.String()
}

// leavesOf — позиции фото в порядке записи в ключе (у P и Q — сначала левая страница).
func leavesOf(key string) []int {
	var out []int
	mapLeaves(key, func(i int) string { out = append(out, i); return "" })
	return out
}

// IsRearranged — раскладка ставит фото не в порядке ленты: листья при обходе
// слева направо идут не по возрастанию.
func IsRearranged(key string) bool {
	return inversions(key) > 0
}

// inversions — насколько раскладка отходит от порядка ленты.
func inversions(key string) int {
	ls := leavesOf(key)
	n := 0
	for i := range ls {
		for j := i + 1; j < len(ls); j++ {
			if ls[i] > ls[j] {
				n++
			}
		}
	}
	return n
}

// shapeKey — ключ, где каждая позиция заменена классом пропорций: раскладки,
// которые лишь меняют местами фото одних пропорций, имеют одну форму.
func shapeKey(key string, aspects []float64) string {
	return mapLeaves(key, func(i int) string {
		if i >= len(aspects) {
			return "?"
		}
		return fmt.Sprintf("%.3f", aspects[i])
	})
}

// relabel переводит локальные позиции блока в позиции во входе: i → idx[i].
func relabel(key string, idx []int) string {
	return mapLeaves(key, func(i int) string { return strconv.Itoa(idx[i]) })
}
