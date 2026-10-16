package spreads_test

// Спека 041, часть 2 (решения 0.11 №30–35): раскладки с главным фото в сборке — круг Save → Load,
// смена формата альбома после сборки, соседи с главным фото у «Разнообразить».

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// isFeatured — ключ с главным фото по геометрии без модификатора страницы (не «воздух»): раскладка
// генератора части 2 из «Других вариантов» или форма Hero/Spine/Pair ряда «Авто» (051).
func isFeatured(key string, asp []float64, prm layout.Params) bool {
	if key == "" || strings.ContainsAny(key, "SOI") {
		return false
	}
	r, ok := layout.Build(key, asp, prm)
	if !ok || variety.Describe(r, prm).Heroes == 0 {
		return false
	}
	if model.IndexOf(model.ExtraRow(asp, prm), key) >= 0 {
		return true
	}
	_, i, _ := model.AutoRow(asp, prm, key)
	if i < 0 {
		return false
	}
	switch variety.Describe(r, prm).Form {
	case forms.Hero, forms.Spine, forms.Pair:
		return true
	}
	return false
}

// stockPhotos — 100 фото stock (testkit.StockAspects) с длинной стороной 2000 px.
func stockPhotos(t *testing.T) []model.PhotoInfo {
	as := testkit.StockAspects(t)
	out := make([]model.PhotoInfo, len(as))
	for i, a := range as {
		w, h := 2000, int(math.Round(2000/a))
		if a < 1 {
			w, h = int(math.Round(2000*a)), 2000
		}
		id := fmt.Sprint("s", i)
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: w, H: h}
	}
	return out
}

// featuredCount — разворотов с раскладкой генератора.
func featuredCount(p *model.Project) int {
	n := 0
	for _, s := range p.Spreads {
		if isFeatured(s.Layout, p.Aspects(s.Photos), p.Params()) {
			n++
		}
	}
	return n
}

// featuredAlbum — 120 одинаковых горизонтальных на 10 разворотов по 12, «Обычное»: по ритму
// главного часть разворотов получает раскладку генератора.
func featuredAlbum(t *testing.T) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	build(t, e, horizontals("f", 120), -1, model.AutoOpts{Spreads: 10, MaxPer: 16, Variety: model.VarietyNormal})
	if featuredCount(edtest.P(e)) == 0 {
		t.Fatal("сборка не поставила ни одной раскладки с главным фото: тест её не проверяет")
	}
	return e
}

// TestAutoBuildFeatured: на одинаковых 12 фото «Обычное» ставит раскладки с главным фото не реже
// раза на 4 разворота и никогда два подряд; «Меньше» — ни одной (как 0.10).
func TestAutoBuildFeatured(t *testing.T) {
	e := featuredAlbum(t)
	p := edtest.P(e)
	hero := make([]bool, len(p.Spreads))
	for i, s := range p.Spreads {
		r, _, _ := p.Layout(s)
		hero[i] = variety.Describe(r, p.Params()).Heroes > 0
	}
	n := 0
	for i, h := range hero {
		if h {
			n++
		}
		if h && i > 0 && hero[i-1] && isFeatured(p.Spreads[i].Layout, p.Aspects(p.Spreads[i].Photos), p.Params()) {
			t.Fatalf("разворот %d: раскладка с главным фото сразу после другой с главным", i)
		}
	}
	if n < len(hero)/4 {
		t.Fatalf("с главным фото %d из %d разворотов", n, len(hero))
	}
	less := edtest.Open(t, model.New(), "")
	build(t, less, horizontals("f", 120), -1, model.AutoOpts{Spreads: 10, MaxPer: 16, Variety: model.VarietyLess})
	if featuredCount(edtest.P(less)) != 0 {
		t.Fatal("«Меньше» поставило раскладку с главным фото")
	}
}

