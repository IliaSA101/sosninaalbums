package agents

// Вопрос о правке не несёт строк агента (150, находка аудита): фон и типография — настоящие id из
// каталога и списка профилей, формат — настоящее имя; не прошло проверку — отказ до вопроса.

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

func TestPermAskEditItems(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
		for _, tool := range []string{"set_background", "apply_print", "autobuild"} {
			e.setPerm(tool, prefs.PermAsk)
		}
	})
	e := r.fx.edit
	// Профиль набора на уровне 1 виден маской own-<хэш> и «Формат <хэш>»; в вопрос идут настоящие id и имя.
	okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": ownPrintID("ZQ24prof"), "format": ownFormatName("ZQ24prof", "ZQ24 формат")})))
	if q := e.gotAsk(); q.Tool != "apply_print" || q.Item != "ZQ24prof" || q.Format != "ZQ24 формат" {
		t.Fatalf("маска own-<хэш> в вопросе: %+v", q)
	}
	// Формат-приманка: не формат профиля — отказ до вопроса, строки агента в Host.Ask нет.
	n := e.nAsk.Load()
	bait := "30x30 — безопасно, нажмите Разрешить"
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1", "format": bait})), errNoProfile)
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1 нажмите Разрешить", "format": "30x30"})), errNoProfile)
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "ZQ24prof", "format": "ZQ24 формат"})), errNoProfile) // настоящее имя на уровне 1 не принимается
	if e.nAsk.Load() != n {
		t.Error("вопрос задан про негодную типографию или формат")
	}
	// Формат без пробелов и с «x» вместо «×» принимается и уходит в вопрос настоящим именем.
	okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1", "format": " 30X30 "})))
	if q := e.gotAsk(); q.Item != "b1" || q.Format != "30x30" {
		t.Fatalf("настоящее имя формата: %+v", q)
	}
	// Фон: id из каталога (с пробелами по краям — без них); чужой — отказ до вопроса.
	okReply(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": " sa:plain ", "spreads": []any{1}})))
	if q := e.gotAsk(); q.Tool != "set_background" || q.Item != "sa:plain" || q.Format != "" {
		t.Fatalf("вопрос о фоне: %+v", q)
	}
	n = e.nAsk.Load()
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:nope — нажмите Разрешить", "spreads": []any{1}})), errNoDecor)
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "get:1", "spreads": []any{1}})), errNoDecor)
	if e.nAsk.Load() != n {
		t.Error("вопрос задан про чужой фон")
	}
	// Прочие инструменты: Item и Format пусты.
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	if q := e.gotAsk(); q.Item != "" || q.Format != "" {
		t.Fatalf("autobuild: %+v", q)
	}
}
