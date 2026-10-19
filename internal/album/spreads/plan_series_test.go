package spreads

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
)

const t0 = 1_700_000_000 // произвольное «сегодня» в секундах Unix

// shot — фото в папке dir, снятое через at секунд после t0 (at < 0 — времени нет).
func shot(dir string, i int, at int64) model.PhotoInfo {
	ph := model.PhotoInfo{ID: fmt.Sprintf("%s%d", dir, i), Path: filepath.Join("/съёмка", dir, fmt.Sprintf("%d.jpg", i))}
	if at >= 0 {
		ph.Taken = t0 + at
	}
	return ph
}

// run — k фото одной папки с шагом step секунд, первое — в момент from.
func run(dir string, k int, from, step int64) []model.PhotoInfo {
	out := make([]model.PhotoInfo, k)
	for i := range out {
		out[i] = shot(dir, i, from+int64(i)*step)
	}
	return out
}

// folders — подряд папки по sizes фото, снятые с паузой в час между папками; каждое третье
// фото вертикальное (пропорции для плана).
func folders(sizes ...int) []model.PhotoInfo {
	var out []model.PhotoInfo
	for f, k := range sizes {
		for _, ph := range run(fmt.Sprint("п", f), k, int64(f)*3600, 20) {
			ph.W, ph.H = 300, 200
			if len(out)%3 == 0 {
				ph.W, ph.H = 200, 300
			}
			out = append(out, ph)
		}
	}
	return out
}

// sizesOf — фото в каждой группе плана.
func sizesOf(groups [][2]int) []int {
	out := make([]int, len(groups))
	for i, g := range groups {
		out[i] = g[1] - g[0]
	}
	return out
}

// TestAutoOptsZeroIsOld (051 §8): нули MinPer и Auto — PlanGroups 0.12 побитово при любых N и
// «не больше» (0, сверх 64, N сверх фото и меньше нуля); Series без диапазона план не трогает,
// только считает границы.
func TestAutoOptsZeroIsOld(t *testing.T) {
	for _, n := range []int{1, 7, 30, 68} {
		j := PlanOf(model.New(), folders(n/2, n-n/2), nil)
		for _, o := range []model.AutoOpts{{}, {Spreads: 1, MaxPer: 64}, {Spreads: 5, MaxPer: 8}, {Spreads: 999, MaxPer: 200}, {Spreads: 3}, {Spreads: -4, MaxPer: 3}} {
			m := max(1, min(o.MaxPer, model.MaxPhotos))
			want := plan.PlanGroups(j.asp, j.prm, o.Spreads, m)
			lo, hi := plan.SpreadRange(n, m)
			for _, series := range []bool{false, true} {
				o.Series = series
				ap, got, err := j.plan(o)
				if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("n=%d %+v: %v, ждали %v (%v)", n, o, got, want, err)
				}
				if ap.MinSpreads != lo || ap.MaxSpreads != hi || ap.From != 0 || ap.To != m || ap.Spreads != want.Spreads {
					t.Fatalf("n=%d %+v: подсказка %+v", n, o, ap)
				}
				wantCuts := 0
				if series && n > 1 {
					wantCuts = 1 // две папки
				}
				if ap.Cuts != wantCuts {
					t.Fatalf("n=%d %+v: границ серий %d, ждали %d", n, o, ap.Cuts, wantCuts)
				}
				// 078: нули новых полей и вариант 1 — тот же план, ни похожих, ни ч/б.
				o1 := o
				o1.Variant = 1
				ap1, got1, err := j.plan(o1)
				if err != nil || fmt.Sprint(got1) != fmt.Sprint(want) || ap1 != ap || ap.Similar != 0 || ap.Gray != 0 {
					t.Fatalf("n=%d %+v: вариант 1 %+v %v, без него %+v", n, o, ap1, got1, ap)
				}
			}
		}
	}
}

// TestPlanRangeSeriesPull: с сериями разрезы встают на смену папок чаще, чем без них, — и в
// «Авто», и в своём диапазоне; Cuts — число смен папок; каждый разворот в [From, To].
func TestPlanRangeSeriesPull(t *testing.T) {
	ph := folders(5, 9, 4, 11, 6, 8, 3, 10, 7, 5) // 68 фото, 9 смен папок
	bound := map[int]bool{}
	for i := 1; i < len(ph); i++ {
		if filepath.Dir(ph[i].Path) != filepath.Dir(ph[i-1].Path) {
			bound[i] = true
		}
	}
	j := PlanOf(model.New(), ph, nil)
	for _, o := range []model.AutoOpts{{Spreads: 10, Auto: true}, {Spreads: 10, MinPer: 3, MaxPer: 12}} {
		var hits [2]int
		for i, series := range []bool{false, true} {
			o.Series = series
			ap, pl, err := j.plan(o)
			if err != nil || pl.Spreads != 10 {
				t.Fatalf("%+v: %+v %v", o, ap, err)
			}
			for _, g := range pl.Groups {
				if bound[g[0]] {
					hits[i]++
				}
			}
			if k := sizesOf(pl.Groups); slices.Min(k) < ap.From || slices.Max(k) > ap.To {
				t.Fatalf("%+v: развороты %v вне [%d, %d]", o, k, ap.From, ap.To)
			}
			if want := map[bool]int{false: 0, true: 9}[series]; ap.Cuts != want {
				t.Fatalf("%+v: границ серий %d, ждали %d", o, ap.Cuts, want)
			}
		}
		t.Logf("%+v: на смене папки %d разрезов без серий, %d с сериями (из 9)", o, hits[0], hits[1])
		if hits[1] <= hits[0] || hits[1] < 7 {
			t.Fatalf("%+v: серии не тянут разрезы: %d → %d из 9", o, hits[0], hits[1])
		}
	}
}