// TestAutoBuildHeroForms (051): правило «два с главным подряд — нет» держится и для форм ряда «Авто»
// (Hero/Spine/Pair), а не только для ключей «Других вариантов»: смешанные фото и stock, «Обычное» и
// «Больше». Форма «С главным» ряда после главного — только если другого допустимого нет; на этих
// наборах такого нет. Лучшая раскладка (Layout "") под правило не подпадает: как до 051 (№40).
func TestAutoBuildHeroForms(t *testing.T) {
	rowForms := 0
	sets := map[string][]model.PhotoInfo{"смесь 60": mixedPhotos("m", 60), "смесь 100": mixedPhotos("m", 100), "stock": stockPhotos(t)}
	for n, ph := range sets {
		for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
			e := edtest.Open(t, model.New(), "")
			build(t, e, ph, -1, model.AutoOpts{Spreads: 11, Auto: true, Variety: v})
			p, prev := edtest.P(e), false
			for i, s := range p.Spreads {
				asp, prm := p.Aspects(s.Photos), p.Params()
				r, _, _ := p.Layout(s)
				in := variety.Describe(r, prm)
				hero := in.Heroes > 0
				if variety.FamilyOf(in.Form) == variety.Hero && prev && s.Layout != "" {
					t.Errorf("%s, уровень %d, разворот %d: форма %s (%q) сразу после разворота с главным", n, v, i, in.Form, s.Layout)
				}
				if f := isFeatured(s.Layout, asp, prm); f {
					if model.IndexOf(model.ExtraRow(asp, prm), s.Layout) < 0 {
						rowForms++
					}
					if prev {
						t.Fatalf("%s, уровень %d, разворот %d: главное фото (%q) сразу после другого с главным", n, v, i, s.Layout)
					}
				}
				prev = hero
			}
		}
	}
	if rowForms == 0 {
		t.Fatal("ни одной формы Hero/Spine/Pair ряда «Авто»: тест правила для них ничего не проверяет")
	}
}

// TestFeaturedSaveLoad: ключи генератора после Save → Load — те же раскладки, счётчики и
// прямоугольники (формат не меняется: ключ нынешней грамматики в Spread.Layout).
func TestFeaturedSaveLoad(t *testing.T) {
	e := featuredAlbum(t)
	view := e.View()
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	got := edtest.Open(t, q, path).View()
	if len(got.Spreads) != len(view.Spreads) {
		t.Fatalf("разворотов %d → %d", len(view.Spreads), len(got.Spreads))
	}
	for i, w := range view.Spreads {
		g := got.Spreads[i]
		if g.Layout != w.Layout || g.Variant != w.Variant || g.Variants != w.Variants || !slices.Equal(g.Rects, w.Rects) {
			t.Fatalf("разворот %d: после Load %q %d/%d, было %q %d/%d", i, g.Layout, g.Variant, g.Variants, w.Layout, w.Variant, w.Variants)
		}
		if w.Layout != "" && w.Variant < 0 {
			t.Fatalf("разворот %d: ключ %q вне ряда «i / N»", i, w.Layout)
		}
	}
}

// TestKeepLayoutsSettings (решение 0.11 №35): ключи, поставленные сборкой (в том числе с главным
// фото), при смене формата альбома держатся, пока строятся для состава, как выбранные вручную
// (признака «выбрано сборкой» нет, решение №18); не строятся — лучшая (""). Отмена — одним шагом;
// та же смена настроек — холостая.
func TestKeepLayoutsSettings(t *testing.T) {
	for _, c := range []struct {
		name     string
		edit     func(*model.Settings)
		keepFeat bool // ключ с главным фото обязан удержаться
	}{
		{"30×20", func(s *model.Settings) { s.PageH = 200 }, true},
		{"без layflat", func(s *model.Settings) { s.Layflat = false }, false},
	} {
		e := featuredAlbum(t)
		before := string(model.Marshal(edtest.P(e)))
		p := edtest.P(e)
		was, wasFeat := make([]string, len(p.Spreads)), make([]bool, len(p.Spreads))
		for i, s := range p.Spreads {
			was[i], wasFeat[i] = s.Layout, isFeatured(s.Layout, p.Aspects(s.Photos), p.Params())
		}
		v0 := e.View()
		st := v0.Settings
		if v := e.SetSettings(st); string(model.Marshal(edtest.P(e))) != before || v.Rev != v0.Rev {
			t.Fatalf("%s: та же смена настроек — не холостая (rev %d → %d)", c.name, v0.Rev, v.Rev)
		}
		c.edit(&st)
		e.SetSettings(st)
		q := edtest.P(e)
		kept, dropped, feat := 0, 0, 0
		for i, s := range q.Spreads {
			if was[i] == "" {
				continue
			}
			_, ok := layout.Build(was[i], q.Aspects(s.Photos), q.Params())
			switch {
			case ok && s.Layout != was[i] && s.Layout != "":
				t.Fatalf("%s, разворот %d: ключ %q строится, а стал %q", c.name, i, was[i], s.Layout)
			case ok:
				kept++
				if s.Layout != "" && wasFeat[i] {
					feat++
				}
			case s.Layout != "":
				t.Fatalf("%s, разворот %d: ключ %q не строится, а остался %q", c.name, i, was[i], s.Layout)
			default:
				dropped++
			}
		}
		if kept == 0 || c.keepFeat && feat == 0 {
			t.Fatalf("%s: удержалось %d (с главным фото %d), ушло %d", c.name, kept, feat, dropped)
		}
		t.Logf("%s: удержалось %d (с главным фото %d), ушло %d", c.name, kept, feat, dropped)
		if e.Undo(); string(model.Marshal(edtest.P(e))) != before {
			t.Fatalf("%s: отмена не вернула проект", c.name)
		}
	}
}

