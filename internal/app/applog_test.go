package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// logLines — строки log.txt и log.old.txt из папки dir.
func logLines(t *testing.T, dir string) (cur, old []string) {
	t.Helper()
	read := func(name string) []string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			t.Fatal(err)
		}
		return strings.Fields(string(b))
	}
	return read("log.txt"), read("log.old.txt")
}

// Ротация внутри сессии: файл перерос лимит — уходит в .old, новый маленький, строки двух
// последних поколений целы; повторная ротация заменяет .old.
func TestRotatingLogInSession(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "log.txt")
	const limit = 100
	rl, err := newRotatingLog(path, limit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rl.f.Close() })
	var total int
	write := func(n int) {
		for range n {
			total++
			if _, err := rl.Write(fmt.Appendf(nil, "строка%03d\n", total)); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(5)
	if _, old := logLines(t, dir); old != nil {
		t.Fatal("рано: .old появился до лимита")
	}
	write(25)
	cur, old := logLines(t, dir)
	if len(old) == 0 {
		t.Fatal("нет log.old.txt после превышения лимита")
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() > limit+30 {
		t.Fatalf("новый файл не мал: %v, %v", st, err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != logFileMode {
		t.Errorf("права после переоткрытия %v", st.Mode().Perm())
	}
	// Последняя строка на месте, внутри поколений порядок и непрерывность без дыр.
	all := append(slices.Clone(old), cur...)
	if all[len(all)-1] != fmt.Sprintf("строка%03d", total) {
		t.Fatalf("последняя строка потеряна: %v", all[len(all)-1])
	}
	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] {
			t.Fatalf("порядок или дыра: %s после %s", all[i], all[i-1])
		}
	}
	write(40) // ещё поколения: .old перезаписан, но две последние пары целы
	cur2, old2 := logLines(t, dir)
	if slices.Equal(old, old2) || len(old2) == 0 {
		t.Fatalf(".old не заменён: %v → %v", old, old2)
	}
	all = append(slices.Clone(old2), cur2...)
	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] || all[i] != fmt.Sprintf("строка%03d", mustNum(t, all[i-1])+1) {
			t.Fatalf("дыра в последних поколениях: %s после %s", all[i], all[i-1])
		}
	}
	if all[len(all)-1] != fmt.Sprintf("строка%03d", total) {
		t.Fatal("последняя строка потеряна")
	}
}

func mustNum(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimPrefix(s, "строка"))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Начальный счёт — размер уже лежащего файла: старый лог ротируется по первой же записи сверх лимита.
func TestRotatingLogStartsFromFileSize(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 80)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rl, err := newRotatingLog(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rl.f.Close() })
	if rl.size != 81 {
		t.Fatalf("начальный размер %d", rl.size)
	}
	for range 3 {
		if _, err := rl.Write([]byte("строка-строка-строка\n")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "log.old.txt")); err != nil {
		t.Fatal("старый файл не ушёл в .old:", err)
	}
}

// Не удалось открыть файл после ротации: запись не пропадает — идёт в консоль, паники и рекурсии
// в log нет; когда открытие снова удаётся, файл продолжается.
func TestRotatingLogReopenFails(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "log.txt")
	rl, err := newRotatingLog(path, 50)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if rl.f != nil {
			_ = rl.f.Close()
		}
	})
	var console bytes.Buffer
	w := logWriter{file: rl, console: &console}
	if _, err := w.Write([]byte(strings.Repeat("a", 60) + "\n")); err != nil {
		t.Fatal(err)
	}
	fail := true
	realOpen := rl.open
	rl.open = func(p string) (*os.File, error) {
		if fail {
			return nil, errors.New("занят")
		}
		return realOpen(p)
	}
	if _, err := w.Write([]byte("потеряться-нельзя\n")); err == nil {
		t.Fatal("ждали ошибку файла")
	}
	if !strings.Contains(console.String(), "потеряться-нельзя") {
		t.Fatalf("запись не дошла до консоли: %q", console.String())
	}
	fail = false
	if _, err := w.Write([]byte("после-сбоя\n")); err != nil {
		t.Fatal(err)
	}
	cur, _ := logLines(t, dir)
	if !slices.Contains(cur, "после-сбоя") {
		t.Fatalf("файл не продолжился: %v", cur)
	}
}

// Сбой переименования (log.old.txt — непустая папка: Remove и Rename падают): файл не закрывается
// и не открывается на каждой строке — следующая попытка только через limit байт; когда помеха
// ушла, ротация проходит.
func TestRotatingLogRenameFailsBacksOff(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "log.txt")
	blocker := filepath.Join(dir, "log.old.txt")
	if err := os.MkdirAll(filepath.Join(blocker, "занято"), 0o700); err != nil {
		t.Fatal(err)
	}
	const limit, lines = 50, 100
	rl, err := newRotatingLog(path, limit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if rl.f != nil {
			_ = rl.f.Close()
		}
	})
	var opens int
	realOpen := rl.open
	rl.open = func(p string) (*os.File, error) {
		opens++
		return realOpen(p)
	}
	line := []byte("0123456789\n")
	total := 0
	for range lines {
		n, err := rl.Write(line)
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	// Одна попытка на каждые limit байт сверх первого превышения; без отсрочки их было бы ~lines.
	if maxOpens := total/limit + 1; opens > maxOpens {
		t.Fatalf("файл переоткрывался %d раз при %d байтах (предел %d): отсрочки нет", opens, total, maxOpens)
	}
	if opens == 0 {
		t.Fatal("ротацию вообще не пробовали")
	}
	// logLines не годится: log.old.txt сейчас папка.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Fields(string(b))); got != lines {
		t.Fatalf("при сбое переименования строки не должны теряться: %d из %d", got, lines)
	}
	// Помеха ушла: ближайшая попытка (через limit байт) ротирует.
	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	for range lines {
		if _, err := rl.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if st, err := os.Stat(blocker); err != nil || st.IsDir() {
		t.Fatalf("после ухода помехи .old не появился: %v", err)
	}
}

// Параллельные писатели (под -race): мьютекс держит счёт и переоткрытие; строки целы (ни склеек,
// ни обрывов), лишних нет, последняя запись на месте.
func TestRotatingLogConcurrent(t *testing.T) {
	dir := testkit.TempDir(t)
	rl, err := newRotatingLog(filepath.Join(dir, "log.txt"), 200)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rl.f.Close() })
	const writers, each = 4, 50
	var wg sync.WaitGroup
	for g := range writers {
		wg.Go(func() {
			for i := range each {
				if _, err := rl.Write(fmt.Appendf(nil, "g%d-%d\n", g, i)); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	if _, err := rl.Write([]byte("g9-final\n")); err != nil {
		t.Fatal(err)
	}
	cur, old := logLines(t, dir)
	if len(old) == 0 {
		t.Fatal("за 200 строк при лимите 200 байт ротации не было")
	}
	all := append(slices.Clone(old), cur...)
	if len(all) > writers*each+1 {
		t.Fatalf("строк больше записанного: %d", len(all))
	}
	whole := regexp.MustCompile(`^(g[0-3]-\d+|g9-final)$`)
	for _, ln := range all {
		if !whole.MatchString(ln) {
			t.Fatalf("склейка или обрыв строки: %q", ln)
		}
	}
	if cur[len(cur)-1] != "g9-final" {
		t.Fatalf("последняя запись потеряна: %q", cur[len(cur)-1])
	}
}
