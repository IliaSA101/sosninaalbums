package collect

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// tmpLeft — свои временные файлы, оставшиеся в папке сбора.
func tmpLeft(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, tmpSuffix) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func mustPlan(t *testing.T, sc *scene, dir string) Plan {
	t.Helper()
	pl, err := Make(sc.p, nil, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

// writerHook — подмена записи во временный файл на время теста.
func writerHook(t *testing.T, wrap func(w io.Writer) io.Writer) {
	t.Helper()
	old := newWriter
	newWriter = func(f *os.File) io.Writer { return wrap(old(f)) }
	t.Cleanup(func() { newWriter = old })
}

type funcWriter func(b []byte) (int, error)

func (f funcWriter) Write(b []byte) (int, error) { return f(b) }

// Сц. 6: повторный сбор в ту же папку — всё «уже на месте», копировать нечего; источник
// изменился (другой размер) — новая копия рядом под « (2)», прежняя цела.
func TestCollectAgainSkipsSame(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	collectTo(t, sc, dir, false)
	pl := mustPlan(t, sc, dir)
	for _, it := range pl.Items {
		if !it.Skip {
			t.Errorf("не «уже на месте»: %s", it.Rel)
		}
	}
	if pl.Bytes != 0 {
		t.Fatalf("байт к копированию %d", pl.Bytes)
	}
	if n, err := Copy(context.Background(), dir, &pl, nil); err != nil || n != 0 {
		t.Fatalf("повтор скопировал %d, %v", n, err)
	}
	src := filepath.Join(sc.a, "IMG_0001.jpg")
	old, _ := os.ReadFile(filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg"))
	put(t, src, "новое содержимое, другой длины")
	pl = mustPlan(t, sc, dir)
	if got := rels(pl)[src]; got != "Фото/JPG/IMG_0001 (2).jpg" {
		t.Fatalf("изменённый источник → %q", got)
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg")); !bytes.Equal(now, old) {
		t.Fatal("прежняя копия перезаписана")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Фото", "JPG", "IMG_0001 (2).jpg")); string(got) != "новое содержимое, другой длины" {
		t.Fatalf("новая копия: %q", got)
	}
}

// Сц. 6: чужой файл с тем же именем цел — план берёт « (2)»; имя заняли после плана — файл
// ложится под « (2)», Rel плана и путь в собранном альбоме — новые, чужой файл цел, временного нет.
func TestCollectNeverOverwrites(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	foreign := filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg")
	put(t, foreign, "чужое, другой длины")
	pl := mustPlan(t, sc, dir)
	if got := rels(pl)[filepath.Join(sc.a, "IMG_0001.jpg")]; got != "Фото/JPG/IMG_0001 (2).jpg" {
		t.Fatalf("при чужом файле → %q", got)
	}
	late := filepath.Join(dir, "Фото", "Прочие", "x.jpg")
	put(t, late, "появился после плана")
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatalf("Copy при появившемся имени: %v", err)
	}
	other := filepath.Join(sc.root, "Прочее", "x.jpg")
	if got := rels(pl)[other]; got != "Фото/Прочие/x (2).jpg" {
		t.Fatalf("появившееся имя → %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Фото", "Прочие", "x (2).jpg")); string(got) != "eeeeeeeeeeeeee" {
		t.Fatalf("копия под « (2)»: %q", got)
	}
	q, _ := Album(sc.p, nil, dir, pl)
	if ref := q.Photos[library.IDFor(filepath.Join(dir, "Фото", "Прочие", "x (2).jpg"))]; ref.Path == "" {
		t.Fatal("собранный альбом не смотрит на « (2)»")
	}
	for path, want := range map[string]string{foreign: "чужое, другой длины", late: "появился после плана"} {
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Fatalf("%s: %q", path, got)
		}
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("остались временные: %v", left)
	}
}

// Сц. 8: отмена посреди второго большого файла — временного нет, первый скопирован, второго нет;
// повтор докопирует, первый «уже на месте».
func TestCollectCancel(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	big1, big2 := filepath.Join(a, "1.jpg"), filepath.Join(a, "2.jpg")
	for _, f := range []string{big1, big2} {
		put(t, f, strings.Repeat("я", 3*bufSize/2)) // 3 куска
	}
	sc := &scene{root: root, p: album(t, []string{a})}
	for _, f := range []string{big1, big2} {
		sc.p.Photos[library.IDFor(f)] = model.PhotoRef{Path: f, W: 3, H: 2}
	}
	dir := filepath.Join(root, "Сбор")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writes := 0
	writerHook(t, func(w io.Writer) io.Writer {
		return funcWriter(func(b []byte) (int, error) {
			if writes++; writes == 4 { // первый кусок второго файла
				cancel()
			}
			return w.Write(b)
		})
	})
	pl := mustPlan(t, sc, dir)
	n, err := Copy(ctx, dir, &pl, nil)
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatalf("отмена: %d, %v", n, err)
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("остались временные: %v", left)
	}
	if _, err := os.Stat(filepath.Join(dir, "Фото", "A", "2.jpg")); err == nil {
		t.Fatal("недокопированный файл остался под своим именем")
	}
	pl = mustPlan(t, sc, dir)
	if !pl.Items[0].Skip || pl.Items[1].Skip {
		t.Fatalf("повтор: %+v", pl.Items)
	}
	if n, err := Copy(context.Background(), dir, &pl, nil); err != nil || n != 1 {
		t.Fatalf("докопирование: %d, %v", n, err)
	}
}

