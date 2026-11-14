package paths

import (
	"runtime"
	"strings"
)

// TrailingDot — в пути есть элемент с точкой или пробелом на конце (кроме «.» и «..») на Windows.
// Win32 срезает их при разборе имени: `C:\Фото\.. \x.jpg` на диске — `C:\x.jpg`, а текст проверки
// мест, Clean и список «проверенных папок» видят папку «.. » внутри «Фото». Настоящие папки
// с таким именем Windows не создаёт, поэтому отказ ничего законного не теряет. На других ОС имя
// «a.» — обычное, там false.
func TrailingDot(path string) bool { return TrailingDotFor(runtime.GOOS == "windows", path) }

// TrailingDotFor — TrailingDot по правилам Windows (windows) или остальных ОС: ОС выбирает
// вызывающий, чтобы правило проверялось таблицей на любой ОС.
func TrailingDotFor(windows bool, path string) bool {
	if !windows {
		return false
	}
	for _, el := range strings.FieldsFunc(path, IsSepRune) {
		if el == "." || el == ".." {
			continue
		}
		if last := el[len(el)-1]; last == '.' || last == ' ' {
			return true
		}
	}
	return false
}
