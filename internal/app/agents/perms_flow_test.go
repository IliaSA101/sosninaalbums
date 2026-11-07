package agents

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Разрешения агентов в службе (150 Т2): запрет, вопрос о правке, тихий слот, окно ступеней, событие.

// lastNote — итог последнего вызова в журнале.
func lastNote(t *testing.T, s *Service) string {
	t.Helper()
	calls, _ := s.Calls()
	if len(calls) == 0 {
		t.Fatal("журнал пуст")
	}
	return calls[0].Note
}

// TestPermsTable: таблица prefs и toolSpec описывают одни и те же инструменты с одним классом; у
// await_answer класса нет, start_program живёт у посредника.
func TestPermsTable(t *testing.T) {
	r := newRig(t, Options{}, "")
	specs := r.svc.Specs()
	byName := map[string]toolSpec{}
	for _, sp := range specs {
		byName[sp.name] = sp
		if sp.name == "await_answer" {
			if sp.perm != "" || prefs.AgentToolClass(sp.name) != "" {
				t.Errorf("await_answer: служебный, класса быть не должно: %q", sp.perm)
			}
			continue
		}
		if sp.perm == "" {
			t.Errorf("%s: нет класса разрешения", sp.name)
		}
		if want := prefs.AgentToolClass(sp.name); sp.perm != want {
			t.Errorf("%s: класс в toolSpec %q, в prefs %q", sp.name, sp.perm, want)
		}
		if sp.title == "" {
			t.Errorf("%s: нет названия для окна", sp.name)
		}
	}
	for _, name := range prefs.AgentToolNames() {
		if _, ok := byName[name]; !ok && name != "start_program" {
			t.Errorf("%s есть в prefs, а службе неизвестен", name)
		}
	}
	if _, ok := byName["start_program"]; ok {
		t.Error("start_program у службы быть не должно: его даёт посредник")
	}
	// Инструменты правок разной природы разошлись по своим классам, а не слиплись в один.
	for tool, class := range map[string]string{"get_contact_sheet": prefs.ClassRead, "show_spread": prefs.ClassShow,
		"set_marks": prefs.ClassEdit, "apply_print": prefs.ClassEdit, "add_folder": prefs.ClassFolder, "create_album": prefs.ClassAlbum} {
		if byName[tool].perm != class {
			t.Errorf("%s: класс %q, ждали %q", tool, byName[tool].perm, class)
		}
	}
	// Tools отдаёт то же без служебных.
	got := r.svc.Tools()
	if len(got) != len(specs)-1 {
		t.Errorf("Tools: %d, ждали %d", len(got), len(specs)-1)
	}
	// Host без правок: только чтение, и классы те же.
	fx := newFixture()
	fx.noEdit = true
	ro := New(fx.host(testkit.TempDir(t), "", func() {}), Options{})
	if len(ro.Tools()) == 0 {
		t.Error("Host без правок: инструментов нет")
	}
	for _, ti := range ro.Tools() {
		if ti.Class != prefs.ClassRead {
			t.Errorf("Host без правок: %s класса %s", ti.Name, ti.Class)
		}
	}
}

// TestPermDenyRead: «Запретить» у чтения — errDenied до Host и до проверки уровня; место чтения
// отдаётся (иначе пять запретов зависили бы службу); «Всегда» возвращает ответ.
func TestPermDenyRead(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.setPerm("get_album", prefs.PermDeny)
		e.setPerm("get_photo_preview", prefs.PermDeny)
	})
	before := r.fx.reads.Load()
	for range 3 * maxCalls { // больше мест, чем есть: утечка места дала бы errCallBusy
		failText(t, tcall(t, cs, "get_album", nil), errDenied)
	}
	failText(t, tcall(t, cs, "get_photo_preview", map[string]any{"id": idA}), errDenied) // уровень 1: не errNoImages
	if r.fx.reads.Load() != before {
		t.Error("запрещённое чтение дошло до Host")
	}
	if got := lastNote(t, r.svc); got != noteDenied {
		t.Errorf("журнал: %q", got)
	}
	okReply(t, tcall(t, cs, "get_photo", map[string]any{"id": idA})) // прочее чтение работает
	r.fx.edit.setPerm("get_album", prefs.PermAllow)
	okReply(t, tcall(t, cs, "get_album", nil))
	if st, _ := r.svc.Calls(); len(st) == 0 || st[0].Note != "" {
		t.Errorf("обычный вызов с пометкой: %+v", st)
	}
}

