//go:build windows

package imgfile

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// readWithin читает файл целиком; зависшее скачивание (колбэк поставщика не ответил) — ошибка, а не вечное ожидание.
func readWithin(t *testing.T, f *os.File) []byte {
	t.Helper()
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := io.ReadAll(f)
		ch <- res{b, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Errorf("чтение заглушки: %v", r.err)
		}
		return r.b
	case <-time.After(testkit.Limit(30 * time.Second)):
		t.Fatal("чтение заглушки зависло: колбэк FETCH_DATA не ответил")
		return nil
	}
}

// cloudPhoto — настоящий маленький JPEG (его байты поставщик отдаёт при скачивании).
func cloudPhoto(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(testkit.ExportJPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestOpenNoReparseCloudStub (160-M1): фото OneDrive «только в сети» — облачная заглушка (точка повторной обработки
// CLOUD_*, Go называет её ModeIrregular). OpenNoReparse и OpenRegular открывают её, чтение скачивает данные через
// поставщика (Cloud Files API, эмуляция testkit.CloudRoot), байты совпадают с оригиналом. Нет фильтра cldflt — пропуск.
func TestOpenNoReparseCloudStub(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	photo := cloudPhoto(t)
	// Блоки независимы: провал одного не прерывает остальные, один прогон собирает все провалы.
	t.Run("заглушка", func(t *testing.T) { checkStubAttrs(t, root, photo) })
	t.Run("OpenNoReparse", func(t *testing.T) { checkOpenNoReparse(t, root, photo) })
	t.Run("OpenRegular", func(t *testing.T) { checkOpenRegular(t, root, photo) })
	t.Run("справка: дескриптор с флагом", func(t *testing.T) { probeFlagged(t, root, photo) })
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}

// checkStubAttrs: созданное эмуляцией — точка повторной обработки с облачным тегом, Go видит ModeIrregular, Lstat данные не скачивает.
func checkStubAttrs(t *testing.T, root *testkit.CloudRoot, photo []byte) {
	p := root.Stub(t, "stub.jpg", photo)
	st, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	attrs := st.Sys().(*syscall.Win32FileAttributeData).FileAttributes
	t.Logf("заглушка: режим %v, атрибуты %#x", st.Mode(), attrs)
	if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		t.Fatal("эмуляция не дала точку повторной обработки: проба не воспроизводит OneDrive")
	}
	if st.Mode().Type() != fs.ModeIrregular {
		t.Errorf("Go видит заглушку как %v, ожидался ModeIrregular (если иначе — защита от ModeIrregular не нужна)", st.Mode().Type())
	}
	if n := root.Fetches(); n != 0 {
		t.Errorf("Lstat скачал заглушку: колбэк вызван %d раз", n)
	}
	h, err := syscall.CreateFile(wstr(t, p), 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatalf("дескриптор заглушки: %v", err)
	}
	tag, ok := reparseTag(h)
	_ = syscall.CloseHandle(h)
	t.Logf("тег точки повторной обработки: %#x", tag)
	if !ok || !isCloudTag(tag) {
		t.Errorf("тег %#x (прочитан=%v) не облачный", tag, ok)
	}
}

// checkOpenNoReparse: заглушка открывается, чтение скачивает данные; скачанная — читается снова без нового скачивания.
func checkOpenNoReparse(t *testing.T, root *testkit.CloudRoot, photo []byte) {
	p := root.Stub(t, "norepars.jpg", photo)
	before := root.Fetches()
	f, err := OpenNoReparse(p)
	if err != nil {
		t.Fatalf("заглушка отклонена: %v", err)
	}
	defer f.Close() //nolint:errcheck // тест: только чтение
	if got := readWithin(t, f); !bytes.Equal(got, photo) {
		t.Errorf("прочитано %d байт, ожидалось %d", len(got), len(photo))
	}
	if root.Fetches() == before {
		t.Error("колбэк FETCH_DATA не вызван: данные не скачивались")
	}
	after := root.Fetches()
	f2, err := OpenNoReparse(p)
	if err != nil {
		t.Fatalf("скачанная заглушка отклонена: %v", err)
	}
	defer f2.Close() //nolint:errcheck // тест: только чтение
	if got := readWithin(t, f2); !bytes.Equal(got, photo) {
		t.Errorf("повторное чтение: %d байт, ожидалось %d", len(got), len(photo))
	}
	if n := root.Fetches(); n != after {
		t.Errorf("повторное чтение скачивало заново: %d вызовов", n-after)
	}
}

// checkOpenRegular: то же для OpenRegular (сканирование папки, DecodeDisk).
func checkOpenRegular(t *testing.T, root *testkit.CloudRoot, photo []byte) {
	p := root.Stub(t, "regular.jpg", photo)
	f, err := OpenRegular(p)
	if err != nil {
		t.Fatalf("заглушка отклонена: %v", err)
	}
	defer f.Close() //nolint:errcheck // тест: только чтение
	if got := readWithin(t, f); !bytes.Equal(got, photo) {
		t.Errorf("прочитано %d байт, ожидалось %d", len(got), len(photo))
	}
}

// probeFlagged — справка для отчёта, не проверка: читает ли дескриптор с FILE_FLAG_OPEN_REPARSE_POINT данные заглушки
// сам (да — переоткрытие reopenData не обязательно; нет — оно и нужно).
func probeFlagged(t *testing.T, root *testkit.CloudRoot, photo []byte) {
	p := root.Stub(t, "flagged.jpg", photo)
	h, err := syscall.CreateFile(wstr(t, p), syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Logf("открытие с флагом: %v", err)
		return
	}
	f := os.NewFile(uintptr(h), p)
	defer f.Close() //nolint:errcheck // тест: только чтение
	got := readWithin(t, f)
	t.Logf("с флагом прочитано %d из %d байт, совпало=%v, скачиваний=%d", len(got), len(photo), bytes.Equal(got, photo), root.Fetches())
}

func wstr(t *testing.T, s string) *uint16 {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestIsCloudTag: облачные теги (CLOUD и CLOUD_1…F) принимаются, ссылки и точка соединения — нет.
func TestIsCloudTag(t *testing.T) {
	for tag, want := range map[uint32]bool{
		0x9000001A: true, 0x9000101A: true, 0x9000601A: true, 0x9000F01A: true,
		0xA000000C: false, // IO_REPARSE_TAG_SYMLINK
		0xA0000003: false, // IO_REPARSE_TAG_MOUNT_POINT
		0x80000023: false, // IO_REPARSE_TAG_AF_UNIX
		0x9000001B: false, 0: false,
	} {
		if got := isCloudTag(tag); got != want {
			t.Errorf("isCloudTag(%#x) = %v, ожидалось %v", tag, got, want)
		}
	}
}
