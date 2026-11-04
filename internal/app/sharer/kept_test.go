package sharer

// Сохранённые страницы «Продолжить» (159): повтор без рендера, подмена и устаревание, жизнь слота.

import (
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// keptDirs — имена папок kp-* в кэше.
func (r *rig) keptDirs() []string {
	list, _ := os.ReadDir(filepath.Join(r.cache, "share"))
	var out []string
	for _, e := range list {
		if strings.HasPrefix(e.Name(), keptPrefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

// keptLeft — число папок kp-* в кэше.
func (r *rig) keptLeft() int { return len(r.keptDirs()) }

// renders — сколько раз страницы рисовались: события share:progress фазы pages (шва RunWeb нет).
func (r *rig) renders() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if pe, ok := e.data[0].(progressEvent); ok && e.name == "share:progress" && pe.Phase == "pages" {
			n++
		}
	}
	return n
}

// pageSet — номера страниц, которые сервер получал (крючок идёт в потоке сервера, поэтому замок).
type pageSet struct {
	mu sync.Mutex
	m  map[int]bool
}

func (p *pageSet) add(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[int]bool{}
	}
	p.m[n] = true
}

func (p *pageSet) has(n int) bool { p.mu.Lock(); defer p.mu.Unlock(); return p.m[n] }

func (p *pageSet) list() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Sorted(maps.Keys(p.m))
}

// watchPages записывает номера всех PUT страниц и ничего не ломает.
func watchPages(r *rig) *pageSet {
	ps := &pageSet{}
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) {
			ps.add(pageNo(q))
		}
		return nil
	})
	return ps
}

func priorLists(prior []*pageSet) [][]int {
	out := make([][]int, len(prior))
	for i, ps := range prior {
		out[i] = ps.list()
	}
	return out
}

// wantResumed требует, что «Продолжить» догрузило ровно недостающее: номера догруженных не
// пересекаются с принятыми раньше, а вместе они дают total страниц. Считаем набор номеров, а не
// разность Fake.Puts(): летящий PUT, который отменили, сервер может дописать позже, а повтор на
// оборванном соединении даёт дубль — оба случая безвредны для продукта.
func wantResumed(t *testing.T, total int, resumed *pageSet, prior ...*pageSet) {
	t.Helper()
	all := map[int]bool{}
	for _, n := range resumed.list() {
		all[n] = true
	}
	for _, ps := range prior {
		for _, n := range ps.list() {
			if resumed.has(n) {
				t.Errorf("страница %d уже была на сервере, а загружена снова", n)
			}
			all[n] = true
		}
	}
	if len(all) != total {
		t.Errorf("страниц всего %d, ждали %d (до обрыва %v, догружено %v)", len(all), total, priorLists(prior), resumed.list())
	}
}

// uploadTotal — сколько страниц ход выгружает (последнее событие фазы upload; шлётся до первого PUT).
func (r *rig) uploadTotal() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if pe, ok := r.events[i].data[0].(progressEvent); ok && r.events[i].name == "share:progress" && pe.Phase == "upload" {
			return pe.Total
		}
	}
	return 0
}

// cutAfter обрывает загрузку после n принятых страниц (413 без повторов); страницы идут тремя
// потоками, поэтому счёт по порядку прихода, а не по номеру. Первый 413 отдаётся, лишь когда сервер
// принял все n страниц и все остальные летящие PUT уже стоят в крючке: отмена выгрузки иначе
// отрезала бы принимаемую страницу либо оставила запоздалый PUT, который сервер примет под чужим
// крючком и испортит счёт «Продолжить». Возвращает номера пропущенных страниц.
func cutAfter(r *rig, n int32) *pageSet {
	var (
		puts     atomic.Int32
		mu       sync.Mutex
		blocked  int
		released bool
	)
	base := r.srv.Fake.Puts()
	passed := &pageSet{}
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) {
			return nil
		}
		if puts.Add(1) <= n {
			passed.add(pageNo(q))
			return nil
		}
		hold := min(uploadWorkers, r.uploadTotal()-int(n)) // PUT, которые клиент держит в полёте
		mu.Lock()
		blocked++
		mu.Unlock()
		end := time.Now().Add(testkit.Limit(10 * time.Second))
		for time.Now().Before(end) {
			if r.srv.Fake.Puts() >= base+int(n) {
				mu.Lock()
				ok := released || blocked >= hold
				released = released || ok
				mu.Unlock()
				if ok {
					break
				}
			}
			time.Sleep(time.Millisecond)
		}
		return &albumstest.Fault{Status: http.StatusRequestEntityTooLarge}
	})
	return passed
}

