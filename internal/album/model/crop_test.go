package model

import (
	"math"
	"testing"
)

// Подрезка (104): зажим шагов, Sheet → 0, Params и LayoutIn.

// TestClampCrop — шаги 0/5/10/15 остаются, прочее — к ближайшему (равное удаление — к меньшему),
// меньше 0 — 0, больше 15 — 15; у листа (Sheet) подрезки нет. ClampSettings не трогает остальное.
func TestClampCrop(t *testing.T) {
	cases := map[int]int{
		-100: 0, -1: 0, 0: 0, 1: 0, 2: 0, 3: 5, 5: 5, 7: 5, 8: 10, 10: 10, 12: 10, 13: 15, 15: 15, 16: 15, 1 << 30: 15,
	}
	for in, want := range cases {
		if got := ClampCrop(in); got != want {
			t.Errorf("ClampCrop(%d) = %d, ждали %d", in, got, want)
		}
		s := DefaultSettings()
		s.Crop = in
		if got := ClampSettings(s).Crop; got != want {
			t.Errorf("ClampSettings.Crop(%d) = %d, ждали %d", in, got, want)
		}
		s.Sheet = true
		if got := ClampSettings(s).Crop; got != 0 {
			t.Errorf("лист: Crop(%d) = %d, ждали 0", in, got)
		}
	}
	if s := DefaultSettings(); ClampSettings(s) != s {
		t.Error("ClampSettings изменил настройки по умолчанию")
	}
}

// TestLayoutIn — Params несёт Crop долей, Lock — только при Crop > 0 и по своим кадрам (Asp > 0)
// фото разворота; у листа Crop = 0. Несколько разворотов с разными кадрами — свои биты у каждого;
// пустой список и больше 64 фото не падают.
func TestLayoutIn(t *testing.T) {
	p := New()
	p.Photos = map[string]PhotoRef{
		"a": {W: 300, H: 200}, "b": {W: 300, H: 200, Asp: 0.8}, "c": {W: 200, H: 300, Asp: 1.5}, "d": {W: 200, H: 300},
	}
	asp, prm := p.LayoutIn([]string{"a", "b", "c", "d"})
	if prm.Crop != 0 || prm.Lock != 0 || len(asp) != 4 {
		t.Fatalf("Crop = 0: Params %+v", prm)
	}
	if got := p.Params(); got != prm {
		t.Fatal("при Crop = 0 пара LayoutIn ≠ Params")
	}
	p.Settings.Crop = 10
	asp, prm = p.LayoutIn([]string{"a", "b", "c", "d"})
	if prm.Crop != 0.1 || prm.Lock != 0b0110 || asp[1] != 0.8 || asp[2] != 1.5 {
		t.Fatalf("Crop = 10: Crop %v, Lock %b, asp %v", prm.Crop, prm.Lock, asp)
	}
	if _, prm = p.LayoutIn([]string{"d", "c"}); prm.Lock != 0b10 {
		t.Fatalf("второй разворот: Lock %b", prm.Lock)
	}
	if asp, prm = p.LayoutIn(nil); len(asp) != 0 || prm.Lock != 0 || prm.Crop != 0.1 {
		t.Fatalf("пустой список: %v %+v", asp, prm)
	}
	if p.Params().Lock != 0 {
		t.Fatal("Params уровня проекта несёт Lock")
	}
	many := make([]string, 70)
	for i := range many {
		many[i] = "b"
	}
	if _, prm = p.LayoutIn(many); prm.Lock != ^uint64(0) {
		t.Fatalf("70 своих кадров: Lock %b — ждали все 64 бита", prm.Lock)
	}
	p.Settings.Sheet = true
	if got := p.Params(); got.Crop != 0 || !got.Sheet {
		t.Fatalf("Params листа: %+v — подрезки быть не должно", got)
	}
}

// TestClampCropExtremes — крайние int: разность c-s у MinInt переполнялась и давала 15 вместо 0.
func TestClampCropExtremes(t *testing.T) {
	for _, c := range []struct{ in, want int }{
		{math.MinInt, 0}, {math.MinInt + 1, 0}, {math.MinInt / 2, 0}, {math.MaxInt, 15}, {math.MaxInt - 1, 15}, {math.MaxInt / 2, 15},
	} {
		if got := ClampCrop(c.in); got != c.want {
			t.Errorf("ClampCrop(%d) = %d, ждали %d", c.in, got, c.want)
		}
	}
}
