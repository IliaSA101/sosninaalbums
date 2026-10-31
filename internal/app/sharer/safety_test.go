package sharer

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// gate держит первую загрузку страницы, пока тест не отпустит её.
type gate struct {
	once           atomic1
	started, freed chan struct{}
	release        sync.Once
}

// newGate — вентиль; при провале теста он всё равно отпускается в Cleanup (раньше закрытия сервера),
// иначе удержанный запрос вешает srv.Close.
func newGate(t *testing.T) *gate {
	g := &gate{started: make(chan struct{}), freed: make(chan struct{})}
	t.Cleanup(g.free)
	return g
}

func (g *gate) free() { g.release.Do(func() { close(g.freed) }) }

// waitStart ждёт, пока первая загрузка упрётся в вентиль, но не вечно.
func (g *gate) waitStart(t *testing.T) {
	t.Helper()
	select {
	case <-g.started:
	case <-time.After(testkit.Limit(30 * time.Second)):
		t.Fatal("загрузка не дошла до вентиля")
	}
}

func (g *gate) fault(q *http.Request) *albumstest.Fault {
	if isPut(q) && g.once.first() {
		close(g.started)
		<-g.freed
	}
	return nil
}

// В1: во время выгрузки A открыт альбом B — запись только у A, у B нет чужой ссылки и хода.
func TestShareSwitchAlbum(t *testing.T) {
	r := newRig(t)
	r.connect()
	pathA := r.curJob().Path
	g := newGate(t)
	r.setFault(g.fault)
	if ok, err := r.svc.Share(Options{Days: 30}); !ok || err != nil {
		t.Fatal(ok, err)
	}
	g.waitStart(t)
	if v := r.state(); !v.Busy || v.Other {
		t.Errorf("альбом A: busy=%v other=%v", v.Busy, v.Other)
	}
	pathB := filepath.Join(filepath.Dir(pathA), "b.salbum")
	r.setJob(testJob(t, pathB, 2, 7))
	v := r.state()
	if !v.Other || v.Busy || v.Link != nil {
		t.Errorf("альбом B: busy=%v other=%v link=%+v", v.Busy, v.Other, v.Link)
	}
	if _, err := r.svc.Share(Options{Days: 30}); err == nil || !strings.Contains(err.Error(), "другого альбома") {
		t.Errorf("выгрузка B во время A: %v", err)
	}
	if _, err := r.svc.Text(); err == nil {
		t.Error("текст клиенту выдан во время чужой выгрузки")
	}
	g.free()
	if msg, stopped := r.wait(); msg != "" || stopped {
		t.Fatalf("итог: %q %v", msg, stopped)
	}
	if _, ok := r.svc.look(pathB); ok {
		t.Error("запись появилась у альбома B")
	}
	if rec, ok := r.svc.look(pathA); !ok || rec.URL == "" || rec.Title != "a" {
		t.Errorf("запись A: %+v %v", rec, ok)
	}
	if v := r.state(); v.Link != nil || v.Other || v.Busy {
		t.Errorf("B после конца: %+v", v)
	}
}

// С1: сохранение другого проекта по пути с записью сбрасывает запись; тот же проект — нет.
func TestShareSaveAsOverwrite(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	path := r.curJob().Path
	r.svc.OnSaved(path, false)
	if _, ok := r.svc.look(path); !ok {
		t.Fatal("обычное сохранение сбросило запись")
	}
	r.svc.OnSaved(path, true)
	if _, ok := r.svc.look(path); ok {
		t.Error("запись пережила «Сохранить как» поверх файла")
	}
	if v := r.state(); v.Link != nil {
		t.Errorf("окно показывает чужую ссылку: %+v", v.Link)
	}
	// Идущая выгрузка этого пути останавливается.
	r2 := newRig(t)
	r2.connect()
	g := newGate(t)
	r2.setFault(g.fault)
	if ok, err := r2.svc.Share(Options{Days: 30}); !ok || err != nil {
		t.Fatal(ok, err)
	}
	g.waitStart(t)
	r2.svc.OnSaved(r2.curJob().Path, true)
	g.free()
	if _, stopped := r2.wait(); !stopped {
		t.Error("выгрузка перезаписанного пути не остановлена")
	}
}

