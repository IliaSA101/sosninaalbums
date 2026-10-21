package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// addLoose берёт отдельный файл ленты (097) и его сайдкар .xmp. Порядок проверок как у броска в
// окно: локальный путь — до любого обращения к диску (UNC иначе вешал бы сетевым запросом), потом
// имя без устройства, расширение, обычный файл; ссылка — только с локальной целью (statLink).
// Всё мимо — файл молча пропускается: его нет в ленте, а проект сам его не теряет.
func (w *scanWalk) addLoose(p string) {
	name := filepath.Base(p)
	if paths.CheckLocalPath(p) != nil || paths.DeviceElem(name) || !isImage(name) {
		return
	}
	info, err := os.Lstat(p)
	if err != nil {
		return
	}
	if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		var ok bool
		if info, ok = statLink(p); !ok {
			return
		}
	} else if !info.Mode().IsRegular() {
		return
	}
	w.add(p, name, info, nil)
	w.looseSidecar(p)
}

// looseSidecar ищет рядом с отдельным файлом `имя.расш.xmp` и `имя.xmp`. Имена фиксированы и лежат
// в папке, выбранной пользователем, — это не путь, угаданный по содержимому XMP. Берётся только
// обычный файл (ссылка вместо сайдкара не читается, как в обходе папки).
func (w *scanWalk) looseSidecar(p string) {
	for _, sp := range [2]string{p + ".xmp", strings.TrimSuffix(p, filepath.Ext(p)) + ".xmp"} {
		if len(w.sides.m) >= maxSidecars {
			return
		}
		if info, err := os.Lstat(sp); err == nil && info.Mode().IsRegular() {
			w.sides.put(sp, info)
		}
	}
}

// StatLink — сведения о файле, на который указывает ссылка p; ok = false для папок, битых ссылок и
// ссылок на нелокальный путь. Для отдельных фото из окна (097): те же правила, что у обхода папки.
func StatLink(p string) (fs.FileInfo, bool) { return statLink(p) }
