package prefs

// Отметка прохода подъёма (LiftDone) и заметка о поднятом месте при ✕ (0.35/153, доправка ревью).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// LiftDone: первая запись ключа ставит её; круг Save→Load; снятая в fn остаётся снятой (файл 0.34);
// мусорный тип в файле — не падение; ключа нет — отметки нет.
func TestLiftDoneRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	if LoadAgentPerms(dir).LiftDone {
		t.Fatal("без ключа отметка стоит")
	}
	UpdateAgentPerms(dir, func(p *AgentPerms) {})
	if !LoadAgentPerms(dir).LiftDone {
		t.Fatal("первая запись ключа не поставила отметку")
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); !strings.Contains(string(data), `"liftDone": true`) {
		t.Fatalf("liftDone не в файле: %s", data)
	}
	// Вторая запись отметку не теряет.
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools = map[string]string{"autobuild": PermDeny} })
	if !LoadAgentPerms(dir).LiftDone {
		t.Fatal("вторая запись потеряла отметку")
	}
	// Файл 0.34: ключ есть, отметки нет — читается как «не выполнен»; запись fn его не воскрешает.
	writeRaw(t, dir, `{"agentPerms":{"edit":false}}`)
	if LoadAgentPerms(dir).LiftDone {
		t.Fatal("файл 0.34: отметка стоит")
	}
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.LiftDone = true })
	if !LoadAgentPerms(dir).LiftDone {
		t.Fatal("отметка не записалась")
	}
	// Мусор: не bool — чтение без паники, отметка не стоит.
	for _, raw := range []string{
		`{"agentPerms":{"liftDone":"yes","edit":false}}`, `{"agentPerms":{"liftDone":1,"edit":false}}`,
		`{"agentPerms":{"liftDone":null,"edit":false}}`, `{"agentPerms":{"liftDone":[],"edit":false}}`,
	} {
		writeRaw(t, dir, raw)
		if LoadAgentPerms(dir).LiftDone {
			t.Errorf("%s: отметка стоит", raw)
		}
	}
}

// ForgetPlace: заметка lifted с To = этому месту уходит, остальные целы; несколько заметок на одно место
// (разные From) уходят все; места без заметки и холостой ключ заметок не трогают.
func TestForgetPlaceLifted(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(s string) string { return filepath.Join(dir, s) }
	a, b := Place{Dir: at("A"), How: PlaceParent}, Place{Dir: at("B"), How: PlaceChosen}
	mk := func() AgentPerms {
		return AgentPerms{Places: []Place{a, b, {Dir: at("C"), How: PlaceParent}}, Lifted: []PlaceLift{
			{at("A/2026"), at("A")}, {at("A/2025"), at("A")}, {at("B/x"), at("B")},
		}}
	}
	p := mk()
	if !p.ForgetPlace(PlaceKey(a)) || len(p.Places) != 2 || len(p.Lifted) != 1 || p.Lifted[0].To != at("B") {
		t.Fatalf("✕ у A: места %+v, lifted %+v", p.Places, p.Lifted)
	}
	p = mk()
	if !p.ForgetPlace(PlaceKey(Place{Dir: at("C"), How: PlaceParent})) || len(p.Lifted) != 3 {
		t.Fatalf("✕ у места без заметки: %+v", p.Lifted)
	}
	p = mk()
	if p.ForgetPlace("нет такого") || len(p.Lifted) != 3 || len(p.Places) != 3 {
		t.Fatalf("холостой ключ изменил: %+v %+v", p.Places, p.Lifted)
	}
	// Последняя заметка: nil, а не пустой срез.
	p = AgentPerms{Places: []Place{a}, Lifted: []PlaceLift{{at("A/2026"), at("A")}}}
	p.ForgetPlace(PlaceKey(a))
	if p.Lifted != nil || len(p.Places) != 0 {
		t.Fatalf("последняя: %+v", p)
	}
	// Круг Save→Load через файл.
	UpdateAgentPerms(dir, func(v *AgentPerms) { *v = mk() })
	UpdateAgentPerms(dir, func(v *AgentPerms) { v.ForgetPlace(PlaceKey(a)) })
	if got := LoadAgentPerms(dir); len(got.Lifted) != 1 || got.Lifted[0].To != at("B") || len(got.Places) != 2 {
		t.Fatalf("круг: %+v", got)
	}
}