// settle гасит запоздалые PUT оборванной выгрузки: клиент отменил летящие запросы, но сервер может
// дочитать их из буфера позже и под уже другим крючком, принять и записать в счёт «Продолжить».
// Закрываем соединения (непрочитанное пропадает), а до следующего крючка или начала нового хода
// (первый запрос хода — health) любой PUT получает 413.
func settle(r *rig) {
	r.srv.CloseClientConnections()
	r.setFault(func(q *http.Request) *albumstest.Fault {
		switch {
		case isPut(q):
			return &albumstest.Fault{Status: http.StatusRequestEntityTooLarge}
		case q.Method == http.MethodGet && strings.HasSuffix(q.URL.Path, "/health"):
			r.setFault(nil)
		}
		return nil
	})
}

// cut — выгрузка, оборванная после n страниц; страницы остались в слоте.
func cut(t *testing.T, r *rig, o Options, n int32) *pageSet {
	t.Helper()
	passed := cutAfter(r, n)
	if msg, stopped := r.run(o); msg == "" || stopped {
		t.Fatalf("ждали обрыв: %q stopped=%v", msg, stopped)
	}
	if got := r.keptLeft(); got != 1 {
		t.Fatalf("после обрыва kp-*: %d, ждали 1", got)
	}
	settle(r)
	if got := len(passed.list()); got != int(n) {
		t.Fatalf("принято страниц %d, ждали %d", got, n)
	}
	return passed
}

// «Продолжить» без правок альбома: страницы не рисуются, грузятся только недостающие; второй
// обрыв подряд оставляет ту же папку.
func TestShareResumeReusesPages(t *testing.T) {
	r := newRig(t)
	r.connect()
	a := cut(t, r, Options{Days: 30}, 2)
	first, rendered := r.keptDirs()[0], r.renders()
	if rendered == 0 {
		t.Fatal("первая попытка не рисовала страницы")
	}
	b := cut(t, r, Options{}, 1) // второй обрыв подряд
	if got := r.keptDirs(); len(got) != 1 || got[0] != first {
		t.Errorf("после второго обрыва kp-*: %v, ждали прежнюю %s", got, first)
	}
	c := watchPages(r)
	r.must(Options{})
	if r.renders() != rendered {
		t.Errorf("«Продолжить» рисовало страницы: событий %d → %d", rendered, r.renders())
	}
	wantResumed(t, 6, c, a, b)
	if id, _ := r.rec(); len(r.srv.Fake.Stored(id)) != 6 {
		t.Errorf("на сервере %d страниц, ждали 6", len(r.srv.Fake.Stored(id)))
	}
	if r.keptLeft() != 0 || r.tempLeft() != 0 || r.state().Link.State != "ok" {
		t.Errorf("после успеха: kp %d, up %d, %s", r.keptLeft(), r.tempLeft(), r.state().Link.State)
	}
}

