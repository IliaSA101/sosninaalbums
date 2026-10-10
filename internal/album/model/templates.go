package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

const (
	// TemplatesVersion — версия templates.json; больше — только чтение. Пишется
	// наименьшая, понимающая все поля файла (MarshalTemplates): 2 — только при угле.
	TemplatesVersion = 2
	// MaxTemplates — шаблонов в файле.
	MaxTemplates = 200
	// MaxTemplateName — знаков в имени.
	MaxTemplateName = 60
	// MaxTemplatesFile — больше 1 МБ файл считаем битым.
	MaxTemplatesFile = 1 << 20
	// MinBoxFrac — меньшая сторона места в долях области.
	MinBoxFrac = 0.01
)

// Ошибки шаблонов (текст уходит в тост).
var (
	ErrTemplatesNewer = errors.New("шаблоны сохранены более новой версией программы")
	ErrNoPlaces       = errors.New("нет мест")
	ErrTooManyTpl     = fmt.Errorf("не больше %d шаблонов", MaxTemplates)
)

// Template — сохранённая сетка: места в долях области Scope, порядок = порядок мест.
type Template struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Scope string `json:"scope"` // TplPage | TplSpread
	Boxes []Box  `json:"boxes"`
}

type templatesFile struct {
	Version   int        `json:"version"`
	Templates []Template `json:"templates"`
}

// ParseTemplates разбирает templates.json: список после зажима и версия файла.
// Больше MaxTemplatesFile или битый JSON — ошибка (вызывающий считает список
// пустым и не затирает файл). Версия новее TemplatesVersion читается так же:
// что понятно — берём, запись вызывающий запрещает.
func ParseTemplates(data []byte) ([]Template, int, error) {
	if len(data) > MaxTemplatesFile {
		return nil, 0, errors.New("templates.json больше 1 МБ")
	}
	// Сначала только версия: тело новой версии может быть устроено иначе, и это
	// не «битый файл», а «новее программы» (его не затирают, запись запрещена).
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, 0, err
	}
	if head.Version < 1 {
		return nil, 0, errors.New("templates.json без версии")
	}
	var f templatesFile
	if err := json.Unmarshal(data, &f); err != nil {
		if head.Version > TemplatesVersion {
			return nil, head.Version, nil
		}
		return nil, 0, err
	}
	return ClampTemplates(f.Templates), head.Version, nil
}

// MarshalTemplates — содержимое templates.json (после ClampTemplates NaN не бывает).
// Версия — наименьшая, понимающая все поля: без углов 1, чтобы 0.3.0/0.4.0
// продолжали править файл (v2 они только читают).
func MarshalTemplates(list []Template) ([]byte, error) {
	if list == nil {
		list = []Template{}
	}
	ver := 1
	if slices.ContainsFunc(list, func(t Template) bool {
		return slices.ContainsFunc(t.Boxes, func(b Box) bool { return b.Rot != 0 })
	}) {
		ver = 2
	}
	return json.MarshalIndent(templatesFile{Version: ver, Templates: list}, "", "  ")
}

// ClampTemplates — зажим недоверенного списка (файл, интерфейс): область не из
// page/spread, шаблон без мест и сверх MaxTemplates выбрасываются; доли — ClampBoxes;
// имя — CleanTemplateName; плохой или повторный ID — новый. Вход не меняется.
func ClampTemplates(in []Template) []Template {
	out := make([]Template, 0, min(len(in), MaxTemplates))
	seen := map[string]bool{}
	for _, t := range in {
		if len(out) >= MaxTemplates {
			break
		}
		if t.Scope != TplPage && t.Scope != TplSpread {
			continue
		}
		boxes := ClampBoxes(t.Boxes)
		if len(boxes) == 0 {
			continue
		}
		if !IsTextID(t.ID) || seen[t.ID] {
			t.ID = NewID()
			for seen[t.ID] {
				t.ID = NewID()
			}
		}
		seen[t.ID] = true
		out = append(out, Template{ID: t.ID, Name: CleanTemplateName(t.Name, len(out)+1), Scope: t.Scope, Boxes: boxes})
	}
	return out
}

// ClampBoxes — доли места в 0..1: NaN/Inf выбрасывает место, W, H ≥ minBoxFrac,
// место целиком внутри области; не больше MaxPlaces мест. Rot — geom.NormDeg; у
// повёрнутого внутри только центр: положение в мм зажмёт clampPlace (hold) —
// иначе копия места у края под углом съезжала бы внутрь.
func ClampBoxes(in []Box) []Box {
	out := make([]Box, 0, min(len(in), MaxPlaces))
	for _, b := range in {
		if len(out) >= MaxPlaces {
			break
		}
		if !finiteBox(b) {
			continue
		}
		b.W = geom.ClampF(b.W, MinBoxFrac, 1)
		b.H = geom.ClampF(b.H, MinBoxFrac, 1)
		if b.Rot = geom.NormDeg(b.Rot); b.Rot == 0 {
			b.X = geom.ClampF(b.X, 0, 1-b.W)
			b.Y = geom.ClampF(b.Y, 0, 1-b.H)
		} else {
			b.X = geom.ClampF(b.X, -b.W/2, 1-b.W/2)
			b.Y = geom.ClampF(b.Y, -b.H/2, 1-b.H/2)
		}
		out = append(out, b)
	}
	return out
}

func finiteBox(b Box) bool {
	for _, v := range [...]float64{b.X, b.Y, b.W, b.H, b.Rot} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// CleanTemplateName — имя без управляющих знаков и лишних пробелов, ≤ MaxTemplateName;
// пустое — «Шаблон N».
func CleanTemplateName(s string, n int) string {
	s = strings.Join(strings.Fields(CleanText(s, 4*MaxTemplateName)), " ")
	if utf8.RuneCountInString(s) > MaxTemplateName {
		s = strings.TrimSpace(string([]rune(s)[:MaxTemplateName]))
	}
	if s == "" {
		return fmt.Sprintf("Шаблон %d", n)
	}
	return s
}

// NoPlacesError — ErrNoPlaces с текстом по области; errors.Is(…, ErrNoPlaces) верно.
type NoPlacesError string

func (e NoPlacesError) Error() string { return string(e) }

func (NoPlacesError) Unwrap() error { return ErrNoPlaces }
