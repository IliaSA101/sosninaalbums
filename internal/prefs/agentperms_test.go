package prefs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// writeRaw кладёт config.json как есть (правка руками, чужая версия).
func writeRaw(t *testing.T, dir, raw string) {
	t.Helper()
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}

// write033 — запись программы 0.33: меняет только agents.*, ключа agentPerms не знает и сохраняет
// его байты как есть (так делает UpdateConfigErr 0.33 со всеми незнакомыми ключами верхнего уровня).
func write033(t *testing.T, dir string, edit bool) {
	t.Helper()
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(ConfigPath(dir)); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	ag := map[string]any{"level": 1}
	if r, ok := raw["agents"]; ok {
		_ = json.Unmarshal(r, &ag)
	}
	ag["edit"] = edit
	b, _ := json.Marshal(ag)
	raw["agents"] = b
	out, _ := json.MarshalIndent(raw, "", "  ")
	writeRaw(t, dir, string(out))
}

// fileFlags — agents.edit и agentPerms.edit из файла.
func fileFlags(t *testing.T, dir string) (agentsEdit, permsEdit bool) {
	t.Helper()
	var f struct {
		Agents     struct{ Edit bool }
		AgentPerms struct{ Edit bool }
	}
	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("%v: %s", err, data)
	}
	return f.Agents.Edit, f.AgentPerms.Edit
}

func wantModes(t *testing.T, ctx string, p AgentPerms, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if p.Tools[k] != v {
			t.Errorf("%s: %s = %q, ждали %q", ctx, k, p.Tools[k], v)
		}
	}
}

// TestAgentsToolsMigrate: нет ключа agentPerms — столбец по agents.edit (design §1), чтение файл
// не меняет и не создаёт, каждый известный инструмент получает значение.
func TestAgentsToolsMigrate(t *testing.T) {
	dir := testkit.TempDir(t)
	fresh := LoadAgentPerms(dir)
	if _, err := os.Stat(ConfigPath(dir)); err == nil {
		t.Fatal("чтение создало config.json")
	}
	if fresh.Edit || len(fresh.Tools) != len(AgentToolNames()) || fresh.Places != nil {
		t.Fatalf("новый: %+v", fresh)
	}
	for _, k := range AgentToolNames() {
		want := PermAllow
		if isEditClass(AgentToolClass(k)) {
			want = PermDeny
		}
		if fresh.Tools[k] != want {
			t.Errorf("новый: %s = %q, ждали %q", k, fresh.Tools[k], want)
		}
	}

	// 0.33 без флажка — как новый; с флажком — «правки вкл.».
	write033(t, dir, false)
	if got := LoadAgentPerms(dir); got.Edit || got.Tools["autobuild"] != PermDeny {
		t.Fatalf("0.33 без флажка: %+v", got)
	}
	write033(t, dir, true)
	before, _ := os.ReadFile(ConfigPath(dir))
	on := LoadAgentPerms(dir)
	wantModes(t, "0.33 с флажком", on, map[string]string{
		"get_album": PermAllow, "get_contact_sheet": PermAllow, "start_program": PermAllow, "show_spread": PermAllow,
		"autobuild": PermAllow, "vary_layouts": PermAllow, "undo_last_edit": PermAllow,
		"set_marks": PermAllow, "set_cover_photo": PermAllow, "set_background": PermAllow, "add_sticker": PermAllow,
		"apply_print": PermAsk, "add_folder": PermAsk, "create_album": PermAsk,
	})
	if !on.Edit {
		t.Fatal("Edit не равен agents.edit")
	}
	if after, _ := os.ReadFile(ConfigPath(dir)); string(after) != string(before) {
		t.Fatalf("чтение изменило файл:\n%s\n%s", before, after)
	}

	// Первая запись: миграция ложится в файл целиком, зеркало сходится с флажком.
	got := UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["show_spread"] = PermAsk })
	if got.Tools["show_spread"] != PermAsk || got.Tools["autobuild"] != PermAllow || !got.Edit {
		t.Fatalf("после записи: %+v", got)
	}
	if a, p := fileFlags(t, dir); !a || !p {
		t.Fatalf("флаги в файле: agents.edit=%v perms.edit=%v", a, p)
	}
	if again := LoadAgentPerms(dir); fmt.Sprint(again) != fmt.Sprint(got) {
		t.Fatalf("круг Save→Load:\n%+v\n%+v", again, got)
	}

	// Инструмент новее записи: правка — «Спрашивать», пока не все правки «Запретить».
	if got.NewMode(true) != PermAllow || got.NewMode(false) != PermAsk {
		t.Fatalf("NewMode: %q %q", got.NewMode(true), got.NewMode(false))
	}
	off := LoadAgentPerms(testkit.TempDir(t))
	if off.NewMode(false) != PermDeny {
		t.Fatalf("все правки «Запретить» — новая тоже: %q", off.NewMode(false))
	}
	// Известный инструмент, которого нет в записанной карте, получает то же правило.
	writeRaw(t, dir, `{"agentPerms":{"tools":{"autobuild":"allow","get_album":"deny"},"edit":false}}`)
	p := LoadAgentPerms(dir)
	wantModes(t, "недостающие", p, map[string]string{"get_album": PermDeny, "list_photos": PermAllow, "set_marks": PermAsk, "create_album": PermAsk})
	// Пустой объект: записано «ничего» — правки «Запретить», чтение «Всегда».
	writeRaw(t, dir, `{"agentPerms":{}}`)
	wantModes(t, "пустой ключ", LoadAgentPerms(dir), map[string]string{"autobuild": PermDeny, "add_folder": PermDeny, "get_album": PermAllow})
}

