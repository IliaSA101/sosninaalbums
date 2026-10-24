package library

import (
	"context"
	"sync"
)

// prio — класс места генерации (090 design §2).
type prio uint8

const (
	now prio = iota // запрос окна: /media, фоны, плитки шаблонов — фотограф ждёт на экране
	bg              // прогрев, подписи, превью в фоне — никто не смотрит
)

// waiter — ждущий места; ready закрывается, когда место выдано (под mu, место уже посчитано занятым).
type waiter struct {
	p       prio
	ready   chan struct{}
	granted bool
}

// sched — места генерации двух классов. Запрос окна берёт любое свободное из total; прогрев — не
// больше bgMax и только когда запросов окна нет ни ждущих, ни идущих: начатое он доделывает, нового
// не берёт (сценарий 1). Ждущие окна — стек: после прокрутки на экране последние запрошенные (090-D1).
type sched struct {
	mu      sync.Mutex
	total   int
	bgMax   int
	run     [2]int
	nowWait []*waiter // стек: новый — первым
	bgWait  []*waiter // очередь: по порядку
}

func newSched(total, bgMax int) *sched {
	total = max(1, total)
	return &sched{total: total, bgMax: min(total, max(1, bgMax))}
}

// fits — можно ли сейчас выдать место классу p (под mu; без учёта ждущих своего класса).
func (s *sched) fits(p prio) bool {
	if s.run[now]+s.run[bg] >= s.total {
		return false
	}
	return p == now || len(s.nowWait) == 0 && s.run[now] == 0 && s.run[bg] < s.bgMax
}

// acquire ждёт место класса p. Снятый ctx — ctx.Err() без обёртки, место не занято (и не занимается,
// если ctx снят ещё до вызова: снятый запрос генерацию не начинает). release идемпотентен.
func (s *sched) acquire(ctx context.Context, p prio) (release func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	queued := len(s.nowWait) > 0
	if p == bg {
		queued = len(s.bgWait) > 0
	}
	if !queued && s.fits(p) {
		s.run[p]++
		s.mu.Unlock()
		return s.releaser(p), nil
	}
	w := &waiter{p: p, ready: make(chan struct{})}
	if p == now {
		s.nowWait = append(s.nowWait, w)
	} else {
		s.bgWait = append(s.bgWait, w)
	}
	s.mu.Unlock()
	select {
	case <-w.ready:
		rel := s.releaser(p)
		if err := ctx.Err(); err != nil { // сняли одновременно с выдачей: генерации всё равно нет
			rel()
			return nil, err
		}
		return rel, nil
	case <-ctx.Done():
	}
	s.mu.Lock()
	if w.granted { // место выдали в ту же минуту — отдать следующему
		s.run[p]--
	} else {
		s.drop(w)
	}
	s.dispatch() // ушёл последний ждущий окна — прогрев может продолжить
	s.mu.Unlock()
	return nil, ctx.Err()
}

// drop убирает ждущего из его списка (под mu). O(n), n — ждущие; их сотни.
func (s *sched) drop(w *waiter) {
	list := &s.bgWait
	if w.p == now {
		list = &s.nowWait
	}
	for i, x := range *list {
		if x == w {
			*list = append((*list)[:i], (*list)[i+1:]...)
			return
		}
	}
}

// dispatch раздаёт свободные места (под mu): сначала вершине стека окна, потом прогреву по порядку.
func (s *sched) dispatch() {
	for len(s.nowWait) > 0 && s.fits(now) {
		w := s.nowWait[len(s.nowWait)-1]
		s.nowWait = s.nowWait[:len(s.nowWait)-1]
		s.grant(w)
	}
	for len(s.bgWait) > 0 && s.fits(bg) {
		w := s.bgWait[0]
		s.bgWait = s.bgWait[1:]
		s.grant(w)
	}
}

func (s *sched) grant(w *waiter) {
	w.granted = true
	s.run[w.p]++
	close(w.ready)
}

// releaser — возврат места; повторный вызов ничего не делает (defer и явный вызов не отдают место дважды).
func (s *sched) releaser(p prio) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.run[p]--
			s.dispatch()
			s.mu.Unlock()
		})
	}
}
