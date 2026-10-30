package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Правки агентом, служебная часть (149 Т2): флажок, место, темп, ключ и rev, «человек занят», копия,
// срок, отмена. Окно вопроса и сам Editor — у фасада (Т3).

// editCalls — по вызову каждого правящего инструмента с годными аргументами.
func editCalls(fx *fixture) []struct {
	name string
	args map[string]any
} {
	a := map[string]any{"album": fx.refArgs()}
	return []struct {
		name string
		args map[string]any
	}{
		{"create_album", map[string]any{}},
		{"add_folder", a},
		{"autobuild", a},
		{"vary_layouts", map[string]any{"album": fx.refArgs(), "spreads": []any{1}}},
		{"undo_last_edit", a},
		// 0.34 (151): тела — Т4, вход и ворота общие.
		{"set_marks", map[string]any{"album": fx.refArgs(), "ids": []any{idB}, "flag": "pick"}},
		{"set_cover_photo", map[string]any{"album": fx.refArgs(), "photo": idB}},
		{"set_background", map[string]any{"album": fx.refArgs(), "item": "sa:plain", "spreads": []any{1}}},
		{"add_sticker", map[string]any{"album": fx.refArgs(), "stickers": []any{map[string]any{"spread": 1, "item": "sa:star"}}}},
		{"apply_print", map[string]any{"album": fx.refArgs(), "profile": "b1"}},
	}
}

// tcallG — вызов для горутин теста: без t.Fatal.
func tcallG(cs *mcp.ClientSession, name string, args map[string]any) (reply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(15*time.Second))
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return reply{}, err
	}
	r := reply{IsError: res.IsError, Text: textOf(res)}
	if res.StructuredContent != nil {
		r.Raw, _ = json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(r.Raw, &r.Data)
	}
	return r, nil
}

// bgCall вызывает инструмент в фоне; ответ приходит в канал.
func bgCall(cs *mcp.ClientSession, name string, args map[string]any) <-chan reply {
	ch := make(chan reply, 1)
	go func() {
		r, err := tcallG(cs, name, args)
		if err != nil {
			r = reply{IsError: true, Text: "протокол: " + err.Error()}
		}
		ch <- r
	}()
	return ch
}

func within(t *testing.T, ch <-chan reply, what string) reply {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatalf("не дождались: %s", what)
		return reply{}
	}
}

// TestEditsOff: флажок выключен — каждый правящий инструмент отвечает словами, где включить, и не доходит
// ни до одного замыкания правок; включили — тот же вызов проходит.
func TestEditsOff(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) { e.allowed = false })
	for _, c := range editCalls(r.fx) {
		failText(t, tcall(t, cs, c.name, c.args), errNoEdits)
	}
	if n := r.fx.edit.writes() + r.fx.edit.nBusy.Load(); n != 0 {
		t.Fatalf("при выключенном флажке служба дошла до замыканий правок: %d", n)
	}
	ed := okReply(t, tcall(t, cs, "get_album", nil)).Data["edits"].(map[string]any)
	if ed["allowed"] != false || ed["blocked"] != "off" {
		t.Fatalf("get_album.edits: %v", ed)
	}
	r.fx.edit.setFlag(true, "")
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	if ed := tcall(t, cs, "get_album", nil).Data["edits"].(map[string]any); ed["allowed"] != true || ed["blocked"] != nil {
		t.Fatalf("get_album.edits после включения: %v", ed)
	}
}

// TestEditOffNoWindow: вопрос при выключенном флажке окна не открывает и места правки не занимает.
func TestEditOffNoWindow(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) { e.allowed = false })
	failText(t, tcall(t, cs, "create_album", map[string]any{"folder_hint": "/x"}), errNoEdits)
	failText(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}), errNoEdits)
	failText(t, tcall(t, cs, "await_answer", map[string]any{"ask": "0123"}), errAskGone)
	if r.fx.edit.nAsk.Load() != 0 {
		t.Fatal("Host.Ask вызван при выключенном флажке")
	}
	select {
	case r.svc.ed.slot <- struct{}{}:
		<-r.svc.ed.slot
	default:
		t.Fatal("место правки занято после отказа")
	}
}

