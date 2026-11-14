package library

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Marks — отметки фото из файла (XMP). Только чтение, пишется при сканировании.
type Marks struct {
	Rating    int    `json:"rating,omitempty"`    // 0–5
	Label     string `json:"label,omitempty"`     // red|yellow|green|blue|purple|other; "" — нет
	LabelText string `json:"labelText,omitempty"` // исходный текст, только у other (≤ 64 знаков, без управляющих)
	Flag      int    `json:"flag,omitempty"`      // 1 отобрано, −1 отклонено
}

// markFields — какие поля встретились в пакете: сайдкар заменяет встроенный XMP по полям.
type markFields uint8

const (
	fieldRating markFields = 1 << iota
	fieldLabel
	fieldFlag
	fieldRejectByRating // отказ взят из Rating = −1, а не из pick: сайдкар с новым рейтингом его снимает
)

// Пределы для недоверенного XMP: сайдкар больше не читается, а глубина и число
// токенов ограничивают время разбора «широких» и «глубоких» пакетов.
const (
	maxXMPBytes  = 1 << 20
	maxXMPDepth  = 32
	maxXMPTokens = 100_000
	maxLabelLen  = 64
	maxXMPText   = 4096 // текст значения свойства: длиннее не бывает, хвост отбрасывается
)

const (
	nsXMP = "http://ns.adobe.com/xap/1.0/"
	nsDM  = "http://ns.adobe.com/xmp/1.0/DynamicMedia/"
)

var (
	errXMPDirective = errors.New("XMP: DOCTYPE/ENTITY не допускаются")
	errXMPSize      = errors.New("XMP: пакет больше предела")
	errXMPDepth     = fmt.Errorf("XMP: глубина больше %d", maxXMPDepth)
	errXMPTokens    = fmt.Errorf("XMP: больше %d токенов", maxXMPTokens)
	errXMPPanic     = errors.New("XMP: паника")
)

// quoteErr — текст из файла для log.txt: обрезан и экранирован, чтобы мегабайтное имя
// или перевод строки в пакете не раздували лог и не подделывали в нём строки.
func quoteErr(v any) string { return fmt.Sprintf("%.200q", fmt.Sprint(v)) }

// parseXMP разбирает пакет XMP (из JPEG или сайдкара). Ошибка — пакет битый или за
// пределами: отметки нулевые, вызывающий пишет строку в log.txt с именем файла.
func parseXMP(packet []byte) (m Marks, f markFields, err error) {
	if len(packet) > maxXMPBytes {
		return Marks{}, 0, fmt.Errorf("%w: %d байт", errXMPSize, len(packet))
	}
	// Пакет Lightroom с историей обработки бывает 50+ КБ: без слов-маркеров XML не разбираем.
	if !bytes.Contains(packet, []byte("Rating")) && !bytes.Contains(packet, []byte("Label")) &&
		!bytes.Contains(packet, []byte("pick")) {
		return Marks{}, 0, nil
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ПАНИКА при разборе XMP: %s", quoteErr(r))
			m, f, err = Marks{}, 0, fmt.Errorf("%w: %s", errXMPPanic, quoteErr(r))
		}
	}()
	m, f, err = decodeXMP(packet)
	if err != nil {
		return Marks{}, 0, err
	}
	return m, f, nil
}

// xmpProp — свойство, которое ищем; propNone — прочие.
type xmpProp uint8

const (
	propNone xmpProp = iota
	propRating
	propLabel
	propPick
)

func propOf(n xml.Name) xmpProp {
	switch {
	case n.Space == nsXMP && n.Local == "Rating":
		return propRating
	case n.Space == nsXMP && n.Local == "Label":
		return propLabel
	case n.Space == nsDM && n.Local == "pick":
		return propPick
	}
	return propNone
}

// xmpRaw — сырые значения: первое вхождение каждого свойства. bad — значение нечитаемо
// (длиннее maxXMPText или вложенная структура): поля как будто нет.
type xmpRaw struct {
	val  [4]string
	seen [4]bool
	bad  [4]bool
}

func (r *xmpRaw) set(p xmpProp, v string, bad bool) {
	if p == propNone || r.seen[p] {
		return
	}
	if bad {
		v = ""
	}
	r.val[p], r.seen[p], r.bad[p] = v, true, bad
}

func decodeXMP(packet []byte) (Marks, markFields, error) {
	d := xml.NewDecoder(bytes.NewReader(packet))
	d.Strict = true
	d.Entity = nil // незнакомая сущность — ошибка разбора, внешние не раскрываются
	var x xmpScan
	for n := 0; ; n++ {
		if n >= maxXMPTokens {
			return Marks{}, 0, errXMPTokens
		}
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Marks{}, 0, fmt.Errorf("XMP: %s", quoteErr(err))
		}
		switch t := tok.(type) {
		case xml.Directive:
			return Marks{}, 0, errXMPDirective
		case xml.StartElement:
			if err := x.start(t); err != nil {
				return Marks{}, 0, err
			}
		case xml.CharData:
			x.chars(t)
		case xml.EndElement:
			x.end()
		}
	}
	m, f := x.raw.marks()
	return m, f, nil
}

