package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestPrefsCustomRoundTrip(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	p := a.SetCustom(prefs.CustomLevel{Base: prefs.LevelStandard, On: []string{"manual", "bad id"}, Off: []string{"zoom"}})
	if p.Custom == nil || p.Custom.Base != prefs.LevelStandard || !slices.Equal(p.Custom.On, []string{"manual"}) ||
		!slices.Equal(p.Custom.Off, []string{"zoom"}) {
		t.Fatalf("SetCustom вернул %+v", p.Custom)
	}
	if p := a.SetLevel(prefs.LevelCustom); p.Level != prefs.LevelCustom || p.First || p.Custom == nil || p.Custom.Base != prefs.LevelStandard {
		t.Fatalf("SetLevel(custom): %+v", p)
	}
	// В файл пишется очищенный набор в формате design §1.1.
	if s := compact(t, prefs.ConfigPath(a.cfgDir)); !strings.Contains(s, `"custom":{"base":"standard","on":["manual"],"off":["zoom"]}`) {
		t.Fatalf("custom в файле: %s", s)
	}
	if st := a.GetState().Prefs; st.Level != prefs.LevelCustom || st.Custom == nil || !slices.Equal(st.Custom.Off, []string{"zoom"}) {
		t.Fatalf("State.Prefs %+v", st)
	}
	// Пустой набор пишется как [], а не null.
	a.SetCustom(prefs.CustomLevel{Base: prefs.LevelFull})
	if s := compact(t, prefs.ConfigPath(a.cfgDir)); !strings.Contains(s, `"custom":{"base":"full","on":[],"off":[]}`) {
		t.Fatalf("пустой набор: %s", s)
	}
}

// compact — файл без пробелов и переводов строк (config.json пишется с отступами).
func compact(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(string(data)), "")
}

// Смена уровня (в том числе устаревшим клиентом) набор «Своего» не трогает.
func TestSetLevelKeepsCustom(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.SetCustom(prefs.CustomLevel{Base: prefs.LevelFull, Off: []string{"templates"}})
	for _, lvl := range []string{prefs.LevelCustom, prefs.LevelSimple, "x", prefs.LevelStandard} {
		if p := a.SetLevel(lvl); p.Custom == nil || p.Custom.Base != prefs.LevelFull || !slices.Equal(p.Custom.Off, []string{"templates"}) {
			t.Fatalf("SetLevel(%q) затёр набор: %+v", lvl, p.Custom)
		}
	}
	a.SetShowStart(false)
	if c := a.loadConfig(); c.Custom == nil || c.Custom.Base != prefs.LevelFull || c.Level != prefs.LevelStandard {
		t.Fatalf("конфиг %+v", c)
	}
}

