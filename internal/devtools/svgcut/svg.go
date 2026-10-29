// Package svgcut режет SVG Recraft (лист предметов, векторные пути по слоям) на отдельные PNG RGBA для
// наборов библиотеки. Инструмент разработчика, не функция программы: чистая логика на stdlib, растр
// делает Chromium внешней командой (PACKPREP_CHROME). Запуск — scripts/svgcut.go.
package svgcut

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Пределы разбора чужого файла (SVG приходит из сети: Recraft).
const (
	MaxSVGBytes = 64 << 20 // размер файла до чтения и разбора
	maxPaths    = 100_000  // путей в файле
	maxGrads    = 10_000   // градиентов
	maxStops    = 200      // остановок одного градиента
	maxDepth    = 16       // вложенность элементов
	maxSheetPx  = 20_000   // сторона viewBox
)

// Path — один контур листа. Порядок в Doc.Paths — порядок рисования (нижний первым).
type Path struct {
	D    string // как в файле
	RGB  [3]uint8
	Grad string // id градиента, если fill="url(#id)"; пусто — плоский цвет RGB
	Box  Box
	Rect bool // осевой прямоугольник
}

// Flat — заливка плоским цветом.
func (p *Path) Flat() bool { return p.Grad == "" }

// Fill — значение атрибута fill.
func (p *Path) Fill() string {
	if p.Grad != "" {
		return "url(#" + p.Grad + ")"
	}
	return fmt.Sprintf("rgb(%d,%d,%d)", p.RGB[0], p.RGB[1], p.RGB[2])
}

// grad — градиент defs: тег, атрибуты и остановки (порядок как в файле).
type grad struct {
	tag   string
	id    string
	attrs [][2]string
	stops [][][2]string
}

// Doc — разобранный лист.
type Doc struct {
	W, H  float64 // viewBox (начало 0 0)
	Paths []Path
	grads []grad
}

var (
	reRGB       = regexp.MustCompile(`^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$`)
	reURL       = regexp.MustCompile(`^url\(#([A-Za-z0-9_.-]{1,64})\)$`)
	reID        = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	reTranslate = regexp.MustCompile(`^\s*translate\(\s*0(?:\.0+)?\s*[, ]\s*0(?:\.0+)?\s*\)\s*$`)
	reAttrVal   = regexp.MustCompile(`^[A-Za-z0-9_.,#()%\s+-]{0,200}$`)
)

// Допустимые атрибуты по элементам; чужой атрибут — отказ (href, style с url и т. п.).
var attrsOK = map[string][]string{
	"svg":            {"version", "xmlns", "style", "viewBox", "width", "height", "preserveAspectRatio"},
	"defs":           nil,
	"metadata":       nil,
	"g":              {"transform"},
	"path":           {"transform", "fill", "d"},
	"linearGradient": {"id", "gradientUnits", "x1", "y1", "x2", "y2", "gradientTransform", "spreadMethod"},
	"radialGradient": {"id", "gradientUnits", "cx", "cy", "r", "fx", "fy", "gradientTransform", "spreadMethod"},
	"stop":           {"class", "offset", "stop-color", "stop-opacity"},
}

// ReadLimited читает не больше MaxSVGBytes; больше — ошибка до разбора.
func ReadLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSVGBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSVGBytes {
		return nil, fmt.Errorf("svgcut: файл больше %d МБ", MaxSVGBytes>>20)
	}
	return data, nil
}

