package spreads_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Подпапка уже добавленной — холостая (ни шага, ни rev), Within — корень; несколько папок сразу.
// Подпапка в другом регистре — та же на Windows и macOS, на Linux — новая папка (paths.Fold, 101).
func TestAddFolderWithin(t *testing.T) {
	root := testkit.TempDir(t)
	spring, banket := filepath.Join(root, "Весна 2026"), filepath.Join(root, "Банкет")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, spring)
	edtest.AddFolder(e, banket)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	upper := strings.ToUpper(filepath.Join(banket, "Торт"))
	subs := map[string]string{
		filepath.Join(spring, "Март", "Избранное"): spring,
		filepath.Join(banket, "Торт"):              banket,
	}
	fold := paths.FoldEqual(upper, filepath.Join(banket, "Торт"))
	if fold {
		subs[upper] = banket
	}
	for sub, want := range subs {
		v, r, err := e.AddFolder(sub)
		if err != nil || r != (model.FolderAdd{Within: want}) || len(v.Folders) != 2 {
			t.Fatalf("%s: %+v %v, папки %q", sub, r, err, v.Folders)
		}
		edtest.Same(t, e, rev, undo, "подпапка")
	}
	if fold {
		return
	}
	if v, r, err := e.AddFolder(upper); err != nil || r != (model.FolderAdd{}) || !slices.Equal(v.Folders, []string{spring, banket, upper}) {
		t.Fatalf("другой регистр на Linux: %+v %v, папки %q", r, err, v.Folders)
	}
}

// Та же папка (на Windows и macOS — и в другом регистре) — холостая, Within — она сама;
// на Linux другой регистр — новая папка (paths.Fold, 101).
func TestAddFolderSame(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "Весна")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, dir)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	upper := strings.ToUpper(dir)
	same := []string{dir, dir + string(filepath.Separator)}
	fold := paths.FoldEqual(upper, dir)
	if fold {
		same = append(same, upper)
	}
	for _, d := range same {
		if _, r, err := e.AddFolder(d); err != nil || r != (model.FolderAdd{Within: dir}) {
			t.Fatalf("%q: %+v %v", d, r, err)
		}
	}
	edtest.Same(t, e, rev, undo, "та же папка")
	if fold {
		return
	}
	if v, r, err := e.AddFolder(upper); err != nil || r != (model.FolderAdd{}) || !slices.Equal(v.Folders, []string{dir, upper}) {
		t.Fatalf("другой регистр на Linux: %+v %v, папки %q", r, err, v.Folders)
	}
}

// Родитель над добавленными встаёт на место первой вошедшей; Merged — сколько вошло; чужие
// папки на местах; одним шагом — Undo возвращает прежний список.
func TestAddFolderParentMerges(t *testing.T) {
	root := testkit.TempDir(t)
	spring := filepath.Join(root, "Весна 2026")
	march, april, other := filepath.Join(spring, "Март"), filepath.Join(spring, "Апрель", "Отбор"), filepath.Join(root, "Банкет")
	e := edtest.Open(t, model.New(), "")
	for _, d := range []string{other, march, april} {
		edtest.AddFolder(e, d)
	}
	edtest.Change(e, func(p *model.Project) { p.FolderAlt = map[string]string{march: filepath.Join(root, "E", "Март")} })
	undo := edtest.UndoLen(e)
	v, r, err := e.AddFolder(spring)
	if err != nil || r != (model.FolderAdd{Merged: 2}) || !slices.Equal(v.Folders, []string{other, spring}) {
		t.Fatalf("%+v %v, папки %q", r, err, v.Folders)
	}
	if edtest.P(e).FolderAlt != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("запасной путь вошедшей: %v, шагов %d", edtest.P(e).FolderAlt, edtest.UndoLen(e)-undo)
	}
	if v := e.Undo(); !slices.Equal(v.Folders, []string{other, march, april}) {
		t.Fatalf("Undo: %q", v.Folders)
	}
}

// Папка поверх отдельных фото забирает их (LooseIn) тем же шагом, пустое добавление уходит;
// Ctrl+Z возвращает и папку, и отдельные.
func TestAddFolderTakesLoose(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Телефон")
	in1, in2, out := filepath.Join(dir, "1.jpg"), filepath.Join(dir, "Сад", "2.jpg"), filepath.Join(root, "Загрузки", "3.jpg")
	e := edtest.Open(t, model.New(), "")
	if _, r, err := e.AddLoose([]string{in1, out}, 10); err != nil || r.Added != 2 {
		t.Fatalf("первое: %+v %v", r, err)
	}
	if _, r, err := e.AddLoose([]string{in2}, 20); err != nil || r.Added != 1 {
		t.Fatalf("второе: %+v %v", r, err)
	}
	before := e.View().Loose
	undo := edtest.UndoLen(e)
	v, r, err := e.AddFolder(dir)
	if err != nil || r != (model.FolderAdd{LooseIn: 2}) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("%+v %v", r, err)
	}
	if len(v.Loose) != 1 || len(v.Loose[0].Files) != 1 || v.Loose[0].Files[0].Path != out {
		t.Fatalf("отдельные после папки: %+v", v.Loose)
	}
	if err := edtest.SourcesInvariant(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	if v := e.Undo(); len(v.Folders) != 0 || !slices.EqualFunc(v.Loose, before, func(a, b model.LooseView) bool { return fmt.Sprint(a) == fmt.Sprint(b) }) {
		t.Fatalf("Undo: папки %q, отдельные %+v", v.Folders, v.Loose)
	}
}

