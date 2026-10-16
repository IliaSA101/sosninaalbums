package paths

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FolderMaxRunes — предел длины имени папки из FolderName: запас до MAX_PATH Windows.
const FolderMaxRunes = 64

// FolderName — имя папки из строки пользователя (имени файла альбома, 047 §4.2): запретные
// для Windows знаки, управляющие и битые руны — пробел, невидимые (Cf: U+200B, RLO…) убираются,
// пробелы схлопываются, не длиннее FolderMaxRunes, точки и пробелы по краям срезаются
// (Windows их отбрасывает сама). Пусто, не локальное имя или устройство (CON, NUL, COM¹,
// CONIN$… — и с расширением: «con.txt» Windows 10 считает устройством, filepath.IsLocal
// его пропускает) — fallback. Идемпотентна при годном fallback.
func FolderName(s, fallback string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == utf8.RuneError || unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteByte(' ')
		case unicode.Is(unicode.Cf, r):
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(name); len(r) > FolderMaxRunes {
		name = string(r[:FolderMaxRunes])
	}
	name = strings.Trim(name, ". ")
	if name == "" || !filepath.IsLocal(name) || reservedName(name) {
		return fallback
	}
	return name
}

// reservedName — основа имени до первой точки (без пробелов, регистр не важен) — имя
// устройства Windows.
func reservedName(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	switch base = strings.ToUpper(strings.TrimSpace(base)); base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if r := []rune(base); len(r) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return strings.ContainsRune("0123456789¹²³", r[3])
	}
	return false
}
