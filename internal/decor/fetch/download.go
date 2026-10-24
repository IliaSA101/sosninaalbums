package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"
)

// brokenError — архив не сошёлся с каталогом (размер, sha256): тост называет набор.
type brokenError struct{ title string }

func (e brokenError) Error() string {
	return "набор «" + e.title + "» скачался с ошибкой — повторите"
}

func (e brokenError) Is(target error) bool { return target == ErrBroken }

// Broken — тот же отказ для набора, испорченного не в дороге: архив сошёлся с каталогом, но
// предмет внутри не сходится с манифестом (ошибка издателя).
func Broken(title string) error { return brokenError{title} }

// Download — архив строки e в файл dst (создаётся заново; при ошибке удаляется). Тело не
// больше e.Size: Content-Length другой — отказ до чтения, лишний байт — обрыв; sha256 считается
// по дороге. Без байтов дольше «застоя» или набор дольше предела — обрыв (ErrTimeout); отмена
// ctx — ErrCancelled. progress (может быть nil) — байты по мере чтения; частоту режет вызывающий.
func (c *Client) Download(ctx context.Context, e Entry, dst string, progress func(done int64)) (err error) {
	if e.Size < 1 || e.Size > MaxZip || !validSum(e.SHA256) {
		return brokenError{e.Title}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	ctx, stop := context.WithTimeoutCause(ctx, c.limit, errPackLimit)
	defer stop()
	// Застой считается и на заголовках: после них таймер сбрасывает каждое чтение.
	stall := time.AfterFunc(c.stall, func() { cancel(errStall) })
	defer stall.Stop()
	resp, err := c.get(ctx, e.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if resp.ContentLength >= 0 && resp.ContentLength != e.Size {
		return brokenError{e.Title}
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return ErrDisk
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = ErrDisk
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	h := sha256.New()
	done, err := c.copyBody(ctx, f, io.TeeReader(io.LimitReader(resp.Body, e.Size+1), h), stall, e, progress)
	if err != nil {
		return err
	}
	if done != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
		return brokenError{e.Title}
	}
	return nil
}

// copyBody — тело в файл кусками; каждый кусок сбрасывает таймер застоя.
func (c *Client) copyBody(ctx context.Context, w io.Writer, r io.Reader, stall *time.Timer, e Entry, progress func(int64)) (int64, error) {
	buf := make([]byte, 64<<10)
	var done int64
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			stall.Reset(c.stall)
			if done += int64(n); done > e.Size {
				return done, brokenError{e.Title}
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return done, ErrDisk
			}
			if progress != nil {
				progress(done)
			}
		}
		if errors.Is(rerr, io.EOF) {
			return done, nil
		}
		// Сервер оборвал тело короче своего Content-Length: архив неполный, а связь есть.
		if errors.Is(rerr, io.ErrUnexpectedEOF) && context.Cause(ctx) == nil {
			return done, brokenError{e.Title}
		}
		if rerr != nil {
			return done, netErr(ctx, rerr)
		}
	}
}
