package app

// Разрешения открытия (160 Т1): класс O не меняется групповым щелчком; причины hint и pick в вопросе при «Всегда».
// Служебная часть (слот только у ключа) — internal/app/agents/perms_open_test.go.

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestPermGroupSkipsOpen: групповой «Правки» и «Чтение» строку «Открыть альбом» не меняют — ни «Всегда», ни «Спрашивать»,
// ни «Запретить». Контроль: create_album той же группой меняется (тест не зелёный просто от пустой группы).
func TestPermGroupSkipsOpen(t *testing.T) {
	a, _ := pubApp(t)
	a.SetAgentPerm("open_album", prefs.PermAllow)
	a.SetAgentPerm("export_album", prefs.PermAsk)
	for _, mode := range []string{prefs.PermAsk, prefs.PermDeny, prefs.PermAllow} {
		a.SetAgentPermGroup("edit", mode)
		a.SetAgentPermGroup("read", prefs.PermDeny)
		a.SetAgentPermGroup("other", mode)
		p := prefs.LoadAgentPerms(a.cfgDir)
		if p.Tools["open_album"] != prefs.PermAllow || p.Tools["export_album"] != prefs.PermAsk {
			t.Fatalf("группа %q тронула открытие/выгрузку: open=%q export=%q", mode, p.Tools["open_album"], p.Tools["export_album"])
		}
		if p.Tools["create_album"] != mode {
			t.Fatalf("контроль: create_album %q, ждали %q", p.Tools["create_album"], mode)
		}
	}
	// Строка в списке окна: группа other, три значения.
	var row *AgentPerm
	for _, r := range a.AgentsInfo().Perms {
		if r.Tool == "open_album" {
			row = &r
		}
	}
	if row == nil || row.Group != "other" || row.Mode != prefs.PermAllow || len(row.Can) != 3 || row.Why != "" || row.Note != permNoteOpen {
		t.Errorf("строка open_album: %+v", row)
	}
}

// TestOpenCallWhy: при «Всегда» вопрос открытия несёт причину по самому вызову — слова агента (hint), ни ключа, ни найденного
// альбома (pick); ключ без слов и любой вызов при «Спрашивать» — без причины (дальше решает проверка путей, 160 Т2).
func TestOpenCallWhy(t *testing.T) {
	r := newOpenRig(t)
	key := r.req(r.target)
	hinted := key
	hinted.Hint = "Ивановых"
	for i, c := range []struct {
		name string
		mode string
		req  agents.AskReq
		want string
	}{
		{"ключ", prefs.PermAllow, key, ""},
		{"слова", prefs.PermAllow, hinted, whyHint},
		{"ничего", prefs.PermAllow, agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"}, whyPick},
		{"ключ, спрашивать", prefs.PermAsk, key, ""},
		{"слова, спрашивать", prefs.PermAsk, hinted, ""},
		{"ничего, спрашивать", prefs.PermAsk, agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"}, ""},
	} {
		prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) { p.Tools["open_album"] = c.mode })
		call := r.ask(c.req)
		ev := r.asked(t, i+1)
		if ev.Why != c.want {
			t.Errorf("%s: Why = %q, ждали %q", c.name, ev.Why, c.want)
		}
		if _, err := r.a.AgentAnswerOpen(ev.ID, false); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		call.wait(t)
	}
}
