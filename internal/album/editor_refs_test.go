package album

// Привязки к выгруженным альбомам (167, editor_refs.go): круг Save → Load, SetRef и KeepRefs без
// шага отмены, следование за отпечатком при Restore и Ctrl+Z, «Сохранить как», снимки, шаблон.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// refsFor — привязки проекта с отпечатком uid к альбомам ids.
func refsFor(uid string, ids ...string) []string {
	var out []string
	for _, id := range ids {
		out = append(out, model.LinkRef(uid, id))
	}
	return out
}

// refsEditor — Editor над файлом path с проектом из отпечатка и привязок к альбомам ids.
func refsEditor(t *testing.T, path string, ids ...string) (*Editor, string) {
	t.Helper()
	p := model.New()
	p.UID = model.NewUID()
	p.SetShareRefs(refsFor(p.UID, ids...))
	return uidEditor(t, p, path), p.UID
}

// fileRefs — привязки в файле path.
func fileRefs(t *testing.T, path string) []string {
	t.Helper()
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return q.ShareRefs()
}

// Открытие файла берёт его привязки; правка и запись (в тот же путь и в другой тем же проектом)
// их сохраняют; круг Save → Load даёт те же.
func TestOpenSaveKeepsLinks(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, uid := refsEditor(t, x, "a", "b")
	want := refsFor(uid, "a", "b")
	if got := e.ShareRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("после открытия %v, ждали %v", got, want)
	}
	st := e.p.Settings
	st.Gap++
	e.SetSettings(st)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, x); !reflect.DeepEqual(got, want) {
		t.Fatalf("после правки и записи в файле %v", got)
	}
	// Запись тем же проектом в другой путь (Save, не SaveOver) привязки несёт.
	y := filepath.Join(dir, "y.salbum")
	if err := e.Save(y); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, y); !reflect.DeepEqual(got, want) {
		t.Fatalf("Save в другой путь: %v", got)
	}
	// Черновик (WriteCopy) тоже несёт.
	d := filepath.Join(dir, "draft.salbum")
	if err := e.WriteCopy(d); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, d); !reflect.DeepEqual(got, want) {
		t.Fatalf("черновик: %v", got)
	}
	// Файл без привязок — Editor без привязок, и в файл их не пишет.
	e2, _ := refsEditor(t, filepath.Join(dir, "z.salbum"))
	if err := e2.Save(""); err != nil || len(e2.ShareRefs()) != 0 || len(fileRefs(t, e2.Path())) != 0 {
		t.Fatalf("пустые привязки: %v %v", e2.ShareRefs(), err)
	}
}

// SetRef: первой, без шага отмены, rev и «изменён»; ложится в файл ближайшей записью; не больше
// четырёх; дубль не растёт; негодная и чужой альбом — отказ.
func TestEditorSetRef(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, uid := refsEditor(t, x)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		rev := e.rev
		if err := e.SetRef(x, uid, model.LinkRef(uid, id)); err != nil {
			t.Fatal(err)
		}
		if len(e.undo) != 0 || e.dirty || e.rev != rev || len(e.redo) != 0 {
			t.Fatalf("SetRef — правка: шагов %d, dirty %v, rev %d→%d", len(e.undo), e.dirty, rev, e.rev)
		}
	}
	// Новая первой, лишнее (a) отброшено.
	want := refsFor(uid, "e", "d", "c", "b")
	if got := e.ShareRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("привязки %v, ждали %v", got, want)
	}
	if got := fileRefs(t, x); len(got) != 0 {
		t.Fatalf("SetRef записал файл сам: %v", got)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, x); !reflect.DeepEqual(got, want) {
		t.Fatalf("в файле %v", got)
	}
	// Дубль — вперёд, без роста.
	if err := e.SetRef(x, uid, model.LinkRef(uid, "c")); err != nil {
		t.Fatal(err)
	}
	if got, want := e.ShareRefs(), refsFor(uid, "c", "e", "d", "b"); !reflect.DeepEqual(got, want) {
		t.Errorf("дубль: %v, ждали %v", got, want)
	}
	// Отказы: негодная привязка, другой отпечаток, другой путь, пустой путь.
	good := model.LinkRef(uid, "z")
	if err := e.SetRef(x, uid, "плохая"); !errors.Is(err, ErrBadRef) {
		t.Errorf("негодная привязка: %v", err)
	}
	for _, c := range []struct{ path, uid string }{
		{x, model.NewUID()}, {filepath.Join(dir, "другой.salbum"), uid}, {"", uid}, {x, ""},
	} {
		if err := e.SetRef(c.path, c.uid, good); !errors.Is(err, ErrUIDChanged) {
			t.Errorf("SetRef(%q, %q): %v, ждали ErrUIDChanged", c.path, c.uid, err)
		}
	}
	if slicesHas(e.ShareRefs(), good) {
		t.Error("отказанная привязка попала в список")
	}
}

