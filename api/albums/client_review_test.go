package albums

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Нулевой expires_at в ответе — ответ не по контракту (Created, Published, Album, список).
func TestClientZeroExpiresRejected(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	album := func(extra string) string {
		return `{"id":"` + testID + `","url":"u","title":"t","has_password":false,"published":true,"pages":2` + extra + `}`
	}
	zero := `,"expires_at":"0001-01-01T00:00:00Z"`
	m := goodManifest(4, false)
	create := func(c *Client) error {
		_, e := c.Create(context.Background(), &CreateRequest{Manifest: *m, ExpiresAt: exp})
		return e
	}
	publish := func(c *Client) error { _, e := c.Publish(context.Background(), testID); return e }
	patch := func(c *Client) error {
		_, e := c.Patch(context.Background(), testID, &Patch{RevokeSessions: true})
		return e
	}
	list := func(c *Client) error { _, e := c.List(context.Background()); return e }
	for name, tc := range map[string]struct {
		body string
		call func(c *Client) error
	}{
		"create":          {`{"id":"` + testID + `","url":"u","missing":[1,3,4]}`, create},
		"create нулевой":  {`{"id":"` + testID + `","url":"u","missing":[1,3,4]` + zero + `}`, create},
		"publish":         {`{"url":"u"}`, publish},
		"publish нулевой": {`{"url":"u"` + zero + `}`, publish},
		"patch":           {album(""), patch},
		"patch нулевой":   {album(zero), patch},
		"список":          {`{"albums":[` + album("") + `]}`, list},
		"список нулевой":  {`{"albums":[` + album(zero) + `]}`, list},
	} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) }))
		if err := tc.call(c); !errors.Is(err, ErrResponse) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// с настоящим сроком тот же ответ проходит
	ok := album(`,"expires_at":"2030-01-01T00:00:00Z"`)
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, ok) }))
	if err := patch(c); err != nil {
		t.Errorf("patch со сроком: %v", err)
	}
}

// APIError.Missing: номера вне 1…limits.pages и повторы отброшены.
func TestAPIErrorMissingClean(t *testing.T) {
	for name, tc := range map[string]struct{ in, want []int }{
		"чистый":     {[]int{3, 5}, []int{3, 5}},
		"нули":       {[]int{0, 3, -1}, []int{3}},
		"повторы":    {[]int{3, 3, 5, 3}, []int{3, 5}},
		"за книгой":  {[]int{3, 301, 1 << 40}, []int{3}},
		"пусто":      {nil, nil},
		"все плохие": {[]int{0, -5}, nil},
	} {
		c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 409, ErrorBody{Error: Error{Code: CodePagesMissing, Missing: tc.in}})
		}))
		_, err := c.Publish(context.Background(), testID)
		var ae *APIError
		if !errors.As(err, &ae) || !slices.Equal(ae.Missing, tc.want) {
			t.Errorf("%s: %v, нужно %v", name, ae, tc.want)
		}
	}
}

// slowLimit — порог времени с множителем SA_TEST_SLOW (как testkit.Limit; testkit модулю недоступен).
// Ответ капает до 30 с, срок 0,6 с: порог растёт с множителем, но остаётся ниже длины ответа до SA_TEST_SLOW=10.
func slowLimit(base time.Duration) time.Duration {
	if v, err := strconv.ParseFloat(os.Getenv("SA_TEST_SLOW"), 64); err == nil && v >= 1 {
		return time.Duration(float64(base) * v)
	}
	return base
}

// У JSON-вызова есть общий срок: ответ, который капает по байту быстрее застоя, не висит вечно.
func TestClientTotalTimeout(t *testing.T) {
	fast(t, func(tm *timeouts) { tm.total = 600 * time.Millisecond })
	c, _ := plainClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		for i := 0; i < 300 && r.Context().Err() == nil; i++ {
			_, _ = io.WriteString(w, " ")
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	start := time.Now()
	_, err := c.List(context.Background())
	var ne *NetError
	if !errors.As(err, &ne) || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > slowLimit(3*time.Second) {
		t.Errorf("капающий ответ: %v за %v", err, time.Since(start))
	}
	if err != nil && strings.Contains(err.Error(), testID) {
		t.Error("id в тексте ошибки")
	}
}
