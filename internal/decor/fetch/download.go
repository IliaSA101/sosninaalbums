package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

// SlowError — набор не уложился в предел времени: тост называет набор и предел. Is(ErrSlow).
type SlowError struct {
	Title string
	Limit time.Duration
}

func (e *SlowError) Error() string {
	if e.Limit < time.Minute {
		return fmt.Sprintf("Связь слишком медленная: «%s» не скачался за %d с", e.Title, int(e.Limit.Seconds()))
	}
	return fmt.Sprintf("Связь слишком медленная: «%s» не скачался за %d мин", e.Title, int((e.Limit+time.Minute-1)/time.Minute))
}

func (e *SlowError) Is(target error) bool { return target == ErrSlow }

// Broken — тот же отказ для набора, испорченного не в дороге: архив сошёлся с каталогом, но
// предмет внутри не сходится с манифестом (ошибка издателя).
func Broken(title string) error { return brokenError{title} }

// Download — архив строки e в файл dst (создаётся заново; при ошибке удаляется). Остаток по
// этому пути (в том числе ссылка или FIFO, подложенные до запуска) убирается, файл открывается
// с O_EXCL: он не следует ссылке и не ждёт читателя, а дескриптор держится до конца записи.
// Условия и ошибки — как у DownloadTo.
func (c *Client) Download(ctx context.Context, e Entry, dst string, progress func(done int64)) (err error) {
	var f *os.File
	defer func() {
		if f == nil {
			return
		}
		if err == nil && !stillSame(f, dst) {
			err = ErrDisk // путь подменён, пока шла сеть: Rename унёс бы чужое
		}
		if cerr := f.Close(); cerr != nil && err == nil {
			err = ErrDisk
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	return c.download(ctx, e, progress, func() (*os.File, error) {
		var oerr error
		f, oerr = OpenNew(dst)
		return f, oerr
	})
}

// OpenNew — свежий файл по пути dst для записи и чтения (0600): остаток по этому пути (в том числе
// ссылка или FIFO) убирается, открытие с O_EXCL не следует ссылке и не ждёт читателя. Вызывающий
// пишет и читает этим дескриптором (DownloadTo, pack.OpenFile), по пути файл заново не открывает.
func OpenNew(dst string) (*os.File, error) {
	_ = os.Remove(dst) // остаток прежней загрузки; нет файла — не ошибка
	return os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
}

// DownloadTemp — архив строки e во временный файл dir/pattern (os.CreateTemp, 0600): файл
// создаётся и пишется одним дескриптором, по пути его никто не открывает заново (ссылку или FIFO,
// подложенные на время сети, загрузка не заденет). Удачно — путь закрытого файла после Sync
// (дальше Rename у вызывающего, он же удаляет файл при своей ошибке); ошибка — файла нет.
func (c *Client) DownloadTemp(ctx context.Context, e Entry, dir, pattern string, progress func(done int64)) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", ErrDisk
	}
	path := f.Name()
	err = c.DownloadTo(ctx, e, f, progress)
	if err == nil && !stillSame(f, path) {
		err = ErrDisk // путь подменён, пока шла сеть: Rename унёс бы чужое
	}
	// Windows: сначала Close, потом Remove.
	if cerr := f.Close(); cerr != nil && err == nil {
		err = ErrDisk
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// stillSame — по пути лежит тот самый файл, что открыт в f (не ссылка и не подмена).
func stillSame(f *os.File, path string) bool {
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() {
		return false
	}
	st, err := f.Stat()
	return err == nil && os.SameFile(lst, st)
}

// DownloadTo — архив строки e в уже открытый для записи пустой файл f. Тело не больше e.Size:
// Content-Length другой — отказ до чтения, лишний байт — обрыв; sha256 считается по дороге;
// в конце f.Sync. Без байтов дольше «застоя» — ErrTimeout; набор дольше предела (PackLimit; c.limit —
// его пол) — ErrSlow; отмена ctx — ErrCancelled. progress (может быть nil) — байты по мере чтения;
// частоту режет вызывающий. f не закрывается и не удаляется ни при каком исходе: это дело вызывающего.
func (c *Client) DownloadTo(ctx context.Context, e Entry, f *os.File, progress func(done int64)) error {
	return c.download(ctx, e, progress, func() (*os.File, error) { return f, nil })
}

// download — общее тело: open зовётся после заголовков ответа (до них файл не нужен и не создаётся).
func (c *Client) download(ctx context.Context, e Entry, progress func(done int64), open func() (*os.File, error)) (err error) {
	if e.Size < 1 || e.Size > MaxZip || !validSum(e.SHA256) {
		return brokenError{e.Title}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	limit := limitFor(c.limit, c.rate, e.Size)
	ctx, stop := context.WithTimeoutCause(ctx, limit, errPackLimit)
	defer stop()
	// Предел набора — не застой: своя ошибка, ход «Скачать все» от неё обрывается (0.29-137-6).
	defer func() {
		if err != nil && errors.Is(context.Cause(ctx), errPackLimit) && !errors.Is(err, ErrDisk) && !errors.Is(err, ErrBroken) {
			err = &SlowError{e.Title, limit}
		}
	}()
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
	f, err := open()
	if err != nil {
		return ErrDisk
	}
	h := sha256.New()
	done, err := c.copyBody(ctx, f, io.TeeReader(io.LimitReader(resp.Body, e.Size+1), h), stall, e, progress)
	if err != nil {
		return err
	}
	if done != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
		return brokenError{e.Title}
	}
	if f.Sync() != nil {
		return ErrDisk
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
