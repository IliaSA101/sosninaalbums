package decor

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// Ссылки в CSS и значениях атрибутов SVG для графа повторов (svgrefs.go). Цель — не потерять
// ссылку: что разобрано неуверенно (строка без конца, «{» внутри правила), считается по худшему
// виду. CSS-экранирование до сюда не доходит — checkCSS его отклоняет.

// urlIDs — id из всех «url(#id)» в значении (слово url — без учёта регистра, id — как написан).
// Принимается только каноничная запись (urlArg), иначе errSVGFragment: не угадываем, как её
// прочтёт Blink. rest сдвигается за аргумент — «url(#url(#…» без «)» иначе читался бы за n².
func urlIDs(v string) ([]string, error) {
	var ids []string
	for rest := v; ; {
		i := indexFold(rest, "url(")
		if i < 0 {
			return ids, nil
		}
		rest = strings.TrimLeft(rest[i+len("url("):], cssSpace)
		arg, n, ok := urlArg(rest)
		if !ok {
			return nil, errSVGFragment
		}
		rest = rest[n:]
		if !strings.HasPrefix(arg, "#") {
			continue
		}
		id, err := fragID(arg[1:])
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
}

// cssSpace — пробельные символы CSS.
const cssSpace = " \t\r\n\f"

// urlArg — аргумент url( (пробелы перед ним срезаны) и длина записи до «)» включительно.
// В кавычках — до той же кавычки и без пробелов по краям внутри («url(' #x')» Blink локальной
// не считает); без кавычек — до «)» или пробела, как url-токен CSS; дальше — пробелы и «)».
// ok=false — запись не каноничная: нет «)» или закрывающей кавычки, кавычка или «(» в аргументе.
func urlArg(s string) (arg string, n int, ok bool) {
	if s != "" && (s[0] == '"' || s[0] == '\'') {
		end := strings.IndexByte(s[1:], s[0])
		if end < 0 {
			return "", 0, false
		}
		arg, n = s[1:1+end], end+2
		if strings.Trim(arg, cssSpace) != arg {
			return "", 0, false
		}
	} else {
		end := strings.IndexAny(s, ")"+cssSpace+`"'(`)
		if end < 0 {
			return "", 0, false
		}
		arg, n = s[:end], end
	}
	tail := strings.TrimLeft(s[n:], cssSpace)
	if tail == "" || tail[0] != ')' {
		return "", 0, false
	}
	return arg, len(s) - len(tail) + 1, true
}

// maxSVGIDLen — самый длинный id в ссылке: у редакторов десятки байт.
const maxSVGIDLen = 256

// fragID — id из фрагмента ссылки. Chromium раскрывает в нём %xx и выбрасывает TAB, LF, CR —
// «#%6C39» и «#l&#9;39» ведут на l39. В файлах редакторов такого нет, поэтому не своя копия
// нормализации, а отказ; слишком длинный id — тоже.
func fragID(s string) (string, error) {
	if len(s) > maxSVGIDLen {
		return "", errSVGFragment
	}
	for _, r := range s {
		if r == '%' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", errSVGFragment
		}
	}
	return s, nil
}

// isC0 — то, что URL-парсер срезает по краям ссылки: управляющие символы и пробел.
func isC0(r rune) bool { return r <= ' ' }

// indexFold — позиция ASCII-подстроки без учёта регистра (ToLower меняет длину у части Юникода).
func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// attrCat — вид ссылки по имени свойства: clip-path, mask, filter рисуются при самом элементе,
// fill и stroke наследуются фигурами; маркеры и всё незнакомое (--переменная, mask-image…) —
// худший вид: наследуется и рисуется на каждой вершине.
func attrCat(name string) refCat {
	switch name {
	case "clip-path", "mask", "filter":
		return refOnce
	case "fill", "stroke":
		return refPaint
	}
	return refMarker
}

// declRefs — ссылки из объявлений «свойство: значение; …» с видом по свойству; что не
// разбирается (строка без конца, скобка) — до конца текста с худшим видом.
func declRefs(body string) ([]svgRef, error) {
	var out []svgRef
	for i := 0; i < len(body); {
		j, ok := cssSkip(body, i)
		cat := refMarker
		if !ok || (j < len(body) && body[j] != ';') {
			j = len(body)
		} else if c := strings.IndexByte(body[i:j], ':'); c >= 0 {
			cat = attrCat(strings.ToLower(strings.TrimSpace(stripCSSComments(body[i : i+c]))))
		}
		ids, err := urlIDs(body[i:j])
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			out = append(out, svgRef{id: id, cat: cat, k: 1})
		}
		i = j + 1
	}
	return out, nil
}

// cssSkip — от позиции i до ближайшего «{», «}» или «;» вне строк, комментариев и url(…) без
// кавычек (в нём CSS читает всё до «)»). ok=false — строка оборвана переводом строки или концом.
func cssSkip(s string, i int) (int, bool) {
	for i < len(s) {
		switch c := s[i]; {
		case c == '{' || c == '}' || c == ';':
			return i, true
		case c == '"' || c == '\'':
			j := strings.IndexAny(s[i+1:], string(c)+"\n\r\f")
			if j < 0 || s[i+1+j] != c {
				return i, false
			}
			i += j + 2
		case c == '/' && strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return len(s), true
			}
			i += j + 4
		case urlToken(s, i):
			j := strings.IndexByte(s[i:], ')')
			if j < 0 {
				return i, false
			}
			i += j + 1
		default:
			i++
		}
	}
	return i, true
}

