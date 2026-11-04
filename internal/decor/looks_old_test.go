package decor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestDecorOldReaderKeepsAlbum — 0.27 не знает вида album (имитация: его нет в kindDirs): её чтение и
// запись реестра (правка своего фона, установка и удаление другого набора) оставляют файл макета и
// запись реестра — своего (library.json) и набора (Наборы/<id>.json) — на месте, а после возврата в
// 0.28 оба макета на плитках и читаются.
func TestDecorOldReaderKeepsAlbum(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 40, 30)
	own, err := s.SaveLook("mine", "Свой макет", lookOf("", ""))
	if err != nil {
		t.Fatal(err)
	}
	look := lookBytes(t, lookOf(libKey(bg), ""), `"auto":{"rule":"x"}`)
	m, src := testPack("wedding-layouts", 1, pit{"bg/a.png", bg}, pit{"looks/main.json", look})
	install(t, s, m, src, nil)
	packKey := model.LookKey(look)

	root := filepath.Join(cfg, RootName)
	packRegPath := filepath.Join(root, packsDir, "wedding-layouts.json")
	read := func(p string) []byte {
		t.Helper()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	ownFile := filepath.Join(root, "Макеты", strings.TrimPrefix(own.ID, "my:")+".json")
	packFile := filepath.Join(root, "Макеты", strings.TrimPrefix(packKey, "my:")+".json")
	files := map[string][]byte{ownFile: read(ownFile), packFile: read(packFile), packRegPath: read(packRegPath)}

	mine := importOK(t, s, "mine", testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 40, 30))
	saved := kindDirs[KindAlbum]
	delete(kindDirs, KindAlbum) // 0.27: видов album нет
	restore := func() { kindDirs[KindAlbum] = saved }
	old := Open(cfg)
	func() {
		defer func() {
			if t.Failed() {
				restore()
			}
		}()
		if _, err := old.Catalog(KindAlbum, nil, 300, 300); err == nil { // имитация работает: вид неизвестен
			t.Fatal("старый читатель знает макеты")
		}
		if err := old.Rename(KindBg, mine[0].ID, "Переименован"); err != nil { // library.json переписан
			t.Fatal(err)
		}
		other, osrc := testPack("other-set", 1, pit{"bg/o.png", renderPNG(t, 41, 30)})
		install(t, old, other, osrc, nil) // реестр другого набора записан
		if _, err := old.RemovePack("other-set", nil); err != nil {
			t.Fatal(err)
		}
	}()
	restore()

	for p, want := range files { // ни файлы макетов, ни реестр набора 0.27 не тронула
		if got := read(p); !bytes.Equal(got, want) {
			t.Errorf("%s изменён:\n%s", filepath.Base(p), got)
		}
	}
	found := false
	for _, e := range readReg(t, cfg).Items {
		found = found || e.Key == own.ID && e.Kind == KindAlbum && e.Name == "Свой макет"
	}
	if !found {
		t.Fatalf("запись своего макета потеряна: %+v", readReg(t, cfg).Items)
	}
	s2 := Open(cfg)
	if _, ok := findItem(t, s2, KindAlbum, own.ID); !ok {
		t.Error("свой макет без плитки после 0.27")
	}
	if got, ok := s2.LookByItem("wedding-layouts", "looks/main.json"); !ok || got != packKey {
		t.Errorf("макет набора после 0.27: %s %v", got, ok)
	}
	for _, id := range []string{own.ID, packKey} {
		if lt, err := s2.Look(id); err != nil || len(lt.Steps) != 3 {
			t.Errorf("Look(%s): %v %+v", id, err, lt)
		}
	}
	if _, ok := s2.Find(libKey(bg)); !ok {
		t.Error("фон макета набора потерян")
	}
}
