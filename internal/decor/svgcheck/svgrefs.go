package svgcheck

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"
)

// Граф ссылок SVG (аудит 0.12): maxSVGUse считает <use>, но не экземпляры — 40 уровней
// «<g id=lN><use href=#lN-1/><use href=#lN-1/></g>» дают 80 <use> и 2^40 экземпляров. Поэтому
// после обхода считается вес каждого элемента: 1 + дети + вес цели каждой ссылки (#id в href,
// url(#id) в атрибутах, style и классах <style>). Повторы сверх самих элементов — не больше
// maxSVGRepeats; ссылка по кругу — отказ. Градиенты в ссылки не входят: их красят один раз.
// fill, stroke и маркеры наследуются: ссылку предка (<g>, <pattern>) рисует каждая фигура-потомок
// и каждая фигура, показанная через <use>; маркер — на каждой вершине (svgcss.go: countVerts).

const (
	maxSVGRepeats = 200000  // экземпляров сверх самих элементов: кружево 60 путей × 300 фигур проходит
	maxSVGEdges   = 1000000 // ссылок после раскрытия классов: дальше дорого считать, а повторов всё равно больше
	svgWeightCap  = 1 << 40 // потолок веса: экспонента не переполняет int64
)

var (
	errSVGRepeats  = errors.New("в SVG слишком много повторов")
	errSVGCycle    = errors.New("в SVG ссылки идут по кругу")
	errSVGCSSRefs  = errors.New("SVG использует стили, которые программа не может проверить")
	errSVGFragment = errors.New("в SVG ссылка на элемент записана необычно (пробел, знак %, нет скобки)")
	errSVGAttrName = errors.New("в SVG атрибут id или class записан необычно (регистр, xml:id)")
)

// refCat — как рисуется цель ссылки.
type refCat uint8

const (
	refOnce   refCat = iota // clip-path, mask, filter, href: один раз при самом элементе
	refPaint                // fill, stroke: наследуются, рисуются каждой фигурой
	refMarker               // маркеры и незнакомое: наследуются, рисуются на каждой вершине
)

// svgRef — ссылка на id с видом и кратностью.
type svgRef struct {
	id  string
	cat refCat
	k   int64
}

// refNode — элемент для подсчёта.
type refNode struct {
	id     string
	hrefs  []string // цели href: у <use> через них наследуются заливки
	grad   bool
	hidden bool // внутри defs, pattern…: без ссылки не рисуется (для счёта shown)
	draw   bool // фигура или текст: на нём рисуются fill, stroke и маркеры
	use    bool
	parent int
	verts  int64 // вершин для маркеров, не меньше 1

	classes []string
	refs    []svgRef
	kids    []int // дети, которые рисуются вместе с элементом
}

// hiddenElems — элементы, которые сами ничего не рисуют: их содержимое появляется только через ссылку.
var hiddenElems = map[string]bool{
	"defs": true, "symbol": true, "clippath": true, "mask": true, "pattern": true, "marker": true,
	"filter": true, "lineargradient": true, "radialgradient": true, "style": true, "metadata": true,
	"title": true, "desc": true,
}

// drawElems — фигуры и текст: к ним применяются fill, stroke и маркеры.
var drawElems = map[string]bool{
	"path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true,
	"polygon": true, "text": true, "tspan": true, "textpath": true,
}

// svgRefs — узлы в порядке документа и открытые сейчас (стек индексов).
type svgRefs struct {
	shown      int // узлов, которые рисуются без ссылок: с ними вес без повторов совпадает
	nodes      []refNode
	open       []int
	rules      [][]svgRef       // ссылки правил «.a, .b {…}», кратность сложена
	classRules map[string][]int // класс → номера правил
	cssComplex []svgRef         // ссылки остальных правил: кого заденут, неизвестно
	inherit    bool             // где-то «inherit»: clip-path, mask, filter тоже наследуются
	context    bool             // где-то «context-fill/stroke»: маркер красится заливкой фигуры
}

// push — новый элемент: ссылки из атрибутов, ребро от родителя.
func (g *svgRefs) push(name string, t xml.StartElement) error {
	n := refNode{
		grad:   name == "lineargradient" || name == "radialgradient",
		draw:   drawElems[name],
		use:    name == "use",
		parent: -1,
		verts:  1,
	}
	if name == "line" {
		n.verts = 2
	}
	for _, a := range t.Attr {
		if err := g.attr(&n, name, a); err != nil {
			return err
		}
	}
	idx := len(g.nodes)
	own := hiddenElems[name]
	n.hidden = own
	if len(g.open) > 0 {
		n.parent = g.open[len(g.open)-1]
		p := &g.nodes[n.parent]
		if !own { // сам defs или pattern родителю не вес: его содержимое — только по ссылке
			p.kids = append(p.kids, idx)
		}
		n.hidden = own || p.hidden
	}
	if !n.hidden {
		g.shown++
	}
	g.nodes = append(g.nodes, n)
	g.open = append(g.open, idx)
	return nil
}

