package assoc

import (
	"errors"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Версии программы и текст «файл новее программы» (098 design §6): версия записавшей программы
// в тексте ошибки открытия. Чистые функции — их зовёт и app (startup.go, files.go).

// NewerText — текст ошибки открытия err для what (первое слово как есть: «альбом» в ошибку для
// фронта — со строчной, как ошибки Go; «Черновик» в начало уведомления): файл новее программы с
// версией из поля app — «альбом сохранён в версии 0.25. Эта программа — 0.23: обновите её»;
// версии в файле нет — «… сохранён более новой версией программы»; не ErrNewer — err.Error().
// mine — версия этой программы. Без точки в конце.
func NewerText(err error, what, mine string) string {
	var ne *salbum.NewerError
	if !errors.As(err, &ne) {
		if errors.Is(err, salbum.ErrNewer) {
			return what + " сохранён более новой версией программы"
		}
		return err.Error()
	}
	theirs, mineShown := shownVers(ne.App, mine)
	switch {
	case theirs == "":
		return what + " сохранён более новой версией программы"
	case mineShown == "":
		return what + " сохранён в версии " + theirs + ": обновите программу"
	}
	return what + " сохранён в версии " + theirs + ". Эта программа — " + mineShown + ": обновите её"
}

// OpenErr — ошибка открытия альбома для ответа фронту: файл новее программы (mine — её версия) —
// с версиями; прочие ошибки как есть.
func OpenErr(err error, mine string) error {
	if errors.Is(err, salbum.ErrNewer) {
		return errors.New(NewerText(err, "альбом", mine))
	}
	return err
}

// shownVers — версии файла и программы для текста: major.minor; если они совпали (файл 0.25.1,
// программа 0.25.0) — три цифры: «сохранён в 0.25, а программа — 0.25» ничего не объясняет.
// Полной версии нет (в файле «0.25») — остаются две.
func shownVers(theirs, mine string) (t, m string) {
	t, m = ShortVer(theirs), ShortVer(mine)
	if t == "" || t != m {
		return t, m
	}
	if tf, mf := fullVer(theirs), fullVer(mine); tf != "" && mf != "" {
		return tf, mf
	}
	return t, m
}

// fullVer — «0.25.1-rc2+abc» → «0.25.1»; не версия (dev, «0.25») — "".
func fullVer(s string) string {
	v, _, ok := splitVer(s)
	if !ok {
		return ""
	}
	return strconv.Itoa(v[0]) + "." + strconv.Itoa(v[1]) + "." + strconv.Itoa(v[2])
}

// ShortVer — «0.25.1-rc2» → «0.25»: в тексте важны major.minor. Не версия (dev, пусто) — "".
func ShortVer(v string) string {
	major, rest, ok := strings.Cut(v, ".")
	if !ok || !digits(major) {
		return ""
	}
	minor := rest
	if i := strings.IndexFunc(rest, func(c rune) bool { return c < '0' || c > '9' }); i >= 0 {
		minor = rest[:i]
	}
	if minor == "" {
		return ""
	}
	return major + "." + minor
}

// digits — непустая строка цифр.
func digits(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }

// splitVer — «0.25.1-rc2+abc» → версия 0.25.1, пред-выпуск true.
func splitVer(s string) (v update.Version, pre string, ok bool) {
	s, _, _ = strings.Cut(s, "+")
	base, pre, _ := strings.Cut(s, "-")
	v, ok = update.ParseVersion(base)
	return v, pre, ok
}

func verKnown(s string) bool { _, _, ok := splitVer(s); return ok }

// cmpVer — -1, 0, 1: a ниже, равна, выше b. Пред-выпуск (-rc2) ниже выпуска; не версия (dev,
// пусто) ниже любой, две не-версии равны.
func cmpVer(a, b string) int {
	va, pa, oka := splitVer(a)
	vb, pb, okb := splitVer(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	if c := va.Compare(vb); c != 0 {
		return c
	}
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	return strings.Compare(pa, pb)
}
