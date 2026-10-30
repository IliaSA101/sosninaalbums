package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Свои шаблоны виньетки (038): форма шаблона в листе (vig.own), формат v7.

const ownTpl = model.VigOwnPrefix + "0123456789ab"

// ownShape — форма шаблона разворота 600×300: учитель, два ученика, групповое; шапка сверху.
func ownShape() *model.VigShape {
	return &model.VigShape{Name: "Мой класс",
		Boxes: []model.Box{{X: 0.03, Y: 0.25, W: 0.1, H: 0.27}, {X: 0.53, Y: 0.25, W: 0.075, H: 0.2},
			{X: 0.63, Y: 0.25, W: 0.075, H: 0.2}, {X: 0.2, Y: 0.6, W: 0.3, H: 0.3}},
		VigGeo: model.VigGeo{Roles: []string{model.RoleTeacher, model.RolePupil, model.RolePupil, model.RoleGroup},
			Head: &model.Box{X: 0.05, Y: 0.02, W: 0.9, H: 0.15}, Pages: 2, Aspect: 2, Ratio: "4:5", W: 600, H: 300}}
}

// vigSpread — виньетка id с параметрами v и местами по ролям учитель, ученик, ученик, групповое.
func vigSpread(id string, v model.Vignette) *model.Spread {
	roles := []string{model.RoleTeacher, model.RolePupil, model.RolePupil, model.RoleGroup}
	s := &model.Spread{ID: id, Photos: []string{}, Kind: model.KindVignette, Vig: &v}
	for i, r := range roles {
		s.Grid = append(s.Grid, &model.Place{ID: fmt.Sprintf("c0ffee%s%02x", id[len(id)-4:], i), X: 20 + float64(i)*100, Y: 80, W: 60, H: 80, Role: r})
	}
	return s
}

