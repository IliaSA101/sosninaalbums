package app

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
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
	// Путь черновика сравнивается по paths.Fold: на Windows и macOS без учёта регистра, на Linux
	// другой регистр — другой файл, он не черновик и в список попадает (101).
	up := strings.ToUpper(a.draftPath())
	want := []string{up}
	if paths.FoldEqual(up, a.draftPath()) {
		want = nil
	}
	a.rememberRecent(up)
	if r := a.loadConfig().Recent; !slices.Equal(r, want) {
		t.Fatalf("черновик в другом регистре: %v, ждали %v", r, want)
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
	// Повтор другим регистром — на Windows и macOS; на Linux это другой альбом (paths.Fold, 101).
	again, upper := first, strings.ToUpper(first)
	if paths.FoldEqual(upper, first) {
		again = upper
	}
	a.rememberRecent(again)
	if r := a.loadConfig().Recent; !slices.Equal(r, []string{again, second}) {
		t.Fatalf("после повтора: %v", r)
	}
	a.rememberRecent(`\\srv\share\x` + projectExt)
	if r := a.loadConfig().Recent; len(r) != 2 {
		t.Fatalf("нелокальный путь попал в список: %v", r)
	}
	if again == first {
		a.rememberRecent(upper)
		if r := a.loadConfig().Recent; !slices.Equal(r, []string{upper, first, second}) {
			t.Fatalf("другой регистр на Linux: %v", r)
		}
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

// «Открыть»: отмена диалога — Cancelled и прежний проект; сбой диалога и удачное
// открытие — без Cancelled (фронт по нему не отличит отмену от открытия сравнением вида).
func TestOpenProjectCancelled(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	first, second := filepath.Join(dir, "a"+projectExt), filepath.Join(dir, "b"+projectExt)
	for _, p := range []string{first, second} {
		if err := salbum.Save(model.New(), p); err != nil {
			t.Fatal(err)
		}
	}
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	pick, fail := "", error(nil)
	openFileDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return pick, fail }

	before := a.editor()
	st, err := a.OpenProject()
	if err != nil || !st.Cancelled || a.editor() != before {
		t.Fatalf("отмена на пустом: err %v, cancelled %v", err, st.Cancelled)
	}
	if b, _ := json.Marshal(st); !strings.Contains(string(b), `"cancelled":true`) {
		t.Fatalf("в JSON нет cancelled: %s", b)
	}
	// Открываем два альбома подряд: каждый раз без признака отмены.
	for _, p := range []string{first, second} {
		pick = p
		st, err = a.OpenProject()
		if err != nil || st.Cancelled || !strings.EqualFold(st.View.Path, p) {
			t.Fatalf("открытие %s: err %v, cancelled %v, path %q", filepath.Base(p), err, st.Cancelled, st.View.Path)
		}
		if b, _ := json.Marshal(st); strings.Contains(string(b), "cancelled") {
			t.Fatalf("cancelled в JSON удачного открытия: %s", b)
		}
	}
	// Отмена после открытия: тот же альбом и тот же rev, признак есть.
	opened, rev := a.editor(), st.View.Rev
	pick = ""
	if st, err = a.OpenProject(); err != nil || !st.Cancelled || a.editor() != opened || st.View.Rev != rev ||
		!strings.EqualFold(st.View.Path, second) {
		t.Fatalf("отмена после открытия: err %v, cancelled %v, path %q", err, st.Cancelled, st.View.Path)
	}
	// Сбой диалога — ошибка, не отмена.
	fail = errors.New("диалог не открылся")
	if st, err = a.OpenProject(); err == nil || st.Cancelled || a.editor() != opened {
		t.Fatalf("сбой диалога: err %v, cancelled %v", err, st.Cancelled)
	}
	// Остальные пути, отдающие State, признак не ставят: в них нет диалога.
	a.updateConfig(func(c *prefs.Config) { c.Recent = []string{first} })
	if st, err = a.OpenRecent(first); err != nil || st.Cancelled {
		t.Fatalf("OpenRecent: err %v, cancelled %v", err, st.Cancelled)
	}
	if st = a.NewProject(""); st.Cancelled {
		t.Fatal("NewProject с Cancelled")
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

// Подсказка о месте запуска объединяется с тостом старта и не теряет его; в обычном месте тост не меняется.
func TestStartNoticeJoinsLaunch(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	for _, place := range []osx.Place{osx.PlaceTemp, osx.PlaceTranslocated} {
		extra := place.Notice()
		if extra == "" {
			t.Fatalf("%q: нет текста", place)
		}
		if got := joinLaunchNotice("", place); got != extra {
			t.Errorf("%q без прежнего тоста: %q", place, got)
		}
		got := joinLaunchNotice("не открылся", place)
		if want := "не открылся\n\n" + extra; got != want {
			t.Errorf("%q с прежним тостом: %q, ждали %q", place, got, want)
		}
	}
	if got := joinLaunchNotice("не открылся", osx.PlaceNormal); got != "не открылся" {
		t.Errorf("обычное место изменило тост: %q", got)
	}
	if got := joinLaunchNotice("", osx.PlaceNormal); got != "" {
		t.Errorf("обычное место без тоста: %q", got)
	}
	if !strings.Contains(buf.String(), "запуск не из установленного места") {
		t.Errorf("в log.txt нет записи: %q", buf.String())
	}
}

// Сквозной путь своих шаблонов сетки (шаблон → плитка «Мои» → применение → перезапуск):
// то, что применили, совпадает с тем, что сохраняли. Общие помощники нужны и тестам виньетки.

const tplTol = 1e-5 // допуск округления долей (templates.json пишет до 1e-5)

// rtLayouts — несимметричные раскладки четырёх мест на развороте 600×300 мм: два места на каждой
// странице, разные размеры и отступы, одно смещено; у раскладки 0 третье место наклонено.
var rtLayouts = [][4][4]float64{
	{{10, 15, 170, 260}, {190, 15, 100, 120}, {330, 40, 120, 120}, {470, 20, 100, 200}},
	{{20, 20, 120, 120}, {150, 60, 130, 200}, {320, 10, 260, 90}, {320, 110, 100, 180}},
	{{12, 12, 276, 130}, {12, 150, 100, 138}, {312, 12, 120, 276}, {440, 40, 148, 200}},
}

// rtApp — App с разворотами «Авто» по counts[i] фото; возвращает ID разворотов.
func rtApp(t *testing.T, counts ...int) (*App, []string) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	ids := make([]string, len(counts))
	for i, n := range counts {
		ph := make([]model.PhotoInfo, n)
		for k := range ph {
			id := fmt.Sprintf("p%d_%d", i, k)
			ph[k] = model.PhotoInfo{ID: id, Path: "/" + id + ".jpg", W: 300, H: 200}
		}
		v, err := a.editor().NewSpread(-1, ph)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = v.Spreads[len(v.Spreads)-1].ID
	}
	return a, ids
}

// restartApp — «перезапуск»: новый App с теми же настройками и проектом, шаблоны читаются из файла.
func restartApp(t *testing.T, a *App) *App {
	t.Helper()
	b := newApp(t)
	b.cfgDir, b.emit = a.cfgDir, func(string, ...any) {}
	b.ed.Store(a.editor())
	b.loadTemplates()
	return b
}

func spreadView(a *App, id string) model.SpreadView {
	for _, s := range a.editor().View().Spreads {
		if s.ID == id {
			return s
		}
	}
	return model.SpreadView{}
}

func spreadPlaces(a *App, id string) []model.PlaceView { return spreadView(a, id).Places }

// areaPlaces — места области (left | right | spread) по возрастанию X, затем Y.
func areaPlaces(ps []model.PlaceView, scope string) []model.PlaceView {
	var out []model.PlaceView
	for _, p := range ps {
		if scope == model.ScopeSpread || (scope == model.ScopeLeft) == (p.X+p.W/2 < 300) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b model.PlaceView) int { return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y)) })
	return out
}

