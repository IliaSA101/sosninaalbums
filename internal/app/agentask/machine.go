// Package agentask — машина вопроса агента человеку (149, design §3, 0.33-149-24; вынос — 162 Ш2б): одна
// запись под одним mutex, состояния asked (срок идёт) → choosing (окно ОС открыто, срок стоит) → done.
// Пакет не знает ни окон ОС, ни Editor, ни разрешений: проверки остаются в app, а нагрузка вопроса
// (P) для машины непрозрачна. Счёт отказов, паузы и тишина — в службе агентов.
package agentask

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// State — состояние записи вопроса.
type State int

const (
	Asked    State = iota // окно показано, срок идёт
	Choosing              // окно ОС открыто (или применяется выбор), срок стоит
	Done                  // итог отдан
)

// Any — FinishIf из любого состояния, кроме Done; Cur без вопроса тоже отдаёт Any.
const Any State = -1

var (
	// ErrBusy — вопрос уже открыт (один за раз).
	ErrBusy = agents.ErrAskBusy
	// ErrClosed — ответ на вопрос, которого нет: закрыт, чужой или другого вида.
	ErrClosed = errors.New("вопрос закрыт")
)

// Outcome — итог вопроса для ждущего вызова Host.Ask.
type Outcome struct {
	Res agents.AskRes
	Err error
}

// Rec — открытый вопрос. Payload пишется до Post и потом не меняется: читать его после Peek/Claim
// можно без замка.
type Rec[P any] struct {
	ID, Kind string
	Payload  P

	state   State
	closing bool          // выход начался: выбор не применять
	timer   *time.Timer   // срок asked
	out     chan Outcome  // итог (буфер 1)
	idle    chan struct{} // закрыт, когда обработчик ответа вернулся (только после choosing)
}

// Machine — вопросы одного App; сроки задаёт New.
type Machine[P any] struct {
	mu       sync.Mutex
	cur      *Rec[P]
	life     time.Duration
	exitWait time.Duration
	onDone   func(id, why string)
	doneEmit chan struct{} // закрыт, когда onDone последнего итога вернулся (nil — итогов не было)
}

// New — машина со сроком жизни вопроса life (состояние asked) и ожиданием выхода exitWait (сколько
// Close ждёт закрытия окна ОС). onDone зовётся на каждый итог после отдачи его ждущему; паники в нём
// ловит вызывающий (guard).
func New[P any](life, exitWait time.Duration, onDone func(id, why string)) *Machine[P] {
	return &Machine[P]{life: life, exitWait: exitWait, onDone: onDone}
}

// same — id совпал с записью (за постоянное время: id — секрет окна).
func same[P any](rec *Rec[P], id string) bool {
	return subtle.ConstantTimeCompare([]byte(id), []byte(rec.ID)) == 1
}

// Post заводит запись и запускает срок. Один вопрос за раз: занято — ErrBusy. prev закрывается, когда
// onDone прежнего итога вернулся (nil — итогов не было): agents:ask не должен обгонять agents:askdone,
// ждёт вызывающий.
func (m *Machine[P]) Post(rec *Rec[P]) (prev <-chan struct{}, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != nil {
		return nil, ErrBusy
	}
	rec.out = make(chan Outcome, 1) // у отказанной и не заведённой записи out == nil: FinishIf/Wait её не трогают
	m.cur = rec
	rec.timer = time.AfterFunc(m.life, func() {
		m.FinishIf(rec, Asked, Outcome{Res: agents.AskRes{Why: agents.WhyTimeout}}, agents.WhyTimeout)
	})
	return m.doneEmit, nil
}

// Peek — открытый вопрос в состоянии asked с этим id; kind "" — любого вида. false — вопроса нет,
// он закрыт, id чужой или вид другой.
func (m *Machine[P]) Peek(id, kind string) (*Rec[P], bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.cur
	if rec == nil || rec.state != Asked || !same(rec, id) || (kind != "" && rec.Kind != kind) {
		return nil, false
	}
	return rec, true
}

