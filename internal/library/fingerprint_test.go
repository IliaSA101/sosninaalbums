package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// scannedDir — папка из трёх фото, уже прочитанная сканером (отпечаток записан).
func scannedDir(t *testing.T) (s *Scanner, dir string) {
	t.Helper()
	dir = testkit.TempDir(t)
	for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		testkit.JPEG(t, dir, n, 40, 30)
	}
	s = NewScanner()
	if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func changed(t *testing.T, s *Scanner, folders ...string) bool {
	t.Helper()
	ch, took, err := s.Changed(context.Background(), Sources{Folders: folders})
	if err != nil || took < 0 {
		t.Fatalf("Changed: %v, %v", took, err)
	}
	return ch
}

func touch(t *testing.T, p string, d time.Duration) {
	t.Helper()
	at := time.Now().Add(d)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestScannerChanged(t *testing.T) {
	t.Run("без изменений", func(t *testing.T) {
		s, dir := scannedDir(t)
		for i := range 3 { // сам вызов отпечаток не трогает
			if changed(t, s, dir) {
				t.Fatalf("проверка %d: без изменений, а changed", i)
			}
		}
	})
	t.Run("первый вызов до Scan", func(t *testing.T) {
		if !changed(t, NewScanner(), testkit.TempDir(t)) {
			t.Fatal("Scan не было, а changed = false")
		}
	})
	t.Run("пустая папка после Scan", func(t *testing.T) {
		dir := testkit.TempDir(t)
		s := NewScanner()
		if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
			t.Fatal(err)
		}
		if changed(t, s, dir) {
			t.Fatal("пустая папка не менялась")
		}
		testkit.JPEG(t, dir, "x.jpg", 10, 10)
		if !changed(t, s, dir) {
			t.Fatal("первое фото в пустой папке не замечено")
		}
	})
	t.Run("новый файл", func(t *testing.T) {
		s, dir := scannedDir(t)
		testkit.JPEG(t, dir, "d.jpg", 40, 30)
		if !changed(t, s, dir) {
			t.Fatal("новый файл не замечен")
		}
	})
	t.Run("несколько новых сразу, в подпапке", func(t *testing.T) {
		s, dir := scannedDir(t)
		sub := filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		testkit.JPEG(t, sub, "d.jpg", 40, 30)
		testkit.JPEG(t, sub, "e.jpg", 40, 30)
		if !changed(t, s, dir) {
			t.Fatal("файлы в подпапке не замечены")
		}
	})
	t.Run("нечитаемый файл", func(t *testing.T) {
		s, dir := scannedDir(t)
		if err := os.WriteFile(filepath.Join(dir, "bad.jpg"), []byte("не картинка"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !changed(t, s, dir) {
			t.Fatal("появление нечитаемого файла не замечено")
		}
	})
	t.Run("не фото не в счёт", func(t *testing.T) {
		s, dir := scannedDir(t)
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if changed(t, s, dir) {
			t.Fatal("текстовый файл изменил отпечаток")
		}
	})
}

// Переименование только регистром (a.jpg → A.jpg) — другой отпечаток: в него идёт путь, а не ключ.
func TestScannerChangedCaseRename(t *testing.T) {
	s, dir := scannedDir(t)
	if err := os.Rename(filepath.Join(dir, "a.jpg"), filepath.Join(dir, "A.jpg")); err != nil {
		t.Fatal(err)
	}
	if !changed(t, s, dir) {
		t.Fatal("переименование a.jpg → A.jpg не замечено")
	}
}

func TestScannerChangedExisting(t *testing.T) {
	t.Run("удалены", func(t *testing.T) {
		s, dir := scannedDir(t)
		if err := os.Remove(filepath.Join(dir, "a.jpg")); err != nil {
			t.Fatal(err)
		}
		if !changed(t, s, dir) {
			t.Fatal("удаление одного не замечено")
		}
		_ = os.Remove(filepath.Join(dir, "b.jpg"))
		_ = os.Remove(filepath.Join(dir, "c.jpg"))
		if !changed(t, s, dir) {
			t.Fatal("удаление всех не замечено")
		}
	})
	t.Run("переименован", func(t *testing.T) {
		s, dir := scannedDir(t)
		if err := os.Rename(filepath.Join(dir, "a.jpg"), filepath.Join(dir, "z.jpg")); err != nil {
			t.Fatal(err)
		}
		if !changed(t, s, dir) {
			t.Fatal("переименование не замечено (число файлов то же)")
		}
	})
	t.Run("изменён размер", func(t *testing.T) {
		s, dir := scannedDir(t)
		p := filepath.Join(dir, "b.jpg")
		old, _ := os.Stat(p)
		testkit.JPEG(t, dir, "b.jpg", 400, 300)
		if st, _ := os.Stat(p); st.Size() == old.Size() {
			t.Fatal("тест: размер не изменился")
		}
		if !changed(t, s, dir) {
			t.Fatal("новый размер не замечен")
		}
	})
	t.Run("изменено время", func(t *testing.T) {
		s, dir := scannedDir(t)
		touch(t, filepath.Join(dir, "c.jpg"), time.Hour)
		if !changed(t, s, dir) {
			t.Fatal("новое время не замечено")
		}
	})
}

func TestScannerChangedSidecar(t *testing.T) {
	t.Run("сайдкар", func(t *testing.T) {
		s, dir := scannedDir(t)
		side := writeSide(t, dir, "a.xmp", `xmp:Rating="1"`)
		if !changed(t, s, dir) {
			t.Fatal("новый сайдкар не замечен")
		}
		// Перечитали с сайдкаром: отпечаток снова актуален.
		if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
			t.Fatal(err)
		}
		if changed(t, s, dir) {
			t.Fatal("после Scan сайдкар ещё changed")
		}
		writeSide(t, dir, "a.xmp", `xmp:Rating="5" xmp:Label="Red"`) // другой размер
		if !changed(t, s, dir) {
			t.Fatal("правка сайдкара (размер) не замечена")
		}
		if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
			t.Fatal(err)
		}
		touch(t, side, time.Hour) // только время
		if !changed(t, s, dir) {
			t.Fatal("правка сайдкара (время) не замечена")
		}
		if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(side); err != nil {
			t.Fatal(err)
		}
		if !changed(t, s, dir) {
			t.Fatal("удаление сайдкара не замечено")
		}
	})
}

