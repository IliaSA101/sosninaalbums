package sharer

// 167 T3 (часть 2б): ветки поиска и записи после доправки по ревью — чужая запись «без смысла»,
// отказ записи найденного, отказы Share пока идёт поиск или правка, пароль при находке, регистр
// пути (Settled), адрес записи при обновлении по списку.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// onlyToken — запросы с токеном были (не только health и list) и все — с токеном token.
func (h *headerLog) onlyToken(t *testing.T, token string) {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n == 0 {
		t.Error("запросов к альбому не было")
	}
	for _, a := range h.auth {
		if a != "" && a != "Bearer "+token { // health идёт без токена
			t.Errorf("запрос с чужим заголовком Authorization: %q", a)
		}
	}
}

// findFor — Find с проверкой ошибки; возвращает Album.Find.
func (r *rig) findFor() string {
	r.t.Helper()
	v, err := r.svc.Find()
	if err != nil {
		r.t.Fatalf("Find: %v", err)
	}
	return v.Album.Find
}

// Чужая запись, которую окно не видит (classify: «записи нет»), ничего не говорит о поиске: альбом
// на сервисе есть, но привязка не нашлась бы — итог «lost», а не «done» с пустым окном.
func TestShareFindForeignEmptyIsLost(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	if err := r.svc.drop(uid); err != nil {
		t.Fatal(err)
	}
	// Пустая чужая запись на диск не ложится (prefs отбрасывает её), живёт только в памяти.
	foreign := prefs.ProjectRecord{Path: filepath.Join(filepath.Dir(r.curJob().Path), "другой.salbum")}
	r.svc.mu.Lock()
	r.svc.mem[uid] = foreign
	r.svc.fnd = map[string]findState{}
	r.svc.mu.Unlock()
	if _, _, _, k := r.svc.classify(r.curJob(), r.srv.URL); k != "none" {
		t.Fatalf("оснастка: состояние чужой записи %q, ждали none", k)
	}
	if got := r.findFor(); got != findLost {
		t.Fatalf("Find: %q, ждали %q", got, findLost)
	}
	if rec, _ := r.svc.look(uid); rec.ID != "" || rec.Path != foreign.Path {
		t.Errorf("поиск тронул чужую запись (альбом %q): %+v", id, rec)
	}
}

// Найденный альбом не записан ни на диск, ни в память (негодный отпечаток): «done» оставил бы
// Create с тем же uid — итог «lost». Отказ только диска (запись в памяти) остаётся «done».
func TestShareFindKeepFailLost(t *testing.T) {
	t.Run("нигде", func(t *testing.T) {
		r := newRig(t)
		id, _ := r.upload("", "")
		bad := "не-отпечаток"
		j := r.curJob()
		j.UID, j.Refs = bad, []string{"ref-" + id}
		r.setJob(j)
		r.svc.h.LinkRef = func(_, id string) string { return "ref-" + id } // у негодного uid настоящий LinkRef пуст
		if got := r.findFor(); got != findLost {
			t.Fatalf("Find: %q, ждали %q", got, findLost)
		}
		if _, ok := r.svc.look(bad); ok {
			t.Error("запись появилась")
		}
	})
	t.Run("только диск", func(t *testing.T) {
		r := newRig(t)
		id, uid := r.upload("", "")
		r.secondComputer(uid)
		// projects.json — папка: прочитать и записать нельзя, запись остаётся в памяти до выхода.
		file := filepath.Join(r.cfg, "projects.json")
		if err := os.RemoveAll(file); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(file, 0o700); err != nil {
			t.Fatal(err)
		}
		if got := r.findFor(); got != "" {
			t.Fatalf("Find: %q, запись в памяти — ждали «done» (пусто)", got)
		}
		if rec, ok := r.svc.look(uid); !ok || rec.ID != id {
			t.Errorf("найденная запись не в памяти: %+v %v", rec, ok)
		}
	})
}

// holdGate — крючок сервера: первый подходящий запрос ждёт release (сервер занят «долгим» ответом).
type holdGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	stop    sync.Once
}

func newHold() *holdGate {
	return &holdGate{entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *holdGate) hold(q *http.Request, match func(*http.Request) bool) *albumstest.Fault {
	if match(q) {
		g.once.Do(func() { close(g.entered); <-g.release })
	}
	return nil
}

func (g *holdGate) open() { g.stop.Do(func() { close(g.release) }) }

func (g *holdGate) wait(t *testing.T) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("запрос не дошёл до сервера")
	}
}

// Share во время поиска отвечает про поиск, во время правки ссылки — про правку (holdNet text);
// после конца обоих выгрузка идёт.
func TestShareRefusedWhileNetBusy(t *testing.T) {
	isList := func(q *http.Request) bool { return q.Method == http.MethodGet && q.URL.Path == "/v1/albums" }
	isPatch := func(q *http.Request) bool { return q.Method == http.MethodPatch }
	cases := []struct {
		name  string
		match func(*http.Request) bool
		start func(r *rig) func() error
		want  string
	}{
		{"поиск", isList, func(r *rig) func() error {
			j := r.curJob()
			j.Refs = []string{model.LinkRef(j.UID, "x")}
			r.setJob(j)
			return func() error { _, err := r.svc.Find(); return err }
		}, "Идёт поиск альбома на сервисе — дождитесь его конца"},
		{"правка", isPatch, func(r *rig) func() error {
			return func() error { _, err := r.svc.Change(Change{Days: 90}); return err }
		}, "Идёт изменение ссылки — дождитесь его конца"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.upload("", "")
			g := newHold()
			t.Cleanup(g.open)
			act := c.start(r)
			r.setFault(func(q *http.Request) *albumstest.Fault { return g.hold(q, c.match) })
			res := make(chan error, 1)
			go func() { res <- act() }()
			g.wait(t)
			err := r.shareErr(Options{Days: 30})
			if err == nil || err.Error() != c.want {
				t.Errorf("Share во время «%s»: %v, ждали %q", c.name, err, c.want)
			}
			g.open()
			select {
			case err := <-res:
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
			case <-time.After(testkit.Limit(10 * time.Second)):
				t.Fatal("действие не закончилось")
			}
			r.setFault(nil)
			r.must(Options{}) // флаг отпущен: выгрузка идёт
		})
	}
}

