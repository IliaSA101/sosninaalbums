package libpacks

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Задание образцов (спека 144 Г2, security 3–5): отмена, повторы, паузы, пределы, чистота папки.

// rowsFx — три набора с листами; возвращает строки каталога.
func rowsFx(fx *prevFx) (rows []fetch.Entry, previews []*fetch.Preview) {
	for i, id := range []string{"pack-a", "pack-b", "pack-c"} {
		p := fx.publish(id, id+".jpg", sheetJPEG(fx.t, 10+i))
		rows = append(rows, fetch.Entry{ID: id, Preview: p})
		previews = append(previews, p)
	}
	return rows, previews
}

// Без согласия (и без папки в памяти) — ни одного запроса.
func TestFetchPreviewsNoConsent(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 0 || len(fx.folder()) != 0 {
		t.Fatalf("без согласия: %d, запросов %d, папка %v", got, fx.totalHits(), fx.folder())
	}
}

// Круг: лист скачан, читается (байт в байт), права 0600/0700, событие; второй вызов качает только
// недостающее.
func TestFetchPreviewsOnlyMissing(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	fx.list(rows...)
	if got := fx.s.FetchPreviews(); got != 3 {
		t.Fatalf("скачано %d, ждали 3", got)
	}
	for i, p := range ps {
		data, err := fx.s.readPreview(p)
		if err != nil || !bytes.Equal(data, fx.body[rows[i].ID+".jpg"]) {
			t.Fatalf("лист %d: %v", i, err)
		}
		if st, _ := os.Stat(fx.sheetFile(p)); st.Mode().Perm() != 0o600 && !isWindows() {
			t.Errorf("права файла %v", st.Mode())
		}
	}
	if st, _ := os.Stat(filepath.Join(fx.dir, previewDirName)); st.Mode().Perm() != 0o700 && !isWindows() {
		t.Errorf("права папки %v", st.Mode())
	}
	ev := fx.ev.of("library:previews")
	if len(ev) != 1 || ev[0].(PreviewsReady).Ready != 3 {
		t.Errorf("событие: %+v", ev)
	}
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 3 {
		t.Fatalf("повтор: %d, запросов %d", got, fx.totalHits())
	}
	if err := os.Remove(fx.sheetFile(ps[1])); err != nil {
		t.Fatal(err)
	}
	if got := fx.s.FetchPreviews(); got != 1 || fx.hitsOf("pack-b.jpg") != 2 || fx.totalHits() != 4 {
		t.Fatalf("недостающий: %d, запросов %d", got, fx.totalHits())
	}
	if len(fx.ev.of("library:previews")) != 2 {
		t.Errorf("второе событие: %+v", fx.ev.of("library:previews"))
	}
}

func isWindows() bool { return os.PathSeparator == '\\' }

// Набор уже в библиотеке, «удалён вами» или нужна программа новее — его лист не качается.
func TestFetchPreviewsOnlyWanted(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	rows[0].MinApp = "" // list() выставит 0.1.0; новая программа — ниже
	fx.list(rows...)
	cat, _, _ := fx.s.cached()
	cat.Packs[2].MinApp = "99.0.0" // каталог общий — в тесте правим до задания
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.SetSkip("pack-b", true) })
	if got := fx.s.FetchPreviews(); got != 1 || fx.hitsOf("pack-a.jpg") != 1 || fx.totalHits() != 1 {
		t.Fatalf("скачано %d, запросов %d", got, fx.totalHits())
	}
}

// Неудача связи — пауза 10 мин: через 1 мин запросов нет, через 11 — снова пробует.
func TestFetchPreviewsRetryPause(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	fx.srv.Close() // связи нет
	if got := fx.s.FetchPreviews(); got != 0 {
		t.Fatalf("оффлайн: %d", got)
	}
	if len(fx.ev.of("library:previews")) != 0 || len(fx.folder()) != 0 && !onlyEmpty(fx.folder()) {
		t.Errorf("событие %v, папка %v", fx.ev.of("library:previews"), fx.folder())
	}
	fx.s.prev.mu.Lock()
	until := fx.s.prev.retryAt
	fx.s.prev.mu.Unlock()
	if !until.After(time.Now().Add(9 * time.Minute)) {
		t.Fatalf("пауза не поставлена: %v", until)
	}
	var tries int
	fx.s.h.Fetcher = func() *fetch.Client { tries++; return nil }
	fx.advance(time.Minute)
	fx.s.FetchPreviews()
	if tries != 0 {
		t.Fatal("в паузе служба дошла до клиента")
	}
	fx.advance(10 * time.Minute)
	fx.s.FetchPreviews()
	if tries != 1 {
		t.Fatalf("после паузы попыток %d", tries)
	}
}

