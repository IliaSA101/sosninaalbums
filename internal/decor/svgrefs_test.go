package decor

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// doublingDefs — «бомба удвоений» в <defs>: n уровней, каждый зовёт предыдущий дважды; по ссылке
// в href (группы с use) или в атрибуте (pattern с двумя rect fill=url).
func doublingDefs(n int, viaURL bool) string {
	var b strings.Builder
	b.WriteString(`<defs><g id="l0"><rect width="1" height="1"/></g>`)
	for i := 1; i <= n; i++ {
		if viaURL {
			fmt.Fprintf(&b, `<pattern id="l%d"><rect fill="url(#l%d)"/><rect fill="url(#l%d)"/></pattern>`, i, i-1, i-1)
		} else {
			fmt.Fprintf(&b, `<g id="l%d"><use href="#l%d"/><use href="#l%d"/></g>`, i, i-1, i-1)
		}
	}
	return b.String() + `</defs>`
}

// doubling — doublingDefs и один use вершины.
func doubling(n int, viaURL bool) string {
	return svgDoc(doublingDefs(n, viaURL) + fmt.Sprintf(`<use href="#l%d"/>`, n))
}

// levels — n уровней pattern по шаблону level(i) (в нём %[1]d — номер, %[2]d — предыдущий) поверх
// l0 и вершина tail.
func levels(n int, head, level, tail string) string {
	var b strings.Builder
	b.WriteString(head + `<defs><g id="l0"><rect/></g>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, level, i, i-1)
	}
	return svgDoc(b.String() + `</defs>` + fmt.Sprintf(tail, n))
}

// flat — группа x из m прямоугольников и k ссылок use на неё: повторов ровно k·(m+1).
func flat(m, k int) string {
	return svgDoc(`<defs><g id="x">` + repeat(`<rect/>`, m) + `</g></defs>` + repeat(`<use href="#x"/>`, k))
}

// verts — путь из n вершин.
func verts(n int) string { return "M0 0" + repeat(" L1 1", n-1) }

// classBomb — удвоение через классы: правило .cN заливает узором предыдущего уровня.
func classBomb(n int) string {
	var css strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&css, ".c%d{fill:url(#l%d)}", i, i-1)
	}
	return levels(n, `<style>`+css.String()+`</style>`, `<pattern id="l%[1]d"><rect class="c%[1]d"/><rect class="c%[1]d"/></pattern>`,
		`<rect fill="url(#l%d)"/>`)
}

// lace — образец из 60 путей и 300 фигур с его заливкой: 18 300 повторов, как у кружева.
func lace(group bool) string {
	defs := `<defs><pattern id="lace">` + repeat(`<path d="M0 0L1 1"/>`, 60) + `</pattern></defs>`
	if group {
		return svgDoc(defs + `<g fill="url(#lace)">` + repeat(`<rect/>`, 300) + `</g>`)
	}
	return svgDoc(defs + repeat(`<rect fill="url(#lace)"/>`, 300))
}

// heavyClip — клип из m путей и k прямоугольников, клип задан стилем css.
func heavyClip(css string, m, k int) string {
	return svgDoc(`<style>` + css + `</style><clipPath id="c">` + repeat(`<path/>`, m) + `</clipPath>` + repeat(`<rect class="a"/>`, k))
}

// nestedMarkers — маркер с путём из n вершин, на каждой — маркер m0; сам он — на пути из n вершин.
// at — куда ставится marker-mid: на путь или на группу вокруг него (наследование).
func nestedMarkers(n int, group bool) string {
	path := func(m string) string {
		if group {
			return `<g marker-mid="url(#` + m + `)"><path d="` + verts(n) + `"/></g>`
		}
		return `<path marker-mid="url(#` + m + `)" d="` + verts(n) + `"/>`
	}
	return svgDoc(`<marker id="m0"><rect/></marker><marker id="m1">` + path("m0") + `</marker>` + path("m1"))
}

// svgRefsBad — отказы подсчёта повторов: имя → файл и ожидаемая ошибка.
var svgRefsBad = map[string]struct {
	doc  string
	want error
}{
	"бомба use 40 уровней":        {doubling(40, false), errSVGRepeats},
	"бомба pattern 6000 уровней":  {doubling(6000, true), errSVGRepeats}, // не переполняет и не валит стек
	"бомба pattern 40 уровней":    {doubling(40, true), errSVGRepeats},
	"повторов на 1 больше":        {flat(999, 201), errSVGRepeats},
	"цепочка 5000: квадрат":       {clipChain(5000), errSVGRepeats}, // каждый rect тянет за собой всю цепочку
	"use на себя":                 {svgDoc(`<g id="a"><use href="#a"/></g>`), errSVGCycle},
	"круг через два":              {svgDoc(`<g id="a"><use href="#b"/></g><g id="b"><use href="#a"/></g>`), errSVGCycle},
	"круг через url":              {svgDoc(`<clipPath id="c"><rect clip-path="url(#c)"/></clipPath><rect clip-path="url(#c)"/>`), errSVGCycle},
	"круг через style":            {svgDoc(`<mask id="m"><rect style="mask:URL( '#m' )"/></mask><g mask="url(#m)"/>`), errSVGCycle},
	"бомба через класс":           {classBomb(20), errSVGRepeats},
	"сложный селектор со ссылкой": {heavyClip(`rect{clip-path:url(#c)}`, 1000, 300), errSVGCSSRefs},
	"at-правило со ссылкой":       {heavyClip(`@media print{.a{clip-path:url(#c)}}`, 1000, 300), errSVGCSSRefs},
	"две тяжёлые цели в сложных": {svgDoc(`<style>rect{fill:url(#p)} circle{fill:url(#q)}</style><pattern id="p"><path/></pattern>` +
		`<pattern id="q"><path/></pattern><rect/>`), errSVGCSSRefs},
	// Фрагмент, который Chromium нормализует (%xx, TAB, LF, CR), — отказ, а не ссылка «в никуда».
	"фрагмент %6C":           {svgDoc(doublingDefs(40, false) + `<use href="#%6C40"/>`), errSVGFragment},
	"фрагмент с TAB":         {svgDoc(doublingDefs(40, false) + `<use href="#l&#9;40"/>`), errSVGFragment},
	"фрагмент с LF":          {svgDoc(doublingDefs(40, false) + `<use href="#l&#10;40"/>`), errSVGFragment},
	"фрагмент с CR в url":    {svgDoc(doublingDefs(40, true) + `<rect fill="url(#l&#13;40)"/>`), errSVGFragment},
	"фрагмент с пробелом":    {svgDoc(doublingDefs(40, true) + `<rect style="fill:url('#l 40')"/>`), errSVGFragment},
	"фрагмент с em-пробелом": {svgDoc(`<g id="l40&#x2003;"/>` + `<use href="#l40&#x2003;"/>`), errSVGFragment},
	"фрагмент % в классе":    {svgDoc(`<style>.a{fill:url("#%6C40")}</style>` + doublingDefs(40, true) + `<rect class="a"/>`), errSVGFragment},
	// «}» и «/*» внутри строки CSS не прячут ссылку.
	"скобка в строке стиля": {svgDoc(`<style>.b{font-family:"}";fill:url(#l40)}</style>` + doublingDefs(40, true) +
		`<rect class="b"/>`), errSVGRepeats},
	"правило в строке": {svgDoc(`<style>rect{x:"}.a{";fill:url(#l40)}</style>` + doublingDefs(40, true) + `<rect/>`), errSVGCSSRefs},
	"комментарий в строке": {svgDoc(`<style>.a{x:"/*"} rect{fill:url(#l40)} .b{y:"*/"}</style>` + doublingDefs(40, true) +
		`<rect/>`), errSVGCSSRefs},
	"перевод строки в строке": {svgDoc("<style>.a{x:\"\n\";fill:url(#l40)}</style>" + doublingDefs(40, true) +
		`<rect class="a"/>`), errSVGCSSRefs},
	// Наследование и маркеры.
	"заливка у pattern 40": {levels(40, ``, `<pattern id="l%[1]d" fill="url(#l%[2]d)"><rect/><rect/></pattern>`,
		`<rect fill="url(#l%d)"/>`), errSVGRepeats},
	"заливка у группы 40": {levels(40, ``, `<pattern id="l%[1]d"><g style="fill:url(#l%[2]d)"><rect/><rect/></g></pattern>`,
		`<rect fill="url(#l%d)"/>`), errSVGRepeats},
	"заливка через use 40": {levels(40, `<defs><g id="two"><rect/><rect/></g></defs>`,
		`<pattern id="l%[1]d"><use href="#two" fill="url(#l%[2]d)"/></pattern>`, `<rect fill="url(#l%d)"/>`), errSVGRepeats},
	"маркер в маркере":         {nestedMarkers(30000, false), errSVGRepeats},
	"маркер в маркере группой": {nestedMarkers(30000, true), errSVGRepeats},
	"context-fill в маркере": {levels(40, `<defs><marker id="m"><path fill="context-fill"/></marker></defs>`,
		`<pattern id="l%[1]d"><path fill="url(#l%[2]d)" marker-start="url(#m)" d="M0 0"/></pattern>`, `<rect fill="url(#l%d)"/>`), errSVGRepeats},
	"clip-path: inherit 40": {levels(40, ``,
		`<clipPath id="l%[1]d"><g clip-path="url(#l%[2]d)"><rect clip-path="inherit"/><rect clip-path="inherit"/></g></clipPath>`,
		`<rect clip-path="url(#l%d)"/>`), errSVGRepeats},
	// Неканоничная запись — отказ, а не догадка, как прочтёт Blink (аудит 8391ede).
	"приманка ID перед id":  {shadowID(40), errSVGAttrName},
	"Class":                 {svgDoc(`<style>.a{fill:url(#p)}</style><pattern id="p"/><rect Class="a"/>`), errSVGAttrName},
	"xml:id":                {svgDoc(`<g xml:id="a"/>`), errSVGAttrName},
	"пробел в кавычках url": {svgDoc(doublingDefs(40, true) + `<rect style="fill:url(' #l40')"/>`), errSVGFragment},
	"пробел в конце кавычек":    {svgDoc(doublingDefs(40, true) + `<rect fill="url(&quot;#l40 &quot;)"/>`), errSVGFragment},
	"пробел в кавычках класса":  {svgDoc(`<style>.a{fill:url(" #l40")}</style>` + doublingDefs(40, true) + `<rect class="a"/>`), errSVGFragment},
	"маркер url(' #m')":         {svgDoc(`<marker id="m0"><rect/></marker><path marker-mid="url(' #m0')" d="M0 0L1 1"/>`), errSVGFragment},
	"@font-face src с пробелом": {svgDoc(`<style>@font-face{font-family:x;src:url(" #F")}</style><font id="F"/>`), errSVGFragment},
	"url без скобки":            {svgDoc(`<rect fill="url(#a"/>`), errSVGFragment},
	"url без кавычки":           {svgDoc(`<rect fill="url('#a)"/>`), errSVGFragment},
	"скобка в url":              {svgDoc(`<rect fill="url(#a(b))"/>`), errSVGFragment},
	"мусор после кавычки":       {svgDoc(`<rect fill="url('#a'b)"/>`), errSVGFragment},
	"id длиннее 256":            {svgDoc(`<use href="#` + repeat("a", 257) + `"/>`), errSVGFragment},
}

// shadowID — «ID» (для Blink не id) раньше настоящего id l<n> и бомба use под ним (проба аудита).
func shadowID(n int) string {
	return svgDoc(fmt.Sprintf(`<g ID="l%d"/>`, n) + doublingDefs(n, false) + fmt.Sprintf(`<use href="#l%d"/>`, n))
}

var svgRefsGood = map[string]string{
	"повторов ровно предел":     flat(999, 200),
	"10 уровней по 2":           doubling(10, false),
	"10 уровней pattern":        doubling(10, true),
	"цепочка 90 ссылок":         svgDoc(chain(90)),
	"ссылка в никуда":           svgDoc(`<use href="#нет"/><rect fill="url(#нет)"/>`),
	"a href не ссылка":          svgDoc(`<g id="a"><a href="#a"><rect/></a></g>`),
	"градиенты не считаются":    svgDoc(`<linearGradient id="a"/><linearGradient id="g" href="#a"/>` + repeat(`<path fill="url(#g)"/>`, 12000)),
	"градиент в классе":         svgDoc(`<style>.st0{fill:url(#g);} /* x */ .st1,.st2{stroke:url(#g)}</style><radialGradient id="g"/>` + repeat(`<rect class="st0 st1"/>`, 12000)),
	"Illustrator clip":          svgDoc(`<defs><path id="SVGID_1_" d="M0 0"/></defs><clipPath id="SVGID_2_"><use xlink:href="#SVGID_1_" style="overflow:visible;"/></clipPath><g style="clip-path:url(#SVGID_2_);"><rect/></g>`),
	"повторный id — первый":     svgDoc(`<g id="x"/><g id="x">` + repeat(`<rect/>`, 50) + `</g>` + repeat(`<use href="#x"/>`, 400)),
	"класс без правила":         svgDoc(`<style>.a{fill:red}</style><rect class="a b"/>`),
	"ссылка из класса в никуда": svgDoc(`<style>.a{fill:url(#нет)}rect{fill:red}</style><rect class="a"/>`),
	"кружево: образец × 300":    lace(false),
	"кружево: заливка у группы": lace(true),
	"CorelDRAW @font-face": svgDoc("<style type=\"text/css\"><![CDATA[\n@font-face { font-family:\"Arial\";font-variant:normal;" +
		"src:url(\"#FontID0\") format(svg)}\n.fil0 {fill:#373435}\n.fnt0 {font-size:7.06px;font-family:'Arial'}\n]]></style>" +
		`<defs><font id="FontID0" horiz-adv-x="556"><font-face font-family="Arial"/>` + repeat(`<glyph unicode="A" d="M0 0L1 1"/>`, 200) +
		`</font></defs>` + repeat(`<text class="fil0 fnt0">A</text>`, 600)), // как сложная ссылка — 600·201·2 > предела
	"скобка в строке, градиент":     svgDoc(`<style>.b{font-family:"}";fill:url(#g)}</style><linearGradient id="g"/><rect class="b"/>`),
	"сложный селектор, лёгкий клип": heavyClip(`rect{clip-path:url(#c)}`, 1, 10),
	"Illustrator: узор в классе": svgDoc(`<style>.st0{fill:url(#pat);} /* c */ .st1{clip-path:url(#c);}</style>` +
		`<pattern id="pat">` + repeat(`<path d="M0 0L1 1"/>`, 20) + `</pattern><clipPath id="c"><use href="#pat"/></clipPath>` +
		repeat(`<rect class="st0 st1"/>`, 300)),
	"маркеры на путях": svgDoc(`<marker id="m"><path d="M0 0L1 1"/></marker>` + repeat(`<path marker-mid="url(#m)" d="`+verts(10)+`"/>`, 100)),
	"Inkscape стрелка context-stroke": svgDoc(`<defs><marker id="Arrow"><path style="fill:context-stroke;stroke:context-stroke" d="M0 0L1 1L0 2z"/></marker></defs>` +
		repeat(`<path style="stroke:#000;marker-end:url(#Arrow)" d="M0 0L10 10"/>`, 100)),
	"пробелы вокруг аргумента url": svgDoc(`<linearGradient id="g"/><rect fill="url( #g )" style="stroke:URL( '#g' )"/>`),
	"id ровно 256":   svgDoc(`<g id="` + repeat("a", 256) + `"/><use href="#` + repeat("a", 256) + `"/>`),
	"rdf:ID — не id": svgDoc(`<g xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" rdf:ID="x"/>`),
	// @charset — at-правило без блока: следующее «.a» остаётся правилом класса, а не сложным.
	"@charset перед правилом": heavyClip(`@charset "UTF-8";.a{clip-path:url(#c)}`, 500, 300),
}

// chain — n групп, каждая зовёт предыдущую один раз: вес растёт линейно, глубина ссылок — n.
func chain(n int) string {
	var b strings.Builder
	b.WriteString(`<g id="c0"/>`)
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, `<g id="c%d"><use href="#c%d"/></g>`, i, i-1)
	}
	return b.String()
}

// clipChain — n клипов, каждый зовёт предыдущий, и по rect на каждый: повторов ~n² (через url, а
// не use — иначе раньше сработает maxSVGUse).
func clipChain(n int) string {
	var b strings.Builder
	b.WriteString(`<clipPath id="c0"/>`)
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, `<clipPath id="c%d"><rect clip-path="url(#c%d)"/></clipPath><rect clip-path="url(#c%d)"/>`, i, i-1, i)
	}
	return svgDoc(b.String())
}

