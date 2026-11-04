package app

// Копия перед первой правкой агента (149, §2.4) и паритет выбора свободных фото с окном.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// copiesIn — файлы копий всех альбомов под root.
func copiesIn(t *testing.T, root string) (files []string) {
	t.Helper()
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func TestAgentBackupFirstEdit(t *testing.T) {
	r := newEditRig(t, 6)
	a := r.a
	if _, err := a.agentAuto(t.Context(), r.req()); err != nil { // в альбоме есть что защищать
		t.Fatal(err)
	}
	album := filepath.Join(testkit.TempDir(t), "Альбом.salbum")
	if err := a.editor().Save(album); err != nil {
		t.Fatal(err)
	}
	key := r.key()
	root := filepath.Join(testkit.TempDir(t), "backups")

	// Копий нет, пустой Root: правки нет.
	a.backups = nil
	if err := a.agentBackup(key); !errors.Is(err, errAgentBackup) {
		t.Fatalf("backups == nil: %v", err)
	}
	a.backups = backup.New("")
	if err := a.agentBackup(key); !errors.Is(err, errAgentBackup) {
		t.Fatalf("Root \"\": %v", err)
	}
	// Диск «полон»: папка копий — файл, создать нечего.
	rootFile := filepath.Join(testkit.TempDir(t), "файл")
	if err := os.WriteFile(rootFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.backups = backup.New(filepath.Join(rootFile, "backups"))
	if err := a.agentBackup(key); !errors.Is(err, errAgentBackup) {
		t.Fatalf("диск полон: %v", err)
	}
	// Том недоступен.
	a.backups = backup.New(root)
	a.bg.Wait() // фоновая проверка пропавших (после автосборки) читает reachable — не подменять под ней
	old := reachable
	reachable = func(string) bool { return false }
	err := a.agentBackup(key)
	reachable = old
	if !errors.Is(err, errAgentBackup) {
		t.Fatalf("том недоступен: %v", err)
	}
	if len(copiesIn(t, root)) != 0 {
		t.Fatal("отказы оставили копии")
	}
	// Ключ чужого альбома: копии нет, ошибка ключа.
	if err := a.agentBackup("kx"); !errors.Is(err, errAgentAlbumChanged) {
		t.Fatalf("чужой ключ: %v", err)
	}

	// Одиннадцать переподключений подряд: одна копия, права 0o600, содержимое — альбом на диске.
	for range 11 {
		if err := a.agentBackup(key); err != nil {
			t.Fatal(err)
		}
	}
	got := copiesIn(t, root)
	if len(got) != 1 {
		t.Fatalf("копий %d, ждали одну: %v", len(got), got)
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(got[0]); err != nil || st.Mode().Perm() != 0o600 {
			t.Fatalf("права копии: %v %v", st, err)
		}
	}
	want, _ := os.ReadFile(album)
	if data, _ := os.ReadFile(got[0]); len(data) == 0 || string(data) != string(want) {
		t.Fatal("копия не равна сохранённому альбому")
	}
}

// Пустой несохранённый альбом: копировать нечего — nil, копий нет.
func TestAgentBackupNothing(t *testing.T) {
	r := newEditRig(t, 2)
	root := filepath.Join(testkit.TempDir(t), "backups")
	r.a.backups = backup.New(root)
	a := r.a
	a.setEditor(blankEditor(""))
	a.agentEd().opts.Key = func(ed uint64) string { return "n" }
	if err := a.agentBackup("n"); err != nil {
		t.Fatalf("пустой альбом: %v", err)
	}
	if len(copiesIn(t, root)) != 0 {
		t.Fatal("копия пустого альбома")
	}
}

// Паритет с окном: выбор свободных фото и N по умолчанию — testdata/parity/autopick.json
// (то же читает vitest: freephotos.ts autoPick, autobuild.ts defaultSpreads).
func TestAutobuildFree(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "autopick.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Pick []struct {
			Name   string
			Photos []struct {
				ID  string
				Xmp *library.Marks
			}
			Own      map[string]model.MarkView
			Placed   []string
			Backdrop []string
			Want     []string
			Skipped  int
		}
		Spreads []struct {
			Photos int
			Range  *[2]int
			Want   int
		}
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Pick) < 5 || len(f.Spreads) < 5 {
		t.Fatalf("в файле мало случаев: %d, %d", len(f.Pick), len(f.Spreads))
	}
	for _, c := range f.Pick {
		t.Run(c.Name, func(t *testing.T) {
			var ph []library.Photo
			for _, p := range c.Photos {
				ph = append(ph, library.Photo{ID: p.ID, Xmp: p.Xmp})
			}
			ids, skipped := freePick(ph, c.Own, c.Placed, c.Backdrop)
			if len(ids) != len(c.Want) || skipped != c.Skipped {
				t.Fatalf("%v (пропущено %d), ждали %v (%d)", ids, skipped, c.Want, c.Skipped)
			}
			for i := range ids {
				if ids[i] != c.Want[i] {
					t.Fatalf("%v, ждали %v", ids, c.Want)
				}
			}
		})
	}
	for _, c := range f.Spreads {
		var rng [2]int
		if c.Range != nil {
			rng = *c.Range
		}
		if got := defaultSpreads(c.Photos, rng); got != c.Want {
			t.Errorf("defaultSpreads(%d, %v) = %d, ждали %d", c.Photos, rng, got, c.Want)
		}
	}
}

// Из ленты берутся свободные: не в альбоме, не фон, не отклонённые; явный список — тем же правилом.
func TestAgentFreeFromLibrary(t *testing.T) {
	r := newEditRig(t, 6)
	a, ed := r.a, r.a.editor()
	rej := -1
	if _, err := ed.SetMarks(a.infos(r.photos[:1]), model.MarkPatch{Flag: &rej}); err != nil {
		t.Fatal(err)
	}
	q := r.req()
	q.Photos = r.photos[:3] // первое отклонено — из явного списка тоже выпадает
	res, err := a.agentAuto(t.Context(), q)
	if err != nil || res.Photos != 2 {
		t.Fatalf("явный список: %+v %v", res, err)
	}
	q = r.req()
	res, err = a.agentAuto(t.Context(), q) // остальные три свободных (два уже стоят, одно отклонено)
	if err != nil || res.Photos != 3 {
		t.Fatalf("остальные: %+v %v", res, err)
	}
}
