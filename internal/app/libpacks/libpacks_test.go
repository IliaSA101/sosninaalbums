package libpacks

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сквозные тесты наборов (установка, сеть, события) остаются в internal/app (0.26-126-15): они
// стоят на опоре App. Здесь — пустые поля Host, планировщик, порядок отзыва, ход, срок вопроса.

// evs — события службы по имени.
type evs struct {
	mu sync.Mutex
	n  map[string][]any
}

func (e *evs) emit(name string, data ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.n == nil {
		e.n = map[string][]any{}
	}
	var d any
	if len(data) > 0 {
		d = data[0]
	}
	e.n[name] = append(e.n[name], d)
}

func (e *evs) of(name string) []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.n[name]
}

// testHost — Host со своей папкой настроек и библиотекой; mut правит поля до New.
func testHost(t *testing.T, ev *evs, mut func(h *Host)) Host {
	t.Helper()
	dir := testkit.TempDir(t)
	store := decor.Open(dir)
	h := Host{
		CfgDir:        func() string { return dir },
		Version:       func() string { return "0.27.0" },
		Store:         func() (*decor.Store, error) { return store, nil },
		Emit:          ev.emit,
		GoBG:          func(fn func()) bool { go fn(); return true },
		StopCtx:       context.Background,
		AlbumLibs:     func() func(string) bool { return func(string) bool { return false } },
		Relink:        func(*decor.Store) {},
		PrintsChanged: func() {},
		PickFile:      func() (string, error) { return "", nil },
		RememberDir:   func(string) {},
		OpenDir:       func(string) error { return nil },
		Fetcher:       func() *fetch.Client { return nil },
		Now:           time.Now,
		Start:         func() (time.Duration, time.Duration, int64) { return 0, time.Hour, 1 << 30 },
		BeforeBegin:   func() {},
		AfterNetWrite: func() {},
	}
	if mut != nil {
		mut(&h)
	}
	return h
}

// d1Zip — набор, собранный скриптом выпуска D1, в своей папке.
func d1Zip(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "decor", "pack", "testdata", "d1-ivory.zip"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "ivory-1.zip")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics).
func TestLibpacksNewPanics(t *testing.T) {
	full := testHost(t, &evs{}, nil)
	testkit.NewPanics(t, full, func(h Host) { New(h) }, "libpacks.New")
}

// Отзыв согласия: флажок снят, идущее обновление отменено, список показывает настройки.
func TestSetLibraryNet(t *testing.T) {
	s := New(testHost(t, &evs{}, nil))
	n, err := s.SetNet(true, true)
	if err != nil || !n.Allowed || !n.AtStart {
		t.Fatalf("разрешить: %v %+v", err, n)
	}
	ctx, done := s.begin(context.Background())
	defer done()
	if !s.Cancelable() {
		t.Fatal("операция в begin, а отменять нечего")
	}
	n, _ = s.SetNet(false, true)
	if n.Allowed || n.AtStart || ctx.Err() == nil {
		t.Fatalf("отзыв: %+v, отменено %v", n, ctx.Err())
	}
	if list, _ := s.List(); list.Net != n {
		t.Fatalf("список: %+v", list.Net)
	}
	s.Cancel() // без идущего — ничего
}

// Ход: смена этапа и конец — сразу, байты подряд — не чаще progressGap.
func TestLibraryProgress(t *testing.T) {
	ev := &evs{}
	s := New(testHost(t, ev, nil))
	prog := s.progress()
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
	if p := got[len(got)-3].(Progress); p.Done != 1000 || p.Title != "А" {
		t.Errorf("конец набора: %+v", p)
	}
}

// Совпадение id со встроенным — своя причина, и для версии 1 (встроенный — версия 1); набор
// из файла — прежняя причина; без скачивания. Через сеть строки с таким id не бывает (fetch
// отбрасывает id не по грамматике) — план проверяется напрямую.
func TestNetPlanBuiltin(t *testing.T) {
	s := New(testHost(t, &evs{}, nil))
	if _, err := s.AddPath(d1Zip(t)); err != nil {
		t.Fatal(err)
	}
	st, _ := s.h.Store()
	cat := fetch.Catalog{Packs: []fetch.Entry{
		{ID: decor.BuiltinPack, Version: 1, MinApp: "0.1.0", Title: "Встроенные-1", Size: 10},
		{ID: decor.BuiltinPack, Version: 7, MinApp: "0.1.0", Title: "Встроенные-7", Size: 10},
		{ID: "ivory", Version: 2, MinApp: "0.1.0", Title: "Айвори", Size: 10},
	}}
	var res Update
	todo, need := s.netPlan(st, cat, &res)
	if len(todo) != 0 || need != 0 || len(res.Skipped) != 3 {
		t.Fatalf("план: %v %d %+v", todo, need, res.Skipped)
	}
	want := []Skip{{Name: "Встроенные-1", Why: whyBuiltin}, {Name: "Встроенные-7", Why: whyBuiltin}, {Name: "Айвори", Why: whyFromFile}}
	for i, k := range res.Skipped {
		if k != want[i] {
			t.Errorf("пропуск %d: %+v", i, k)
		}
	}
}

