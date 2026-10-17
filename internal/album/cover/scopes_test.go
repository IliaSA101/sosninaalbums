package cover_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestScopesNoPanic — страж 061: каждый метод Editor с областью получает "spine" на обложке
// (в both с фото корешка и в front), на развороте и на листе 031. Паники нет; ошибка — только
// ErrScope (у шаблона разворота — ErrTplScope); на развороте и листе — холостая правка; на обложке меняют проект только методы
// фото фона, узоры, цвет, сетка и раскладка корешок не принимают.
func TestScopesNoPanic(t *testing.T) {
	const sp = model.ScopeSpine
	pat := model.Pattern{Kind: "dots", Scale: 1}
	frac := []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}
	type call struct {
		name  string
		photo bool // правка фото фона: на обложке в both меняет проект
		tpl   bool // шаблон разворота на чужую область — прежний отказ ErrTplScope (как у любой не spread)
		do    func(e *album.Editor, id string) error
	}
	calls := []call{
		{"SetBackground", false, false, func(e *album.Editor, id string) error { e.SetBackground(id, sp, "#112233"); return nil }},
		{"MirrorGrid", false, false, func(e *album.Editor, id string) error { e.MirrorGrid(id, sp); return nil }},
		{"ApplyGrid", false, false, func(e *album.Editor, id string) error {
			_, _, err := e.ApplyGrid(id, sp, model.TplPage, frac)
			return err
		}},
		{"ApplyGridSpread", false, true, func(e *album.Editor, id string) error {
			_, _, err := e.ApplyGrid(id, sp, model.TplSpread, frac)
			return err
		}},
		{"PlaceBoxes", false, false, func(e *album.Editor, id string) error { _, _, _ = e.PlaceBoxes(id, sp); return nil }},
		{"GridChoices", false, false, func(e *album.Editor, id string) error { e.GridChoices(id, 2, sp, nil); return nil }},
		{"StepLayout", false, false, func(e *album.Editor, id string) error { e.StepLayout(id, sp, 1); return nil }},
		{"ApplyPattern", false, false, func(e *album.Editor, id string) error { _, err := e.ApplyPattern(id, sp, pat, ""); return err }},
		{"SetPattern", false, false, func(e *album.Editor, id string) error { e.SetPattern(id, sp, pat, ""); return nil }},
		{"ClearPattern", false, false, func(e *album.Editor, id string) error { e.ClearPattern(id, sp); return nil }},
		{"SetBackdrop", true, false, func(e *album.Editor, id string) error { _, err := e.SetBackdrop(id, sp, info(pid(2))); return err }},
		{"ApplyBackdrop", true, false, func(e *album.Editor, id string) error { _, err := e.ApplyBackdrop(id, sp, info(pid(4))); return err }},
		{"SetBackdropBox", true, false, func(e *album.Editor, id string) error { e.SetBackdropBox(id, sp, 300, 5, 20, 100, 0, ""); return nil }},
		{"SetBackdropFrame", true, false, func(e *album.Editor, id string) error { e.SetBackdropFrame(id, sp, 1.5, 0.1, 0); return nil }},
		{"SetBackdropAngle", true, false, func(e *album.Editor, id string) error { e.SetBackdropAngle(id, sp, 2, ""); return nil }},
		{"StraightenBackdrop", true, false, func(e *album.Editor, id string) error { e.StraightenBackdrop(id, sp, 10, 1); return nil }},
		{"FlipBackdrop", true, false, func(e *album.Editor, id string) error { e.FlipBackdrop(id, sp, "v"); return nil }},
		{"SetBackdropFade", true, false, func(e *album.Editor, id string) error { e.SetBackdropFade(id, sp, 0.3, ""); return nil }},
		{"FitBackdrop", true, false, func(e *album.Editor, id string) error { e.FitBackdrop(id, sp); return nil }},
		{"ClearBackdrop", true, false, func(e *album.Editor, id string) error { e.ClearBackdrop(id, sp); return nil }},
	}
	type target struct {
		name    string
		ed      func(t *testing.T) (*album.Editor, string)
		changes bool // методы фото фона меняют проект
	}
	targets := []target{
		{"обложка both", func(t *testing.T) (*album.Editor, string) { return spineAlbum(t, 6), model.CoverID }, true},
		{"обложка front", func(t *testing.T) (*album.Editor, string) { return coverAlbum(t, 6), model.CoverID }, false},
		{"разворот", func(t *testing.T) (*album.Editor, string) {
			e := coverAlbum(t, 6)
			return e, edtest.P(e).Spreads[2].ID
		}, false},
		// Место на корешке не у центра: «Отразить» по корешку сдвинуло бы его.
		{"обложка с местом на корешке", func(t *testing.T) (*album.Editor, string) {
			e := spineAlbum(t, 6)
			if _, err := e.AddPlace(model.CoverID, 1, ""); err != nil {
				t.Fatal(err)
			}
			pl, cg := sheet(e).Grid[len(sheet(e).Grid)-1], edtest.P(e).CoverGeo()
			e.SetPlaces(model.CoverID, []model.PlaceBox{{ID: pl.ID, X: cg.X[2], Y: 100, W: 3, H: 20}}, "")
			return e, model.CoverID
		}, true},
		{"лист 031", func(t *testing.T) (*album.Editor, string) {
			p := model.NewSheet(200, 300)
			p.Photos[pid(0)] = model.PhotoRef{Path: edtest.Abs(pid(0) + ".jpg"), W: 300, H: 200}
			p.Spreads[0].Photos = []string{pid(0)}
			e := edtest.Open(t, p, "")
			return e, edtest.P(e).Spreads[0].ID
		}, false},
	}
	for _, tg := range targets {
		for _, c := range calls {
			t.Run(fmt.Sprintf("%s/%s", tg.name, c.name), func(t *testing.T) {
				e, id := tg.ed(t)
				before, rev, undo := string(model.Marshal(edtest.P(e))), edtest.Rev(e), edtest.UndoLen(e)
				var err error
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("паника: %v", r)
						}
					}()
					err = c.do(e, id)
				}()
				if err != nil && !errors.Is(err, model.ErrScope) && !c.tpl {
					t.Fatalf("ошибка не ErrScope: %v", err)
				}
				changed := string(model.Marshal(edtest.P(e))) != before
				if want := tg.changes && c.photo && c.name != "FitBackdrop"; changed != want {
					t.Fatalf("проект изменился: %v, ждали %v (ошибка %v)", changed, want, err)
				}
				if !changed {
					edtest.Same(t, e, rev, undo, "холостая")
				}
				if tg.changes && c.photo && err != nil {
					t.Fatalf("фото корешка в both: %v", err)
				}
			})
		}
	}
}