// Сц. 7: диск кончился посреди записи — ErrDiskFull, временный удалён, файла под именем нет.
func TestCollectDiskFull(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	full := error(syscall.ENOSPC)
	if runtime.GOOS == "windows" {
		full = syscall.Errno(112) // ERROR_DISK_FULL
	}
	writerHook(t, func(io.Writer) io.Writer {
		return funcWriter(func([]byte) (int, error) {
			return 0, &fs.PathError{Op: "write", Path: `C:\секрет\x.tmp`, Err: full}
		})
	})
	n, err := Copy(context.Background(), dir, new(mustPlan(t, sc, dir)), nil)
	if !errors.Is(err, ErrDiskFull) || n != 0 {
		t.Fatalf("диск заполнен: %d, %v", n, err)
	}
	if strings.Contains(err.Error(), "секрет") {
		t.Fatalf("в ошибке путь: %v", err)
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("остались временные: %v", left)
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Errorf("в папке файл %s", p)
		}
		return nil
	})
	if !errors.Is(diskFull(syscall.Errno(39)), ErrDiskFull) == (runtime.GOOS == "windows") {
		t.Fatal("ERROR_HANDLE_DISK_FULL на Windows — заполнен, на других ОС — другой код")
	}
	if errors.Is(diskFull(errors.New("другое")), ErrDiskFull) {
		t.Fatal("прочая ошибка — не «диск заполнен»")
	}
}

// Сц. 7: места мало — отказ до копирования с объёмами; папки сбора ещё нет — место по ближайшей
// существующей; хватает с запасом — nil; свободное место не узнать — ошибка.
func TestCollectNoSpace(t *testing.T) {
	root := testkit.TempDir(t)
	var asked string
	free := uint64(4 << 30)
	var ferr error
	old := freeBytes
	freeBytes = func(dir string) (uint64, error) { asked = dir; return free, ferr }
	t.Cleanup(func() { freeBytes = old })
	dir := filepath.Join(root, "нет", "Иванов")
	err := CheckSpace(dir, 6<<30)
	if err == nil || !strings.Contains(err.Error(), "нужно 6,0 ГБ, свободно 4,0 ГБ") || asked != root {
		t.Fatalf("мало места: %v (спросили %s)", err, asked)
	}
	if vol := filepath.VolumeName(root); vol != "" && !strings.Contains(err.Error(), "на диске "+vol+":") {
		t.Fatalf("нет буквы диска: %v", err)
	}
	if err := CheckSpace(dir, 4<<30-Reserve); err != nil {
		t.Fatalf("ровно с запасом: %v", err)
	}
	if err := CheckSpace(dir, 4<<30-Reserve+1); err == nil {
		t.Fatal("байт сверх запаса — должно не хватить")
	}
	if err := CheckSpace(dir, -5); err != nil {
		t.Fatalf("отрицательный объём: %v", err)
	}
	ferr = errors.New("нет ответа")
	if err := CheckSpace(dir, 1); err == nil || !strings.Contains(err.Error(), "свободное место") {
		t.Fatalf("ошибка FreeBytes: %v", err)
	}
}

