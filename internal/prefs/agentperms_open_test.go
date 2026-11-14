package prefs

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestPermOpenClassO: open_album — класс O (0.36-160-1): «Всегда» есть и переживает чистку и запись, в счёт правок
// не входит; умолчания и откат 0.33 — как у Q; от выгрузки (Q) класс отличается.
func TestPermOpenClassO(t *testing.T) {
	if AgentToolClass("open_album") != ClassOpen || AgentToolClass("export_album") != ClassAskOnly {
		t.Fatalf("классы: open=%q export=%q", AgentToolClass("open_album"), AgentToolClass("export_album"))
	}
	if got := PermCan("open_album"); !slices.Equal(got, []string{PermAllow, PermAsk, PermDeny}) {
		t.Errorf("PermCan(open_album) = %v", got)
	}
	for v, want := range map[string]string{PermAllow: PermAllow, PermAsk: PermAsk, PermDeny: PermDeny, "": PermDeny, "мусор": PermDeny, "ALLOW": PermDeny} {
		if got := cleanMode("open_album", v); got != want {
			t.Errorf("cleanMode(%q) = %q, ждали %q", v, got, want)
		}
	}
	// Умолчания для имени, которого нет в записи: как у Q — «Спрашивать» при живой правке, иначе «Запретить»;
	// «Всегда» не появляется само.
	if newToolMode(ClassOpen, map[string]string{"autobuild": PermAsk}) != PermAsk || newToolMode(ClassOpen, nil) != PermDeny {
		t.Error("newToolMode(O)")
	}
	if newToolMode(ClassOpen, map[string]string{"open_album": PermAllow, "get_album": PermAllow}) != PermDeny {
		t.Error("O и чтение в счёт правок не входят")
	}
	if column(false)["open_album"] != PermDeny || column(true)["open_album"] != PermAsk {
		t.Errorf("столбцы: %q / %q", column(false)["open_album"], column(true)["open_album"])
	}

	// Круг записи → чтение: «Всегда» остаётся, соседние инструменты не задеты; повторная запись её не теряет.
	dir := testkit.TempDir(t)
	want := UpdateAgentPerms(dir, func(p *AgentPerms) {
		p.Tools["open_album"], p.Tools["export_album"] = PermAllow, PermAsk
	})
	wantModes(t, "запись", want, map[string]string{"open_album": PermAllow, "export_album": PermAsk})
	RememberDir(dir, DirPhotos, dir)
	wantModes(t, "чтение", LoadAgentPerms(dir), map[string]string{"open_album": PermAllow, "export_album": PermAsk})
	wantModes(t, "повторная запись", UpdateAgentPerms(dir, func(*AgentPerms) {}), map[string]string{"open_album": PermAllow})
	// «Всегда» у выгрузки в том же файле (правка мимо PermCan) — «Спрашивать»: O на Q не влияет.
	writeRaw(t, dir, `{"agentPerms":{"tools":{"export_album":"allow","open_album":"allow"}}}`)
	wantModes(t, "allow в файле", LoadAgentPerms(dir), map[string]string{"export_album": PermAsk, "open_album": PermAllow})
}

// TestPermOpenRollback033: флажок 0.33 выключили — «Всегда» у открытия не переживает откат (O → «Запретить», как Q);
// вернули без записи 0.34 — «Всегда» как записано; 0.33 без agentPerms с флажком — «Спрашивать».
func TestPermOpenRollback033(t *testing.T) {
	dir := testkit.TempDir(t)
	saved := UpdateAgentPerms(dir, func(p *AgentPerms) {
		for _, k := range []string{"autobuild", "vary_layouts", "undo_last_edit"} {
			p.Tools[k] = PermAllow
		}
		p.Tools["add_folder"], p.Tools["create_album"] = PermAsk, PermAsk
		p.Tools["open_album"] = PermAllow
	})
	if !saved.Edit {
		t.Fatalf("зеркало не включило правки: %+v", saved)
	}
	write033(t, dir, false)
	wantModes(t, "флажок выключили в 0.33", LoadAgentPerms(dir), map[string]string{"open_album": PermDeny})
	write033(t, dir, true)
	wantModes(t, "флажок вернули", LoadAgentPerms(dir), map[string]string{"open_album": PermAllow})
	dir = testkit.TempDir(t)
	write033(t, dir, true)
	wantModes(t, "0.33 с флажком", LoadAgentPerms(dir), map[string]string{"open_album": PermAsk})
}