// Запись набора не меняет уровень и флажок: «Свой» включает SetLevel отдельно.
func TestSetCustomKeepsLevel(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if p := a.SetCustom(prefs.CustomLevel{Base: prefs.LevelStandard}); !p.First || p.Level != prefs.LevelSimple || !p.ShowStart {
		t.Fatalf("первый запуск после SetCustom: %+v", p)
	}
	a.SetLevel(prefs.LevelFull)
	a.SetShowStart(false)
	if p := a.SetCustom(prefs.CustomLevel{Base: prefs.LevelSimple, On: []string{"zoom"}}); p.Level != prefs.LevelFull || p.ShowStart || p.First {
		t.Fatalf("SetCustom затёр уровень или флажок: %+v", p)
	}
	if c := a.loadConfig(); c.Level != prefs.LevelFull || !c.HideStart || c.Custom == nil || !slices.Equal(c.Custom.On, []string{"zoom"}) {
		t.Fatalf("конфиг %+v", c)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if p, _ := a.prefsAndRecent(); !p.First || p.Level != prefs.LevelSimple || !p.ShowStart {
		t.Fatalf("до записи: %+v", p)
	}
	if got := a.SetShowStart(false); got != (prefs.Prefs{Level: prefs.LevelSimple, First: true}) {
		t.Fatalf("SetShowStart вернул %+v", got)
	}
	got := a.SetLevel("full")
	if got != (prefs.Prefs{Level: prefs.LevelFull}) {
		t.Fatalf("SetLevel вернул %+v", got)
	}
	if p := a.GetState().Prefs; p != (prefs.Prefs{Level: prefs.LevelFull}) {
		t.Fatalf("State.Prefs %+v", p)
	}
	// Чужой уровень от UI — simple; флажок возвращается.
	a.SetLevel("x")
	a.SetShowStart(true)
	if p := a.GetState().Prefs; p != (prefs.Prefs{Level: prefs.LevelSimple, ShowStart: true}) {
		t.Fatalf("после нормализации %+v", p)
	}
	// Настройки не затирают проект и папки диалогов.
	a.updateConfig(func(c *prefs.Config) { c.LastProject = "p.salbum" })
	a.SetLevel("standard")
	if c := a.loadConfig(); c.LastProject != "p.salbum" || c.Level != prefs.LevelStandard {
		t.Fatalf("конфиг %+v", c)
	}
}

// Смена уровня не трогает флажок стартового окна, и наоборот; «устаревший» клиент
// (знает старый флажок) пишет только своё поле и чужое не затирает.
func TestPrefsFieldsIndependent(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.SetShowStart(false)
	for _, lvl := range []string{"standard", "full", "simple"} {
		if p := a.SetLevel(lvl); p.ShowStart || p.Level != lvl {
			t.Fatalf("SetLevel(%q) затёр флажок: %+v", lvl, p)
		}
	}
	if c := a.loadConfig(); !c.HideStart {
		t.Fatalf("hideStart потерян: %+v", c)
	}
	a.SetLevel("full")
	a.SetShowStart(true)
	if p := a.SetShowStart(false); p.Level != prefs.LevelFull || p.ShowStart {
		t.Fatalf("SetShowStart затёр уровень: %+v", p)
	}
	if p := a.SetShowStart(true); p.Level != prefs.LevelFull || !p.ShowStart {
		t.Fatalf("SetShowStart(true): %+v", p)
	}
	// Второй клиент снял флажок, первый (устаревший) меняет уровень: флажок остаётся снятым.
	a.SetShowStart(false)
	a.SetLevel("standard")
	if p := a.GetState().Prefs; p != (prefs.Prefs{Level: prefs.LevelStandard}) {
		t.Fatalf("устаревший клиент затёр чужое поле: %+v", p)
	}
}

// Черновик в список последних не попадает.
func TestRememberRecentSkipsDraft(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.rememberRecent(a.draftPath())
	a.rememberRecent("")
	if r := a.loadConfig().Recent; len(r) != 0 {
		t.Fatalf("черновик в списке: %v", r)
	}
	// Путь черновика сравнивается без учёта регистра, как все пути Windows.
	a.rememberRecent(strings.ToUpper(a.draftPath()))
	if r := a.loadConfig().Recent; len(r) != 0 {
		t.Fatalf("черновик в другом регистре попал в список: %v", r)
	}
}

// Альбом встаёт наверх списка последних; повтор (в другом регистре) не плодит дубль, а поднимает запись;
// нелокальный путь не пишется.
func TestRememberRecent(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	dir := testkit.TempDir(t)
	first, second := filepath.Join(dir, "a"+projectExt), filepath.Join(dir, "b"+projectExt)
	a.rememberRecent(first)
	a.rememberRecent(second)
	if r := a.loadConfig().Recent; !slices.Equal(r, []string{second, first}) {
		t.Fatalf("список: %v", r)
	}
	again := strings.ToUpper(first)
	a.rememberRecent(again)
	if r := a.loadConfig().Recent; !slices.Equal(r, []string{again, second}) {
		t.Fatalf("после повтора: %v", r)
	}
	a.rememberRecent(`\\srv\share\x` + projectExt)
	if r := a.loadConfig().Recent; len(r) != 2 {
		t.Fatalf("нелокальный путь попал в список: %v", r)
	}
}

func TestOpenRecentOnlyListed(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	listed, other := filepath.Join(dir, "a"+projectExt), filepath.Join(dir, "b"+projectExt)
	for _, p := range []string{listed, other} {
		if err := salbum.Save(model.New(), p); err != nil {
			t.Fatal(err)
		}
	}
	a.updateConfig(func(c *prefs.Config) { c.Recent = []string{listed} })
	before := a.editor()

	if _, err := a.OpenRecent(other); err == nil || a.editor() != before {
		t.Fatalf("путь вне списка открыт: %v", err)
	}
	// Чужое расширение даже из списка не открывается.
	txt := filepath.Join(dir, "x.txt")
	os.WriteFile(txt, []byte("{}"), 0o644)
	a.updateConfig(func(c *prefs.Config) { c.Recent = append(c.Recent, txt) })
	if _, err := a.OpenRecent(txt); err == nil || a.editor() != before {
		t.Fatalf("не .salbum открыт: %v", err)
	}
	st, err := a.OpenRecent(strings.ToUpper(listed[:1]) + listed[1:])
	if err != nil {
		t.Fatal(err)
	}
	if a.editor() == before || st.View.Path == "" {
		t.Fatal("альбом из списка не открылся")
	}
	if c := a.loadConfig(); c.LastProject != st.View.Path || len(c.Recent) == 0 || !strings.EqualFold(c.Recent[0], listed) {
		t.Fatalf("конфиг после открытия: %+v", c)
	}
	// Файла нет: ошибка, запись остаётся до ForgetRecent.
	gone := filepath.Join(dir, "gone"+projectExt)
	a.updateConfig(func(c *prefs.Config) { c.Recent = append(c.Recent, gone) })
	if _, err := a.OpenRecent(gone); err == nil {
		t.Fatal("нет файла, а ошибки нет")
	}
	if !slices.Contains(a.loadConfig().Recent, gone) {
		t.Fatal("запись пропала сама")
	}
	rows := a.ForgetRecent(gone)
	if slices.ContainsFunc(rows, func(r prefs.RecentProject) bool { return r.Path == gone }) || len(rows) != 2 {
		t.Fatalf("ForgetRecent: %+v", rows)
	}
	if got := a.Recent(); !slices.Equal(got, rows) {
		t.Fatalf("Recent = %+v, want %+v", got, rows)
	}
	if rows := a.ClearRecent(); len(rows) != 0 || len(a.loadConfig().Recent) != 0 || len(a.Recent()) != 0 {
		t.Fatalf("ClearRecent: %+v, config %+v", rows, a.loadConfig().Recent)
	}
}

// Байты файла: encode копирует поля руками, круговой тест прошёл бы и без записи.
func TestNewProjectProduct(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	for _, in := range []string{"album", "calendar", ""} {
		st := a.NewProject(in)
		if st.View.Product != model.ProductAlbum {
			t.Fatalf("NewProject(%q): product %q", in, st.View.Product)
		}
		path := filepath.Join(dir, "p"+projectExt)
		if err := a.editor().Save(path); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), `"product": "album"`) {
			t.Fatalf("в файле нет product: %s", data)
		}
	}
}
