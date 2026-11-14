package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Размер файла из ленты доезжает до PhotoRef: по нему сверяются пути при переносе проекта.
func TestAddPhotosKeepsSize(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	path := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	if _, _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	waitScan(t, a)
	photos := a.GetState().Photos
	if len(photos) != 1 || photos[0].Size == 0 {
		t.Fatalf("лента: %+v", photos)
	}
	v, err := a.NewSpread(0, []string{photos[0].ID})
	if err != nil || len(v.Spreads) != 1 {
		t.Fatalf("NewSpread: %v %+v", err, v)
	}
	ref, ok := a.editor().Ref(photos[0].ID)
	st, _ := os.Stat(path)
	if !ok || ref.Size == 0 || ref.Size != st.Size() {
		t.Fatalf("Size в PhotoRef = %d, ждали %d", ref.Size, st.Size())
	}
}

// Список папок читается под тем же замком, что и выдача поколения (rescanMu; App.mu — уже после
// Folders, чтобы прогрев раскладок его не держал): rescan, ждущий замок, видит папки,
// добавленные за время ожидания (иначе устаревший список победил бы новый).
func TestRescanReadsFoldersUnderLock(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dirA, dirB := testkit.TempDir(t), testkit.TempDir(t)
	testkit.JPEG(t, dirA, "a.jpg", 20, 10)
	testkit.JPEG(t, dirB, "b.jpg", 20, 10)
	ed := a.editor()
	if _, _, err := ed.AddFolder(dirA); err != nil {
		t.Fatal(err)
	}
	a.rescanMu.Lock()
	done := make(chan struct{})
	go func() { a.rescan(); close(done) }()
	time.Sleep(50 * time.Millisecond) // rescan упёрся в замок
	if _, _, err := ed.AddFolder(dirB); err != nil {
		t.Fatal(err)
	}
	a.rescanMu.Unlock()
	<-done
	waitScan(t, a)
	if n := len(a.GetState().Photos); n != 2 {
		t.Fatalf("в ленте %d фото, ждали 2 (a и b)", n)
	}
}

// После смены папок во время прогрева лента получает сброс (thumbs 0/0), а хвост
// отменённого прогрева молчит: иначе «Готовлю миниатюры 39 / 1476» остаётся навсегда.
func TestRescanResetsThumbs(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	var mu sync.Mutex
	var thumbs [][2]int
	a.emit = func(name string, data ...any) {
		if name == "thumbs" {
			mu.Lock()
			thumbs = append(thumbs, [2]int{data[0].(int), data[1].(int)})
			mu.Unlock()
		}
	}
	dir := testkit.TempDir(t)
	for i := range 120 {
		testkit.JPEG(t, dir, fmt.Sprintf("p%03d.jpg", i), 600, 400)
	}
	ed := a.editor()
	if _, _, err := ed.AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	deadline := time.Now().Add(10 * time.Second)
	for { // ждём, когда прогрев запущен
		a.mu.Lock()
		warming := a.warm != nil
		a.mu.Unlock()
		if warming {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("прогрев не начался")
		}
		time.Sleep(time.Millisecond)
	}
	ed.RemoveFolder(dir)
	a.rescan() // как после NewProject/смены папок
	mu.Lock()
	mark := len(thumbs)
	mu.Unlock()
	waitScan(t, a)
	time.Sleep(300 * time.Millisecond) // воркеры отменённого прогрева дорабатывают текущие фото

	mu.Lock()
	defer mu.Unlock()
	if mark == 0 || thumbs[mark-1] != [2]int{0, 0} {
		t.Fatalf("сразу после rescan ждали сброс 0/0, события: %v (mark %d)", thumbs, mark)
	}
	if len(thumbs) != mark {
		t.Fatalf("после сброса пришли события устаревшего прогрева: %v", thumbs[mark:])
	}
}

