package model

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Имя подборки: пробелы и управляющие знаки схлопнуты, предел 40 рун (кириллица — руны, не байты).
func TestCleanBinName(t *testing.T) {
	long := strings.Repeat("я", MaxBinName+5)
	cases := map[string]string{
		"":                        "",
		"   ":                     "",
		" Обязательно ":           "Обязательно",
		"Для\tродителей\r\n\x00!": "Для родителей !",
		"a b":                     "a b",
		long:                      strings.Repeat("я", MaxBinName),
		strings.Repeat("я", MaxBinName-1) + " x": strings.Repeat("я", MaxBinName-1),
		"bad\xffutf": "bad utf",
	}
	for in, want := range cases {
		if got := CleanBinName(in); got != want {
			t.Errorf("CleanBinName(%q) = %q, ждали %q", in, got, want)
		}
		if n := utf8.RuneCountInString(CleanBinName(in)); n > MaxBinName {
			t.Errorf("%q: %d рун", in, n)
		}
	}
}

// Свободное имя — первое незанятое «Подборка N» без учёта регистра; пустой список — «Подборка 1».
func TestFreeBinName(t *testing.T) {
	if got := FreeBinName(nil); got != "Подборка 1" {
		t.Fatalf("пусто: %q", got)
	}
	bins := []Bin{{"b1", "подборка 1"}, {"b2", "Подборка 3"}}
	if got := FreeBinName(bins); got != "Подборка 2" {
		t.Fatalf("дыра: %q", got)
	}
	bins = append(bins, Bin{"b3", "Подборка 2"})
	if got := FreeBinName(bins); got != "Подборка 4" {
		t.Fatalf("подряд: %q", got)
	}
}

// Занятость имени: регистр и лишние пробелы не в счёт, своя подборка (except) — не помеха.
func TestBinNameTaken(t *testing.T) {
	bins := []Bin{{"b1", "Обязательно"}, {"b2", "Запас"}}
	for name, want := range map[string]bool{"обязательно": true, "  ЗАПАС ": true, "Запас 2": false, "": false} {
		if got := BinNameTaken(bins, name, ""); got != want {
			t.Errorf("%q: %v", name, got)
		}
	}
	if BinNameTaken(bins, "обязательно", "b1") || !BinNameTaken(bins, "обязательно", "b2") {
		t.Fatal("except не учтён")
	}
	if BinNameTaken(nil, "x", "") {
		t.Fatal("пустой список")
	}
}

func TestOkBinID(t *testing.T) {
	for id, want := range map[string]bool{
		"b3f9a01c2": true, "0": true, strings.Repeat("a", 16): true,
		"": false, strings.Repeat("a", 17): false, "B1": false, "b-1": false, "b 1": false, "../x": false, "б1": false,
	} {
		if OkBinID(id) != want {
			t.Errorf("%q: ждали %v", id, want)
		}
	}
}

// Новый ID — верный, «b» + 8 hex, занятый пропускается.
func TestNewBinID(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		id := NewBinID(func(s string) bool { return seen[s] })
		if !OkBinID(id) || len(id) != 9 || id[0] != 'b' || seen[id] {
			t.Fatalf("ID %q", id)
		}
		seen[id] = true
	}
	first, calls := "", 0
	id := NewBinID(func(s string) bool {
		calls++
		if first == "" {
			first = s
			return true
		}
		return false
	})
	if calls != 2 || id == first {
		t.Fatalf("занятый ID не пропущен: %q %q, вызовов %d", first, id, calls)
	}
}

// Зажим из файла: плохой и повторный ID прочь, имена чистые и разные, пустое — «Подборка N»,
// сверх MaxBins — хвост; fixed считает поправки.
func TestClampBins(t *testing.T) {
	if out, fixed := ClampBins(nil); out != nil || fixed != 0 {
		t.Fatalf("пусто: %v %d", out, fixed)
	}
	long := strings.Repeat("ж", MaxBinName)
	in := []Bin{
		{"b1", "Обязательно"},
		{"b1", "Повтор ID"},
		{"Bad", "Плохой ID"},
		{"", "Без ID"},
		{"b2", " обязательно "},
		{"b3", ""},
		{"b4", "\t\n"},
		{"b5", long},
		{"b6", long},
		{"b7", "Запас"},
	}
	out, fixed := ClampBins(in)
	want := []Bin{
		{"b1", "Обязательно"},
		{"b2", "обязательно 2"},
		{"b3", "Подборка 1"},
		{"b4", "Подборка 2"},
		{"b5", long},
		{"b6", strings.Repeat("ж", MaxBinName-2) + " 2"},
		{"b7", "Запас"},
	}
	if !slices.Equal(out, want) {
		t.Fatalf("зажим:\n%v\nждали\n%v", out, want)
	}
	if fixed != 3+4 { // три отброшены, четыре имени поправлены
		t.Fatalf("поправок %d", fixed)
	}
	// Предел: 50 остаются, хвост прочь; верный список — без поправок.
	var many []Bin
	for i := range MaxBins + 3 {
		many = append(many, Bin{fmt.Sprintf("b%d", i), fmt.Sprintf("П %d", i)})
	}
	out, fixed = ClampBins(many)
	if len(out) != MaxBins || fixed != 3 || out[MaxBins-1].ID != fmt.Sprintf("b%d", MaxBins-1) {
		t.Fatalf("предел: %d, поправок %d", len(out), fixed)
	}
	if out, fixed = ClampBins(many[:MaxBins]); len(out) != MaxBins || fixed != 0 {
		t.Fatalf("ровно предел: %d, поправок %d", len(out), fixed)
	}
	if set := BinSet(out); len(set) != MaxBins || !set["b0"] || set["b50"] {
		t.Fatalf("BinSet: %d", len(set))
	}
}

// Членство: WithBins сортирует и убирает повторы, пусто — nil; None и SameMark видят подборки;
// Clone, View и Side.Clone не делят срез с исходником.
func TestMarkBins(t *testing.T) {
	m := Mark{Path: "a"}.WithBins([]string{"b2", "b1", "b2"})
	if !slices.Equal(m.Bins, []string{"b1", "b2"}) || m.None() {
		t.Fatalf("WithBins: %v, None %v", m.Bins, m.None())
	}
	if e := m.WithBins([]string{}); e.Bins != nil || !e.None() {
		t.Fatalf("пустое членство: %#v", e.Bins)
	}
	if SameMark(m, m.WithBins([]string{"b1"})) || !SameMark(m, m.WithBins([]string{"b2", "b1"})) {
		t.Fatal("SameMark не видит подборки")
	}
	c, v := m.Clone(), m.View()
	c.Bins[0], v.Bins[1] = "x", "y"
	if m.Bins[0] != "b1" || m.Bins[1] != "b2" {
		t.Fatalf("Clone/View делят срез: %v", m.Bins)
	}
	s := Side{Marks: map[string]Mark{"a": m}, Bins: []Bin{{"b1", "Обязательно"}, {"b2", "Запас"}}}
	cs := s.Clone()
	cs.Bins[0].Name = "x"
	cs.Marks["a"].Bins[0] = "x"
	delete(cs.Marks, "a")
	if s.Bins[0].Name != "Обязательно" || s.Marks["a"].Bins[0] != "b1" || len(s.Marks) != 1 {
		t.Fatalf("Side.Clone делит данные: %+v", s)
	}
	if z := (Side{}).Clone(); z.Marks != nil || z.Bins != nil {
		t.Fatalf("пустая сторона: %+v", z)
	}
}