// С2: токен строго по origin — пустой токен к новому адресу не берёт прежний, запросы по записи
// другого сервиса не идут, прежний токен после подключения к другому сервису удалён.
func TestShareConnectNoTokenReuse(t *testing.T) {
	r, b := newRig(t), newRig(t) // r — служба и сервер A; у b берём только сервер B
	var logA, logB headerLog
	r.setFault(logA.fault)
	b.setFault(logB.fault)
	r.connect()
	originA, tokenA, _ := r.svc.conn()
	r.must(Options{Days: 30})
	if _, err := r.svc.Connect(b.srv.URL, ""); err == nil || !strings.Contains(err.Error(), "Введите токен") {
		t.Fatalf("пустой токен: %v", err)
	}
	if _, err := r.svc.Connect(b.srv.URL, "wrong-token-0123456789"); err == nil || !strings.Contains(err.Error(), "Токен не подошёл") {
		t.Fatalf("чужой токен: %v", err)
	}
	if tok, _ := r.store.Get(originA); tok != tokenA {
		t.Fatal("неудачное подключение стёрло прежний токен")
	}
	if _, err := r.svc.Connect(b.srv.URL, b.srv.Token); err != nil {
		t.Fatal(err)
	}
	if tok, _ := r.store.Get(originA); tok != "" {
		t.Error("токен прежнего сервиса остался после подключения к другому")
	}
	if l := r.state().Link; l == nil || l.State != "other" {
		t.Fatalf("запись A при подключении к B: %+v", l)
	}
	// Правки и отзыв по записи другого сервиса — отказ, сеть не тронута.
	logB.mu.Lock()
	before := logB.n
	logB.mu.Unlock()
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Error("правка записи другого сервиса принята")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("отзыв записи другого сервиса принят")
	}
	// «Выгрузить» — новая ссылка на B только после вопроса; «Нет» — ничего.
	r.answer = false
	if ok, err := r.svc.Share(Options{Days: 30}); ok || err != nil || logB.n != before {
		t.Fatalf("«Нет»: %v %v, запросов к B %d", ok, err, logB.n-before)
	}
	r.answer = true
	r.must(Options{Days: 30})
	if l := r.state().Link; l.State != "ok" || !strings.HasPrefix(l.URL, b.srv.URL) {
		t.Errorf("новая ссылка: %+v", l)
	}
	for _, h := range logA.auth {
		if strings.Contains(h, b.srv.Token) {
			t.Error("токен B ушёл на сервер A")
		}
	}
	for _, h := range logB.auth {
		if strings.Contains(h, tokenA) {
			t.Error("токен A ушёл на сервер B")
		}
	}
}

// T4-7: недоделанная запись (id без ссылки) прежнего сервиса — «нет ссылки», а не готовая ссылка
// other; новая выгрузка на текущий сервис идёт только после вопроса и не продолжает чужой id.
func TestShareOtherIncomplete(t *testing.T) {
	r, b := newRig(t), newRig(t)
	r.connect()
	var puts atomic.Int32
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) && puts.Add(1) > 2 {
			return &albumstest.Fault{Status: http.StatusRequestEntityTooLarge}
		}
		return nil
	})
	if msg, _ := r.run(Options{Days: 30}); msg == "" {
		t.Fatal("ждали обрыв")
	}
	oldID, _ := r.rec()
	if _, err := r.svc.Connect(b.srv.URL, b.srv.Token); err != nil {
		t.Fatal(err)
	}
	if l := r.state().Link; l != nil {
		t.Fatalf("недоделанная запись другого сервиса выглядит ссылкой: %+v", l)
	}
	r.answer = false
	if ok, err := r.svc.Share(Options{Days: 30}); ok || err != nil || len(r.asks) != 1 || b.srv.Fake.Albums() != 0 {
		t.Fatalf("«Нет»: %v %v, вопросов %d, альбомов на B %d", ok, err, len(r.asks), b.srv.Fake.Albums())
	}
	if !strings.HasSuffix(r.asks[0], lostLink) { // запись затрётся: обещать «останется» нельзя
		t.Errorf("вопрос о другом сервисе: %q", r.asks[0])
	}
	if _, err := r.svc.Share(Options{}); err == nil { // «Продолжить» чужую запись — отказ, не продолжение
		t.Error("Days 0 принят для записи другого сервиса")
	}
	r.answer = true
	r.must(Options{Days: 30})
	if id, _ := r.rec(); id == oldID || b.srv.Fake.Albums() != 1 || r.state().Link.State != "ok" {
		t.Errorf("новая ссылка на B: id %q (был %q), альбомов %d", id, oldID, b.srv.Fake.Albums())
	}
}

