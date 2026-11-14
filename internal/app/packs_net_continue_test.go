package app

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// «Обновить» не рвётся на первой ошибке набора (0.29-137-7, design §4): неудачи — в res.Failed,
// ход обрывают только то, что повторится у следующего набора.

// fail — сбой архива по l.mode (id набора — до «-v» в имени): hang, 500, 429, 404, drop, redirect;
// true — ответ уже дан.
func (l *netLib) fail(w http.ResponseWriter, r *http.Request) bool {
	name := strings.TrimPrefix(r.URL.Path, "/dl/")
	id, _, _ := strings.Cut(name, "-v")
	l.mu.Lock()
	mode, zip := l.mode[id], l.zips[name]
	l.mu.Unlock()
	switch mode {
	case "hang":
		w.Header().Set("Content-Length", itoa(len(zip))) // длина верная: застой, а не «битый»
		_, _ = w.Write(zip[:len(zip)/2])
		w.(http.Flusher).Flush()
		<-r.Context().Done() // клиент оборвёт по застою
	case "500":
		http.Error(w, "сбой", http.StatusInternalServerError)
	case "429":
		http.Error(w, "слишком часто", http.StatusTooManyRequests)
	case "404":
		http.NotFound(w, r)
	case "drop":
		c, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = c.Close()
		}
	case "redirect":
		http.Redirect(w, r, "https://evil.example/x.zip", http.StatusFound)
	default:
		return false
	}
	return true
}

func (l *netLib) setMode(mode map[string]string) {
	l.mu.Lock()
	l.mode = mode
	l.mu.Unlock()
}

// fourZips — четыре набора по порядку каталога: ivory («Свадьба — айвори»), sage, ruby, plum.
func fourZips(t *testing.T) []string {
	t.Helper()
	zip := d1Zip(t)
	out := []string{zip}
	for _, p := range [][2]string{{"sage", "Шалфей"}, {"ruby", "Рубин"}, {"plum", "Слива"}} {
		out = append(out, rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = p[0], p[1] }))
	}
	return out
}

// getAll — «Обновить» (обновляет стоящие и сохраняет список) и затем «Скачать все»: так
// установка новых наборов выглядела до выборочной установки (137), а теперь «Обновить» новое
// только показывает. checked после первого шага сбрасывается: тесты смотрят, пишет ли его сам
// второй ход. Итог — сумма обоих (ошибка «Обновить» возвращается сразу).
func getAll(t *testing.T, a *App) (LibraryUpdate, error) {
	t.Helper()
	up, err := a.UpdateLibrary()
	if err != nil {
		return up, err
	}
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = 0 })
	all, err := a.DownloadAllLibrary()
	all.Added += up.Added
	all.Updated += up.Updated
	all.Bytes += up.Bytes
	all.Skipped = append(up.Skipped, all.Skipped...)
	all.Failed = append(up.Failed, all.Failed...)
	return all, err
}

func failedNames(res LibraryUpdate) []string {
	out := []string{}
	for _, k := range res.Failed {
		out = append(out, k.Name)
	}
	return out
}

