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

	_ "golang.org/x/image/webp" // регистрация декодера: imaging.Decode в OpenOrientedCtx узнаёт WebP
)

// Dims — размеры и формат по заголовку (до EXIF-поворота).
type Dims struct {
	W, H   int
	Format string      // "jpeg" | "png" | "webp"
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

// pngMagic — подпись PNG; у JPEG достаточно маркера SOI (FF D8); WebP — RIFF????WEBP (12 байт).
const (
	pngMagic  = "\x89PNG\r\n\x1a\n"
	magicLen  = 12
	riffMagic = "RIFF"
	webpMagic = "WEBP"
	// Флаги в первом байте данных чанка VP8X.
	vp8xAnimation = 0x02
	vp8xICC       = 0x20
	vp8xAlpha     = 0x10
)

// DimsOf — размеры по заголовку из r: только JPEG, PNG и WebP (GIF, BMP и TIFF под .jpg — отказ:
// миниатюра и экспорт их не прочтут), стороны > 0, площадь ≤ maxPx (0 — MaxPixels; больше MaxPixels не бывает).
// Больше — ошибка, обёрнутая в ErrTooBig, с размерами. Позиция r после вызова не определена.
func DimsOf(r io.Reader, maxPx int64) (Dims, error) {
	if maxPx <= 0 || maxPx > MaxPixels {
		maxPx = MaxPixels
	}
	// Формат — по магическим байтам, и разбирает заголовок только свой декодер: image.DecodeConfig
	// пробовал бы все зарегистрированные (imaging подключает TIFF/BMP/GIF), а нужны три.
	var magic [magicLen]byte
	k, err := io.ReadFull(r, magic[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Dims{}, err // сбой чтения — не «формат не тот»
	}
	src := r
	r = io.MultiReader(bytes.NewReader(magic[:k]), r)
	var (
		cfg    image.Config
		format string
	)
	switch {
	case k >= 2 && magic[0] == 0xFF && magic[1] == 0xD8:
		cfg, err = jpeg.DecodeConfig(r)
		format = "jpeg"
	case k >= len(pngMagic) && string(magic[:len(pngMagic)]) == pngMagic:
		cfg, err = png.DecodeConfig(r)
		format = "png"
	case k == magicLen && string(magic[:4]) == riffMagic && string(magic[8:]) == webpMagic:
		// Чанки обходятся вручную до кадра: анимация, расхождение кадра с холстом и VP8X без кадра — отказ;
		// модель (lossy / с альфой / lossless) нужна весу декода. Читается исходный r — он сразу за RIFF.
		var wh webpHead
		wh, err = readWebPHead(src)
		cfg = image.Config{ColorModel: wh.model, Width: wh.w, Height: wh.h}
		format = "webp"
	default:
		return Dims{}, errors.New("формат не поддерживается (нужен JPEG, PNG или WebP)")
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
