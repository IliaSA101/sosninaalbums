package app

// Тексты «Как подключить» и списки окна ИИ-агентов на настоящем App (0.31-143-42, -43): путь
// программы — из a.agentsExe, мёртвый путь гасит тексты, DTO окна повторяют типы службы.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// agentsExeApp — App без сокета, у которого путь программы задан тестом; XDG и дом — чистые.
func agentsExeApp(t *testing.T, exe string, err error) *App {
	t.Helper()
	home := testkit.TempDir(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	a := newApp(t)
	a.agentsExe = func() (string, error) { return exe, err }
	return a
}

// tempAside уводит os.TempDir() на Windows в другую папку: ExeHint считает exe под Temp
// переносным из zip (мёртвым путём), а exe теста лежит в testkit.TempDir под тем же %TEMP%. Звать
// после создания exe. Вне Windows ничего не делает.
func tempAside(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	other := testkit.TempDir(t)
	t.Setenv("TMP", other)
	t.Setenv("TEMP", other)
}

// TestClients: живой путь — четыре клиента и ни одного предупреждения; мёртвый (удалён, не файл,
// относительный, os.Executable не отдал путь) — Setups пуст, но не nil, Hint не пуст. Путь с
// кавычками и $ проходит в JSON окна без потерь.
func TestClients(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "Мои альбомы it's $HOME")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "SosninaAlbums")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	tempAside(t)
	info := agentsExeApp(t, exe, nil).AgentsInfo()
	// Переносной exe (Windows, рядом нет uninstall.exe) — отдельное предупреждение, не мёртвый путь.
	if len(info.Setups) != 4 || info.Hint != "" || info.Portable != (runtime.GOOS == "windows") {
		t.Fatalf("живой путь: %d клиентов, hint %q, portable %v", len(info.Setups), info.Hint, info.Portable)
	}
	var cfg struct {
		Servers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(info.Setups[0].Text), &cfg); err != nil || cfg.Servers["sosnina-albums"].Command != exe {
		t.Errorf("путь не вернулся из JSON окна: %v %+v", err, cfg)
	}
	if want := (runtime.GOOS == "windows" || runtime.GOOS == "darwin"); info.Setups[0].File != want {
		t.Errorf("File у Claude Desktop = %v на %s", info.Setups[0].File, runtime.GOOS)
	}

	// Мёртвые пути.
	nofile := filepath.Join(dir, "нет-такого")
	dead := map[string]struct {
		exe string
		err error
	}{
		"удалён":            {nofile, nil},
		"каталог":           {dir, nil},
		"относительный":     {"SosninaAlbums", nil},
		"пустой":            {"", nil},
		"ошибка Executable": {"", os.ErrNotExist},
	}
	for name, d := range dead {
		info := agentsExeApp(t, d.exe, d.err).AgentsInfo()
		if info.Setups == nil || len(info.Setups) != 0 || info.Hint == "" {
			t.Errorf("%s: setups %#v, hint %q", name, info.Setups, info.Hint)
		}
		b, _ := json.Marshal(info)
		var m map[string]json.RawMessage
		_ = json.Unmarshal(b, &m)
		for _, k := range []string{"setups", "clients", "calls"} {
			if string(m[k]) != "[]" {
				t.Errorf("%s: %s в JSON = %s, ждали []", name, k, m[k])
			}
		}
	}

	// Линукс: XDG_CONFIG_HOME не там, где ищет посредник — предупреждение над живыми текстами.
	if runtime.GOOS == "linux" {
		a := agentsExeApp(t, exe, nil)
		t.Setenv("XDG_CONFIG_HOME", testkit.TempDir(t))
		if info := a.AgentsInfo(); len(info.Setups) != 4 || info.Hint == "" {
			t.Errorf("XDG: %d клиентов, hint %q", len(info.Setups), info.Hint)
		}
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(os.Getenv("HOME"), ".config"))
		if info := a.AgentsInfo(); info.Hint != "" {
			t.Errorf("XDG по умолчанию: hint %q", info.Hint)
		}
	}
}

