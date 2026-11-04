package spreads_test

// Спека 078 (сц. 6–7): ч/б развороты через Editor — доля, тон фото, любимые, план = сборка.

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// grayOf — какие развороты альбома целиком ч/б (у всех фото Adj.Gray).
func grayOf(p *model.Project) []bool {
	out := make([]bool, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = len(s.Photos) > 0
		for _, id := range s.Photos {
			if r := p.Photos[id].Adj; r == nil || !r.Gray {
				out[i] = false
			}
		}
	}
	return out
}

// countTrue — сколько истинных.
func countTrue(b []bool) int {
	k := 0
	for _, v := range b {
		if v {
			k++
		}
	}
	return k
}

// TestAutoGrayShare: доля ч/б разворотов по уровню (10/20/30 % с округлением), первый и последний
// цветные, не подряд; уровень 0 — ни одного фото с тоном; меньше трёх разворотов — ч/б нет.
func TestAutoGrayShare(t *testing.T) {
	ph, _ := scenes("g", 100)
	for _, c := range []struct{ spreads, gray int }{{12, 1}, {12, 2}, {12, 3}, {20, 2}, {12, 0}, {2, 3}} {
		e := edtest.Open(t, model.New(), "")
		o := model.AutoOpts{Spreads: c.spreads, Auto: true, Gray: c.gray, Variety: model.VarietyNormal}
		build(t, e, ph, -1, o)
		p := edtest.P(e)
		n, gray := len(p.Spreads), grayOf(p)
		want := 0
		if c.gray > 0 && n >= 3 {
			want = max(1, int(math.Round(float64(n)*[]float64{0, .1, .2, .3}[c.gray])))
		}
		if got := countTrue(gray); got != want {
			t.Fatalf("%+v: ч/б %d из %d, ждали %d", c, got, n, want)
		}
		if want > 0 && (gray[0] || gray[n-1]) {
			t.Fatalf("%+v: край ч/б %v", c, gray)
		}
		for i := 1; i < n; i++ {
			if gray[i] && gray[i-1] {
				t.Fatalf("%+v: ч/б подряд %v", c, gray)
			}
		}
		if c.gray == 0 {
			for id, r := range p.Photos {
				if r.Adj != nil {
					t.Fatalf("без ч/б у %s тон %+v", id, r.Adj)
				}
			}
		}
	}
}

// TestAutoGrayKeepsTone: сборка ставит «Ч/б», а яркость и контраст фото, заданные до сборки,
// сохраняются; уже ч/б фото не меняется; фото цветных разворотов тон не получают.
func TestAutoGrayKeepsTone(t *testing.T) {
	ph, _ := scenes("k", 40)
	for i := range ph {
		ph[i].Sig.Color = 60 // всё цветное, кроме двух фото посередине
	}
	ph[20].Sig.Color, ph[21].Sig.Color = 1, 1
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[20:22])
	e.SetTone(ph[20].ID, 20, -10)
	e.SetGray([]string{ph[21].ID})
	e.SetTone(ph[21].ID, 0, 30)
	before := edtest.P(e).Photos[ph[21].ID].Adj
	build(t, e, ph, -1, model.AutoOpts{Spreads: 10, Auto: true, Gray: 1})
	p := edtest.P(e)
	if a := p.Photos[ph[20].ID].Adj; a == nil || *a != (model.Tone{Gray: true, Bright: 20, Contrast: -10}) {
		t.Fatalf("тон фото до сборки: %+v", a)
	}
	if a := p.Photos[ph[21].ID].Adj; a == nil || *a != *before {
		t.Fatalf("уже ч/б фото изменилось: %+v, было %+v", a, before)
	}
	gray := grayOf(p)
	if countTrue(gray) != 1 {
		t.Fatalf("ч/б разворотов %v", gray)
	}
	for i, s := range p.Spreads {
		for _, id := range s.Photos {
			if a := p.Photos[id].Adj; !gray[i] && a != nil {
				t.Fatalf("фото %s цветного разворота с тоном %+v", id, a)
			}
		}
	}
}

// TestAutoGrayFavoritesColor: наименее цветный разворот с фото 5★ остаётся цветным, ч/б — следующий;
// без звезды ч/б — он сам.
func TestAutoGrayFavoritesColor(t *testing.T) {
	ph := mixedPhotos("f", 30)
	o := model.AutoOpts{Spreads: 10, MaxPer: 3, Gray: 1} // ровно: группы не зависят от цветности
	probe := edtest.Open(t, model.New(), "")
	build(t, probe, ph, -1, model.AutoOpts{Spreads: 10, MaxPer: 3})
	groups := edtest.P(probe).Spreads
	color := map[string]float32{}
	for i, s := range groups {
		for _, id := range s.Photos {
			color[id] = 50
			switch i {
			case 4:
				color[id] = 5
			case 6:
				color[id] = 10
			}
		}
	}
	for i := range ph {
		ph[i].Sig.Ok, ph[i].Sig.Color = true, color[ph[i].ID]
	}
	for _, star := range []bool{false, true} {
		in := slices.Clone(ph)
		if star {
			for i := range in {
				if in[i].ID == groups[4].Photos[0] {
					in[i].Xmp.Rating = 5
				}
			}
		}
		e := edtest.Open(t, model.New(), "")
		build(t, e, in, -1, o)
		gray, want := grayOf(edtest.P(e)), 4
		if star {
			want = 6
		}
		if countTrue(gray) != 1 || !gray[want] {
			t.Fatalf("звезда %v: ч/б %v, ждали разворот %d", star, gray, want)
		}
	}
}

// TestAutoGrayPlanMatchesBuild: AutoPlan.Gray, пометки «Что соберётся» и собранные ч/б развороты
// совпадают на уровнях, вариантах и путях плана.
func TestAutoGrayPlanMatchesBuild(t *testing.T) {
	ph, _ := scenes("p", 90)
	for _, base := range []model.AutoOpts{{Spreads: 12, Auto: true}, {Spreads: 10, MinPer: 3, MaxPer: 12}, {Spreads: 9, MaxPer: 20}} {
		for gray := 1; gray <= model.MaxAutoLevel; gray++ {
			for _, variant := range []int{1, 2, 5} {
				o := base
				o.Gray, o.Variant, o.Variety = gray, variant, model.VarietyMore
				name := fmt.Sprintf("%+v", o)
				e := edtest.Open(t, model.New(), "")
				ap, err := e.AutoPlan(ph, o)
				if err != nil {
					t.Fatal(err)
				}
				pv, err := e.AutoPreview(ph, -1, o)
				if err != nil {
					t.Fatal(err)
				}
				build(t, e, ph, -1, o)
				got := grayOf(edtest.P(e))
				shown := make([]bool, len(pv.Spreads))
				for i, s := range pv.Spreads {
					shown[i] = s.Gray
				}
				if ap.Gray != countTrue(got) || !slices.Equal(shown, got) || ap.Gray == 0 {
					t.Fatalf("%s: план %d, предпросмотр %v, собрано %v", name, ap.Gray, shown, got)
				}
			}
		}
	}
}