// Старый черновик (v1) перед первой записью копируется в «черновик (для 0.3).salbum», один раз.
func TestDraftV1BackedUp(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	v1 := []byte(`{"version":1,"folders":[],"settings":{"pageW":300,"pageH":300,"margin":12,"gap":3,"layflat":true,"dpi":300},"spreads":[],"photos":{}}`)
	if err := os.WriteFile(a.draftPath(), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	p, path, _ := a.loadStartup("")
	if p == nil || path != "" {
		t.Fatalf("черновик не открылся: %v %q", p, path)
	}
	a.ed.Store(mustEditor(t, p, path))
	a.flushSave()
	bak := salbum.BackupName(a.draftPath(), 1, 1)
	if got, _ := os.ReadFile(bak); string(got) != string(v1) {
		t.Fatalf("в копии для 0.3 не исходный черновик: %q", got)
	}
	if got, _ := os.ReadFile(a.draftPath()); string(got) == string(v1) {
		t.Fatal("черновик не записан в новой версии")
	}
	// Повторная запись копию не трогает.
	if err := os.WriteFile(bak, []byte("метка"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.flushSave()
	if got, _ := os.ReadFile(bak); string(got) != "метка" {
		t.Fatal("копия перезаписана")
	}
	// Новый проект копию не делает.
	b := newApp(t)
	b.cfgDir = testkit.TempDir(t)
	b.flushSave()
	if _, err := os.Stat(salbum.BackupName(b.draftPath(), 1, 1)); err == nil {
		t.Fatal("лишняя копия у нового проекта")
	}
}

type emitted struct {
	name string
	data []any
}

// Два rescan подряд: итог по второму, устаревшее поколение после запуска второго ничего не шлёт
// и не меняет реестр фото; убранная папка исчезает из ленты сразу. Под -race — без гонок.
func TestRescanGeneration(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var events []emitted
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, emitted{name, data})
	}

	dirA, dirB := testkit.TempDir(t), testkit.TempDir(t)
	for i := range 150 {
		testkit.JPEG(t, dirA, fmt.Sprintf("a%03d.jpg", i), 20, 10)
	}
	for i := range 5 {
		testkit.JPEG(t, dirB, fmt.Sprintf("b%d.jpg", i), 20, 10)
	}
	// Чтение второго поколения держит проба тома dirB (gate), пока не проверен флаг: без неё
	// 5 фото прочлись бы раньше проверки (на Linux — так и было). Первое поколение не держим —
	// оно должно и правда идти рядом со вторым.
	gate := make(chan struct{})
	var open sync.Once
	release := func() { open.Do(func() { close(gate) }) }
	old := reachable
	t.Cleanup(func() { reachable = old })
	t.Cleanup(release) // раньше ожидания фоновых задач newApp: Cleanup — в обратном порядке
	reachable = func(p string) bool {
		if p == dirB {
			<-gate
		}
		return old(p)
	}
	ed := a.editor()
	if _, _, err := ed.AddFolder(dirA); err != nil {
		t.Fatal(err)
	}
	a.rescan() // первое сканирование ещё идёт, когда начнётся второе
	ed.RemoveFolder(dirA)
	if _, _, err := ed.AddFolder(dirB); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	mu.Lock()
	mark := len(events)
	mu.Unlock()

	// сразу после возврата в ленте нет фото убранной папки
	for _, p := range a.GetState().Photos {
		if !library.Under(p.Path, dirB) {
			t.Fatalf("фото убранной папки осталось в ленте: %s", p.Path)
		}
	}
	if !a.GetState().Scan.Active {
		t.Fatal("после rescan сканирование должно быть активным")
	}
	release()

	deadline := time.Now().Add(10 * time.Second)
	for a.GetState().Scan.Active {
		if time.Now().After(deadline) {
			t.Fatal("сканирование не закончилось")
		}
		time.Sleep(5 * time.Millisecond)
	}
	st := a.GetState()
	if len(st.Photos) != 5 || st.Scan.Found != 5 || st.Scan.Read != 5 || st.Scan.Walking {
		t.Fatalf("итог: фото %d, ход %+v", len(st.Photos), st.Scan)
	}
	for _, p := range st.Photos {
		if !library.Under(p.Path, dirB) {
			t.Fatalf("в итоге чужое фото: %s", p.Path)
		}
	}
	a.mu.Lock()
	gen, byID := a.scanGen, len(a.byID)
	a.mu.Unlock()
	if gen != 2 || byID != 5 {
		t.Fatalf("поколение %d, реестр %d фото", gen, byID)
	}

	mu.Lock()
	defer mu.Unlock()
	var lastScan library.ScanProgress
	for i, e := range events {
		switch e.name {
		case "library":
			if i < mark {
				continue
			}
			for _, p := range e.data[0].([]library.Photo) {
				if !library.Under(p.Path, dirB) {
					t.Fatalf("после второго rescan пришло фото устаревшего поколения: %s", p.Path)
				}
			}
		case "scan":
			lastScan = e.data[0].(library.ScanProgress)
		}
	}
	if lastScan.Active || lastScan.Found != 5 {
		t.Fatalf("последнее событие scan: %+v", lastScan)
	}
}

// /media отдаёт верный Content-Type: прозрачный PNG — image/png, фото — image/jpeg.
func TestMediaContentType(t *testing.T) {
	dir := testkit.TempDir(t)
	a := newApp(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	logo := testkit.PNG(t, dir, "logo.png", 40, 20, color.NRGBA{200, 30, 30, 255})
	pic := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	a.byID["logo"] = library.Photo{ID: "logo", Path: logo, Name: "logo.png"}
	a.byID["pic"] = library.Photo{ID: "pic", Path: pic, Name: "a.jpg"}
	h := a.media(http.NotFoundHandler())
	for _, c := range []struct{ url, want string }{
		{"/media/t/logo", "image/png"}, {"/media/p/logo", "image/png"},
		{"/media/t/pic", "image/jpeg"}, {"/media/p/pic", "image/jpeg"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.url, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != c.want {
			t.Errorf("%s: %d, %q, ждали %q", c.url, rec.Code, rec.Header().Get("Content-Type"), c.want)
		}
	}
}

// Паника в методе с ошибкой — errInternal, а не nil: интерфейс не сочтёт операцию успешной.
func TestGuardErr(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	f := func() (err error) {
		defer guardErr("тест", &err)
		panic("сбой")
	}
	if err := f(); !errors.Is(err, errInternal) {
		t.Fatalf("ждали errInternal, получили %v", err)
	}
	ok := func() (err error) {
		defer guardErr("тест", &err)
		return errors.New("обычная")
	}
	if err := ok(); err == nil || errors.Is(err, errInternal) {
		t.Fatalf("обычная ошибка должна дойти как есть: %v", err)
	}
}

// guard глотает панику и пишет её в лог с именем места; без паники молчит.
func TestGuardRecovers(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	func() { defer guard("тихо") }()
	if buf.Len() != 0 {
		t.Fatalf("без паники в логе: %q", buf.String())
	}
	func() {
		defer guard("место сбоя")
		panic("сбой")
	}()
	if out := buf.String(); !strings.Contains(out, "ПАНИКА в место сбоя: сбой") {
		t.Fatalf("паника не записана: %q", out)
	}
}

// Чтения копий и буфера (WhereUsed, ClipState, CopyItems) под guard: паника в Editor не роняет
// процесс, ответ пустой, в log.txt — строка с именем метода. App без альбома (editor() = nil)
// даёт панику на разыменовании.
func TestReadsGuarded(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a := &App{}
	if got := a.WhereUsed([]string{"x", "y"}); got == nil || len(got) != 0 {
		t.Errorf("WhereUsed = %#v, ждали пустой список", got)
	}
	if got := a.CopyItems("s1", model.ClipSel{Refs: []string{"x"}}); got != (model.ClipInfo{}) {
		t.Errorf("CopyItems = %+v, ждали пустую сводку", got)
	}
	_ = a.ClipState() // чужого альбома нет: пусто, паники наружу нет
	out := buf.String()
	for _, name := range []string{"WhereUsed", "CopyItems"} {
		if !strings.Contains(out, "ПАНИКА в "+name+":") {
			t.Errorf("в логе нет паники %s: %q", name, out)
		}
	}
}

// publish устаревшего поколения (и «из будущего») молчит: ни событий, ни изменений реестра и хода;
// актуальное поколение обновляет реестр и шлёт library и scan, список nil — только scan.
func TestPublishStaleGeneration(t *testing.T) {
	a := newApp(t)
	var mu sync.Mutex
	var events []emitted
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, emitted{name, data})
	}
	names := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(events))
		for i, e := range events {
			out[i] = e.name
		}
		return out
	}
	old := library.Photo{ID: "old", Path: `C:\old.jpg`}
	fresh := []library.Photo{{ID: "n1", Path: `C:\n1.jpg`}, {ID: "n2", Path: `C:\n2.jpg`}}
	a.mu.Lock()
	a.scanGen = 5
	a.photos, a.byID = []library.Photo{old}, byIDMap([]library.Photo{old})
	a.scan = library.ScanProgress{Found: 1}
	a.mu.Unlock()

	for _, gen := range []int{4, 0, 6} {
		a.publish(gen, fresh, library.ScanProgress{Active: true, Found: 2})
	}
	if n := names(); len(n) != 0 {
		t.Fatalf("чужое поколение прислало события: %v", n)
	}
	a.mu.Lock()
	if len(a.photos) != 1 || a.photos[0].ID != "old" || len(a.byID) != 1 || a.scan.Found != 1 || a.scan.Active {
		t.Fatalf("реестр изменился: фото %v, ход %+v", a.photos, a.scan)
	}
	a.mu.Unlock()

	// Актуальное поколение: реестр, ход и оба события.
	a.publish(5, fresh, library.ScanProgress{Active: true, Found: 2})
	if n := names(); !slices.Equal(n, []string{"library", "scan"}) {
		t.Fatalf("события: %v", n)
	}
	a.mu.Lock()
	if len(a.photos) != 2 || len(a.byID) != 2 || a.scan.Found != 2 || !a.scan.Active {
		t.Fatalf("реестр не обновился: %v, ход %+v", a.photos, a.scan)
	}
	a.mu.Unlock()

	// Список nil (ход без списка): реестр остаётся, уходит только scan.
	a.publish(5, nil, library.ScanProgress{Found: 2})
	if n := names(); !slices.Equal(n, []string{"library", "scan", "scan"}) {
		t.Fatalf("события после хода: %v", n)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.photos) != 2 || a.scan.Active {
		t.Fatalf("ход без списка: фото %d, %+v", len(a.photos), a.scan)
	}
}