// №7: 401 посреди хода — «подключите заново», токен убран, записи целы; то же при правке.
func TestShareToken401(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	path := r.curJob().Path
	r.setJob(testJob(t, path, 3, 5)) // правка: нужна новая редакция
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPut && !isPut(q) {
			return &albumstest.Fault{Status: http.StatusUnauthorized}
		}
		return nil
	})
	msg, stopped := r.run(Options{})
	if stopped || !strings.Contains(msg, "подключите сервис заново") {
		t.Fatalf("итог: %q %v", msg, stopped)
	}
	origin, token, _ := r.svc.conn()
	if token != "" || origin == "" {
		t.Errorf("токен %q после 401 (адрес %q)", token, origin)
	}
	if _, ok := r.svc.look(path); !ok {
		t.Error("запись пропала")
	}
	if v := r.state(); v.Service.Token || v.Service.Host == "" {
		t.Errorf("подключение: %+v", v.Service)
	}
	// Правка: тоже 401.
	r2 := newRig(t)
	r2.connect()
	r2.must(Options{Days: 30})
	r2.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPatch {
			return &albumstest.Fault{Status: http.StatusUnauthorized}
		}
		return nil
	})
	if _, err := r2.svc.Change(Change{Days: 90}); err == nil || !strings.Contains(err.Error(), "подключите сервис заново") {
		t.Errorf("правка: %v", err)
	}
	if _, tok, _ := r2.svc.conn(); tok != "" {
		t.Error("токен остался после 401 на правке")
	}
}

// №14: ссылка сервиса, не прошедшая проверку, не принимается; запись остаётся без url.
func TestShareBadURL(t *testing.T) {
	for _, bad := range []string{"javascript:alert(1)//", "ftp://x.test", "https://user:pw@evil.test"} {
		r := newRig(t)
		r.connect()
		r.srv.Fake.SetURL(bad)
		msg, stopped := r.run(Options{Days: 30})
		if stopped || !strings.Contains(msg, "ссылку, которую программа не принимает") {
			t.Errorf("%s: итог %q %v", bad, msg, stopped)
		}
		rec, ok := r.svc.look(r.curJob().Path)
		if !ok || rec.URL != "" || rec.ID == "" {
			t.Errorf("%s: запись %+v %v", bad, rec, ok)
		}
		if _, err := r.svc.Text(); err == nil {
			t.Error("текст клиенту выдан без ссылки")
		}
	}
	// Open: открывается только проверенная ссылка.
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	opened := ""
	r.svc.h.OpenURL = func(u string) { opened = u }
	if err := r.svc.Open(); err != nil || opened == "" {
		t.Fatalf("Open: %v %q", err, opened)
	}
}

// Журнал, события и тексты ошибок не содержат токена, пароля, id и названия (design §6 п. 1).
func TestShareSecretsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	r := newRig(t)
	const password, title = "секрет-4827", "Свадьба Секретная"
	r.connect()
	var n atomic.Int32 // крючок зовут три рабочих потока загрузки сразу
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPut(q) {
			if n.Add(1) <= 2 {
				return &albumstest.Fault{Status: http.StatusBadGateway}
			}
		}
		return nil
	})
	r.must(Options{Title: title, Password: password, Days: 30})
	id, _ := r.rec()
	r.setFault(func(*http.Request) *albumstest.Fault {
		return &albumstest.Fault{Status: http.StatusInternalServerError}
	})
	r.setJob(testJob(t, r.curJob().Path, 3, 9))
	msg, _ := r.run(Options{})
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if q.Method == http.MethodPatch {
			return &albumstest.Fault{Status: http.StatusUnauthorized}
		}
		return nil
	})
	_, errChange := r.svc.Change(Change{Days: 90})
	all := buf.String() + fmt.Sprint(r.events) + msg + fmt.Sprint(errChange)
	for name, secret := range map[string]string{"токен": r.srv.Token, "пароль": password, "id": id, "название": title} {
		if secret != "" && strings.Contains(all, secret) {
			t.Errorf("в журнале, событиях или ошибках есть %s", name)
		}
	}
	if !strings.Contains(buf.String(), "ответ сервиса 502") {
		t.Errorf("журнал без статуса ответа:\n%s", buf.String())
	}
	for _, e := range r.events {
		if strings.Contains(fmt.Sprintf("%+v", e), "http") {
			t.Errorf("событие несёт адрес: %+v", e)
		}
	}
}