// TestPermDenyEdit: «Запретить» у правки, папки и альбома — errDenied; ни одно замыкание правок
// не тронуто, место правки и счёт отказов целы (запрет — не отказ человека).
func TestPermDenyEdit(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
		for _, tool := range []string{"autobuild", "vary_layouts", "undo_last_edit", "add_folder", "create_album", "show_spread", "set_marks"} {
			e.setPerm(tool, prefs.PermDeny)
		}
	})
	a := map[string]any{"album": r.fx.refArgs()}
	for range 3 {
		failText(t, tcall(t, cs, "create_album", nil), errDenied)
		failText(t, tcall(t, cs, "add_folder", a), errDenied)
		failText(t, tcall(t, cs, "autobuild", a), errDenied)
		failText(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}}), errDenied)
		failText(t, tcall(t, cs, "undo_last_edit", a), errDenied)
		failText(t, tcall(t, cs, "show_spread", map[string]any{"album": map[string]any{"key": r.fx.ref().Key}, "spread": 1}), errDenied)
		failText(t, tcall(t, cs, "set_marks", map[string]any{"album": r.fx.refArgs(), "ids": []any{idB}, "flag": "pick"}), errDenied)
	}
	if n := r.fx.edit.writes() + r.fx.edit.nBusy.Load(); n != 0 {
		t.Fatalf("запрет дошёл до замыканий правок: %d", n)
	}
	if s := r.svc.ed.strikes; s != 0 {
		t.Errorf("запрет записан как отказ человека: %d", s)
	}
	select {
	case r.svc.ed.slot <- struct{}{}:
		<-r.svc.ed.slot
	default:
		t.Fatal("место правки занято после запрета")
	}
	if got := lastNote(t, r.svc); got != noteDenied {
		t.Errorf("журнал: %q", got)
	}
	// Запрещён один инструмент — остальные правки работают; get_album называет доступность правок.
	r.fx.edit.setPerm("autobuild", prefs.PermAllow)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
	if ed := tcall(t, cs, "get_album", nil).Data["edits"].(map[string]any); ed["allowed"] != true {
		t.Errorf("get_album.edits при одной разрешённой правке: %v", ed)
	}
}

// TestPermLive: разрешения читаются на каждый вызов — смена без перезапуска службы и подключения.
func TestPermLive(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	e := r.fx.edit
	a := func() map[string]any { return map[string]any{"album": r.fx.refArgs()} }
	okReply(t, tcall(t, cs, "autobuild", a()))
	n := e.nPerm.Load()
	e.setPerm("autobuild", prefs.PermDeny)
	failText(t, tcall(t, cs, "autobuild", a()), errDenied)
	e.setPerm("autobuild", prefs.PermAllow)
	okReply(t, tcall(t, cs, "autobuild", a()))
	if got := e.nPerm.Load() - n; got != 2 {
		t.Errorf("Host.Perm вызван %d раз на два вызова", got)
	}
	// Незнакомое значение и «Спрашивать» у чтения — «Запретить» (закрыто по умолчанию).
	e.setPerm("get_album", "Allow")
	failText(t, tcall(t, cs, "get_album", nil), errDenied)
	e.setPerm("get_album", prefs.PermAsk)
	failText(t, tcall(t, cs, "get_album", nil), errDenied)
	e.setPerm("get_album", "")
	failText(t, tcall(t, cs, "get_album", nil), errDenied)
}

