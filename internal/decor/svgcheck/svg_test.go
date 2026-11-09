package svgcheck

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const svgOpen = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 50">`

func svgDoc(body string) string { return svgOpen + body + `</svg>` }

// nested — n вложенных g внутри корня и rect в самой глубине: глубина дерева n + 2.
func nested(n int) string {
	return svgDoc(strings.Repeat("<g>", n) + `<rect width="1" height="1"/>` + strings.Repeat("</g>", n))
}

// svgGood — чистые файлы: разный состав, но только разрешённое.
var svgGood = map[string]string{
	"минимальный":   svgDoc(`<rect width="10" height="10"/>`),
	"с объявлением": `<?xml version="1.0" encoding="UTF-8"?>` + "\n<!-- комментарий -->\n" + svgDoc(`<path d="M0 0L10 10"/>`),
	"градиент":      svgDoc(`<defs><linearGradient id="a"><stop offset="0"/></linearGradient></defs><rect fill="url(#a)" style="stroke:url('#a')"/>`),
	"use и xlink":   svgDoc(`<defs><g id="x"/></defs><use xlink:href="#x"/><use href="#x"/>`),
	"встроенный PNG": svgDoc(`<image width="1" height="1" href="data:image/png;base64,` + tinyPNG + `"/>` +
		`<image width="1" height="1" xlink:href=" DATA:image/jpeg;base64,` + tinyJPEG + `"/>`),
	"стиль на #id":   svgDoc(`<style>.a{fill:url(#g)} .b{stroke:red}</style><rect class="a"/>`),
	"width и height": `<svg xmlns="http://www.w3.org/2000/svg" width="200px" height="100"><circle r="5"/></svg>`,
	"BOM":            "\xef\xbb\xbf" + svgDoc(`<rect/>`),
	// Редакторы (CorelDRAW, Affinity): простой DOCTYPE с внешним DTD — без внутренней части.
	"DOCTYPE SVG 1.1":              `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + svgDoc(`<rect/>`),
	"объявление и DOCTYPE":         `<?xml version="1.0"?>` + "\n" + `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + "\n" + svgDoc(`<rect/>`),
	"DOCTYPE простой":              `<!DOCTYPE svg>` + svgDoc(`<rect/>`),
	"DOCTYPE в одинарных кавычках": `<!DOCTYPE svg PUBLIC '-//W3C//DTD SVG 1.0//EN' 'http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd'>` + svgDoc(`<rect/>`),
	"DOCTYPE в две строки":         `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN"` + "\n  " + `"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + svgDoc(`<rect/>`),
	"DOCTYPE SYSTEM в одинарных":   `<!DOCTYPE svg SYSTEM 'a.dtd'>` + svgDoc(`<rect/>`),
	"DOCTYPE SYSTEM":               `<!DOCTYPE svg SYSTEM "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + svgDoc(`<rect/>`),
	// Blink не читает style в пространстве SVG с префиксом как CSS: url( там не ссылка.
	"s:style с url(http": svgDoc(`<rect xmlns:s="http://www.w3.org/2000/svg" s:style="fill:url(http://evil.example/a)"/>`),
	// Чужие пространства (Inkscape): значения — не CSS и не ссылки.
	"inkscape:export-filename": svgDoc(`<rect xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" inkscape:export-filename="C:\Users\user\монограмма (1).png"/>`),
	"inkscape:label url(":      svgDoc(`<rect xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" inkscape:label="см. url(старый"/>`),
	"сущности":                 svgDoc(`<text>a &amp; b &lt; c &#x41;</text>`),
	"CDATA":                    svgDoc(`<style><![CDATA[.a{fill:red}]]></style>`),
	"глубина 64":               nested(62),
	"текст со слэшем":          svgDoc(`<text>C:\папка\файл</text><rect data-x="C:\a"/>`),
}

