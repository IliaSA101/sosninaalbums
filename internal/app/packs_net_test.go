package app

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// netLib — библиотека на своём TLS-сервере: конверт каталога (body cat, подпись sig) и архивы по
// имени; считает запросы (каталог и архивы отдельно). Сеть наружу тесты не трогают.
type netLib struct {
	srv      *httptest.Server
	priv     ed25519.PrivateKey
	src      fetch.Source
	hits     atomic.Int32
	archives atomic.Int32

	mu    sync.Mutex
	cat   []byte
	sig   []byte
	seq   int64 // seq следующего publish; 0 — 1
	zips  map[string][]byte
	block chan struct{} // не nil — архив отдаёт половину и ждёт
	half  func()        // зовётся, когда половина архива отдана
}

func newNetLib(t *testing.T) *netLib {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	l := &netLib{priv: priv, zips: map[string][]byte{}}
	l.srv = httptest.NewTLSServer(http.HandlerFunc(l.serve))
	t.Cleanup(l.srv.Close)
	roots := l.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	l.src = fetch.Source{CatalogURL: l.srv.URL + "/catalog.json", Prefix: l.srv.URL + "/dl/", Hosts: []string{"127.0.0.1"},
		Keys: []ed25519.PublicKey{pub}, Roots: roots}
	return l
}

func (l *netLib) serve(w http.ResponseWriter, r *http.Request) {
	l.hits.Add(1)
	l.mu.Lock()
	cat, sig, block, half := l.cat, l.sig, l.block, l.half
	zip, ok := l.zips[strings.TrimPrefix(r.URL.Path, "/dl/")]
	l.mu.Unlock()
	switch {
	case r.URL.Path == "/catalog.json":
		env, _ := json.Marshal(map[string]string{"sig": string(sig), "body": base64.StdEncoding.EncodeToString(cat)})
		_, _ = w.Write(env)
	case ok:
		l.archives.Add(1)
		if block != nil {
			w.Header().Set("Content-Length", itoa(len(zip)))
			_, _ = w.Write(zip[:len(zip)/2])
			w.(http.Flusher).Flush()
			if half != nil {
				half()
			}
			select {
			case <-block:
			case <-r.Context().Done():
			}
			return
		}
		_, _ = w.Write(zip)
	default:
		http.NotFound(w, r)
	}
}

// publish — архивы в каталоге (строки как у library-pack.mjs), body подписан ключом сервера.
// mut правит строку перед подписью.
func (l *netLib) publish(t *testing.T, mut func(row map[string]any), zips ...string) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	rows := []map[string]any{}
	for _, p := range zips {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
			MinApp  string `json:"minApp"`
			Title   string `json:"title"`
		}
		if err := json.Unmarshal(readManifest(t, p), &m); err != nil {
			t.Fatal(err)
		}
		name := m.ID + "-v" + itoa(m.Version) + "/" + m.ID + "-" + itoa(m.Version) + ".zip"
		l.zips[name] = data
		sum := sha256.Sum256(data)
		row := map[string]any{"id": m.ID, "version": m.Version, "minApp": m.MinApp, "title": m.Title, "size": len(data),
			"sha256": hex.EncodeToString(sum[:]), "url": l.src.Prefix + name, "counts": map[string]int{"bg": 2}}
		if mut != nil {
			mut(row)
		}
		rows = append(rows, row)
	}
	l.cat, _ = json.Marshal(map[string]any{"format": 1, "seq": max(l.seq, 1), "packs": rows})
	l.sig = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(l.priv, l.cat)))
}

// netApp — App с библиотекой и источником netLib; согласие — по allowed.
func netApp(t *testing.T, allowed bool) (*App, *netLib) {
	t.Helper()
	a := decorApp(t)
	l := newNetLib(t)
	a.net.client = func() *fetch.Client { return fetch.New(l.src) }
	if allowed {
		if _, err := a.SetLibraryNet(true, false); err != nil {
			t.Fatal(err)
		}
	}
	return a, l
}

// capture — события App с данными.
type capture struct {
	mu  sync.Mutex
	evs []capEvent
}

type capEvent struct {
	name string
	data any
}

func captureEvents(a *App) *capture {
	c := &capture{}
	a.emit = func(name string, data ...any) {
		var d any
		if len(data) > 0 {
			d = data[0]
		}
		c.mu.Lock()
		c.evs = append(c.evs, capEvent{name, d})
		c.mu.Unlock()
	}
	return c
}

