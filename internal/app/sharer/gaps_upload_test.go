package sharer

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тесты Т6 (0.32-146-T6): ветки загрузки и публикации, которые ревью Т4 нашло непокрытыми.

const page2 = "/pages/2"

func isPage2(q *http.Request) bool { return isPut(q) && strings.HasSuffix(q.URL.Path, page2) }

// pageNo — номер страницы из адреса PUT .../pages/N (0, если не разобрался).
func pageNo(q *http.Request) int {
	n, err := strconv.Atoi(q.URL.Path[strings.LastIndex(q.URL.Path, "/")+1:])
	if err != nil {
		return 0
	}
	return n
}

func isPublish(q *http.Request) bool {
	return q.Method == http.MethodPost && strings.HasSuffix(q.URL.Path, "/publish")
}

// lostPage — сервер «принял» страницу 2 (204), но не сохранил её: так выглядит потеря страницы до
// публикации. Дальше publish ответит 409 pages_missing. n — сколько раз страницу ждали.
func lostPage(r *rig, times int, puts, publishes *atomic.Int32) {
	r.setFault(func(q *http.Request) *albumstest.Fault {
		switch {
		case isPage2(q):
			if int(puts.Add(1)) <= times {
				return &albumstest.Fault{Status: http.StatusNoContent}
			}
		case isPublish(q):
			publishes.Add(1)
		}
		return nil
	})
}

// 409 pages_missing при publish: потерянная страница догружается один раз, публикация повторяется,
// ссылка готова; все 6 страниц на сервере.
func TestSharePublishPagesMissingReupload(t *testing.T) {
	r := newRig(t)
	r.connect()
	var puts, pubs atomic.Int32
	lostPage(r, 1, &puts, &pubs)
	r.must(Options{Days: 30})
	id, _ := r.rec()
	if r.srv.Fake.Published(id) == nil || len(r.srv.Fake.Stored(id)) != 6 {
		t.Errorf("не опубликовано или страниц на сервере %d", len(r.srv.Fake.Stored(id)))
	}
	if puts.Load() != 2 || pubs.Load() != 2 {
		t.Errorf("страницу 2 слали %d раз (ждали 2), публикаций %d (ждали 2)", puts.Load(), pubs.Load())
	}
	if l := r.state().Link; l == nil || l.State != "ok" || l.URL == "" {
		t.Errorf("ссылка: %+v", l)
	}
	if r.tempLeft() != 0 {
		t.Error("осталась временная папка")
	}
}

// Сервис теряет страницу и после догрузки: публикация повторяется ровно один раз, дальше —
// видимая ошибка без кода сервиса, запись «не закончена» с id (следующий «Продолжить» её догрузит).
func TestSharePublishPagesMissingTwice(t *testing.T) {
	r := newRig(t)
	r.connect()
	var puts, pubs atomic.Int32
	lostPage(r, 1000, &puts, &pubs)
	msg, stopped := r.run(Options{Days: 30})
	if stopped || msg == "" || strings.Contains(msg, "pages_missing") || strings.Contains(msg, "albums:") {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	if pubs.Load() != 2 || puts.Load() != 2 {
		t.Errorf("публикаций %d, загрузок страницы 2: %d; ждали по 2 (одна догрузка)", pubs.Load(), puts.Load())
	}
	l := r.state().Link
	if l == nil || l.State != "incomplete" || l.URL != "" {
		t.Fatalf("запись после отказа: %+v", l)
	}
	r.setFault(nil)
	r.must(Options{}) // сервер снова хранит страницы: «Продолжить» догружает потерянную
	id, _ := r.rec()
	if r.srv.Fake.Published(id) == nil || r.state().Link.State != "ok" {
		t.Error("после «Продолжить» ссылка не опубликована")
	}
}

// corrupt портит байт в середине тела запроса: длина и сигнатура прежние, sha256 — нет.
func corrupt(q *http.Request) {
	b, _ := io.ReadAll(q.Body)
	b[len(b)/2] ^= 0xFF
	q.Body = io.NopCloser(bytes.NewReader(b))
}

// hash_mismatch при загрузке страницы: повтор не поможет — без пауз, одна попытка, понятный текст
// без кода сервиса; запись «не закончена», после починки «Продолжить» доводит ссылку.
func TestShareHashMismatch(t *testing.T) {
	r := newRig(t)
	r.connect()
	var tries atomic.Int32
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPage2(q) {
			tries.Add(1)
			corrupt(q)
		}
		return nil
	})
	msg, stopped := r.run(Options{Days: 30})
	if stopped || !strings.Contains(msg, "повреждённой") || strings.Contains(msg, "hash_mismatch") || strings.Contains(msg, "409") {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	if tries.Load() != 1 || len(r.waits) != 0 {
		t.Errorf("попыток %d (ждали 1), пауз %v (ждали нет)", tries.Load(), r.waits)
	}
	l := r.state().Link
	id, ok := r.rec()
	if !ok || l == nil || l.State != "incomplete" || l.URL != "" || r.srv.Fake.Published(id) != nil {
		t.Fatalf("после отказа: запись %v %+v", ok, l)
	}
	r.setFault(nil)
	r.must(Options{})
	if id2, _ := r.rec(); id2 != id || r.state().Link.State != "ok" || len(r.srv.Fake.Stored(id)) != 6 {
		t.Error("«Продолжить» не довело ссылку")
	}
}

