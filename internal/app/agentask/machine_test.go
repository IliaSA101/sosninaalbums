package agentask

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// rig — машина с журналом onDone.
type rig struct {
	m    *Machine[string]
	mu   sync.Mutex
	done []string // «id:why»
}

func newRig(life, exitWait time.Duration) *rig {
	r := &rig{}
	r.m = New[string](life, exitWait, func(id, why string) {
		r.mu.Lock()
		r.done = append(r.done, id+":"+why)
		r.mu.Unlock()
	})
	return r
}

func (r *rig) log() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.done...)
}

func post(t *testing.T, m *Machine[string], id, kind string) *Rec[string] {
	t.Helper()
	rec := &Rec[string]{ID: id, Kind: kind, Payload: "p-" + id}
	if _, err := m.Post(rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestPostBusyAndPrev(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	a := post(t, r.m, "a", "add")
	if prev, err := r.m.Post(&Rec[string]{ID: "b"}); !errors.Is(err, ErrBusy) || prev != nil {
		t.Fatalf("второй вопрос: %v %v", prev, err)
	}
	if !r.m.FinishIf(a, Any, Outcome{Res: agents.AskRes{Why: agents.WhyAnswered}}, agents.WhyAnswered) {
		t.Fatal("не закрылся")
	}
	b := &Rec[string]{ID: "b", Kind: "add"}
	prev, err := r.m.Post(b)
	if err != nil || prev == nil {
		t.Fatalf("после итога: %v %v", prev, err)
	}
	select {
	case <-prev: // onDone прежнего вопроса уже вернулся
	default:
		t.Fatal("prev не закрыт после onDone")
	}
}

func TestPeek(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	if _, ok := r.m.Peek("a", ""); ok {
		t.Fatal("вопроса нет, а Peek нашёл")
	}
	rec := post(t, r.m, "a", "edit")
	for _, tc := range []struct{ id, kind string }{{"a", ""}, {"a", "edit"}} {
		if got, ok := r.m.Peek(tc.id, tc.kind); !ok || got != rec || got.Payload != "p-a" {
			t.Errorf("Peek(%q,%q) = %v %v", tc.id, tc.kind, got, ok)
		}
	}
	for _, tc := range []struct{ id, kind string }{{"b", ""}, {"", ""}, {"a", "add"}, {"aa", ""}} {
		if _, ok := r.m.Peek(tc.id, tc.kind); ok {
			t.Errorf("Peek(%q,%q) нашёл лишнее", tc.id, tc.kind)
		}
	}
	if _, err := r.m.Claim("a", "edit", true, true); err != nil { // choosing: Peek только asked
		t.Fatal(err)
	}
	if _, ok := r.m.Peek("a", ""); ok {
		t.Error("Peek видит вопрос в choosing")
	}
}

func TestClaim(t *testing.T) {
	cases := []struct {
		name      string
		id, kind  string
		yes, on   bool
		wantRec   bool
		wantErr   error
		wantWhy   string // "" — вопрос остался asked / choosing
		wantState State
	}{
		{"да", "a", "add", true, true, true, nil, "", Choosing},
		{"отказ", "a", "add", false, true, false, nil, agents.WhyDeclined, Any},
		{"отказ другого вида", "a", "create", false, true, false, nil, agents.WhyDeclined, Any},
		{"да, разрешение снято", "a", "add", true, false, false, nil, agents.WhyOff, Any},
		{"вид не тот", "a", "create", true, true, false, ErrClosed, "", Asked},
		{"чужой id", "x", "add", true, true, false, ErrClosed, "", Asked},
		{"пустой id", "", "add", false, true, false, ErrClosed, "", Asked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(time.Minute, time.Second)
			post(t, r.m, "a", "add")
			rec, err := r.m.Claim(tc.id, tc.kind, tc.yes, tc.on)
			if (rec != nil) != tc.wantRec || !errors.Is(err, tc.wantErr) {
				t.Fatalf("rec %v, err %v", rec, err)
			}
			if st, _ := r.m.Cur(); st != tc.wantState {
				t.Errorf("состояние %v, ждали %v", st, tc.wantState)
			}
			if got := r.log(); tc.wantWhy == "" && len(got) != 0 || tc.wantWhy != "" && (len(got) != 1 || got[0] != "a:"+tc.wantWhy) {
				t.Errorf("onDone: %v", got)
			}
		})
	}
}

func TestClaimTwice(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	post(t, r.m, "a", "add")
	rec, err := r.m.Claim("a", "add", true, true)
	if err != nil || rec == nil {
		t.Fatal(rec, err)
	}
	if _, err := r.m.Claim("a", "add", true, true); !errors.Is(err, ErrClosed) {
		t.Fatalf("повторный ответ в choosing: %v", err)
	}
	if !r.m.Begin(rec) {
		t.Fatal("Begin в choosing")
	}
	r.m.Release(rec)
}

func TestFinishIfFrom(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	rec := post(t, r.m, "a", "add")
	o := Outcome{Res: agents.AskRes{Why: agents.WhyGone}}
	if _, err := r.m.Claim("a", "add", true, true); err != nil {
		t.Fatal(err)
	}
	if r.m.FinishIf(rec, Asked, o, agents.WhyGone) { // gone и срок не прерывают выбор
		t.Fatal("FinishIf(Asked) закрыл choosing")
	}
	if r.m.Begin(&Rec[string]{}) {
		t.Fatal("Begin чужой записи")
	}
	if !r.m.FinishIf(rec, Any, o, agents.WhyGone) || r.m.FinishIf(rec, Any, o, agents.WhyGone) {
		t.Fatal("Any: первый раз true, второй false")
	}
	if r.m.Begin(rec) {
		t.Error("Begin после итога")
	}
	if got := r.log(); len(got) != 1 {
		t.Errorf("onDone: %v", got)
	}
}

func TestTimeout(t *testing.T) {
	r := newRig(20*time.Millisecond, time.Second)
	rec := post(t, r.m, "a", "add")
	got := r.m.Wait(context.Background(), rec)
	if got.Res.Why != agents.WhyTimeout || got.Err != nil {
		t.Fatalf("итог: %+v", got)
	}
	if st, open := r.m.Cur(); open || st != Any {
		t.Errorf("вопрос остался: %v %v", st, open)
	}
	// choosing срок не трогает
	r = newRig(20*time.Millisecond, time.Second)
	post(t, r.m, "a", "add")
	rec, err := r.m.Claim("a", "add", true, true)
	if err != nil || rec == nil {
		t.Fatal(rec, err)
	}
	time.Sleep(80 * time.Millisecond)
	if st, _ := r.m.Cur(); st != Choosing {
		t.Errorf("срок прервал choosing: %v", st)
	}
	r.m.Release(rec)
}

func TestWaitGone(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	rec := post(t, r.m, "a", "add")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := r.m.Wait(ctx, rec); got.Res.Why != agents.WhyGone {
		t.Fatalf("итог: %+v", got)
	}
	// в choosing ctx итог не отдаёт: ждёт ответа человека или выхода
	r = newRig(time.Minute, time.Second)
	rec = post(t, r.m, "a", "add")
	if _, err := r.m.Claim("a", "add", true, true); err != nil {
		t.Fatal(err)
	}
	res := make(chan Outcome, 1)
	go func() { res <- r.m.Wait(ctx, rec) }()
	select {
	case o := <-res:
		t.Fatalf("Wait вернулся в choosing: %+v", o)
	case <-time.After(50 * time.Millisecond):
	}
	r.m.FinishIf(rec, Any, Outcome{Err: errors.New("e")}, agents.WhyGone)
	if o := <-res; o.Err == nil {
		t.Errorf("итог: %+v", o)
	}
}

func TestClose(t *testing.T) {
	r := newRig(time.Minute, 30*time.Millisecond)
	r.m.Close() // вопроса нет — не паникует
	rec := post(t, r.m, "a", "add")
	r.m.Close()
	if got := r.m.Wait(context.Background(), rec); got.Res.Why != agents.WhyClosed {
		t.Fatalf("итог: %+v", got)
	}
	// choosing: выход ждёт Release, не дольше exitWait, выбор после начала выхода не применяется
	r = newRig(time.Minute, 5*time.Second)
	rec = post(t, r.m, "a", "add")
	cl, err := r.m.Claim("a", "add", true, true)
	if err != nil || cl == nil {
		t.Fatal(cl, err)
	}
	closed := make(chan struct{})
	go func() { r.m.Close(); close(closed) }()
	deadline := time.Now().Add(5 * time.Second)
	for r.m.Begin(rec) {
		if time.Now().After(deadline) {
			t.Fatal("выход не начался")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-closed:
		t.Fatal("Close не дождался Release")
	default:
	}
	r.m.Release(cl)
	<-closed
	if got := r.m.Wait(context.Background(), rec); got.Res.Why != agents.WhyClosed {
		t.Fatalf("итог: %+v", got)
	}
	// зависшее окно выход не держит
	r = newRig(time.Minute, 20*time.Millisecond)
	post(t, r.m, "a", "add")
	if _, err := r.m.Claim("a", "add", true, true); err != nil {
		t.Fatal(err)
	}
	r.m.Close()
	if got := r.log(); len(got) != 1 || got[0] != "a:"+agents.WhyClosed {
		t.Errorf("onDone: %v", got)
	}
}

// Параллельные ответы: ровно один закрывает вопрос, остальные получают ErrClosed или nil без второго итога.
func TestFinishRace(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	rec := post(t, r.m, "a", "add")
	var wg sync.WaitGroup
	won := make(chan bool, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			won <- r.m.FinishIf(rec, Any, Outcome{}, agents.WhyAnswered)
		}()
	}
	wg.Wait()
	close(won)
	n := 0
	for w := range won {
		if w {
			n++
		}
	}
	if n != 1 || len(r.log()) != 1 {
		t.Errorf("закрыли %d раз, onDone %v", n, r.log())
	}
}

// Порядок завершения: итог ждущему → onDone → закрытие prev следующего Post. Пока onDone держится, Wait
// уже вернул итог, запись снята (новый Post проходит), но prev ещё открыт.
func TestFinishOrder(t *testing.T) {
	inDone, release := make(chan struct{}), make(chan struct{})
	m := New[string](time.Minute, time.Second, func(string, string) {
		close(inDone)
		<-release
	})
	a := post(t, m, "a", "add")
	finished := make(chan bool, 1)
	go func() {
		finished <- m.FinishIf(a, Any, Outcome{Res: agents.AskRes{Why: agents.WhyAnswered}}, agents.WhyAnswered)
	}()
	select {
	case <-inDone:
	case <-time.After(5 * time.Second):
		t.Fatal("onDone не вызван")
	}
	waited := make(chan Outcome, 1)
	go func() { waited <- m.Wait(context.Background(), a) }()
	select {
	case o := <-waited:
		if o.Res.Why != agents.WhyAnswered {
			t.Fatalf("итог: %+v", o)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait не получил итог, пока onDone держится")
	}
	prev, err := m.Post(&Rec[string]{ID: "b", Kind: "add"})
	if err != nil || prev == nil {
		t.Fatalf("Post при висящем onDone: %v %v", prev, err)
	}
	select {
	case <-prev:
		t.Fatal("prev закрыт до возврата onDone")
	case <-finished:
		t.Fatal("FinishIf вернулся до возврата onDone")
	default:
	}
	close(release)
	select {
	case <-prev:
	case <-time.After(5 * time.Second):
		t.Fatal("prev не закрылся после onDone")
	}
	if !<-finished {
		t.Fatal("FinishIf вернул false")
	}
}

// Запись без Post (нулевое State равно Asked) не виснет: FinishIf — false, Wait — ErrClosed; отказанная
// ErrBusy запись тоже не заведена.
func TestUnposted(t *testing.T) {
	r := newRig(time.Minute, time.Second)
	free := &Rec[string]{ID: "x"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, from := range []State{Asked, Any} {
			if r.m.FinishIf(free, from, Outcome{}, "why") {
				t.Errorf("FinishIf(%v) закрыл незаведённую запись", from)
			}
		}
		if o := r.m.Wait(context.Background(), free); !errors.Is(o.Err, ErrClosed) {
			t.Errorf("Wait: %+v", o)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("вызов на записи без Post завис")
	}
	cur := post(t, r.m, "a", "add")
	busy := &Rec[string]{ID: "b"}
	if _, err := r.m.Post(busy); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if r.m.FinishIf(busy, Any, Outcome{}, "why") || len(r.log()) != 0 {
		t.Fatal("отказанная запись закрылась или дала onDone")
	}
	if !r.m.FinishIf(cur, Any, Outcome{}, "why") {
		t.Fatal("заведённая запись не закрылась")
	}
}
