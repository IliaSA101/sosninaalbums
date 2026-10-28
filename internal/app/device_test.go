package app

// Имена устройств DOS с расширением в путях альбома (073-10).

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Черновик с фото «CON.jpg», «nul.jpg» и папкой «Con.2024» открывается при запуске и не уходит
// в .bad. На ОС, где это файлы (Windows 11), фото на месте; на «старой ОС» (подмена: Windows 10)
// — недостающие (fileExists их не открывает), обычное фото — на месте; редактор принимает проект.
func TestDraftDeviceNamesKept(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	onDisk := filepath.IsLocal("CON.jpg")
	if !onDisk {
		t.Cleanup(paths.SetDeviceOracle(func(string) bool { return false }))
	}
	dir := testkit.TempDir(t)
	files := []string{filepath.Join(dir, "CON.jpg"), filepath.Join(dir, "Con.2024", "nul.jpg"), filepath.Join(dir, "a.jpg")}
	p := model.New()
	s := &model.Spread{ID: "s1"}
	for _, f := range files {
		if onDisk {
			if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
				t.Fatal(err)
			}
			testkit.JPEG(t, filepath.Dir(f), filepath.Base(f), 30, 20)
		}
		id := library.IDFor(f)
		p.Photos[id] = model.PhotoRef{Path: f, W: 30, H: 20}
		s.Photos = append(s.Photos, id)
	}
	p.Spreads = []*model.Spread{s}
	// Папки — соседи, а не вложенные: вложенную Load сворачивает в корень (093-4).
	p.Folders = []string{filepath.Join(dir, "Свадьба"), filepath.Join(dir, "Con.2024")}
	devIDs := []string{library.IDFor(files[0]), library.IDFor(files[1])}
	slices.Sort(devIDs)

	for _, old := range []bool{false, true} {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		if err := salbum.Save(p, a.draftPath()); err != nil {
			t.Fatal(err)
		}
		restore := func() {}
		if old {
			restore = paths.SetDeviceOracle(paths.Device)
		}
		got, path, notice := a.loadStartup("")
		if path != "" || notice != "" || len(got.Photos) != len(files) {
			t.Fatalf("старая ОС %v: путь %q, уведомление %q, фото %d", old, path, notice, len(got.Photos))
		}
		if bad, _ := filepath.Glob(filepath.Join(a.cfgDir, "черновик.bad-*")); len(bad) != 0 {
			t.Fatalf("старая ОС %v: черновик ушёл в %v", old, bad)
		}
		if _, err := os.Stat(a.draftPath()); err != nil {
			t.Fatalf("старая ОС %v: черновика нет: %v", old, err)
		}
		mustEditor(t, got, "")
		missing := missingOf(got.Photos, fileExists)
		switch {
		case old && !slices.Equal(missing, devIDs):
			t.Fatalf("старая ОС: недостающие %v, ждали %v", missing, devIDs)
		case old && len(got.Folders) != 1:
			t.Fatalf("старая ОС: папки %v, ждали без Con.2024", got.Folders)
		case !old && onDisk && len(missing) != 0:
			t.Fatalf("Windows 11: недостающие %v", missing)
		case !old && len(got.Folders) != 2:
			t.Fatalf("Windows 11: папки %v", got.Folders)
		}
		restore()
	}
}
