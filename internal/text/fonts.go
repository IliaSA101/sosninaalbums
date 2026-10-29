// Пакет text — встроенные шрифты, перенос строк и отрисовка текста в экспорте.
// Экран и экспорт меряют одними и теми же файлами, поэтому переносы совпадают.
package text

import (
	"embed"
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"
	"sync"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/*.ttf fonts/OFL-*.txt
var fontFS embed.FS

// Значения по умолчанию и пределы стиля.
const (
	DefaultFont  = "montserrat"
	DefaultSize  = 14.0
	DefaultColor = "#222222"
	MinSize      = 6.0
	MaxSize      = 200.0
)

// Style — оформление текста; хранится в проекте (JSON) и приходит из интерфейса.
type Style struct {
	Font   string  `json:"font"`  // id из families
	Size   float64 `json:"size"`  // pt
	Color  string  `json:"color"` // #rrggbb
	Align  string  `json:"align"` // left | center | right
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
}

// Face — одно начертание гарнитуры для интерфейса.
type Face struct {
	File   string `json:"file"`
	Bold   bool   `json:"bold"`
	Italic bool   `json:"italic"`
}

// Family — гарнитура: css — имя для font-family в интерфейсе, group — раздел
// списка (my | script | kids | serif | sans | display). Hidden и NoCyr — только у
// своих: убрана из списка (×) и нет русских букв.
type Family struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	CSS    string `json:"css"`
	Group  string `json:"group"`
	Faces  []Face `json:"faces"`
	Hidden bool   `json:"hidden,omitempty"`
	NoCyr  bool   `json:"noCyr,omitempty"`
}

// Families возвращает копию списка гарнитур: встроенные, затем свои
// зарегистрированные (по имени без регистра, сломанные не отдаются).
func Families() []Family {
	out := make([]Family, len(families))
	for i, f := range families {
		out[i] = f
		out[i].Faces = append([]Face(nil), f.Faces...)
	}
	return append(out, customFamilies()...)
}

// FontFile отдаёт байты шрифта по имени файла. Имя ищется среди известных
// начертаний (белый список): путь из запроса в ФС не попадает. Своё начертание —
// «<16 hex>.<ttf|otf>» зарегистрированной гарнитуры.
func FontFile(name string) ([]byte, bool) {
	for _, f := range families {
		for _, fc := range f.Faces {
			if fc.File == name {
				b, err := fontFS.ReadFile("fonts/" + name)
				return b, err == nil
			}
		}
	}
	return customFile(name)
}

var (
	parseOnce sync.Once
	parsed    map[string]*sfnt.Font
	parseErr  error
)

// loadFonts разбирает все шрифты один раз.
func loadFonts() (map[string]*sfnt.Font, error) {
	parseOnce.Do(func() {
		parsed = map[string]*sfnt.Font{}
		for _, f := range families {
			for _, fc := range f.Faces {
				b, err := fontFS.ReadFile("fonts/" + fc.File)
				if err != nil {
					parseErr = err
					return
				}
				sf, err := sfnt.Parse(b)
				if err != nil {
					parseErr = fmt.Errorf("шрифт %s: %w", fc.File, err)
					return
				}
				parsed[fc.File] = sf
			}
		}
	})
	return parsed, parseErr
}

// builtinFamily — встроенная гарнитура по id или nil.
func builtinFamily(id string) *Family {
	for i := range families {
		if families[i].ID == id {
			return &families[i]
		}
	}
	return nil
}

// family ищет гарнитуру по id: встроенная, своя зарегистрированная, иначе — по умолчанию.
func family(id string) Family {
	if f := builtinFamily(id); f != nil {
		return *f
	}
	if c := lookupCustom(id); c != nil {
		return c.asFamily()
	}
	return *builtinFamily(DefaultFont)
}

// has сообщает, есть ли у гарнитуры начертание с такими признаками.
func (f Family) has(bold, italic bool) bool {
	for _, fc := range f.Faces {
		if fc.Bold == bold && fc.Italic == italic {
			return true
		}
	}
	return false
}

// faceFile выбирает файл начертания; нужного нет — обычное начертание гарнитуры
// (у своей без обычного — первое из курсива, жирного, жирного курсива).
func faceFile(st Style) string {
	if c := lookupCustom(st.Font); c != nil {
		f := c.face(st.Bold, st.Italic)
		return f.Key + "." + f.Ext
	}
	f := family(st.Font)
	for _, fc := range f.Faces {
		if fc.Bold == st.Bold && fc.Italic == st.Italic {
			return fc.File
		}
	}
	return f.Faces[0].File
}

// font возвращает разобранный шрифт стиля и имя его файла. Своя гарнитура, которой
// нет или чей файл не читается, — Montserrat того же начертания: никогда не nil,
// иначе Layout вернул бы пустой блок и текст исчез бы.
func font(st Style) (*sfnt.Font, string) {
	if IsCustomID(st.Font) {
		if c := usableCustom(st.Font); c != nil {
			f := c.face(st.Bold, st.Italic)
			if sf := f.d.parse(c.load); sf != nil {
				return sf, f.Key + "." + f.Ext
			}
		}
		st.Font = DefaultFont
	}
	m, err := loadFonts()
	file := faceFile(st)
	if err != nil {
		return nil, file
	}
	return m[file], file
}

// Clamped приводит стиль к допустимому: неизвестная гарнитура — Montserrat,
// жирный и курсив сбрасываются, если у гарнитуры нет такого начертания, размер в 6..200 pt (NaN/Inf — по умолчанию),
// выравнивание и цвет вне формата — по умолчанию. Id своей гарнитуры остаётся
// всегда (082-5): незарегистрированная рисуется Montserrat, у которой есть все
// четыре начертания, поэтому её жирный и курсив не трогаются — иначе альбом
// потерял бы их на компьютере без шрифта.
func (s Style) Clamped() Style {
	var f Family
	if b := builtinFamily(s.Font); b != nil {
		f = *b
	} else if c := lookupCustom(s.Font); c != nil {
		f = c.asFamily()
	} else {
		if !IsCustomID(s.Font) {
			s.Font = DefaultFont
		}
		f = *builtinFamily(DefaultFont)
	}
	s.Bold = s.Bold && f.has(true, false)
	s.Italic = s.Italic && f.has(false, true)
	if !f.has(s.Bold, s.Italic) { // пара без своего файла (жирный курсив при неполном наборе)
		s.Bold, s.Italic = false, false
	}
	if math.IsNaN(s.Size) || math.IsInf(s.Size, 0) {
		s.Size = DefaultSize
	}
	s.Size = math.Min(math.Max(s.Size, MinSize), MaxSize)
	switch s.Align {
	case "left", "center", "right":
	default:
		s.Align = "center"
	}
	if _, ok := parseColor(s.Color); !ok {
		s.Color = DefaultColor
	}
	return s
}

// FamilyIDs — идентификаторы гарнитур по порядку (для тестов и проверок).
func FamilyIDs() []string {
	ids := make([]string, len(families))
	for i, f := range families {
		ids[i] = f.ID
	}
	sort.Strings(ids)
	return ids
}

// parseColor разбирает «#rrggbb».
func parseColor(s string) (color.RGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, false
	}
	var c [3]uint8
	for i := range c {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return color.RGBA{}, false
		}
		c[i] = uint8(v)
	}
	return color.RGBA{c[0], c[1], c[2], 255}, true
}

// fallback — запасной шрифт стиля (Montserrat того же начертания) и имя его
// файла: знак, которого нет в гарнитуре, берётся из него, как в CSS
// `"SA X", "SA Montserrat"`. st уже Clamped. У самой Montserrat запасного нет.
func fallback(st Style) (*sfnt.Font, string) {
	if st.Font == DefaultFont || (IsCustomID(st.Font) && usableCustom(st.Font) == nil) {
		return nil, "" // своя без файла уже рисуется Montserrat
	}
	return font(Style{Font: DefaultFont, Bold: st.Bold, Italic: st.Italic})
}

// glyphSrc — шрифт, из которого взят глиф, и его буфер.
type glyphSrc struct {
	f *sfnt.Font
	b *sfnt.Buffer
}

// pickGlyph ищет знак в основном шрифте, затем в запасном (fb.f может быть nil);
// ok=false — знака нет нигде, он пропускается. Буфер у каждого шрифта свой.
func pickGlyph(main, fb glyphSrc, r rune) (src glyphSrc, g sfnt.GlyphIndex, ok bool) {
	if g, err := main.f.GlyphIndex(main.b, r); err == nil && g != 0 {
		return main, g, true
	}
	if fb.f != nil {
		if g, err := fb.f.GlyphIndex(fb.b, r); err == nil && g != 0 {
			return fb, g, true
		}
	}
	return glyphSrc{}, 0, false
}

// walkGlyphs — общий обход строки для measure и Draw: глиф (из основного шрифта
// или запасного), его позиция pos и ширина adv в масштабе unit (единицы
// основного шрифта ×64). Кернинг только между знаками одного шрифта, как в
// браузере; знак без глифа нигде пропускается.
func walkGlyphs(main, alt glyphSrc, s string, unit fixed.Int26_6, fn func(r rune, src glyphSrc, g sfnt.GlyphIndex, pos, adv fixed.Int26_6)) {
	var prev sfnt.GlyphIndex
	var prevFont *sfnt.Font
	var pos fixed.Int26_6
	for _, r := range s {
		src, g, ok := pickGlyph(main, alt, r)
		if !ok {
			continue
		}
		adv, err := src.f.GlyphAdvance(src.b, g, unit, xfont.HintingNone)
		if err != nil {
			continue
		}
		if prevFont == src.f { // nil до первого глифа
			if k, err := src.f.Kern(src.b, prev, g, unit, xfont.HintingNone); err == nil {
				pos += k
			}
		}
		fn(r, src, g, pos, adv)
		pos += adv
		prev, prevFont = g, src.f
	}
}