// TestVaryLayoutsHeroRhythm: «Разнообразить» берёт ритм главного из соседей альбома (ctxOf, решение
// №34): у первого разворота пора не пришла — без главного; после пяти без главного — с главным;
// сразу за ним — снова без (не два главных подряд).
func TestVaryLayoutsHeroRhythm(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ids := edtest.SpreadIDs(build(t, e, horizontals("r", 120), -1, model.AutoOpts{Spreads: 10, MaxPer: 16}))
	feat := func(i int) bool {
		p := edtest.P(e)
		s := p.Spreads[i]
		return isFeatured(s.Layout, p.Aspects(s.Photos), p.Params())
	}
	if featuredCount(edtest.P(e)) != 0 {
		t.Fatal("заготовка «Меньше» уже с главным фото")
	}
	for _, c := range []struct {
		at   int
		want bool
	}{{0, false}, {5, true}, {6, false}} {
		e.VaryLayouts([]string{ids[c.at]}, model.VarietyNormal, nil)
		if got := feat(c.at); got != c.want {
			t.Fatalf("разворот %d: с главным %v, ждали %v (%q)", c.at, got, c.want, edtest.P(e).Spreads[c.at].Layout)
		}
	}
}

// counter — счётчик «i / N» разворота i в виде валиден: ключ в ряду «Авто» + «Другие варианты»,
// позиция в пределах ряда. out — ключ вправе быть вне ряда («— / N», variant −1), но только если
// его там правда нет: после смены формата KeepLayouts держит ключ, пока он строится (решение 0.11 №71).
func counter(t *testing.T, e *album.Editor, i int, what string, out bool) {
	t.Helper()
	sv := e.View().Spreads[i]
	if out && sv.Variant == -1 && sv.Variants > 0 {
		p := edtest.P(e)
		asp := p.Aspects(p.Spreads[i].Photos)
		row, j, _ := model.AutoRow(asp, p.Params(), sv.Layout)
		if j >= 0 || model.IndexOf(model.ExtraRow(asp, p.Params()), sv.Layout) >= 0 || len(row) == 0 {
			t.Fatalf("%s: ключ %q в ряду, а счётчик «—»", what, sv.Layout)
		}
		t.Logf("%s: ключ %q держится вне ряда («— / %d»)", what, sv.Layout, sv.Variants)
		return
	}
	if sv.Variants <= 0 || sv.Variant < 0 || sv.Variant >= sv.Variants {
		t.Fatalf("%s: счётчик %d / %d, ключ %q", what, sv.Variant, sv.Variants, sv.Layout)
	}
}

