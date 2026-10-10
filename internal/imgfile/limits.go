package imgfile

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
)

// MaxPixels — предел размера оригинала. Полный декод 250 Мп — это ~1 ГБ RGBA;
// файл с «раздутым» заголовком иначе может уронить программу по памяти.
const MaxPixels = 250_000_000

// CheckSize читает только заголовок и отказывает, если размер нулевой или больше MaxPixels.
func CheckSize(path string) error {
	_, _, err := checkFile(path)
	return err
}

// OpenRegular открывает обычный файл: устройство (CON, NUL на старых Windows), канал или
// каталог отклоняются по дескриптору уже открытого файла — защита в глубину поверх проверки пути.
func OpenRegular(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !st.Mode().IsRegular() {
		_ = f.Close()
		return nil, errors.New("не обычный файл")
	}
	return f, nil
}

// checkFile — CheckSize, но возвращает ещё и число пикселей (для бюджета памяти)
// и класс цветового профиля (Adobe RGB пересчитывается при декоде).
func checkFile(path string) (int64, Color, error) {
	f, err := OpenRegular(path)
	if err != nil {
		return 0, ColorNone, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	return checkOpened(f)
}

// checkOpened — проверка заголовка уже открытого файла; указатель после неё не определён.
func checkOpened(f *os.File) (int64, Color, error) {
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return 0, ColorNone, err
	}
	// imaging подключает и TIFF/BMP/GIF: у их декодеров в x/image известные уязвимости, а нужны только эти два.
	if format != "jpeg" && format != "png" {
		return 0, ColorNone, fmt.Errorf("формат %q не поддерживается (нужен JPEG или PNG)", format)
	}
	if err := CheckDims(cfg.Width, cfg.Height); err != nil {
		return 0, ColorNone, err
	}
	col := ColorNone
	if _, err := f.Seek(0, io.SeekStart); err == nil {
		col = Header(f).Color
	}
	return int64(cfg.Width) * int64(cfg.Height), col, nil
}

func CheckDims(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("некорректный размер изображения %dx%d", w, h)
	}
	if int64(w)*int64(h) > MaxPixels {
		return fmt.Errorf("изображение %dx%d слишком большое (больше %d Мп)", w, h, MaxPixels/1_000_000)
	}
	return nil
}

// OpenOriented — единственный способ декодировать оригинал: сначала проверка размера
// по заголовку, потом место в общем бюджете памяти и полный декод с учётом EXIF-поворота.
// Фото в Adobe RGB пересчитывается в sRGB сразу после декода, до любого уменьшения:
// миниатюры, превью и экспорт получают одинаковые цвета. Вызывающий обязан вызвать release,
// когда изображение больше не нужно.
func OpenOriented(path string) (img image.Image, release func(), err error) {
	// Файл не держим открытым, пока ждём бюджет: проверка по пути, место, и только потом открытие.
	n, col, err := checkFile(path)
	if err != nil {
		return nil, nil, err
	}
	if col == ColorAdobe {
		n *= 2 // YCbCr и NRGBA-копия живут одновременно, бюджет рассчитан на ~4 байта на пиксель
	}
	DecodeBudget.Acquire(n)
	done := false
	defer func() {
		// Ошибка или паника декодера: место возвращается, паника идёт дальше.
		if !done {
			DecodeBudget.Release(n)
		}
	}()
	f, err := OpenRegular(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	// Заголовок сверяется ещё раз на дескрипторе, с которого идёт декод: файл,
	// подменённый (переименованием или заменой) между проверкой и открытием, не обходит
	// лимиты. Перезапись на месте при FILE_SHARE_WRITE после этой проверки не закрывается.
	n2, col2, err := checkOpened(f)
	if err != nil {
		return nil, nil, err
	}
	if col2 == ColorAdobe {
		n2 *= 2
	}
	if n2 != n || col2 != col {
		return nil, nil, errors.New("файл изменился во время чтения")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	img, err = decodeImage(f, imaging.AutoOrientation(true))
	if err != nil {
		return nil, nil, err
	}
	if col == ColorAdobe {
		img = colorspace.AdobeToSRGB(img)
	}
	done = true
	return img, func() { DecodeBudget.Release(n) }, nil
}

// decodeImage — сам декод; в тестах подменяется, чтобы имитировать панику.
var decodeImage = imaging.Decode
