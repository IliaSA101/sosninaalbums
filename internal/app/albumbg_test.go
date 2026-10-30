package app

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
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

// «Как в альбоме» для фона (115): SetAlbumBgToAll ставит фон альбома и снимает свои фоны
// разворотов одним шагом; «скрыт» без своего фона и обложка остаются; плитка в недавних один
// раз; чужая плитка — отказ без шага, свои фоны на месте.
func TestAppSetAlbumBgToAll(t *testing.T) {
	a, own, _ := bgToApp(t)
	ed := a.editor()
	if _, err := a.SetAlbumBg("sa:cream-gradient", true); err != nil {
		t.Fatal(err)
	}
	ed.SetNoAlbumBg("s1", true)
	targets := []model.GridTarget{{Spread: "s2", Scope: "spread"}}
	if r, err := a.ApplyBackgroundTo(targets, own); err != nil || r.Spreads != 1 {
		t.Fatalf("свой фон: %+v %v", r, err)
	}
	before := ed.View()
	if len(before.Spreads[1].Backdrops) != 1 || before.Spreads[1].Backdrops[0].Album {
		t.Fatalf("исходные фоны: %+v %+v", before.Spreads[1].Backdrops, before.Cover.Sheet.Backdrops)
	}
	recent := len(prefs.RecentDecor(a.cfgDir, decor.KindBg))

	if v, err := a.SetAlbumBgToAll("sa:нет-такого", false); !errors.Is(err, decor.ErrNotFound) || v.Rev != before.Rev {
		t.Fatalf("чужая плитка: %v rev %d→%d", err, before.Rev, v.Rev)
	}
	if got := len(prefs.RecentDecor(a.cfgDir, decor.KindBg)); got != recent || len(ed.View().Spreads[1].Backdrops) != 1 {
		t.Fatalf("после отказа: недавних %d→%d", recent, got)
	}

	v, err := a.SetAlbumBgToAll("sa:confetti", false)
	if err != nil || v.Rev != before.Rev+1 || v.AlbumBg == nil || v.AlbumBg.Preset != "sa:confetti" {
		t.Fatalf("постановка: %v rev %d %+v", err, v.Rev, v.AlbumBg)
	}
	s1, s2 := v.Spreads[0], v.Spreads[1]
	if !s1.NoAlbumBg || len(s1.Patterns) != 0 {
		t.Fatalf("s1 скрыт без своего: скрыт %v, узоры %+v", s1.NoAlbumBg, s1.Patterns)
	}
	if len(s2.Backdrops) != 0 || len(s2.Patterns) != 1 || !s2.Patterns[0].Album {
		t.Fatalf("s2: фото %+v узоры %+v", s2.Backdrops, s2.Patterns)
	}
	if len(v.Cover.Sheet.Backdrops)+len(v.Cover.Sheet.Patterns) != 0 {
		t.Fatalf("обложка задета: %+v", v.Cover.Sheet.Backdrops)
	}
	if got := prefs.RecentDecor(a.cfgDir, decor.KindBg); len(got) != recent+1 || got[0] != "sa:confetti" {
		t.Fatalf("недавние: %v", got)
	}
	// Тот же фон, своих нет — холостая, недавние не дублируются.
	if again, err := a.SetAlbumBgToAll("sa:confetti", false); err != nil || again.Rev != v.Rev {
		t.Fatalf("повтор: %v rev %d→%d", err, v.Rev, again.Rev)
	}
	if got := len(prefs.RecentDecor(a.cfgDir, decor.KindBg)); got != recent+1 {
		t.Fatalf("недавние после повтора: %d", got)
	}
	if u := ed.Undo(); len(u.Spreads[1].Backdrops) != 1 || u.AlbumBg == nil || u.AlbumBg.Preset != "sa:cream-gradient" {
		t.Fatalf("отмена одним шагом: %+v", u.AlbumBg)
	}
}
