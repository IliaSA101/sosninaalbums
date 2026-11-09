package library

import (
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"io/fs"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// Skip — файл, который найден, но не прочитан как картинка, и готовая русская причина (117).
type Skip struct {
	Path string
	Why  string
}

// headLen — сколько первых байтов нужно, чтобы узнать формат: ftyp-бренд (до 12), флаги VP8X (до 21), BMP (до 14).
const headLen = 32

// advice — совет к любой причине, кроме «слишком большое»: файл можно пересохранить.
const advice = ". Откройте в Paint и сохраните как JPEG («Сохранить как» → «Изображение в формате JPEG»)"

// unreadError — ошибка чтения заголовка с готовой причиной: сканер берёт её без второго открытия файла.
type unreadError struct {
	err error
	why string
}

func (e *unreadError) Error() string { return e.err.Error() }
func (e *unreadError) Unwrap() error { return e.err }

// withWhy оборачивает err причиной по первым байтам открытого файла f (позиция не важна).
func withWhy(err error, f io.ReadSeeker) error {
	var head []byte
	if _, serr := f.Seek(0, io.SeekStart); serr == nil {
		head, _ = io.ReadAll(io.LimitReader(f, headLen))
	}
	return &unreadError{err: err, why: Why(err, head)}
}

// whyOf — причина из ошибки чтения; без своей (файл не открылся) — по самой ошибке.
func whyOf(err error) string {
	var u *unreadError
	if errors.As(err, &u) {
		return u.why
	}
	return failWhy(err) + advice
}

// failWhy — причина по тексту ошибки, когда формат не узнан: файл не открылся, не обычный файл,
// сбой чтения. Текст — без пути (PathError → операция и причина ОС): лог отправляют разработчику (073-7).
func failWhy(err error) string {
	return "не удалось прочитать: " + errText(err)
}

// errText — текст ошибки без пути файла.
func errText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Op + ": " + pe.Err.Error()
	}
	return err.Error()
}

// ShortWhy — причина без совета «сохраните как JPEG»: для строки журнала.
func ShortWhy(why string) string { return strings.TrimSuffix(why, advice) }

// Why — причина, по которой файл не читается, одной русской фразой для окна: err — ошибка
// imgfile.DimsOf, head — первые байты файла (до 32). Совет «сохраните как JPEG» — у всех, кроме
// «слишком большое».
func Why(err error, head []byte) string {
	if errors.Is(err, imgfile.ErrTooBig) {
		return fmt.Sprintf("слишком большое (больше %d Мп)", imgfile.MaxPixels/1_000_000)
	}
	return whyKind(err, head) + advice
}

func whyKind(err error, head []byte) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return failWhy(err)
	}
	if len(head) == 0 {
		return "пустой файл"
	}
	if name := foreignFormat(head); name != "" {
		return "внутри " + name + ", а не JPEG"
	}
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8}):
		var ue jpeg.UnsupportedError
		if errors.As(err, &ue) {
			return "JPEG со сжатием, которое программа не читает"
		}
		return "файл повреждён: " + errText(err)
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return "файл повреждён: " + errText(err)
	case isWebP(head):
		if len(head) > 20 && string(head[12:16]) == "VP8X" && head[20]&0x02 != 0 {
			return "анимированный WebP"
		}
		return "WebP, который программа не читает: " + errText(err)
	}
	return "не похоже на картинку"
}

func isWebP(h []byte) bool {
	return len(h) >= 12 && string(h[:4]) == "RIFF" && string(h[8:12]) == "WEBP"
}

// foreignFormat — название формата, который программа не читает, по первым байтам; "" — другой.
func foreignFormat(h []byte) string {
	if len(h) >= 12 && string(h[4:8]) == "ftyp" {
		switch string(h[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1":
			return "HEIC"
		case "avif", "avis":
			return "AVIF"
		}
	}
	switch {
	case bytes.HasPrefix(h, []byte("GIF8")):
		return "GIF"
	case bytes.HasPrefix(h, []byte("II*\x00")), bytes.HasPrefix(h, []byte("MM\x00*")):
		return "TIFF"
	case len(h) >= 14 && h[0] == 'B' && h[1] == 'M' && bytes.Equal(h[6:10], []byte{0, 0, 0, 0}):
		return "BMP" // «BM» одних мало: добавлены нули зарезервированных полей
	}
	return ""
}