func (c *capture) of(name string) []any {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []any
	for _, e := range c.evs {
		if e.name == name {
			out = append(out, e.data)
		}
	}
	return out
}

func noDownloadLeft(t *testing.T, a *App) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(a.decorLib.Load().Root(), "Наборы", ".dl-ivory.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("временный файл скачивания остался: %v", err)
	}
}

// Без согласия — ни одного запроса: кнопка, проверка при запуске, список наборов, флажок без
// согласия; config.json с atStart без allowed — тоже.
func TestNoNetWithoutConsent(t *testing.T) {
	a, l := netApp(t, false)
	l.publish(t, nil, d1Zip(t))
	defer setStart(0, time.Hour)()
	if _, err := a.UpdateLibrary(); !errors.Is(err, errNoConsent) {
		t.Fatalf("без согласия: %v", err)
	}
	n, err := a.SetLibraryNet(false, true)
	if err != nil || n.Allowed || n.AtStart {
		t.Fatalf("флажок без согласия: %v %+v", err, n)
	}
	a.scheduleLibraryCheck()
	raw := `{"libraryNet":{"atStart":true}}`
	if err := os.WriteFile(prefs.ConfigPath(a.cfgDir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	a.scheduleLibraryCheck()
	a.startCheck()
	a.bg.Wait()
	if list, err := a.LibraryPacks(); err != nil || list.Net.Allowed || list.Net.AtStart || len(list.Packs) != 1 {
		t.Fatalf("список: %v %+v", err, list)
	}
	if l.hits.Load() != 0 {
		t.Fatalf("запросов без согласия: %d", l.hits.Load())
	}
}

// Обновление: новый набор ставится из сети (origin net), ход и library:changed; повтор — «всё
// свежее» без скачивания; временного файла нет; checked записан.
func TestUpdateLibrary(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	before := time.Now().UnixMilli()
	res, err := a.UpdateLibrary()
	fi, _ := os.Stat(zip)
	if err != nil || res.Added != 1 || res.Updated != 0 || res.Bytes != fi.Size() || res.Skipped == nil || len(res.Skipped) != 0 {
		t.Fatalf("обновление: %v %+v", err, res)
	}
	list, _ := a.LibraryPacks()
	if len(list.Packs) != 2 || list.Packs[1].ID != "ivory" || list.Packs[1].Origin != decor.OriginNet || !list.Packs[1].Removable ||
		list.Net.Checked < before || !list.Net.Allowed || list.Busy {
		t.Fatalf("список: %+v", list)
	}
	if len(ev.of("library:changed")) != 1 || len(packBgIDs(t, a)) != 2 {
		t.Fatalf("событий %d, плиток %d", len(ev.of("library:changed")), len(packBgIDs(t, a)))
	}
	stages := map[string]bool{}
	for _, d := range ev.of("library:progress") {
		p := d.(LibraryProgress)
		stages[p.Stage] = true
		if p.Stage == "download" && (p.Title != "Свадьба — айвори" || p.Total != fi.Size() || p.Done > p.Total) {
			t.Errorf("ход: %+v", p)
		}
	}
	if !stages["catalog"] || !stages["download"] || !stages["install"] {
		t.Errorf("этапы: %v", stages)
	}
	noDownloadLeft(t, a)
	again, err := a.UpdateLibrary()
	if err != nil || again.Added+again.Updated != 0 || l.archives.Load() != 1 || len(ev.of("library:changed")) != 1 {
		t.Fatalf("повтор: %v %+v, архивов %d", err, again, l.archives.Load())
	}
}

// Новая версия стоящего из сети — обновляется без вопроса; ключи старой версии, которых нет в
// новой, уходят. (Стоящий из файла сеть не заменяет — TestUpdateLibraryFromFile.)
func TestUpdateLibraryNewVersion(t *testing.T) {
	a, l := netApp(t, true)
	src := d1Zip(t)
	l.publish(t, nil, src)
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("первая версия: %v %+v", err, res)
	}
	before := packBgIDs(t, a)
	v2 := rezip(t, src, func(m map[string]any) { m["version"] = 2 }, before2File(t, src))
	l.publish(t, nil, v2)
	res, err := a.UpdateLibrary()
	if err != nil || res.Updated != 1 || res.Added != 0 {
		t.Fatalf("обновление: %v %+v", err, res)
	}
	list, _ := a.LibraryPacks()
	if p := list.Packs[1]; p.Version != 2 || p.Origin != decor.OriginNet || p.Bg != 1 {
		t.Fatalf("набор: %+v", p)
	}
	if after := packBgIDs(t, a); len(after) != 1 || len(before) != 2 {
		t.Fatalf("фоны: %v → %v", before, after)
	}
}

// before2File — имя второго фона набора (его нет в версии 2).
func before2File(t *testing.T, zip string) string {
	t.Helper()
	var m struct {
		Items []struct{ File, Kind string } `json:"items"`
	}
	if err := json.Unmarshal(readManifest(t, zip), &m); err != nil {
		t.Fatal(err)
	}
	var bg []string
	for _, it := range m.Items {
		if it.Kind == decor.KindBg {
			bg = append(bg, it.File)
		}
	}
	return bg[len(bg)-1]
}

// Удалённый пользователем — «Обновить» не возвращает (и не скачивает); «Добавить из файла»
// ставит и снимает из удалённых.
func TestUpdateLibrarySkipRemoved(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("первое: %v %+v", err, res)
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	if skip := prefs.LibraryNetOf(a.cfgDir).Skip; len(skip) != 1 || skip[0] != "ivory" {
		t.Fatalf("удалённые: %v", skip)
	}
	res, err := a.UpdateLibrary()
	if err != nil || res.Added != 0 || l.archives.Load() != 1 {
		t.Fatalf("удалённый вернулся: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if _, err := a.AddPackPath(zip); err != nil {
		t.Fatal(err)
	}
	if skip := prefs.LibraryNetOf(a.cfgDir).Skip; len(skip) != 0 {
		t.Fatalf("из файла не снял: %v", skip)
	}
}

// «Вернуть удалённые»: счётчик в списке и ответе удаления; сброс без сети и согласия, после
// него «Обновить» ставит набор снова; пустой пропуск — config.json не переписывается.
func TestResetLibrarySkip(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, d1Zip(t))
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("первое: %v %+v", err, res)
	}
	list, err := a.RemovePack("ivory")
	if err != nil || list.Net.Skipped != 1 {
		t.Fatalf("удаление: %v %+v", err, list.Net)
	}
	if list, _ := a.LibraryPacks(); list.Net.Skipped != 1 {
		t.Fatalf("список: %+v", list.Net)
	}
	if _, err := a.SetLibraryNet(false, false); err != nil {
		t.Fatal(err)
	}
	hits := l.hits.Load()
	n, err := a.ResetLibrarySkip()
	if err != nil || n.Skipped != 0 || n.Allowed || l.hits.Load() != hits {
		t.Fatalf("сброс: %v %+v, запросов %d", err, n, l.hits.Load()-hits)
	}
	if skip := prefs.LibraryNetOf(a.cfgDir).Skip; len(skip) != 0 {
		t.Fatalf("пропуск остался: %v", skip)
	}
	if _, err := a.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("после сброса не вернулся: %v %+v", err, res)
	}

	// Несколько сразу (мусор в файле не считается); сеть и согласие сброс не трогает.
	prefs.UpdateLibraryNet(a.cfgDir, func(c *prefs.LibraryNet) { c.Skip = []string{"ivory", "rose", "-bad", "rose"} })
	if list, _ := a.LibraryPacks(); list.Net.Skipped != 2 {
		t.Fatalf("несколько: %+v", list.Net)
	}
	if n, _ := a.ResetLibrarySkip(); n.Skipped != 0 || !n.Allowed {
		t.Fatalf("сброс нескольких: %+v", n)
	}

	// Холостой: файл с меткой в конце — перезапись её сняла бы.
	path := prefs.ConfigPath(a.cfgDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, "\n\n"...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := a.ResetLibrarySkip(); err != nil || n.Skipped != 0 || !n.Allowed {
		t.Fatalf("холостой: %v %+v", err, n)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatalf("config.json переписан холостым сбросом")
	}
}