// svgBad — отказы: причина → файл. Каждая строка нужна отдельно: убери проверку — упадёт именно она.
var svgBad = map[string]string{
	"script":             svgDoc(`<script>alert(1)</script>`),
	"SCRIPT в верхнем":   svgDoc(`<SCRIPT>1</SCRIPT>`),
	"foreignObject":      svgDoc(`<foreignObject><div/></foreignObject>`),
	"iframe":             svgDoc(`<iframe/>`),
	"object":             svgDoc(`<object/>`),
	"embed":              svgDoc(`<embed/>`),
	"audio":              svgDoc(`<audio/>`),
	"video":              svgDoc(`<video/>`),
	"animate":            svgDoc(`<rect><animate attributeName="x"/></rect>`),
	"animateTransform":   svgDoc(`<rect><animateTransform/></rect>`),
	"set":                svgDoc(`<rect><set attributeName="href" to="javascript:1"/></rect>`),
	"onload":             strings.Replace(svgDoc(``), "<svg ", `<svg onload="x()" `, 1),
	"onclick в глубине":  svgDoc(`<g><rect onclick="x()"/></g>`),
	"ONCLICK":            svgDoc(`<rect ONCLICK="x()"/>`),
	"x:onload":           svgDoc(`<rect xmlns:x="http://example.com/x" x:onload="x()"/>`),
	"x:src":              svgDoc(`<rect xmlns:x="http://example.com/x" x:src="http://evil.example/a"/>`),
	"onbegin":            svgDoc(`<rect onbegin="x()"/>`),
	"href http":          svgDoc(`<image href="http://evil.example/a.png"/>`),
	"href https":         svgDoc(`<use href="https://evil.example/a.svg#x"/>`),
	"href без схемы":     svgDoc(`<image href="//evil.example/a.png"/>`),
	"href файл":          svgDoc(`<image href="file:///c:/secret.png"/>`),
	"href относительный": svgDoc(`<image href="a.png"/>`),
	"href javascript":    svgDoc(`<a href="javascript:alert(1)"><rect/></a>`),
	"xlink:href внешний": svgDoc(`<image xlink:href="http://evil.example/a.png"/>`),
	"href data svg":      svgDoc(`<image href="data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="/>`),
	"href data html":     svgDoc(`<image href="data:text/html;base64,PGI+"/>`),
	"href пустой":        svgDoc(`<image href=""/>`),
	"feImage внешний":    svgDoc(`<filter><feImage href="http://evil.example/a.png"/></filter>`),
	"url в атрибуте":     svgDoc(`<rect fill="url(http://evil.example/a)"/>`),
	"url с пробелом":     svgDoc(`<rect fill="url( 'http://evil.example/a')"/>`),
	"url в style":        svgDoc(`<rect style="fill:url(https://evil.example/a)"/>`),
	"url в cursor":       svgDoc(`<rect style="cursor:url(//evil.example/c.cur),auto"/>`),
	"URL в верхнем":      svgDoc(`<rect style="fill:URL(http://evil.example/a)"/>`),
	"@import в style":    svgDoc(`<style>@import url(http://evil.example/a.css);</style>`),
	"@import без url":    svgDoc(`<style>@import "http://evil.example/a.css";</style>`),
	"@import в атрибуте": svgDoc(`<rect style="@import 'http://evil.example/a.css'"/>`),
	"url в тексте style": svgDoc(`<style>.a{background:url(http://evil.example/a)}</style>`),
	"url в CDATA style":  svgDoc(`<style><![CDATA[.a{fill:url(http://evil.example/a)}]]></style>`),
	// Аудит 048 G10: стиль режут комментарием, CDATA или дочерним элементом.
	"@import с комментарием": svgDoc(`<style>@imp<!---->ort "https://evil/a.css";</style>`),
	"@import с CDATA":        svgDoc(`<style>@imp<![CDATA[ort "https://evil/a.css";]]></style>`),
	"url с CDATA":            svgDoc(`<style>.a{fill:ur<![CDATA[l(https://evil/x.svg#p)}]]></style>`),
	"url с дочерним":         svgDoc(`<style>.a{fill:ur<g/>l(https://evil/x.svg#p)}</style>`),
	"image-set":              svgDoc(`<style>.a{fill:image-set("http://evil.example/a" 1x)}</style>`),
	"src( в стиле":           svgDoc(`<style>@font-face{font-family:x;src:src("http://evil.example/x.woff")}</style>`),
	"SRC( в атрибуте":        svgDoc(`<rect style="fill:SRC('//evil.example/a')"/>`),
	"CSS-экранирование":      svgDoc(`<style>.a{fill:u\72l(http://evil.example/a)}</style>`),
	"экранирование в attr":   svgDoc(`<rect style="fill:u\72l(http://evil.example/a)"/>`),
	"два DOCTYPE":            `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + svgDoc(`<rect/>`),
	"DOCTYPE после корня":    svgDoc(`<rect/>`) + `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">`,
	"DOCTYPE в элементе":     svgDoc(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">`),
	"DOCTYPE со вложенным <": `<!DOCTYPE svg SYSTEM "a<b">` + svgDoc(`<rect/>`),
	"doctype строчными":      `<!doctype svg>` + svgDoc(`<rect/>`),
	"ELEMENT вне DOCTYPE":    `<!ELEMENT svg ANY>` + svgDoc(`<rect/>`),
	"DOCTYPE с []":           `<!DOCTYPE svg []>` + svgDoc(`<rect/>`),
	"комментарий прячет [":   `<!DOCTYPE svg <!--[<!ENTITY x "y">]--> >` + svgDoc(`<rect/>`),
	"DOCTYPE разные кавычки": `<!DOCTYPE svg SYSTEM "a.dtd'>` + svgDoc(`<rect/>`),
	"DOCTYPE кавычка внутри": `<!DOCTYPE svg SYSTEM 'a'b'>` + svgDoc(`<rect/>`),
	"DOCTYPE не svg":         `<!DOCTYPE html>` + svgDoc(`<rect/>`),
	"DOCTYPE и потом xml":    `<!DOCTYPE svg><?xml version="1.0"?>` + svgDoc(`<rect/>`),
	"ENTITY":                 `<!DOCTYPE svg [<!ENTITY x "y">]>` + svgDoc(`<text>&x;</text>`),
	"ENTITY внешняя":         `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///c:/windows/win.ini">]>` + svgDoc(`<text>&x;</text>`),
	"неизвестная сущность":   svgDoc(`<text>&x;</text>`),
	"xml-stylesheet":         `<?xml-stylesheet href="http://evil.example/a.css"?>` + svgDoc(`<rect/>`),
	"PI в середине":          svgDoc(`<?php echo 1 ?><rect/>`),
	"второе объявление":      `<?xml version="1.0"?><?xml version="1.0"?>` + svgDoc(`<rect/>`),
	"глубина 65":             nested(63),
	"глубина 200":            nested(200),
	"без пространства":       `<svg viewBox="0 0 10 10"><rect/></svg>`,
	"чужое пространство":     `<svg xmlns="http://example.com/x" viewBox="0 0 10 10"/>`,
	"корень не svg":          `<html xmlns="http://www.w3.org/2000/svg"><body/></html>`,
	"два корня":              svgDoc(`<rect/>`) + svgDoc(`<rect/>`),
	"текст вне корня":        svgDoc(`<rect/>`) + "хвост",
	"не закрыт":              svgOpen + `<rect/>`,
	"закрыт не тот":          svgOpen + `<g></rect></svg>`,
	"не XML":                 "это не SVG",
	"пустой":                 "",
	"только пробелы":         "   \n",
	"кодировка":              `<?xml version="1.0" encoding="windows-1251"?>` + svgDoc(`<rect/>`),
	"нет размера":            `<svg xmlns="http://www.w3.org/2000/svg"/>`,
	"width 0":                `<svg xmlns="http://www.w3.org/2000/svg" width="0" height="10"/>`,
	"width процент":          `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%"/>`,
	"width отрицательный":    `<svg xmlns="http://www.w3.org/2000/svg" width="-5" height="10"/>`,
	"viewBox из трёх":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10"/>`,
	"viewBox NaN":            `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 NaN 10"/>`,
	"viewBox нулевой":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0 0"/>`,
	"viewBox Inf":            `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 Inf 10"/>`,
	"слишком вытянут":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100000 10"/>`,
}

// TestEditorSamples — файлы реальных редакторов (testdata/editors; без личных путей) принимаются.
func TestEditorSamples(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "editors", "*.svg"))
	if err != nil || len(files) < 10 {
		t.Fatalf("образцы редакторов не найдены: %v (%d)", err, len(files))
	}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := Check(data); err != nil {
			t.Errorf("%s: файл редактора отклонён: %v", filepath.Base(name), err)
		}
	}
}

