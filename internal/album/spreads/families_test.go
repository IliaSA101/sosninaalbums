package spreads_test

// Спека 065: семейства раскладок в автосборке и «Разнообразить» — пул сужается до выбора,
// разрешённых нет — лучшая; «Все» — поведение 0.14 побитово.

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

var allFamilies = []string{"dense", "hero", "air", "bleed", "strip", "figure"}

// famOf — семейство действующей раскладки, как у плитки подборки (Choice.family).
func famOf(p *model.Project, r layout.Result) string {
	return string(variety.FamilyOf(forms.Of(r.Key, r.Rects, p.Params())))
}

// famCheck — у каждого разворота «Авто» с фото раскладка разрешённого семейства или лучшая
// (запасной). Возвращает развороты с разрешённой и запасных.
func famCheck(t *testing.T, e *album.Editor, fs []string) (hit, fallback int) {
	t.Helper()
	p := edtest.P(e)
	for i, s := range p.Spreads {
		r, best, ok := p.Layout(s)
		if !ok || len(s.Grid) > 0 {
			continue
		}
		if slices.Contains(fs, famOf(p, r)) {
			hit++
			continue
		}
		if r.Key != best[0].Key {
			t.Errorf("%v: разворот %d — %s (%s), не разрешённая и не лучшая", fs, i, famOf(p, r), r.Key)
		}
		fallback++
	}
	return hit, fallback
}

// TestAutoBuildFamiliesAllIsOld: nil, пусто, все шесть (в любом порядке, с повторами) и только
// мусор — одни и те же развороты и ключи на каждом уровне: «Все» = 0.14 (эталоны не меняются).
func TestAutoBuildFamiliesAllIsOld(t *testing.T) {
	ph := mixedPhotos("o", 60)
	for _, v := range levels {
		run := func(fs []string) []string {
			e := edtest.Open(t, model.New(), "")
			build(t, e, ph, -1, model.AutoOpts{Spreads: 10, MaxPer: 12, Variety: v, Families: fs})
			return spreadsShape(edtest.P(e))
		}
		old := run(nil)
		mixed := append(slices.Clone(allFamilies), "dense", "мусор")
		slices.Reverse(mixed)
		for _, fs := range [][]string{{}, allFamilies, mixed, {"мусор", ""}} {
			if got := run(fs); !slices.Equal(got, old) {
				t.Errorf("уровень %d, %q: %v ≠ без семейств %v", v, fs, got, old)
			}
		}
	}
}

// TestAutoBuildFamiliesDense: на «Обычном» и «Больше» каждый разворот — из разрешённых семейств
// или запасной = лучшая; одно семейство и набор; фильтр действует (без него ленты и прочее были бы
// не лучшими); число запасных совпадает с AutoPreview.
func TestAutoBuildFamiliesDense(t *testing.T) {
	ph := mixedPhotos("d", 90)
	for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
		for _, fs := range [][]string{{"dense"}, {"dense", "hero"}, {"strip"}, {"figure", "air"}} {
			e := edtest.Open(t, model.New(), "")
			o := model.AutoOpts{Spreads: 16, MaxPer: 12, Variety: v, Families: fs}
			pv, err := e.AutoPreview(ph, -1, o)
			if err != nil {
				t.Fatal(err)
			}
			build(t, e, ph, -1, o)
			hit, fallback := famCheck(t, e, fs)
			if hit == 0 || hit+fallback != 16 {
				t.Errorf("уровень %d, %q: разрешённых %d, запасных %d из 16", v, fs, hit, fallback)
			}
			if pv.Fallback != fallback {
				t.Errorf("уровень %d, %q: предпросмотр обещал %d запасных, собрано %d", v, fs, pv.Fallback, fallback)
			}
		}
	}
}

