package edtest

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// PutFile создаёт файл из n байт (с папками).
func PutFile(t testing.TB, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func MustDecode(t *testing.T, data []byte, dir string) *model.Project {
	t.Helper()
	p, err := salbum.Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := GridInvariants(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func SkipNotWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("пути с буквами дисков")
	}
}

// RelinkSetup — проект с двумя фото на «пропавшем диске E» и папкой съёмки;
// файлы лежат на «диске F» (папки в корне временной папки).
type RelinkSetup struct {
	E                *album.Editor
	IdA, IdB         string
	NewA, NewB       string
	OldFolder, NewFd string
}

func NewRelinkSetup(t *testing.T) RelinkSetup {
	t.Helper()
	root := testkit.TempDir(t)
	s := RelinkSetup{
		OldFolder: filepath.Join(root, "E", "Съёмки", "Ивановы"),
		NewFd:     filepath.Join(root, "F", "Ивановы"),
	}
	oldA, oldB := filepath.Join(s.OldFolder, "Отбор", "1.jpg"), filepath.Join(s.OldFolder, "Отбор", "2.jpg")
	s.NewA, s.NewB = filepath.Join(s.NewFd, "Отбор", "1.jpg"), filepath.Join(s.NewFd, "Отбор", "2.jpg")
	PutFile(t, s.NewA, 30)
	PutFile(t, s.NewB, 40)
	s.IdA, s.IdB = library.IDFor(oldA), library.IDFor(oldB)
	s.E = Open(t, model.New(), "")
	AddFolder(s.E, s.OldFolder)
	NewSpread(s.E, -1, []model.PhotoInfo{
		{ID: s.IdA, Path: oldA, W: 300, H: 200, Size: 30},
		{ID: s.IdB, Path: oldB, W: 300, H: 200, Size: 40},
	})
	Change(s.E, func(p *model.Project) { // кадр должен переехать вместе с фото
		r := p.Photos[s.IdA]
		r.Frame = &model.Frame{S: 2}
		p.Photos[s.IdA] = r
	})
	return s
}

var Update = flag.Bool("update", false, "перезаписать голдены: testdata/parity/marks.json, testdata/replay.json")
