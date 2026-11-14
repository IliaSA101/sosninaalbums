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

// add запоминает пути, принятые добавлением, если ok() (под замком набора) подтверждает, что альбом, в который
// добавляли, всё ещё текущий; nil — без проверки. Вернёт false, если не запомнил. Проверка под тем же замком, что и
// switchTo: добавление в закрываемый альбом не попадёт в набор нового, а добавление в новый не потеряется.
func (f *freshFiles) add(ps []string, ok func() bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ok != nil && !ok() {
		return false
	}
	if f.m == nil {
		f.m = make(map[string]bool, len(ps))
	}
	for _, p := range ps {
		f.m[model.LooseKey(p)] = true
	}
	return true
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

// switchTo забывает всё: открыт или закрыт другой альбом (setEditor). Убранные из проекта файлы уходят только
// так: набор не больше числа файлов, добавленных в этом альбоме за сессию. store (смена текущего редактора, может
// быть nil) выполняется под замком набора вместе со сбросом: порядок «Store, затем reset» без замка терял добавление
// в новый альбом (оно стоит между ними и стирается), «reset, затем Store» принимал добавление в закрываемый.
func (f *freshFiles) switchTo(store func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if store != nil {
		store()
	}
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
