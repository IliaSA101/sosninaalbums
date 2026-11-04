package app

import (
	"context"
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/app/colorpick"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Чтение фото для ИИ-агентов, контактного листа, поиска лиц и цветов (160-M1, хвост): фото в облаке (заглушка OneDrive
// «только в сети») оригинал не открывает — скачивание тихим путём недопустимо. Берётся готовая копия кэша миниатюр,
// а без неё — library.ErrCloudOnly (причина — library.WhyCloudOnly) и запись «в облаке» (noteCloud), чтобы плитка и
// list_photos показали её, а не «нет файла».

// photoCopy — файл копии (preview — превью, иначе миниатюра) фото p с ID id. cloud — фото проекта в облаке
// (mediaPhotoCloud): только готовая копия. Иначе зовётся get; его ErrCloudOnly (фото ленты стало заглушкой после
// сканирования, Cache.stubCopy) — тот же поиск готовой копии. anySize — годится копия другого размера: для картинок,
// которые смотрят (лист, превью агента); расчёты с памятью по «путь/размер/время» (лица, цвета) берут только свой.
// Копии нет — library.ErrCloudOnly и noteCloud(id). Оригинал не открывается.
func (a *App) photoCopy(id string, p library.Photo, cloud, preview, anySize bool, get func() (string, error)) (string, error) {
	if a.cache == nil {
		return "", errNoCache
	}
	if !cloud {
		f, err := get()
		if !errors.Is(err, library.ErrCloudOnly) {
			return f, err
		}
	}
	var f string
	var ok bool
	if anySize {
		f, _, ok = a.cache.CachedAny(p, preview)
	} else {
		f, ok = a.cache.Cached(p, preview)
	}
	if !ok {
		// Запись «только в облаке» — когда нет ни одной копии (как cloudNoCopy); копия другого размера есть — отказ без неё.
		if _, _, has := a.cache.CachedAny(p, preview); !has {
			a.noteCloud(id)
		}
		return "", library.ErrCloudOnly
	}
	return f, nil
}

// photoCopyCtx — photoCopy с общим получением копии кэша: FileCtx (холст, расчёты).
func (a *App) photoCopyCtx(ctx context.Context, id string, p library.Photo, cloud, preview, anySize bool) (string, error) {
	return a.photoCopy(id, p, cloud, preview, anySize, func() (string, error) { return a.cache.FileCtx(ctx, p, preview) })
}

// colorCopy — Host.Copy окна цвета: пипетка и «С фото» берут копию фото id своего размера (расчёт с памятью, чужой
// не годится), лист для агента (Sheet) — anySize. Нет фото или файла — colorpick.ErrNoPhoto (окно пропускает такое фото).
func (a *App) colorCopy(ctx context.Context, id string, preview, anySize bool) (string, error) {
	p, ok, gone, cloud := a.mediaPhotoCloud(id)
	if !ok || gone {
		return "", colorpick.ErrNoPhoto
	}
	return a.photoCopyCtx(ctx, id, p, cloud, preview, anySize)
}
