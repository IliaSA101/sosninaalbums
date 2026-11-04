package app

// 160-M1, доправка 6: фото проекта в облаке без готовой копии попадает в State.Unread с причиной (плитка «не читается»,
// запросов /media/t нет), запись снимается, когда файл скачали или копия появилась.

import (
	"net/http"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// unreadLog записывает события `unread` (последняя карта) вместо отправки в окно.
type unreadLog struct {
	mu   sync.Mutex
	last map[string]string
	n    int
}

func (u *unreadLog) hook(a *App) {
	a.emit = func(name string, data ...any) {
		if name != "unread" || len(data) != 1 {
			return
		}
		m, _ := data[0].(map[string]string)
		u.mu.Lock()
		u.last, u.n = m, u.n+1
		u.mu.Unlock()
	}
}

func (u *unreadLog) get() (map[string]string, int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.last, u.n
}

// recheck — полный пересчёт файлов проекта (так делают открытие, rescan и автосверка окна) до конца.
func recheck(t *testing.T, a *App) {
	t.Helper()
	a.refreshMissing()
	waitMissing(t, a)
}

func TestCloudUnreadState(t *testing.T) {
	t.Run("заглушка без копии", func(t *testing.T) {
		r := newCloudRig(t)
		var ev unreadLog
		ev.hook(r.a)
		recheck(t, r.a)
		st := r.a.GetState()
		if st.Unread[r.stub.ID] != library.WhyCloudOnly {
			t.Fatalf("State.Unread[stub] = %q, ожидалась причина «в облаке»; всё: %v", st.Unread[r.stub.ID], st.Unread)
		}
		if _, bad := st.Unread[r.plain.ID]; bad {
			t.Errorf("обычное фото в Unread: %v", st.Unread)
		}
		if last, n := ev.get(); n == 0 || last[r.stub.ID] != library.WhyCloudOnly {
			t.Errorf("событие unread: %d раз, %v", n, last)
		}
		if n := cloudCacheFiles(t, r.cacheDir); n != 0 {
			t.Errorf("в кэше %d файлов: оригинал заглушки читался", n)
		}
		if n := r.colorReads.Load(); n != 0 {
			t.Errorf("заголовок заглушки прочитан %d раз", n)
		}
		// Повторная сверка без перемен — без нового события.
		_, before := ev.get()
		recheck(t, r.a)
		if _, after := ev.get(); after != before {
			t.Errorf("повторная сверка прислала unread (%d → %d)", before, after)
		}
	})
	t.Run("пересканирование не снимает", func(t *testing.T) {
		r := newCloudRig(t)
		recheck(t, r.a)
		r.a.rescan() // prunedUnread чистит только отдельные файлы src: это фото вне src
		waitScan(t, r.a)
		if got := r.a.GetState().Unread[r.stub.ID]; got != library.WhyCloudOnly {
			t.Errorf("после rescan: %q", got)
		}
		waitMissing(t, r.a)
		if got := r.a.GetState().Unread[r.stub.ID]; got != library.WhyCloudOnly {
			t.Errorf("после rescan и сверки: %q", got)
		}
	})
	t.Run("с копией", func(t *testing.T) {
		r := newCloudRig(t)
		r.warmBeforeCloud(t) // копии построены, пока файл был на диске
		recheck(t, r.a)
		if st := r.a.GetState(); len(st.Unread) != 0 {
			t.Errorf("есть готовая копия, а Unread = %v", st.Unread)
		}
	})
	t.Run("файл снова обычный", func(t *testing.T) {
		r := newCloudRig(t)
		var ev unreadLog
		ev.hook(r.a)
		recheck(t, r.a)
		if r.a.GetState().Unread[r.stub.ID] == "" {
			t.Fatal("предпосылка: запись не появилась")
		}
		r.stubIsCloud.Store(false) // файл скачали
		recheck(t, r.a)
		if st := r.a.GetState(); len(st.Unread) != 0 {
			t.Errorf("файл скачан, а Unread = %v", st.Unread)
		}
		if last, _ := ev.get(); len(last) != 0 {
			t.Errorf("последнее событие unread не снимает запись: %v", last)
		}
	})
	t.Run("фото убрано из альбома", func(t *testing.T) {
		r := newCloudRig(t)
		recheck(t, r.a)
		r.a.editor().Undo()      // последний NewSpread — «plain»
		v := r.a.editor().Undo() // и «stub»
		r.a.syncMissing(v.Rev)
		if st := r.a.GetState(); len(st.Unread) != 0 {
			t.Errorf("фото убрано, а Unread = %v", st.Unread)
		}
	})
}

// TestCloudThumbRequestNotes: запрос миниатюры раньше сверки файлов — 404 один раз, а плитка тут же получает причину;
// копия, отданная позже, запись снимает.
func TestCloudThumbRequestNotes(t *testing.T) {
	r := newCloudRig(t)
	var ev unreadLog
	ev.hook(r.a)
	if rec := origReq(r.a, http.MethodGet, "/media/t/"+r.stub.ID); rec.Code != http.StatusNotFound {
		t.Fatalf("код %d", rec.Code)
	}
	if got := r.a.GetState().Unread[r.stub.ID]; got != library.WhyCloudOnly {
		t.Fatalf("после 404: %q", got)
	}
	_, n := ev.get()
	origReq(r.a, http.MethodGet, "/media/t/"+r.stub.ID) // та же причина — тишина
	if _, n2 := ev.get(); n2 != n {
		t.Errorf("повтор прислал unread: %d → %d", n, n2)
	}
	r.warmBeforeCloud(t) // копия появилась (файл был на диске)
	if rec := origReq(r.a, http.MethodGet, "/media/t/"+r.stub.ID); rec.Code != http.StatusOK {
		t.Fatalf("с копией код %d", rec.Code)
	}
	if st := r.a.GetState(); len(st.Unread) != 0 {
		t.Errorf("копия отдана, а Unread = %v", st.Unread)
	}
}
