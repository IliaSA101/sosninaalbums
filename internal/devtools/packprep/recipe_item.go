package packprep

import (
	"errors"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/sheetcut"
)

// allowedFields — какие поля предмета (кроме src, mode, out, theme, family) допустимы в режиме.
// Лишнее поле — отказ: молча проигнорированная правка рецепта хуже ошибки.
var allowedFields = map[string][]string{
	ModeBg:          {"name", "trim"},
	ModeFrame:       {"name", "bg", "thr", "holes"},
	ModeCornerFrame: {"name", "line", "color"},
	ModeQuarters:    {"name", "names", "kind", "style", "bg", "thr", "holes"},
	ModeRows:        {"name", "names", "kind", "style", "bg", "thr", "holes", "n"},
	ModeSheet:       {"names", "kind", "style", "grid", "bg"},
	ModeSvg:         {"names", "kind", "style", "grid", "gap", "side", "knock", "split", "drop"},
}

// setFields — имена заданных (ненулевых) полей предмета, которые зависят от режима.
func (it Item) setFields() []string {
	var s []string
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"name", it.Name != ""}, {"names", len(it.Names) > 0}, {"kind", it.Kind != ""}, {"style", it.Style != ""},
		{"grid", it.Grid != ""}, {"bg", it.BG != ""}, {"thr", it.Thr != 0}, {"holes", it.Holes != ""}, {"n", it.N != 0},
		{"gap", it.Gap != 0}, {"side", it.Side != 0}, {"trim", it.Trim != 0}, {"line", it.Line != ""}, {"color", it.Color != ""},
		{"knock", len(it.Knock) > 0}, {"split", len(it.Split) > 0}, {"drop", len(it.Drop) > 0},
	} {
		if f.set {
			s = append(s, f.name)
		}
	}
	return s
}

func (it Item) validate() error {
	allowed, ok := allowedFields[it.Mode]
	if !ok {
		return fmt.Errorf("mode %q: нужно bg, frame, cornerframe, quarters, rows, sheet или svg", it.Mode)
	}
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	for _, f := range it.setFields() {
		if !slices.Contains(allowed, f) {
			add("поле %q не годится для режима %q", f, it.Mode)
		}
	}
	if err := it.checkSrc(); err != nil {
		errs = append(errs, err)
	}
	if it.Theme != "" && !themeSet[it.Theme] {
		add("theme %q: нужна тема библиотеки", it.Theme)
	}
	if it.Family != "" && !validFamily(it.Family) {
		add("family %q: строчные латиница, цифры и «-», до 40 знаков", it.Family)
	}
	if it.Style != "" && (!runesIn(it.Style, 40) || !cleanText(it.Style)) {
		add("style: до 40 знаков, без управляющих знаков")
	}
	errs = append(errs, it.checkKind()...)
	errs = append(errs, it.checkOut()...)
	errs = append(errs, it.checkNames()...)
	errs = append(errs, it.checkParams()...)
	return errors.Join(errs...)
}

