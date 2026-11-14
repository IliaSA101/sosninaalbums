package model

import (
	"strings"
	"testing"
)

// ClampLookRef: ключ решает, есть ли макет; набор и запись — парой; имя — одна строка ≤ 60.
func TestClampLookRef(t *testing.T) {
	const key = "my:0123456789abcdef"
	if ClampLookRef(nil) != nil || ClampLookRef(&LookRef{Key: "my:x"}) != nil {
		t.Fatal("нет ключа — нет макета")
	}
	in := &LookRef{Key: key, Name: "Свадьба\tлето", Pack: "wedding-layouts", Item: "a/b.json"}
	if got := ClampLookRef(in); got == in || *got != (LookRef{Key: key, Name: "Свадьба лето", Pack: "wedding-layouts", Item: "a/b.json"}) {
		t.Fatalf("годная ссылка: %+v (копия: %v)", got, got != in)
	}
	for _, item := range []string{"\xff.json", "a\u0085b", "..", "a/../b", strings.Repeat("x", maxLookItem+1),
		"a\u202eb.json", "a\u2066b", "a\u200bb", "a\u200db", "\ufeffa.json", "a\u00adb", "a\u2060b"} {
		if got := ClampLookRef(&LookRef{Key: key, Pack: "wedding", Item: item}); got.Pack != "" || got.Item != "" {
			t.Errorf("запись %q: %+v", item, got)
		}
	}
	// Остальное не сужено (0.28-131-11): кириллица, пробел, эмодзи, точка в имени — годны.
	for _, item := range []string{"Классика/лето 2026.json", "a.b/c.json", "😀.json", "a/b/c"} {
		if got := ClampLookRef(&LookRef{Key: key, Pack: "wedding", Item: item}); got.Item != item {
			t.Errorf("запись %q снята: %+v", item, got)
		}
	}
	for _, id := range []string{"con", "prn", "aux", "nul", "com0", "lpt9", "ab", "a_b", "Abc", "-ab"} {
		if ValidPackID(id) {
			t.Errorf("id набора %q принят", id)
		}
	}
	for _, id := range []string{"abc", "com", "com10", "lpt", "a-b", "0ab", strings.Repeat("a", 40)} {
		if !ValidPackID(id) {
			t.Errorf("id набора %q отвергнут", id)
		}
	}
}

// FixLook: без макета шагов нет; шаг вне 1…24 и у голого листа (одни фото) — 0, имя главы и надпись шаг держат; у обложки — всегда 0;
// несколько листов сразу; повтор ничего не меняет.
func TestFixLook(t *testing.T) {
	p := New()
	p.Look = &LookRef{Key: "my:0123456789abcdef"}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID, BgL: "#ffffff", LookStep: 2}}
	pl := []*Place{{ID: "0a0000000001", W: 10, H: 10}}
	p.Spreads = []*Spread{
		{ID: "s1", Grid: pl, LookStep: 1},
		{ID: "s2", Stickers: []*Sticker{{ID: "st"}}, LookStep: MaxLookSteps},
		{ID: "s3", Pat: &Pattern{}, LookStep: MaxLookSteps + 1},
		{ID: "s4", BgPhotoR: &Backdrop{}, LookStep: -1},
		{ID: "s5", Photos: []string{"a"}, LookStep: 5},
		{ID: "s6", BgR: "#000000", LookStep: 7},
		// Имя главы и надписи держат номер: шаг из одной надписи или со стикером без файла на этом ПК
		// иначе терял бы его после постановки, и повтор макета создавал бы разворот заново.
		{ID: "s7", Name: "Глава", LookStep: 3},
		{ID: "s8", Texts: []*TextBlock{{ID: "t"}}, LookStep: 4},
	}
	want := []int{1, MaxLookSteps, 0, 0, 0, 7, 3, 4}
	for range 2 {
		p.FixLook()
		for i, s := range p.Spreads {
			if s.LookStep != want[i] {
				t.Fatalf("лист %s: шаг %d, ждали %d", s.ID, s.LookStep, want[i])
			}
		}
		if p.Cover.Sheet.LookStep != 0 || p.Look == nil {
			t.Fatalf("обложка %d, макет %+v", p.Cover.Sheet.LookStep, p.Look)
		}
	}
	p.Look.Key = "битый"
	p.FixLook()
	if p.Look != nil || p.Spreads[0].LookStep != 0 || p.Spreads[5].LookStep != 0 {
		t.Fatalf("битый ключ: %+v, шаги %d %d", p.Look, p.Spreads[0].LookStep, p.Spreads[5].LookStep)
	}
	// Лист с одним номером шага пуст: его можно удалить молча.
	if !(&Spread{ID: "x"}).Empty() || (&Spread{ID: "x", LookStep: 1}).Empty() {
		t.Fatal("Empty и LookStep")
	}
}