// Claim сверяет ответ человека с открытым вопросом: id, вид и состояние asked. Отказ (yes = false)
// закрывает вопрос любого вида; «да» на вопрос вида kind при on (разрешение на момент ответа)
// переводит в choosing (срок стоит) — rec возвращается, обработчик обязан вызвать Release. rec == nil
// без ошибки — вопрос закрыт этим вызовом (отказ, разрешение снято). Id чужой, вопрос закрыт или вид
// «да» не совпал — ErrClosed, вопрос остаётся как был.
func (m *Machine[P]) Claim(id, kind string, yes, on bool) (*Rec[P], error) {
	m.mu.Lock()
	rec := m.cur
	if rec == nil || rec.state != Asked || !same(rec, id) || (yes && rec.Kind != kind) {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	why := agents.WhyDeclined
	if yes && !on {
		why = agents.WhyOff
	}
	if !yes || !on {
		m.mu.Unlock()
		m.FinishIf(rec, Any, Outcome{Res: agents.AskRes{Why: why}}, why)
		return nil, nil
	}
	rec.state, rec.idle = Choosing, make(chan struct{})
	rec.timer.Stop()
	m.mu.Unlock()
	return rec, nil
}

// Release — обработчик ответа, получивший rec от Claim, вернулся (ровно один раз на Claim). Вопрос,
// который обработчик не закрыл (его паника поймана снаружи), закрывается здесь итогом gone: срок в
// choosing стоит, Wait ctx не смотрит, и без этого агент завис бы, а следующий вопрос получал ErrBusy.
// На обычных путях вопрос уже Done (или выход начался и закроет его Close) — вызов вернёт false.
func (m *Machine[P]) Release(rec *Rec[P]) {
	close(rec.idle) // первым: Close ждёт idle, а FinishIf ниже его не держит
	m.mu.Lock()
	closing := rec.closing
	m.mu.Unlock()
	if !closing {
		m.FinishIf(rec, Choosing, Outcome{Res: agents.AskRes{Why: agents.WhyGone}}, agents.WhyGone)
	}
}

// Begin — выбор можно применять: вопрос не закрыт и выход не начался.
func (m *Machine[P]) Begin(rec *Rec[P]) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return rec.state == Choosing && !rec.closing
}

// FinishIf закрывает вопрос из состояния from (Any — из любого, кроме Done): итог ждущему вызову,
// затем onDone. Срок и gone не прерывают выбор папки (0.33-149-24). false — уже закрыт или состояние
// не то.
func (m *Machine[P]) FinishIf(rec *Rec[P], from State, o Outcome, why string) bool {
	m.mu.Lock()
	// Нулевое State равно Asked, поэтому запись без Post отсекается по out: отправка в nil-канал повисла бы.
	if rec.out == nil || rec.state == Done || (from != Any && rec.state != from) {
		m.mu.Unlock()
		return false
	}
	rec.state = Done
	if rec.timer != nil {
		rec.timer.Stop()
	}
	// Запись снимается до итога: получив его, агент может сразу спросить снова, а не поймать ErrBusy.
	if m.cur == rec {
		m.cur = nil
	}
	emitted := make(chan struct{})
	m.doneEmit = emitted
	m.mu.Unlock()
	defer close(emitted) // следующий вопрос уходит только после onDone (Post → prev)
	rec.out <- o         // итог не ждёт события: зависшая отправка в окно не держит агента
	m.onDone(rec.ID, why)
	return true
}

// Wait ждёт итог вопроса или конец ctx. ctx отменён — итог gone из asked; в choosing итог придёт от
// ответа человека или от Close.
func (m *Machine[P]) Wait(ctx context.Context, rec *Rec[P]) Outcome {
	if rec.out == nil { // не заведена через Post: ждать нечего
		return Outcome{Err: ErrClosed}
	}
	select {
	case o := <-rec.out:
		return o
	case <-ctx.Done():
	}
	m.FinishIf(rec, Asked, Outcome{Res: agents.AskRes{Why: agents.WhyGone}}, agents.WhyGone)
	return <-rec.out
}

// Close — выход: вопрос снимается (итог closed). В choosing ждёт возврата окна ОС и применения, не
// дольше exitWait: зависшее окно выход не держит; выбор после начала выхода не применяется.
func (m *Machine[P]) Close() {
	m.mu.Lock()
	rec := m.cur
	if rec == nil || rec.state == Done {
		m.mu.Unlock()
		return
	}
	rec.closing = true
	idle := rec.idle
	m.mu.Unlock()
	if idle != nil {
		select {
		case <-idle:
		case <-time.After(m.exitWait):
		}
	}
	m.FinishIf(rec, Any, Outcome{Res: agents.AskRes{Why: agents.WhyClosed}}, agents.WhyClosed)
}

// Cur — состояние текущего вопроса для тестов app; false — вопроса нет (состояние Any).
func (m *Machine[P]) Cur() (State, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur == nil {
		return Any, false
	}
	return m.cur.state, true
}
