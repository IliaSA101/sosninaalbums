package gen

import (
	"math"
	"reflect"
	"testing"
)

// TestFrameMetaFrozen — таблица видов заморожена (079-28): толщина по умолчанию и Ins выпущенного
// вида держат окно фото у альбомов с Width = 0. Правка вида — новый ключ, а не правка строки.
func TestFrameMetaFrozen(t *testing.T) {
	want := map[string]struct {
		w    float64
		ins  [4]float64
		inks int
	}{
		"line": {1.5, [4]float64{1, 1, 1, 1}, 1}, "double": {3, [4]float64{1, 1, 1, 1}, 1},
		"mat": {8, [4]float64{1, 1, 1, 1}, 2}, "polaroid": {5, [4]float64{1, 1, 1, 3.5}, 1},
		"stitch": {0.6, [4]float64{}, 1}, "corners": {7, [4]float64{}, 1},
		"gold": {4, [4]float64{1, 1, 1, 1}, 2}, "scallop": {4, [4]float64{1, 1, 1, 1}, 1},
		"soft": {8, [4]float64{}, 0},
	}
	fs := Frames()
	if len(fs) != len(want) {
		t.Fatalf("видов %d, ждали %d", len(fs), len(want))
	}
	seen := map[string]bool{}
	for _, m := range fs {
		w, ok := want[m.Kind]
		if !ok || seen[m.Kind] || !ValidKind(m.Kind) || m.Name == "" {
			t.Errorf("%+v: чужой, повтор, грамматика или имя", m)
			continue
		}
		seen[m.Kind] = true
		if m.Width != w.w || m.Ins != w.ins || len(m.Inks) != w.inks || m.Mask != (m.Kind == "soft") {
			t.Errorf("%s: толщина %v, Ins %v, цветов %d, маска %v — вид выпущен, менять нельзя", m.Kind, m.Width, m.Ins, len(m.Inks), m.Mask)
		}
		if got := ClampFrameInks(m.Kind, m.Inks); !reflect.DeepEqual(got, nilIfEmpty(m.Inks)) {
			t.Errorf("%s: цвета по умолчанию не зажаты: %v", m.Kind, got)
		}
		if ClampFrameWidth(m.Width) != m.Width {
			t.Errorf("%s: толщина по умолчанию вне зажима", m.Kind)
		}
	}
	// Копии: правка снаружи таблицу не трогает.
	fs[0].Inks[0] = "#000000"
	if m, _ := FrameLookup(fs[0].Kind); m.Inks[0] == "#000000" {
		t.Fatal("Frames отдал общий срез цветов")
	}
	if FrameKnown("nope") || !FrameKnown("line") {
		t.Fatal("FrameKnown")
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// TestClampFrameInks — цвета: регистр, предел вида, плохой — цвет вида; у неизвестного — до 2 и
// обрыв на плохом.
func TestClampFrameInks(t *testing.T) {
	cases := []struct {
		kind string
		in   []string
		want []string
	}{
		{"line", nil, nil},
		{"line", []string{}, nil},
		{"line", []string{"#ABCDEF", "#000000"}, []string{"#abcdef"}},
		{"line", []string{"red"}, []string{"#ffffff"}},
		{"mat", []string{"#111111", "bad", "#222222"}, []string{"#111111", "#c8a24a"}},
		{"soft", []string{"#111111"}, nil},
		{"new-kind", []string{"#1", "#222222"}, nil},
		{"new-kind", []string{"#111111", "#222222", "#333333", "#444444", "#555555"}, []string{"#111111", "#222222"}},
	}
	for _, c := range cases {
		if got := ClampFrameInks(c.kind, c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %v → %v, ждали %v", c.kind, c.in, got, c.want)
		}
	}
}

// TestClampFrameWidth — толщина: 0 — по умолчанию, края 0,2 и 20, шаг 0,1 без хвоста дроби.
func TestClampFrameWidth(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, -1: 0, 0.01: 0.2, 0.26: 0.3, 7.04: 7, 20: 20, 1e308: 20} {
		if got := ClampFrameWidth(in); got != want || ClampFrameWidth(got) != got {
			t.Errorf("%v → %v, ждали %v", in, got, want)
		}
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if ClampFrameWidth(v) != 0 {
			t.Errorf("%v → %v", v, ClampFrameWidth(v))
		}
	}
}