func slicesHas(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// KeepRefs (открытие своим путём, 167-24): без сверки пути, по отпечатку; несколько сразу —
// первая остаётся первой; негодные и чужой отпечаток не мешают.
func TestEditorKeepRefs(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, uid := refsEditor(t, x, "old")
	rev := e.rev
	e.KeepRefs(uid, model.LinkRef(uid, "n1"), "плохая", model.LinkRef(uid, "n2"), model.LinkRef(uid, "n1"))
	if want := refsFor(uid, "n1", "n2", "old"); !reflect.DeepEqual(e.ShareRefs(), want) {
		t.Errorf("привязки %v, ждали %v", e.ShareRefs(), want)
	}
	if len(e.undo) != 0 || e.dirty || e.rev != rev {
		t.Errorf("KeepRefs — правка: шагов %d, dirty %v", len(e.undo), e.dirty)
	}
	// Чужой или негодный отпечаток текущий альбом не затрагивает.
	other := model.NewUID()
	e.KeepRefs(other, model.LinkRef(other, "q"))
	e.KeepRefs("", model.LinkRef(uid, "w"))
	e.KeepRefs(uid)
	if want := refsFor(uid, "n1", "n2", "old"); !reflect.DeepEqual(e.ShareRefs(), want) {
		t.Errorf("после чужих вызовов %v", e.ShareRefs())
	}
	// Привязка чужого отпечатка живёт в карте, но в файл не идёт, пока он не текущий.
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	if got := fileRefs(t, x); !reflect.DeepEqual(got, refsFor(uid, "n1", "n2", "old")) {
		t.Errorf("в файле %v", got)
	}
}

// Привязки следуют за отпечатком: Restore копии с другим отпечатком даёт его привязки, Ctrl+Z
// возвращает прежний отпечаток и его привязки, Ctrl+Y — копии. Запись в каждый момент несёт
// привязки текущего отпечатка, а не чужие.
func TestRestoreUndoRefsFollowUID(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, ua := refsEditor(t, x, "a1", "a2")
	y := filepath.Join(dir, "копия", "y.salbum")
	if err := os.MkdirAll(filepath.Dir(y), 0o755); err != nil {
		t.Fatal(err)
	}
	_, ub := refsEditor(t, y, "b1")
	cp, err := LoadBase(y, dir)
	if err != nil {
		t.Fatal(err)
	}
	wantA, wantB := refsFor(ua, "a1", "a2"), refsFor(ub, "b1")
	st := e.p.Settings
	st.Gap += 2 // копия отличается от альбома — Restore не холостой
	e.SetSettings(st)
	if _, err := e.Restore(cp); err != nil {
		t.Fatal(err)
	}
	check := func(step, uid string, want []string) {
		t.Helper()
		if e.p.UID != uid {
			t.Fatalf("%s: отпечаток %q, ждали %q", step, e.p.UID, uid)
		}
		if got := e.ShareRefs(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: привязки %v, ждали %v", step, got, want)
		}
		if err := e.Save(""); err != nil {
			t.Fatal(err)
		}
		if got := fileRefs(t, x); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: в файле %v, ждали %v", step, got, want)
		}
	}
	check("Restore", ub, wantB)
	e.Undo()
	check("Ctrl+Z", ua, wantA)
	e.Redo()
	check("Ctrl+Y", ub, wantB)
	e.Undo()
	check("Ctrl+Z ещё раз", ua, wantA)

	// Restore копии того же отпечатка: привязки копии дополняют известные (первые — свои).
	z := filepath.Join(dir, "копия", "z.salbum")
	p := model.New()
	p.UID = ua
	p.SetShareRefs(refsFor(ua, "a1", "a3"))
	p.Settings.Gap = e.p.Settings.Gap + 5
	uidEditor(t, p, z)
	cp, err = LoadBase(z, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Restore(cp); err != nil {
		t.Fatal(err)
	}
	check("Restore того же отпечатка", ua, refsFor(ua, "a1", "a2", "a3"))
}

// «Сохранить как» другим проектом (SaveOver) выдаёт новый отпечаток без привязок; прежний файл и
// прежний отпечаток привязки сохраняют; не удавшаяся запись ничего не меняет.
func TestSaveAsDropsRefs(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, uid := refsEditor(t, x, "a", "b")
	want := refsFor(uid, "a", "b")

	bad := filepath.Join(x, "n.salbum") // «папка» — обычный файл: запись не удастся
	if err := saveNew(e, bad); err == nil {
		t.Fatal("запись внутрь обычного файла удалась")
	}
	if e.p.UID != uid || !reflect.DeepEqual(e.ShareRefs(), want) {
		t.Fatalf("неудачная запись изменила отпечаток %q или привязки %v", e.p.UID, e.ShareRefs())
	}

	y := filepath.Join(dir, "y.salbum")
	if err := saveNew(e, y); err != nil {
		t.Fatal(err)
	}
	if e.p.UID == uid || len(e.ShareRefs()) != 0 {
		t.Fatalf("новый проект: отпечаток %q, привязки %v", e.p.UID, e.ShareRefs())
	}
	if got := fileRefs(t, y); len(got) != 0 {
		t.Fatalf("в новом файле привязки %v", got)
	}
	if got := fileRefs(t, x); !reflect.DeepEqual(got, want) {
		t.Fatalf("прежний файл потерял привязки: %v", got)
	}
	// Ctrl+Z после «Сохранить как» отпечаток не возвращает (swapUID правит и снимки): привязок нет.
	if err := e.Save(""); err != nil || len(fileRefs(t, y)) != 0 {
		t.Fatalf("повторная запись нового проекта: %v", err)
	}
}

// «Выгрузить как новый» (SaveFresh) выдаёт отпечаток без привязок прежнего; печать открытой копии
// (другого пути) на это не влияет.
func TestIssueNoRefs(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	p := model.New()
	p.UID = model.NewUID()
	p.SetUIDAt(model.UIDTag(p.UID, filepath.Join(dir, "другой.salbum"))) // печать не этого пути
	p.SetShareRefs(refsFor(p.UID, "a"))
	e := uidEditor(t, p, x)
	if got := e.ShareRefs(); len(got) != 1 {
		t.Fatalf("привязки открытой копии %v", got)
	}
	path, uid := e.Ident()
	nu, err := e.SaveFresh(path, uid)
	if err != nil || nu == uid {
		t.Fatalf("SaveFresh: %q %v", nu, err)
	}
	if got := e.ShareRefs(); len(got) != 0 {
		t.Errorf("новый отпечаток получил привязки %v", got)
	}
	if got := fileRefs(t, x); len(got) != 0 {
		t.Errorf("в файле с новым отпечатком привязки %v", got)
	}
	// Привязка нового отпечатка кладётся как обычно.
	if err := e.SetRef(x, nu, model.LinkRef(nu, "n")); err != nil || len(e.ShareRefs()) != 1 {
		t.Errorf("SetRef после выдачи: %v %v", err, e.ShareRefs())
	}
}

// Снимки несут привязки отдельно (Clone их теряет): SnapshotAt для выгрузки, SnapshotSideRefs для
// сбора; ответ — копия; привязки — того отпечатка, что в проекте снимка.
func TestShareJobCarriesLinks(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, uid := refsEditor(t, x, "a", "b")
	want := refsFor(uid, "a", "b")

	path, p, _, refs := e.SnapshotAt()
	if path != x || p.UID != uid || !reflect.DeepEqual(refs, want) {
		t.Fatalf("SnapshotAt: %q %q %v", path, p.UID, refs)
	}
	if len(p.ShareRefs()) != 0 {
		t.Error("Clone унёс привязки — они должны идти только отдельным полем")
	}
	refs[0] = "испорчено"
	if got := e.ShareRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("порча ответа SnapshotAt дошла до Editor: %v", got)
	}

	q, side, refs := e.SnapshotSideRefs()
	if q.UID != uid || !reflect.DeepEqual(refs, want) || side.Marks == nil {
		t.Fatalf("SnapshotSideRefs: %q %v", q.UID, refs)
	}
	refs[1] = "испорчено"
	if got := e.ShareRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("порча ответа SnapshotSideRefs дошла до Editor: %v", got)
	}
	if p2, _ := e.SnapshotSide(); p2.UID != uid {
		t.Error("SnapshotSide")
	}

	// Новый проект («Сохранить как» другим) — в снимке его отпечаток и пустые привязки.
	if err := saveNew(e, filepath.Join(dir, "n.salbum")); err != nil {
		t.Fatal(err)
	}
	if _, p, _, refs := e.SnapshotAt(); p.UID == uid || len(refs) != 0 {
		t.Errorf("после SaveOver снимок: %q %v", p.UID, refs)
	}
	if _, _, refs := e.SnapshotSideRefs(); len(refs) != 0 {
		t.Errorf("после SaveOver снимок сбора: %v", refs)
	}
}