func TestSVGRefsBad(t *testing.T) {
	for name, c := range svgRefsBad {
		err := CheckSVG([]byte(c.doc))
		switch {
		case !errors.Is(err, c.want):
			t.Errorf("%s: %v, ждали %v", name, err, c.want)
		case !hasCyrillic(err.Error()):
			t.Errorf("%s: причина не по-русски: %v", name, err)
		}
	}
	// Оракул видит бомбу за приманкой «ID» и за BOM в начале файла.
	for name, doc := range map[string]string{"ID": shadowID(40), "BOM": "\xef\xbb\xbf" + doubling(40, false)} {
		if oracleRepeats([]byte(doc)) == "" {
			t.Errorf("оракул пропустил бомбу: %s", name)
		}
	}
}

func TestSVGRefsGood(t *testing.T) {
	for name, doc := range svgRefsGood {
		if err := CheckSVG([]byte(doc)); err != nil {
			t.Errorf("%s: отклонён: %v", name, err)
		}
	}
}

// TestSVGRefsFast — правило с n ссылками и узел с n одинаковыми классами (было n² копий под
// ioMu: 37 с при n=2000) и тысяча разных ссылок на тысячу узлов (предел рёбер).
func TestSVGRefsFast(t *testing.T) {
	same := func(n int) string {
		return svgDoc(`<style>.a{` + repeat(`fill:url(#x);`, n) + `}</style><defs><pattern id="x"><rect/></pattern></defs>` +
			`<rect class="` + repeat("a ", n) + `"/><rect class="a"/>`)
	}
	var css, defs strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&css, "fill:url(#x%d);", i)
		fmt.Fprintf(&defs, `<pattern id="x%d"/>`, i)
	}
	wide := svgDoc(`<style>.a{` + css.String() + `}</style><defs>` + defs.String() + `</defs>` + repeat(`<rect class="a"/>`, 1001))
	// Аудит 8391ede: ссылки правила в никуда и на градиенты тоже рёбра (было 8,6 с и приём);
	// «url(#» без «)» — отказ за один проход (было n²: 2 МБ — ~20 минут).
	var none, grads, gdefs strings.Builder
	for i := range 80000 {
		fmt.Fprintf(&none, "fill:url(#n%d);", i)
	}
	for i := range 5000 {
		fmt.Fprintf(&grads, "fill:url(#g%d);", i)
		fmt.Fprintf(&gdefs, `<linearGradient id="g%d"/>`, i)
	}
	nowhere := svgDoc(`<style>.a{` + none.String() + `}</style>` + repeat(`<g class="a"/>`, 19000))
	gradient := svgDoc(`<style>.a{` + grads.String() + `}</style>` + gdefs.String() + repeat(`<g class="a"/>`, 14000))
	for _, c := range []struct {
		name, doc string
		want      error
	}{{"n=2000", same(2000), nil}, {"n=4000", same(4000), nil}, {"1000 × 1001", wide, errSVGRepeats},
		{"80 000 в никуда × 19 000", nowhere, errSVGRepeats}, {"5000 градиентов × 14 000", gradient, errSVGRepeats},
		{"url(#url(# в атрибуте", svgDoc(`<rect fill="` + repeat("url(#", 400000) + `"/>`), errSVGFragment},
		{"url(#url(# в стиле", svgDoc(`<style>` + repeat("url(#", 400000) + `</style>`), errSVGFragment}} {
		if len(c.doc) > MaxSVGBytes {
			t.Fatalf("%s: %d байт — больше предела, проверка не дойдёт до графа", c.name, len(c.doc))
		}
		var err error
		d := testkit.Fastest(3, func() { err = CheckSVG([]byte(c.doc)) })
		if d > testkit.Limit(time.Second) {
			t.Errorf("%s: %v", c.name, d)
		}
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, err, c.want)
		}
	}
}