// Ошибки: подпись, хэш архива, отмена посреди скачивания, занятая библиотека, набор новее
// программы — библиотека прежняя, временного файла нет, checked не записан (кроме пропуска).
func TestUpdateLibraryErrors(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	l.mu.Lock()
	l.sig = []byte(base64.StdEncoding.EncodeToString(make([]byte, 64)))
	l.mu.Unlock()
	if _, err := a.UpdateLibrary(); !errors.Is(err, fetch.ErrSignature) || l.archives.Load() != 0 {
		t.Fatalf("подпись: %v", err)
	}
	l.publish(t, func(r map[string]any) { r["sha256"] = strings.Repeat("0", 64) }, zip)
	if _, err := a.UpdateLibrary(); !errors.Is(err, fetch.ErrBroken) || !strings.Contains(err.Error(), "айвори") {
		t.Fatalf("хэш: %v", err)
	}
	noDownloadLeft(t, a)
	// Отмена: архив отдаёт половину и ждёт; «Отменить» посреди скачивания.
	l.publish(t, nil, zip)
	l.mu.Lock()
	l.block = make(chan struct{})
	l.half = a.CancelLibrary
	l.mu.Unlock()
	defer close(l.block)
	if _, err := a.UpdateLibrary(); !errors.Is(err, fetch.ErrCancelled) {
		t.Fatalf("отмена: %v", err)
	}
	noDownloadLeft(t, a)
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 || list.Net.Checked != 0 || len(ev.of("library:changed")) != 0 {
		t.Fatalf("библиотека изменилась: %+v", list)
	}
	// Занято другой операцией наборов.
	a.packs.lock()
	if _, err := a.UpdateLibrary(); !errors.Is(err, errPackBusy) {
		t.Fatalf("занято: %v", err)
	}
	a.packs.unlock()
	// Набор новее программы — пропуск с причиной, без скачивания.
	defer func(old []byte) { wailsJSON = old }(wailsJSON)
	wailsJSON = []byte(`{"info":{"productVersion":"0.17.0"}}`)
	hits := l.archives.Load()
	l.publish(t, func(r map[string]any) { r["minApp"] = "0.99.0" }, zip)
	res, err := a.UpdateLibrary()
	if err != nil || res.Added != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "0.99.0") || l.archives.Load() != hits {
		t.Fatalf("minApp: %v %+v", err, res)
	}
}