// TestAutoBuildFamiliesLess: «Меньше» + фильтр — первая разрешённая (лучшая — Layout ""), запасной —
// лучшая; «Меньше» + «Все» — Layout "" у всех, как 0.10–0.14.
func TestAutoBuildFamiliesLess(t *testing.T) {
	ph := mixedPhotos("l", 60)
	for _, fs := range [][]string{nil, {}, allFamilies} {
		e := edtest.Open(t, model.New(), "")
		build(t, e, ph, -1, model.AutoOpts{Spreads: 10, MaxPer: 12, Variety: model.VarietyLess, Families: fs})
		for _, s := range spreadsShape(edtest.P(e)) {
			if s[len(s)-1] != '|' {
				t.Fatalf("«Меньше» + %q записало ключ: %s", fs, s)
			}
		}
	}
	// Малые развороты (по 2 фото): другой состав семейств в пуле (воздух, в край).
	small := mixedPhotos("k", 24)
	for _, c := range []struct {
		ph []model.PhotoInfo
		n  int
		fs []string
	}{{ph, 10, []string{"dense"}}, {ph, 10, []string{"strip"}}, {ph, 10, []string{"hero", "figure"}}, {ph, 10, []string{"air"}},
		{small, 12, []string{"bleed"}}, {small, 12, []string{"air"}}, {small, 12, []string{"hero"}}} {
		fs := c.fs
		e := edtest.Open(t, model.New(), "")
		build(t, e, c.ph, -1, model.AutoOpts{Spreads: c.n, MaxPer: 12, Variety: model.VarietyLess, Families: fs})
		hit, _ := famCheck(t, e, fs)
		if hit == 0 {
			t.Errorf("«Меньше» + %q: ни одного разрешённого", fs)
		}
		p := edtest.P(e)
		for i, s := range p.Spreads {
			r, best, _ := p.Layout(s)
			want := best[0].Key // запасной
			for _, b := range poolKeys(p, s) {
				if slices.Contains(fs, b.fam) {
					want = b.key // первая разрешённая в порядке пула
					break
				}
			}
			if r.Key != want {
				t.Errorf("«Меньше» + %q: разворот %d — %s, ждали первую разрешённую %s", fs, i, r.Key, want)
			}
		}
	}
}

type keyFam struct{ key, fam string }

// poolKeys — пул выбора разворота в порядке подборки: ряд «Авто», затем «Другие варианты».
func poolKeys(p *model.Project, s *model.Spread) []keyFam {
	asp, prm := p.Aspects(s.Photos), p.Params()
	row, _ := model.MoreRow(asp, prm, 0)
	var out []keyFam
	for _, r := range slices.Concat(row, model.ExtraRow(asp, prm)) {
		out = append(out, keyFam{r.Key, famOf(p, r)})
	}
	return out
}

// TestVaryLayoutsFamilies: «Разнообразить» с «Лентами» — только ленты и запасные, один шаг отмены;
// только мусор — как без семейств; нечего менять — холостая (rev и отмена те же).
func TestVaryLayoutsFamilies(t *testing.T) {
	ph := mixedPhotos("w", 72)
	open := func() (*album.Editor, []string) {
		e := edtest.Open(t, model.New(), "")
		return e, edtest.SpreadIDs(build(t, e, ph, -1, model.AutoOpts{Spreads: 12, MaxPer: 12, Variety: model.VarietyNormal}))
	}
	e, ids := open()
	undo := edtest.UndoLen(e)
	before := spreadsShape(edtest.P(e))
	e.VaryLayouts(ids, model.VarietyNormal, []string{"strip"}, nil)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d → %d, ждали один", undo, edtest.UndoLen(e))
	}
	if hit, _ := famCheck(t, e, []string{"strip"}); hit == 0 {
		t.Error("ни одной ленты")
	}
	if slices.Equal(spreadsShape(edtest.P(e)), before) {
		t.Error("«Ленты» ничего не поменяли")
	}
	e.Undo()
	if !slices.Equal(spreadsShape(edtest.P(e)), before) {
		t.Error("отмена не вернула раскладки")
	}

	// Мусор = «Все»: тот же итог, что без семейств.
	a, idsA := open()
	b, idsB := open()
	a.VaryLayouts(idsA, model.VarietyNormal, nil, nil)
	b.VaryLayouts(idsB, model.VarietyNormal, []string{"мусор"}, nil)
	if !slices.Equal(spreadsShape(edtest.P(a)), spreadsShape(edtest.P(b))) {
		t.Error("семейства из мусора дали не то, что без семейств")
	}

	// Холостая: неизвестные развороты.
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	if v := e.VaryLayouts([]string{"нет"}, model.VarietyNormal, []string{"strip"}, nil); v.Rev != rev || edtest.UndoLen(e) != undo {
		t.Errorf("холостая: rev %d → %d, отмена %d → %d", rev, v.Rev, undo, edtest.UndoLen(e))
	}
}

