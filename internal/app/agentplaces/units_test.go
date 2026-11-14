package agentplaces

// Юнит-тесты пакета (155 Т1, критерий 155-4): нулевые и nil-правила, подсказка, пробы, том и сверка
// с местами — без App. Сценарии с окном и событиями остаются в internal/app.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// shortRules — правила с коротким пределом пробы: зависший диск не задерживает тест.
func shortRules(windows bool, disk func(string) (string, error)) *Rules {
	o := Opts{Fold: func(s string) string { return s }, Timeout: 20 * time.Millisecond, Disk: disk, Homes: []string{"/home/u"}}
	if windows {
		o.Windows, o.Fold, o.Homes = true, strings.ToLower, []string{`C:\Users\u`}
	}
	return New(o)
}

// Нулевое Rules{} и new(Rules) отказывают (ErrPlaceBad), а не разрешают всё; nil — прежняя ErrTooWide.
func TestZeroRulesRefuse(t *testing.T) {
	// Каждая операция возвращает ошибку правил; у нулевого значения она одна — ErrPlaceBad.
	ops := map[string]func(r *Rules) error{
		"Folder":     func(r *Rules) error { return r.Folder("/data/x") },
		"Static":     func(r *Rules) error { _, err := r.Static("/data/x"); return err },
		"Resolve":    func(r *Rules) error { _, _, err := r.Resolve("/data/x"); return err },
		"JudgeWrite": func(r *Rules) error { _, err := r.JudgeWrite("/data/x"); return err },
		"WriteDir":   func(r *Rules) error { _, err := r.WriteDir("/data/x"); return err },
		"ExportPick": func(r *Rules) error { _, err := r.ExportPick("/data/x"); return err },
		"Env.Static": func(r *Rules) error { return (&Env{Rules: r}).Static("/data/x", prefs.PlaceChosen) },
		"Env.Admit":  func(r *Rules) error { _, err := (&Env{Rules: r}).Admit("/data/x", prefs.PlaceChosen); return err },
	}
	for name, r := range map[string]*Rules{"Rules{}": {}, "new(Rules)": new(Rules)} {
		for op, f := range ops {
			if err := f(r); !errors.Is(err, ErrPlaceBad) {
				t.Errorf("%s %s: %v", name, op, err)
			}
		}
		if dir, ok := r.Hint("/data/x"); ok || dir != "" {
			t.Errorf("%s Hint: %q %v", name, dir, ok)
		}
		if dir, label := HintShown(r, "/data/x"); dir != "" || label != "" {
			t.Errorf("%s HintShown: %q %q", name, dir, label)
		}
		if r.HiddenHint(`C:\x`, func(string) bool { return false }) || r.VolumeMoved(`C:\x`, `D:\x`) {
			t.Errorf("%s: HiddenHint или VolumeMoved не отказали", name)
		}
	}
	var none *Rules
	if err := none.Folder("/data/x"); !errors.Is(err, ErrTooWide) {
		t.Errorf("nil Folder: %v", err)
	}
	if _, err := none.JudgeWrite("/data/x"); !errors.Is(err, ErrTooWide) {
		t.Errorf("nil JudgeWrite: %v", err)
	}
	var noEnv *Env
	if err := noEnv.Static("/data/x", prefs.PlaceChosen); !errors.Is(err, ErrPlaceBad) {
		t.Errorf("nil Env.Static: %v", err)
	}
	if _, err := noEnv.Admit("/data/x", prefs.PlaceParent); !errors.Is(err, ErrPlaceBad) {
		t.Errorf("nil Env.Admit: %v", err)
	}
	if noEnv.VolOfDir("/data/x") != nil || noEnv.BindInside(nil, nil) != nil {
		t.Error("nil Env: том не должен находиться")
	}
	// Собранные правила на том же пути проходят: отказ выше — от признака сборки, а не от пути.
	if err := unixRules("linux", diskOK).Folder("/data/x"); err != nil {
		t.Errorf("собранные правила: %v", err)
	}
}