// TestAutoBuildIsPlan (051: подсказка = сборка) на уровне групп: AutoBuild кладёт ровно группы
// plan при тех же опциях — ровно, «Авто», свой диапазон, с сериями и без. И когда вход под
// замком другой (те же пропорции, но другие папки и время), план считается заново, а не
// берётся из Prepare.
func TestAutoBuildIsPlan(t *testing.T) {
	ph := folders(5, 9, 4, 11, 6, 8, 3, 10, 7, 5)
	flat := slices.Clone(ph) // те же пропорции, одна папка, без времени — серий нет
	for i := range flat {
		flat[i].Path, flat[i].Taken = filepath.Join("/съёмка", "всё", fmt.Sprint(i, ".jpg")), 0
	}
	for _, o := range []model.AutoOpts{
		{Spreads: 10, MaxPer: 12}, {Spreads: 10, Auto: true}, {Spreads: 10, Auto: true, Series: true},
		{Spreads: 9, MinPer: 3, MaxPer: 12}, {Spreads: 9, MinPer: 3, MaxPer: 12, Series: true}, {Spreads: 99, MinPer: 6, MaxPer: 8, Series: true},
	} {
		for _, stale := range []bool{false, true} {
			p := model.New()
			j := PlanOf(p, ph, nil)
			_, want, err := j.plan(o)
			if err != nil {
				t.Fatal(err)
			}
			pre := Prepare(j, ph, o)
			if stale {
				pre = Prepare(PlanOf(p, flat, nil), ph, o)
			}
			if _, err := AutoBuild(p, pre, -1, nil); err != nil {
				t.Fatal(err)
			}
			var got []int
			for _, s := range p.Spreads {
				got = append(got, len(s.Photos))
			}
			if fmt.Sprint(got) != fmt.Sprint(sizesOf(want.Groups)) {
				t.Fatalf("%+v stale=%v: собрано %v, план %v", o, stale, got, sizesOf(want.Groups))
			}
		}
	}
	// Серии в самом деле меняют план: иначе проверка stale выше ничего не ловит.
	o := model.AutoOpts{Spreads: 10, Auto: true, Series: true}
	_, a, _ := PlanOf(model.New(), ph, nil).plan(o)
	_, b, _ := PlanOf(model.New(), flat, nil).plan(o)
	if fmt.Sprint(a.Groups) == fmt.Sprint(b.Groups) {
		t.Fatal("план с сериями совпал с планом без них")
	}
}

// TestPlanJobSameLib: вход плана с другим Lib — не тот же (Lib снимает смену папки из серий),
// поэтому Prepare под замком, где Lib сменился, не подсовывает прежний план. Времени нет —
// границы серий только по папкам, и Lib их убирает: планы в самом деле разные.
func TestPlanJobSameLib(t *testing.T) {
	ph := folders(5, 9, 4, 11, 6, 8, 3, 10, 7, 5)
	for i := range ph {
		ph[i].Taken = 0
	}
	lib := slices.Clone(ph)
	for i := range lib {
		lib[i].Lib = "my:0123456789abcdef"
	}
	if !PlanOf(model.New(), ph, nil).same(PlanOf(model.New(), slices.Clone(ph), nil)) {
		t.Fatal("одинаковый вход признан другим")
	}
	if PlanOf(model.New(), ph, nil).same(PlanOf(model.New(), lib, nil)) {
		t.Fatal("смена Lib не замечена")
	}
	o := model.AutoOpts{Spreads: 10, Auto: true, Series: true}
	_, a, _ := PlanOf(model.New(), ph, nil).plan(o)
	_, b, _ := PlanOf(model.New(), lib, nil).plan(o)
	if fmt.Sprint(a.Groups) == fmt.Sprint(b.Groups) {
		t.Fatal("Lib не меняет план: проверка stale ниже ничего не ловит")
	}
	p := model.New()
	pre := Prepare(PlanOf(p, lib, nil), ph, o) // посчитано для lib, а собираем ph
	if _, err := AutoBuild(p, pre, -1, nil); err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, s := range p.Spreads {
		got = append(got, len(s.Photos))
	}
	if fmt.Sprint(got) != fmt.Sprint(sizesOf(a.Groups)) {
		t.Fatalf("собрано %v, план без Lib %v", got, sizesOf(a.Groups))
	}
}