// TestPermDenyClosesAsk: вопрос висит, инструмент запретили — вопрос закрыт итогом off, агент получает
// errDenied, счёт отказов не растёт (в окне ничего не нужно было отвечать).
func TestPermDenyClosesAsk(t *testing.T) {
	closed := make(chan struct{})
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			<-ctx.Done()
			close(closed)
			return AskRes{Why: WhyGone}, nil
		}
	})
	p := okReply(t, tcall(t, cs, "create_album", nil)).Data["pending"].(map[string]any)
	r.fx.edit.setPerm("add_folder", prefs.PermDeny) // чужой инструмент вопрос не трогает
	r.svc.PermsChanged()
	select {
	case <-closed:
		t.Fatal("вопрос create_album закрыт запретом add_folder")
	case <-time.After(50 * time.Millisecond):
	}
	r.fx.edit.setPerm("create_album", prefs.PermDeny)
	r.svc.PermsChanged()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("вопрос не закрыт запретом")
	}
	failText(t, tcall(t, cs, "await_answer", map[string]any{"ask": p["ask"]}), errDenied)
	if s := r.svc.ed.strikes; s != 0 {
		t.Errorf("закрытие запретом посчитано отказом: %d", s)
	}
	r.fx.edit.setPerm("create_album", prefs.PermAllow)
	r.fx.edit.setPerm("add_folder", prefs.PermAllow)
	r.fx.edit.ask = nil
	okReply(t, tcall(t, cs, "create_album", nil)) // пауз нет
}

// TestPermAskEdit: «Спрашивать» у правки — вопрос вида edit с названием и числами входа, правка после
// «да» тем же вызовом; отказ и молчание правку не делают и считаются отказом человека.
func TestPermAskEdit(t *testing.T) {
	clk := newClock()
	o := askOpts(clk)
	o.CallTimeout, o.AutoTimeout = 300*time.Millisecond, 300*time.Millisecond
	r, cs := editRig(t, o, func(e *editFake) {
		for _, tool := range []string{"autobuild", "vary_layouts", "show_spread", "apply_print"} {
			e.setPerm(tool, prefs.PermAsk)
		}
	})
	e := r.fx.edit
	// «да»: вопрос, затем тело.
	variety := 2
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs(), "variety": variety, "families": []any{"air"}, "photos": []any{idB}}))
	q := e.gotAsk()
	if q.Kind != AskEdit || q.Tool != "autobuild" || q.Client != "Claude Code" || q.Count != 1 || q.Variety != 2 ||
		!slices.Equal(q.Families, []string{"air"}) || q.Key != r.fx.ref().Key || q.Always || q.Hint != "" {
		t.Fatalf("вопрос об автосборке: %+v", q)
	}
	if e.nAuto.Load() != 1 || e.nBackup.Load() != 1 {
		t.Fatalf("после «да»: auto=%d backup=%d", e.nAuto.Load(), e.nBackup.Load())
	}
	if got := lastNote(t, r.svc); got != noteYes {
		t.Errorf("журнал: %q", got)
	}
	okReply(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1, "sp2"}, "variety": 2}))
	if q = e.gotAsk(); q.Tool != "vary_layouts" || !slices.Equal(q.Spreads, []string{"sp1", "sp2"}) || q.Variety != 2 {
		t.Fatalf("вопрос о раскладках: %+v", q)
	}
	okReply(t, tcall(t, cs, "show_spread", map[string]any{"album": map[string]any{"key": r.fx.ref().Key}, "spread": "cover"}))
	if q = e.gotAsk(); q.Tool != "show_spread" || len(q.Spreads) != 1 || q.Spreads[0] != "cover" || q.Rev != 0 {
		t.Fatalf("вопрос о показе: %+v", q)
	}
	if e.nShow.Load() != 1 || e.nBackup.Load() != 2 { // копия — только у правящих
		t.Fatalf("show=%d backup=%d", e.nShow.Load(), e.nBackup.Load())
	}
	okReply(t, tcall(t, cs, "apply_print", map[string]any{"album": r.fx.refArgs(), "profile": "b1", "format": "30x30"}))
	if g := e.got34().lastPrint; e.nPrint.Load() != 1 || g.Profile != "b1" || g.Format != "30x30" {
		t.Fatalf("после «да» на формат: print=%d %+v", e.nPrint.Load(), g)
	}
	if q = e.gotAsk(); q.Tool != "apply_print" || q.Item != "b1" || q.Format != "30x30" {
		t.Fatalf("вопрос о формате: %+v", q)
	}
	// Нет такого разворота — отказ до вопроса, окна нет.
	n := e.nAsk.Load()
	failText(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{99}}), errNoSpreadFlat)
	if e.nAsk.Load() != n {
		t.Error("вопрос задан про несуществующий разворот")
	}

	// «Отмена»: правки нет, пауза, как у вопроса о папке.
	e.ask = answerWith(WhyDeclined)
	nAuto := e.nAuto.Load()
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errDeclined)
	if e.nAuto.Load() != nAuto {
		t.Error("отказ человека, а правка сделана")
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errAskCooldown)
	if got := lastNote(t, r.svc); got != "отказано" {
		t.Errorf("журнал: %q", got)
	}
	// Человек не ответил за срок вызова: вопрос закрыт, правки нет, место свободно.
	clk.Add(time.Minute)
	e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) { <-ctx.Done(); return AskRes{Why: WhyGone}, nil }
	failText(t, tcall(t, cs, "vary_layouts", map[string]any{"album": r.fx.refArgs(), "spreads": []any{1}}), errTimeout)
	waitFor(t, "место правки свободно", func() bool {
		select {
		case r.svc.ed.slot <- struct{}{}:
			<-r.svc.ed.slot
			return true
		default:
			return false
		}
	})
}