// oracleRef — ссылка оракула с кратностью.
type oracleRef struct {
	id string
	k  int64
}

// oracleNode — элемент для независимого подсчёта повторов (сырые токены, без CSS).
type oracleNode struct {
	id    string
	grad  bool
	hide  bool  // сам не рисуется: defs, pattern… и всё внутри
	edges []int // дети; ссылки добавляются позже
	refs  []oracleRef
}

// oracleRepeats — независимая проверка принятого файла: круг по ссылкам или повторов больше
// предела недопустимы. CSS и наследование через use не учитываются, так что оракул слабее
// проверки и не даёт ложных тревог; заливки и маркеры атрибутов предков — учитываются.
func oracleRepeats(data []byte) string {
	nodes, shown := oracleParse(data)
	if len(nodes) == 0 {
		return ""
	}
	first := map[string]int{}
	for i, n := range nodes {
		if _, ok := first[n.id]; !ok && n.id != "" {
			first[n.id] = i
		}
	}
	type edge struct {
		to int
		k  int64
	}
	out := make([][]edge, len(nodes))
	for i := range nodes {
		for _, c := range nodes[i].edges {
			out[i] = append(out[i], edge{c, 1})
		}
		for _, r := range nodes[i].refs {
			if j, ok := first[r.id]; ok && !nodes[j].grad {
				out[i] = append(out[i], edge{j, r.k})
			}
		}
	}
	memo := make([]int64, len(nodes))
	onPath := make([]bool, len(nodes))
	var walk func(i int) (int64, bool)
	walk = func(i int) (int64, bool) {
		if memo[i] > 0 {
			return memo[i], true
		}
		if onPath[i] {
			return 0, false
		}
		onPath[i] = true
		sum := int64(1)
		for _, e := range out[i] {
			x, ok := walk(e.to)
			if !ok {
				return 0, false
			}
			sum = min(sum+e.k*x, 1<<40)
		}
		onPath[i], memo[i] = false, sum
		return sum, true
	}
	total, ok := walk(0)
	switch {
	case !ok:
		return "ссылки по кругу"
	case total-int64(shown) > maxSVGRepeats:
		return "повторов больше предела"
	}
	return ""
}

