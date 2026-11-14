//go:build windows

package collect

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cloudAlbum — проект с одним фото-заглушкой stub.
func cloudAlbum(stub string, size int) *model.Project {
	p := model.New()
	p.Folders = []string{filepath.Dir(stub)}
	id := library.IDFor(stub)
	p.Photos[id] = model.PhotoRef{Path: stub, W: 3, H: 2, Size: int64(size)}
	p.Spreads = []*model.Spread{{ID: "0123456789a1", Photos: []string{id}}}
	return p
}

// TestCollectCloudStubSource (аудит 160-M1 №1): фото и сайдкар — облачные заглушки OneDrive. Сбор — явное действие
// над фото: план их пускает, copyOne открывает через OpenLocal (скачивает), и Copy не обрывается «изменился во время
// сбора». Повторный сбор в ту же папку узнаёт копии: «уже на месте», без « (2)». Блоки независимы.
func TestCollectCloudStubSource(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	data := bytes.Repeat([]byte("cloud-photo."), 50)
	side := []byte("<x:xmpmeta/>")
	stub := root.Stub(t, "IMG_0001.jpg", data)
	root.Stub(t, "IMG_0001.jpg.xmp", side)
	p := cloudAlbum(stub, len(data))
	dir := filepath.Join(testkit.TempDir(t), "out")

	pl, err := Make(p, nil, dir, os.Lstat)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("план", func(t *testing.T) {
		if len(pl.Items) != 2 || len(pl.Missing) != 0 {
			t.Fatalf("план: %d записей, пропавших %d (ждали фото и сайдкар)", len(pl.Items), len(pl.Missing))
		}
	})
	n, err := Copy(context.Background(), dir, &pl, nil)
	t.Run("копирование", func(t *testing.T) {
		if err != nil || n != 2 {
			t.Fatalf("Copy: %d, %v", n, err)
		}
		for _, it := range pl.Items {
			got, rerr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(it.Rel)))
			want := data
			if it.Kind == KindXMP {
				want = side
			}
			if rerr != nil || !bytes.Equal(got, want) {
				t.Errorf("%s: %d байт, %v", it.Rel, len(got), rerr)
			}
		}
		if root.Fetches() == 0 {
			t.Error("поставщик не вызывался: данные заглушки не скачаны")
		}
	})
	t.Run("повторный сбор", func(t *testing.T) {
		again, err := Make(p, nil, dir, os.Lstat)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range again.Items {
			if !it.Skip {
				t.Errorf("не «уже на месте»: %s", it.Rel)
			}
		}
	})
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}

// TestSameCloudStub (аудит 160-M1 №3): прежняя копия в папке назначения стала заглушкой (папка OneDrive выгрузила
// её) — same и names.free узнают её по размеру и времени, имя « (2)» не берётся; другой размер — не та же.
func TestSameCloudStub(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	data := []byte("12345")
	stub := root.Stub(t, "a.jpg", data)
	stStub, err := os.Lstat(stub)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(testkit.TempDir(t), "a.jpg")
	put(t, src, string(data))
	if err := os.Chtimes(src, stStub.ModTime(), stStub.ModTime()); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(root.Dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("листинг корня: %v %v", ents, err)
	}
	t.Run("та же копия", func(t *testing.T) {
		if !same(root.Dir, ents[0], st) {
			t.Error("заглушка с тем же размером и временем не узнана")
		}
		nm := names{used: map[string]bool{}, onDisk: map[string]fs.DirEntry{nameKey("a.jpg"): ents[0]}, dir: root.Dir}
		if name, skip, ok := nm.pick("a.jpg", st); !ok || !skip || name != "a.jpg" {
			t.Errorf("pick = %q skip=%v ok=%v, ждали a.jpg «уже на месте»", name, skip, ok)
		}
	})
	t.Run("другой размер", func(t *testing.T) {
		put(t, src, "123456")
		longer, err := os.Stat(src)
		if err != nil {
			t.Fatal(err)
		}
		if same(root.Dir, ents[0], longer) {
			t.Error("заглушка другого размера признана той же")
		}
	})
	t.Run("без скачивания", func(t *testing.T) {
		if n := root.Fetches(); n != 0 {
			t.Errorf("сверка скачала заглушку: %d", n)
		}
	})
}

// TestSaveCloudStubPhoto (аудит 160-M1 №2): после сбора фото собранного альбома — заглушка (копии в папке OneDrive
// выгружаются); Save их не считает пропавшими.
func TestSaveCloudStubPhoto(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	data := []byte("12345")
	stub := root.Stub(t, "a.jpg", data)
	q := cloudAlbum(stub, len(data))
	if err := Save(q, model.Side{}, filepath.Join(testkit.TempDir(t), "a.salbum"), Plan{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
}