// TestPermAskRevChanged: пока человек отвечал, альбом правили (или открыли другой) — «да» не применяет
// правку к чужой ревизии: агент перечитывает альбом.
func TestPermAskRevChanged(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) { e.setPerm("autobuild", prefs.PermAsk) })
	e := r.fx.edit
	e.ask = func(context.Context, AskReq) (AskRes, error) {
		r.fx.bump() // человек правит, пока вопрос открыт
		return AskRes{Why: WhyAnswered}, nil
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errAlbumEdited)
	if e.nAuto.Load() != 0 || e.nBackup.Load() != 0 {
		t.Fatalf("правка применена к чужой ревизии: auto=%d backup=%d", e.nAuto.Load(), e.nBackup.Load())
	}
	e.ask = func(context.Context, AskReq) (AskRes, error) {
		r.fx.mu.Lock()
		r.fx.ed++ // открыли другой альбом
		r.fx.mu.Unlock()
		return AskRes{Why: WhyAnswered}, nil
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errAlbumChanged)
	if e.nAuto.Load() != 0 {
		t.Fatal("правка применена к другому альбому")
	}
	// «да» без перемен проходит и сбрасывает счёт отказов.
	e.ask = nil
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}))
}

// quietAsk — Host.Ask «Всегда»: при Always фасад отвечает тишиной (новая ревизия), иначе человек
// отвечает «да». Считает то и другое.
func quietAsk(r *rig, quiet, asked *atomic.Int32) func(context.Context, AskReq) (AskRes, error) {
	return func(_ context.Context, q AskReq) (AskRes, error) {
		if q.Always {
			quiet.Add(1)
			return AskRes{Why: WhyQuiet, Album: r.fx.bump(), Folder: 1}, nil
		}
		asked.Add(1)
		return AskRes{Why: WhyAnswered, Album: r.fx.bump(), Folder: 1}, nil
	}
}

// TestQuietBudget: восемь папок подряд при «Всегда» — три тихих, пять вопросов; через десять минут
// слот снова есть. Журнал различает «без вопроса» и «да».
func TestQuietBudget(t *testing.T) {
	clk := newClock()
	var quiet, asked atomic.Int32
	r, cs := editRig(t, askOpts(clk), nil)
	r.fx.edit.ask = quietAsk(r, &quiet, &asked)
	var notes []string
	for range 8 {
		okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
		notes = append(notes, lastNote(t, r.svc))
	}
	if quiet.Load() != 3 || asked.Load() != 5 {
		t.Fatalf("тихих %d, вопросов %d", quiet.Load(), asked.Load())
	}
	if want := []string{noteQuiet, noteQuiet, noteQuiet, noteYes, noteYes, noteYes, noteYes, noteYes}; !slices.Equal(notes, want) {
		t.Errorf("журнал: %q", notes)
	}
	clk.Add(quietWindow + time.Second)
	okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
	if quiet.Load() != 4 {
		t.Errorf("слот не вернулся через окно: %d", quiet.Load())
	}
	// Вопрос вместо тишины (фасад не пустил) слот возвращает: «Спрашивать» по месту не съедает бюджет.
	r.fx.edit.ask = func(_ context.Context, q AskReq) (AskRes, error) {
		return AskRes{Why: WhyAnswered, Album: r.fx.bump(), Folder: 1}, nil
	}
	for range 6 {
		okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
	}
	r.fx.edit.ask = quietAsk(r, &quiet, &asked)
	okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
	if quiet.Load() != 5 {
		t.Errorf("слоты, отданные вопросом, пропали: тихих %d", quiet.Load())
	}
	// «Спрашивать» и «Запретить» тихого слота не берут.
	r.fx.edit.setPerm("add_folder", prefs.PermAsk)
	okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
	if q := r.fx.edit.gotAsk(); q.Always {
		t.Error("Always при «Спрашивать»")
	}
}

