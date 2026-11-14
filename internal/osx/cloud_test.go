package osx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// allowAll — фильтр без отказов; refuseAll — фильтр, отвергающий всё (чужая подмена CheckLocalPath).
func allowAll(string) error  { return nil }
func refuseAll(string) error { return errors.New("не локальный") }

// dropboxFile — где CloudRoots ищет info.json на этой ОС: Windows — %APPDATA% (переменную ставим тесту), прочие — ~/.dropbox.
func dropboxFile(t *testing.T, home string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		appdata := filepath.Join(home, "AppData", "Roaming")
		t.Setenv("APPDATA", appdata)
		t.Setenv("LOCALAPPDATA", "")
		return filepath.Join(appdata, "Dropbox", "info.json")
	}
	return filepath.Join(home, ".dropbox", "info.json")
}

func writeCloudFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// dropboxJSON — info.json с этими корнями (personal, business).
func dropboxJSON(t *testing.T, personal, business string) []byte {
	t.Helper()
	m := map[string]map[string]any{}
	if personal != "" {
		m["personal"] = map[string]any{"path": personal, "host": 123}
	}
	if business != "" {
		m["business"] = map[string]any{"path": business, "is_team": true}
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestCloudRootsDropbox: корни personal и business из info.json; чистятся (Clean, без повторов), фильтр local применяется к
// каждому корню; нет файла — пусто.
func TestCloudRootsDropbox(t *testing.T) {
	home := testkit.TempDir(t)
	if got := CloudRoots(home, allowAll); len(got) != 0 {
		t.Fatalf("файла нет, корни %v", got)
	}
	personal, business := testkit.AbsPath("Данные/Dropbox/"), testkit.AbsPath("Работа/Dropbox (Team)")
	writeCloudFile(t, dropboxFile(t, home), dropboxJSON(t, personal, business))
	want := []string{filepath.Clean(personal), filepath.Clean(business)}
	if got := CloudRoots(home, allowAll); !slices.Equal(got, want) {
		t.Errorf("корни %q, ждали %q", got, want)
	}
	// Одинаковые personal и business — один корень.
	writeCloudFile(t, dropboxFile(t, home), dropboxJSON(t, personal, personal))
	if got := CloudRoots(home, allowAll); len(got) != 1 {
		t.Errorf("повтор: %q", got)
	}
	// Фильтр вызывающего отвергает и файл настроек, и корни: пусто. Без фильтра (nil) — тоже пусто, закрыто по умолчанию.
	if got := CloudRoots(home, refuseAll); len(got) != 0 {
		t.Errorf("фильтр отказал, корни %q", got)
	}
	if got := CloudRoots(home, nil); got != nil {
		t.Errorf("без фильтра %q", got)
	}
	// Фильтр, отвергающий только один корень, оставляет второй.
	writeCloudFile(t, dropboxFile(t, home), dropboxJSON(t, personal, business))
	only := func(p string) error {
		if strings.Contains(p, "Team") {
			return errors.New("нет")
		}
		return nil
	}
	if got := CloudRoots(home, only); !slices.Equal(got, want[:1]) {
		t.Errorf("частичный фильтр %q", got)
	}
}

// TestCloudRootsDropboxFiles: где ищется info.json — Windows %APPDATA% и %LOCALAPPDATA%, прочие ОС ~/.dropbox (чистая функция,
// таблица любой ОС).
func TestCloudRootsDropboxFiles(t *testing.T) {
	env := map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\u\AppData\Local`}
	get := func(k string) string { return env[k] }
	if got := dropboxFiles("windows", "", get); len(got) != 2 || !strings.HasSuffix(got[0], filepath.Join("Dropbox", "info.json")) {
		t.Errorf("windows: %q", got)
	}
	if got := dropboxFiles("windows", "/h", func(string) string { return "" }); len(got) != 0 {
		t.Errorf("windows без переменных: %q", got)
	}
	for _, goos := range []string{"linux", "darwin"} {
		if got := dropboxFiles(goos, "/h", get); len(got) != 1 || got[0] != filepath.Join("/h", ".dropbox", "info.json") {
			t.Errorf("%s: %q", goos, got)
		}
		if got := dropboxFiles(goos, "", get); len(got) != 0 {
			t.Errorf("%s без домашней: %q", goos, got)
		}
	}
}

// TestCloudRootsYandex: config.cfg Яндекс.Диска (Linux) — строка dir с кавычками и без; прочие ключи не читаются.
func TestCloudRootsYandex(t *testing.T) {
	root := testkit.AbsPath("Данные/Яндекс.Диск")
	for _, c := range []struct{ name, cfg, want string }{
		{"в кавычках", "proxy=\"no\"\nauth=\"/x\"\ndir=\"" + root + "\"\nexclude-dirs=\"a\"\n", root},
		{"без кавычек", "dir=" + root + "\n", root},
		{"пробелы вокруг", "  dir = \"" + root + "\"  \r\n", root},
		{"нет dir", "proxy=\"no\"\nread-only=\"\"\n", ""},
		{"пустой dir", "dir=\"\"\n", ""},
		{"не абсолютный", "dir=\"Yandex.Disk\"\n", ""},
		{"управляющий знак", "dir=\"" + root + "\x07\"\n", ""},
		{"пусто", "", ""},
	} {
		got := cleanCloudRoots(parseYandexConfig([]byte(c.cfg)), allowAll)
		if c.want == "" && len(got) != 0 || c.want != "" && !slices.Equal(got, []string{filepath.Clean(c.want)}) {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
	if runtime.GOOS != "linux" {
		return // путь ~/.config/yandex-disk — только Linux
	}
	home := testkit.TempDir(t)
	writeCloudFile(t, filepath.Join(home, ".config", "yandex-disk", "config.cfg"), []byte("dir=\""+root+"\"\n"))
	if got := CloudRoots(home, allowAll); !slices.Equal(got, []string{root}) {
		t.Errorf("CloudRoots: %q", got)
	}
}

// TestCloudRootsBadFiles: чужой info.json — 10 МБ, битый, не JSON, папка вместо файла, корни с UNC, относительные и с
// управляющими знаками: облака нет, журнал чист, паники нет. FIFO и ссылка — cloud_unix_test.go.
func TestCloudRootsBadFiles(t *testing.T) {
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(old) })
	good := testkit.AbsPath("Данные/Dropbox")
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"10 МБ", append([]byte(`{"personal":{"path":"`+good+`"},"pad":"`), bytes.Repeat([]byte("x"), 10<<20)...)},
		{"чуть больше предела", append(dropboxJSON(t, good, ""), bytes.Repeat([]byte(" "), maxCloudFile)...)},
		{"усечённый", dropboxJSON(t, good, "")[:12]},
		{"не JSON", []byte("dir=/x")},
		{"пустой", nil},
		{"массив", []byte(`[1,2,3]`)},
		{"path числом", []byte(`{"personal":{"path":5}}`)},
		{"personal строкой", []byte(`{"personal":"/x"}`)},
		{"относительный", dropboxJSON(t, "Dropbox", "..")},
		{"управляющий знак", dropboxJSON(t, good+"\n/etc", "")},
		{"UNC", dropboxJSON(t, `\\srv\share\Dropbox`, "//srv/share/Dropbox")},
	} {
		home := testkit.TempDir(t)
		writeCloudFile(t, dropboxFile(t, home), c.data)
		// Фильтр как у CheckLocalPath: UNC и относительные не локальны.
		local := func(p string) error {
			if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
				return errors.New("UNC")
			}
			return nil
		}
		if got := CloudRoots(home, local); len(got) != 0 {
			t.Errorf("%s: корни %q", c.name, got)
		}
	}
	// Папка на месте info.json.
	home := testkit.TempDir(t)
	if err := os.MkdirAll(dropboxFile(t, home), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CloudRoots(home, allowAll); len(got) != 0 {
		t.Errorf("папка вместо файла: %q", got)
	}
	// Парсеры на крайних входах не паникуют и ничего не отдают.
	for _, data := range [][]byte{nil, {}, {0}, bytes.Repeat([]byte{'['}, 100000), bytes.Repeat([]byte("dir=\n"), 40000)} {
		_ = parseDropboxInfo(data)
		_ = parseYandexConfig(data)
	}
	if parseDropboxInfo(bytes.Repeat([]byte(" "), maxCloudFile+1)) != nil || parseYandexConfig(bytes.Repeat([]byte("a"), maxCloudFile+1)) != nil {
		t.Error("входы больше предела разобраны")
	}
	if logs.Len() != 0 {
		t.Errorf("журнал не чист: %q", logs.String())
	}
}

// TestCloudRootsLimit: корней не больше maxCloudRoots, повторы (на Windows — с другим регистром) схлопываются.
func TestCloudRootsLimit(t *testing.T) {
	var raw []string
	for i := range maxCloudRoots * 2 {
		raw = append(raw, testkit.AbsPath("Облако/"+strings.Repeat("п", 1+i)))
	}
	if got := cleanCloudRoots(raw, allowAll); len(got) != maxCloudRoots {
		t.Errorf("корней %d, ждали %d", len(got), maxCloudRoots)
	}
	dup := []string{testkit.AbsPath("Облако/A"), testkit.AbsPath("Облако/A/"), testkit.AbsPath("Облако/./A")}
	if got := cleanCloudRoots(dup, allowAll); len(got) != 1 {
		t.Errorf("повторы: %q", got)
	}
	long := testkit.AbsPath(strings.Repeat("д", maxCloudPath))
	if got := cleanCloudRoots([]string{long}, allowAll); len(got) != 0 {
		t.Error("слишком длинный путь принят")
	}
}

// TestVirtualCloudVol: метки тома и ФС виртуальных дисков (таблица любой ОС).
func TestVirtualCloudVol(t *testing.T) {
	for _, c := range []struct {
		label, fs string
		fixed     bool
		want      bool
	}{
		{"Google Drive", "NTFS", true, true},
		{"google drive (G:)", "NTFS", true, true},
		{"Яндекс.Диск", "NTFS", true, true},
		{"pCloud Drive", "FAT32", true, true},
		{"Box", "NTFS", true, true},
		{"Box", "NTFS", false, true}, // метка судит и съёмный
		{"Работа", "NTFS", true, false},
		{"Съёмки", "exFAT", false, false},
		{"Съёмки", "FAT", false, false}, // на карте бывает FAT
		{"", "FAT32", true, false},
		{"Диск", "CDFS", true, true},    // несъёмный с чужой ФС
		{"Диск", "", true, true},        // ФС не прочиталась на несъёмном: реже тихо
		{"Boxing", "NTFS", true, false}, // «Box» только целиком
		{"ReFS", "ReFS", true, false},
	} {
		if got := virtualCloudVol(c.label, c.fs, c.fixed); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
