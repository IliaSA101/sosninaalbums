package osx

// «Документы», «Рабочий стол», «Загрузки» пользователя (149, design §2.3): такие папки целиком
// нельзя назвать папкой съёмки, подпапки — можно. Адреса берутся у ОС, а не по именам: на русской
// системе это «~/Документы», на Windows они могут лежать на другом диске.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxUserDirsFile — предел чтения user-dirs.dirs: обычный файл — несколько строк.
const maxUserDirsFile = 64 << 10

// xdgKeys — ключи user-dirs.dirs, которые нам нужны.
var xdgKeys = []string{"XDG_DOCUMENTS_DIR", "XDG_DESKTOP_DIR", "XDG_DOWNLOAD_DIR"}

// plainNames — имена папок без локализации: запасной список на любой ОС (Mac — единственный).
var plainNames = []string{"Documents", "Desktop", "Downloads"}

// UserDirs — абсолютные пути «Документы», «Рабочий стол», «Загрузки» без повторов. Windows —
// известные папки (SHGetKnownFolderPath), Linux — XDG user-dirs.dirs; всегда ещё ~/Documents,
// ~/Desktop, ~/Downloads (Mac и ОС без файла). Домашней папки нет — только то, что дала ОС.
func UserDirs() []string {
	home, _ := os.UserHomeDir()
	return userDirsIn(home, ConfigDir(), knownDirs())
}

// userDirsIn — UserDirs от домашней папки home, папки настроек cfg (в ней user-dirs.dirs) и
// адресов known, которые дала ОС. Относительные и пустые пути отбрасываются.
func userDirsIn(home, cfg string, known []string) []string {
	var out []string
	add := func(p string) {
		if p = filepath.Clean(p); filepath.IsAbs(p) && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, p := range known {
		add(p)
	}
	if home != "" && filepath.IsAbs(home) {
		if cfg != "" {
			for _, p := range readXDGDirs(filepath.Join(cfg, "user-dirs.dirs"), home) {
				add(p)
			}
		}
		for _, n := range plainNames {
			add(filepath.Join(home, n))
		}
	}
	return out
}

// readXDGDirs — пути из файла user-dirs.dirs; нет файла, он не читается или больше предела — пусто.
func readXDGDirs(file, home string) []string {
	f, err := os.Open(file) //nolint:gosec // G304: файл настроек пользователя в его папке настроек
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(f, maxUserDirsFile+1))
	if err != nil || len(data) > maxUserDirsFile {
		return nil
	}
	return parseXDGDirs(data, home)
}

// parseXDGDirs — значения XDG_DOCUMENTS_DIR, XDG_DESKTOP_DIR, XDG_DOWNLOAD_DIR: строки вида
// KEY="$HOME/Папка" (комментарии и прочие ключи пропускаются). Допустимо только «$HOME» в начале
// или абсолютный путь; всё остальное (подстановки, относительные пути, пустое) — пропуск.
func parseXDGDirs(data []byte, home string) []string {
	var out []string
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		key, val, ok := strings.Cut(strings.TrimSpace(string(line)), "=")
		if !ok || !slices.Contains(xdgKeys, strings.TrimSpace(key)) {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) < 2 || val[0] != '"' || val[len(val)-1] != '"' {
			continue
		}
		val = val[1 : len(val)-1]
		switch {
		case val == "$HOME" || strings.HasPrefix(val, "$HOME/"):
			val = home + val[len("$HOME"):]
		case strings.ContainsAny(val, "$`\\"):
			continue // другая подстановка или экранирование: не разбираем
		}
		if filepath.IsAbs(val) {
			out = append(out, val)
		}
	}
	return out
}