// TestQuietBudgetParallel: восемь одновременных вызовов — тихих не больше трёх, прочие errEditBusy,
// errCallBusy (предел одновременных вызовов службы: на медленном раннере все восемь успевают встать
// разом) или вопрос; сам бюджет под замком выдаёт ровно три слота при любой гонке.
func TestQuietBudgetParallel(t *testing.T) {
	var quiet, asked atomic.Int32
	r, cs := editRig(t, askOpts(newClock()), nil)
	base := quietAsk(r, &quiet, &asked)
	r.fx.edit.ask = func(ctx context.Context, q AskReq) (AskRes, error) {
		time.Sleep(15 * time.Millisecond) // вызовы налезают друг на друга
		return base(ctx, q)
	}
	args := r.fx.refArgs()
	var chs []<-chan reply
	for range 8 {
		chs = append(chs, bgCall(cs, "add_folder", map[string]any{"album": args}))
	}
	var busy, other int
	for _, ch := range chs {
		switch got := within(t, ch, "add_folder"); {
		case !got.IsError:
		case got.Text == errEditBusy.Error(), got.Text == errCallBusy.Error():
			busy++
		case got.Text == errAlbumEdited.Error(): // ревизия ушла, пока вызов ждал места
			other++
		default:
			t.Errorf("неожиданный ответ: %q", got.Text)
		}
	}
	if quiet.Load() > 3 {
		t.Fatalf("тихих %d, предел 3", quiet.Load())
	}
	t.Logf("тихих %d, вопросов %d, занято %d, ревизия %d", quiet.Load(), asked.Load(), busy, other)
	// Места вызовов вернулись: следующий вызов не упирается в предел одновременных.
	if got := within(t, bgCall(cs, "add_folder", map[string]any{"album": args}), "add_folder после гонки"); got.IsError && got.Text == errCallBusy.Error() {
		t.Error("место вызова не освободилось после гонки")
	}

	// Сам бюджет: сто гонщиков — ровно три слота.
	e := newEditState(Options{}.withDefaults())
	var got atomic.Int32
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := e.takeQuiet(); ok {
				got.Add(1)
			}
		}()
	}
	wg.Wait()
	if got.Load() != quietMax {
		t.Fatalf("takeQuiet выдал %d слотов, ждали %d", got.Load(), quietMax)
	}
}

// TestQuietNoStrikeReset: тихий итог не обнуляет ступени отказов и не растит их — агент не стирает
// отказы человека тихими вызовами между вопросами.
func TestQuietNoStrikeReset(t *testing.T) {
	clk := newClock()
	var mode atomic.Value
	mode.Store("decline")
	r, cs := editRig(t, askOpts(clk), func(e *editFake) {
		e.ask = func(_ context.Context, q AskReq) (AskRes, error) {
			if mode.Load() == "quiet" {
				return AskRes{Why: WhyQuiet, Album: e.f.bump(), Folder: 1}, nil
			}
			return AskRes{Why: WhyDeclined}, nil
		}
	})
	add := func() reply { return tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}) }
	failText(t, add(), errDeclined)
	if r.svc.ed.strikes != 1 {
		t.Fatalf("после отказа ступеней %d", r.svc.ed.strikes)
	}
	clk.Add(31 * time.Second) // пауза кончилась
	mode.Store("quiet")
	okReply(t, add())
	if r.svc.ed.strikes != 1 {
		t.Fatalf("тихий итог сменил счёт: %d", r.svc.ed.strikes)
	}
	mode.Store("decline")
	clk.Add(time.Second)
	failText(t, add(), errDeclined)
	failText(t, add(), errAskMuted) // это второй отказ подряд: пять минут, а не первая ступень
}

