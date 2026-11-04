package app

// 160-M1, доправка 6: фото проекта в облаке без готовой копии попадает в State.Unread с причиной (плитка «не читается»,
// запросов /media/t нет), запись снимается, когда файл скачали или копия появилась.

import (
	"bytes"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
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

// TestCloudOtherSizeCopy: у заглушки остался один размер копии (Trim первым убирает превью) — запрос другого размера
// отдаёт её без пометки «в облаке»: ни записи в Unread, ни событий на кругах «запрос + сверка» (раньше плитка
// мигала: noteCloud ставил, сверка снимала). Блоки независимы.
func TestCloudOtherSizeCopy(t *testing.T) {
	for _, tc := range []struct {
		name         string
		drop, asking string // убираемый размер копии и запрашиваемый адрес
	}{
		{"миниатюра есть, превью нет", "/media/p/", "/media/p/"},
		{"превью есть, миниатюры нет", "/media/t/", "/media/t/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCloudRig(t)
			thumb, prev := r.warmBeforeCloud(t)
			have := thumb
			if tc.drop == "/media/t/" {
				have = prev
			}
			rm, ok := r.a.cache.Cached(r.stub, tc.drop == "/media/p/")
			if !ok {
				t.Fatal("копия для удаления не найдена")
			}
			if err := os.Remove(rm); err != nil {
				t.Fatal(err)
			}
			var ev unreadLog
			ev.hook(r.a)
			for i := range 3 {
				rec := origReq(r.a, http.MethodGet, tc.asking+r.stub.ID)
				if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), have) {
					t.Fatalf("круг %d: код %d, %d байт (ждали копию другого размера, %d байт)", i, rec.Code, rec.Body.Len(), len(have))
				}
				if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
					t.Errorf("круг %d: Cache-Control %q: копия чужого размера не должна кэшироваться навсегда", i, cc)
				}
				recheck(t, r.a)
			}
			if _, n := ev.get(); n != 0 {
				t.Errorf("событий unread: %d, ожидалось 0", n)
			}
			if st := r.a.GetState(); len(st.Unread) != 0 {
				t.Errorf("Unread = %v", st.Unread)
			}
		})
	}
}

// TestCloudNoStatForGone: пропавшее фото сверка облака не трогает — ни одного Stat. Фото недоступного тома тоже «пропавшее»
// (fileExists отвечает «нет» по reachable без Stat), так что SMB-таймаут на каждое фото не набегает: отдельной пробы тома
// сверка облака не делает (вторая проба удваивала бы обращения к тому).
func TestCloudNoStatForGone(t *testing.T) {
	r := newCloudRig(t)
	r.a.exists = func(p string) bool { return filepath.Base(p) != "stub.jpg" } // как fileExists на мёртвом томе
	oldS := cloudStub
	t.Cleanup(func() { cloudStub = oldS })
	var calls atomic.Int64
	cloudStub = func(p string) bool {
		if filepath.Base(p) == "stub.jpg" {
			calls.Add(1)
		}
		return false
	}
	recheck(t, r.a)
	if n := calls.Load(); n != 0 {
		t.Errorf("cloudStub вызван %d раз для пропавшего фото", n)
	}
	if st := r.a.GetState(); len(st.Unread) != 0 {
		t.Errorf("Unread = %v", st.Unread)
	}
}

// ribbonRig — cloudRig с фото ленты: «stub.jpg» (оно же в проекте) и «rib.jpg» (только в ленте, в проекте нет) стоят
// в реестре byID; признак облака — и у app (cloudStub), и у library (dehydrated) по имени файла и флагам.
type ribbonRig struct {
	*cloudRig
	rib      library.Photo
	ribCloud atomic.Bool
	ev       unreadLog
}

func newRibbonRig(t *testing.T) *ribbonRig {
	t.Helper()
	r := &ribbonRig{cloudRig: newCloudRig(t)}
	r.ev.hook(r.a)
	p := testkit.JPEG(t, testkit.TempDir(t), "rib.jpg", 48, 36)
	r.rib = library.Photo{ID: library.IDFor(p), Path: p, Name: "rib.jpg", W: 48, H: 36}
	r.a.mu.Lock()
	r.a.byID = map[string]library.Photo{r.stub.ID: r.stub, r.rib.ID: r.rib}
	r.a.mu.Unlock()
	r.ribCloud.Store(true)
	isCloud := func(name string) bool {
		return name == "stub.jpg" && r.stubIsCloud.Load() || name == "rib.jpg" && r.ribCloud.Load()
	}
	oldS := cloudStub
	t.Cleanup(func() { cloudStub = oldS })
	cloudStub = func(path string) bool { return isCloud(filepath.Base(path)) }
	library.UseDehydratedForTest(t, func(st fs.FileInfo) bool { return st != nil && isCloud(st.Name()) })
	return r
}