// samePlaces — места got = want, сдвинутые на dx по X (мм, допуск округления 1e-5 доли), по порядку.
func samePlaces(t *testing.T, tag string, got, want []model.PlaceView, dx float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: мест %d, ждали %d", tag, len(got), len(want))
	}
	const tol = tplTol * 600
	for i, g := range got {
		w := want[i]
		if math.Abs(g.X-w.X-dx) > tol || math.Abs(g.Y-w.Y) > tol || math.Abs(g.W-w.W) > tol || math.Abs(g.H-w.H) > tol ||
			math.Abs(g.Rot-w.Rot) > 1e-3 || g.Role != w.Role {
			t.Fatalf("%s: место %d: %+v, ждали %+v (dx %v)", tag, i, g, w, dx)
		}
	}
}

// fracOf — места в долях области с левым краем x0 и размером w×h.
func fracOf(ps []model.PlaceView, x0, w, h float64) []model.Box {
	out := make([]model.Box, len(ps))
	for i, p := range ps {
		out[i] = model.Box{X: (p.X - x0) / w, Y: p.Y / h, W: p.W / w, H: p.H / h, Rot: p.Rot}
	}
	return out
}

func sameBoxes(t *testing.T, tag string, got, want []model.Box) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: долей %d, ждали %d", tag, len(got), len(want))
	}
	for i, g := range got {
		w := want[i]
		if math.Abs(g.X-w.X) > tplTol || math.Abs(g.Y-w.Y) > tplTol || math.Abs(g.W-w.W) > tplTol || math.Abs(g.H-w.H) > tplTol || math.Abs(g.Rot-w.Rot) > 1e-3 {
			t.Fatalf("%s: доля %d: %+v, ждали %+v", tag, i, g, w)
		}
	}
}

