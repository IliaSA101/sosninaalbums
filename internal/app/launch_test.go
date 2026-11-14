package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// saveAlbumN пишет в path альбом из n разворотов (по числу разворотов альбомы различимы в виде).
func saveAlbumN(t testing.TB, path string, n int) {
	t.Helper()
	p := model.New()
	for range n {
		p.Spreads = append(p.Spreads, &model.Spread{Name: "р"})
	}
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
}

// extEvents подменяет a.emit и копит имена из событий open:external.
func extEvents(a *App) (names *[]string, mu *sync.Mutex) {
	names, mu = new([]string), new(sync.Mutex)
	a.emit = func(name string, data ...any) {
		if name != "open:external" {
			return
		}
		mu.Lock()
		*names = append(*names, data[0].(string))
		mu.Unlock()
	}
	return names, mu
}

func TestLaunchArgs(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(dir, "Свадьба"+projectExt)
	upper := filepath.Join(dir, "ВЕСНА.SALBUM")
	long := filepath.Join(dir, strings.Repeat("д", paths.MaxPathLen)+projectExt)
	cases := []struct {
		name string
		args []string
		cwd  string
		file string
		pid  int
	}{
		{"нет аргументов", nil, dir, "", 0},
		{"пустой список", []string{}, "", "", 0},
		{"абсолютный путь", []string{abs}, "", abs, 0},
		{"расширение в верхнем регистре", []string{upper}, "", upper, 0},
		{"относительный от cwd", []string{"Свадьба" + projectExt}, dir, abs, 0},
		{"относительный без cwd — отказ", []string{"Свадьба" + projectExt}, "", "", 0},
		{"путь с .. чистится", []string{filepath.Join(dir, "x", "..", "Свадьба"+projectExt)}, "", abs, 0},
		{"не альбом", []string{filepath.Join(dir, "a.jpg")}, "", "", 0},
		{"слишком длинный путь", []string{long}, "", "", 0},
		{"пустая строка и мусор", []string{"", "-x", "--"}, dir, "", 0},
		{"флаг и значение раздельно", []string{afterPIDFlag, "4242", abs}, "", abs, 4242},
		{"флаг через =", []string{afterPIDFlag + "=77", abs}, "", abs, 77},
		{"флаг после файла", []string{abs, afterPIDFlag, "9"}, "", abs, 9},
		{"только флаг", []string{afterPIDFlag, "5"}, "", "", 5},
		{"флаг без значения", []string{abs, afterPIDFlag}, "", abs, 0},
		{"значение не число", []string{afterPIDFlag, "abc", abs}, "", abs, 0},
		{"нулевой pid", []string{afterPIDFlag + "=0", abs}, "", abs, 0},
		{"отрицательный pid", []string{afterPIDFlag, "-3", abs}, "", abs, 0},
		{"pid-значение не съедает альбом", []string{afterPIDFlag, abs}, "", abs, 0},
		{"флаги wails dev мимо", []string{"-assetdir", "x", "-loglevel", "debug", abs}, "", abs, 0},
		{"два альбома — первый", []string{abs, upper}, "", abs, 0},
		{"первый отклонён — второй не ищем", []string{"a" + projectExt, upper}, "", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file, pid := launchArgs(c.args, c.cwd)
			if file != c.file || pid != c.pid {
				t.Fatalf("launchArgs(%q, %q) = %q, %d; ждали %q, %d", c.args, c.cwd, file, pid, c.file, c.pid)
			}
		})
	}
}

// Сетевой/двойной-слэш путь не открывается: граница безопасности (paths), не только расширение.
func TestLaunchArgsRejectsNonLocal(t *testing.T) {
	for _, arg := range []string{"//srv/share/a" + projectExt, `\\srv\share\a` + projectExt, `\\?\C:\a` + projectExt} {
		if file, _ := launchArgs([]string{arg}, ""); file != "" {
			t.Errorf("%q принят: %q", arg, file)
		}
	}
}

