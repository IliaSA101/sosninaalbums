package app

// Сцена кадра пипетки (спека 109 §4, решение 109-g3): картинки из превью, недоступные — убраны.

import (
	"context"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// pickPreviews готовит сцену к кадру: фото, фоны и фото-стикеры рисуются из превью 2048 px кэша
// миниатюр (оно в sRGB и без EXIF-поворота). Картинка, чьё превью или файл не открылись, и
// рамочка-картинка без файла убираются: render на пропавшем пути бросает весь холст, а пипетка
// не должна падать из-за одного файла.
func (a *App) pickPreviews(ctx context.Context, refs map[string]model.PhotoRef, es *render.Spread) {
	byPath := make(map[string]string, len(refs)) // путь оригинала → ID фото
	for id, r := range refs {
		if old, ok := byPath[r.Path]; !ok || id < old {
			byPath[r.Path] = id // от порядка карты результат не зависит
		}
	}
	done := map[string]string{} // путь оригинала → путь превью; "" — не открылось
	preview := func(path string) string {
		if got, ok := done[path]; ok {
			return got
		}
		file := ""
		if id, ok := byPath[path]; ok && a.cache != nil {
			if p, ok, gone := a.mediaPhoto(id); ok && !gone {
				file, _ = a.cache.FileCtx(ctx, p, true) // ошибка — файл пропал или не читается
			}
		}
		if file == "" {
			logRare(a.pick.skipped.Add(1), "пипетка: картинка не открылась, в кадре её нет")
		}
		done[path] = file
		return file
	}
	swap := func(list []render.Photo) []render.Photo {
		for i := range list {
			list[i].Path = preview(list[i].Path)
		}
		return slices.DeleteFunc(list, func(ph render.Photo) bool { return ph.Path == "" })
	}
	es.Photos = swap(es.Photos)
	es.Stickers = swap(es.Stickers)
	for i := range es.Backdrops {
		es.Backdrops[i].Path = preview(es.Backdrops[i].Path)
	}
	es.Backdrops = slices.DeleteFunc(es.Backdrops, func(b render.Backdrop) bool { return b.Path == "" })
	// Рамочки-картинки и окна по ним render открывает по файлу, как в экспорте: нет файла — нет рамочки.
	fileOK := fileProbe()
	es.Borders = slices.DeleteFunc(es.Borders, func(b render.BorderDraw) bool { return b.Pic && !fileOK(b.Path) })
	for _, list := range [][]render.Photo{es.Photos, es.Stickers} {
		for i := range list {
			if w := list[i].Win; w != nil && w.Path != "" && !fileOK(w.Path) {
				list[i].Win = nil
			}
		}
	}
}

// fileProbe — проверка файла рамочки с памятью по пути: том и Stat по разу на путь.
func fileProbe() func(path string) bool {
	ok := map[string]bool{}
	return func(path string) bool {
		v, seen := ok[path]
		if !seen {
			// reachable первым: Stat по отвалившемуся сетевому тому висит до таймаута SMB.
			v = path != "" && reachable(path)
			if v {
				_, err := library.StatLocal(path)
				v = err == nil
			}
			ok[path] = v
		}
		return v
	}
}
