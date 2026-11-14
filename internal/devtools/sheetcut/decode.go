package sheetcut

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // листы Flux — JPEG
	_ "image/png"  // листы Mystic и Recraft — PNG
	"io"
)

// MaxInputBytes — предел файла листа до чтения: PNG 4K с запасом, больше — не лист.
const MaxInputBytes = 200 << 20

// DecodeSheet читает PNG или JPEG листа. Размер файла и число пикселей проверяются до
// декодирования: заголовок формата может обещать десятки гигапикселей при коротком файле.
// size — размер источника в байтах (из Stat), меньше нуля — неизвестен (проверка только по пикселям).
func DecodeSheet(r io.ReadSeeker, size int64) (image.Image, error) {
	if size > MaxInputBytes {
		return nil, fmt.Errorf("sheetcut: файл %d МБ больше предела %d МБ", size>>20, MaxInputBytes>>20)
	}
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return nil, fmt.Errorf("sheetcut: заголовок картинки: %w", err)
	}
	// Произведение считаем в int64: 32-битный int переполнился бы на заголовке 65535×65535.
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > maxSheetPx {
		return nil, fmt.Errorf("sheetcut: лист %d×%d вне пределов (до %d Мпикс)", cfg.Width, cfg.Height, maxSheetPx/1_000_000)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("sheetcut: источник не перематывается")
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("sheetcut: декодирование: %w", err)
	}
	return img, nil
}
