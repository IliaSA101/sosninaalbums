package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
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

// putUIDRecord кладёт запись о ссылке пути album с отпечатком uid.
func putUIDRecord(t *testing.T, a *App, album, uid string) {
	t.Helper()
	rec := prefs.ShareRecord{Origin: "https://albums.example.org", ID: "LiZ9u4CAHMYsK8nxXrvchR", Title: "Альбом",
		URL: "https://albums.example.org/a/LiZ9u4CAHMYsK8nxXrvchR/", ExpiresAt: time.Now().Add(24 * time.Hour), UID: uid}
	if err := shareStore(a).Put(album, rec); err != nil {
		t.Fatal(err)
	}
}

func hasLink(t *testing.T, a *App) bool {
	t.Helper()
	v, err := a.ShareState()
	if err != nil {
		t.Fatal(err)
	}
	return v.Link != nil
}

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
	pinned, err := a.pinShare(path, uid)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == uid || !model.GoodUID(pinned) { // выдан при открытии без печати пути — новый с печатью
		t.Fatalf("pinShare: %q (был %q)", pinned, uid)
	}
	uid = pinned
	if got := diskUID(t, file); got != uid {
		t.Errorf("в файле %q, ждали %q", got, uid)
	}
	if v := a.editor().View(); v.Dirty || v.Rev != rev {
		t.Errorf("запись отпечатка стала правкой: dirty=%v rev %d -> %d", v.Dirty, rev, v.Rev)
	}
	// Запись о ссылке с этим отпечатком действует и после перезапуска (файл теперь несёт uid).
	putUIDRecord(t, a, file, uid)
	if !hasLink(t, a) {
		t.Error("запись с отпечатком альбома не видна")
	}
	// Второй раз файл не пишется: отпечаток в нём уже есть.
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := a.pinShare(path, uid); again != uid || err != nil {
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
	if _, err := a.pinShare(path, model.NewUID()); err == nil {
		t.Error("pinShare принял отпечаток другого проекта")
	}
}

func TestSaveAsNewProjectNewUID(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	_, old := a.editor().Ident()
	old, err := a.pinShare(file, old)
	if err != nil {
		t.Fatal(err)
	}
	putUIDRecord(t, a, file, old)
	if !hasLink(t, a) {
		t.Fatal("запись альбома не видна до «Сохранить как»")
	}
	// Тот же проект в тот же файл: отпечаток прежний.
	if _, err := a.saveAs(a.editor(), file); err != nil {
		t.Fatal(err)
	}
	if _, uid := a.editor().Ident(); uid != old {
		t.Errorf("«Сохранить как» в тот же файл сменило отпечаток: %q -> %q", old, uid)
	}
	// Другой путь: новый проект — новый отпечаток и в редакторе, и в файле.
	other := filepath.Join(testkit.TempDir(t), "Копия"+projectExt)
	if _, err := a.saveAs(a.editor(), other); err != nil {
		t.Fatal(err)
	}
	_, fresh := a.editor().Ident()
	if !model.GoodUID(fresh) || fresh == old {
		t.Fatalf("отпечаток нового проекта: %q (прежний %q)", fresh, old)
	}
	if got := diskUID(t, other); got != fresh {
		t.Errorf("в новом файле %q, ждали %q", got, fresh)
	}
	// Запись прежнего проекта (стирание не удалось или она вернулась из копии shares.json) теперь чужая.
	putUIDRecord(t, a, other, old)
	if hasLink(t, a) {
		t.Error("запись с отпечатком прежнего проекта действует для нового")
	}
	putUIDRecord(t, a, other, fresh)
	if !hasLink(t, a) {
		t.Error("запись с отпечатком нового проекта не видна")
	}
}