// oracleOpen — открытый элемент: номер и заливки с маркерами, которые получат его фигуры.
type oracleOpen struct {
	i            int
	paint, marks []string
}

// oracleParse — узлы по сырым токенам и число рисуемых без ссылок.
func oracleParse(data []byte) (nodes []oracleNode, shown int) {
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	var open []oracleOpen
	for {
		tok, err := d.RawToken()
		if err != nil {
			return nodes, shown
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var cur oracleOpen
			nodes, cur = oracleAdd(nodes, open, t)
			if !nodes[cur.i].hide {
				shown++
			}
			open = append(open, cur)
		case xml.EndElement:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		}
	}
}

// oracleAdd — новый узел под открытым open: ребро от родителя, заливки и маркеры предков.
func oracleAdd(nodes []oracleNode, open []oracleOpen, t xml.StartElement) ([]oracleNode, oracleOpen) {
	nd, own, paint, marks, v := oracleStart(t)
	nd.hide = own
	cur := oracleOpen{i: len(nodes)}
	if len(open) > 0 {
		p := open[len(open)-1]
		if !own {
			nodes[p.i].edges = append(nodes[p.i].edges, len(nodes))
		}
		nd.hide = own || nodes[p.i].hide
		cur.paint, cur.marks = p.paint, p.marks
	}
	if v > 0 { // фигура: заливки и маркеры предков рисует она, маркер — на каждой вершине
		for _, id := range cur.paint {
			nd.refs = append(nd.refs, oracleRef{id, 1})
		}
		for _, id := range cur.marks {
			nd.refs = append(nd.refs, oracleRef{id, v})
		}
		for _, r := range oracleMarks(t) {
			nd.refs = append(nd.refs, oracleRef{r, v - 1})
		}
	}
	cur.paint = append(cur.paint[:len(cur.paint):len(cur.paint)], paint...)
	cur.marks = append(cur.marks[:len(cur.marks):len(cur.marks)], marks...)
	return append(nodes, nd), cur
}