// ownFile — файл текущей версии: альбом 300×300 с данными разворотами.
func ownFile(t *testing.T, spreads ...*model.Spread) []byte {
	t.Helper()
	st := model.DefaultSettings()
	st.PageW, st.PageH = 300, 300
	data, err := json.Marshal(salbum.FileProject{Version: model.FormatVersion, Settings: st, Folders: []salbum.FileFolder{},
		Photos: map[string]salbum.FilePhoto{}, Spreads: spreads})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Круг Save → Load: форма своего шаблона в листе без потерь рядом с виньеткой каталога и
// обычным разворотом; вид отдаёт имя вместо формы; первая правка после открытия холостая.
func TestSaveLoadVigOwn(t *testing.T) {
	dir := testkit.TempDir(t)
	own := model.Vignette{Tpl: ownTpl, Teachers: 1, Pupils: 2, Ratio: "4:5", Own: ownShape(), Shape: model.ShapeOval}
	cat := model.Vignette{Tpl: "g-top", Teachers: 1, Pupils: 2, Ratio: model.VigRatio, Round: 10}
	data := ownFile(t, vigSpread("aaaaaaaa0001", own), vigSpread("aaaaaaaa0002", cat), &model.Spread{ID: "aaaaaaaa0003", Photos: []string{}, Name: "Просто"})
	p := edtest.MustDecode(t, data, dir)
	s1, s2 := p.Spreads[0], p.Spreads[1]
	if s1.Vig == nil || !s1.Vig.Same(own) || !s1.Vig.Own.Equal(ownShape()) || s1.Vig.Own.Groups() != 1 {
		t.Fatalf("форма после чтения: %+v", s1.Vig)
	}
	if s2.Vig == nil || s2.Vig.Own != nil || !s2.Vig.Same(cat) || model.VigPlaces(*s1.Vig) != 4 || model.VigPlaces(*s2.Vig) != 4 {
		t.Fatalf("каталог: %+v", s2.Vig)
	}
	e := edtest.Open(t, p, "")
	v := e.View()
	vv0, vv1 := v.Spreads[0].Vig, v.Spreads[1].Vig
	if vv0 == nil || vv0.OwnName != "Мой класс" || vv0.Own != nil || !vv0.Group || vv0.Places != 4 || vv0.Tpl != ownTpl {
		t.Fatalf("вид своего: %+v", vv0)
	}
	if vv1 == nil || vv1.OwnName != "" || !vv1.Group || vv1.Places != 4 {
		t.Fatalf("вид каталога: %+v", vv1)
	}
	if raw, _ := json.Marshal(vv0); bytes.Contains(raw, []byte(`"own":`)) || !bytes.Contains(raw, []byte(`"ownName":"Мой класс"`)) {
		t.Fatalf("вид таскает форму: %s", raw)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.Change(e, func(*model.Project) {})
	edtest.Same(t, e, rev, undo, "первая правка после открытия")
	for _, s := range edtest.P(e).Spreads {
		if err := edtest.VigInvariant(s); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "own.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion || bytes.Count(raw, []byte(`"own"`)) != 1 {
		t.Fatalf("файл: версия %d, own %d раз", f.Version, bytes.Count(raw, []byte(`"own"`)))
	}
	var doc struct { // размер листа шаблона (точный режим) — в vig.own файла
		Spreads []struct {
			Vig *struct{ Own *struct{ W, H float64 } }
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Spreads[0].Vig.Own == nil || doc.Spreads[0].Vig.Own.W != 600 || doc.Spreads[0].Vig.Own.H != 300 {
		t.Fatalf("vig.own без размера листа: %v", err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), model.Marshal(q)) || !q.Spreads[0].Vig.Own.Equal(ownShape()) {
		t.Fatalf("потери при круге:\n%s\n%s", model.Marshal(edtest.P(e)), model.Marshal(q))
	}
	if q0, q1 := q.Spreads[0].Vig, q.Spreads[1].Vig; q0.Shape != model.ShapeOval || q0.Round != 0 || q1.Shape != "" || q1.Round != 10 {
		t.Fatalf("форма виньетки после круга: %+v, %+v", q0, q1)
	}
}

// Load: плохая форма снимается (шаблон по умолчанию) или зажимается; форма не к своему
// шаблону или не к числу страниц листа — прочь; групповые шаблона входят в предел мест
// (сверх — меньше учеников, затем учителей); второй FixVignettes ничего не меняет.
func TestLoadClampsVigOwn(t *testing.T) {
	def := model.DefaultVignette().Tpl
	with := func(f func(*model.Vignette)) model.Vignette {
		v := model.Vignette{Tpl: ownTpl, Teachers: 1, Pupils: 2, Ratio: "4:5", Own: ownShape()}
		f(&v)
		return v
	}
	groups := func(n int) func(*model.Vignette) {
		return func(v *model.Vignette) {
			v.Teachers, v.Pupils = model.MaxTeachers, model.MaxPupils
			v.Own.Boxes = slices.Repeat(v.Own.Boxes[:1], n+1)
			v.Own.Roles = append(slices.Repeat([]string{model.RoleGroup}, n), model.RolePupil)
		}
	}
	vs := []model.Vignette{
		with(func(v *model.Vignette) {
			v.Own.Roles = []string{model.RoleGroup, model.RoleGroup, model.RoleGroup, model.RoleGroup}
		}), // 0
		with(func(v *model.Vignette) { v.Own.Pages = 1 }),   // 1: форма листа на развороте
		with(func(v *model.Vignette) { v.Tpl = "g-top" }),   // 2: форма у шаблона каталога
		with(func(v *model.Vignette) { v.Own = nil }),       // 3: свой шаблон без формы
		with(func(v *model.Vignette) { v.Own.Pages = 7 }),   // 4
		with(func(v *model.Vignette) { v.Own.Boxes = nil }), // 5
		with(func(v *model.Vignette) { // 6: зажим полей
			v.Own.Name = "  \u0000 "
			v.Own.Aspect, v.Own.Ratio = 1e9, "16:9"
			v.Own.Roles = v.Own.Roles[:1]
			v.Own.Boxes[1].X = 5
			v.Own.Head = &model.Box{X: -1, Y: -1, W: 3, H: 0}
			v.Own.W = 49 // меньше страницы
		}),
		with(groups(3)),                // 7: 18 + 64 + 3 групповых
		with(groups(63)),               // 8: групповых 63 — учеников нет, учитель один
		with(func(*model.Vignette) {}), // 9: исправная
		with(func(v *model.Vignette) { v.Own.W, v.Own.H = 0, 0 }),     // 10: шаблон до поля размера
		with(func(v *model.Vignette) { v.Own.W, v.Own.H = 600, 1e9 }), // 11: одна сторона негодная
		with(func(v *model.Vignette) { v.Own.W, v.Own.H = -600, 300 }),
	}
	var spreads []*model.Spread
	for i, v := range vs {
		spreads = append(spreads, vigSpread(fmt.Sprintf("5e1f0a2b%04x", i), v))
	}
	dir := testkit.TempDir(t)
	p := edtest.MustDecode(t, ownFile(t, spreads...), dir)
	if len(p.Spreads) != len(vs) {
		t.Fatalf("разворотов %d", len(p.Spreads))
	}
	got := func(i int) model.Vignette { return *p.Spreads[i].Vig }
	for _, i := range []int{0, 1, 3, 4, 5} {
		if v := got(i); v.Own != nil || v.Tpl != def {
			t.Errorf("%d: форма %+v, шаблон %q — ждали снятую и %q", i, v.Own, v.Tpl, def)
		}
	}
	if v := got(2); v.Own != nil || v.Tpl != "g-top" {
		t.Errorf("2: %+v", v)
	}
	o := got(6).Own
	if o == nil || o.Name != "Шаблон 1" || o.Aspect != model.MaxVigAspect || o.Ratio != model.VigRatio ||
		!slices.Equal(o.Roles, []string{model.RoleTeacher, model.RolePupil, model.RolePupil, model.RolePupil}) ||
		o.Boxes[1].X+o.Boxes[1].W > 1 || o.Head == nil || *o.Head != (model.Box{W: 1, H: model.MinBoxFrac}) || o.W != 0 || o.H != 0 {
		t.Errorf("6: %+v", o)
	}
	if v := got(7); v.Own.Groups() != 3 || v.Teachers != model.MaxTeachers || v.Pupils != model.MaxPlaces-model.MaxTeachers-3 || model.VigPlaces(v) != model.MaxPlaces {
		t.Errorf("7: учителей %d, учеников %d, мест %d", v.Teachers, v.Pupils, model.VigPlaces(v))
	}
	if v := got(8); v.Own.Groups() != 63 || v.Teachers != 1 || v.Pupils != 0 || model.VigPlaces(v) != model.MaxPlaces {
		t.Errorf("8: учителей %d, учеников %d, мест %d", v.Teachers, v.Pupils, model.VigPlaces(v))
	}
	if v := got(9); !v.Own.Equal(ownShape()) || v.Tpl != ownTpl {
		t.Errorf("9: исправная форма изменена: %+v", v)
	}
	// Без размера (или с негодным) форма та же, размер 0 — шаблон применяется пересчётом.
	for _, i := range []int{10, 11, 12} {
		o, want := got(i).Own, ownShape()
		want.W, want.H = 0, 0
		if !o.Equal(want) || got(i).Tpl != ownTpl {
			t.Errorf("%d: %+v", i, o)
		}
	}
	for _, s := range p.Spreads {
		if err := edtest.VigInvariant(s); err != nil {
			t.Fatal(err)
		}
	}
	// Второй проход — те же указатели: менять нечего, копии разворота делят форму.
	before, vigs := model.Marshal(p), make([]*model.Vignette, len(p.Spreads))
	for i, s := range p.Spreads {
		vigs[i] = s.Vig
	}
	p.FixVignettes()
	for i, s := range p.Spreads {
		if s.Vig != vigs[i] {
			t.Errorf("%d: второй FixVignettes заменил параметры", i)
		}
	}
	if !bytes.Equal(before, model.Marshal(p)) {
		t.Fatal("второй FixVignettes изменил проект")
	}
}

// NaN в форме (из правки: в JSON-файле NaN не бывает) — место уходит вместе с ролью,
// пропорция 1; формы нет — указатель прежний, ничего не меняется.
func TestClampVignetteOwnNaN(t *testing.T) {
	sh := ownShape()
	sh.Boxes[0].W, sh.Aspect = math.NaN(), math.NaN()
	v := model.ClampVignette(model.Vignette{Tpl: ownTpl, Own: sh})
	if v.Own == nil || v.Own == sh || v.Own.Aspect != 1 || len(v.Own.Boxes) != 3 ||
		!slices.Equal(v.Own.Roles, []string{model.RolePupil, model.RolePupil, model.RoleGroup}) || !math.IsNaN(sh.Aspect) {
		t.Fatalf("зажим NaN: %+v (вход %+v)", v.Own, sh)
	}
	good := ownShape()
	if w := model.ClampVignette(model.Vignette{Tpl: ownTpl, Own: good}); w.Own != good {
		t.Fatal("исправная форма заменена копией")
	}
	if w := model.ClampVignette(model.Vignette{Tpl: "g-top"}); w.Own != nil || w.Tpl != "g-top" {
		t.Fatalf("без формы: %+v", w)
	}
	a, b := model.Vignette{Tpl: ownTpl, Own: ownShape()}, model.Vignette{Tpl: ownTpl, Own: ownShape()}
	if !a.Same(b) || a.Same(model.Vignette{Tpl: ownTpl}) || (model.Vignette{}).Same(a) {
		t.Fatal("Same: форма сравнивается по содержимому, nil — только с nil")
	}
	b.Own.Head.H = 0.2
	if a.Same(b) {
		t.Fatal("Same не видит другую шапку")
	}
}

// Golden v6 (testdata/v6.salbum.json: виньетка с подписями, шапкой и полями) открывается как
// текущая версия (038: v7, 045: v8): меняется только номер; первая запись делает .v6.bak с байтами исходника.
func TestMigrateV6ToV7(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v6.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := p.Spreads[1]
	if p.Version != model.FormatVersion || edtest.From(p) != 6 || s.Kind != model.KindVignette || s.Vig == nil || s.Vig.Own != nil ||
		s.Vig.Head != "c0ffee0000a1" || s.Caps == nil || s.Vars == nil || s.Vars.Title != "Школа № 5" || len(s.Grid) != 3 {
		t.Fatalf("версия %d, from %d, виньетка %+v %+v", p.Version, edtest.From(p), s, s.Vig)
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatal("запись поверх v6 должна делать бэкап")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(salbum.BackupName(path, 6, 1)); err != nil || !bytes.Equal(bak, data) {
		t.Fatalf("копия для 0.10: %v, совпадает с исходником: %v", err, bytes.Equal(bak, data))
	}
	saved, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion || strings.Contains(string(saved), `"own"`) {
		t.Fatalf("в файле версия %d, own: %v", f.Version, strings.Contains(string(saved), `"own"`))
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("потери при v6 → текущей:\n%s\n%s", before, model.Marshal(q))
	}
}