// Timeout и Disk отдают то, что задано; Default на этой машине отвергает домашнюю папку и корень.
func TestRulesAccessorsAndDefault(t *testing.T) {
	r := New(Opts{Timeout: 7 * time.Millisecond, Disk: diskOK})
	if r.Timeout() != 7*time.Millisecond {
		t.Errorf("Timeout = %v", r.Timeout())
	}
	if p, err := r.Disk()("/x"); err != nil || p != "/x" {
		t.Errorf("Disk: %q %v", p, err)
	}
	home := realTemp(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	d := Default()
	if d == nil {
		t.Fatal("Default вернул nil")
	}
	if d.Folder(home) == nil {
		t.Error("Default принял домашнюю папку")
	}
	if d.Folder(filepath.VolumeName(home)+string(filepath.Separator)) == nil {
		t.Error("Default принял корень тома")
	}
	if d.Timeout() != osx.ProbeTimeout {
		t.Errorf("предел Default = %v", d.Timeout())
	}
}

// Resolve: путь после ссылок возвращается; ссылка в чужое, ошибка и зависание диска — отказ.
func TestResolve(t *testing.T) {
	r := unixRules("linux", func(p string) (string, error) { return p + "/real", nil })
	cleaned, resolved, err := r.Resolve("/data/x/")
	if err != nil || cleaned != "/data/x" || resolved != "/data/x/real" {
		t.Errorf("Resolve: %q %q %v", cleaned, resolved, err)
	}
	for name, disk := range map[string]func(string) (string, error){
		"в системное": func(string) (string, error) { return "/etc", nil },
		"ошибка":      func(string) (string, error) { return "", errors.New("нет") },
		"пустой путь": func(string) (string, error) { return "", nil },
	} {
		if _, _, err := unixRules("linux", disk).Resolve("/data/x"); err == nil {
			t.Errorf("%s: принято", name)
		}
	}
	if _, _, err := unixRules("linux", diskOK).Resolve("/etc"); err == nil {
		t.Error("системная папка принята до диска")
	}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	hang := shortRules(false, func(string) (string, error) { <-release; return "", nil })
	if _, _, err := hang.Resolve("/data/x"); !errors.Is(err, ErrTooWide) {
		t.Errorf("зависший диск: %v", err)
	}
}

// HiddenHint: Windows — элемент с точкой под домашней и скрытый ниже корня; зависшая проба — скрытый.
func TestHiddenHint(t *testing.T) {
	no, yes := func(string) bool { return false }, func(string) bool { return true }
	win := winRules(diskOK)
	for _, c := range []struct {
		p      string
		hidden func(string) bool
		want   bool
	}{
		{`C:\Users\u\.ssh\x`, no, true}, {`C:\Users\u\Pics\x`, no, false}, {`C:\Users\u\Pics`, yes, true}, {`D:\Фото`, no, false},
		{`C:\Users\u`, no, false},
	} {
		if got := win.HiddenHint(c.p, c.hidden); got != c.want {
			t.Errorf("%q: %v, ждали %v", c.p, got, c.want)
		}
	}
	if unixRules("linux", diskOK).HiddenHint("/home/u/.ssh", yes) {
		t.Error("на Unix скрытое по атрибутам не проверяется")
	}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	if !shortRules(true, diskOK).HiddenHint(`D:\x`, func(string) bool { <-release; return false }) {
		t.Error("зависшая проба атрибутов должна считаться скрытым")
	}
}

// VolumeMoved: смена тома Windows (буква без регистра, UNC), Unix и nil — нет.
func TestVolumeMoved(t *testing.T) {
	win := winRules(diskOK)
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{`C:\x`, `D:\x`, true}, {`c:\x`, `C:\y`, false}, {`\\srv\share\a`, `\\srv\share\b`, false},
		{`\\srv\share\a`, `\\srv\other\a`, true}, {`C:\x`, `\\srv\share\x`, true},
	} {
		if got := win.VolumeMoved(c.a, c.b); got != c.want {
			t.Errorf("%q → %q: %v", c.a, c.b, got)
		}
	}
	if unixRules("linux", diskOK).VolumeMoved("/a", "/b") || (*Rules)(nil).VolumeMoved(`C:\a`, `D:\a`) {
		t.Error("Unix и nil не меняют том")
	}
}

