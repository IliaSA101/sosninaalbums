package fetch

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// afterRT — транспорт, который отменяет ctx сразу после ответа сервера: так выглядит отмена,
// обогнавшая закрытие соединения (ответ уже пришёл, а ctx отменён).
type afterRT struct {
	inner  http.RoundTripper
	cancel context.CancelFunc
}

func (a afterRT) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := a.inner.RoundTrip(r)
	a.cancel()
	return resp, err
}

// Отмена при уже пришедшем ответе с неверной длиной, коротким или пустым телом — ErrCancelled,
// не ErrBroken; без отмены те же ответы остаются ErrBroken. Файла нет (оба пути: Download, DownloadTemp).
func TestDownloadCancelBeatsBroken(t *testing.T) {
	body := make([]byte, 3000)
	cases := map[string]http.HandlerFunc{
		"длина другая": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body[:10]) },
		"короткое тело": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body[:10])
		},
		"пустое тело": func(http.ResponseWriter, *http.Request) {},
	}
	for name, handler := range cases {
		for _, cancelled := range []bool{true, false} {
			t.Run(name+"/"+strconv.FormatBool(cancelled), func(t *testing.T) {
				l := newTestLib(t)
				l.mux.HandleFunc("/x.zip", handler)
				c := l.client()
				want := ErrBroken
				ctx, cancel := context.WithCancel(context.Background())
				if cancelled {
					c.hc.Transport = afterRT{c.hc.Transport, cancel}
					want = ErrCancelled
				}
				defer cancel()
				e := entryFor(l, "/x.zip", body)
				dir := testkit.TempDir(t)
				dst := filepath.Join(dir, "d.tmp")
				err := c.Download(ctx, e, dst, nil)
				if !errors.Is(err, want) || (cancelled && errors.Is(err, ErrBroken)) {
					t.Errorf("Download: %v, нужна %v", err, want)
				}
				assertGone(t, dst)
				// Второй путь — со свежим ctx: отменённый отказал бы ещё на запросе.
				ctx2, cancel2 := context.WithCancel(context.Background())
				defer cancel2()
				if cancelled {
					c.hc.Transport = afterRT{c.hc.Transport.(afterRT).inner, cancel2}
				}
				_, err = c.DownloadTemp(ctx2, e, dir, "t-*.tmp", nil)
				if !errors.Is(err, want) || (cancelled && errors.Is(err, ErrBroken)) {
					t.Errorf("DownloadTemp: %v, нужна %v", err, want)
				}
				if left, _ := filepath.Glob(filepath.Join(dir, "t-*.tmp")); len(left) != 0 {
					t.Errorf("остались %v", left)
				}
			})
		}
	}
}