// urlToken — с позиции i начинается «url(» без кавычки после него (отдельное слово, не xurl().
func urlToken(s string, i int) bool {
	if i+4 > len(s) || !strings.EqualFold(s[i:i+4], "url(") || i > 0 && nameByte(s[i-1]) {
		return false
	}
	arg := strings.TrimLeft(s[i+4:], " \t\r\n\f")
	return arg == "" || arg[0] != '"' && arg[0] != '\''
}

// nameByte — байт может быть частью имени CSS (тогда «url(» — хвост другого слова).
func nameByte(c byte) bool {
	return c == '-' || c == '_' || c >= 0x80 || c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z'
}

// cssRule — правило <style>: селектор (или at-правило) и тело без внешних скобок.
type cssRule struct{ sel, body string }

// cssRules — правила верхнего уровня; ok=false — вложенные скобки (@media), лишняя «}» или
// оборванная строка: тогда кого задевает ссылка, неизвестно.
func cssRules(s string) (rules []cssRule, ok bool) {
	start := 0
	for i := 0; ; {
		j, ok := cssSkip(s, i)
		switch {
		case !ok || j < len(s) && s[j] == '}':
			return nil, false
		case j >= len(s):
			return rules, true
		case s[j] == ';': // at-правило без блока («@charset "UTF-8";») — следующее правило своё;
			start, i = j+1, j+1 // «x;.a{…}» Blink отбрасывает, а у нас оно — «.a»: лишний счёт, не пропуск
			continue
		}
		k, ok := cssSkip(s, j+1)
		for ok && k < len(s) && s[k] == ';' {
			k, ok = cssSkip(s, k+1)
		}
		if !ok || k >= len(s) || s[k] != '}' {
			return nil, false
		}
		rules = append(rules, cssRule{sel: s[start:j], body: s[j+1 : k]})
		start, i = k+1, k+1
	}
}

// css — текст <style>. Ссылки правил «.класс{…}» привязываются к классам; @font-face
// пропускается (CorelDRAW пишет src:url("#FontID0"), а SVG-шрифты Chromium не рисует);
// остальные — сложные. Ссылки ищутся и по всему тексту: не попавшие в разбор правил
// (комментарий, непонятная разметка) тоже сложные.
func (g *svgRefs) css(text string) error {
	g.flags(text)
	all, err := urlIDs(text)
	if err != nil || len(all) == 0 {
		return err
	}
	left := map[string]int{}
	for _, id := range all {
		left[id]++
	}
	rules, _ := cssRules(text) // не разобралось — все ссылки останутся сложными
	for _, r := range rules {
		if err := g.cssRule(r, left); err != nil {
			return err
		}
	}
	for id, k := range left {
		if k > 0 {
			g.cssComplex = append(g.cssComplex, svgRef{id: id, cat: refMarker, k: int64(k)})
		}
	}
	return nil
}

// cssRule — одно правило: ссылки к классам, в сложные или (у @font-face) никуда; left — сколько
// ссылок текста ещё не разобрано.
func (g *svgRefs) cssRule(r cssRule, left map[string]int) error {
	refs, err := declRefs(r.body)
	if err != nil || len(refs) == 0 {
		return err
	}
	for _, x := range refs {
		left[x.id]--
	}
	if strings.ContainsAny(r.sel, `"'`) { // строка в селекторе — не «.класс»; комментарий в ней не комментарий
		g.cssComplex = append(g.cssComplex, refs...)
		return nil
	}
	sel := strings.TrimSpace(stripCSSComments(r.sel))
	if strings.EqualFold(sel, "@font-face") {
		return nil
	}
	classes, ok := simpleClasses(sel)
	if !ok {
		g.cssComplex = append(g.cssComplex, refs...)
		return nil
	}
	sum := map[refKey]int64{} // «fill:url(#x)» n раз — одна ссылка с кратностью n, а не n строк
	for _, x := range refs {
		sum[refKey{x.id, x.cat}] += x.k
	}
	rule := make([]svgRef, 0, len(sum))
	byKey := func(a, b refKey) int { return cmp.Or(strings.Compare(a.id, b.id), cmp.Compare(a.cat, b.cat)) }
	for _, key := range slices.SortedFunc(maps.Keys(sum), byKey) { // одинаковый файл — одинаковый обход
		rule = append(rule, svgRef{id: key.id, cat: key.cat, k: sum[key]})
	}
	ri := len(g.rules)
	g.rules = append(g.rules, rule)
	if g.classRules == nil {
		g.classRules = map[string][]int{}
	}
	for _, cl := range classes {
		g.classRules[cl] = append(g.classRules[cl], ri)
	}
	return nil
}

// refKey — ссылка правила без кратности.
type refKey struct {
	id  string
	cat refCat
}

// stripCSSComments убирает /* … */ (Illustrator кладёт их в <style>).
func stripCSSComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteByte(' ')
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return b.String()
		}
		s = s[i+2+j+2:]
	}
}

// simpleClasses — селектор из одних «.класс» через запятую; иначе ok=false.
func simpleClasses(sel string) (cls []string, ok bool) {
	for _, p := range strings.Split(sel, ",") {
		p = strings.TrimSpace(p)
		if len(p) < 2 || p[0] != '.' {
			return nil, false
		}
		for _, r := range p[1:] {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
				return nil, false
			}
		}
		cls = append(cls, p[1:])
	}
	return cls, true
}

// countVerts — сверху число вершин в d или points (для маркеров): каждая буква команды, точка
// и начало числа, плюс два конца.
func countVerts(v string) int64 {
	n := int64(2)
	for i := 0; i < len(v); i++ {
		c := v[i]
		digit := c >= '0' && c <= '9'
		if c == '.' || c|0x20 >= 'a' && c|0x20 <= 'z' || digit && (i == 0 || v[i-1] < '0' || v[i-1] > '9') {
			n++
		}
	}
	return n
}