// Имя флага — контракт между версиями: 0.25 запускает 0.26+ со своим флагом, и наоборот.
func TestAfterPidContract(t *testing.T) {
	if afterPIDFlag != "--after-pid" {
		t.Fatalf("имя флага сменилось: %q", afterPIDFlag)
	}
	f := filepath.Join(testkit.TempDir(t), "a"+projectExt)
	// Ровно так «Открыть в 0.X» собирает аргументы: флаг, pid, файл.
	if file, pid := launchArgs([]string{afterPIDFlag, "31337", f}, ""); file != f || pid != 31337 {
		t.Fatalf("разбор своих аргументов: %q %d", file, pid)
	}
}

func TestStartupArgFile(t *testing.T) {
	t.Run("аргумент важнее последнего и становится последним", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		dir := testkit.TempDir(t)
		arg, last := filepath.Join(dir, "arg"+projectExt), filepath.Join(dir, "last"+projectExt)
		saveAlbumN(t, arg, 3)
		saveAlbumN(t, last, 1)
		a.argFile = arg
		p, path, notice := a.loadStartup(last)
		if path != arg || len(p.Spreads) != 3 || notice != "" {
			t.Fatalf("путь %q, разворотов %d, заметка %q", path, len(p.Spreads), notice)
		}
		cfg := a.loadConfig()
		if cfg.LastProject != arg || len(cfg.Recent) == 0 || cfg.Recent[0] != arg {
			t.Fatalf("LastProject %q, Recent %v", cfg.LastProject, cfg.Recent)
		}
		// Аргумент отдаётся один раз: следующий старт идёт как обычно.
		if _, path, _ := a.loadStartup(last); path != last {
			t.Fatalf("повторно путь %q", path)
		}
	})
	t.Run("аргумент и есть последний — открывается один раз", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		arg := filepath.Join(testkit.TempDir(t), "a"+projectExt)
		saveAlbumN(t, arg, 2)
		a.argFile = arg
		if p, path, notice := a.loadStartup(arg); path != arg || len(p.Spreads) != 2 || notice != "" {
			t.Fatalf("путь %q, разворотов %d, заметка %q", path, len(p.Spreads), notice)
		}
	})
	t.Run("файла нет — заметка и последний", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		dir := testkit.TempDir(t)
		last := filepath.Join(dir, "last"+projectExt)
		saveAlbumN(t, last, 1)
		missing := filepath.Join(dir, "нет"+projectExt)
		a.argFile = missing
		_, path, notice := a.loadStartup(last)
		if path != last || notice != "Альбом не найден: нет"+projectExt+"." {
			t.Fatalf("путь %q, заметка %q", path, notice)
		}
		if a.loadConfig().LastProject == missing {
			t.Fatal("несуществующий файл стал последним")
		}
	})
	t.Run("сломанный файл не затёрт, открыт последний", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		dir := testkit.TempDir(t)
		last, bad := filepath.Join(dir, "last"+projectExt), filepath.Join(dir, "bad"+projectExt)
		saveAlbumN(t, last, 1)
		os.WriteFile(bad, []byte("{не json"), 0o644)
		a.argFile = bad
		_, path, notice := a.loadStartup(last)
		if path != last || !strings.Contains(notice, "Не открылся проект") {
			t.Fatalf("путь %q, заметка %q", path, notice)
		}
		if data, _ := os.ReadFile(bad); string(data) != "{не json" {
			t.Fatal("чужой файл изменён")
		}
	})
	t.Run("без аргумента и без последнего — пусто", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		if p, path, notice := a.loadStartup(""); p == nil || path != "" || notice != "" {
			t.Fatalf("проект %v, путь %q, заметка %q", p, path, notice)
		}
	})
	t.Run("копия из папки копий уходит в очередь", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		root := testkit.TempDir(t)
		a.backups = backup.New(root)
		last := filepath.Join(testkit.TempDir(t), "last"+projectExt)
		saveAlbumN(t, last, 1)
		cp := filepath.Join(root, "k", "копия"+projectExt)
		a.argFile = cp
		_, path, _ := a.loadStartup(last)
		if path != last || a.ext != cp {
			t.Fatalf("путь %q, очередь %q", path, a.ext)
		}
	})
}

