package sharer

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

// Первая выгрузка: ссылка, запись у файла альбома, страницы на сервере, временная папка убрана.
func TestShareFirst(t *testing.T) {
	r := newRig(t)
	if v := r.state(); v.Service.Host != "" || v.Link != nil {
		t.Fatalf("до подключения: %+v", v)
	}
	r.connect()
	r.must(Options{Title: "Свадьба", Password: "лето-4827", Days: 90})
	v := r.state()
	if v.Link == nil || v.Link.State != "ok" || !strings.HasPrefix(v.Link.URL, r.srv.URL) || v.Link.Password != "лето-4827" || v.Link.Title != "Свадьба" {
		t.Fatalf("ссылка: %+v", v.Link)
	}
	if !strings.HasPrefix(v.Link.Published, "Выгружено ") || !strings.HasSuffix(v.Link.Published, "как «Свадьба»") || !strings.HasPrefix(v.Link.Until, "до ") {
		t.Errorf("строки ссылки: %+v", v.Link)
	}
	id, ok := r.rec()
	if !ok || r.srv.Fake.Published(id) == nil || len(r.srv.Fake.Stored(id)) != 6 || r.srv.Fake.Puts() != 6 {
		t.Errorf("сервер: запись %v, страниц %d, загрузок %d", ok, len(r.srv.Fake.Stored(id)), r.srv.Fake.Puts())
	}
	if n := r.tempLeft(); n != 0 {
		t.Errorf("временных папок осталось %d", n)
	}
	var sawPages, sawUpload, sawPublish bool
	for _, e := range r.events {
		if e.name == "share:progress" {
			if len(e.data) != 1 {
				t.Fatalf("share:progress: %d аргументов, ждали один объект", len(e.data))
			}
			switch e.data[0].(progressEvent).Phase {
			case "pages":
				sawPages = true
			case "upload":
				sawUpload = true
			case "publish":
				sawPublish = true
			}
		}
	}
	if !sawPages || !sawUpload || !sawPublish {
		t.Errorf("фазы хода: страницы %v, загрузка %v, публикация %v", sawPages, sawUpload, sawPublish)
	}
	text, err := r.svc.Text()
	if err != nil || !strings.Contains(text, v.Link.URL) || !strings.Contains(text, "Пароль: лето-4827") {
		t.Errorf("текст клиенту: %q %v", text, err)
	}
}

// T5-6: «Продолжить» (Days 0) после обрыва, когда Create уже поставил пароль: пароль на сервере
// остаётся, служба не шлёт password и не берёт название и пароль из окна.
func TestShareResumePassword(t *testing.T) {
	r := newRig(t)
	r.connect()
	var (
		mu     sync.Mutex
		bodies []string // тела запросов, кроме загрузки страниц: «МЕТОД путь тело»
		puts   atomic.Int32
	)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) {
			if q.Method != http.MethodGet {
				b := q.Method + " " + q.URL.Path + " " + bodyOf(q)
				mu.Lock()
				bodies = append(bodies, b)
				mu.Unlock()
			}
		} else if puts.Add(1) > 2 {
			return &albumstest.Fault{Status: http.StatusRequestEntityTooLarge}
		}
		return nil
	})
	if msg, _ := r.run(Options{Title: "Свадьба", Password: "лето-4827", Days: 30}); msg == "" {
		t.Fatal("ждали обрыв")
	}
	rec, _ := r.svc.look(r.curJob().Path)
	if rec.URL != "" || rec.Password != "лето-4827" {
		t.Fatalf("после обрыва: url %q, пароль %q", rec.URL, rec.Password)
	}
	mu.Lock()
	bodies = bodies[:0]
	mu.Unlock()
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) && q.Method != http.MethodGet {
			b := q.Method + " " + q.URL.Path + " " + bodyOf(q)
			mu.Lock()
			bodies = append(bodies, b)
			mu.Unlock()
		}
		return nil
	})
	// Окно шлёт «чужие» поля: они игнорируются.
	r.must(Options{Title: "Другое", Password: "", Days: 0})
	l := r.state().Link
	if l == nil || l.State != "ok" || l.Password != "лето-4827" || l.Title != "Свадьба" {
		t.Fatalf("ссылка после продолжения: %+v", l)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, b := range bodies {
		if strings.Contains(b, "password") || strings.Contains(b, "Другое") {
			t.Errorf("запрос несёт пароль или название окна: %s", b)
		}
	}
	resp, err := http.Get(l.URL + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("альбом открыт без пароля: %d", resp.StatusCode)
	}
}