// TestEditElevated: процесс от администратора правок не принимает, даже с флажком; get_album говорит почему.
func TestEditElevated(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) { e.allowed, e.blocked = true, "elevated" })
	for _, c := range editCalls(r.fx) {
		failText(t, tcall(t, cs, c.name, c.args), errElevated)
	}
	if r.fx.edit.writes() != 0 {
		t.Fatal("правка дошла до Host")
	}
	if ed := tcall(t, cs, "get_album", nil).Data["edits"].(map[string]any); ed["allowed"] != false || ed["blocked"] != "elevated" {
		t.Fatalf("get_album.edits: %v", ed)
	}
}

// TestEditSlot: правка одна за раз — пока идёт одна, вторая и «покажи» получают «занято», чтение и
// await_answer — отвечают; после конца первой место свободно.
func TestEditSlot(t *testing.T) {
	release := make(chan struct{})
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.auto = func(context.Context, AutoReq) (AutoRes, error) {
			<-release
			return AutoRes{Album: e.f.bump(), Added: 1}, nil
		}
	})
	cs2 := client(t, r.sock, "второй")
	first := bgCall(cs, "autobuild", map[string]any{"album": r.fx.refArgs()})
	waitFor(t, "автосборка дошла до Host", func() bool { return r.fx.edit.nAuto.Load() == 1 })
	failText(t, tcall(t, cs2, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}}), errEditBusy)
	failText(t, tcall(t, cs2, "show_spread", map[string]any{"album": map[string]any{"key": r.fx.ref().Key}, "spread": 1}), errEditBusy)
	okReply(t, tcall(t, cs2, "get_album", nil))
	failText(t, tcall(t, cs2, "await_answer", map[string]any{"ask": "x"}), errAskGone) // не errEditBusy и не errCallBusy
	if r.fx.edit.nVary.Load() != 0 || r.fx.edit.nShow.Load() != 0 {
		t.Fatal("отказанные правки дошли до Host")
	}
	close(release)
	if got := within(t, first, "первая правка"); got.IsError {
		t.Fatalf("первая правка: %q", got.Text)
	}
	okReply(t, tcall(t, cs2, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}}))
}

// TestEditRate: не больше Rate правок в минуту на программу; минута прошла — снова можно.
func TestEditRate(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, Options{Now: clk.Now, Rate: 3}, nil)
	for range 3 {
		okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errEditRate)
	if r.fx.edit.nAuto.Load() != 3 {
		t.Fatalf("Host.Auto вызван %d раз, ждали 3", r.fx.edit.nAuto.Load())
	}
	clk.Add(61 * time.Second)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
}

// TestEditRateCountsAll: в темп идут и вызовы, которые потом отказаны (неверный ключ, rev): агент,
// бьющийся в отказ, упирается в предел (М9).
func TestEditRateCountsAll(t *testing.T) {
	r, cs := editRig(t, Options{Now: newClock().Now, Rate: 2}, nil)
	bad := map[string]any{"album": map[string]any{"key": "e000000000000", "rev": 1}}
	failText(t, tcall(t, cs, "autobuild", bad), errAlbumChanged)
	failText(t, tcall(t, cs, "autobuild", bad), errAlbumChanged)
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errEditRate)
	if r.fx.edit.writes() != 0 {
		t.Fatal("отказанные правки дошли до Host")
	}
}

// TestEditExpectRev: ключ и rev сверяются до копии и Host; Host получает то, что назвал агент.
func TestEditExpectRev(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	ref := r.fx.ref()
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": map[string]any{"key": ref.Key, "rev": ref.Rev - 1}}), errAlbumEdited)
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": map[string]any{"key": "e1", "rev": ref.Rev}}), errAlbumChanged)
	r.fx.mu.Lock()
	r.fx.ed = 0
	r.fx.mu.Unlock()
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errNoAlbum)
	r.fx.mu.Lock()
	r.fx.ed = 7
	r.fx.mu.Unlock()
	if r.fx.edit.writes() != 0 {
		t.Fatal("Host вызван при неверных ключе или rev")
	}
	got := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"].(map[string]any)
	if got["rev"].(float64) != float64(ref.Rev+1) || got["key"] != ref.Key {
		t.Fatalf("album в ответе: %v", got)
	}
	if e := r.fx.edit; e.gotAuto().Key != ref.Key || e.gotAuto().Rev != ref.Rev || e.nBackup.Load() != 1 || e.gotBackups()[0] != ref.Key {
		t.Fatalf("Host получил %+v, копий %v", e.gotAuto(), e.gotBackups())
	}
}

