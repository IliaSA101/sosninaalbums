package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// evApp — App без окна: события собираются списком (имя, данные).
type evRec struct {
	name string
	data []any
}

func evApp(t *testing.T) (*App, func(name string) []evRec) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var all []evRec
	a.emit = func(name string, data ...any) {
		mu.Lock()
		all = append(all, evRec{name, data})
		mu.Unlock()
	}
	return a, func(name string) []evRec {
		mu.Lock()
		defer mu.Unlock()
		var out []evRec
		for _, e := range all {
			if e.name == name {
				out = append(out, e)
			}
		}
		return out
	}
}

// Лента после rescan держит фото отдельных файлов, у которых нет папки, и убирает прочие.
func TestRescanKeptLoose(t *testing.T) {
	root := testkit.TempDir(t)
	one, two := filepath.Join(root, "Одна"), filepath.Join(root, "Две")
	for _, d := range []string{one, two} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inOne := testkit.JPEG(t, one, "a.jpg", 20, 10)
	inTwo := testkit.JPEG(t, two, "b.jpg", 20, 10)
	loose := testkit.JPEG(t, two, "c.jpg", 20, 10)
	photos := []library.Photo{{Path: inOne}, {Path: inTwo}, {Path: loose}, {Path: filepath.Join(root, "Чужое", "x.jpg")}}
	// Отдельный файл в другом регистре — тот же на Windows и macOS, на Linux — чужой (paths.Fold, 101).
	upLoose := []string{loose}
	if !paths.FoldEqual(strings.ToUpper(loose), loose) {
		upLoose = nil
	}
	cases := []struct {
		name string
		src  library.Sources
		want []string
	}{
		{"папка", library.Sources{Folders: []string{one}}, []string{inOne}},
		{"отдельное без папки", library.Sources{Files: []string{loose}}, []string{loose}},
		{"отдельное в другом регистре", library.Sources{Files: []string{strings.ToUpper(loose)}}, upLoose},
		{"папка и отдельное", library.Sources{Folders: []string{one}, Files: []string{loose}}, []string{inOne, loose}},
		{"две папки и файл", library.Sources{Folders: []string{one, two}, Files: []string{loose}}, []string{inOne, inTwo, loose}},
		{"пусто", library.Sources{}, nil},
	}
	for _, c := range cases {
		var got []string
		for _, p := range keptPhotos(photos, c.src) {
			got = append(got, p.Path)
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: оставлено %v, ждали %v", c.name, got, c.want)
		}
	}
	if got := photosUnder(photos, nil, nil); got != nil {
		t.Errorf("без источников: %v", got)
	}

	// Через App: убрали папку — фото отдельного файла из неё остаётся, остальные уходят.
	a, _ := evApp(t)
	for _, d := range []string{one, two} {
		if _, _, err := a.editor().AddFolder(d); err != nil {
			t.Fatal(err)
		}
	}
	a.rescan()
	waitScan(t, a)
	if n := len(photoNames(a)); n != 3 {
		t.Fatalf("после чтения папок фото %d", n)
	}
	a.RemoveFolder(two)
	if _, err := a.AddFilePaths([]string{loose, inTwo}); err != nil {
		t.Fatal(err)
	}
	waitScan(t, a)
	if got := photoNames(a); fmt.Sprint(got) != "[b.jpg c.jpg a.jpg]" {
		t.Errorf("лента %v, ждали фото папки и двух отдельных", got)
	}
}