func onlyEmpty(names []string) bool { return len(names) == 0 }

// 404 одного листа — один запрос за сеанс, остальные скачаны; пауза не ставится.
func TestFetchPreviewsBadRowOncePerSession(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	fx.mu.Lock()
	delete(fx.body, "pack-b.jpg")
	fx.mu.Unlock()
	fx.list(rows...)
	for range 3 {
		fx.s.FetchPreviews()
	}
	if fx.hitsOf("pack-b.jpg") != 1 || fx.hitsOf("pack-a.jpg") != 1 || fx.hitsOf("pack-c.jpg") != 1 {
		t.Fatalf("запросы: %v", fx.hits)
	}
	if !fx.s.sheetPresent(ps[0]) || !fx.s.sheetPresent(ps[2]) || fx.s.sheetPresent(ps[1]) {
		t.Errorf("листы на диске: %v", fx.folder())
	}
	if fx.s.prev.paused(time.Now()) {
		t.Error("404 поставил паузу")
	}
}

// 403/429/5xx — ограничение частоты: задание кончается и ставит паузу, дальше листы не просит.
func TestFetchPreviews429Pauses(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, http.StatusForbidden, http.StatusBadGateway} {
		fx := newPrevFx(t, nil)
		rows, _ := rowsFx(fx)
		fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
		fx.list(rows...)
		if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 1 {
			t.Fatalf("%d: скачано %d, запросов %d", code, got, fx.totalHits())
		}
		fx.advance(time.Minute)
		fx.s.FetchPreviews()
		if fx.totalHits() != 1 {
			t.Errorf("%d: в паузе пошли запросы %d", code, fx.totalHits())
		}
	}
}

// Отказ диска (Rename) — пауза, не плохой лист: после паузы лист просят снова.
func TestFetchPreviewsErrDiskPauses(t *testing.T) {
	fail := true
	fx := newPrevFx(t, func(h *Host) {
		h.Rename = func(a, b string) error {
			if fail {
				return errors.New("диск")
			}
			return os.Rename(a, b)
		}
	})
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 1 {
		t.Fatalf("отказ: %d, запросов %d", got, fx.totalHits())
	}
	fail = false
	fx.advance(11 * time.Minute)
	if got := fx.s.FetchPreviews(); got != 3 {
		t.Fatalf("после паузы: %d", got)
	}
}

// Rename упал: ни .dl-*.tmp, ни <sha>.jpg.
func TestFetchPreviewsRenameFail(t *testing.T) {
	fx := newPrevFx(t, func(h *Host) { h.Rename = func(string, string) error { return errors.New("нет") } })
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	fx.s.FetchPreviews()
	if names := fx.folder(); len(names) != 0 {
		t.Fatalf("в папке остались %v", names)
	}
}

// Тело больше заявленного размера (и меньше) — файла нет, лист в плохих, ход дальше.
func TestFetchPreviewsOversizeNoFile(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	big := append(bytes.Clone(fx.body["pack-a.jpg"]), bytes.Repeat([]byte{1}, fetch.MaxPreview+1)...)
	fx.mu.Lock()
	fx.body["pack-a.jpg"] = big // Content-Length больше заявленного
	fx.mu.Unlock()
	fx.setOverride("pack-b.jpg", func(w http.ResponseWriter, _ *http.Request) { // без Content-Length, потоком
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte{2}, int(ps[1].Size)))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte{3})
	})
	fx.list(rows...)
	// Два битых подряд — связь (144 Н4): первое задание встаёт на паузу, третий лист не просит.
	if got := fx.s.FetchPreviews(); got != 0 {
		t.Fatalf("скачано %d, ждали 0 (пауза после двух подряд)", got)
	}
	fx.advance(11 * time.Minute)
	// Второе задание: первый в плохих, второй снова битый (один подряд) — ход идёт к третьему.
	if got := fx.s.FetchPreviews(); got != 1 {
		t.Fatalf("скачано %d, ждали 1 (третий)", got)
	}
	if names := fx.folder(); len(names) != 1 || names[0] != ps[2].SHA256+".jpg" {
		t.Fatalf("в папке %v", names)
	}
}