// Обрыв соединения без остановки: сеть повторяется 2/4/8 с, после трёх повторов — «Нет связи»;
// запись «не закончена» с id, «Продолжить» догружает только остаток.
func TestShareDropMidUpload(t *testing.T) {
	r := newRig(t)
	r.connect()
	// Обрыв по номеру страницы, а не по порядку прихода: страницы идут тремя потоками, и при счёте
	// «третий PUT и далее» оборванной могла оказаться любая, а её соседа отмена выгрузки отрезала на
	// лету, до сохранения (CI go-race: на сервере 1 страница вместо 2). Страницы 3+ обрываются, но
	// только когда 1 и 2 уже сохранены; их самих не трогаем.
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) || pageNo(q) < 3 {
			return nil
		}
		end := time.Now().Add(testkit.Limit(5 * time.Second))
		for r.srv.Fake.Puts() < 2 && time.Now().Before(end) {
			time.Sleep(time.Millisecond)
		}
		return &albumstest.Fault{Drop: true}
	})
	msg, stopped := r.run(Options{Days: 30})
	if stopped || !strings.Contains(msg, "Нет связи") {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	r.mu.Lock()
	waits := append([]time.Duration(nil), r.waits...)
	r.mu.Unlock()
	want := map[time.Duration]bool{2 * time.Second: false, 4 * time.Second: false, 8 * time.Second: false}
	for _, d := range waits {
		if _, ok := want[d]; ok {
			want[d] = true
		}
	}
	for d, seen := range want {
		if !seen {
			t.Errorf("нет паузы %v среди %v", d, waits)
		}
	}
	id, ok := r.rec()
	l := r.state().Link
	if !ok || l == nil || l.State != "incomplete" || l.URL != "" || r.srv.Fake.Published(id) != nil || r.tempLeft() != 0 {
		t.Fatalf("после обрыва: запись %v %+v", ok, l)
	}
	have := r.srv.Fake.Puts()
	if have != 2 {
		t.Fatalf("на сервере %d страниц, ждали 2", have)
	}
	r.setFault(nil)
	r.must(Options{})
	if got := r.srv.Fake.Puts() - have; got != 6-have {
		t.Errorf("догружено %d страниц, ждали %d", got, 6-have)
	}
	if id2, _ := r.rec(); id2 != id || r.state().Link.State != "ok" {
		t.Error("ссылка не та или не опубликована")
	}
}

// Одиночный обрыв — повтор через 2 с, выгрузка доходит до конца.
func TestShareDropOnceRetried(t *testing.T) {
	r := newRig(t)
	r.connect()
	var once atomic1
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) && once.first() {
			return &albumstest.Fault{Drop: true}
		}
		return nil
	})
	r.must(Options{Days: 30})
	if r.srv.Fake.Puts() != 6 || len(r.waits) == 0 || r.waits[0] != 2*time.Second {
		t.Errorf("загрузок %d, паузы %v", r.srv.Fake.Puts(), r.waits)
	}
}

// Предел страниц сервиса: ошибка до рендера и до создания альбома — ни Create, ни страниц; на
// самом пределе выгрузка идёт.
func TestSharePagesLimit(t *testing.T) {
	r := newRigLimits(t, albums.Limits{Pages: 4})
	r.connect()
	var reqs headerLog
	r.setFault(reqs.fault)
	msg, stopped := r.run(Options{Days: 30})
	if stopped || !strings.Contains(msg, "В альбоме 6 страниц, сервис принимает не больше 4") {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	if reqs.n != 0 || r.srv.Fake.Albums() != 0 || r.srv.Fake.Puts() != 0 || r.tempLeft() != 0 {
		t.Errorf("запросов %d, альбомов %d, страниц %d, папок %d", reqs.n, r.srv.Fake.Albums(), r.srv.Fake.Puts(), r.tempLeft())
	}
	if _, ok := r.rec(); ok {
		t.Error("запись о ссылке создана до проверки предела")
	}
	if l := r.state(); l.Link != nil || l.Busy || r.svc.Busy() {
		t.Errorf("состояние после отказа: %+v", l)
	}

	edge := newRigLimits(t, albums.Limits{Pages: 6}) // ровно столько страниц, сколько в альбоме
	edge.connect()
	edge.must(Options{Days: 30})
}

// Предел размера страницы сервиса: понятная ошибка без Create.
func TestSharePageBytesLimit(t *testing.T) {
	r := newRigLimits(t, albums.Limits{PageBytes: 100})
	r.connect()
	msg, stopped := r.run(Options{Days: 30})
	if stopped || msg == "" || strings.Contains(msg, "albums:") || r.srv.Fake.Albums() != 0 {
		t.Fatalf("итог: %q stopped=%v, альбомов %d", msg, stopped, r.srv.Fake.Albums())
	}
}
