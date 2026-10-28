package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/app/updater"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// updFeed — «GitHub» выпуска на своём TLS-сервере: подписанный манифест и установщик. Сеть наружу
// тесты не трогают.
type updFeed struct {
	srv   *httptest.Server
	priv  ed25519.PrivateKey
	src   fetch.Source
	hits  atomic.Int32 // все запросы
	setup atomic.Int32 // запросы установщика

	mu    sync.Mutex
	env   []byte
	data  []byte
	block chan struct{} // не nil — установщик отдаёт половину и ждёт
	gate  chan struct{} // не nil — манифест ждёт закрытия (ручная проверка «стоит» в фазе checking)
}

func newUpdFeed(t *testing.T) *updFeed {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &updFeed{priv: priv}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	roots := f.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	f.src = fetch.Source{CatalogURL: f.srv.URL + "/latest/" + update.ManifestName, Prefix: f.srv.URL + "/dl/",
		Hosts: []string{"127.0.0.1"}, Keys: []ed25519.PublicKey{pub}, Roots: roots}
	return f
}

func (f *updFeed) serve(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	f.mu.Lock()
	env, data, block, gate := f.env, f.data, f.block, f.gate
	f.mu.Unlock()
	if gate != nil && strings.HasSuffix(r.URL.Path, update.ManifestName) {
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	}
	switch {
	case strings.HasSuffix(r.URL.Path, update.ManifestName) && env != nil:
		_, _ = w.Write(env)
	case strings.HasSuffix(r.URL.Path, "-setup.exe") && data != nil:
		f.setup.Add(1)
		w.Header().Set("Content-Length", itoa(len(data)))
		if block == nil {
			_, _ = w.Write(data)
			return
		}
		_, _ = w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		select {
		case <-block:
		case <-r.Context().Done():
		}
	default:
		http.NotFound(w, r)
	}
}

// publish — выпуск ver: установщик размером size и подписанный манифест (rest — лишние поля).
func (f *updFeed) publish(t *testing.T, ver string, size int, rest map[string]any) []byte {
	t.Helper()
	data := bytes.Repeat([]byte{byte(len(ver)), 7, 9}, size/3+1)[:size]
	sum := sha256.Sum256(data)
	m := map[string]any{"format": 1, "app": "sosninaalbums", "version": ver, "notes": []string{"Первое", "Второе"},
		"files": map[string]any{update.FileKey: map[string]any{"name": update.SetupName(ver), "size": size, "sha256": hex.EncodeToString(sum[:])}}}
	for k, v := range rest {
		m[k] = v
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	env, err := json.Marshal(map[string]string{"sig": base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, body)),
		"body": base64.StdEncoding.EncodeToString(body)})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.env, f.data = env, data
	f.mu.Unlock()
	return data
}

// updApp — App с обновлением на feed: версия 0.25.0, Windows, установленная копия (рядом
// uninstall.exe), свои папки; события собираются; запуск установщика и выход записываются.
type updRig struct {
	a       *App
	f       *updFeed
	ev      *capture
	exeDir  string
	starts  [][2]string // (установщик, командная строка)
	quits   atomic.Int32
	auto    bool
	bgCalls atomic.Int32  // сколько раз служба просила фон (Host.GoBG)
	gate    chan struct{} // не nil: фон службы стартует после закрытия канала (без таймеров в тесте)

	// Подмены, которые тест меняет по ходу; Host читает их при каждом вызове.
	version, goos, dir string
	exe                func() (string, error)
	free               func(dir string) (uint64, error)
	start              func(setup, cmdline string) error
}

