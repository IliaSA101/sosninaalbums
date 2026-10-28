package grid_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Заготовки и шаг макета (131 G2) в копиях: «Копировать оформление», «Копировать раскладку»
// с оформлением, дубль листа, буфер (Ctrl+C/V, Ctrl+D).

const lookKey = "my:0123456789abcdef"

// lookEd — s1: места a, b, цвет левой страницы, заготовка «[Имена]» и обычная надпись, шаг макета 2
// (альбом помнит макет). s2 — «Авто» с фото d, s3 — пустой.
func lookEd(t *testing.T) (e *album.Editor, draftID, plainID string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 127, 127, "b")}, []string{"d"}, []string{})
	e.SetBackground("s1", model.ScopeLeft, "#aabbcc")
	draftID, plainID = edtest.AddText(t, e, "s1", 0), edtest.AddText(t, e, "s1", 1)
	edtest.Change(e, func(p *model.Project) {
		sp, i := p.FindText(draftID)
		sp.Texts[i].Text, sp.Texts[i].Draft = "[Имена]", true
		sp, i = p.FindText(plainID)
		sp.Texts[i].Text = "Привет"
		p.Look = &model.LookRef{Key: lookKey, Name: "Свадьба"}
		p.Spreads[0].LookStep = 2
	})
	return e, draftID, plainID
}

// drafts — число заготовок и обычных надписей листа.
func drafts(s *model.Spread) (draft, plain int) {
	for _, tb := range s.Texts {
		if tb.Draft {
			draft++
		} else {
			plain++
		}
	}
	return draft, plain
}

func stepsOf(p *model.Project) []int {
	var out []int
	for _, s := range p.Spreads {
		out = append(out, s.LookStep)
	}
	return out
}

func steps(e *album.Editor) []int { return stepsOf(edtest.P(e)) }

func wantSteps(t *testing.T, e *album.Editor, want ...int) {
	t.Helper()
	got := steps(e)
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("шаги листов %v, ждали %v", got, want)
		}
	}
}

// TestDuplicateKeepsLookStep — дубль с оформлением несёт шаг макета и заготовку (новые ID), без
// оформления — нет; без макета в альбоме шаг сам сходит на 0; отмена — одним шагом; после
// Save → Load шаг и признак на месте.
func TestDuplicateKeepsLookStep(t *testing.T) {
	e, draftID, _ := lookEd(t)
	undo := edtest.UndoLen(e)
	if _, err := e.DuplicateGrid("s1", 2, true); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d → %d", undo, edtest.UndoLen(e))
	}
	wantSteps(t, e, 2, 2, 2)
	for i := 1; i <= 2; i++ {
		s := edtest.P(e).Spreads[i]
		if d, pl := drafts(s); d != 1 || pl != 1 {
			t.Fatalf("копия %d: заготовок %d, обычных %d", i, d, pl)
		}
		for _, tb := range s.Texts {
			if tb.ID == draftID {
				t.Fatalf("копия %d: ID заготовки не сменился", i)
			}
		}
	}
	q := edtest.SaveLoad(t, e)
	if got := stepsOf(q); len(got) < 3 || got[1] != 2 || got[2] != 2 {
		t.Fatalf("после Save → Load шаги: %v", got)
	}
	if d, _ := drafts(q.Spreads[1]); d != 1 {
		t.Fatal("после Save → Load заготовка пропала")
	}
	e.Undo()
	if len(edtest.P(e).Spreads) != 3 {
		t.Fatalf("отмена оставила листов %d", len(edtest.P(e).Spreads))
	}
	// Без оформления: ни надписей, ни шага.
	if _, err := e.DuplicateGrid("s1", 1, false); err != nil {
		t.Fatal(err)
	}
	cp := edtest.P(e).Spreads[1]
	if cp.LookStep != 0 || len(cp.Texts) != 0 {
		t.Fatalf("копия без оформления: шаг %d, надписей %d", cp.LookStep, len(cp.Texts))
	}
	// Источник без шага — копия без шага; без макета в альбоме шаг не живёт.
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].LookStep = 0 })
	e.DuplicateGrid("s1", 1, true)
	if edtest.P(e).Spreads[1].LookStep != 0 {
		t.Fatal("копия получила шаг, которого нет у источника")
	}
	edtest.Change(e, func(p *model.Project) { p.Look, p.Spreads[0].LookStep = nil, 3 })
	if edtest.P(e).Spreads[0].LookStep != 0 {
		t.Fatal("шаг без макета остался")
	}
}

// TestDraftCopyDesign — заготовка и шаг макета едут вместе с оформлением: «Копировать оформление»
// (с надписями и без), «Копировать раскладку» с оформлением, буфер и дубль выделенного. Несколько
// целей одним шагом; страница → страница шаг не трогает; повтор — холостая; отмена — один шаг.
func TestDraftCopyDesign(t *testing.T) {
	t.Run("оформление с надписями", copyDecorTexts)
	t.Run("шаг без надписей, шаг при том же оформлении", copyDecorStep)
	t.Run("страница на страницу", copyDecorPage)
	t.Run("раскладка с оформлением", copyGridDecor)
	t.Run("буфер и дубль выделенного", copyClipDraft)
}

// lookTargets — листы s2 («Авто») и s3 (пустой) целиком.
func lookTargets() []model.GridTarget {
	return []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}, {Spread: "s3", Scope: model.ScopeSpread}}
}

