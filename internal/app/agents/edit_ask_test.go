package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Вопросы агента, служебная часть (149 Т2): pending и await_answer, счёт отказов на программу.
// Окно, выбор папки и состояния asked → choosing → done — у фасада (Т3).

// askOpts — малые сроки и свои часы.
func askOpts(clk *fakeClock) Options {
	return Options{Now: clk.Now, AskWait: 60 * time.Millisecond, BusyWait: 30 * time.Millisecond}
}

// answerWith — Host.Ask, который сразу заканчивается итогом why.
func answerWith(why string) func(context.Context, AskReq) (AskRes, error) {
	return func(context.Context, AskReq) (AskRes, error) { return AskRes{Why: why}, nil }
}

// TestAskPending: вопрос не успели закрыть за AskWait — pending с жетоном и словами для человека;
// место правки держится до итога; жетон работает только в своём подключении; второе ожидание по
// жетону сразу получает pending; итог отдаётся один раз.
func TestAskPending(t *testing.T) {
	answer := make(chan AskRes)
	clk := newClock()
	o := askOpts(clk)
	o.AskWait = 400 * time.Millisecond
	r, cs := editRig(t, o, func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			select {
			case res := <-answer:
				return res, nil
			case <-ctx.Done():
				return AskRes{Why: WhyGone}, nil
			}
		}
	})
	other := client(t, r.sock, "другой")
	p := okReply(t, tcall(t, cs, "create_album", map[string]any{"folder_hint": "Петровы"})).Data["pending"].(map[string]any)
	tok := p["ask"].(string)
	if len(tok) != 32 || !strings.Contains(p["text"].(string), "await_answer") || !strings.Contains(p["text"].(string), "окне") {
		t.Fatalf("pending: %v", p)
	}
	if q := r.fx.edit.gotAsk(); q.Kind != AskCreate || q.Client != "Claude Code" || q.Hint != "Петровы" {
		t.Fatalf("Host.Ask получил %+v", q)
	}
	failText(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()}), errEditBusy) // место у вопроса
	failText(t, tcall(t, other, "await_answer", map[string]any{"ask": tok}), errAskGone)         // чужое подключение
	failText(t, tcall(t, cs, "await_answer", map[string]any{"ask": strings.Repeat("0", 32)}), errAskGone)

	first := bgCall(cs, "await_answer", map[string]any{"ask": tok})
	waitFor(t, "первое ожидание", func() bool {
		r.svc.ed.mu.Lock()
		defer r.svc.ed.mu.Unlock()
		return r.svc.ed.cur != nil && r.svc.ed.cur.waiting
	})
	second := okReply(t, tcall(t, cs, "await_answer", map[string]any{"ask": tok}))
	if second.Data["pending"] == nil {
		t.Fatalf("второе ожидание: %v", second.Data)
	}
	select {
	case <-first:
		t.Fatal("первое ожидание кончилось вместе со вторым")
	default:
	}
	answer <- AskRes{Why: WhyAnswered, Album: r.fx.bump(), Folder: 1, Scanning: true}
	res := within(t, first, "итог по жетону")
	if res.IsError || res.Data["folder"].(map[string]any)["n"].(float64) != 1 || res.Data["scan"].(map[string]any)["active"] != true || res.Data["album"] == nil {
		t.Fatalf("итог: %q %v", res.Text, res.Data)
	}
	failText(t, tcall(t, cs, "await_answer", map[string]any{"ask": tok}), errAskClosed)
	okReply(t, tcall(t, cs, "autobuild", map[string]any{"album": r.fx.refArgs()})) // место свободно
}

// TestAskPendingResultKept: человек ответил, пока агент не ждал, — итог ждёт await_answer до AskLife.
func TestAskPendingResultKept(t *testing.T) {
	clk := newClock()
	o := askOpts(clk)
	hold := make(chan struct{})
	r, cs := editRig(t, o, func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			<-hold
			return AskRes{Why: WhyDeclined}, nil
		}
	})
	tok := okReply(t, tcall(t, cs, "create_album", nil)).Data["pending"].(map[string]any)["ask"]
	close(hold)
	waitFor(t, "итог готов", func() bool {
		r.svc.ed.mu.Lock()
		defer r.svc.ed.mu.Unlock()
		return r.svc.ed.cur != nil && r.svc.ed.cur.finished()
	})
	failText(t, tcall(t, cs, "await_answer", map[string]any{"ask": tok}), errDeclined)
	// Итог не забрали дольше AskLife — жетон мёртв.
	hold2 := make(chan struct{})
	r2, cs2 := editRig(t, o, func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) { <-hold2; return AskRes{Why: WhyGone}, nil }
	})
	tok2 := okReply(t, tcall(t, cs2, "create_album", nil)).Data["pending"].(map[string]any)["ask"]
	close(hold2)
	waitFor(t, "итог готов", func() bool {
		r2.svc.ed.mu.Lock()
		defer r2.svc.ed.mu.Unlock()
		return r2.svc.ed.cur != nil && r2.svc.ed.cur.finished()
	})
	clk.Add(121 * time.Second)
	failText(t, tcall(t, cs2, "await_answer", map[string]any{"ask": tok2}), errAskGone)
}

