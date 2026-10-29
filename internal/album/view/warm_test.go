package view_test

// Прогрев раскладок Editor.Warm перед View (073: окно «Открываем альбом»).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// coldReset — холодные кэши раскладок, рядов «Авто» и «Других вариантов».
func coldReset() {
	layout.ResetCache()
	variety.ResetCache()
}

// warmTotal — сколько разворотов греет Warm: «Авто» с фото (как layAll).
func warmTotal(e *album.Editor) int {
	n := 0
	for _, s := range edtest.P(e).Spreads {
		if len(s.Grid) == 0 && len(s.Photos) > 0 {
			n++
		}
	}
	return n
}

// warmRun — Warm с записью хода; проверяет, что done шёл 1, 2, …, total без повторов.
func warmRun(t *testing.T, e *album.Editor) {
	t.Helper()
	if err := warmSeq(e, warmTotal(e)); err != nil {
		t.Fatal(err)
	}
}

// warmSeq — Warm с проверкой хода; ошибка, а не t.Fatal — зовётся и из горутин (total считан
// заранее: проект без замка Editor читать нельзя).
func warmSeq(e *album.Editor, total int) error {
	var got []int
	bad := -1
	e.Warm(func(done, n int) {
		if n != total {
			bad = n
		}
		got = append(got, done)
	})
	if bad >= 0 {
		return fmt.Errorf("total %d, ждали %d", bad, total)
	}
	if len(got) != total {
		return fmt.Errorf("ход %v, ждали 1…%d", got, total)
	}
	for i, d := range got {
		if d != i+1 {
			return fmt.Errorf("ход не по порядку: %v", got)
		}
	}
	return nil
}

