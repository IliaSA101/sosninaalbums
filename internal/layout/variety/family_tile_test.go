package variety

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// tilePool — пул «Две крупных»: a закрывает 85 % области (плотная), b — 50 %, c — пустые Rects,
// d — не пара (мозаика).
func tilePool() ([]Cand, layout.Params) {
	p := layout.Params{PageW: 100, PageH: 100} // область в полях 200×100 = 20000 мм²
	mk := func(key string, f forms.Form, area float64) Cand {
		c := Cand{Result: layout.Result{Key: key}, Info: Info{Form: f}}
		if area > 0 {
			c.Rects = []layout.Rect{{W: area / 100, H: 100}}
		}
		return c
	}
	return []Cand{mk("a", forms.Pair, 17000), mk("b", forms.Pair, 10000), mk("c", forms.Pair, 0), mk("d", forms.Mosaic, 5000)}, p
}

// TestByFamilyTile (104): фильтр окна идёт по семье плитки (TileFamily): «Две крупных» с заполнением
// ≥ 0,8 проходят «Плотно», остальные пары — только «С главным»; границы: пустые Rects и нулевая
// область не плотные; смена Params меняет выборку; пул и порядок сохраняются.
func TestByFamilyTile(t *testing.T) {
	pool, p := tilePool()
	cases := []struct {
		allow map[Family]bool
		want  string
		fb    bool
	}{
		{map[Family]bool{Dense: true}, "ad", false}, // a — плотная пара, d — мозаика
		{map[Family]bool{Hero: true}, "bc", false},  // остальные пары — по форме
		{map[Family]bool{Dense: true, Hero: true}, "abcd", false},
		{map[Family]bool{Strip: true}, "a", true}, // разрешённых нет — лучшая
	}
	for _, c := range cases {
		if got, fb := ByFamily(pool, c.allow, p); famKeys(got) != c.want || fb != c.fb {
			t.Errorf("%v: %q fb=%v, ждали %q fb=%v", c.allow, famKeys(got), fb, c.want, c.fb)
		}
	}
	// Без области (нулевые Params) «Плотно» у пар нет: остаётся семья формы.
	if got, _ := ByFamily(pool, map[Family]bool{Hero: true}, layout.Params{}); famKeys(got) != "abc" {
		t.Errorf("нулевые Params: %q, ждали abc", famKeys(got))
	}
	// Фильтр и плитка — одна функция.
	for _, c := range pool {
		fam := TileFamily(c.Form, c.Rects, p)
		got, fb := ByFamily(pool, map[Family]bool{fam: true}, p)
		if fb || !famHas(got, c.Key) {
			t.Errorf("%s: семья плитки %s, а фильтр её не пропустил", c.Key, fam)
		}
	}
}

func famHas(cs []Cand, key string) bool {
	for _, c := range cs {
		if c.Key == key {
			return true
		}
	}
	return false
}
