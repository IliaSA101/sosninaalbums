package spreads_test

// 064 «Фото через сгиб»: смена положения в настройках альбома и прямой ключ при правиле
// «Только широкие» (decisions 0.15 №7, №9, №17).

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// foldEd — альбом из трёх разворотов: три вертикальных 2:3 (первая «Любых» кладёт узкое фото
// на корешок, «Широких» — нет), два горизонтальных (первая одна во всех положениях), три
// вертикальных + горизонтальное. foldAny — положение до сборки: развороты, собранные при одном
// положении, при смене на другое закрепляются (KeepLayouts), и проверять было бы нечего.
func foldEd(t *testing.T, foldAny bool) (*album.Editor, []string) {
	t.Helper()
	v, h := [2]int{200, 300}, [2]int{300, 200}
	e := edtest.Open(t, model.New(), "")
	if st := edtest.P(e).Settings; !st.Layflat || st.FoldAny {
		t.Fatalf("новый альбом: layflat %v, foldAny %v — ждали «Только широкие»", st.Layflat, st.FoldAny)
	}
	undo := edtest.UndoLen(e)
	setFold(e, true, foldAny)
	if edtest.P(e).Settings.FoldAny != foldAny || edtest.UndoLen(e) != undo+b2i(foldAny) {
		t.Fatalf("положение не встало: foldAny %v, шагов %d", edtest.P(e).Settings.FoldAny, edtest.UndoLen(e)-undo)
	}
	edtest.NewSpread(e, -1, whPhotos("v", v, v, v))
	edtest.NewSpread(e, -1, whPhotos("h", h, h))
	return e, edtest.SpreadIDs(edtest.NewSpread(e, -1, whPhotos("k", v, v, v, h)))
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// setFold — настройки альбома с положением сгиба (layflat, foldAny), остальное как есть.
func setFold(e *album.Editor, layflat, foldAny bool) model.View {
	st := edtest.P(e).Settings
	st.Layflat, st.FoldAny = layflat, foldAny
	return e.SetSettings(st)
}

// autoKey — первая раскладка разворота i при геометрии prm.
func autoKey(t *testing.T, e *album.Editor, i int, prm layout.Params) layout.Result {
	t.Helper()
	p := edtest.P(e)
	asp := p.Aspects(p.Spreads[i].Photos)
	row := layout.Cached(asp, prm, model.RowLen(len(asp)))
	if len(row) == 0 {
		t.Fatalf("разворот %d: ряд пуст", i)
	}
	return row[0]
}

func rectsOf(v model.View) [][]layout.Rect {
	out := make([][]layout.Rect, len(v.Spreads))
	for i, s := range v.Spreads {
		out[i] = s.Rects
	}
	return out
}

func layoutsOf(p *model.Project) []string {
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = s.Layout
	}
	return out
}

