package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Продолжение statlocal_test.go: места internal/app, где os.Stat шёл по ссылке на сетевой путь
// (спека 110, Т6). В каждом тесте контроль — ссылка с локальной целью работает как прежде.

// RelinkFolderPath: ссылка на сетевой путь вместо папки — «не папка», проект не меняется.
func TestRelinkFolderNetLink(t *testing.T) {
	a, moved, _ := relinkApp(t)
	a.rescan()
	waitScan(t, a)
	rev := a.editor().View().Rev
	if _, err := a.RelinkFolderPath(netLinkNamed(t, testkit.TempDir(t), "папка", moved, true)); err == nil || !strings.Contains(err.Error(), "не папка") {
		t.Fatalf("ссылка на сетевой путь: err %v, ждали «не папка»", err)
	}
	if a.editor().View().Rev != rev {
		t.Error("отказ изменил проект")
	}
	// Контроль: ссылка на локальную папку принята (файлы внутри обход по ссылке не берёт — это не Stat).
	if _, err := a.RelinkFolderPath(netLinkNamed(t, testkit.TempDir(t), "папка", moved, false)); err != nil {
		t.Fatalf("контроль: ссылка на локальную папку: %v", err)
	}
}

// AddPackPath: ссылка на сетевой путь не доходит до pack.Open — «файл набора не найден».
func TestAddPackPathNetLink(t *testing.T) {
	a := decorApp(t)
	zip := d1Zip(t)
	_, err := a.AddPackPath(netLinkNamed(t, testkit.TempDir(t), "ivory.zip", zip, true))
	if err == nil || !strings.Contains(err.Error(), "файл набора не найден") {
		t.Fatalf("ссылка на сетевой путь: %v, ждали «файл набора не найден»", err)
	}
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
		t.Fatalf("набор поставился: %+v", list.Packs)
	}
	res, err := a.AddPackPath(netLinkNamed(t, testkit.TempDir(t), "ivory.zip", zip, false))
	if err != nil || res.Pack.ID != "ivory" {
		t.Fatalf("контроль: ссылка на локальный набор: %v %+v", err, res)
	}
}

// sameFile: ссылка на сетевой путь — не тот же файл, хотя настоящая цель на диске есть;
// ссылка на локальный файл — тот же, разные файлы и отсутствующий — нет.
func TestSameFileNetLink(t *testing.T) {
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	other := testkit.JPEG(t, dir, "b.jpg", 10, 10)
	if !sameFile(file, netLink(t, dir, file, false)) {
		t.Fatal("контроль: ссылка на локальный файл — не тот же файл")
	}
	if sameFile(other, file) {
		t.Error("разные файлы приняты за один")
	}
	if sameFile(file, filepath.Join(dir, "нет.jpg")) {
		t.Error("нет файла — не тот же")
	}
	dir2 := testkit.TempDir(t)
	file2 := testkit.JPEG(t, dir2, "a.jpg", 10, 10)
	link := netLink(t, dir2, file2, true)
	if sameFile(file2, link) || sameFile(link, file2) {
		t.Error("ссылка на сетевой путь принята за тот же файл")
	}
}

// SuggestDrive: путь на кандидате в диски — ссылка на сетевой путь — фото не находит. Буквы
// дисков есть только на Windows, где ссылок нет, поэтому проверяется сама проба driveHas.
func TestSuggestDriveNetLink(t *testing.T) {
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "a.jpg", 10, 10)
	if !driveHas(file) || !driveHas(netLink(t, dir, file, false)) {
		t.Fatal("контроль: файл и ссылка на локальный файл не найдены")
	}
	if driveHas(filepath.Join(dir, "нет.jpg")) {
		t.Error("нет файла — найден")
	}
	dir2 := testkit.TempDir(t)
	file2 := testkit.JPEG(t, dir2, "a.jpg", 10, 10)
	if driveHas(netLink(t, dir2, file2, true)) {
		t.Error("ссылка на сетевой путь принята за найденное фото")
	}
}
