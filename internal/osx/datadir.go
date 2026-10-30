package osx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppName — имя папки данных программы (настройки, кэш миниатюр, WebView2, копии).
const AppName = "SosninaAlbums"

// StandEnv — режим стенда QA: несколько копий wails dev на одной машине, у каждой свои
// настройки, кэш, WebView2 и замок второго экземпляра (иначе они делят config.json и черновик).
const StandEnv = "SA_STAND"

// standMax — предел длины имени стенда: имя входит в имена папок и замка.
const standMax = 16

// Stand — имя стенда из SA_STAND; "" — обычный запуск. Имя — только латиница и цифры: оно
// входит в путь папки данных, а ..\, разделители или «C:» вывели бы из неё. Регистр сводится
// к нижнему: на Windows «B» и «b» — одна папка, замок должен совпадать с ней.
// Только в сборке с тегом dev (standEnabled); в выпуске всегда "" и переменная не читается.
func Stand() (string, error) {
	return standFrom(standEnabled, os.Getenv)
}

// standFrom — тело Stand с подставляемыми enabled и getenv: проверяется в тестах без тега dev.
func standFrom(enabled bool, getenv func(string) string) (string, error) {
	if !enabled {
		return "", nil
	}
	return ParseStand(getenv(StandEnv))
}

// ParseStand — проверка имени стенда (см. Stand).
func ParseStand(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > standMax {
		return "", fmt.Errorf("%s: имя стенда длиннее %d знаков", StandEnv, standMax)
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("%s=%q: имя стенда — только латиница и цифры", StandEnv, s)
		}
	}
	return strings.ToLower(s), nil
}

// DataName — имя папки данных: у стенда своё рядом с основной, не внутри неё (стенд не
// трогает настоящие настройки, и его можно удалить целиком). stand уже проверен ParseStand.
func DataName(stand string) string {
	if stand == "" {
		return AppName
	}
	return AppName + "-stand-" + stand
}

// WebViewDir — папка данных WebView2: %LOCALAPPDATA%\<name>\WebView2 (name — DataName). Без неё
// WebView2 кладёт данные в %AppData%\<имя exe>, а exe называется по версии, и каждая
// версия теряла бы localStorage. Local, а не Roaming: кэш большой. "" — пусть решает Wails.
func WebViewDir(name string) string {
	base := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(base) { // относительный путь положил бы данные WebView2 в текущую папку
		base = ""
	}
	if base == "" {
		base = CacheDir()
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, name, "WebView2")
}