// TestVaryThenDropAndSettings — стык 041 и 042: «Разнообразить» поставило ключ из «Других
// вариантов» (с главным фото), затем бросок на этот «Авто»-разворот и смена формата. Ключ живёт
// по KeepLayouts: другой состав — лучшая (""), тот же состав в другом порядке и формат, где ключ
// строится, — ключ держится; счётчик валиден (после смены формата ключ может выпасть из ряда —
// «—», №71), отмена возвращает ключ байт в байт.
func TestVaryThenDropAndSettings(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	ids := edtest.SpreadIDs(build(t, e, horizontals("r", 120), -1, model.AutoOpts{Spreads: 10, MaxPer: 16}))
	const at = 5
	e.VaryLayouts([]string{ids[at]}, model.VarietyNormal, nil)
	p := edtest.P(e)
	s := p.Spreads[at]
	key := s.Layout
	if !isFeatured(key, p.Aspects(s.Photos), p.Params()) {
		t.Fatalf("«Разнообразить» не поставило ключ генератора: %q", key)
	}
	counter(t, e, at, "после «Разнообразить»", false)
	layoutAt := func() string { return edtest.P(e).Spreads[at].Layout }

	// Фото с соседнего разворота: состав другой — ключ сброшен; drop сам сверяет Undo/Redo байт в байт.
	other := edtest.P(e).Spreads[at-1].Photos[0]
	before := model.Marshal(edtest.P(e))
	if _, out := drop(t, e, ids[at], "", edtest.Photos(other), 0.5, ""); out.Fit != 1 || out.Ask {
		t.Fatalf("бросок на «Авто»: %+v", out)
	}
	if layoutAt() != "" {
		t.Fatalf("состав изменился, а ключ %q остался", layoutAt())
	}
	counter(t, e, at, "после броска чужого фото", false)
	if e.Undo(); layoutAt() != key || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("отмена броска: ключ %q, ждали %q", layoutAt(), key)
	}

	// Своё же фото: тот же состав в другом порядке, пропорции одинаковые — ключ строится и держится.
	own := edtest.P(e).Spreads[at].Photos[0]
	drop(t, e, ids[at], "", edtest.Photos(own), 0.5, "")
	if ph := edtest.P(e).Spreads[at].Photos; ph[len(ph)-1] != own || layoutAt() != key {
		t.Fatalf("бросок своего фото: последнее %q, ключ %q, ждали %q", ph[len(ph)-1], layoutAt(), key)
	}
	counter(t, e, at, "после броска своего фото", false)
	e.Undo()

	// Смена формата после «Разнообразить»: ключ держится, если строится, иначе "" (как у сборки, №35).
	for _, c := range []struct {
		name string
		edit func(*model.Settings)
	}{
		{"30×20", func(s *model.Settings) { s.PageH = 200 }},
		{"без layflat", func(s *model.Settings) { s.Layflat = false }},
	} {
		before := model.Marshal(edtest.P(e))
		st := e.View().Settings
		c.edit(&st)
		e.SetSettings(st)
		q := edtest.P(e)
		_, builds := layout.Build(key, q.Aspects(q.Spreads[at].Photos), q.Params())
		if want := map[bool]string{true: key, false: ""}[builds]; layoutAt() != want {
			t.Fatalf("%s: ключ строится %v, на развороте %q, ждали %q", c.name, builds, layoutAt(), want)
		}
		counter(t, e, at, c.name, true)
		if e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) || layoutAt() != key {
			t.Fatalf("%s: отмена не вернула ключ: %q", c.name, layoutAt())
		}
	}
}

// TestSetLayoutFormKey (051 G6): ключ формы (Q, G, L) ставится одним шагом отмены, повтор — холостой,
// отмена возвращает прежнюю, круг Save → Load сохраняет ключ; на листе (Sheet) ключ Q не строится —
// отказ холостой (rev и шаги отмены стоят), а G и L на весь лист строятся.
func TestSetLayoutFormKey(t *testing.T) {
	for _, sheet := range []bool{false, true} {
		p := model.New()
		p.Settings.Sheet = sheet
		e := edtest.Open(t, p, "")
		id := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d")).Spreads[0].ID
		for _, key := range []string{"Q[H(0,1)|H(2,3)]", "G(H(0,1),H(2,3))", "L(0,1,2,3)"} {
			before, rev, undo := model.Marshal(edtest.P(e)), e.View().Rev, edtest.UndoLen(e)
			v := e.SetLayout(id, key)
			if sheet && strings.HasPrefix(key, "Q") {
				if v.Rev != rev || edtest.UndoLen(e) != undo || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
					t.Fatalf("лист: %q не отказ или не холостой", key)
				}
				continue
			}
			if v.Spreads[0].Layout != key || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
				t.Fatalf("лист %t: %q — стоит %q, rev %d→%d, шагов %d→%d", sheet, key, v.Spreads[0].Layout, rev, v.Rev, undo, edtest.UndoLen(e))
			}
			if e.SetLayout(id, key).Rev != v.Rev || edtest.UndoLen(e) != undo+1 {
				t.Fatalf("лист %t: повтор %q не холостой", sheet, key)
			}
			if q := edtest.SaveLoad(t, e); q.Spreads[0].Layout != key {
				t.Fatalf("лист %t: после Save → Load %q, ждали %q", sheet, q.Spreads[0].Layout, key)
			}
			if e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) {
				t.Fatalf("лист %t: отмена %q не вернула прежнее", sheet, key)
			}
		}
	}
}