// setLayout ставит на закреплённом развороте sid раскладку rtLayouts[l] (места — по порядку вида).
func setLayout(t *testing.T, a *App, sid string, l int) {
	t.Helper()
	ps := spreadPlaces(a, sid)
	if len(ps) != 4 {
		t.Fatalf("мест %d, ждали 4", len(ps))
	}
	bs := make([]model.PlaceBox, 4)
	for i, c := range rtLayouts[l] {
		bs[i] = model.PlaceBox{ID: ps[i].ID, X: c[0], Y: c[1], W: c[2], H: c[3]}
	}
	a.SetPlaces(sid, bs, "")
	for i := range ps {
		ang := 0.0
		if l == 0 && i == 2 {
			ang = 7
		}
		a.SetPlaceAngle(sid, ps[i].ID, ang)
	}
}

// tplWant — что ждём от сохранённого шаблона: id, имя, область (spread | page), места источника
// (мм, по возрастанию X) и левый край области источника.
type tplWant struct {
	id, name, tpl string
	src           []model.PlaceView
	x0            float64
}

// saveTpl сохраняет область sc разворота src; шаблон встаёт в конец списка.
func saveTpl(t *testing.T, a *App, src, sc, name, wantName string) tplWant {
	t.Helper()
	n := len(a.templateList())
	if err := a.SaveGridTemplate(src, sc, name); err != nil {
		t.Fatal(err)
	}
	list := a.templateList()
	if len(list) != n+1 {
		t.Fatalf("шаблонов %d, ждали %d", len(list), n+1)
	}
	w := tplWant{id: list[n].ID, name: wantName, tpl: model.TplSpread, src: areaPlaces(spreadPlaces(a, src), sc)}
	if sc != model.ScopeSpread {
		w.tpl = model.TplPage
	}
	if sc == model.ScopeRight {
		w.x0 = 300
	}
	return w
}

