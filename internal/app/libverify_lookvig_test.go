package app

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Шаг-виньетка в library-verify (164 G1): надписи — провал на любом наборе; style и htext — только при
// minApp не ниже 0.36.0. Провалы независимы и собираются за один прогон.

// lvVigStep — шаг-виньетка split-g с оформлением из vig (nil — как в 0.35).
func lvVigStep(bg string, vig *model.LookVig) model.LookStep {
	st := lvStep(2, 2, bg)
	st.Role, st.Vig = model.RoleVignette, vig
	return st
}

func TestLibraryVerifyLookVig(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, _ := lvGood(t)
	base := &model.LookVig{Tpl: "split-g", Teachers: 6, Ratio: "3:4"}
	styled := *base
	styled.Style, styled.HText = &model.VigStyle{TText: "{Фамилия}"}, "Наши воспитатели"
	onlyHead := *base
	onlyHead.HText = "Наш класс"
	text := model.TplText{Anchor: model.Anchor{AY: 1, DY: -0.1}, W: 0.5, Text: "Надпись", Style: model.PlainStyle()}
	withText, withDraft := lvVigStep(bg, base), lvVigStep(bg, base)
	withText.Tpl.Texts = []model.TplText{text}
	withDraft.Tpl.Drafts = []model.TplText{text}
	set := func(minApp string, steps ...model.LookStep) string {
		t.Helper()
		return patchManifest(t, packZip(t, "kids-layouts", 1, files[0], lvLook(t, "v", nil, append([]model.LookStep{lvStep(2, 2, bg)}, steps...)...)), map[string]any{"minApp": minApp})
	}
	run := func(minApp string, steps ...model.LookStep) (string, int) {
		t.Helper()
		return g.run(t, "-app", "0.36.0", set(minApp, steps...))
	}

	// Старый шаг без полей — как раньше, на minApp 0.28.0.
	if out, code := run("0.28.0", lvVigStep(bg, base)); code != 0 {
		t.Errorf("шаг без новых полей: код %d\n%s", code, out)
	}
	// style и htext при minApp ниже 0.36.0 — провал (оба шага названы); с 0.36.0 — годно.
	out, code := run("0.28.0", lvVigStep(bg, &styled), lvVigStep(bg, &onlyHead))
	if code != 1 || !strings.Contains(out, "шаг 2: шаг-виньетка со style или htext, а minApp 0.28.0 ниже 0.36.0") || !strings.Contains(out, "шаг 3: шаг-виньетка со style или htext") {
		t.Errorf("minApp ниже 0.36.0: код %d\n%s", code, out)
	}
	if out, code = run("0.36.0", lvVigStep(bg, &styled), lvVigStep(bg, &onlyHead)); code != 0 {
		t.Errorf("minApp 0.36.0: код %d\n%s", code, out)
	}
	// Надписи у шага-виньетки — провал на любом наборе (и на 0.36.0, и на 0.28.0); Texts и Drafts названы отдельно.
	for _, minApp := range []string{"0.28.0", "0.36.0"} {
		out, code = run(minApp, withText, withDraft, lvVigStep(bg, base))
		if code != 1 || !strings.Contains(out, "шаг 2: у шага-виньетки надписей 1 (texts 1, drafts 0)") || !strings.Contains(out, "шаг 3: у шага-виньетки надписей 1 (texts 0, drafts 1)") {
			t.Errorf("надписи при minApp %s: код %d\n%s", minApp, code, out)
		}
		if strings.Contains(out, "шаг 4: у шага-виньетки") {
			t.Errorf("годный шаг назван провалом\n%s", out)
		}
	}
}
