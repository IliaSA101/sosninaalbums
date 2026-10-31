package prefs

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var askOnlyTools = []string{"export_album", "open_album"}

// TestPermAskOnlyClass: класс Q (0.35-152-2/-11) — «Всегда» нет ни в PermCan, ни после чистки;
// свежий столбец и столбец «0.33, правки вкл.» — «Запретить» / «Спрашивать».
func TestPermAskOnlyClass(t *testing.T) {
	if isEditClass(ClassAskOnly) {
		t.Fatal("Q не входит в классы правок")
	}
	for _, k := range askOnlyTools {
		if AgentToolClass(k) != ClassAskOnly {
			t.Fatalf("%s: класс %q", k, AgentToolClass(k))
		}
		if got := PermCan(k); !slices.Equal(got, []string{PermAsk, PermDeny}) {
			t.Errorf("%s: PermCan %v", k, got)
		}
		if got := column(false)[k]; got != PermDeny {
			t.Errorf("%s: column(false) %q", k, got)
		}
		if got := column(true)[k]; got != PermAsk {
			t.Errorf("%s: column(true) %q", k, got)
		}
		for v, want := range map[string]string{PermAllow: PermAsk, PermAsk: PermAsk, PermDeny: PermDeny, "": PermDeny, "мусор": PermDeny, "ALLOW": PermDeny} {
			if got := cleanMode(k, v); got != want {
				t.Errorf("%s: cleanMode(%q) = %q, ждали %q", k, v, got, want)
			}
		}
	}
	// Чужое имя и чтение: прежнее правило.
	if cleanMode("tool_from_future", PermAllow) != PermAllow || cleanMode("get_album", PermAsk) != PermDeny {
		t.Error("cleanMode ушёл от прежнего правила для не-Q")
	}
	// newToolMode: есть живая правка — «Спрашивать»; все правки «Запретить» или записи нет — «Запретить».
	if newToolMode(ClassAskOnly, map[string]string{"autobuild": PermDeny, "add_folder": PermAsk}) != PermAsk {
		t.Error("newToolMode(Q) при живой правке")
	}
	if newToolMode(ClassAskOnly, map[string]string{"autobuild": PermDeny}) != PermDeny || newToolMode(ClassAskOnly, nil) != PermDeny {
		t.Error("newToolMode(Q) без живых правок")
	}
	if newToolMode(ClassAskOnly, map[string]string{"export_album": PermAsk, "get_album": PermAllow}) != PermDeny {
		t.Error("Q и чтение в счёт правок не входят")
	}
	// Правка config.json «Всегда»: чтение сверенного вида даёт «Спрашивать», запись сохраняет её.
	dir := testkit.TempDir(t)
	writeRaw(t, dir, `{"agentPerms":{"tools":{"export_album":"allow","open_album":"allow"}}}`)
	got := LoadAgentPerms(dir)
	wantModes(t, "allow в файле", got, map[string]string{"export_album": PermAsk, "open_album": PermAsk})
	if got = UpdateAgentPerms(dir, func(*AgentPerms) {}); got.Tools["export_album"] != PermAsk {
		t.Errorf("после записи: %q", got.Tools["export_album"])
	}
}

// TestPermRollbackQ033: конфиг 0.33 без agentPerms — столбец; конфиг 0.34 + флажок выключили в 0.33 —
// Q «Запретить»; включили — Q как записано.
func TestPermRollbackQ033(t *testing.T) {
	dir := testkit.TempDir(t)
	write033(t, dir, true)
	wantModes(t, "0.33 без agentPerms, флажок вкл.", LoadAgentPerms(dir), map[string]string{"export_album": PermAsk, "open_album": PermAsk})
	write033(t, dir, false)
	wantModes(t, "0.33 без agentPerms, флажок выкл.", LoadAgentPerms(dir), map[string]string{"export_album": PermDeny, "open_album": PermDeny})

	dir = testkit.TempDir(t)
	saved := UpdateAgentPerms(dir, func(p *AgentPerms) {
		for _, k := range []string{"autobuild", "vary_layouts", "undo_last_edit"} {
			p.Tools[k] = PermAllow
		}
		p.Tools["add_folder"], p.Tools["create_album"] = PermAsk, PermAsk
		p.Tools["export_album"], p.Tools["open_album"] = PermAsk, PermDeny
	})
	if !saved.Edit {
		t.Fatalf("зеркало не включило правки: %+v", saved)
	}
	write033(t, dir, false)
	wantModes(t, "флажок выключили в 0.33", LoadAgentPerms(dir), map[string]string{"export_album": PermDeny, "open_album": PermDeny})
	// Возврат флажка до записи 0.34 — Q как было.
	write033(t, dir, true)
	wantModes(t, "флажок вернули", LoadAgentPerms(dir), map[string]string{"export_album": PermAsk, "open_album": PermDeny})

	// rollback(true) Q не трогает: 0.34 записала edit=false и Q «Спрашивать», 0.33 включила флажок.
	dir = testkit.TempDir(t)
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["export_album"] = PermAsk })
	write033(t, dir, true)
	wantModes(t, "0.33 включила", LoadAgentPerms(dir), map[string]string{"export_album": PermAsk, "open_album": PermDeny})
}

