package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// waitIdle ждёт конца чтения вместе с хвостом readFolders (лог, события): Scan.Active сбрасывается раньше.
func waitIdle(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.mu.Lock()
		busy := a.scanning
		a.mu.Unlock()
		if !busy {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("чтение не закончилось")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func skippedEvents(evs func(string) []evRec) []ExportsSkipped {
	var out []ExportsSkipped
	for _, e := range evs("exports:skipped") {
		out = append(out, e.data[0].(ExportsSkipped))
	}
	return out
}

// exports:skipped (121-4): первое чтение — событие со счётом и папкой первого файла; то же самое при
// следующем чтении — тишина; новый файл — событие со всем счётом; убрали все — тишина, вернулся — снова.
// В log.txt — счёт без путей. Файлы выгрузки в ленту не идут.
func TestExportsSkippedEvent(t *testing.T) {
	root := testkit.TempDir(t)
	album := filepath.Join(root, "ФА25")
	testkit.JPEG(t, album, "IMG_1.jpg", 40, 30)
	testkit.JPEG(t, album, "01.jpg", 40, 30) // фото фотографа с именем выгрузки
	ours := []string{
		testkit.ExportJPEG(t, album, "00.jpg", 40, 30),
		testkit.ExportJPEG(t, album, "export/01.jpg", 40, 30),
		testkit.ExportJPEG(t, album, "export/02.jpg", 40, 30),
	}
	buf := captureLog(t)
	a, evs := evApp(t)
	if _, _, err := a.editor().AddFolder(album); err != nil {
		t.Fatal(err)
	}
	read := func() {
		t.Helper()
		a.rescan()
		waitIdle(t, a)
	}
	steps := []struct {
		name   string
		before func()
		want   []ExportsSkipped // события всего, накопленные к концу шага
		line   string           // строка лога этого чтения; "" — нет
	}{
		{"первое чтение", func() {}, []ExportsSkipped{{3, "ФА25"}}, "  выгрузка программы: 3 файла — не в ленте"},
		{"те же файлы", func() {}, []ExportsSkipped{{3, "ФА25"}}, "  выгрузка программы: 3 файла — не в ленте"},
		{"новый файл в подпапке", func() { ours = append(ours, testkit.ExportJPEG(t, album, "export/03.jpg", 40, 30)) },
			[]ExportsSkipped{{3, "ФА25"}, {4, "export"}}, "  выгрузка программы: 4 файла — не в ленте"},
		{"все убраны", func() {
			for _, p := range ours {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			}
		}, []ExportsSkipped{{3, "ФА25"}, {4, "export"}}, ""},
		{"один вернулся", func() { testkit.ExportJPEG(t, album, "export/09.jpg", 40, 30) },
			[]ExportsSkipped{{3, "ФА25"}, {4, "export"}, {1, "export"}}, "  выгрузка программы: 1 файл — не в ленте"},
	}
	for _, s := range steps {
		s.before()
		mark := len(buf.String())
		read()
		if got := skippedEvents(evs); fmt.Sprint(got) != fmt.Sprint(s.want) {
			t.Errorf("%s: события %v, ждали %v", s.name, got, s.want)
		}
		logged := buf.String()[mark:]
		if has := strings.Contains(logged, "выгрузка программы"); has != (s.line != "") || s.line != "" && !strings.Contains(logged, s.line+"\n") {
			t.Errorf("%s: в логе %q, ждали строку %q", s.name, logged, s.line)
		}
		if strings.Contains(logged, root) || strings.Contains(logged, "ФА25") {
			t.Errorf("%s: в логе путь: %q", s.name, logged)
		}
		if got := photoNames(a); fmt.Sprint(got) != "[01.jpg IMG_1.jpg]" {
			t.Errorf("%s: лента %v", s.name, got)
		}
	}
}

// Устаревшее поколение чтения событие не шлёт и память не меняет; без файлов выгрузки — ни события, ни строки.
func TestPublishOursStaleAndEmpty(t *testing.T) {
	a, evs := evApp(t)
	buf := captureLog(t)
	a.publishOurs(a.scanGen, nil, nil)
	a.publishOurs(a.scanGen+1, []string{filepath.Join("x", "ФА25", "01.jpg")}, nil) // поколение не текущее
	if len(evs("exports:skipped")) != 0 || len(a.oursSeen) != 0 || strings.Contains(buf.String(), "ФА25") {
		t.Errorf("события %v, память %v, лог %q", evs("exports:skipped"), a.oursSeen, buf.String())
	}
	a.publishOurs(a.scanGen, []string{filepath.Join("x", "ФА25", "01.jpg")}, nil)
	if got := skippedEvents(evs); fmt.Sprint(got) != "[{1 ФА25}]" {
		t.Errorf("текущее поколение: %v", got)
	}
}

// Том пропал на время чтения: ключи под ним остаются в памяти, вернулся — тоста нет; файл, который
// появился за это время, и файлы живых папок ведут себя как обычно. Папок несколько.
func TestExportsSkippedVolumeBack(t *testing.T) {
	root := testkit.TempDir(t)
	albums := []string{filepath.Join(root, "ФА25"), filepath.Join(root, "ФА26"), filepath.Join(root, "ФА27")}
	for _, d := range albums {
		testkit.JPEG(t, d, "IMG_1.jpg", 40, 30)
		testkit.ExportJPEG(t, d, "01.jpg", 40, 30)
	}
	a, evs := evApp(t)
	for _, d := range albums {
		if _, _, err := a.editor().AddFolder(d); err != nil {
			t.Fatal(err)
		}
	}
	read := func() {
		t.Helper()
		a.rescan()
		waitIdle(t, a)
	}
	read()
	if got := len(skippedEvents(evs)); got != 1 {
		t.Fatalf("первое чтение: событий %d, ждали 1", got)
	}
	// ФА25 и ФА26 пропали (том), ФА27 читается: событие не нужно.
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(p string) bool { return !strings.Contains(p, "ФА25") && !strings.Contains(p, "ФА26") }
	read()
	if got := len(skippedEvents(evs)); got != 1 {
		t.Fatalf("том пропал: событий %d, ждали 1", got)
	}
	reachable = old
	read()
	if got := skippedEvents(evs); len(got) != 1 {
		t.Fatalf("том вернулся: события %v, ждали одно (первое)", got)
	}
}
