package app

// Состояние агента (машина вопросов и правки) живёт в полях App (155 М3, 0.35-155-6).

import (
	"runtime"
	"testing"
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
	apps[0].askM().life++
	if apps[0].askM().life == apps[1].askM().life {
		t.Fatal("срок вопроса общий")
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
