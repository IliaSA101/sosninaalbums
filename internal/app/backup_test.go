package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// backupApp — App с хранилищем копий во временной папке и альбомом file из n пустых
// разворотов (сохранён, открыт). notices — тосты.
func backupApp(t *testing.T, n int) (a *App, file string, notices func() []string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	var mu sync.Mutex
	var got []string
	a.emit = func(name string, data ...any) {
		if name == "notice" {
			mu.Lock()
			got = append(got, data[0].(string))
			mu.Unlock()
		}
	}
	file = filepath.Join(testkit.TempDir(t), "Ивановы"+projectExt)
	ed := mustEditor(t, model.New(), "")
	for range n {
		if _, err := ed.NewSpread(-1, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.Save(file); err != nil {
		t.Fatal(err)
	}
	a.ed.Store(ed)
	return a, file, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}

func addSpread(t *testing.T, a *App) {
	t.Helper()
	if _, err := a.editor().NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
}

func backups(t *testing.T, a *App) []BackupInfo {
	t.Helper()
	list, err := a.Backups()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// Сохранение с правкой — копия прежнего файла; «Восстановить» — один шаг, Ctrl+Z возвращает.
func TestRestoreBackupOneUndo(t *testing.T) {
	a, file, _ := backupApp(t, 2)
	addSpread(t, a)
	a.flushSave() // перезапись: в копию уходит файл с двумя разворотами
	list := backups(t, a)
	if len(list) != 1 || list[0].Spreads != 2 {
		t.Fatalf("копии: %+v", list)
	}
	rev := a.editor().View().Rev
	v, err := a.RestoreBackup(list[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 2 || v.Rev != rev+1 || v.Path != file {
		t.Fatalf("после восстановления: %d разворотов, rev %d (было %d), путь %q", len(v.Spreads), v.Rev, rev, v.Path)
	}
	if v = a.Undo(); len(v.Spreads) != 3 {
		t.Fatalf("отмена: %d разворотов, ждали 3", len(v.Spreads))
	}
	// Текущий файл перед заменой ушёл в копию.
	if list := backups(t, a); len(list) != 2 || list[0].Spreads != 3 {
		t.Fatalf("копия перед восстановлением: %+v", list)
	}
}

// Имя не из списка — «копия не найдена», альбом не меняется.
func TestRestoreBackupBadName(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	if err := a.backups.Force(file, backup.Key(file)); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(file), "20261009-153000"+projectExt)
	if err := os.WriteFile(outside, []byte(`{"version":4,"spreads":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	for _, name := range []string{`..\x`, `C:\x`, "", "x.salbum", outside, `..\..\20261009-153000.salbum`, "20000101-000000.salbum"} {
		if _, err := a.RestoreBackup(name); !errors.Is(err, backup.ErrNotFound) {
			t.Errorf("%q: %v", name, err)
		}
	}
	if a.editor().View().Rev != rev {
		t.Fatal("альбом изменился")
	}
	var nilApp = newApp(t) // без хранилища копий
	if _, err := nilApp.RestoreBackup("20261009-153000.salbum"); err == nil {
		t.Error("без хранилища — не ошибка")
	}
}

// Альбом переехал вместе с фото: относительный путь в копии считается от папки альбома,
// а не от папки копий, и находит фото на новом месте.
func TestRestoreBackupRelativePaths(t *testing.T) {
	root := testkit.TempDir(t)
	oldDir, newDir := filepath.Join(root, "Было"), filepath.Join(root, "Стало")
	photo := testkit.JPEG(t, filepath.Join(oldDir, "Фото"), "a.jpg", 40, 30)
	oldFile, newFile := filepath.Join(oldDir, "альбом"+projectExt), filepath.Join(newDir, "альбом"+projectExt)
	ed := mustEditor(t, model.New(), "")
	if _, err := ed.NewSpread(-1, []model.PhotoInfo{{ID: library.IDFor(photo), Path: photo, W: 40, H: 30}}); err != nil {
		t.Fatal(err)
	}
	if err := ed.Save(oldFile); err != nil {
		t.Fatal(err)
	}
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	if err := a.backups.Force(oldFile, backup.Key(newFile)); err != nil { // копия под ключом нового места
		t.Fatal(err)
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(newFile)
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, p, newFile))
	a.editor().DeleteSpread(a.editor().View().Spreads[0].ID) // чтобы восстановление не было холостым

	list := backups(t, a)
	if len(list) != 1 {
		t.Fatalf("копии: %+v", list)
	}
	if _, err := a.RestoreBackup(list[0].Name); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(newDir, "Фото", "a.jpg")
	for _, r := range a.editor().Refs() {
		if r.Path != want {
			t.Fatalf("путь фото %q, ждали %q", r.Path, want)
		}
	}
	if len(a.editor().Refs()) != 1 {
		t.Fatal("фото не восстановлено")
	}
}

// Копия, открытая через «Открыть», — несохранённый альбом: путь пуст, LastProject и «Последние»
// не меняются, тост о «Сохранить как». Прежний черновик ушёл в копию.
func TestOpenFromBackupsIsUnsaved(t *testing.T) {
	a, file, notices := backupApp(t, 2)
	if err := a.backups.Force(file, backup.Key(file)); err != nil {
		t.Fatal(err)
	}
	copyPath, err := a.backups.File(backup.Key(file), backups(t, a)[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	// Перед открытием — черновик с разворотом.
	draft := mustEditor(t, model.New(), "")
	if _, err := draft.NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	if err := draft.WriteDraft(a.draftPath()); err != nil {
		t.Fatal(err)
	}
	st, err := a.openPath(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if st.View.Path != "" || len(st.View.Spreads) != 2 {
		t.Fatalf("путь %q, разворотов %d", st.View.Path, len(st.View.Spreads))
	}
	if c := a.loadConfig(); c.LastProject != "" || len(c.Recent) != 0 {
		t.Fatalf("конфиг тронут: %q %v", c.LastProject, c.Recent)
	}
	if n := notices(); len(n) != 1 || n[0] != backupNotice {
		t.Fatalf("тосты: %q", n)
	}
	if list, _ := a.backups.List(backup.DraftKey); len(list) != 1 {
		t.Fatalf("черновик не скопирован: %+v", list)
	}
}

// «Сохранить как» в папку копий — отказ, файл не создан.
func TestSaveAsIntoBackupsRefused(t *testing.T) {
	a, _, _ := backupApp(t, 1)
	target := filepath.Join(a.backups.Root, "свой"+projectExt)
	if _, err := a.saveAs(a.editor(), target); err == nil {
		t.Fatal("сохранение в папку копий принято")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("файл создан: %v", err)
	}
	// Поверх чужого файла — сначала его копия.
	other := filepath.Join(testkit.TempDir(t), "чужой"+projectExt)
	if err := os.WriteFile(other, []byte(`{"spreads":[{},{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveAs(a.editor(), other); err != nil {
		t.Fatal(err)
	}
	if list, _ := a.backups.List(backup.Key(other)); len(list) != 1 || list[0].Spreads != 3 {
		t.Fatalf("копия перезаписанного файла: %+v", list)
	}
}

// «Новый» удаляет черновик — сначала его копия; пустой черновик не копируется.
func TestDraftBackedUpBeforeRemove(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	addSpread(t, a)
	a.NewProject("album") // flushSave пишет черновик, затем он удаляется
	if _, err := os.Stat(a.draftPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("черновик не удалён: %v", err)
	}
	list, err := a.backups.List(backup.DraftKey)
	if err != nil || len(list) != 1 || list[0].Spreads != 1 {
		t.Fatalf("копия черновика: %+v, %v", list, err)
	}
	a.NewProject("album") // пустой черновик
	if list, _ := a.backups.List(backup.DraftKey); len(list) != 1 {
		t.Fatalf("пустой черновик скопирован: %+v", list)
	}
}

// Папка копий недоступна (Root — файл): сохранение идёт как обычно.
func TestBackupFailureDoesNotBlockSave(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	root := filepath.Join(testkit.TempDir(t), "файл")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.backups = backup.New(root)
	addSpread(t, a)
	a.flushSave()
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Spreads) != 2 {
		t.Fatalf("сохранено разворотов %d, ждали 2", len(p.Spreads))
	}
	if _, err := a.SaveProject(); err != nil {
		t.Fatalf("Ctrl+S: %v", err)
	}
}

// 10 копий, восстановлена самая давняя: перед заменой текущий файл ушёл в копию, а сама
// восстановленная осталась — после Ctrl+Z её версия на месте. Холостое восстановление дубля не плодит.
func TestRestoreOldestBackupSurvivesUndo(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	key := backup.Key(file)
	for range backup.Keep {
		if err := a.backups.Force(file, key); err != nil {
			t.Fatal(err)
		}
		addSpread(t, a)
		if err := a.editor().Save(""); err != nil {
			t.Fatal(err)
		}
	}
	list := backups(t, a)
	oldest := list[len(list)-1]
	if len(list) != backup.Keep || oldest.Spreads != 1 {
		t.Fatalf("копии: %+v", list)
	}
	if _, err := a.RestoreBackup(oldest.Name); err != nil {
		t.Fatal(err)
	}
	if v := a.Undo(); len(v.Spreads) != backup.Keep+1 {
		t.Fatalf("отмена: %d разворотов", len(v.Spreads))
	}
	list = backups(t, a)
	if len(list) != backup.Keep || !slices.ContainsFunc(list, func(b BackupInfo) bool { return b.Name == oldest.Name }) {
		t.Fatalf("восстановленная копия пропала: %+v", list)
	}
	if list[0].Spreads != backup.Keep+1 {
		t.Fatalf("текущий файл не в копии: %+v", list[0])
	}
	// Самая новая копия совпадает с файлом и альбомом: восстановление холостое, копий столько же.
	if _, err := a.RestoreBackup(list[0].Name); err != nil {
		t.Fatal(err)
	}
	if got := backups(t, a); len(got) != len(list) || got[0].Name != list[0].Name {
		t.Fatalf("дубль: %+v", got)
	}
}

// Перезапись без правок тоже снимает копию: decode мог поправить файл при загрузке, и исходник
// иначе ушёл бы без следа. Повторная запись того же — без дубля.
func TestSaveWithoutEditsBacksUp(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	orig := append(raw, "\n\n"...) // Save запишет иначе
	if err := os.WriteFile(file, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, p, file))
	a.flushSave()
	list := backups(t, a)
	if len(list) != 1 || list[0].Size != int64(len(orig)) {
		t.Fatalf("копия исходника: %+v", list)
	}
	a.flushSave()
	if _, err := a.SaveProject(); err != nil {
		t.Fatal(err)
	}
	if got := backups(t, a); len(got) != 1 {
		t.Fatalf("дубли: %+v", got)
	}
}

// Пропавший диск: копия не пытается читать файл.
func TestBackupSkipsUnreachable(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	old := reachable
	reachable = func(string) bool { return false }
	t.Cleanup(func() { reachable = old })
	a.backupBefore(file, backup.Key(file), true)
	if list := backups(t, a); len(list) != 0 {
		t.Fatalf("копия с недоступного диска: %+v", list)
	}
}

// В лог — без путей пользователя.
func TestBackupErrHidesPaths(t *testing.T) {
	const secret = `C:\Users\Имя\Альбомы\Ивановы.salbum`
	for _, err := range []error{
		&fs.PathError{Op: "open", Path: secret, Err: fs.ErrPermission},
		&os.LinkError{Op: "rename", Old: secret + ".1.tmp", New: secret, Err: fs.ErrExist},
	} {
		if got := paths.NoPath(err).Error(); strings.Contains(got, "Имя") || !strings.Contains(got, err.(interface{ Unwrap() error }).Unwrap().Error()) {
			t.Errorf("лог: %q", got)
		}
	}
}
