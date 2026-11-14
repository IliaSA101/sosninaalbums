package app

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// bgToApp — decorApp с обложкой и своим фоном; all — оба разворота и обложка, весь лист.
func bgToApp(t *testing.T) (a *App, own string, all []model.GridTarget) {
	t.Helper()
	a = decorApp(t)
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "свой.jpg", 50, 50))
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	return a, ids[0], []model.GridTarget{{Spread: "s1", Scope: "spread"}, {Spread: "s2", Scope: "spread"}, {Spread: model.CoverID, Scope: "spread"}}
}

// Отказы фона на набор до правки: корешок, мусор в области, неизвестная плитка — без шага,
// без недавних; план — та же ошибка и пустой.
func TestAppApplyBackgroundToRefuse(t *testing.T) {
	a, own, all := bgToApp(t)
	rev := a.editor().View().Rev
	for _, c := range []struct {
		ts   []model.GridTarget
		id   string
		want error
	}{
		{append(slices.Clone(all), model.GridTarget{Spread: "s1", Scope: model.ScopeSpine}), own, errBackdropScope},
		{[]model.GridTarget{{Spread: "s1", Scope: "top"}}, "sa:confetti", errBackdropScope},
		{all, "sa:нет-такого", decor.ErrNotFound},
	} {
		if r, err := a.ApplyBackgroundTo(c.ts, c.id); !errors.Is(err, c.want) || r.View.Rev != rev || r.Spreads != 0 {
			t.Fatalf("%+v: %v rev %d→%d", c, err, rev, r.View.Rev)
		}
		if plan, err := a.PlanBackgroundTo(c.ts, c.id); !errors.Is(err, c.want) || plan != (model.BgPlan{}) {
			t.Fatalf("план %+v: %+v %v", c, plan, err)
		}
	}
	if r := prefs.RecentDecor(a.cfgDir, decor.KindBg); len(r) != 0 {
		t.Fatalf("недавние после отказов: %v", r)
	}
}

// Фон на набор листов (077): встроенный и свой на развороты одним шагом, обложка не цель,
// вопрос — по числу фото фона; повтор — холостая; файл помнит; отмена — по шагу на правку.
func TestAppApplyBackgroundTo(t *testing.T) {
	a, own, all := bgToApp(t)
	if plan, err := a.PlanBackgroundTo(all, "sa:cream-gradient"); err != nil || plan != (model.BgPlan{Spreads: 2}) {
		t.Fatalf("план узора: %+v %v", plan, err)
	}
	r, err := a.ApplyBackgroundTo(all, "sa:cream-gradient")
	if err != nil || r.Spreads != 2 {
		t.Fatalf("узор на все: %v %+v", err, r.Spreads)
	}
	for _, s := range r.View.Spreads {
		if len(s.Patterns) != 1 || s.Patterns[0].Preset != "sa:cream-gradient" || s.Bg[0] != "#fbf6ef" {
			t.Fatalf("%s: %+v %v", s.ID, s.Patterns, s.Bg)
		}
	}
	if c := r.View.Cover.Sheet; len(c.Patterns) != 0 {
		t.Fatalf("обложка задета: %+v", c.Patterns)
	}
	again, err := a.ApplyBackgroundTo(all, "sa:cream-gradient")
	if err != nil || again.Spreads != 0 || again.View.Rev != r.View.Rev {
		t.Fatalf("повтор: %v изменено %d rev %d→%d", err, again.Spreads, r.View.Rev, again.View.Rev)
	}

	// Своё на левые: узор листа уходит правой; второй раз другое фото — вопрос о двух фото.
	left := []model.GridTarget{{Spread: "s1", Scope: "left"}, {Spread: "s2", Scope: "left"}}
	if r, err = a.ApplyBackgroundTo(left, own); err != nil || r.Spreads != 2 {
		t.Fatalf("своё на левые: %v %d", err, r.Spreads)
	}
	for _, s := range r.View.Spreads {
		if len(s.Backdrops) != 1 || s.Backdrops[0].Lib != own || len(s.Patterns) != 1 || s.Patterns[0].Scope != "right" {
			t.Fatalf("%s: фото %+v узоры %+v", s.ID, s.Backdrops, s.Patterns)
		}
	}
	if plan, err := a.PlanBackgroundTo(left, "sa:gold-dots"); err != nil || plan != (model.BgPlan{Spreads: 2, Photos: 2}) {
		t.Fatalf("план замены фото: %+v %v", plan, err)
	}
	if got := prefs.RecentDecor(a.cfgDir, decor.KindBg); len(got) != 2 || got[0] != own || got[1] != "sa:cream-gradient" {
		t.Fatalf("недавние: %v", got)
	}

	savedBgTo(t, a, own)

	// Одна правка — один шаг: две отмены возвращают альбом без фона.
	a.Undo()
	v := a.Undo()
	for _, s := range v.Spreads {
		if len(s.Patterns) != 0 || len(s.Backdrops) != 0 {
			t.Fatalf("после двух отмен, %s: %+v %+v", s.ID, s.Patterns, s.Backdrops)
		}
	}
}

// savedBgTo — круг Save → Load после «своё на левые» поверх узора на весь лист: своё ключом
// слева, узор плитки справа на каждом развороте.
func savedBgTo(t *testing.T, a *App, own string) {
	t.Helper()
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Spreads {
		if s.BgPhotoL == nil || s.BgPhotoL.Photo != model.LibPhotoID(own) || s.PatR == nil || s.PatR.Preset != "sa:cream-gradient" {
			t.Fatalf("после Load, %s: %+v %+v", s.ID, s.BgPhotoL, s.PatR)
		}
	}
}
