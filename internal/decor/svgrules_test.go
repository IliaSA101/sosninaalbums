package decor

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

// Тесты правил svgrules.go (аудит 048 G10): пространство XHTML, запрещённые атрибуты, встроенный
// растр, число use и октав. Образцы svgGood/svgBad живут в svg_test.go.

// encPNG и encJPEG — настоящие картинки w×h в base64.
func encPNG(w, h int) string {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h)))
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func encJPEG(w, h int) string {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, w, h)), nil)
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

var tinyPNG, tinyJPEG = encPNG(1, 1), encJPEG(1, 1)

// bombPNG — PNG, чей заголовок врёт про размер w×h (CRC IHDR пересчитан, иначе читатель откажет раньше).
func bombPNG(w, h uint32) string {
	raw, _ := base64.StdEncoding.DecodeString(tinyPNG)
	binary.BigEndian.PutUint32(raw[16:], w)
	binary.BigEndian.PutUint32(raw[20:], h)
	binary.BigEndian.PutUint32(raw[29:], crc32.ChecksumIEEE(raw[12:29]))
	return base64.StdEncoding.EncodeToString(raw)
}

// bombJPEG — JPEG с SOF0, объявляющим 65535×65535.
func bombJPEG() string {
	raw, _ := base64.StdEncoding.DecodeString(tinyJPEG)
	i := bytes.Index(raw, []byte{0xFF, 0xC0})
	copy(raw[i+5:], []byte{0xFF, 0xFF, 0xFF, 0xFF}) // высота, ширина
	return base64.StdEncoding.EncodeToString(raw)
}

func img(format, b64 string) string {
	return `<image width="1" height="1" href="data:image/` + format + `;base64,` + b64 + `"/>`
}

func repeat(s string, n int) string { return strings.Repeat(s, n) }

const xhtml = "http://www.w3.org/1999/xhtml"

// svgGoodRules — типичные файлы редакторов и края пределов: должны проходить.
var svgGoodRules = map[string]string{
	"Inkscape": `<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\n<!-- Created with Inkscape -->\n" +
		`<svg xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:cc="http://creativecommons.org/ns#" ` +
		`xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:svg="http://www.w3.org/2000/svg" ` +
		`xmlns="http://www.w3.org/2000/svg" xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd" ` +
		`xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" width="210mm" height="297mm" ` +
		`viewBox="0 0 210 297" version="1.1" id="svg8" inkscape:version="0.92" sodipodi:docname="x.svg">` +
		`<defs id="defs2"/><sodipodi:namedview id="base" pagecolor="#ffffff" inkscape:pageopacity="0.0" ` +
		`inkscape:zoom="0.35" inkscape:cx="400" showgrid="false"/>` +
		`<metadata id="metadata5"><rdf:RDF><cc:Work rdf:about=""><dc:format>image/svg+xml</dc:format>` +
		`<dc:type rdf:resource="http://purl.org/dc/dcmitype/StillImage"/><dc:title/></cc:Work></rdf:RDF></metadata>` +
		`<g inkscape:label="Слой 1" inkscape:groupmode="layer" id="layer1">` +
		`<rect style="fill:#ff0000;stroke-width:0.26" id="rect1" width="50" height="50" x="10" y="10"/></g></svg>`,
	"Illustrator": `<?xml version="1.0" encoding="utf-8"?>` + "\n<!-- Generator: Adobe Illustrator 24.0.0 -->\n" +
		`<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" ` +
		`x="0px" y="0px" viewBox="0 0 100 100" style="enable-background:new 0 0 100 100;" xml:space="preserve">` +
		"<style type=\"text/css\">\n\t.st0{fill:#FF0000;}\n\t/* комментарий CSS */\n</style>" +
		`<rect class="st0" width="10" height="10"/></svg>`,
	"base64 с переводами строк":      svgDoc(img("png", tinyPNG[:8]+"\n  "+tinyPNG[8:])),
	"пространство, похожее на XHTML": svgDoc(`<g xmlns:h="HTTP://www.w3.org/1999/XHTML"/>`),
	"use ровно 500":                  svgDoc(`<defs><g id="x"/></defs>` + repeat(`<use href="#x"/>`, maxSVGUse)),
	"numOctaves 8":                   svgDoc(`<filter><feTurbulence numOctaves="8"/></filter>`),
	"numOctaves 0":                   svgDoc(`<filter><feTurbulence numOctaves="0"/></filter>`),
	"numOctaves у другого":           svgDoc(`<rect numOctaves="100"/>`),
	"data- и ссылка на пространство": svgDoc(`<rect data-x="1" xlink:title="t"/>` +
		`<g xmlns:h="http://example.com/h"/>`),
}