func newUpdApp(t *testing.T) *updRig {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	r := &updRig{a: a, f: newUpdFeed(t), ev: captureEvents(a), exeDir: filepath.Join(testkit.TempDir(t), "Sosnina Albums"), auto: true}
	if err := os.MkdirAll(r.exeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.exeDir, "uninstall.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r.version, r.goos = "0.25.0", "windows"
	r.exe = func() (string, error) { return filepath.Join(r.exeDir, "SosninaAlbums.exe"), nil }
	r.dir = filepath.Join(testkit.TempDir(t), "Updates")
	r.free = func(string) (uint64, error) { return 1 << 40, nil }
	r.start = func(setup, cmd string) error { r.starts = append(r.starts, [2]string{setup, cmd}); return nil }
	a.upd = updater.New(r.host(a.updHost()))
	return r
}

// host — Host службы с подменами рига: поля читают поля r, их тест меняет по ходу. Служба
// собирается здесь, до первого вызова и до фона; заменять a.upd позже — гонка (126-19).
func (r *updRig) host(h updater.Host) updater.Host {
	h.Client = func() (*update.Client, bool) { return update.New(r.f.src), r.auto }
	h.Version = func() string { return r.version }
	h.GOOS = func() string { return r.goos }
	h.Exe = func() (string, error) { return r.exe() }
	h.Dir = func() string { return r.dir }
	h.Free = func(d string) (uint64, error) { return r.free(d) }
	h.Start = func(setup, cmd string) error { return r.start(setup, cmd) }
	h.Quit = func() { r.quits.Add(1) }
	h.GoBG = func(fn func()) bool {
		r.bgCalls.Add(1)
		if g := r.gate; g != nil {
			inner := fn
			fn = func() { <-g; inner() }
		}
		return r.a.goBG(fn)
	}
	return h
}

// newSession — «новый запуск программы»: служба с чистым состоянием (фаза, предложение, раз за
// сеанс) и теми же подменами. Только когда фон прошлой службы уже закончен (a.bg.Wait).
func (r *updRig) newSession() {
	r.a.bg.Wait()
	r.a.upd = updater.New(r.host(r.a.updHost()))
}

func (r *updRig) states() []UpdateInfo {
	var out []UpdateInfo
	for _, d := range r.ev.of("update:state") {
		out = append(out, UpdateInfo(d.(updater.Info)))
	}
	return out
}

// Полный круг: проверка → предложение → скачивание с ходом → ready → установка: pending записан до
// запуска, командная строка тихая, программа закрывается один раз.
func TestUpdateFlow(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	data := r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if info := a.UpdateInfo(); info.Phase != "idle" || info.Mode != "install" || !info.AtStart || info.Notes == nil || info.Version != "" {
		t.Fatalf("начальное: %+v", info)
	}
	info, err := a.CheckUpdate()
	if err != nil || info.Phase != "available" || info.Version != "0.25.1" || info.Mode != "install" ||
		len(info.Notes) != 2 || info.Checked == 0 || info.Error != "" {
		t.Fatalf("проверка: %v %+v", err, info)
	}
	if st := r.states(); len(st) != 2 || st[0].Phase != "checking" || st[1].Phase != "available" {
		t.Errorf("события state: %+v", st)
	}
	if u := prefs.UpdateOf(a.cfgDir); u.Seen != "0.25.1" || u.Checked != info.Checked {
		t.Errorf("config: %+v", u)
	}
	if len(r.ev.of("update:offer")) != 0 {
		t.Error("кнопка шлёт update:offer")
	}
	info, err = a.DownloadUpdate()
	if err != nil || info.Phase != "ready" || info.Done != int64(len(data)) || info.Total != int64(len(data)) {
		t.Fatalf("скачивание: %v %+v", err, info)
	}
	prog := r.ev.of("update:progress")
	if len(prog) == 0 || prog[len(prog)-1].(updater.Progress) != (updater.Progress{Done: int64(len(data)), Total: int64(len(data))}) {
		t.Errorf("ход: %v", prog)
	}
	if err := a.InstallUpdate(); err != nil {
		t.Fatal(err)
	}
	setupPath := filepath.Join(r.dir, update.SetupName("0.25.1"))
	want := `"` + setupPath + `" /S /relaunch /D=` + r.exeDir
	if len(r.starts) != 1 || r.starts[0] != [2]string{setupPath, want} || r.quits.Load() != 1 {
		t.Fatalf("запуск: %v, выходов %d", r.starts, r.quits.Load())
	}
	if u := prefs.UpdateOf(a.cfgDir); u.Pending != "0.25.1" || u.PendingAt == 0 {
		t.Errorf("pending: %+v", u)
	}
}

// Переносная копия (нет uninstall.exe) ставится обычным мастером; Mac и Linux, выпуск без
// установщика Windows и minAuto новее — только страница: скачать и установить нельзя.
func TestUpdateModes(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	r.exeDir = filepath.Join(testkit.TempDir(t), "Sosnina Albums 0.25.0") // переносная: uninstall.exe нет
	r.exe = func() (string, error) { return filepath.Join(r.exeDir, "SosninaAlbums-0.25.0.exe"), nil }
	if info, _ := a.CheckUpdate(); info.Mode != "setup" || info.Phase != "available" {
		t.Fatalf("переносная: %+v", info)
	}
	if _, err := a.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := a.InstallUpdate(); err != nil || len(r.starts) != 1 || r.starts[0][1] != `"`+r.starts[0][0]+`"` {
		t.Fatalf("мастер: %v %v", err, r.starts)
	}
	for name, mut := range map[string]func(*updRig){
		"linux":   func(r *updRig) { r.goos = "linux" },
		"darwin":  func(r *updRig) { r.goos = "darwin" },
		"minAuto": func(r *updRig) { r.f.publish(t, "0.25.1", update.MinSetup, map[string]any{"minAuto": "0.25.5"}) },
		"без Windows": func(r *updRig) {
			r.f.publish(t, "0.25.1", update.MinSetup, map[string]any{"files": map[string]any{}})
		},
	} {
		p := newUpdApp(t)
		p.f.publish(t, "0.25.1", update.MinSetup, nil)
		mut(p)
		info, err := p.a.CheckUpdate()
		if err != nil || info.Phase != "available" || info.Mode != "page" {
			t.Errorf("%s: %v %+v", name, err, info)
			continue
		}
		// Не отказ (иначе «Повторить» по кругу): снимок со страничным режимом, скачивания нет.
		if info, err := p.a.DownloadUpdate(); err != nil || info.Mode != "page" || info.Phase != "available" {
			t.Errorf("%s: скачивание: %v %+v", name, err, info)
		}
		if err := p.a.InstallUpdate(); err == nil || len(p.starts) != 0 || p.f.setup.Load() != 0 {
			t.Errorf("%s: установка: %v, запусков %d", name, err, len(p.starts))
		}
	}
	// Без предложения и до скачивания: ошибки, ничего не запущено.
	q := newUpdApp(t)
	if info, err := q.a.DownloadUpdate(); err != nil || info.Phase != "idle" || info.Version != "" || q.f.hits.Load() != 0 {
		t.Errorf("без предложения: %v %+v, запросов %d", err, info, q.f.hits.Load())
	}
	if err := q.a.InstallUpdate(); !errors.Is(err, updater.ErrNotReady) || len(q.starts) != 0 {
		t.Errorf("не скачано: %v", err)
	}
}

// Подмена файла после проверки: установщик не запущен, pending не записан, программа не закрыта.
func TestInstallRehash(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if info, err := a.DownloadUpdate(); err != nil || info.Phase != "ready" {
		t.Fatalf("скачивание: %v %+v", err, info)
	}
	path := filepath.Join(r.dir, update.SetupName("0.25.1"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.InstallUpdate(); !errors.Is(err, update.ErrVerify) {
		t.Fatalf("подмена: %v", err)
	}
	if len(r.starts) != 0 || r.quits.Load() != 0 || prefs.UpdateOf(a.cfgDir).Pending != "" {
		t.Errorf("после отказа: запусков %d, выходов %d, pending %q", len(r.starts), r.quits.Load(), prefs.UpdateOf(a.cfgDir).Pending)
	}
	if info := a.UpdateInfo(); info.Phase != "error" || info.Error == "" {
		t.Errorf("фаза: %+v", info)
	}
}

// Установщик не запустился: pending снят, программа не закрыта, ошибка словами без пути.
func TestInstallStartFails(t *testing.T) {
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	r.start = func(string, string) error { return &os.PathError{Op: "fork", Path: r.dir, Err: os.ErrPermission} }
	if _, err := r.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	err := r.a.InstallUpdate()
	if !errors.Is(err, updater.ErrStart) || strings.Contains(err.Error(), r.dir) {
		t.Fatalf("ошибка: %v", err)
	}
	if r.quits.Load() != 0 || prefs.UpdateOf(r.a.cfgDir).Pending != "" {
		t.Errorf("выходов %d, pending %q", r.quits.Load(), prefs.UpdateOf(r.a.cfgDir).Pending)
	}
}

// Ошибки проверки: кнопка — фаза error со словами без адреса; проверка при запуске — только log.
func TestUpdateCheckErrors(t *testing.T) {
	r := newUpdApp(t) // манифеста нет — 404
	info, err := r.a.CheckUpdate()
	if err != nil || info.Phase != "error" || info.Error == "" || strings.Contains(info.Error, "127.0.0.1") || info.Checked != 0 {
		t.Fatalf("кнопка: %v %+v", err, info)
	}
	if prefs.UpdateOf(r.a.cfgDir).Checked != 0 {
		t.Error("checked записан после неудачи")
	}
	r2 := newUpdApp(t)
	r2.a.upd.CheckSilent()
	if info := r2.a.UpdateInfo(); info.Phase != "idle" || info.Error != "" {
		t.Errorf("тихая: %+v", info)
	}
	if len(r2.ev.of("update:state")) != 0 || len(r2.ev.of("update:offer")) != 0 {
		t.Error("тихая неудача шлёт события")
	}
	// Повтор после ошибки работает: фаза выходит из error.
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if info, _ := r.a.CheckUpdate(); info.Phase != "available" || info.Error != "" {
		t.Errorf("повтор: %+v", info)
	}
	// Манифест с чужой подписью — «не прошло проверку».
	r.f.priv = func() ed25519.PrivateKey { _, p, _ := ed25519.GenerateKey(rand.Reader); return p }()
	r.f.publish(t, "0.25.2", update.MinSetup, nil)
	if info, _ := r.a.CheckUpdate(); info.Phase != "error" || info.Error != update.ErrVerify.Error() {
		t.Errorf("подпись: %+v", info)
	}
}

// Версия ниже виденной — «последняя», без предложения; seen не падает.
func TestUpdateNoRollback(t *testing.T) {
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	prefs.UpdateUpdate(r.a.cfgDir, func(u *prefs.Update) { u.Seen = "0.26.0" })
	info, err := r.a.CheckUpdate()
	if err != nil || info.Phase != "latest" || info.Version != "" {
		t.Fatalf("откат: %v %+v", err, info)
	}
	if u := prefs.UpdateOf(r.a.cfgDir); u.Seen != "0.26.0" || u.Checked == 0 {
		t.Errorf("config: %+v", u)
	}
	// Своя версия новее файла — тоже «последняя».
	r.version = "0.25.1"
	prefs.UpdateUpdate(r.a.cfgDir, func(u *prefs.Update) { u.Seen = "" })
	if info, _ := r.a.CheckUpdate(); info.Phase != "latest" {
		t.Errorf("та же версия: %+v", info)
	}
}

// Отмена скачивания: фаза возвращается к предложению, .part нет, не ошибка.
func TestUpdateCancel(t *testing.T) {
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	r.f.mu.Lock()
	r.f.block = make(chan struct{})
	r.f.mu.Unlock()
	if _, err := r.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	done := make(chan UpdateInfo, 1)
	go func() { info, _ := r.a.DownloadUpdate(); done <- info }()
	deadline := time.Now().Add(5 * time.Second)
	for r.f.setup.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	// Второе скачивание параллельно не стартует — отдаёт состояние первого.
	if info, err := r.a.DownloadUpdate(); err != nil || info.Phase != "downloading" {
		t.Errorf("параллельное: %v %+v", err, info)
	}
	r.a.CancelUpdate()
	select {
	case info := <-done:
		if info.Phase != "available" || info.Error != "" {
			t.Fatalf("после отмены: %+v", info)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("скачивание не оборвалось")
	}
	if list, _ := os.ReadDir(r.dir); len(list) != 0 {
		t.Errorf("в папке %v", list)
	}
	r.a.CancelUpdate() // отмена без операции — тихо
}

// Скачанное и сверенное не качается заново (повторное «Обновить» → ready без запроса).
func TestUpdateReuse(t *testing.T) {
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := r.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if info, err := r.a.DownloadUpdate(); err != nil || info.Phase != "ready" {
			t.Fatalf("скачивание: %v %+v", err, info)
		}
	}
	if r.f.setup.Load() != 1 {
		t.Errorf("установщик скачан %d раз", r.f.setup.Load())
	}
	// Мало места: ошибка с числом, сети нет.
	q := newUpdApp(t)
	q.f.publish(t, "0.25.1", update.MinSetup, nil)
	q.free = func(string) (uint64, error) { return 10, nil }
	if _, err := q.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	info, _ := q.a.DownloadUpdate()
	if info.Phase != "error" || !strings.Contains(info.Error, "нужно 2 МБ") || q.f.setup.Load() != 0 {
		t.Errorf("мало места: %+v", info)
	}
}

// Итог после перезапуска: версия стоит — Just (один раз), не стоит и недавно — Failed, давно —
// молча; pending снимается в любом случае; ничего не было — пусто.
func TestUpdatePending(t *testing.T) {
	now := time.Now()
	for name, c := range map[string]struct {
		cur, pending      string
		at                time.Time
		wantJust, wantBad string
	}{
		"поставилась":            {"0.25.1", "0.25.1", now.Add(-time.Minute), "0.25.1", ""},
		"поставилась новее":      {"0.25.2", "0.25.1", now.Add(-time.Minute), "0.25.1", ""},
		"прежняя недавно":        {"0.25.0", "0.25.1", now.Add(-time.Minute), "", "0.25.1"},
		"прежняя давно":          {"0.25.0", "0.25.1", now.Add(-2 * time.Hour), "", ""},
		"часы назад":             {"0.25.0", "0.25.1", now.Add(time.Hour), "", ""},
		"граница часа":           {"0.25.0", "0.25.1", now.Add(-updater.FailWindow - time.Second), "", ""},
		"dev: сравнить нечем":    {"dev", "0.25.1", now.Add(-time.Minute), "", ""},
		"нет итога":              {"0.25.0", "", now, "", ""},
		"мусор в pending":        {"0.25.0", "скоро", now, "", ""},
		"то же с суффиксом cur":  {"0.25.1-rc1", "0.25.1", now.Add(-time.Minute), "", ""},
		"pending без времени":    {"0.25.0", "0.25.1", time.UnixMilli(0), "", ""},
		"новая версия без часов": {"0.25.1", "0.25.1", time.UnixMilli(0), "0.25.1", ""},
	} {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		h := a.updHost()
		h.Version = func() string { return c.cur }
		a.upd = updater.New(h)
		prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Pending, u.PendingAt = c.pending, c.at.UnixMilli() })
		a.upd.Startup()
		info := a.UpdateInfo()
		if info.Just != c.wantJust || info.Failed != c.wantBad {
			t.Errorf("%s: just %q failed %q", name, info.Just, info.Failed)
		}
		if again := a.UpdateInfo(); again.Just != "" || again.Failed != "" {
			t.Errorf("%s: итог отдан дважды: %+v", name, again)
		}
		if u := prefs.UpdateOf(a.cfgDir); u.Pending != "" || u.PendingAt != 0 {
			t.Errorf("%s: pending не снят: %+v", name, u)
		}
	}
}

// Расписание: выключено, свежая проверка, dev без стенда — ни одного запроса; иначе один запрос
// через задержку и раз за сеанс; снятие флажка за время задержки отменяет; выход — тоже.
func TestUpdateSchedule(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	run := func() int32 {
		r.newSession() // новый сеанс: чистая фаза (тихая проверка не идёт, если фаза уже есть) и «раз за сеанс»
		before := r.f.hits.Load()
		a.upd.Schedule()
		a.bg.Wait()
		return r.f.hits.Load() - before
	}
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Off = true })
	if n := run(); n != 0 {
		t.Errorf("выключено: запросов %d", n)
	}
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Off, u.Checked = false, time.Now().Add(-time.Hour).UnixMilli() })
	if n := run(); n != 0 {
		t.Errorf("свежая проверка: запросов %d", n)
	}
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Checked = time.Now().Add(-21 * time.Hour).UnixMilli() })
	r.auto = false
	if n := run(); n != 0 {
		t.Errorf("dev без стенда: запросов %d", n)
	}
	r.auto = true
	if n := run(); n != 1 {
		t.Fatalf("пора: запросов %d", n)
	}
	if offers := r.ev.of("update:offer"); len(offers) != 1 || offers[0].(updater.Info).Version != "0.25.1" || offers[0].(updater.Info).Phase != "available" {
		t.Errorf("update:offer: %+v", offers)
	}
	// Раз за сеанс: второй вызов (перезагрузка окна) не идёт в сеть.
	hits := r.f.hits.Load()
	a.upd.Schedule()
	a.bg.Wait()
	if r.f.hits.Load() != hits {
		t.Error("повторная проверка за сеанс")
	}
	// Часы ушли назад (checked в будущем) — пора.
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Checked = time.Now().Add(time.Hour).UnixMilli() })
	if n := run(); n != 1 {
		t.Errorf("checked из будущего: запросов %d", n)
	}
	// «Последняя»: тоста нет.
	r.ev = captureEvents(a)
	r.version = "0.25.1"
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Checked = 0 })
	if n := run(); n != 1 || len(r.ev.of("update:offer")) != 0 {
		t.Errorf("последняя: запросов %d, offer %d", n, len(r.ev.of("update:offer")))
	}
}

