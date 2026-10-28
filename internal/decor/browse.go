package decor

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Разметка вида и стиля стикеров и «Новые» (спека 133 ступень 2, design §4–§5). Манифест набора
// полей form/style не получает (0.27-129-109 б): вид и стиль Go выводит из имени и размера реестра.

// Виды стикера (Item.Form); "" — обычный стикер.
const (
	FormCorner  = "corner"
	FormDivider = "divider"
)

// ThemeNew — «Новые»: выборка предметов свежих наборов, как recent. Своё в неё не кладётся.
const ThemeNew = "new"

// Правила «Новых» (решение 0.28-133-10).
const (
	newWindowMs = int64(30 * 24 * time.Hour / time.Millisecond) // свежий набор — не старше 30 суток
	firstBatch  = int64(24 * time.Hour / time.Millisecond)      // всё, что встало в первые сутки, — первая партия
	// minInstalled — 2025-01-01 UTC: что раньше (в том числе 0 после сброса часов) — не дата установки.
	minInstalled = int64(1735689600000)
	// Ширина стиля «X: Y» — число рун X.
	styleMin, styleMax = 2, 20
	// Разделитель — сторона не меньше чем в 4 раза длиннее другой (≥: «Гирлянда» 2800×700 — ровно 4:1).
	dividerRatio = 4
	maxSide      = 1 << 30 // реестр зажимает размер раньше; тут — чтобы умножение на 4 не переполнялось
)

// formOf — вид стикера набора или встроенного: разделитель по пропорции (w, h из реестра; у
// встроенных 0 — по пропорции не станет), уголок — слово «уголок» или слово на «уголк». Свои не
// размечаются (вызывающий не зовёт): широкий логотип не должен уйти в «Разделители».
func formOf(name string, w, h int) string {
	if w > 0 && h > 0 && w <= maxSide && h <= maxSide && (w >= dividerRatio*h || h >= dividerRatio*w) {
		return FormDivider
	}
	// \b в RE2 — только ASCII, поэтому слова — по unicode.IsLetter.
	for _, word := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if word == "уголок" || strings.HasPrefix(word, "уголк") {
			return FormCorner
		}
	}
	return ""
}

// styleOf — стиль «X» из имени «X: Y» (X — 2…20 рун без цифр, после двоеточия есть текст); иначе "".
// Берётся только у обычных стикеров (formOf == ""): имя разделителя «Разделитель: яблоко» — не стиль.
func styleOf(name string) string {
	x, y, ok := strings.Cut(name, ": ")
	if !ok || strings.TrimSpace(y) == "" {
		return ""
	}
	if n := utf8.RuneCountInString(x); n < styleMin || n > styleMax {
		return ""
	}
	if strings.ContainsFunc(x, unicode.IsDigit) {
		return ""
	}
	return x
}

// markForm ставит Form и Style стикеру набора или встроенному; другим видам и своему — ничего.
func markForm(it *Item, kind string) {
	if kind != KindSticker || it.Own {
		return
	}
	it.Form = formOf(it.Name, it.W, it.H)
	if it.Form == "" {
		it.Style = styleOf(it.Name)
	}
}

// dateOK — t похоже на дату установки (0 и сброшенные часы — нет).
func dateOK(t int64) bool { return t >= minInstalled }

// packBase — граница первой партии: самое раннее из Base и Installed стоящих наборов, кроме skip и
// примера темы (первая партия нового пользователя — каталог, а не пример, 0.30-142-20);
// false — дат нет. Под mu.
func (s *Store) packBase(skip *packReg) (int64, bool) {
	var base int64
	found := false
	for _, r := range s.packs {
		if r == skip || s.isDemo(r) {
			continue
		}
		for _, t := range [2]int64{r.Base, r.Installed} {
			if dateOK(t) && (!found || t < base) {
				base, found = t, true
			}
		}
	}
	return base, found
}

// freshPacks — id наборов, которые считаются новыми в момент now (design §4). Под mu.
func (s *Store) freshPacks(now int64) map[string]bool {
	base, found := s.packBase(nil)
	if !found {
		return nil
	}
	out := map[string]bool{}
	for _, r := range s.packs {
		if !dateOK(r.Installed) {
			continue
		}
		at := min(r.Installed, now) // дата из будущего — «сейчас»
		if at-base > firstBatch && now-at <= newWindowMs {
			out[r.ID] = true
		}
	}
	return out
}

// newIDs — ID плиток свежих наборов в порядке каталога (пусто — [], не nil).
func newIDs(items []Item, fresh map[string]bool) []string {
	out := []string{}
	if len(fresh) == 0 {
		return out
	}
	for _, it := range items {
		if it.PackID != "" && fresh[it.PackID] {
			out = append(out, it.ID)
		}
	}
	return out
}