// svgBadRules — отказы пунктов 2–4 аудита.
var svgBadRules = map[string]string{
	"XHTML по умолчанию":       svgDoc(`<div xmlns="` + xhtml + `"/>`),
	"XHTML с префиксом":        svgDoc(`<h:div xmlns:h="` + xhtml + `"/>`),
	"XHTML на корне":           `<svg xmlns="http://www.w3.org/2000/svg" xmlns:h="` + xhtml + `" viewBox="0 0 1 1"/>`,
	"XHTML объявлен, не нужен": svgDoc(`<g xmlns:h="` + xhtml + `"/>`),
	"use 501":            svgDoc(`<defs><g id="x"/></defs>` + repeat(`<use href="#x"/>`, maxSVGUse+1)),
	"USE в верхнем 501":  svgDoc(repeat(`<USE href="#x"/>`, maxSVGUse+1)),
	"numOctaves 9":       svgDoc(`<filter><feTurbulence numOctaves="9"/></filter>`),
	"numOctaves 1000":    svgDoc(`<filter><feTurbulence numOctaves="1000"/></filter>`),
	"numOctaves Inf":     svgDoc(`<filter><feTurbulence numOctaves="inf"/></filter>`),
	"numOctaves NaN":     svgDoc(`<filter><feTurbulence numOctaves="nan"/></filter>`),
	"numOctaves мусор":   svgDoc(`<filter><feTurbulence numOctaves="много"/></filter>`),
	"numOctaves регистр": svgDoc(`<filter><FETURBULENCE NUMOCTAVES="99"/></filter>`),
	"PNG не PNG":         svgDoc(img("png", base64.StdEncoding.EncodeToString([]byte("не картинка")))),
	"PNG не base64":      svgDoc(img("png", "!!!")),
	"PNG пустой":         svgDoc(img("png", "")),
	"JPEG под видом PNG": svgDoc(img("png", tinyJPEG)),
	"PNG под видом JPEG": svgDoc(img("jpeg", tinyPNG)),
	"бомба PNG":          svgDoc(img("png", bombPNG(20000, 20000))),
	"бомба PNG узкая":    svgDoc(img("png", bombPNG(1<<30, 1))),
	"бомба PNG нулевая":  svgDoc(img("png", bombPNG(0, 5))),
	"бомба JPEG":         svgDoc(img("jpeg", bombJPEG())),
	"сумма картинок":     svgDoc(img("png", bombPNG(15000, 15000)) + img("png", bombPNG(15000, 15000))),
	"xml:base":           svgDoc(`<g xml:base="http://evil.example/a.svg"><use href="#x"/></g>`),
	"xml:base пустой":    svgDoc(`<g xml:base=""/>`),
}

// badAttrCases — запрещённые атрибуты на любом элементе и в любом пространстве имён.
func badAttrCases() map[string]string {
	m := map[string]string{}
	for _, a := range []string{"src", "srcset", "data", "action", "formaction", "poster", "background", "ping", "http-equiv"} {
		m["атрибут "+a] = svgDoc(`<rect ` + a + `="http://evil.example/a"/>`)
		m["атрибут "+a+" в верхнем"] = svgDoc(`<rect ` + strings.ToUpper(a) + `="x"/>`)
		m["атрибут xlink:"+a] = svgDoc(`<rect xlink:` + a + `="x"/>`)
		m["атрибут "+a+" пустой"] = svgDoc(`<g><rect ` + a + `=""/></g>`)
	}
	return m
}

func TestSVGRulesBad(t *testing.T) {
	all := badAttrCases()
	for k, v := range svgBadRules {
		all[k] = v
	}
	for name, doc := range all {
		err := CheckSVG([]byte(doc))
		if err == nil {
			t.Errorf("%s: принят", name)
		} else if !hasCyrillic(err.Error()) {
			t.Errorf("%s: причина не по-русски: %v", name, err)
		}
	}
}

func TestSVGRulesGood(t *testing.T) {
	for name, doc := range svgGoodRules {
		if err := CheckSVG([]byte(doc)); err != nil {
			t.Errorf("%s: чистый файл отклонён: %v", name, err)
		}
	}
}

