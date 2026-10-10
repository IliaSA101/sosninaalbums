package paths

import (
	"path/filepath"
	"slices"
	"strings"
)

// SplitPath режет путь на элементы по любому из разделителей: старый путь мог
// быть записан и на другой ОС.
func SplitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
}

// splitVol делит путь на том («E:») и элементы без разделителей.
func splitVol(p string) (string, []string) {
	vol := filepath.VolumeName(p)
	return vol, SplitPath(p[len(vol):])
}

func joinVol(vol string, el []string) string {
	return filepath.Join(append([]string{vol + string(filepath.Separator)}, el...)...)
}

// PrefixMap находит, какой префикс пути переехал: общий хвост элементов (без
// учёта регистра) отрезается, остатки — пара «было → стало». E:\Съёмки\Ивановы\Отбор\1.jpg →
// F:\Ивановы\Отбор\1.jpg даёт E:\Съёмки → F:\. Нет общего хвоста (файл
// переименован), префиксы совпали или «стало» не локальный путь — ok = false.
func PrefixMap(oldPath, newPath string) (from, to string, ok bool) {
	ov, oe := splitVol(oldPath)
	nv, ne := splitVol(newPath)
	tail := 0
	for tail < len(oe) && tail < len(ne) && strings.EqualFold(oe[len(oe)-1-tail], ne[len(ne)-1-tail]) {
		tail++
	}
	if tail == 0 {
		return "", "", false
	}
	from, to = joinVol(ov, oe[:len(oe)-tail]), joinVol(nv, ne[:len(ne)-tail])
	if SamePath(from, to) || CheckLocalPath(to) != nil {
		return "", "", false
	}
	return from, to, true
}

// ReplacePrefix заменяет в dir префикс from на to; dir должна лежать в from
// (или быть ею) по элементам, без учёта регистра. Нелокальный результат (to — UNC,
// устройство) — ok = false: перепривязка не должна уводить на сетевую шару.
func ReplacePrefix(dir, from, to string) (string, bool) {
	dv, de := splitVol(dir)
	fv, fe := splitVol(from)
	tv, te := splitVol(to)
	if !strings.EqualFold(dv, fv) || len(fe) > len(de) {
		return "", false
	}
	for i, el := range fe {
		if !strings.EqualFold(el, de[i]) {
			return "", false
		}
	}
	res := joinVol(tv, append(slices.Clone(te), de[len(fe):]...))
	if CheckLocalPath(res) != nil {
		return "", false
	}
	return res, true
}

// DriveLetter приводит «e», «E» и «E:» к «E»; не буква — ok = false.
func DriveLetter(s string) (string, bool) {
	s = strings.TrimSuffix(s, ":")
	if len(s) == 1 && (s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z') {
		return strings.ToUpper(s), true
	}
	return "", false
}

// SwapDrive меняет букву диска в пути с from на to; путь на другом диске, to не
// буква диска или нелокальный результат — ok = false.
func SwapDrive(path, from, to string) (string, bool) {
	letter, ok := DriveLetter(to)
	if !ok {
		return "", false
	}
	vol := filepath.VolumeName(path)
	if len(vol) != 2 || vol[1] != ':' || !strings.EqualFold(vol[:1], from) {
		return "", false
	}
	res := letter + path[1:]
	if CheckLocalPath(res) != nil {
		return "", false
	}
	return res, true
}
