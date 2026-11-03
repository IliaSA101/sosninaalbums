package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// maxSkippedKinds — сколько видов расширений перечислять в строке лога.
const maxSkippedKinds = 10

// skippedSummary — нечитаемые фото для log.txt счётом по расширениям: «3 файла .heic, 1 файл .cr3».
// Имена не пишем: в них фамилии клиентов, а лог фотограф отправляет разработчику (073-7);
// по расширению видно тип беды. Необычное расширение (не до 8 латинских букв и цифр) — «прочее» (paths.LogExt).
func skippedSummary(skipped []string) string {
	keys := make([]skipKey, len(skipped))
	for i, s := range skipped {
		keys[i] = skipKey{ext: paths.LogExt(s)}
	}
	return summarizeSkips(keys)
}

// skippedReasons — то же с причиной в скобках: «2 файла .jpg (внутри HEIC, а не JPEG)». Причина —
// готовая фраза library.Why без совета и без путей (в тексте ошибки только операция и причина ОС).
func skippedReasons(skipped []library.Skip) string {
	keys := make([]skipKey, len(skipped))
	for i, s := range skipped {
		keys[i] = skipKey{ext: paths.LogExt(s.Path), why: logWhy(library.ShortWhy(s.Why))}
	}
	return summarizeSkips(keys)
}

// maxWhyLen — длиннее причину в журнале не пишем.
const maxWhyLen = 120

// logWhy — причина одной строкой и не длиннее maxWhyLen символов.
func logWhy(why string) string {
	why = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, why)
	if r := []rune(why); len(r) > maxWhyLen {
		why = string(r[:maxWhyLen]) + "…"
	}
	return why
}

// skipKey — вид пропуска: расширение и причина ("" — без причины).
type skipKey struct{ ext, why string }

func summarizeSkips(skipped []skipKey) string {
	count := map[skipKey]int{}
	for _, s := range skipped {
		count[s]++
	}
	kinds := make([]skipKey, 0, len(count))
	for k := range count {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		if count[kinds[i]] != count[kinds[j]] {
			return count[kinds[i]] > count[kinds[j]]
		}
		if kinds[i].ext != kinds[j].ext {
			return kinds[i].ext < kinds[j].ext
		}
		return kinds[i].why < kinds[j].why
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
		part := fmt.Sprintf("%d %s %s", n, word, k.ext)
		if k.ext == "" {
			part = fmt.Sprintf("%d %s без расширения", n, word)
		}
		if k.why != "" {
			part += " (" + k.why + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