// Правка одного разворота: ссылка та же, загружаются 2 страницы.
func TestShareUpdateSameLink(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	first := r.state().Link
	r.setJob(testJob(t, r.curJob().Path, 3, 0))
	if k := r.state().Link.State; k != "ok" {
		t.Fatalf("тот же альбом: %s", k)
	}
	sp := r.curJob().Spreads
	sp[1].Bg[0].B, sp[1].Bg[1].B = 250, 251
	r.setJob(jobOf(t, r.curJob().Path, sp))
	if k := r.state().Link.State; k != "changed" {
		t.Fatalf("после правки: %s", k)
	}
	before := r.srv.Fake.Puts()
	r.must(Options{}) // 0: срок и пароль прежние
	if got := r.srv.Fake.Puts() - before; got != 2 {
		t.Errorf("загружено %d страниц, ждали 2", got)
	}
	l := r.state().Link
	if l.State != "ok" || l.URL != first.URL || l.Password != first.Password || l.ExpiresAt != first.ExpiresAt {
		t.Errorf("ссылка изменилась: было %+v стало %+v", first, l)
	}
}

// Обрыв после k страниц: запись incomplete с id, «Продолжить» догружает только остаток.
func TestShareResume(t *testing.T) {
	r := newRig(t)
	r.connect()
	var puts atomic.Int32
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) && puts.Add(1) > 2 {
			return &albumstest.Fault{Status: http.StatusRequestEntityTooLarge}
		}
		return nil
	})
	msg, stopped := r.run(Options{Days: 30})
	if msg == "" || stopped {
		t.Fatalf("ждали ошибку: %q %v", msg, stopped)
	}
	v := r.state()
	if v.Link == nil || v.Link.State != "incomplete" || v.Link.URL != "" {
		t.Fatalf("после обрыва: %+v", v.Link)
	}
	id, _ := r.rec()
	if r.srv.Fake.Published(id) != nil || r.tempLeft() != 0 {
		t.Error("опубликовано при обрыве или осталась временная папка")
	}
	have := r.srv.Fake.Puts()
	r.setFault(nil)
	r.must(Options{})
	if got := r.srv.Fake.Puts() - have; got != 6-have {
		t.Errorf("догружено %d страниц, ждали %d", got, 6-have)
	}
	if id2, _ := r.rec(); id2 != id || r.state().Link.State != "ok" {
		t.Error("ссылка не та или не опубликована")
	}
}

// Остановка: публикации нет, временная папка удалена, запись incomplete.
func TestShareStop(t *testing.T) {
	r := newRig(t)
	r.connect()
	var once atomic.Bool
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) && once.CompareAndSwap(false, true) {
			r.svc.Cancel()
			return &albumstest.Fault{Drop: true}
		}
		return nil
	})
	msg, stopped := r.run(Options{Days: 30})
	if msg != "" || !stopped {
		t.Fatalf("итог: %q stopped=%v", msg, stopped)
	}
	id, ok := r.rec()
	if !ok || r.srv.Fake.Published(id) != nil || r.tempLeft() != 0 || r.state().Link.State != "incomplete" {
		t.Errorf("после остановки: запись %v, временных %d", ok, r.tempLeft())
	}
	if r.svc.Busy() {
		t.Error("флаг выгрузки не снят")
	}
}