// Undo и Redo добавления отдельных фото перечитывают ленту: фото уходит и возвращается.
func TestStepUndoLooseRescans(t *testing.T) {
	a, _ := evApp(t)
	dir := testkit.TempDir(t)
	f1 := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	f2 := testkit.JPEG(t, dir, "b.jpg", 20, 10)
	r, err := a.AddFilePaths([]string{f1, f2})
	if err != nil || r.Added != 2 || r.Batch == "" {
		t.Fatalf("добавление: %+v, %v", r, err)
	}
	waitScan(t, a)
	if n := len(photoNames(a)); n != 2 {
		t.Fatalf("после добавления фото %d", n)
	}
	if v := a.Undo(); len(v.Loose) != 0 {
		t.Fatalf("после Undo добавления %d", len(v.Loose))
	}
	waitScan(t, a)
	if n := len(photoNames(a)); n != 0 {
		t.Fatalf("после Undo в ленте %d фото", n)
	}
	if v := a.Redo(); len(v.Loose) != 1 {
		t.Fatalf("после Redo добавлений %d", len(v.Loose))
	}
	waitScan(t, a)
	if n := len(photoNames(a)); n != 2 {
		t.Fatalf("после Redo фото %d", n)
	}
	// Убрать один файл: сразу из ленты, шаг отмены возвращает.
	v := a.RemoveLoose(r.Batch, f1)
	waitScan(t, a)
	if len(v.Loose) != 1 || len(v.Loose[0].Files) != 1 || fmt.Sprint(photoNames(a)) != "[b.jpg]" {
		t.Fatalf("после RemoveLoose: %+v, лента %v", v.Loose, photoNames(a))
	}
	a.Undo()
	waitScan(t, a)
	if n := len(photoNames(a)); n != 2 {
		t.Fatalf("после Undo удаления фото %d", n)
	}
	// Неизвестное — холостая: вид тот же, шага нет.
	rev := a.editor().View().Rev
	if v := a.RemoveLoose("000000000000", ""); v.Rev != rev {
		t.Errorf("холостое удаление сменило rev: %d → %d", rev, v.Rev)
	}
}

// Нелокальные пути и слишком длинный список отклоняются до обращения к диску.
func TestAddFilePathsUNCNoStat(t *testing.T) {
	a, _ := evApp(t)
	captureLog(t)
	var statted []string
	prev := looseStat
	looseStat = func(p string) (fs.FileInfo, error) {
		statted = append(statted, p)
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { looseStat = prev })

	bad := []string{`\\сервер\шара\a.jpg`, `//host/share/b.jpg`, `\\?\C:\x.jpg`, "относительный/c.jpg"}
	if runtime.GOOS == "windows" {
		t.Cleanup(paths.SetDeviceOracle(paths.Device)) // как судит ОС (073-10): с расширением — устройство
		bad = append(bad, `C:\x\COM1.jpg`, `C:\Фото\NUL.png`)
	}
	rev := a.editor().View().Rev
	// Локальный путь не фото — под текущую ОС: `C:\…` на Linux не абсолютный и ушёл бы в Bad.
	r, err := a.AddFilePaths(append(bad, filepath.Join(testkit.TempDir(t), "фото", "читаемо.txt")))
	if err != nil {
		t.Fatal(err)
	}
	if len(statted) != 0 {
		t.Fatalf("Stat вызван до отказа: %v", statted)
	}
	if r.Bad != len(bad) || r.Skipped != 1 || r.Added != 0 || r.Batch != "" || r.View.Rev != rev {
		t.Errorf("итог %+v", r)
	}

	// Список длиннее предела: ошибка без единого обращения к диску и к CheckLocalPath-пути.
	long := make([]string, model.MaxLooseFiles+1)
	for i := range long {
		long[i] = fmt.Sprintf(`\\srv\s\%d.jpg`, i)
	}
	r, err = a.AddFilePaths(long)
	if err == nil || !strings.Contains(err.Error(), "2 000") || !strings.Contains(err.Error(), "1") {
		t.Errorf("длинный список: %v", err)
	}
	if len(statted) != 0 || r.Bad != 0 || len(r.View.Loose) != 0 {
		t.Errorf("длинный список дошёл до разбора: stat %d, %+v", len(statted), r)
	}
	if got := captureLogText(t, a, bad); strings.Contains(got, "сервер") || strings.Contains(got, "host") {
		t.Errorf("в логе путь: %q", got)
	}
}