// TestCloudRibbonPhoto: фото ленты стало заглушкой после сканирования. Запрос миниатюры — 404 без чтения оригинала
// и запись в Unread (serveThumb → noteCloud); файл скачали — сверка снимает запись фото, которого в проекте нет
// (ветка ленты cloudScan), а удачная отдача копии снимает её сразу (clearCloud). Блоки независимы.
func TestCloudRibbonPhoto(t *testing.T) {
	t.Run("запрос ставит запись", func(t *testing.T) {
		r := newRibbonRig(t)
		if rec := origReq(r.a, http.MethodGet, "/media/t/"+r.rib.ID); rec.Code != http.StatusNotFound {
			t.Fatalf("код %d", rec.Code)
		}
		if got := r.a.GetState().Unread[r.rib.ID]; got != library.WhyCloudOnly {
			t.Fatalf("Unread[rib] = %q", got)
		}
		if n := cloudCacheFiles(t, r.cacheDir); n != 0 {
			t.Errorf("в кэше %d файлов: оригинал читался", n)
		}
	})
	t.Run("лента: копия другого размера", func(t *testing.T) {
		r := newRibbonRig(t)
		want, _ := r.warmBeforeCloud(t) // обе копии построены, пока файл на диске; дальше он в облаке
		rm, ok := r.a.cache.Cached(r.stub, true)
		if !ok {
			t.Fatal("превью не построено")
		}
		if err := os.Remove(rm); err != nil { // как Trim: превью ушло первым
			t.Fatal(err)
		}
		_, n := r.ev.get()
		rec := origReq(r.a, http.MethodGet, "/media/p/"+r.stub.ID)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("код %d, %d байт; ждали миниатюру (%d байт)", rec.Code, rec.Body.Len(), len(want))
		}
		if _, n2 := r.ev.get(); n2 != n || len(r.a.GetState().Unread) != 0 {
			t.Errorf("запись «в облаке» поставлена при готовой копии: %v", r.a.GetState().Unread)
		}
	})
	t.Run("сверка снимает запись ленты", func(t *testing.T) {
		r := newRibbonRig(t)
		origReq(r.a, http.MethodGet, "/media/t/"+r.rib.ID)
		recheck(t, r.a)
		if r.a.GetState().Unread[r.rib.ID] == "" {
			t.Fatal("сверка сняла запись, пока файл в облаке")
		}
		r.ribCloud.Store(false) // файл скачали
		recheck(t, r.a)
		if got := r.a.GetState().Unread[r.rib.ID]; got != "" {
			t.Errorf("файл скачан, а Unread[rib] = %q", got)
		}
		if last, _ := r.ev.get(); last[r.rib.ID] != "" {
			t.Errorf("последнее событие несёт запись: %v", last)
		}
	})
	t.Run("отдача копии снимает запись", func(t *testing.T) {
		r := newRibbonRig(t)
		origReq(r.a, http.MethodGet, "/media/t/"+r.rib.ID)
		if r.a.GetState().Unread[r.rib.ID] == "" {
			t.Fatal("записи нет")
		}
		r.ribCloud.Store(false)
		if rec := origReq(r.a, http.MethodGet, "/media/t/"+r.rib.ID); rec.Code != http.StatusOK {
			t.Fatalf("код %d", rec.Code)
		}
		if got := r.a.GetState().Unread[r.rib.ID]; got != "" {
			t.Errorf("копия отдана, а Unread[rib] = %q", got)
		}
	})
}

// TestCloudCopyIDKeys: у копии фото (ID «X-2») запись «в облаке» стоит под ID копии — тем же, что у missing и у Cell:
// и из сверки, и из запроса миниатюры.
func TestCloudCopyIDKeys(t *testing.T) {
	r := newCloudRig(t)
	info := model.PhotoInfo{ID: model.CopyReq(r.stub.ID), Path: r.stub.Path, W: r.stub.W, H: r.stub.H}
	if _, err := r.a.editor().NewSpread(-1, []model.PhotoInfo{info}); err != nil {
		t.Fatal(err)
	}
	cp := model.CopyID(r.stub.ID, 2)
	t.Run("запрос раньше сверки", func(t *testing.T) {
		if rec := origReq(r.a, http.MethodGet, "/media/t/"+cp); rec.Code != http.StatusNotFound {
			t.Fatalf("код %d", rec.Code)
		}
		if got := r.a.GetState().Unread[cp]; got != library.WhyCloudOnly {
			t.Errorf("Unread[%s] = %q; всё: %v", cp, got, r.a.GetState().Unread)
		}
	})
	t.Run("сверка", func(t *testing.T) {
		recheck(t, r.a)
		u := r.a.GetState().Unread
		if u[cp] == "" || u[r.stub.ID] == "" || len(u) != 2 {
			t.Errorf("Unread = %v; ждали и %s, и %s", u, r.stub.ID, cp)
		}
	})
}
