package app

// Состояние агента (машина вопросов и правки) живёт в полях App (155 М3, 0.35-155-6).

import (
	"runtime"
	"testing"
	"time"
	"weak"
)

// У разных App — свои машина вопросов и состояние правок; у одного — одни и те же при повторе;
// и у &App{} без NewApp (нулевое значение) тоже.
func TestAskMachinePerApp(t *testing.T) {
	apps := []*App{NewApp(), NewApp(), {}, {}}
	for i, a := range apps {
		m, e := a.askM(), a.agentEd()
		if m == nil || e == nil || m.rules == nil {
			t.Fatalf("App %d: пустое состояние", i)
		}
		if a.askM() != m || a.agentEd() != e {
			t.Fatalf("App %d: повторный вызов дал другое состояние", i)
		}
		for j := 0; j < i; j++ {
			if apps[j].askM() == m || apps[j].agentEd() == e {
				t.Fatalf("App %d и %d делят состояние", i, j)
			}
		}
	}
	// правка одного не видна другому
	apps[0].AgentsBusy(true)
	if !apps[0].agentEd().busyNow() || apps[1].agentEd().busyNow() {
		t.Fatal("занятость просочилась в другой App")
	}
}

// Машины вопросов у разных App — разные: сравнение попарно по умолчанию, без newAskQ (он подменил бы
// машину и скрыл общую).
func TestAskMachinePerAppDistinct(t *testing.T) {
	apps := []*App{NewApp(), NewApp(), {}, {}}
	qs := make([]any, len(apps))
	for i, a := range apps {
		q := a.askQ()
		if q == nil {
			t.Fatalf("App %d: нет машины вопросов", i)
		}
		if a.askQ() != q {
			t.Fatalf("App %d: повторный askQ дал другую машину", i)
		}
		qs[i] = q
	}
	for i := range qs {
		for j := i + 1; j < len(qs); j++ {
			if qs[i] == qs[j] {
				t.Fatalf("App %d и %d делят машину вопросов", i, j)
			}
		}
	}
	// после подмены в одном App остальные не затронуты
	before := apps[1].askQ()
	newAskQ(apps[0], time.Second, time.Second)
	if apps[0].askQ() == qs[0] || apps[1].askQ() != before {
		t.Fatal("подмена машины задела другой App")
	}
}

// Выброшенный App с созданным состоянием освобождается: глобальных карт, держащих его, нет.
func TestAgentStateNoLeak(t *testing.T) {
	a := &App{}
	a.askM()
	a.agentEd()
	w := weak.Make(a)
	a = nil
	for i := 0; i < 5 && w.Value() != nil; i++ {
		runtime.GC()
	}
	if w.Value() != nil {
		t.Fatal("App с состоянием агента не освобождён")
	}
}