// State.FromFile: true, только если открылся именно файл из аргумента (0.25-q112-start).
func TestStartupFromFile(t *testing.T) {
	open := func(a *App, arg, last string) bool {
		a.argFile = arg
		_, path, _ := a.loadStartup(last)
		return openedArg(arg, path)
	}
	t.Run("файл из аргумента открылся", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		dir := testkit.TempDir(t)
		arg, last := filepath.Join(dir, "arg"+projectExt), filepath.Join(dir, "last"+projectExt)
		saveAlbumN(t, arg, 2)
		saveAlbumN(t, last, 1)
		if !open(a, arg, last) {
			t.Fatal("fromFile = false")
		}
	})
	t.Run("аргумент и есть последний", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		arg := filepath.Join(testkit.TempDir(t), "a"+projectExt)
		saveAlbumN(t, arg, 2)
		if !open(a, arg, arg) {
			t.Fatal("fromFile = false")
		}
	})
	t.Run("аргумента нет", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		last := filepath.Join(testkit.TempDir(t), "last"+projectExt)
		saveAlbumN(t, last, 1)
		if open(a, "", last) || open(a, "", "") {
			t.Fatal("fromFile = true без аргумента")
		}
	})
	t.Run("файла из аргумента нет", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		dir := testkit.TempDir(t)
		last := filepath.Join(dir, "last"+projectExt)
		saveAlbumN(t, last, 1)
		if open(a, filepath.Join(dir, "нет"+projectExt), last) || open(a, filepath.Join(dir, "нет"+projectExt), "") {
			t.Fatal("fromFile = true, хотя открылся прежний или черновик")
		}
	})
	t.Run("сломанный файл", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		bad := filepath.Join(testkit.TempDir(t), "bad"+projectExt)
		os.WriteFile(bad, []byte("{не json"), 0o644)
		if open(a, bad, "") {
			t.Fatal("fromFile = true для сломанного файла")
		}
	})
	t.Run("GetState отдаёт один раз", func(t *testing.T) {
		a := newApp(t)
		a.cfgDir = testkit.TempDir(t)
		a.ed.Store(mustEditor(t, model.New(), ""))
		a.mu.Lock()
		a.startFromFile = true
		a.mu.Unlock()
		if !a.GetState().FromFile {
			t.Fatal("первый GetState: FromFile = false")
		}
		if a.GetState().FromFile {
			t.Fatal("повторный GetState: FromFile = true, закрыл бы окно после открытия другого альбома")
		}
	})
}

// takeArg и Mac: файл, пришедший до startup, — аргумент запуска, после — очередь.
func TestMacOpen(t *testing.T) {
	a := newApp(t)
	names, mu := extEvents(a)
	dir := testkit.TempDir(t)
	first, second := filepath.Join(dir, "a"+projectExt), filepath.Join(dir, "b"+projectExt)

	a.macOpen(first)
	a.macOpen(filepath.Join(dir, "x.txt")) // не альбом
	a.macOpen("относительный" + projectExt)
	if a.argFile != first || a.ext != "" || len(*names) != 0 {
		t.Fatalf("до startup: файл %q, очередь %q, события %v", a.argFile, a.ext, *names)
	}
	if got := a.takeArg(); got != first {
		t.Fatalf("takeArg: %q", got)
	}
	a.macOpen(second)
	mu.Lock()
	defer mu.Unlock()
	if a.ext != second || len(*names) != 1 || (*names)[0] != "b"+projectExt {
		t.Fatalf("после startup: очередь %q, события %v", a.ext, *names)
	}
}