// InAny пропускает пустые папки; ScreenshotDirs знает «Screenshots» и «Снимки экрана» в OneDrive.
func TestInAnyAndScreenshotDirs(t *testing.T) {
	r := unixRules("linux", diskOK)
	if !InAny(r, "/a/b", []string{"", "/x", "/a"}) || InAny(r, "/a/b", []string{"", "/x"}) || InAny(r, "/a", nil) {
		t.Error("InAny")
	}
	od := realTemp(t)
	t.Setenv("OneDrive", od)
	var shots, snaps bool
	for _, d := range ScreenshotDirs() {
		shots = shots || d == filepath.Join(od, "Pictures", "Screenshots")
		snaps = snaps || d == filepath.Join(od, "Изображения", "Снимки экрана")
	}
	if !shots || !snaps {
		t.Errorf("OneDrive: Screenshots=%v, Снимки экрана=%v", shots, snaps)
	}
}

// HintShown и HintLabel: последний элемент, длинное имя усечено «…», негодная подсказка — пусто.
func TestHintShown(t *testing.T) {
	r := unixRules("linux", diskOK)
	if dir, label := HintShown(r, "/data/Петровы"); dir != "/data/Петровы" || label != "Петровы" {
		t.Errorf("годная: %q %q", dir, label)
	}
	long := strings.Repeat("ж", 80)
	if _, label := HintShown(r, "/data/"+long); label != strings.Repeat("ж", hintShownMax-1)+"…" {
		t.Errorf("длинная: %q", label)
	}
	for _, bad := range []string{"/etc", "", "относительный", "/data/" + strings.Repeat("я", agentPathMax)} {
		if dir, label := HintShown(r, bad); dir != "" || label != "" {
			t.Errorf("%.20q: %q %q", bad, dir, label)
		}
	}
	if HintShown(nil, "/data/x"); lastElem(`C:\a\Б`, true) != "Б" || lastElem(`C:\a/Б`, false) != `Б` {
		t.Error("lastElem")
	}
}

// Probe: привязанное и непривязанное место, пропавший диск и папка, нераспознанный том.
func TestProbe(t *testing.T) {
	dir := realTemp(t)
	gone := filepath.Join(dir, "нет")
	vol := func(v osx.Vol, err error) func(string) (osx.Vol, error) {
		return func(string) (osx.Vol, error) { return v, err }
	}
	path := func(p, label string, err error) func(string, string) (string, string, error) {
		return func(string, string) (string, string, error) { return p, label, err }
	}
	bound := &prefs.PlaceVol{ID: "A", Rel: "x"}
	for name, c := range map[string]struct {
		pl   prefs.Place
		vol  func(string) (osx.Vol, error)
		path func(string, string) (string, string, error)
		want ProbeRes
	}{
		"диск пропал":   {prefs.Place{Dir: dir, Vol: bound}, nil, path("", "", osx.ErrVolGone), ProbeRes{State: StateNoDisk}},
		"том не узнан":  {prefs.Place{Dir: dir, Vol: bound}, nil, path("", "", errors.New("?")), ProbeRes{}},
		"нашёлся":       {prefs.Place{Dir: dir, Vol: bound}, nil, path(dir, "Фото", nil), ProbeRes{HasPath: true, Dir: dir, Label: "Фото"}},
		"папки нет":     {prefs.Place{Dir: dir, Vol: bound}, nil, path(gone, "Фото", nil), ProbeRes{State: StateNoFolder, HasPath: true, Dir: gone, Label: "Фото"}},
		"без привязки":  {prefs.Place{Dir: dir}, vol(osx.Vol{Label: "Карта"}, nil), nil, ProbeRes{State: StateHere, Here: "Карта"}},
		"без тома":      {prefs.Place{Dir: dir}, vol(osx.Vol{}, errors.New("?")), nil, ProbeRes{}},
		"нет папки":     {prefs.Place{Dir: gone}, vol(osx.Vol{}, nil), nil, ProbeRes{State: StateNoFolder}},
		"путь негоден":  {prefs.Place{Dir: "относительный"}, vol(osx.Vol{}, nil), nil, ProbeRes{State: StateNoFolder}},
		"файл не папка": {prefs.Place{Dir: filepath.Join(dir, "f")}, vol(osx.Vol{}, nil), nil, ProbeRes{State: StateNoFolder}},
	} {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := Probe(c.pl, c.vol, c.path); got != c.want {
			t.Errorf("%s: %+v, ждали %+v", name, got, c.want)
		}
	}
}

