package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Корень-ссылка (0.22-sec-1): обход из неё не берёт ни файла — и с разделителем на конце (без
// Clean WalkDir прошёл бы по цели без проверки локальности); её подпапка, добавленная своей
// записью, читается. Договор с paths.Reaches: корень-ссылка ничего не накрывает.
func TestScanLinkedRoot(t *testing.T) {
	target := testkit.TempDir(t)
	testkit.JPEG(t, target, "a.jpg", 20, 10)
	testkit.JPEG(t, filepath.Join(target, "День2"), "b.jpg", 20, 10)
	link := testkit.DirLink(t, target, filepath.Join(testkit.TempDir(t), "Ссылка"))
	sep := string(filepath.Separator)
	for _, root := range []string{link, link + sep} {
		if photos, _ := Scan([]string{root}); len(photos) != 0 {
			t.Errorf("корень-ссылка %q: %d фото, ждали 0", root, len(photos))
		}
	}
	photos, _ := Scan([]string{link, filepath.Join(link, "День2")})
	if len(photos) != 1 || photos[0].Name != "b.jpg" {
		t.Fatalf("подпапка корня-ссылки: %+v, ждали b.jpg", photos)
	}
	want := []Want{{ID: "1", Old: `C:\старое\a.jpg`}}
	got, err := FindByName(context.Background(), link+sep, want, 0)
	if err != nil || len(got.Paths) != 0 {
		t.Fatalf("поиск из корня-ссылки с разделителем: %+v, %v", got, err)
	}
}

// Цепочка ссылок (0.22-sec-4): statLink и LinkTarget проверяют каждое звено по сырой записи —
// A → B (локальная) → UNC отказ, хотя сам A на диске читается; раньше statLink смотрел только
// первое звено и Stat шёл по цепочке в сеть.
func TestLinkTargetChain(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	b := filepath.Join(dir, "b.jpg")
	unc := `\\127.0.0.1\nosuchshare\x.jpg`
	if runtime.GOOS != "windows" {
		unc = "//nosuchhost/x.jpg"
	}
	links := map[string]string{a: b, b: unc}
	t.Cleanup(SetReadLink(func(p string) (string, error) {
		if v, ok := links[p]; ok {
			return v, nil
		}
		return "", os.ErrInvalid
	}))
	if _, ok := statLink(a); ok {
		t.Fatal("цепочка с UNC на втором звене прошла")
	}
	if _, _, err := LinkTarget(a); !errors.Is(err, ErrLinkNotLocal) {
		t.Fatalf("LinkTarget: %v, ждали ErrLinkNotLocal", err)
	}
	if _, _, err := PlainFile(a, ".jpg"); err != nil {
		t.Fatalf("не ссылка по Lstat — PlainFile читает как файл: %v", err) // проверка цепочки — только у ссылок
	}
	links[b] = filepath.Join(dir, "c.jpg") // вся цепочка локальна: решает диск
	if st, ok := statLink(a); !ok || st.Size() == 0 {
		t.Fatalf("локальная цепочка: %v %v", st, ok)
	}
}

// LinkTarget на настоящей папке-ссылке: цель — папка (бросок в окно добавит её как папку),
// statLink — отказ (это не файл); битая ссылка — ошибка диска, а не «нелокальна».
func TestLinkTargetDir(t *testing.T) {
	target := testkit.TempDir(t)
	link := testkit.DirLink(t, target, filepath.Join(testkit.TempDir(t), "Ссылка"))
	if _, st, err := LinkTarget(link); err != nil || !st.IsDir() {
		t.Fatalf("папка-ссылка: %v %v", st, err)
	}
	if _, ok := statLink(link); ok {
		t.Fatal("папка-ссылка прошла как файл")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LinkTarget(link); err == nil || errors.Is(err, ErrLinkNotLocal) {
		t.Fatalf("битая ссылка: %v", err)
	}
}
