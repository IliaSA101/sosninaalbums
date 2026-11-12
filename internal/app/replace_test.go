package app

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// replaceApp — App с «Авто»-разворотом из трёх фото (a2, b2, c) и фото ленты «n» (в альбоме его нет); emits считает рассылки вида.
func replaceApp(t *testing.T) (a *App, sid string, emits *atomic.Int32) {
	t.Helper()
	a = gridApp(t)
	emits = new(atomic.Int32)
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			emits.Add(1)
		}
	}
	for id, sz := range map[string][2]int{"a2": {300, 200}, "b2": {200, 300}, "c": {300, 200}, "n": {200, 300}} {
		a.byID[id] = library.Photo{ID: id, Path: testkit.AbsPath(id + ".jpg"), Name: id + ".jpg", W: sz[0], H: sz[1]}
	}
	v, err := a.editor().NewSpread(-1, a.infos([]string{"a2", "b2", "c"}))
	if err != nil {
		t.Fatal(err)
	}
	return a, v.Spreads[len(v.Spreads)-1].ID, emits
}

// TestAppReplacePhoto (168): Left и вид после замены, одна рассылка; холостая правка — без рассылки
// и без смены rev; пропавшее при пересканировании фото — ErrNoPhoto, проект прежний.
func TestAppReplacePhoto(t *testing.T) {
	a, sid, emits := replaceApp(t)
	rev, ev := a.editor().View().Rev, emits.Load()

	r, err := a.ReplacePhoto(sid, "b2", "n")
	if err != nil || r.Left != 1 || len(r.Named) != 0 {
		t.Fatalf("ReplacePhoto: err %v, left %d, named %v", err, r.Left, r.Named)
	}
	var got []string
	for _, s := range r.View.Spreads {
		if s.ID == sid {
			got = s.Photos
		}
	}
	if !slices.Equal(got, []string{"a2", "n", "c"}) || r.View.Rev != rev+1 || emits.Load() != ev+1 {
		t.Fatalf("фото %v, rev %d → %d, рассылок %d", got, rev, r.View.Rev, emits.Load()-ev)
	}

	// Холостая: неизвестный target и то же фото — rev и рассылки прежние.
	rev, ev = a.editor().View().Rev, emits.Load()
	for _, c := range [][2]string{{"нет", "b2"}, {"n", "n"}} {
		r, err = a.ReplacePhoto(sid, c[0], c[1])
		if err != nil || r.Left != 0 || r.View.Rev != rev || emits.Load() != ev {
			t.Fatalf("холостая %v: err %v, left %d, rev %d → %d, рассылок %d", c, err, r.Left, rev, r.View.Rev, emits.Load()-ev)
		}
	}

	// Фото пропало при пересканировании (нет ни в ленте, ни в альбоме): ошибка, проект прежний.
	r, err = a.ReplacePhoto(sid, "a2", "призрак")
	if !errors.Is(err, errNoPhoto) || r.View.Rev != rev || a.editor().View().Rev != rev || emits.Load() != ev {
		t.Fatalf("пропавшее фото: err %v, rev %d → %d, рассылок %d", err, rev, a.editor().View().Rev, emits.Load()-ev)
	}

	// Отмена — один шаг: прежнее фото вернулось на место.
	a.Undo()
	if p := a.editor().View().Spreads; !slices.Equal(p[len(p)-1].Photos, []string{"a2", "b2", "c"}) {
		t.Fatalf("после Undo: %v", p[len(p)-1].Photos)
	}
}
