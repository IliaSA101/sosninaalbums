package app

// Подписи кадров в памяти App (078 §4.3): PrepareSigs, событие sigs:progress, отмена, infos.

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// sigApp — App с n фото ленты (t0…, размер 100+i), подменённой подписью shot и сбором событий
// sigs:progress в got.
func sigApp(t *testing.T, n int, shot func(p library.Photo) (imgsig.Sig, error)) (a *App, ids []string, got func() []SigState) {
	t.Helper()
	a = newApp(t)
	a.ed.Store(mustEditor(t, model.New(), ""))
	for i := range n {
		id := fmt.Sprint("t", i)
		a.byID[id] = library.Photo{ID: id, Path: `C:\свадьба\` + id + `.jpg`, W: 300, H: 200, Size: int64(100 + i)}
		ids = append(ids, id)
	}
	a.sig.shot = shot
	var mu sync.Mutex
	var evs []SigState
	a.emit = func(name string, data ...any) {
		if name == "sigs:progress" {
			mu.Lock()
			evs = append(evs, data[0].(SigState))
			mu.Unlock()
		}
	}
	return a, ids, func() []SigState {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(evs)
	}
}

// sigFake — подпись по размеру файла; t3 не читается.
func sigFake(calls *atomic.Int32) func(p library.Photo) (imgsig.Sig, error) {
	return func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		if p.ID == "t3" {
			return imgsig.Sig{}, errors.New("битая миниатюра")
		}
		return imgsig.Sig{D: uint64(p.Size), Color: 10, Sharp: 1, Ok: true}, nil
	}
}

// TestPrepareSigsProgress: ответ сразу — готово из памяти; остальное в фоне, ход растёт, последнее
// событие — done = total; неизвестные и повторные ID не считаются; не прочитавшееся — тоже готово
// (Ok = false, больше не пробуется); второй вызов — всё готово без работы и событий.
func TestPrepareSigsProgress(t *testing.T) {
	var calls atomic.Int32
	a, ids, got := sigApp(t, 40, sigFake(&calls))
	if st := a.PrepareSigs(append(slices.Clone(ids), "нет", ids[0])); st != (SigState{0, 40}) {
		t.Fatalf("ответ %+v", st)
	}
	a.bg.Wait()
	evs := got()
	if len(evs) == 0 || evs[len(evs)-1] != (SigState{40, 40}) {
		t.Fatalf("события %v", evs)
	}
	for i := 1; i < len(evs); i++ {
		if evs[i].Done < evs[i-1].Done || evs[i].Total != 40 {
			t.Fatalf("ход не растёт: %v", evs)
		}
	}
	if calls.Load() != 40 {
		t.Fatalf("подписей посчитано %d", calls.Load())
	}
	if st := a.PrepareSigs(ids); st != (SigState{40, 40}) || calls.Load() != 40 {
		t.Fatalf("второй вызов: %+v, подписей %d", st, calls.Load())
	}
	a.bg.Wait()
	if len(got()) != len(evs) {
		t.Fatal("второй вызов прислал события")
	}
	if st := a.PrepareSigs(nil); st != (SigState{}) {
		t.Fatalf("пустой вызов: %+v", st)
	}
}

// TestInfosCarrySig: infos кладёт подпись из памяти; не прочитавшееся — Ok = false; файл сменил
// размер — подписи нет, и PrepareSigs считает его заново.
func TestInfosCarrySig(t *testing.T) {
	var calls atomic.Int32
	a, ids, _ := sigApp(t, 6, sigFake(&calls))
	a.PrepareSigs(ids)
	a.bg.Wait()
	for _, info := range a.infos(ids) {
		if ok := info.ID != "t3"; info.Sig.Ok != ok || ok && info.Sig.D != uint64(info.Size) {
			t.Fatalf("%s: подпись %+v", info.ID, info.Sig)
		}
	}
	a.mu.Lock()
	p := a.byID["t1"]
	p.Size = 999
	a.byID["t1"] = p
	a.mu.Unlock()
	if s := a.infos([]string{"t1"})[0].Sig; s.Ok {
		t.Fatalf("файл другой, а подпись прежняя: %+v", s)
	}
	if st := a.PrepareSigs(ids); st != (SigState{5, 6}) {
		t.Fatalf("после смены размера: %+v", st)
	}
	a.bg.Wait()
	if s := a.infos([]string{"t1"})[0].Sig; !s.Ok || s.D != 999 {
		t.Fatalf("пересчитанная подпись: %+v", s)
	}
}

// TestPrepareSigsCancel: новый вызов (и пустой — окно закрыто) отменяет работу: отменённая молчит,
// а посчитанное ею остаётся в памяти.
func TestPrepareSigsCancel(t *testing.T) {
	block, started := make(chan struct{}), make(chan struct{}, 100)
	var calls atomic.Int32
	a, ids, got := sigApp(t, 300, func(p library.Photo) (imgsig.Sig, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-block
		calls.Add(1)
		return imgsig.Sig{D: 1, Ok: true}, nil
	})
	a.PrepareSigs(ids)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("работа не началась")
	}
	if st := a.PrepareSigs(nil); st != (SigState{}) {
		t.Fatalf("отмена: %+v", st)
	}
	close(block)
	a.bg.Wait()
	if evs := got(); len(evs) != 0 {
		t.Fatalf("отменённая работа прислала %v", evs)
	}
	n := int(calls.Load())
	if n == 0 || n == 300 {
		t.Fatalf("посчитано %d из 300: отмена не остановила работу или ничего не дошло", n)
	}
	if st := a.PrepareSigs(ids); st.Done != n || st.Total != 300 {
		t.Fatalf("после отмены: %+v, в памяти %d", st, n)
	}
	a.bg.Wait()
	if evs := got(); len(evs) == 0 || evs[len(evs)-1] != (SigState{300, 300}) {
		t.Fatalf("новая работа: %v", evs)
	}
}

// TestAutoSimilarFromLibrary: App отдаёт подписи в отбор: два кадра с одной подписью — один
// отброшен (план и список сходятся), без подготовки подписей — никого.
func TestAutoSimilarFromLibrary(t *testing.T) {
	a, ids, _ := sigApp(t, 8, func(p library.Photo) (imgsig.Sig, error) {
		size, k := uint64(p.Size), uint64(0x9e3779b97f4a7c15)
		if p.ID == "t6" {
			size = 105 // как у t5: дубль
		}
		d := size * k
		return imgsig.Sig{D: d, P: ^d, Sharp: float32(200 - p.Size), Color: 20, Ok: true}, nil
	})
	o := model.AutoOpts{Spreads: 2, Auto: true, Similar: 1}
	if sd, err := a.AutoSimilar(ids, o); err != nil || len(sd) != 0 {
		t.Fatalf("без подписей: %v %v", sd, err)
	}
	a.PrepareSigs(ids)
	a.bg.Wait()
	sd, err := a.AutoSimilar(ids, o)
	if err != nil || !slices.Equal(sd, []model.SimilarDrop{{ID: "t6", Like: "t5", Dist: 0}}) {
		t.Fatalf("похожие %v %v", sd, err)
	}
	ap, err := a.AutoPlan(ids, o)
	if err != nil || ap.Similar != 1 || ap.Photos != 7 {
		t.Fatalf("план %+v %v", ap, err)
	}
}

// TestPrepareSigsPanic: паника декодера на битых файлах (t2, t5) гасится в воркере — программа жива,
// ход доходит до конца, битые запомнены с Ok = false и больше не пробуются, соседи посчитаны.
func TestPrepareSigsPanic(t *testing.T) {
	var calls atomic.Int32
	a, ids, got := sigApp(t, 10, func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		if p.ID == "t2" || p.ID == "t5" {
			panic("битый JPEG")
		}
		return imgsig.Sig{D: uint64(p.Size), Ok: true}, nil
	})
	a.PrepareSigs(ids)
	a.bg.Wait()
	if evs := got(); len(evs) == 0 || evs[len(evs)-1] != (SigState{10, 10}) {
		t.Fatalf("события %v", evs)
	}
	for _, info := range a.infos(ids) {
		if ok := info.ID != "t2" && info.ID != "t5"; info.Sig.Ok != ok {
			t.Fatalf("%s: подпись %+v", info.ID, info.Sig)
		}
	}
	if st := a.PrepareSigs(ids); st != (SigState{10, 10}) || calls.Load() != 10 {
		t.Fatalf("второй вызов: %+v, подписей %d", st, calls.Load())
	}
}

// TestPrepareSigsEmitOrder: событие прошлой работы, уже прошедшее проверку поколения, уходит до
// ответа нового PrepareSigs (gen++ под emitMu): новый вызов ждёт, пока оно отправится.
func TestPrepareSigsEmitOrder(t *testing.T) {
	a, ids, _ := sigApp(t, 4, func(p library.Photo) (imgsig.Sig, error) {
		return imgsig.Sig{D: 1, Ok: true}, nil
	})
	inEmit, release := make(chan struct{}), make(chan struct{})
	// Провал до отпускания не должен оставить воркер висеть в emit: bg.Wait очистки ждал бы вечно.
	free := sync.OnceFunc(func() { close(release) })
	defer free()
	var once sync.Once
	a.emit = func(name string, _ ...any) {
		if name == "sigs:progress" {
			once.Do(func() {
				close(inEmit)
				<-release
			})
		}
	}
	a.PrepareSigs(ids[:2])
	select {
	case <-inEmit:
	case <-time.After(10 * time.Second):
		t.Fatal("события нет")
	}
	answered := make(chan struct{})
	go func() {
		a.PrepareSigs(ids[2:])
		close(answered)
	}()
	select {
	case <-answered:
		t.Fatal("новый вызов ответил, пока событие прошлой работы не отправлено")
	case <-time.After(50 * time.Millisecond):
	}
	free()
	select {
	case <-answered:
	case <-time.After(10 * time.Second):
		t.Fatal("новый вызов не ответил")
	}
	a.bg.Wait()
}

// TestPrepareSigsIOErr: ошибка ввода-вывода (диск отключён, кэша нет) записи не кладёт — следующий
// вызов пробует снова и, когда диск вернулся, считает; ошибка разбора (t3) запоминается.
func TestPrepareSigsIOErr(t *testing.T) {
	var back atomic.Bool
	var calls atomic.Int32
	a, ids, got := sigApp(t, 6, func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		switch {
		case p.ID == "t1" && !back.Load():
			return imgsig.Sig{}, fmt.Errorf("превью: %w", &fs.PathError{Op: "open", Path: p.Path, Err: syscall.ENOENT})
		case p.ID == "t2" && !back.Load():
			return imgsig.Sig{}, errNoCache
		case p.ID == "t3":
			return imgsig.Sig{}, errors.New("invalid JPEG format")
		}
		return imgsig.Sig{D: uint64(p.Size), Ok: true}, nil
	})
	a.PrepareSigs(ids)
	a.bg.Wait()
	if evs := got(); len(evs) == 0 || evs[len(evs)-1] != (SigState{6, 6}) {
		t.Fatalf("события %v: непрочитанные тоже не ждут", evs)
	}
	if st := a.PrepareSigs(ids); st != (SigState{4, 6}) {
		t.Fatalf("второй вызов: %+v (t1, t2 — снова в работу)", st)
	}
	a.bg.Wait()
	if calls.Load() != 8 {
		t.Fatalf("подписей %d: повторно — только t1 и t2", calls.Load())
	}
	back.Store(true)
	a.PrepareSigs(ids)
	a.bg.Wait()
	for _, info := range a.infos(ids) {
		if ok := info.ID != "t3"; info.Sig.Ok != ok {
			t.Fatalf("%s: подпись %+v", info.ID, info.Sig)
		}
	}
	// Без подмены и без кэша миниатюр (cacheShot → errNoCache) в памяти ничего не остаётся.
	b, bids, _ := sigApp(t, 3, nil)
	b.PrepareSigs(bids)
	b.bg.Wait()
	if st := b.PrepareSigs(bids); st != (SigState{0, 3}) {
		t.Fatalf("без кэша: %+v", st)
	}
	b.bg.Wait()
}
