package variety

import (
	"slices"
	"testing"
)

// TestPickSeedZeroIsOld — варианты 0 и 1 (и отрицательный) — выбор 0.19 побитово на всём
// корпусе и всех уровнях, с соседями цепочки и без.
func TestPickSeedZeroIsOld(t *testing.T) {
	ctx := Ctx{Prev: []string{"a", "b"}, PrevKey: "k", Next: "n", Since: 3, SinceHero: 2}
	for _, a := range corpus() {
		for lv := Less; lv <= More; lv++ {
			for _, c := range []Ctx{{}, ctx} {
				want := Pick(a.spreads, a.p, lv, c)
				for _, seed := range []int{0, 1, -5} {
					c.Seed = seed
					if got := Pick(a.spreads, a.p, lv, c); !slices.Equal(got, want) {
						t.Fatalf("%s, уровень %d, Seed %d: выбор изменился", a.name, lv, seed)
					}
				}
			}
		}
	}
}

// TestPickSeedDeterministic — тот же номер на том же входе даёт тот же выбор, «шум» в [0, 1)
// и зависит от номера и места; пустые пулы остаются −1.
func TestPickSeedDeterministic(t *testing.T) {
	for _, a := range corpus()[:6] {
		for lv := Less; lv <= More; lv++ {
			for seed := 2; seed <= 6; seed++ {
				c := Ctx{Seed: seed}
				if !slices.Equal(Pick(a.spreads, a.p, lv, c), Pick(a.spreads, a.p, lv, c)) {
					t.Fatalf("%s, уровень %d, Seed %d: повторный Pick дал другое", a.name, lv, seed)
				}
			}
		}
	}
	u := seedNoise(3, 7, "H[a|b]")
	for range 3 {
		if v := seedNoise(3, 7, "H[a|b]"); v != u || v < 0 || v >= 1 {
			t.Fatalf("шум не стабилен или вне [0,1): %v и %v", v, u)
		}
	}
	if seedNoise(3, 7, "x") == seedNoise(4, 7, "x") || seedNoise(3, 7, "x") == seedNoise(3, 8, "x") {
		t.Error("шум не зависит от номера или места")
	}
	empty := []Spread{{Star: -1}, {Pool: []Cand{cand("a", "a", 1, true)}, Star: -1}}
	for lv := Less; lv <= More; lv++ {
		if got := Pick(empty, geomSq, lv, Ctx{Seed: 4}); got[0] != -1 || got[1] != 0 {
			t.Errorf("уровень %d: пустой пул с вариантом: %v", lv, got)
		}
	}
}

// TestPickSeedDiffers — варианты 2–6 выбирают другие раскладки, чем вариант 1: на корпусе
// не меньше трети разворотов у «Обычного» и «Больше», у «Меньше» — заметная доля.
func TestPickSeedDiffers(t *testing.T) {
	floor := map[Level]float64{Less: 0.15, Normal: 0.30, More: 0.30}
	for lv := Less; lv <= More; lv++ {
		var diff, all int
		for seed := 2; seed <= 6; seed++ {
			for _, a := range corpus() {
				base := Pick(a.spreads, a.p, lv, Ctx{})
				got := Pick(a.spreads, a.p, lv, Ctx{Seed: seed})
				for i := range got {
					all++
					if got[i] != base[i] {
						diff++
					}
				}
			}
		}
		share := ratio(diff, all)
		t.Logf("уровень %d: отличается %.0f%% (%d/%d)", lv, 100*share, diff, all)
		if share < floor[lv] {
			t.Errorf("уровень %d: варианты 2–6 отличаются у %.0f%% разворотов, ждали ≥ %.0f%%", lv, 100*share, 100*floor[lv])
		}
	}
}

// TestPickSeedKeepsRules — правила выбора у вариантов не хуже варианта 1 больше чем на 10 %:
// повторы вида, соседи одного ключа, доля «звезда — главное», средняя потеря, σ заполнения.
func TestPickSeedKeepsRules(t *testing.T) {
	var less total // «Меньше» без вариантов — потолок для соседей с одним ключом
	for _, a := range corpus() {
		less.add(measure(a, Less))
	}
	for lv := Normal; lv <= More; lv++ {
		var base total
		for _, a := range corpus() {
			base.add(measureCtx(a, lv, Ctx{}))
		}
		for seed := 2; seed <= 6; seed++ {
			var tt total
			for _, a := range corpus() {
				tt.add(measureCtx(a, lv, Ctx{Seed: seed}))
			}
			t.Logf("уровень %d, вариант %d: повторов %.1f%% (было %.1f%%), ключей %d (%d), звезда %.0f%% (%.0f%%), потеря %.4f (%.4f), σ %.3f (%.3f)",
				lv, seed, 100*tt.repeatShare(), 100*base.repeatShare(), tt.sameKeys, base.sameKeys,
				100*tt.starShare(), 100*base.starShare(), tt.avgLoss(), base.avgLoss(), tt.sigma(), base.sigma())
			if tt.repeatShare() > base.repeatShare()*1.1+0.002 {
				t.Errorf("уровень %d, вариант %d: повторов вида %.1f%% > %.1f%% × 1,1", lv, seed, 100*tt.repeatShare(), 100*base.repeatShare())
			}
			// Абсолют мал (0–5 на 24 альбома), поэтому не «+10 % к варианту 1», а не больше десятой доли «Меньше».
			if tt.sameKeys*10 > less.sameKeys {
				t.Errorf("уровень %d, вариант %d: соседей с одним ключом %d (вариант 1: %d, «Меньше»: %d)", lv, seed, tt.sameKeys, base.sameKeys, less.sameKeys)
			}
			if tt.starShare() < base.starShare()*0.9 {
				t.Errorf("уровень %d, вариант %d: звезда главная %.2f < %.2f × 0,9", lv, seed, tt.starShare(), base.starShare())
			}
			if tt.avgLoss() > base.avgLoss()*1.1 {
				t.Errorf("уровень %d, вариант %d: средняя потеря %.4f > %.4f × 1,1", lv, seed, tt.avgLoss(), base.avgLoss())
			}
			if tt.sigma() < base.sigma()*0.9 {
				t.Errorf("уровень %d, вариант %d: σ заполнения %.3f < %.3f × 0,9", lv, seed, tt.sigma(), base.sigma())
			}
		}
	}
}

// TestPickSeedLess — «Меньше» с вариантом берёт только ряд «Авто» с потерей ≤ 0,02, лучшая
// всегда допустима; номер варианта перебирает допустимые, вариант 1 — лучшая.
func TestPickSeedLess(t *testing.T) {
	s := Spread{Star: -1, Pool: []Cand{
		cand("a", "a", 1.00, true), cand("b", "b", 0.99, true), cand("c", "c", 0.50, true),
		cand("d", "d", 0.995, false), // вне ряда «Авто»
	}}
	seen := map[int]bool{}
	for seed := 2; seed < 40; seed++ {
		k := Pick([]Spread{s}, geomSq, Less, Ctx{Seed: seed})[0]
		if k != 0 && k != 1 {
			t.Fatalf("Seed %d: выбран %d — вне допустимых [0 1]", seed, k)
		}
		seen[k] = true
	}
	if len(seen) != 2 {
		t.Errorf("варианты не перебирают допустимые раскладки: %v", seen)
	}
	if k := Pick([]Spread{s}, geomSq, Less, Ctx{Seed: 1})[0]; k != 0 {
		t.Errorf("вариант 1: %d, ждали лучшую 0", k)
	}
}
