package sharer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// Тесты по ревью и аудиту 0.32-146-T4-8: запись не воскресает, 404 не от альбома, флаги хода.

// freezeShares делает projects.json «новее программы»: записи читаются, стереть или записать нельзя.
func freezeShares(t *testing.T, cfg string) {
	t.Helper()
	p := prefs.ProjectsPath(cfg)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b = regexp.MustCompile(`"version":\s*\d+`).ReplaceAll(b, []byte(`"version": 999`))
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func isHealth(q *http.Request) bool { return strings.HasSuffix(q.URL.Path, "/health") }

// Аудит п. 1: «Отозвать», а projects.json не пишется (стереть запись нельзя) — запись отозванного
// альбома не возвращается, «Обновить по ссылке» не шлёт страницы в Update старого id.
func TestShareDropNotWritten(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	freezeShares(t, r.cfg)
	uid := r.curJob().UID
	if _, ok := r.svc.look(uid); !ok {
		t.Fatal("оснастка: запись должна читаться из замороженного файла")
	}
	if _, err := r.svc.Revoke(); err == nil || !strings.Contains(err.Error(), "убрать не удалось") {
		t.Fatalf("Revoke при замороженном файле: %v", err)
	}
	h := &headerLog{}
	r.setFault(h.fault)
	if _, ok := r.svc.look(uid); ok {
		t.Fatal("запись пережила отзыв: стереть на диске не вышло, метки нет")
	}
	if v := r.state(); v.Link != nil {
		t.Errorf("окно показывает прежнюю ссылку: %+v", v.Link)
	}
	if _, err := r.svc.Text(); err == nil {
		t.Error("«Скопировать» отдало ссылку отозванного альбома")
	}
	ok, err := r.svc.Share(Options{}) // «Обновить по ссылке»
	if ok || err == nil {
		t.Fatalf("Days:0 без записи: %v %v", ok, err)
	}
	if h.n != 0 {
		t.Errorf("к сервису ушло %d запросов по записи отозванного альбома", h.n)
	}
}

// Метка «сброшена» снимается новой записью: ссылка нового альбома видна.
func TestShareDropMarkCleared(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	freezeShares(t, r.cfg)
	_, _ = r.svc.Revoke()              // запись не стёрлась: метка «сброшена»
	msg, _ := r.run(Options{Days: 30}) // запись нового ляжет только в память: projects.json заморожен
	if !strings.Contains(msg, "Ссылка готова, но записать её") {
		t.Fatalf("итог: %q", msg)
	}
	if v := r.state(); v.Link == nil || v.Link.State != "ok" {
		t.Errorf("новая ссылка не видна: %+v", v.Link)
	}
}

// Аудит п. 3: 404 на health и create — не «ссылки больше нет»; живая ссылка не помечается.
func TestShareNotFoundNotAlbum(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	sp := r.curJob().Spreads
	sp[0].Bg[0].B = 99
	r.setJob(jobOf(t, r.curJob().Path, sp)) // сцена изменилась: state changed
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isHealth(q) {
			return &albumstest.Fault{Status: http.StatusNotFound}
		}
		return nil
	})
	msg, _ := r.run(Options{})
	if !strings.Contains(msg, noServiceText) {
		t.Errorf("текст при 404 на health: %q", msg)
	}
	if l := r.state().Link; l == nil || l.State != "changed" {
		t.Errorf("живая ссылка после 404 на health: %+v", l)
	}
	if rec, _ := r.svc.look(r.curJob().UID); rec.Gone {
		t.Error("запись помечена «нет на сервере»")
	}

	// Create: новый альбом, 404 на POST /v1/albums — запись не создаётся.
	r2 := newRig(t)
	r2.connect()
	r2.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPost && strings.HasSuffix(q.URL.Path, "/v1/albums") {
			return &albumstest.Fault{Status: http.StatusNotFound}
		}
		return nil
	})
	msg, _ = r2.run(Options{Days: 30})
	if !strings.Contains(msg, noServiceText) || strings.Contains(msg, "больше нет") {
		t.Errorf("текст при 404 на create: %q", msg)
	}
	if _, ok := r2.rec(); ok {
		t.Error("запись появилась без создания")
	}
}