// TestQuietWhileMuted: пауза, тишина и третья ступень гасят и тихий путь — Host не спрошен, слот
// бюджета не занят.
func TestQuietWhileMuted(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyDeclined) })
	e := r.fx.edit
	add := func() reply { return tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}) }
	create := func() reply { return tcall(t, cs, "create_album", nil) }
	failText(t, add(), errDeclined)
	n := e.nAsk.Load()
	failText(t, add(), errAskCooldown)
	failText(t, create(), errAskCooldown)
	clk.Add(31 * time.Second)
	failText(t, add(), errDeclined)
	failText(t, add(), errAskMuted)
	failText(t, create(), errAskMuted)
	clk.Add(5*time.Minute + time.Second)
	failText(t, add(), errDeclined) // третья
	failText(t, add(), errAskOff)
	failText(t, create(), errAskOff)
	if got := e.nAsk.Load() - n; got != 2 {
		t.Errorf("Host.Ask вызван %d раз в паузах (ждали 2 настоящих вопроса)", got)
	}
	r.svc.ed.mu.Lock()
	taken := len(r.svc.ed.quiet)
	r.svc.ed.mu.Unlock()
	if taken != 0 {
		t.Errorf("тихие слоты не возвращены: %d", taken)
	}
	r.svc.AllowAsks()
	e.ask = nil
	okReply(t, add())
}

// TestQuietUndoLast: тихая папка пишет запись отмены — undo_last_edit её отменяет; тихий альбом — как
// «да» в 149: создание не отменяется.
func TestQuietUndoLast(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), nil)
	var ref AlbumRef
	r.fx.edit.ask = func(_ context.Context, q AskReq) (AskRes, error) {
		if !q.Always {
			t.Error("ждали тихий путь")
		}
		ref = r.fx.bump()
		return AskRes{Why: WhyQuiet, Album: ref, Folder: 2}, nil
	}
	out := okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}))
	if out.Data["folder"].(map[string]any)["n"] != float64(2) {
		t.Fatalf("итог тихой папки: %v", out.Data)
	}
	okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": map[string]any{"key": ref.Key, "rev": ref.Rev}}))
	if got := r.fx.edit.gotUndo(); got != ref {
		t.Fatalf("Undo получил %+v, ждали %+v", got, ref)
	}
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errUndoNotOwn) // вторая подряд
	// Тихий альбом.
	r.fx.edit.ask = func(context.Context, AskReq) (AskRes, error) {
		return AskRes{Why: WhyQuiet, Album: r.fx.bump()}, nil
	}
	okReply(t, tcall(t, cs, "create_album", nil))
	failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errCreateNoUndo)
	if got := lastNote(t, r.svc); got != "" {
		t.Logf("журнал после undo: %q", got)
	}
}

// TestStrikesWindow: ступени отказов считаются в окне 30 минут; залп окно не ослабляет, а третья
// ступень им не снимается.
func TestStrikesWindow(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyDeclined) })
	add := func() reply { return tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}) }

	// Два отказа, потом 31 минута: следующий отказ — снова первая ступень (30 с, не 5 минут).
	failText(t, add(), errDeclined)
	clk.Add(31 * time.Second)
	failText(t, add(), errDeclined)
	failText(t, add(), errAskMuted)
	clk.Add(31 * time.Minute)
	failText(t, add(), errDeclined)
	failText(t, add(), errAskCooldown)
	clk.Add(31 * time.Second)
	if err := r.svc.ed.admit(); err != nil {
		t.Fatalf("после 30 с первой ступени вопросы закрыты: %v", err)
	}

	// Залп в окне: 30 с, 5 минут, третий — до «Разрешить вопросы»; окно её не снимает.
	r2, cs2 := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyDeclined) })
	add2 := func() reply { return tcall(t, cs2, "add_folder", map[string]any{"album": r2.fx.refArgs()}) }
	failText(t, add2(), errDeclined)
	clk.Add(31 * time.Second)
	failText(t, add2(), errDeclined)
	clk.Add(5*time.Minute + time.Second)
	failText(t, add2(), errDeclined)
	failText(t, add2(), errAskOff)
	if !r2.svc.AsksMuted() {
		t.Fatal("третья ступень не включена")
	}
	clk.Add(2 * time.Hour)
	failText(t, add2(), errAskOff)
	if !r2.svc.AsksMuted() {
		t.Fatal("окно сняло третью ступень")
	}
	r2.svc.AllowAsks()
	if r2.svc.AsksMuted() {
		t.Fatal("AllowAsks не снял третью ступень")
	}
}