// oracleStart — узел по открывающему тегу, признак «сам не рисуется» (defs, pattern, градиент…),
// ссылки fill/stroke и marker-* для потомков и вершины фигуры (0 — не фигура).
func oracleStart(t xml.StartElement) (nd oracleNode, own bool, paint, marks []string, v int64) {
	n := strings.ToLower(t.Name.Local)
	nd.grad = strings.HasSuffix(n, "gradient")
	for _, a := range t.Attr {
		if a.Name.Space != "" && !strings.EqualFold(a.Name.Local, "href") { // чужие пространства — не CSS
			continue
		}
		switch an := strings.ToLower(a.Name.Local); {
		case a.Name.Local == "id" && a.Name.Space == "": // как у Blink в XML: «ID» и xml:id — не id
			nd.id = a.Value
		case an == "href" && n != "a" && strings.HasPrefix(strings.TrimFunc(a.Value, isC0), "#"):
			nd.refs = append(nd.refs, oracleRef{oracleFrag(strings.TrimFunc(a.Value, isC0)[1:]), 1})
		case an == "fill" || an == "stroke":
			paint = append(paint, oracleURLs(a.Value)...)
		case strings.HasPrefix(an, "marker-"):
			marks = append(marks, oracleURLs(a.Value)...)
		}
		if a.Name.Space != "" { // у xlink:href url(#) не разбирается, как и у чужих атрибутов
			continue
		}
		for _, id := range oracleURLs(a.Value) {
			nd.refs = append(nd.refs, oracleRef{id, 1})
		}
	}
	switch n {
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "text", "tspan", "textpath":
		v = oracleVerts(n, t)
	case "defs", "symbol", "clippath", "mask", "pattern", "marker", "filter", "style", "metadata", "title", "desc":
		return nd, true, paint, marks, v
	}
	return nd, nd.grad, paint, marks, v
}