// App.AutoBuild: сверх MaxSpreads — ошибка с причиной, текущий вид и проект не меняются.
func TestAutoBuildSpreadLimit(t *testing.T) {
	a := newApp(t)
	p := model.New()
	for range model.MaxSpreads {
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#ffffff"}) // не пустые: пустой хвост сборка заняла бы (035)
	}
	a.ed.Store(mustEditor(t, p, ""))
	a.byID["a"] = library.Photo{ID: "a", Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	v, err := a.AutoBuild([]string{"a"}, -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if err == nil || !strings.Contains(err.Error(), "200 разворотов") { // 058: предел 200
		t.Fatalf("ждали ошибку про 200 разворотов, получили %v", err)
	}
	if len(v.Spreads) != model.MaxSpreads {
		t.Fatalf("вид изменился: %d разворотов", len(v.Spreads))
	}
}

// AutoOpts (041): поля приходят из UI по именам JSON; Variety вне 0…2 не ломает сборку
// (зажим). VaryLayouts: ничего подходящего — холостая (rev тот же: по нему UI показывает тост
// «менять нечего»); правка — один шаг отмены поверх сборки.
func TestAutoBuildOpts(t *testing.T) {
	var o model.AutoOpts
	if err := json.Unmarshal([]byte(`{"spreads":2,"maxPer":3,"variety":2,"families":["dense","hero"]}`), &o); err != nil ||
		!reflect.DeepEqual(o, model.AutoOpts{Spreads: 2, MaxPer: 3, Variety: model.VarietyMore, Families: []string{"dense", "hero"}}) {
		t.Fatalf("AutoOpts из JSON: %+v %v", o, err)
	}
	a := newApp(t)
	a.ed.Store(mustEditor(t, model.New(), ""))
	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		a.byID[id] = library.Photo{ID: id, Path: testkit.AbsPath(id + ".jpg"), W: 300, H: 200}
	}
	if ap, err := a.AutoPlan(ids, model.AutoOpts{Spreads: 2, MaxPer: 3, Variety: -7}); err != nil || ap.Spreads != 2 {
		t.Fatalf("AutoPlan: %+v %v", ap, err)
	}
	v, err := a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 2, MaxPer: 3, Variety: 99})
	if err != nil || len(v.Spreads) != 2 || !v.CanUndo {
		t.Fatalf("AutoBuild: %v, разворотов %d", err, len(v.Spreads))
	}
	for _, sel := range [][]string{nil, {}, {"нет"}} {
		if got, err := a.VaryLayouts(sel, 1, nil); err != nil || got.Rev != v.Rev {
			t.Fatalf("VaryLayouts(%v): %v, rev %d → %d", sel, err, v.Rev, got.Rev)
		}
	}
	all := []string{v.Spreads[0].ID, v.Spreads[1].ID, "нет"}
	got, err := a.VaryLayouts(all, -1, nil)
	if err != nil || got.Rev != v.Rev+1 || len(got.Spreads) != 2 {
		t.Fatalf("VaryLayouts: %v, rev %d → %d", err, v.Rev, got.Rev)
	}
	// Отмена: сначала «Разнообразить», затем сборка — по шагу.
	if u := a.Undo(); u.Spreads[0].Layout != v.Spreads[0].Layout || u.Spreads[1].Layout != v.Spreads[1].Layout {
		t.Fatalf("отмена «Разнообразить»: %s %s", u.Spreads[0].Layout, u.Spreads[1].Layout)
	}
	if u := a.Undo(); len(u.Spreads) != 0 || u.CanUndo {
		t.Fatalf("после отмены: разворотов %d, canUndo %v", len(u.Spreads), u.CanUndo)
	}
}

