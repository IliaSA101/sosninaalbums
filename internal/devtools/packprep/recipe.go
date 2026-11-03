// Package packprep собирает папку набора библиотеки по рецепту: нарезает листы (sheetcut, svgcut),
// готовит фоны (свой Lanczos), пишет manifest.src.json и лист проверки _check/ с report.json.
// Инструмент разработчика (спека 129, design §2.3–§2.4), не функция программы: stdlib и devtools.
// Запуск — scripts/packprep.go.
package packprep

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
)

// Режимы предмета рецепта (Item.Mode).
const (
	ModeBg          = "bg"          // фон: байт в байт или Lanczos до 3602 (разворот — 7205×3602), JPEG q92
	ModeFrame       = "frame"       // рамка: sheetcut.ModeFrame
	ModeCornerFrame = "cornerframe" // рамка из четырёх уголков набора (cornerframe.go)
	ModeQuarters    = "quarters"    // уголки 2×2: sheetcut.ModeQuarters
	ModeRows        = "rows"        // разделители: sheetcut.ModeRows
	ModeSheet       = "sheet"       // лист стикеров с белым фоном (NB2): sheetcut, режим листа
	ModeSvg         = "svg"         // SVG Recraft: svgcut
)

// Виды предметов для бюджета и листа проверки; в манифесте corner и divider — sticker.
const (
	KindBg      = "bg"
	KindSticker = "sticker"
	KindFrame   = "frame"
	KindCorner  = "corner"
	KindDivider = "divider"
)

// Пределы рецепта.
const (
	MaxRecipeBytes = 1 << 20 // рецепт больше — не рецепт
	maxRecipeItems = 200
	maxNameRunes   = 60  // pack.Clean(name, 60)
	maxAboutRunes  = 140 // спека 129: описание в две строки окна «Наборы» (0.27-129-5)
	maxTitleRunes  = 60
	maxAuthorRunes = 60
	maxSourceRunes = 200
	maxFamilyCap   = 2 // предметов-источников одного семейства (0.27-129-19)
)

