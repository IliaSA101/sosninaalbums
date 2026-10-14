package app

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// gridApp — App с разворотом-сеткой s1 [a, пусто] и «Авто» s2 [b]; папок в ленте нет.
func gridApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Grid: []*model.Place{
			{ID: "00000000000a", X: 12, Y: 12, W: 100, H: 100, Photo: "a"},
			{ID: "00000000000b", X: 312, Y: 12, W: 100, H: 100}}},
		{ID: "s2", Photos: []string{"b"}},
	}
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: `C:\b.jpg`, W: 300, H: 200}
	a.ed.Store(mustEditor(t, p, ""))
	return a
}

// Фото альбома, чьей папки нет в ленте, бросается на место по сведениям проекта;
// неизвестный ID пропускается.
func TestAppPlacePhotosUsesRefs(t *testing.T) {
	a := gridApp(t)
	r, err := a.PlacePhotos("s1", "00000000000b", []string{"b", "zz"})
	if err != nil || r.Left != 0 || len(r.View.Spreads) != 1 || !slices.Equal(r.View.Spreads[0].Photos, []string{"a", "b"}) {
		t.Fatalf("err %v, left %d, вид %+v", err, r.Left, r.View.Spreads)
	}
	if r.View.EmptyPlaces != 0 || !r.View.Spreads[0].Grid {
		t.Fatalf("пустых мест %d", r.View.EmptyPlaces)
	}
}

// Ошибка правки — отклонённый промис с текущим видом, Left = 0.
func TestAppGridErrors(t *testing.T) {
	a := gridApp(t)
	rev := a.editor().View().Rev
	r, err := a.ApplyGrid("s1", "left", "spread", []model.Box{{W: 0.5, H: 0.5}})
	if err == nil || r.Left != 0 || r.View.Rev != rev || len(r.View.Spreads) != 2 {
		t.Fatalf("ApplyGrid: err %v, %+v", err, r)
	}
	if _, err := a.ArrangePlaces("s1", "splitV", []string{"00000000000a"}); err != nil {
		t.Fatal(err)
	}
	r, err = a.CopyGrid("s1", "left", []model.GridTarget{{Spread: "s2", Scope: "spread"}}, false, true)
	if err != nil || r.Left != 0 || len(r.View.Spreads[1].Places) != 4 || r.View.Spreads[1].Places[0].Photo != "b" {
		t.Fatalf("CopyGrid: err %v, left %d", err, r.Left)
	}
	if plan := a.PlanCopyGrid("s1", "left", []model.GridTarget{{Spread: "s2", Scope: "left"}}, false, false); plan.Frozen != 0 {
		t.Fatalf("план: %+v", plan)
	}
}

