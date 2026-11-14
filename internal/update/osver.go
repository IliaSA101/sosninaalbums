package update

import "sort"

// maxOSDigits — цифр в одной части версии системы: в сборке Windows их пять (10.0.22631).
const maxOSDigits = 6

// ParseOS — версия системы «13», «13.0», «10.0.22631»: 1…3 части по 1…6 цифр, недостающие части
// равны 0. Любая другая строка (знаки, пробелы, пустые части, четыре части) — false. Это не
// ParseVersion: у программы ровно x.y.z и до 4 цифр, у системы форма вольнее (171-2).
func ParseOS(s string) ([3]int, bool) {
	var v [3]int
	part, n := 0, 0 // номер части и сколько в ней цифр
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if n++; n > maxOSDigits {
				return [3]int{}, false
			}
			v[part] = v[part]*10 + int(c-'0')
		case c == '.' && n > 0 && part < 2:
			part, n = part+1, 0
		default:
			return [3]int{}, false
		}
	}
	if n == 0 { // пусто или точка в конце
		return [3]int{}, false
	}
	return v, true
}

// cmpOS — -1, 0, 1 по частям, числом.
func cmpOS(a, b [3]int) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

// OSLess — система a строго старше b. Любая из строк не версия — false: сравнить нечем, правило
// не срабатывает (как Newer).
func OSLess(a, b string) bool {
	va, ok1 := ParseOS(a)
	vb, ok2 := ParseOS(b)
	return ok1 && ok2 && cmpOS(va, vb) < 0
}

// stepFor — ступень лестницы lastOn: версия программы для систем ниже порога; берётся
// наименьший порог, который больше версии системы (Mac 12 при порогах 13 и 14 получает версию
// для 12). "" — ступени нет: система не ниже ни одного порога или не версия.
func stepFor(steps map[string]string, sys string) string {
	sv, ok := ParseOS(sys)
	if !ok {
		return ""
	}
	best, bestKey, bestVal := [3]int{}, "", ""
	for key, val := range steps {
		kv, ok := ParseOS(key)
		if !ok || cmpOS(kv, sv) <= 0 {
			continue
		}
		// При равных порогах (разбор их не пропускает) — детерминированно меньшая версия.
		if c := cmpOS(kv, best); bestKey == "" || c < 0 || c == 0 && val < bestVal {
			best, bestKey, bestVal = kv, key, val
		}
	}
	return bestVal
}

// stepsSorted — ступени по возрастанию порога (разбор и тесты); ключи уже прошли ParseOS.
func stepsSorted(steps map[string]string) []string {
	keys := make([]string, 0, len(steps))
	for k := range steps {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := ParseOS(keys[i])
		b, _ := ParseOS(keys[j])
		return cmpOS(a, b) < 0
	})
	return keys
}
