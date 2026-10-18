package app

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// lockedBuf — буфер для лога теста; потокобезопасный (сканирование пишет из горутины).
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func captureLog(t *testing.T) *lockedBuf {
	t.Helper()
	buf := &lockedBuf{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

// dropApp — App без окна: события собираются в список текстов notice.
func dropApp(t *testing.T) (*App, *[]string) {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var notices []string
	a.emit = func(name string, data ...any) {
		if name == "notice" {
			mu.Lock()
			notices = append(notices, data[0].(string))
			mu.Unlock()
		}
	}
	return a, &notices
}

// Перетаскивание: CheckLocalPath раньше Stat — UNC-папка и устройство не доходят до файловой
// системы (иначе окно ждало бы сетевого таймаута); отказы — один тост со счётом.
func TestDropFolderChecksPathFirst(t *testing.T) {
	a, notices := dropApp(t)
	captureLog(t)
	var statted []string
	prev := dropStat
	dropStat = func(p string) (fs.FileInfo, error) {
		statted = append(statted, p)
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { dropStat = prev })

	bad := []string{`\\сервер\шара\Съёмка`, `//host/share`, `\\?\C:\x`, "относительная/папка"}
	if runtime.GOOS == "windows" {
		bad = append(bad, `C:\x\COM1`, `C:\Фото\NUL`, `C:\CON\Съёмка`, `D:\a\lpt9.`)
		// С расширением — как судит ОС (073-10): подмена «Windows 10» — устройство.
		t.Cleanup(paths.SetDeviceOracle(paths.Device))
		bad = append(bad, `D:\a\lpt9.txt`, `C:\Фото\Con.2024`)
	}
	a.dropPaths(bad)
	if len(statted) != 0 {
		t.Fatalf("Stat вызван до отказа: %v", statted)
	}
	// Все отказы — один тост со счётом (075-10).
	want := fmt.Sprintf("Не добавлено путей: %d", len(bad))
	if len(*notices) != 1 || !strings.Contains((*notices)[0], want) {
		t.Fatalf("тосты %v, ждали один с %q", *notices, want)
	}
	if v := a.editor().View(); len(v.Folders) != 0 {
		t.Errorf("папки добавились: %v", v.Folders)
	}

	// Пустой список — холостая правка: ни тостов, ни Stat.
	*notices = nil
	a.dropPaths(nil)
	if len(*notices) != 0 || len(statted) != 0 {
		t.Errorf("пустое перетаскивание: тосты %v, Stat %v", *notices, statted)
	}
}

// Хорошие папки добавляются, файл и пропавший путь молчат; среди нескольких брошенных
// плохой не мешает остальным.
func TestDropFolderAddsGood(t *testing.T) {
	a, notices := dropApp(t)
	captureLog(t)
	one, two := testkit.TempDir(t), testkit.TempDir(t)
	file := testkit.JPEG(t, one, "a.jpg", 20, 20)
	bad := `\\сервер\шара`
	a.dropPaths([]string{one, bad, file, filepath.Join(one, "нет"), two})
	waitScan(t, a)
	if len(*notices) != 1 || !strings.Contains((*notices)[0], "сервер") {
		t.Errorf("тосты: %v", *notices)
	}
	folders := a.editor().View().Folders
	if len(folders) != 2 {
		t.Fatalf("папок %d, ждали 2: %v", len(folders), folders)
	}
	// Повторное перетаскивание той же папки — не дубль.
	a.dropPaths([]string{one})
	waitScan(t, a)
	if got := len(a.editor().View().Folders); got != 2 {
		t.Errorf("после повторного броска папок %d", got)
	}
}

// Папка «Con.2024» на Windows 11 — обычная (073-10): добавляется без ложного тоста
// «зарезервировано», вместе с соседней.
func TestDropFolderDeviceExt(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	t.Cleanup(paths.SetDeviceOracle(func(string) bool { return false }))
	a, notices := dropApp(t)
	captureLog(t)
	root := testkit.TempDir(t)
	con, plain := filepath.Join(root, "Con.2024"), filepath.Join(root, "Отбор")
	for _, d := range []string{con, plain} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a.dropPaths([]string{con, plain})
	waitScan(t, a)
	if len(*notices) != 0 {
		t.Errorf("тосты: %v", *notices)
	}
	if folders := a.editor().View().Folders; len(folders) != 2 {
		t.Fatalf("папки %v, ждали обе", folders)
	}
}

// В log.txt отказ без пути (имя пользователя и название съёмки), в тосте одного пути — с путём.
func TestDropFolderLogNoPath(t *testing.T) {
	a, notices := dropApp(t)
	buf := captureLog(t)
	a.dropPaths([]string{`\\сервер\Иванов_свадьба\Съёмка`})
	a.dropPaths([]string{"Петров/относительный"})
	logged := buf.String()
	if !strings.Contains(logged, "перетаскивание:") {
		t.Fatalf("в логе нет записи: %q", logged)
	}
	for _, leak := range []string{"сервер", "Иванов", "Съёмка", "Петров", "относительный"} {
		if strings.Contains(logged, leak) {
			t.Errorf("в логе путь: %q", logged)
			break
		}
	}
	if len(*notices) != 2 || !strings.Contains((*notices)[0], "Иванов_свадьба") {
		t.Errorf("тост должен нести путь: %v", *notices)
	}
}

// Бросок 12 путей с сетевого диска — один тост со счётом и одна строка лога без путей (075-10);
// один путь — прежний текст с путём; хорошая папка среди плохих добавляется, счёт плохих — только их.
func TestDropManyNotLocal(t *testing.T) {
	a, notices := dropApp(t)
	buf := captureLog(t)
	bad := make([]string, 12)
	for i := range bad {
		bad[i] = fmt.Sprintf(`\\NAS\Иванов_свадьба\Съёмка%d`, i)
	}
	a.dropPaths(bad)
	if len(*notices) != 1 || !strings.Contains((*notices)[0], "Не добавлено путей: 12") ||
		!strings.Contains((*notices)[0], `C:\папка`) {
		t.Fatalf("тосты: %v", *notices)
	}
	if strings.Contains((*notices)[0], "NAS") {
		t.Errorf("тост со счётом несёт путь: %q", (*notices)[0])
	}
	logged := buf.String()
	if got := strings.Count(logged, "перетаскивание:"); got != 1 {
		t.Errorf("строк лога %d, ждали 1: %q", got, logged)
	}
	for _, leak := range []string{"NAS", "Иванов", "Съёмка"} {
		if strings.Contains(logged, leak) {
			t.Errorf("в логе путь: %q", logged)
			break
		}
	}

	// Один путь — прежний текст с путём; пустой бросок молчит.
	*notices = nil
	a.dropPaths([]string{bad[0]})
	if len(*notices) != 1 || !strings.Contains((*notices)[0], "NAS") || strings.Contains((*notices)[0], "Не добавлено путей") {
		t.Errorf("один путь: %v", *notices)
	}

	// Два плохих и одна хорошая: счёт — только плохих, хорошая добавлена.
	*notices = nil
	good := testkit.TempDir(t)
	a.dropPaths([]string{bad[0], good, bad[1]})
	waitScan(t, a)
	if len(*notices) != 1 || !strings.Contains((*notices)[0], "Не добавлено путей: 2") {
		t.Errorf("смесь: %v", *notices)
	}
	if got := len(a.editor().View().Folders); got != 1 {
		t.Errorf("папок %d, ждали 1", got)
	}
}

// Нечитаемые фото попадают в log.txt счётом по расширениям, без имён (073-7).
func TestScanLogNoNames(t *testing.T) {
	a, _ := dropApp(t)
	buf := captureLog(t)
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "хороший.jpg", 20, 20)
	garbage := []byte("не картинка")
	for _, n := range []string{"Иванов_1.jpg", "Иванов_2.JPG", "Иванов_3.jpg", "Петрова.jpeg"} {
		if err := os.WriteFile(filepath.Join(dir, n), garbage, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	waitScan(t, a)
	a.bg.Wait() // лог пишется в конце фоновой задачи
	logged := buf.String()
	if !strings.Contains(logged, "не читается: 3 файла .jpg, 1 файл .jpeg") {
		t.Errorf("сводка по расширениям не найдена: %q", logged)
	}
	for _, leak := range []string{"Иванов", "Петрова", "хороший"} {
		if strings.Contains(logged, leak) {
			t.Errorf("в логе имя %q: %q", leak, logged)
		}
	}
}

func TestSkippedSummary(t *testing.T) {
	if got := skippedSummary(nil); got != "" {
		t.Errorf("пусто: %q", got)
	}
	got := skippedSummary([]string{`C:\a\1.HEIC`, `C:\a\2.heic`, `C:\a\3.heic`, "/x/y.cr3", "/x/без", "/x/конец.", "/x/a.ф\nб", "/x/b.ф\nб"})
	for _, want := range []string{"3 файла .heic", "1 файл .cr3", "2 файла прочее", "2 файла без расширения"} {
		if !strings.Contains(got, want) {
			t.Errorf("нет %q в %q", want, got)
		}
	}
	if !strings.HasPrefix(got, "3 файла .heic") { // чаще — раньше
		t.Errorf("порядок: %q", got)
	}
	// Много видов: первые 10 и «и ещё N видов».
	var many []string
	for i := range 15 {
		many = append(many, "x/a."+string(rune('a'+i)))
	}
	got = skippedSummary(many)
	if !strings.HasSuffix(got, "и ещё 5 видов") || strings.Count(got, ", ") != maxSkippedKinds {
		t.Errorf("предел видов: %q", got)
	}
}