// 429 ждёт Retry-After, 5xx — 2 с; страницы доходят.
func TestShareRetry(t *testing.T) {
	r := newRig(t)
	r.connect()
	var n atomic.Int32
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) {
			return nil
		}
		switch n.Add(1) {
		case 1:
			return &albumstest.Fault{Status: http.StatusTooManyRequests, RetryAfter: 7}
		case 2:
			return &albumstest.Fault{Status: http.StatusBadGateway}
		}
		return nil
	})
	r.must(Options{Days: 30})
	var has7, has2 bool
	for _, d := range r.waits {
		has7 = has7 || d == 7*time.Second
		has2 = has2 || d == 2*time.Second || d == 4*time.Second // 5xx: 2 с, а на второй попытке той же страницы — 4 с
	}
	if !has7 || !has2 || r.srv.Fake.Puts() != 6 {
		t.Errorf("паузы %v, загрузок %d", r.waits, r.srv.Fake.Puts())
	}
	// 401 на странице — без повторов.
	if _, ok := retryDelay(errTestAPI(401), 0); ok {
		t.Error("401 повторяется")
	}
}

// 404 на новую редакцию: запись «нет на сервере», вопрос, «Да» — новая ссылка, «Нет» — остановка.
func TestShareGone(t *testing.T) {
	for _, yes := range []bool{true, false} {
		r := newRig(t)
		r.connect()
		r.must(Options{Days: 30})
		old := r.state().Link.URL
		r.setFault(func(q *http.Request) *albumstest.Fault {
			if q.Method == http.MethodPut && !isPut(q) {
				return &albumstest.Fault{Status: http.StatusNotFound}
			}
			return nil
		})
		sp := r.curJob().Spreads
		sp[0].Bg[0].B = 99
		r.setJob(jobOf(t, r.curJob().Path, sp))
		r.answer = yes
		msg, stopped := r.run(Options{})
		if len(r.asks) != 1 || r.asks[0] != goneQuestion {
			t.Fatalf("вопросы: %q", r.asks)
		}
		l := r.state().Link
		if yes {
			if msg != "" || stopped {
				t.Fatalf("«Да»: %q %v", msg, stopped)
			}
			if l.URL == old || l.State != "ok" {
				t.Errorf("«Да»: ссылка %q (прежняя %q), %s", l.URL, old, l.State)
			}
		} else if !stopped || l.State != "gone" || l.URL != old {
			t.Errorf("«Нет»: stopped=%v, %+v", stopped, l)
		}
	}
}

// Правка срока и пароля: PATCH, при смене пароля revoke_sessions; пустая правка запроса не шлёт.
func TestShareChange(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Password: "pass-1234", Days: 30})
	var (
		bmu    sync.Mutex // крючок идёт в потоке сервера, тест читает из своего
		bodies []string
	)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPatch {
			b := bodyOf(q)
			bmu.Lock()
			bodies = append(bodies, b)
			bmu.Unlock()
		}
		return nil
	})
	bs := func() []string { bmu.Lock(); defer bmu.Unlock(); return append([]string(nil), bodies...) }
	if _, err := r.svc.Change(Change{}); err != nil || len(bs()) != 0 {
		t.Fatalf("холостая правка: %v, запросов %d", err, len(bs()))
	}
	v, err := r.svc.Change(Change{Days: 365, SetPassword: true, Password: "new-5678"})
	if err != nil || v.Link.Password != "new-5678" || len(bs()) != 1 || !strings.Contains(bs()[0], `"revoke_sessions":true`) {
		t.Fatalf("смена пароля и срока: %v %+v %q", err, v.Link, bs())
	}
	if _, err := r.svc.Change(Change{Days: 7}); err == nil {
		t.Error("срок 7 дней принят")
	}
	if _, err := r.svc.Change(Change{SetPassword: true, Password: "a"}); err == nil {
		t.Error("пароль из одного знака принят")
	}
	v, err = r.svc.Change(Change{SetPassword: true}) // снять пароль
	if err != nil || v.Link.Password != "" {
		t.Errorf("снять пароль: %v %+v", err, v.Link)
	}
	// Несколько запросов подряд: «Продлить» без пароля пароль не трогает.
	if v, err = r.svc.Change(Change{Days: 30}); err != nil || v.Link.Password != "" || strings.Contains(bs()[len(bs())-1], "password") {
		t.Errorf("продление: %v %+v %q", err, v.Link, bs()[len(bs())-1])
	}
}

