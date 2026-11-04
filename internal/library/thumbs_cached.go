package library

import (
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// colorKeys — по одному классу цвета на каждый вариант ключа кэша (keyFrom): без профиля, Adobe, P3/ICC. Частные
// (с версией пересчёта в ключе) раньше общего: устаревшая копия под «без профиля» не должна затенить верную.
var colorKeys = [...]imgfile.Color{imgfile.ColorAdobe, imgfile.ColorP3, imgfile.ColorNone}

// Cached — уже готовая копия фото (миниатюра или превью) без генерации: оригинал не открывается и не читается.
// Нужна там, где фото — облачная заглушка (скачивание недопустимо) и цвет по заголовку узнать нельзя: ключ кэша
// зависит от класса цвета, поэтому перебираются все его варианты (p.Color не используется). false — копии нет.
func (c *Cache) Cached(p Photo, preview bool) (string, bool) {
	st, err := os.Stat(p.Path) // Stat не скачивает файл; один на все варианты ключа
	if err != nil {
		return "", false
	}
	isPNG := isPNGPath(p.Path)
	for _, col := range colorKeys {
		if f, ok := c.find(keyFrom(p.Path, st, col), preview, isPNG); ok {
			return f, true
		}
	}
	return "", false
}

// CachedAny — Cached запрошенного размера, а если его нет — копия другого размера (exact = false): для показа заглушки
// облака лучше мелкая картинка, чем серая плитка. Только для раздачи в окне: расчёты (поиск лиц, агент, цвета) берут
// свой размер. false — нет ни миниатюры, ни превью.
func (c *Cache) CachedAny(p Photo, preview bool) (f string, exact, ok bool) {
	if f, ok := c.Cached(p, preview); ok {
		return f, true, true
	}
	f, ok = c.Cached(p, !preview)
	return f, false, ok
}

// stubCopy — фото p лежит в облаке (заглушка OneDrive «только в сети»), а get не нашёл копию по его цвету: оригинал
// открывать нельзя, генерация скачала бы файл. stub = false — файл на диске, генерация разрешена. Иначе — копия под
// любым классом цвета, но строго своего размера, иначе ErrCloudOnly: общий File(p, true) кормит расчёты (лица, цвета,
// агент), подмена 640-пиксельного превью 400-пиксельной миниатюрой дала бы им худший итог, а ключи их памяти после
// скачивания не меняются. Копию другого размера для показа берёт только раздача /media (CachedAny). Отказ в кэш неудач
// не пишется (файл скачают — всё заработает).
func (c *Cache) stubCopy(p Photo, preview bool) (f string, stub bool, err error) {
	st, serr := StatLocal(p.Path)
	if serr != nil || !dehydrated(st) {
		return "", false, nil
	}
	if f, ok := c.Cached(p, preview); ok {
		return f, true, nil
	}
	return "", true, cloudOnlyError()
}
