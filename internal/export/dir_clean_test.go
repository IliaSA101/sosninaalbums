package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пока шёл рендер, наш файл заменили чужим под тем же именем: удаление «хвостов» его не трогает.
func TestRemoveFilesRechecksMarker(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	for _, n := range []string{"05.jpg", "06.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, n), ourData, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ours, _, err := PlanDir(dir)
	if err != nil || len(ours) != 2 {
		t.Fatalf("ours = %v, %v", fileNames(ours), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "05.jpg"), []byte("чужой"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeFiles(dir, fileNames(ours)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "05.jpg")); err != nil {
		t.Fatalf("подменённый 05.jpg удалён: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "06.jpg")); !os.IsNotExist(err) {
		t.Fatalf("наш 06.jpg не удалён: %v", err)
	}
}

// Остатки WriteFileAtomic старше часа удаляются; свежие и чужие .tmp остаются.
func TestRemoveStaleTmp(t *testing.T) {
	dir := testkit.TempDir(t)
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	files := map[string]bool{ // имя → должен удалиться
		"05.jpg.123456.tmp": true, "0012-1.JPG.9.TMP": true,
		"06.jpg.777.tmp": false, // свежий: возможно, пишется сейчас
		"00.jpg.1.tmp":   false, "05.jpg.tmp": false, "05-3.jpg.1.tmp": false,
		"keep.tmp": false, "05.jpg.12a.tmp": false, "5.jpg.1.tmp": false,
		"1-03.jpg.5.tmp": true, "03.jpg.5.tmp": true, // свой остаток: с префиксом плана и без
		"2024-05.jpg.7.tmp": false, // чужой файл: число впереди — не префикс плана
	}
	for n := range files {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if n != "06.jpg.777.tmp" {
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	sub := filepath.Join(dir, "07.jpg.1.tmp") // папка с таким именем — не трогаем
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sub, old, old); err != nil {
		t.Fatal(err)
	}
	removeStaleTmp(dir, now)
	for n, gone := range files {
		_, err := os.Stat(filepath.Join(dir, n))
		if gone != os.IsNotExist(err) {
			t.Errorf("%s: удалён=%v, ждали %v", n, os.IsNotExist(err), gone)
		}
	}
	if _, err := os.Stat(sub); err != nil {
		t.Errorf("папка удалена: %v", err)
	}
}
