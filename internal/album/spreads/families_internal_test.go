package spreads

import (
	"maps"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// TestAllowOf (065): мусор и повторы отбрасываются; пусто, только мусор и все шесть (в любом
// порядке, с повторами) — nil («Все», как 0.14); порядок имён значения не имеет.
func TestAllowOf(t *testing.T) {
	all := []string{"figure", "strip", "bleed", "air", "hero", "dense", "dense"}
	for _, fs := range [][]string{nil, {}, {"x", "", "Dense"}, all} {
		if got := allowOf(fs); got != nil {
			t.Errorf("%q: %v, ждали nil", fs, got)
		}
	}
	cases := []struct {
		in   []string
		want map[variety.Family]bool
	}{
		{[]string{"dense"}, map[variety.Family]bool{variety.Dense: true}},
		{[]string{"hero", "мусор", "dense", "hero"}, map[variety.Family]bool{variety.Dense: true, variety.Hero: true}},
		{[]string{"dense", "hero"}, map[variety.Family]bool{variety.Hero: true, variety.Dense: true}},
		{all[:5], map[variety.Family]bool{variety.Figure: true, variety.Strip: true, variety.Bleed: true, variety.Air: true, variety.Hero: true}},
	}
	for _, c := range cases {
		if got := allowOf(c.in); !maps.Equal(got, c.want) {
			t.Errorf("%q: %v, ждали %v", c.in, got, c.want)
		}
	}
	// Уровень пула: с фильтром — не ниже «Обычного», без него — как есть.
	if poolLv(variety.Less, nil) != variety.Less || poolLv(variety.Less, map[variety.Family]bool{variety.Dense: true}) != variety.Normal ||
		poolLv(variety.More, map[variety.Family]bool{variety.Dense: true}) != variety.More {
		t.Error("poolLv")
	}
}