// TestSetSettingsFoldKeepsSpreads — «Любые фото» → «Только широкие» одним шагом отмены: свёрстанное
// не меняется — «Авто», чья первая меняется, закрепляется прежним ключом, где не меняется —
// остаётся "", выбранный руками ключ стоит (decisions 0.15 №17). Повтор — холостой. Save → Load не
// закрепляет заново. «Любые» → «Нет»: фото через корешок раскладываются заново (№9), FoldAny
// без layflat обнуляется зажимом.
func TestSetSettingsFoldKeepsSpreads(t *testing.T) {
	e, ids := foldEd(t, true)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	setFold(e, true, true)
	edtest.Same(t, e, rev, undo, "повтор «Любые фото»")

	anyPrm := edtest.P(e).Params()
	widePrm := anyPrm
	widePrm.FoldAny = false
	a0, w0 := autoKey(t, e, 0, anyPrm), autoKey(t, e, 0, widePrm)
	p := edtest.P(e)
	if asp := p.Aspects(p.Spreads[0].Photos); a0.Key == w0.Key || layout.FoldOK(a0.Rects, asp, widePrm) {
		t.Fatalf("три вертикальных: первая «Любых» %q, «Широких» %q — тест не проверяет", a0.Key, w0.Key)
	}
	if a1, w1 := autoKey(t, e, 1, anyPrm), autoKey(t, e, 1, widePrm); a1.Key != w1.Key {
		t.Fatalf("два горизонтальных: первая меняется (%q → %q) — тест не проверяет", a1.Key, w1.Key)
	}
	k2 := edtest.PickKey(t, e, 2, func(k string) bool {
		r, ok := layout.Build(k, p.Aspects(p.Spreads[2].Photos), widePrm)
		return ok && layout.FoldOK(r.Rects, p.Aspects(p.Spreads[2].Photos), widePrm)
	})
	e.SetLayout(ids[2], k2)
	before := rectsOf(e.View())
	want := []string{a0.Key, "", k2}

	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	v := setFold(e, true, false)
	if got := layoutsOf(edtest.P(e)); !slices.Equal(got, want) || edtest.P(e).Settings.FoldAny || edtest.UndoLen(e) != undo+1 || v.Rev == rev {
		t.Fatalf("«Любые» → «Широкие»: ключи %q (ждали %q), foldAny %v, шагов %d", got, want, edtest.P(e).Settings.FoldAny, edtest.UndoLen(e)-undo)
	}
	if got := rectsOf(v); !slices.EqualFunc(got, before, slices.Equal) {
		t.Fatalf("свёрстанное сдвинулось:\n%v\n%v", got, before)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	setFold(e, true, false)
	edtest.Same(t, e, rev, undo, "повтор «Широкие»")

	q := edtest.SaveLoad(t, e)
	if got := layoutsOf(q); !slices.Equal(got, want) || q.Settings.FoldAny || !q.Settings.Layflat || edtest.From(q) != model.FormatVersion {
		t.Fatalf("Save → Load: ключи %q (ждали %q), layflat %v, foldAny %v, from %d", got, want, q.Settings.Layflat, q.Settings.FoldAny, edtest.From(q))
	}

	e.Undo()
	if p := edtest.P(e); !p.Settings.FoldAny || p.Spreads[0].Layout != "" || p.Spreads[2].Layout != k2 {
		t.Fatalf("отмена: foldAny %v, ключи %q", p.Settings.FoldAny, layoutsOf(p))
	}
	if got := rectsOf(e.View()); !slices.EqualFunc(got, before, slices.Equal) {
		t.Fatal("отмена сдвинула свёрстанное")
	}

	// «Любые» → «Нет»: FoldAny из формы зажим снимает, ни одно фото не пересекает корешок.
	undo = edtest.UndoLen(e)
	v = setFold(e, false, true)
	pw := edtest.P(e).Settings.PageW
	if st := edtest.P(e).Settings; st.Layflat || st.FoldAny || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Нет»: layflat %v, foldAny %v, шагов %d", st.Layflat, st.FoldAny, edtest.UndoLen(e)-undo)
	}
	for i, rs := range rectsOf(v) {
		for _, r := range rs {
			if r.X < pw-1e-6 && r.X+r.W > pw+1e-6 {
				t.Errorf("«Нет», разворот %d: фото %+v через корешок", i, r)
			}
		}
	}
	if edtest.P(e).Spreads[0].Layout == a0.Key {
		t.Errorf("«Нет»: ключ через корешок %q остался", a0.Key)
	}
	e.Undo()
	if st := edtest.P(e).Settings; !st.Layflat || !st.FoldAny {
		t.Fatalf("отмена «Нет»: layflat %v, foldAny %v", st.Layflat, st.FoldAny)
	}
}

// TestSetLayoutKeepsOldFoldKey — ключ старого альбома с узким фото на корешке (закреплён при
// чтении v10 или выбран при «Любых»): пока стоит — показывается, повторная установка холостая;
// тот же ключ на другом развороте и после смены раскладки не ставится (decisions 0.15 №7);
// отмена возвращает его, Save → Load сохраняет.
func TestSetLayoutKeepsOldFoldKey(t *testing.T) {
	e, ids := foldEd(t, false)
	prm := edtest.P(e).Params()
	anyPrm := prm
	anyPrm.FoldAny = true
	old := autoKey(t, e, 0, anyPrm).Key
	edtest.NewSpread(e, -1, whPhotos("x", [2]int{200, 300}, [2]int{200, 300}, [2]int{200, 300}))
	ids = append(ids, edtest.SpreadIDs(e.View())[3])
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Layout = old })
	if v := e.View(); v.Spreads[0].Layout != old {
		t.Fatalf("стоящий ключ %q не показан: %q", old, v.Spreads[0].Layout)
	}

	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetLayout(ids[0], old)
	edtest.Same(t, e, rev, undo, "повтор стоящего ключа")
	e.SetLayout(ids[3], old) // те же пропорции, но ключ там не стоит
	edtest.Same(t, e, rev, undo, "ключ вне правила на другом развороте")
	if p := edtest.P(e); p.Spreads[0].Layout != old || p.Spreads[3].Layout != "" {
		t.Fatalf("ключи %q", layoutsOf(p))
	}

	k := edtest.PickKey(t, e, 0, edtest.AnyKey)
	e.SetLayout(ids[0], k)
	if got := edtest.P(e).Spreads[0].Layout; got != k || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("ключ из ряда не встал: %q (ждали %q)", got, k)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetLayout(ids[0], old)
	edtest.Same(t, e, rev, undo, "ушедший ключ вне правила")

	e.Undo()
	if got := edtest.P(e).Spreads[0].Layout; got != old {
		t.Fatalf("отмена: %q, ждали %q", got, old)
	}
	if got := layoutsOf(edtest.SaveLoad(t, e)); got[0] != old || got[3] != "" {
		t.Fatalf("Save → Load: %q", got)
	}
}

