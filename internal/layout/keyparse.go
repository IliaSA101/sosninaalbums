package layout

// Грамматика ключа раскладки: дерево, разбор, запись страниц и преобразования
// позиций (mapLeaves, relabel, shapeKey). Геометрия по ключу — key.go.

import (
	"fmt"
	"strconv"
	"strings"
)

// Ключ однозначно описывает раскладку с точностью до размеров страницы: это и
// идентификатор варианта, и то, что хранится в файле. Листья — позиции фото во
// входе (0…n−1), сквозные по всему развороту:
//
//	дерево := позиция | ("H"|"V") "(" дерево ("," дерево)+ ")"
//	ключ   := дерево                          блок на весь разворот (только layflat)
//	        | "P[" стр "|" стр "]"            постранично
//	        | "B[full:" i "]" | "B[right:" i "]" | "B[pages:" i "," j "]"   в край
//	стр    := "-"                             пустая страница
//	        | дерево                          вписано в поля по центру
//	        | ("S"|"O"|"I") "(" дерево ")"    поменьше по центру / к внешнему краю / к корешку
//	        | "E(" i ")"                      одно фото в край на всю страницу
//
// Ключ не зависит от Params: после смены настроек Build либо строит то же
// дерево в новой геометрии, либо отказывает. У одной раскладки один ключ:
// P[-|E(i)] пишется как B[right:i], P[E(i)|E(j)] — как B[pages:i,j].

type keyTree struct {
	leaf  int // позиция фото; -1 у составного узла
	horiz bool
	kids  []*keyTree
}

func (t *keyTree) String() string {
	if t == nil {
		return "-"
	}
	if t.leaf >= 0 {
		return strconv.Itoa(t.leaf)
	}
	parts := make([]string, len(t.kids))
	for i, k := range t.kids {
		parts[i] = k.String()
	}
	if t.horiz {
		return "H(" + strings.Join(parts, ",") + ")"
	}
	return "V(" + strings.Join(parts, ",") + ")"
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

// node собирает дерево для расчёта; листья ссылаются на позиции во входе.
func (t *keyTree) node(aspects []float64, gap float64) *node {
	if t.leaf >= 0 {
		return &node{leaf: t.leaf, a: 1 / aspects[t.leaf], key: strconv.Itoa(t.leaf)}
	}
	kids := make([]*node, len(t.kids))
	for i, k := range t.kids {
		kids[i] = k.node(aspects, gap)
	}
	return compose(t.horiz, kids, gap)
}

type parsedKey struct {
	kind  byte        // 'T' — дерево на весь разворот, 'P' — постранично, 'B' — в край
	tree  *keyTree    // 'T'
	pages [2]*keyTree // 'P'; nil — пустая страница
	mods  [2]byte     // 'P': модификатор страницы (pageMods), 0 — вписано по центру
	bleed string      // 'B': full | right | pages
	idx   []int       // 'B': позиции фото по ячейкам
}

// pageMods — модификаторы страницы: E — в край, S — поменьше по центру,
// O — прижато к внешнему краю, I — к корешку.
const pageMods = "ESOI"

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

// pageForm — ключ как две страницы: P[…], а также B[right:i] и B[pages:i,j]
// (это страницы «в край»). false — у раскладки нет страниц.
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
	case 'P':
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

func (k *keyParser) tree() (*keyTree, bool) {
	if k.i >= len(k.s) {
		return nil, false
	}
	c := k.s[k.i]
	if c != 'H' && c != 'V' {
		n, ok := k.num()
		return &keyTree{leaf: n}, ok
	}
	k.i++
	if !k.eat('(') {
		return nil, false
	}
	t := &keyTree{leaf: -1, horiz: c == 'H'}
	for {
		kid, ok := k.tree()
		if !ok {
			return nil, false
		}
		t.kids = append(t.kids, kid)
		if k.eat(',') {
			continue
		}
		if k.eat(')') && len(t.kids) >= 2 {
			return t, true
		}
		return nil, false
	}
}

func (k *keyParser) page() (*keyTree, byte, bool) {
	if k.eat('-') {
		return nil, 0, true
	}
	if k.i+1 < len(k.s) && k.s[k.i+1] == '(' && strings.IndexByte(pageMods, k.s[k.i]) >= 0 {
		m := k.s[k.i]
		k.i += 2
		t, ok := k.tree()
		// В край ставится только одно фото.
		if !ok || !k.eat(')') || m == 'E' && t.leaf < 0 {
			return nil, 0, false
		}
		return t, m, true
	}
	t, ok := k.tree()
	return t, 0, ok
}

func parseKey(key string) (parsedKey, bool) {
	k := &keyParser{s: key}
	var r parsedKey
	var ok bool
	switch {
	case strings.HasPrefix(key, "P["):
		k.i = 2
		r.kind = 'P'
		var ok1, ok2 bool
		r.pages[0], r.mods[0], ok1 = k.page()
		ok = ok1 && k.eat('|')
		if ok {
			r.pages[1], r.mods[1], ok2 = k.page()
			ok = ok2 && k.eat(']')
		}
	case strings.HasPrefix(key, "B["):
		r.kind = 'B'
		rest := key[2:]
		colon := strings.IndexByte(rest, ':')
		if colon < 0 {
			return r, false
		}
		r.bleed = rest[:colon]
		k.i = 2 + colon + 1
		cells := map[string]int{"full": 1, "right": 1, "pages": 2}[r.bleed]
		if cells == 0 {
			return r, false
		}
		for c := range cells {
			if c > 0 && !k.eat(',') {
				return r, false
			}
			n, okN := k.num()
			if !okN {
				return r, false
			}
			r.idx = append(r.idx, n)
		}
		ok = k.eat(']')
	default:
		r.kind = 'T'
		r.tree, ok = k.tree()
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

// mapLeaves заменяет каждую позицию фото в ключе на f(позиция).
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

// leavesOf — позиции фото в порядке записи в ключе (для P — сначала левая страница).
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
