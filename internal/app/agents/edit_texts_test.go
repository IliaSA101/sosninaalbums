package agents

import (
	"strings"
	"testing"
)

// Неизвестное семейство раскладок: ошибка называет допустимые значения, а не общую фразу.
func TestVaryUnknownFamilyNamesValues(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), nil)
	rep := tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}, "families": []string{"dense", "wide"}})
	if !rep.IsError || strings.Contains(rep.Text, "сверьтесь с inputSchema") {
		t.Fatalf("общая фраза вместо значений: %q", rep.Text)
	}
	for _, f := range families {
		if !strings.Contains(rep.Text, f) {
			t.Errorf("в тексте нет %q: %q", f, rep.Text)
		}
	}
	if !strings.Contains(rep.Text, "элемент 2") {
		t.Errorf("не назван номер элемента: %q", rep.Text)
	}
}

// Третья ступень тишины говорит про кнопку и перезапуск, первые две — про «сами через минуты».
func TestAskMuteTexts(t *testing.T) {
	if !strings.Contains(errAskOff.Error(), "Разрешить вопросы") || !strings.Contains(errAskOff.Error(), "перезапуска") ||
		strings.Contains(errAskOff.Error(), "сами") {
		t.Errorf("третья ступень: %q", errAskOff.Error())
	}
	if !strings.Contains(errAskMuted.Error(), "сами через несколько минут") {
		t.Errorf("вторая ступень изменилась: %q", errAskMuted.Error())
	}
}

// Пометки журнала: лимит правок — «лимит», а не «занят»; await_answer не помечен как правка.
func TestJournalLimitNoteAndAwaitNotEdit(t *testing.T) {
	if got := noteOf(toolSpec{edit: true}, errEditRate); got != "лимит" {
		t.Errorf("лимит: %q", got)
	}
	for _, e := range []*agentErr{errEditBusy, errUserBusy, errAskBusy} {
		if got := noteOf(toolSpec{edit: true}, e); got != "занят" {
			t.Errorf("%v: %q", e, got)
		}
	}
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyAnswered) })
	_ = tcall(t, cs, "await_answer", map[string]any{"ask": strings.Repeat("0", 32)}) // жетона нет: отказ, но вызов записан
	waitFor(t, "запись в журнале", func() bool { c, _ := r.svc.Calls(); return len(c) > 0 })
	calls, _ := r.svc.Calls()
	for _, c := range calls {
		if c.Tool == "await_answer" && c.Edit {
			t.Error("await_answer помечен как правка")
		}
	}
}