var (
	idRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,39}$`)
	verRe    = regexp.MustCompile(`^\d{1,4}\.\d{1,4}\.\d{1,4}$`)
	themeSet = map[string]bool{"basic": true, "wedding": true, "kids": true, "kindergarten": true, "school": true, "grad": true, "holiday": true}
)

// Recipe — рецепт набора: шапка манифеста и предметы (design §2.3). Неизвестное поле — отказ.
type Recipe struct {
	ID      string            `json:"id"`
	Version int               `json:"version"`
	MinApp  string            `json:"minApp"`
	Title   string            `json:"title"`
	Theme   string            `json:"theme"`
	Author  string            `json:"author"`
	License string            `json:"license"`
	Source  string            `json:"source"`
	About   string            `json:"about"`
	Needs   string            `json:"needs,omitempty"` // id набора-донора (дополнение, 0.27-129-12)
	Items   []Item            `json:"items"`
	Skip    map[string]string `json:"skip,omitempty"` // «src» или «src#N» → причина брака
}

// Item — один источник рецепта: файл и способ его обработки. Лишние для режима поля — отказ.
type Item struct {
	Src    string   `json:"src"`              // относительно -src, только внутри
	Mode   string   `json:"mode"`             // Mode*
	Kind   string   `json:"kind,omitempty"`   // вид: по умолчанию из режима; sheet, svg, rows, quarters — sticker, corner, divider
	Theme  string   `json:"theme,omitempty"`  // тема предмета; по умолчанию тема набора
	Style  string   `json:"style,omitempty"`  // стиль стикеров («Значки», «Рисунки»): бюджет ≤ 36 на стиль
	Family string   `json:"family,omitempty"` // семейство; по умолчанию из имени файла (familyOf)
	Out    string   `json:"out"`              // файл (bg, frame) или префикс имён (остальные), внутри папки набора
	Name   string   `json:"name,omitempty"`   // имя предмета; у многоштучных — основа имени
	Names  []string `json:"names,omitempty"`  // имена по номерам нарезки (sheet, svg, rows, quarters)

	Grid  string  `json:"grid,omitempty"`  // sheet, svg: «КОЛОНКИxРЯДЫ»
	BG    string  `json:"bg,omitempty"`    // sheet, frame, quarters, rows: режим снятия фона (sheetcut.BG*); пусто — auto
	Thr   int     `json:"thr,omitempty"`   // frame, quarters, rows: «почти белый» вручную, 225…248
	Holes string  `json:"holes,omitempty"` // frame, quarters, rows: clean, all, none
	N     int     `json:"n,omitempty"`     // rows: число рядов; 0 — auto
	Gap   float64 `json:"gap,omitempty"`   // svg: пути ближе — один предмет; 0 — умолчание svgcut
	Side  int     `json:"side,omitempty"`  // svg: длинная сторона предмета, px
	Trim  float64 `json:"trim,omitempty"`  // bg: срезать по краям, % стороны, 0…5
	Line  string  `json:"line,omitempty"`  // cornerframe: стороны — none (пусто, по умолчанию) или thin (линия)
	Color string  `json:"color,omitempty"` // cornerframe: цвет линии «#rrggbb»; пусто — доминирующий цвет уголков

	Knock [][2]float64       `json:"knock,omitempty"` // svg: точки вырезов в координатах листа
	Split []svgcut.SplitLine `json:"split,omitempty"` // svg: резать склеенные группы
	Drop  []int              `json:"drop,omitempty"`  // svg: снять группы
}

// outKind — вид предмета рецепта: заданный или по режиму.
func (it Item) outKind() string {
	if it.Kind != "" {
		return it.Kind
	}
	switch it.Mode {
	case ModeBg:
		return KindBg
	case ModeFrame, ModeCornerFrame:
		return KindFrame
	case ModeQuarters:
		return KindCorner
	case ModeRows:
		return KindDivider
	}
	return KindSticker
}

// multi — режим даёт несколько файлов (префикс out, имена по номерам).
func (it Item) multi() bool {
	return it.Mode != ModeBg && it.Mode != ModeFrame && it.Mode != ModeCornerFrame
}

// ParseRecipe разбирает и проверяет рецепт: размер до разбора, неизвестные поля и данные после
// объекта — отказ. Пути и дубли имён проверяет Run (нужны файлы), здесь — всё, что видно из рецепта.
func ParseRecipe(data []byte) (*Recipe, error) {
	if len(data) > MaxRecipeBytes {
		return nil, fmt.Errorf("packprep: рецепт %d КБ больше предела %d КБ", len(data)>>10, MaxRecipeBytes>>10)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r Recipe
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("packprep: рецепт: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("packprep: рецепт: после объекта есть лишние данные")
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

func runesIn(s string, n int) bool { return utf8.RuneCountInString(s) <= n }

// cleanText — без управляющих знаков и пробелов по краям.
func cleanText(s string) bool {
	if strings.TrimSpace(s) != s {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsControl)
}

func (r *Recipe) validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf("packprep: "+format, a...)) }
	switch {
	case !idRe.MatchString(r.ID):
		add("id %q не по грамматике ^[a-z0-9][a-z0-9-]{2,39}$", r.ID)
	case r.Version < 1 || r.Version > 999999:
		add("version %d: нужно 1…999999", r.Version)
	}
	if !verRe.MatchString(r.MinApp) {
		add("minApp %q: нужно x.y.z", r.MinApp)
	}
	if !themeSet[r.Theme] {
		add("theme %q: нужна тема библиотеки (basic, wedding, kids, kindergarten, school, grad, holiday)", r.Theme)
	}
	if r.License != "CC0-1.0" {
		add("license %q: наборы библиотеки выпускаются только как CC0-1.0", r.License)
	}
	for _, f := range []struct {
		name, v string
		n       int
		must    bool
	}{{"title", r.Title, maxTitleRunes, true}, {"author", r.Author, maxAuthorRunes, true}, {"source", r.Source, maxSourceRunes, true}, {"about", r.About, maxAboutRunes, true}} {
		switch {
		case f.must && f.v == "":
			add("%s не задан", f.name)
		case !runesIn(f.v, f.n) || !cleanText(f.v):
			add("%s: до %d знаков, без управляющих знаков и пробелов по краям", f.name, f.n)
		}
	}
	if r.Needs != "" && (!idRe.MatchString(r.Needs) || r.Needs == r.ID) {
		add("needs %q: id другого набора по грамматике", r.Needs)
	}
	if len(r.Items) == 0 || len(r.Items) > maxRecipeItems {
		add("items: от 1 до %d", maxRecipeItems)
	}
	for i := range r.Items {
		if err := r.Items[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("packprep: items[%d] %q: %w", i, r.Items[i].Src, err))
		}
	}
	errs = append(errs, r.checkSkip()...)
	return errors.Join(errs...)
}

// checkSkip — ключи skip ссылаются на src рецепта (или «src#N»), причина не пуста.
func (r *Recipe) checkSkip() (errs []error) {
	srcs := map[string]bool{}
	for _, it := range r.Items {
		srcs[it.Src] = true
	}
	for k, why := range r.Skip {
		src := k
		if i := strings.LastIndexByte(k, '#'); i >= 0 {
			src = k[:i]
			if n := parseIndex(k[i+1:]); n < 1 || !r.multiSrc(src) {
				errs = append(errs, fmt.Errorf("packprep: skip %q: «#N» — номер предмета (с 1) у многоштучного источника", k))
			}
		}
		if !srcs[src] {
			errs = append(errs, fmt.Errorf("packprep: skip %q: такого src в рецепте нет", k))
		}
		if strings.TrimSpace(why) == "" {
			errs = append(errs, fmt.Errorf("packprep: skip %q: причина не задана", k))
		}
	}
	return errs
}

func (r *Recipe) multiSrc(src string) bool {
	for _, it := range r.Items {
		if it.Src == src && it.multi() {
			return true
		}
	}
	return false
}

// parseIndex — натуральное число из s; иначе 0.
func parseIndex(s string) int {
	n := 0
	if s == "" || len(s) > 6 {
		return 0
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