// oracleMarks — ссылки собственных marker-* фигуры.
func oracleMarks(t xml.StartElement) (ids []string) {
	for _, a := range t.Attr {
		if strings.HasPrefix(strings.ToLower(a.Name.Local), "marker-") {
			ids = append(ids, oracleURLs(a.Value)...)
		}
	}
	return ids
}

// oracleVerts — вершины фигуры снизу: команды в d, пары чисел в points, у line — две.
func oracleVerts(n string, t xml.StartElement) int64 {
	v := int64(1)
	for _, a := range t.Attr {
		switch an := strings.ToLower(a.Name.Local); {
		case n == "line":
			v = 2
		case n == "path" && an == "d":
			for _, c := range a.Value {
				if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
					v++
				}
			}
		case (n == "polyline" || n == "polygon") && an == "points":
			nums := 0
			for _, f := range strings.FieldsFunc(a.Value, func(r rune) bool { return r == ',' || r == ' ' }) {
				if _, err := strconv.ParseFloat(f, 64); err == nil {
					nums++
				}
			}
			v = max(v, int64(nums/2))
		}
	}
	return v
}

// oracleURLs — id из «url(#id)» в значении; asciiLower не меняет длину, позиции совпадают
// с исходным значением (id берётся с регистром).
func oracleURLs(v string) (ids []string) {
	low := asciiLower(v)
	for pos := 0; ; {
		i := strings.Index(low[pos:], "url(")
		if i < 0 {
			return ids
		}
		pos += i + len("url(")
		if id, ok := oracleID(v[pos:]); ok {
			ids = append(ids, id)
		}
	}
}

// asciiLower — нижний регистр только для ASCII, длина строки не меняется.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// oracleID — id аргумента url(…) по худшему прочтению: в кавычках — до той же кавычки, без них —
// до «)»; пробелы по краям, и внутри кавычек тоже, срезаны («url(' #x')» — ссылка на x); затем
// фрагмент как у Chromium (oracleFrag).
func oracleID(arg string) (string, bool) {
	arg = strings.TrimLeft(arg, " \t\r\n\f")
	if arg != "" && (arg[0] == '"' || arg[0] == '\'') {
		q := arg[0]
		arg = arg[1:]
		if i := strings.IndexByte(arg, q); i >= 0 {
			arg = arg[:i]
		}
	} else if i := strings.IndexByte(arg, ')'); i >= 0 {
		arg = arg[:i]
	}
	arg = strings.Trim(arg, " \t\r\n\f")
	if !strings.HasPrefix(arg, "#") {
		return "", false
	}
	return oracleFrag(arg[1:]), true
}

// oracleFrag — фрагмент URL, как его видит Chromium: TAB, LF, CR выброшены, %xx раскрыты.
func oracleFrag(s string) string {
	s = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(s)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if x, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(x))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
