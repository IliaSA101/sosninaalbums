package agentplaces

// Подсчёт папки перед тихим добавлением (150, §4.5): пределы, облачная заглушка, сетевой том внутри, ссылки,
// зависший диск. Проводка подмен App (st_dev, quietRemote…) — internal/app/agentquiet_test.go.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// quietHooks — подмены «ничего особенного»: том тот же, облака нет; изменяет тест.
func quietHooks(capPhotos bool) QuietHooks {
	return QuietHooks{
		Dev:    func(string) (uint64, bool) { return 1, true },
		Remote: func(string) bool { return false },
		Pause:  func() {},
		Cloud:  func(fs.DirEntry) bool { return false },

		CapPhotos: capPhotos,
	}
}

func touchAll(t *testing.T, dir string, names ...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func manyNames(n int, ext string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("f%05d%s", i, ext)
	}
	return out
}

// Пределы: 5000 фото — можно, 5001 — нельзя; 20001 запись и 501 папка — нельзя; RAW+XMP считаются записями,
// не фото; пусто и несуществующая папка (Big) — край. Без предела фото (открытие альбома) 5001 фото проходит.
func TestCountQuietLimits(t *testing.T) {
	mix := append(append(manyNames(2000, ".jpg"), manyNames(2000, ".xmp")...), manyNames(2000, ".cr2")...)
	d500 := realDir(t, "много_папок")
	for i := range 501 {
		if err := os.Mkdir(filepath.Join(d500, fmt.Sprint(i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d500ok := realDir(t, "папок_500")
	for i := range 499 { // вместе с корнем — ровно 500
		if err := os.Mkdir(filepath.Join(d500ok, fmt.Sprint(i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name      string
		dir       string
		capPhotos bool
		over      bool
		photos    int
	}{
		{"5000 фото", touchAll(t, realDir(t, "ровно"), manyNames(5000, ".jpg")...), true, false, 5000},
		{"5001 фото", touchAll(t, realDir(t, "лишнее"), manyNames(5001, ".jpg")...), true, true, 5001},
		{"5001 фото без предела", touchAll(t, realDir(t, "лишнее2"), manyNames(5001, ".jpg")...), false, false, 5001},
		{"RAW+JPG+XMP 6000 записей при 2000 фото", touchAll(t, realDir(t, "смесь"), mix...), true, false, 2000},
		{"20001 запись", touchAll(t, realDir(t, "записи"), manyNames(20001, ".txt")...), true, true, 0},
		{"20001 запись без предела фото", touchAll(t, realDir(t, "записи2"), manyNames(20001, ".txt")...), false, true, 0},
		{"501 папка", d500, true, true, 0},
		{"500 папок", d500ok, true, false, 0},
		{"пусто", realDir(t, "пусто"), true, false, 0},
	}
	for _, c := range cases {
		got := CountQuiet(c.dir, testkit.Limit(20*time.Second), quietHooks(c.capPhotos))
		if got.OverCap(c.capPhotos) != c.over || got.Cloud || got.Link || got.Remote {
			t.Errorf("%s: %+v, ждали over=%v", c.name, got, c.over)
		}
		if !c.over && got.Photos != c.photos {
			t.Errorf("%s: фото %d, ждали %d", c.name, got.Photos, c.photos)
		}
	}
	// Папки нет, путь не локальный, пустая строка — не уложились: Big, без паники.
	for _, bad := range []string{filepath.Join(realDir(t), "нет-такой"), "", "относительный/путь"} {
		if got := CountQuiet(bad, testkit.Limit(time.Second), quietHooks(true)); !got.Big || !got.Over() {
			t.Errorf("%q: %+v, ждали Big", bad, got)
		}
	}
	// Файл вместо папки: ReadDir не прочитает — тоже Big.
	f := filepath.Join(touchAll(t, realDir(t, "файл"), "a.jpg"), "a.jpg")
	if got := CountQuiet(f, testkit.Limit(time.Second), quietHooks(true)); !got.Big {
		t.Errorf("файл вместо папки: %+v", got)
	}
}

// Облачная заглушка во вложенной папке останавливает обход; обычные фото — нет.
func TestCountQuietCloud(t *testing.T) {
	dir := touchAll(t, realDir(t, "Облако"), "локально.jpg", "заметка.txt")
	touchAll(t, filepath.Join(dir, "вложенная"), "облако.jpg")
	h := quietHooks(true)
	if got := CountQuiet(dir, testkit.Limit(5*time.Second), h); got.Cloud || got.Photos != 2 || got.Over() {
		t.Errorf("обычная папка: %+v", got)
	}
	h.Cloud = func(e fs.DirEntry) bool { return e.Name() == "облако.jpg" }
	if got := CountQuiet(dir, testkit.Limit(5*time.Second), h); !got.Cloud || got.Link {
		t.Errorf("заглушка: %+v", got)
	}
}

// Сетевой том внутри: подпапка с другим st_dev и признаком сети — Remote; тот же st_dev или проба корня
// без ответа — нет (проба тома не вызывается зря).
func TestCountQuietRemote(t *testing.T) {
	dir := realDir(t, "место")
	nas := filepath.Join(dir, "nas")
	touchAll(t, nas, "a.jpg")
	touchAll(t, filepath.Join(dir, "свой"), "b.jpg")
	var probed atomic.Value
	h := quietHooks(true)
	h.Dev = func(p string) (uint64, bool) {
		if p == nas {
			return 2, true
		}
		return 1, true
	}
	h.Remote = func(p string) bool { probed.Store(p); return true }
	got := CountQuiet(dir, testkit.Limit(5*time.Second), h)
	if !got.Remote || !got.Over() || probed.Load() != nas {
		t.Errorf("сетевой том: %+v, проба %v", got, probed.Load())
	}
	// Корень не отвечает на st_dev — сравнивать не с чем: Remote не ставится.
	probed.Store("")
	h.Dev = func(p string) (uint64, bool) { return 2, p != dir }
	if got = CountQuiet(dir, testkit.Limit(5*time.Second), h); got.Remote || probed.Load() != "" {
		t.Errorf("корень без st_dev: %+v, проба %v", got, probed.Load())
	}
	// Другой том, но не сетевой (внешний диск) — не Remote.
	h = quietHooks(true)
	h.Dev = func(p string) (uint64, bool) { return uint64(len(p)), true }
	if got = CountQuiet(dir, testkit.Limit(5*time.Second), h); got.Remote {
		t.Errorf("локальный том: %+v", got)
	}
}

// Ссылка (файл или папка) внутри — Link, обход встаёт: сканер идёт по ссылкам с локальной целью (аудит В1).
func TestCountQuietLink(t *testing.T) {
	target := touchAll(t, realDir(t, "цель"), "x.jpg")
	for _, name := range []string{"файл.jpg", "папка"} {
		dir := touchAll(t, realDir(t, "со_ссылкой_"+name), "a.jpg")
		to := target
		if name == "файл.jpg" {
			to = filepath.Join(target, "x.jpg")
		}
		if err := os.Symlink(to, filepath.Join(dir, name)); err != nil {
			t.Skipf("ссылку не создать: %v", err)
		}
		if got := CountQuiet(dir, testkit.Limit(5*time.Second), quietHooks(true)); !got.Link {
			t.Errorf("%s: %+v, ждали Link", name, got)
		}
	}
}

// Зависший диск: порция не возвращается — вызов не дольше срока, итог Big; после освобождения обход видит stop и
// выходит, не читая дальше.
func TestCountQuietHang(t *testing.T) {
	// Больше одной порции: без stop обход пошёл бы за второй и позвал Pause снова.
	dir := touchAll(t, realDir(t, "зависшая"), manyNames(quietBatch+10, ".jpg")...)
	release := make(chan struct{})
	again := make(chan struct{})
	var calls atomic.Int32
	h := quietHooks(true)
	h.Pause = func() {
		if calls.Add(1) == 1 {
			<-release
			return
		}
		close(again) // вторая порция после срока — ошибка
	}
	passed := make(chan struct{}) // зависшая порция дочитана: обход дошёл до проверки stop
	var once sync.Once
	h.Cloud = func(fs.DirEntry) bool { once.Do(func() { close(passed) }); return false }
	start := time.Now()
	got := CountQuiet(dir, 150*time.Millisecond, h)
	if !got.Big || !got.Over() || time.Since(start) > testkit.Limit(3*time.Second) {
		t.Errorf("зависший диск: %+v за %v", got, time.Since(start))
	}
	close(release)
	select {
	case <-passed:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("зависшая порция не вышла после освобождения")
	}
	select {
	case <-again:
		t.Error("обход читал дальше после срока")
	case <-time.After(100 * time.Millisecond): // отрицательное ожидание: второй порции не будет
	}
}

// Срок уже исчерпан до чтения (0): первая же порция видит until — Big, без зависания.
func TestCountQuietZeroTimeout(t *testing.T) {
	dir := touchAll(t, realDir(t, "срок"), "a.jpg")
	if got := CountQuiet(dir, 0, quietHooks(true)); !got.Big {
		t.Errorf("срок 0: %+v", got)
	}
}
