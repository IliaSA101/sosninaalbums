package app

// Подписи кадров для автосборки (078 design §4.3): отбор похожих и ч/б развороты читают их из
// памяти App (infos), без диска. Готовит PrepareSigs в фоне, ход — событием sigs:progress.

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// SigState — ход подготовки подписей: done из total фото готовы (в том числе без подписи —
// не прочитались, такие не ждут).
type SigState struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// sigEntry — подпись в памяти и размер файла, для которого она посчитана: файл сменил размер —
// запись не годится (тот же размер и другой файл — до перезапуска, решение 078-2).
type sigEntry struct {
	size int64
	sig  imgsig.Sig
}

// sigState — фоновая подготовка: память подписей и отмена идущей работы (под App.mu).
type sigState struct {
	mem    map[string]sigEntry
	cancel context.CancelFunc
	gen    int // поколение работы: отменённая молчит
	// shot — подпись фото; в тестах подменяется (по умолчанию — Cache.ShotSig).
	shot func(p library.Photo) (imgsig.Sig, error)
}

// errNoCache — кэша миниатюр нет (не создался при запуске): подписей нет.
var errNoCache = errors.New("кэша миниатюр нет")

// sigEvery — не чаще этого шлётся sigs:progress (последнее — всегда).
const sigEvery = 200 * time.Millisecond

// sigOf — подпись фото из памяти; нет или файл другой — Ok = false. Под a.mu.
func (a *App) sigOf(p library.Photo) imgsig.Sig {
	if e, ok := a.sig.mem[p.ID]; ok && e.size == p.Size {
		return e.sig
	}
	return imgsig.Sig{}
}

// PrepareSigs — подготовить подписи кадров ids для окна автосборки (078-12). Ответ сразу:
// сколько уже готово; остальное считается в фоне (воркеров — половина ядер), ход — событием
// sigs:progress (SigState) не чаще 200 мс и последним при done = total. Новый вызов отменяет
// прежнюю работу; пустой ids — только отмена (окно закрыто). Неизвестные ID не считаются.
func (a *App) PrepareSigs(ids []string) (st SigState) {
	defer guard("PrepareSigs")
	// gen++ под emitMu (порядок emitMu → mu, как в emitSigs): событие прошлой работы, уже
	// прошедшее проверку поколения, уходит до ответа нового вызова, а не после него.
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sig.cancel != nil {
		a.sig.cancel()
		a.sig.cancel = nil
	}
	a.sig.gen++
	seen := make(map[string]bool, len(ids))
	var todo []library.Photo
	for _, id := range ids {
		p, ok := a.byID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		st.Total++
		if e, ok := a.sig.mem[id]; ok && e.size == p.Size {
			st.Done++
		} else {
			todo = append(todo, p)
		}
	}
	if len(todo) == 0 || a.stopped.Load() {
		return st
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	a.sig.cancel = cancel
	gen, start := a.sig.gen, st
	a.bg.Go(func() {
		defer cancel()
		a.runSigs(ctx, gen, todo, start)
	})
	return st
}

// runSigs считает подписи todo в фоне; start — что было готово до них.
func (a *App) runSigs(ctx context.Context, gen int, todo []library.Photo, start SigState) {
	defer guard("runSigs")
	shot := a.sig.shot
	if shot == nil {
		shot = a.cacheShot
	}
	pr := &sigProgress{st: start}
	jobs := make(chan library.Photo)
	var wg sync.WaitGroup
	for range max(1, runtime.NumCPU()/2) {
		wg.Go(func() {
			for p := range jobs {
				a.keepSig(p, shot)
				if ctx.Err() != nil {
					continue
				}
				if st, due := pr.step(); due {
					a.emitSigs(gen, st)
				}
			}
		})
	}
	for _, p := range todo {
		select {
		case jobs <- p:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
}

// keepSig считает подпись p и кладёт в память; не разобралась (битый файл, паника декодера) —
// Ok = false, больше не пробуем в этой сессии. Ошибка ввода-вывода (диск отключён, кэша нет) —
// записи нет: следующее окно попробует снова. Посчитанное годится и отменённой работе.
func (a *App) keepSig(p library.Photo, shot func(p library.Photo) (imgsig.Sig, error)) {
	s, keep := trySig(p, shot)
	if !keep {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sig.mem == nil {
		a.sig.mem = map[string]sigEntry{}
	}
	a.sig.mem[p.ID] = sigEntry{size: p.Size, sig: s}
}

// trySig — подпись p; keep = false — файл не прочитан (запоминать нечего). Паника декодера
// гасится здесь, как в warmOne (library/thumbs.go): воркер runSigs без recover уронил бы программу.
func trySig(p library.Photo, shot func(p library.Photo) (imgsig.Sig, error)) (s imgsig.Sig, keep bool) {
	defer func() {
		if r := recover(); r != nil {
			// Без имени файла: имена клиентов в log.txt не пишем (073-7).
			log.Printf("ПАНИКА при подписи кадра (файл %q): %v\n%s", paths.LogExt(p.Name), r, debug.Stack())
			s, keep = imgsig.Sig{}, true
		}
	}()
	s, err := shot(p)
	if err != nil {
		return imgsig.Sig{}, !sigIOErr(err)
	}
	return s, true
}

// sigIOErr — ошибка чтения, а не разбора: файл может прочитаться позже (диск вернётся).
func sigIOErr(err error) bool {
	var pe *fs.PathError
	var le *os.LinkError
	var se *os.SyscallError
	return errors.Is(err, errNoCache) || errors.As(err, &pe) || errors.As(err, &le) || errors.As(err, &se)
}

// sigProgress — ход одной работы для событий: done и время последнего события.
type sigProgress struct {
	mu   sync.Mutex
	st   SigState
	last time.Time
}

// step — ещё одно фото готово; due — пора слать событие (не чаще sigEvery, последнее — всегда).
func (g *sigProgress) step() (st SigState, due bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.st.Done++
	if due = g.st.Done == g.st.Total || time.Since(g.last) >= sigEvery; due {
		g.last = time.Now()
	}
	return g.st, due
}

// emitSigs шлёт ход, если работа ещё текущая: отменённая молчит. emitMu держит порядок событий.
func (a *App) emitSigs(gen int, st SigState) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	cur := gen == a.sig.gen
	a.mu.Unlock()
	if cur {
		a.emit("sigs:progress", st)
	}
}

// cacheShot — подпись через кэш миниатюр; кэша нет — подписи нет.
func (a *App) cacheShot(p library.Photo) (imgsig.Sig, error) {
	if a.cache == nil {
		return imgsig.Sig{}, errNoCache
	}
	return a.cache.ShotSig(p)
}
