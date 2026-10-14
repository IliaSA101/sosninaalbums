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

	"golang.org/x/image/font/sfnt"
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
// списка (script | kids | serif | sans).
type Family struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	CSS   string `json:"css"`
	Group string `json:"group"`
	Faces []Face `json:"faces"`
}

// Families возвращает копию списка гарнитур.
func Families() []Family {
	out := make([]Family, len(families))
	for i, f := range families {
		out[i] = f
		out[i].Faces = append([]Face(nil), f.Faces...)
	}
	return out
}

// FontFile отдаёт байты встроенного шрифта по имени файла. Имя ищется среди
// известных начертаний (белый список): путь из запроса в ФС не попадает.
func FontFile(name string) ([]byte, bool) {
	for _, f := range families {
		for _, fc := range f.Faces {
			if fc.File == name {
				b, err := fontFS.ReadFile("fonts/" + name)
				return b, err == nil
			}
		}
	}
	return nil, false
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

// family ищет гарнитуру по id; неизвестный id — гарнитура по умолчанию.
func family(id string) Family {
	def := Family{}
	for _, f := range families {
		if f.ID == id {
			return f
		}
		if f.ID == DefaultFont {
			def = f
		}
	}
	return def
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

// faceFile выбирает файл начертания; нужного нет — обычное начертание гарнитуры.
func faceFile(st Style) string {
	f := family(st.Font)
	for _, fc := range f.Faces {
		if fc.Bold == st.Bold && fc.Italic == st.Italic {
			return fc.File
		}
	}
	return f.Faces[0].File
}

// font возвращает разобранный шрифт стиля и имя его файла.
func font(st Style) (*sfnt.Font, string) {
	m, err := loadFonts()
	file := faceFile(st)
	if err != nil {
		return nil, file
	}
	return m[file], file
}

// Clamped приводит стиль к допустимому: неизвестная гарнитура — Montserrat,
// жирный и курсив сбрасываются, если у гарнитуры нет такого начертания, размер в 6..200 pt (NaN/Inf — по умолчанию),
// выравнивание и цвет вне формата — по умолчанию.
func (s Style) Clamped() Style {
	known := false
	for _, f := range families {
		if f.ID == s.Font {
			known = true
		}
	}
	if !known {
		s.Font = DefaultFont
	}
	f := family(s.Font)
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
