package agents

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Открытие альбома по просьбе агента (152 Т4): схема, ключ, name_hint, итоги вопроса.
// Вопрос, окно ОС и само открытие — у фасада (internal/app/agentopen_test.go).

func recents(names ...string) []prefs.RecentProject {
	var out []prefs.RecentProject
	for _, n := range names {
		out = append(out, prefs.RecentProject{Path: filepath.Join("/albums", n+".salbum"), Name: n, Exists: true})
	}
	return out
}

func nameOf(r *prefs.RecentProject) string {
	if r == nil {
		return ""
	}
	return r.Name
}

// TestOpenNameHint: основа слова подсказки ≥ 5 знаков, окончание срезается, слова — начала слов имени;
// несколько совпадений, ноль, чужое имя, пустая подсказка — «не нашли».
func TestOpenNameHint(t *testing.T) {
	list := recents("Свадьба Ивановы", "Орленко", "Белов 2026", "Ёлкины", "Крещение Смирновых")
	for _, c := range []struct{ hint, want string }{
		{"Ивановых", "Свадьба Ивановы"},
		{"ивановых", "Свадьба Ивановы"},
		{"Иванов", "Свадьба Ивановы"},
		{"Орлов", ""}, // «орл» совпал бы с «Орленко»
		{"Белых", ""}, // «бел» совпал бы с «Белов»
		{"Орленко", "Орленко"},
		{"Елкины", "Ёлкины"}, // ё = е
		{"свадьба, Ивановых!", "Свадьба Ивановы"},
		{"свадьба", "Свадьба Ивановы"},
		{"Смирнов", "Крещение Смирновых"},
		{"Свадьба Петровых", ""},
		{"", ""},
		{"!!!", ""},
	} {
		if got := nameOf(recentByName(list, c.hint)); got != c.want {
			t.Errorf("%q: нашли %q, ждали %q", c.hint, got, c.want)
		}
	}
	two := recents("Иванов А", "Иванов Б")
	if got := recentByName(two, "Иванов"); got != nil {
		t.Errorf("два Ивановых: нашли %q", got.Name)
	}
	gone := recents("Иванов А")
	gone[0].Exists = false
	if got := recentByName(gone, "Иванов"); got != nil {
		t.Error("файл не на месте найден по имени")
	}
	// Составная форма: «й» как «и» + знак краткой в подсказке сравнивается с цельной в имени.
	if got := recentByName(recents("Крайний"), "Кра\u0438\u0306ний"); got == nil {
		t.Error("NFD не приведён к NFC")
	}
}

// answerFixed — Host.Ask, который сразу заканчивается итогом res.
func answerFixed(res AskRes) func(context.Context, AskReq) (AskRes, error) {
	return func(context.Context, AskReq) (AskRes, error) { return res, nil }
}

// TestOpenAlbumNoPathInput: поля path нет (схема отвергает), album и name_hint вместе, name_hint длиннее
// 60 знаков и с двунаправленными знаками — отказ до вопроса.
func TestOpenAlbumNoPathInput(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	for _, args := range []map[string]any{
		{"path": "/home/u/Альбом.salbum"},
		{"album": "k", "path": "/x"},
		{"album": "k", "name_hint": "Иванов"},
		{"name_hint": strings.Repeat("я", 61)},
		{"name_hint": "Иванов\u202e"},
		{"album": ""},
		{"album": 5},
	} {
		if res := tcall(t, cs, "open_album", args); !res.IsError {
			t.Errorf("%v: принято: %v", args, res.Data)
		}
	}
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Errorf("вопросов %d", n)
	}
}

// TestOpenAlbumUnknownKey: ключ не из недавних — «нет такого», файл не на месте — свой отказ; вопроса нет.
func TestOpenAlbumUnknownKey(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	failText(t, tcall(t, cs, "open_album", map[string]any{"album": "ffffffffffff"}), errNoRecent)
	failText(t, tcall(t, cs, "open_album", map[string]any{"album": RecentKey(r.fx.recent[1].Path)}), errOpenGone)
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Errorf("вопросов %d", n)
	}
}

// TestOpenAlbumSameKey: ключ уже открытого альбома — opened: false без вопроса. Подсказка того же альбома вопрос
// задаёт всегда: короткий ответ выдал бы агенту итог поиска по именам (оракул, 0.35-152-25).
func TestOpenAlbumSameKey(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.ask = answerFixed(AskRes{Why: WhyAnswered, Album: AlbumRef{Key: "new", Rev: 3}})
	})
	r.fx.mu.Lock()
	r.fx.view.Path = r.fx.recent[0].Path
	r.fx.mu.Unlock()
	res := okReply(t, tcall(t, cs, "open_album", map[string]any{"album": RecentKey(r.fx.recent[0].Path)}))
	if res.Data["opened"] != false || res.Data["album"] == nil {
		t.Errorf("по ключу: %v", res.Data)
	}
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Errorf("по ключу вопросов %d", n)
	}
	for _, hint := range []string{"ZQ5_wedding", "Петровых"} { // нашлась (она же открыта) и не нашлась — одинаково
		res = okReply(t, tcall(t, cs, "open_album", map[string]any{"name_hint": hint}))
		if res.Data["opened"] != true {
			t.Errorf("name_hint %q: %v", hint, res.Data)
		}
	}
	if n := r.fx.edit.nAsk.Load(); n != 2 {
		t.Errorf("по name_hint вопросов %d, ждали 2", n)
	}
}