// TestCheckSVG — чистые принимаются, остальное отклоняется с причиной по-русски и без путей.
func TestCheckSVG(t *testing.T) {
	for name, doc := range svgGood {
		if err := Check([]byte(doc)); err != nil {
			t.Errorf("%s: чистый файл отклонён: %v", name, err)
		}
	}
	for name, doc := range svgBad {
		err := Check([]byte(doc))
		if err == nil {
			t.Errorf("%s: принят", name)
			continue
		}
		if !hasCyrillic(err.Error()) {
			t.Errorf("%s: причина не по-русски: %v", name, err)
		}
	}
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if r >= 'А' && r <= 'я' {
			return true
		}
	}
	return false
}

// TestCheckSVGLimits — размер и число элементов: ровно на пределе — можно, на единицу больше — отказ.
func TestCheckSVGLimits(t *testing.T) {
	pad := func(n int) []byte { // чистый файл ровно из n байт: комментарий добивает
		doc := svgDoc(`<!---->`)
		return []byte(strings.Replace(doc, "<!---->", "<!--"+strings.Repeat("x", n-len(doc))+"-->", 1))
	}
	if err := Check(pad(MaxBytes)); err != nil {
		t.Errorf("ровно 2 МБ: %v", err)
	}
	if err := Check(pad(MaxBytes + 1)); err == nil {
		t.Error("2 МБ + 1 байт принят")
	}
	if err := Check(bytes.Repeat([]byte("a"), 3<<20)); err == nil {
		t.Error("3 МБ принят")
	}
	elems := func(n int) string { return svgDoc(strings.Repeat("<g/>", n-1)) } // корень — первый элемент
	if err := Check([]byte(elems(maxSVGElems))); err != nil {
		t.Errorf("ровно %d элементов: %v", maxSVGElems, err)
	}
	if err := Check([]byte(elems(maxSVGElems + 1))); err == nil {
		t.Errorf("%d элементов приняты", maxSVGElems+1)
	}
	attrs := func(n int) string {
		var b strings.Builder
		b.WriteString(`<rect`)
		for i := range n {
			b.WriteString(` a` + strings.Repeat("b", i%5) + string(rune('a'+i/5%26)) + string(rune('a'+i/130)) + `="1"`)
		}
		return svgDoc(b.String() + `/>`)
	}
	if err := Check([]byte(attrs(maxSVGAttrs))); err != nil {
		t.Errorf("ровно %d атрибутов: %v", maxSVGAttrs, err)
	}
	if err := Check([]byte(attrs(maxSVGAttrs + 1))); err == nil {
		t.Errorf("%d атрибутов приняты", maxSVGAttrs+1)
	}
}