// TestAgentsToolsClean: чистка — только строение; диск не трогается; мусор не паникует.
func TestAgentsToolsClean(t *testing.T) {
	dir := testkit.TempDir(t)
	root := filepath.Join(dir, "нет", "такой", "папки") // на диске её нет: clean диск не смотрит
	tools := map[string]string{
		"autobuild": "ALLOW", "vary_layouts": "", "undo_last_edit": "всегда", "add_folder": PermAsk,
		"get_album": PermAsk, "start_program": PermAsk, "get_photo": PermDeny, "show_spread": PermAsk,
		"future_tool": PermAsk, "Bad": PermAllow, "has1digit": PermAllow, "": PermAllow,
		strings.Repeat("a", 65): PermAllow, strings.Repeat("a", 64): PermAllow,
	}
	long := filepath.Join(root, strings.Repeat("д", 600)) // > 1 КБ в байтах
	places := []Place{
		{root, PlaceChosen}, {root, PlaceParent}, {strings.ToUpper(root), PlaceParent},
		{"относительный/путь", PlaceChosen}, {"", PlaceChosen}, {long, PlaceChosen},
		{filepath.Join(root, "x"), "other"}, {filepath.Join(root, "y"), ""},
		{filepath.Join(root, "bad\x00nul"), PlaceChosen}, {filepath.Join(root, "bad\xffutf"), PlaceChosen},
		{filepath.Join(root, "tab\there"), PlaceChosen}, {root + string(filepath.Separator), PlaceParent},
	}
	got := AgentPerms{Tools: tools, Places: places}.clean()
	wantModes(t, "чистка", got, map[string]string{
		"autobuild": PermDeny, "vary_layouts": PermDeny, "undo_last_edit": PermDeny, "add_folder": PermAsk,
		"get_album": PermDeny, "start_program": PermDeny, "get_photo": PermDeny, "show_spread": PermAsk,
		"future_tool": PermAsk, strings.Repeat("a", 64): PermAllow,
	})
	for _, bad := range []string{"Bad", "has1digit", "", strings.Repeat("a", 65)} {
		if _, ok := got.Tools[bad]; ok {
			t.Errorf("имя %q осталось", bad)
		}
	}
	// Регистр папки на Windows и macOS не важен: ВЕРХНИЙ регистр — повтор; на Linux — другое место.
	wantPlaces := 1
	if runtime.GOOS == "linux" {
		wantPlaces = 2
	}
	if len(got.Places) != wantPlaces || got.Places[0] != (Place{root, PlaceChosen}) {
		t.Fatalf("места: %+v", got.Places)
	}
	if _, err := os.Stat(root); err == nil {
		t.Fatal("clean создал папку")
	}

	// Больше 10 мест — первые 10; чужих имён не больше 32 сверх известных, результат стабилен.
	many := make([]Place, 0, 40)
	for i := range 40 {
		many = append(many, Place{filepath.Join(dir, fmt.Sprintf("m%02d", i)), PlaceParent})
	}
	big := map[string]string{}
	for i := range 200 {
		big[fmt.Sprintf("future_%c%c", 'a'+i/26, 'a'+i%26)] = PermAsk
	}
	c1 := AgentPerms{Tools: big, Places: many}.clean()
	c2 := AgentPerms{Tools: big, Places: many}.clean()
	if len(c1.Places) != MaxPlaces || c1.Places[0].Dir != many[0].Dir || len(c1.Tools) != maxUnknownTools || fmt.Sprint(c1) != fmt.Sprint(c2) {
		t.Fatalf("пределы: места %d, имена %d", len(c1.Places), len(c1.Tools))
	}
	// Пусто и nil: ни паники, ни карты с пустышками.
	if e := (AgentPerms{}).clean(); e.Tools != nil || e.Places != nil {
		t.Fatalf("пусто: %+v", e)
	}
	// Чистка копирует: правка результата не трогает источник.
	src := AgentPerms{Tools: map[string]string{"autobuild": PermAllow}, Places: []Place{{root, PlaceChosen}}}
	cp := src.clean()
	cp.Tools["autobuild"] = PermDeny
	cp.Places[0].How = PlaceParent
	if src.Tools["autobuild"] != PermAllow || src.Places[0].How != PlaceChosen {
		t.Fatal("clean поделился картой или срезом")
	}

	// Мусор в файле: не те типы, null, огромные строки, «Запретить» вместо паники.
	for _, raw := range []string{
		`{"agentPerms":5}`, `{"agentPerms":"x"}`, `{"agentPerms":null}`, `{"agentPerms":[]}`,
		`{"agentPerms":{"tools":"x"}}`, `{"agentPerms":{"tools":{"autobuild":5,"add_folder":null}}}`,
		`{"agentPerms":{"places":{"a":1}}}`, `{"agentPerms":{"places":[1,"x",null,{"dir":5}]}}`,
		`{"agentPerms":{"edit":"да"}}`, `{"agentPerms":{"tools":{"autobuild":"` + strings.Repeat("я", 5000) + `"}}}`,
	} {
		writeRaw(t, dir, raw)
		p := LoadAgentPerms(dir)
		if p.Tools["autobuild"] == PermAllow {
			t.Errorf("%.50s: autobuild открыт мусором", raw)
		}
		if p.Tools["get_album"] == "" {
			t.Errorf("%.50s: нет значения у get_album", raw)
		}
	}
}