// Другие байты при верном размере — sha не сошёлся, файла нет.
func TestFetchPreviewsBadSumNoFile(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	bad := bytes.Clone(fx.body["pack-a.jpg"])
	bad[len(bad)/2] ^= 0xff
	fx.mu.Lock()
	fx.body["pack-a.jpg"] = bad
	fx.mu.Unlock()
	fx.list(rows...)
	fx.s.FetchPreviews()
	if fx.s.sheetPresent(ps[0]) || !fx.s.prev.isBad(ps[0].SHA256) || len(fx.folder()) != 2 {
		t.Fatalf("папка %v", fx.folder())
	}
}

// Отзыв во время листа: задание обрывается, ни файла, ни .dl-*.tmp, новых запросов нет;
// пауза не ставится.
func TestFetchPreviewsRevokeStops(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	started := make(chan struct{})
	fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done() // держим ответ, пока клиент не оборвёт
	})
	fx.list(rows...)
	done := make(chan int)
	go func() { done <- fx.s.FetchPreviews() }()
	<-started
	if _, err := fx.s.SetNet(false, false); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Errorf("скачано %d", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("задание не остановилось")
	}
	if fx.totalHits() != 1 || len(fx.folder()) != 0 {
		t.Errorf("запросов %d, папка %v", fx.totalHits(), fx.folder())
	}
	if fx.s.prev.paused(time.Now()) {
		t.Error("отмена поставила паузу")
	}
}

// Отзыв между проверкой согласия и prevBegin (крючок BeforeBegin): ни одного запроса.
func TestFetchPreviewsRevokeRace(t *testing.T) {
	var fx *prevFx
	fx = newPrevFx(t, func(h *Host) {
		h.BeforeBegin = func() { prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = false }) }
	})
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 0 {
		t.Fatalf("после отзыва: %d, запросов %d", got, fx.totalHits())
	}
}

// «Отменить» окна «Наборы» (Cancel) образцы не трогает: лист докачан, Cancelable не видит их.
func TestCancelLibraryKeepsPreviews(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	started, release := make(chan struct{}), make(chan struct{})
	fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write(fx.body["pack-a.jpg"])
	})
	fx.list(rows...)
	done := make(chan int)
	go func() { done <- fx.s.FetchPreviews() }()
	<-started
	if fx.s.Cancelable() {
		t.Error("образцы показали кнопку «Отменить»")
	}
	fx.s.Cancel()
	close(release)
	if got := <-done; got != 3 || !fx.s.sheetPresent(ps[0]) {
		t.Fatalf("после Cancel скачано %d", got)
	}
}

// Задание не берёт замок наборов и не держит временный файл вне своей папки: во время листа
// «Скачать» (замок) свободен, .dl-*.tmp лежит в папке образцов.
func TestFetchPreviewsNotUnderLock(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	started, release := make(chan struct{}), make(chan struct{})
	fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write(fx.body["pack-a.jpg"])
	})
	fx.list(rows...)
	done := make(chan int)
	go func() { done <- fx.s.FetchPreviews() }()
	<-started
	rel, ok := fx.s.Hold()
	if !ok {
		t.Fatal("замок наборов занят заданием образцов")
	}
	rel()
	tmp, _ := filepath.Glob(filepath.Join(fx.dir, previewDirName, ".dl-*.tmp"))
	if len(tmp) != 1 {
		t.Errorf("временный файл: %v", tmp)
	}
	close(release)
	<-done
	if tmp, _ = filepath.Glob(filepath.Join(fx.dir, previewDirName, ".dl-*.tmp")); len(tmp) != 0 {
		t.Errorf("остатки: %v", tmp)
	}
}

// Выход программы (StopCtx отменён) — ни одного запроса.
func TestFetchPreviewsAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fx := newPrevFx(t, func(h *Host) { h.StopCtx = func() context.Context { return ctx } })
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	cancel()
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 0 {
		t.Fatalf("после выхода: %d, запросов %d", got, fx.totalHits())
	}
}