// Parse разбирает SVG Recraft. Белый список элементов: svg, defs, linearGradient, radialGradient, stop,
// path, g, metadata; всё прочее (image, script, use, foreignObject, DOCTYPE …) — отказ, как и
// относительные команды пути и любой transform, кроме translate(0,0).
func Parse(data []byte) (*Doc, error) {
	if len(data) > MaxSVGBytes {
		return nil, fmt.Errorf("svgcut: файл больше %d МБ", MaxSVGBytes>>20)
	}
	dec := xml.NewDecoder(io.LimitReader(bytes.NewReader(data), MaxSVGBytes))
	dec.Strict = true
	dec.Entity = nil // незнакомая сущность — ошибка разбора, внешние не раскрываются
	doc := &Doc{}
	var stack []string
	var cur *grad
	seenRoot := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("svgcut: разбор XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) >= maxDepth {
				return nil, fmt.Errorf("svgcut: вложенность больше %d", maxDepth)
			}
			if err := doc.start(t, stack, &cur, &seenRoot); err != nil {
				return nil, err
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			if n := len(stack); n > 0 {
				if stack[n-1] == "linearGradient" || stack[n-1] == "radialGradient" {
					cur = nil
				}
				stack = stack[:n-1]
			}
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return nil, errors.New("svgcut: текст внутри SVG не допускается")
			}
		case xml.Directive:
			return nil, errors.New("svgcut: DOCTYPE и прочие директивы не допускаются")
		case xml.ProcInst:
			if t.Target != "xml" {
				return nil, fmt.Errorf("svgcut: инструкция <?%s?> не допускается", t.Target)
			}
		}
	}
	if !seenRoot {
		return nil, errors.New("svgcut: нет элемента svg")
	}
	if len(doc.Paths) == 0 {
		return nil, errors.New("svgcut: в файле нет путей")
	}
	return doc, doc.checkRefs()
}