// TestEditBackupFail: копия не сделана — правки нет, агенту словами; журнал помечает «копия».
func TestEditBackupFail(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.backup = func(string) error { return errors.New("/secret/path: диск полон") }
	})
	rep := tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})
	failText(t, rep, errBackup)
	if strings.Contains(rep.Text, "secret") || r.fx.edit.nAuto.Load() != 0 {
		t.Fatalf("копия не сделана, а правка шла или путь вышел: %q", rep.Text)
	}
	waitFor(t, "журнал", func() bool { c, _ := r.svc.Calls(); return len(c) > 0 })
	if c, _ := r.svc.Calls(); !c[0].Edit || c[0].Note != "копия" || c[0].OK {
		t.Fatalf("журнал: %+v", c[0])
	}
}

// TestEditUserBusy: человек занят — правка ждёт не дольше BusyWait, затем «повторите позже»; Host.Busy
// получает контекст со сроком, копия и правка не начинаются.
func TestEditUserBusy(t *testing.T) {
	var busy atomic.Bool
	busy.Store(true)
	r, cs := editRig(t, Options{BusyWait: 60 * time.Millisecond}, func(e *editFake) {
		e.busy = func(ctx context.Context) bool {
			if busy.Load() {
				<-ctx.Done()
				return true
			}
			return false
		}
	})
	start := time.Now()
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errUserBusy)
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Fatalf("ожидание не ограничено BusyWait: %v", d)
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errUserBusy) // место не застряло
	if e := r.fx.edit; e.nBackup.Load() != 0 || e.nAuto.Load() != 0 {
		t.Fatal("копия или правка пошли при занятом человеке")
	}
	busy.Store(false)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
}

// TestEditExportBeforeBusy: выгрузка идёт, и человек «занят» окном — create_album и open_album сразу отвечают
// «идёт выгрузка», не ждут BusyWait и не доходят до Busy; прочие правки и инструменты без выгрузки — как раньше.
func TestEditExportBeforeBusy(t *testing.T) {
	var hold atomic.Bool
	hold.Store(true)
	r, cs := editRig(t, Options{BusyWait: time.Hour}, func(e *editFake) {
		e.hold = func(tool string) bool { return hold.Load() && tool != "autobuild" }
		e.busy = func(ctx context.Context) bool { <-ctx.Done(); return true }
	})
	start := time.Now()
	failText(t, tcall(t, cs, "create_album", nil), errExporting)
	failText(t, tcall(t, cs, "open_album", map[string]any{"name_hint": "Петровы"}), errExporting)
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Fatalf("ждали Busy: %v", d)
	}
	if r.fx.edit.nBusy.Load() != 0 || r.fx.edit.nAsk.Load() != 0 {
		t.Fatalf("Busy %d, вопросов %d", r.fx.edit.nBusy.Load(), r.fx.edit.nAsk.Load())
	}
	failText(t, tcall(t, cs, "create_album", nil), errExporting) // место не застряло
	hold.Store(false)
	r.svc.o.BusyWait = 20 * time.Millisecond
	failText(t, tcall(t, cs, "create_album", nil), errUserBusy) // выгрузки нет — прежний ответ
}

// TestEditLateNoApply (служебная часть): срок вызова вышел — агент получает «время вышло», а Host
// видит отменённый контекст (проверка под замком Editor — у фасада); правка не записывается и
// отмены на неё нет; журнал помечает «поздно».
func TestEditLateNoApply(t *testing.T) {
	seen := make(chan error, 1)
	r, cs := editRig(t, Options{AutoTimeout: 120 * time.Millisecond}, func(e *editFake) {
		e.auto = func(ctx context.Context, _ AutoReq) (AutoRes, error) {
			<-ctx.Done()
			seen <- ctx.Err()
			return AutoRes{}, ErrLate
		}
	})
	// Тело Host кончается сразу за сроком: агент получит «время вышло» или «срок вышел — правка не применена».
	if rep := tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}); !rep.IsError || rep.Text != errTimeout.Error() && rep.Text != errLate.Error() {
		t.Fatalf("после срока: %q", rep.Text)
	}
	select {
	case err := <-seen:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Host получил контекст с ошибкой %v", err)
		}
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("Host не увидел конец срока")
	}
	waitFor(t, "место правки свободно", func() bool {
		rel, ok := r.svc.ed.takeSlot()
		if ok {
			rel()
		}
		return ok
	})
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errUndoNoOwn) // правка не применена: записи нет
	if c, _ := r.svc.Calls(); len(c) == 0 || c[len(c)-1].Note != "поздно" || !c[len(c)-1].Edit {
		t.Fatalf("журнал: %+v", c)
	}
}

