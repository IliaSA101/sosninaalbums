package app

// Контракт разрешений агентов (150, Т1): методы пишут agentPerms одной записью, поля AgentsInfo,
// заглушки Т3 без паники.

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

func TestAgentPermsMethods(t *testing.T) {
	a, events := pubApp(t)
	dir := a.cfgDir
	info := a.AgentsInfo()
	if info.Perms == nil || info.Places == nil || len(info.Places) != 0 {
		t.Fatalf("умолчание: perms=%v places=%v (ждали не nil, места пусты)", info.Perms, info.Places)
	}
	n := len(events())

	// Одна строка: допустимое значение пишется, чужое имя и режим вне Can — без изменений.
	a.SetAgentPerm("autobuild", prefs.PermAllow)
	a.SetAgentPerm("get_album", prefs.PermAsk) // у чтения «Спрашивать» нет
	a.SetAgentPerm("get_album", "всегда")
	a.SetAgentPerm("нет_такого", prefs.PermAllow)
	a.SetAgentPerm("", prefs.PermAllow)
	p := prefs.LoadAgentPerms(dir)
	if p.Tools["autobuild"] != prefs.PermAllow || p.Tools["get_album"] != prefs.PermAllow || p.Tools["нет_такого"] != "" {
		t.Fatalf("SetAgentPerm: %+v", p.Tools)
	}
	if got := names(events()[n:]); !slices.Contains(got, "agents:state") {
		t.Errorf("окна не получили agents:state: %v", got)
	}

	// Группа edit: E, F, A; запуск и показ не трогает. Группа read: «Спрашивать» не ставится.
	a.SetAgentPermGroup("edit", prefs.PermAsk)
	a.SetAgentPermGroup("read", prefs.PermAsk)
	a.SetAgentPermGroup("other", prefs.PermDeny)
	a.SetAgentPermGroup("", prefs.PermDeny)
	p = prefs.LoadAgentPerms(dir)
	for _, k := range prefs.AgentToolNames() {
		want := prefs.PermAllow
		if c := prefs.AgentToolClass(k); c == prefs.ClassEdit || c == prefs.ClassFolder || c == prefs.ClassAlbum {
			want = prefs.PermAsk
		} else if c == prefs.ClassAskOnly {
			want = prefs.PermDeny // группа «Правки» Q не трогает, свежий столбец — «Запретить»
		}
		if p.Tools[k] != want {
			t.Errorf("группа: %s = %q, ждали %q", k, p.Tools[k], want)
		}
	}
	// Зеркало: «Спрашивать» у autobuild — 0.33 видит правки выключенными.
	if prefs.AgentsOf(dir).Edit || p.Edit {
		t.Errorf("зеркало: agents.edit=%v perms.edit=%v", prefs.AgentsOf(dir).Edit, p.Edit)
	}
	a.SetAgentPermGroup("edit", prefs.PermAllow)
	a.SetAgentPerm("add_folder", prefs.PermAsk)
	if !prefs.AgentsOf(dir).Edit {
		t.Error("пять правок 0.33 разрешены — флажок должен быть включён")
	}

	// Места: список в AgentsInfo, забыть — права целы.
	place := filepath.Join(dir, "Фото")
	prefs.AddPlace(dir, prefs.Place{Dir: place, How: prefs.PlaceChosen})
	prefs.AddPlace(dir, prefs.Place{Dir: filepath.Join(dir, "2026"), How: prefs.PlaceParent})
	places := a.AgentsInfo().Places
	if len(places) != 2 || places[1] != (AgentPlace{Key: prefs.PlaceKey(prefs.Place{Dir: place, How: prefs.PlaceChosen}), Dir: place, Chosen: true}) || places[0].Chosen {
		t.Fatalf("места: %+v", places)
	}
	info = a.ForgetAgentPlaces()
	if len(info.Places) != 0 || prefs.LoadAgentPerms(dir).Tools["autobuild"] != prefs.PermAllow {
		t.Fatalf("после забывания: %+v", info.Places)
	}
	if info = a.ForgetAgentPlaces(); len(info.Places) != 0 { // холостое
		t.Fatalf("повторное забывание: %+v", info.Places)
	}

	// Ответ — JSON со списками, не null.
	b, _ := json.Marshal(a.AgentsInfo())
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"perms", "places", "lifted"} {
		if v, ok := m[k].([]any); !ok || v == nil {
			t.Errorf("в AgentsInfo нет списка %q: %s", k, b)
		}
	}
}

// Теги и порядок полей — контракт с frontend/wailsjs.
func TestAgentPermTypes(t *testing.T) {
	if got := jsonTags(reflect.TypeFor[AgentPerm]()); !slices.Equal(got, []string{"tool", "title", "group", "mode", "can", "why", "note"}) {
		t.Errorf("AgentPerm: %v", got)
	}
	if got := jsonTags(reflect.TypeFor[AgentPlace]()); !slices.Equal(got, []string{"key", "dir", "label", "chosen", "bound", "narrow", "narrowRoot", "state", "here"}) {
		t.Errorf("AgentPlace: %v", got)
	}
}

// Без прежнего альбома и без вопроса методы Т3b отвечают ошибкой, ничего не меняют и не паникуют на
// пустом и огромном (настоящие пути — TestAgentOpenPrev, TestFolderAskOther).
func TestAgentPermNoOpen(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	before := a.editor().View().Rev
	if st, err := a.AgentOpenPrev(); err == nil || st.Cancelled {
		t.Errorf("AgentOpenPrev: %+v, %v", st, err)
	}
	for _, id := range []string{"", "нет", strings.Repeat("я", 1<<16)} {
		if _, err := a.AnswerAgentOther(id); err == nil {
			t.Errorf("AnswerAgentOther(%.5q): ошибки нет", id)
		}
		a.SetAgentPerm(id, id)
		a.SetAgentPermGroup(id, id)
	}
	if a.editor().View().Rev != before {
		t.Error("заглушки изменили альбом")
	}
	if len(prefs.LoadAgentPerms(a.cfgDir).Places) != 0 {
		t.Error("заглушка добавила место")
	}
}

// TestAgentPermWhyAskOnly: у выгрузки и открытия альбома («Всегда» нет) пояснение под строкой есть, у правок
// с «Всегда» и у остальных классов прежнее: чтение и запуск — своё, правка — пусто.
func TestAgentPermWhyAskOnly(t *testing.T) {
	a, _ := pubApp(t)
	got := map[string]AgentPerm{}
	for _, p := range a.AgentsInfo().Perms {
		got[p.Tool] = p
	}
	for tool, why := range map[string]string{"export_album": permWhyExp, "open_album": permWhyOpen} {
		if p, ok := got[tool]; !ok || p.Why != why || !strings.Contains(p.Why, "Можно запретить") {
			t.Errorf("%s: %+v", tool, p)
		}
	}
	if permWhyExp == permWhyOpen || !strings.Contains(permWhyExp, "папк") || !strings.Contains(permWhyOpen, "файл") {
		t.Error("тексты выгрузки (папка) и открытия (файл) смешаны")
	}
	if got["autobuild"].Why != "" || got["get_album"].Why != permWhyRead || got[startToolName].Why != permWhyStart {
		t.Errorf("прочие пояснения изменились: %q %q %q", got["autobuild"].Why, got["get_album"].Why, got[startToolName].Why)
	}
}