// Сценарий аудита на уровне App: шаблон копируют каждому клиенту в одну папку X. Копия шаблона
// открыта как есть (отпечаток шаблона), но запись X ей чужая и в окне, и в снимке выгрузки;
// выгрузка Иванова — новый отпечаток с печатью X. Запись Иванова для копии шаблона Петрова в том
// же X — чужая, и после записи без правок. Восстановление копии из папки копий X: копия Иванова —
// запись своя (и после Ctrl+Z / Ctrl+Y), копия шаблона — чужая.
func TestTemplateCopySamePathForeign(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	dir := testkit.TempDir(t)
	tpl := filepath.Join(dir, "Шаблон"+projectExt)
	if err := a.editor().SaveNew(tpl); err != nil { // шаблон — «Сохранить как»
		t.Fatal(err)
	}
	_, tplUID := a.editor().Ident()
	putUIDRecord(t, a, tpl, tplUID)
	if !hasLink(t, a) {
		t.Fatal("запись шаблона по его пути не видна")
	}
	x := filepath.Join(dir, "Клиент"+projectExt)
	copyTo := func() {
		data, err := os.ReadFile(tpl)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(x, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	foreign := func(who string) {
		t.Helper()
		if _, uid := a.editor().Ident(); uid != tplUID {
			t.Fatalf("%s: открытие сменило отпечаток копии шаблона: %q", who, uid)
		}
		if _, own := a.editor().ShareIdent(); own != "" || !a.shareJob().Unsealed {
			t.Fatalf("%s: запись X для копии шаблона своя: %q", who, own)
		}
	}
	copyTo()
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	foreign("Иванов")
	putUIDRecord(t, a, x, tplUID) // запись с отпечатком шаблона по пути X — тоже не своя
	if hasLink(t, a) {
		t.Fatal("копия шаблона видит запись X с отпечатком шаблона")
	}
	ivan, err := a.pinShare(x, tplUID)
	if err != nil {
		t.Fatal(err)
	}
	if ivan == tplUID || diskUID(t, x) != ivan || a.shareJob().Unsealed {
		t.Fatalf("выгрузка Иванова: отпечаток %q, в файле %q", ivan, diskUID(t, x))
	}
	putUIDRecord(t, a, x, ivan)
	if !hasLink(t, a) {
		t.Fatal("запись Иванова не видна")
	}
	restoreIvanCopies(t, a, x, ivan, tplUID)
	copyTo() // архив Иванова унесли, Петрову — снова шаблон
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	foreign("Петров")
	if hasLink(t, a) {
		t.Fatal("Петров видит ссылку Иванова")
	}
	a.saveMu.Lock()
	a.save() // смена альбома или выход: запись без правок
	a.saveMu.Unlock()
	if _, err := a.openPath(x); err != nil {
		t.Fatal(err)
	}
	foreign("Петров после записи")
	if hasLink(t, a) {
		t.Fatal("Петров после записи без правок видит ссылку Иванова")
	}
	petr, err := a.pinShare(x, tplUID)
	if err != nil || petr == ivan || petr == tplUID || diskUID(t, x) != petr {
		t.Fatalf("выгрузка Петрова: %q, %v", petr, err)
	}
}

// restoreIvanCopies — восстановление копий из папки копий X: копия Иванова — запись своя (и после
// Ctrl+Z / Ctrl+Y), копия шаблона (снята pinShare перед записью отпечатка) — чужая.
func restoreIvanCopies(t *testing.T, a *App, x, ivan, tplUID string) {
	t.Helper()
	if err := a.backups.Force(x, backupKey(x)); err != nil { // копия файла Иванова (байты X)
		t.Fatal(err)
	}
	addSpread(t, a)
	// В папке копий X — копия шаблона (pinShare перед записью отпечатка) и копия Иванова.
	var mine, tplCopy string
	for _, b := range backups(t, a) {
		file, err := a.backups.File(backupKey(x), b.Name)
		if err != nil {
			t.Fatal(err)
		}
		if diskUID(t, file) == ivan {
			mine = b.Name
		} else {
			tplCopy = b.Name
		}
	}
	if mine == "" || tplCopy == "" {
		t.Fatalf("копии X: своя %q, шаблона %q", mine, tplCopy)
	}
	if _, err := a.RestoreBackup(tplCopy); err != nil {
		t.Fatal(err)
	}
	if _, uid := a.editor().Ident(); uid != tplUID || hasLink(t, a) {
		t.Fatalf("копия шаблона из папки копий X: отпечаток %q, ссылка Иванова видна %v", uid, hasLink(t, a))
	}
	if _, err := a.RestoreBackup(mine); err != nil {
		t.Fatal(err)
	}
	if _, uid := a.editor().Ident(); uid != ivan || !hasLink(t, a) {
		t.Fatalf("копия X сменила отпечаток Иванова: %q", uid)
	}
	a.Undo()
	if hasLink(t, a) {
		t.Fatal("Ctrl+Z к копии шаблона: ссылка Иванова видна")
	}
	a.Redo()
	if !hasLink(t, a) {
		t.Fatal("Ctrl+Y к копии Иванова: ссылка не видна")
	}
}

// Регрессия аудита на уровне App: альбом на USB-диске с записью о ссылке; диск получил другую
// букву — альбом открыли там, записали без правок и с правкой. Диск снова прежний — ссылка видна.
func TestShareLinkSurvivesOtherPath(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	root := testkit.TempDir(t)
	e := filepath.Join(root, "E", "Иванов"+projectExt)
	if err := os.MkdirAll(filepath.Dir(e), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.editor().SaveNew(e); err != nil {
		t.Fatal(err)
	}
	_, uid := a.editor().Ident()
	putUIDRecord(t, a, e, uid)
	f := filepath.Join(root, "F", "Иванов"+projectExt)
	unplug := func() { // диск отключён: открыт другой альбом, запись при смене не идёт по старому пути
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
	a.save() // без правок
	a.saveMu.Unlock()
	addSpread(t, a)
	a.saveMu.Lock()
	a.save() // автосохранение правки
	a.saveMu.Unlock()
	if got := diskUID(t, f); got != uid {
		t.Fatalf("запись по другому пути сменила отпечаток: %q", got)
	}
	if _, own := a.editor().ShareIdent(); own != "" || !a.shareJob().Unsealed {
		t.Fatalf("на F: запись пути своя: %q", own)
	}
	unplug()
	if err := os.Rename(filepath.Dir(f), filepath.Dir(e)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.openPath(e); err != nil {
		t.Fatal(err)
	}
	if _, own := a.editor().ShareIdent(); own != uid || !hasLink(t, a) {
		t.Fatalf("диск снова E: — запись чужая (%q), ссылка видна %v", own, hasLink(t, a))
	}
}
