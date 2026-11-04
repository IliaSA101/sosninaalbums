package collect

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Отказы одного файла (copyOne): том источника не отвечает, источник удалён после плана, на месте
// временного — папка, источник растёт во время копирования, Sync/Close сообщили о нехватке места.
// Везде — ошибка, своего временного нет, чужое цело. Имя заняли во время копирования — « (2)».
func TestCollectCopyOneFails(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	src := filepath.Join(a, "1.jpg")
	put(t, src, "содержимое")
	sc := &scene{root: root, p: album(t, []string{a})}
	sc.p.Photos[library.IDFor(src)] = model.PhotoRef{Path: src, W: 3, H: 2}
	dir := filepath.Join(root, "Сбор")
	final := filepath.Join(dir, "Фото", "A", "1.jpg")
	try := func(name, want string, setup func()) {
		t.Helper()
		pl := mustPlan(t, sc, dir)
		setup()
		_, err := Copy(context.Background(), dir, &pl, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, ждали «%s»", name, err, want)
		}
		if left := tmpLeft(t, dir); len(left) != 0 {
			t.Errorf("%s: остались временные %v", name, left)
		}
	}
	oldReach := reachable
	try("том не отвечает", "не отвечает", func() { reachable = func(string) bool { return false } })
	reachable = oldReach
	try("источник удалён", "не открылся", func() {
		if err := os.Rename(src, src+".x"); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.Rename(src+".x", src); err != nil {
		t.Fatal(err)
	}
	blocker := final + "." + strconv.Itoa(os.Getpid()) + tmpSuffix
	try("на месте временного папка", "", func() {
		if err := os.MkdirAll(blocker, 0o755); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	grown := false // один раз: иначе чтение догоняет дописанное без конца
	writerHook(t, func(w io.Writer) io.Writer {
		return funcWriter(func(b []byte) (int, error) {
			if grown {
				return w.Write(b)
			}
			grown = true
			if f, err := os.OpenFile(src, os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				_, _ = f.WriteString("ещё")
				_ = f.Close()
			}
			return w.Write(b)
		})
	})
	try("источник вырос", "изменился", func() {})
	put(t, src, "содержимое")
	writerHook(t, func(w io.Writer) io.Writer {
		return funcWriter(func(b []byte) (int, error) {
			_ = os.WriteFile(final, []byte("чужой"), 0o644)
			return w.Write(b)
		})
	})
	// Имя заняли во время копирования — не ошибка: файл ложится под « (2)», чужой цел.
	busy := mustPlan(t, sc, dir)
	if _, err := Copy(context.Background(), dir, &busy, nil); err != nil || busy.Items[0].Rel != "Фото/A/1 (2).jpg" {
		t.Fatalf("имя заняли: %v, %+v", err, busy.Items)
	}
	if got, _ := os.ReadFile(final); string(got) != "чужой" {
		t.Fatalf("чужой файл заменён: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Фото", "A", "1 (2).jpg")); string(got) != "содержимое" {
		t.Fatalf("копия под « (2)»: %q", got)
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("имя заняли: остались временные %v", left)
	}
	for _, f := range []string{final, filepath.Join(dir, "Фото", "A", "1 (2).jpg")} {
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
	}
	writerHook(t, func(w io.Writer) io.Writer { return w })
	oldSync := syncClose
	syncClose = func(f *os.File) error { _ = f.Close(); return syscall.ENOSPC }
	t.Cleanup(func() { syncClose = oldSync })
	if _, err := Copy(context.Background(), dir, new(mustPlan(t, sc, dir)), nil); !errors.Is(err, ErrDiskFull) {
		t.Fatalf("Sync без места: %v", err)
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("Sync без места: остались временные %v", left)
	}
}

// Папка сбора — файл или лежит под файлом; отмена до начала — ничего не копируется.
func TestCollectBadDir(t *testing.T) {
	sc := newScene(t)
	file := filepath.Join(sc.root, "файл")
	put(t, file, "x")
	for _, dir := range []string{file, filepath.Join(file, "Иванов")} {
		if _, err := Copy(context.Background(), dir, new(mustPlan(t, sc, "")), nil); err == nil {
			t.Errorf("папка сбора %s принята", dir)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := filepath.Join(sc.root, "Сбор")
	if n, err := Copy(ctx, dir, new(mustPlan(t, sc, dir)), nil); !errors.Is(err, context.Canceled) || n != 0 {
		t.Fatalf("отмена до начала: %d, %v", n, err)
	}
}

// Источник с нелокальным путём или на неотвечающем томе — пропавший; на месте имени назначения
// папка (не обычный файл) — имя дальше, не «уже на месте».
func TestCollectSourceChecks(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	f := filepath.Join(a, "IMG.jpg")
	p := album(t, []string{a}, f)
	p.Photos["rel"] = model.PhotoRef{Path: filepath.Join("отн", "x.jpg")}
	dir := filepath.Join(root, "Сбор")
	if err := os.MkdirAll(filepath.Join(dir, "Фото", "A", "IMG.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	pl, err := Make(p, nil, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Missing) != 1 || pl.Missing[0] != "rel" || rels(pl)[f] != "Фото/A/IMG (2).jpg" {
		t.Fatalf("план: %v, пропавшие %v", rels(pl), pl.Missing)
	}
	old := reachable
	reachable = func(string) bool { return false }
	t.Cleanup(func() { reachable = old })
	if pl, err = Make(p, nil, "", os.Stat); err != nil || len(pl.Items) != 0 || len(pl.Missing) != 2 {
		t.Fatalf("том не отвечает: %+v, %v", pl, err)
	}
}

// Save: записать некуда (на месте папки альбома — файл) — ошибка; записанное не открывается
// (путь с NUL) — ошибка, а не тихий успех.
func TestCollectSaveFails(t *testing.T) {
	root := testkit.TempDir(t)
	file := filepath.Join(root, "файл")
	put(t, file, "x")
	q := model.New()
	if err := Save(q, model.Side{}, filepath.Join(file, "a.salbum"), Plan{}); err == nil {
		t.Fatal("запись под файлом прошла")
	}
	dir := filepath.Join(root, "Сбор")
	q.Photos["bad"] = model.PhotoRef{Path: filepath.Join(dir, "a\x00.jpg"), W: 3, H: 2}
	q.Spreads = []*model.Spread{{ID: "0123456789a1", Photos: []string{"bad"}}}
	if err := Save(q, model.Side{}, filepath.Join(dir, "a.salbum"), Plan{}); err == nil || !strings.Contains(err.Error(), "не открылся") {
		t.Fatalf("неоткрываемый альбом: %v", err)
	}
}

// Имена фото заняли после плана: фото — под « (2)», сайдкары обоих видов — за ними
// («IMG (2).xmp», «K (2).jpg.xmp»); имя сайдкара заняли — он не копируется (SideBusy), из Items
// убран, фото скопировано, чужие файлы целы.
func TestCollectSidecarFollows(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	j, k, l := filepath.Join(a, "IMG.jpg"), filepath.Join(a, "K.jpg"), filepath.Join(a, "L.jpg")
	p := album(t, []string{a}, j, k, l)
	put(t, filepath.Join(a, "IMG.xmp"), "<short/>")
	put(t, k+".xmp", "<long/>")
	put(t, filepath.Join(a, "L.xmp"), "<l/>")
	dir := filepath.Join(root, "Сбор")
	pl, err := Make(p, nil, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "Фото", "A")
	foreign := map[string]string{"IMG.jpg": "чужое фото", "K.jpg": "чужое фото 2", "L.xmp": "чужой сайдкар"}
	for name, s := range foreign {
		put(t, filepath.Join(sub, name), s)
	}
	n, err := Copy(context.Background(), dir, &pl, nil)
	if err != nil || n != 5 || pl.SideBusy != 1 || len(pl.Items) != 5 {
		t.Fatalf("Copy: %d, %v; SideBusy %d, элементов %d", n, err, pl.SideBusy, len(pl.Items))
	}
	want := map[string]string{
		j: "Фото/A/IMG (2).jpg", filepath.Join(a, "IMG.xmp"): "Фото/A/IMG (2).xmp",
		k: "Фото/A/K (2).jpg", k + ".xmp": "Фото/A/K (2).jpg.xmp", l: "Фото/A/L.jpg",
	}
	if got := rels(pl); !mapsEqual(got, want) {
		t.Fatalf("имена:\n%v\nждали\n%v", got, want)
	}
	for name, s := range map[string]string{"IMG (2).xmp": "<short/>", "K (2).jpg.xmp": "<long/>"} {
		foreign[name] = s
	}
	for name, s := range foreign {
		if got, _ := os.ReadFile(filepath.Join(sub, name)); string(got) != s {
			t.Errorf("%s: %q, ждали %q", name, got, s)
		}
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("остались временные %v", left)
	}
}

// Звено папки подменили за время копирования (на месте папки — файл): своего файла — до
// переименования, папки следующего файла — до его записи. Ошибка «файл или ссылка», через
// подменённое звено ничего не записано, скопированное до того цело.
func TestCollectChainChanged(t *testing.T) {
	root := testkit.TempDir(t)
	a, b := filepath.Join(root, "A"), filepath.Join(root, "B")
	p := album(t, []string{a, b}, filepath.Join(a, "1.jpg"), filepath.Join(b, "2.jpg"))
	dir := filepath.Join(root, "Сбор")
	oldSync := syncClose
	t.Cleanup(func() { syncClose = oldSync })
	for _, c := range []struct {
		swap   string
		copied int
	}{{"A", 0}, {"B", 1}} {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		pl, err := Make(p, nil, dir, os.Stat)
		if err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(dir, "Фото", c.swap)
		swapped := false
		syncClose = func(f *os.File) error {
			err := oldSync(f)
			if !swapped {
				swapped = true
				if rerr := os.Rename(sub, sub+".x"); rerr != nil {
					t.Error(rerr)
				}
				put(t, sub, "файл на месте папки")
			}
			return err
		}
		n, err := Copy(context.Background(), dir, &pl, nil)
		if err == nil || !strings.Contains(err.Error(), "файл или ссылка") || n != c.copied {
			t.Fatalf("подменена %s: %d, %v", c.swap, n, err)
		}
		if got, _ := os.ReadFile(sub); string(got) != "файл на месте папки" {
			t.Fatalf("подменённое звено %s изменено: %q", c.swap, got)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Фото", "A", "1.jpg")); string(got) != "xxxxx" {
		t.Fatalf("скопированное до подмены: %q", got)
	}
}

// Переименование временного отказало не из-за занятого имени — ошибка, временного нет; все
// « (N)» заняты — ошибка «слишком много», а не бесконечный перебор.
func TestCollectPlaceFails(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	p := album(t, []string{a}, filepath.Join(a, "1.jpg"))
	dir := filepath.Join(root, "Сбор")
	old := renameNoReplace
	t.Cleanup(func() { renameNoReplace = old })
	tries := 0
	for _, c := range []struct {
		err  error
		want string
	}{{errors.New("отказ ФС"), "отказ ФС"}, {fs.ErrExist, "слишком много файлов"}} {
		tries = 0
		renameNoReplace = func(string, string) error { tries++; return c.err }
		pl, err := Make(p, nil, dir, os.Stat)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Copy(context.Background(), dir, &pl, nil); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%v: %v", c.err, err)
		}
		if left := tmpLeft(t, dir); len(left) != 0 {
			t.Fatalf("%v: остались временные %v", c.err, left)
		}
	}
	if tries != maxDup {
		t.Fatalf("попыток имени %d, ждали %d", tries, maxDup)
	}
}

// hungReader — чтение умершего сетевого диска: висит, пока не закроют unblock.
type hungReader struct{ started, unblock chan struct{} }

func (h hungReader) Read([]byte) (int, error) {
	close(h.started)
	<-h.unblock
	return 0, io.EOF
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// «Остановить» при висящем чтении: Read сразу возвращает отмену, источник закрывается, когда
// брошенное чтение вернётся (Close на Windows ждал бы его); без брошенного — закрывается сразу.
func TestCollectCancelHungRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := hungReader{started: make(chan struct{}), unblock: make(chan struct{})}
	r := &ctxReader{ctx: ctx, r: h}
	done := make(chan error, 1)
	go func() {
		_, err := r.Read(make([]byte, 8))
		done <- err
	}()
	<-h.started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Read после отмены: %v", err)
		}
	case <-time.After(5 * time.Second):
		close(h.unblock)
		t.Fatal("Read висит после отмены")
	}
	closed := make(chan struct{})
	r.release(closerFunc(func() error { close(closed); return nil }))
	select {
	case <-closed:
		t.Fatal("источник закрыт до возврата брошенного чтения")
	default:
	}
	close(h.unblock)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("источник не закрыт после возврата чтения")
	}
	var n int64
	r2 := &ctxReader{ctx: context.Background(), r: strings.NewReader("ab"), on: func(d int64) { n += d }}
	if got, err := r2.Read(make([]byte, 8)); got != 2 || err != nil || n != 2 {
		t.Fatalf("обычное чтение: %d, %v, ход %d", got, err, n)
	}
	shut := false
	r2.release(closerFunc(func() error { shut = true; return nil }))
	if !shut {
		t.Fatal("без брошенного чтения источник не закрыт сразу")
	}
}
