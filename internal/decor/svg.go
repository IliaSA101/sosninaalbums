package decor

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Проверка чужого SVG (design §5.4, решение №85). Go ничего не рисует и не «чистит»: файл либо
// чист, либо отказ с причиной. Рисует его окно программы через <img> (скрипты и сеть закрыты) и
// присылает PNG по одноразовому токену (svgtoken.go). Проверка — по токенам XML: белый список
// для корня, ссылок (#id и встроенный растр) и CSS, чёрный — для элементов и атрибутов
// (svgrules.go); пространства имён редакторов (inkscape:, sodipodi:, rdf: …) пропускаются.

// Пределы SVG (design §5.4).
const (
	MaxSVGBytes = 2 << 20 // исходник
	maxSVGDepth = 64      // вложенность элементов
	maxSVGElems = 20000   // элементов всего
	maxSVGAttrs = 256     // атрибутов у одного элемента
	// SVGSide — длинная сторона PNG, который окно рисует из SVG; maxSVGRatio — самая вытянутая
	// рамка (короче этого короткая сторона стала бы одним пикселем).
	SVGSide     = 4096
	maxSVGRatio = 1000
)

var svgNS = "http://www.w3.org/2000/svg"

// badElems — элементы, которые не пускаются ни при каких условиях (по локальному имени).
var badElems = map[string]bool{
	"script": true, "foreignobject": true, "iframe": true, "object": true, "embed": true,
	"audio": true, "video": true, "set": true, "handler": true, "listener": true,
}

// svgSize — размер, который окно должно нарисовать: длинная сторона SVGSide.
type svgSize struct{ W, H int }

// CheckSVG — SVG чист: размер, глубина, число элементов, корень, элементы, атрибуты,
// ссылки и CSS по белому списку. Ошибка — причина для тоста (по-русски, без путей).
func CheckSVG(data []byte) error {
	_, err := inspectSVG(data)
	return err
}

var errSVGXML = errors.New("SVG не разбирается как XML")

// svgWalker — обход токенов XML: стек открытых элементов и счётчики пределов.
type svgWalker struct {
	stack   []string // локальные имена открытых элементов в нижнем регистре
	elems   int
	uses    int    // элементов use
	pixels  int64  // пикселей во встроенных картинках
	css     []byte // текст открытого <style>: куски и CDATA склеиваются, иначе @imp<!---->ort проскочит
	root    bool
	seenPI  bool
	doctype bool // простой DOCTYPE уже был (второй — отказ)
	size    svgSize
	refs    svgRefs // граф ссылок для подсчёта повторов (svgrefs.go)
}

// inspectSVG — CheckSVG и размер будущего PNG по viewBox (иначе width/height) корня.
func inspectSVG(data []byte) (svgSize, error) {
	if len(data) > MaxSVGBytes {
		return svgSize{}, fmt.Errorf("SVG больше %d МБ", MaxSVGBytes>>20)
	}
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	d.Strict = true // Entity не задан: допустимы только пять предопределённых
	var w svgWalker
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return svgSize{}, errSVGXML
		}
		if err := w.token(tok); err != nil {
			return svgSize{}, err
		}
	}
	if !w.root || len(w.stack) != 0 {
		return svgSize{}, errSVGXML
	}
	if err := w.refs.check(); err != nil {
		return svgSize{}, err
	}
	return w.size, nil
}

// token — один токен XML: отказ или продолжение.
func (w *svgWalker) token(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.Directive:
		return w.directive(t)
	case xml.ProcInst:
		// Только объявление <?xml …?> первым токеном; стили и прочее — отказ.
		if t.Target != "xml" || w.root || w.seenPI {
			return errors.New("в SVG есть инструкция обработки")
		}
		w.seenPI = true
	case xml.StartElement:
		return w.start(t)
	case xml.Comment:
		if w.inStyle() {
			return errors.New("в стиле SVG есть комментарий")
		}
	case xml.EndElement:
		if len(w.stack) == 0 {
			return errSVGXML
		}
		if w.inStyle() {
			css := string(w.css)
			w.css = w.css[:0]
			if err := checkCSS(css, true); err != nil {
				return err
			}
			if err := w.refs.css(css); err != nil {
				return err
			}
		}
		w.stack = w.stack[:len(w.stack)-1]
		w.refs.pop()
	case xml.CharData:
		if len(w.stack) == 0 {
			if len(bytes.TrimSpace(t)) > 0 {
				return errors.New("текст вне корня SVG")
			}
			return nil
		}
		if w.inStyle() {
			w.css = append(w.css, t...)
		}
	}
	return nil
}

