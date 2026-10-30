package library

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// DecodeError — оригинал не декодируется (усечён, повреждён, формат не тот): заголовок прочитался,
// фото в ленте есть, а миниатюры нет. Why — готовая причина для окна, тот же текст, что у Skip.Why.
// Кэш запоминает неудачу по ключу файла (размер и время входят в ключ): повторных декодов нет,
// а перезаписанный файл читается снова.
type DecodeError struct {
	Why string
	err error
}

func (e *DecodeError) Error() string { return e.err.Error() }
func (e *DecodeError) Unwrap() error { return e.err }

// whyMax — предел причины в байтах: текст ошибки декодера не должен раздувать State и события.
const whyMax = 600

// decodeFail оборачивает ошибку открытия оригинала src в DecodeError, если она про содержимое файла.
// Отмена, пропавший файл, сбой чтения диска (PathError: замок антивируса, обрыв тома) и подмена
// во время чтения — не порча: ошибка возвращается как есть и не запоминается.
func decodeFail(src string, err error) error {
	var pe *fs.PathError
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, fs.ErrNotExist) || errors.Is(err, imgfile.ErrChanged) || errors.As(err, &pe) {
		return err
	}
	return &DecodeError{Why: clipWhy(decodeWhy(src, err)), err: err}
}

// decodeWhy — причина по ошибке и первым байтам файла (файл не открылся — по самой ошибке).
func decodeWhy(src string, err error) string {
	f, _, oerr := OpenLocal(src)
	if oerr != nil {
		return whyOf(err)
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	head, _ := io.ReadAll(io.LimitReader(f, headLen))
	return Why(err, head)
}

// clipWhy обрезает причину до whyMax байт по границе руны.
func clipWhy(s string) string {
	if len(s) <= whyMax {
		return s
	}
	s = s[:whyMax]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}

// badMax — предел запомненных неудач; дальше память начинается заново (тысячи битых файлов — редкость).
const badMax = 4096

// failed — запомненная неудача декода файла с ключом key.
func (c *Cache) failed(key string) *DecodeError {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bad[key]
}

// noteFail запоминает неудачу декода; прочие ошибки возвращает как есть.
func (c *Cache) noteFail(key string, err error) error {
	var de *DecodeError
	if !errors.As(err, &de) {
		return err
	}
	c.mu.Lock()
	if c.bad == nil || len(c.bad) >= badMax {
		c.bad = map[string]*DecodeError{}
	}
	c.bad[key] = de
	c.mu.Unlock()
	return err
}
