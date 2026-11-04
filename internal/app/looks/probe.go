package looks

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Экспорт для тестов App: рисование плитки и её шаблоны проверяются из корня, где живёт настоящий
// рендер (Host.Image) и библиотека.

// Tile — шаблоны плитки шага i макета t (lookTile).
func Tile(t model.AlbumTpl, i int, bg, front bool, r float64) ([]model.SpreadTpl, bool) {
	return lookTile(t, i, bg, front, r)
}

// Draw — картинка плитки (PNG или JPEG) без кэша и HTTP.
func (l *Service) Draw(tpls []model.SpreadTpl, px int, r float64, libs map[string]model.PhotoInfo, photos [][]model.PhotoInfo, jpg bool) ([]byte, error) {
	return l.draw(tpls, px, r, libs, photos, jpg)
}

// TileKeys — ключи библиотеки всех шаблонов плитки.
func (l *Service) TileKeys(tpls []model.SpreadTpl) []string { return l.keys(tpls) }

// PhotoPx — большая сторона кадра плитки с фото.
const PhotoPx = lookPhotoPx

// MaxPhotos — предел кандидатов на места листа.
const MaxPhotos = lookMaxPhotos
