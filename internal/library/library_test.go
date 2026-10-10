package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestScanSubfoldersAndSymlinkLoop(t *testing.T) {
	root := testkit.TempDir(t)
	testkit.JPEG(t, root, "a.jpg", 40, 30)
	testkit.JPEG(t, filepath.Join(root, "sub", "deep"), "b.jpg", 30, 40)
	testkit.JPEGOriented(t, root, "rot.jpg", 40, 30, 6)
	testkit.JPEG(t, filepath.Join(root, ".hidden"), "c.jpg", 10, 10)
	bomb := testkit.HugeJPEG(t, root, "bomb.jpg", 30000, 30000)
	// ссылка на корень внутри корня: обход не должен зациклиться (на Windows без прав — пропуск)
	loop := os.Symlink(root, filepath.Join(root, "sub", "loop")) == nil
	if !loop {
		t.Log("симлинк не создался (нет прав): проверка цикла ссылок пропущена, остальное проверяется")
	}

	photos, skipped := Scan([]string{root})
	if len(photos) != 3 {
		t.Fatalf("фото: %d, ждали 3 (loop=%v)", len(photos), loop)
	}
	for _, p := range photos {
		if p.Name == "rot.jpg" && (p.W != 30 || p.H != 40) {
			t.Fatalf("EXIF-поворот не учтён: %dx%d", p.W, p.H)
		}
	}
	if len(skipped) != 1 || skipped[0] != bomb {
		t.Fatalf("skipped=%v", skipped)
	}
}

func TestFileRejectsHugeImage(t *testing.T) {
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, c.Close) // фоновая чистка кэша не должна держать папку
	p := testkit.HugeJPEG(t, dir, "bomb.jpg", 30000, 30000)
	if _, err := c.File(Photo{Path: p, Name: "bomb.jpg"}, false); err == nil || !strings.Contains(err.Error(), "слишком") {
		t.Fatal("ждали отказ для 900 Мп")
	}
	if err := imgfile.CheckSize(testkit.JPEG(t, dir, "ok.jpg", 50, 50)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.File(Photo{Path: filepath.Join(dir, "ok.jpg"), Name: "ok.jpg"}, false); err != nil {
		t.Fatal(err)
	}
}

func TestTrimRemovesOldest(t *testing.T) {
	dir := testkit.TempDir(t)
	c := &Cache{dir: dir}
	now := time.Now()
	for i, n := range []string{"old", "mid", "new"} {
		p := filepath.Join(dir, n+".jpg")
		if err := os.WriteFile(p, make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(time.Duration(i-10) * time.Hour)
		os.Chtimes(p, mt, mt)
	}
	c.Trim(250, 150) // 300 > 250: убрать до ≤150, значит два самых старых
	for n, want := range map[string]bool{"old": false, "mid": false, "new": true} {
		if _, err := os.Stat(filepath.Join(dir, n+".jpg")); (err == nil) != want {
			t.Errorf("%s: есть=%v, ждали %v", n, err == nil, want)
		}
	}
	c.Trim(250, 150) // уже в пределах — ничего не трогает
	if _, err := os.Stat(filepath.Join(dir, "new.jpg")); err != nil {
		t.Error("new удалён")
	}
}

func TestTrimRemovesStaleTmp(t *testing.T) {
	dir := testkit.TempDir(t)
	c := &Cache{dir: dir}
	old, fresh := filepath.Join(dir, "a.jpg.tmp"), filepath.Join(dir, "b.jpg.tmp")
	os.WriteFile(old, []byte("x"), 0o644)
	os.WriteFile(fresh, []byte("x"), 0o644)
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(old, past, past)
	c.Trim(1<<20, 1<<20)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("старый .tmp должен быть удалён")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("свежий .tmp (идущая запись) удалён")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := testkit.TempDir(t)
	p := filepath.Join(dir, "a.jpg")
	if err := WriteFileAtomic(p, []byte("data")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "data" {
		t.Fatal("содержимое не записалось")
	}
	noTmp := func(when string) {
		t.Helper()
		if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) > 0 {
			t.Fatalf(".tmp остался %s: %v", when, left)
		}
	}
	noTmp("после успеха")
	// Цель — каталог: Rename не удастся, .tmp не должен остаться.
	bad := filepath.Join(dir, "d")
	os.MkdirAll(filepath.Join(bad, "x"), 0o755)
	if err := WriteFileAtomic(bad, []byte("data")); err == nil {
		t.Fatal("ждали ошибку записи поверх каталога")
	}
	noTmp("после ошибки")
}

// Имя b.jpg.tmp занято (как призраком 02.JPG.tmp, который антивирус держит открытым после
// удаления 02.jpg): запись всё равно проходит — временное имя у неё своё.
func TestWriteFileAtomicBusyTmpName(t *testing.T) {
	dir := testkit.TempDir(t)
	p := filepath.Join(dir, "b.jpg")
	if err := os.Mkdir(p+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("data")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "data" {
		t.Fatal("содержимое не записалось")
	}
}

// Каталог вместо файла: readHead отклоняет его по дескриптору (OpenRegular — в imgfile).
func TestReadHeadRejectsDir(t *testing.T) {
	if _, _, err := readHead(testkit.TempDir(t)); err == nil {
		t.Error("каталог должен отклоняться")
	}
}