// TestAwaitAnswerNoSem: await_answer не занимает место чтения (и не мешается, когда оно одно и занято).
func TestAwaitAnswerNoSem(t *testing.T) {
	var hold atomic.Bool
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	o := askOpts(newClock())
	o.MaxCalls = 1
	r, cs := editRig(t, o, func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			<-ctx.Done()
			return AskRes{Why: WhyGone}, nil
		}
		e.f.hook = func() {
			if hold.Load() {
				entered <- struct{}{}
				<-release
			}
		}
	})
	tok := okReply(t, tcall(t, cs, "create_album", nil)).Data["pending"].(map[string]any)["ask"]
	hold.Store(true)
	busy := bgCall(cs, "get_album", nil) // единственное место чтения
	<-entered
	failText(t, tcall(t, cs, "get_photo", map[string]any{"id": idA}), errCallBusy)
	if aw := tcall(t, cs, "await_answer", map[string]any{"ask": tok}); aw.IsError || aw.Data["pending"] == nil {
		t.Fatalf("await_answer при занятом месте чтения: %q", aw.Text)
	}
	close(release)
	within(t, busy, "чтение")
	_ = r
}

// TestAskCancelCounts: любое закрытие вопроса без «да» (отказ, срок, отмена вызова, неверная папка) —
// пауза; не отказ человека (программа закрывается, флажок, черновик занят, окно занято) не считается.
func TestAskCancelCounts(t *testing.T) {
	cases := []struct {
		name  string
		why   string
		err   *agentErr
		count bool
	}{
		{"declined", WhyDeclined, errDeclined, true}, {"timeout", WhyTimeout, errAskTimeout, true}, {"gone", WhyGone, errAskGone, true},
		{"too_wide", WhyTooWide, errFolderTooWide, true}, {"ours", WhyOurs, errFolderOurs, true},
		{"closed", WhyClosed, errClosing, false}, {"off", WhyOff, errNoEdits, false}, {"draft", WhyDraftLocked, errDraftLocked, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, cs := editRig(t, askOpts(newClock()), func(e *editFake) { e.ask = answerWith(tc.why) })
			failText(t, tcall(t, cs, "create_album", nil), tc.err)
			next := tcall(t, cs, "create_album", nil)
			if tc.count {
				failText(t, next, errAskCooldown)
				if r.fx.edit.nAsk.Load() != 1 {
					t.Fatal("в паузу окно открыто снова")
				}
			} else {
				failText(t, next, tc.err)
				if r.fx.edit.nAsk.Load() != 2 {
					t.Fatal("не отказ человека, а окно закрыто на паузу")
				}
			}
		})
	}
	t.Run("host_error", func(t *testing.T) {
		r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
			e.ask = func(context.Context, AskReq) (AskRes, error) { return AskRes{}, ErrAskBusy }
		})
		failText(t, tcall(t, cs, "create_album", nil), errAskBusy)
		failText(t, tcall(t, cs, "create_album", nil), errAskBusy)
		if r.fx.edit.nAsk.Load() != 2 {
			t.Fatal("вопрос, которого не было, засчитан отказом")
		}
	})
	t.Run("call_canceled", func(t *testing.T) {
		o := askOpts(newClock())
		o.AskWait = testkit.Limit(5 * time.Second)
		started := make(chan struct{}, 1)
		r, cs := editRig(t, o, func(e *editFake) {
			e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
				started <- struct{}{}
				<-ctx.Done()
				return AskRes{Why: WhyGone}, nil
			}
		})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()
		if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_album", Arguments: map[string]any{}}); err == nil {
			t.Fatal("отменённый вызов вернул ответ")
		}
		waitFor(t, "вопрос закрыт отменой", func() bool {
			r.svc.ed.mu.Lock()
			defer r.svc.ed.mu.Unlock()
			return r.svc.ed.strikes == 1
		})
		failText(t, tcall(t, cs, "create_album", nil), errAskCooldown)
	})
}