// оформление с надписями.
func copyDecorTexts(t *testing.T) {
	spreads := lookTargets()
	e, _, _ := lookEd(t)
	undo := edtest.UndoLen(e)
	if _, err := e.CopyDecor("s1", model.ScopeSpread, spreads, true, false); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d → %d", undo, edtest.UndoLen(e))
	}
	wantSteps(t, e, 2, 2, 2)
	for _, i := range []int{1, 2} {
		if d, pl := drafts(edtest.P(e).Spreads[i]); d != 1 || pl != 1 {
			t.Fatalf("лист %d: заготовок %d, обычных %d", i+1, d, pl)
		}
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CopyDecor("s1", model.ScopeSpread, spreads, true, false); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "повтор копии оформления")
	q := edtest.SaveLoad(t, e)
	if g := stepsOf(q); g[1] != 2 || g[2] != 2 {
		t.Fatalf("после Save → Load шаги %v", g)
	}
	if d, _ := drafts(q.Spreads[2]); d != 1 {
		t.Fatal("после Save → Load заготовка пропала")
	}
	e.Undo()
	wantSteps(t, e, 2, 0, 0)
	if d, pl := drafts(edtest.P(e).Spreads[2]); d+pl != 0 {
		t.Fatal("отмена оставила надписи")
	}
}

// шаг без надписей, шаг при том же оформлении.
func copyDecorStep(t *testing.T) {
	spreads := lookTargets()
	e, _, _ := lookEd(t)
	if _, err := e.CopyDecor("s1", model.ScopeSpread, spreads, false, false); err != nil {
		t.Fatal(err)
	}
	wantSteps(t, e, 2, 2, 2)
	if d, pl := drafts(edtest.P(e).Spreads[2]); d+pl != 0 {
		t.Fatal("надписи без флажка поехали")
	}
	// Оформление на s3 уже то же, шаг другой: повтор всё равно ставит шаг источника.
	edtest.Change(e, func(p *model.Project) { p.Spreads[2].LookStep = 5 })
	rev := edtest.Rev(e)
	if _, err := e.CopyDecor("s1", model.ScopeSpread, spreads[1:], false, false); err != nil {
		t.Fatal(err)
	}
	wantSteps(t, e, 2, 2, 2)
	if edtest.Rev(e) != rev+1 {
		t.Fatalf("rev %d → %d: шаг не одной правкой", rev, edtest.Rev(e))
	}
}

// страница на страницу.
func copyDecorPage(t *testing.T) {
	e, _, _ := lookEd(t)
	if _, err := e.CopyDecor("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s3", Scope: model.ScopeRight}}, true, false); err != nil {
		t.Fatal(err)
	}
	wantSteps(t, e, 2, 0, 0)
	if d, pl := drafts(edtest.P(e).Spreads[2]); d != 1 || pl != 0 { // надпись «Привет» — на правой странице источника
		t.Fatalf("заготовка левой страницы: заготовок %d, обычных %d", d, pl)
	}
}

// раскладка с оформлением.
func copyGridDecor(t *testing.T) {
	spreads := lookTargets()
	e, _, _ := lookEd(t)
	undo := edtest.UndoLen(e)
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, spreads, true, false); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d → %d", undo, edtest.UndoLen(e))
	}
	wantSteps(t, e, 2, 2, 2)
	if d, pl := drafts(edtest.P(e).Spreads[1]); d != 1 || pl != 1 {
		t.Fatalf("s2: заготовок %d, обычных %d", d, pl)
	}
	edtest.MustGrid(t, e)
	e.Undo()
	wantSteps(t, e, 2, 0, 0)
	// Без оформления шаг не едет; со страницы на страницу — тоже.
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, spreads, false, false); err != nil {
		t.Fatal(err)
	}
	wantSteps(t, e, 2, 0, 0)
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s3", Scope: model.ScopeLeft}}, true, false); err != nil {
		t.Fatal(err)
	}
	wantSteps(t, e, 2, 0, 0)
}

// буфер и дубль выделенного.
func copyClipDraft(t *testing.T) {
	e, draftID, plainID := lookEd(t)
	c, ok := e.TakeClip("s1", model.ClipSel{Texts: []string{draftID, plainID}})
	if !ok {
		t.Fatal("снимок пуст")
	}
	undo := edtest.UndoLen(e)
	_, out, err := e.PasteClip(&c, model.PasteReq{Sheet: "s3"})
	if err != nil || len(out.Texts) != 2 {
		t.Fatalf("вставка: %v, надписей %d", err, len(out.Texts))
	}
	if d, pl := drafts(edtest.P(e).Spreads[2]); d != 1 || pl != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("вставка: заготовок %d, обычных %d, шагов %d → %d", d, pl, undo, edtest.UndoLen(e))
	}
	if _, out, err = e.DuplicateClip("s1", model.ClipSel{Texts: []string{draftID}}); err != nil || len(out.Texts) != 1 {
		t.Fatalf("дубль: %v, надписей %d", err, len(out.Texts))
	}
	if d, pl := drafts(edtest.P(e).Spreads[0]); d != 2 || pl != 1 {
		t.Fatalf("дубль: заготовок %d, обычных %d", d, pl)
	}
	q := edtest.SaveLoad(t, e)
	if d, _ := drafts(q.Spreads[0]); d != 2 {
		t.Fatalf("после Save → Load заготовок %d", d)
	}
}