// tplApp — App с папкой настроек во временной папке и разворотом из двух фото.
func tplApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	v, err := a.editor().NewSpread(-1, []model.PhotoInfo{
		{ID: "a", Path: "/a.jpg", W: 300, H: 200}, {ID: "b", Path: "/b.jpg", W: 200, H: 300},
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, v.Spreads[0].ID
}

func readTpl(t *testing.T, a *App) []byte {
	t.Helper()
	data, err := os.ReadFile(prefs.TemplatesPath(a.cfgDir))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func badFiles(t *testing.T, a *App) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(a.cfgDir, "templates.bad-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Шаблон переживает перезапуск: запись в cfgDir, чтение при старте, переименование и удаление.
func TestTemplatesRoundTrip(t *testing.T) {
	a, sid := tplApp(t)
	a.loadTemplates() // файла нет: пусто, не ошибка
	if n := len(a.templateList()); n != 0 {
		t.Fatalf("шаблонов %d", n)
	}
	if err := a.SaveGridTemplate(sid, "spread", " Мой\n шаблон "); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGridTemplate(sid, "left", ""); err != nil && !errors.Is(err, model.ErrNoPlaces) {
		t.Fatal(err)
	}
	if len(badFiles(t, a)) != 0 {
		t.Fatal("исправный файл не должен уходить в .bad")
	}

	b := newApp(t) // «перезапуск»
	b.cfgDir = a.cfgDir
	b.ed.Store(a.editor())
	b.loadTemplates()
	got := b.templateList()
	if !slices.EqualFunc(got, a.templateList(), func(x, y model.Template) bool {
		return x.ID == y.ID && x.Name == y.Name && x.Scope == y.Scope && slices.Equal(x.Boxes, y.Boxes)
	}) || len(got) == 0 || got[0].Name != "Мой шаблон" || got[0].Scope != model.TplSpread || len(got[0].Boxes) != 2 {
		t.Fatalf("после перезапуска: %+v", got)
	}
	if c := b.GridChoices(sid, 2, "spread"); len(c.Mine) == 0 || c.Mine[0].ID != got[0].ID || len(c.Builtin) == 0 {
		t.Fatalf("GridChoices: %+v", c)
	}

	id := got[0].ID
	if err := b.RenameGridTemplate(id, "Новое"); err != nil {
		t.Fatal(err)
	}
	if err := b.RenameGridTemplate("нет", "x"); err == nil {
		t.Fatal("переименован несуществующий")
	}
	b.loadTemplates()
	if l := b.templateList(); l[0].Name != "Новое" {
		t.Fatalf("имя не записалось: %+v", l)
	}
	if err := b.DeleteGridTemplate(id); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteGridTemplate(id); err == nil || err.Error() != "Шаблон не найден" {
		t.Fatalf("удалён дважды: %v", err)
	}
	b.loadTemplates()
	for _, tp := range b.templateList() {
		if tp.ID == id {
			t.Fatal("шаблон не удалился")
		}
	}
	// Разворот без мест: нечего сохранять.
	e := mustEditor(t, model.New(), "")
	b.ed.Store(e)
	if err := b.SaveGridTemplate("нет", "spread", "x"); err == nil {
		t.Fatal("сохранён шаблон несуществующего разворота")
	}
}

// Битый и слишком большой templates.json не затираются: перед первой записью — в .bad-<время>.
func TestTemplatesBadNotOverwritten(t *testing.T) {
	big := bytes.Repeat([]byte(" "), model.MaxTemplatesFile+10)
	for name, content := range map[string][]byte{"битый": []byte("{не json"), "большой": big, "без версии": []byte(`{"templates":[]}`)} {
		t.Run(name, func(t *testing.T) {
			a, sid := tplApp(t)
			if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), content, 0o644); err != nil {
				t.Fatal(err)
			}
			a.loadTemplates()
			if len(a.templateList()) != 0 || len(a.GridChoices(sid, 2, "spread").Builtin) == 0 {
				t.Fatal("битый файл должен читаться как пустой список")
			}
			if !bytes.Equal(readTpl(t, a), content) || len(badFiles(t, a)) != 0 {
				t.Fatal("чтение не должно менять файл")
			}
			if err := a.SaveGridTemplate(sid, "spread", "n"); err != nil {
				t.Fatal(err)
			}
			bad := badFiles(t, a)
			if len(bad) != 1 {
				t.Fatalf(".bad: %v", bad)
			}
			if old, _ := os.ReadFile(bad[0]); !bytes.Equal(old, content) {
				t.Fatal("в .bad лежит не исходный файл")
			}
			a.loadTemplates()
			if l := a.templateList(); len(l) != 1 || l[0].Name != "n" {
				t.Fatalf("новый файл: %+v", l)
			}
			if err := a.SaveGridTemplate(sid, "spread", "m"); err != nil || len(badFiles(t, a)) != 1 {
				t.Fatalf("второй .bad не нужен: %v %v", err, badFiles(t, a))
			}
		})
	}
}

// Если .bad убрать не удалось, файл остаётся как был и запись отказывает.
func TestTemplatesBadRenameFails(t *testing.T) {
	a, sid := tplApp(t)
	content := []byte("{не json")
	if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	a.rename = func(string, string) error { return errors.New("занято") }
	if err := a.SaveGridTemplate(sid, "spread", "n"); err == nil {
		t.Fatal("запись прошла при неубранном битом файле")
	}
	if !bytes.Equal(readTpl(t, a), content) {
		t.Fatal("битый файл затёрт")
	}
}

// Файл новой версии читается, но не пишется ни одним методом.
func TestTemplatesNewerReadOnly(t *testing.T) {
	a, sid := tplApp(t)
	content := []byte(`{"version":5,"templates":[{"id":"0123456789ab","name":"Н","scope":"spread","boxes":[{"x":0,"y":0,"w":0.5,"h":0.5},{"x":0.5,"y":0.5,"w":0.5,"h":0.5}]}]}`)
	if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	if c := a.GridChoices(sid, 2, "spread"); len(c.Mine) != 1 || c.Mine[0].Name != "Н" {
		t.Fatalf("шаблоны новой версии не видны: %+v", c.Mine)
	}
	for name, err := range map[string]error{
		"save":   a.SaveGridTemplate(sid, "spread", "x"),
		"rename": a.RenameGridTemplate("0123456789ab", "x"),
		"delete": a.DeleteGridTemplate("0123456789ab"),
	} {
		if !errors.Is(err, model.ErrTemplatesNewer) || !strings.Contains(err.Error(), "более новой версией") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if !bytes.Equal(readTpl(t, a), content) || len(badFiles(t, a)) != 0 {
		t.Fatal("файл новой версии изменён")
	}
}

// Методы App над сеткой: места, слои, выравнивание, зеркало, копии — вид меняется как задумано.
func TestAppGridSurface(t *testing.T) {
	a := gridApp(t)
	const pa, pb = "00000000000a", "00000000000b"
	rev := func() int { return a.editor().View().Rev }
	places := func(v model.View) []model.PlaceView { return v.Spreads[0].Places }
	grew := func(name string, before int, v model.View) {
		t.Helper()
		if v.Rev <= before {
			t.Errorf("%s: правка не применена (rev %d → %d)", name, before, v.Rev)
		}
	}

	r := rev()
	v, err := a.AddPlace("s1", 0)
	if err != nil {
		t.Fatal(err)
	}
	grew("AddPlace", r, v)
	if len(places(v)) != 3 {
		t.Fatalf("мест после AddPlace: %d", len(places(v)))
	}
	newID := places(v)[2].ID

	r = rev()
	v = a.SetPlaces("s1", []model.PlaceBox{{ID: pb, X: 300, Y: 20, W: 90, H: 80}}, "")
	grew("SetPlaces", r, v)
	if p := places(v)[1]; p.X != 300 || p.W != 90 || p.H != 80 {
		t.Errorf("место: %+v", p)
	}
	r = rev()
	v = a.SetPlaceAngle("s1", pb, 20)
	grew("SetPlaceAngle", r, v)
	if places(v)[1].Rot != 20 {
		t.Errorf("угол места: %v", places(v)[1].Rot)
	}
	r = rev()
	v = a.SetPlaceFit("s1", []string{pa}, "contain")
	grew("SetPlaceFit", r, v)
	if places(v)[0].Fit != "contain" {
		t.Errorf("fit: %q", places(v)[0].Fit)
	}
	r = rev()
	v = a.ArrangeLayers("s1", "front", []string{pa})
	grew("ArrangeLayers", r, v)

	// Выравнивание и распределение: два места, op влево.
	r = rev()
	v = a.AlignItems("s1", []string{pa, pb}, "left", "sel")
	grew("AlignItems", r, v)
	// место b повёрнуто: выравнивается его описанная рамка, поэтому сравниваем сдвиг, а не X.
	if places(v)[1].X >= 300 {
		t.Errorf("выравнивание влево не сдвинуло место: X %v", places(v)[1].X)
	}
	// Два места по всему развороту помещаются без наложения, три в рамке выделения — нет.
	if a.AlignOverlap("s1", []string{pa, pb}, "distH", "spread") {
		t.Error("AlignOverlap: два места по развороту не должны перекрываться")
	}
	if !a.AlignOverlap("s1", []string{pa, pb, newID}, "distH", "sel") {
		t.Error("AlignOverlap: три места в рамке выделения должны перекрываться")
	}

	// Группа: перенос, поворот, масштаб по осям.
	r = rev()
	v = a.TransformGroup("s1", []string{pa, pb}, 1, 0, 0, 5, 0, "")
	grew("TransformGroup", r, v)
	r = rev()
	v = a.RotateGroup("s1", []string{pa, pb}, 5, 210, 148)
	grew("RotateGroup", r, v)
	r = rev()
	v = a.ScaleGroupAxes("s1", []string{pa, pb}, 0.9, 0.9, 0, 210, 148)
	grew("ScaleGroupAxes", r, v)

	// Зеркало сетки и закрепление «Авто».
	before := places(a.editor().View())[1] // newID: X 110, W 100, без поворота
	if before.ID != newID {
		t.Fatalf("порядок мест изменился: %+v", before)
	}
	r = rev()
	v = a.MirrorGrid("s1", "spread")
	grew("MirrorGrid", r, v)
	// Отражение по ширине разворота (600 мм): X' = 600 − X − W.
	if p := places(v)[1]; math.Abs(p.X-(600-before.X-before.W)) > 0.01 {
		t.Errorf("MirrorGrid: X %v, ждали %v", p.X, 600-before.X-before.W)
	}
	r = rev()
	v = a.SetGrid("s2", true)
	grew("SetGrid", r, v)
	if !v.Spreads[1].Grid {
		t.Error("разворот s2 не закреплён")
	}

	// Заполнение пустых мест фото из ленты: фото a и b уже в альбоме — у ленты только неизвестные.
	if res := a.FillPlaces([]string{"zz"}, "s1"); res.Left != 0 {
		t.Errorf("FillPlaces неизвестного: %+v", res)
	}

	// Дубликат: новый разворот с теми же местами без фото.
	r = rev()
	v, err = a.DuplicateGrid("s1", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	grew("DuplicateGrid", r, v)
	if len(v.Spreads) != 3 {
		t.Errorf("разворотов после дубликата: %d", len(v.Spreads))
	}

	// Удаление мест и выделения: пустое место уходит, фото остаётся в ленте разворота.
	r = rev()
	v = a.DeletePlaces("s1", []string{newID})
	grew("DeletePlaces", r, v)
	if slices.ContainsFunc(places(v), func(p model.PlaceView) bool { return p.ID == newID }) {
		t.Error("место не удалено")
	}
	r = rev()
	v = a.DeleteSelection("s1", nil, []string{pb}, nil)
	grew("DeleteSelection", r, v)
	if slices.ContainsFunc(places(v), func(p model.PlaceView) bool { return p.ID == pb }) {
		t.Error("выделенное место не удалено")
	}
}

// vigTplApp — App с листом-виньеткой 204×304 (1 учитель, 24 ученика) и папкой настроек.
func vigTplApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	e := mustEditor(t, album.NewVignetteSheet(204, 304), "")
	a.ed.Store(e)
	return a, e.View().Spreads[0].ID
}

// Свои шаблоны виньетки (038): сохранить, пережить «перезапуск», применить по "my:", переименовать
// и удалить по VigChoice.id; лист после удаления шаблона продолжает работать.
func TestSaveVignetteTemplate(t *testing.T) {
	a, sid := vigTplApp(t)
	if err := a.SaveVignetteTemplate(sid, " Мой\n класс "); err != nil {
		t.Fatal(err)
	}
	got := a.templateList()
	if len(got) != 1 || got[0].Name != "Мой класс" || got[0].Kind != model.TplVignette || got[0].Vig == nil ||
		len(got[0].Boxes) != 25 || len(got[0].Vig.Roles) != 25 || got[0].Scope != model.TplSpread {
		t.Fatalf("шаблон: %+v", got)
	}
	if raw := readTpl(t, a); !bytes.Contains(raw, []byte(`"version":4`)) || !bytes.Contains(raw, []byte(`"kind":"vignette"`)) {
		t.Fatalf("файл: %.80s", raw)
	}
	b := newApp(t) // «перезапуск»
	b.cfgDir, b.emit = a.cfgDir, func(string, ...any) {}
	b.ed.Store(a.editor())
	b.loadTemplates()
	if l := b.templateList(); len(l) != 1 || l[0].ID != got[0].ID || *l[0].Vig.Style.Caps != *got[0].Vig.Style.Caps {
		t.Fatalf("после перезапуска: %+v", l)
	}
	// В «Моих шаблонах» сетки его нет.
	if c := b.GridChoices(sid, 25, "spread"); len(c.Mine) != 0 {
		t.Fatalf("Mine: %+v", c.Mine)
	}
	id := model.VigOwnPrefix + got[0].ID
	cs := b.VignetteChoices(sid, 1, 24, "3:4")
	if last := cs[len(cs)-1]; len(cs) != 18 || last.ID != id || !last.Own || last.Gone || last.Name != "Мой класс" {
		t.Fatalf("VignetteChoices: %d, %+v", len(cs), cs[len(cs)-1])
	}
	r, err := b.SetVignette(sid, model.Vignette{Tpl: id, Teachers: 1, Pupils: 20})
	if err != nil || r.Left != 0 || r.View.Spreads[0].Vig.OwnName != "Мой класс" || r.View.Spreads[0].Vig.Manual {
		t.Fatalf("SetVignette: %v, %+v", err, r.View.Spreads[0].Vig)
	}
	if _, err := b.PreviewVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: id, Teachers: 1, Pupils: 18}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetupVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: id, Teachers: 1, Pupils: 22}}); err != nil {
		t.Fatal(err)
	}
	// Переименование и удаление — по id карточки как есть («my:…»).
	if err := b.RenameGridTemplate(id, "Новый"); err != nil {
		t.Fatal(err)
	}
	if cs = b.VignetteChoices(sid, 1, 22, "3:4"); cs[len(cs)-1].Name != "Новый" {
		t.Fatalf("имя после переименования: %+v", cs[len(cs)-1])
	}
	if err := b.DeleteGridTemplate(id); err != nil {
		t.Fatal(err)
	}
	if len(b.templateList()) != 0 {
		t.Fatal("шаблон не удалился")
	}
	if cs = b.VignetteChoices(sid, 1, 22, "3:4"); len(cs) != 18 || !cs[17].Gone {
		t.Fatalf("после удаления: %d, %+v", len(cs), cs[len(cs)-1])
	}
	if r, err = b.SetVignette(sid, model.Vignette{Tpl: id, Teachers: 1, Pupils: 19}); err != nil || r.Left != 0 || len(r.View.Spreads[0].Places) != 20 {
		t.Fatalf("лист без шаблона: %v, left %d, мест %d", err, r.Left, len(r.View.Spreads[0].Places))
	}
	if _, err := b.SetVignette(sid, model.Vignette{Tpl: "my:чужой", Teachers: 1, Pupils: 19}); err == nil || err.Error() != "Шаблон не найден" {
		t.Fatalf("чужой шаблон: %v", err)
	}
	if err := b.DeleteGridTemplate(id); err == nil {
		t.Fatal("удалён дважды")
	}
}

