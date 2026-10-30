package app

// Разрешения агентов, фасад (150, Т2): строки окна из таблицы службы, Host.Perm из config.json на каждый
// вызов, общий вход правок и событие «запрещено».

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestAgentPermRows: строки окна — инструменты службы с названиями и запуск программы; значения —
// сверенный вид prefs (миграция флажка 0.33), группы и допустимые значения по классу.
func TestAgentPermRows(t *testing.T) {
	a := newApp(t)
	rows := a.AgentsInfo().Perms
	by := map[string]AgentPerm{}
	for _, r := range rows {
		by[r.Tool] = r
		if r.Title == "" || prefs.AgentToolClass(r.Tool) == "" || !slices.Equal(r.Can, prefs.PermCan(r.Tool)) {
			t.Errorf("строка %+v: нет названия, класса в prefs или Can разошёлся", r)
		}
		if !slices.Contains(r.Can, r.Mode) {
			t.Errorf("%s: режим %q вне Can %v", r.Tool, r.Mode, r.Can)
		}
	}
	if len(rows) != len(a.agents.Tools())+1 || len(by) != len(rows) {
		t.Fatalf("строк %d, инструментов службы %d (+ запуск), без повторов %d", len(rows), len(a.agents.Tools()), len(by))
	}
	for tool, want := range map[string]struct{ group, mode string }{
		"get_album": {"read", "allow"}, "get_contact_sheet": {"read", "allow"}, "show_spread": {"other", "allow"},
		"start_program": {"other", "allow"}, "autobuild": {"edit", "deny"}, "add_folder": {"edit", "deny"}, "create_album": {"edit", "deny"},
	} {
		if r := by[tool]; r.Group != want.group || r.Mode != want.mode {
			t.Errorf("%s: группа %q режим %q, ждали %+v", tool, r.Group, r.Mode, want)
		}
	}
	if r := by["start_program"]; r.Title != "Запустить программу" || r.Why == "" || slices.Contains(r.Can, prefs.PermAsk) {
		t.Errorf("start_program: %+v", r)
	}
	if r := by["get_album"]; r.Why == "" || slices.Contains(r.Can, prefs.PermAsk) {
		t.Errorf("get_album: причина, почему «Спрашивать» нет: %+v", r)
	}
	if r := by["autobuild"]; r.Why != "" || !slices.Contains(r.Can, prefs.PermAsk) {
		t.Errorf("autobuild: %+v", r)
	}
	if by["get_photo_preview"].Note == "" || by["create_album"].Note == "" {
		t.Error("нет пояснений под строками картинок и папки")
	}
	// Флажок 0.33 включён и ключа agentPerms нет: правки «Всегда», папка и альбом «Спрашивать».
	prefs.UpdateAgents(a.cfgDir, func(c *prefs.Agents) { c.Edit = true })
	by = map[string]AgentPerm{}
	for _, r := range a.AgentsInfo().Perms {
		by[r.Tool] = r
	}
	if by["autobuild"].Mode != "allow" || by["add_folder"].Mode != "ask" || by["create_album"].Mode != "ask" {
		t.Errorf("миграция флажка: %+v %+v %+v", by["autobuild"], by["add_folder"], by["create_album"])
	}
	// Смена строки видна в следующем AgentsInfo.
	if got := a.SetAgentPerm("show_spread", prefs.PermAsk).Perms; !slices.ContainsFunc(got, func(r AgentPerm) bool { return r.Tool == "show_spread" && r.Mode == "ask" }) {
		t.Error("SetAgentPerm не виден в Perms")
	}
}

// TestAgentPermHost: Host.Perm читает config.json на каждый вызов; незаписанный инструмент — умолчание
// для нового; процесс от администратора — blocked; общий вход правок — «не все правки запрещены».
func TestAgentPermHost(t *testing.T) {
	a := newApp(t)
	if m, b := a.agentPerm("autobuild"); m != "deny" || b != "" {
		t.Fatalf("новый: %q %q", m, b)
	}
	if on, _ := a.agentEditState(); on {
		t.Fatal("новый пользователь: правки разрешены")
	}
	a.SetAgentPerm("show_spread", prefs.PermAsk) // показ в группу правок не входит
	if on, _ := a.agentEditState(); on {
		t.Fatal("«Спрашивать» у показа открыло правки")
	}
	a.SetAgentPerm("apply_print", prefs.PermAsk)
	if m, _ := a.agentPerm("apply_print"); m != "ask" {
		t.Fatalf("apply_print: %q", m)
	}
	if on, _ := a.agentEditState(); !on {
		t.Fatal("одна правка «Спрашивать» — правки возможны")
	}
	a.SetAgentPerm("apply_print", prefs.PermDeny)
	if on, _ := a.agentEditState(); on {
		t.Fatal("все правки запрещены, а правки возможны")
	}
	if m, _ := a.agentPerm("get_album"); m != "allow" {
		t.Fatalf("get_album: %q", m)
	}
	if m, _ := a.agentPerm("future_tool"); m != "deny" { // правка новее записи, все прочие «Запретить»
		t.Fatalf("незнакомый инструмент: %q", m)
	}
	a.askM().elevated = func() bool { return true }
	if _, b := a.agentPerm("autobuild"); b != agentBlockedElevated {
		t.Fatalf("администратор: %q", b)
	}
	if on, b := a.agentEditState(); on || b != agentBlockedElevated {
		t.Fatalf("администратор: %v %q", on, b)
	}
}

// TestAgentsDeniedEvent: запрет по инструменту → agents:denied с названием; неизвестный клиент — «Агент».
func TestAgentsDeniedEvent(t *testing.T) {
	a, events := pubApp(t)
	a.agentsDenied("", "create_album", "Создать альбом")
	a.agentsDenied("Claude Code", "get_album", "Сводка альбома")
	var got []AgentDenied
	for _, e := range events() {
		if e.name == "agents:denied" {
			got = append(got, e.data.(AgentDenied))
		}
	}
	want := []AgentDenied{{Client: "Агент", Tool: "create_album", Title: "Создать альбом"}, {Client: "Claude Code", Tool: "get_album", Title: "Сводка альбома"}}
	if !slices.Equal(got, want) {
		t.Fatalf("события: %+v", got)
	}
}