// С5: нет папки кэша — отказ; уборка трогает только up-* старше суток и не идёт по ссылкам.
func TestShareTempCleanup(t *testing.T) {
	r := newRig(t)
	r.connect()
	for _, base := range []string{"", "relative/cache"} {
		r.svc.h.CacheDir = func() string { return base }
		if ok, err := r.svc.Share(Options{Days: 30}); ok || err == nil || !strings.Contains(err.Error(), "Нет папки кэша") {
			t.Errorf("кэш %q: %v %v", base, ok, err)
		}
		if r.svc.Busy() {
			t.Fatal("флаг остался")
		}
	}
	root := filepath.Join(r.cache, "share")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(testkit.TempDir(t), "keep")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-age)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldDir, newDir, foreign := mk("up-old", 48*time.Hour), mk("up-new", time.Hour), mk("other", 48*time.Hour)
	link := filepath.Join(root, "up-link")
	linkOK := os.Symlink(outside, link) == nil // нет прав на ссылки (Windows): проверяем остальное
	file := filepath.Join(root, "up-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(file, old, old)
	CleanTemp(r.cache, time.Now())
	CleanTemp("", time.Now())
	CleanTemp("relative", time.Now())
	gone := func(p string) bool { _, err := os.Lstat(p); return err != nil }
	if !gone(oldDir) {
		t.Error("старая up-* не удалена")
	}
	keep := []string{newDir, foreign, file, filepath.Join(outside, "f.txt")}
	if linkOK {
		keep = append(keep, link)
	}
	for _, p := range keep {
		if gone(p) {
			t.Errorf("тронуто лишнее: %s", filepath.Base(p))
		}
	}
	// Выгрузка: папка хода внутри share (0700) убирается в конце, чужое не тронуто.
	r.svc.h.CacheDir = func() string { return r.cache }
	r.must(Options{Days: 30})
	if gone(newDir) || linkOK && gone(link) {
		t.Error("выгрузка тронула чужое")
	}
	if st, err := os.Stat(root); err != nil || runtime.GOOS != "windows" && st.Mode().Perm() != 0o700 {
		t.Errorf("папка share: %v %v", st, err)
	}
}

// shares.json новее программы: ссылка готова, но не записана — окно её показывает, ошибка видна.
func TestShareRecordNotWritten(t *testing.T) {
	r := newRig(t)
	r.connect()
	if err := os.WriteFile(prefs.SharesPath(r.cfg), []byte(`{"version": 999, "albums": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	msg, stopped := r.run(Options{Days: 30})
	if stopped || !strings.Contains(msg, "Ссылка готова, но записать её") {
		t.Fatalf("итог: %q %v", msg, stopped)
	}
	if v := r.state(); v.Link == nil || v.Link.State != "ok" || v.Link.URL == "" {
		t.Errorf("ссылка не видна: %+v", v.Link)
	}
	if _, err := r.svc.Text(); err != nil {
		t.Errorf("текст клиенту: %v", err)
	}
}

// «Отключить»: токен убран, адрес остаётся.
func TestShareForget(t *testing.T) {
	r := newRig(t)
	r.connect()
	v, err := r.svc.Forget()
	if err != nil || v.Service.Token || v.Service.URL == "" {
		t.Errorf("Forget: %v %+v", err, v.Service)
	}
	if ok, err := r.svc.Share(Options{Days: 30}); ok || err == nil {
		t.Error("выгрузка без токена")
	}
}