// Сохранение: не виньетка и лист без мест — ошибка без записи; новее файл — ErrTemplatesNewer.
func TestSaveVignetteTemplateRefusals(t *testing.T) {
	a, sid := vigTplApp(t)
	a.editor().ClearVignette(sid)
	if err := a.SaveVignetteTemplate(sid, "x"); err == nil || err.Error() != "Это не виньетка" {
		t.Fatalf("обычный разворот: %v", err)
	}
	if err := a.SaveVignetteTemplate("нет", "x"); err == nil {
		t.Fatal("неизвестный лист")
	}
	if _, err := a.SetVignette(sid, model.Vignette{Tpl: "grid"}); err != nil { // виньетка без людей
		t.Fatal(err)
	}
	if err := a.SaveVignetteTemplate(sid, "x"); !errors.Is(err, model.ErrNoPlaces) {
		t.Fatalf("без мест: %v", err)
	}
	if _, err := os.Stat(prefs.TemplatesPath(a.cfgDir)); err == nil || len(a.templateList()) != 0 {
		t.Fatal("отказ оставил шаблон или файл")
	}
	if _, err := a.SetVignette(sid, model.Vignette{Tpl: "grid", Pupils: 10}); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"version":5,"templates":[]}`)
	if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	if err := a.SaveVignetteTemplate(sid, "x"); !errors.Is(err, model.ErrTemplatesNewer) || !bytes.Equal(readTpl(t, a), content) {
		t.Fatalf("новая версия: %v", err)
	}
}

// Предел файла 1 МБ: сохранение, дающее файл больше, отказывает ErrTemplatesFull — файл и список прежние.
func TestSaveVignetteTemplateFull(t *testing.T) {
	a, sid := vigTplApp(t)
	long := strings.Repeat("я", model.MaxCaptionRunes)
	if _, err := a.SetVignette(sid, model.Vignette{Tpl: "grid", Pupils: 64, TText: long}); err != nil {
		t.Fatal(err)
	}
	caps := *a.editor().View().Spreads[0].Caps
	caps.Text = long // длинные шаблоны подписей раздувают шаблон: ~1,2 КБ сверх мест
	if v := a.SetSheetCaps(sid, true, caps); v.Spreads[0].Caps.Text != long {
		t.Fatal("шаблон подписей не встал")
	}
	var err error
	n := 0
	for n < model.MaxTemplates {
		if err = a.SaveVignetteTemplate(sid, ""); err != nil {
			break
		}
		n++
	}
	if !errors.Is(err, model.ErrTemplatesFull) || n == 0 || n == model.MaxTemplates {
		t.Fatalf("после %d шаблонов: %v", n, err)
	}
	if len(a.templateList()) != n || len(readTpl(t, a)) > model.MaxTemplatesFile || len(badFiles(t, a)) != 0 {
		t.Fatalf("после отказа: шаблонов %d из %d, файл %d байт", len(a.templateList()), n, len(readTpl(t, a)))
	}
	before := readTpl(t, a)
	if err := a.SaveVignetteTemplate(sid, "ещё"); !errors.Is(err, model.ErrTemplatesFull) || !bytes.Equal(readTpl(t, a), before) {
		t.Fatalf("повторный отказ: %v", err)
	}
	// Удаление освобождает место: после него сохранение снова проходит.
	if err := a.DeleteGridTemplate(model.VigOwnPrefix + a.templateList()[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveVignetteTemplate(sid, "ещё"); err != nil {
		t.Fatalf("после удаления: %v", err)
	}
}

// «Вернуть авто» из App: один шаг отмены, Left, холостая на нетронутой.
func TestAppResetVignette(t *testing.T) {
	a, sid := vigTplApp(t)
	rev := a.editor().View().Rev
	if r, err := a.ResetVignette(sid); err != nil || r.View.Rev != rev || r.Left != 0 {
		t.Fatalf("нетронутая: %v, %+v", err, r.Left)
	}
	pl := a.editor().View().Spreads[0].Places[0]
	v := a.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 2, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	if !v.Spreads[0].Vig.Manual {
		t.Fatal("сдвиг не сделал виньетку ручной")
	}
	r, err := a.ResetVignette(sid)
	if err != nil || r.View.Spreads[0].Vig.Manual || r.View.Rev != v.Rev+1 {
		t.Fatalf("Reset: %v, manual %v, rev %d→%d", err, r.View.Spreads[0].Vig.Manual, v.Rev, r.View.Rev)
	}
	if v = a.editor().Undo(); !v.Spreads[0].Vig.Manual {
		t.Fatal("одна отмена не вернула сдвиг")
	}
	if _, err := a.ResetVignette("нет"); err == nil {
		t.Fatal("неизвестный лист")
	}
}

// Свой шаблон на развороте с другим числом фото: ряд «Мои» берётся по числу мест, лишние фото
// уходят в ленту (Drop, Left), пустые места остаются; отмена — один шаг; холостая правка.
func TestTemplateGridOtherCounts(t *testing.T) {
	a, ids, sp, lf, rt := gridSaved(t)
	d4b, d2, d6, d6b := ids[2], ids[3], ids[4], ids[5]
	if c := a.GridChoices(d2, 0, "spread"); c.N != 2 || len(c.Mine) != 2 || c.Mine[0].ID != lf.id || c.Mine[1].ID != rt.id {
		t.Fatalf("2 фото: %+v", c.Mine)
	}
	c := a.GridChoices(d2, 4, "spread")
	if len(c.Mine) != 1 || c.Mine[0].ID != sp.id || c.Mine[0].Drop != 0 {
		t.Fatalf("на 4 места при 2 фото: %+v", c.Mine)
	}
	r, err := a.ApplyGrid(d2, "spread", "spread", c.Mine[0].Boxes)
	if err != nil || r.Left != 0 {
		t.Fatalf("%v, left %d", err, r.Left)
	}
	if v := spreadView(a, d2); len(v.Places) != 4 || len(v.Photos) != 2 {
		t.Fatalf("2 фото в 4 местах: мест %d, фото %d", len(v.Places), len(v.Photos))
	}
	a.editor().Undo()
	if c = a.GridChoices(d6, 4, "spread"); len(c.Mine) != 1 || c.Mine[0].Drop != 2 {
		t.Fatalf("6 фото на 4 места: %+v", c.Mine)
	}
	if r, err = a.ApplyGrid(d6, "spread", "spread", c.Mine[0].Boxes); err != nil || r.Left != 2 {
		t.Fatalf("%v, left %d", err, r.Left)
	}
	if v := spreadView(a, d6); len(v.Photos) != 4 {
		t.Fatalf("фото осталось %d, ждали 4", len(v.Photos))
	}
	samePlaces(t, "6 фото", areaPlaces(spreadPlaces(a, d6), "spread"), sp.src, 0)
	a.editor().Undo()
	if v := spreadView(a, d6); len(v.Photos) != 6 || v.Grid {
		t.Fatalf("отмена: фото %d, сетка %v", len(v.Photos), v.Grid)
	}
	if c = a.GridChoices(d6, 0, "spread"); c.N != 6 || len(c.Mine) != 0 {
		t.Fatalf("на 6 мест шаблонов нет: %+v", c.Mine)
	}
	// Шаблон страницы на разворот — места на обе страницы, фото сверх мест уходят в ленту.
	if c = a.GridChoices(d6b, 2, "spread"); len(c.Mine) != 2 || c.Mine[0].Drop != 2 {
		t.Fatalf("страница на разворот: %+v", c.Mine)
	}
	if r, err = a.ApplyGrid(d6b, "spread", "page", c.Mine[0].Boxes); err != nil || r.Left != 2 {
		t.Fatalf("%v, left %d", err, r.Left)
	}
	got := areaPlaces(spreadPlaces(a, d6b), "spread")
	samePlaces(t, "страница: левая", got[:2], lf.src, 0)
	samePlaces(t, "страница: правая", got[2:], lf.src, 300)

	// Холостая правка: пустые и неверные доли — отказ без шага и rev.
	a.editor().Undo()
	rev := a.editor().View().Rev
	for _, bad := range [][]model.Box{nil, {{X: math.NaN(), Y: 0, W: 0.5, H: 0.5}}} {
		if r, err = a.ApplyGrid(d4b, "spread", "spread", bad); err == nil || r.View.Rev != rev || a.editor().View().Rev != rev {
			t.Fatalf("неверные доли: %v, rev %d→%d", err, rev, a.editor().View().Rev)
		}
	}
}