// TestAgentsEditMirrorStrict: agents.edit = autobuild, vary_layouts, undo_last_edit «Всегда» и
// add_folder, create_album не «Запретить»; «Спрашивать» у любой из первых трёх флажок выключает.
func TestAgentsEditMirrorStrict(t *testing.T) {
	dir := testkit.TempDir(t)
	set := func(kv ...string) AgentPerms {
		return UpdateAgentPerms(dir, func(p *AgentPerms) {
			for i := 0; i < len(kv); i += 2 {
				p.Tools[kv[i]] = kv[i+1]
			}
		})
	}
	check := func(ctx string, want bool) {
		t.Helper()
		a, p := fileFlags(t, dir)
		if a != want || p != want || LoadAgentPerms(dir).Edit != want || AgentsOf(dir).Edit != want {
			t.Errorf("%s: agents.edit=%v perms.edit=%v, ждали %v", ctx, a, p, want)
		}
	}
	set("show_spread", PermAsk)
	check("все правки «Запретить»", false)
	set("autobuild", PermAllow, "vary_layouts", PermAllow, "undo_last_edit", PermAllow)
	check("папка и альбом «Запретить»", false)
	set("add_folder", PermAsk, "create_album", PermAsk)
	check("пять правок как в 0.33", true)
	set("create_album", PermAllow)
	check("папка «Спрашивать», альбом «Всегда»", true)
	for _, tool := range []string{"autobuild", "vary_layouts", "undo_last_edit"} {
		set(tool, PermAsk)
		check(tool+" «Спрашивать»", false)
		set(tool, PermAllow)
		check(tool+" снова «Всегда»", true)
	}
	set("add_folder", PermDeny)
	check("папка «Запретить»", false)
	set("add_folder", PermAsk, "set_marks", PermDeny, "apply_print", PermAsk)
	check("правки 0.34 зеркало не трогают", true)
	// Зеркало не стирает остальной agents: уровень и On целы, ключ agents при умолчании исчезает.
	UpdateAgents(dir, func(a *Agents) { a.On, a.Level = true, 3 })
	set("autobuild", PermDeny)
	if a := AgentsOf(dir); !a.On || a.Level != 3 || a.Edit {
		t.Fatalf("agents после зеркала: %+v", a)
	}
	UpdateAgents(dir, func(a *Agents) { *a = Agents{} })
	set("show_spread", PermAllow)
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), `"agents"`) {
		t.Fatalf("умолчание agents осталось в файле: %s", data)
	}
}