// attr — один атрибут: id, классы, вершины, href и url(#…) с видом по имени.
func (g *svgRefs) attr(n *refNode, name string, a xml.Attr) error {
	an := strings.ToLower(a.Name.Local)
	if oddIDAttr(a, an) {
		return errSVGAttrName
	}
	switch {
	case an == "id" && a.Name.Space == "":
		n.id = a.Value
	case an == "class" && a.Name.Space == "":
		n.classes = strings.Fields(a.Value)
	case an == "d" && name == "path", an == "points" && (name == "polyline" || name == "polygon"):
		n.verts = max(n.verts, countVerts(a.Value))
	case an == "href" && name != "a": // <a> — переход по ссылке, ничего не рисует
		if v := strings.TrimFunc(a.Value, isC0); strings.HasPrefix(v, "#") {
			id, err := fragID(v[1:])
			if err != nil {
				return err
			}
			n.hrefs = append(n.hrefs, id)
			n.refs = append(n.refs, svgRef{id: id, cat: refOnce, k: 1})
		}
	}
	if a.Name.Space != "" { // href и xlink:href разобраны выше; в чужих атрибутах нет ни CSS, ни url(#)
		return nil
	}
	g.flags(a.Value)
	if an == "style" {
		refs, err := declRefs(a.Value)
		n.refs = append(n.refs, refs...)
		return err
	}
	ids, err := urlIDs(a.Value)
	for _, id := range ids {
		n.refs = append(n.refs, svgRef{id: id, cat: attrCat(an), k: 1})
	}
	return err
}

// xmlURL — пространство префикса xml: (xml:id, xml:base).
const xmlURL = "http://www.w3.org/XML/1998/namespace"

// oddIDAttr — id или class не в каноничном виде: «ID», «Class», xml:id. Blink в SVG-документе
// (XML) их id и классом не считает; угадывать, на кого он поведёт ссылку, не будем — отказ
// (аудит 8391ede: приманка «ID» перед настоящим id прятала бомбу use). Чужие пространства
// (rdf:ID) — не id ни для кого.
func oddIDAttr(a xml.Attr, an string) bool {
	if an != "id" && an != "class" {
		return false
	}
	switch a.Name.Space {
	case "":
		return a.Name.Local != an
	case xmlURL, "xml":
		return true
	}
	return false
}

// flags — «inherit» и «context-…» где угодно в файле: считать по худшему случаю.
func (g *svgRefs) flags(v string) {
	if !g.inherit && strings.ContainsAny(v, "iI") {
		g.inherit = indexFold(v, "inherit") >= 0
	}
	if !g.context && strings.ContainsAny(v, "xX") {
		g.context = indexFold(v, "context-") >= 0
	}
}

// pop — закрытие элемента.
func (g *svgRefs) pop() { g.open = g.open[:len(g.open)-1] }

// check — после обхода: разрешает ссылки, ищет круги и считает повторы.
func (g *svgRefs) check() error {
	if len(g.nodes) == 0 {
		return nil
	}
	r, err := g.graph()
	if err != nil {
		return err
	}
	if err := r.weight(0); err != nil {
		return err
	}
	if r.w[0]-int64(g.shown) > maxSVGRepeats {
		return errSVGRepeats
	}
	return r.complex(r.w[0])
}

// refEdge — разрешённая ссылка: номер цели, вид, кратность.
type refEdge struct {
	to  int
	cat refCat
	k   int64
}

// refGraph — разрешённые ссылки и память обхода.
type refGraph struct {
	g       *svgRefs
	ids     map[string]int
	once    [][]refEdge // рисуются при самом элементе
	inh     [][]refEdge // свои наследуемые: рисуют фигуры-потомки
	head    []int       // ближайший предок-или-сам со своими наследуемыми, -1 — нет
	up      []int       // у такого узла — head родителя
	state   []uint8     // 0 — не считан, 1 — в обходе, 2 — готов
	w, d, v []int64     // вес; фигур и вершин, которые элемент показывает
	cp, cm  []int64     // заливки и маркеры по цепочке head
	chained []bool
	rules   [][]refEdge // правила классов без ссылок в никуда и на градиенты
	edges   int         // все ссылки, и в никуда тоже: иначе их перебор бесплатен
	maxV    int64       // больше всех вершин у одной фигуры
}