// TestPermNewToolAfterGroupDeny: запись 0.34, где Q ещё нет. После «Правки: Запретить» новые Q —
// «Запретить»; пока есть живая правка — «Спрашивать»; записанное не перетирается.
func TestPermNewToolAfterGroupDeny(t *testing.T) {
	denyEdits := func(p *AgentPerms) {
		for k, c := range toolClass {
			if isEditClass(c) {
				p.Tools[k] = PermDeny
			}
		}
	}
	for _, c := range []struct {
		name string
		edit func(*AgentPerms)
		want string
	}{
		{"правки запрещены", denyEdits, PermDeny},
		{"живая автосборка", func(p *AgentPerms) { denyEdits(p); p.Tools["autobuild"] = PermAllow }, PermAsk},
		{"живая только папка", func(p *AgentPerms) { denyEdits(p); p.Tools["add_folder"] = PermAsk }, PermAsk},
	} {
		dir := testkit.TempDir(t)
		UpdateAgentPerms(dir, c.edit)
		// Файл, как его оставила 0.34: имён Q в tools нет.
		UpdateConfig(dir, func(cfg *Config) {
			for _, k := range askOnlyTools {
				delete(cfg.AgentPerms.Tools, k)
			}
		})
		wantModes(t, c.name, LoadAgentPerms(dir), map[string]string{"export_album": c.want, "open_album": c.want})
	}
	// Записанное «Запретить» при живых правках не превращается в «Спрашивать».
	dir := testkit.TempDir(t)
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["autobuild"], p.Tools["export_album"] = PermAllow, PermDeny })
	wantModes(t, "записанное", LoadAgentPerms(dir), map[string]string{"export_album": PermDeny})
}

// TestAgentsRoundTripQ034: круг записи → чтение → запись; Q со «Спрашивать» и «Запретить» и имя
// новее (0.34 хранит незнакомые) переживают запись 0.33 (RememberDir) и повторную запись.
func TestAgentsRoundTripQ034(t *testing.T) {
	dir := testkit.TempDir(t)
	want := UpdateAgentPerms(dir, func(p *AgentPerms) {
		p.Tools["autobuild"] = PermAllow
		p.Tools["export_album"], p.Tools["open_album"] = PermAsk, PermDeny
		p.Tools["export_album_next"] = PermAsk
	})
	if want.Tools["export_album"] != PermAsk || want.Tools["open_album"] != PermDeny {
		t.Fatalf("запись: %+v", want.Tools)
	}
	RememberDir(dir, DirPhotos, dir)
	wantModes(t, "чтение", LoadAgentPerms(dir), map[string]string{"export_album": PermAsk, "open_album": PermDeny, "export_album_next": PermAsk})
	again := UpdateAgentPerms(dir, func(*AgentPerms) {})
	wantModes(t, "повторная запись", again, map[string]string{"export_album": PermAsk, "open_album": PermDeny, "export_album_next": PermAsk})
	// «Всегда» в файле из круга не возвращается.
	writeRaw(t, dir, `{"agentPerms":{"tools":{"export_album":"allow"},"edit":false}}`)
	if UpdateAgentPerms(dir, func(*AgentPerms) {}).Tools["export_album"] != PermAsk {
		t.Error("allow пережил запись")
	}
	// Пустая карта tools: умолчание, не паника.
	writeRaw(t, dir, `{"agentPerms":{"tools":{}}}`)
	wantModes(t, "пустые tools", LoadAgentPerms(dir), map[string]string{"export_album": PermDeny, "open_album": PermDeny})
}