// checkMine: плитки «Мои» разворота dst на n мест идут в порядке want, с их id, именем и долями;
// применение каждой (на область scope) даёт места своего шаблона; Undo возвращает «Авто».
func checkMine(t *testing.T, a *App, dst string, n int, scope string, want []tplWant) {
	t.Helper()
	c := a.GridChoices(dst, n, scope)
	if len(c.Mine) != len(want) {
		t.Fatalf("«Мои» (%d мест, %s): плиток %d, ждали %d", n, scope, len(c.Mine), len(want))
	}
	for i, w := range want {
		m, tag, aw := c.Mine[i], fmt.Sprintf("%s #%d", w.name, i), 300.0
		if w.tpl == model.TplSpread {
			aw = 600
		}
		if m.ID != w.id || m.Name != w.name || m.Scope != w.tpl {
			t.Fatalf("%s: плитка %+v, ждали %+v", tag, m, w)
		}
		sameBoxes(t, tag+": превью", m.Boxes, fracOf(w.src, w.x0, aw, 300))
		if _, err := a.ApplyGrid(dst, scope, m.Scope, m.Boxes); err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		off := 0.0
		if scope == model.ScopeRight {
			off = 300
		}
		samePlaces(t, tag+": применение", areaPlaces(spreadPlaces(a, dst), scope), w.src, off-w.x0)
		if a.editor().Undo(); spreadView(a, dst).Grid {
			t.Fatalf("%s: отмена не вернула «Авто»", tag)
		}
	}
}

// gridSaved — App с источником из четырёх мест (раскладка 0) и тремя шаблонами с него: разворот,
// левая и правая страница; цели — развороты «Авто» на 4, 4, 4, 2, 6 и 6 фото.
func gridSaved(t *testing.T) (a *App, ids []string, sp, lf, rt tplWant) {
	t.Helper()
	a, ids = rtApp(t, 4, 4, 4, 2, 6, 6)
	a.SetGrid(ids[0], true)
	setLayout(t, a, ids[0], 0)
	sp = saveTpl(t, a, ids[0], "spread", "Моя сетка", "Моя сетка")
	lf = saveTpl(t, a, ids[0], "left", "Левая", "Левая")
	rt = saveTpl(t, a, ids[0], "right", "Правая", "Правая")
	if len(sp.src) != 4 || len(lf.src) != 2 || len(rt.src) != 2 || sp.src[2].Rot != 7 {
		t.Fatalf("источник: %d/%d/%d мест", len(sp.src), len(lf.src), len(rt.src))
	}
	return a, ids, sp, lf, rt
}

// Один сохранённый шаблон на разворот и на страницы: превью в «Мои», применение, перезапуск.
func TestTemplateGridEndToEnd(t *testing.T) {
	a, ids, sp, lf, rt := gridSaved(t)
	d4, d4b, d2 := ids[1], ids[2], ids[3]
	checkMine(t, a, d4, 4, "spread", []tplWant{sp})
	c := a.GridChoices(d4, 4, "spread")
	if c.Mine[0].Drop != 0 || c.N != 4 {
		t.Fatalf("Drop %d, N %d", c.Mine[0].Drop, c.N)
	}
	r, err := a.ApplyGrid(d4, "spread", "spread", c.Mine[0].Boxes)
	if err != nil || r.Left != 0 {
		t.Fatalf("%v, left %d", err, r.Left)
	}
	samePlaces(t, "разворот", areaPlaces(spreadPlaces(a, d4), "spread"), sp.src, 0)
	for _, p := range spreadPlaces(a, d4) {
		if p.Photo == "" {
			t.Fatal("фото не встало на место шаблона")
		}
	}

	// «Перезапуск»: плитки и применение те же.
	b := restartApp(t, a)
	checkMine(t, b, d4b, 4, "spread", []tplWant{sp})
	checkMine(t, b, d2, 2, "left", []tplWant{lf, rt})  // страницы: правая на левую страницу, со сдвигом
	checkMine(t, b, d2, 2, "right", []tplWant{lf, rt}) // и левая на правую
}
