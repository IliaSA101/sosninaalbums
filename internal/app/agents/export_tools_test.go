package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Выгрузка по просьбе агента (152 Т3): схема, отказы, итоги вопроса, ход в get_album.
// Вопрос, папка и сама выгрузка — у фасада (internal/app/agentexport_test.go).

func exportArgs(r *rig, extra map[string]any) map[string]any {
	a := map[string]any{"album": r.fx.refArgs()}
	for k, v := range extra {
		a[k] = v
	}
	return a
}

// TestExportSchema: числа вне перечня и недопустимые знаки — отказ до вопроса; допустимое доходит до Host.Ask
// со своими полями; print: plain и id типографии из list_print_profiles; чужой id — «нет такой типографии».
func TestExportSchema(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.ask = answerFixed(AskRes{Why: WhyAnswered, Album: AlbumRef{Key: "k", Rev: 1}, Files: 25})
	})
	for _, bad := range []map[string]any{
		{"quality": 90}, {"quality": 0}, {"dpi": 71}, {"dpi": 1201}, {"range": strings.Repeat("1", 257)},
		{"range": "1\u202e"}, {"folder_hint": strings.Repeat("я", 2049)}, {"path": "/x"}, {"folder": "/x"},
		{"print": strings.Repeat("a", 200)}, {"cover": "да"},
	} {
		if res := tcall(t, cs, "export_album", exportArgs(r, bad)); !res.IsError {
			t.Errorf("%v: принято: %v", bad, res.Data)
		}
	}
	if res := tcall(t, cs, "export_album", exportArgs(r, map[string]any{"print": "no-such"})); !res.IsError || res.Text != errNoProfile.Error() {
		t.Errorf("чужая типография: %q", res.Text)
	}
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Fatalf("отказ схемы дошёл до вопроса: %d", n)
	}
	res := okReply(t, tcall(t, cs, "export_album", exportArgs(r, map[string]any{"range": " 3-5, 8 ", "pages": true, "cover": true,
		"cover_only": false, "quality": 80, "dpi": 600, "print": "plain", "folder_hint": "Петровы"})))
	if res.Data["started"] != true || res.Data["files"].(float64) != 25 || res.Data["album"] == nil || res.Data["opened"] != nil {
		t.Fatalf("итог: %v", res.Data)
	}
	got := r.fx.edit.gotAsk()
	if got.Kind != AskExport || got.Range != "3-5, 8" || !got.Pages || !got.Cover || got.CoverOnly || got.Quality != 80 || got.DPI != 600 ||
		got.Print != "plain" || got.Hint != "Петровы" || got.Key != r.fx.ref().Key || got.Client != "Claude Code" {
		t.Errorf("Host.Ask: %+v", got)
	}
	okReply(t, tcall(t, cs, "export_album", exportArgs(r, map[string]any{"print": "b1"})))
	if p := r.fx.edit.gotAsk().Print; p != "b1" {
		t.Errorf("print b1 -> %q", p)
	}
}

// TestExportDenied: «Запретить» не доходит до Host; ревизия и ключ сверяются до вопроса.
func TestExportDenied(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	r.fx.edit.setPerm("export_album", prefs.PermDeny)
	failText(t, tcall(t, cs, "export_album", exportArgs(r, nil)), errDenied)
	r.fx.edit.setPerm("export_album", prefs.PermAsk)
	stale := exportArgs(r, nil)
	stale["album"] = map[string]any{"key": r.fx.ref().Key, "rev": r.fx.ref().Rev - 1}
	failText(t, tcall(t, cs, "export_album", stale), errAlbumEdited)
	other := exportArgs(r, nil)
	other["album"] = map[string]any{"key": "ffff", "rev": r.fx.ref().Rev}
	failText(t, tcall(t, cs, "export_album", other), errAlbumChanged)
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Errorf("вопросов %d", n)
	}
}

// TestExportPrecheckNoWindow: отказ проверок выгрузки (Host вернул ExportFail) — текст окна «Экспорт» агенту,
// без счёта отказов: следующий вызов не упирается в паузу.
func TestExportPrecheckNoWindow(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
		e.ask = func(context.Context, AskReq) (AskRes, error) {
			return AskRes{}, ExportFail("в альбоме нечего экспортировать")
		}
	})
	for range 3 {
		res := tcall(t, cs, "export_album", exportArgs(r, nil))
		if !res.IsError || res.Text != "в альбоме нечего экспортировать" {
			t.Fatalf("ответ: %v %q", res.IsError, res.Text)
		}
	}
}

// TestExportLaterDeclineNoStrike: «Отмена» первого вопроса — счёт отказов (пауза); остановка после «да»
// (предупреждение, окно ОС) — «человек остановил» без счёта и без паузы.
func TestExportLaterDeclineNoStrike(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), nil)
	r.fx.edit.ask = answerFixed(AskRes{Why: WhyStopped})
	for range 3 {
		failText(t, tcall(t, cs, "export_album", exportArgs(r, nil)), errStopped)
	}
	r.fx.edit.ask = answerFixed(AskRes{Why: WhyDeclined})
	failText(t, tcall(t, cs, "export_album", exportArgs(r, nil)), errDeclined)
	failText(t, tcall(t, cs, "export_album", exportArgs(r, nil)), errAskCooldown)
	clk.Add(askCooldown + time.Second)
	r.fx.edit.ask = answerFixed(AskRes{Why: WhyAnswered, Album: r.fx.ref(), Files: 3})
	okReply(t, tcall(t, cs, "export_album", exportArgs(r, nil)))
	// Выгрузка не меняет альбом: undo не находит правок агента: записи отмены выгрузка не оставляет.
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errUndoNoOwn)
}

// TestExportStateInGetAlbum: get_album.export — ход выгрузки этого альбома; у другого альбома — idle;
// без выгрузок — idle.
func TestExportStateInGetAlbum(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	st := func() map[string]any {
		return okReply(t, tcall(t, cs, "get_album", nil)).Data["export"].(map[string]any)
	}
	if s := st(); s["state"] != "idle" || s["done"].(float64) != 0 {
		t.Fatalf("до выгрузки: %v", s)
	}
	r.fx.edit.mu.Lock()
	r.fx.edit.export = ExportStatus{State: "running", Done: 4, Total: 25}
	r.fx.edit.mu.Unlock()
	if s := st(); s["state"] != "running" || s["done"].(float64) != 4 || s["total"].(float64) != 25 {
		t.Errorf("идёт: %v", s)
	}
	r.fx.edit.mu.Lock()
	r.fx.edit.exportEd = 999 // ход принадлежит другому альбому
	r.fx.edit.mu.Unlock()
	if s := st(); s["state"] != "idle" {
		t.Errorf("чужой альбом: %v", s)
	}
}
