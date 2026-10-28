// Package update — обновление программы (спека 111): подписанный манифест выпуска, проверка
// версии, скачивание и сверка установщика, его запуск. Сеть — только decor/fetch (forbidigo);
// внутри stdlib, decor/fetch и osx. Версии и их сравнение (ParseVersion, Newer) использует и
// спека 112 («альбом из более новой версии»).
package update

import "strconv"

// Version — x.y.z числами (части по 1…4 цифры): так версия выпуска стоит в wails.json, теге и
// имени установщика. Суффиксы («-rc1») и «dev» не версия: их разбор вызывающий решает сам.
type Version [3]int

// ParseVersion — «0.25.1» → {0,25,1}; любая другая строка (лишние части, знаки, пустые части,
// больше 4 цифр, пробелы) — false.
func ParseVersion(s string) (Version, bool) {
	var v Version
	part, n := 0, 0 // номер части и сколько в ней цифр
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if n++; n > 4 {
				return Version{}, false
			}
			v[part] = v[part]*10 + int(c-'0')
		case c == '.' && n > 0 && part < 2:
			part, n = part+1, 0
		default:
			return Version{}, false
		}
	}
	if part != 2 || n == 0 {
		return Version{}, false
	}
	return v, true
}

// String — «0.25.1».
func (v Version) String() string {
	return strconv.Itoa(v[0]) + "." + strconv.Itoa(v[1]) + "." + strconv.Itoa(v[2])
}

// Compare — -1, 0, 1: v меньше, равна, больше w (по частям, числом: 0.9.0 < 0.10.0).
func (v Version) Compare(w Version) int {
	for i := range v {
		switch {
		case v[i] < w[i]:
			return -1
		case v[i] > w[i]:
			return 1
		}
	}
	return 0
}

// Newer — a строго новее b. Любая из строк не версия (dev, пусто, суффикс) — false: сравнить
// нечем, значит и «новее» сказать нельзя (ни предложения обновления, ни отказа открыть файл).
func Newer(a, b string) bool {
	va, ok1 := ParseVersion(a)
	vb, ok2 := ParseVersion(b)
	return ok1 && ok2 && va.Compare(vb) > 0
}