// TestAskMuteAcrossSessions: отказы считаются на программу, а не на подключение: второй отказ (от
// другого подключения) — пять минут тишины для всех; новое подключение паузу не обходит.
func TestAskMuteAcrossSessions(t *testing.T) {
	clk := newClock()
	r, a := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyDeclined) })
	b := client(t, r.sock, "B")
	failText(t, tcall(t, a, "create_album", nil), errDeclined)
	failText(t, tcall(t, b, "create_album", nil), errAskCooldown)
	clk.Add(31 * time.Second)
	failText(t, tcall(t, b, "create_album", nil), errDeclined) // второй подряд — уже от B
	c := client(t, r.sock, "C")
	clk.Add(31 * time.Second)
	failText(t, tcall(t, c, "create_album", nil), errAskMuted)
	failText(t, tcall(t, a, "add_folder", map[string]any{"album": r.fx.refArgs()}), errAskMuted)
	clk.Add(5 * time.Minute)
	r.fx.edit.ask = nil // человек согласился бы: тишина кончилась, вопрос снова задаётся
	okReply(t, tcall(t, c, "create_album", nil))
	if r.fx.edit.nAsk.Load() != 3 {
		t.Fatalf("окно открывали %d раз, ждали 3", r.fx.edit.nAsk.Load())
	}
	if r.svc.AsksMuted() {
		t.Fatal("пятиминутная тишина — не третья ступень")
	}
}

// TestAskThirdStrike: третье закрытие подряд — вопросов нет до перезапуска или AllowAsks; «да» обнуляет счёт.
func TestAskThirdStrike(t *testing.T) {
	clk := newClock()
	r, cs := editRig(t, askOpts(clk), func(e *editFake) { e.ask = answerWith(WhyDeclined) })
	for i, want := range []*agentErr{errDeclined, errDeclined, errDeclined} {
		failText(t, tcall(t, cs, "create_album", nil), want)
		clk.Add(6 * time.Minute)
		if got := r.svc.AsksMuted(); got != (i == 2) {
			t.Fatalf("после отказа %d: AsksMuted %v", i+1, got)
		}
	}
	changed := r.changed.Load()
	clk.Add(48 * time.Hour)
	failText(t, tcall(t, cs, "create_album", nil), errAskOff)
	failText(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs()}), errAskOff)
	if r.fx.edit.nAsk.Load() != 3 {
		t.Fatal("окно открыто при третьей ступени")
	}
	r.svc.AllowAsks()
	if r.svc.AsksMuted() || r.changed.Load() == changed {
		t.Fatalf("AllowAsks не снял тишину или не сообщил (%d → %d)", changed, r.changed.Load())
	}
	failText(t, tcall(t, cs, "create_album", nil), errDeclined)
	clk.Add(31 * time.Second)
	r.fx.edit.ask = nil // «да»
	okReply(t, tcall(t, cs, "create_album", nil))
	r.fx.edit.ask = answerWith(WhyDeclined)
	failText(t, tcall(t, cs, "create_album", nil), errDeclined)
	clk.Add(31 * time.Second)
	failText(t, tcall(t, cs, "create_album", nil), errDeclined) // счёт пошёл заново: это второй отказ, не мёртвая тишина
	if r.svc.AsksMuted() {
		t.Fatal("«да» не обнулило счёт")
	}
}

// TestAskShutdownNotRefusal: программа закрывается — вопрос кончается «closed» и отказом человека не считается.
func TestAskShutdownNotRefusal(t *testing.T) {
	r, cs := editRig(t, askOpts(newClock()), func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			<-ctx.Done()
			return AskRes{Why: WhyGone}, nil // фасад не различает «ушёл» и «закрыто»: различает служба
		}
	})
	okReply(t, tcall(t, cs, "create_album", nil)) // pending
	done := make(chan struct{})
	go func() { r.svc.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("Close застрял на открытом вопросе")
	}
	waitFor(t, "вопрос закончен", func() bool {
		r.svc.ed.mu.Lock()
		defer r.svc.ed.mu.Unlock()
		return r.svc.ed.cur != nil && r.svc.ed.cur.finished()
	})
	r.svc.ed.mu.Lock()
	n := r.svc.ed.strikes
	r.svc.ed.mu.Unlock()
	if n != 0 {
		t.Fatalf("закрытие программы засчитано отказом (%d)", n)
	}
}

