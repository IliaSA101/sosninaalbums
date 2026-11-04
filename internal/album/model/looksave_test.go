package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
)

// stepC — шаг с цветом c на обеих страницах: разные цвета — разные TplKey.
func stepC(c string) LookStep {
	return LookStep{Tpl: SpreadTpl{V: TplV, Pages: 2, Ratio: 2, Page: []TplBg{{Color: c}, {Color: c}}}}
}

func rolesOf(st []LookStep) []string {
	out := make([]string, len(st))
	for i, s := range st {
		out[i] = s.Role
	}
	return out
}

func colorsOf(st []LookStep) []string {
	out := make([]string, len(st))
	for i, s := range st {
		out[i] = s.Tpl.Page[0].Color
	}
	return out
}

// TestMergeLookSteps: повторяемые сливаются по TplKey (первый остаётся, порядок круга цел), начало
// и конец — как есть, даже одинаковые; вход не меняется.
func TestMergeLookSteps(t *testing.T) {
	in := []LookStep{stepC("#000001"), stepC("#00000a"), stepC("#00000b"), stepC("#00000a"), stepC("#00000b"), stepC("#00000a"), stepC("#000001")}
	rep := []bool{false, true, true, true, true, true, false}
	cp := slices.Clone(in)
	got, err := MergeLookSteps(in, rep)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"#000001", "#00000a", "#00000b", "#000001"}; !slices.Equal(colorsOf(got), want) {
		t.Errorf("шаги: %v, ждали %v", colorsOf(got), want)
	}
	if want := []string{"", RoleRepeat, RoleRepeat, ""}; !slices.Equal(rolesOf(got), want) {
		t.Errorf("роли: %q, ждали %q", rolesOf(got), want)
	}
	if !slices.EqualFunc(in, cp, func(a, b LookStep) bool { return a.Role == b.Role && TplKey(a.Tpl) == TplKey(b.Tpl) }) {
		t.Error("вход изменился")
	}
	// Без повтора одинаковые шаги не сливаются.
	got, err = MergeLookSteps([]LookStep{stepC("#000001"), stepC("#000001")}, nil)
	if err != nil || len(got) != 2 || got[0].Role != "" {
		t.Errorf("без повтора: %v, %d", err, len(got))
	}
	// Пустой вход — пустая серия без ошибки (решает вызывающий).
	if got, err = MergeLookSteps(nil, nil); err != nil || len(got) != 0 {
		t.Errorf("пусто: %v, %d", err, len(got))
	}
	// rep короче шагов: недостающее — «один раз».
	if got, _ = MergeLookSteps([]LookStep{stepC("#000001"), stepC("#00000a")}, []bool{true}); !slices.Equal(rolesOf(got), []string{RoleRepeat, ""}) {
		t.Errorf("короткий rep: %q", rolesOf(got))
	}
}

// TestMergeLookStepsLimit: 24 разных — серия, 25 — ErrLookMany; слитые повторы в счёт идут один раз.
func TestMergeLookStepsLimit(t *testing.T) {
	mk := func(n int) []LookStep {
		out := make([]LookStep, n)
		for i := range out {
			out[i] = stepC(fmt.Sprintf("#0000%02x", i))
		}
		return out
	}
	if got, err := MergeLookSteps(mk(MaxLookSteps), nil); err != nil || len(got) != MaxLookSteps {
		t.Errorf("24: %v, %d", err, len(got))
	}
	if _, err := MergeLookSteps(mk(MaxLookSteps+1), nil); !errors.Is(err, ErrLookMany) {
		t.Errorf("25 без повтора: %v", err)
	}
	all := make([]bool, MaxLookSteps+1)
	for i := range all {
		all[i] = true
	}
	if _, err := MergeLookSteps(mk(MaxLookSteps+1), all); !errors.Is(err, ErrLookMany) {
		t.Errorf("25 разных повторяемых: %v", err)
	}
	dup := append(mk(MaxLookSteps), stepC("#000000")) // 25-й равен первому
	if got, err := MergeLookSteps(dup, all); err != nil || len(got) != MaxLookSteps {
		t.Errorf("25 с повтором: %v, %d", err, len(got))
	}
}

// TestMergeLookStepsVignette: виньетка никогда не повторяемая; между повторяемыми — ErrLookVigRep.
func TestMergeLookStepsVignette(t *testing.T) {
	vig := LookStep{Role: RoleVignette, Vig: &LookVig{Tpl: "x"}, Tpl: stepC("#000001").Tpl}
	got, err := MergeLookSteps([]LookStep{vig, stepC("#00000a"), stepC("#00000b")}, []bool{true, true, true})
	if err != nil || !slices.Equal(rolesOf(got), []string{RoleVignette, RoleRepeat, RoleRepeat}) {
		t.Errorf("виньетка на краю диапазона: %v, %q", err, rolesOf(got))
	}
	got, err = MergeLookSteps([]LookStep{stepC("#00000a"), stepC("#00000b"), vig}, []bool{true, true, true})
	if err != nil || !slices.Equal(rolesOf(got), []string{RoleRepeat, RoleRepeat, RoleVignette}) {
		t.Errorf("виньетка в конце: %v, %q", err, rolesOf(got))
	}
	if _, err = MergeLookSteps([]LookStep{stepC("#00000a"), vig, stepC("#00000b")}, []bool{true, true, true}); !errors.Is(err, ErrLookVigRep) {
		t.Errorf("виньетка среди повторяемых: %v", err)
	}
}

// TestSaveLookOptsJSON: поля параметров совпадают с контрактом окна (savelook.ts).
func TestSaveLookOptsJSON(t *testing.T) {
	var o SaveLookOpts
	if err := json.Unmarshal([]byte(`{"theme":"mine","name":"Н","ids":["a","b"],"repFrom":2,"repTo":1,"places":true,"drafts":true,"cover":true}`), &o); err != nil {
		t.Fatal(err)
	}
	if o.Theme != "mine" || o.Name != "Н" || len(o.Ids) != 2 || o.RepFrom != 2 || o.RepTo != 1 || !o.Places || !o.Drafts || !o.Cover {
		t.Errorf("%+v", o)
	}
}
