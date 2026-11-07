package app

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// «Диск снова доступен» (157): том не ответил за ProbeTimeout и стал «мёртвым», а потом ответил —
// недостающие фото этого тома пересчитываются сразу, без ожидания фокуса окна или тика папок, и
// интерфейс получает drive:back с числом найденных. Образец — сверка папок 088.

// Швы: подменяются в тестах (на Linux том пути пуст, а повтор ждёт секунды).
var (
	volumeOf   = osx.VolumeKey
	kickVolume = osx.KickVolume
	backRetry  = 2 * time.Second // пауза между попытками проверки папок, когда ворота заняты
	backTries  = 15              // попыток всего: 15 × 2 с ≈ 30 с
)

// driveBack — данные события drive:back: буква диска без двоеточия и число найденных фото (> 0).
type driveBack struct {
	Drive  string `json:"drive"`
	Photos int    `json:"photos"`
}

// backSnap — недостающие фото ожившего тома (ID → путь) и поколение пересчёта на момент снимка.
// Итог пересчёта того же или старшего поколения таблицу не закрывает: он мог начаться до оживления
// и видеть том мёртвым.
type backSnap struct {
	gen   int
	paths map[string]string
}

// volumeBack — обработчик osx.OnVolumeBack: том vol ответил после «мёртвого». Зовётся до пробуждения
// ждущих Reachable той же пробы, поэтому только снимок под замком и goBG — без ввода-вывода.
func (a *App) volumeBack(vol string) {
	if a.stopped.Load() {
		return
	}
	a.mu.Lock()
	snap := map[string]string{}
	for _, id := range a.missing {
		if p, ok := a.missRefs[id]; ok && volumeOf(p) == vol {
			snap[id] = p
		}
	}
	if len(snap) > 0 {
		if a.missBack == nil {
			a.missBack = map[string]*backSnap{}
		}
		b := a.missBack[vol]
		if b == nil {
			b = &backSnap{paths: map[string]string{}}
			a.missBack[vol] = b
		}
		b.gen = a.missGen
		for id, p := range snap {
			b.paths[id] = p
		}
	}
	// Проверка открытия могла назвать фото тома пропавшими, но итог ещё не прислать: пересчёт нужен
	// и при пустом снимке (тоста не будет — полосы пользователь не видел).
	recount := len(snap) > 0 || a.missPending.Load() > 0
	a.mu.Unlock()
	a.goBG(func() {
		defer guard("диск снова отвечает")
		a.backWork(vol, recount)
	})
}

// backWork — работа после оживления тома: строка в журнал, пересчёт недостающих и сверка папок.
func (a *App) backWork(vol string, recount bool) {
	log.Printf("диск %s снова отвечает", vol)
	if recount {
		a.refreshMissingQuiet() // новое поколение: висящий пересчёт отброшен, снимок остаётся
	}
	a.recheckFolders()
}

// recheckFolders — сверка папок, чтобы лента увидела папку ожившего тома. Ворота (чтение папок,
// идущая проверка, порог частоты) могут отказать, а тик 10 с идёт только при активном окне, поэтому
// отказ повторяется через backRetry, не больше backTries раз. Горутина живёт только после оживления.
func (a *App) recheckFolders() {
	for range backTries {
		if a.beginCheck(false) {
			return
		}
		t := time.NewTimer(backRetry)
		select {
		case <-a.stopCtx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// takeBack закрывает снимки оживших томов итогом полного пересчёта gen (refs — проверенные фото,
// ids — новый список недостающих); зовётся под a.mu. Число тома — фото снимка, чей путь не менялся
// и которых в списке больше нет; перепривязанное («Найти…», «Диск E: → F:») не входит. Тома без
// найденных (и не буквы) молчат. Порядок — по букве.
func (a *App) takeBack(gen int, refs map[string]string, ids []string) []driveBack {
	var out []driveBack
	for vol, b := range a.missBack {
		if gen <= b.gen {
			continue // пересчёт начался до снимка: том для него мог быть ещё мёртв
		}
		delete(a.missBack, vol)
		n := 0
		for id, p := range b.paths {
			if a.missRefs[id] != p || refs[id] != p {
				continue
			}
			if _, still := slices.BinarySearch(ids, id); !still {
				n++
			}
		}
		if letter, ok := paths.DriveLetter(vol); ok && n > 0 {
			out = append(out, driveBack{Drive: letter, Photos: n})
		}
	}
	slices.SortFunc(out, func(x, y driveBack) int { return strings.Compare(x.Drive, y.Drive) })
	return out
}

// kickMissing — проба тома без ожидания по одному пути недостающих фото на каждую букву: выдернутый
// и вставленный диск с той же буквой иначе не заметить до первого Reachable. Без stat — по строкам.
func (a *App) kickMissing() {
	a.mu.Lock()
	one := map[string]string{}
	for _, id := range a.missing {
		p := a.missRefs[id]
		if vol := volumeOf(p); vol != "" && one[vol] == "" {
			one[vol] = p
		}
	}
	a.mu.Unlock()
	for _, p := range one {
		kickVolume(p)
	}
}