// Дополнение (needs, 129 §5): без донора netPlan его не берёт и о нём молчит — иначе каждое
// «Обновить» и каждый запуск качали бы его заново и писали тост. Донор в плане (в любом месте
// каталога, донор встаёт раньше) или уже стоит — ставится; в skip или нет вовсе — тихий пропуск; пустой needs — как обычный набор.
func TestNetPlanSkipsOrphanAddon(t *testing.T) {
	s := New(testHost(t, &evs{}, nil))
	if _, err := s.AddPath(d1Zip(t)); err != nil { // стоит донор "ivory"
		t.Fatal(err)
	}
	st, _ := s.h.Store()
	row := func(id, needs string) fetch.Entry {
		return fetch.Entry{ID: id, Version: 1, MinApp: "0.1.0", Title: id, Size: 10, Needs: needs}
	}
	cases := []struct {
		name string
		skip []string
		cat  []fetch.Entry
		want string // id в плане через запятую
	}{
		{"донор в плане раньше", nil, []fetch.Entry{row("sage", ""), row("sage-add", "sage")}, "sage,sage-add"},
		{"донор в плане позже", nil, []fetch.Entry{row("sage-add", "sage"), row("sage", "")}, "sage,sage-add"},
		{"цепочка в обратном порядке", nil, []fetch.Entry{row("sage-b", "sage-a"), row("sage-a", "sage"), row("sage", "")}, "sage,sage-a,sage-b"},
		{"цикл дополнений", nil, []fetch.Entry{row("sage-a", "sage-b"), row("sage-b", "sage-a")}, ""},
		{"донор позже, но в skip", []string{"sage"}, []fetch.Entry{row("sage-add", "sage"), row("sage", "")}, ""},
		{"донор стоит", nil, []fetch.Entry{row("ivory-add", "ivory")}, "ivory-add"},
		{"донор в skip", []string{"sage"}, []fetch.Entry{row("sage", ""), row("sage-add", "sage")}, ""},
		{"донор стоит, но в skip", []string{"ivory"}, []fetch.Entry{row("ivory-add", "ivory")}, ""},
		{"донора нет", nil, []fetch.Entry{row("sage-add", "sage")}, ""},
		{"донор пропущен по minApp", nil, []fetch.Entry{
			{ID: "sage", Version: 1, MinApp: "99.0.0", Title: "sage", Size: 10}, row("sage-add", "sage")}, ""},
		{"цепочка дополнений", nil, []fetch.Entry{row("sage", ""), row("sage-a", "sage"), row("sage-b", "sage-a")}, "sage,sage-a,sage-b"},
		{"пустой needs", nil, []fetch.Entry{row("sage", ""), row("moss", "")}, "sage,moss"},
	}
	for _, tc := range cases {
		prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.Skip = tc.skip })
		var res Update
		todo, need := s.netPlan(st, fetch.Catalog{Packs: tc.cat}, &res)
		var ids []string
		for _, e := range todo {
			ids = append(ids, e.ID)
		}
		if got := strings.Join(ids, ","); got != tc.want || need != int64(10*len(ids)) {
			t.Errorf("%s: в плане %q (%d байт), нужно %q", tc.name, got, need, tc.want)
		}
		// Пропуск дополнения — без записи в Skipped (тоста нет); причина есть только у донора с minApp.
		wantSkips := 0
		if tc.name == "донор пропущен по minApp" {
			wantSkips = 1
		}
		if len(res.Skipped) != wantSkips {
			t.Errorf("%s: пропуски с причиной: %+v", tc.name, res.Skipped)
		}
	}
}