// Сц. 11: каталог плана — ссылка (symlink/junction) или файл — отказ, через него не пишем.
func TestCollectRejectsLink(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	elsewhere := filepath.Join(sc.root, "Чужая папка")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pl := mustPlan(t, sc, dir)
	put(t, filepath.Join(dir, DirOwn), "файл на месте папки")
	if _, err := Copy(context.Background(), dir, &pl, nil); err == nil || !strings.Contains(err.Error(), "файл или ссылка") {
		t.Fatalf("файл на месте папки: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, DirOwn)); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, DirPhotos)
	if err := os.Symlink(elsewhere, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("ссылку на папку не создать: %v", err)
		}
		// Без прав на символьную ссылку — junction (его создаёт любой пользователь).
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, elsewhere).CombinedOutput(); err != nil { //nolint:gosec // пути теста
			t.Skipf("ни ссылки, ни junction (%v: %s)", err, out)
		}
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err == nil || !strings.Contains(err.Error(), "файл или ссылка") {
		t.Fatalf("ссылка на месте папки: %v", err)
	}
	if ents, _ := os.ReadDir(elsewhere); len(ents) != 0 {
		t.Fatalf("запись через ссылку: %v", ents)
	}
}

// Источник изменился после плана — ошибка, временного и файла под именем нет; Rel вне папки сбора
// (ошибка плана) — отказ до записи.
func TestCollectFileChanged(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl := mustPlan(t, sc, dir)
	put(t, filepath.Join(sc.root, "Прочее", "x.jpg"), "длиннее, чем было в плане")
	if _, err := Copy(context.Background(), dir, &pl, nil); err == nil || !strings.Contains(err.Error(), "изменился") {
		t.Fatalf("изменённый источник: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Фото", "Прочие", "x.jpg")); err == nil {
		t.Fatal("изменённый файл скопирован")
	}
	if left := tmpLeft(t, dir); len(left) != 0 {
		t.Fatalf("остались временные: %v", left)
	}
	for _, bad := range []string{"../x.jpg", "Фото/../../x.jpg", "/x.jpg", "", "Фото//x.jpg"} {
		bp := Plan{Items: []Item{{Kind: KindPhoto, Src: pl.Items[0].Src, Rel: bad, Size: pl.Items[0].Size}}}
		if _, err := Copy(context.Background(), dir, &bp, nil); err == nil || !strings.Contains(err.Error(), "недопустимый путь") {
			t.Fatalf("Rel %q: %v", bad, err)
		}
	}
}

// Ход: первый вызов — 0 из всего, последний — всё; не чаще progressEvery (часы подменены),
// счётчики не убывают; «уже на месте» в счёт не входит. Обрыв прошлого сбора (свой временный)
// убирается, чужой файл с похожим именем — нет.
func TestCollectProgress(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	stale := filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg.123"+tmpSuffix)
	keep := filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg.tmp")
	put(t, stale, "обрыв")
	put(t, keep, "чужой")
	tick := time.Unix(0, 0)
	old := now
	now = func() time.Time { tick = tick.Add(progressEvery / 2); return tick }
	t.Cleanup(func() { now = old })
	pl := mustPlan(t, sc, dir)
	type call struct {
		f, tf int
		b, tb int64
	}
	var calls []call
	n, err := Copy(context.Background(), dir, &pl, func(f, tf int, b, tb int64) { calls = append(calls, call{f, tf, b, tb}) })
	if err != nil {
		t.Fatal(err)
	}
	total := len(pl.Items)
	first, last := calls[0], calls[len(calls)-1]
	if first != (call{0, total, 0, pl.Bytes}) || last != (call{total, total, pl.Bytes, pl.Bytes}) || n != total {
		t.Fatalf("ход: первый %+v, последний %+v, всего %d", first, last, n)
	}
	if len(calls) > total+2 {
		t.Fatalf("ход слишком часто: %d вызовов на %d файлов", len(calls), total)
	}
	for i := 1; i < len(calls); i++ {
		if calls[i].f < calls[i-1].f || calls[i].b < calls[i-1].b {
			t.Fatalf("ход убывает: %+v → %+v", calls[i-1], calls[i])
		}
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("свой временный файл обрыва не убран")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("удалён чужой файл")
	}
	// Повтор: всё на месте — ход 0 из 0, ни одного копирования.
	calls = nil
	if n, err := Copy(context.Background(), dir, new(mustPlan(t, sc, dir)), func(f, tf int, b, tb int64) { calls = append(calls, call{f, tf, b, tb}) }); err != nil || n != 0 || calls[len(calls)-1] != (call{}) {
		t.Fatalf("повтор: %d, %v, %+v", n, err, calls)
	}
}
