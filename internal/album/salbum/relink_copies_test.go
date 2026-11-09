package salbum

import (
	"image/color"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Перепривязка по файлу (098-20): запись из moves тянет всю семью копий; кадры записей целы.

// copiesAt — проект: файл old стоит записью и копией на двух листах с кадрами S=2 и S=3.
func copiesAt(old string) (*model.Project, string) {
	a := library.IDFor(old)
	p := model.New()
	p.Photos[a] = model.PhotoRef{Path: old, W: 4, H: 4, Frame: &model.Frame{S: 2}}
	p.Photos[model.CopyID(a, 2)] = model.PhotoRef{Path: old, W: 4, H: 4, Frame: &model.Frame{S: 3}}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{a}}, {ID: "s2", Photos: []string{model.CopyID(a, 2)}}}
	return p, a
}

// checkMoved — обе записи на новом пути с новым ID файла, кадры и места прежние.
func checkMoved(t *testing.T, p *model.Project, np string) {
	t.Helper()
	b := library.IDFor(np)
	r1, ok1 := p.Photos[b]
	r2, ok2 := p.Photos[model.CopyID(b, 2)]
	if !ok1 || !ok2 || r1.Path != np || r2.Path != np || r1.Frame.S != 2 || r2.Frame.S != 3 {
		t.Fatalf("записи после перепривязки: %v", p.Photos)
	}
	if p.Spreads[0].Photos[0] != b || p.Spreads[1].Photos[0] != model.CopyID(b, 2) {
		t.Fatalf("места: %v, %v", p.Spreads[0].Photos, p.Spreads[1].Photos)
	}
}

// В moves — одна запись семьи (копия): едут обе, пара — одна на файл; повтор — ноль пар.
func TestRelinkMovesCopies(t *testing.T) {
	dir := testkit.TempDir(t)
	np := testkit.PNG(t, filepath.Join(dir, "новое"), "a.png", 4, 4, color.NRGBA{A: 255})
	p, a := copiesAt(filepath.Join(dir, "старое", "a.png"))
	files, _ := Relink(p, map[string]string{model.CopyID(a, 2): np}, "")
	if len(files) != 1 || files[0].New != np || files[0].BgOnly {
		t.Fatalf("пары: %+v", files)
	}
	checkMoved(t, p, np)
	if files, _ := Relink(p, map[string]string{model.CopyID(a, 2): np}, ""); len(files) != 0 {
		t.Fatalf("повтор: %+v", files)
	}
}

// Новый путь занят файлом другой семьи — не переезжает никто; путь своей семьи — не помеха.
func TestRelinkTakenByOtherFamily(t *testing.T) {
	dir := testkit.TempDir(t)
	np := testkit.PNG(t, dir, "b.png", 4, 4, color.NRGBA{A: 255})
	p, a := copiesAt(filepath.Join(dir, "старое", "a.png"))
	b := library.IDFor(np)
	p.Photos[b] = model.PhotoRef{Path: np, W: 4, H: 4}
	p.Spreads[0].Photos = append(p.Spreads[0].Photos, b)
	if files, _ := Relink(p, map[string]string{a: np}, ""); len(files) != 0 {
		t.Fatalf("занятый путь: %+v", files)
	}
	if _, ok := p.Photos[model.CopyID(a, 2)]; !ok {
		t.Fatal("копия пропала")
	}
}

// Смена буквы диска: копии того же файла едут вместе.
func TestRelinkDriveCopies(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("пути с буквами дисков")
	}
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	np := testkit.PNG(t, root, "a.png", 4, 4, color.NRGBA{A: 255})
	p, _ := copiesAt(from + ":" + np[len(vol):])
	if files, _ := RelinkDrive(p, from, to); len(files) != 1 {
		t.Fatalf("пары: %+v", files)
	}
	checkMoved(t, p, np)
}
