//go:build windows

package imgfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// attrInfo — сведения о файле с заданными атрибутами Windows в Sys() (как их кладёт Go).
type attrInfo struct {
	fs.FileInfo
	attrs uint32
}

func (a attrInfo) Sys() any { return &syscall.Win32FileAttributeData{FileAttributes: a.attrs} }

// foreignInfo — сведения без атрибутов Windows в Sys() (подмена типа: fileAttrs отдаёт 0).
type foreignInfo struct{ fs.FileInfo }

func (foreignInfo) Sys() any { return 42 }

// TestDehydratedAttrs: Dehydrated читает атрибуты Windows из Sys(): «не на диске», «открытие скачает», «чтение скачает» —
// облачные; обычный файл, чужой тип Sys() и nil — нет. Блоки независимы.
func TestDehydratedAttrs(t *testing.T) {
	p := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 8, 8)
	st, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		info fs.FileInfo
		want bool
	}{
		"offline":          {attrInfo{st, attrOffline}, true},
		"recall on open":   {attrInfo{st, attrRecallOpen}, true},
		"recall on access": {attrInfo{st, attrRecallAccess}, true},
		"с прочими битами": {attrInfo{st, syscall.FILE_ATTRIBUTE_ARCHIVE | attrRecallAccess}, true},
		"обычные":          {attrInfo{st, syscall.FILE_ATTRIBUTE_ARCHIVE | syscall.FILE_ATTRIBUTE_READONLY}, false},
		"без атрибутов":    {attrInfo{st, 0}, false},
		"чужой Sys()":      {foreignInfo{st}, false},
		"nil":              {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Dehydrated(tc.info); got != tc.want {
				t.Errorf("Dehydrated = %v, ожидалось %v", got, tc.want)
			}
		})
	}
}

// TestHandleHelpersEdges: reparseTag и fileID на негодном дескрипторе и на обычном файле — отказ без паники; один и тот же
// файл по двум дескрипторам даёт один том и индекс. Блоки независимы.
func TestHandleHelpersEdges(t *testing.T) {
	p := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 8, 8)
	t.Run("reparseTag: негодный дескриптор", func(t *testing.T) {
		if tag, ok := reparseTag(syscall.InvalidHandle); ok {
			t.Errorf("тег %#x у негодного дескриптора", tag)
		}
	})
	t.Run("reparseTag: обычный файл", func(t *testing.T) {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() //nolint:errcheck // тест: только чтение
		if tag, ok := reparseTag(syscall.Handle(f.Fd())); ok {
			t.Errorf("тег %#x у обычного файла", tag)
		}
	})
	t.Run("fileID: негодный дескриптор", func(t *testing.T) {
		if _, _, ok := fileID(syscall.InvalidHandle); ok {
			t.Error("fileID вернул ok для негодного дескриптора")
		}
	})
	t.Run("fileID: один файл — один индекс", func(t *testing.T) {
		f1, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f1.Close() //nolint:errcheck // тест: только чтение
		f2, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f2.Close() //nolint:errcheck // тест: только чтение
		v1, i1, ok1 := fileID(syscall.Handle(f1.Fd()))
		v2, i2, ok2 := fileID(syscall.Handle(f2.Fd()))
		if !ok1 || !ok2 || v1 != v2 || i1 != i2 || i1 == 0 {
			t.Errorf("fileID: (%d,%d,%v) и (%d,%d,%v)", v1, i1, ok1, v2, i2, ok2)
		}
	})
}

// TestReopenDataEdges: переоткрытие отказывает, когда дескриптор — не заглушка. Обычный файл — ErrChanged (тег не облачный:
// это уже другой объект), дескриптор закрыт; не файл (канал) — ErrCloudUnavailable, если Windows не переоткрыла канал.
func TestReopenDataEdges(t *testing.T) {
	t.Run("обычный файл", func(t *testing.T) {
		p := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 8, 8)
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		g, err := reopenData(f, p)
		if g != nil {
			_ = g.Close()
		}
		if !errors.Is(err, ErrChanged) || g != nil {
			t.Errorf("reopenData обычного файла: %v, ожидалась ErrChanged", err)
		}
		if _, serr := f.Stat(); serr == nil {
			t.Error("исходный дескриптор не закрыт")
		}
	})
	t.Run("не файл", func(t *testing.T) {
		var r, w syscall.Handle
		if err := syscall.CreatePipe(&r, &w, nil, 0); err != nil {
			t.Skipf("канал не создался: %v", err)
		}
		defer syscall.CloseHandle(w) //nolint:errcheck // тест
		f := os.NewFile(uintptr(r), "pipe")
		if f == nil {
			_ = syscall.CloseHandle(r)
			t.Skip("os.NewFile не принял канал")
		}
		g, err := reopenData(f, "pipe")
		if g != nil {
			_ = g.Close()
		}
		var pe *os.PathError
		switch {
		case errors.Is(err, ErrCloudUnavailable):
			if !errors.As(err, &pe) || pe.Op != "open" {
				t.Errorf("ошибка %v: ждали PathError open", err)
			}
		case errors.Is(err, ErrChanged):
			t.Skip("Windows переоткрыла канал: ветка отказа ReOpenFile на этой системе недостижима")
		default:
			t.Errorf("reopenData канала: %v", err)
		}
	})
}

// TestCloudStubPathEdges: cloudStubPath отказывает без паники и без обращения к сети — ошибка EvalSymlinks, итог вне
// локальных путей, итог не существует, обычный файл (тега нет). Разворот ссылок подменён швом evalSymlinks.
func TestCloudStubPathEdges(t *testing.T) {
	dir := testkit.TempDir(t)
	plain := testkit.ExportJPEG(t, dir, "a.jpg", 8, 8)
	withEval := func(t *testing.T, f func(string) (string, error)) {
		t.Helper()
		old := evalSymlinks
		evalSymlinks = f
		t.Cleanup(func() { evalSymlinks = old })
	}
	t.Run("EvalSymlinks отказал", func(t *testing.T) {
		withEval(t, func(string) (string, error) { return "", fs.ErrNotExist })
		if cloudStubPath(plain) {
			t.Error("признана заглушкой при ошибке разворота")
		}
	})
	t.Run("итог вне локальных путей", func(t *testing.T) {
		withEval(t, func(string) (string, error) { return `\\127.0.0.1\нет\a.jpg`, nil })
		if cloudStubPath(plain) {
			t.Error("итог на сетевом пути признан заглушкой")
		}
	})
	t.Run("итога нет на диске", func(t *testing.T) {
		withEval(t, func(string) (string, error) { return filepath.Join(dir, "нет", "a.jpg"), nil })
		if cloudStubPath(plain) {
			t.Error("несуществующий итог признан заглушкой")
		}
	})
	t.Run("обычный файл", func(t *testing.T) {
		if cloudStubPath(plain) {
			t.Error("обычный файл признан заглушкой")
		}
	})
	t.Run("несуществующий путь", func(t *testing.T) {
		if cloudStubPath(filepath.Join(dir, "нет.jpg")) {
			t.Error("несуществующий файл признан заглушкой")
		}
	})
}

// TestOpenNoReparseBadName: имя с нулевым байтом отклоняется уже при переводе в UTF-16 (внутренний openNoReparse без
// проверки пути OpenNoReparse); негодный дескриптор не выдаётся.
func TestOpenNoReparseBadName(t *testing.T) {
	p := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 8, 8)
	if f, err := openNoReparse(p + "\x00x"); err == nil {
		_ = f.Close()
		t.Error("имя с нулевым байтом открылось")
	}
}
