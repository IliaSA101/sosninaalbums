package sharer

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

// captureLog собирает журнал на время теста.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old); log.SetFlags(flags) })
	return &buf
}

// stopAfterTwo: две страницы доходят, на третьей идёт «Остановить»; остальные страницы сервер не видит.
func stopAfterTwo(r *rig) {
	var n atomic.Int32
	var once, stopped atomic.Bool
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) || n.Add(1) <= 2 {
			return nil
		}
		if once.CompareAndSwap(false, true) {
			// Обе принятые страницы должны успеть вернуться клиенту до отмены.
			for range 200 {
				if r.srv.Fake.Puts() >= 2 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			time.Sleep(150 * time.Millisecond)
			r.svc.Cancel()
			stopped.Store(true)
		}
		for range 500 { // остальные страницы ждут остановки, а не рвутся как сбой сети
			if stopped.Load() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		return &albumstest.Fault{Drop: true}
	})
}

// T5-13 п. 2: при остановке последнее показанное «Загружено N из M» — истинное N. Событие хода
// идёт не чаще gap, поэтому без итогового события окно застревало на «0 из 6», хотя 2 страницы ушли.
func TestShareStopKeepsCount(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.svc.gap = time.Hour // события хода — только первое и итоговое
	stopAfterTwo(r)
	if msg, stopped := r.run(Options{Days: 30}); msg != "" || !stopped {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	var last progressEvent
	var seen int
	r.mu.Lock()
	for _, e := range r.events {
		if pe, ok := e.data[0].(progressEvent); ok && e.name == "share:progress" && pe.Phase == "upload" {
			last, seen = pe, seen+1
		}
	}
	r.mu.Unlock()
	if seen < 2 || last.Total != 6 || last.Done != 2 || last.Bytes <= 0 || last.Bytes > last.TotalBytes {
		t.Errorf("последний ход загрузки (%d событий): %+v, ждали 2 из 6", seen, last)
	}
	if got := r.srv.Fake.Puts(); got != 2 {
		t.Errorf("на сервере %d страниц, ждали 2", got)
	}
}

// Итоговое событие хода не дублирует уже отправленное: при успехе (done == total) его нет.
func TestShareProgressFlushOnce(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.svc.gap = time.Hour
	r.must(Options{Days: 30})
	var ups []progressEvent
	r.mu.Lock()
	for _, e := range r.events {
		if pe, ok := e.data[0].(progressEvent); ok && pe.Phase == "upload" {
			ups = append(ups, pe)
		}
	}
	r.mu.Unlock()
	// Начальное 0 из 6 и последняя страница (done == total); итоговое событие не дублирует последнюю.
	if len(ups) != 2 || ups[0].Done != 0 || ups[1].Done != 6 || ups[1].Total != 6 {
		t.Errorf("события загрузки: %+v", ups)
	}
}

// T5-13 п. 3: остановка — не сбой. В журнале нет строк «страница N: остановлено» и других
// строк «…: остановлено»; итог выгрузки — одна строка.
func TestShareStopLogsOnce(t *testing.T) {
	r := newRig(t)
	r.connect()
	buf := captureLog(t)
	stopAfterTwo(r)
	if msg, stopped := r.run(Options{Days: 30}); msg != "" || !stopped {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	var stops []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "остановл") {
			stops = append(stops, l)
		}
	}
	if len(stops) != 1 || !strings.HasSuffix(stops[0], "выгрузка: остановлена") {
		t.Errorf("строки об остановке: %q\nвесь журнал:\n%s", stops, buf.String())
	}
}
