package app

// Выход и события фасада службы агентов (143, Г5): выход закрывает работающую службу, SetAgents
// после выхода и в гонке с ним сокет не открывает, agents:state на SetAgents уходит один раз,
// ClearAgentsLog шлёт состояние и оставляет счётчик дня.

import (
	"bytes"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// noSock — файла сокета нет.
func noSock(t *testing.T, r *agRig, when string) {
	t.Helper()
	if _, err := os.Lstat(r.sock); !os.IsNotExist(err) {
		t.Errorf("%s: файл сокета на месте (%v)", when, err)
	}
}

func TestAgentsStopOnExit(t *testing.T) {
	r := newAgRig(t)
	r.start(t)
	if _, err := os.Lstat(r.sock); err != nil {
		t.Fatalf("после включения сокета нет: %v", err)
	}
	if n := len(r.a.AgentsInfo().Clients); n != 1 {
		t.Fatalf("подключённых %d, ждали 1", n)
	}
	r.a.stopBackground()
	noSock(t, r, "после выхода")
	if info := r.a.AgentsInfo(); info.Status != "off" || len(info.Clients) != 0 {
		t.Errorf("после выхода: %q, клиентов %d", info.Status, len(info.Clients))
	}
	// Клиент отключён: вызов не проходит (соединение оборвано).
	if _, err := r.cs.ListTools(t.Context(), nil); err == nil {
		t.Error("клиент после выхода всё ещё работает")
	}
	// Включение после выхода: сокет не открывается.
	info, err := r.a.SetAgents(true)
	if err != nil || info.Status != "off" {
		t.Errorf("SetAgents(true) после выхода: %+v %v", info, err)
	}
	noSock(t, r, "SetAgents(true) после выхода")
	// Запуск программы, начавшийся после выхода, тоже ничего не открывает.
	if err := os.WriteFile(prefs.ConfigPath(r.cfg), []byte(`{"agents":{"on":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r.a.startAgents()
	r.a.bg.Wait()
	noSock(t, r, "startAgents после выхода")
}

// Выход пришёл, пока SetAgents(true) открывал сокет: stopAgents видит «выключено» и не закрывает,
// сокет закрывает сам SetAgents. Остановка внутри Start — через Host.Sock, без пауз по времени.
func TestAgentsExitDuringStart(t *testing.T) {
	r := newAgRig(t)
	h := r.a.agentsHost()
	open := h.Sock
	entered, release := make(chan struct{}), make(chan struct{})
	first := true
	h.Sock = func() string {
		if first {
			first = false
			close(entered)
			<-release
		}
		return open()
	}
	r.a.agents = agents.New(h, agents.Options{OnCall: r.a.agentsCalled})

	done := make(chan AgentsInfo, 1)
	go func() {
		info, _ := r.a.SetAgents(true)
		done <- info
	}()
	select {
	case <-entered:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("Start не дошёл до Host.Sock")
	}
	r.a.stopBackground() // служба ещё off: Close не зовётся
	cfgBefore, errBefore := os.ReadFile(prefs.ConfigPath(r.cfg))
	close(release)
	select {
	case info := <-done:
		if info.Status != "off" {
			t.Errorf("SetAgents в гонке с выходом: %q", info.Status)
		}
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("SetAgents не вернулся")
	}
	if st, _ := r.a.agents.State(); st != "off" {
		t.Errorf("служба после выхода: %q", st)
	}
	noSock(t, r, "после гонки")
	// Обещание closeBackground: после выхода config.json не пишется (On=true не попал на диск).
	cfgAfter, errAfter := os.ReadFile(prefs.ConfigPath(r.cfg))
	if (errBefore == nil) != (errAfter == nil) || !bytes.Equal(cfgBefore, cfgAfter) {
		t.Errorf("config.json изменился после выхода: было %q, стало %q", cfgBefore, cfgAfter)
	}
}

// SetAgents уведомляет окна одним agents:state: Start/Close уже шлют его через Host.Changed.
func TestAgentsStateOncePerSet(t *testing.T) {
	r := newAgRig(t)
	count := func() int {
		n := 0
		for _, e := range r.eventNames() {
			if e == "agents:state" {
				n++
			}
		}
		return n
	}
	for i, on := range []bool{true, true, false, false} {
		before := count()
		if _, err := r.a.SetAgents(on); err != nil {
			t.Fatal(err)
		}
		if got := count() - before; got != 1 {
			t.Errorf("SetAgents(%v) шаг %d: agents:state %d раз, ждали 1", on, i, got)
		}
	}
}

func TestClearAgentsLogFacade(t *testing.T) {
	r := newAgRig(t)
	r.start(t)
	r.call(t, "get_album", map[string]any{})
	r.call(t, "list_spreads", map[string]any{})
	agWait(t, "журнал: 2 вызова", func() bool { return len(r.a.AgentsInfo().Calls) == 2 })
	before := len(r.eventNames())
	info := r.a.ClearAgentsLog()
	if len(info.Calls) != 0 || info.Today != 2 {
		t.Errorf("после очистки: записей %d, today %d (ждали 0 и 2)", len(info.Calls), info.Today)
	}
	if got := r.eventNames()[before:]; !slices.Equal(got, []string{"agents:state"}) {
		t.Errorf("события очистки: %v", got)
	}
	if again := r.a.ClearAgentsLog(); len(again.Calls) != 0 || again.Today != 2 { // холостая очистка
		t.Errorf("повторная очистка: %+v", again)
	}
}
