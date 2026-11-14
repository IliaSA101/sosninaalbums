package export

import (
	"fmt"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// placeSuffix — хвост имени соседней папки выгрузки: «ФА25 — выгрузка».
const placeSuffix = " — выгрузка"

// AlbumFolderOf — папка фото альбома, в которую ляжет выгрузка в dir: dir — она сама (DirKey) или сканер
// дойдёт до dir из неё (paths.Cover.Root, 093-4). in = false — вне папок альбома: папка-родитель папки
// фото, подпапка на «.» или за ссылкой (сканер туда не заходит) вопроса не требуют.
func AlbumFolderOf(dir string, folders []string, linked func(string) bool) (root string, in bool) {
	key := paths.DirKey(dir)
	for _, f := range folders {
		if paths.DirKey(f) == key {
			return f, true
		}
	}
	return paths.NewCover(folders, linked).Root(dir, false)
}

// NextToAlbum — соседняя папка для выгрузки: <родитель root>/<имя root> — выгрузка. ok = false — у root
// нет родителя (корень диска) или соседняя сама в папках альбома (иначе вопрос повторился бы).
// Файл или ссылка на её месте здесь не видны: их ловит paths.BookDir вызывающего.
func NextToAlbum(root string, folders []string, linked func(string) bool) (dir string, ok bool) {
	root = filepath.Clean(root)
	parent := filepath.Dir(root)
	if parent == root || filepath.Base(root) == string(filepath.Separator) {
		return "", false
	}
	dir = filepath.Join(parent, filepath.Base(root)+placeSuffix)
	if _, in := AlbumFolderOf(dir, folders, linked); in {
		return "", false
	}
	return dir, true
}

// placeHead — начало вопроса и отказа: сама папка фото или подпапка (сц. 1–2). Только имена, без путей.
func placeHead(chosen, root string) string {
	name, rootName := dirName(chosen), dirName(root)
	if paths.DirKey(chosen) == paths.DirKey(root) {
		return fmt.Sprintf("Папка «%s» — папка фото этого альбома: развороты смешаются с фото.", name)
	}
	return fmt.Sprintf("Папка «%s» лежит в папке фото этого альбома «%s»: развороты смешаются с фото.", name, rootName)
}

// PlaceQuestion — вопрос «выгрузить рядом?» (сц. 1–3): chosen — выбранная папка, root — папка фото,
// next — соседняя, exists — она уже есть (тогда без слова «новую»).
func PlaceQuestion(chosen, root, next string, exists bool) string {
	if exists {
		return fmt.Sprintf("%s Выгрузить в папку «%s» рядом?", placeHead(chosen, root), dirName(next))
	}
	return fmt.Sprintf("%s Выгрузить рядом, в новую папку «%s»?", placeHead(chosen, root), dirName(next))
}

// PlaceRefusal — отказ, когда соседней быть не может (сц. 5).
func PlaceRefusal(chosen, root string) string {
	return placeHead(chosen, root) + " Выберите другую папку."
}

// dirName — имя папки для текста; у корня диска (Base — разделитель) — сам путь.
func dirName(p string) string {
	p = filepath.Clean(p)
	if b := filepath.Base(p); b != string(filepath.Separator) && b != "." {
		return b
	}
	return p
}