// Остатки .dl-*.tmp прошлого запуска убираются в начале задания; чужие файлы в папке — нет.
func TestFetchPreviewsCleansTmp(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	dir := filepath.Join(fx.dir, previewDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{".dl-1.tmp", "чужой.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fx.s.FetchPreviews()
	names := fx.folder()
	if slices.Contains(names, ".dl-1.tmp") || !slices.Contains(names, "чужой.txt") {
		t.Fatalf("в папке %v", names)
	}
}

// Только чтение библиотеки и список без превью — задание не идёт в сеть.
func TestFetchPreviewsNothingToDo(t *testing.T) {
	fx := newPrevFx(t, nil)
	if fx.s.FetchPreviews() != 0 { // списка нет
		t.Fatal("без списка")
	}
	fx.list(fetch.Entry{ID: "plain-pack"})
	if fx.s.FetchPreviews() != 0 || fx.totalHits() != 0 || len(fx.folder()) != 0 {
		t.Fatalf("строка без превью: запросов %d, папка %v", fx.totalHits(), fx.folder())
	}
}

// Два набора с общим листом — один запрос; параллельный вызов задания молча выходит.
func TestFetchPreviewsSharedSheetAndParallel(t *testing.T) {
	fx := newPrevFx(t, nil)
	p := fx.publish("pack-a", "shared.jpg", sheetJPEG(t, 50))
	fx.list(fetch.Entry{ID: "pack-a", Preview: p}, fetch.Entry{ID: "pack-b", Preview: p})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); fx.s.FetchPreviews() }()
	}
	wg.Wait()
	if fx.hitsOf("shared.jpg") != 1 {
		t.Fatalf("запросов %d", fx.hitsOf("shared.jpg"))
	}
}

// Гонки: задание ∥ новый список ∥ клетки ∥ чтение списка; запускать с -race.
func TestPreviewRace(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(4)
		go func() { defer wg.Done(); fx.s.FetchPreviews() }()
		go func() { defer wg.Done(); fx.s.remember(fetch.Catalog{Seq: int64(100 + i), Packs: rows}) }()
		go func() {
			defer wg.Done()
			for j := range 3 {
				_, _ = fx.s.Cell(context.Background(), rows[j].ID, "bg", j)
			}
		}()
		go func() {
			defer wg.Done()
			cat, _, _ := fx.s.cached()
			_ = previewWanted(*cat, nil, nil, "0.27.0")
			fx.s.gcPreviews(*cat)
		}()
	}
	wg.Wait()
}

// Журнал образцов — без адресов и путей, формат «библиотека: образцы — …»; слова свои по классу
// ошибки (404, битый лист, нет связи), тексты fetch («набор «…», «библиотека на GitHub») не идут.
func TestFetchPreviewsLogHasNoAddress(t *testing.T) {
	flip := func(fx *prevFx, name string) { // тот же размер, другой sha
		fx.mu.Lock()
		b := bytes.Clone(fx.body[name])
		b[len(b)/2] ^= 1
		fx.body[name] = b
		fx.mu.Unlock()
	}
	cases := []struct {
		name, want string
		setup      func(fx *prevFx)
	}{
		{"нет связи", "нет связи", func(fx *prevFx) { fx.srv.Close() }},
		{"404", "лист не найден на сервере", func(fx *prevFx) {
			fx.mu.Lock()
			delete(fx.body, "pack-a.jpg")
			fx.mu.Unlock()
		}},
		{"битый лист", "лист не сошёлся с каталогом", func(fx *prevFx) { flip(fx, "pack-a.jpg") }},
		{"ответ 502", "ответ 502", func(fx *prevFx) {
			fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := logTo(t)
			fx := newPrevFx(t, nil)
			rows, _ := rowsFx(fx)
			fx.list(rows...)
			c.setup(fx)
			fx.s.FetchPreviews()
			out := buf.String()
			if !strings.Contains(out, "библиотека: образцы — ") || !strings.Contains(out, c.want) {
				t.Errorf("нет слов %q: %q", c.want, out)
			}
			for _, bad := range []string{"127.0.0.1", fx.dir, "набор «", "библиотека на GitHub", "набор не записывается", "скачался с ошибкой"} {
				if strings.Contains(out, bad) {
					t.Errorf("в журнале %q: %q", bad, out)
				}
			}
		})
	}
}