// TestEditPanicNoLeak: паника в Host не оставляет место правки занятым и не роняет службу.
func TestEditPanicNoLeak(t *testing.T) {
	var boom atomic.Bool
	boom.Store(true)
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.auto = func(context.Context, AutoReq) (AutoRes, error) {
			if boom.Load() {
				panic("тест")
			}
			return AutoRes{Album: e.f.bump(), Added: 1}, nil
		}
	})
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errInternal)
	boom.Store(false)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
}

// TestAutobuildScanBudget: ожидание ленты — остаток срока вызова минус запас (В8); ленту не дождались —
// словами; свободных нет — словами.
func TestAutobuildScanBudget(t *testing.T) {
	r, cs := editRig(t, Options{AutoTimeout: 4 * time.Second, ScanSlack: time.Second}, nil)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	if ms := r.fx.edit.gotAuto().ScanWaitMs; ms < 2000 || ms > 3000 {
		t.Fatalf("ScanWaitMs %d, ждали ≈ 3000 (4 с − 1 с)", ms)
	}
	r2, cs2 := editRig(t, Options{AutoTimeout: 200 * time.Millisecond, ScanSlack: time.Second}, nil)
	okReply(t, tcall(t, cs2, "autobuild", map[string]any{"album": r2.fx.refArgs()}))
	if ms := r2.fx.edit.gotAuto().ScanWaitMs; ms != 0 {
		t.Fatalf("запас больше срока, а ждать %d мс", ms)
	}
	r.fx.edit.auto = func(context.Context, AutoReq) (AutoRes, error) { return AutoRes{}, ErrScanning }
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errScanning)
	r.fx.edit.auto = func(context.Context, AutoReq) (AutoRes, error) { return AutoRes{}, ErrNoFree }
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errNoFree)
}

// TestAutoOptsNoAlbumAgain: умолчания окна «Автосборка» (Авто, серии, вариант 1, «Обычное», все семейства);
// заданное агентом доходит как есть; Album и Again у запроса нет вовсе.
func TestAutoOptsNoAlbumAgain(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	got := r.fx.edit.gotAuto()
	if !got.PerAuto || !got.Series || got.Variety != 1 || got.Variant != 1 || got.Similar != 0 || got.Spreads != 0 ||
		len(got.Families) != 0 || len(got.Photos) != 0 || got.Client != "Claude Code" {
		t.Fatalf("умолчания: %+v", got)
	}
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs(), "spreads": 10, "perSpread": map[string]any{"min": 2, "max": 5},
		"series": false, "variety": 0, "variant": 7, "similar": 2, "families": []string{"air", "air", "dense"}, "photos": []string{idA, idA, idB}}))
	got = r.fx.edit.gotAuto()
	if got.PerAuto || got.PerMin != 2 || got.PerMax != 5 || got.Series || got.Variety != 0 || got.Variant != 7 || got.Similar != 2 ||
		got.Spreads != 10 || strings.Join(got.Families, ",") != "air,dense" || strings.Join(got.Photos, ",") != idA+","+idB {
		t.Fatalf("заданное агентом: %+v", got)
	}
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs(), "perSpread": "auto"}))
	if !r.fx.edit.gotAuto().PerAuto {
		t.Fatal("perSpread auto")
	}
	for _, bad := range []map[string]any{{"perSpread": "много"}, {"perSpread": map[string]any{"min": 5, "max": 2}}, {"perSpread": map[string]any{"min": 1}},
		{"variety": 3}, {"similar": 4}, {"spreads": 201}, {"families": []string{"wide"}}, {"album": map[string]any{"key": "k"}}, {"again": true}, {"album": nil}} {
		args := map[string]any{"album": r.fx.refArgs()}
		for k, v := range bad {
			args[k] = v
		}
		if rep := tcall(t, cs, "autobuild", args); !rep.IsError || !strings.HasPrefix(rep.Text, "неверные аргументы") {
			t.Errorf("%v: ждали отказ схемы, получили %q", bad, rep.Text)
		}
	}
}

