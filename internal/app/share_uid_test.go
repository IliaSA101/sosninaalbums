package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Отпечаток проекта (146 T4-9) на уровне App: pinShare пишет его в файл альбома до 0.32, а
// «Сохранить как» в новый путь выдаёт новый — запись прежнего проекта становится чужой.

// diskUID — отпечаток, лежащий в файле path (читается мимо редактора).
func diskUID(t *testing.T, path string) string {
	t.Helper()
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return p.UID
}

func linkOf(t *testing.T, a *App) *ShareLink {
	t.Helper()
	v, err := a.ShareState()
	if err != nil {
		t.Fatal(err)
	}
	return v.Link
}

func hasLink(t *testing.T, a *App) bool { return linkOf(t, a) != nil }

func TestPinShareWritesUID(t *testing.T) {
	a, _, _ := backupApp(t, 0)
	file := filepath.Join(testkit.TempDir(t), "Старый"+projectExt)
	if err := salbum.Write(model.New(), file, "", model.Side{}); err != nil { // как файл до 0.32: без uid
		t.Fatal(err)
	}
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, p, file))
	if diskUID(t, file) != "" {
		t.Fatal("в исходном файле уже есть отпечаток")
	}
	path, uid := a.editor().Ident()
	if !model.GoodUID(uid) {
		t.Fatalf("редактор не выдал отпечаток: %q", uid)
	}
	rev := a.editor().View().Rev
	pinned, err := a.pinShare(path, uid, false)
	if err != nil {
		t.Fatal(err)
	}
	if pinned != uid { // печать пути решений не принимает: отпечаток тот же, в файл ложится как есть (167-6)
		t.Fatalf("pinShare: %q (был %q)", pinned, uid)
	}
	if got := diskUID(t, file); got != uid {
		t.Errorf("в файле %q, ждали %q", got, uid)
	}
	if v := a.editor().View(); v.Dirty || v.Rev != rev {
		t.Errorf("запись отпечатка стала правкой: dirty=%v rev %d -> %d", v.Dirty, rev, v.Rev)
	}
	// Запись о ссылке с этим отпечатком действует и после перезапуска (файл теперь несёт uid).
	putProjectRecord(t, a, uid, file)
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Error("своя запись не видна")
	}
	// Второй раз файл не пишется: отпечаток в нём уже есть.
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := a.pinShare(path, uid, false); again != uid || err != nil {
		t.Fatal(again, err)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Error("повторная pinShare переписала файл")
	}
	// Альбом сменился: чужой отпечаток не пишется.
	if _, err := a.pinShare(path, model.NewUID(), false); err == nil {
		t.Error("pinShare принял отпечаток другого проекта")
	}
}

// «Сохранить как» в другой путь — новый проект: новый отпечаток, ссылка прежнего ему не принадлежит.
func TestSaveAsNewProjectNewUID(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	old, err := a.pinShare(file, mustUID(t, a), false)
	if err != nil {
		t.Fatal(err)
	}
	putProjectRecord(t, a, old, file)
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatal("запись альбома не видна до «Сохранить как»")
	}
	// Тот же проект в тот же файл: отпечаток прежний.
	if _, err := a.saveAs(a.editor(), file); err != nil {
		t.Fatal(err)
	}
	if _, uid := a.editor().Ident(); uid != old {
		t.Errorf("«Сохранить как» в тот же файл сменило отпечаток: %q -> %q", old, uid)
	}
	// Другой путь: новый проект — новый отпечаток и в редакторе, и в файле, ссылки нет.
	other := filepath.Join(testkit.TempDir(t), "Копия"+projectExt)
	if _, err := a.saveAs(a.editor(), other); err != nil {
		t.Fatal(err)
	}
	_, fresh := a.editor().Ident()
	if !model.GoodUID(fresh) || fresh == old || diskUID(t, other) != fresh {
		t.Fatalf("отпечаток нового проекта: %q (прежний %q, в файле %q)", fresh, old, diskUID(t, other))
	}
	if hasLink(t, a) {
		t.Error("новый проект видит ссылку прежнего")
	}
}