// Ответ 404 (нет одного ассета) и битый архив посреди списка: остальные ставятся, неудачные — в Failed с причиной,
// checked пишется (поштучные неудачи проверку закрывают: иначе битый у издателя архив качался бы
// при каждом запуске), library:changed один раз; после починки «Скачать все» доставит остальное.
func TestUpdateContinuesAfterFailures(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zips := fourZips(t)
	l.publish(t, func(r map[string]any) {
		if r["id"] == "ruby" {
			r["sha256"] = strings.Repeat("0", 64)
		}
	}, zips...)
	l.setMode(map[string]string{"sage": "404"})
	res, err := getAll(t, a)
	if err != nil || res.Added != 2 {
		t.Fatalf("итог: %v %+v", err, res)
	}
	if got := failedNames(res); len(got) != 2 || got[0] != "Шалфей" || got[1] != "Рубин" {
		t.Fatalf("Failed: %v", res.Failed)
	}
	if res.Failed[0].Why != fetch.ErrNotFound.Error() || res.Failed[1].Why != "скачался с ошибкой, повторите" {
		t.Errorf("причины: %+v", res.Failed)
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked == 0 {
		t.Error("checked не записан при поштучных неудачах")
	}
	if got := installedOf(t, ev, 1); len(got) != 2 {
		t.Errorf("library:changed: %v", got)
	}
	noDownloadLeft(t, a)
	// Починили: оставшиеся два ставятся (строки раздела прежние), Failed пуст.
	l.setMode(nil)
	l.publish(t, nil, zips...)
	res, err = a.DownloadAllLibrary()
	if err != nil || res.Added != 2 || len(res.Failed) != 0 || res.Available != 0 {
		t.Fatalf("повтор: %v %+v", err, res)
	}
}

// Все наборы падают поштучно — ошибки вызова нет, Failed со всеми; без неудач Failed — пустой
// список, не nil (интерфейс не проверяет null).
func TestUpdateAllFailNoError(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, fourZips(t)...)
	l.setMode(map[string]string{"ivory": "404", "sage": "404", "ruby": "404", "plum": "404"})
	res, err := getAll(t, a)
	if err != nil || res.Added != 0 || len(res.Failed) != 4 || l.archives.Load() != 4 {
		t.Fatalf("все 404: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	l.setMode(nil)
	l.publish(t, nil, d1Zip(t))
	res, err = a.UpdateLibrary()
	if err != nil || res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("пустой Failed: %v %#v", err, res.Failed)
	}
}

// quick — клиент службы ждёт застой 150 мс, а не 30 с.
func quick(t *testing.T, a *App, l *netLib) {
	t.Helper()
	rebuildPacks(t, a, func(h *libpacks.Host) {
		h.Fetcher = func() *fetch.Client {
			c := fetch.New(l.src)
			c.SetTimeouts(150*time.Millisecond, time.Minute)
			return c
		}
	})
	if _, err := a.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
}

// Один тайм-аут набора ходу не мешает, второй подряд — обрыв: третий набор даже не запрошен.
func TestUpdateSecondTimeoutAborts(t *testing.T) {
	a, l := netApp(t, true)
	quick(t, a, l)
	l.publish(t, nil, fourZips(t)...)
	l.setMode(map[string]string{"ivory": "hang", "sage": "hang"})
	res, err := getAll(t, a)
	if !errors.Is(err, fetch.ErrTimeout) || l.archives.Load() != 2 || res.Added != 0 {
		t.Fatalf("два подряд: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := failedNames(res); len(got) != 1 || got[0] != "Свадьба — айвори" {
		t.Errorf("Failed до обрыва: %v", res.Failed)
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked != 0 {
		t.Error("checked записан при обрыве")
	}
}

// Тайм-ауты не подряд (между ними успех или иная неудача) ход не рвут: Failed со всеми.
func TestUpdateTimeoutsNotInARow(t *testing.T) {
	a, l := netApp(t, true)
	quick(t, a, l)
	l.publish(t, nil, fourZips(t)...)
	l.setMode(map[string]string{"ivory": "hang", "ruby": "hang"}) // тайм-аут, успех, тайм-аут, успех
	res, err := getAll(t, a)
	if err != nil || res.Added != 2 || l.archives.Load() != 4 {
		t.Fatalf("через успех: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := failedNames(res); len(got) != 2 || got[0] != "Свадьба — айвори" || got[1] != "Рубин" ||
		res.Failed[0].Why != fetch.ErrTimeout.Error() {
		t.Errorf("Failed: %+v", res.Failed)
	}
	b := fresh(t, l)
	quick(t, b, l)
	l.setMode(map[string]string{"ivory": "hang", "sage": "404", "ruby": "hang"}) // тайм-аут, 404, тайм-аут, успех
	res, err = getAll(t, b)
	if err != nil || len(res.Failed) != 3 || res.Added != 1 {
		t.Fatalf("через 404: %v %+v", err, res)
	}
}

// Нет связи (обрыв соединения) на одном наборе ходу не мешает — обрыв CDN на одном архиве не
// должен стоить остальных; второй обрыв подряд — рвёт ход, третий набор не запрошен. Адрес мимо
// белого списка рвёт ход на первом же наборе (транспорт сам повторяет оборванный GET раз —
// запросов на набор до двух).
func TestUpdateConnectionErrorsAbort(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, fourZips(t)...)
	l.setMode(map[string]string{"ivory": "drop"})
	res, err := getAll(t, a)
	if err != nil || res.Added != 3 || len(res.Failed) != 1 || res.Failed[0].Why != fetch.ErrOffline.Error() {
		t.Fatalf("один обрыв: %v %+v", err, res)
	}
	b := fresh(t, l)
	l.setMode(map[string]string{"ivory": "drop", "sage": "drop"})
	res, err = getAll(t, b)
	if !errors.Is(err, fetch.ErrOffline) || res.Added != 0 || len(res.Failed) != 1 || res.Failed[0].Name != "Свадьба — айвори" {
		t.Fatalf("два обрыва подряд: %v %+v", err, res)
	}
	c := fresh(t, l)
	before := l.archives.Load()
	l.setMode(map[string]string{"ivory": "redirect"})
	res, err = getAll(t, c)
	if !errors.Is(err, fetch.ErrAddress) || l.archives.Load()-before > 2 || res.Added != 0 || len(res.Failed) != 0 {
		t.Fatalf("адрес мимо списка: %v %+v, архивов %d", err, res, l.archives.Load()-before)
	}
}

// Ответ GitHub с ошибкой (429, 5xx) на втором наборе рвёт весь ход: сервер ответит так же и на
// следующий. Первый поставлен, третий не запрошен, в Failed ничего (обрыв — ошибкой вызова).
func TestNetStatusStopsBatch(t *testing.T) {
	for _, code := range []string{"429", "500"} {
		a, l := netApp(t, true)
		l.publish(t, nil, fourZips(t)...)
		l.setMode(map[string]string{"sage": code})
		res, err := getAll(t, a)
		var se *fetch.StatusError
		if !errors.As(err, &se) || l.archives.Load() != 2 || res.Added != 1 || len(res.Failed) != 0 {
			t.Errorf("%s: %v %+v, архивов %d", code, err, res, l.archives.Load())
		}
		if prefs.LibraryNetOf(a.cfgDir).Checked != 0 {
			t.Errorf("%s: checked записан при обрыве", code)
		}
	}
}

// Два битых архива подряд — обрыв (подмена тела по дороге повторится), битые через успех — нет.
func TestNetBrokenTwiceStops(t *testing.T) {
	bad := func(ids ...string) func(r map[string]any) {
		return func(r map[string]any) {
			if slices.Contains(ids, r["id"].(string)) {
				r["sha256"] = strings.Repeat("0", 64)
			}
		}
	}
	a, l := netApp(t, true)
	l.publish(t, bad("ivory", "sage"), fourZips(t)...)
	res, err := getAll(t, a)
	if !errors.Is(err, fetch.ErrBroken) || l.archives.Load() != 2 || res.Added != 0 {
		t.Fatalf("два подряд: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := failedNames(res); len(got) != 1 || got[0] != "Свадьба — айвори" {
		t.Errorf("Failed до обрыва: %v", res.Failed)
	}
	b := fresh(t, l)
	l.publish(t, bad("ivory", "ruby"), fourZips(t)...) // битый, успех, битый, успех
	res, err = getAll(t, b)
	if err != nil || res.Added != 2 || len(res.Failed) != 2 || l.archives.Load() != 6 {
		t.Fatalf("через успех: %v %+v, архивов %d", err, res, l.archives.Load())
	}
}