// TestSetLayoutRejectsFoldKey — прямой ключ «в край» на разворот судит правило сгиба по кадру
// (decisions 0.15 №13): на вертикальных страницах 20×30 квадратный кадр в B[full:0] строится
// (обрезка 0,75), но его середина — на сгибе: при «Широких» холостая правка, при «Любых» ставится.
// Кадр 1,5 через корешок проходит. Прежний foldAllowed (forms.SpineOK) «в край» не проверял.
func TestSetLayoutRejectsFoldKey(t *testing.T) {
	const key = "B[full:0]"
	e := edtest.Open(t, model.New(), "")
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 200, 300
	e.SetSettings(st)
	edtest.NewSpread(e, -1, whPhotos("q", [2]int{300, 300}))
	ids := edtest.SpreadIDs(edtest.NewSpread(e, -1, whPhotos("w", [2]int{450, 300})))
	p := edtest.P(e)
	for i := range ids {
		if _, ok := layout.Build(key, p.Aspects(p.Spreads[i].Photos), p.Params()); !ok {
			t.Fatalf("разворот %d: %s не строится — тест не проверяет", i, key)
		}
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetLayout(ids[0], key)
	edtest.Same(t, e, rev, undo, "квадрат в край через сгиб")
	e.SetLayout(ids[1], key)
	if r, _, _ := edtest.P(e).Layout(edtest.P(e).Spreads[1]); r.Key != key || edtest.UndoLen(e) > undo+1 {
		t.Fatalf("широкий кадр в край не встал: действует %q", r.Key)
	}

	setFold(e, true, true)
	undo = edtest.UndoLen(e)
	e.SetLayout(ids[0], key)
	if got := edtest.P(e).Spreads[0].Layout; got != key || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Любые»: квадрат в край не встал: %q, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	if got := layoutsOf(edtest.SaveLoad(t, e)); got[0] != key {
		t.Fatalf("Save → Load: %q", got)
	}
}

// TestAutoBuildFoldWide — сквозной критерий 064 (спека, «владелец 2026-10-17»): автосборка 36
// вертикальных 2:3 в 12 разворотов при «Только широкие» на каждом уровне разнообразия — у
// каждого разворота View.Rects проходят layout.FoldOK; Save → Load раскладок не меняет. При
// «Любые» на том же наборе есть разворот вне правила — иначе тест ничего не проверяет.
func TestAutoBuildFoldWide(t *testing.T) {
	wh := make([][2]int, 36)
	for i := range wh {
		wh[i] = [2]int{200, 300}
	}
	ph := whPhotos("f", wh...)
	for _, lv := range levels {
		for _, foldAny := range []bool{false, true} {
			e := edtest.Open(t, model.New(), "")
			setFold(e, true, foldAny)
			v := build(t, e, ph, -1, model.AutoOpts{Spreads: 12, MaxPer: 12, Variety: lv})
			p := edtest.P(e)
			wide := p.Params()
			wide.FoldAny = false
			if len(v.Spreads) != 12 || len(p.Spreads) != 12 || p.Settings.FoldAny != foldAny {
				t.Fatalf("уровень %v, «Любые» %v: разворотов %d/%d, foldAny %v", lv, foldAny, len(v.Spreads), len(p.Spreads), p.Settings.FoldAny)
			}
			out := 0
			for i, s := range v.Spreads {
				if !layout.FoldOK(s.Rects, p.Aspects(p.Spreads[i].Photos), wide) {
					out++
					if !foldAny {
						t.Errorf("уровень %v, разворот %d (%s): узкое фото через корешок", lv, i, p.Spreads[i].Layout)
					}
				}
			}
			if foldAny && out == 0 {
				t.Errorf("уровень %v, «Любые»: все развороты в правиле — тест не проверяет", lv)
			}
			if q := edtest.SaveLoad(t, e); !slices.Equal(layoutsOf(q), layoutsOf(p)) {
				t.Errorf("уровень %v, «Любые» %v: Save → Load сменил раскладки %q → %q", lv, foldAny, layoutsOf(p), layoutsOf(q))
			}
		}
	}
}
