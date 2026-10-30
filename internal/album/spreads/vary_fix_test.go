package spreads_test

// Спека 041, доправка после ревью: устаревший вход сборки (Prepare до замка), пулы вне замка,
// круг Save → Load, Redo «Разнообразить», соседи при переносе фото, предел PhotosOf.

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// staleBase — альбом из двух разворотов («Меньше») с первыми 16 фото набора ph.
func staleBase(t *testing.T, ph []model.PhotoInfo) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	build(t, e, ph[:16], -1, model.AutoOpts{Spreads: 2, MaxPer: 8})
	return e
}

// TestAutoBuildStalePrep: между Prepare (до замка) и AutoBuild (под замком) проект поменялся —
// сборка с таким pre даёт то же, что свежая без него: фото, раскладки и ключи. Сетка у разворота
// (её фото из сборки уходят), поворот фото (другие пропорции), другой пустой хвост.
func TestAutoBuildStalePrep(t *testing.T) {
	ph := mixedPhotos("v", 40)
	mutations := map[string]func(t *testing.T, e *album.Editor){
		"сетка": func(t *testing.T, e *album.Editor) { edtest.FreezeAt(t, e, 0) },
		"поворот": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) {
				r := p.Photos[ph[3].ID]
				r.Frame = &model.Frame{S: 1, R: 90}
				p.Photos[ph[3].ID] = r
			})
		},
		"хвост": func(t *testing.T, e *album.Editor) {
			if _, err := e.NewSpread(-1, nil); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range mutations {
		for _, v := range []model.Variety{model.VarietyLess, model.VarietyNormal, model.VarietyMore} {
			o := model.AutoOpts{Spreads: 4, MaxPer: 12, Variety: v}
			stale := staleBase(t, ph)
			pre := spreads.Prepare(spreads.PlanOf(edtest.P(stale), ph, nil), ph, o)
			mutate(t, stale)
			if _, err := edtest.Try(stale, "", func(p *model.Project) error {
				_, err := spreads.AutoBuild(p, pre, -1, nil)
				return err
			}); err != nil {
				t.Fatalf("%s, Variety %d: %v", name, v, err)
			}
			fresh := staleBase(t, ph)
			mutate(t, fresh)
			build(t, fresh, ph, -1, o)
			got, want := spreadsShape(edtest.P(stale)), spreadsShape(edtest.P(fresh))
			if len(want) < 4 || !slices.Equal(got, want) {
				t.Fatalf("%s, Variety %d: устаревший pre дал %v, свежая сборка %v", name, v, got, want)
			}
		}
	}
}

// TestAutoBuildPoolsOutsideLock: пулы, посчитанные вне замка (параллельно), и пулы под замком
// (выключены крючком теста) на смешанных фото дают одни и те же ключи на всех уровнях.
func TestAutoBuildPoolsOutsideLock(t *testing.T) {
	ph := mixedPhotos("v", 60)
	run := func(par bool, v model.Variety) []string {
		model.SetParallelLayouts(par)
		t.Cleanup(func() { model.SetParallelLayouts(true) })
		layout.ResetCache()
		variety.ResetCache()
		e := edtest.Open(t, model.New(), "")
		build(t, e, ph, -1, model.AutoOpts{Spreads: 8, MaxPer: 12, Variety: v})
		return spreadsShape(edtest.P(e))
	}
	for _, v := range levels {
		off, on := run(false, v), run(true, v)
		if len(off) != 8 || !slices.Equal(on, off) {
			t.Fatalf("Variety %d: пулы вне замка %v, под замком %v", v, on, off)
		}
	}
}

// TestPrepareWarmsExtra: «Другие варианты» греются в Prepare на любом уровне, и на «Меньше»
// тоже (пулов там нет): счётчик «i / N» в View берёт их под замком.
func TestPrepareWarmsExtra(t *testing.T) {
	ph := mixedPhotos("w", 6) // один разворот
	asp := make([]float64, len(ph))
	for i, x := range ph {
		asp[i] = float64(x.W) / float64(x.H)
	}
	prm := edtest.P(edtest.Open(t, model.New(), "")).Params()
	for _, v := range levels {
		variety.ResetCache()
		if variety.Cached(asp, prm, model.RowLen(len(asp))) {
			t.Fatal("кэш не холодный")
		}
		e := edtest.Open(t, model.New(), "")
		spreads.Prepare(spreads.PlanOf(edtest.P(e), ph, nil), ph, model.AutoOpts{Spreads: 1, MaxPer: 64, Variety: v})
		if !variety.Cached(asp, prm, model.RowLen(len(asp))) {
			t.Fatalf("Variety %d: «Другие варианты» не прогреты в Prepare", v)
		}
	}
}

