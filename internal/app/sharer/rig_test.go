package sharer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image/color"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/secret"
	"github.com/IliaSA101/sosninaalbums/internal/share"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Общая оснастка тестов службы: поддельный сервер (httptest — сеть только так, forbidigo 067),
// Host с памятью вместо хранилища ключей и записью событий, снимок альбома из цветных листов.

type event struct {
	name string
	data []any
}

// rig — служба на поддельном сервере.
type rig struct {
	t     *testing.T
	srv   *albumstest.Server
	svc   *Service
	cfg   string
	cache string
	store *secret.Memory

	mu      sync.Mutex
	events  []event
	asks    []string
	answer  bool // ответ на вопросы «Да / Нет»
	job     share.Job
	done    chan event
	waits   []time.Duration
	exportB bool
	pins    []string // отпечатки, которые служба просила записать в файл (Pin)
	pinErr  error    // ответ Pin; nil — записан
	pinUID  string   // отпечаток, который выдаёт Pin; "" — прежний
	wg      sync.WaitGroup

	fault func(r *http.Request) *albumstest.Fault // крючок сервера; nil — без отказов
}

func newRig(t *testing.T) *rig {
	t.Helper()
	root := testkit.TempDir(t)
	r := &rig{t: t, cfg: filepath.Join(root, "cfg"), cache: filepath.Join(root, "cache"), store: secret.NewMemory(),
		answer: true, done: make(chan event, 64)}
	for _, d := range []string{r.cfg, r.cache} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	r.srv = albumstest.New(albumstest.Options{Fault: func(q *http.Request) *albumstest.Fault {
		r.mu.Lock()
		f := r.fault
		r.mu.Unlock()
		if f == nil {
			return nil
		}
		return f(q)
	}})
	t.Cleanup(r.srv.Close)
	r.job = testJob(t, filepath.Join(root, "a.salbum"), 3, 0)
	r.svc = New(Host{
		Plan:       func() share.Job { r.mu.Lock(); defer r.mu.Unlock(); return r.job },
		Ident:      r.ident,
		Pin:        r.pin,
		Emit:       r.emit,
		GoBG:       func(fn func()) bool { r.wg.Go(fn); return true },
		StopCtx:    context.Background,
		Ask:        r.ask,
		ExportBusy: func() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.exportB },
		LiftMemory: func() func() { return func() {} },
		CheckURL:   printprof.CheckURL,
		OpenURL:    func(string) {},
		Fold:       paths.Fold,
		CfgDir:     func() string { return r.cfg },
		CacheDir:   func() string { return r.cache },
		Store:      r.store, AllowLoopback: true, Version: "0.32.0-test",
	})
	r.svc.gap = 0
	r.svc.wait = func(_ context.Context, d time.Duration) error {
		r.mu.Lock()
		r.waits = append(r.waits, d)
		r.mu.Unlock()
		return nil
	}
	t.Cleanup(r.wg.Wait)
	return r
}

func (r *rig) emit(name string, data ...any) {
	e := event{name, data}
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
	if name == "share:done" {
		r.done <- e
	}
}

func (r *rig) ask(msg string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asks = append(r.asks, msg)
	return r.answer, nil
}

// ident — Host.Ident как у App (ShareIdent): печать не этого пути — без отпечатка.
func (r *rig) ident() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.Unsealed {
		return r.job.Path, ""
	}
	return r.job.Path, r.job.UID
}

func (r *rig) pin(_, uid string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pins = append(r.pins, uid)
	if r.pinErr != nil {
		return "", r.pinErr
	}
	if r.pinUID != "" { // выдача нового отпечатка с печатью пути: альбом дальше с ним (как App)
		r.job.UID, r.job.Unsealed = r.pinUID, false
		return r.pinUID, nil
	}
	return uid, nil
}

func (r *rig) setFault(f func(q *http.Request) *albumstest.Fault) {
	r.mu.Lock()
	r.fault = f
	r.mu.Unlock()
}

func (r *rig) setJob(j share.Job) { r.mu.Lock(); r.job = j; r.mu.Unlock() }

func (r *rig) curJob() share.Job { r.mu.Lock(); defer r.mu.Unlock(); return r.job }

// connect подключает службу к серверу rig.
func (r *rig) connect() {
	r.t.Helper()
	if _, err := r.svc.Connect(r.srv.URL, r.srv.Token); err != nil {
		r.t.Fatalf("Connect: %v", err)
	}
}

// run запускает выгрузку и ждёт share:done; возвращает его (err, stopped).
func (r *rig) run(o Options) (msg string, stopped bool) {
	r.t.Helper()
	r.drainDone() // чужое запоздалое share:done не должно сойти за итог этого запуска
	ok, err := r.svc.Share(o)
	if err != nil || !ok {
		r.t.Fatalf("Share: %v %v", ok, err)
	}
	return r.wait()
}