// TestSVGStyleSplit — любой способ порезать «@import»/«url(» внутри <style> — отказ, а склейка
// кусков без вредного — нет; <style> несколько раз подряд не копит чужой текст.
func TestSVGStyleSplit(t *testing.T) {
	bad := []string{
		`<style>@imp<!---->ort "x";</style>`,
		`<style>.a{}<!-- c --></style>`,
		`<style>@im<![CDATA[po]]>rt "x";</style>`,
		`<style>.a{fill:u<![CDATA[rl]]>(//x)}</style>`,
		`<style><g/></style>`,
		`<style>.a{}<g><style>x</style></g></style>`,
	}
	for _, b := range bad {
		if CheckSVG([]byte(svgDoc(b))) == nil {
			t.Errorf("принят: %s", b)
		}
	}
	good := []string{
		`<style>.a{fill:red}</style><style><![CDATA[.b{fill:url(#g)}]]></style>`,
		`<style>.a{fil</style><style>l:red}</style>`, // обрывки в разных <style> не склеиваются
		`<style>.a{fill:u<![CDATA[]]>rl(#g)}</style>`,
	}
	for _, g := range good {
		if err := CheckSVG([]byte(svgDoc(g))); err != nil {
			t.Errorf("отклонён: %s: %v", g, err)
		}
	}
	// Буфер не течёт между <style>: хвост «url(» из первого не должен ломать второй.
	if err := CheckSVG([]byte(svgDoc(`<style>.a{}</style><style>.b{x:#g)}</style>`))); err != nil {
		t.Errorf("буфер протёк между style: %v", err)
	}
}

// TestSVGRasterLimits — край: ровно 250 Мп проходит, на 10000 пикселей больше — отказ.
func TestSVGRasterLimits(t *testing.T) {
	if err := CheckSVG([]byte(svgDoc(img("png", bombPNG(10000, 25000))))); err != nil {
		t.Errorf("ровно 250 Мп: %v", err)
	}
	if err := CheckSVG([]byte(svgDoc(img("png", bombPNG(10000, 25001))))); err == nil {
		t.Error("250 Мп + 10000 пикселей приняты")
	}
}

// TestSVGFilterCost — стоимость фильтров: предел проходит, предел+1, пара чисел, мусор и регистр — нет.
func TestSVGFilterCost(t *testing.T) {
	good := []string{
		`<feGaussianBlur stdDeviation="100"/>`, `<feGaussianBlur stdDeviation="100, 0.5"/>`,
		`<feGaussianBlur stdDeviation="0"/>`, `<feGaussianBlur/>`, `<feDropShadow stdDeviation="100"/>`,
		`<feMorphology radius="100"/>`, `<feMorphology radius="100 1"/>`,
		`<feConvolveMatrix order="9"/>`, `<feConvolveMatrix order="9 3"/>`,
		`<rect stdDeviation="1000" radius="1000" order="1000"/>`, // у чужих элементов эти атрибуты ничего не значат
	}
	bad := []string{
		`<feGaussianBlur stdDeviation="100.5"/>`, `<feGaussianBlur stdDeviation="1 101"/>`,
		`<feGaussianBlur stdDeviation="1e9"/>`, `<feGaussianBlur stdDeviation="inf"/>`,
		`<feGaussianBlur stdDeviation="nan"/>`, `<feGaussianBlur stdDeviation="много"/>`,
		`<FEGAUSSIANBLUR STDDEVIATION="500"/>`, `<feDropShadow stdDeviation="101"/>`,
		`<feMorphology radius="101"/>`, `<feMorphology radius="3,200"/>`,
		`<feConvolveMatrix order="10"/>`, `<feConvolveMatrix order="3 10"/>`,
	}
	for _, f := range good {
		if err := CheckSVG([]byte(svgDoc(`<filter>` + f + `</filter>`))); err != nil {
			t.Errorf("%s: отклонён: %v", f, err)
		}
	}
	for _, f := range bad {
		err := CheckSVG([]byte(svgDoc(`<filter>` + f + `</filter>`)))
		if err == nil {
			t.Errorf("%s: принят", f)
		} else if !hasCyrillic(err.Error()) {
			t.Errorf("%s: причина не по-русски: %v", f, err)
		}
	}
}
