package app

// Частота событий журнала агентов (0.31-143-43): первый вызов — agents:call, вызовы чаще
// agentsCallGap склеиваются в один agents:state, выход снимает таймер.

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// pubApp — App, у которого события собираются в список (имя и полезная нагрузка).
type pubEvent struct {
	name string
	data any
}

func pubApp(t *testing.T) (*App, func() []pubEvent) {
	t.Helper()
	a := newApp(t)
	var mu sync.Mutex
	var evs []pubEvent
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		var d any
		if len(data) > 0 {
			d = data[0]
		}
		evs = append(evs, pubEvent{name, d})
	}
	return a, func() []pubEvent {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(evs)
	}
}

func names(evs []pubEvent) (out []string) {
	for _, e := range evs {
		out = append(out, e.name)
	}
	return out
}

func TestAgentsCallThrottle(t *testing.T) {
	a, events := pubApp(t)
	call := func(n int) {
		a.agentsCalled(agents.Call{At: time.Now(), Client: "К", Tool: "Сводка альбома", OK: true, Ms: n}, n)
	}

	// Первый вызов — сразу agents:call с вызовом и счётчиком дня.
	call(1)
	evs := events()
	if len(evs) != 1 || evs[0].name != "agents:call" {
		t.Fatalf("первый вызов: %v", names(evs))
	}
	ev, ok := evs[0].data.(AgentCallEvent)
	if !ok || ev.Today != 1 || ev.Tool != "Сводка альбома" || ev.Client != "К" || ev.Ms != 1 {
		t.Fatalf("нагрузка agents:call: %#v", evs[0].data)
	}

	// Всплеск внутри окна: ни одного agents:call, один agents:state в конце окна — со всем журналом.
	for i := 2; i <= 20; i++ {
		call(i)
	}
	if got := names(events()); len(got) != 1 {
		t.Fatalf("всплеск выдал события сразу: %v", got)
	}
	agWait(t, "agents:state в конце окна", func() bool { return len(events()) == 2 })
	evs = events()
	if evs[1].name != "agents:state" {
		t.Fatalf("в конце окна: %v", names(evs))
	}
	if _, ok := evs[1].data.(AgentsInfo); !ok {
		t.Errorf("нагрузка agents:state: %T", evs[1].data)
	}
	time.Sleep(agentsCallGap + 100*time.Millisecond) // больше событий не будет: склеенные ушли одним
	if got := names(events()); len(got) != 2 {
		t.Fatalf("после склейки: %v", got)
	}
	a.agentsPub.mu.Lock()
	if a.agentsPub.timer != nil {
		t.Error("таймер остался после срабатывания")
	}
	a.agentsPub.mu.Unlock()

	// Окно прошло — следующий вызов снова идёт сразу как agents:call.
	call(21)
	if got := names(events()); !slices.Equal(got, []string{"agents:call", "agents:state", "agents:call"}) {
		t.Fatalf("вызов после паузы: %v", got)
	}
}

// Выход: таймер склейки снят и новый не заводится; события после stopAgents не приходят.
func TestAgentsCallThrottleStop(t *testing.T) {
	a, events := pubApp(t)
	call := func(n int) { a.agentsCalled(agents.Call{At: time.Now(), Tool: "т", OK: true}, n) }
	call(1)
	call(2) // внутри окна: таймер заведён
	a.agentsPub.mu.Lock()
	armed := a.agentsPub.timer != nil
	a.agentsPub.mu.Unlock()
	if !armed {
		t.Fatal("таймер склейки не заведён")
	}
	a.stopAgents()
	a.agentsPub.mu.Lock()
	if a.agentsPub.timer != nil {
		t.Error("stopAgents не снял таймер")
	}
	a.agentsPub.mu.Unlock()
	time.Sleep(agentsCallGap + 100*time.Millisecond)
	if got := names(events()); !slices.Equal(got, []string{"agents:call"}) {
		t.Fatalf("после stopAgents пришли события: %v", got)
	}

	// Программа закрывается: новый вызов внутри окна таймер не заводит.
	b, bEvents := pubApp(t)
	b.stopped.Store(true)
	b.agentsCalled(agents.Call{}, 1)
	b.agentsCalled(agents.Call{}, 2)
	b.agentsPub.mu.Lock()
	if b.agentsPub.timer != nil {
		t.Error("таймер заведён после закрытия")
	}
	b.agentsPub.mu.Unlock()
	time.Sleep(agentsCallGap + 100*time.Millisecond)
	if got := names(bEvents()); !slices.Equal(got, []string{"agents:call"}) {
		t.Fatalf("при закрытии: %v", got)
	}
	b.stopAgents() // повторный и холостой выход без службы — без паники
	b.stopAgents()
}