// pinShare fresh («выгрузить как новый»): новый отпечаток без привязок записан в файл, прежний
// остаётся прежнему.
func TestPinShareFresh(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	old := mustUID(t, a)
	if err := a.editor().SetRef(file, old, model.LinkRef(old, "alb")); err != nil {
		t.Fatal(err)
	}
	nu, err := a.pinShare(file, old, true)
	if err != nil || nu == old || diskUID(t, file) != nu {
		t.Fatalf("fresh: %q %v (был %q, в файле %q)", nu, err, old, diskUID(t, file))
	}
	if refs := a.editor().ShareRefs(); len(refs) != 0 {
		t.Errorf("привязки прежнего ушли к новому: %v", refs)
	}
	if _, err := a.pinShare(file, old, true); err == nil {
		t.Error("fresh по устаревшему отпечатку принят")
	}
}

// Копия шаблона в другом файле — тот же uid, но запись шаблона не её: moved, без секретов (167-12).
// Диск снова прежний — запись своя.
func TestShareLinkSurvivesOtherPath(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	root := testkit.TempDir(t)
	e := filepath.Join(root, "E", "Иванов"+projectExt)
	if err := os.MkdirAll(filepath.Dir(e), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.editor().SaveOver(e); err != nil {
		t.Fatal(err)
	}
	_, uid := a.editor().Ident()
	putProjectRecord(t, a, uid, e)
	f := filepath.Join(root, "F", "Иванов"+projectExt)
	unplug := func() { // диск отключён: открыт другой альбом
		t.Helper()
		a.ed.Store(mustEditor(t, model.New(), ""))
	}
	unplug()
	if err := os.Rename(filepath.Dir(e), filepath.Dir(f)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.openPath(f); err != nil {
		t.Fatal(err)
	}
	a.saveMu.Lock()
	a.save()
	a.saveMu.Unlock()
	addSpread(t, a)
	a.saveMu.Lock()
	a.save()
	a.saveMu.Unlock()
	if got := diskUID(t, f); got != uid {
		t.Fatalf("запись по другому пути сменила отпечаток: %q", got)
	}
	if l := linkOf(t, a); l == nil || l.Own || l.Password != "" || l.Title != "" {
		t.Fatalf("на F: %+v", l)
	}
	unplug()
	if err := os.Rename(filepath.Dir(f), filepath.Dir(e)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.openPath(e); err != nil {
		t.Fatal(err)
	}
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("диск снова E: — запись чужая: %+v", l)
	}
}

// Сценарий аудита на уровне App в модели по uid (167): шаблон копируют каждому клиенту в одну папку X.
// Копия шаблона в X несёт uid шаблона, а запись шаблона привязана к его файлу, — для копии это
// moved: без пароля и названия, чужой ссылкой она не владеет. Иванов выгружает «как новый» — новый
// uid, своя запись. Петров с копией шаблона в том же X ссылку Иванова не видит (и после записи без
// правок); при восстановлении копий X и Ctrl+Z / Ctrl+Y видимость ссылки следует за uid.
func TestTemplateCopySamePathForeign(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	dir := testkit.TempDir(t)
	tpl := filepath.Join(dir, "Шаблон"+projectExt)
	if _, err := a.editor().SaveOver(tpl); err != nil { // шаблон — «Сохранить как»
		t.Fatal(err)
	}
	tplUID := mustUID(t, a)
	putProjectRecord(t, a, tplUID, tpl)
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("запись шаблона по его пути не своя: %+v", l)
	}
	x := filepath.Join(dir, "Клиент"+projectExt)
	copyTo := func() {
		t.Helper()
		data, err := os.ReadFile(tpl)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(x, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// notMine — ссылка либо не видна, либо чужая (шаблон жив по своему пути — other, иначе moved):
	// не своя, без пароля и названия, с подписью, откуда она.
	notMine := func(who string) {
		t.Helper()
		if l := linkOf(t, a); l != nil && (l.Own || l.Password != "" || l.Title != "" || l.From == "" ||
			(l.State != "other" && l.State != "moved")) {
			t.Fatalf("%s: копия шаблона владеет чужой записью: %+v", who, l)
		}
	}
	copyTo()
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	if uid := mustUID(t, a); uid != tplUID {
		t.Fatalf("Иванов: открытие сменило отпечаток копии шаблона: %q", uid)
	}
	notMine("Иванов")

	ivan, err := a.pinShare(x, tplUID, true) // «выгрузить как новый»
	if err != nil {
		t.Fatal(err)
	}
	if ivan == tplUID || !model.GoodUID(ivan) || diskUID(t, x) != ivan {
		t.Fatalf("выгрузка Иванова: отпечаток %q, в файле %q", ivan, diskUID(t, x))
	}
	putProjectRecord(t, a, ivan, x)
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("запись Иванова не своя: %+v", l)
	}
	if !hasShareRecord(a, tplUID) {
		t.Fatal("запись шаблона пропала")
	}
	restoreXCopies(t, a, x, ivan, tplUID, notMine)

	copyTo() // архив Иванова унесли, Петрову — снова шаблон
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	if uid := mustUID(t, a); uid != tplUID {
		t.Fatalf("Петров: открытие сменило отпечаток копии шаблона: %q", uid)
	}
	notMine("Петров")
	a.saveMu.Lock()
	a.save() // смена альбома или выход: запись без правок
	a.saveMu.Unlock()
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	notMine("Петров после записи")
	petr, err := a.pinShare(x, tplUID, true)
	if err != nil || petr == ivan || petr == tplUID || diskUID(t, x) != petr {
		t.Fatalf("выгрузка Петрова: %q, %v", petr, err)
	}
	putProjectRecord(t, a, petr, x)
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("запись Петрова не своя: %+v", l)
	}
	if !hasShareRecord(a, ivan) {
		t.Error("запись Иванова стёрта выгрузкой Петрова")
	}
}

// restoreXCopies — восстановление копий из папки копий X: копия Иванова — запись своя (и после
// Ctrl+Z / Ctrl+Y), копия шаблона (снята pinShare перед записью нового uid) — не своя.
func restoreXCopies(t *testing.T, a *App, x, ivan, tplUID string, notMine func(string)) {
	t.Helper()
	if err := a.backups.Force(x, backupKey(x)); err != nil { // копия файла Иванова (байты X)
		t.Fatal(err)
	}
	addSpread(t, a)
	var mine, tplCopy string
	for _, b := range backups(t, a) {
		file, err := a.backups.File(backupKey(x), b.Name)
		if err != nil {
			t.Fatal(err)
		}
		switch diskUID(t, file) {
		case ivan:
			mine = b.Name
		case tplUID:
			tplCopy = b.Name
		}
	}
	if mine == "" || tplCopy == "" {
		t.Fatalf("копии X: своя %q, шаблона %q", mine, tplCopy)
	}
	if _, err := a.RestoreBackup(tplCopy); err != nil {
		t.Fatal(err)
	}
	if uid := mustUID(t, a); uid != tplUID {
		t.Fatalf("копия шаблона из папки копий X: отпечаток %q", uid)
	}
	notMine("копия шаблона")
	if _, err := a.RestoreBackup(mine); err != nil {
		t.Fatal(err)
	}
	if uid := mustUID(t, a); uid != ivan {
		t.Fatalf("копия X сменила отпечаток Иванова: %q", uid)
	}
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("копия Иванова: запись не своя: %+v", l)
	}
	a.Undo()
	if uid := mustUID(t, a); uid != tplUID {
		t.Fatalf("Ctrl+Z: отпечаток %q, ждали шаблона", uid)
	}
	notMine("Ctrl+Z к копии шаблона")
	a.Redo()
	if l := linkOf(t, a); l == nil || !l.Own {
		t.Fatalf("Ctrl+Y к копии Иванова: запись не своя: %+v", l)
	}
}
