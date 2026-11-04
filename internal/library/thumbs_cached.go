package library

import "github.com/IliaSA101/sosninaalbums/internal/imgfile"

// colorKeys — по одному классу цвета на каждый вариант ключа кэша (cacheKey): без профиля, Adobe, P3/ICC.
var colorKeys = [...]imgfile.Color{imgfile.ColorNone, imgfile.ColorAdobe, imgfile.ColorP3}

// Cached — уже готовая копия фото (миниатюра или превью) без генерации: оригинал не открывается и не читается.
// Нужна там, где фото — облачная заглушка (скачивание недопустимо) и цвет по заголовку узнать нельзя: ключ кэша
// зависит от класса цвета, поэтому перебираются все его варианты (p.Color не используется). false — копии нет.
func (c *Cache) Cached(p Photo, preview bool) (string, bool) {
	isPNG := isPNGPath(p.Path)
	for _, col := range colorKeys {
		key, err := cacheKey(p.Path, col)
		if err != nil {
			return "", false
		}
		if f, ok := c.find(key, preview, isPNG); ok {
			return f, true
		}
	}
	return "", false
}