// Отзыв: «Нет» — ничего, «Да» — DELETE и запись удалена.
func TestShareRevoke(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	r.answer = false
	v, err := r.svc.Revoke()
	if err != nil || v.Link == nil || r.srv.Fake.Albums() != 1 {
		t.Fatalf("«Нет»: %v %+v, альбомов %d", err, v.Link, r.srv.Fake.Albums())
	}
	r.answer = true
	v, err = r.svc.Revoke()
	if err != nil || v.Link != nil || r.srv.Fake.Albums() != 0 {
		t.Fatalf("«Да»: %v %+v, альбомов %d", err, v.Link, r.srv.Fake.Albums())
	}
	if r.asks[0] != revokeQuestion {
		t.Errorf("вопрос: %q", r.asks[0])
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("отзыв без ссылки принят")
	}
}

// Отказы до отправки: не сохранён, причина снимка, не подключён, нет токена, идёт экспорт, неверные настройки.
func TestShareRefusals(t *testing.T) {
	r := newRig(t)
	ok := r.curJob()
	try := func(name, want string, o Options) {
		t.Helper()
		started, err := r.svc.Share(o)
		if started || err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v %v, ждали «%s»", name, started, err, want)
		}
		if r.svc.Busy() {
			t.Fatalf("%s: флаг выгрузки остался", name)
		}
	}
	try("не подключён", "не подключён", Options{Days: 30})
	r.connect()
	r.setJob(share0("/x/a.salbum", "Сначала сохраните альбом", true))
	try("не сохранён", "Сначала сохраните", Options{Days: 30})
	if v := r.state(); !v.Album.Save || v.Album.Block == "" {
		t.Errorf("состояние: %+v", v.Album)
	}
	r.setJob(share0("/x/a.salbum", "Лист виньетки нельзя отправить по ссылке", false))
	try("лист 031", "Лист виньетки", Options{Days: 30})
	r.setJob(share0("/x/a.salbum", "В альбоме нет разворотов", false))
	try("нет разворотов", "нет разворотов", Options{Days: 30})
	j0 := ok
	j0.Refuse = "Нет фото: IMG_1.jpg"
	r.setJob(j0)
	try("пропавшие фото", "Нет фото", Options{Days: 30})
	r.setJob(ok)
	r.mu.Lock()
	r.exportB = true
	r.mu.Unlock()
	try("идёт экспорт", "идёт экспорт", Options{Days: 30})
	r.mu.Lock()
	r.exportB = false
	r.mu.Unlock()
	try("срок", "месяц", Options{Days: 7})
	try("новая ссылка без срока", "Выберите срок", Options{})
	try("пароль", "пароль", Options{Days: 30, Password: "x"})
	// Вопрос «Нет»: ничего не отправлено и не создано.
	j := ok
	j.Questions = func() []string { return []string{"Пустые поля. Продолжить?"} }
	r.setJob(j)
	r.answer = false
	if started, err := r.svc.Share(Options{Days: 30}); started || err != nil || r.srv.Fake.Albums() != 0 || r.tempLeft() != 0 {
		t.Errorf("«Нет» на вопрос: %v %v, альбомов %d", started, err, r.srv.Fake.Albums())
	}
	if len(r.asks) != 1 {
		t.Errorf("вопросов %d", len(r.asks))
	}
}