func TestSecondLaunchOpen(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	names, mu := extEvents(a)
	dir := testkit.TempDir(t)
	one, two := filepath.Join(dir, "one"+projectExt), filepath.Join(dir, "two"+projectExt)
	saveAlbumN(t, one, 2)
	saveAlbumN(t, two, 4)
	cur := filepath.Join(dir, "cur"+projectExt)
	saveAlbumN(t, cur, 1)
	if _, err := a.openPath(cur); err != nil {
		t.Fatal(err)
	}
	pending := func() string { return a.GetState().Pending }

	// Без альбома среди аргументов очереди нет.
	a.secondLaunch(options.SecondInstanceData{Args: []string{"-x", "a.jpg", afterPIDFlag, "5"}, WorkingDirectory: dir})
	a.secondLaunch(options.SecondInstanceData{})
	if pending() != "" || len(*names) != 0 {
		t.Fatalf("лишнее событие: %v, в очереди %q", *names, pending())
	}

	// Два запуска подряд: последний побеждает, события — по одному на запуск.
	a.secondLaunch(options.SecondInstanceData{Args: []string{one}})
	a.secondLaunch(options.SecondInstanceData{Args: []string{"two" + projectExt}, WorkingDirectory: dir}) // относительный
	mu.Lock()
	got := strings.Join(*names, ",")
	mu.Unlock()
	if got != "one.salbum,two.salbum" || pending() != "two"+projectExt {
		t.Fatalf("события %q, в очереди %q", got, pending())
	}

	st, err := a.OpenExternal()
	if err != nil || st.Cancelled || a.editor().Path() != two || len(st.View.Spreads) != 4 {
		t.Fatalf("OpenExternal: %v, cancelled %v, путь %q, разворотов %d", err, st.Cancelled, a.editor().Path(), len(st.View.Spreads))
	}
	if st.Pending != "" || a.loadConfig().LastProject != two {
		t.Fatalf("очередь %q, последний %q", st.Pending, a.loadConfig().LastProject)
	}
	// Очередь пуста — отмена, альбом прежний.
	ed := a.editor()
	if st, err := a.OpenExternal(); err != nil || !st.Cancelled || a.editor() != ed {
		t.Fatalf("пустая очередь: %v, cancelled %v", err, st.Cancelled)
	}
	// Этот же альбом уже открыт — холостое открытие: окно вперёд, редактор тот же.
	a.secondLaunch(options.SecondInstanceData{Args: []string{two}})
	if st, err := a.OpenExternal(); err != nil || !st.Cancelled || a.editor() != ed {
		t.Fatalf("тот же путь: %v, cancelled %v", err, st.Cancelled)
	}
}

// Путь в очереди проверяется ещё раз при открытии: подмена не .salbum — ошибка, прежний альбом цел.
func TestOpenExternalRejects(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	cur := filepath.Join(dir, "cur"+projectExt)
	saveAlbumN(t, cur, 2)
	if _, err := a.openPath(cur); err != nil {
		t.Fatal(err)
	}
	ed := a.editor()
	for _, bad := range []string{filepath.Join(dir, "a.jpg"), "относительный" + projectExt, "//srv/x" + projectExt} {
		a.mu.Lock()
		a.ext = bad
		a.mu.Unlock()
		if _, err := a.OpenExternal(); err == nil || a.editor() != ed {
			t.Errorf("%q открыт: %v", bad, err)
		}
		if a.GetState().Pending != "" {
			t.Errorf("%q остался в очереди", bad)
		}
	}
	// Файла нет — ошибка, прежний альбом остаётся.
	a.queueExternal(filepath.Join(dir, "нет"+projectExt))
	if _, err := a.OpenExternal(); err == nil || a.editor() != ed {
		t.Errorf("несуществующий открыт: %v", err)
	}
}

// Параллельные второй запуск, GetState и OpenExternal (под -race): очередь под a.mu.
func TestExternalQueueRace(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	var files []string
	for i, n := range []int{1, 2, 3} {
		f := filepath.Join(dir, string(rune('a'+i))+projectExt)
		saveAlbumN(t, f, n)
		files = append(files, f)
	}
	var wg sync.WaitGroup
	for _, f := range files {
		wg.Go(func() { a.secondLaunch(options.SecondInstanceData{Args: []string{f}}) })
		wg.Go(func() { a.GetState() })
	}
	wg.Wait()
	if _, err := a.OpenExternal(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, a.editor().Path()) {
		t.Fatalf("открыт %q", a.editor().Path())
	}
}