// Обрыв «Обновить по ссылке» у готовой ссылки: повтор берёт сохранённые страницы.
func TestShareUpdateResumeReusesPages(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	sp := r.curJob().Spreads
	sp[1].Bg[0].B, sp[1].Bg[1].B = 250, 251
	r.setJob(jobOf(t, r.curJob().Path, sp))
	if r.keptLeft() != 0 {
		t.Fatal("слот после успеха")
	}
	a := cut(t, r, Options{}, 1)
	rendered := r.renders()
	c := watchPages(r)
	r.must(Options{})
	if r.renders() != rendered {
		t.Errorf("повтор рисовал страницы: %d → %d", rendered, r.renders())
	}
	wantResumed(t, 2, c, a) // изменённый разворот — две страницы, одна уже ушла
	if r.state().Link.State != "ok" || r.keptLeft() != 0 {
		t.Errorf("после повтора: %s, kp %d", r.state().Link.State, r.keptLeft())
	}
}

// Другой хэш сцены или поколение шрифтов: страницы готовятся заново, слот после этого пуст.
func TestShareResumeSceneChangedRenders(t *testing.T) {
	for name, change := range map[string]func(r *rig){
		"сцена": func(r *rig) {
			sp := r.curJob().Spreads
			sp[0].Bg[0].B = 99
			r.setJob(jobOf(t, r.curJob().Path, sp))
		},
		"шрифты": func(r *rig) { j := r.curJob(); j.Fonts++; r.setJob(j) },
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			r.connect()
			cut(t, r, Options{Days: 30}, 2)
			old, rendered := r.keptDirs()[0], r.renders()
			change(r)
			r.must(Options{})
			if r.renders() == rendered {
				t.Error("страницы не готовились заново")
			}
			if _, err := os.Stat(filepath.Join(r.cache, "share", old)); err == nil || r.keptLeft() != 0 {
				t.Errorf("старая kp-* осталась: kp %d, %v", r.keptLeft(), err)
			}
		})
	}
}

// pageFile — первый файл страницы в папке dir.
func pageFile(t *testing.T, dir string) string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil || len(list) == 0 {
		t.Fatalf("в сохранённой папке нет файлов: %v", err)
	}
	return filepath.Join(dir, list[0].Name())
}

// Повреждённая папка: нет файла страницы — строка в журнале, папка удалена, подготовка заново.
func TestShareResumeBrokenKeptRenders(t *testing.T) {
	r := newRig(t)
	r.connect()
	cut(t, r, Options{Days: 30}, 2)
	dir := filepath.Join(r.cache, "share", r.keptDirs()[0])
	if err := os.Remove(pageFile(t, dir)); err != nil {
		t.Fatal(err)
	}
	buf, rendered := captureLog(t), r.renders()
	r.must(Options{})
	if r.renders() == rendered {
		t.Error("страницы не готовились заново")
	}
	if !strings.Contains(buf.String(), "сохранённые страницы не годятся") {
		t.Errorf("нет строки в журнале:\n%s", buf.String())
	}
	if _, err := os.Stat(dir); err == nil || r.keptLeft() != 0 {
		t.Errorf("повреждённая папка осталась: kp %d, %v", r.keptLeft(), err)
	}
	if r.srv.Fake.Puts() != 6 || r.state().Link.State != "ok" {
		t.Errorf("загрузок %d, %s", r.srv.Fake.Puts(), r.state().Link.State)
	}
}

// Подменённый байт страницы между попытками: sha256 новой сборки не совпадает с манифестом первой
// подготовки — страницы готовятся заново, подменённый файл на сервер не уходит.
func TestShareResumeTamperedRenders(t *testing.T) {
	r := newRig(t)
	r.connect()
	cut(t, r, Options{Days: 30}, 2)
	dir := filepath.Join(r.cache, "share", r.keptDirs()[0])
	f := pageFile(t, dir)
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)/2] ^= 0xff // один байт внутри файла: заголовок цел, DecodeConfig не заметит
	if err := os.WriteFile(f, b, 0o600); err != nil {
		t.Fatal(err)
	}
	buf, rendered := captureLog(t), r.renders()
	r.must(Options{})
	if r.renders() == rendered || !strings.Contains(buf.String(), "сохранённые страницы не годятся") {
		t.Errorf("подмена не замечена: рендеров %d → %d\n%s", rendered, r.renders(), buf.String())
	}
	id, _ := r.rec()
	if r.srv.Fake.Published(id) == nil || len(r.srv.Fake.Stored(id)) != 6 || r.keptLeft() != 0 {
		t.Errorf("не опубликовано или страниц %d, kp %d", len(r.srv.Fake.Stored(id)), r.keptLeft())
	}
}

