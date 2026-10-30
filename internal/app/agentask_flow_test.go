package app

// Машина вопроса агента (149, design §3, 0.33-149-24): asked → choosing → done.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// В choosing срок стоит: окно ОС держится дольше AskLife, итог — «да», а не timeout.
func TestAskChoosingState(t *testing.T) {
	r := newAskRig(t)
	r.a.askM().life = 60 * time.Millisecond
	release := r.holdDialog()
	defer release()
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	if r.state() != askAsked {
		t.Fatalf("состояние %v, ждали asked", r.state())
	}
	ans := r.answerAsync(id, true)
	r.dialogOpen(t)
	waitUntil(t, "choosing", func() bool { return r.state() == askChoosing })
	time.Sleep(250 * time.Millisecond) // в четыре раза дольше срока
	if !c.pending(0) || len(r.evOf("agents:askdone")) != 0 {
		t.Fatalf("срок прервал выбор папки: %+v %v", c.res, r.evOf("agents:askdone"))
	}
	// Второй ответ в choosing — вопрос уже не asked.
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) {
		t.Errorf("второй ответ: %v", err)
	}
	if _, err := r.a.AnswerAgent(id, false); !errors.Is(err, errAskClosed) {
		t.Errorf("отказ в choosing: %v", err)
	}
	release()
	ans.wait(t)
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if d := r.done(t, 1); d.Why != agents.WhyAnswered || d.ID != id {
		t.Errorf("agents:askdone: %+v", d)
	}
}

