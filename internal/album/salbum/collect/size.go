package collect

import (
	"strconv"
	"strings"
)

// SizeText — объём для текстов сбора: «6,3 ГБ», «4,5 МБ», «120 МБ», «< 0,1 МБ» — как formatSize
// окна экспорта (МБ до 1 ГБ), с запятой.
func SizeText(n int64) string {
	const mb, gb = 1 << 20, 1 << 30
	switch {
	case n >= gb:
		return comma(float64(n)/gb, 1) + " ГБ"
	case n < mb/10:
		return "< 0,1 МБ"
	case n < 10*mb:
		return comma(float64(n)/mb, 1) + " МБ"
	}
	return comma(float64(n)/mb, 0) + " МБ"
}

func comma(v float64, prec int) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', prec, 64), ".", ",", 1)
}
