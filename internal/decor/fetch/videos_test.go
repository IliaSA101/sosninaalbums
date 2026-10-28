package fetch

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func videoRowMap(src Source, id string, ver int) map[string]any {
	return map[string]any{"id": id, "version": ver, "minApp": "0.30.0", "seconds": 48,
		"files": map[string]any{
			"webm": map[string]any{"size": 476000, "sha256": strings.Repeat("a", 64), "url": src.Prefix + "videos-1/" + id + "-" + strconv.Itoa(ver) + ".webm"},
			"mp4":  map[string]any{"size": 418000, "sha256": strings.Repeat("b", 64), "url": src.Prefix + "videos-1/" + id + "-" + strconv.Itoa(ver) + ".mp4"},
		}}
}

func videosBody(t *testing.T, rows any, packs ...map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"format": 1, "seq": 3, "packs": packs, "videos": rows})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func skipIDs(c Catalog) string {
	var ids []string
	for _, s := range c.Skipped {
		ids = append(ids, s.ID+": "+s.Why)
	}
	return strings.Join(ids, "; ")
}

// videos: годные строки берутся, плохие пропускаются с причиной; каталог без videos — как раньше.
func TestParseCatalogVideos(t *testing.T) {
	l := newTestLib(t)
	src := l.source()
	good := videoRowMap(src, "autobuild", 1)
	mod := func(f func(m map[string]any)) map[string]any {
		m := videoRowMap(src, "cover", 1)
		f(m)
		return m
	}
	file := func(m map[string]any, kind string) map[string]any {
		return m["files"].(map[string]any)[kind].(map[string]any)
	}
	rows := []any{
		good,
		mod(func(m map[string]any) { m["id"] = "Cover" }),
		mod(func(m map[string]any) { m["id"] = "ab" }),
		mod(func(m map[string]any) { m["version"] = 0 }),
		mod(func(m map[string]any) { m["version"] = 1_000_000 }),
		mod(func(m map[string]any) { m["minApp"] = "0.30" }),
		mod(func(m map[string]any) { m["seconds"] = 0 }),
		mod(func(m map[string]any) { m["seconds"] = 601 }),
		mod(func(m map[string]any) { m["files"] = map[string]any{} }),
		mod(func(m map[string]any) { m["files"] = map[string]any{"WEBM": file(m, "webm"), "avi": file(m, "webm")} }),
		mod(func(m map[string]any) { file(m, "webm")["size"] = 0 }),
		mod(func(m map[string]any) { file(m, "webm")["size"] = MaxVideoFile + 1 }),
		mod(func(m map[string]any) { file(m, "mp4")["sha256"] = "zz" }),
		mod(func(m map[string]any) { file(m, "webm")["url"] = "https://evil.example/x.webm" }),
		mod(func(m map[string]any) { file(m, "webm")["url"] = src.Prefix + "videos-1/../x.webm" }),
		mod(func(m map[string]any) { file(m, "webm")["url"] = src.Prefix + "videos-1/a%2fb.webm" }),
		videoRowMap(src, "autobuild", 1), // повтор пары
		// предел размера включительно, один вид, лишний вид игнорируется
		mod(func(m map[string]any) {
			m["id"] = "okedge"
			m["files"] = map[string]any{"mp4": map[string]any{"size": MaxVideoFile, "sha256": strings.Repeat("c", 64), "url": src.Prefix + "videos-1/okedge-1.mp4"},
				"av1": map[string]any{"size": 1}}
		}),
		videoRowMap(src, "autobuild", 2),
	}
	cat, err := ParseCatalog(videosBody(t, rows), src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range cat.Videos {
		got = append(got, v.ID+"/"+strconv.Itoa(v.Version)+"/"+strconv.Itoa(len(v.Files)))
	}
	if want := "autobuild/1/2 okedge/1/1 autobuild/2/2"; strings.Join(got, " ") != want {
		t.Errorf("роликов %v, ждали %s; пропущено: %s", got, want, skipIDs(cat))
	}
	if len(cat.Skipped) != len(rows)-3 {
		t.Errorf("пропущено %d, ждали %d: %s", len(cat.Skipped), len(rows)-3, skipIDs(cat))
	}
	for _, s := range cat.Skipped {
		if s.Why == "" || strings.Contains(s.Why, "http") {
			t.Errorf("причина %+v", s)
		}
	}
	// Каталог без videos — Videos пуст, не nil; наборы как прежде.
	old, err := ParseCatalog([]byte(`{"format":1,"seq":1,"packs":[]}`), src)
	if err != nil || old.Videos == nil || len(old.Videos) != 0 || len(old.Skipped) != 0 {
		t.Errorf("без videos: %+v, %v", old, err)
	}
	for _, null := range []string{`"videos":null`, `"videos":[]`} {
		c, err := ParseCatalog([]byte(`{"format":1,"seq":1,"packs":[],`+null+`}`), src)
		if err != nil || len(c.Videos) != 0 || len(c.Skipped) != 0 {
			t.Errorf("%s: %+v, %v", null, c, err)
		}
	}
}

// Битая строка videos не роняет каталог: набор на месте, строка в Skipped. То же — videos не список,
// строка не объект, лишние строки сверх MaxVideos.
func TestParseCatalogVideosBadRowKeepsPacks(t *testing.T) {
	l := newTestLib(t)
	src := l.source()
	pack := row("ivory", 1, src.Prefix+"ivory-v1/ivory-1.zip", []byte("zip"))
	over := make([]any, MaxVideos+1)
	for i := range over {
		over[i] = videoRowMap(src, "v"+strconv.Itoa(100+i), 1)
	}
	exact := over[:MaxVideos]
	for name, tc := range map[string]struct {
		rows   any
		videos int
		skips  int
	}{
		"версия строкой": {[]any{map[string]any{"id": "autobuild", "version": "2", "minApp": "0.30.0", "seconds": 5, "files": map[string]any{}}}, 0, 1},
		"files списком":  {[]any{map[string]any{"id": "autobuild", "version": 1, "minApp": "0.30.0", "seconds": 5, "files": []any{}}}, 0, 1},
		"строка числом":  {[]any{5, "x", nil, []any{}}, 0, 4},
		"не список":      {map[string]any{"id": "x"}, 0, 1},
		"строка":         {"videos", 0, 1},
		"число":          {42, 0, 1},
		"сверх предела":  {over, 0, 1},
		"ровно предел":   {exact, MaxVideos, 0},
		"битая и годная": {[]any{5, videoRowMap(src, "autobuild", 1)}, 1, 1},
	} {
		cat, err := ParseCatalog(videosBody(t, tc.rows, pack), src)
		if err != nil {
			t.Errorf("%s: каталог отвергнут: %v", name, err)
			continue
		}
		if len(cat.Packs) != 1 || cat.Packs[0].ID != "ivory" {
			t.Errorf("%s: наборы %+v", name, cat.Packs)
		}
		if len(cat.Videos) != tc.videos || len(cat.Skipped) != tc.skips {
			t.Errorf("%s: роликов %d (ждали %d), пропущено %d (ждали %d): %s", name, len(cat.Videos), tc.videos, len(cat.Skipped), tc.skips, skipIDs(cat))
		}
	}
	// Обрезанный JSON каталога целиком — по-прежнему ErrCatalog.
	if _, err := ParseCatalog([]byte(`{"format":1,"seq":1,"packs":[],"videos":[{"id":`), src); err == nil {
		t.Error("обрезанный каталог принят")
	}
}

// NeedsApp ролика и Entry одинаковы; ValidID — одна грамматика.
func TestVideoNeedsAppAndValidID(t *testing.T) {
	v := Video{MinApp: "0.30.0"}
	for app, want := range map[string]bool{"0.29.9": true, "0.30.0": false, "0.30.1": false, "1.0.0": false, "dev": false, "": false} {
		if v.NeedsApp(app) != want || (Entry{MinApp: "0.30.0"}).NeedsApp(app) != want {
			t.Errorf("NeedsApp(%q) != %v", app, want)
		}
	}
	for id, want := range map[string]bool{"cover": true, "a-b": true, "ab": false, "": false, "-ab": false, "Cover": false, "a_b": false, strings.Repeat("a", 40): true, strings.Repeat("a", 41): false} {
		if ValidID(id) != want {
			t.Errorf("ValidID(%q) != %v", id, want)
		}
	}
}

// Фаззинг разбора videos: любые байты — без паники.
func FuzzParseVideos(f *testing.F) {
	src := GitHub()
	f.Add([]byte(`[{"id":"cover","version":1,"minApp":"0.30.0","seconds":5,"files":{"webm":{"size":10,"sha256":"` + strings.Repeat("a", 64) + `","url":"` + AssetPrefix + `videos-1/c.webm"}}}]`))
	f.Add([]byte(`[`))
	f.Add([]byte(`[[],{},null,1,"x"]`))
	f.Add([]byte(`{"a":1}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		vs, _ := parseVideos(data, src)
		if len(vs) > MaxVideos {
			t.Fatalf("роликов %d", len(vs))
		}
		for _, v := range vs {
			if !ValidID(v.ID) || len(v.Files) == 0 {
				t.Fatalf("негодная строка принята: %+v", v)
			}
		}
	})
}