// TestAgentsRoundTrip033: версия 0.33 в том же config.json — права не шире прежнего выбора.
func TestAgentsRoundTrip033(t *testing.T) {
	t.Run("запись 0.33 не меняет права", rt033Same)
	t.Run("флажок выключен в 0.33", rt033Off)
	t.Run("флажок включён в 0.33", rt033On)
	t.Run("запись одной строки после отката", rt033OneRow)
	t.Run("чужие ключи", rt033Foreign)
	t.Run("сбой записи", rt033Fail)
}

// 0.34 записала права, 0.33 переписала файл своей записью (RememberDir) — права те же.
func rt033Same(t *testing.T) {
	dir := testkit.TempDir(t)
	want := UpdateAgentPerms(dir, func(p *AgentPerms) {
		p.Tools["autobuild"], p.Tools["vary_layouts"], p.Tools["undo_last_edit"] = PermAllow, PermAllow, PermAllow
		p.Tools["add_folder"], p.Tools["create_album"] = PermAsk, PermAsk
		p.Tools["set_marks"] = PermDeny
		p.Places = []Place{{filepath.Join(dir, "Фото"), PlaceChosen}}
	})
	RememberDir(dir, DirPhotos, dir)
	write033(t, dir, true) // 0.33 пишет то же значение флажка
	if got := LoadAgentPerms(dir); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("права изменились:\n%+v\n%+v", got, want)
	}
}

// Флажок выключили в 0.33: правки, папка, альбом — «Запретить» (в том числе правки 0.34);
// чтение, места и start_program целы. Включили обратно до записи 0.34 — права как были.
func rt033Off(t *testing.T) {
	dir := testkit.TempDir(t)
	place := Place{filepath.Join(dir, "Фото"), PlaceParent}
	saved := UpdateAgentPerms(dir, func(p *AgentPerms) {
		for _, k := range []string{"autobuild", "vary_layouts", "undo_last_edit", "set_marks"} {
			p.Tools[k] = PermAllow
		}
		p.Tools["add_folder"], p.Tools["create_album"], p.Tools["apply_print"] = PermAsk, PermAsk, PermAsk
		p.Tools["get_photo_preview"] = PermDeny
		p.Places = []Place{place}
	})
	if !saved.Edit {
		t.Fatal("зеркало не включило правки")
	}
	write033(t, dir, false)
	off := LoadAgentPerms(dir)
	for _, k := range AgentToolNames() {
		if isEditClass(AgentToolClass(k)) && off.Tools[k] != PermDeny {
			t.Errorf("%s после выключения в 0.33: %q", k, off.Tools[k])
		}
	}
	wantModes(t, "чтение", off, map[string]string{"get_photo_preview": PermDeny, "get_album": PermAllow, "start_program": PermAllow})
	if off.Edit || len(off.Places) != 1 || off.Places[0] != place {
		t.Fatalf("Edit/места: %+v", off)
	}
	write033(t, dir, true)
	if back := LoadAgentPerms(dir); fmt.Sprint(back) != fmt.Sprint(saved) {
		t.Fatalf("возврат флажка до записи 0.34 изменил права:\n%+v\n%+v", back, saved)
	}
}