// setStart — задержка и период проверки при запуске на время теста.
func setStart(delay, every time.Duration) func() {
	d, e, o := startDelay, startEvery, startOffer
	startDelay, startEvery = delay, every
	return func() { startDelay, startEvery, startOffer = d, e, o }
}

// Проверка при запуске: с согласием и флажком — раз за сеанс (повтор domReady не повторяет),
// ставит молча и шлёт тост; свежий checked — без запроса; checked из будущего — пора.
func TestStartCheckOnce(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	l.publish(t, nil, d1Zip(t))
	if _, err := a.SetLibraryNet(true, true); err != nil {
		t.Fatal(err)
	}
	// Неудачная проверка (нет подписи в конверте) checked не пишет, но перезагрузка окна (второй domReady)
	// за тот же сеанс в сеть не идёт.
	good := l.sig
	l.mu.Lock()
	l.sig = nil
	l.mu.Unlock()
	a.scheduleLibraryCheck()
	a.bg.Wait()
	a.scheduleLibraryCheck()
	a.bg.Wait()
	if l.hits.Load() != 1 || len(ev.of("notice")) != 0 || prefs.LibraryNetOf(a.cfgDir).Checked != 0 {
		t.Fatalf("неудачная проверка: запросов %d, тостов %d", l.hits.Load(), len(ev.of("notice")))
	}
	l.mu.Lock()
	l.sig = good
	l.mu.Unlock()
	a.net.started.Store(false) // следующий сеанс
	l.hits.Store(0)
	a.scheduleLibraryCheck()
	a.scheduleLibraryCheck()
	a.bg.Wait()
	if l.hits.Load() != 2 || l.archives.Load() != 1 || len(ev.of("library:updated")) != 1 || len(ev.of("notice")) != 0 || len(ev.of("library:changed")) != 1 {
		t.Fatalf("запросов %d, архивов %d, итогов %d", l.hits.Load(), l.archives.Load(), len(ev.of("library:updated")))
	}
	// Фразу собирает фронт; Go отдаёт числа и тот же LibraryUpdate, что кнопка (073-2).
	if u := ev.of("library:updated")[0].(LibraryUpdate); u.Added != 1 || u.Updated != 0 {
		t.Errorf("итог: %+v", u)
	}
	for name, tc := range map[string]struct {
		checked time.Duration // назад от сейчас; < 0 — в будущем
		atStart bool
		want    int32
	}{
		"свежий":      {time.Hour, true, 0},
		"старый":      {21 * time.Hour, true, 1},
		"из будущего": {-time.Hour, true, 1},
		"без флажка":  {21 * time.Hour, false, 0},
	} {
		a.net.started.Store(false)
		prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) {
			n.AtStart, n.Checked = tc.atStart, time.Now().Add(-tc.checked).UnixMilli()
		})
		hits := l.hits.Load()
		a.scheduleLibraryCheck()
		a.bg.Wait()
		if got := l.hits.Load() - hits; got != tc.want {
			t.Errorf("%s: запросов %d, ждали %d", name, got, tc.want)
		}
	}
	// Выход программы до задержки — проверки нет.
	defer setStart(time.Hour, 20*time.Hour)()
	a.net.started.Store(false)
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = 0 })
	hits := l.hits.Load()
	a.scheduleLibraryCheck()
	a.stopCancel()
	a.bg.Wait()
	if l.hits.Load() != hits {
		t.Error("проверка после выхода")
	}
}