// Флажок снят за время задержки — проверки нет; выход программы до задержки — тоже.
func TestUpdateScheduleCancel(t *testing.T) {
	defer setStart(time.Hour, 20*time.Hour)()
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	a.upd.Schedule()
	if info := a.SetUpdateCheck(false); info.AtStart {
		t.Fatalf("флажок: %+v", info)
	}
	a.stopCancel() // конец задержки раньше часа
	a.bg.Wait()
	if r.f.hits.Load() != 0 {
		t.Error("проверка после выхода")
	}
	// Флажок снят за время задержки: перечитывается после ожидания. Задержка нулевая, а фон
	// держит ворота: флажок снимается до его старта по порядку, а не по таймеру (под нагрузкой
	// запись config.json дольше любой короткой задержки).
	startDelay = 0
	b := newUpdApp(t)
	b.gate = make(chan struct{})
	b.f.publish(t, "0.25.1", update.MinSetup, nil)
	b.a.upd.Schedule()
	if b.bgCalls.Load() != 1 {
		t.Fatal("проверка не запланирована")
	}
	b.a.SetUpdateCheck(false)
	close(b.gate)
	b.a.bg.Wait()
	if b.f.hits.Load() != 0 {
		t.Error("проверка при снятом флажке")
	}
	// Без снятия то же приводит к запросу (тест не пуст).
	c := newUpdApp(t)
	c.f.publish(t, "0.25.1", update.MinSetup, nil)
	c.a.upd.Schedule()
	c.a.bg.Wait()
	if c.f.hits.Load() != 1 {
		t.Errorf("без снятия: запросов %d", c.f.hits.Load())
	}
}