// Порядок отзыва «запись → stop»: операция, вставшая в begin между записью и stop, перечитывает
// согласие и не идёт в сеть. Обратный порядок (stop → запись) пустил бы её: stop ещё нечего
// отменять, а согласие ещё не снято.
func TestRevokeWriteBeforeStop(t *testing.T) {
	var s *Service
	var got error
	calls := 0
	h := testHost(t, &evs{}, func(h *Host) {
		h.Fetcher = func() *fetch.Client { t.Error("сеть после отзыва"); return nil }
		h.AfterNetWrite = func() {
			calls++
			ctx, done, err := s.beginNet()
			if err == nil {
				defer done()
				if ctx.Err() == nil {
					got = errors.New("операция прошла после отзыва")
					return
				}
			}
			got = err
		}
	})
	s = New(h)
	if _, err := s.SetNet(true, false); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if _, err := s.SetNet(false, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !errors.Is(got, ErrNoConsent) {
		t.Fatalf("отзыв: вызовов %d, %v", calls, got)
	}
}

// Другие ошибки packEmptyErr не трогает; без причин — ошибка как есть; с причинами — первая и
// «и ещё N».
func TestPackEmptyErr(t *testing.T) {
	if err := packEmptyErr(ErrBusy, []Skip{{Name: "x", Why: "y"}}); err.Error() != ErrBusy.Error() {
		t.Errorf("чужая ошибка: %v", err)
	}
	if err := packEmptyErr(decor.ErrPackEmpty, nil); err.Error() != decor.ErrPackEmpty.Error() {
		t.Errorf("без причин: %v", err)
	}
	err := packEmptyErr(decor.ErrPackEmpty, []Skip{{Name: "А", Why: "п"}, {Name: "Б", Why: "р"}, {Name: "В", Why: "с"}})
	if !errors.Is(err, decor.ErrPackEmpty) || !strings.Contains(err.Error(), "А — п (и ещё 2)") || strings.Contains(err.Error(), "Б") {
		t.Errorf("с причинами: %v", err)
	}
}

// Вопрос «Заменить?»: GoBG отказал (App закрывается) — вопрос снят сразу, распакованное
// удалено, ответа нет; со сторожем — ответ раньше срока снимает сторожа (он не висит TTL).
func TestPendingGuard(t *testing.T) {
	refuse := false
	stopCtx, stop := context.WithCancel(context.Background())
	defer stop()
	var started sync.WaitGroup
	var stopped sync.WaitGroup
	s := New(testHost(t, &evs{}, func(h *Host) {
		h.StopCtx = func() context.Context { return stopCtx }
		h.GoBG = func(fn func()) bool {
			if refuse {
				return false
			}
			started.Add(1)
			stopped.Add(1)
			go func() { defer stopped.Done(); started.Done(); fn() }()
			return true
		}
	}))
	v1 := d1Zip(t)
	if _, err := s.AddPath(v1); err != nil {
		t.Fatal(err)
	}
	v2 := rezipVersion(t, v1, 2)
	res, err := s.AddPath(v2)
	if err != nil || res.Token == "" {
		t.Fatalf("вопрос: %v %+v", err, res)
	}
	started.Wait()
	if _, err := s.Confirm(res.Token); err != nil { // ответ — сторож завершается, не дожидаясь TTL
		t.Fatalf("замена: %v", err)
	}
	waitGuards := func(what string) {
		t.Helper()
		done := make(chan struct{})
		go func() { stopped.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("сторож вопроса не завершился " + what)
		}
	}
	waitGuards("после ответа")

	// Выход программы: сторож снимает вопрос и удаляет распакованное, не дожидаясь срока.
	res, err = s.AddPath(rezipVersion(t, v1, 4))
	if err != nil || res.Token == "" {
		t.Fatalf("вопрос перед выходом: %v %+v", err, res)
	}
	st, err := s.h.Store()
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpFiles(t, st.Root())) == 0 {
		t.Fatal("распакованного нет до выхода: проверка ничего не доказывает")
	}
	started.Wait()
	stop()
	waitGuards("после выхода программы")
	if left := tmpFiles(t, st.Root()); len(left) != 0 {
		t.Errorf("после выхода осталась распаковка: %v", left)
	}
	if _, err := s.Confirm(res.Token); !errors.Is(err, ErrStale) {
		t.Fatalf("ответ после выхода программы: %v", err)
	}

	refuse = true
	res, err = s.AddPath(rezipVersion(t, v1, 3))
	if err != nil || res.Token == "" {
		t.Fatalf("вопрос при закрытом фоне: %v %+v", err, res)
	}
	if _, err := s.Confirm(res.Token); !errors.Is(err, ErrStale) {
		t.Fatalf("ответ после закрытия: %v", err)
	}
}

// tmpFiles — временные файлы распаковки (*.tmp) под корнем библиотеки.
func tmpFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".tmp") {
			out = append(out, p)
		}
		return err
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return out
}

// rezipVersion — копия набора с другой версией манифеста.
func rezipVersion(t *testing.T, src string, ver int) string {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close() //nolint:errcheck // тест: чтение
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		if f.Name == "manifest.json" {
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			m["version"] = ver
			data, _ = json.Marshal(m)
		}
		w, _ := zw.Create(f.Name)
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "ivory-v.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