// Подключённый клиент и журнал на стенде: имя и время доходят до окна в мс; новые вызовы первыми.
func TestAgentsInfoLists(t *testing.T) {
	r := newAgRig(t)
	if info := r.a.AgentsInfo(); len(info.Clients) != 0 || len(info.Calls) != 0 || info.Today != 0 || info.Status != "off" {
		t.Fatalf("до включения: %+v", info)
	}
	before := time.Now()
	r.start(t)
	r.call(t, "get_album", nil)
	r.call(t, "list_spreads", nil)
	var info AgentsInfo
	agWait(t, "клиент и два вызова в журнале", func() bool { info = r.a.AgentsInfo(); return len(info.Clients) == 1 && len(info.Calls) == 2 })
	if c := info.Clients[0]; c.Name != "тест" || c.Since < before.UnixMilli()-1000 || c.Since > time.Now().UnixMilli() {
		t.Errorf("клиент: %+v", c)
	}
	if info.Calls[0].Tool == info.Calls[1].Tool || !info.Calls[0].OK || info.Today != 2 || info.Calls[0].Client != "тест" {
		t.Errorf("журнал: %+v today %d", info.Calls, info.Today)
	}
	if info.Calls[0].At < info.Calls[1].At {
		t.Errorf("новые не первыми: %+v", info.Calls)
	}
}

// Конвертеры окна: AgentCall и AgentClient повторяют поля службы (время — в unix мс); новое поле
// службы без пары в app — провал, а не молчаливая потеря.
func TestAgentTypesConvert(t *testing.T) {
	at := time.Date(2026, 10, 29, 12, 30, 15, 123_456_789, time.UTC)
	got := agentCall(agents.Call{At: at, Client: "К", Tool: "Сводка альбома", OK: true, Ms: 42, Edit: true, Note: "копия"})
	if want := (AgentCall{At: at.UnixMilli(), Client: "К", Tool: "Сводка альбома", OK: true, Ms: 42, Edit: true, Note: "копия"}); got != want {
		t.Errorf("agentCall = %+v, ждали %+v", got, want)
	}
	if got.At != 1_793_277_015_123 {
		t.Errorf("At = %d: не unix мс", got.At)
	}
	if z := agentCall(agents.Call{}); z.At != (time.Time{}).UnixMilli() || z.Client != "" || z.OK || z.Ms != 0 || z.Edit || z.Note != "" {
		t.Errorf("нулевой вызов: %+v", z)
	}
	names := func(t reflect.Type) []string {
		var out []string
		for f := range t.Fields() {
			out = append(out, f.Name)
		}
		return out
	}
	if a, s := names(reflect.TypeFor[AgentCall]()), names(reflect.TypeFor[agents.Call]()); !slices.Equal(a, s) {
		t.Errorf("поля AgentCall %v и agents.Call %v разошлись", a, s)
	}
	if a, s := names(reflect.TypeFor[AgentClient]()), names(reflect.TypeFor[agents.Client]()); !slices.Equal(a, s) {
		t.Errorf("поля AgentClient %v и agents.Client %v разошлись", a, s)
	}
	// JSON-теги окна: контракт с frontend/wailsjs.
	for typ, want := range map[reflect.Type][]string{
		reflect.TypeFor[AgentCall]():   {"at", "client", "tool", "ok", "ms", "edit", "note"},
		reflect.TypeFor[AgentClient](): {"name", "since"},
	} {
		var tags []string
		for f := range typ.Fields() {
			tags = append(tags, f.Tag.Get("json"))
		}
		if !slices.Equal(tags, want) {
			t.Errorf("%s: теги %v, ждали %v", typ, tags, want)
		}
	}
	// AgentCallEvent несёт вызов и счётчик в одном объекте.
	b, _ := json.Marshal(AgentCallEvent{AgentCall: got, Today: 7})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["today"] != float64(7) || m["tool"] != "Сводка альбома" || m["at"] == nil {
		t.Errorf("событие agents:call: %s", b)
	}
}
