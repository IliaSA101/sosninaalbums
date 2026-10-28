package fetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// Предел набора — от размера (0.29-137-6): пол 15 мин, большой — по 64 КиБ/с; края — 0, минус,
// выше MaxZip (не растёт за MaxZip), без переполнения.
func TestPackLimit(t *testing.T) {
	for name, tc := range map[string]struct {
		size int64
		want time.Duration
	}{
		"ноль":            {0, 15 * time.Minute},
		"минус":           {-5, 15 * time.Minute},
		"малый 1 МиБ":     {1 << 20, 15 * time.Minute},
		"70 МиБ":          {70 << 20, 1120 * time.Second},
		"граница пола":    {15 * 60 * minRate, 15 * time.Minute},
		"MaxZip":          {MaxZip, 4800 * time.Second},
		"выше MaxZip":     {MaxZip + 1<<40, 4800 * time.Second},
		"максимум int64":  {1<<63 - 1, 4800 * time.Second},
		"минимум int64":   {-1 << 63, 15 * time.Minute},
		"чуть выше пола":  {15*60*minRate + minRate, 15*time.Minute + time.Second},
		"половина MaxZip": {MaxZip / 2, 2400 * time.Second},
	} {
		if got := PackLimit(tc.size); got != tc.want {
			t.Errorf("%s: %v, ждали %v", name, got, tc.want)
		}
	}
	// Нулевая и отрицательная скорость — только пол (так тесты сужают предел), без деления на ноль.
	for _, rate := range []int64{0, -1} {
		if got := limitFor(time.Second, rate, MaxZip); got != time.Second {
			t.Errorf("скорость %d: %v", rate, got)
		}
	}
}

// Download берёт больший из пола клиента и предела по размеру: медленный, но идущий большой набор
// не обрывается по полу, малый — по-прежнему по нему.
func TestDownloadLimitBySize(t *testing.T) {
	l := newTestLib(t)
	body := bytes.Repeat([]byte{3}, 100_000) // по размеру — около 1,5 с
	l.mux.HandleFunc("/slow.zip", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(400 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	})
	c := l.client()
	c.SetTimeouts(time.Minute, 100*time.Millisecond) // пол уже ответа сервера; rate — штатный
	dst := dlPath(t)
	if err := c.Download(context.Background(), entryFor(l, "/slow.zip", body), dst, nil); err != nil {
		t.Fatalf("предел по размеру: %v", err)
	}
	// Малый набор: 1000 Б — по размеру 15 мс, держит пол 100 мс; сервер дольше — обрыв.
	small := body[:1000]
	l.mux.HandleFunc("/small.zip", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(small)
	})
	dst2 := dlPath(t)
	if err := c.Download(context.Background(), entryFor(l, "/small.zip", small), dst2, nil); !errors.Is(err, ErrSlow) {
		t.Fatalf("малый: %v", err)
	}
	assertGone(t, dst2)
}

// SetTimeouts: нулевое значение своё не меняет.
func TestSetTimeouts(t *testing.T) {
	c := New(Source{})
	c.SetTimeouts(0, 0)
	if c.stall != stallTimeout || c.limit != packTimeout {
		t.Fatalf("0 изменил: %v %v", c.stall, c.limit)
	}
	c.SetTimeouts(time.Second, 0)
	if c.stall != time.Second || c.limit != packTimeout {
		t.Fatalf("только застой: %v %v", c.stall, c.limit)
	}
}

// Тост предела: минуты округляются вверх (1120 с — «19 мин», а не «18»), до минуты — секунды;
// краевые значения — ровно минута, без секунды, 0.
func TestSlowErrorText(t *testing.T) {
	for _, tc := range []struct {
		limit time.Duration
		want  string
	}{
		{1120 * time.Second, "не скачался за 19 мин"},
		{18 * time.Minute, "не скачался за 18 мин"},
		{18*time.Minute + time.Nanosecond, "не скачался за 19 мин"},
		{time.Minute, "не скачался за 1 мин"},
		{59 * time.Second, "не скачался за 59 с"},
		{0, "не скачался за 0 с"},
	} {
		got := (&SlowError{Title: "X", Limit: tc.limit}).Error()
		if want := "Связь слишком медленная: «X» " + tc.want; got != want {
			t.Errorf("%v: %q, ждали %q", tc.limit, got, want)
		}
	}
}