// Флажок: круг запись → чтение; по умолчанию включён.
func TestUpdateFlag(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	if !a.UpdateInfo().AtStart {
		t.Fatal("по умолчанию выключено")
	}
	if info := a.SetUpdateCheck(false); info.AtStart || a.UpdateInfo().AtStart || !prefs.UpdateOf(a.cfgDir).Off {
		t.Fatalf("снят: %+v", info)
	}
	if info := a.SetUpdateCheck(true); !info.AtStart || prefs.UpdateOf(a.cfgDir).Off {
		t.Fatalf("включён: %+v", info)
	}
	// Кнопка ходит в сеть и при снятом флажке.
	a.SetUpdateCheck(false)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if info, _ := a.CheckUpdate(); info.Phase != "available" || info.AtStart {
		t.Errorf("кнопка при снятом флажке: %+v", info)
	}
}

// Страница выпуска: проверенная версия, без предложения — последний выпуск; адрес не от интерфейса.
func TestOpenUpdatePage(t *testing.T) {
	var opened []string
	old := openURL
	t.Cleanup(func() { openURL = old })
	openURL = func(_ *App, u string) { opened = append(opened, u) }
	r := newUpdApp(t)
	if err := r.a.OpenUpdatePage(); err != nil {
		t.Fatal(err)
	}
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := r.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := r.a.OpenUpdatePage(); err != nil {
		t.Fatal(err)
	}
	want := []string{appkit.RepoURL + "/releases/latest", appkit.RepoURL + "/releases/tag/v0.25.1"}
	if len(opened) != 2 || opened[0] != want[0] || opened[1] != want[1] {
		t.Errorf("адреса %v", opened)
	}
}

// Уборка: при запуске в папке скачанного уходят недокачанное и установленные версии.
func TestUpdateCleanAtStart(t *testing.T) {
	defer setStart(time.Hour, 20*time.Hour)()
	r := newUpdApp(t)
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{update.SetupName("0.25.0"), update.SetupName("0.25.1"), update.SetupName("0.25.1") + ".part", "чужое.txt"} {
		if err := os.WriteFile(filepath.Join(r.dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.a.upd.Schedule()
	r.a.stopCancel()
	r.a.bg.Wait()
	list, _ := os.ReadDir(r.dir)
	var got []string
	for _, e := range list {
		got = append(got, e.Name())
	}
	if len(got) != 2 || got[0] != "чужое.txt" && got[1] != "чужое.txt" {
		t.Errorf("осталось %v", got)
	}
}
