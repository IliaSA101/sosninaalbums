//go:build windows

package app

// Winqa (150, design §4.1 п. 3, §4.5): облачные заглушки OneDrive и папка OneDrive как точка
// повторной обработки. Junction в конце и середине пути — TestQuietJunction (agentplaces_windows_test.go).
// Локально не запускаются: сборка проверяется GOOS=windows go vet, прогон — в winqa.

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Заглушка OneDrive: файл с атрибутом OFFLINE (RECALL_ON_* ставит только провайдер облака, а OFFLINE
// задаётся SetFileAttributes) — подсчёт видит облако, обычная папка рядом — нет.
func TestQuietOneDrivePlaceholder(t *testing.T) {
	base := testkit.TempDir(t)
	cloud, plain := filepath.Join(base, "Облако"), filepath.Join(base, "Обычная")
	for _, d := range []string{cloud, plain} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		testkit.JPEG(t, d, "a.jpg", 600, 400)
	}
	p, err := syscall.UTF16PtrFromString(filepath.Join(cloud, "a.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_NORMAL|0x1000); err != nil { // OFFLINE
		t.Skipf("атрибут OFFLINE не поставить: %v", err)
	}
	if got := countQuiet(cloud); !got.cloud {
		t.Errorf("заглушка не замечена: %+v", got)
	}
	if got := countQuiet(plain); got.cloud || got.photos != 1 {
		t.Errorf("обычная папка: %+v", got)
	}
}

// Папка OneDrive: Go 1.23+ может отдавать ей ModeIrregular (облачная точка повторной обработки).
// Тест пишет итог Lstat в лог (для design §4.1 п. 3) и проверяет следствие: ссылка в пути — hasLinkPart
// (вопрос link), обычная папка — нет.
func TestQuietOneDriveIrregular(t *testing.T) {
	od := os.Getenv("OneDrive")
	if od == "" {
		t.Skip("OneDrive не установлен на этом ПК")
	}
	st, err := os.Lstat(od)
	if err != nil {
		t.Skipf("папка OneDrive не читается: %v", err)
	}
	link := st.Mode()&(os.ModeIrregular|os.ModeSymlink) != 0
	t.Logf("Lstat(OneDrive): mode=%v irregular=%v symlink=%v", st.Mode(), st.Mode()&os.ModeIrregular != 0, st.Mode()&os.ModeSymlink != 0)
	inside := filepath.Join(od, "Pictures")
	if _, err := os.Lstat(inside); err != nil {
		t.Skipf("в OneDrive нет Pictures: %v", err)
	}
	if got := hasLinkPart(inside); got != link {
		t.Errorf("hasLinkPart(OneDrive\\Pictures)=%v при ссылке в корне OneDrive=%v", got, link)
	}
}
