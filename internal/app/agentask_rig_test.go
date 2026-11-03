package app

// Стенд тестов вопросов агента (149, Т3b): настоящий App с альбомом и лентой, окна ОС и «окно
// вперёд» подменены, события собираются с данными. Тесты: agentask_flow_test.go (машина),
// agentask_apply_test.go (применение, создание альбома, отмена).

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// askRig — стенд вопросов: альбом с одной папкой («Съёмка»), флажок правок включён.
type askRig struct {
	*editRig

	mu      sync.Mutex
	evs     []pubEvent
	fronts  int
	titles  []string // заголовки окон ОС
	defs    []string // DefaultDirectory окон ОС
	picked  string   // что вернёт окно ОС
	pickErr error
	hold    chan struct{} // не nil — окно ОС ждёт закрытия канала
	opened  chan struct{} // окно ОС открылось
	other   string        // вторая папка съёмки (годная к добавлению)
}

func newAskRig(t *testing.T) *askRig {
	t.Helper()
	r := &askRig{editRig: newEditRig(t, 0), opened: make(chan struct{}, 8)}
	a := r.a
	a.emit = func(name string, data ...any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		var d any
		if len(data) > 0 {
			d = data[0]
		}
		r.evs = append(r.evs, pubEvent{name, d})
	}
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	setAgentsEdit(a, true)
	r.other = filepath.Join(testkit.TempDir(t), "Вторая съёмка")
	if err := os.Mkdir(r.other, 0o755); err != nil {
		t.Fatal(err)
	}
	r.picked = r.other
	m := a.askM()
	m.life, m.exitWait = time.Minute, 300*time.Millisecond
	m.rules = permissiveRules
	m.front = func() { r.mu.Lock(); r.fronts++; r.mu.Unlock() }
	m.pickDir = func(title, def string) (string, error) {
		r.mu.Lock()
		r.titles, r.defs = append(r.titles, title), append(r.defs, def)
		hold, dir, err := r.hold, r.picked, r.pickErr
		r.mu.Unlock()
		r.opened <- struct{}{}
		if hold != nil {
			select {
			case <-hold:
			case <-time.After(20 * time.Second):
			}
		}
		return dir, err
	}
	return r
}

// holdDialog — окно ОС будет открыто, пока не вызвана release.
func (r *askRig) holdDialog() (release func()) {
	ch := make(chan struct{})
	r.mu.Lock()
	r.hold = ch
	r.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { close(ch) }) }
}

// events — события по имени, данные.
func (r *askRig) evOf(name string) (out []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.evs {
		if e.name == name {
			out = append(out, e.data)
		}
	}
	return out
}

func (r *askRig) fronted() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fronts
}

// askCall — идущий вызов agentAsk.
type askCall struct {
	res    agents.AskRes
	err    error
	done   chan struct{}
	cancel context.CancelFunc
}

// ask запускает agentAsk в своей горутине; отмена вызова — call.cancel.
func (r *askRig) ask(req agents.AskReq) *askCall {
	ctx, cancel := context.WithCancel(context.Background())
	c := &askCall{done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(c.done)
		c.res, c.err = r.a.agentAsk(ctx, req)
	}()
	return c
}

// wait ждёт итог вызова.
func (c *askCall) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("agentAsk не вернулся")
	}
}

// pending — вызов ещё идёт после небольшой паузы.
func (c *askCall) pending(d time.Duration) bool {
	select {
	case <-c.done:
		return false
	case <-time.After(d):
		return true
	}
}

// addReq — просьба добавить папку в открытый альбом.
func (r *askRig) addReq() agents.AskReq {
	return agents.AskReq{Kind: agents.AskAdd, Client: "Claude Code", Key: r.key(), Rev: r.rev()}
}

func (r *askRig) createReq() agents.AskReq {
	return agents.AskReq{Kind: agents.AskCreate, Client: "Claude Code"}
}

// shown ждёт появления вопроса и возвращает его событие.
func (r *askRig) asked(t *testing.T, n int) AgentAsk {
	t.Helper()
	waitUntil(t, "событие agents:ask", func() bool { return len(r.evOf("agents:ask")) >= n })
	return r.evOf("agents:ask")[n-1].(AgentAsk)
}

// done ждёт agents:askdone с номером n и возвращает его.
func (r *askRig) done(t *testing.T, n int) AgentAskDone {
	t.Helper()
	waitUntil(t, "событие agents:askdone", func() bool { return len(r.evOf("agents:askdone")) >= n })
	return r.evOf("agents:askdone")[n-1].(AgentAskDone)
}

// dialogOpen ждёт открытия окна ОС.
func (r *askRig) dialogOpen(t *testing.T) {
	t.Helper()
	select {
	case <-r.opened:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("окно ОС не открылось")
	}
}

// state — состояние текущего вопроса (-1 — вопроса нет).
func (r *askRig) state() askState {
	m := r.a.askM()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur == nil {
		return askAny
	}
	return m.cur.state
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	end := time.Now().Add(testkit.Limit(10 * time.Second))
	for !ok() {
		if time.Now().After(end) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// answer зовёт AnswerAgent в своей горутине (окно ОС может держать вызов).
type answerCall struct {
	err  error
	done chan struct{}
}

func (r *askRig) answerAsync(id string, yes bool) *answerCall {
	c := &answerCall{done: make(chan struct{})}
	go func() {
		defer close(c.done)
		_, c.err = r.a.AnswerAgent(id, yes)
	}()
	return c
}

func (c *answerCall) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("AnswerAgent не вернулся")
	}
}
