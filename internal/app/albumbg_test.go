package app

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Фон альбома из окна «Фоны» (098 §3): встроенный и свой на оба разворота одним шагом, цвет
// страниц пресета не ставится, обложка не задета; повтор — холостая; чужая плитка — отказ без
// шага и без недавних.
func TestAppSetAlbumBg(t *testing.T) {
	a, own, _ := bgToApp(t)
	rev := a.editor().View().Rev
	if v, err := a.SetAlbumBg("sa:нет-такого", false); !errors.Is(err, decor.ErrNotFound) || v.Rev != rev {
		t.Fatalf("чужая плитка: %v rev %d→%d", err, rev, v.Rev)
	}
	if r := prefs.RecentDecor(a.cfgDir, decor.KindBg); len(r) != 0 {
		t.Fatalf("недавние после отказа: %v", r)
	}
	v, err := a.SetAlbumBg("sa:cream-gradient", true)
	if err != nil || v.Rev != rev+1 || v.AlbumBg == nil || v.AlbumBg.Preset != "sa:cream-gradient" {
		t.Fatalf("узор: %v rev %d %+v", err, v.Rev, v.AlbumBg)
	}
	for _, s := range v.Spreads {
		if len(s.Patterns) != 2 || !s.Patterns[0].Album || s.Bg[0] == "#fbf6ef" {
			t.Fatalf("%s: %+v %v", s.ID, s.Patterns, s.Bg)
		}
	}
	if c := v.Cover.Sheet; len(c.Patterns)+len(c.Backdrops) != 0 {
		t.Fatalf("обложка задета: %+v", c.Patterns)
	}
	if again, err := a.SetAlbumBg("sa:cream-gradient", true); err != nil || again.Rev != v.Rev {
		t.Fatalf("повтор: %v rev %d→%d", err, v.Rev, again.Rev)
	}
	v, err = a.SetAlbumBg(own, false)
	if err != nil || v.AlbumBg.Lib != own {
		t.Fatalf("своё: %v %+v", err, v.AlbumBg)
	}
	for _, s := range v.Spreads {
		if len(s.Backdrops) != 1 || !s.Backdrops[0].Album || s.Backdrops[0].Lib != own || len(s.Patterns) != 0 {
			t.Fatalf("%s: фото %+v узоры %+v", s.ID, s.Backdrops, s.Patterns)
		}
	}
	if got := prefs.RecentDecor(a.cfgDir, decor.KindBg); len(got) != 2 || got[0] != own {
		t.Fatalf("недавние: %v", got)
	}
}
