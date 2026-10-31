package agentplaces

// Правила записи для выгрузки по просьбе агента (152 Т3): таблицы пути, диск и подмена между вопросом и
// щелчком. Чистые тесты правил переехали из app вместе с правилами (155 Т1).

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// realTemp — временная папка настоящим путём: правило папки отдаёт путь после ссылок и подмены тома, а на Windows
// TempDir раннера в 8.3 (RUNNER~1) — сравнивать «показанное» с «созданным» можно только в одной форме.
func realTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err == nil {
		p, err = osx.RealPath(p)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// writeOpts — правила записи для таблицы: Windows или Unix, домашняя, три целые папки, наши, системные деревья.
func writeOpts(windows bool) Opts {
	o := Opts{Windows: windows, Fold: paths.Fold, Timeout: time.Second, Disk: diskOK}
	if windows {
		o.Fold = strings.ToLower // регистр Windows не важен, а тест идёт и на Linux
		o.System = "C:"
		o.Homes = []string{`C:\Users\u`}
		o.Whole = []string{`C:\Users\u\Desktop`, `C:\Users\u\Documents`, `C:\Users\u\Downloads`}
		o.Ours = []string{`C:\Users\u\AppData\Roaming\SosninaAlbums`, `C:\Program Files\SosninaAlbums`}
		o.Lists = "windows"
		return o
	}
	o.Homes = []string{"/home/u"}
	o.Whole = []string{"/home/u/Desktop", "/home/u/Documents", "/home/u/Downloads"}
	o.Ours = []string{"/home/u/.config/sosninaalbums"}
	o.Lists = "linux"
	return o
}

func writeRules(windows bool) *Rules { return New(writeOpts(windows)) }

// TestAgentExportHintDestination: куда агент может предложить выгрузку (U59, U61): флешка, «Рабочий стол\Петровы» — да;
// системные, корень системного тома, домашняя, скрытое (`.ssh` и на Windows), UNC, папка программы — нет.
func TestAgentExportHintDestination(t *testing.T) {
	win := writeRules(true)
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{`E:\`, true}, {`E:\Выгрузки\Петровы`, true}, {`C:\Users\u\Desktop`, true}, {`C:\Users\u\Desktop\Петровы`, true},
		{`C:\Users\u\Documents\x`, true}, {`C:\Users\u\Петровы`, true},
		{`C:\Windows`, false}, {`C:\Windows\Temp\x`, false}, {`C:\`, false}, {`C:\Users`, false}, {`C:\Users\u`, false},
		{`C:\Users\u\.ssh`, false}, {`D:\a\.hidden\x`, false}, {`\\server\share\x`, false}, {`C:\Program Files\SosninaAlbums\x`, false},
		{`C:\Users\u\AppData\Roaming\SosninaAlbums`, false}, {`relative\x`, false}, {`D:\System Volume Information`, false},
		{"E:\\a\u202eb", false}, {"", false},
	} {
		if _, err := win.JudgeWrite(c.p); (err == nil) != c.ok {
			t.Errorf("Windows %q: err=%v, ждали ok=%v", c.p, err, c.ok)
		}
	}
	unix := writeRules(false)
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{"/media/u/USB/Выгрузки", true}, {"/home/u/Desktop/Петровы", true}, {"/home/u/Документы2/x", true},
		{"/etc/x", false}, {"/", false}, {"/home/u", false}, {"/home", false}, {"/home/u/.ssh", false},
		{"/home/u/.config/sosninaalbums/x", false}, {"//server/x", false}, {"/media", false}, {"/proc/self", false},
	} {
		if _, err := unix.JudgeWrite(c.p); (err == nil) != c.ok {
			t.Errorf("Unix %q: err=%v, ждали ok=%v", c.p, err, c.ok)
		}
	}
}

// diskWriteRules — правила записи на настоящем диске: таблица своей ОС (путь Unix на Windows не локален, а
// временная папка Windows лежит под AppData, которую продукт не берёт, — поэтому списки writeRules, а не машины).
func diskWriteRules() *Rules {
	o := writeOpts(runtime.GOOS == "windows")
	o.Disk = nil
	return New(o)
}

// systemDir — системная папка этой ОС: ссылка в неё должна отказывать.
func systemDir() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows`
	}
	return "/etc"
}

// TestAgentExportHintDisk: подсказка на настоящем диске: папка есть — можно, нет последней — «создать»,
// глубже одного уровня, файл вместо папки, ссылка в системную папку — нет; ссылка в свою папку показывается
// настоящим путём; после вопроса папку подменили ссылкой в чужое — повтор при щелчке отказывает (У60).
func TestAgentExportHintDisk(t *testing.T) {
	base := realTemp(t)
	r := diskWriteRules()
	sys := systemDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{base}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	own := mk("Выгрузки")
	if w, err := r.WriteDir(own); err != nil || w.Create() || w.Path() != own {
		t.Errorf("есть папка: %+v %v", w, err)
	}
	if w, err := r.WriteDir(filepath.Join(own, "Петровы")); err != nil || !w.Create() {
		t.Errorf("нет последней: %+v %v", w, err)
	}
	if _, err := r.WriteDir(filepath.Join(own, "а", "б")); err == nil {
		t.Error("глубже одного уровня создавать нельзя")
	}
	file := filepath.Join(base, "файл.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.WriteDir(file); err == nil {
		t.Error("файл вместо папки принят")
	}
	toEtc, toReal := filepath.Join(base, "в_etc"), filepath.Join(base, "в_свою")
	if err := os.Symlink(sys, toEtc); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	if err := os.Symlink(own, toReal); err != nil {
		t.Fatal(err)
	}
	if _, err := r.WriteDir(toEtc); err == nil {
		t.Error("ссылка в системную папку принята")
	}
	if _, err := r.WriteDir(filepath.Join(toEtc, "Петровы")); err == nil {
		t.Error("новая папка внутри ссылки в системную папку принята")
	}
	if w, err := r.WriteDir(toReal); err != nil || r.Key(w.Path()) != r.Key(own) {
		t.Errorf("ссылка в свою папку: %+v %v (ждали настоящий путь %q)", w, err, own)
	}
	shown := filepath.Join(own, "Новая")
	w, err := r.WriteDir(shown)
	if err != nil || !w.Create() {
		t.Fatalf("показанная: %+v %v", w, err)
	}
	if err := os.Symlink(sys, shown); err != nil { // между вопросом и щелчком
		t.Fatal(err)
	}
	if _, err := r.ExportPick(shown); err == nil {
		t.Error("подмена ссылкой в системную папку между вопросом и щелчком не поймана")
	}
}

// TestAgentExportPickCreateRecheck: папку, которой не было, успели подменить — повтор при щелчке её не берёт; обычная
// папка вместо несуществующей — берётся как существующая.
func TestAgentExportPickCreateRecheck(t *testing.T) {
	exists := map[string]string{"/media/u/X": "/media/u/X"}
	o := writeOpts(false)
	o.Disk = func(p string) (string, error) {
		if v, ok := exists[p]; ok {
			return v, nil
		}
		return "", fs.ErrNotExist
	}
	r := New(o)
	w, err := r.WriteDir("/media/u/X/Петровы")
	if err != nil || !w.Create() || w.Path() != "/media/u/X/Петровы" {
		t.Fatalf("новая папка: %+v %v", w, err)
	}
	if _, err := r.WriteDir("/media/u/X/Глубже/Петровы"); err == nil {
		t.Error("глубже одного уровня создавать нельзя")
	}
	exists["/media/u/X/Петровы"] = "/media/u/X/Петровы" // появилась обычная папка
	if w, err := r.ExportPick("/media/u/X/Петровы"); err != nil || w.Create() {
		t.Errorf("появилась обычная: %+v %v", w, err)
	}
	exists["/media/u/X/Петровы"] = "/etc" // подменили ссылкой в чужое
	if _, err := r.ExportPick("/media/u/X/Петровы"); err == nil {
		t.Error("ссылка в системную папку принята")
	}
	exists["/media/u/X/Петровы"] = "/media/u/Y/Петровы" // ссылка в другое место: ключ не совпал
	if _, err := r.ExportPick("/media/u/X/Петровы"); err == nil {
		t.Error("ссылка в другую папку принята")
	}
}

// TestWriteDirNewNameValid: имя новой папки из подсказки-пути годно, как и подсказка-имя: точка и пробел на краю,
// знаки `<>:"|?*`, имя устройства, управляющие знаки — отказ; годное имя — «создать».
func TestWriteDirNewNameValid(t *testing.T) {
	base := realTemp(t)
	r := diskWriteRules()
	for _, bad := range []string{"Петровы.", "Петровы ", "a<b", "a|b", "CON", "a\u202eb", "a?b"} {
		if w, err := r.WriteDir(filepath.Join(base, bad)); err == nil {
			t.Errorf("негодное имя %q принято: %+v", bad, w)
		}
	}
	for _, ok := range []string{"Петровы", "Петровы 2027", "a.b"} {
		if w, err := r.WriteDir(filepath.Join(base, ok)); err != nil || !w.Create() {
			t.Errorf("годное имя %q: %+v, %v", ok, w, err)
		}
	}
	// Существующую папку с «плохим» именем (чужая, уже есть на диске) правило имени не трогает: она не создаётся.
	odd := filepath.Join(base, "Старая.")
	if err := os.Mkdir(odd, 0o755); err != nil {
		t.Skipf("имя не создаётся на этой ОС: %v", err)
	}
	if w, err := r.WriteDir(odd); err != nil || w.Create() {
		t.Errorf("существующая папка: %+v, %v", w, err)
	}
}

// winVolume: буква, UNC и пути без тома.
func TestWinVolume(t *testing.T) {
	for in, want := range map[string]string{
		`C:\x`: `C:`, `d:`: `d:`, `\\srv\share\x`: `\\srv\share`, `\\srv\share`: `\\srv\share`, `\\srv`: "", `/x`: "", "": "", `x`: "",
	} {
		if got := winVolume(in); got != want {
			t.Errorf("winVolume(%q) = %q, ждали %q", in, got, want)
		}
	}
}