// TestVarietySaveLoad: сборка «Обычное» и «Больше» на смешанных фото и «Разнообразить» — после
// Save → Load те же раскладки, счётчики «i / N» (решение 0.11 №36) и прямоугольники, включая
// ключи из «Других вариантов» и страницы с модификаторами («воздух»).
func TestVarietySaveLoad(t *testing.T) {
	ph := mixedPhotos("v", 120)
	extra, air := false, false
	for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
		e := edtest.Open(t, model.New(), "")
		view := build(t, e, ph, -1, model.AutoOpts{Spreads: 16, MaxPer: 12, Variety: v})
		view = e.VaryLayouts(edtest.SpreadIDs(view)[3:11], v, nil, nil)
		p := edtest.P(e)
		for _, s := range p.Spreads {
			asp, prm := p.Aspects(s.Photos), p.Params()
			extra = extra || model.IndexOf(model.ExtraRow(asp, prm), s.Layout) >= 0
			air = air || variety.Describe(mustBuild(t, s.Layout, asp, prm), prm).Family == variety.Air
		}
		path := filepath.Join(testkit.TempDir(t), "v.salbum")
		if err := e.Save(path); err != nil {
			t.Fatal(err)
		}
		q, err := album.LoadBase(path, filepath.Dir(path)) // то же, что salbum.Load: Decode от папки файла
		if err != nil {
			t.Fatal(err)
		}
		got := edtest.Open(t, q, path).View()
		if len(got.Spreads) != len(view.Spreads) {
			t.Fatalf("Variety %d: разворотов %d → %d", v, len(view.Spreads), len(got.Spreads))
		}
		for i, w := range view.Spreads {
			g := got.Spreads[i]
			if g.Layout != w.Layout || g.Variant != w.Variant || g.Variants != w.Variants || !slices.Equal(g.Rects, w.Rects) {
				t.Fatalf("Variety %d, разворот %d: после Load %q %d/%d, было %q %d/%d", v, i, g.Layout, g.Variant, g.Variants, w.Layout, w.Variant, w.Variants)
			}
			if w.Layout != "" && w.Variant < 0 {
				t.Fatalf("Variety %d, разворот %d: ключ %q вне ряда «i / N»", v, i, w.Layout)
			}
		}
	}
	if !extra || !air {
		t.Fatalf("в сборках нет ключей из «Других вариантов» (%v) или воздуха (%v): тест не проверяет их круг", extra, air)
	}
}

// mustBuild — раскладка по ключу (пусто — лучшая); не строится — паника теста.
func mustBuild(t *testing.T, key string, asp []float64, prm layout.Params) layout.Result {
	t.Helper()
	if key == "" {
		return layout.Cached(asp, prm, model.RowLen(len(asp)))[0]
	}
	r, ok := layout.Build(key, asp, prm)
	if !ok {
		t.Fatalf("ключ %q не строится", key)
	}
	return r
}

// TestVaryLayoutsRedo: Undo возвращает проект, Redo — те же раскладки; и сам Redo — один шаг,
// rev растёт, CanUndo/CanRedo верны.
func TestVaryLayoutsRedo(t *testing.T) {
	e, ids := varyEd(t)
	before := model.Marshal(edtest.P(e))
	e.VaryLayouts(ids, model.VarietyNormal, nil, nil)
	after, shape := model.Marshal(edtest.P(e)), spreadsShape(edtest.P(e))
	if string(after) == string(before) {
		t.Fatal("«Разнообразить» ничего не поменяло")
	}
	undo := e.Undo()
	if !undo.CanRedo || string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatal("отмена не вернула проект")
	}
	redo := e.Redo()
	if redo.Rev <= undo.Rev || !redo.CanUndo || redo.CanRedo {
		t.Fatalf("Redo: rev %d → %d, CanUndo %v, CanRedo %v", undo.Rev, redo.Rev, redo.CanUndo, redo.CanRedo)
	}
	if got := spreadsShape(edtest.P(e)); !slices.Equal(got, shape) || string(model.Marshal(edtest.P(e))) != string(after) {
		t.Fatalf("Redo вернул %v, ждали %v", got, shape)
	}
	// ещё круг: Undo после Redo снова возвращает исходный проект
	if e.Undo(); string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatal("вторая отмена не вернула проект")
	}
}

// TestAutoBuildMiddleMovesNeighbor: вставка в середину фото из соседнего разворота (он опустеет и
// исчезнет) выбирает раскладку так же, как если бы соседа не было: опустевший не сосед.
func TestAutoBuildMiddleMovesNeighbor(t *testing.T) {
	ph := horizontals("h", 48)
	for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
		// шесть разворотов по 8; фото пятого (ph[32:40]) уезжают в новый разворот после него
		moved := edtest.Open(t, model.New(), "")
		build(t, moved, ph, -1, model.AutoOpts{Spreads: 6, MaxPer: 8})
		build(t, moved, ph[32:40], 5, model.AutoOpts{Spreads: 1, MaxPer: 8, Variety: v})
		// то же без пятого разворота вовсе
		ref := edtest.Open(t, model.New(), "")
		build(t, ref, slices.Concat(ph[:32], ph[40:]), -1, model.AutoOpts{Spreads: 5, MaxPer: 8})
		build(t, ref, ph[32:40], 4, model.AutoOpts{Spreads: 1, MaxPer: 8, Variety: v})
		got, want := spreadsShape(edtest.P(moved)), spreadsShape(edtest.P(ref))
		if len(got) != 6 || !slices.Equal(got, want) {
			t.Fatalf("Variety %d: с переносом %v, без соседа %v", v, got, want)
		}
	}
}

// TestPhotosOfLimit: ID дальше MaxSpreads не разбираются (как у DeleteSpreads); до предела — все.
func TestPhotosOfLimit(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	for _, c := range []struct{ n, want int }{
		{0, 0}, {1, 2}, {model.MaxSpreads, 2 * model.MaxSpreads}, {model.MaxSpreads + 1, 2 * model.MaxSpreads}, {3 * model.MaxSpreads, 2 * model.MaxSpreads},
	} {
		ids := make([]string, c.n)
		for i := range ids {
			ids[i] = s1
		}
		if got := e.PhotosOf(ids); len(got) != c.want {
			t.Fatalf("%d ID: фото %d, ждали %d", c.n, len(got), c.want)
		}
	}
}
