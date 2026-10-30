package library

import (
	"context"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// waitFor ждёт условие под s.mu (ожидающие встали в список); не дождались — t.Fatal.
func waitFor(t *testing.T, s *sched, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(5 * time.Second))
	for {
		s.mu.Lock()
		ok := cond()
		s.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// grabbed — место выдано ждущему: имя и его release.
type grabbed struct {
	name    string
	release func()
}

// goAcquire ставит ждущего name; выданное место приходит в out.
func goAcquire(ctx context.Context, s *sched, p prio, name string, out chan<- grabbed) {
	go func() {
		rel, err := s.acquire(ctx, p)
		if err != nil {
			out <- grabbed{name: name + ":" + err.Error()}
			return
		}
		out <- grabbed{name, rel}
	}()
}

func next(t *testing.T, out <-chan grabbed) grabbed {
	t.Helper()
	select {
	case g := <-out:
		return g
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("место никому не выдано")
		return grabbed{}
	}
}

// nobody — место пока никому не выдано.
func nobody(t *testing.T, out <-chan grabbed) {
	t.Helper()
	select {
	case g := <-out:
		t.Fatalf("место выдано %s раньше срока", g.name)
	case <-time.After(30 * time.Millisecond):
	}
}

// Пределы: ноль мест не бывает, прогреву не больше общего.
func TestSchedLimits(t *testing.T) {
	for _, c := range []struct{ total, bgMax, wantT, wantB int }{
		{0, 0, 1, 1}, {2, 5, 2, 2}, {12, 6, 12, 6}, {1, -3, 1, 1},
	} {
		s := newSched(c.total, c.bgMax)
		if s.total != c.wantT || s.bgMax != c.wantB {
			t.Errorf("newSched(%d,%d) = %d/%d, ждали %d/%d", c.total, c.bgMax, s.total, s.bgMax, c.wantT, c.wantB)
		}
	}
}

// Мест — две трети ядер, не меньше двух: на 12 ядрах 4 P свободны для правки и View (090M-D1).
func TestPlacesFor(t *testing.T) {
	for _, c := range []struct{ cpu, want int }{{12, 8}, {2, 2}, {1, 2}, {4, 2}, {0, 2}, {6, 4}, {13, 8}, {24, 8}, {32, 8}} {
		if got := placesFor(c.cpu); got != c.want {
			t.Errorf("placesFor(%d) = %d, ждали %d", c.cpu, got, c.want)
		}
	}
}

// Очередь хвостов — два на ядро, не больше 24: предел памяти постоянный (090M-D4).
func TestTailsFor(t *testing.T) {
	for _, c := range []struct{ cpu, want int }{{0, 0}, {1, 2}, {6, 12}, {12, 24}, {13, 24}, {32, 24}} {
		if got := tailsFor(c.cpu); got != c.want {
			t.Errorf("tailsFor(%d) = %d, ждали %d", c.cpu, got, c.want)
		}
	}
}

// Ждут запрос окна и прогрев — освободившееся место получает окно, хотя прогрев встал раньше.
func TestSchedNowBeforeBg(t *testing.T) {
	s := newSched(1, 1)
	hold, err := s.acquire(context.Background(), bg)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan grabbed, 4)
	goAcquire(context.Background(), s, bg, "bg1", out)
	waitFor(t, s, "bg1 ждёт", func() bool { return len(s.bgWait) == 1 })
	goAcquire(context.Background(), s, now, "now1", out)
	waitFor(t, s, "now1 ждёт", func() bool { return len(s.nowWait) == 1 })
	hold()
	hold() // повторный release — ничего: место не отдаётся дважды
	g := next(t, out)
	if g.name != "now1" {
		t.Fatalf("первым место получил %s, ждали now1", g.name)
	}
	nobody(t, out) // прогрев не входит, пока идёт запрос окна
	g.release()
	if g := next(t, out); g.name != "bg1" {
		t.Fatalf("после окна место получил %s, ждали bg1", g.name)
	} else {
		g.release()
	}
	if s.run != [2]int{} {
		t.Errorf("занято после всех release: %v", s.run)
	}
}

// Из ждущих запросов окна первым идёт самый новый (090-D1); прогрев — по порядку прихода.
func TestSchedNowNewestFirst(t *testing.T) {
	for _, p := range []prio{now, bg} {
		s := newSched(1, 1)
		hold, err := s.acquire(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		out := make(chan grabbed, 3)
		names := []string{"a", "b", "c"}
		for i, n := range names {
			goAcquire(context.Background(), s, p, n, out)
			waitFor(t, s, n+" ждёт", func() bool { return len(s.nowWait)+len(s.bgWait) == i+1 })
		}
		want := []string{"c", "b", "a"}
		if p == bg {
			want = names
		}
		hold()
		for _, w := range want {
			g := next(t, out)
			if g.name != w {
				t.Fatalf("класс %d: место получил %s, ждали %s", p, g.name, w)
			}
			g.release()
		}
	}
}

// Прогрев не входит, пока занят или ждёт хоть один запрос окна, даже при свободных местах;
// своих мест у него не больше bgMax; запрос окна входит, пока есть любое общее место.
func TestSchedBgWaitsNow(t *testing.T) {
	s := newSched(4, 2)
	ctx := context.Background()
	w, err := s.acquire(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan grabbed, 4)
	goAcquire(ctx, s, bg, "bg1", out)
	waitFor(t, s, "bg1 ждёт", func() bool { return len(s.bgWait) == 1 })
	nobody(t, out) // 3 места свободны, но идёт запрос окна
	w()
	b1 := next(t, out)
	b2, err := s.acquire(ctx, bg)
	if err != nil {
		t.Fatal(err)
	}
	goAcquire(ctx, s, bg, "bg3", out)
	waitFor(t, s, "bg3 ждёт", func() bool { return len(s.bgWait) == 1 })
	nobody(t, out) // прогреву — не больше bgMax = 2
	// Окно входит рядом с идущим прогревом: 2 общих места свободны.
	n1, err := s.acquire(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := s.acquire(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	goAcquire(ctx, s, now, "now3", out)
	waitFor(t, s, "now3 ждёт", func() bool { return len(s.nowWait) == 1 })
	b1.release() // место — окну, а не прогреву, ждавшему раньше
	if g := next(t, out); g.name != "now3" {
		t.Fatalf("место получил %s, ждали now3", g.name)
	} else {
		g.release()
	}
	b2()
	n1()
	nobody(t, out) // n2 ещё идёт — прогрев ждёт
	n2()
	if g := next(t, out); g.name != "bg3" {
		t.Fatalf("место получил %s, ждали bg3", g.name)
	} else {
		g.release()
	}
	if s.run != [2]int{} || len(s.nowWait)+len(s.bgWait) != 0 {
		t.Errorf("остаток: run %v, ждут %d/%d", s.run, len(s.nowWait), len(s.bgWait))
	}
}

// Снятый запрос: ошибка ctx без обёртки, из стека убран, место не занимает; уже снятый — отказ сразу,
// даже при свободном месте. Гонка «выдали и сняли» не теряет места.
func TestSchedCancelWaiting(t *testing.T) {
	s := newSched(1, 1)
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.acquire(done, now); err != context.Canceled { // ровно ctx.Err(), без обёртки
		t.Fatalf("уже снятый: %v", err)
	}
	if s.run != [2]int{} {
		t.Fatalf("снятый занял место: %v", s.run)
	}

	hold, _ := s.acquire(context.Background(), now)
	out := make(chan grabbed, 4)
	ctxA, cancelA := context.WithCancel(context.Background())
	goAcquire(ctxA, s, now, "a", out)
	goAcquire(context.Background(), s, bg, "bg", out)
	waitFor(t, s, "a и bg ждут", func() bool { return len(s.nowWait) == 1 && len(s.bgWait) == 1 })
	cancelA()
	if g := next(t, out); g.name != "a:"+context.Canceled.Error() {
		t.Fatalf("снятый: %s", g.name)
	}
	waitFor(t, s, "стек окна пуст", func() bool { return len(s.nowWait) == 0 })
	hold()
	if g := next(t, out); g.name != "bg" { // снятый запрос окна прогрев больше не держит
		t.Fatalf("место получил %s, ждали bg", g.name)
	} else {
		g.release()
	}

	// Гонка: снятие одновременно с выдачей. Место либо отдано и возвращено, либо не выдавалось.
	for range testkit.Steps(200, 40) {
		hold, _ := s.acquire(context.Background(), now)
		ctx, cancel := context.WithCancel(context.Background())
		res := make(chan error, 1)
		go func() {
			rel, err := s.acquire(ctx, now)
			if err == nil {
				rel()
			}
			res <- err
		}()
		waitFor(t, s, "ждёт", func() bool { return len(s.nowWait) == 1 })
		go cancel()
		hold()
		<-res
		cancel()
		waitFor(t, s, "места свободны", func() bool { return s.run == [2]int{} && len(s.nowWait) == 0 })
	}
}

// Паника под местом: defer release отдаёт его, следующий входит сразу.
func TestSchedPanicFreesSlot(t *testing.T) {
	s := newSched(1, 1)
	func() {
		defer func() { _ = recover() }()
		rel, err := s.acquire(context.Background(), now)
		if err != nil {
			t.Fatal(err)
		}
		defer rel()
		panic("декодер")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(2*time.Second))
	defer cancel()
	rel, err := s.acquire(ctx, bg)
	if err != nil {
		t.Fatalf("место не вернулось после паники: %v", err)
	}
	rel()
}