// gone (отмена вызова агента) выбор папки не прерывает: итог — выбор человека.
func TestAskFrozenWhileDialog(t *testing.T) {
	r := newAskRig(t)
	release := r.holdDialog()
	defer release()
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	ans := r.answerAsync(id, true)
	r.dialogOpen(t)
	waitUntil(t, "choosing", func() bool { return r.state() == askChoosing })
	c.cancel()
	if !c.pending(150 * time.Millisecond) {
		t.Fatalf("gone прервал выбор: %+v", c.res)
	}
	if n := len(r.evOf("agents:askdone")); n != 0 {
		t.Fatalf("окно закрыто при выборе папки: %d", n)
	}
	release()
	ans.wait(t)
	c.wait(t)
	if c.res.Why != agents.WhyAnswered || c.err != nil {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if got := r.a.editor().View().Folders; len(got) != 2 {
		t.Errorf("папка человека не добавлена: %v", got)
	}
}

// Флажок выключен: до окна — off без окна; после окна, до окна ОС и после окна ОС — off без правки.
func TestAskFlagOff(t *testing.T) {
	r := newAskRig(t)
	setAgentsEdit(r.a, false)
	c := r.ask(r.addReq())
	c.wait(t)
	if c.res.Why != agents.WhyOff || c.err != nil || len(r.evOf("agents:ask")) != 0 || r.fronted() != 0 {
		t.Fatalf("флажок выключен до вопроса: %+v %v, окон %d", c.res, c.err, len(r.evOf("agents:ask")))
	}

	// Выключили, пока вопрос висел: «Выбрать папку…» → off, окно ОС не открывается.
	setAgentsEdit(r.a, true)
	c = r.ask(r.addReq())
	id := r.asked(t, 1).ID
	setAgentsEdit(r.a, false)
	if _, err := r.a.AnswerAgent(id, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyOff || len(r.titles) != 0 {
		t.Fatalf("выключили до окна ОС: %+v, окон ОС %d", c.res, len(r.titles))
	}

	// Выключили, пока открыто окно ОС: выбор не применяется.
	setAgentsEdit(r.a, true)
	release := r.holdDialog()
	defer release()
	c = r.ask(r.addReq())
	id = r.asked(t, 2).ID
	ans := r.answerAsync(id, true)
	r.dialogOpen(t)
	setAgentsEdit(r.a, false)
	release()
	ans.wait(t)
	c.wait(t)
	if c.res.Why != agents.WhyOff {
		t.Fatalf("выключили в окне ОС: %+v", c.res)
	}
	if got := r.a.editor().View().Folders; len(got) != 1 {
		t.Errorf("папка добавлена при выключенном флажке: %v", got)
	}
	if d := r.done(t, 2); d.Why != agents.WhyOff {
		t.Errorf("agents:askdone: %+v", d)
	}
}

// Вид «да» не тот: create-вопрос не принимает AnswerAgent(true), add — AgentAnswerCreate; вопрос
// остаётся открытым, окно ОС не открывается. Отказ AnswerAgent(false) закрывает вопрос любого вида.
func TestAskKindMismatch(t *testing.T) {
	r := newAskRig(t)
	before := r.rev()
	c := r.ask(r.createReq())
	id := r.asked(t, 1).ID
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) {
		t.Fatalf("add-ответ на create: %v", err)
	}
	if r.state() != askAsked || len(r.titles) != 0 {
		t.Fatalf("вопрос тронут: %v, окон ОС %d", r.state(), len(r.titles))
	}
	if _, err := r.a.AnswerAgent(id, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyDeclined {
		t.Fatalf("отказ create: %+v", c.res)
	}

	c = r.ask(r.addReq())
	id = r.asked(t, 2).ID
	if _, err := r.a.AgentAnswerCreate(id, false); !errors.Is(err, errAskClosed) {
		t.Fatalf("create-ответ на add: %v", err)
	}
	if r.state() != askAsked || len(r.titles) != 0 || r.rev() != before {
		t.Fatalf("вопрос или альбом тронуты: %v", r.state())
	}
	if _, err := r.a.AnswerAgent(id, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
}

// «Да» для add: папка в тот же альбом одним шагом, итог с ключом, номером и признаками.
func TestAskYes(t *testing.T) {
	r := newAskRig(t)
	before, def := r.rev(), r.a.dialogDir("photos") // после «да» папка запоминается и умолчание сдвигается
	c := r.ask(r.addReq())
	ev := r.asked(t, 1)
	if ev.Kind != agents.AskAdd || ev.Hint != "" || ev.OK == "" || len(ev.Lines) == 0 || !strings.Contains(ev.Lines[0], "Claude Code") {
		t.Errorf("agents:ask: %+v", ev)
	}
	if r.fronted() != 1 {
		t.Errorf("окно вперёд: %d раз", r.fronted())
	}
	v, err := r.a.AnswerAgent(ev.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || c.res.Merged || c.res.Folder != 2 {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if c.res.Album.Key != r.key() || c.res.Album.Rev != r.rev() || c.res.Album.Rev <= before {
		t.Errorf("ссылка на альбом: %+v, rev %d", c.res.Album, r.rev())
	}
	if len(v.Folders) != 2 || v.Rev != c.res.Album.Rev {
		t.Errorf("вид: папок %d, rev %d", len(v.Folders), v.Rev)
	}
	if !strings.Contains(r.titles[0], "Claude Code") || r.defs[0] != def {
		t.Errorf("окно ОС: %q, %q", r.titles[0], r.defs[0])
	}
	// Та же папка ещё раз: уже в альбоме — Merged, правки нет.
	rev := r.rev()
	c = r.ask(r.addReq())
	if _, err := r.a.AnswerAgent(r.asked(t, 2).ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if !c.res.Merged || c.res.Folder != 2 || r.rev() != rev || c.res.Album.Rev != rev {
		t.Errorf("повтор: %+v, rev %d→%d", c.res, rev, r.rev())
	}
}

// Отказ: кнопка, отмена окна ОС и сбой окна — declined, альбом цел.
func TestAskDenied(t *testing.T) {
	r := newAskRig(t)
	rev := r.rev()
	c := r.ask(r.addReq())
	if _, err := r.a.AnswerAgent(r.asked(t, 1).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyDeclined || c.err != nil {
		t.Fatalf("«Отмена»: %+v, %v", c.res, c.err)
	}

	for i, set := range []func(){
		func() { r.mu.Lock(); r.picked = ""; r.mu.Unlock() },                       // отмена окна ОС
		func() { r.mu.Lock(); r.pickErr = errors.New("сбой окна"); r.mu.Unlock() }, // сбой окна ОС
	} {
		set()
		c = r.ask(r.addReq())
		if _, err := r.a.AnswerAgent(r.asked(t, i+2).ID, true); err != nil {
			t.Fatal(err)
		}
		c.wait(t)
		if c.res.Why != agents.WhyDeclined || c.err != nil {
			t.Fatalf("окно ОС, случай %d: %+v, %v", i, c.res, c.err)
		}
	}
	if r.rev() != rev || len(r.a.editor().View().Folders) != 1 {
		t.Error("отказ изменил альбом")
	}
	if n := len(r.evOf("agents:askdone")); n != 3 {
		t.Errorf("agents:askdone: %d", n)
	}
}

// Срок asked вышел: timeout, окно закрыто, поздний ответ — «вопрос закрыт».
func TestAskTimeout(t *testing.T) {
	r := newAskRig(t)
	r.a.askM().life = 30 * time.Millisecond
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	c.wait(t)
	if c.res.Why != agents.WhyTimeout || c.err != nil {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if d := r.done(t, 1); d.Why != agents.WhyTimeout || d.ID != id {
		t.Errorf("agents:askdone: %+v", d)
	}
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) {
		t.Errorf("поздний ответ: %v", err)
	}
	if len(r.titles) != 0 || r.state() != askAny {
		t.Errorf("окно ОС %d, состояние %v", len(r.titles), r.state())
	}
}

// Один вопрос за раз; не тот ключ, ревизия или не готовое окно — ошибки службы.
func TestAskBusy(t *testing.T) {
	r := newAskRig(t)
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	if res, err := r.a.agentAsk(t.Context(), r.createReq()); !errors.Is(err, agents.ErrAskBusy) || res.Why != "" {
		t.Fatalf("второй вопрос: %+v, %v", res, err)
	}
	if n := len(r.evOf("agents:ask")); n != 1 {
		t.Errorf("второе окно: %d", n)
	}
	if _, err := r.a.AnswerAgent(id, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)

	req := r.addReq()
	req.Rev++
	if _, err := r.a.agentAsk(t.Context(), req); !errors.Is(err, agents.ErrAlbumEdited) {
		t.Errorf("не та ревизия: %v", err)
	}
	req = r.addReq()
	req.Key += "x"
	if _, err := r.a.agentAsk(t.Context(), req); !errors.Is(err, agents.ErrAlbumChanged) {
		t.Errorf("не тот ключ: %v", err)
	}
	if r.state() != askAny || len(r.evOf("agents:ask")) != 1 {
		t.Error("отказанный вопрос оставил следы")
	}

	b := NewApp() // startup не закончен
	if _, err := b.agentAsk(t.Context(), r.addReq()); !errors.Is(err, agents.ErrNotReady) {
		t.Errorf("окно не готово: %v", err)
	}
	if _, err := r.a.agentAsk(t.Context(), agents.AskReq{Kind: "x"}); err == nil {
		t.Error("неизвестный вид принят")
	}
}

// Агент ушёл (ctx отменён) до ответа: gone, окно закрыто, можно спрашивать снова.
func TestAskClientGone(t *testing.T) {
	r := newAskRig(t)
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	c.cancel()
	c.wait(t)
	if c.res.Why != agents.WhyGone || c.err != nil {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if d := r.done(t, 1); d.Why != agents.WhyGone || d.ID != id {
		t.Errorf("agents:askdone: %+v", d)
	}
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) {
		t.Errorf("ответ после gone: %v", err)
	}
	c = r.ask(r.addReq())
	r.asked(t, 2)
	if _, err := r.a.AnswerAgent(r.evOf("agents:ask")[1].(AgentAsk).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
}

// Чужой, пустой, огромный и закрытый id — «вопрос закрыт»; ничего не меняется и не паникует.
func TestAskStaleID(t *testing.T) {
	r := newAskRig(t)
	rev := r.rev()
	for _, id := range []string{"", "нет", strings.Repeat("я", 1<<16)} {
		for _, yes := range []bool{false, true} {
			if _, err := r.a.AnswerAgent(id, yes); !errors.Is(err, errAskClosed) { // вопроса нет совсем
				t.Errorf("AnswerAgent(%.5q, %v) без вопроса: %v", id, yes, err)
			}
		}
		if _, err := r.a.AgentAnswerCreate(id, false); !errors.Is(err, errAskClosed) {
			t.Errorf("AgentAnswerCreate(%.5q) без вопроса: %v", id, err)
		}
	}
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	for _, bad := range []string{"", id[:len(id)-1], id + "0", strings.ToUpper(id) + "!", strings.Repeat("я", 1<<16)} {
		if _, err := r.a.AnswerAgent(bad, true); !errors.Is(err, errAskClosed) {
			t.Errorf("чужой id %.8q: %v", bad, err)
		}
	}
	if r.state() != askAsked || len(r.titles) != 0 {
		t.Fatal("чужой id тронул вопрос")
	}
	if len(id) != 32 || id == strings.Repeat("0", 32) {
		t.Errorf("id %q: ждали 16 байт hex", id)
	}
	if _, err := r.a.AnswerAgent(id, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) { // уже закрыт
		t.Errorf("закрытый вопрос: %v", err)
	}
	if r.rev() != rev {
		t.Error("альбом изменился")
	}
}

// Выход: вопрос снимается (closed) до остановки службы, окно закрывается, новые вопросы — closed.
// В choosing выход ждёт окно ОС (но не дольше exitWait); выбор после начала выхода не применяется.
func TestAskShutdown(t *testing.T) {
	r := newAskRig(t)
	c := r.ask(r.addReq())
	id := r.asked(t, 1).ID
	r.a.stopAgents()
	c.wait(t)
	if c.res.Why != agents.WhyClosed || c.err != nil {
		t.Fatalf("asked: %+v, %v", c.res, c.err)
	}
	if d := r.done(t, 1); d.Why != agents.WhyClosed || d.ID != id {
		t.Errorf("agents:askdone: %+v", d)
	}
	if _, err := r.a.AnswerAgent(id, true); !errors.Is(err, errAskClosed) {
		t.Errorf("ответ после выхода: %v", err)
	}

	// choosing: выход ждёт возврата окна ОС; выбор не применяется.
	r2 := newAskRig(t)
	r2.a.askM().exitWait = 5 * time.Second
	release := r2.holdDialog()
	defer release()
	c = r2.ask(r2.addReq())
	id = r2.asked(t, 1).ID
	ans := r2.answerAsync(id, true)
	r2.dialogOpen(t)
	waitUntil(t, "choosing", func() bool { return r2.state() == askChoosing })
	exited := make(chan struct{})
	go func() { r2.a.stopAgents(); close(exited) }()
	select {
	case <-exited:
		t.Fatal("выход не дождался окна ОС")
	case <-time.After(150 * time.Millisecond):
	}
	if !c.pending(0) {
		t.Fatalf("вызов вернулся до закрытия окна ОС: %+v", c.res)
	}
	release()
	<-exited
	ans.wait(t)
	c.wait(t)
	if c.res.Why != agents.WhyClosed || len(r2.a.editor().View().Folders) != 1 {
		t.Errorf("choosing: %+v, папок %d", c.res, len(r2.a.editor().View().Folders))
	}

	// Окно ОС зависло: выход ждёт не дольше exitWait.
	r3 := newAskRig(t)
	hang := r3.holdDialog()
	defer hang()
	c = r3.ask(r3.addReq())
	id = r3.asked(t, 1).ID
	ans = r3.answerAsync(id, true)
	r3.dialogOpen(t)
	waitUntil(t, "choosing", func() bool { return r3.state() == askChoosing })
	start := time.Now()
	r3.a.stopAgents()
	if time.Since(start) < 250*time.Millisecond {
		t.Errorf("выход не ждал окно ОС: %v", time.Since(start))
	}
	c.wait(t)
	if c.res.Why != agents.WhyClosed {
		t.Errorf("зависшее окно: %+v", c.res)
	}
	hang()
	ans.wait(t)

	// Программа закрывается: новый вопрос — closed, окна нет.
	r.a.stopped.Store(true)
	c = r.ask(r.addReq())
	c.wait(t)
	if c.res.Why != agents.WhyClosed || len(r.evOf("agents:ask")) != 1 {
		t.Errorf("вопрос при выходе: %+v", c.res)
	}
}

// Итог отдан ждущему вызову — следующий вопрос уже можно задать: запись вопроса снимается до
// отправки итога, а не после события askdone (иначе агент, получив ответ, ловил ErrAskBusy).
func TestAskNextAfterDone(t *testing.T) {
	r := newAskRig(t)
	tokens := make(chan struct{}, 3)
	rec := r.a.emit
	r.a.emit = func(name string, data ...any) {
		rec(name, data...)
		if name == "agents:askdone" {
			<-tokens // закрытие вопроса застряло на событии окна
		}
	}
	t.Cleanup(func() { close(tokens) })
	for i := 1; i <= 3; i++ {
		c := r.ask(r.addReq())
		id := r.asked(t, i).ID
		go func() { _, _ = r.a.AnswerAgent(id, false) }()
		c.wait(t)
		if c.err != nil || c.res.Why != agents.WhyDeclined {
			t.Fatalf("вопрос %d: %+v, %v", i, c.res, c.err)
		}
		if r.state() != askAny {
			t.Fatalf("вопрос %d: итог отдан, а запись вопроса жива (состояние %v)", i, r.state())
		}
		tokens <- struct{}{}
		waitUntil(t, "askdone", func() bool { return len(r.evOf("agents:askdone")) >= i })
	}
}
