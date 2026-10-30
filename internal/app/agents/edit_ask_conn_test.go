package agents

import (
	"runtime"
	"testing"
	"time"
)

// Присмотр за подключением один на подключение, а не на вопрос: после сотни вопросов подряд число
// горутин не растёт, а запись присмотра пуста (итог каждого снял вопрос).
func TestAskWatcherNoGoroutineGrowth(t *testing.T) {
	o := askOpts(newClock())
	o.Rate = 1000
	r, cs := editRig(t, o, func(e *editFake) { e.ask = answerWith(WhyAnswered) })
	okReply(t, tcall(t, cs, "create_album", nil)) // присмотр за подключением заведён
	base := runtime.NumGoroutine()
	for i := range 100 {
		okReply(t, tcall(t, cs, "create_album", nil))
		if len(r.svc.ed.slot) != 0 { // место свободно сразу после ответа, без допусков
			t.Fatalf("вопрос %d: место правки занято после ответа", i)
		}
	}
	waitFor(t, "вопросы сняты с присмотра", func() bool {
		r.svc.ed.mu.Lock()
		defer r.svc.ed.mu.Unlock()
		n := 0
		for _, set := range r.svc.ed.asks {
			n += len(set)
		}
		return n == 0
	})
	// Допуск на фоновые горутины SDK и тестов; на горутину-на-вопрос прирост был бы около сотни.
	for end := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > base+8 && time.Now().Before(end); {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > base+8 {
		t.Fatalf("горутин было %d, стало %d после 100 вопросов", base, n)
	}
}

// Итог вопроса виден агенту только после того, как место правки и присмотр сняты: иначе следующая
// просьба сразу после ответа получает ложное «занято». Проверка без гонки: release вызывается из
// finishAsk до close(done), поэтому наблюдатель done обязан увидеть флаг.
func TestFinishAskReleasesBeforeDone(t *testing.T) {
	r, _ := editRig(t, askOpts(newClock()), nil)
	for _, why := range []string{WhyAnswered, WhyDeclined, WhyTimeout, WhyGone} {
		rel, ok := r.svc.ed.takeSlot()
		if !ok {
			t.Fatal("место правки занято до вопроса")
		}
		p := &pend{done: make(chan struct{}), cancel: func() {}}
		r.svc.finishAsk(askRun{p: p, release: rel}, AskRes{Why: why}, nil, false)
		<-p.done
		if len(r.svc.ed.slot) != 0 {
			t.Fatalf("%s: место правки занято в момент готового итога", why)
		}
	}
}
