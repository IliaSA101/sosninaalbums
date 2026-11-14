package guidevideo

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Имя файла из id и версии: обход пути, регистр, имена устройств, длина, версия вне 1…999999.
func TestDownloadNameFromID(t *testing.T) {
	long := strings.Repeat("a", 40)
	for _, tc := range []struct {
		id   string
		ver  int
		kind string
		want string // "" — отказ
	}{
		{"cover", 1, "webm", "cover-1.webm"},
		{"cover-export", 12, "mp4", "cover-export-12.mp4"},
		{long, 999999, "webm", long + "-999999.webm"},
		{"../x", 1, "webm", ""},
		{"a/b", 1, "webm", ""},
		{`a\b`, 1, "webm", ""},
		{"Cover", 1, "webm", ""},
		{"a:b", 1, "webm", ""},
		{"a.b", 1, "webm", ""},
		{"ab", 1, "webm", ""}, // короче трёх знаков
		{"-ab", 1, "webm", ""},
		{"", 1, "webm", ""},
		{long + "a", 1, "webm", ""}, // 41 знак
		{"cover", 0, "webm", ""},
		{"cover", 1_000_000, "webm", ""},
		{"cover", -1, "webm", ""},
		{"cover", 1, "WEBM", ""},
		{"cover", 1, "avi", ""},
		{"cover", 1, "", ""},
		{"con", 1, "webm", "con-1.webm"}, // с «-» и версией это не имя устройства
	} {
		got, ok := fileName(tc.id, tc.ver, tc.kind)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("fileName(%q, %d, %q) = %q, %v; ждали %q", tc.id, tc.ver, tc.kind, got, ok, tc.want)
		}
		if ok && len(got) > 52 {
			t.Errorf("имя %q длиннее 52", got)
		}
	}
}

// Разбор имени обратный: id — до последнего «-», число каноничное, вид из двух; прочее — не наше.
func TestParseName(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		ver  int
		kind string
		ok   bool
	}{
		{"cover-1.webm", "cover", 1, "webm", true},
		{"cover-export-3.mp4", "cover-export", 3, "mp4", true},
		{"cover-1-2.webm", "cover-1", 2, "webm", true},
		{"cover-01.webm", "", 0, "", false}, // не каноничное число
		{"cover-+1.webm", "", 0, "", false},
		{"cover-0.webm", "", 0, "", false},
		{"cover-1000000.webm", "", 0, "", false},
		{"cover-x.webm", "", 0, "", false},
		{"cover-1.WEBM", "", 0, "", false},
		{"cover-1.webm.tmp", "", 0, "", false},
		{"cover-1.webm.bak", "", 0, "", false},
		{"cover1.webm", "", 0, "", false},
		{"co-1.webm", "", 0, "", false}, // id короче трёх
		{"gv-123.tmp", "", 0, "", false},
		{"", "", 0, "", false},
		{"-1.webm", "", 0, "", false},
	} {
		id, ver, kind, ok := parseName(tc.name)
		if ok != tc.ok || id != tc.id || ver != tc.ver || kind != tc.kind {
			t.Errorf("parseName(%q) = %q %d %q %v; ждали %q %d %q %v", tc.name, id, ver, kind, ok, tc.id, tc.ver, tc.kind, tc.ok)
		}
	}
}

// Круг: имя из fileName разбирается назад в те же части.
func TestNameRoundTrip(t *testing.T) {
	for _, id := range []string{"cover", "cover-export", "a-b-c", "looks-5", "x12"} {
		for _, ver := range []int{1, 9, 10, 999999} {
			for _, kind := range []string{"webm", "mp4"} {
				name, ok := fileName(id, ver, kind)
				if !ok {
					t.Fatalf("fileName(%q, %d)", id, ver)
				}
				if gid, gver, gkind, ok := parseName(name); !ok || gid != id || gver != ver || gkind != kind {
					t.Errorf("%q разобрано как %q %d %q", name, gid, gver, gkind)
				}
			}
		}
	}
}

func video(id string, ver int, minApp string, kinds ...string) fetch.Video {
	v := fetch.Video{ID: id, Version: ver, MinApp: minApp, Seconds: 10, Files: map[string]fetch.VideoFile{}}
	for _, k := range kinds {
		v.Files[k] = fetch.VideoFile{Size: 10, SHA256: strings.Repeat("a", 64), URL: "https://x/" + k}
	}
	return v
}

// Выбор: самая новая версия с minApp не выше программы; darwin — mp4, прочие — webm, нужного нет — другой.
func TestPickFormat(t *testing.T) {
	cat := fetch.Catalog{Videos: []fetch.Video{
		video("both", 1, "0.30.0", "webm", "mp4"),
		video("onlymp4", 1, "0.30.0", "mp4"),
		video("onlywebm", 1, "0.30.0", "webm"),
		video("rolled", 1, "0.30.0", "webm"),
		video("rolled", 3, "0.30.0", "webm"),
		video("rolled", 2, "0.30.0", "webm"),
		video("newer", 2, "0.31.0", "webm"),
		video("newer", 1, "0.30.0", "webm"),
		video("future", 1, "0.40.0", "webm"),
		video("badid", 1, "0.30.0", "webm"),
	}}
	cat.Videos[len(cat.Videos)-1].ID = "../x"
	for goos, want := range map[string]map[string]string{
		"linux":   {"both": "webm", "onlymp4": "mp4", "onlywebm": "webm"},
		"windows": {"both": "webm", "onlymp4": "mp4", "onlywebm": "webm"},
		"darwin":  {"both": "mp4", "onlymp4": "mp4", "onlywebm": "webm"},
	} {
		got := pick(cat, "0.30.0", goos)
		for id, kind := range want {
			if got[id].kind != kind {
				t.Errorf("%s/%s: вид %q, ждали %q", goos, id, got[id].kind, kind)
			}
		}
		if got["rolled"].video.Version != 3 {
			t.Errorf("%s: переснятый ролик — версия %d, ждали 3", goos, got["rolled"].video.Version)
		}
		if got["newer"].video.Version != 1 {
			t.Errorf("%s: ролик новее программы взят: версия %d, ждали 1 (у старой программы свой)", goos, got["newer"].video.Version)
		}
		if _, ok := got["future"]; ok {
			t.Errorf("%s: ролик только для новой программы взят", goos)
		}
		if _, ok := got["../x"]; ok || len(got) != 5 {
			t.Errorf("%s: выбрано %d роликов, ждали 5", goos, len(got))
		}
	}
	// Пустой каталог и dev-версия: nil-безопасно, minApp dev-сборки не мешает.
	if got := pick(fetch.Catalog{}, "0.30.0", "linux"); len(got) != 0 {
		t.Errorf("пустой каталог: %v", got)
	}
	if got := pick(cat, "dev", "linux"); len(got) != 6 {
		t.Errorf("сборка dev берёт все: %d", len(got))
	}
}

// New паникует с именем пустого поля Host; полный Host проходит.
func TestNewPanics(t *testing.T) {
	testkit.NewPanics(t, newLab(t).host, func(h Host) { New(h) }, "guidevideo.New")
}
