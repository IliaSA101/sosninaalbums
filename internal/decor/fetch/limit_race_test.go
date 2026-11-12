package fetch

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// lateRT — транспорт, который после ответа сервера ждёт срабатывания ctx: так выглядит предел
// (или застой), сработавший вровень с ответом, когда сервер успел отдать пустое или короткое тело.
type lateRT struct{ inner http.RoundTripper }

func (a lateRT) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := a.inner.RoundTrip(r)
	if err == nil {
		<-r.Context().Done()
	}
	return resp, err
}

// Ответ неверной длины, пришедший при уже сработавшем пределе набора или застое, — ErrSlow/ErrTimeout,
// не ErrBroken (флак TestDownloadLimitBySize на медленном раннере). Порча по sha256 при полной
// длине остаётся ErrBroken; без срабатывания неверная длина — ErrBroken (cancel_race_test.go).
func TestDownloadLimitBeatsBroken(t *testing.T) {
	body := make([]byte, 3000)
	for i := range body {
		body[i] = byte(i)
	}
	cases := map[string]http.HandlerFunc{
		"длина другая": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body[:10]) },
		"пустое тело":  func(http.ResponseWriter, *http.Request) {},
		"короткое тело": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body[:10])
		},
	}
	for name, handler := range cases {
		for _, stall := range []bool{false, true} {
			t.Run(name+"/застой="+strconv.FormatBool(stall), func(t *testing.T) {
				l := newTestLib(t)
				l.mux.HandleFunc("/x.zip", handler)
				c := l.client()
				want := ErrSlow
				if stall {
					c.SetTimeouts(50*time.Millisecond, time.Minute)
					want = ErrTimeout
				} else {
					c.SetTimeouts(time.Minute, 50*time.Millisecond)
				}
				c.rate = 0 // предел — только пол
				c.hc.Transport = lateRT{c.hc.Transport}
				dst := dlPath(t)
				err := c.Download(context.Background(), entryFor(l, "/x.zip", body), dst, nil)
				if !errors.Is(err, want) || errors.Is(err, ErrBroken) {
					t.Errorf("%v, нужна %v", err, want)
				}
				assertGone(t, dst)
			})
		}
	}
	// Полная длина, но другое содержимое: порча остаётся порчей.
	l := newTestLib(t)
	bad := append([]byte(nil), body...)
	bad[0]++
	l.serve("/bad.zip", bad)
	c := l.client()
	dst := dlPath(t)
	if err := c.Download(context.Background(), entryFor(l, "/bad.zip", body), dst, nil); !errors.Is(err, ErrBroken) {
		t.Errorf("порча: %v", err)
	}
	assertGone(t, dst)
}
