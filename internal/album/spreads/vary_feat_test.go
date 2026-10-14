package spreads_test

// Спека 041, часть 2 (решения 0.11 №30–35): раскладки с главным фото в сборке — круг Save → Load,
// смена формата альбома после сборки, соседи с главным фото у «Разнообразить».

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// isFeatured — ключ из «Других вариантов» с главным фото по геометрии и без модификатора страницы
// (не «воздух»): раскладка генератора части 2.
func isFeatured(key string, asp []float64, prm layout.Params) bool {
	if key == "" || model.IndexOf(model.ExtraRow(asp, prm), key) < 0 || strings.ContainsAny(key, "SOI") {
		return false
	}
	r, ok := layout.Build(key, asp, prm)
	return ok && variety.Describe(r, prm).Heroes > 0
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