// Одно добавление: ID 12 hex, время, файлы по порядку выбора (пути Clean), в виде — ID ленты.
func TestAddLooseBatch(t *testing.T) {
	root := testkit.TempDir(t)
	a, b, c := filepath.Join(root, "Телефон", "b.jpg"), filepath.Join(root, "Загрузки", "a.png"), filepath.Join(root, "x", "c.JPEG")
	e := edtest.Open(t, model.New(), "")
	v, r, err := e.AddLoose([]string{a, b + string(filepath.Separator), filepath.Join(root, "x", ".", "c.JPEG")}, 1792501500)
	if err != nil || r.Added != 3 || !model.ValidID(r.Batch) || r.InFolder+r.Dup+r.Over != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if len(v.Loose) != 1 || v.Loose[0].ID != r.Batch || v.Loose[0].At != 1792501500 {
		t.Fatalf("вид: %+v", v.Loose)
	}
	for i, want := range []string{a, b, c} {
		if f := v.Loose[0].Files[i]; f.Path != want || f.Photo == "" {
			t.Fatalf("файл %d: %+v, ждали %s", i, f, want)
		}
	}
	// Второе добавление — в конец, отрицательное время — 0.
	v, r2, err := e.AddLoose([]string{filepath.Join(root, "d.jpg")}, -5)
	if err != nil || len(v.Loose) != 2 || v.Loose[1].ID != r2.Batch || v.Loose[1].At != 0 || r2.Batch == r.Batch {
		t.Fatalf("второе: %+v %v %+v", r2, err, v.Loose)
	}
}

// Пропуски: в папке альбома (Folder — папка первого такого), уже отдельное или повтор в списке
// (регистр), сверх предела; всё пропущено — холостая; пустой список — холостая; UNC — ошибка.
func TestAddLooseSkips(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Март 2026")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, dir)
	old := filepath.Join(root, "Телефон", "1.jpg")
	if _, r, err := e.AddLoose([]string{old}, 1); err != nil || r.Added != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	edtest.SetDirty(e, false) // сохранили
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	in := filepath.Join(dir, "Избранное", "2.jpg")
	// Повтор другим регистром — на Windows и macOS; на Linux это другой файл (проверка ниже).
	dup, upper := old, strings.ToUpper(old)
	if paths.FoldEqual(upper, old) {
		dup = upper
	}
	_, r, err := e.AddLoose([]string{in, dup, filepath.Join(dir, "3.jpg")}, 2)
	if err != nil || r != (model.LooseAdded{InFolder: 2, Folder: filepath.Dir(in), Dup: 1}) {
		t.Fatalf("всё пропущено: %+v %v", r, err)
	}
	edtest.Same(t, e, rev, undo, "всё пропущено")
	if edtest.Dirty(e) {
		t.Fatal("холостая правка сделала альбом изменённым")
	}
	if _, r, err := e.AddLoose(nil, 3); err != nil || r != (model.LooseAdded{}) {
		t.Fatalf("пустой список: %+v %v", r, err)
	}
	edtest.Same(t, e, rev, undo, "пустой список")
	newf := filepath.Join(root, "Телефон", "4.jpg")
	v, r, err := e.AddLoose([]string{newf, newf, old}, 4)
	if err != nil || r.Added != 1 || r.Dup != 2 || len(v.Loose) != 2 {
		t.Fatalf("повтор в списке: %+v %v", r, err)
	}
	rev = edtest.Rev(e)
	if v, _, err := e.AddLoose([]string{`\\srv\share\5.jpg`, filepath.Join(root, "5.jpg")}, 5); err == nil || v.Rev != rev {
		t.Fatalf("UNC: %v, rev %d → %d", err, rev, v.Rev)
	}
	if dup == old {
		if v, r, err := e.AddLoose([]string{upper}, 6); err != nil || r.Added != 1 || r.Dup != 0 || len(v.Loose) != 3 {
			t.Fatalf("другой регистр на Linux: %+v %v", r, err)
		}
	}
}

