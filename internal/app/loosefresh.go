package app

import (
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// freshFiles — отдельные фото, только что добавленные пользователем и ещё не прочитанные законченным
// сканированием (160-M1). Только им сканирование читает заглушку облака (скачивает, Sources.Explicit):
// добавление — явное действие над файлом; пересканирование и открытие проекта — нет. Набор живёт до
// конца сканирования, которое их взяло: отменённое новым запуском (rescan подряд при броске, Undo)
// их не снимает, и следующее прочтёт тоже. Нигде не сохраняется. Нулевое значение готово к работе.
type freshFiles struct {
	mu sync.Mutex
	m  map[string]bool // model.LooseKey пути
}

// add запоминает пути, принятые добавлением.
func (f *freshFiles) add(ps []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = make(map[string]bool, len(ps))
	}
	for _, p := range ps {
		f.m[model.LooseKey(p)] = true
	}
}

// of — те из files (отдельные файлы проекта), что ещё свежие, в порядке files. Набор не меняет: устаревшее
// чтение (снимок источников до добавления) не должно стереть признак, нужный чтению, которое запустило добавление.
func (f *freshFiles) of(files []string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.m) == 0 {
		return nil
	}
	var out []string
	for _, p := range files {
		if f.m[model.LooseKey(p)] {
			out = append(out, p)
		}
	}
	return out
}

// reset забывает всё: открыт или закрыт другой альбом (setEditor). Убранные из проекта файлы уходят только
// так: набор не больше числа файлов, добавленных в этом альбоме за сессию.
func (f *freshFiles) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m = nil
}

// done снимает признак с ps: сканирование, прочитавшее их, закончилось.
func (f *freshFiles) done(ps []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range ps {
		delete(f.m, model.LooseKey(p))
	}
}