// directive — «<!…>» вне элементов: разрешён один простой DOCTYPE до корня (CorelDRAW, Affinity,
// старый Illustrator пишут DOCTYPE SVG 1.1 с внешним DTD). Внутренняя часть «[…]» (там живут
// ENTITY) и любой «<» внутри — отказ. Внешний DTD не грузится ни здесь (encoding/xml его не
// читает: Entity не задан, сущности вне пяти предопределённых — ошибка), ни в WebView
// (браузер не загружает внешние DTD для SVG-как-картинки).
func (w *svgWalker) directive(t xml.Directive) error {
	ok := !w.root && !w.doctype && !bytes.ContainsAny(t, "[<") &&
		bytes.HasPrefix(t, []byte("DOCTYPE")) && len(t) > len("DOCTYPE") && strings.IndexByte(" \t\r\n", t[len("DOCTYPE")]) >= 0
	if !ok {
		return errors.New("в SVG есть DOCTYPE с внутренней частью, ENTITY или лишняя директива")
	}
	w.doctype = true
	return nil
}

// inStyle — самый внутренний открытый элемент — <style>.
func (w *svgWalker) inStyle() bool {
	return len(w.stack) > 0 && w.stack[len(w.stack)-1] == "style"
}

// start — открывающий тег: пределы, единственный корень с размером, проверка элемента.
func (w *svgWalker) start(t xml.StartElement) error {
	w.elems++
	if w.elems > maxSVGElems {
		return fmt.Errorf("в SVG больше %d элементов", maxSVGElems)
	}
	if len(w.stack) >= maxSVGDepth {
		return fmt.Errorf("в SVG вложенность больше %d", maxSVGDepth)
	}
	if w.inStyle() {
		return errors.New("в стиле SVG есть вложенный элемент")
	}
	if len(w.stack) == 0 {
		if w.root {
			return errors.New("в SVG больше одного корня")
		}
		w.root = true
		sz, err := rootSize(t)
		if err != nil {
			return err
		}
		w.size = sz
	}
	name := strings.ToLower(t.Name.Local)
	if name == "use" {
		if w.uses++; w.uses > maxSVGUse {
			return fmt.Errorf("в SVG больше %d элементов use", maxSVGUse)
		}
	}
	if err := w.checkElem(name, t); err != nil {
		return err
	}
	w.stack = append(w.stack, name)
	return w.refs.push(name, t)
}

// checkElem — один элемент: имя, число атрибутов, сами атрибуты (svgrules.go).
func (w *svgWalker) checkElem(name string, t xml.StartElement) error {
	if badElems[name] || strings.HasPrefix(name, "animate") {
		return fmt.Errorf("в SVG есть недопустимый элемент «%s»", name)
	}
	if len(t.Attr) > maxSVGAttrs {
		return errors.New("в SVG слишком много атрибутов")
	}
	return w.checkAttrs(name, t)
}

// checkCSS — значение атрибута или текст <style>: ни @import, ни url( не на #id, ни
// image-set( и src( (вторая запись ссылки в CSS), ни CSS-экранирования (u\72l( обходит поиск
// по подстроке). css — это стиль.
func checkCSS(v string, css bool) error {
	if !css && !strings.ContainsAny(v, "(@") {
		return nil
	}
	l := strings.ToLower(v)
	if strings.Contains(l, "@import") || strings.Contains(l, "image-set(") || strings.Contains(l, "expression(") ||
		strings.Contains(l, "src(") {
		return errors.New("в SVG есть внешний стиль")
	}
	if strings.Contains(l, `\`) && (css || strings.Contains(l, "(")) {
		return errors.New("в SVG есть экранирование в стиле")
	}
	for rest := l; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			return nil
		}
		arg := strings.TrimLeft(rest[i+len("url("):], " \t\r\n'\"")
		if !strings.HasPrefix(arg, "#") {
			return errors.New("в SVG есть внешняя ссылка")
		}
		rest = arg
	}
}

// rootSize — корень «svg» в пространстве SVG с размером: по viewBox, иначе по width и height.
func rootSize(t xml.StartElement) (svgSize, error) {
	var zero svgSize
	if strings.ToLower(t.Name.Local) != "svg" || t.Name.Space != svgNS {
		return zero, errors.New("это не SVG (нужен корень svg с xmlns)")
	}
	var w, h float64
	var vb, ww, hh string
	for _, a := range t.Attr {
		switch strings.ToLower(a.Name.Local) {
		case "viewbox":
			vb = a.Value
		case "width":
			ww = a.Value
		case "height":
			hh = a.Value
		}
	}
	if vb != "" {
		f := strings.FieldsFunc(vb, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' })
		if len(f) != 4 {
			return zero, errors.New("в SVG не читается viewBox")
		}
		w, h = svgNum(f[2]), svgNum(f[3])
	} else {
		w, h = svgNum(ww), svgNum(hh)
	}
	if !(w > 0) || !(h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return zero, errors.New("у SVG нет размера (viewBox или width и height)")
	}
	long, short := max(w, h), min(w, h)
	if long/short > maxSVGRatio {
		return zero, errors.New("SVG слишком вытянут")
	}
	px := max(1, int(math.Round(SVGSide*short/long)))
	if w >= h {
		return svgSize{SVGSide, px}, nil
	}
	return svgSize{px, SVGSide}, nil
}

// svgNum — число с необязательной единицей длины (px, pt, mm…); процент и мусор — NaN.
func svgNum(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimRightFunc(s, func(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' })
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}