func (it Item) checkSrc() error {
	if it.Src == "" || !filepath.IsLocal(it.Src) || strings.Contains(it.Src, `\`) {
		return fmt.Errorf("src %q: путь внутри -src, с «/», без «..» и абсолютных", it.Src)
	}
	ext := strings.ToLower(path.Ext(it.Src))
	if it.Mode == ModeCornerFrame {
		if _, err := cornerPaths(it.Src); err != nil {
			return err
		}
		return nil
	}
	if it.Mode == ModeSvg {
		if ext != ".svg" {
			return fmt.Errorf("src %q: режиму svg нужен .svg", it.Src)
		}
		return nil
	}
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("src %q: нужен PNG или JPEG", it.Src)
	}
	return nil
}

func (it Item) checkKind() []error {
	switch it.Mode {
	case ModeBg, ModeFrame, ModeCornerFrame:
		return nil // вид задан режимом, kind запрещён общей проверкой
	}
	if it.Kind != "" && !slices.Contains([]string{KindSticker, KindCorner, KindDivider}, it.Kind) {
		return []error{fmt.Errorf("kind %q: нужно sticker, corner или divider", it.Kind)}
	}
	return nil
}

func (it Item) checkOut() []error {
	if it.Out == "" {
		return []error{errors.New("out не задан")}
	}
	switch it.Mode {
	case ModeBg:
		if ext := strings.ToLower(path.Ext(it.Out)); ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return []error{fmt.Errorf("out %q: фон — .jpg или .png", it.Out)}
		}
	case ModeFrame, ModeCornerFrame:
		if path.Ext(it.Out) != ".png" {
			return []error{fmt.Errorf("out %q: рамка — только .png", it.Out)}
		}
	default:
		if strings.HasSuffix(it.Out, "/") {
			return []error{fmt.Errorf("out %q: префикс имён, а не папка", it.Out)}
		}
		if !ValidFileName(it.Out + "01.png") {
			return []error{fmt.Errorf("out %q: префикс не даёт имя по правилам набора", it.Out)}
		}
		return nil
	}
	if !ValidFileName(it.Out) {
		return []error{fmt.Errorf("out %q: имя не по правилам набора (строчные a-z0-9._-, до 3 частей через «/»)", it.Out)}
	}
	return nil
}

func (it Item) checkNames() (errs []error) {
	if !it.multi() {
		if it.Name == "" {
			errs = append(errs, errors.New("name не задан"))
		}
	} else if it.Name != "" && len(it.Names) > 0 {
		errs = append(errs, errors.New("name и names вместе: имена по номерам или основа имени"))
	} else if it.Name == "" && len(it.Names) == 0 {
		errs = append(errs, errors.New("нужны names (по номерам нарезки) или name (основа имени)"))
	} else if it.Name != "" && (it.Mode == ModeSheet || it.Mode == ModeSvg) {
		errs = append(errs, fmt.Errorf("режиму %s нужны names по номерам нарезки", it.Mode))
	}
	for _, n := range append([]string{it.Name}, it.Names...) {
		if n != "" && (!runesIn(n, maxNameRunes) || !cleanText(n)) {
			errs = append(errs, fmt.Errorf("имя %q: до %d знаков, без управляющих знаков и пробелов по краям", n, maxNameRunes))
		}
	}
	for i, n := range it.Names {
		if n == "" {
			errs = append(errs, fmt.Errorf("names[%d] пусто (предмет без имени пропускают через skip)", i))
		}
	}
	return errs
}

func (it Item) checkParams() (errs []error) {
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if it.Grid != "" {
		if _, _, err := sheetcut.ParseGrid(it.Grid); err != nil {
			errs = append(errs, err)
		}
	}
	if !slices.Contains([]string{"", sheetcut.BGAuto, sheetcut.BGNone, sheetcut.BGOwn, sheetcut.BGSeed, sheetcut.BGRembg}, it.BG) {
		add("bg %q: нужно auto, none, own, seed или rembg", it.BG)
	}
	if it.Thr != 0 && (it.Thr < 225 || it.Thr > 248) {
		add("thr %d: нужно 225…248", it.Thr)
	}
	if !slices.Contains([]string{"", sheetcut.HolesClean, sheetcut.HolesAll, sheetcut.HolesNone}, it.Holes) {
		add("holes %q: нужно clean, all или none", it.Holes)
	}
	if it.N < 0 || it.N > 20 {
		add("n %d: рядов от 1 до 20, 0 — auto", it.N)
	}
	if it.Gap < 0 || math.IsNaN(it.Gap) || math.IsInf(it.Gap, 0) {
		add("gap %v: не меньше нуля", it.Gap)
	}
	if it.Side != 0 && (it.Side < 64 || it.Side > sheetcut.MaxSidePx) {
		add("side %d: 64…%d", it.Side, sheetcut.MaxSidePx)
	}
	if it.Line != "" && it.Line != LineNone && it.Line != LineThin {
		add("line %q: нужно none или thin", it.Line)
	}
	if it.Color != "" {
		if _, err := parseHexColor(it.Color); err != nil {
			errs = append(errs, err)
		}
	}
	if it.Trim < 0 || it.Trim > 5 || math.IsNaN(it.Trim) {
		add("trim %v: 0…5 %% стороны", it.Trim)
	}
	for _, k := range it.Knock {
		if math.IsNaN(k[0]) || math.IsNaN(k[1]) || math.IsInf(k[0], 0) || math.IsInf(k[1], 0) {
			add("knock: числа должны быть конечными")
		}
	}
	return errs
}

// validFamily — строчные a-z, цифры и «-», до 40 знаков.
func validFamily(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// ValidFileName — имя файла предмета по правилам набора (pack.ValidName):
// ^[a-z0-9][a-z0-9._-]{0,63}(/[a-z0-9][a-z0-9._-]{0,63}){0,2}$ и без «..». Копия: devtools не
// импортирует internal.
func ValidFileName(name string) bool {
	if name == "" || strings.Contains(name, "..") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 64 || !lowerDigit(p[0]) {
			return false
		}
		for i := 1; i < len(p); i++ {
			if c := p[i]; !lowerDigit(c) && c != '.' && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}

func lowerDigit(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') }
