package layout

import (
	"strconv"
	"strings"
	"testing"
)

// Выше MaxPhotos всё отказывает сразу (без долгого счёта), на пределе — работает.
func TestMaxPhotos(t *testing.T) {
	p := Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}
	// Ряды по 8 фото на левой странице: один ряд из 64 фото уже меньше миллиметра.
	mk := func(n int) ([]float64, string) {
		asp := make([]float64, n)
		var rows []string
		for i := 0; i < n; i += 8 {
			ids := []string{}
			for j := i; j < min(i+8, n); j++ {
				asp[j] = 1.5
				ids = append(ids, strconv.Itoa(j))
			}
			if len(ids) == 1 {
				rows = append(rows, ids[0])
			} else {
				rows = append(rows, "H("+strings.Join(ids, ",")+")")
			}
		}
		return asp, "P[V(" + strings.Join(rows, ",") + ")|-]"
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
