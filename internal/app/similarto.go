package app

// «Похожие к этому» для окна сравнения кадров (138 design §2.3): соседи кадра по ленте,
// у которых подпись близка (порог «Похожие» автосборки 078). Подписи — из памяти App; чего нет,
// досчитывается с пределом по времени.

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/similar"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Пределы SimilarTo.
const (
	// similarOrderMax — длиннее порядок ленты не принимается (как oldIDsMax).
	similarOrderMax = 100_000
	// similarSide — сколько известных кадров по каждую сторону берётся в разбор: Window (12) для
	// пар плюс 10 соседей, которые нужны series.Cuts для местной медианы.
	similarSide = 22
	// similarShow — кадр и до трёх похожих: столько ячеек в окне сравнения.
	similarShow = 4
	// similarWait — сколько ждать досчёта подписей.
	similarWait = 2 * time.Second
)

// SimilarResult — ответ SimilarTo.
type SimilarResult struct {
	IDs  []string `json:"ids"`  // кадр и до 3 похожих в порядке ленты; пусто — см. Why
	More int      `json:"more"` // сколько похожих не вошло (больше 3)
	Why  string   `json:"why"`  // "" | "none" — похожих нет | "nosig" — у кадра нет подписи
}

// SimilarTo — кадр id и до трёх ближайших похожих среди соседей в порядке ленты order (UI шлёт
// замороженный список). Правила пары — как у отбора похожих автосборки (серия, время, ориентация,
// ±12 по ленте). Неизвестные и повторные id пропускаются; пути UI не передаёт. order длиннее
// 100 000 — ошибка. Не успели подсчитать подпись за 2 с — кадр без неё в сравнение не идёт.
func (a *App) SimilarTo(id string, order []string) (res SimilarResult, err error) {
	defer guardErr("SimilarTo", &err)
	res = SimilarResult{IDs: []string{}}
	if len(order) > similarOrderMax {
		return res, errors.New("слишком длинный список кадров")
	}
	win, at := a.similarWindow(id, order)
	if at < 0 {
		res.Why = "none"
		return res, nil
	}
	a.fillSigs(win, at)
	infos := a.similarInfos(win)
	idx, more := similar.Around(infos, at, similarShow)
	switch {
	case idx == nil:
		res.Why = "nosig"
	case len(idx) < 2:
		res.Why = "none"
	default:
		for _, i := range idx {
			res.IDs = append(res.IDs, win[i].ID)
		}
		res.More = more
	}
	return res, nil
}

// similarWindow — известные кадры вокруг id в порядке ленты (не больше similarSide с каждой
// стороны; повтор id — по первому вхождению) и позиция id среди них; -1 — id пуст, неизвестен
// или не в order.
func (a *App) similarWindow(id string, order []string) (win []library.Photo, at int) {
	if id == "" || !slices.Contains(order, id) {
		return nil, -1
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.byID[id]; !ok {
		return nil, -1
	}
	seen := make(map[string]bool, len(order))
	known := make([]string, 0, len(order))
	for _, x := range order {
		if _, ok := a.byID[x]; ok && !seen[x] {
			seen[x] = true
			known = append(known, x)
		}
	}
	i := slices.Index(known, id)
	lo, hi := max(0, i-similarSide), min(len(known), i+similarSide+1)
	for _, x := range known[lo:hi] {
		win = append(win, a.byID[x])
	}
	return win, i - lo
}

// similarInfos — PhotoInfo окна с подписями из памяти (как App.infos). Под a.mu.
func (a *App) similarInfos(win []library.Photo) []model.PhotoInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]model.PhotoInfo, len(win))
	for i, p := range win {
		out[i] = model.PhotoInfo{ID: p.ID, Path: p.Path, W: p.W, H: p.H, Size: p.Size, Taken: p.Taken, Sig: a.sigOf(p)}
	}
	return out
}

// fillSigs досчитывает подписи кадров окна, которых нет в памяти (ближайшие к at — первыми), и
// ждёт не дольше similarWait. Не успевшие остаются без подписи; воркеры дорабатывают текущий
// кадр и останавливаются, готовое лежит в памяти для следующего вызова. Подпись не требует a.mu.
func (a *App) fillSigs(win []library.Photo, at int) {
	var todo []int
	a.mu.Lock()
	for i, p := range win {
		if e, ok := a.sig.mem[p.ID]; !ok || e.size != p.Size {
			todo = append(todo, i)
		}
	}
	wait, shot := a.sig.wait, a.sig.shot
	a.mu.Unlock()
	if len(todo) == 0 || a.stopped.Load() {
		return
	}
	if wait <= 0 {
		wait = similarWait
	}
	if shot == nil {
		shot = a.cacheShot
	}
	slices.SortStableFunc(todo, func(x, y int) int { return max(x-at, at-x) - max(y-at, at-y) })
	ctx, cancel := context.WithTimeout(a.stopCtx, wait)
	defer cancel()
	done := make(chan struct{})
	if !a.goBG(func() {
		defer close(done)
		defer guard("SimilarTo/fill")
		jobs := make(chan library.Photo)
		var wg sync.WaitGroup
		for range min(len(todo), max(1, runtime.NumCPU()/2)) {
			wg.Go(func() {
				for p := range jobs {
					if ctx.Err() == nil {
						a.keepSig(p, shot)
					}
				}
			})
		}
		for _, i := range todo {
			select {
			case jobs <- win[i]:
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				break
			}
		}
		close(jobs)
		wg.Wait()
	}) {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}