// TestAskClientSpoof: имя клиента в вопросе — очищенное; похожее на программу — как неизвестное.
func TestAskClientSpoof(t *testing.T) {
	cases := map[string]string{"Claude Code": "Claude Code", "Sosnina Albums": "", "SOSNINA": "", "Sоsnina": "", // о — кириллическая
		"Сосн Альбомс": "", "": "", "OpenㅤCode": "OpenCode", "A B C": "ABC", "x́̂̃̄y": "x́̂y"}
	for name, want := range cases {
		r, cs := editRigNamed(t, name)
		okReply(t, tcall(t, cs, "create_album", nil))
		if got := r.fx.edit.gotAsk().Client; got != want {
			t.Errorf("клиент %q: в вопросе %q, ждали %q", name, got, want)
		}
	}
}

func editRigNamed(t *testing.T, name string) (*rig, *mcp.ClientSession) {
	t.Helper()
	r := newRig(t, Options{}, "")
	setLevel(r.cfg, 1)
	r.start(t)
	return r, client(t, r.sock, name)
}

// TestAskHintLimit: folder_hint длиннее 4096 байт — отказ до вопроса.
func TestAskHintLimit(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	failText(t, tcall(t, cs, "create_album", map[string]any{"folder_hint": strings.Repeat("я", 2049)}), errBadFolder) // 4098 байт, 2049 знаков
	failText(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs(), "folder_hint": strings.Repeat("я", 2049)}), errBadFolder)
	if r.fx.edit.nAsk.Load() != 0 {
		t.Fatal("вопрос открыт с негодной подсказкой")
	}
	okReply(t, tcall(t, cs, "create_album", map[string]any{"folder_hint": strings.Repeat("a", 4096)}))
	r.svc.ed.mu.Lock()
	r.svc.ed.strikes = 0
	r.svc.ed.mu.Unlock()
	okReply(t, tcall(t, cs, "add_folder", map[string]any{"album": r.fx.refArgs(), "folder_hint": "x"}))
	if q := r.fx.edit.gotAsk(); q.Kind != AskAdd || q.Key != r.fx.ref().Key || q.Rev == 0 {
		t.Fatalf("add_folder: %+v", q)
	}
}

// TestBridgeStdoutClean: в канал и в stdout процесса не попадает ничего, кроме строк JSON-RPC —
// ни при отказах, ни при pending: посредник копирует этот поток клиенту байт в байт.
func TestBridgeStdoutClean(t *testing.T) {
	old := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pw
	restore := func() { os.Stdout = old }
	defer restore()

	hold := make(chan struct{})
	r, _ := editRig(t, askOpts(newClock()), func(e *editFake) {
		e.ask = func(ctx context.Context, _ AskReq) (AskRes, error) {
			select {
			case <-hold:
			case <-ctx.Done():
			}
			return AskRes{Why: WhyGone}, nil
		}
	})
	c := raw(t, r.sock)
	br := handshake(t, c)
	calls := []string{
		`{"name":"create_album","arguments":{}}`,
		`{"name":"autobuild","arguments":{"album":{"key":"x","rev":1}}}`,
		`{"name":"await_answer","arguments":{"ask":"нет"}}`,
		`{"name":"vary_layouts","arguments":{"junk":1}}`,
		`{"name":"undo_last_edit","arguments":{"album":{"key":"x","rev":1}}}`,
		`{"name":"show_spread","arguments":{"album":{"key":"x"},"spread":1}}`,
	}
	for i, p := range calls {
		line := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":%s}`+"\n", i+10, p)
		if _, err := c.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	_ = c.SetReadDeadline(time.Now().Add(testkit.Limit(10 * time.Second)))
	for range calls {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("ответ: %v", err)
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil || m["jsonrpc"] != "2.0" || m["id"] == nil {
			t.Fatalf("в канал попала не строка JSON-RPC: %q", line)
		}
	}
	close(hold)
	restore()
	_ = pw.Close()
	if b, _ := io.ReadAll(bufio.NewReader(pr)); len(b) != 0 {
		t.Fatalf("в stdout процесса записано: %q", b)
	}
}