// TestAutoBuildFamiliesUndo: сборка с семействами — ровно один шаг отмены, на каждом уровне и для
// нескольких наборов (в конец и в начало альбома, где уже есть развороты); отмена возвращает проект
// побайтно, повтор — тот же собранный (как у «Разнообразить» с «Лентами»).
func TestAutoBuildFamiliesUndo(t *testing.T) {
	first, more := mixedPhotos("u", 30), mixedPhotos("v", 60)
	for _, v := range levels {
		for _, c := range []struct {
			at int
			fs []string
		}{{-1, []string{"strip"}}, {0, []string{"dense", "hero"}}, {-1, []string{"figure", "air", "bleed"}}} {
			e := edtest.Open(t, model.New(), "")
			build(t, e, first, -1, model.AutoOpts{Spreads: 5, MaxPer: 12, Variety: model.VarietyNormal})
			before := string(model.Marshal(edtest.P(e)))
			undo, spreadsN := edtest.UndoLen(e), len(edtest.P(e).Spreads)

			build(t, e, more, c.at, model.AutoOpts{Spreads: 8, MaxPer: 12, Variety: v, Families: c.fs})
			built := string(model.Marshal(edtest.P(e)))
			if edtest.UndoLen(e) != undo+1 {
				t.Fatalf("уровень %d, %q: шагов отмены %d → %d, ждали один", v, c.fs, undo, edtest.UndoLen(e))
			}
			if len(edtest.P(e).Spreads) <= spreadsN || built == before {
				t.Fatalf("уровень %d, %q: сборка ничего не добавила", v, c.fs)
			}
			e.Undo()
			if got := string(model.Marshal(edtest.P(e))); got != before {
				t.Errorf("уровень %d, %q: отмена вернула другой проект", v, c.fs)
			}
			if edtest.UndoLen(e) != undo || edtest.RedoLen(e) != 1 {
				t.Errorf("уровень %d, %q: после отмены undo %d, redo %d", v, c.fs, edtest.UndoLen(e), edtest.RedoLen(e))
			}
			e.Redo()
			if got := string(model.Marshal(edtest.P(e))); got != built {
				t.Errorf("уровень %d, %q: повтор вернул другой проект", v, c.fs)
			}
		}
	}
}

// TestAutoBuildFamiliesLessSaveLoad: «Меньше» + фильтр впервые пишет непустой Layout на этом уровне —
// он переживает Save→Load (ключи и фото разворотов те же); «Меньше» + «Все» остаётся с пустыми.
func TestAutoBuildFamiliesLessSaveLoad(t *testing.T) {
	ph := mixedPhotos("s", 60)
	for _, fs := range [][]string{{"dense"}, {"strip"}, {"hero", "figure"}, {"air"}, nil} {
		e := edtest.Open(t, model.New(), "")
		build(t, e, ph, -1, model.AutoOpts{Spreads: 10, MaxPer: 12, Variety: model.VarietyLess, Families: fs})
		want := layoutsOf(edtest.P(e))
		keyed := 0
		for _, k := range want {
			if k != "" {
				keyed++
			}
		}
		if (fs != nil) != (keyed > 0) {
			t.Fatalf("«Меньше» + %q: непустых ключей %d (фильтр должен их писать, «Все» — нет)", fs, keyed)
		}
		// ID фото при загрузке пересчитываются от пути — сравниваем ключи и число фото.
		q := edtest.SaveLoad(t, e)
		if got := layoutsOf(q); !slices.Equal(got, want) {
			t.Errorf("«Меньше» + %q: после Save→Load %q, было %q", fs, got, want)
		}
		for i, s := range edtest.P(e).Spreads {
			if len(q.Spreads[i].Photos) != len(s.Photos) {
				t.Errorf("«Меньше» + %q: разворот %d — фото %d → %d", fs, i, len(s.Photos), len(q.Spreads[i].Photos))
			}
		}
	}
}

// TestVaryLayoutsFamiliesSame: «Разнообразить» с фильтром, когда вид уже не меняется (после первого
// прохода «Лент» второй ничего не находит), — без шага отмены и без правки проекта; первый проход
// при этом шаг даёт.
func TestVaryLayoutsFamiliesSame(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ids := edtest.SpreadIDs(build(t, e, mixedPhotos("n", 72), -1, model.AutoOpts{Spreads: 12, MaxPer: 12, Variety: model.VarietyNormal}))
	fs := []string{"strip"}
	undo := edtest.UndoLen(e)
	e.VaryLayouts(ids, model.VarietyNormal, fs, nil)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("первый проход: шагов отмены %d → %d, ждали один", undo, edtest.UndoLen(e))
	}
	edtest.SetDirty(e, false)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	before := string(model.Marshal(edtest.P(e)))
	e.VaryLayouts(ids, model.VarietyNormal, fs, nil)
	edtest.Same(t, e, rev, undo, "повтор «Разнообразить» с «Лентами»")
	if edtest.Dirty(e) || string(model.Marshal(edtest.P(e))) != before {
		t.Error("холостой повтор тронул проект или пометил его изменённым")
	}
}
