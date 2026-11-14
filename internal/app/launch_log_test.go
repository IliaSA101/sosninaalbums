package app

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/wailsapp/wails/v2/pkg/options"
)

var driveRe = regexp.MustCompile(`[A-Za-z]:`)

// Строки про альбом из командной строки (спека 118): все исходы, ни пути, ни имени файла.
func TestLaunchAlbumLogNoPath(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	secret := "Секрет-Иванов"
	one, two := filepath.Join(dir, secret+"1"+projectExt), filepath.Join(dir, secret+"2"+projectExt)
	saveAlbumN(t, one, 2)
	saveAlbumN(t, two, 3)
	cur := filepath.Join(dir, "cur"+projectExt)
	saveAlbumN(t, cur, 1)
	if _, err := a.openPath(cur); err != nil {
		t.Fatal(err)
	}
	buf := captureLog(t)
	flags := log.Flags()
	log.SetFlags(0) // в отметке времени есть «/»
	t.Cleanup(func() { log.SetFlags(flags) })
	seen := 0
	step := func() []string { // строки, появившиеся с прошлого вызова
		all := buf.String()
		s := strings.TrimSpace(all[seen:])
		seen = len(all)
		var out []string
		for _, l := range strings.Split(s, "\n") { // прочие строки (сканер, память) — не предмет теста
			if strings.HasPrefix(l, "альбом из командной строки") {
				out = append(out, l)
			}
		}
		return out
	}
	want := func(name string, got []string, lines ...string) {
		t.Helper()
		if strings.Join(got, "|") != strings.Join(lines, "|") {
			t.Fatalf("%s: строки %q, ждали %q", name, got, lines)
		}
	}
	var all []string

	// Старт: нет аргумента — тишина; альбом открылся; не открылся; ушёл в очередь (папка копий).
	a.logLaunchArg("", "")
	want("нет аргумента", step())
	a.logLaunchArg(one, one)
	all = append(all, step()...)
	a.logLaunchArg(two, cur)
	all = append(all, step()...)
	a.ext = two
	a.logLaunchArg(two, "")
	all = append(all, step()...)
	a.ext = ""
	want("старт", all, logArgOpened, logArgFailed, logArgQueued)

	// Второй запуск: другой альбом — в очереди, потом открыт; тот же — уже открыт; нет файла — не открылся.
	all = nil
	a.secondLaunch(options.SecondInstanceData{Args: []string{one}})
	all = append(all, step()...)
	if _, err := a.OpenExternal(); err != nil {
		t.Fatal(err)
	}
	all = append(all, step()...)
	a.secondLaunch(options.SecondInstanceData{Args: []string{one}})
	all = append(all, step()...)
	if st, err := a.OpenExternal(); err != nil || !st.Cancelled { // тот же альбом: холостое открытие
		t.Fatalf("тот же альбом: %v, cancelled %v", err, st.Cancelled)
	}
	all = append(all, step()...) // строки у холостого открытия нет
	a.queueExternal(filepath.Join(dir, secret+"нет"+projectExt))
	step()
	if _, err := a.OpenExternal(); err == nil {
		t.Fatal("несуществующий альбом открылся")
	}
	all = append(all, step()...)
	want("второй запуск", all, logArgQueued, logArgOpened, logArgAlready, logArgFailed)

	for _, l := range append(all, logArgOpened, logArgQueued, logArgAlready, logArgFailed) {
		if strings.ContainsAny(l, `\/`) || driveRe.MatchString(l) || strings.Contains(l, secret) ||
			strings.Contains(l, filepath.Base(dir)) || strings.Contains(l, projectExt) {
			t.Errorf("в строке путь или имя файла: %q", l)
		}
	}
}
