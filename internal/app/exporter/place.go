package exporter

import (
	"errors"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// outsideAlbum — выбранная папка выгрузки dir вне папок фото альбома (121): внутри — вопрос «выгрузить
// рядом?». Возвращает папку выгрузки out, mk — путь, если её надо создать после всех вопросов (как
// папку книги), ok = false — «Нет» или отказ (ничего не пишем; при отказе err — текст для тоста).
func (s *Service) outsideAlbum(ed *album.Editor, dir string) (out, mk string, ok bool, err error) {
	folders, _ := ed.Sources() // отдельные файлы (097) не проверяем: сканер их папку не обходит
	root, in := export.AlbumFolderOf(dir, folders, paths.Linked)
	if !in {
		return dir, "", true, nil
	}
	next, ok := export.NextToAlbum(root, folders, paths.Linked)
	if !ok {
		return "", "", false, errors.New(export.PlaceRefusal(dir, root))
	}
	next, exists, err := paths.BookDir(filepath.Dir(next), filepath.Base(next), "выгрузки")
	if err != nil {
		return "", "", false, err
	}
	if yes, err := s.h.Ask(export.PlaceQuestion(dir, root, next, exists)); !yes {
		return "", "", false, err
	}
	if exists {
		return next, "", true, nil
	}
	return next, next, true, nil
}
