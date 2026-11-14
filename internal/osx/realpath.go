package osx

import "strings"

// RealPath — настоящий путь существующей папки: подставленный диск (subst X: C:\Users\u) и
// отображённая папка раскрыты до исходного тома. filepath.EvalSymlinks их не раскрывает (это не
// ссылки файловой системы), и правила скрытого и домашней папки смотрели бы на введённое `X:\.ssh`
// (аудит 0.34, М4). Вне Windows путь возвращается как есть: подставных дисков там нет.
func RealPath(p string) (string, error) { return realPath(p) }

// trimFinalPrefix снимает префикс «\\?\» с пути GetFinalPathNameByHandle: `\\?\C:\x` → `C:\x`,
// `\\?\UNC\srv\sh` → `\\srv\sh`. Прочее (в том числе том без буквы) — как есть. Вынесено без привязки к ОС ради таблицы теста.
func trimFinalPrefix(s string) string {
	switch {
	case strings.HasPrefix(s, `\\?\UNC\`):
		return `\\` + s[len(`\\?\UNC\`):]
	case strings.HasPrefix(s, `\\?\`) && (len(s) == 4 || len(s) >= 6 && s[5] == ':'):
		return s[len(`\\?\`):] // диск с буквой; `\\?\Volume{…}` без буквы остаётся как есть и правил не пройдёт
	}
	return s
}