func TestScannerChangedScope(t *testing.T) {
	t.Run("другой список папок", func(t *testing.T) {
		s, dir := scannedDir(t)
		other := testkit.TempDir(t) // пустая: файлов столько же, сколько нет, — но список другой
		if !changed(t, s, dir, other) {
			t.Fatal("добавленная папка не замечена")
		}
		if !changed(t, s, other) {
			t.Fatal("другая папка не замечена")
		}
		if !changed(t, s) {
			t.Fatal("пустой список папок не замечен")
		}
	})
	t.Run("Scan отменён", func(t *testing.T) {
		s, dir := scannedDir(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := s.Scan(ctx, Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err == nil {
			t.Fatal("отменённый Scan без ошибки")
		}
		if !changed(t, s, dir) {
			t.Fatal("после отменённого Scan changed = false")
		}
		// Следующий законченный Scan снова даёт актуальный отпечаток.
		if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); err != nil {
			t.Fatal(err)
		}
		if changed(t, s, dir) {
			t.Fatal("после законченного Scan changed")
		}
	})
	t.Run("обход отменён", func(t *testing.T) {
		s, dir := scannedDir(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := s.Changed(ctx, Sources{Folders: []string{dir}}); err == nil {
			t.Fatal("отменённый Changed без ошибки")
		}
		if changed(t, s, dir) {
			t.Fatal("отмена Changed испортила отпечаток")
		}
	})
}

func TestFingerprintOfOrderFree(t *testing.T) {
	a := scanFile{key: "/x/a.jpg", size: 1, mod: 2}
	b := scanFile{key: "/x/b.jpg", size: 3, mod: 4, side: sidecar{path: "/x/b.xmp", size: 5, mod: 6}}
	f1 := fingerprintOf(Sources{Folders: []string{"/x"}}, []scanFile{a, b})
	f2 := fingerprintOf(Sources{Folders: []string{"/x"}}, []scanFile{b, a})
	if f1 != f2 {
		t.Fatalf("порядок меняет отпечаток: %+v %+v", f1, f2)
	}
	if fingerprintOf(Sources{Folders: []string{"/x"}}, nil) == f1 {
		t.Fatal("пустой набор совпал с непустым")
	}
}

// Ключ отпечатка — по правилу paths.Fold: на Linux списки `/x/Весна` и `/x/ВЕСНА` разные
// (соседняя по регистру папка не «без изменений»), на Windows и macOS — одинаковые.
func TestFingerprintKeyFold(t *testing.T) {
	a := fingerprintOf(Sources{Folders: []string{"/x/Весна"}}, nil)
	b := fingerprintOf(Sources{Folders: []string{"/x/ВЕСНА"}}, nil)
	if got := a.key == b.key; got != paths.FoldEqual("/x/Весна", "/x/ВЕСНА") {
		t.Errorf("ключи %q и %q: совпали = %v", a.key, b.key, got)
	}
	if a != fingerprintOf(Sources{Folders: []string{"/x/Весна"}}, nil) {
		t.Error("ключ нестабилен")
	}
}

func photosAt(paths ...string) []Photo {
	out := make([]Photo, len(paths))
	for i, p := range paths {
		out[i] = Photo{Path: p}
	}
	return out
}

func wantPaths(t *testing.T, got []Photo, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("получили %d фото, ждали %v", len(got), want)
	}
	for i := range got {
		if got[i].Path != want[i] {
			t.Fatalf("позиция %d: %s, ждали %s", i, got[i].Path, want[i])
		}
	}
}

func TestMergeNatural(t *testing.T) {
	ph, eq := photosAt, wantPaths
	t.Run("естественный порядок", func(t *testing.T) {
		eq(t, MergeNatural(ph("/a/img-2.jpg", "/a/img-10.jpg"), ph("/a/img-9.jpg", "/b/img-1.jpg")),
			"/a/img-2.jpg", "/a/img-9.jpg", "/a/img-10.jpg", "/b/img-1.jpg")
	})
	t.Run("регистр не важен", func(t *testing.T) {
		eq(t, MergeNatural(ph("/A/b.jpg"), ph("/a/a.jpg", "/a/c.jpg")), "/a/a.jpg", "/A/b.jpg", "/a/c.jpg")
	})
	t.Run("пустые", func(t *testing.T) {
		if got := MergeNatural(nil, nil); len(got) != 0 {
			t.Fatalf("nil+nil: %v", got)
		}
		eq(t, MergeNatural(ph("/a.jpg"), nil), "/a.jpg")
		eq(t, MergeNatural(nil, ph("/a.jpg")), "/a.jpg")
	})
	t.Run("целиком после и до", func(t *testing.T) {
		eq(t, MergeNatural(ph("/a.jpg", "/b.jpg"), ph("/c.jpg", "/d.jpg")), "/a.jpg", "/b.jpg", "/c.jpg", "/d.jpg")
		eq(t, MergeNatural(ph("/c.jpg", "/d.jpg"), ph("/a.jpg", "/b.jpg")), "/a.jpg", "/b.jpg", "/c.jpg", "/d.jpg")
	})
	t.Run("равные пути — сначала a; входы целы", func(t *testing.T) {
		a := []Photo{{Path: "/x.jpg", Name: "a"}}
		b := []Photo{{Path: "/X.jpg", Name: "b"}}
		got := MergeNatural(a, b)
		if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
			t.Fatalf("порядок равных: %+v", got)
		}
		if a[0].Name != "a" || b[0].Name != "b" || len(a) != 1 || len(b) != 1 {
			t.Fatal("входной список изменён")
		}
	})
	t.Run("совпадает с порядком Scan", func(t *testing.T) {
		dir := testkit.TempDir(t)
		for _, n := range []string{"img-1.jpg", "img-2.jpg", "img-9.jpg", "img-10.jpg", "img-11.jpg", "IMG-3.jpg"} {
			testkit.JPEG(t, dir, n, 10, 10)
		}
		all, _ := Scan([]string{dir})
		var a, b []Photo
		for i, p := range all { // раскидываем через один
			if i%2 == 0 {
				a = append(a, p)
			} else {
				b = append(b, p)
			}
		}
		got := MergeNatural(a, b)
		for i := range all {
			if got[i].Path != all[i].Path {
				t.Fatalf("позиция %d: %s, ждали %s", i, got[i].Path, all[i].Path)
			}
		}
	})
}
