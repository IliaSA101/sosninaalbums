package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Плитки «Можно скачать» сквозь App (спека 144 Г3): каталог вида, PackItem, /media/lp/, события.

// previewSheet — лист образцов набора ivory: ряд фонов 2 × 64×48 и ряд разворотов 1 × 64×48.
func previewSheet(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 128, 96))
	for y := range 96 {
		for x := range 128 {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Предметы ivory из манифеста: src = sha256[:16] записи.
const (
	ivoryBg1    = "20c18770354cf007"
	ivoryBg2    = "8355541da24dc41f"
	ivorySpread = "e8963aff597481c3"
)

// tilesApp — App с согласием и каталогом из набора ivory с листом образцов; список сохранён
// (UpdateLibrary), лист на диске (FetchPreviews).
func tilesApp(t *testing.T, sheetOnDisk bool) (*App, *netLib) {
	t.Helper()
	a, l := netApp(t, true)
	sheet := previewSheet(t)
	sum := sha256.Sum256(sheet)
	l.mu.Lock()
	l.zips["ivory-sheet.jpg"] = sheet
	l.mu.Unlock()
	l.publish(t, func(row map[string]any) {
		row["preview"] = map[string]any{"url": l.src.Prefix + "ivory-sheet.jpg", "sha256": hex.EncodeToString(sum[:]),
			"size": len(sheet), "w": 128, "h": 96, "rows": []map[string]any{
				{"kind": "bg", "w": 64, "h": 48, "items": []string{ivoryBg1, ivoryBg2}},
				{"kind": "spread", "w": 64, "h": 48, "items": []string{ivorySpread}},
			}}
	}, d1Zip(t))
	if _, err := a.UpdateLibrary(); err != nil {
		t.Fatal(err)
	}
	if sheetOnDisk {
		if got := a.libp.FetchPreviews(); got != 1 {
			t.Fatalf("лист не скачан: %d", got)
		}
	}
	return a, l
}

// Каталог вида: плитки, тема «Можно скачать» последней, без приглашения; другие виды — пусто.
func TestDecorCatalogGet(t *testing.T) {
	a, _ := tilesApp(t, true)
	c, err := a.DecorCatalog("bg")
	if err != nil || len(c.Get) != 2 || c.Invite {
		t.Fatalf("фоны: %v get=%d invite=%v", err, len(c.Get), c.Invite)
	}
	last := c.Themes[len(c.Themes)-1]
	if last.ID != "get" || last.Name != "Можно скачать" || last.Count != 2 {
		t.Errorf("последняя тема: %+v", last)
	}
	g := c.Get[0]
	if g.Pack != "ivory" || g.Src != ivoryBg1 || g.ID != "get:ivory:"+ivoryBg1 || !strings.HasPrefix(g.URL, "/media/lp/ivory/bg/0?v=") ||
		g.W != 64 || g.H != 48 || g.Bytes == 0 || g.With == nil || g.Title != "Свадьба — айвори" {
		t.Errorf("плитка: %+v", g)
	}
	if sp, _ := a.DecorCatalog("spread"); len(sp.Get) != 1 || sp.Get[0].Src != ivorySpread {
		t.Errorf("развороты: %+v", sp.Get)
	}
	for _, kind := range []string{"sticker", "frame", "album"} {
		o, err := a.DecorCatalog(kind)
		if err != nil || o.Get == nil || len(o.Get) != 0 || o.Themes[len(o.Themes)-1].ID == "get" {
			t.Errorf("%s: %v %+v", kind, err, o.Get)
		}
	}
	if b, _ := json.Marshal(c); !strings.Contains(string(b), `"get":[{`) || !strings.Contains(string(b), `"invite":false`) {
		t.Errorf("JSON: %s", b)
	}
}

// Каталог и плитки в сеть не ходят: ни одного запроса за вызов.
func TestDecorCatalogNoNet(t *testing.T) {
	a, l := tilesApp(t, true)
	before := l.hits.Load()
	for range 3 {
		for _, kind := range []string{"bg", "sticker", "frame", "spread", "album"} {
			if _, err := a.DecorCatalog(kind); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := l.hits.Load(); got != before {
		t.Errorf("DecorCatalog сходил в сеть: %d запросов", got-before)
	}
}

// Приглашение и согласие: списка не было — приглашение, плиток нет; список есть, согласие снято —
// ни плиток, ни приглашения.
func TestDecorCatalogInvite(t *testing.T) {
	a, l := netApp(t, false)
	c, err := a.DecorCatalog("bg")
	if err != nil || !c.Invite || c.Get == nil || len(c.Get) != 0 {
		t.Fatalf("без списка: %v invite=%v get=%v", err, c.Invite, c.Get)
	}
	if l.hits.Load() != 0 {
		t.Errorf("запросов %d", l.hits.Load())
	}
	b, _ := tilesApp(t, true)
	if _, err := b.SetLibraryNet(false, false); err != nil {
		t.Fatal(err)
	}
	if c, _ := b.DecorCatalog("bg"); c.Invite || len(c.Get) != 0 {
		t.Errorf("согласие снято: invite=%v get=%d", c.Invite, len(c.Get))
	}
}

// Ключ плитки скачивания в недавние не попадает.
func TestRememberDecorRejectsGet(t *testing.T) {
	dir := testkit.TempDir(t)
	prefs.RememberDecor(dir, "bg", "get:ivory:"+ivoryBg1)
	prefs.RememberDecor(dir, "bg", "my:"+ivoryBg1)
	if got := prefs.RecentDecor(dir, "bg"); len(got) != 1 || got[0] != "my:"+ivoryBg1 {
		t.Errorf("недавние: %v", got)
	}
}

// Скачать набор и поставить: PackItem отдаёт ту же плитку, что каталог вида; плиток скачивания больше нет.
func TestDownloadThenPackItem(t *testing.T) {
	a, _ := tilesApp(t, true)
	if _, err := a.PackItem("bg", "ivory", ivoryBg1); !errors.Is(err, decor.ErrNoItem) {
		t.Fatalf("до скачивания: %v", err)
	}
	res, err := a.DownloadLibraryPack("ivory")
	if err != nil || !res.Picked {
		t.Fatalf("скачивание: %v %+v", err, res)
	}
	for kind, src := range map[string]string{"bg": ivoryBg2, "spread": ivorySpread} {
		it, err := a.PackItem(kind, "ivory", src)
		if err != nil || it.PackID != "ivory" || it.ID == "" {
			t.Fatalf("%s: %v %+v", kind, err, it)
		}
		cat, _ := a.DecorCatalog(kind)
		found := false
		for _, c := range cat.Items {
			found = found || c.ID == it.ID
		}
		if !found || len(cat.Get) != 0 {
			t.Errorf("%s: плитки %s нет в каталоге или остались плитки скачивания (%d)", kind, it.ID, len(cat.Get))
		}
	}
	for _, bad := range [][3]string{{"bg", "ivory", "nope"}, {"bg", "nope", ivoryBg1}, {"print", "ivory", ivoryBg1}, {"bg", "../ivory", ivoryBg1},
		{"bg", "ivory", strings.ToUpper(ivoryBg1)}, {"sticker", "ivory", ivoryBg1}} {
		if _, err := a.PackItem(bad[0], bad[1], bad[2]); !errors.Is(err, decor.ErrNoItem) {
			t.Errorf("%v: %v", bad, err)
		}
	}
}

// FetchPackPreviews: ответ сразу, задание фоном, событие library:previews один раз; повтор ничего
// не качает; без согласия — ни запроса.
func TestFetchPackPreviews(t *testing.T) {
	a, l := tilesApp(t, false)
	ev := captureEvents(a)
	if c, _ := a.DecorCatalog("bg"); len(c.Get) != 0 {
		t.Fatalf("плитки без листа: %d", len(c.Get))
	}
	for range 2 {
		if err := a.FetchPackPreviews(); err != nil {
			t.Fatal(err)
		}
		a.bg.Wait()
	}
	got := ev.of("library:previews")
	if len(got) != 1 || got[0].(libpacks.PreviewsReady).Ready != 1 {
		t.Fatalf("события: %v", got)
	}
	if c, _ := a.DecorCatalog("bg"); len(c.Get) != 2 {
		t.Errorf("плиток после докачки: %d", len(c.Get))
	}
	if l.hits.Load() > 3 { // каталог, лист; повтор лист не просит
		t.Errorf("запросов %d", l.hits.Load())
	}

	b, lb := tilesApp(t, false)
	if _, err := b.SetLibraryNet(false, false); err != nil {
		t.Fatal(err)
	}
	before := lb.hits.Load()
	if err := b.FetchPackPreviews(); err != nil {
		t.Fatal(err)
	}
	b.bg.Wait()
	if lb.hits.Load() != before {
		t.Errorf("без согласия %d запросов", lb.hits.Load()-before)
	}
}

// Метод отвечает сразу, задание идёт фоном: лист отдаётся наполовину и держится, а вызов уже вернулся;
// до конца задания события нет.
func TestFetchPackPreviewsInBackground(t *testing.T) {
	a, l := tilesApp(t, false)
	ev := captureEvents(a)
	block, reached := make(chan struct{}), make(chan struct{})
	l.mu.Lock()
	l.block, l.half = block, func() { close(reached) }
	l.mu.Unlock()
	released := false
	release := func() {
		if !released {
			released = true
			close(block)
		}
	}
	defer release()
	done := make(chan error, 1)
	go func() { done <- a.FetchPackPreviews() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("метод ждёт конца задания")
	}
	select {
	case <-reached:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("задание не пошло в фоне")
	}
	if got := ev.of("library:previews"); len(got) != 0 {
		t.Errorf("событие до конца задания: %v", got)
	}
	release()
	a.bg.Wait() // задание кончилось (оборванный лист — без событий: готовых листов нет)
}

// Паника в плитках не роняет каталог: «Можно скачать» пусто, приглашения нет, остальное отдано, в журнале
// только тип паники.
func TestDecorCatalogTilesPanic(t *testing.T) {
	a, l := tilesApp(t, true)
	if c, _ := a.DecorCatalog("bg"); len(c.Get) != 2 {
		t.Fatalf("до подмены плиток %d", len(c.Get))
	}
	rebuildPacks(t, a, func(h *libpacks.Host) {
		h.Fetcher = func() *fetch.Client { return fetch.New(l.src) }
		h.Store = func() (*decor.Store, error) { panic("сбой плиток") }
	})
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	c, err := a.DecorCatalog("bg")
	if err != nil || c.Kind != "bg" || len(c.Items) == 0 {
		t.Fatalf("каталог после паники: %v kind=%q items=%d", err, c.Kind, len(c.Items))
	}
	if c.Get == nil || len(c.Get) != 0 || c.Invite {
		t.Errorf("плитки после паники: %v invite=%v", c.Get, c.Invite)
	}
	if out := buf.String(); !strings.Contains(out, "сбой (string)") || strings.Contains(out, "сбой плиток") {
		t.Errorf("журнал: %q", out)
	}
}

// Раздача /media/lp/ через общий обработчик: годная клетка, хвосты, методы; общий разбор и сеть
// не задеты.
func TestMediaLP(t *testing.T) {
	a, l := tilesApp(t, true)
	next := false
	h := a.media(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next = true }))
	before := l.hits.Load()
	do := func(method, url string, hdr ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, nil)
		for i := 0; i+1 < len(hdr); i += 2 {
			r.Header.Set(hdr[i], hdr[i+1])
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := do("GET", "/media/lp/ivory/bg/1?v=abcdef12&px=64")
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		err != nil || cfg.Width != 64 || cfg.Height != 48 {
		t.Fatalf("годная клетка: %d %v %v", w.Code, w.Header(), err)
	}
	for _, url := range []string{
		"/media/lp/", "/media/lp/x", "/media/lp/ivory", "/media/lp/ivory/bg", "/media/lp/ivory/bg/2", "/media/lp/ivory/bg/-1",
		"/media/lp/ivory/bg/01", "/media/lp/%2e%2e/bg/0", "/media/lp/ivory%2Fbg%2F0%2F1", "/media/lp/ivory/Bg/0", "/media/lp/ivory/bg/0/1",
		"/media/lp/nope/bg/0", "/media/lp/ivory/sticker/0", "/media/lp/t/x",
	} {
		if w := do("GET", url); w.Code != 404 {
			t.Errorf("GET %s: %d", url, w.Code)
		}
	}
	if w := do("POST", "/media/lp/ivory/bg/0"); w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST: %d %v", w.Code, w.Header())
	}
	if w := do("HEAD", "/media/lp/ivory/bg/0"); w.Code != 200 || w.Body.Len() != 0 {
		t.Errorf("HEAD: %d, тело %d", w.Code, w.Body.Len())
	}
	if w := do("GET", "/media/lp/ivory/bg/0", "If-None-Match", "*", "If-Modified-Since", "Wed, 21 Oct 2037 07:28:00 GMT"); w.Code != 200 {
		t.Errorf("условный запрос: %d", w.Code)
	}
	if next {
		t.Error("запрос ушёл в следующий обработчик")
	}
	if l.hits.Load() != before {
		t.Errorf("раздача сходила в сеть: %d запросов", l.hits.Load()-before)
	}
}