// Пределы: файлов сверх MaxLooseFiles — Over (добавляется сколько влезло), добавлений сверх
// MaxLooseAdds — ошибка без правки.
func TestAddLooseLimit(t *testing.T) {
	root := testkit.TempDir(t)
	e := edtest.Open(t, model.New(), "")
	list := make([]string, model.MaxLooseFiles+3)
	for i := range list {
		list[i] = filepath.Join(root, fmt.Sprintf("%d.jpg", i))
	}
	if _, r, err := e.AddLoose(list[:model.MaxLooseFiles-1], 1); err != nil || r.Added != model.MaxLooseFiles-1 {
		t.Fatalf("почти предел: %+v %v", r, err)
	}
	if _, r, err := e.AddLoose(list[model.MaxLooseFiles-1:], 2); err != nil || r.Added != 1 || r.Over != 3 {
		t.Fatalf("сверх предела: %+v %v", r, err)
	}
	f := edtest.Open(t, model.New(), "")
	edtest.Change(f, func(p *model.Project) {
		for i := range model.MaxLooseAdds {
			p.Loose = append(p.Loose, model.LooseAdd{ID: fmt.Sprintf("%012x", i), Files: []model.LooseFile{{Path: list[i]}}})
		}
	})
	rev := edtest.Rev(f)
	if v, _, err := f.AddLoose(list[model.MaxLooseAdds:model.MaxLooseAdds+1], 3); err == nil || v.Rev != rev || len(v.Loose) != model.MaxLooseAdds {
		t.Fatalf("добавлений сверх предела: %v", err)
	}
	// На пределе добавлений повтор — не ошибка, а холостая со счётом.
	if _, r, err := f.AddLoose(list[:1], 4); err != nil || r.Dup != 1 {
		t.Fatalf("повтор на пределе: %+v %v", r, err)
	}
}

// Убрать файл, добавление, последний файл добавления; неизвестное — холостая.
func TestRemoveLoose(t *testing.T) {
	root := testkit.TempDir(t)
	a, b, c := filepath.Join(root, "a.jpg"), filepath.Join(root, "b.jpg"), filepath.Join(root, "c.jpg")
	e := edtest.Open(t, model.New(), "")
	_, r1, _ := e.AddLoose([]string{a, b}, 1)
	_, r2, _ := e.AddLoose([]string{c}, 2)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.RemoveLoose("000000000000", "")
	e.RemoveLoose(r1.Batch, filepath.Join(root, "нет.jpg"))
	// Другой регистр — тот же файл на Windows и macOS; на Linux — неизвестный (paths.Fold, 101).
	ua := strings.ToUpper(a)
	if !paths.FoldEqual(ua, a) {
		e.RemoveLoose(r1.Batch, ua)
		ua = a
	}
	edtest.Same(t, e, rev, undo, "неизвестное")
	v := e.RemoveLoose(r1.Batch, ua)
	if len(v.Loose) != 2 || len(v.Loose[0].Files) != 1 || v.Loose[0].Files[0].Path != b {
		t.Fatalf("файл: %+v", v.Loose)
	}
	if v = e.RemoveLoose(r1.Batch, b); len(v.Loose) != 1 || v.Loose[0].ID != r2.Batch {
		t.Fatalf("последний файл: %+v", v.Loose)
	}
	if v = e.RemoveLoose(r2.Batch, ""); len(v.Loose) != 0 || v.Loose == nil || edtest.P(e).Loose != nil {
		t.Fatalf("добавление целиком: %+v", v.Loose)
	}
	if edtest.UndoLen(e) != undo+3 {
		t.Fatalf("шагов %d, ждали 3", edtest.UndoLen(e)-undo)
	}
	if v = e.Undo(); len(v.Loose) != 1 || v.Loose[0].ID != r2.Batch {
		t.Fatalf("Undo: %+v", v.Loose)
	}
}

// Отвязка корня убирает и вложенные записи (проект не из Load) одним шагом, с запасными путями.
func TestRemoveFolderNested(t *testing.T) {
	root := testkit.TempDir(t)
	spring, march, other := filepath.Join(root, "Весна"), filepath.Join(root, "Весна", "Март"), filepath.Join(root, "Весна копия")
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		p.Folders = []string{spring, march, other}
		p.FolderAlt = map[string]string{march: filepath.Join(root, "E"), other: filepath.Join(root, "F")}
	})
	undo := edtest.UndoLen(e)
	v := e.RemoveFolder(spring)
	if !slices.Equal(v.Folders, []string{other}) || edtest.UndoLen(e) != undo+1 || len(edtest.P(e).FolderAlt) != 1 {
		t.Fatalf("папки %q, запасные %v", v.Folders, edtest.P(e).FolderAlt)
	}
	if v = e.RemoveFolder(other); v.Folders == nil || len(v.Folders) != 0 || edtest.P(e).FolderAlt != nil {
		t.Fatalf("последняя: %q %v", v.Folders, edtest.P(e).FolderAlt)
	}
}

// Тело AddLoose мимо обёртки: нелокальный путь — ошибка, проект не трогается.
func TestAddLooseBodyChecksPath(t *testing.T) {
	p := model.New()
	for _, f := range []string{`\\server\share\1.jpg`, `\\?\C:\1.jpg`, "1.jpg", ""} {
		if _, err := spreads.AddLoose(p, []string{f}, 1); err == nil || p.Loose != nil {
			t.Errorf("%q: %v, %+v", f, err, p.Loose)
		}
	}
}
