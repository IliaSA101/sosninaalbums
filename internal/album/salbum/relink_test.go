package salbum_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestRelinkRekeys(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	before := s.E.View()
	v := s.E.Relink(map[string]string{s.IdA: s.NewA, s.IdB: s.NewB}, "")
	nA, nB := library.IDFor(s.NewA), library.IDFor(s.NewB)
	if got := v.Spreads[0].Photos; !slices.Equal(got, []string{nA, nB}) {
		t.Fatalf("ID в развороте: %v, ждали [%s %s]", got, nA, nB)
	}
	refs := s.E.Refs()
	if r := refs[nA]; r.Path != s.NewA || r.Frame == nil || r.Frame.S != 2 {
		t.Errorf("фото A: %+v (кадр должен остаться)", r)
	}
	if _, old := refs[s.IdA]; old {
		t.Error("старый ID остался")
	}
	// Папка переехала по общему хвосту: …\E\Съёмки → …\F.
	if !slices.Equal(v.Folders, []string{s.NewFd}) {
		t.Errorf("папки: %v, ждали [%s]", v.Folders, s.NewFd)
	}
	// Один шаг отмены возвращает пути, ID и папку.
	u := s.E.Undo()
	if !slices.Equal(u.Spreads[0].Photos, before.Spreads[0].Photos) || !slices.Equal(u.Folders, before.Folders) {
		t.Errorf("после отмены: %v %v", u.Spreads[0].Photos, u.Folders)
	}
	if r := s.E.Refs()[s.IdA]; r.Frame == nil {
		t.Error("отмена откатила и правку кадра: relink не один шаг")
	}
}

func TestRelinkAddFolderAndGuards(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	other := filepath.Join(filepath.Dir(s.NewFd), "Чужая")
	// Имя файла другое: префикс не находится, папки с новым фото нет — добавляется выбранная.
	renamed := filepath.Join(s.NewFd, "Отбор", "новое.jpg")
	edtest.PutFile(t, renamed, 30)
	s.E.RemoveFolder(s.OldFolder)
	v := s.E.Relink(map[string]string{s.IdA: renamed}, s.NewFd)
	if !slices.Equal(v.Folders, []string{s.NewFd}) {
		t.Errorf("addFolder: %v", v.Folders)
	}
	// Ничего не перепривязано (неизвестный ID, UNC) — проект и папки прежние.
	rev := s.E.View().Rev
	v = s.E.Relink(map[string]string{"нет": s.NewA, s.IdB: `\\srv\share\2.jpg`}, other)
	if v.Rev != rev || len(v.Folders) != 1 || s.E.Refs()[s.IdB].Path == `\\srv\share\2.jpg` {
		t.Errorf("пустая перепривязка изменила проект: %v", v.Folders)
	}
	// Путь, занятый другим фото альбома, не берётся.
	s.E.Relink(map[string]string{s.IdB: renamed}, "")
	if s.E.Refs()[s.IdB].Path == renamed {
		t.Error("два фото на одном файле")
	}
}

func TestRelinkDrive(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root) // реальный диск; «пропавший» — любая другая буква
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	tail := root[len(vol):]
	newFolder := filepath.Join(root, "Съёмка")
	edtest.PutFile(t, filepath.Join(newFolder, "1.jpg"), 10)
	oldPath := from + ":" + filepath.Join(tail, "Съёмка", "1.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, from+":"+filepath.Join(tail, "Съёмка"))
	id := library.IDFor(oldPath)
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: id, Path: oldPath, W: 3, H: 2, Size: 10}})
	v := e.RelinkDrive(from, to)
	nid := library.IDFor(filepath.Join(newFolder, "1.jpg"))
	if v.Spreads[0].Photos[0] != nid || !slices.Equal(v.Folders, []string{newFolder}) {
		t.Fatalf("смена диска: %v %v", v.Spreads[0].Photos, v.Folders)
	}
	if v2 := e.RelinkDrive(from, to); v2.Rev != v.Rev {
		t.Error("повторная смена диска — не холостая правка")
	}
}

// Папка не нашлась ни по относительному пути, ни по abs: abs не теряется при записи.
func TestFolderAltKept(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(testkit.TempDir(t), "нет", "Съёмка")
	f := salbum.FileProject{
		Version: model.FormatVersion, Settings: model.DefaultSettings(), Spreads: []*model.Spread{},
		Folders: []salbum.FileFolder{{Path: "Съёмка", Abs: abs}},
		Photos:  map[string]salbum.FilePhoto{},
	}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(dir, "Съёмка")
	if !slices.Equal(p.Folders, []string{rel}) || p.FolderAlt[rel] != abs {
		t.Fatalf("папка: %v, alt %v", p.Folders, p.FolderAlt)
	}
	if err := edtest.Open(t, p, path).Save(""); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, path).Folders; len(got) != 1 || got[0].Path != "Съёмка" || got[0].Abs != abs {
		t.Fatalf("после сохранения: %+v", got)
	}
	// Папка появилась на месте — запасной путь не нужен.
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatal(err)
	}
	p2, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.FolderAlt) != 0 {
		t.Errorf("папка нашлась, alt не нужен: %v", p2.FolderAlt)
	}
}
