//go:build windows

package imgfile

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fsctlSetReparsePoint — FSCTL_SET_REPARSE_POINT.
const fsctlSetReparsePoint = 0x900A4

// foreignTagFile — пустой файл с точкой повторной обработки чужого (не облачного) тега: такой же ModeIrregular у Go,
// как у заглушки OneDrive, но файлом он быть не должен. Не поставилась — пропуск с причиной.
func foreignTagFile(t *testing.T, tag uint32) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), "foreign.jpg")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(wstr(t, p), syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatalf("открытие для точки повторной обработки: %v", err)
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // тест
	buf := make([]byte, 8)       // заголовок REPARSE_DATA_BUFFER без данных: тег, длина 0, резерв
	binary.LittleEndian.PutUint32(buf, tag)
	var n uint32
	if err := syscall.DeviceIoControl(h, fsctlSetReparsePoint, &buf[0], uint32(len(buf)), nil, 0, &n, nil); err != nil {
		t.Skipf("чужой тег %#x не поставился: %v", tag, err)
	}
	return p
}

// TestRegularOrCloudWindows: заглушка Cloud Files API — файл (и Dehydrated, без скачивания); обычный файл — да;
// точка соединения и точка повторной обработки со сторонним тегом — нет, и OpenNoReparse их по-прежнему отклоняет.
func TestRegularOrCloudWindows(t *testing.T) {
	// Блоки независимы: провал одного не прерывает остальные.
	t.Run("заглушка", func(t *testing.T) {
		root := testkit.NewCloudRoot(t)
		p := root.Stub(t, "stub.jpg", cloudPhoto(t))
		st, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !RegularOrCloud(p, st) {
			t.Error("облачная заглушка не признана файлом")
		}
		if !Dehydrated(st) {
			t.Errorf("заглушка без данных не признана облачной по атрибутам %#x", fileAttrs(st))
		}
		if n := root.Fetches(); n != 0 {
			t.Errorf("проверка предиката скачала файл: колбэк вызван %d раз", n)
		}
	})
	t.Run("обычный файл", func(t *testing.T) {
		p := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 16, 16)
		st, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !RegularOrCloud(p, st) || Dehydrated(st) {
			t.Error("обычный файл: должен быть файлом и не облачным")
		}
	})
	t.Run("точка соединения", func(t *testing.T) {
		link := testkit.Junction(t, testkit.TempDir(t), filepath.Join(testkit.TempDir(t), "j.jpg"))
		st, err := os.Lstat(link)
		if err != nil {
			t.Fatal(err)
		}
		if RegularOrCloud(link, st) {
			t.Error("точка соединения признана файлом")
		}
	})
	t.Run("сторонний тег", func(t *testing.T) {
		p := foreignTagFile(t, 0x80000099)
		st, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("режим %v, тип %v", st.Mode(), st.Mode().Type())
		if RegularOrCloud(p, st) {
			t.Error("точка повторной обработки со сторонним тегом признана файлом")
		}
		if f, err := OpenNoReparse(p); err == nil {
			_ = f.Close()
			t.Error("OpenNoReparse открыл файл со сторонним тегом")
		}
		if f, err := OpenRegular(p); err == nil {
			_ = f.Close()
			t.Error("OpenRegular открыл файл со сторонним тегом")
		}
	})
}
