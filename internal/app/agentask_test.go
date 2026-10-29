package app

// Контракт правок агента (149, Т1): флажок в config.json, поля AgentsInfo, типы и имена событий,
// методы-заглушки без паники.

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// jsonTags — теги json полей типа по порядку.
func jsonTags(typ reflect.Type) []string {
	var out []string
	for f := range typ.Fields() {
		out = append(out, f.Tag.Get("json"))
	}
	return out
}

func TestSetAgentsEdit(t *testing.T) {
	a, events := pubApp(t)
	dir := a.cfgDir
	if info := a.AgentsInfo(); info.Edit || info.EditBlocked != "" || info.AsksMuted {
		t.Fatalf("умолчание: %+v", info)
	}
	if err := os.WriteFile(prefs.ConfigPath(dir), []byte(foreignConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	// Включение сохраняется в config.json, не трогает уровень и доступ, чужие ключи целы.
	a.SetAgentsLevel(3)
	if info := a.SetAgentsEdit(true); !info.Edit || info.Level != 3 || info.Status != "off" {
		t.Fatalf("включено: %+v", info)
	}
	if got := prefs.AgentsOf(dir); got != (prefs.Agents{Edit: true, Level: 3}) {
		t.Fatalf("config.json: %+v", got)
	}
	if raw := readConfig(t, dir); !strings.Contains(raw, `"edit": true`) || !strings.Contains(raw, `"future"`) || !strings.Contains(raw, `/x/a.salbum`) {
		t.Fatalf("config.json: %s", raw)
	}
	// Повтор холостой: тот же итог. Смена уровня флажок не снимает.
	if !a.SetAgentsEdit(true).Edit || !a.SetAgentsLevel(1).Edit {
		t.Fatal("флажок потерян")
	}
	// Перезапуск программы: второй App на той же папке видит флажок.
	b := newApp(t)
	b.cfgDir = dir
	if !b.AgentsInfo().Edit {
		t.Error("после перезапуска флажок сброшен")
	}
	// Выключение: флажок снят, уровень цел; все окна получили agents:state.
	n := len(events())
	if info := a.SetAgentsEdit(false); info.Edit || info.Level != 1 {
		t.Fatalf("выключено: %+v", info)
	}
	if got := names(events()[n:]); !slices.Equal(got, []string{"agents:state"}) {
		t.Errorf("события выключения: %v", got)
	}
	if prefs.AgentsOf(dir).Edit || !strings.Contains(readConfig(t, dir), `"future"`) {
		t.Errorf("config.json: %s", readConfig(t, dir))
	}
	// Ответ метода — JSON с полями контракта.
	b2, _ := json.Marshal(a.AgentsInfo())
	var m map[string]any
	_ = json.Unmarshal(b2, &m)
	for _, k := range []string{"edit", "editBlocked", "asksMuted", "calls", "clients", "setups"} {
		if _, ok := m[k]; !ok {
			t.Errorf("в AgentsInfo нет поля %q: %s", k, b2)
		}
	}
}

// Теги и порядок полей — контракт с frontend/wailsjs; имена событий — с events.md.
func TestAgentEditContract(t *testing.T) {
	for typ, want := range map[reflect.Type][]string{
		reflect.TypeFor[AgentAsk]():     {"id", "kind", "title", "lines", "hint", "ok"},
		reflect.TypeFor[AgentAskDone](): {"id", "why"},
		reflect.TypeFor[AgentEdited]():  {"client", "text", "rev", "go"},
		reflect.TypeFor[AgentShow]():    {"spread"},
	} {
		if got := jsonTags(typ); !slices.Equal(got, want) {
			t.Errorf("%s: теги %v, ждали %v", typ, got, want)
		}
	}
	tags := jsonTags(reflect.TypeFor[AgentsInfo]())
	for _, k := range []string{"edit", "editBlocked", "asksMuted"} {
		if !slices.Contains(tags, k) {
			t.Errorf("в AgentsInfo нет тега %q: %v", k, tags)
		}
	}

	a, events := pubApp(t)
	a.agentAsked(AgentAsk{ID: "i", Kind: "create", Lines: []string{"а"}})
	a.agentAskDone(AgentAskDone{ID: "i", Why: "declined"})
	a.agentEdited(AgentEdited{Client: "К", Text: "Собрано разворотов: 24", Rev: 5, Go: "s1"})
	a.agentShown(AgentShow{Spread: "s8"})
	evs := events()
	want := []string{"agents:ask", "agents:askdone", "agents:edited", "agents:show"}
	if got := names(evs); !slices.Equal(got, want) {
		t.Fatalf("события %v, ждали %v", got, want)
	}
	if v, ok := evs[2].data.(AgentEdited); !ok || v.Rev != 5 || v.Go != "s1" {
		t.Errorf("agents:edited: %#v", evs[2].data)
	}
	// Нулевые значения уходят в JSON целиком: интерфейс видит lines и go, а не пропажу ключа.
	b, _ := json.Marshal(AgentEdited{})
	if string(b) != `{"client":"","text":"","rev":0,"go":""}` {
		t.Errorf("пустое agents:edited: %s", b)
	}
}

// Заглушки до Т3: честная ошибка, ничего не меняют и не паникуют — на пустых и огромных значениях.
func TestAgentEditStubs(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	for _, id := range []string{"", "нет", strings.Repeat("я", 1<<16)} {
		for _, yes := range []bool{false, true} {
			if _, err := a.AnswerAgent(id, yes); err == nil || !strings.Contains(err.Error(), "ещё не готовы") {
				t.Errorf("AnswerAgent(%.5q, %v): err = %v", id, yes, err)
			}
		}
		if st, err := a.AgentAnswerCreate(id); err == nil || st.Photos != nil || st.Version != "" {
			t.Errorf("AgentAnswerCreate(%.5q): %+v, %v", id, st, err)
		}
	}
	before := a.editor().View().Rev
	a.AgentsBusy(true)
	a.AgentsBusy(false)
	a.AllowAgentAsks()
	a.AllowAgentAsks()
	if a.editor().View().Rev != before {
		t.Error("заглушки изменили альбом")
	}
}
