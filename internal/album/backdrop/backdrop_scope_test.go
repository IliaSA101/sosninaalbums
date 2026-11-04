package backdrop_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Неизвестная область (мусор из интерфейса) — холостая правка фона: ни шага, ни rev.
func TestBackdropUnknownScopeNoop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "spread", "bg")
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetBackdropFrame(s1, "середина", 2, 0.5, 0.5)
	e.FitBackdrop(s1, "середина")
	e.ClearBackdrop(s1, "середина")
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
	if edtest.BdOf(e, s1, "spread") == nil {
		t.Fatal("фон разворота пропал от чужой области")
	}
}

// Тела правок узора без обёртки Editor (061): корешок обложки у узора — ErrScope или холостая,
// без паники; обёртка ApplyPattern отсекает его раньше, тело — страховка для прямых вызовов.
func TestPatternBodiesSpine(t *testing.T) {
	p := model.New()
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Spreads = []*model.Spread{{ID: "s1"}}
	p.Cover = &model.Cover{Spec: pr.Spec, Layout: model.CoverBoth, Sheet: &model.Spread{ID: model.CoverID}}
	pat := model.Pattern{Kind: "dots", Scale: 1}
	if err := backdrop.ApplyPattern(p, model.CoverID, model.ScopeSpine, pat, "#112233"); !errors.Is(err, model.ErrScope) {
		t.Fatalf("узор на корешок: %v", err)
	}
	backdrop.SetPattern(p, model.CoverID, model.ScopeSpine, pat)
	backdrop.ClearPattern(p, model.CoverID, model.ScopeSpine)
	if s := p.Cover.Sheet; len(s.Patterns()) != 0 || s.BgL != "" || s.BgR != "" {
		t.Fatalf("корешок задел лист: %+v", s)
	}
}
