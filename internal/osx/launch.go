package osx

import (
	"path/filepath"
	"strings"
)

// Place — откуда запущена программа, если это место значит «не установлена как следует».
type Place string

const (
	PlaceNormal       Place = ""                  // обычная папка
	PlaceTemp         Place = "temp"              // временная папка: запуск из архива (Проводник, 7-Zip, WinRAR)
	PlaceTranslocated Place = "app-translocation" // macOS: программа не перенесена из «Загрузок»
)

// LaunchPlace определяет место запуска по пути exe и временной папке tempDir. Диск читает
// только longPath (Windows: короткие имена 8.3 → длинные). Пустые строки — «обычное место».
func LaunchPlace(exe, tempDir string) Place {
	if exe == "" {
		return PlaceNormal
	}
	exe = filepath.Clean(exe)
	if strings.Contains(strings.ToLower(filepath.ToSlash(exe)), "/apptranslocation/") {
		return PlaceTranslocated
	}
	if tempDir == "" {
		return PlaceNormal
	}
	if inside(longPath(filepath.Dir(exe)), longPath(filepath.Clean(tempDir))) {
		return PlaceTemp
	}
	return PlaceNormal
}

// inside — dir совпадает с root или лежит внутри; без учёта регистра (NTFS и APFS по умолчанию
// нечувствительны), границу ищем по разделителю, чтобы «Temp2» не считался внутри «Temp».
func inside(dir, root string) bool {
	if root == "" || filepath.Dir(root) == root { // корень диска — не «временная папка»
		return false
	}
	dir, root = strings.ToLower(dir), strings.ToLower(strings.TrimRight(root, `\/`))
	return dir == root || strings.HasPrefix(dir, root+string(filepath.Separator))
}

// Тексты тоста при запуске не из того места (спека 039, решение 9).
const (
	noticeTemp         = "Программа запущена прямо из архива. Закройте её и распакуйте архив: «Извлечь все», затем запускайте SosninaAlbums.exe из распакованной папки."
	noticeTranslocated = "Программа запущена без переноса из «Загрузок». Закройте её, перетащите sosninaalbums в «Программы» и откройте оттуда."
)

// Notice — текст для пользователя; для обычного места пустой.
func (p Place) Notice() string {
	switch p {
	case PlaceTemp:
		return noticeTemp
	case PlaceTranslocated:
		return noticeTranslocated
	case PlaceNormal:
	}
	return ""
}