// start обрабатывает открывающий тег: белый список, атрибуты, разбор путей и градиентов.
func (doc *Doc) start(t xml.StartElement, stack []string, cur **grad, seenRoot *bool) error {
	name := t.Name.Local
	if t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg" {
		return fmt.Errorf("svgcut: элемент %s:%s из чужого пространства имён", t.Name.Space, name)
	}
	allowed, ok := attrsOK[name]
	if !ok {
		return fmt.Errorf("svgcut: элемент <%s> не допускается", name)
	}
	if (name == "svg") != (len(stack) == 0) {
		return fmt.Errorf("svgcut: <%s> на недопустимом месте", name)
	}
	attrs := map[string]string{}
	for _, a := range t.Attr {
		if a.Name.Space != "" && a.Name.Space != "xmlns" {
			return fmt.Errorf("svgcut: <%s>: атрибут %s:%s не допускается", name, a.Name.Space, a.Name.Local)
		}
		if a.Name.Space == "xmlns" || !contains(allowed, a.Name.Local) {
			return fmt.Errorf("svgcut: <%s>: атрибут %q не допускается", name, a.Name.Local)
		}
		attrs[a.Name.Local] = a.Value
	}
	if tr, has := attrs["transform"]; has && !reTranslate.MatchString(tr) {
		return fmt.Errorf("svgcut: <%s>: transform %q не допускается (только translate(0,0))", name, tr)
	}
	parent := ""
	if len(stack) > 0 {
		parent = stack[len(stack)-1]
	}
	switch name {
	case "svg":
		*seenRoot = true
		return doc.setViewBox(attrs["viewBox"])
	case "defs":
		if parent != "svg" {
			return errors.New("svgcut: <defs> только в корне")
		}
	case "metadata":
		if parent != "svg" {
			return errors.New("svgcut: <metadata> только в корне")
		}
	case "g":
		if parent != "svg" && parent != "g" {
			return errors.New("svgcut: <g> на недопустимом месте")
		}
	case "linearGradient", "radialGradient":
		return doc.startGrad(name, t.Attr, parent, cur)
	case "stop":
		return doc.addStop(t.Attr, *cur)
	case "path":
		if parent != "svg" && parent != "g" {
			return errors.New("svgcut: <path> на недопустимом месте")
		}
		return doc.addPath(attrs)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// setViewBox читает viewBox «0 0 W H»; начало не в нуле или лист больше предела — отказ.
func (doc *Doc) setViewBox(vb string) error {
	f := strings.Fields(strings.ReplaceAll(vb, ",", " "))
	if len(f) != 4 {
		return fmt.Errorf("svgcut: viewBox %q: нужны четыре числа", vb)
	}
	var v [4]float64
	for i, s := range f {
		x, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e9 {
			return fmt.Errorf("svgcut: viewBox %q: плохое число", vb)
		}
		v[i] = x
	}
	if v[0] != 0 || v[1] != 0 {
		return fmt.Errorf("svgcut: viewBox %q: начало должно быть 0 0", vb)
	}
	if v[2] < 1 || v[3] < 1 || v[2] > maxSheetPx || v[3] > maxSheetPx {
		return fmt.Errorf("svgcut: лист %g×%g вне пределов 1…%d", v[2], v[3], maxSheetPx)
	}
	doc.W, doc.H = v[2], v[3]
	return nil
}

func (doc *Doc) startGrad(tag string, xa []xml.Attr, parent string, cur **grad) error {
	if parent != "defs" {
		return fmt.Errorf("svgcut: <%s> только в <defs>", tag)
	}
	if len(doc.grads) >= maxGrads {
		return fmt.Errorf("svgcut: градиентов больше %d", maxGrads)
	}
	g := grad{tag: tag}
	for _, a := range xa {
		if a.Name.Local == "id" {
			if !reID.MatchString(a.Value) {
				return fmt.Errorf("svgcut: id градиента %q не годится", a.Value)
			}
			g.id = a.Value
		} else if !reAttrVal.MatchString(a.Value) {
			return fmt.Errorf("svgcut: <%s>: значение %s=%q не допускается", tag, a.Name.Local, a.Value)
		}
		g.attrs = append(g.attrs, [2]string{a.Name.Local, a.Value})
	}
	if g.id == "" {
		return fmt.Errorf("svgcut: <%s> без id", tag)
	}
	doc.grads = append(doc.grads, g)
	*cur = &doc.grads[len(doc.grads)-1]
	return nil
}

func (doc *Doc) addStop(xa []xml.Attr, g *grad) error {
	if g == nil {
		return errors.New("svgcut: <stop> вне градиента")
	}
	if len(g.stops) >= maxStops {
		return fmt.Errorf("svgcut: остановок градиента больше %d", maxStops)
	}
	var st [][2]string
	for _, a := range xa {
		if !reAttrVal.MatchString(a.Value) {
			return fmt.Errorf("svgcut: <stop>: значение %s=%q не допускается", a.Name.Local, a.Value)
		}
		st = append(st, [2]string{a.Name.Local, a.Value})
	}
	g.stops = append(g.stops, st)
	return nil
}

func (doc *Doc) addPath(attrs map[string]string) error {
	if len(doc.Paths) >= maxPaths {
		return fmt.Errorf("svgcut: путей больше %d", maxPaths)
	}
	p := Path{D: attrs["d"]}
	fill := strings.TrimSpace(attrs["fill"])
	switch m, u := reRGB.FindStringSubmatch(fill), reURL.FindStringSubmatch(fill); {
	case m != nil:
		for i := 0; i < 3; i++ {
			v, _ := strconv.Atoi(m[i+1])
			if v > 255 {
				return fmt.Errorf("svgcut: путь %d: fill %q: канал больше 255", len(doc.Paths)+1, fill)
			}
			p.RGB[i] = uint8(v & 0xff) // v ≤ 255 проверено выше
		}
	case u != nil:
		p.Grad = u[1]
	default:
		return fmt.Errorf("svgcut: путь %d: fill %q не допускается (нужны rgb() или url(#id))", len(doc.Paths)+1, fill)
	}
	info, err := scanPath(p.D)
	if err != nil {
		return fmt.Errorf("svgcut: путь %d: %w", len(doc.Paths)+1, err)
	}
	p.Box, p.Rect = info.box, info.rect
	doc.Paths = append(doc.Paths, p)
	return nil
}

// checkRefs: каждый url(#id) ведёт на градиент из defs, id не повторяются.
func (doc *Doc) checkRefs() error {
	ids := map[string]bool{}
	for _, g := range doc.grads {
		if ids[g.id] {
			return fmt.Errorf("svgcut: id градиента %q повторяется", g.id)
		}
		ids[g.id] = true
	}
	for i, p := range doc.Paths {
		if p.Grad != "" && !ids[p.Grad] {
			return fmt.Errorf("svgcut: путь %d: градиент %q не определён", i+1, p.Grad)
		}
	}
	return nil
}

// Sheet — номер пути-листа: первый осевой прямоугольник во весь лист с минимальным каналом ≥ 245
// (по форме, а не по номеру: у садика лист 253,253,252). −1 — листа нет.
func (doc *Doc) Sheet() int {
	const tol = 0.5
	for i, p := range doc.Paths {
		if !p.Rect || !p.Flat() || min(p.RGB[0], p.RGB[1], p.RGB[2]) < 245 {
			continue
		}
		b := p.Box
		if b.X0 <= tol && b.Y0 <= tol && b.X1 >= doc.W-tol && b.Y1 >= doc.H-tol {
			return i
		}
	}
	return -1
}