// TestAutoBuildForms (051 G6): «Больше» на 100 смешанных фото в «Авто» ставит формы ряда «Авто»
// (ключи вне ряда layout и «Других вариантов») на нескольких разворотах, соседних одной формы нет;
// сборка — один шаг отмены; круг Save → Load сохраняет ключи и прямоугольники; «Меньше» — ни одной
// формы.
func TestAutoBuildForms(t *testing.T) {
	formed := func(e *album.Editor) (n, same int) {
		p, prev := edtest.P(e), forms.Form("")
		for _, s := range p.Spreads {
			asp, prm := p.Aspects(s.Photos), p.Params()
			r, _, _ := p.Layout(s)
			f := variety.Describe(r, prm).Form
			if s.Layout != "" && model.IndexOf(layout.Cached(asp, prm, model.RowLen(len(asp))), s.Layout) < 0 &&
				model.IndexOf(model.ExtraRow(asp, prm), s.Layout) < 0 {
				n++
			}
			if f == prev {
				same++
			}
			prev = f
		}
		return n, same
	}
	opts := model.AutoOpts{Spreads: 11, Auto: true, Variety: model.VarietyMore}
	e := edtest.Open(t, model.New(), "")
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	view := build(t, e, mixedPhotos("m", 100), -1, opts)
	if n, same := formed(e); n < 2 || same != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Больше»: форм ряда %d, соседей одной формы %d, шагов отмены %d", n, same, edtest.UndoLen(e)-undo)
	}
	got := edtest.Open(t, edtest.SaveLoad(t, e), "").View()
	for i, w := range view.Spreads {
		if g := got.Spreads[i]; g.Layout != w.Layout || !slices.Equal(g.Rects, w.Rects) || g.Variant != w.Variant {
			t.Fatalf("разворот %d: после Load %q %d, было %q %d", i, g.Layout, g.Variant, w.Layout, w.Variant)
		}
	}
	if e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена сборки не вернула альбом")
	}
	opts.Variety = model.VarietyLess
	less := edtest.Open(t, model.New(), "")
	build(t, less, mixedPhotos("m", 100), -1, opts)
	if n, _ := formed(less); n != 0 {
		t.Fatalf("«Меньше» поставило %d форм", n)
	}
}

// TestAutoBuildInRow (051, QA пачки 2): всё, что ставит автосборка на stock 100, видно в листании —
// ключ в ряду «Авто» подборки (model.AutoRow) или в «Других вариантах» за ним (решение 0.11 №38), и
// счётчик «i / N» показывает номер, а не «—»; обе геометрии, «Обычное» и «Больше», несколько N.
func TestAutoBuildInRow(t *testing.T) {
	ph := stockPhotos(t)
	beyond := 0 // ключей не на первом месте ряда: иначе тест ничего не проверяет
	for _, flat := range []bool{true, false} {
		for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
			for _, n := range []int{8, 11, 14} {
				p := model.New()
				p.Settings.Layflat = flat
				e := edtest.Open(t, p, "")
				view := build(t, e, ph, -1, model.AutoOpts{Spreads: n, Auto: true, Variety: v})
				name := fmt.Sprintf("layflat %t, уровень %d, N %d", flat, v, n)
				beyond += inRow(t, name, edtest.P(e), view)
			}
		}
	}
	if beyond == 0 {
		t.Fatal("сборка везде поставила лучшую: проверять нечего")
	}
}

// inRow — у каждого разворота ключ в ряду «Авто» или в «Других вариантах» и номер в счётчике;
// возвращает, сколько ключей не на первом месте.
func inRow(t *testing.T, name string, q *model.Project, view model.View) (beyond int) {
	t.Helper()
	for i, s := range q.Spreads {
		asp, prm := q.Aspects(s.Photos), q.Params()
		row, at, _ := model.AutoRow(asp, prm, s.Layout)
		if s.Layout == "" {
			at = 0
		}
		if at < 0 {
			at = model.IndexOf(model.ExtraRow(asp, prm), s.Layout)
		}
		if sv := view.Spreads[i]; at < 0 || sv.Variant < 0 {
			t.Fatalf("%s, разворот %d: %q вне ряда (%d из %d), счётчик %d", name, i, s.Layout, at, len(row), sv.Variant)
		}
		if at > 0 {
			beyond++
		}
	}
	return beyond
}
