package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// maxSkippedKinds — сколько видов расширений перечислять в строке лога.
const maxSkippedKinds = 10

// skippedSummary — нечитаемые фото для log.txt счётом по расширениям: «3 файла .heic, 1 файл .cr3».
// Имена не пишем: в них фамилии клиентов, а лог фотограф отправляет разработчику (073-7);
// по расширению видно тип беды. Необычное расширение (не до 8 латинских букв и цифр) — «прочее» (paths.LogExt).
func skippedSummary(skipped []string) string {
	count := map[string]int{}
	for _, s := range skipped {
		count[paths.LogExt(s)]++
	}
	kinds := make([]string, 0, len(count))
	for k := range count {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		if count[kinds[i]] != count[kinds[j]] {
			return count[kinds[i]] > count[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	var parts []string
	for i, k := range kinds {
		if i == maxSkippedKinds {
			rest := len(kinds) - i
			parts = append(parts, fmt.Sprintf("и ещё %d %s", rest, plural(rest, "вид", "вида", "видов")))
			break
		}
		n := count[k]
		word := plural(n, "файл", "файла", "файлов")
		if k == "" {
			parts = append(parts, fmt.Sprintf("%d %s без расширения", n, word))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s %s", n, word, k))
		}
	}
	return strings.Join(parts, ", ")
}