// Находка альбома с паролем: пароль неизвестен, но обновление по ссылке его не трогает — на сервер
// не уходит ни одного PATCH, запись остаётся «пароль неизвестен».
func TestShareFindKeepsPassword(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "pass-1234")
	r.secondComputer(uid)
	get := reqLog(r, nil, nil)
	if got := r.findFor(); got != "" {
		t.Fatalf("Find: %q", got)
	}
	changeScene(t, r)
	r.must(Options{Choice: "update"})
	r.must(Options{})
	for _, line := range get() {
		if strings.HasPrefix(line, "PATCH") {
			t.Errorf("на сервер ушёл %s", line)
		}
	}
	rec, _ := r.svc.look(uid)
	if rec.ID != id || rec.Password != "" || !rec.PassUnknown {
		t.Errorf("запись после находки и обновления: %+v", rec)
	}
}

// refresh: пароль записи сверяется со списком сервиса — снят на сервисе (запись его забывает),
// появился (запись «неизвестен»), не менялся (свой пароль остаётся).
func TestShareFindRefreshPassword(t *testing.T) {
	cases := []struct {
		name        string
		serverPass  string
		local       prefs.ProjectRecord // что лежало в записи до поиска (поверх выгруженной)
		wantPass    string
		wantUnknown bool
	}{
		{"снят на сервисе", "", prefs.ProjectRecord{Password: "old-1234"}, "", false},
		{"снят, был «неизвестен»", "", prefs.ProjectRecord{PassUnknown: true}, "", false},
		{"поставлен на сервисе", "pass-1234", prefs.ProjectRecord{}, "", true},
		{"не менялся", "pass-1234", prefs.ProjectRecord{Password: "pass-1234"}, "pass-1234", false},
		{"неизвестен остаётся", "pass-1234", prefs.ProjectRecord{PassUnknown: true}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			_, uid := r.upload("", c.serverPass)
			rec, _ := r.svc.look(uid)
			rec.Password, rec.PassUnknown = c.local.Password, c.local.PassUnknown
			if err := r.svc.put(uid, rec); err != nil {
				t.Fatal(err)
			}
			if got := r.findFor(); got != "" {
				t.Fatalf("Find: %q", got)
			}
			got, _ := r.svc.look(uid)
			if got.Password != c.wantPass || got.PassUnknown != c.wantUnknown {
				t.Errorf("пароль %q, неизвестен %v; ждали %q, %v", got.Password, got.PassUnknown, c.wantPass, c.wantUnknown)
			}
		})
	}
}

// caseVariant — тот же файл в другом регистре имени.
func caseVariant(path string) string {
	return filepath.Join(filepath.Dir(path), strings.ToUpper(filepath.Base(path)))
}

// Регистр пути записи переписывает Settled (открытие, когда альбом уже текущий); State запись не
// меняет. Где регистр значим (Linux), записи другого регистра свои не бывают — путь не трогается.
func TestSettledFixesCaseStateDoesNot(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("", "")
	path := r.curJob().Path
	variant := caseVariant(path)
	rec, _ := r.svc.look(uid)
	rec.Path = variant
	if err := r.svc.put(uid, rec); err != nil {
		t.Fatal(err)
	}
	pathOf := func() string { got, _ := r.svc.look(uid); return got.Path }

	r.state()
	r.state()
	if got := pathOf(); got != variant {
		t.Fatalf("State переписал путь: %q (было %q)", got, variant)
	}
	r.svc.Settled(filepath.Join(filepath.Dir(path), "другой.salbum"), uid) // не своя запись
	r.svc.Settled(path, model.NewUID())                                    // записи нет
	if got := pathOf(); got != variant {
		t.Fatalf("Settled чужого пути переписал: %q", got)
	}
	r.svc.Settled(path, uid)
	want := variant
	if paths.Fold("A") != "A" { // регистр не значим: запись своя, путь — настоящий
		want = path
	}
	if got := pathOf(); got != want {
		t.Errorf("после Settled путь %q, ждали %q", got, want)
	}
}

// Список сервиса говорит о записи подключённого адреса: запись другого адреса (сервис переехал) с
// тем же id не обновляется по чужому списку — ни срок, ни «нет на сервере», ни ссылка.
func TestShareFindRefreshSkipsOtherOrigin(t *testing.T) {
	r := newRig(t)
	id, uid := r.upload("", "")
	foreign, hits := foreignServer(t)
	rec, _ := r.svc.look(uid)
	rec.Origin, rec.URL, rec.Gone = foreign, foreign+"/a/"+id+"/", true
	rec.ExpiresAt = r.svc.now().AddDate(0, 0, 10)
	if err := r.svc.put(uid, rec); err != nil {
		t.Fatal(err)
	}
	before, _ := r.svc.look(uid)
	if got := r.findFor(); got != "" {
		t.Fatalf("Find: %q", got)
	}
	after, _ := r.svc.look(uid)
	if !after.Gone || after.Origin != foreign || after.URL != before.URL || !after.ExpiresAt.Equal(before.ExpiresAt) {
		t.Errorf("запись другого адреса обновлена по списку: было %+v, стало %+v", before, after)
	}
	if hits.Load() != 0 {
		t.Errorf("запросов на чужой адрес: %d", hits.Load())
	}
}
