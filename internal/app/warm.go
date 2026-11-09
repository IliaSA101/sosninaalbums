package app

import (
	"context"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
)

// Ход прогрева раскладок при открытии (073, решение 073-5): альбом 200 × 16 фото считается
// секунды, окно «Открываем альбом» показывает «Раскладки: done из total».

// warmFirst — первое событие не раньше этого от начала счёта: тёплый кэш и малый альбом
// событий не шлют; warmEvery — не чаще одного на столько. Переменные — ради теста.
var (
	warmFirst = 150 * time.Millisecond
	warmEvery = 100 * time.Millisecond
)

// warmEditor греет раскладки ed до замка App (GetState): лента и события library/scan не стоят,
// пока считается View. Если ход показывали — последнее событие всегда (total, total), и при
// панике: иначе окно открытия не погаснет. Паника прогрева — в лог: View посчитает сам.
func (a *App) warmEditor(ed *album.Editor) {
	defer guard("прогреве раскладок")
	a.layouts.enter()
	defer a.layouts.leave()
	t0 := time.Now()
	var last time.Time
	sent, done, total := false, 0, 0
	defer func() {
		if sent && done != total {
			a.emitProgress(total, total)
		}
	}()
	// Зовётся по одному (Warm), поэтому поля без замка.
	ed.Warm(func(d, n int) {
		now := time.Now()
		if sent && d == n || !sent && now.Sub(t0) >= warmFirst || sent && now.Sub(last) >= warmEvery {
			sent, last, done, total = true, now, d, n
			a.emitProgress(d, n)
			return
		}
		total = n
	})
}

// busyGate — идут ли прогревы раскладок (warmEditor): фоновый прогрев миниатюр их пропускает
// вперёд (073-11) — на стенде вместе они шли на 30–50 % дольше. Нулевое значение — свободен.
type busyGate struct {
	mu   sync.Mutex
	n    int
	free chan struct{} // закрывается, когда n снова 0
}

func (g *busyGate) enter() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n == 0 {
		g.free = make(chan struct{})
	}
	g.n++
}

func (g *busyGate) leave() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n--
	if g.n == 0 {
		close(g.free)
	}
}

// wait ждёт, пока прогревов не останется, или отмены ctx.
func (g *busyGate) wait(ctx context.Context) {
	g.mu.Lock()
	ch := g.free
	idle := g.n == 0
	g.mu.Unlock()
	if idle {
		return
	}
	select {
	case <-ch:
	case <-ctx.Done():
	}
}

// emitProgress шлёт open:progress. Зовётся из воркера раскладки, где паника роняет процесс, —
// поэтому со своим recover: без хода альбом всё равно откроется.
func (a *App) emitProgress(done, total int) {
	defer guard("событии хода открытия")
	a.emit("open:progress", done, total) // два числа, как thumbs и collect:progress (events.md)
}