// TestVaryLayoutsXmp: App берёт оценки из XMP сканера (не от UI) и для сборки, и для
// «Разнообразить»: 5★ у последнего из 4 горизонтальных — главное фото (у лучшей раскладки
// главное — первое, так что без звезды тест падает).
func TestVaryLayoutsXmp(t *testing.T) {
	a := newApp(t)
	a.ed.Store(mustEditor(t, model.New(), ""))
	ids := []string{"a", "b", "c", "d"}
	for _, id := range ids {
		a.byID[id] = library.Photo{ID: id, Path: testkit.AbsPath(id + ".jpg"), W: 300, H: 200}
	}
	a.byID["d"] = library.Photo{ID: "d", Path: testkit.AbsPath("d.jpg"), W: 300, H: 200, Xmp: &library.Marks{Rating: 5}}
	big := func(v model.View) float64 {
		r, other := v.Spreads[0].Rects, 0.0
		for _, x := range r[:3] {
			other = max(other, x.W*x.H)
		}
		return r[3].W * r[3].H / other
	}
	v, err := a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 1, MaxPer: 64, Variety: model.VarietyNormal})
	if err != nil || big(v) < 1.6 {
		t.Fatalf("сборка: %v, фото d больше других в %.2f раза", err, big(v))
	}
	a.Undo()
	if v, err = a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 1, MaxPer: 64}); err != nil || big(v) >= 1.6 {
		t.Fatalf("«Меньше»: %v, %.2f — звезда не должна влиять", err, big(v))
	}
	if v, err = a.VaryLayouts([]string{v.Spreads[0].ID}, 0, nil); err != nil || big(v) < 1.6 {
		t.Fatalf("«Разнообразить»: %v, фото d больше других в %.2f раза", err, big(v))
	}
}

