package osx

import (
	"path/filepath"
	"strings"
)

// ExeHint — предупреждение про путь программы: "" — путь годится. Путь бывает временным: Mac —
// App Translocation (новый при каждом запуске, если .app не в «Программах»), Windows — exe из zip
// без распаковки (под папкой Temp), прочее — exe удалён или заменён (regular сообщает, что на
// этом месте обычный файл). Настройка агента или запуск посредником с таким путём сломаются при
// следующем запуске. Одно правило для окна «ИИ-агенты» и для посредника (start_program, 149 §5).
func ExeHint(exe, goos, tmpDir string, regular func(path string) bool) string {
	dead := exe == "" || !filepath.IsAbs(exe)
	if !dead {
		switch {
		case goos == "darwin" && strings.Contains(exe, "/AppTranslocation/"):
			dead = true
		case goos == "windows" && inWindowsTemp(exe, tmpDir):
			dead = true
		case !regular(exe):
			dead = true
		}
	}
	if !dead {
		return ""
	}
	return "Перенесите программу в постоянную папку (Mac — в «Программы», Windows — распакуйте zip) и откройте окно снова."
}

// inWindowsTemp — путь exe лежит во временной папке Windows. Сравнивается и с длинной записью
// (в %TMP% бывает 8.3: C:\Users\IVANOV~1\…, а os.Executable отдаёт длинный путь), и по элементу
// Temp<N>_*.zip, в который Проводник распаковывает открытый zip, где бы ни лежала Temp.
func inWindowsTemp(exe, tmpDir string) bool {
	if hasZipTemp(exe) {
		return true
	}
	exes := []string{exe, longPath(exe)}
	dirs := []string{tmpDir, longPath(tmpDir)}
	for _, e := range exes {
		for _, d := range dirs {
			if e != "" && d != "" && strings.HasPrefix(strings.ToLower(e), strings.ToLower(filepath.Clean(d))+`\`) {
				return true
			}
		}
	}
	return false
}

// hasZipTemp — в пути есть папка вида Temp<цифры>_<имя>.zip (регистр не важен).
func hasZipTemp(path string) bool {
	for _, el := range strings.Split(strings.ToLower(path), `\`) {
		rest, ok := strings.CutPrefix(el, "temp")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(strings.TrimLeft(rest, "0123456789"), "_")
		if ok && len(name) > len(".zip") && strings.HasSuffix(name, ".zip") {
			return true
		}
	}
	return false
}