// 0.34 записала edit=false; 0.33 включила флажок: по столбцу только пять правок, остальное — как записано.
func rt033On(t *testing.T) {
	dir := testkit.TempDir(t)
	saved := UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["get_photo"] = PermDeny })
	if saved.Edit || saved.Tools["set_marks"] != PermDeny || saved.Tools["apply_print"] != PermDeny {
		t.Fatalf("исходное: %+v", saved)
	}
	write033(t, dir, true)
	on := LoadAgentPerms(dir)
	wantModes(t, "включение", on, map[string]string{
		"autobuild": PermAllow, "vary_layouts": PermAllow, "undo_last_edit": PermAllow,
		"add_folder": PermAsk, "create_album": PermAsk,
		"set_marks": PermDeny, "set_cover_photo": PermDeny, "set_background": PermDeny, "add_sticker": PermDeny,
		"apply_print": PermDeny, "get_photo": PermDeny, "show_spread": PermAllow,
	})
	if !on.Edit {
		t.Fatal("Edit должен совпасть с agents.edit")
	}
	// «Спрашивать» у правки 0.34 тоже остаётся как записано, а не растёт до «Всегда».
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["set_marks"] = PermAsk })
	write033(t, dir, false)
	write033(t, dir, true)
	if got := LoadAgentPerms(dir); got.Tools["set_marks"] != PermAsk {
		t.Fatalf("set_marks после включения: %q", got.Tools["set_marks"])
	}
}

// После выключения в 0.33 первая правка одной строки пишет сверенный вид: прочие правки не
// возвращаются из старых tools в «Всегда».
func rt033OneRow(t *testing.T) {
	dir := testkit.TempDir(t)
	UpdateAgentPerms(dir, func(p *AgentPerms) {
		for _, k := range []string{"autobuild", "vary_layouts", "undo_last_edit", "set_marks", "add_sticker"} {
			p.Tools[k] = PermAllow
		}
		p.Tools["add_folder"], p.Tools["create_album"] = PermAsk, PermAsk
	})
	write033(t, dir, false)
	got := UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["show_spread"] = PermAsk })
	for _, k := range AgentToolNames() {
		if isEditClass(AgentToolClass(k)) && got.Tools[k] != PermDeny {
			t.Errorf("%s вернулся из старых tools: %q", k, got.Tools[k])
		}
	}
	if a, p := fileFlags(t, dir); a || p {
		t.Fatalf("флаги: agents.edit=%v perms.edit=%v", a, p)
	}
	if again := LoadAgentPerms(dir); again.Tools["autobuild"] != PermDeny || again.Tools["show_spread"] != PermAsk {
		t.Fatalf("в файле: %+v", again)
	}
}

