package svgcut

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

const svgNS = "http://www.w3.org/2000/svg"

// esc экранирует значение атрибута: в выход SVG попадает только проверенное, но экранирование
// держит разметку целой при любых данных.
func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func attr(b *strings.Builder, name, val string) {
	b.WriteByte(' ')
	b.WriteString(name)
	b.WriteString(`="`)
	b.WriteString(esc(val))
	b.WriteByte('"')
}

// writeDefs пишет градиенты листа как есть (пути ссылаются на них по id).
func (doc *Doc) writeDefs(b *strings.Builder) {
	for _, g := range doc.grads {
		b.WriteString("<" + g.tag)
		for _, a := range g.attrs {
			attr(b, a[0], a[1])
		}
		b.WriteString(">")
		for _, st := range g.stops {
			b.WriteString("<stop")
			for _, a := range st {
				attr(b, a[0], a[1])
			}
			b.WriteString("/>")
		}
		b.WriteString("</" + g.tag + ">")
	}
}

// maskID — id маски выреза пути pi: «k<N>», а если файл уже называет так градиент (reID это пропускает),
// к id дописывается «_» — иначе url(#id) ссылался бы на того, кто в defs раньше.
func (doc *Doc) maskID(pi int) string {
	id := "k" + strconv.Itoa(pi)
	for taken := true; taken; {
		taken = false
		for _, g := range doc.grads {
			if g.id == id {
				id += "_"
				taken = true
				break
			}
		}
	}
	return id
}

// writePath пишет путь с заливкой fill (пусто — собственная заливка пути).
func writePath(b *strings.Builder, p *Path, fill string) {
	if fill == "" {
		fill = p.Fill()
	}
	b.WriteString("<path")
	attr(b, "fill", fill)
	attr(b, "d", p.D)
	b.WriteString("/>")
}

// open пишет начало документа: viewBox — область листа в координатах листа, размер — в пикселях;
// preserveAspectRatio=none, чтобы округление размера не сдвигало содержимое.
func open(b *strings.Builder, view Box, pw, ph int, extra string) {
	fmt.Fprintf(b, `<svg xmlns="%s" width="%d" height="%d" viewBox="%s %s %s %s" preserveAspectRatio="none"%s>`,
		svgNS, pw, ph, num(view.X0), num(view.Y0), num(view.W()), num(view.H()), extra)
}

// itemSVG собирает документ предмета: defs листа, пути группы в порядке рисования. Вырез (knocked)
// стирает всё, что нарисовано ниже белого пути, там, где виден сам белый путь; пути над ним остаются
// целыми — это «белый путь минус пути над ним», иначе прорезался бы кант над пустотой.
func itemSVG(doc *Doc, g group, knocked map[int]bool, frame Box, pw, ph int) []byte {
	var b strings.Builder
	open(&b, frame, pw, ph, "")
	b.WriteString("<defs>")
	doc.writeDefs(&b)
	acc := ""
	for _, pi := range g.Paths {
		if knocked[pi] {
			id := doc.maskID(pi)
			fmt.Fprintf(&b, `<mask id="%s" maskUnits="userSpaceOnUse" x="%s" y="%s" width="%s" height="%s">`,
				id, num(frame.X0), num(frame.Y0), num(frame.W()), num(frame.H()))
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="#fff"/>`, num(frame.X0), num(frame.Y0), num(frame.W()), num(frame.H()))
			writePath(&b, &doc.Paths[pi], "#000")
			b.WriteString("</mask>")
			if acc != "" {
				acc = `<g mask="url(#` + id + `)">` + acc + "</g>"
			}
			continue
		}
		var pb strings.Builder
		writePath(&pb, &doc.Paths[pi], "")
		acc += pb.String()
	}
	b.WriteString("</defs>")
	b.WriteString(acc)
	b.WriteString("</svg>")
	return []byte(b.String())
}

// hitSVG — служебный растр листа для попадания точек: белые пути закрашены цветами-номерами
// (rgb кодирует номер+1), остальные — чёрным, края без сглаживания. Прозрачный пиксель — пустой лист,
// чёрный — не белый путь. Лист-фон не рисуется.
func hitSVG(doc *Doc, sheet int, white func(i int) bool, pw, ph int) []byte {
	var b strings.Builder
	open(&b, Box{0, 0, doc.W, doc.H}, pw, ph, ` shape-rendering="crispEdges"`)
	for i := range doc.Paths {
		if i <= sheet { // лист и всё под ним не видно
			continue
		}
		fill := "#000"
		if white(i) {
			id := i + 1
			fill = fmt.Sprintf("rgb(%d,%d,%d)", id>>16&255, id>>8&255, id&255)
		}
		writePath(&b, &doc.Paths[i], fill)
	}
	b.WriteString("</svg>")
	return []byte(b.String())
}

// overlaySVG — лист с подсветкой: все пути как есть на сером, белые пути обведены (красным — как есть,
// зелёным — с вырезом) и пронумерованы wN, рамки групп — синим с номером группы (для split и drop).
func overlaySVG(doc *Doc, sheet int, white func(i int) bool, whiteN map[int]int, knocked map[int]bool, groups []group, pw, ph int) []byte {
	var b strings.Builder
	open(&b, Box{0, 0, doc.W, doc.H}, pw, ph, "")
	b.WriteString("<defs>")
	doc.writeDefs(&b)
	b.WriteString("</defs>")
	fmt.Fprintf(&b, `<rect width="%s" height="%s" fill="#8c8c8c"/>`, num(doc.W), num(doc.H))
	for i := range doc.Paths {
		if i > sheet {
			writePath(&b, &doc.Paths[i], "")
		}
	}
	stroke := num(max(1.5, doc.W/700))
	font := num(max(10, doc.W/110))
	for i := range doc.Paths {
		if !white(i) {
			continue
		}
		col := "#e00"
		if knocked[i] {
			col = "#0b0"
		}
		p := &doc.Paths[i]
		b.WriteString(`<path fill="none"`)
		attr(&b, "stroke", col)
		attr(&b, "stroke-width", stroke)
		attr(&b, "d", p.D)
		b.WriteString("/>")
		cx, cy := p.Box.Center()
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="sans-serif" font-size="%s" fill="%s">w%d</text>`, num(cx), num(cy), font, col, whiteN[i])
	}
	for _, g := range groups {
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="#06f" stroke-width="%s"/>`,
			num(g.Box.X0), num(g.Box.Y0), num(g.Box.W()), num(g.Box.H()), stroke)
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="sans-serif" font-size="%s" fill="#06f">%d</text>`,
			num(g.Box.X0), num(g.Box.Y0-2), font, g.Base)
	}
	b.WriteString("</svg>")
	return []byte(b.String())
}
