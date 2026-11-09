package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestLinkRef(t *testing.T) {
	uid := NewUID()
	ref := LinkRef(uid, "album-1")
	if !GoodLinkRef(ref) {
		t.Fatalf("ref %q не годится", ref)
	}
	if LinkRef(uid, "album-1") != ref {
		t.Error("ref не детерминирован")
	}
	// uid — соль, id — различитель; порядок не путается (uid="a",id="bc" ≠ uid="ab",id="c").
	if LinkRef(uid, "album-2") == ref || LinkRef(NewUID(), "album-1") == ref {
		t.Error("ref не зависит от uid или id")
	}
	if LinkRef("", "x") != "" || LinkRef("не-uid", "x") != "" || LinkRef(uid, "") != "" {
		t.Error("негодный вход должен давать пустой ref")
	}
	// Эталон формата: смена алгоритма ломает поиск по файлам прежних версий.
	const fixed = "00112233445566778899aabbccddeeff"
	// Значение посчитано отдельно: printf 'sa-link\0<uid>\0abc' | sha256sum | cut -c1-32.
	if got := LinkRef(fixed, "abc"); got != "214a49ffc9ffe085151527b240e67283" {
		t.Errorf("эталон ref: %q", got)
	}
}

func TestGoodLinkRef(t *testing.T) {
	good := strings.Repeat("a1", 16)
	if !GoodLinkRef(good) {
		t.Error("годная отвергнута")
	}
	for _, bad := range []string{"", good[:31], good + "0", strings.ToUpper(good), strings.Repeat("g", 32), strings.Repeat("a", 31) + "я"} {
		if GoodLinkRef(bad) {
			t.Errorf("негодная принята: %q", bad)
		}
	}
}

func TestAddLinkRef(t *testing.T) {
	uid := NewUID()
	var refs []string
	var all []string
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		r := LinkRef(uid, id)
		all = append(all, r)
		refs = AddLinkRef(refs, r)
	}
	// Новая первой, лишнее отброшено: остались e, d, c, b.
	want := []string{all[4], all[3], all[2], all[1]}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("после пяти: %v", refs)
	}
	// Дубль переезжает вперёд без роста; исходный срез не правится.
	before := append([]string(nil), refs...)
	got := AddLinkRef(refs, all[2])
	if !reflect.DeepEqual(got, []string{all[2], all[4], all[3], all[1]}) || !reflect.DeepEqual(refs, before) {
		t.Errorf("дубль: %v, исходный %v", got, refs)
	}
	// Негодная привязка список не меняет.
	if got := AddLinkRef(refs, "xx"); !reflect.DeepEqual(got, refs) {
		t.Errorf("негодная изменила список: %v", got)
	}
	if got := AddLinkRef(nil, ""); len(got) != 0 {
		t.Errorf("пустая из nil: %v", got)
	}
}

func TestCleanLinkRefs(t *testing.T) {
	uid := NewUID()
	r := func(id string) string { return LinkRef(uid, id) }
	if CleanLinkRefs(nil) != nil || CleanLinkRefs([]string{}) != nil {
		t.Error("пустой вход — nil")
	}
	got := CleanLinkRefs([]string{r("a"), "bad", r("a"), strings.ToUpper(r("b")), r("c"), r("d"), r("e"), r("f")})
	if want := []string{r("a"), r("c"), r("d"), r("e")}; !reflect.DeepEqual(got, want) {
		t.Errorf("очистка: %v", got)
	}
	// Массив длиннее предела — отброшен целиком, даже если начало годное.
	long := make([]string, RawShareRefs+1)
	for i := range long {
		long[i] = r("a")
	}
	if got := CleanLinkRefs(long); got != nil {
		t.Errorf("длинный массив не отброшен: %v", got)
	}
	// Ровно предел — разбирается.
	if got := CleanLinkRefs(long[:RawShareRefs]); len(got) != 1 {
		t.Errorf("массив на пределе: %v", got)
	}
}

func TestProjectShareRefsOutsideJSON(t *testing.T) {
	p := New()
	p.UID = NewUID()
	p.SetShareRefs([]string{LinkRef(p.UID, "x"), "bad"})
	if len(p.ShareRefs()) != 1 {
		t.Fatalf("привязки: %v", p.ShareRefs())
	}
	if strings.Contains(string(Marshal(p)), p.ShareRefs()[0]) {
		t.Error("привязка попала в JSON снимка")
	}
	if len(p.Clone().ShareRefs()) != 0 {
		t.Error("Clone унёс привязки (поле вне JSON)")
	}
	// ShareRefs отдаёт копию.
	got := p.ShareRefs()
	got[0] = "испорчено"
	if p.ShareRefs()[0] == "испорчено" {
		t.Error("ShareRefs отдал внутренний срез")
	}
}