// Чужие ключи верхнего уровня и вложенные поля agentPerms (новее) переживают запись 0.34.
func rt033Foreign(t *testing.T) {
	dir := testkit.TempDir(t)
	writeRaw(t, dir, `{"future":{"a":1},"agentPerms":{"tools":{"set_future":"ask"},"edit":false}}`)
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["get_album"] = PermDeny })
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"future"`) || LoadAgentPerms(dir).Tools["set_future"] != PermAsk {
		t.Fatalf("потеряно чужое: %s", data)
	}
}

// Сбой записи: файл прежний целиком — agents.edit и agentPerms.edit оба старые.
func rt033Fail(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("права папки не держат запись (Windows или root)")
	}
	dir := testkit.TempDir(t)
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["show_spread"] = PermAsk })
	before, _ := os.ReadFile(ConfigPath(dir))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	_, err := UpdateAgentPermsErr(dir, func(p *AgentPerms) {
		for _, k := range []string{"autobuild", "vary_layouts", "undo_last_edit", "add_folder", "create_album"} {
			p.Tools[k] = PermAllow
		}
	})
	if err == nil {
		t.Fatal("запись в закрытую папку без ошибки")
	}
	if after, _ := os.ReadFile(ConfigPath(dir)); string(after) != string(before) {
		t.Fatalf("файл изменился при сбое:\n%s\n%s", before, after)
	}
	if a, p := fileFlags(t, dir); a || p {
		t.Fatalf("после сбоя: agents.edit=%v perms.edit=%v", a, p)
	}
}

// TestPlacesPush: наверх без повторов, не больше 10, «chosen» не понижается, «Забыть места» не
// трогает права, круг Save→Load, негодное строение — без записи.
func TestPlacesPush(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(n int) string { return filepath.Join(dir, fmt.Sprintf("p%02d", n)) }

	if got := PushPlace(nil, Place{"", PlaceChosen}); len(got) != 0 {
		t.Fatalf("пустое место: %+v", got)
	}
	var list []Place
	for i := range 12 {
		list = PushPlace(list, Place{at(i), PlaceParent})
	}
	if len(list) != MaxPlaces || list[0].Dir != at(11) || list[MaxPlaces-1].Dir != at(2) {
		t.Fatalf("после 12: %d, первое %q, последнее %q", len(list), list[0].Dir, list[len(list)-1].Dir)
	}
	// Повтор уходит наверх без дубля; «chosen» после «parent» остаётся «chosen».
	list = PushPlace(list, Place{at(5), PlaceChosen})
	list = PushPlace(list, Place{at(7), PlaceParent})
	list = PushPlace(list, Place{at(5) + string(filepath.Separator), PlaceParent})
	if len(list) != MaxPlaces || list[0] != (Place{at(5), PlaceChosen}) || list[1].Dir != at(7) {
		t.Fatalf("повторы: %+v", list)
	}
	n := 0
	for _, p := range list {
		if p.Dir == at(5) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("дубль места: %+v", list)
	}
	// Источник не меняется.
	src := []Place{{at(1), PlaceParent}}
	_ = PushPlace(src, Place{at(2), PlaceParent})
	if len(src) != 1 || src[0].Dir != at(1) {
		t.Fatal("PushPlace изменил источник")
	}

	// Через файл: круг Save→Load, права инструментов целы, негодное — без записи.
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Tools["get_photo"] = PermDeny })
	for i := range 3 {
		AddPlace(dir, Place{at(i), PlaceParent})
	}
	p := LoadAgentPerms(dir)
	if len(p.Places) != 3 || p.Places[0].Dir != at(2) || p.Tools["get_photo"] != PermDeny {
		t.Fatalf("после AddPlace: %+v", p)
	}
	before, _ := os.ReadFile(ConfigPath(dir))
	AddPlace(dir, Place{"relative", PlaceChosen})
	AddPlace(dir, Place{at(9), "zzz"})
	if after, _ := os.ReadFile(ConfigPath(dir)); string(after) != string(before) {
		t.Fatalf("негодное место записано:\n%s", after)
	}
	if got := ForgetPlaces(dir); len(got.Places) != 0 || got.Tools["get_photo"] != PermDeny {
		t.Fatalf("ForgetPlaces: %+v", got)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), `"places"`) || !strings.Contains(string(data), `"get_photo"`) {
		t.Fatalf("файл после ForgetPlaces: %s", data)
	}
	// Пустой список: повторное «забыть» не падает.
	if got := ForgetPlaces(dir); len(got.Places) != 0 {
		t.Fatalf("повторное забывание: %+v", got)
	}
	// Размер: 10 мест по 1 КБ — config.json далеко от 64 КБ посредника.
	var fat []Place
	for i := range 20 {
		fat = PushPlace(fat, Place{filepath.Join(dir, strings.Repeat("я", 500), fmt.Sprint(i)), PlaceParent})
	}
	UpdateAgentPerms(dir, func(p *AgentPerms) { p.Places = fat })
	if st, err := os.Stat(ConfigPath(dir)); err != nil || st.Size() > 32<<10 {
		t.Fatalf("config.json с 10 местами: %v, %d байт", err, st.Size())
	}
}