// TestUndoLastOwn: undo_last_edit отменяет только последнюю правку этого подключения при неизменной
// ревизии; чужую, устаревшую и правку «менять нечего» — нет.
func TestUndoLastOwn(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	other := client(t, r.sock, "другой")
	a := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"].(map[string]any)
	failText(t, tcall(t, other, "undo_last_edit", map[string]any{"album": a}), errUndoNoOwn) // чужая: у того подключения правок нет
	r.fx.bump()                                                                              // человек правил после агента
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": a}), errUndoNotOwn)
	if r.fx.edit.nUndo.Load() != 0 {
		t.Fatal("Host.Undo вызван для не своей правки")
	}
	v := okReply(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{"sp1"}})).Data["album"].(map[string]any)
	got := okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": v})).Data["album"].(map[string]any)
	if r.fx.edit.gotUndo().Rev != int(v["rev"].(float64)) || got["rev"].(float64) != float64(r.fx.ref().Rev) {
		t.Fatalf("Undo получил %+v, ответ %v", r.fx.edit.gotUndo(), got)
	}
	// Правка «менять нечего» (0 разворотов) шага не ставит: отменять нечего.
	r.fx.edit.vary = func(context.Context, VaryReq) (AlbumRef, int, error) { return r.fx.ref(), 0, nil }
	z := okReply(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}})).Data
	if z["changed"].(float64) != 0 {
		t.Fatalf("changed: %v", z)
	}
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": z["album"]}), errUndoNotOwn)
	// Автосборка без шага (свободных нет — Added 0): отменять нечего.
	r.fx.edit.auto = func(context.Context, AutoReq) (AutoRes, error) { return AutoRes{Album: r.fx.ref()}, nil }
	n := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"]
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": n}), errUndoNotOwn)
	// Host нашёл, что ревизия ушла: запись снимается.
	r.fx.edit.auto = nil
	b := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"].(map[string]any)
	r.fx.edit.undo = func(context.Context, AlbumRef) (AlbumRef, error) { return AlbumRef{}, ErrAlbumEdited }
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": b}), errUndoNotOwn)
	if _, ok := r.svc.ed.getRec(nil); ok {
		t.Fatal("посторонняя запись")
	}
}

// TestUndoLastOwnNoChain: отмена не строит цепочку — вторая подряд отказана; после создания альбома
// отмена говорит «не отменяется»; добавление уже стоящей папки шага не ставит.
func TestUndoLastOwnNoChain(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	a := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"].(map[string]any)
	undone := okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": a})).Data["album"]
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": undone}), errUndoNotOwn)
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": a}), errUndoNotOwn)
	if r.fx.edit.nUndo.Load() != 1 {
		t.Fatalf("Host.Undo вызван %d раз", r.fx.edit.nUndo.Load())
	}
	// Ревизия после отмены не изменилась (Host так может): цепочку держит снятая запись, а не rev.
	r.fx.edit.undo = func(context.Context, AlbumRef) (AlbumRef, error) { return r.fx.ref(), nil }
	c := okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})).Data["album"]
	okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": c}))
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": c}), errUndoNotOwn)
	r.fx.edit.undo = nil
	// Создание: запись для отмены не ставится.
	okReply(t, tcall(t, cs, "create_album", map[string]any{}))
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errCreateNoUndo)
	// Папка, уже стоявшая в альбоме (merged), — не правка.
	r.fx.edit.ask = func(context.Context, AskReq) (AskRes, error) {
		return AskRes{Why: WhyAnswered, Album: r.fx.ref(), Folder: 1, Merged: true}, nil
	}
	m := okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()})).Data
	if m["folder"].(map[string]any)["merged"] != true {
		t.Fatalf("add_folder: %v", m)
	}
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": m["album"]}), errCreateNoUndo)
	// Настоящая добавленная папка — своя правка.
	r.fx.edit.ask = nil
	f := okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()})).Data["album"]
	okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": f}))
}

