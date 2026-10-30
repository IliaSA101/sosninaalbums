package app

// Правила мест съёмок (150, design §4.2): строковые таблицы на любой ОС и дисковая проверка при
// пополнении. App-сторона (источники, засев, окно) — agentplaces_app_test.go.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func unixEnv(goos string) *placeEnv {
	e := &placeEnv{rules: unixRules(goos, diskOK), system: "C:"}
	e.cloud = cloudRoots(goos, e.rules.homes[0], func(string) string { return "" })
	return e
}

func winEnv() *placeEnv {
	env := map[string]string{"OneDrive": `C:\Users\u\OneDrive`, "OneDriveCommercial": `E:\Облако`}
	return &placeEnv{rules: winRules(diskOK), system: "C:", cloud: cloudRoots("windows", `C:\Users\u`, func(k string) string { return env[k] })}
}

// Место из родителя (строгие правила): путь из пользовательских и широких папок — не место.
// Таблицы Windows идут через правила Windows на любой ОС (design §11, «таблица Windows-путей на Linux CI»).
func TestPlaceNotFromUserDirs(t *testing.T) {
	type c struct {
		p   string
		bad bool
	}
	groups := []struct {
		name  string
		env   *placeEnv
		cases []c
	}{
		{"linux", unixEnv("linux"), []c{
			{"/home/u/Pictures", true}, {"/home/u/Работа", true}, {"/home/u/Dropbox", true}, // прямые подпапки домашней
			{"/home/u", true}, {"/home", true}, {"/", true},
			{"/home/u/Документы", true}, {"/home/u/Документы/Работа", true}, {"/home/u/Документы/Работа/2026", true},
			{"/home/u/Загрузки/x", true}, {"/home/u/Рабочий стол/Съёмка", true},
			{"/media/u/DISK", true}, {"/media/u", true}, {"/media", true}, // том, все тома разом, каталог
			{"/run/media/u", true}, {"/run/media/u/DISK", true}, {"/mnt/disk", true}, {"/mnt", true},
			{"/etc/x", true}, {"/home/u/.ssh", true}, {"/home/u/Pictures/../.ssh", true},
			{"/home/u/Pictures/Свадьба", false}, {"/home/u/Pictures/Свадьба/2026", false},
			{"/media/u/DISK/Фото", false}, {"/run/media/u/DISK/Фото/2026", false}, {"/mnt/disk/Photos", false},
			{"/data/Фото", false}, {"/home/uu", false}, {"/home/u/Работа/Свадьбы", false},
			{"относительный/путь", true}, {"", true}, {"/home/u/Pictures/Свадьба\x00", true},
			{"/" + strings.Repeat("я", 5000), true},
		}},
		{"darwin", unixEnv("darwin"), []c{
			{"/Users/u/Pictures", true}, {"/Users/u/Документы/Работа", true}, {"/Volumes/X", true}, {"/Volumes", true},
			{"/Users/u/Library/Mobile Documents/com~apple~CloudDocs", true},
			{"/Users/u/Library/Mobile Documents/com~apple~CloudDocs/Фото", true},
			{"/Users/u/Library/CloudStorage/OneDrive-X", true}, {"/Users/u/Library/CloudStorage/OneDrive-X/Фото", true},
			{"/Volumes/X/Фото", false}, {"/Users/u/Pictures/Свадьба", false},
		}},
		{"windows", winEnv(), []c{
			{`C:\Users\u\Pictures`, true}, {`C:\Users\u\Работа`, true}, {`C:\Users\u`, true}, {`C:\Users`, true},
			{`C:\Users\u\Documents`, true}, {`C:\Users\u\Documents\Работа`, true}, {`C:\Users\u\Desktop\Съёмка`, true},
			{`C:\Users\u\OneDrive`, true}, {`C:\Users\u\OneDrive\Фото`, true}, // корень OneDrive и его подпапка
			{`E:\Облако`, true}, {`E:\Облако\Фото`, true}, {`e:\облако\фото`, true}, // не-домашнее облако, регистр
			{`C:\`, true}, {`D:\`, true}, {`C:\Windows\x`, true}, {`C:\Users\u\AppData\Local\x`, true},
			{`\\сервер\ресурс\Фото`, true}, {`\\?\C:\Фото`, true}, {`D:`, true}, {`Фото\2026`, true},
			{`D:\Фото`, false}, {`d:\фото`, false}, {`E:\Облако\Фото\2026`, false}, {`C:\Users\u\Pictures\Свадьба`, false},
			{`C:\Users\u\OneDrive\Фото\2026`, false}, {`D:\Съёмки\2026`, false},
		}},
	}
	for _, g := range groups {
		for _, c := range g.cases {
			err := g.env.static(c.p, prefs.PlaceParent)
			if (err != nil) != c.bad {
				t.Errorf("%s %q: err = %v, плохое = %v", g.name, c.p, err, c.bad)
			}
		}
	}
	// Способ не из двух — не место; пустое окружение (правил нет) — fail-closed.
	if unixEnv("linux").static("/data/Фото", "agent") == nil || (*placeEnv)(nil).static("/data/Фото", prefs.PlaceParent) == nil {
		t.Error("чужой способ и отсутствие правил должны отказывать")
	}
}

// Явное место («Папка со съёмками…»): мягче строгих — корень несистемного тома, Pictures, Videos
// можно; системный том, домашняя, «Документы / Рабочий стол / Загрузки» целиком и /media/<u> — нет.
func TestPlaceExplicit(t *testing.T) {
	type c struct {
		p   string
		bad bool
	}
	for name, g := range map[string]struct {
		env   *placeEnv
		cases []c
	}{
		"linux": {unixEnv("linux"), []c{
			{"/home/u/Pictures", false}, {"/home/u/Videos", false}, {"/home/u/Работа", false},
			{"/home/u/Документы/Свадьба", false}, {"/media/u/DISK", false}, {"/mnt/disk", false}, {"/data/Фото", false},
			{"/", true}, {"/home/u", true}, {"/home/u/Документы", true}, {"/home/u/Загрузки", true},
			{"/media/u", true}, {"/run/media/u", true}, {"/media", true}, {"/etc", true}, {"/home/u/.ssh", true},
		}},
		"windows": {winEnv(), []c{
			{`D:\`, false}, {`d:\`, false}, {`E:\`, false}, {`D:\Фото`, false}, {`C:\Users\u\Pictures`, false},
			{`C:\Users\u\Videos`, false}, {`C:\Users\u\Documents\Свадьба`, false},
			{`C:\`, true}, {`c:\`, true}, {`C:\Users\u`, true}, {`C:\Users\u\Documents`, true}, {`C:\Users\u\Desktop`, true},
			{`C:\Windows`, true}, {`\\сервер\ресурс`, true}, {`D:`, true},
		}},
	} {
		for _, c := range g.cases {
			err := g.env.static(c.p, prefs.PlaceChosen)
			if (err != nil) != c.bad {
				t.Errorf("%s %q: err = %v, плохое = %v", name, c.p, err, c.bad)
			}
		}
	}
}

func TestUnixMountShape(t *testing.T) {
	for _, c := range []struct {
		p            string
		userRoot, vo bool
	}{
		{"/media/u", true, false}, {"/media/u/DISK", false, true}, {"/media/u/DISK/x", false, false},
		{"/run/media/u", true, false}, {"/run/media/u/D", false, true}, {"/run/media/u/D/x", false, false},
		{"/Volumes/X", false, true}, {"/Volumes/X/y", false, false}, {"/mnt/d", false, true}, {"/mnt/d/y", false, false},
		{"/media", false, false}, {"/data/x", false, false}, {"/run/user/1", false, false}, {"/", false, false},
	} {
		if u, v := unixMountShape(c.p); u != c.userRoot || v != c.vo {
			t.Errorf("%s: (%v, %v), ждали (%v, %v)", c.p, u, v, c.userRoot, c.vo)
		}
	}
}

// fakeDisk подменяет st_dev и таблицу монтирования: устройство — по длиннейшей точке. Корень "/" —
// общий для всех путей (на Windows тома нет, как и у devOf); точки — пути ОС.
func fakeDisk(t *testing.T, points ...string) (devCalls *atomic.Int32) {
	t.Helper()
	oldDev, oldMounts := placeDev, placeMounts
	t.Cleanup(func() { placeDev, placeMounts = oldDev, oldMounts })
	devCalls = new(atomic.Int32)
	placeMounts = func() ([]string, error) { return append([]string{"/"}, points...), nil }
	placeDev = func(p string) (uint64, bool) {
		devCalls.Add(1)
		best, id := -1, uint64(0)
		for i, m := range append([]string{"/"}, points...) {
			sep := string(filepath.Separator)
			if (m == "/" || p == m || strings.HasPrefix(p, strings.TrimSuffix(m, sep)+sep)) && len(m) > best {
				best, id = len(m), uint64(i)
			}
		}
		return id, true
	}
	return devCalls
}

// realEnv — правила без запретов (тестовые папки лежат под TMPDIR) поверх настоящего диска.
func realEnv() *placeEnv {
	return &placeEnv{rules: &folderRules{windows: runtime.GOOS == "windows", fold: unixRules("linux", diskOK).fold, timeout: time.Second, disk: diskProbe}, system: "C:"}
}

func realDir(t *testing.T, elems ...string) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(append([]string{base}, elems...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Дисковые правила при пополнении: корень тома (st_dev), предок точки монтирования (mountinfo), ссылка
// в пути, несуществующая папка и файл — не место из родителя; явному месту корень тома можно.
func TestPlaceDiskRules(t *testing.T) {
	base := realDir(t, "data", "Фото")
	vol := filepath.Join(filepath.Dir(base), "vol") // отдельный том
	nas := filepath.Join(filepath.Dir(base), "nas") // точка монтирования внутри «data»
	for _, d := range []string{vol, nas, filepath.Join(vol, "Съёмка")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	data := filepath.Dir(base)
	fakeDisk(t, vol, filepath.Join(data, "nas"))
	e := realEnv()

	for _, c := range []struct {
		name, dir, how string
		ok             bool
	}{
		{"обычная папка", base, prefs.PlaceParent, true},
		{"корень тома (st_dev)", vol, prefs.PlaceParent, false},
		{"в томе", filepath.Join(vol, "Съёмка"), prefs.PlaceParent, true},
		{"предок точки монтирования", filepath.Dir(data), prefs.PlaceParent, false},
		{"сама точка монтирования", nas, prefs.PlaceParent, false},
		{"нет папки", filepath.Join(base, "нет"), prefs.PlaceParent, false},
		{"явное: корень тома", vol, prefs.PlaceChosen, true},
		{"явное: предок точки монтирования", filepath.Dir(data), prefs.PlaceChosen, true},
	} {
		got, err := e.admit(c.dir, c.how)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, годно = %v", c.name, err, c.ok)
		}
		if err == nil && got != c.dir {
			t.Errorf("%s: настоящий путь %q, ждали %q", c.name, got, c.dir)
		}
	}
	// Файл вместо папки.
	file := filepath.Join(base, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.admit(file, prefs.PlaceParent); err == nil {
		t.Error("файл принят как место")
	}
	// Таблица монтирования не читается — нельзя (а не «монтирований нет»).
	placeMounts = func() ([]string, error) { return nil, os.ErrPermission }
	if _, err := e.admit(base, prefs.PlaceParent); err == nil {
		t.Error("ошибка mountinfo: ждали отказ")
	}
}

// Ссылка: место хранится настоящим путём; ссылка в середине или в конце пути, ведущая в широкое, — отказ.
func TestPlaceSymlinks(t *testing.T) {
	real1 := realDir(t, "Фото")
	link := filepath.Join(filepath.Dir(real1), "ссылка")
	if err := os.Symlink(real1, link); err != nil {
		t.Skip(err)
	}
	fakeDisk(t)
	e := realEnv()
	got, err := e.admit(link, prefs.PlaceParent)
	if err != nil || got != real1 {
		t.Errorf("ссылка на обычную папку: %q, %v; ждали настоящий путь %q", got, err, real1)
	}
	// Ссылка, ведущая в запрещённое правилом 149: настоящий путь проверяется ещё раз.
	e.rules.trees = []string{real1}
	if _, err := e.admit(link, prefs.PlaceParent); err == nil {
		t.Error("ссылка в системное дерево принята")
	}
	if !hasLinkPart(link) || hasLinkPart(real1) {
		t.Errorf("hasLinkPart: ссылка %v, обычная %v", hasLinkPart(link), hasLinkPart(real1))
	}
	if !hasLinkPart(filepath.Join(link, "нет")) {
		t.Error("нечитаемое звено должно считаться ссылкой")
	}
}

// Чтение мест диск не трогает: подменённый stat не вызывается; пополнение зовёт его под within, а
// зависший диск не вешает пополнение дольше предела.
func TestPlaceDiskOnPushOnly(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	dir := realDir(t, "Съёмки")
	devCalls := fakeDisk(t)
	e := realEnv()
	a.askM().rules = func() *folderRules { return e.rules }

	prefs.AddPlace(a.cfgDir, prefs.Place{Dir: dir, How: prefs.PlaceChosen})
	if n := devCalls.Load(); n != 0 {
		t.Fatalf("AddPlace (хранение) обратился к диску %d раз", n)
	}
	for range 3 {
		if got := a.usablePlaces(); len(got) != 1 || got[0].Dir != dir {
			t.Fatalf("usablePlaces = %v", got)
		}
	}
	if n := devCalls.Load(); n != 0 {
		t.Fatalf("чтение мест обратилось к диску %d раз", n)
	}

	a.pushPlaces(prefs.PlaceParent, filepath.Join(dir, "Свадьба"))
	if devCalls.Load() != 0 {
		t.Error("несуществующая папка не должна доходить до st_dev")
	}
	sub := realDir(t, "Съёмки", "Свадьба")
	a.pushPlaces(prefs.PlaceParent, sub)
	if devCalls.Load() == 0 {
		t.Error("пополнение не проверило устройство")
	}

	// Зависший st_dev: пополнение уходит по пределу, места не меняются.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	placeDev = func(string) (uint64, bool) { <-release; return 0, true }
	e.rules.timeout = 50 * time.Millisecond
	before := prefs.LoadAgentPerms(a.cfgDir).Places
	start := time.Now()
	a.pushPlaces(prefs.PlaceParent, realDir(t, "Другая", "Свадьба"))
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Errorf("зависший диск держал пополнение %v", d)
	}
	if got := prefs.LoadAgentPerms(a.cfgDir).Places; len(got) != len(before) {
		t.Errorf("зависшая проба изменила места: %v → %v", before, got)
	}
}

// Места, ставшие негодными после смены правил, при чтении пропускаются (без диска).
func TestPlaceUsableDropsBad(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	// prefs хранит только пути своей ОС (filepath.IsAbs): на Windows те же случаи — Windows-правилами.
	e := unixEnv("linux")
	dirs := [4]string{"/data/Фото", "/home/u/Работа", "/home/u/Pictures", "/etc"}
	if runtime.GOOS == "windows" {
		e = winEnv()
		dirs = [4]string{`D:\Фото`, `C:\Users\u\Работа`, `C:\Users\u\Pictures`, `C:\Windows`}
	}
	a.askM().rules = func() *folderRules { return e.rules }
	for _, p := range []prefs.Place{
		{Dir: dirs[0], How: prefs.PlaceParent},
		{Dir: dirs[1], How: prefs.PlaceParent}, // прямая подпапка домашней
		{Dir: dirs[2], How: prefs.PlaceChosen}, // явное — можно
		{Dir: dirs[3], How: prefs.PlaceChosen},
	} {
		prefs.AddPlace(a.cfgDir, p)
	}
	got := a.usablePlaces()
	if len(got) != 2 || got[0].Dir != dirs[2] || got[0].How != prefs.PlaceChosen || got[1].Dir != dirs[0] {
		t.Errorf("usablePlaces = %v", got)
	}
	// Правил нет (fail-closed) — мест нет.
	a.askM().rules = func() *folderRules { return nil }
	if got := a.usablePlaces(); len(got) != 0 {
		t.Errorf("без правил: %v", got)
	}
}