// TestDeniedEventOnce: agents:denied — первый запрет по инструменту за запуск программы, с именем клиента и
// названием; повтор, обычные вызовы и паника обработчика события тишину не нарушают.
func TestDeniedEventOnce(t *testing.T) {
	type ev struct{ client, tool, title string }
	var mu sync.Mutex
	var got []ev
	o := Options{OnDenied: func(client, tool, title string) {
		mu.Lock()
		got = append(got, ev{client, tool, title})
		mu.Unlock()
		if tool == "list_albums" {
			panic("обработчик события упал")
		}
	}}
	r, cs := editRig(t, o, func(e *editFake) {
		e.setPerm("create_album", prefs.PermDeny)
		e.setPerm("get_album", prefs.PermDeny)
		e.setPerm("list_albums", prefs.PermDeny)
	})
	okReply(t, tcall(t, cs, "get_photo", map[string]any{"id": idA}))
	for range 3 {
		failText(t, tcall(t, cs, "create_album", nil), errDenied)
	}
	failText(t, tcall(t, cs, "get_album", nil), errDenied)
	failText(t, tcall(t, cs, "list_albums", nil), errDenied) // обработчик паникует — ответ тот же
	failText(t, tcall(t, cs, "get_album", nil), errDenied)
	mu.Lock()
	want := []ev{{"Claude Code", "create_album", "Создать альбом"}, {"Claude Code", "get_album", "Сводка альбома"}, {"Claude Code", "list_albums", "Последние альбомы"}}
	if !slices.Equal(got, want) {
		t.Errorf("события: %+v\nждали %+v", got, want)
	}
	mu.Unlock()
	// Перезапуск доступа запуск программы не меняет: тост не повторяется.
	r.svc.Close()
	if err := r.svc.Start(); err != nil {
		t.Fatal(err)
	}
	cs2 := client(t, r.sock, "Claude Code")
	failText(t, tcall(t, cs2, "create_album", nil), errDenied)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Errorf("повтор события после перезапуска доступа: %d", len(got))
	}
}

// TestToolsContractPerms: список инструментов от разрешений не зависит (0.34-150-2): «Запретить» всё —
// тот же tools/list, что и при «Всегда»; клиент с закешированным списком получает отказ, а не «нет такого».
func TestToolsContractPerms(t *testing.T) {
	list := func(deny bool) []byte {
		r, cs := editRig(t, Options{}, func(e *editFake) {
			if deny {
				for _, name := range prefs.AgentToolNames() {
					e.setPerm(name, prefs.PermDeny)
				}
			}
		})
		_ = r
		res, err := cs.ListTools(callCtx(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		slices.SortFunc(res.Tools, func(a, b *mcp.Tool) int { return strings.Compare(a.Name, b.Name) })
		b, err := json.Marshal(res.Tools)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if a, b := list(false), list(true); string(a) != string(b) {
		t.Fatal("tools/list зависит от разрешений")
	}
}

// TestPermOneRead: с Host.PermLevel вызов берёт разрешение и уровень из одного чтения config.json;
// Host.Perm (второе чтение) не зовётся, уровень — тот, что вернул PermLevel (ревью 150, п. 8).
func TestPermOneRead(t *testing.T) {
	var nLevel atomic.Int32
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.permLvl = func(tool string) (string, string, int) {
			nLevel.Add(1)
			return prefs.PermAllow, "", 1
		}
	})
	setLevel(r.cfg, 3)                                               // файл говорит 3, а PermLevel — 1: уровень берётся из него
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errNoImages) // свой вызов
	n, l := r.fx.edit.nPerm.Load(), nLevel.Load()
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errNoImages)
	if got, gotL := r.fx.edit.nPerm.Load()-n, nLevel.Load()-l; got != 0 || gotL != 1 {
		t.Errorf("Host.Perm %d раз, PermLevel %d раз на вызов: ждали 0 и 1", got, gotL)
	} // уровень 1 из PermLevel, хотя файл говорит 3
}