// TestShowSpreadNoEditFlag: «покажи» флажка правок не требует и копии не делает, но ключ, место и
// «человек занят» проходит; обложка и номер переводятся в ID разворота.
func TestShowSpreadNoEditFlag(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) { e.allowed = false })
	key := map[string]any{"key": r.fx.ref().Key}
	okShow := func(spread any, want int, id string) {
		t.Helper()
		got := okReply(t, tcall(t, cs, "show_spread", map[string]any{"album": key, "spread": spread})).Data["spread"].(float64)
		if int(got) != want || r.fx.edit.gotShow() != id {
			t.Fatalf("show %v: номер %v, id %q", spread, got, r.fx.edit.gotShow())
		}
	}
	okShow(1, 1, "sp1")
	okShow("sp1", 1, "sp1")
	okShow("cover", 0, model.CoverID)
	failText(t, tcall(t, cs, "show_spread", map[string]any{"album": key, "spread": 99}), errNoSpread)
	failText(t, tcall(t, cs, "show_spread", map[string]any{"album": map[string]any{"key": "e1"}, "spread": 1}), errAlbumChanged)
	if e := r.fx.edit; e.nBusy.Load() == 0 || e.nBackup.Load() != 0 || e.nShow.Load() != 3 {
		t.Fatalf("busy %d копий %d показов %d", e.nBusy.Load(), e.nBackup.Load(), e.nShow.Load())
	}
	c, _ := r.svc.Calls()
	if c[len(c)-1].Edit {
		t.Fatal("«покажи» помечено как правка")
	}
}

// TestVaryLayoutsArgs: развороты — номер или id, без повторов; обложка и чужое — отказ; по умолчанию variety 1.
func TestVaryLayoutsArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	okReply(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1, "sp1", "2"}, "families": []string{"dense"}}))
	if got := r.fx.edit.gotVary(); strings.Join(got.Spreads, ",") != "sp1,sp2" || got.Variety != 1 || len(got.Families) != 1 || got.Client != "Claude Code" {
		t.Fatalf("Host.Vary получил %+v", got)
	}
	for _, bad := range []any{9, "нет"} {
		failText(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1, bad}}), errNoSpreadFlat)
	}
	failText(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1, "cover"}}), errCoverNotTarget("vary_layouts"))
	for _, bad := range []map[string]any{{"spreads": []any{}}, {"variety": 3}, {"variety": 0}, {"families": []string{"x"}}} {
		args := map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}}
		for k, v := range bad {
			args[k] = v
		}
		if rep := tcall(t, cs, "vary_layouts", args); !rep.IsError || !strings.HasPrefix(rep.Text, "неверные аргументы") {
			t.Errorf("%v: %q", bad, rep.Text)
		}
	}
}

// TestEditToolsNoHost: Host без замыканий правок (или с неполным набором) — инструментов правки в
// списке нет, get_album о правках молчит; остальное работает.
func TestEditToolsNoHost(t *testing.T) {
	r := newRig(t, Options{}, "")
	r.fx.noEdit = true
	r.svc = New(r.fx.host(r.cfg, r.sock, func() {}), Options{})
	t.Cleanup(r.svc.Close)
	setLevel(r.cfg, 1)
	r.start(t)
	cs := client(t, r.sock, "x")
	res, err := cs.ListTools(callCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 11 {
		t.Fatalf("инструментов %d, ждали 11 чтения", len(res.Tools))
	}
	for _, tl := range res.Tools {
		if _, edit := editTools[tl.Name]; edit {
			t.Errorf("инструмент правки %s без замыканий Host", tl.Name)
		}
	}
	a := okReply(t, tcall(t, cs, "get_album", nil)).Data
	if a["edits"] != nil || a["scan"] != nil {
		t.Fatalf("get_album: %v", a)
	}
	// Неполный набор считается пустым.
	h := newFixture().host("", "", func() {})
	if !h.canEdit() {
		t.Fatal("полный набор не принят")
	}
	h.Undo = nil
	if h.canEdit() {
		t.Fatal("набор без Undo принят")
	}
}

// TestEditJournal: правящие вызовы помечены Edit, отказ — итогом словами.
func TestEditJournal(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	r.fx.edit.ask = func(context.Context, AskReq) (AskRes, error) { return AskRes{Why: WhyDeclined}, nil }
	failText(t, tcall(t, cs, "create_album", map[string]any{}), errDeclined)
	failText(t, tcall(t, cs, "create_album", map[string]any{}), errAskCooldown)
	okReply(t, tcall(t, cs, "get_album", nil))
	waitFor(t, "журнал", func() bool { c, _ := r.svc.Calls(); return len(c) == 4 })
	c, _ := r.svc.Calls()
	want := []struct {
		tool string
		edit bool
		note string
	}{{"Сводка альбома", false, ""}, {"Создать альбом", true, "отказано"}, {"Создать альбом", true, "отказано"}, {"Автосборка", true, ""}}
	for i, w := range want {
		if c[i].Tool != w.tool || c[i].Edit != w.edit || c[i].Note != w.note {
			t.Errorf("запись %d: %+v, ждали %+v", i, c[i], w)
		}
	}
}
