package variety

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// tileCounts — плитки подборки (ряд «Авто» + «Другие варианты») по TileFamily и по FamilyOf.
func tileCounts(as []float64, p layout.Params) (tile, form map[Family]int, pairs int) {
	tile, form = map[Family]int{}, map[Family]int{}
	for _, rs := range [][]layout.Result{RowResults(as, p, 24), Extra(as, p, 24)} {
		for _, r := range rs {
			f := forms.Of(r.Key, r.Rects, p)
			tile[TileFamily(f, r.Rects, p)]++
			form[FamilyOf(f)]++
			if f == forms.Pair {
				pairs++
			}
		}
	}
	return tile, form, pairs
}

// TestTileFamilyDensePair (0.23-q7): «Две крупных», закрывающие ≥ 0,8 области в полях, — «Плотно»
// в подборке. Владелец: 6 вертикальных 2:3 на 600×305 — при 12/3 и 3/3 «Плотно» не пусто, а
// FamilyOf (ряд, Pick, ритм) тот же, что был.
func TestTileFamilyDensePair(t *testing.T) {
	as := sameAspects(6, 2.0/3)
	for _, mg := range [][2]float64{{12, 3}, {3, 3}, {0, 0}, {12, 0}} {
		p := layout.Params{PageW: 300, PageH: 305, Margin: mg[0], Gap: mg[1], Layflat: true, FoldAny: true}
		tile, form, pairs := tileCounts(as, p)
		t.Logf("поля %g, промежуток %g: Плотно %d (по форме %d), пар %d", mg[0], mg[1], tile[Dense], form[Dense], pairs)
		if tile[Dense] < form[Dense] || tile[Dense] == 0 {
			t.Errorf("поля %g/%g: Плотно %d при %d по форме — ждали не пусто и не меньше", mg[0], mg[1], tile[Dense], form[Dense])
		}
		if tile[Dense]-form[Dense] > pairs {
			t.Errorf("поля %g/%g: прибавка %d больше числа пар %d", mg[0], mg[1], tile[Dense]-form[Dense], pairs)
		}
		// Семья плитки лишь перекладывает «Две крупных» из «С главным»: остальные семьи не растут.
		for _, f := range Families() {
			if f != Dense && tile[f] > form[f] {
				t.Errorf("поля %g/%g: %s выросло %d → %d", mg[0], mg[1], f, form[f], tile[f])
			}
		}
	}
}

// TestTileFamilyRule: правило на границе — порог 0,8 включительно, только «Две крупных»; пустая
// область, ноль фото и другие формы — как FamilyOf.
func TestTileFamilyRule(t *testing.T) {
	p := layout.Params{PageW: 100, PageH: 100}
	one := func(share float64) []layout.Rect { // одно фото площадью share от разворота 200×100
		return []layout.Rect{{X: 0, Y: 0, W: 200 * share, H: 100}}
	}
	cases := []struct {
		name string
		f    forms.Form
		rs   []layout.Rect
		p    layout.Params
		want Family
	}{
		{"пара 0,8", forms.Pair, one(0.8), p, Dense},
		{"пара 0,79", forms.Pair, one(0.79), p, Hero},
		{"пара 1", forms.Pair, one(1), p, Dense},
		{"пара без фото", forms.Pair, nil, p, Hero},
		{"пара, область 0", forms.Pair, one(1), layout.Params{PageW: 10, PageH: 10, Margin: 5}, Hero},
		{"главное 0,95", forms.Hero, one(0.95), p, Hero},
		{"мозаика 0,3", forms.Mosaic, one(0.3), p, Dense},
		{"лента 0,95", forms.Strip, one(0.95), p, Strip},
		{"воздух 0,95", forms.Air, one(0.95), p, Air},
		{"центр 0,95", forms.Contrast, one(0.95), p, Figure},
		{"в край 0,95", forms.Bleed, one(0.95), p, Bleed},
	}
	for _, c := range cases {
		if got := TileFamily(c.f, c.rs, c.p); got != c.want {
			t.Errorf("%s: %s, ждали %s", c.name, got, c.want)
		}
	}
	// Поля сужают область: фото на 0,8 области (180×80) при полях 10 — «Плотно».
	pm := layout.Params{PageW: 100, PageH: 100, Margin: 10}
	rs := []layout.Rect{{X: 10, Y: 10, W: 180 * 0.8, H: 80}}
	if got := TileFamily(forms.Pair, rs, pm); got != Dense {
		t.Errorf("поля 10, пара 0,8 области: %s, ждали Dense", got)
	}
}

// TestTileFamilyKinds: горизонтальные и смешанные наборы — «Плотно» не убывает, а «С главным»
// теряет ровно то, что стало «Плотно».
func TestTileFamilyKinds(t *testing.T) {
	mix := []float64{1.5, 2.0 / 3, 1.5, 1.5, 2.0 / 3, 1.5, 2.0 / 3, 1.5}
	sets := map[string][]float64{
		"горизонтальные 6": sameAspects(6, 1.5), "горизонтальные 4": sameAspects(4, 1.5),
		"вертикальные 8": sameAspects(8, 2.0/3), "смешанные 8": mix, "смешанные 5": mix[:5],
	}
	for name, as := range sets {
		for _, mg := range [][2]float64{{12, 3}, {3, 3}} {
			p := layout.Params{PageW: 300, PageH: 305, Margin: mg[0], Gap: mg[1], Layflat: true, FoldAny: true}
			tile, form, _ := tileCounts(as, p)
			if tile[Dense] < form[Dense] {
				t.Errorf("%s %g/%g: Плотно %d → %d убыло", name, mg[0], mg[1], form[Dense], tile[Dense])
			}
			if d := tile[Dense] - form[Dense]; tile[Hero] != form[Hero]-d {
				t.Errorf("%s %g/%g: «С главным» %d → %d при прибавке Плотно %d", name, mg[0], mg[1], form[Hero], tile[Hero], d)
			}
			t.Logf("%s %g/%g: Плотно %d → %d", name, mg[0], mg[1], form[Dense], tile[Dense])
		}
	}
}
