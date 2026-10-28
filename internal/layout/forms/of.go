package forms

// Of — вид раскладки по ключу и геометрии: размечает и новые ключи, и старые (ряд Layouts,
// featured). Разбор строки — поверхностный (скобки и модификаторы), без layout.parseKey: ключ
// здесь уже принят Build, а неприёмный не должен ронять — вернётся мозаика.

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Form — вид раскладки: по нему ряд «Авто» и выбор сборки чередуют картинки.
type Form string

// Виды (design §5). Mosaic — раскладка ряда Layouts как есть; «через корешок», «с главным» и
// «две крупных» строит variety/featured.go, остальные — Gen.
const (
	Mosaic   Form = "mosaic"
	Strip    Form = "strip"
	Rows     Form = "rows"
	Grid     Form = "grid"
	Air      Form = "air"
	Asym     Form = "asym"
	Spine    Form = "spine"
	Hero     Form = "hero"
	Pair     Form = "pair"
	Contrast Form = "contrast"
	Ladder   Form = "ladder"
	Frame    Form = "frame"
	Bleed    Form = "bleed"
	Scatter  Form = "scatter" // «Разброс» (105): корень Z, только «Другие варианты»
)

var names = map[Form]string{
	Mosaic: "Плотно", Strip: "Лента", Rows: "Ряды", Grid: "Сетка", Air: "Воздух",
	Asym: "Разные страницы", Spine: "Через корешок", Hero: "С главным фото", Pair: "Две крупных",
	Contrast: "Мелкое — крупное", Ladder: "Лесенка", Frame: "Рамка", Bleed: "В край", Scatter: "Разброс",
}

// Name — подпись вида по-русски; неизвестный вид — пусто.
func Name(f Form) string { return names[f] }

const (
	// airFill — разворот с заполнением ниже считается воздушным (как в variety).
	airFill = 0.55
	// stripShare — одиночный ряд H(…) на разворот — лента, если занимает не больше этой доли
	// площади разворота (иначе это обычная мозаика из одного ряда).
	stripShare = 0.5
	// sparseMax, denseMin — «разные страницы»: редкая страница ≤ 4 фото, плотная — от 4 и больше редкой.
	sparseMax = 4
	denseMin  = 4
)

// side — блок ключа: модификатор (S O I U D, 0 — нет) и корень без обёртки; "" — пустая страница.
type side struct {
	mod  byte
	root string
}

// splitTop делит s по sep на верхнем уровне скобок.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, from := 0, 0
	for i := range len(s) {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[from:i])
				from = i + 1
			}
		}
	}
	return append(out, s[from:])
}

// unwrap снимает «M(…)» с блока: модификатор из mods и корень внутри.
func unwrap(s, mods string) side {
	if len(s) > 2 && strings.IndexByte(mods, s[0]) >= 0 && s[1] == '(' && s[len(s)-1] == ')' {
		return side{s[0], s[2 : len(s)-1]}
	}
	return side{0, s}
}

// shapeOf — разбор ключа на блоки: 'T' — на весь разворот (один блок), 'P' / 'Q' — две страницы.
func shapeOf(key string) (kind byte, sides []side) {
	inner := func(from int) []side {
		if len(key) < from+2 || key[len(key)-1] != ']' {
			return nil
		}
		var out []side
		for _, s := range splitTop(key[from:len(key)-1], '|') {
			if s == "-" {
				s = ""
			}
			out = append(out, unwrap(s, "SOIUD"))
		}
		return out
	}
	switch {
	case strings.HasPrefix(key, "P["):
		return 'P', inner(2)
	case strings.HasPrefix(key, "Q"):
		return 'Q', inner(strings.IndexByte(key, '[') + 1)
	}
	return 'T', []side{unwrap(key, "SUD")}
}

// leavesIn — сколько фото в блоке (цепочки цифр).
func leavesIn(s string) int {
	n := 0
	for i := range len(s) {
		if s[i] >= '0' && s[i] <= '9' && (i == 0 || s[i-1] < '0' || s[i-1] > '9') {
			n++
		}
	}
	return n
}