// xmpScan — состояние разбора пакета между токенами.
type xmpScan struct {
	raw     xmpRaw
	depth   int
	capProp xmpProp // свойство-элемент, текст которого сейчас собираем
	capAt   int
	capBad  bool // у свойства-элемента вложенный элемент или текст длиннее maxXMPText
	text    []byte
}

func (x *xmpScan) start(t xml.StartElement) error {
	x.depth++
	if x.depth > maxXMPDepth {
		return errXMPDepth
	}
	for _, a := range t.Attr {
		x.raw.set(propOf(a.Name), a.Value, len(a.Value) > maxXMPText) // длинное не храним
	}
	if x.capProp != propNone {
		x.capBad = true // rdf:Alt и прочие структуры значением не считаются
	} else if p := propOf(t.Name); p != propNone && !x.raw.seen[p] {
		x.capProp, x.capAt, x.capBad, x.text = p, x.depth, false, x.text[:0]
	}
	return nil
}

// chars копирует текст: байты токена живут только до следующего Token().
func (x *xmpScan) chars(t xml.CharData) {
	if x.capProp == propNone || x.depth != x.capAt || x.capBad {
		return
	}
	if len(x.text)+len(t) > maxXMPText {
		x.capBad = true
	} else {
		x.text = append(x.text, t...)
	}
}

func (x *xmpScan) end() {
	if x.capProp != propNone && x.depth == x.capAt {
		x.raw.set(x.capProp, string(x.text), x.capBad)
		x.capProp = propNone
	}
	x.depth--
}

// marks переводит сырые значения в отметки. Нечитаемое значение — как будто поля нет:
// сайдкар с мусором не затирает верную отметку из JPEG.
func (r *xmpRaw) marks() (Marks, markFields) {
	var m Marks
	var f markFields
	if r.seen[propRating] && !r.bad[propRating] {
		if v, ok := parseRating(r.val[propRating]); ok {
			f |= fieldRating
			if v < 0 {
				m.Flag = -1 // Bridge: Rating = −1 — «отклонено»
				f |= fieldFlag | fieldRejectByRating
			} else {
				m.Rating = v
			}
		}
	}
	// Пустая метка — явное снятие; метка из одних управляющих символов — мусор, поля нет.
	if r.seen[propLabel] && !r.bad[propLabel] {
		if v := r.val[propLabel]; strings.TrimSpace(v) == "" {
			f |= fieldLabel
		} else if c := cleanLabel(v); c != "" {
			f |= fieldLabel
			m.Label, m.LabelText = labelColor(c)
		}
	}
	if r.seen[propPick] && !r.bad[propPick] {
		ok := true
		switch strings.TrimSpace(r.val[propPick]) {
		case "1":
			m.Flag = 1
		case "0":
			m.Flag = 0
		case "-1":
			m.Flag = -1
		default:
			ok = false
		}
		if ok { // флаг теперь из pick, а не из рейтинга
			f = (f | fieldFlag) &^ fieldRejectByRating
		}
	}
	return m, f
}

// parseRating: камеры и программы пишут и «3», и «3.0»; −1 — отклонено, вне −1…5 — мусор.
func parseRating(s string) (int, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || !(v > -1.5 && v < 5.5) { // NaN тоже сюда
		return 0, false
	}
	return int(math.Round(v)), true
}

// cleanLabel убирает управляющие и форматирующие символы (в том числе смену направления
// письма) и неверный UTF-8, обрезает до maxLabelLen знаков: текст метки идёт в подсказку.
func cleanLabel(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(strings.ToValidUTF8(s, "")) {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			continue
		}
		if n == maxLabelLen {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// labelNames — наборы меток Lightroom и Bridge по порядку красный…фиолетовый.
var labelNames = map[string]string{
	"red": "red", "yellow": "yellow", "green": "green", "blue": "blue", "purple": "purple",
	// Русский Lightroom (гипотеза, сверить на образце фотографа)
	"красный": "red", "желтый": "yellow", "зеленый": "green", "синий": "blue", "фиолетовый": "purple",
	// Bridge
	"select": "red", "second": "yellow", "approved": "green", "review": "blue", "to do": "purple",
	// Review Status
	"to delete": "red", "color correction needed": "yellow", "good to use": "green",
	"retouching needed": "blue", "to print": "purple",
}

// labelColor переводит текст xmp:Label в цвет: знакомое имя — цвет, иначе other с исходным текстом.
func labelColor(text string) (color, keep string) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", ""
	}
	key := strings.ReplaceAll(strings.ToLower(t), "ё", "е")
	if c, ok := labelNames[key]; ok {
		return c, ""
	}
	return "other", t
}
