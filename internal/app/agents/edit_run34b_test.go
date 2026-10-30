package agents

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// Доправка 151 по итогам ревью и аудита: формы id правящих инструментов и устойчивые маски типографий.

// TestEditIDForms: правящие инструменты принимают только формы id, которые выдаёт чтение (ID файла;
// копию «id-2» — как файл). «id-new» (запрос копии интерфейса), пути и «..» — skipped или отказ, в
// Host они не доходят. Страж на set_marks, set_cover_photo и ids листа get_contact_sheet.
func TestEditIDForms(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	bad := []string{idB + "-new", idA + "-new", "../" + idA, "/etc/passwd", "C:\\x\\" + idA + ".jpg", idA + "/../" + idB, "-new"}
	// set_marks: годный id идёт в Host, негодные — в skipped (после очистки), Host «-new» не видит.
	ids := []any{idB}
	for _, b := range bad {
		ids = append(ids, b)
	}
	rep := okReply(t, tcall(t, cs, "set_marks", args34(r, map[string]any{"ids": ids, "flag": "pick"})))
	for _, id := range r.fx.edit.got34().lastMark.IDs {
		if model.IsCopyReq(id) {
			t.Fatalf("запрос копии дошёл до Host: %v", r.fx.edit.got34().lastMark.IDs)
		}
	}
	sk, _ := rep.Data["skipped"].([]any)
	for _, want := range []string{idB + "-new", idA + "-new"} {
		if !slices.Contains(sk, any(want)) {
			t.Errorf("%s нет в skipped %v", want, sk)
		}
	}
	// Только запрос копии: Host не получает ни одного id, правки нет.
	rep = okReply(t, tcall(t, cs, "set_marks", args34(r, map[string]any{"ids": []any{idA + "-new"}, "flag": "pick"})))
	if got := r.fx.edit.got34().lastMark.IDs; len(got) != 0 {
		t.Fatalf("запрос копии дошёл до Host: %v", got)
	}
	if sk, _ := rep.Data["skipped"].([]any); len(sk) != 1 || sk[0] != idA+"-new" {
		t.Fatalf("skipped: %s", rep.Raw)
	}
	// set_cover_photo и лист-сетка: такого фото нет в ленте.
	n := r.fx.edit.nCover.Load()
	for _, b := range bad {
		failText(t, tcall(t, cs, "set_cover_photo", args34(r, map[string]any{"photo": b})), errNoPhoto)
		if rp := tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{b}}); !rp.IsError {
			t.Errorf("get_contact_sheet принял id %q", b)
		}
	}
	if r.fx.edit.nCover.Load() != n {
		t.Fatal("Host.Cover вызван для негодного id")
	}
}

// printIDs читает list_print_profiles: id профилей по порядку.
func printIDs(t *testing.T, call func() reply) []string {
	t.Helper()
	var out []string
	items, _ := okReply(t, call()).Data["items"].([]any)
	for _, it := range items {
		out = append(out, it.(map[string]any)["id"].(string))
	}
	return out
}

// TestPrintMaskStable: маска своей типографии не зависит от порядка списка. Типографию удалили между
// list_print_profiles и apply_print: старая маска либо указывает на тот же профиль, либо errNoProfile,
// но не на чужой. То же для масок форматов; настоящий id в масках не виден.
func TestPrintMaskStable(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	list := func() reply { return tcall(t, cs, "list_print_profiles", nil) }
	before := printIDs(t, list)
	if len(before) != 3 || before[0] != "b1" {
		t.Fatalf("список: %v", before)
	}
	libMask, ownMask := before[1], before[2]
	if libMask != ownPrintID("ZQ24prof") || ownMask != ownPrintID("own1") || libMask == ownMask ||
		strings.Contains(libMask+ownMask, "ZQ24") || strings.Contains(ownMask, "own1") {
		t.Fatalf("маски %q и %q", libMask, ownMask)
	}
	// Удалили первую не встроенную (профиль набора ZQ24prof): own1 «сдвинулся» с места 2 на место 1.
	r.fx.mu.Lock()
	r.fx.prints = slices.DeleteFunc(r.fx.prints, func(p printprof.Info) bool { return p.ID == "ZQ24prof" })
	r.fx.mu.Unlock()
	if after := printIDs(t, list); !slices.Equal(after, []string{"b1", ownMask}) {
		t.Fatalf("после удаления: %v (маска own1 должна остаться %q)", after, ownMask)
	}
	okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": ownMask})))
	if g := r.fx.edit.got34().lastPrint; g.Profile != "own1" {
		t.Fatalf("старая маска own1 указала на %q", g.Profile)
	}
	n := r.fx.edit.nPrint.Load()
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": libMask})), errNoProfile) // удалённая — не чужая
	if r.fx.edit.nPrint.Load() != n {
		t.Fatal("Host.Print вызван по маске удалённой типографии")
	}
	// Формат: маска от id профиля и имени, а не «Формат N».
	items := okReply(t, list()).Data["items"].([]any)
	fm := items[1].(map[string]any)["formats"].([]any)[0].(map[string]any)["name"].(string)
	if fm != ownFormatName("own1", "ZQ18 формат") || strings.Contains(fm, "ZQ18") || !strings.HasPrefix(fm, "Формат ") {
		t.Fatalf("маска формата %q", fm)
	}
}

// TestSpreadFamilyNoHost: Host без Families (старая сборка) — поле family не выходит, листы читаются.
func TestSpreadFamilyNoHost(t *testing.T) {
	r := newRig(t, Options{}, "")
	setLevel(r.cfg, 1)
	h := r.fx.host(r.cfg, r.sock, func() {})
	h.Families = nil
	r.svc = New(h, Options{})
	t.Cleanup(r.svc.Close)
	r.start(t)
	cs := client(t, r.sock, "Claude Code")
	items, _ := okReply(t, tcall(t, cs, "list_spreads", nil)).Data["items"].([]any)
	if len(items) == 0 {
		t.Fatal("листов нет")
	}
	for _, it := range items {
		if _, has := it.(map[string]any)["family"]; has {
			t.Fatalf("family без Host.Families: %v", it)
		}
	}
	if sp := okReply(t, tcall(t, cs, "get_spread", map[string]any{"spread": 1})); sp.Data["family"] != nil {
		t.Fatalf("get_spread: %s", sp.Raw)
	}
}
