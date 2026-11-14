package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/demopack"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const demoF = demopack.File

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// demoAt — файл примера с малым размером и суммой под содержимое data; cands — как передали.
func demoAt(data []byte, cands ...string) *demoFile {
	return &demoFile{cands: cands, size: int64(len(data)), sum: sumHex(data)}
}

// demoDir кладёт data в <tmp>/data/<File>; возвращает путь файла.
func demoDir(t *testing.T, data []byte) string {
	t.Helper()
	dir := filepath.Join(testkit.TempDir(t), "data")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, demoF)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDemoFilePathsByOS — кандидаты по ОС: data рядом с exe везде, Resources на Mac, пакет
// данных на Linux; /usr/bin/data пропускается; неизвестный exe — только системные пути.
func TestDemoFilePathsByOS(t *testing.T) {
	cases := []struct {
		name, goos, exe string
		want            []string
	}{
		{"windows", "windows", `C:\Program Files\Sosnina\SosninaAlbums.exe`, []string{`C:\Program Files\Sosnina\data\` + demoF}},
		{"windows, косая черта", "windows", `D:/Sa/SosninaAlbums.exe`, []string{`D:/Sa\data\` + demoF}},
		{"windows, UNC (кандидат есть, Has его отвергнет)", "windows", `\\srv\share\Sa\app.exe`, []string{`\\srv\share\Sa\data\` + demoF}},
		{"mac", "darwin", "/Applications/Sosnina.app/Contents/MacOS/Sosnina", []string{
			"/Applications/Sosnina.app/Contents/MacOS/data/" + demoF, "/Applications/Sosnina.app/Contents/Resources/" + demoF}},
		{"linux, переносной", "linux", "/opt/sa/sosninaalbums", []string{"/opt/sa/data/" + demoF, "/usr/share/sosninaalbums/" + demoF}},
		{"linux, /usr/bin", "linux", "/usr/bin/sosninaalbums", []string{"/usr/share/sosninaalbums/" + demoF}},
		{"linux, exe неизвестен", "linux", "", []string{"/usr/share/sosninaalbums/" + demoF}},
		{"windows, exe без папки", "windows", "app.exe", nil},
		{"freebsd", "freebsd", "/opt/sa/app", []string{"/opt/sa/data/" + demoF}},
	}
	for _, c := range cases {
		if got := demoPaths(c.goos, c.exe); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}

// TestDemoFileMacSymlink — запуск Mac через ссылку: папка ищется у настоящего exe.
func TestDemoFileMacSymlink(t *testing.T) {
	root, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	macos := filepath.Join(root, "Sa.app", "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macos, "Sa")
	if err := os.WriteFile(exe, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "sa-link")
	if err := os.Symlink(exe, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if got := demoExe("darwin", link); got != exe {
		t.Errorf("darwin: %q, ждали %q", got, exe)
	}
	if got := demoExe("linux", link); got != link {
		t.Errorf("linux ссылку не раскрывает: %q", got)
	}
	if got := demoExe("darwin", filepath.Join(root, "нет")); got != filepath.Join(root, "нет") {
		t.Errorf("нет файла: путь как был, получили %q", got)
	}
	// Пути Mac — строки с «/»: на Windows их даёт ToSlash (демо-путь Mac режется только по «/»).
	want := filepath.ToSlash(filepath.Join(root, "Sa.app", "Contents", "Resources", demoF))
	if got := demoPaths("darwin", filepath.ToSlash(demoExe("darwin", link))); !slices.Contains(got, want) {
		t.Errorf("Resources не найден: %q", got)
	}
}

// TestDemoFileHas — Has: размер, вид файла, порядок кандидатов, один раз за сеанс.
func TestDemoFileHas(t *testing.T) {
	data := []byte("пример-набора-0123456789")
	good := demoDir(t, data)
	t.Run("есть", func(t *testing.T) {
		if !demoAt(data, good).Has() {
			t.Error("файл есть, Has false")
		}
	})
	t.Run("нет файла и кандидатов", func(t *testing.T) {
		if demoAt(data, filepath.Join(testkit.TempDir(t), "нет")).Has() || demoAt(data).Has() {
			t.Error("файла нет, Has true")
		}
	})
	t.Run("размер не тот: меньше, больше, пустой", func(t *testing.T) {
		for _, n := range []int{0, len(data) - 1, len(data) + 1, len(data) * 50} {
			if demoAt(data, demoDir(t, bytes.Repeat([]byte("a"), n))).Has() {
				t.Errorf("размер %d принят", n)
			}
		}
	})
	t.Run("папка вместо файла", func(t *testing.T) {
		if demoAt(data, filepath.Dir(good)).Has() {
			t.Error("папка принята")
		}
	})
	t.Run("ссылка пропускается", func(t *testing.T) {
		link := filepath.Join(testkit.TempDir(t), "l.zip")
		if err := os.Symlink(good, link); err != nil {
			t.Skipf("ссылку не создать: %v", err)
		}
		if demoAt(data, link).Has() {
			t.Error("ссылка принята")
		}
		if !demoAt(data, link, good).Has() {
			t.Error("после ссылки идёт следующий кандидат")
		}
	})
	demoHasCandidates(t, data, good)
}

// demoHasCandidates — порядок кандидатов, нелокальные пути, один раз за сеанс.
func demoHasCandidates(t *testing.T, data []byte, good string) {
	t.Helper()
	t.Run("первый кандидат негодный — берётся второй", func(t *testing.T) {
		bad := demoDir(t, []byte("не тот размер"))
		d := demoAt(data, bad, good)
		if !d.Has() || d.path != good {
			t.Errorf("взят %q", d.path)
		}
	})
	t.Run("нелокальный путь", func(t *testing.T) {
		if demoAt(data, "//srv/share/data/"+demoF, "относительный/"+demoF, "", "a\x00b").Has() {
			t.Error("нелокальный путь принят")
		}
	})
	t.Run("проверка один раз за сеанс", func(t *testing.T) {
		p := demoDir(t, data)
		d := demoAt(data, p)
		if !d.Has() {
			t.Fatal("файл есть")
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if !d.Has() {
			t.Error("повторный Has заново смотрит на диск")
		}
	})
}

// TestDemoReadOK — Read отдаёт ровно байты файла, повторно тоже.
func TestDemoReadOK(t *testing.T) {
	data := bytes.Repeat([]byte("кадр"), 1000)
	d := demoAt(data, demoDir(t, data))
	for range 2 {
		got, ok := d.Read()
		if !ok || !bytes.Equal(got, data) {
			t.Fatalf("Read: ok=%v, %d байт", ok, len(got))
		}
	}
}

// TestDemoReadRefusals — битые входы: Read отказывает без паники, Has после отказа false до
// конца сеанса, в log.txt одна строка без пути.
func TestDemoReadRefusals(t *testing.T) {
	data := []byte("0123456789abcdef")
	other := []byte("fedcba9876543210") // тот же размер, другие байты
	cases := map[string]func(p string) error{
		"чужие байты того же размера": func(p string) error { return os.WriteFile(p, other, 0o600) },
		"файл вырос":                  func(p string) error { return os.WriteFile(p, append(append([]byte{}, data...), 'x'), 0o600) },
		"файл усечён":                 func(p string) error { return os.WriteFile(p, data[:5], 0o600) },
		"файл пуст":                   func(p string) error { return os.WriteFile(p, nil, 0o600) },
		"файл огромный":               func(p string) error { return os.WriteFile(p, bytes.Repeat([]byte("z"), 1<<20), 0o600) },
		"файл удалён":                 os.Remove,
		"на месте файла папка": func(p string) error {
			if err := os.Remove(p); err != nil {
				return err
			}
			return os.Mkdir(p, 0o700)
		},
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			buf := logTo(t)
			p := demoDir(t, data)
			d := demoAt(data, p)
			if !d.Has() {
				t.Fatal("до подмены файл есть")
			}
			if err := spoil(p); err != nil { // подмена между Has и Read
				t.Fatal(err)
			}
			if got, ok := d.Read(); ok || got != nil {
				t.Fatalf("подменённый файл принят: ok=%v, %d байт", ok, len(got))
			}
			if d.Has() {
				t.Error("после отказа Has должен быть false")
			}
			if _, ok := d.Read(); ok {
				t.Error("повторный Read принял")
			}
			if n := strings.Count(buf.String(), "пример:"); n != 1 {
				t.Errorf("строк в log.txt %d, ждали одну: %q", n, buf.String())
			}
			if strings.Contains(buf.String(), filepath.Dir(p)) {
				t.Errorf("в log.txt путь: %q", buf.String())
			}
		})
	}
}

// TestDemoReadNoFile — файла нет: Read молча отказывает, log.txt пуст (компактная — обычный случай).
func TestDemoReadNoFile(t *testing.T) {
	buf := logTo(t)
	d := demoAt([]byte("x"), filepath.Join(testkit.TempDir(t), "нет"))
	if got, ok := d.Read(); ok || got != nil || buf.Len() != 0 {
		t.Errorf("ok=%v, log %q", ok, buf.String())
	}
}

// TestDemoReadRealZip — боевые размер и сумма: настоящий zip из demopack ставится из data/ рядом
// с exe и открывается проверкой набора.
func TestDemoReadRealZip(t *testing.T) {
	zip, err := os.ReadFile(filepath.Join("..", "demopack", demopack.File))
	if err != nil {
		t.Fatal(err)
	}
	p := demoDir(t, zip)
	exe := filepath.Join(filepath.Dir(filepath.Dir(p)), "SosninaAlbums")
	d := newDemoFile(runtime.GOOS, exe)
	if !d.Has() {
		t.Fatal("файл рядом с exe не найден")
	}
	got, ok := d.Read()
	if !ok || !bytes.Equal(got, zip) {
		t.Fatalf("Read: ok=%v, %d байт из %d", ok, len(got), len(zip))
	}
	if _, err := pack.OpenReader(bytes.NewReader(got), int64(len(got)), "0.29.0"); err != nil {
		t.Errorf("OpenReader: %v", err)
	}
	// усечённый настоящий файл — другой размер: Has false, установки нет
	if err := os.WriteFile(p, zip[:len(zip)-1], 0o600); err != nil {
		t.Fatal(err)
	}
	if newDemoFile(runtime.GOOS, exe).Has() {
		t.Error("усечённый файл принят")
	}
}

// TestDemoOff — SA_DEMO=off действует только в сборке для разработки; выключенный пример пуст.
func TestDemoOff(t *testing.T) {
	for _, c := range []struct {
		env  string
		dev  bool
		want bool
	}{
		{"off", true, true}, {"off", false, false}, {"", true, false}, {"OFF", true, false}, {"on", true, false}, {"off ", true, false},
	} {
		if got := demoOff(c.env, c.dev); got != c.want {
			t.Errorf("demoOff(%q, %v) = %v", c.env, c.dev, got)
		}
	}
	f := demoAt([]byte("x"))
	if d := makeDemo(f, true); d.ID != "" || d.Has != nil || d.Read != nil {
		t.Errorf("выключенный пример не пуст: %+v", d)
	}
	d := makeDemo(f, false)
	e := d.Entry
	if d.ID != demopack.ID || d.Version != demopack.Version || d.Asset != demopack.Asset || !slices.Equal(d.Related, demopack.Related) ||
		e.ID != demopack.ID || e.Size != demopack.Size || e.SHA256 != demopack.SHA256 || e.URL != "" ||
		e.Title != demopack.Title || e.About != demopack.About || e.License != demopack.License ||
		e.Counts.Bg != 4 || e.Counts.Sticker != 7 || e.Counts.Spread != 4 || e.Counts.Frame != 2 || d.Has == nil || d.Read == nil {
		t.Errorf("описание примера: %+v", d)
	}
	// не dev-сборка (тесты идут без тега): переменная не выключает пример
	t.Setenv("SA_DEMO", "off")
	var a App
	if a.demo().ID != demopack.ID {
		t.Error("в выпуске SA_DEMO=off не должен действовать")
	}
}