func (r *rig) wait() (string, bool) {
	r.t.Helper()
	select {
	case e := <-r.done:
		d := e.data[0].(doneEvent)
		return d.Err, d.Stopped
	case <-time.After(testkit.Limit(30 * time.Second)):
		// Состояние для разбора: события и стеки (раньше провал не оставлял следов).
		r.mu.Lock()
		names := make([]string, len(r.events))
		for i, e := range r.events {
			names[i] = e.name
		}
		r.mu.Unlock()
		buf := make([]byte, 1<<16)
		r.t.Fatalf("share:done не пришло; события %v; busy=%v\n%s", names, r.svc.Busy(), buf[:runtime.Stack(buf, true)])
	}
	return "", false
}

// drainDone забирает неразобранные share:done.
func (r *rig) drainDone() {
	for {
		select {
		case <-r.done:
		default:
			return
		}
	}
}

// state — State без ошибки.
func (r *rig) state() View {
	r.t.Helper()
	v, err := r.svc.State()
	if err != nil {
		r.t.Fatal(err)
	}
	return v
}

// rec — запись текущего альбома из shares.json.
func (r *rig) rec() (id string, ok bool) {
	rec, ok := r.svc.look(r.curJob().Path)
	return rec.ID, ok
}

// tempLeft — число папок up-* в кэше.
func (r *rig) tempLeft() int {
	list, _ := os.ReadDir(filepath.Join(r.cache, "share"))
	n := 0
	for _, e := range list {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			n++
		}
	}
	return n
}

// testJob — снимок из spreads разворотов; каждая страница своего цвета, hue сдвигает цвета (другое
// содержимое — другие sha256 и другой хэш сцены).
func testJob(t *testing.T, path string, spreads int, hue uint8) share.Job {
	t.Helper()
	sp := make([]render.Spread, spreads)
	for i := range sp {
		l := color.RGBA{R: 30 + uint8(i)*40, G: 120, B: 60 + hue, A: 255}
		rr := color.RGBA{R: 30 + uint8(i)*40, G: 200, B: 60 + hue, A: 255}
		sp[i] = render.Spread{Bg: [2]color.RGBA{l, rr}}
	}
	return jobOf(t, path, sp)
}

func jobOf(t *testing.T, path string, sp []render.Spread) share.Job {
	t.Helper()
	spec := export.WebSpec{DPI: 50, PageW: 100, PageH: 60, BleedX: 3, BleedY: 3, Quality: export.WebQuality}
	scene, err := share.SceneHash(sp, nil, spec, func(string) (int64, int64, bool) { return 0, 0, false })
	if err != nil {
		t.Fatal(err)
	}
	return share.Job{Path: path, UID: uidOf(path), Title: strings.TrimSuffix(filepath.Base(path), ".salbum"), Pages: 2 * len(sp),
		Scene: scene, Spreads: sp, Spec: spec}
}

// headerLog — запросы сервера по Authorization: для проверки, что токен ушёл только куда надо.
type headerLog struct {
	mu   sync.Mutex
	auth []string
	n    int // запросов, кроме health и list
}

func (h *headerLog) fault(q *http.Request) *albumstest.Fault {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.auth = append(h.auth, q.Header.Get("Authorization"))
	if !strings.HasSuffix(q.URL.Path, "/health") && (q.Method != http.MethodGet || q.URL.Path != "/v1/albums") {
		h.n++
	}
	return nil
}

// bodyOf читает тело запроса и возвращает его обратно в запрос (крючок сервера видит запрос до разбора).
func bodyOf(q *http.Request) string {
	b, _ := io.ReadAll(q.Body)
	q.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

func isPut(q *http.Request) bool {
	return q.Method == http.MethodPut && strings.Contains(q.URL.Path, "/pages/")
}

// must запускает выгрузку и требует успешного конца.
func (r *rig) must(o Options) {
	r.t.Helper()
	if msg, stopped := r.run(o); msg != "" || stopped {
		r.t.Fatalf("выгрузка закончилась не успехом: %q stopped=%v", msg, stopped)
	}
}

// share0 — снимок с отказом (не сохранён, лист виньетки, нет разворотов, пропавшие фото).
func share0(path, block string, save bool) share.Job {
	return share.Job{Path: path, Block: block, Save: save, Title: "a"}
}

func errTestAPI(status int) error { return &albums.APIError{Status: status} }

// atomic1 — «первый раз» для крючков сервера.
type atomic1 struct{ done atomic.Bool }

func (a *atomic1) first() bool { return a.done.CompareAndSwap(false, true) }

// uidOf — отпечаток проекта снимка по пути: у одного пути один проект, пока тест не сменит его сам.
func uidOf(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:16])
}