// Успех: слот удаляется.
func TestShareKeptDroppedOnSuccess(t *testing.T) {
	r := newRig(t)
	r.connect()
	cut(t, r, Options{Days: 30}, 2) // keptLeft()==1 проверил cut
	r.must(Options{})
	if r.keptLeft() != 0 {
		t.Errorf("после успеха kp-*: %d", r.keptLeft())
	}
	// Другая выгрузка с другим ключом тоже убирает чужой слот сразу.
	r2 := newRig(t)
	r2.connect()
	cut(t, r2, Options{Days: 30}, 2)
	job := testJob(t, filepath.Join(filepath.Dir(r2.curJob().Path), "b.salbum"), 3, 7)
	r2.setJob(job)
	// Чужой слот уходит сразу, в match, а не к концу хода (152-ФЗ): первый PUT нового хода держим
	// и в этот момент слота уже нет.
	old := r2.keptDirs()[0]
	var (
		hit  = make(chan struct{})
		free = make(chan struct{})
		once sync.Once
	)
	r2.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) {
			once.Do(func() { close(hit) })
			select {
			case <-free:
			case <-time.After(testkit.Limit(30 * time.Second)):
			}
		}
		return nil
	})
	r2.drainDone()
	if ok, err := r2.svc.Share(Options{Days: 30}); err != nil || !ok {
		t.Fatalf("Share: %v %v", ok, err)
	}
	select {
	case <-hit:
	case <-time.After(testkit.Limit(30 * time.Second)):
		close(free)
		t.Fatal("загрузка не началась")
	}
	_, statErr := os.Stat(filepath.Join(r2.cache, "share", old))
	gone := r2.keptLeft() == 0 && statErr != nil
	close(free)
	if msg, stopped := r2.wait(); msg != "" || stopped {
		t.Fatalf("выгрузка закончилась не успехом: %q stopped=%v", msg, stopped)
	}
	if !gone {
		t.Errorf("чужой слот жив, пока идёт загрузка: kp %d, %v", r2.keptLeft(), statErr)
	}
	if r2.keptLeft() != 0 {
		t.Errorf("слот другого альбома остался: %d", r2.keptLeft())
	}
}

// «Отозвать» после обрыва «Обновить по ссылке»: слот удаляется.
func TestShareKeptDroppedOnRevoke(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	sp := r.curJob().Spreads
	sp[1].Bg[0].B, sp[1].Bg[1].B = 250, 251
	r.setJob(jobOf(t, r.curJob().Path, sp))
	cut(t, r, Options{}, 1)
	r.answer = true
	if v, err := r.svc.Revoke(); err != nil || v.Link != nil {
		t.Fatalf("Revoke: %v %+v", err, v.Link)
	}
	if r.keptLeft() != 0 {
		t.Errorf("после отзыва kp-*: %d", r.keptLeft())
	}
}

// Новый проект в том же пути убирает слот; сохранение того же проекта и другой путь — нет.
func TestShareKeptDroppedOnNewProject(t *testing.T) {
	r := newRig(t)
	r.connect()
	cut(t, r, Options{Days: 30}, 2)
	path := r.curJob().Path
	r.svc.OnSaved(path, false)
	r.svc.OnSaved(filepath.Join(filepath.Dir(path), "other.salbum"), true)
	r.svc.OnSaved("", true)
	if r.keptLeft() != 1 {
		t.Fatalf("слот тронут чужим сохранением: %d", r.keptLeft())
	}
	r.svc.OnSaved(path, true)
	if r.keptLeft() != 0 {
		t.Errorf("после нового проекта kp-*: %d", r.keptLeft())
	}
}

