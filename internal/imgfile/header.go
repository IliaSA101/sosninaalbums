package imgfile

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
)

// Dims — размеры и формат по заголовку (до EXIF-поворота).
type Dims struct {
	W, H   int
	Format string      // "jpeg" | "png"
	Color  color.Model // CMYK без ICC виден здесь
}

// ErrTooBig — площадь по заголовку больше предела: декодировать нельзя.
var ErrTooBig = errors.New("изображение слишком большое")

// tooBigError — ErrTooBig с размерами и пределом в тексте; errors.Is(err, ErrTooBig) работает.
type tooBigError struct {
	w, h  int
	maxPx int64
}

func (e *tooBigError) Error() string {
	return fmt.Sprintf("изображение %dx%d слишком большое (больше %d Мп)", e.w, e.h, e.maxPx/1_000_000)
}

func (e *tooBigError) Unwrap() error { return ErrTooBig }

// pngMagic — подпись PNG; у JPEG достаточно маркера SOI (FF D8).
const pngMagic = "\x89PNG\r\n\x1a\n"

// DimsOf — размеры по заголовку из r: только JPEG и PNG (у других декодеров x/image известные
// уязвимости), стороны > 0, площадь ≤ maxPx (0 — MaxPixels; больше MaxPixels не бывает).
// Больше — ошибка, обёрнутая в ErrTooBig, с размерами. Позиция r после вызова не определена.
func DimsOf(r io.Reader, maxPx int64) (Dims, error) {
	if maxPx <= 0 || maxPx > MaxPixels {
		maxPx = MaxPixels
	}
	// Формат — по магическим байтам, и разбирает заголовок только свой декодер: image.DecodeConfig
	// пробовал бы все зарегистрированные (imaging подключает TIFF/BMP/GIF), а нужны два.
	var magic [len(pngMagic)]byte
	k, err := io.ReadFull(r, magic[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Dims{}, err // сбой чтения — не «формат не тот»
	}
	r = io.MultiReader(bytes.NewReader(magic[:k]), r)
	var (
		cfg    image.Config
		format string
	)
	switch {
	case k >= 2 && magic[0] == 0xFF && magic[1] == 0xD8:
		cfg, err = jpeg.DecodeConfig(r)
		format = "jpeg"
	case k == len(pngMagic) && string(magic[:]) == pngMagic:
		cfg, err = png.DecodeConfig(r)
		format = "png"
	default:
		return Dims{}, errors.New("формат не поддерживается (нужен JPEG или PNG)")
	}
	if err != nil {
		return Dims{}, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Dims{}, fmt.Errorf("некорректный размер изображения %dx%d", cfg.Width, cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPx {
		return Dims{}, &tooBigError{w: cfg.Width, h: cfg.Height, maxPx: maxPx}
	}
	return Dims{W: cfg.Width, H: cfg.Height, Format: format, Color: cfg.ColorModel}, nil
}

// FileDims — DimsOf обычного файла (OpenRegular): устройство и каталог не читает, ссылку проходит
// по цели; проверку цели делает вызывающий (library.PlainFile).
func FileDims(path string, maxPx int64) (Dims, error) {
	f, err := OpenRegular(path)
	if err != nil {
		return Dims{}, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	return DimsOf(f, maxPx)
}