// При запуске новое больше предела — только library:offer (число и байты), без скачивания;
// checked записан; кнопка потом ставит.
func TestUpdateLibraryOffer(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	fi, _ := os.Stat(zip)
	startOffer = fi.Size() - 1
	a.startCheck()
	offers := ev.of("library:offer")
	if len(offers) != 1 || offers[0].(LibraryOffer) != (LibraryOffer{Count: 1, Bytes: fi.Size()}) || l.archives.Load() != 0 {
		t.Fatalf("предложение: %+v, архивов %d", offers, l.archives.Load())
	}
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 || list.Net.Checked == 0 || len(ev.of("notice")) != 0 {
		t.Fatalf("после предложения: %+v", list)
	}
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("кнопка: %v %+v", err, res)
	}
	// Ошибка при запуске — только log.txt, без тоста.
	l.mu.Lock()
	l.sig = nil
	l.mu.Unlock()
	a.startCheck()
	if len(ev.of("notice")) != 0 || len(ev.of("library:offer")) != 1 {
		t.Fatalf("ошибка при запуске дала тост")
	}
}

// Отзыв согласия: флажок снят, идущее обновление отменено, LibraryPacks показывает настройки.
func TestSetLibraryNet(t *testing.T) {
	a, _ := netApp(t, false)
	n, err := a.SetLibraryNet(true, true)
	if err != nil || !n.Allowed || !n.AtStart {
		t.Fatalf("разрешить: %v %+v", err, n)
	}
	ctx, done := a.net.begin(a.stopCtx)
	defer done()
	n, _ = a.SetLibraryNet(false, true)
	if n.Allowed || n.AtStart || ctx.Err() == nil {
		t.Fatalf("отзыв: %+v, отменено %v", n, ctx.Err())
	}
	if list, _ := a.LibraryPacks(); list.Net != n {
		t.Fatalf("список: %+v", list.Net)
	}
	a.CancelLibrary() // без идущего — ничего
}

// Ход: смена этапа и конец — сразу, байты подряд — не чаще progressGap.
func TestLibraryProgress(t *testing.T) {
	a := decorApp(t)
	ev := captureEvents(a)
	prog := a.libProgress()
	prog("catalog", "", 0, 0)
	for i := range 1000 {
		prog("download", "А", int64(i), 1000)
	}
	prog("download", "А", 1000, 1000)
	prog("download", "Б", 1, 1000)
	prog("install", "Б", 0, 0)
	got := ev.of("library:progress")
	if len(got) < 5 || len(got) > 10 {
		t.Fatalf("событий %d", len(got))
	}
	if p := got[len(got)-3].(LibraryProgress); p.Done != 1000 || p.Title != "А" {
		t.Errorf("конец набора: %+v", p)
	}
}

