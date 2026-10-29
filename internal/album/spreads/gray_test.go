package spreads

// Выбор ч/б разворотов (078 design §6): доля, края, соседи, любимые и исходно ч/б.

import (
	"math"
	"testing"
)

// grp — разворот с одной цветностью c у k фото (NaN — без подписи); fav — любимый.
func grp(c float32, k int, fav bool) grayGroup {
	g := grayGroup{fav: fav}
	for range k {
		g.colors = append(g.colors, c)
	}
	return g
}

// picked — номера выбранных разворотов.
func picked(gray []bool) []int {
	var out []int
	for i, g := range gray {
		if g {
			out = append(out, i)
		}
	}
	return out
}

// TestGrayPickShare: доля по уровню (округление, не меньше одного при N ≥ 3), первый и последний
// цветные, соседей нет, пока хватает несоседних; N < 3, уровень 0 и пустой план — ч/б нет.
func TestGrayPickShare(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 10, 15, 40} {
		groups := make([]grayGroup, n)
		for i := range groups {
			groups[i] = grp(float32(10+i%7), 3, false)
		}
		for share := range 5 { // 4 — за пределом уровней: как 3
			gray := grayPick(groups, share)
			lv := min(share, 3)
			want := 0
			if lv > 0 && n >= 3 {
				want = min(max(int(math.Round(float64(n)*grayShares[lv])), 1), n-2)
			}
			got := picked(gray)
			if len(got) != want {
				t.Fatalf("n=%d уровень %d: ч/б %v, ждали %d", n, share, got, want)
			}
			if want > 0 && (gray[0] || gray[n-1]) {
				t.Fatalf("n=%d уровень %d: край ч/б %v", n, share, got)
			}
			for k := 1; k < len(got); k++ {
				if got[k] == got[k-1]+1 && want <= (n-1)/2 {
					t.Fatalf("n=%d уровень %d: соседние ч/б %v", n, share, got)
				}
			}
		}
	}
}

// TestGrayPickOrder: меньше цветности — раньше; разворот с исходно ч/б фото — первым при любой
// средней; ничья — меньший номер; любимый пропускается, пока есть другие, но берётся вторым
// проходом; фото без подписи — медиана альбома.
func TestGrayPickOrder(t *testing.T) {
	g := []grayGroup{grp(1, 2, false), grp(30, 2, false), grp(9, 2, false), grp(30, 2, false), grp(20, 2, false), grp(5, 1, false)}
	if got := picked(grayPick(g, 1)); len(got) != 1 || got[0] != 2 { // 10 % от 6 = 1; края не в счёт
		t.Fatalf("наименее цветный: %v", got)
	}
	g[3] = grayGroup{colors: []float32{50, 3}} // среднее 26,5, но одно фото исходно ч/б
	if got := picked(grayPick(g, 1)); len(got) != 1 || got[0] != 3 {
		t.Fatalf("исходно ч/б — первым: %v", got)
	}
	g[3] = grp(30, 2, false)
	g[2].fav = true
	if got := picked(grayPick(g, 1)); len(got) != 1 || got[0] != 4 {
		t.Fatalf("любимый пропущен: %v", got)
	}
	// 30 % от 5 = 2: первым проходом — только 2 (1 и 3 любимые), вторым — 3 (меньше цветность),
	// уже без запрета соседей.
	f := []grayGroup{grp(1, 1, false), grp(30, 1, true), grp(20, 1, false), grp(10, 1, true), grp(1, 1, false)}
	if got := picked(grayPick(f, 3)); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("второй проход: %v", got)
	}
	nan := float32(math.NaN())
	h := []grayGroup{grp(10, 1, false), grp(nan, 2, false), grp(40, 1, false), grp(25, 1, false), grp(10, 1, false)}
	// медиана {10, 10, 25, 40} = 17,5: разворот без подписи впереди 25 и 40
	if got := picked(grayPick(h, 1)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("без подписи — медиана: %v", got)
	}
	none := []grayGroup{grp(nan, 1, false), grp(nan, 1, false), grp(nan, 2, false), grp(nan, 1, false)}
	if got := picked(grayPick(none, 1)); len(got) != 1 || got[0] != 1 { // подписей нет: все равны, меньший номер
		t.Fatalf("без подписей: %v", got)
	}
	tie := []grayGroup{grp(10, 1, false), grp(7, 1, false), grp(9, 1, false), grp(7, 1, false), grp(10, 1, false)}
	if got := picked(grayPick(tie, 1)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ничья — меньший номер: %v", got)
	}
}
