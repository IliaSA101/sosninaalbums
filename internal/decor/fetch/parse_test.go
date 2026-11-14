package fetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
)

// Parse — тот же путь проверки, что у Catalog, но по байтам конверта (сохранённый на диске
// список, 137): подпись, seq и пределы; Catalog отдаёт конверт в Env байт в байт.
func TestClientParse(t *testing.T) {
	l := newTestLib(t)
	env := l.envelope(bodyJSON(t, 7))
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(env) })
	c := l.client()
	got, err := c.Catalog(context.Background(), 0)
	if err != nil || !bytes.Equal(got.Env, env) {
		t.Fatalf("Catalog: %v, конверт совпал %v", err, bytes.Equal(got.Env, env))
	}
	for _, tc := range []struct {
		name string
		env  []byte
		min  int64
		want error
	}{
		{"верный", env, 7, nil},
		{"seq меньше принятого", env, 8, ErrSignature},
		{"байт в конверте", bytes.Replace(env, []byte(`"sig":"`), []byte(`"sig":"A`), 1), 0, ErrSignature},
		{"пустой", nil, 0, ErrCatalog},
		{"мусор", []byte("{{"), 0, ErrCatalog},
		{"больше предела", bytes.Repeat([]byte(" "), MaxEnvelope+1), 0, ErrCatalog},
	} {
		cat, err := c.Parse(tc.env, tc.min)
		if !errors.Is(err, tc.want) || err == nil && (cat.Seq != 7 || cat.Env != nil) {
			t.Errorf("%s: %v %+v", tc.name, err, cat)
		}
	}
}