// captureLogText — log.txt после AddFilePaths(ps): только счёты.
func captureLogText(t *testing.T, a *App, ps []string) string {
	t.Helper()
	buf := captureLog(t)
	if _, err := a.AddFilePaths(ps); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// Файлы отбираются по виду: не фото, папка с именем фото, пропавший — Skipped; настоящие — добавляются.
func TestAddFilePathsKinds(t *testing.T) {
	a, _ := evApp(t)
	dir := testkit.TempDir(t)
	good := testkit.JPEG(t, dir, "g.jpg", 20, 10)
	txt := filepath.Join(dir, "n.txt")
	lnk := filepath.Join(dir, "ярлык.lnk")
	fakeDir := filepath.Join(dir, "папка.jpg")
	for _, p := range []string{txt, lnk} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := a.AddFilePaths([]string{txt, lnk, fakeDir, filepath.Join(dir, "нет.jpg"), good, good})
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 1 || r.Skipped != 4 || r.Dup != 1 || r.Bad != 0 {
		t.Errorf("итог %+v", r)
	}
	// Повторное добавление того же — холостая: ни события, ни шага.
	rev := a.editor().View().Rev
	r, err = a.AddFilePaths([]string{good})
	if err != nil || r.Added != 0 || r.Dup != 1 || r.Batch != "" || r.View.Rev != rev {
		t.Errorf("повтор: %+v, %v", r, err)
	}
}

// Сверх MaxLooseAdds: ошибка, проект не изменён, чтение папок не начиналось (нет полупроекта).
func TestAddFilePathsOverBatches(t *testing.T) {
	a, ev := evApp(t)
	dir := testkit.TempDir(t)
	for i := range model.MaxLooseAdds {
		if _, _, err := a.editor().AddLoose([]string{filepath.Join(dir, fmt.Sprintf("f%d.jpg", i))}, 1); err != nil {
			t.Fatal(err)
		}
	}
	extra := testkit.JPEG(t, dir, "extra.jpg", 20, 10)
	before := a.editor().View()
	a.mu.Lock()
	gen := a.scanGen
	a.mu.Unlock()
	r, err := a.AddFilePaths([]string{extra})
	if err == nil {
		t.Fatal("ждали отказ сверх предела добавлений")
	}
	after := a.editor().View()
	if after.Rev != before.Rev || len(after.Loose) != model.MaxLooseAdds || r.Added != 0 {
		t.Errorf("проект изменён: rev %d → %d, добавлений %d, итог %+v", before.Rev, after.Rev, len(after.Loose), r)
	}
	a.mu.Lock()
	gen2 := a.scanGen
	a.mu.Unlock()
	if gen2 != gen || len(ev("library")) != 0 {
		t.Errorf("чтение папок началось после отказа: поколение %d → %d", gen, gen2)
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("не та ошибка: %v", err)
	}
}

// Бросок файлов в окно — одно добавление и одно событие loose:added; файлы внутри брошенной папки
// не дублируются; не фото считаются.
func TestDropFilesLoose(t *testing.T) {
	a, ev := evApp(t)
	captureLog(t)
	root := testkit.TempDir(t)
	folder := filepath.Join(root, "Папка")
	other := filepath.Join(root, "Прочее")
	for _, d := range []string{folder, other} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inside := testkit.JPEG(t, folder, "in.jpg", 20, 10)
	f1 := testkit.JPEG(t, other, "a.jpg", 20, 10)
	f2 := testkit.JPEG(t, other, "b.png", 20, 10)
	note := filepath.Join(other, "n.txt")
	if err := os.WriteFile(note, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.dropPaths([]string{f1, folder, inside, f2, note})
	waitScan(t, a)
	v := a.editor().View()
	if len(v.Loose) != 1 || len(v.Loose[0].Files) != 2 {
		t.Fatalf("добавления %+v, ждали одно из двух файлов", v.Loose)
	}
	evs := ev("loose:added")
	if len(evs) != 1 {
		t.Fatalf("событий loose:added %d", len(evs))
	}
	got, ok := evs[0].data[0].(model.LooseAdded)
	if !ok || got.Added != 2 || got.InFolder != 1 || got.Skipped != 1 || got.Batch != v.Loose[0].ID {
		t.Errorf("событие %+v", evs[0].data)
	}
	if len(ev("folders:added")) != 1 {
		t.Errorf("folders:added %d раз", len(ev("folders:added")))
	}
	if names := photoNames(a); len(names) != 3 {
		t.Errorf("лента %v, ждали три фото", names)
	}
	// Бросок одних файлов, уже добавленных — событие есть (тост), добавления нет.
	a.dropPaths([]string{f1})
	if len(ev("loose:added")) != 2 || len(a.editor().View().Loose) != 1 {
		t.Errorf("повторный бросок: событий %d, добавлений %d", len(ev("loose:added")), len(a.editor().View().Loose))
	}
}

// folders:added: новая папка, повтор, вложенная, родитель с вошедшими папкой и отдельным фото.
func TestFoldersAddedEvent(t *testing.T) {
	a, ev := evApp(t)
	root := testkit.TempDir(t)
	parent := filepath.Join(root, "Съёмка")
	child := filepath.Join(parent, "Банкет")
	for _, d := range []string{parent, child} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	photo := testkit.JPEG(t, child, "p.jpg", 20, 10)
	last := func() FolderAdded {
		t.Helper()
		evs := ev("folders:added")
		if len(evs) == 0 {
			t.Fatal("события нет")
		}
		return evs[len(evs)-1].data[0].(FolderAdded)
	}
	if _, err := a.AddFolderPath(child); err != nil {
		t.Fatal(err)
	}
	if e := last(); e != (FolderAdded{Dir: child}) {
		t.Errorf("новая: %+v", e)
	}
	// Повтор другим регистром — на Windows и macOS; на Linux это другая папка (paths.Fold, 101).
	again := strings.ToUpper(child)
	if !paths.FoldEqual(again, child) {
		again = child
	}
	if _, err := a.AddFolderPath(again); err != nil {
		t.Fatal(err)
	}
	if e := last(); e.Dir != again || e.Within != again || e.Merged != 0 {
		t.Errorf("повтор: %+v, Within должен быть равен Dir", e)
	}
	// Отдельное фото вне папок, затем родитель: забирает и папку, и фото.
	other := testkit.TempDir(t)
	loose := testkit.JPEG(t, other, "o.jpg", 20, 10)
	if _, err := a.AddFilePaths([]string{loose}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddFolderPath(other); err != nil {
		t.Fatal(err)
	}
	if e := last(); e.Within != "" || e.LooseIn != 1 {
		t.Errorf("папка с отдельным: %+v", e)
	}
	if _, err := a.AddFolderPath(parent); err != nil {
		t.Fatal(err)
	}
	if e := last(); e.Within != "" || e.Merged != 1 {
		t.Errorf("родитель: %+v", e)
	}
	if _, err := a.AddFolderPath(filepath.Join(parent, "Банкет")); err != nil {
		t.Fatal(err)
	}
	if e := last(); e.Within != parent {
		t.Errorf("вложенная: %+v, ждали Within = %s", e, parent)
	}
	if n := len(ev("folders:added")); n != 5 {
		t.Errorf("событий %d, ждали 5", n)
	}
	// Ошибка добавления (нелокальный путь) — без события.
	if _, err := a.AddFolderPath(`\\srv\share\x`); err == nil {
		t.Error("UNC принят")
	}
	if n := len(ev("folders:added")); n != 5 {
		t.Errorf("событие на ошибку: %d", n)
	}
	_ = photo
}

// ChooseFiles: фильтр изображений и стартовая папка, итог добавления; отмена диалога — холостая.
func TestChooseFiles(t *testing.T) {
	a, _ := evApp(t)
	dir := testkit.TempDir(t)
	f := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	prev := openFilesDialog
	t.Cleanup(func() { openFilesDialog = prev })
	var opts wr.OpenDialogOptions
	openFilesDialog = func(_ context.Context, o wr.OpenDialogOptions) ([]string, error) {
		opts = o
		return []string{f}, nil
	}
	r, err := a.ChooseFiles()
	if err != nil || r.Added != 1 {
		t.Fatalf("итог %+v, %v", r, err)
	}
	if len(opts.Filters) != 1 || !strings.Contains(opts.Filters[0].Pattern, "*.png") {
		t.Errorf("фильтр %+v", opts.Filters)
	}
	// Второй раз диалог открывается в папке прошлого выбора.
	openFilesDialog = func(_ context.Context, o wr.OpenDialogOptions) ([]string, error) {
		opts = o
		return nil, nil
	}
	rev := a.editor().View().Rev
	r, err = a.ChooseFiles()
	if err != nil || r.Added != 0 || r.View.Rev != rev {
		t.Errorf("отмена диалога: %+v, %v", r, err)
	}
	if !strings.EqualFold(opts.DefaultDirectory, dir) {
		t.Errorf("стартовая папка %q, ждали %q", opts.DefaultDirectory, dir)
	}
}

// Сканер видит файл, пропавший с диска: после чтения его нет в ленте, запись остаётся.
func TestRescanLooseMissingFile(t *testing.T) {
	a, _ := evApp(t)
	dir := testkit.TempDir(t)
	f := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	if _, err := a.AddFilePaths([]string{f}); err != nil {
		t.Fatal(err)
	}
	waitScan(t, a)
	if err := os.Remove(f); err != nil {
		t.Fatal(err)
	}
	a.Rescan()
	waitScan(t, a)
	if n := len(photoNames(a)); n != 0 {
		t.Errorf("пропавший файл в ленте: %d", n)
	}
	if v := a.editor().View(); len(v.Loose) != 1 {
		t.Errorf("запись потеряна: %+v", v.Loose)
	}
}
