package app

import (
	"log"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ExportsSkipped — данные события `exports:skipped` (121): сколько файлов выгрузки программы лежит в
// папках фото и в какой папке нашёлся первый из новых. Dir — имя папки, без пути.
type ExportsSkipped struct {
	N   int    `json:"n"`
	Dir string `json:"dir"`
}

// publishOurs пишет в log.txt счёт файлов выгрузки, не взятых в ленту, и шлёт `exports:skipped`, если
// среди них есть новые против прошлого законченного чтения. Пути в лог не идут (фамилии клиентов).
// dead — папки недоступного тома: их не читали, поэтому ключи под ними остаются в памяти, иначе
// вернувшийся том показал бы тост второй раз. Устаревшее поколение молчит и память не трогает.
func (a *App) publishOurs(gen int, ours, dead []string) {
	if len(ours) > 0 {
		log.Printf("  выгрузка программы: %d %s — не в ленте", len(ours), plural(len(ours), "файл", "файла", "файлов"))
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if gen != a.scanGen || a.stopped.Load() {
		a.mu.Unlock()
		return
	}
	// Каждое законченное чтение заменяет память целиком: убранные файлы забываются,
	// вернувшиеся — снова новые. Кроме папок, которые не читались (dead).
	next := make(map[string]bool, len(ours))
	if len(dead) > 0 {
		under := paths.UnderAny(dead)
		for k := range a.oursSeen {
			if under(k) {
				next[k] = true
			}
		}
	}
	first := ""
	for _, p := range ours {
		k := paths.Fold(p)
		next[k] = true
		if first == "" && !a.oursSeen[k] {
			first = p
		}
	}
	a.oursSeen = next
	a.mu.Unlock()
	if first != "" {
		a.emit("exports:skipped", ExportsSkipped{N: len(ours), Dir: filepath.Base(filepath.Dir(first))})
	}
}