// Поля подписей (030): имя фото альбома и фото ленты, вставка списка, ошибка вставки.
func TestNamesMethods(t *testing.T) {
	a := newApp(t)
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("Иванов Петя.jpg"), W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: testkit.AbsPath("b.jpg"), W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a", "b"}}}
	a.ed.Store(mustEditor(t, p, ""))
	a.byID["lib"] = library.Photo{ID: "lib", Path: testkit.AbsPath("Петрова Анна Сергеевна.jpg")}
	if pv := a.Person("a"); pv.Last != "Иванов" || pv.First != "Петя" || !pv.FromFile {
		t.Fatalf("из имени файла: %+v", pv)
	}
	if pv := a.Person("lib"); pv.Middle != "Сергеевна" || !pv.FromFile {
		t.Fatalf("фото ленты: %+v", pv)
	}
	if pv := a.Person("нет"); pv != (model.PersonView{}) {
		t.Fatalf("чужое фото: %+v", pv)
	}
	r, err := a.PasteNames("s1", model.NamesOpts{Text: "Сидоров Иван\nКозлов Пётр\nЛишний"})
	if err != nil || r.Extra != 1 || r.Missing != 0 || r.View.Rev == 0 || r.Named != r.Seats || r.Seats == 0 {
		t.Fatalf("вставка: %+v, %v", r, err)
	}
	if pv := a.Person("a"); pv.Last != "Сидоров" || pv.FromFile {
		t.Fatalf("после вставки: %+v", pv)
	}
	if _, err := a.PasteNames("s1", model.NamesOpts{Text: strings.Repeat("А Б\n", 1001)}); err == nil {
		t.Fatal("длинный список принят")
	}
}