// graph — ссылки по id (при совпадении id — первый, как в браузере) и цепочки наследования.
func (g *svgRefs) graph() (*refGraph, error) {
	n := len(g.nodes)
	r := &refGraph{
		g: g, ids: make(map[string]int, n), once: make([][]refEdge, n), inh: make([][]refEdge, n),
		head: make([]int, n), up: make([]int, n), state: make([]uint8, n),
		w: make([]int64, n), d: make([]int64, n), v: make([]int64, n),
		cp: make([]int64, n), cm: make([]int64, n), chained: make([]bool, n), maxV: 1,
	}
	for i, nd := range g.nodes {
		if _, dup := r.ids[nd.id]; nd.id != "" && !dup {
			r.ids[nd.id] = i
		}
	}
	r.rules = make([][]refEdge, len(g.rules))
	for ri, rule := range g.rules {
		for _, x := range rule {
			if t, ok := r.target(x.id); ok {
				r.rules[ri] = append(r.rules[ri], refEdge{to: t, cat: x.cat, k: x.k})
			}
		}
	}
	stamp := make([]int, len(g.rules)) // правило уже взято для узла stamp-1
	for i := range g.nodes {
		if err := r.linkNode(i, stamp); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// linkNode — ссылки узла i из атрибутов и правил его классов (правило — один раз, даже если
// подошли несколько его классов), затем место в цепочке наследования. В счёт рёбер идёт каждая
// ссылка правила, и в никуда тоже (аудит 8391ede: 80 000 таких на 19 000 узлов — 8,6 с).
func (r *refGraph) linkNode(i int, stamp []int) error {
	nd := &r.g.nodes[i]
	for _, x := range nd.refs {
		r.edges++
		if t, ok := r.target(x.id); ok {
			r.link(i, refEdge{to: t, cat: x.cat, k: x.k})
		}
	}
	for _, c := range uniqStrings(nd.classes) {
		for _, ri := range r.g.classRules[c] {
			r.edges++
			if stamp[ri] == i+1 {
				continue
			}
			stamp[ri] = i + 1
			r.edges += len(r.g.rules[ri])
			for _, e := range r.rules[ri] {
				r.link(i, e)
			}
		}
		if r.edges > maxSVGEdges {
			return errSVGRepeats
		}
	}
	if r.edges > maxSVGEdges {
		return errSVGRepeats
	}
	ph := -1
	if nd.parent >= 0 {
		ph = r.head[nd.parent]
	}
	r.head[i] = ph
	if len(r.inh[i]) > 0 {
		r.head[i], r.up[i] = i, ph
	}
	if nd.draw {
		r.maxV = max(r.maxV, nd.verts)
	}
	return nil
}

// link — ссылка узла i. Наследуемая у не-фигуры считается и при самом элементе: через
// --переменную или filter:var() она могла достаться и ему.
func (r *refGraph) link(i int, x refEdge) {
	draw := r.g.nodes[i].draw
	if x.cat == refOnce || !draw {
		r.once[i] = append(r.once[i], refEdge{to: x.to, cat: refOnce, k: x.k})
	}
	if x.cat == refOnce && !r.g.inherit {
		return
	}
	r.inh[i] = append(r.inh[i], refEdge{to: x.to, cat: max(x.cat, refPaint), k: x.k})
}

// target — номер элемента с id; градиент не цель: его красят один раз.
func (r *refGraph) target(id string) (int, bool) {
	t, ok := r.ids[id]
	return t, ok && !r.g.nodes[t].grad
}

// weight — экземпляры элемента i вместе с вложенными и подставленными по ссылкам; заодно
// фигуры и вершины, которые он показывает (для наследования через <use>).
func (r *refGraph) weight(i int) error {
	switch r.state[i] {
	case 1:
		return errSVGCycle
	case 2:
		return nil
	}
	r.state[i] = 1
	nd := &r.g.nodes[i]
	w, d, v := int64(1), int64(0), int64(0)
	if nd.draw {
		d, v = 1, nd.verts
	}
	for _, c := range nd.kids {
		if err := r.weight(c); err != nil {
			return err
		}
		w, d, v = satAdd(w, r.w[c]), satAdd(d, r.d[c]), satAdd(v, r.v[c])
	}
	for _, e := range r.once[i] {
		if err := r.weight(e.to); err != nil {
			return err
		}
		w = satAdd(w, satMul(e.k, r.w[e.to]))
	}
	if nd.use {
		ud, uv := r.useShows(i)
		d, v = satAdd(d, ud), satAdd(v, uv)
	}
	cost, err := r.inherited(i)
	if err != nil {
		return err
	}
	r.state[i] = 2
	r.w[i], r.d[i], r.v[i] = satAdd(w, cost), d, v
	return nil
}

// useShows — фигуры и вершины, которые показывает <use> (цели уже посчитаны в weight).
func (r *refGraph) useShows(i int) (d, v int64) {
	for _, h := range r.g.nodes[i].hrefs {
		if t, ok := r.target(h); ok && r.state[t] == 2 {
			d, v = satAdd(d, r.d[t]), satAdd(v, r.v[t])
		}
	}
	return d, v
}

// inherited — цена наследуемых ссылок там, где их рисуют: фигура — заливку один раз, маркеры
// на каждой вершине; <use> — так же за каждую показанную фигуру.
func (r *refGraph) inherited(i int) (int64, error) {
	nd := &r.g.nodes[i]
	var d, v int64
	switch {
	case nd.draw:
		d, v = 1, nd.verts
	case nd.use:
		d, v = r.useShows(i)
	default:
		return 0, nil
	}
	p, m, err := r.chain(r.head[i])
	if err != nil {
		return 0, err
	}
	cost := satAdd(satMul(d, p), satMul(v, m))
	if r.g.context { // содержимое маркера может залить себя заливкой фигуры: на каждой вершине ещё p
		cost = satAdd(cost, satMul(satMul(v, m), p))
	}
	return cost, nil
}

// chain — сумма весов наследуемых заливок и маркеров узла h и его предков.
func (r *refGraph) chain(h int) (p, m int64, err error) {
	if h < 0 {
		return 0, 0, nil
	}
	if r.chained[h] {
		return r.cp[h], r.cm[h], nil
	}
	if p, m, err = r.chain(r.up[h]); err != nil {
		return 0, 0, err
	}
	for _, e := range r.inh[h] {
		if err := r.weight(e.to); err != nil { // цель внутри того, кто её наследует, — круг
			return 0, 0, err
		}
		if x := satMul(e.k, r.w[e.to]); e.cat == refPaint {
			p = satAdd(p, x)
		} else {
			m = satAdd(m, x)
		}
	}
	r.cp[h], r.cm[h], r.chained[h] = p, m, true
	return p, m, nil
}

// complex — ссылки сложных правил: кого заденут, неизвестно, поэтому каждый экземпляр документа
// считается задетым всеми (маркеры — на самой длинной фигуре). Цель с содержимым — одна: две
// могли бы ссылаться друг на друга экспонентой; её содержимое тоже задето — ×(1+mult).
func (r *refGraph) complex(total int64) error {
	var kp, km, wt int64
	heavy := -1
	for _, x := range r.g.cssComplex {
		t, ok := r.target(x.id)
		if !ok {
			continue
		}
		if r.weight(t) != nil {
			return errSVGCSSRefs
		}
		if x.cat == refMarker {
			km = satAdd(km, x.k)
		} else {
			kp = satAdd(kp, x.k)
		}
		if r.w[t] > 1 {
			if heavy >= 0 && heavy != t {
				return errSVGCSSRefs
			}
			heavy = t
		}
		wt = max(wt, r.w[t])
	}
	if kp+km == 0 {
		return nil
	}
	if r.g.context {
		return errSVGCSSRefs
	}
	mult := satAdd(kp, satMul(km, r.maxV))
	extra := satMul(satMul(total, mult), satMul(wt, satAdd(1, mult)))
	if satAdd(total, extra)-int64(r.g.shown) > maxSVGRepeats {
		return errSVGCSSRefs
	}
	return nil
}

// satAdd, satMul — сложение и умножение с потолком svgWeightCap (аргументы не больше потолка).
func satAdd(a, b int64) int64 { return min(a+b, svgWeightCap) }

func satMul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > svgWeightCap/b {
		return svgWeightCap
	}
	return min(a*b, svgWeightCap)
}

// uniqStrings — без повторов: class="a a … a" — один класс.
func uniqStrings(s []string) []string {
	if len(s) < 2 {
		return s
	}
	s = slices.Clone(s)
	slices.Sort(s)
	return slices.Compact(s)
}