// TestOpenAlbumAsk: ключ → вопрос с ключом, без подсказки; подсказка нашла → ключ и подсказка; не нашла →
// вопрос без обоих (итог поиска агенту не виден: ответ один). Итог — ключ и ревизия нового альбома, без папки и ленты.
func TestOpenAlbumAsk(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.ask = answerFixed(AskRes{Why: WhyAnswered, Album: AlbumRef{Key: "new", Rev: 3}})
	})
	r.fx.mu.Lock()
	r.fx.view.Path = "" // открыт другой (несохранённый) альбом
	r.fx.mu.Unlock()
	key := RecentKey(r.fx.recent[0].Path)
	for _, c := range []struct {
		args map[string]any
		want AskReq
	}{
		{map[string]any{"album": key}, AskReq{Key: key}},
		{map[string]any{"name_hint": "ZQ5"}, AskReq{Key: key, Hint: "ZQ5"}},
		{map[string]any{"name_hint": "Петровых"}, AskReq{}},
		{nil, AskReq{}},
	} {
		res := okReply(t, tcall(t, cs, "open_album", c.args))
		if res.Data["opened"] != true || res.Data["folder"] != nil || res.Data["scan"] != nil {
			t.Errorf("%v: итог %v", c.args, res.Data)
		}
		if a := res.Data["album"].(map[string]any); a["key"] != "new" || a["rev"].(float64) != 3 {
			t.Errorf("%v: альбом в итоге: %v", c.args, a)
		}
		got := r.fx.edit.gotAsk()
		if got.Kind != AskOpen || got.Key != c.want.Key || got.Hint != c.want.Hint || got.Client != "Claude Code" {
			t.Errorf("%v: Host.Ask %+v, ждали %+v", c.args, got, c.want)
		}
	}
}

// TestOpenAlbumPending: человек не успел — pending, await_answer отдаёт тот же итог opened: true.
func TestOpenAlbumPending(t *testing.T) {
	answer := make(chan AskRes)
	o := askOpts(newClock())
	o.AskWait = 100 * time.Millisecond
	_, cs := editRig(t, o, func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			select {
			case res := <-answer:
				return res, nil
			case <-ctx.Done():
				return AskRes{Why: WhyGone}, nil
			}
		}
	})
	p := okReply(t, tcall(t, cs, "open_album", nil)).Data["pending"].(map[string]any)
	next := bgCall(cs, "await_answer", map[string]any{"ask": p["ask"]})
	answer <- AskRes{Why: WhyAnswered, Album: AlbumRef{Key: "n", Rev: 1}}
	res := within(t, next, "итог по жетону")
	if res.IsError || res.Data["opened"] != true || res.Data["folder"] != nil {
		t.Fatalf("итог: %q %v", res.Text, res.Data)
	}
}

// TestOpenAlbumDeniedAndStopped: «Запретить» не доходит до Host; «Отмена» — счёт отказов (пауза), остановка
// после «да» (отмена окна ОС) — без счёта; undo после открытия — «не правка агента».
func TestOpenAlbumDeniedAndStopped(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), nil)
	r.fx.edit.setPerm("open_album", prefs.PermDeny)
	failText(t, tcall(t, cs, "open_album", nil), errDenied)
	if n := r.fx.edit.nAsk.Load(); n != 0 {
		t.Fatalf("запрет дошёл до Host: %d", n)
	}
	r.fx.edit.setPerm("open_album", prefs.PermAllow) // «Всегда» читается как «Спрашивать»

	r.fx.edit.ask = answerFixed(AskRes{Why: WhyStopped})
	failText(t, tcall(t, cs, "open_album", nil), errStopped)
	failText(t, tcall(t, cs, "open_album", nil), errStopped) // паузы нет: остановка не отказ человека

	r.fx.edit.ask = answerFixed(AskRes{Why: WhyDeclined})
	failText(t, tcall(t, cs, "open_album", nil), errDeclined)
	failText(t, tcall(t, cs, "open_album", nil), errAskCooldown)

	clk.Add(askCooldown + time.Second)
	r.fx.edit.ask = answerFixed(AskRes{Why: WhyAnswered, Album: r.fx.ref()})
	okReply(t, tcall(t, cs, "open_album", nil))
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errUndoNotOwn)
}

// TestAskClosesWithConnection: подключение закрылось, пока вопрос выгрузки или открытия висит, — контекст
// Host.Ask отменён (фасад закроет окно итогом «gone», как у create_album/add_folder, 149).
func TestAskClosesWithConnection(t *testing.T) {
	for _, tc := range []struct {
		tool string
		args func(r *rig) map[string]any
	}{
		{"create_album", func(*rig) map[string]any { return map[string]any{} }},
		{"export_album", func(r *rig) map[string]any { return map[string]any{"album": r.fx.refArgs()} }},
		{"open_album", func(*rig) map[string]any { return map[string]any{"name_hint": "Петровы"} }},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			started, gone := make(chan struct{}, 1), make(chan struct{})
			o := askOpts(newClock())
			o.AskWait = 20 * time.Millisecond
			r, cs := editRig(t, o, func(e *editFake) {
				e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
					started <- struct{}{}
					<-ctx.Done()
					close(gone)
					return AskRes{Why: WhyGone}, nil
				}
			})
			okReply(t, tcall(t, cs, tc.tool, tc.args(r))) // pending: человек не ответил
			<-started
			_ = cs.Close()
			select {
			case <-gone:
			case <-time.After(testkit.Limit(10 * time.Second)):
				t.Fatal("вопрос остался после закрытия подключения")
			}
		})
	}
}
