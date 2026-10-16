package fsfera

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestBookDir — папка книги (047 §4.3): нет — создать (exists=false), есть — та же, выбрана
// сама папка книги (регистр не важен) — без вложенной; файл, ссылка и junction на её месте,
// имя с разделителем — отказ.
func TestBookDir(t *testing.T) {
	parent := testkit.TempDir(t)
	if dir, ok, err := BookDir(parent, "Иванов"); err != nil || ok || dir != filepath.Join(parent, "Иванов") {
		t.Fatalf("нет папки: %q %v %v", dir, ok, err)
	}
	if _, err := os.Stat(filepath.Join(parent, "Иванов")); err == nil {
		t.Fatal("BookDir создал папку: создаёт вызывающий после вопросов")
	}
	book := filepath.Join(parent, "Иванов")
	if err := os.Mkdir(book, 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, ok, err := BookDir(parent, "Иванов"); err != nil || !ok || dir != book {
		t.Fatalf("есть папка: %q %v %v", dir, ok, err)
	}
	if dir, ok, err := BookDir(book, "иванов"); err != nil || !ok || dir != book {
		t.Fatalf("выбрана сама папка книги: %q %v %v", dir, ok, err)
	}
	if err := os.WriteFile(filepath.Join(parent, "Петров"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BookDir(parent, "Петров"); err == nil || !strings.Contains(err.Error(), "файл или ссылка") {
		t.Fatalf("файл на месте папки: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "a/b", `..\x`} {
		if _, _, err := BookDir(parent, bad); err == nil {
			t.Errorf("имя %q принято", bad)
		}
	}
	links := 0
	if err := os.Symlink(book, filepath.Join(parent, "Ссылка")); err == nil {
		links++
		if _, _, err := BookDir(parent, "Ссылка"); err == nil {
			t.Error("символьная ссылка принята за папку книги")
		}
	}
	if runtime.GOOS == "windows" {
		j := filepath.Join(parent, "Junction")
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", j, book).CombinedOutput(); err != nil {
			t.Logf("junction не создан (%v: %s)", err, out)
		} else {
			links++
			if _, _, err := BookDir(parent, "Junction"); err == nil {
				t.Error("junction принят за папку книги")
			}
		}
	}
	if links == 0 {
		t.Log("ни ссылки, ни junction: проверка ссылок пропущена")
	}
}

// TestExtra — посторонние файлы папки книги: свои (из written, регистр не важен, и прежние
// с меткой программы), служебные, скрытые и папки не считаются; остатки записи (в т. ч.
// 00.jpg.N.tmp) — посторонние: загрузчик их возьмёт; first — первый по имени; нет папки — 0.
func TestExtra(t *testing.T) {
	dir := testkit.TempDir(t)
	put := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"00.jpg", "01.JPG", "desktop.ini", "Thumbs.db", ".DS_Store", ".заметка"} {
		put(n)
	}
	if err := os.Mkdir(filepath.Join(dir, "Вложенная"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Прежняя выгрузка: 03.jpg с меткой программы — свой, его удалит экспорт.
	if err := export.Run(t.Context(), []render.Spread{{}, {}, {}}, render.Options{PageW: 10, PageH: 10, DPI: 72, Quality: 80}, dir, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "02.jpg")); err != nil {
		t.Fatal(err)
	}
	written := []string{"00.jpg", "01.jpg"}
	if n, first := Extra(dir, written); n != 0 || first != "" {
		t.Fatalf("только свои и служебные: %d %q", n, first)
	}
	put("notes.txt")
	put("99.jpg") // номер без метки — чужой
	put("Обложка.psd")
	put("05.jpg.123.tmp")
	put("00.jpg.7.tmp")
	if n, first := Extra(dir, written); n != 5 || first != "00.jpg.7.tmp" {
		t.Fatalf("посторонние: %d %q", n, first)
	}
	if n, first := Extra(filepath.Join(dir, "нет"), nil); n != 0 || first != "" {
		t.Fatalf("нет папки: %d %q", n, first)
	}
}

// TestOldCover — свой 00.jpg прежней выгрузки (метка программы) — к удалению; чужой 00.jpg,
// папка на его месте и нет папки — нет.
func TestOldCover(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := OldCover(dir); got != nil {
		t.Fatalf("пустая папка: %q", got)
	}
	if err := export.Run(t.Context(), []render.Spread{{}}, render.Options{PageW: 10, PageH: 10, DPI: 72, Quality: 80}, dir, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	ours, err := os.ReadFile(filepath.Join(dir, "01.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		data []byte
		want int
	}{{[]byte("x"), 0}, {ours, 1}} {
		if err := os.WriteFile(filepath.Join(dir, "00.jpg"), c.data, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := OldCover(dir); len(got) != c.want || c.want == 1 && got[0] != "00.jpg" {
			t.Errorf("00.jpg (%d байт): %q", len(c.data), got)
		}
	}
	sub := filepath.Join(testkit.TempDir(t), "книга")
	if err := os.MkdirAll(filepath.Join(sub, "00.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if OldCover(sub) != nil || OldCover(filepath.Join(dir, "нет")) != nil {
		t.Fatal("папка 00.jpg или нет папки книги")
	}
}