// Connect на адрес без сервиса: текст про адрес, не про ссылку (health и list).
func TestShareConnectNotFound(t *testing.T) {
	for _, onHealth := range []bool{true, false} {
		r := newRig(t)
		r.setFault(func(q *http.Request) *albumstest.Fault {
			if isHealth(q) == onHealth {
				return &albumstest.Fault{Status: http.StatusNotFound}
			}
			return nil
		})
		_, err := r.svc.Connect(r.srv.URL, r.srv.Token)
		if err == nil || !strings.Contains(err.Error(), noServiceText) || strings.Contains(err.Error(), "больше нет") {
			t.Errorf("health=%v: %v", onHealth, err)
		}
	}
}

// 404 на запрос по id остаётся «ссылки больше нет»; удачная правка пометку снимает.
func TestShareGoneByID(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	sp := r.curJob().Spreads
	sp[0].Bg[0].B = 99
	r.setJob(jobOf(t, r.curJob().Path, sp))
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) {
			return &albumstest.Fault{Status: http.StatusNotFound}
		}
		return nil
	})
	if msg, _ := r.run(Options{}); !strings.Contains(msg, "Ссылки больше нет") {
		t.Errorf("404 на страницу: %q", msg)
	}
	if l := r.state().Link; l == nil || l.State != "gone" {
		t.Fatalf("пометка не поставлена: %+v", l)
	}
	r.setFault(nil)
	if _, err := r.svc.Change(Change{Days: 90}); err != nil {
		t.Fatal(err)
	}
	if rec, _ := r.svc.look(r.curJob().UID); rec.Gone {
		t.Error("удачный Patch не снял пометку")
	}
}

// Аудит п. 5: опоздавший put не пишет, когда на путь легло другое сохранение (OnSaved), — запись
// остаётся прежней (она по uid и не стирается, 167-10).
func TestShareLatePutSkipped(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	path, uid := r.curJob().Path, r.curJob().UID
	run := &runState{path: path}
	r.svc.mu.Lock()
	r.svc.run = run
	r.svc.mu.Unlock()
	rec, _ := r.svc.look(uid)
	r.svc.OnSaved(path, true)
	late := rec
	late.Title = "опоздавший"
	wrote, err := r.svc.putIf(uid, late, r.svc.own(t.Context(), run))
	if wrote || err != nil {
		t.Errorf("putIf после OnSaved: %v %v", wrote, err)
	}
	if got, _ := r.svc.look(uid); got.Title != rec.Title {
		t.Errorf("опоздавшая запись легла: %q", got.Title)
	}
}

// Сохранение другого проекта, пока идут вопросы: старт отказывает, запросов нет, записи нет.
func TestShareSavedDuringQuestions(t *testing.T) {
	r := newRig(t)
	r.connect()
	job := r.curJob()
	job.Questions = func() []string { return []string{"Продолжить?"} }
	r.setJob(job)
	h := &headerLog{}
	r.setFault(h.fault)
	r.svc.h.Ask = func(string) (bool, error) { r.svc.OnSaved(job.Path, true); return true, nil }
	ok, err := r.svc.Share(Options{Days: 30})
	if ok || err == nil || !strings.Contains(err.Error(), "сохранён заново") {
		t.Fatalf("Share: %v %v", ok, err)
	}
	if h.n != 0 || r.svc.Busy() || r.tempLeft() != 0 || r.keptLeft() != 0 {
		t.Errorf("запросов %d, busy=%v, папок %d", h.n, r.svc.Busy(), r.tempLeft())
	}
	if _, ok := r.rec(); ok {
		t.Error("запись создана")
	}
}

