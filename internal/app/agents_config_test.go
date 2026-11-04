package app

// Настройки ИИ-агентов в config.json (0.31-143-42): уровень зажат, чужие ключи целы, круг
// Save→Load, выключение проходит и в состоянии error (его причина снимается).

import (
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// foreignConfig — config.json со своими ключами, которых программа не знает (будущая версия, ручная правка).
const foreignConfig = `{"future":{"a":[1,2,{"b":null}]},"zzz":"x","recent":["/x/a.salbum"]}`

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(prefs.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAgentsConfigClean(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	dir := a.cfgDir
	if err := os.WriteFile(prefs.ConfigPath(dir), []byte(foreignConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Уровень: вне 1…3 — зажим к краю; по умолчанию ключа agents в файле нет.
	for lv, want := range map[int]int{-7: 1, 0: 1, 1: 1, 2: 2, 3: 3, 4: 3, 99: 3, 1 << 40: 3} {
		if got := a.SetAgentsLevel(lv).Level; got != want || a.AgentsInfo().Level != want || prefs.AgentsOf(dir).Level != want {
			t.Errorf("SetAgentsLevel(%d) = %d, ждали %d", lv, got, want)
		}
	}
	a.SetAgentsLevel(1)
	if strings.Contains(readConfig(t, dir), `"agents"`) {
		t.Errorf("умолчание записано в config.json: %s", readConfig(t, dir))
	}

	// Рукой записанный мусор уровня — 1 и в окне, и в службе; On читается как есть.
	for _, lv := range []string{"0", "-1", "4", "99", "2147483647"} {
		raw := `{"agents":{"on":true,"level":` + lv + `},"future":{"keep":1}}`
		if err := os.WriteFile(prefs.ConfigPath(dir), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := a.AgentsInfo().Level; got != 1 || prefs.AgentsOf(dir) != (prefs.Agents{On: true, Level: 1}) {
			t.Errorf("config.json с level %s: окно %d, prefs %+v", lv, got, prefs.AgentsOf(dir))
		}
	}
	// Тип не тот: ключ agents целиком мусор — умолчание, а не паника.
	for _, raw := range []string{`{"agents":"да"}`, `{"agents":{"level":"три"}}`, `{"agents":null}`, `{"agents":[]}`, ``, `не json`} {
		if err := os.WriteFile(prefs.ConfigPath(dir), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := a.AgentsInfo(); got.Level != 1 || got.Status != "off" {
			t.Errorf("config.json %q: %+v", raw, got)
		}
	}

	// Чужие ключи и «Последние» переживают запись настроек агентов; круг Save → Load.
	if err := os.WriteFile(prefs.ConfigPath(dir), []byte(foreignConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	a.SetAgentsLevel(3)
	a.SetAgentsLevel(2)
	got := readConfig(t, dir)
	for _, want := range []string{`"future"`, `"zzz"`, `"recent"`, `"b": null`, `/x/a.salbum`, `"agents"`} {
		if !strings.Contains(got, want) {
			t.Errorf("в config.json потерян %s: %s", want, got)
		}
	}
	if prefs.AgentsOf(dir) != (prefs.Agents{Level: 2}) || a.AgentsInfo().Level != 2 {
		t.Errorf("круг config.json → agents: %+v", prefs.AgentsOf(dir))
	}
	// Второй App на той же папке видит те же настройки (перезапуск программы).
	b := newApp(t)
	b.cfgDir = dir
	if b.AgentsInfo().Level != 2 {
		t.Errorf("после перезапуска уровень %d", b.AgentsInfo().Level)
	}
}

// Выключить можно и в состоянии error: путь сокета не открылся (слишком длинная папка данных),
// On в config.json не записан; SetAgents(false) возвращает off без причины и пишет On=false.
func TestAgentsConfigError(t *testing.T) {
	r := newAgRig(t)
	r.a.data = strings.Repeat("д", 60) // путь сокета длиннее предела: открыть нечем
	if info, err := r.a.SetAgents(true); err != nil || info.Status != "error" || info.Error == "" {
		t.Fatalf("включение с длинным путём: %+v %v", info, err)
	}
	if prefs.AgentsOf(r.cfg).On {
		t.Error("On записан, хотя включить не вышло")
	}
	if info := r.a.AgentsInfo(); info.Status != "error" || strings.Contains(info.Error, "/") || strings.Contains(info.Error, "д"+"д") {
		t.Errorf("причина с путём или без текста: %+v", info)
	}
	// Запуск программы с On=true в config.json и тем же отказом: error, On остаётся — пользователь не просил выключать.
	prefs.UpdateAgents(r.cfg, func(c *prefs.Agents) { c.On = true })
	r.a.startAgents()
	r.a.bg.Wait()
	if info := r.a.AgentsInfo(); info.Status != "error" || !prefs.AgentsOf(r.cfg).On {
		t.Fatalf("старт с On=true: %+v, On %v", info, prefs.AgentsOf(r.cfg).On)
	}
	info, err := r.a.SetAgents(false)
	if err != nil || info.Status != "off" || info.Error != "" || prefs.AgentsOf(r.cfg).On {
		t.Fatalf("SetAgents(false) в error: %+v %v On %v", info, err, prefs.AgentsOf(r.cfg).On)
	}
	// Выключить уже выключенное — холостое, без ошибок; потом включить на нормальном пути.
	if info, err := r.a.SetAgents(false); err != nil || info.Status != "off" {
		t.Fatalf("повторное выключение: %+v %v", info, err)
	}
	r.a.data = "sa"
	if info, _ := r.a.SetAgents(true); info.Status != "on" || !prefs.AgentsOf(r.cfg).On {
		t.Fatalf("включение после исправления: %+v", info)
	}
	r.a.stopAgents()
}
