package decor

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // форматы для DecodeConfig встроенного растра
	_ "image/png"
	"math"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// Правила элементов и атрибутов SVG сверх «ни скриптов, ни сети» (аудит 048 G10).

const (
	xhtmlNS = "http://www.w3.org/1999/xhtml"
	// maxSVGUse — <use>: каждый может ссылаться на группу с другими <use>, число экземпляров растёт
	// экспонентой («миллиард смехов»); 500 хватает любой иконке.
	maxSVGUse = 500
	// maxSVGOctaves — numOctaves у feTurbulence: цена фильтра растёт с числом октав.
	maxSVGOctaves = 8
	// Цена остальных фильтров: размытие и морфология растут с радиусом, свёртка — с квадратом порядка.
	maxSVGBlur   = 100
	maxSVGMorph  = 100
	maxSVGMatrix = 9
)

// filterLimits — элемент фильтра → (атрибут, предел каждого числа в нём).
var filterLimits = map[string]struct {
	attr  string
	limit float64
}{
	"fegaussianblur":   {"stddeviation", maxSVGBlur},
	"fedropshadow":     {"stddeviation", maxSVGBlur},
	"femorphology":     {"radius", maxSVGMorph},
	"feconvolvematrix": {"order", maxSVGMatrix},
}

// checkFilterCost — числа атрибута фильтра (одно или пара) не выше предела; не число — отказ.
// Предел приблизительный: числа — в единицах SVG (пользовательских или долях рамки при
// primitiveUnits=objectBoundingBox), а цена растёт с радиусом в пикселях после масштаба viewBox
// под SVGSide. Пересчёт в пиксели потребовал бы трансформаций всех предков — не делаем.
func checkFilterCost(name, an, v string) error {
	l, ok := filterLimits[name]
	if !ok || an != l.attr {
		return nil
	}
	f := strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' })
	for _, s := range f {
		if x, err := strconv.ParseFloat(s, 64); err != nil || math.IsNaN(x) || x > l.limit {
			return fmt.Errorf("в SVG слишком дорогой фильтр «%s»", name)
		}
	}
	return nil
}

// badAttrs — атрибуты, которыми HTML- и медиаэлементы грузят ресурс или перенаправляют страницу;
// в SVG им делать нечего, поэтому отказ в любом пространстве имён (по локальному имени).
var badAttrs = map[string]bool{
	"src": true, "srcset": true, "data": true, "action": true, "formaction": true,
	"poster": true, "background": true, "ping": true, "http-equiv": true,
}

// checkAttrs — атрибуты элемента name: пространство XHTML, запрещённые атрибуты, обработчики,
// ссылки, CSS и numOctaves у feTurbulence. Чужие пространства редакторов (inkscape:, sodipodi:,
// rdf: …) не мешают: опасно только то, что исполняется или грузится.
func (w *svgWalker) checkAttrs(name string, t xml.StartElement) error {
	if t.Name.Space == xhtmlNS {
		return fmt.Errorf("в SVG есть элемент HTML «%s»", name)
	}
	for _, a := range t.Attr {
		an := strings.ToLower(a.Name.Local)
		if (a.Name.Space == "xmlns" || an == "xmlns") && strings.TrimSpace(a.Value) == xhtmlNS {
			return errors.New("в SVG подключено пространство имён HTML")
		}
		if badAttrs[an] {
			return fmt.Errorf("в SVG есть недопустимый атрибут «%s»", an)
		}
		if an == "base" && (a.Name.Space == xmlURL || a.Name.Space == "xml") { // меняет основу ссылок
			return errors.New("в SVG есть недопустимый атрибут «xml:base»")
		}
		if strings.HasPrefix(an, "on") {
			return fmt.Errorf("в SVG есть обработчик события «%s»", an)
		}
		if an == "href" {
			if err := w.checkHref(a.Value); err != nil {
				return err
			}
		}
		if err := checkValue(name, an, a); err != nil {
			return err
		}
	}
	return nil
}

// checkValue — значение атрибута: октавы и цена фильтра, CSS. CSS и url(…) читает Blink только
// в атрибутах без пространства имён: inkscape:label, export-filename и прочие чужие значения —
// не стиль (путь с обратной косой и скобкой не повод для отказа).
func checkValue(name, an string, a xml.Attr) error {
	if name == "feturbulence" && an == "numoctaves" {
		if v, err := strconv.ParseFloat(strings.TrimSpace(a.Value), 64); err != nil || math.IsNaN(v) || v > maxSVGOctaves {
			return fmt.Errorf("у feTurbulence больше %d октав", maxSVGOctaves)
		}
	}
	if err := checkFilterCost(name, an, a.Value); err != nil {
		return err
	}
	if a.Name.Space != "" {
		return nil
	}
	return checkCSS(a.Value, an == "style")
}

// checkHref — ссылка только внутрь файла (#id) или на встроенный растр PNG/JPEG, который
// действительно читается как такой растр в пределах imgfile (бомба сжатия — отказ).
func (w *svgWalker) checkHref(v string) error {
	v = strings.TrimSpace(v)
	l := strings.ToLower(v)
	if strings.HasPrefix(l, "#") {
		return nil
	}
	for _, f := range [...]string{"png", "jpeg"} {
		if p := "data:image/" + f + ";base64,"; strings.HasPrefix(l, p) {
			return w.checkRaster(f, v[len(p):])
		}
	}
	return errors.New("в SVG есть внешняя ссылка")
}

// checkRaster — встроенная картинка: base64 (пробелы и переводы строк допустимы), формат как
// заявлен, размер в пределах; суммарно по файлу — тоже (иначе много «лёгких» картинок
// съедят память окна).
func (w *svgWalker) checkRaster(format, b64 string) error {
	b64 = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, b64)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return errors.New("встроенная картинка в SVG не читается")
	}
	cfg, got, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || got != format {
		return errors.New("встроенная картинка в SVG не читается")
	}
	if err := imgfile.CheckDims(cfg.Width, cfg.Height); err != nil {
		return fmt.Errorf("встроенная картинка в SVG: %w", err)
	}
	if w.pixels += int64(cfg.Width) * int64(cfg.Height); w.pixels > imgfile.MaxPixels {
		return errors.New("встроенные картинки в SVG слишком велики")
	}
	return nil
}