// Редактор нового альбома из шаблона: новый отпечаток, без файла, привязок и печати; вёрстка, отметки
// и настройки шаблона на месте; запись по новому пути не несёт ничего от шаблона.
func TestTemplateEditorFresh(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "шаблон.salbum")
	src, uid := refsEditor(t, x, "a", "b")
	st := src.p.Settings
	st.Gap += 3
	src.SetSettings(st)
	if err := src.Save(""); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(x)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ShareRefs()) != 2 || p.UID != uid {
		t.Fatalf("шаблон: %v %q", p.ShareRefs(), p.UID)
	}
	e, err := NewTemplateEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	if e.Path() != "" || !model.GoodUID(e.p.UID) || e.p.UID == uid {
		t.Fatalf("новый альбом: путь %q, отпечаток %q (шаблона %q)", e.Path(), e.p.UID, uid)
	}
	if len(e.ShareRefs()) != 0 || e.p.UIDAt() != "" {
		t.Errorf("привязки %v, печать %q от шаблона", e.ShareRefs(), e.p.UIDAt())
	}
	if e.p.Settings.Gap != st.Gap {
		t.Error("настройки шаблона потеряны")
	}
	y := filepath.Join(dir, "новый.salbum")
	if err := saveNew(e, y); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(y)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "shareRefs") || strings.Contains(string(data), uid) {
		t.Error("в файле нового альбома остались отпечаток или привязки шаблона")
	}
	// Шаблон на диске цел.
	if got := fileRefs(t, x); len(got) != 2 {
		t.Errorf("шаблон потерял привязки: %v", got)
	}
	// Два альбома из одного шаблона — разные отпечатки.
	p1, _ := salbum.Load(x)
	p2, _ := salbum.Load(x)
	e1, _ := NewTemplateEditor(p1)
	e2, _ := NewTemplateEditor(p2)
	if e1.p.UID == e2.p.UID {
		t.Error("два альбома из шаблона с одним отпечатком")
	}
}