func marshalView(t *testing.T, e *album.Editor) []byte {
	t.Helper()
	data, err := json.Marshal(e.View())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestWarmSameView: View после Warm байт в байт тот же, что холодный без него (смесь «Авто»,
// постраничных, перестановок, «Ручной» и пустого), ряды всех греемых разворотов в кэше. Warm — не
// правка: rev, dirty и отмена прежние. Повторный Warm (тёплый кэш, повторное открытие) — тот же ход.
func TestWarmSameView(t *testing.T) {
	t.Cleanup(coldReset)
	e := edtest.Open(t, mixedProject(t), "")
	if warmTotal(e) < 6 || warmTotal(e) == len(edtest.P(e).Spreads) {
		t.Fatalf("заготовка: греется %d из %d разворотов", warmTotal(e), len(edtest.P(e).Spreads))
	}
	coldReset()
	want := marshalView(t, e)
	coldReset()
	rev, dirty, undo := edtest.Rev(e), edtest.Dirty(e), edtest.UndoLen(e)
	warmRun(t, e)
	p := edtest.P(e)
	for _, s := range p.Spreads {
		if len(s.Grid) == 0 && len(s.Photos) > 0 && !variety.RowCached(p.Aspects(s.Photos), p.Params(), model.RowLen(len(s.Photos))) {
			t.Fatalf("ряд разворота %s на %d фото не прогрет", s.ID, len(s.Photos))
		}
	}
	if edtest.Rev(e) != rev || edtest.Dirty(e) != dirty || edtest.UndoLen(e) != undo {
		t.Fatalf("Warm поменял rev/dirty/отмену: %d/%v/%d → %d/%v/%d", rev, dirty, undo, edtest.Rev(e), edtest.Dirty(e), edtest.UndoLen(e))
	}
	if got := marshalView(t, e); !bytes.Equal(got, want) {
		t.Fatal("View после прогрева отличается от холодного")
	}
	warmRun(t, e)
	if got := marshalView(t, e); !bytes.Equal(got, want) {
		t.Fatal("View после повторного прогрева отличается")
	}
}

// TestWarmEmptyAndUndo: пустой альбом и progress == nil — ни вызовов, ни паники. После правки
// Warm шага отмены не добавляет и не рвёт: Undo отменяет ту правку, rev после Warm прежний.
func TestWarmEmptyAndUndo(t *testing.T) {
	t.Cleanup(coldReset)
	e := edtest.Open(t, model.New(), "")
	calls := 0
	e.Warm(func(int, int) { calls++ })
	e.Warm(nil)
	if calls != 0 {
		t.Fatalf("пустой альбом: %d вызовов хода", calls)
	}
	e = edtest.Open(t, bigProject(4, 9), "")
	id := edtest.P(e).Spreads[1].ID
	e.SetSpreadName(id, "Прогулка")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	coldReset()
	warmRun(t, e)
	e.Warm(nil)
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("Warm после правки: rev %d → %d, шагов %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	v := e.Undo()
	if v.Spreads[1].Name != "" {
		t.Fatalf("Undo после Warm не отменил название: %q", v.Spreads[1].Name)
	}
}

// TestWarmEditDuring (073-11): правки посреди счёта Warm не ждут его (под e.mu — только снимок
// входа) и вид не портят: прогрев шёл по устаревшему снимку, а View тот же, что холодный. Две
// правки — два шага отмены, холостая — без шага; Undo дважды возвращает прежний вид. Под -race.
func TestWarmEditDuring(t *testing.T) {
	t.Cleanup(coldReset)
	e := edtest.Open(t, bigProject(8, 12), "")
	p0 := edtest.P(e)
	photo, to, named := p0.Spreads[0].Photos[0], p0.Spreads[1].ID, p0.Spreads[2].ID
	st := p0.Settings
	st.Gap++ // другие Params: все ключи снимка устарели
	coldReset()
	orig, err := json.Marshal(e.View().Spreads) // без rev и флагов отмены: после Undo они другие
	if err != nil {
		t.Fatal(err)
	}
	coldReset()
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edited := make(chan struct{})
	var moveErr error
	waited := false
	first := true
	e.Warm(func(done, n int) {
		if !first {
			return
		}
		first = false
		go func() {
			defer close(edited)
			e.SetSettings(st)
			_, moveErr = e.MovePhoto(photo, to, 0)
			e.SetSpreadName(named, "") // и так без названия: холостая
		}()
		select {
		case <-edited:
		case <-time.After(testkit.Limit(2 * time.Second)):
			waited = true // правки ждут конца прогрева: не ждём их здесь, иначе тупик
		}
	})
	<-edited
	if moveErr != nil {
		t.Fatal(moveErr)
	}
	if waited {
		t.Fatal("правки во время Warm ждали конца прогрева")
	}
	if edtest.Rev(e) != rev+2 || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("rev %d → %d, шагов %d → %d; ждали +2 и +2", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	got := marshalView(t, e)
	coldReset()
	if want := marshalView(t, e); !bytes.Equal(got, want) {
		t.Fatal("вид после прогрева по устаревшему снимку отличается от холодного")
	}
	e.Undo()
	coldReset()
	if got, _ := json.Marshal(e.Undo().Spreads); !bytes.Equal(got, orig) {
		t.Fatal("Undo после правок во время прогрева не вернул прежний вид")
	}
}

// TestWarmConcurrent: Warm одновременно с View и правками (go test -race): правка не ждёт счёта,
// прогревы идут по одному (warmMu), ход каждого полон, итоговый вид тот же, что у холодного.
func TestWarmConcurrent(t *testing.T) {
	t.Cleanup(coldReset)
	e := edtest.Open(t, bigProject(8, 10), "")
	id, total := edtest.P(e).Spreads[0].ID, warmTotal(e)
	coldReset()
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Go(func() { errs[i] = warmSeq(e, total) })
	}
	wg.Go(func() {
		for i := range 4 {
			e.SetSpreadName(id, fmt.Sprint("Имя ", i))
			e.View()
		}
	})
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := marshalView(t, e)
	coldReset()
	if got := marshalView(t, e); !bytes.Equal(got, want) {
		t.Fatal("вид после параллельного прогрева отличается от холодного")
	}
}

// BenchmarkWarmView200x16 — открытие худшего альбома через Warm (073): время операции — прогрев и
// View, view-ms — один View после прогрева (то, что App держит под своим замком).
func BenchmarkWarmView200x16(b *testing.B) {
	e := edtest.Open(b, bigProject(200, 16), "")
	var view time.Duration
	for b.Loop() {
		coldReset()
		e.Warm(nil)
		t0 := time.Now()
		e.View()
		view += time.Since(t0)
	}
	b.ReportMetric(float64(view.Microseconds())/1000/float64(b.N), "view-ms/op")
}
