package export

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Unit — одна цель экспорта: разворот Spread (с 1) и Page: 0 — разворот в выбранном виде,
// 1 — левая страница, 2 — правая (записи «7л» / «7п», решение пользователя Q1).
type Unit struct{ Spread, Page int }

const maxRangeLen = 1000

var (
	// \p{Zs}: NBSP и прочие пробелы из мессенджера — в JS их берёт \s, в RE2 \s только ASCII.
	dashRe     = regexp.MustCompile(`[\s\p{Zs}]*[-–—][\s\p{Zs}]*`)
	rangeSepRe = regexp.MustCompile(`[,;\s\p{Zs}]+`)
	spanRe     = regexp.MustCompile(`^(\d{1,6})(?:-(\d{1,6}))?$`)
	pageRe     = regexp.MustCompile(`^(\d{1,6})([лпlr])$`)
	pageSpanRe = regexp.MustCompile(`^\d{1,6}[лпlr]?-\d{1,6}[лпlr]?$`)
	loneRe     = regexp.MustCompile(`^[лпlr]$`)
)

// ParseRange разбирает диапазон окна экспорта («3-5, 8, 7п») в цели по возрастанию без повторов;
// n — число разворотов. Пустая строка — ошибка: «все» обозначает ExportOptions.Range == "" и
// до сюда не доходит. UI проверяет то же поле через export/exportrange.ts; примеры общие —
// testdata/parity/ranges.json. Буква страницы бывает только у одиночного номера, слитно: «7п».
func ParseRange(s string, n int) ([]Unit, error) {
	if n < 1 {
		return nil, errors.New("нет разворотов")
	}
	if utf8.RuneCountInString(s) > maxRangeLen {
		return nil, errors.New("диапазон слишком длинный")
	}
	src := strings.TrimSpace(dashRe.ReplaceAllString(s, "-"))
	if src == "" {
		return nil, errors.New("укажите номера разворотов")
	}
	set := map[Unit]bool{}
	for _, tok := range rangeSepRe.Split(src, -1) {
		if tok == "" {
			continue
		}
		units, err := parseToken(tok, n)
		if err != nil {
			return nil, err
		}
		for _, u := range units {
			set[u] = true
		}
	}
	if len(set) == 0 {
		return nil, errors.New("укажите номера разворотов") // одни запятые
	}
	out := make([]Unit, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	slices.SortFunc(out, func(x, y Unit) int {
		if x.Spread != y.Spread {
			return x.Spread - y.Spread
		}
		return x.Page - y.Page
	})
	return out, nil
}

// parseToken разбирает одну запись диапазона: «7п» (страница), «3» или «3-5» (развороты).
func parseToken(tok string, n int) ([]Unit, error) {
	low := strings.ToLower(tok)
	if loneRe.MatchString(low) {
		return nil, errors.New("номер страницы пишется слитно: 7п")
	}
	if m := pageRe.FindStringSubmatch(low); m != nil {
		num, _ := strconv.Atoi(m[1])
		if err := checkSpread(num, n); err != nil {
			return nil, err
		}
		return []Unit{{num, pageIndex(m[2])}}, nil
	}
	if pageSpanRe.MatchString(low) && strings.ContainsAny(low, "лпlr") {
		return nil, fmt.Errorf("страницу можно указать только у одного разворота: 7п (не «%s»)", tok)
	}
	m := spanRe.FindStringSubmatch(low)
	if m == nil {
		return nil, fmt.Errorf("не понимаю «%s»", tok)
	}
	a, _ := strconv.Atoi(m[1])
	b := a
	if m[2] != "" {
		b, _ = strconv.Atoi(m[2])
	}
	if err := checkSpread(a, n); err != nil {
		return nil, err
	}
	if a > b {
		return nil, fmt.Errorf("диапазон «%s» задан наоборот", tok)
	}
	if err := checkSpread(b, n); err != nil {
		return nil, err
	}
	out := make([]Unit, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, Unit{i, 0})
	}
	return out, nil
}

// ErrSheetPages — «л»/«п» в диапазоне проекта-листа (031): у листа страниц нет.
var ErrSheetPages = errors.New("в листе нет страниц: укажите номера листов без «л» и «п»")

// CheckSheet — цели проекта-листа: страница («7п») — ErrSheetPages.
func CheckSheet(units []Unit) error {
	if slices.ContainsFunc(units, func(u Unit) bool { return u.Page != 0 }) {
		return ErrSheetPages
	}
	return nil
}

// pageIndex: «л»/«l» — левая страница (1), «п»/«r» — правая (2).
func pageIndex(letter string) int {
	if letter == "л" || letter == "l" {
		return 1
	}
	return 2
}

func checkSpread(num, n int) error {
	switch {
	case num < 1:
		return errors.New("номера начинаются с 1")
	case num > n:
		return fmt.Errorf("разворота %d нет (всего %d)", num, n)
	}
	return nil
}