// missSetApp — разворот из фото x, y (файлов нет) и ok (есть: exists подменён), недостающие
// уже посчитаны; events — события missing по порядку.
func missSetApp(t *testing.T) (a *App, x, y string, events func() [][]string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	events = collected[[]string](a, "missing")
	a.exists = func(p string) bool { return filepath.Base(p) == "ok.jpg" }
	infos, ids := missInfos(a, "x.jpg", "y.jpg", "ok.jpg")
	if _, err := a.editor().NewSpread(-1, infos); err != nil {
		t.Fatal(err)
	}
	a.refreshMissing()
	waitMissing(t, a)
	x, y = ids[0], ids[1]
	if m := a.GetState().Missing; !slices.Equal(m, sorted(x, y)) {
		t.Fatalf("недостающие при загрузке %v", m)
	}
	return a, x, y, events
}

// Убрали битое фото — полоса «Не найдено» уходит без переоткрытия альбома; отмена её возвращает.
func TestMissingFollowsEdits(t *testing.T) {
	a, x, y, events := missSetApp(t)
	check := func(step string, want []string, n int) {
		t.Helper()
		waitMissing(t, a)
		evs := events()
		if len(evs) != n {
			t.Fatalf("%s: событий missing %d, ждали %d: %v", step, len(evs), n, evs)
		}
		if last := evs[n-1]; last == nil || !slices.Equal(last, want) {
			t.Fatalf("%s: событие %#v, ждали %v", step, last, want)
		}
		if m := a.GetState().Missing; !slices.Equal(m, want) {
			t.Fatalf("%s: GetState %v, ждали %v", step, m, want)
		}
	}
	a.RemovePhotos([]string{x})
	if a.missPending.Load() != 0 {
		t.Fatal("убрали фото: пересчёт с os.Stat не нужен")
	}
	check("убрали x", []string{y}, 2)
	a.RemovePhotos([]string{y})
	check("убрали y", []string{}, 3)
	a.Undo()
	check("отмена y", []string{y}, 4)
	a.Undo()
	check("отмена x", sorted(x, y), 5)

	// Набор фото прежний, убыло только целое фото, холостая правка — событий нет.
	rev := a.editor().View().Rev
	a.RemovePhotos([]string{"нет-такого"})
	if a.editor().View().Rev != rev {
		t.Fatal("холостая правка дала шаг")
	}
	if _, err := a.NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	a.RemovePhotos([]string{library.IDFor(filepath.Join(a.cfgDir, "ok.jpg"))})
	check("без смены недостающих", sorted(x, y), 5)
}

// collected подменяет a.emit: копит данные событий name (тип T) по порядку; результат — их копия.
func collected[T any](a *App, name string) func() []T {
	mu, got := new(sync.Mutex), []T(nil)
	a.emit = func(n string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		if n == name {
			got = append(got, data[0].(T))
		}
	}
	return func() []T {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}