// TestSvgSize — размер PNG: длинная сторона 4096, пропорция по viewBox, а без него — по width и height.
func TestSvgSize(t *testing.T) {
	cases := []struct {
		doc  string
		want Size
	}{
		{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"/>`, Size{4096, 2048}},
		{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="10,20,100,200"/>`, Size{2048, 4096}},
		{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 7 7"/>`, Size{4096, 4096}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="10mm" height="30mm"/>`, Size{1365, 4096}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 10 5"/>`, Size{4096, 2048}}, // viewBox важнее
		{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1"/>`, Size{4096, 4}},                         // предел вытянутости
	}
	for _, c := range cases {
		got, err := Inspect([]byte(c.doc))
		if err != nil || got != c.want {
			t.Errorf("%s → %v, %v; ждали %v", c.doc, got, err, c.want)
		}
	}
}

// oracleDoctype — грамматика допустимого DOCTYPE, разобранная вручную по токенам (не копия
// regex из кода): DOCTYPE svg [PUBLIC лит лит | SYSTEM лит], лит — в любых парных кавычках.
// Внутренняя часть «[…]» и вложенные «<» отвергаются отдельно.
func oracleDoctype(t []byte) bool {
	if strings.ContainsAny(string(t), "[<") {
		return false
	}
	const space = " \t\r\n"
	rest := string(t)
	// next — следующий токен: слово до пробела или литерал от кавычки до такой же;
	// за токеном обязателен пробел или конец (иначе «'a'b» слипается).
	next := func() (tok string, lit, ok bool) {
		rest = strings.TrimLeft(rest, space)
		if rest == "" {
			return "", false, false
		}
		if q := rest[0]; q == '"' || q == '\'' {
			end := strings.IndexByte(rest[1:], q)
			if end < 0 {
				return "", false, false
			}
			tok, rest, lit = rest[1:1+end], rest[end+2:], true
		} else {
			end := strings.IndexAny(rest, space)
			if end < 0 {
				end = len(rest)
			}
			tok, rest = rest[:end], rest[end:]
		}
		if rest != "" && !strings.ContainsRune(space, rune(rest[0])) {
			return "", false, false
		}
		return tok, lit, true
	}
	if w, lit, ok := next(); !ok || lit || w != "DOCTYPE" {
		return false
	}
	if w, lit, ok := next(); !ok || lit || w != "svg" {
		return false
	}
	kind, lit, ok := next()
	if !ok {
		return strings.TrimSpace(rest) == "" // «DOCTYPE svg»
	}
	n := 0
	switch {
	case lit:
		return false
	case kind == "PUBLIC":
		n = 2
	case kind == "SYSTEM":
		n = 1
	default:
		return false
	}
	for ; n > 0; n-- {
		if _, l, ok := next(); !ok || !l {
			return false
		}
	}
	_, _, more := next()
	return !more && strings.TrimSpace(rest) == ""
}

// oracleState — сырой обход для oracle: открытые элементы (это <style>?), текст стиля, счётчик use.
type oracleState struct {
	isStyle []bool
	css     strings.Builder
	uses    int
	started bool // корень уже открыт
	doctype bool // простой DOCTYPE уже был
}

func (o *oracleState) inStyle() bool { return len(o.isStyle) > 0 && o.isStyle[len(o.isStyle)-1] }

// token — запрещённое в одном токене ("" — чисто).
func (o *oracleState) token(tok xml.Token) string {
	switch t := tok.(type) {
	case xml.Directive:
		// один DOCTYPE до корня по грамматике (ENTITY живёт во внутренней части «[…]»)
		if o.started || o.doctype || !oracleDoctype(t) {
			return "directive"
		}
		o.doctype = true
	case xml.ProcInst:
		if t.Target != "xml" {
			return "procinst " + t.Target
		}
	case xml.StartElement:
		o.started = true
		if o.inStyle() {
			return "child in style"
		}
		if strings.EqualFold(t.Name.Local, "use") {
			if o.uses++; o.uses > 500 {
				return "use count"
			}
		}
		if why := oracleElem(t); why != "" {
			return why
		}
		o.isStyle = append(o.isStyle, strings.EqualFold(t.Name.Local, "style"))
	case xml.Comment:
		if o.inStyle() {
			return "comment in style"
		}
	case xml.EndElement:
		return o.end()
	case xml.CharData:
		if o.inStyle() {
			o.css.Write(t)
		}
	}
	return ""
}

// end — закрытие элемента; у <style> проверяется весь накопленный текст сразу.
func (o *oracleState) end() string {
	if len(o.isStyle) == 0 {
		return ""
	}
	if o.inStyle() {
		why := cssBad(strings.ToLower(o.css.String()))
		o.css.Reset()
		if why != "" {
			return why
		}
	}
	o.isStyle = o.isStyle[:len(o.isStyle)-1]
	return ""
}

// oracle — независимая проверка принятого файла (сырые токены, без разрешения пространств):
// чего бы ни нашёл фаззинг, принятый Check файл не должен содержать запрещённого.
func oracle(data []byte) string {
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	var o oracleState
	for {
		tok, err := d.RawToken()
		if err != nil {
			return "" // не разбирается сырым разбором — Check тоже отказал бы; здесь не судим
		}
		if why := o.token(tok); why != "" {
			return why
		}
	}
}

// oracleElem — запрещённое в имени и атрибутах элемента.
func oracleElem(t xml.StartElement) string {
	n := strings.ToLower(t.Name.Local)
	switch n {
	case "script", "foreignobject", "iframe", "object", "embed", "audio", "video", "set":
		return "element " + n
	}
	if strings.HasPrefix(n, "animate") {
		return "element " + n
	}
	for _, a := range t.Attr {
		if why := oracleAttr(n, a); why != "" {
			return why
		}
	}
	return ""
}

// oracleLimit — предельное число в атрибуте фильтра (n — элемент, a — атрибут, всё в нижнем регистре).
type oracleLimit struct {
	n, a string
	max  float64
}

var oracleLimits = map[string]oracleLimit{
	"fegaussianblur": {"fegaussianblur", "stddeviation", 100}, "fedropshadow": {"fedropshadow", "stddeviation", 100},
	"femorphology": {"femorphology", "radius", 100}, "feconvolvematrix": {"feconvolvematrix", "order", 9},
}

// oracleAttr — запрещённое в одном атрибуте элемента n (имя в нижнем регистре).
func oracleAttr(n string, a xml.Attr) string {
	an, v := strings.ToLower(a.Name.Local), strings.ToLower(strings.TrimSpace(a.Value))
	if strings.HasPrefix(an, "on") {
		return "attr " + an
	}
	switch an {
	case "src", "srcset", "data", "action", "formaction", "poster", "background", "ping", "http-equiv":
		return "attr " + an
	}
	if an == "base" && a.Name.Space == "xml" { // сырые токены: префикс, а не пространство
		return "xml:base"
	}
	if (a.Name.Space == "xmlns" || an == "xmlns") && strings.TrimSpace(a.Value) == "http://www.w3.org/1999/xhtml" { // имена пространств различают регистр
		return "xhtml"
	}
	if an == "numoctaves" && n == "feturbulence" {
		if f, err := strconv.ParseFloat(v, 64); err != nil || !(f <= 8) {
			return "numOctaves " + v
		}
	}
	if lim, ok := oracleLimits[n]; ok && an == lim.a {
		for _, s := range strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' }) {
			if f, err := strconv.ParseFloat(s, 64); err != nil || !(f <= lim.max) {
				return "filter " + n + " " + v
			}
		}
	}
	if an == "href" && !strings.HasPrefix(v, "#") && !strings.HasPrefix(v, "data:image/png;base64,") && !strings.HasPrefix(v, "data:image/jpeg;base64,") {
		return "href " + v
	}
	if a.Name.Space != "" { // CSS и url(…) — только в атрибутах без пространства имён
		return ""
	}
	return cssBad(v)
}

// cssBad — в значении CSS есть внешнее (v уже в нижнем регистре).
func cssBad(v string) string {
	if strings.Contains(v, "@import") || strings.Contains(v, "src(") {
		return "@import или src("
	}
	for i := 0; ; {
		j := strings.Index(v[i:], "url(")
		if j < 0 {
			return ""
		}
		i += j + len("url(")
		if arg := strings.TrimLeft(v[i:], " \t\r\n'\""); !strings.HasPrefix(arg, "#") {
			return "url(" + arg
		}
	}
}

// FuzzCheckSVG — любой вход: без паники и зависания; принятый файл проходит независимую
// проверку и даёт размер PNG в пределах. В обычном go test — затравки (чистые и вредные);
// поиск — go test -fuzz FuzzCheckSVG -fuzztime=20s.
func FuzzCheckSVG(f *testing.F) {
	for _, doc := range svgGood {
		f.Add([]byte(doc))
	}
	for _, doc := range svgBad {
		f.Add([]byte(doc))
	}
	for _, doc := range svgBadRules {
		f.Add([]byte(doc))
	}
	for _, doc := range svgGoodRules {
		f.Add([]byte(doc))
	}
	for _, c := range svgRefsBad {
		f.Add([]byte(c.doc))
	}
	for _, doc := range svgRefsGood {
		f.Add([]byte(doc))
	}
	f.Add([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect style="fill:u\72l(//x)"/></svg>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		size, err := Inspect(data)
		if err != nil {
			return
		}
		if size.W < 1 || size.H < 1 || size.W > Side || size.H > Side || max(size.W, size.H) != Side {
			t.Fatalf("принят с размером %v: %q", size, data)
		}
		if why := oracle(data); why != "" {
			t.Fatalf("принят, но %s: %q", why, data)
		}
		if why := oracleRepeats(data); why != "" {
			t.Fatalf("принят, но %s: %q", why, data)
		}
		if Check(data) != nil {
			t.Fatalf("Check и Inspect расходятся: %q", data)
		}
	})
}
