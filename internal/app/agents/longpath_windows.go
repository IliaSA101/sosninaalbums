package agents

import "golang.org/x/sys/windows"

// longPathName — длинная запись пути (8.3 → полное имя). Пусто — не получилось или путь не
// существует: тогда остаётся сравнение по тексту.
func longPathName(p string) string {
	if p == "" {
		return ""
	}
	in, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return ""
	}
	n, err := windows.GetLongPathName(in, nil, 0)
	if err != nil || n == 0 {
		return ""
	}
	out := make([]uint16, n)
	n, err = windows.GetLongPathName(in, &out[0], n)
	if err != nil || n == 0 || int(n) > len(out) {
		return ""
	}
	return windows.UTF16ToString(out[:n])
}