// Выход во время выгрузки: слот есть, ход висит на загрузке; после Shutdown нет ни kp-*, ни up-*,
// а ход, который закончится позже, ничего не сохраняет.
func TestShareKeptDroppedOnShutdown(t *testing.T) {
	r := newRig(t)
	r.connect()
	cut(t, r, Options{Days: 30}, 2)
	var (
		hit  = make(chan struct{})
		free = make(chan struct{})
		once sync.Once
	)
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if !isPut(q) {
			return nil
		}
		once.Do(func() { close(hit) })
		select {
		case <-free:
		case <-time.After(testkit.Limit(30 * time.Second)):
		}
		return &albumstest.Fault{Drop: true}
	})
	r.drainDone()
	if ok, err := r.svc.Share(Options{}); err != nil || !ok {
		t.Fatalf("Share: %v %v", ok, err)
	}
	select {
	case <-hit:
	case <-time.After(testkit.Limit(30 * time.Second)):
		t.Fatal("загрузка не началась")
	}
	r.svc.Shutdown()
	if runtime.GOOS != "windows" && r.keptLeft() != 0 { // на Windows открытый файл не удалить до конца хода
		t.Errorf("после Shutdown kp-*: %d", r.keptLeft())
	}
	close(free)
	r.wait()
	if r.keptLeft() != 0 || r.tempLeft() != 0 {
		t.Errorf("после конца хода kp-* %d, up-* %d", r.keptLeft(), r.tempLeft())
	}
	// Ход, начатый после выхода, слота не заводит.
	r.setFault(nil)
	cutAfter(r, 1)
	r.run(Options{Days: 30})
	if r.keptLeft() != 0 {
		t.Errorf("слот заведён после Shutdown: %d", r.keptLeft())
	}
}

// Тот же пароль: Change ничего не шлёт и не сбрасывает входы клиента; с новым сроком уходит только срок.
func TestChangeSamePasswordNoop(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Password: "pass-1234", Days: 30})
	var (
		bmu    sync.Mutex // крючок идёт в потоке сервера
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
	got := func() []string { bmu.Lock(); defer bmu.Unlock(); return append([]string(nil), bodies...) }
	before := r.state().Link
	v, err := r.svc.Change(Change{SetPassword: true, Password: "pass-1234"})
	if err != nil || len(got()) != 0 || v.Link == nil || *v.Link != *before {
		t.Fatalf("тот же пароль без срока: %v, PATCH %q, ссылка %+v → %+v", err, got(), before, v.Link)
	}
	// Пароля не было и не задан: тоже холостой.
	r2 := newRig(t)
	r2.connect()
	r2.must(Options{Days: 30})
	var patches2 atomic.Int32
	r2.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPatch {
			patches2.Add(1)
		}
		return nil
	})
	if _, err := r2.svc.Change(Change{SetPassword: true}); err != nil {
		t.Fatalf("пустой пароль при пустом: %v", err)
	}
	if n := patches2.Load(); n != 0 {
		t.Errorf("пустой пароль при пустом: ушло %d PATCH, ждали 0", n)
	}
	// Тот же пароль и срок: в теле только срок.
	if _, err := r.svc.Change(Change{SetPassword: true, Password: "pass-1234", Days: 90}); err != nil {
		t.Fatal(err)
	}
	b := got()
	if len(b) != 1 || !strings.Contains(b[0], "expires_at") || strings.Contains(b[0], "password") || strings.Contains(b[0], "revoke_sessions") {
		t.Errorf("тело PATCH: %q", b)
	}
	// Другой пароль — как раньше: пароль и сброс входов.
	if _, err := r.svc.Change(Change{SetPassword: true, Password: "other-5678"}); err != nil {
		t.Fatal(err)
	}
	if b = got(); len(b) != 2 || !strings.Contains(b[1], `"revoke_sessions":true`) {
		t.Errorf("смена пароля: %q", b)
	}
}
