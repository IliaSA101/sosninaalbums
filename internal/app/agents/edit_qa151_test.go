package agents

import (
	"strings"
	"testing"
)

// QA 151: undo_last_edit из подключения без своих правок не говорит «альбом менялся» (запись об
// отмене привязана к соединению) — честно: правок в этом подключении не было; «менялся» — только
// когда своя правка есть и ревизия ушла.
func TestUndoTextNoOwnVsChanged(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	a := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"].(map[string]any)
	fresh := client(t, r.sock, "новое")
	want := "в этом подключении агент ещё не правил альбом — отменить прежние правки может человек: Ctrl+Z"
	for range 2 { // повтор не меняет ответ и не заводит запись
		if rep := tcall(t, fresh, "undo_last_edit", map[string]any{"album": a}); !rep.IsError || rep.Text != want {
			t.Fatalf("новое подключение: %q", rep.Text)
		}
	}
	if r.fx.edit.nUndo.Load() != 0 {
		t.Fatal("Host.Undo вызван без своей правки")
	}
	r.fx.bump() // человек правил после агента: у автора правки ответ прежний
	if rep := tcall(t, cs, "undo_last_edit", map[string]any{"album": a}); !rep.IsError ||
		rep.Text != "после вашей правки альбом менялся — попросите человека нажать Ctrl+Z" {
		t.Fatalf("своя правка, rev сдвинулся: %q", rep.Text)
	}
	// «Менялся» сняло запись, но подключение уже правило: это не «ещё не правил».
	if rep := tcall(t, cs, "undo_last_edit", map[string]any{"album": a}); !rep.IsError || rep.Text == want {
		t.Fatalf("после снятой записи подключение названо неправившим: %q", rep.Text)
	}
}

// QA 151: «cover» там, где обложка не цель, отвечает своим текстом по смыслу инструмента, а не
// «нет такого разворота — … «cover» …»; неизвестный номер — прежний текст.
func TestCoverNotTargetTexts(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	for _, spread := range []string{"cover", "Cover", " COVER "} { // регистр и пробелы, как у findSpread
		failText(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": spread, "item": "sa:star"}}})), errCoverNotTarget("add_sticker"))
	}
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"add_sticker", map[string]any{"stickers": []any{map[string]any{"spread": "cover", "item": "sa:star"}}},
			"на обложку стикеры не ставятся — номер разворота с 1 или id из list_spreads"},
		{"set_background", map[string]any{"item": "sa:plain", "spreads": []any{"cover"}},
			"на обложку фон этим инструментом не ставится — номер разворота с 1 или id из list_spreads"},
		{"vary_layouts", map[string]any{"spreads": []any{"cover"}},
			"у обложки нет раскладки для перебора — номер разворота с 1 или id из list_spreads"},
	}
	for _, c := range cases {
		rep := tcall(t, cs, c.tool, args34(r, c.args))
		if !rep.IsError || rep.Text != c.want {
			t.Errorf("%s: %q, ждали %q", c.tool, rep.Text, c.want)
		}
		if strings.Contains(rep.Text, "«cover»") {
			t.Errorf("%s: текст снова предлагает «cover»: %q", c.tool, rep.Text)
		}
	}
}

// QA 151: неизвестный номер там, где обложка не цель, — без «cover»; где допустима (get_spread,
// show_spread) — прежний текст с «cover».
func TestNoSpreadTexts(t *testing.T) {
	const flat = "нет такого разворота — номер с 1 или id из list_spreads"
	const withCover = "нет такого разворота — номер с 1, «cover» (обложка) или id из list_spreads"
	r, cs := editRig(t, Options{}, nil)
	for _, ref := range []any{99, "nope"} {
		cases := []struct {
			tool string
			args map[string]any
			want string
		}{
			{"add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": ref, "item": "sa:star"}}}), flat},
			{"set_background", args34(r, map[string]any{"item": "sa:plain", "spreads": []any{ref}}), flat},
			{"vary_layouts", args34(r, map[string]any{"spreads": []any{ref}}), flat},
			{"show_spread", map[string]any{"album": map[string]any{"key": r.fx.refArgs()["key"]}, "spread": ref}, withCover},
			{"get_spread", map[string]any{"spread": ref}, withCover},
		}
		for _, c := range cases {
			rep := tcall(t, cs, c.tool, c.args)
			if !rep.IsError || rep.Text != c.want {
				t.Errorf("%s %v: %q, ждали %q", c.tool, ref, rep.Text, c.want)
			}
		}
	}
}
