package plan

import (
	"math"
	"reflect"
	"slices"
	"testing"
)

// TestRangeWavePhase — сдвиг волны варианта (078): Phase 0 и отрицательная — волна 0.19
// побитово; Phase ≥ 1 держит сумму n и редкий первый разворот, а цели разных фаз различаются
// (на периоде каждая фаза — свой сдвиг); повторный вызов даёт то же.
func TestRangeWavePhase(t *testing.T) {
	for _, c := range []struct{ n, N, a, b int }{
		{100, 11, 3, 16}, // обычная волна (период 7)
		{60, 20, 2, 4},   // узкий диапазон: мягкая волна (период 12)
		{37, 5, 4, 12},
		{3, 1, 1, 3}, // одна цель: сдвигать нечего
		{0, 0, 1, 1}, // пусто
	} {
		base := rangeTargets(c.n, c.N, c.a, c.b)
		if !reflect.DeepEqual(base, rangeTargetsPhase(c.n, c.N, c.a, c.b, 0)) ||
			!reflect.DeepEqual(base, rangeTargetsPhase(c.n, c.N, c.a, c.b, -3)) {
			t.Fatalf("%+v: фаза 0 или отрицательная не равна волне 0.19", c)
		}
		seen := map[string]int{}
		period := 7
		if c.b-c.a <= 3 {
			period = 12
		}
		for ph := 1; ph <= period; ph++ {
			tg := rangeTargetsPhase(c.n, c.N, c.a, c.b, ph)
			if len(tg) != len(base) {
				t.Fatalf("%+v фаза %d: целей %d, ждали %d", c, ph, len(tg), len(base))
			}
			sum := 0.0
			for _, v := range tg {
				sum += v
			}
			if len(tg) > 0 && (math.Abs(sum-float64(c.n)) > 1e-9 || slices.Min(tg) != tg[0]) {
				t.Fatalf("%+v фаза %d: сумма %g, первая цель %g не самая малая (%v)", c, ph, sum, tg[0], tg)
			}
			if !reflect.DeepEqual(tg, rangeTargetsPhase(c.n, c.N, c.a, c.b, ph)) {
				t.Fatalf("%+v фаза %d: два вызова разошлись", c, ph)
			}
			seen[bitsOf(tg)] = ph
		}
		// Цели различаются между фазами периода, если есть что сдвигать.
		if c.N >= 12 && len(seen) < period {
			t.Errorf("%+v: различных наборов целей %d из %d фаз", c, len(seen), period)
		}
	}
}

// bitsOf — ключ набора целей для сравнения «побитово».
func bitsOf(tg []float64) string {
	b := make([]byte, 0, len(tg)*8)
	for _, v := range tg {
		u := math.Float64bits(v)
		for i := range 8 {
			b = append(b, byte(u>>(8*i)))
		}
	}
	return string(b)
}

// TestRangeWavePhasePlan — полный план с Phase: покрытие, границы, отличие от фазы 0 и
// неизменность фазы 0 против PlanOpts без поля.
func TestRangeWavePhasePlan(t *testing.T) {
	asp := weddingAspects(5, 200)
	o := PlanOpts{Spreads: 22, MinPer: 3, MaxPer: 16}
	base := PlanRange(asp, album30, o)
	o.Phase = 0
	if !reflect.DeepEqual(base, PlanRange(asp, album30, o)) {
		t.Fatal("Phase 0 изменила план")
	}
	diff := 0
	for ph := 1; ph <= 6; ph++ {
		o.Phase = ph
		pl := PlanRange(asp, album30, o)
		checkPlan(t, pl, 200, 22, 16)
		if !reflect.DeepEqual(pl, PlanRange(asp, album30, o)) {
			t.Fatalf("фаза %d недетерминирована", ph)
		}
		if !reflect.DeepEqual(pl.Groups, base.Groups) {
			diff++
		}
	}
	if diff < 5 {
		t.Errorf("границы отличаются от фазы 0 лишь у %d из 6 фаз", diff)
	}
}
