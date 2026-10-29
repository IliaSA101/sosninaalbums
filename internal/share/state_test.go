package share

import (
	"testing"
	"time"
)

// Все семь состояний design §3.4 и порядок проверок.
func TestShareState(t *testing.T) {
	const origin = "https://albums.example.org"
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	pub := func(mod func(r *Record)) *Record {
		r := &Record{Origin: origin, ID: "abc", URL: origin + "/a/abc/", ExpiresAt: now.Add(24 * time.Hour), Scene: "s1"}
		if mod != nil {
			mod(r)
		}
		return r
	}
	for _, c := range []struct {
		name  string
		rec   *Record
		scene string
		want  Kind
	}{
		{"нет записи", nil, "s1", StateNone},
		{"пустая запись", &Record{}, "s1", StateNone},
		{"не опубликована", pub(func(r *Record) { r.URL = "" }), "s1", StateIncomplete},
		{"опубликована", pub(nil), "s1", StateOK},
		{"сцена другая", pub(nil), "s2", StateChanged},
		{"срок прошёл", pub(func(r *Record) { r.ExpiresAt = now.Add(-time.Second) }), "s1", StateExpired},
		{"срок в эту секунду", pub(func(r *Record) { r.ExpiresAt = now }), "s1", StateExpired},
		{"срок и правка: срок важнее", pub(func(r *Record) { r.ExpiresAt = now.Add(-time.Hour) }), "s2", StateExpired},
		{"срока нет", pub(func(r *Record) { r.ExpiresAt = time.Time{} }), "s1", StateOK},
		{"404", pub(func(r *Record) { r.Gone = true }), "s1", StateGone},
		{"404 важнее срока", pub(func(r *Record) { r.Gone, r.ExpiresAt = true, now.Add(-time.Hour) }), "s2", StateGone},
		{"другой сервис", pub(func(r *Record) { r.Origin = "https://other.example.org" }), "s1", StateOther},
		{"другой сервис важнее 404", pub(func(r *Record) { r.Origin, r.Gone = "https://other.example.org", true }), "s2", StateOther},
		{"другой сервис, не опубликована", pub(func(r *Record) { r.Origin, r.URL = "http://localhost:1", "" }), "s1", StateOther},
		{"404 до публикации", pub(func(r *Record) { r.URL, r.Gone = "", true }), "s1", StateGone},
	} {
		if got := State(c.rec, origin, c.scene, now); got != c.want {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
	// Подключён другой сервис — запись прежнего «чужая», а не «ок».
	if got := State(pub(nil), "", "s1", now); got != StateOther {
		t.Errorf("сервис не подключён: %q", got)
	}
}