// Два ErrBroken подряд — связь, не листы: пауза и выход, второй не в плохих, третий не запрошен.
func TestFetchPreviewsTwoBrokenIsLink(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	fx.mu.Lock()
	for _, n := range []string{"pack-a.jpg", "pack-b.jpg"} {
		b := bytes.Clone(fx.body[n])
		b[len(b)/2] ^= 1
		fx.body[n] = b
	}
	fx.mu.Unlock()
	fx.list(rows...)
	if got := fx.s.FetchPreviews(); got != 0 {
		t.Fatalf("скачано %d", got)
	}
	if fx.hitsOf("pack-c.jpg") != 0 {
		t.Error("после второго битого задание пошло дальше")
	}
	if !fx.s.prev.isBad(ps[0].SHA256) || fx.s.prev.isBad(ps[1].SHA256) {
		t.Errorf("плохие: первый %v, второй %v", fx.s.prev.isBad(ps[0].SHA256), fx.s.prev.isBad(ps[1].SHA256))
	}
	if !fx.s.prev.paused(fx.s.h.Now().Add(9 * time.Minute)) {
		t.Error("пауза не поставлена")
	}
	// Связь вернулась: после паузы второй и третий листы качаются (первый в плохих до конца сеанса).
	fx.mu.Lock()
	fx.body["pack-b.jpg"] = sheetJPEG(t, 11)
	fx.mu.Unlock()
	fx.advance(11 * time.Minute)
	if got := fx.s.FetchPreviews(); got != 2 {
		t.Errorf("после паузы скачано %d", got)
	}
}

// Битый лист, за ним целый, затем снова битый: подряд их нет — паузы нет, оба битых в плохих.
func TestFetchPreviewsBrokenNotConsecutive(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	fx.mu.Lock()
	for _, n := range []string{"pack-a.jpg", "pack-c.jpg"} {
		b := bytes.Clone(fx.body[n])
		b[len(b)/2] ^= 1
		fx.body[n] = b
	}
	fx.mu.Unlock()
	fx.list(rows...)
	if got := fx.s.FetchPreviews(); got != 1 {
		t.Fatalf("скачано %d", got)
	}
	if fx.s.prev.paused(fx.s.h.Now()) || !fx.s.prev.isBad(ps[0].SHA256) || !fx.s.prev.isBad(ps[2].SHA256) {
		t.Error("непрерывной серии нет: паузы быть не должно, оба битых в плохих")
	}
}

// Дополнение, чей донор не стоит и не в каталоге, плиток не даёт; донор в каталоге или в библиотеке — даёт.
func TestPreviewWantedOrphanAddon(t *testing.T) {
	p := &fetch.Preview{}
	ent := func(id, needs string) fetch.Entry {
		return fetch.Entry{ID: id, Needs: needs, Preview: p, Version: 1, MinApp: "0.1.0"}
	}
	cat := fetch.Catalog{Packs: []fetch.Entry{
		ent("orphan", "ghost"), ent("kid", "base"), ent("base", ""), ent("kid-have", "installed"),
	}}
	have := map[string]decor.PackInfo{"installed": {ID: "installed"}}
	var ids []string
	for _, e := range previewWanted(cat, have, nil, "0.27.0") {
		ids = append(ids, e.ID)
	}
	if slices.Contains(ids, "orphan") || !slices.Contains(ids, "kid") || !slices.Contains(ids, "base") || !slices.Contains(ids, "kid-have") {
		t.Errorf("отобраны %v", ids)
	}
}

// Остатки .dl-*.tmp убираются, даже когда качать нечего (все листы на месте или строк с превью нет).
func TestFetchPreviewsCleansTmpWhenNothingToDo(t *testing.T) {
	for _, name := range []string{"нет строк с превью", "все листы на месте"} {
		fx := newPrevFx(t, nil)
		if name == "нет строк с превью" {
			fx.list(fetch.Entry{ID: "plain-pack"})
		} else {
			rows, ps := rowsFx(fx)
			for i, p := range ps {
				fx.put(p, sheetJPEG(t, 10+i))
			}
			fx.list(rows...)
		}
		dir := filepath.Join(fx.dir, previewDirName)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".dl-9.tmp"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		fx.s.FetchPreviews()
		if slices.Contains(fx.folder(), ".dl-9.tmp") || fx.totalHits() != 0 {
			t.Errorf("%s: папка %v, запросов %d", name, fx.folder(), fx.totalHits())
		}
	}
}
