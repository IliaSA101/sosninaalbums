package agents

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestExportAllowStillAsks: «Всегда» у «только с вопросом» (правка config.json мимо PermCan) службой
// читается как «Спрашивать», тихой ветки нет; «Спрашивать» и «Запретить» проходят как есть.
// Ход export_album с обработчиком — export_tools_test.go.
func TestExportAllowStillAsks(t *testing.T) {
	r := newRig(t, Options{}, "")
	for _, name := range []string{"export_album", "open_album"} {
		sp := toolSpec{name: name, perm: prefs.ClassAskOnly}
		for _, c := range []struct{ set, want string }{
			{prefs.PermAllow, prefs.PermAsk},
			{prefs.PermAsk, prefs.PermAsk},
			{prefs.PermDeny, prefs.PermDeny},
			{"мусор", prefs.PermDeny},
		} {
			r.fx.edit.setPerm(name, c.set)
			if mode, _, _ := r.svc.permLevel(sp); mode != c.want {
				t.Errorf("%s: %q -> %q, ждали %q", name, c.set, mode, c.want)
			}
		}
	}
	// Для сравнения: у чтения «Спрашивать» по-прежнему «Запретить».
	r.fx.edit.setPerm("get_album", prefs.PermAsk)
	if mode, _, _ := r.svc.permLevel(toolSpec{name: "get_album", perm: prefs.ClassRead}); mode != prefs.PermDeny {
		t.Errorf("чтение с ask: %q", mode)
	}
}

// TestExportAllowStillAsksHandler: с «Всегда» в config.json обработчик export_album и open_album всё равно
// задаёт вопрос (Host.Ask вызван, AskReq.Always нет) — тихой ветки у класса «только с вопросом» нет.
func TestExportAllowStillAsksHandler(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.ask = answerFixed(AskRes{Why: WhyAnswered, Album: AlbumRef{Key: "k", Rev: 1}, Files: 3})
	})
	for _, name := range []string{"export_album", "open_album"} {
		r.fx.edit.setPerm(name, prefs.PermAllow)
		before := r.fx.edit.nAsk.Load()
		args := exportArgs(r, nil)
		if name == "open_album" {
			args = map[string]any{}
		}
		tcall(t, cs, name, args)
		if got := r.fx.edit.nAsk.Load() - before; got != 1 {
			t.Errorf("%s с allow: вопросов %d, ждали 1", name, got)
		}
		if q := r.fx.edit.gotAsk(); q.Always {
			t.Errorf("%s с allow: AskReq.Always = true, тихая ветка открыта", name)
		}
		r.svc.ed.mu.Lock()
		r.svc.ed.strikes = 0
		r.svc.ed.mu.Unlock()
	}
}
