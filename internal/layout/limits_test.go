package layout

import (
	"strconv"
	"strings"
	"testing"
)

// Выше MaxPhotos всё отказывает сразу (без долгого счёта), на пределе — работает.
func TestMaxPhotos(t *testing.T) {
	p := Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}
	mk := func(n int) ([]float64, string) {
		asp := make([]float64, n)
		ids := make([]string, n)
		for i := range asp {
			asp[i] = 1.5
			ids[i] = strconv.Itoa(i)
		}
		return asp, "P[H(" + strings.Join(ids, ",") + ")|-]"
	}
	asp, key := mk(MaxPhotos)
	if len(Layouts(asp, p, 5)) == 0 {
		t.Fatal("на пределе Layouts должен работать")
	}
	if _, ok := Build(key, asp, p); !ok {
		t.Fatal("на пределе Build должен работать")
	}
	asp, key = mk(MaxPhotos + 1)
	if r := Layouts(asp, p, 5); len(r) != 0 {
		t.Fatal("Layouts выше предела должен отказать")
	}
	if r, _ := Rearranged(asp, p, 5); len(r) != 0 {
		t.Fatal("Rearranged выше предела должен отказать")
	}
	if _, ok := Build(key, asp, p); ok {
		t.Fatal("Build выше предела должен отказать")
	}
	if _, _, _, ok := PageChoices(key, asp, p, 0, 5); ok {
		t.Fatal("PageChoices выше предела должен отказать")
	}
}