// Аудит п. 6: пока идёт отзыв (вопрос открыт), выгрузка не стартует; Change и Revoke не идут во время выгрузки.
func TestShareNetFlag(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	var shareErr error
	r.svc.h.Ask = func(string) (bool, error) {
		_, shareErr = r.svc.Share(Options{Days: 30})
		if _, chErr := r.svc.Change(Change{Days: 90}); chErr == nil {
			t.Error("Change прошёл во время Revoke")
		}
		return false, nil
	}
	if _, err := r.svc.Revoke(); err != nil {
		t.Fatal(err)
	}
	if shareErr == nil || !strings.Contains(shareErr.Error(), "изменение ссылки") {
		t.Errorf("Share во время Revoke: %v", shareErr)
	}
	if _, err := r.svc.Change(Change{Days: 90}); err != nil { // флаг отпущен
		t.Errorf("Change после Revoke: %v", err)
	}

	g := newGate(t)
	r.setFault(g.fault)
	sp := r.curJob().Spreads
	sp[0].Bg[0].B = 77
	r.setJob(jobOf(t, r.curJob().Path, sp))
	if ok, err := r.svc.Share(Options{}); !ok || err != nil {
		t.Fatal(ok, err)
	}
	g.waitStart(t)
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Error("Change во время выгрузки")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("Revoke во время выгрузки")
	}
	g.free()
	r.wait()
}

// Аудит п. 7: путь берётся дёшево, без Plan; run.path стоит до Plan, и свой альбом не «чужой».
func TestShareCheapPath(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	plans := 0
	inner := r.svc.h.Plan
	var seen string
	r.svc.h.Plan = func() share.Job {
		plans++
		r.svc.mu.Lock()
		if r.svc.run != nil {
			seen = r.svc.run.path
		}
		r.svc.mu.Unlock()
		return inner()
	}
	if _, err := r.svc.Text(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Change(Change{}); err != nil {
		t.Fatal(err)
	}
	if plans != 1 { // Change без правки зовёт State, а тот — Plan один раз
		t.Errorf("Plan вызван %d раз при Text и Change", plans)
	}
	plans = 0
	r.must(Options{})
	if seen != r.curJob().Path {
		t.Errorf("run.path к началу Plan: %q", seen)
	}
}

// Первое подключение с неверным токеном: свой текст без «заново»; при выгрузке текст прежний.
func TestShareConnectWrongToken(t *testing.T) {
	r := newRig(t)
	_, err := r.svc.Connect(r.srv.URL, "x"+r.srv.Token[1:])
	if err == nil || err.Error() != connectRefusedText {
		t.Fatalf("Connect с чужим токеном: %v", err)
	}
	if got := errText(errTestAPI(http.StatusUnauthorized)); !strings.Contains(got, "заново") {
		t.Errorf("текст токена при выгрузке изменился: %q", got)
	}
}

// Критерий спеки: выгрузка, правка и отзыв не трогают файл альбома — байты те же (FormatVersion
// прежний), рядом не появляется новых файлов.
func TestShareLeavesAlbumFileIntact(t *testing.T) {
	r := newRig(t)
	r.connect()
	path := r.curJob().Path
	if err := salbum.Write(model.New(), path, "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	names := func() []string {
		list, _ := os.ReadDir(filepath.Dir(path))
		out := make([]string, len(list))
		for i, e := range list {
			out[i] = e.Name()
		}
		return out
	}
	was := names()
	r.must(Options{Days: 30})
	if _, err := r.svc.Change(Change{Days: 90, SetPassword: true, Password: "слово1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Revoke(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("файл альбома изменён выгрузкой: %v", err)
	}
	var head struct{ Version int }
	if err := json.Unmarshal(after, &head); err != nil || head.Version != model.FormatVersion {
		t.Errorf("версия формата: %d (ждали %d), %v", head.Version, model.FormatVersion, err)
	}
	if now := names(); len(now) != len(was) {
		t.Errorf("рядом с альбомом появились файлы: %v → %v", was, now)
	}
}