// Подмены тома и диска: nil берёт настоящие, заданные — свои; VolOfDir держит предел.
func TestEnvVolSubstitutes(t *testing.T) {
	var e *Env
	if e.VolFn() == nil || e.VolPathFn() == nil || (&Env{}).VolFn() == nil || (&Env{}).VolPathFn() == nil {
		t.Fatal("без подмены должны быть настоящие функции")
	}
	e = &Env{Rules: shortRules(false, diskOK)}
	e.Vol = func(string) (osx.Vol, error) { return osx.Vol{ID: "A", Label: "Л", Rel: "x/y"}, nil }
	e.VolPath = func(id, rel string) (string, string, error) { return id + rel, "Л", nil }
	if p, l, err := e.VolPathFn()("A", "x"); p != "Ax" || l != "Л" || err != nil {
		t.Errorf("VolPathFn: %q %q %v", p, l, err)
	}
	if v := e.VolOfDir("/data"); v == nil || *v != (prefs.PlaceVol{ID: "A", Label: "Л", Rel: "x/y"}) {
		t.Errorf("VolOfDir: %+v", v)
	}
	e.Vol = func(string) (osx.Vol, error) { return osx.Vol{}, errors.New("?") }
	if e.VolOfDir("/data") != nil {
		t.Error("ошибка тома должна дать nil")
	}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	e.Vol = func(string) (osx.Vol, error) { <-release; return osx.Vol{}, nil }
	if e.VolOfDir("/data") != nil {
		t.Error("зависшее чтение тома должно дать nil")
	}
	// Подмены диска не заданы: настоящие DevOf и таблица монтирования.
	env := realEnv()
	dir := realTemp(t)
	if _, ok := env.devFn()(dir); !ok {
		t.Error("DevOf не узнал настоящую папку")
	}
	_, ok := env.devFn()(filepath.Join(dir, "нет"))
	if runtime.GOOS != "windows" {
		if ok {
			t.Error("DevOf узнал несуществующую папку")
		}
	} else if !ok {
		// На Windows DevOf не различает тома (dev_windows.go), корни отсекает правило 149.
		t.Error("DevOf на Windows должен вернуть (0, true)")
	}
	if env.mountsFn() == nil {
		t.Error("mountsFn")
	}
	if runtime.GOOS != "windows" {
		if _, err := env.mountsFn()(); err != nil {
			t.Errorf("таблица монтирования: %v", err)
		}
	}
}

// BindInside: привязывает только места без диска и только если проба их тома совпала с добавленным.
func TestBindInside(t *testing.T) {
	e := &Env{Rules: shortRules(false, diskOK)}
	e.Vol = func(p string) (osx.Vol, error) {
		return osx.Vol{ID: map[string]string{"/a": "X", "/q": "Y"}[p], Rel: "r"}, nil
	}
	add := []prefs.Place{{Dir: "/a/c", How: prefs.PlaceChosen, Vol: &prefs.PlaceVol{ID: "X"}}, {Dir: "/z", How: prefs.PlaceChosen}}
	cur := []prefs.Place{
		{Dir: "/a", How: prefs.PlaceChosen},                                // внутри нового, том совпал
		{Dir: "/q/w", How: prefs.PlaceChosen},                              // не связано с добавленными
		{Dir: "/a/c/d", How: prefs.PlaceChosen},                            // снаружи: том пробы пуст, не совпал
		{Dir: "/a", How: prefs.PlaceParent, Vol: &prefs.PlaceVol{ID: "X"}}, // уже с диском
	}
	got := e.BindInside(cur, add)
	if len(got) != 1 || got[prefs.PlaceKey(cur[0])].ID != "X" {
		t.Errorf("BindInside = %v", got)
	}
	if e.BindInside(nil, add) != nil || e.BindInside(cur, nil) != nil {
		t.Error("пустой список не должен ничего привязывать")
	}
}