// readManifest — manifest.json архива.
func readManifest(t *testing.T, zipPath string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close() //nolint:errcheck // тест: чтение
	rc, err := zr.Open("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close() //nolint:errcheck // тест: чтение
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func itoa(n int) string { return strconv.Itoa(n) }

// Набор из файла сеть не заменяет: новая версия в каталоге — пропуск с причиной без скачивания,
// стоит прежний; новый набор того же каталога ставится; та же версия — без пропуска.
func TestUpdateLibraryFromFile(t *testing.T) {
	a, l := netApp(t, true)
	src := d1Zip(t)
	if _, err := a.AddPackPath(src); err != nil {
		t.Fatal(err)
	}
	v2 := rezip(t, src, func(m map[string]any) { m["version"] = 2 })
	sage := rezip(t, src, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, v2, sage)
	res, err := a.UpdateLibrary()
	if err != nil || res.Added != 1 || res.Updated != 0 || len(res.Skipped) != 1 || l.archives.Load() != 1 {
		t.Fatalf("обновление: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if k := res.Skipped[0]; k.Name != "Свадьба — айвори" || k.Why != "стоит из файла — обновите из файла или удалите" {
		t.Errorf("пропуск: %+v", k)
	}
	list, _ := a.LibraryPacks()
	if len(list.Packs) != 3 || list.Packs[1].ID != "ivory" || list.Packs[1].Version != 1 || list.Packs[1].Origin != decor.OriginFile ||
		list.Packs[2].ID != "sage" || list.Packs[2].Origin != decor.OriginNet {
		t.Fatalf("наборы: %+v", list.Packs)
	}
	l.publish(t, nil, src, sage)
	if res, err := a.UpdateLibrary(); err != nil || len(res.Skipped) != 0 || res.Added+res.Updated != 0 || l.archives.Load() != 1 {
		t.Fatalf("та же версия: %v %+v", err, res)
	}
}

// seq каталога: принятый сохраняется; меньший — «не прошёл проверку» без скачивания и без записи
// (и при запуске); равный (повтор выпуска) — принимается; отзыв согласия seq не стирает.
func TestUpdateLibrarySeq(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	seq := func() int64 { return prefs.LibraryNetOf(a.cfgDir).CatalogSeq }
	l.seq = 5
	l.publish(t, nil)
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 0 || seq() != 5 {
		t.Fatalf("seq 5: %v %+v, сохранён %d", err, res, seq())
	}
	l.seq = 4
	l.publish(t, nil, zip)
	if _, err := a.UpdateLibrary(); !errors.Is(err, fetch.ErrSignature) || l.archives.Load() != 0 || seq() != 5 {
		t.Fatalf("seq 4: %v, архивов %d, сохранён %d", err, l.archives.Load(), seq())
	}
	a.startCheck()
	if l.archives.Load() != 0 || seq() != 5 {
		t.Fatalf("seq 4 при запуске: архивов %d", l.archives.Load())
	}
	l.seq = 5
	l.publish(t, nil, zip)
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 || seq() != 5 {
		t.Fatalf("повтор seq 5: %v %+v", err, res)
	}
	l.seq = 9
	l.publish(t, nil, zip)
	if _, err := a.UpdateLibrary(); err != nil || seq() != 9 {
		t.Fatalf("seq 9: %v, сохранён %d", err, seq())
	}
	if _, err := a.SetLibraryNet(false, false); err != nil || seq() != 9 {
		t.Fatalf("отзыв: %v, сохранён %d", err, seq())
	}
}

// Ошибка диска при установке скачанного: тост — постоянный текст без пути (fetch.ErrDisk), в
// log.txt — тип ошибки, не её текст; папки настроек нет ни там, ни там; библиотека прежняя.
func TestUpdateLibraryDiskError(t *testing.T) {
	a, l := netApp(t, true)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	l.publish(t, nil, d1Zip(t))
	dir := filepath.Join(a.decorLib.Load().Root(), "Фоны") // папка вида — файл: временный фон не создать
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("не папка"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.UpdateLibrary()
	if !errors.Is(err, fetch.ErrDisk) || strings.Contains(err.Error(), a.cfgDir) {
		t.Fatalf("ошибка: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, a.cfgDir) || strings.Contains(out, filepath.ToSlash(a.cfgDir)) || !strings.Contains(out, "ошибка установки (*") {
		t.Fatalf("log.txt: %s", out)
	}
	noDownloadLeft(t, a)
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
		t.Fatalf("библиотека изменилась: %+v", list.Packs)
	}
}

// Отзыв согласия между первой проверкой и begin: согласие перечитывается после регистрации
// операции — ни одного запроса (кнопка и проверка при запуске); замок и отмена отпущены.
func TestRevokeBeforeBegin(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, d1Zip(t))
	old := beforeBegin
	t.Cleanup(func() { beforeBegin = old })
	beforeBegin = func() {
		if _, err := a.SetLibraryNet(false, false); err != nil {
			t.Error(err)
		}
	}
	if _, err := a.UpdateLibrary(); !errors.Is(err, errNoConsent) {
		t.Fatalf("кнопка: %v", err)
	}
	if _, err := a.SetLibraryNet(true, true); err != nil {
		t.Fatal(err)
	}
	a.startCheck()
	if l.hits.Load() != 0 {
		t.Fatalf("запросов после отзыва: %d", l.hits.Load())
	}
	beforeBegin = old
	if _, err := a.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("после отзыва и согласия: %v %+v", err, res)
	}
	a.net.mu.Lock()
	defer a.net.mu.Unlock()
	if a.net.cancel != nil {
		t.Error("отмена операции осталась")
	}
}