func isLeaf(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// kids — дети корня «X(…)»; nil, если корень не такой.
func kids(root string, op byte) []string {
	if len(root) < 3 || root[0] != op || root[1] != '(' || root[len(root)-1] != ')' {
		return nil
	}
	return splitTop(root[2:len(root)-1], ',')
}

// isRow — блок: фото или ряд H из одних фото.
func isRow(root string) bool {
	if isLeaf(root) {
		return true
	}
	ks := kids(root, 'H')
	for _, k := range ks {
		if !isLeaf(k) {
			return false
		}
	}
	return len(ks) > 0
}

// rowSizes — сколько фото в рядах G; false — корень не G.
func rowSizes(root string) ([]int, bool) {
	ks := kids(root, 'G')
	if ks == nil {
		return nil, false
	}
	out := make([]int, len(ks))
	for i, k := range ks {
		out[i] = leavesIn(k)
	}
	return out, true
}

// isGridSizes — ровная сетка: все ряды как первый, последний — не длиннее.
func isGridSizes(sz []int) bool {
	if len(sz) < 2 || sz[0] < 2 {
		return false
	}
	for _, n := range sz[:len(sz)-1] {
		if n != sz[0] {
			return false
		}
	}
	return sz[len(sz)-1] <= sz[0]
}

// gridForm — раскладка из одних G: ровная сетка или ряды.
func gridForm(sides []side) (Form, bool) {
	f, seen := Grid, false
	for _, s := range sides {
		if s.root == "" {
			continue
		}
		sz, ok := rowSizes(s.root)
		if !ok {
			return "", false
		}
		seen = true
		if !isGridSizes(sz) {
			f = Rows
		}
	}
	return f, seen
}

// isFrame — H(V(…),фото,V(…)): крупное в рамке из столбцов.
func isFrame(sides []side) bool {
	if len(sides) != 1 {
		return false
	}
	ks := kids(sides[0].root, 'H')
	return len(ks) == 3 && strings.HasPrefix(ks[0], "V(") && isLeaf(ks[1]) && strings.HasPrefix(ks[2], "V(")
}

// isAsym — «разные страницы»: одна страница редкая (S, U, D; ≤ 4 фото), другая плотная.
func isAsym(kind byte, sides []side) bool {
	if kind != 'P' || len(sides) != 2 {
		return false
	}
	for i := range 2 {
		sp, de := sides[i], sides[1-i]
		if sp.root == "" || de.root == "" || strings.IndexByte("SUD", sp.mod) < 0 || de.mod != 0 {
			continue
		}
		n, m := leavesIn(sp.root), leavesIn(de.root)
		if n <= sparseMax && m >= denseMin && m > n {
			return true
		}
	}
	return false
}

// bboxShare — доля площади разворота внутри полей, которую занимает рамка вокруг всех фото.
func bboxShare(rects []layout.Rect, p layout.Params) float64 {
	area := (2*p.PageW - 2*p.Margin) * (p.PageH - 2*p.Margin)
	if len(rects) == 0 || area <= 0 {
		return 1
	}
	x0, y0, x1, y1 := bbox(rects)
	return (x1 - x0) * (y1 - y0) / area
}

func bbox(rects []layout.Rect) (x0, y0, x1, y1 float64) {
	x0, y0 = rects[0].X, rects[0].Y
	x1, y1 = x0, y0
	for _, r := range rects {
		x0, y0 = min(x0, r.X), min(y0, r.Y)
		x1, y1 = max(x1, r.X+r.W), max(y1, r.Y+r.H)
	}
	return x0, y0, x1, y1
}

// isStrip — лента: на разворот — ряд H(…) вверху/внизу (U, D) или с воздухом по высоте; на
// страницах Q — по ряду на каждой.
func isStrip(kind byte, sides []side, rects []layout.Rect, p layout.Params) bool {
	switch kind {
	case 'T':
		s := sides[0]
		if s.mod == 'S' || kids(s.root, 'H') == nil || !isRow(s.root) {
			return false
		}
		return s.mod != 0 || len(rects) > 0 && bboxShare(rects, p) <= stripShare
	case 'Q':
		return len(sides) == 2 && isRow(sides[0].root) && isRow(sides[1].root)
	}
	return false
}

// fillOf — сумма площадей фото / площадь разворота.
func fillOf(rects []layout.Rect, p layout.Params) float64 {
	all := 2 * p.PageW * p.PageH
	if all <= 0 {
		return 1
	}
	sum := 0.0
	for _, r := range rects {
		sum += r.W * r.H
	}
	return sum / all
}

func hasMod(sides []side) bool {
	for _, s := range sides {
		if s.mod != 0 && s.mod != 'O' && s.mod != 'I' {
			return true
		}
	}
	return false
}

// Of — вид раскладки: порядок проверок — в край, разброс, лесенка, мелкое — крупное, лента, разные
// страницы, ряды и сетки, рамка, по главному фото (через корешок, главное, пара), воздух, мозаика
// (рамка раньше главного: один и тот же ключ H(V(…),n,V(…)) — рамка, а не «через корешок»).
// Ключи разворота без главных и особого вида — мозаика, как и весь ряд Layouts.
func Of(key string, rects []layout.Rect, p layout.Params) Form {
	switch {
	case strings.Contains(key, "B[") || strings.Contains(key, "E("):
		return Bleed
	case strings.Contains(key, "Z("):
		return Scatter
	case strings.Contains(key, "L(") || strings.Contains(key, "A("):
		return Ladder
	case strings.IndexByte(key, 'm') >= 0 || strings.IndexByte(key, 'n') >= 0:
		return Contrast
	}
	kind, sides := shapeOf(key)
	if len(sides) == 0 {
		return Mosaic
	}
	switch {
	case isStrip(kind, sides, rects, p):
		return Strip
	case isAsym(kind, sides):
		return Asym
	}
	if f, ok := gridForm(sides); ok {
		return f
	}
	if isFrame(sides) {
		return Frame
	}
	if f, ok := heroForm(rects, p); ok {
		return f
	}
	switch {
	case hasMod(sides) || len(rects) > 0 && fillOf(rects, p) < airFill:
		return Air
	}
	return Mosaic
}

// heroForm — вид по главному фото: через корешок, главное или пара крупных.
func heroForm(rects []layout.Rect, p layout.Params) (Form, bool) {
	h := Heroes(rects)
	switch h.N {
	case 1:
		if r := rects[h.Idx[0]]; p.Layflat && !p.Sheet && r.X < p.PageW-foldEps && r.X+r.W > p.PageW+foldEps {
			return Spine, true
		}
		return Hero, true
	case 2:
		return Pair, true
	}
	return "", false
}