func TestVolSegsRelBelow(t *testing.T) {
	for in, want := range map[string]int{"": 0, "a": 1, "a/b": 2, "a/b/c": 3} {
		if got := volSegs(in); got != want {
			t.Errorf("volSegs(%q) = %d", in, got)
		}
	}
	r := winRules(diskOK)
	for _, c := range []struct {
		rel, base string
		want      bool
	}{{"a/b", "a", true}, {"A/B", "a", true}, {"a", "a", false}, {"a", "", true}, {"", "", false}, {"ab", "a", false}} {
		if got := r.relBelow(c.rel, c.base); got != c.want {
			t.Errorf("relBelow(%q, %q) = %v", c.rel, c.base, got)
		}
	}
}

// diskNote: с меткой диска и без метки (место привязано, но метки у него не было).
func TestDiskNote(t *testing.T) {
	if n := diskNote(prefs.Place{Dir: "/x", Vol: &prefs.PlaceVol{ID: "a", Label: "Фото-архив"}}); n != "Место «/x» — на диске «Фото-архив»" {
		t.Errorf("с меткой: %q", n)
	}
	n := diskNote(prefs.Place{Dir: "/x", Vol: &prefs.PlaceVol{ID: "a"}})
	if !strings.Contains(n, "другом диске") || strings.Contains(n, "на диске «") {
		t.Errorf("без метки: %q", n)
	}
}

// Scan: место без привязки — по пути и разрешению в себя; с диском — по номеру тома; чужой диск и
// нераспознанный том дают причину и строку, остальное — пусто.
func TestScan(t *testing.T) {
	e := unixEnv("linux")
	chosen := func(dir string, v *prefs.PlaceVol) prefs.Place {
		return prefs.Place{Dir: dir, How: prefs.PlaceChosen, Vol: v}
	}
	here := osx.Vol{ID: "A", Rel: "Клиент/Свадьба"}
	for name, c := range map[string]struct {
		disk   func(string) (string, error)
		res    string
		places []prefs.Place
		v      osx.Vol
		verr   error
		want   ScanRes
	}{
		"без привязки, внутри":   {diskOK, "/data/Клиент/Свадьба", []prefs.Place{chosen("/data/Клиент", nil)}, here, nil, ScanRes{OK: true}},
		"сама папка места":       {diskOK, "/data/Клиент", []prefs.Place{chosen("/data/Клиент", nil)}, here, nil, ScanRes{}},
		"не в себя":              {func(string) (string, error) { return "/x", nil }, "/data/Клиент/С", []prefs.Place{chosen("/data/Клиент", nil)}, here, nil, ScanRes{}},
		"диск ошибка":            {func(string) (string, error) { return "", errors.New("?") }, "/data/Клиент/С", []prefs.Place{chosen("/data/Клиент", nil)}, here, nil, ScanRes{}},
		"тот же диск":            {diskOK, "/data/Клиент/Свадьба", []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "A", Rel: "Клиент"})}, here, nil, ScanRes{OK: true}},
		"тот же диск, не внутри": {diskOK, "/data/Клиент/Свадьба", []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "A", Rel: "Другое"})}, here, nil, ScanRes{}},
		"чужой диск": {diskOK, "/data/Клиент/Свадьба", []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "B", Label: "Архив"})}, here, nil,
			ScanRes{Why: WhyDisk, Note: "Место «/data/Клиент» — на диске «Архив»"}},
		"диск не узнан": {diskOK, "/data/Клиент/Свадьба", []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "B"})}, osx.Vol{}, errors.New("?"), ScanRes{Why: WhyVol}},
		"вне места":     {diskOK, "/other/x", []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "B"})}, here, nil, ScanRes{}},
		"нет мест":      {diskOK, "/data/x", nil, here, nil, ScanRes{}},
	} {
		if got := Scan(e, c.disk, c.res, c.places, c.v, c.verr); got != c.want {
			t.Errorf("%s: %+v, ждали %+v", name, got, c.want)
		}
	}
	// Второе место подходит по диску, хотя первое подсказало причину: итог — OK.
	places := []prefs.Place{chosen("/data/Клиент", &prefs.PlaceVol{ID: "B"}), chosen("/data/Клиент", &prefs.PlaceVol{ID: "A", Rel: "Клиент"})}
	if got := Scan(e, diskOK, "/data/Клиент/Свадьба", places, here, nil); !got.OK {
		t.Errorf("второе место: %+v", got)
	}
}
