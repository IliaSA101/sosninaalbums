package decor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installZip ставит набор с размером zip из каталога (zip < 0 — размер не передан).
func installZip(t *testing.T, s *Store, id string, zip int64, origin string, items ...pit) PackInfo {
	t.Helper()
	m, src := testPack(id, 1, items...)
	st, err := s.StagePack(m, src, origin, strings.Repeat("ab", 32))
	if err != nil || st.Ask != "" {
		t.Fatalf("StagePack %s: %v %+v", id, err, st)
	}
	defer st.Close()
	if zip >= 0 {
		st.SetZipSize(zip)
	}
	info, err := s.InstallPack(st, nil)
	if err != nil {
		t.Fatalf("InstallPack %s: %v", id, err)
	}
	return info
}

func packBytes(s *Store, id string) int64 {
	for _, p := range s.Packs() {
		if p.ID == id {
			return p.Bytes
		}
	}
	return -1
}

// 137-31: набор из сети отдаёт размер zip, прочие — сумму предметов; круг запись → чтение → перезапуск;
// «Освободится» остаётся суммой файлов на диске.
func TestPackZipSizeBytes(t *testing.T) {
	s, cfg := openTemp(t)
	a, b := renderPNG(t, 71, 30), renderPNG(t, 72, 30)
	sum := int64(len(a) + len(b))
	zipNet := sum + 777 // zip больше суммы: манифест и заголовки
	info := installZip(t, s, "net-one", zipNet, OriginNet, pit{"bg/a.png", a}, pit{"bg/b.png", b})
	installZip(t, s, "net-two", 12345, OriginNet, pit{"bg/c.png", renderPNG(t, 73, 30)})
	installZip(t, s, "from-file", -1, OriginFile, pit{"bg/d.png", renderPNG(t, 74, 30)})
	installZip(t, s, "net-nosize", -1, OriginNet, pit{"bg/e.png", renderPNG(t, 75, 30)})
	if info.Bytes != zipNet {
		t.Errorf("ответ установки %d, ждали %d", info.Bytes, zipNet)
	}
	check := func(when string, st *Store) {
		t.Helper()
		if got := packBytes(st, "net-one"); got != zipNet {
			t.Errorf("%s: net-one %d, ждали %d", when, got, zipNet)
		}
		if got := packBytes(st, "net-two"); got != 12345 {
			t.Errorf("%s: net-two %d", when, got)
		}
		for _, id := range []string{"from-file", "net-nosize"} { // размера нет — сумма предметов
			if got := packBytes(st, id); got <= 0 || got == zipNet {
				t.Errorf("%s: %s %d, ждали сумму предметов", when, id, got)
			}
		}
	}
	check("сразу", s)
	check("после перезапуска", Open(cfg))
	raw, err := os.ReadFile(filepath.Join(cfg, RootName, packsDir, "net-one.json"))
	if err != nil || !strings.Contains(string(raw), `"zipSize"`) {
		t.Errorf("реестр без zipSize: %v %s", err, raw)
	}
	if raw, _ := os.ReadFile(filepath.Join(cfg, RootName, packsDir, "from-file.json")); strings.Contains(string(raw), "zipSize") {
		t.Errorf("zipSize у набора без размера: %s", raw)
	}
	if got := s.PackDiskBytes("net-one"); got != sum {
		t.Errorf("PackDiskBytes %d, ждали %d", got, sum)
	}
	if got := s.PackDiskBytes("нет-такого"); got != 0 {
		t.Errorf("PackDiskBytes чужого %d", got)
	}
	plan, err := s.PlanRemovePack("net-one", nil)
	if err != nil || plan.Bytes != sum {
		t.Errorf("освободится %d (%v), ждали сумму файлов %d", plan.Bytes, err, sum)
	}
}

// Обновление набора берёт размер новой версии; обновление «из файла» (размера нет) возвращает сумму предметов.
func TestPackZipSizeUpdate(t *testing.T) {
	s, _ := openTemp(t)
	a := renderPNG(t, 81, 30)
	installZip(t, s, "upd-pack", 5000, OriginNet, pit{"bg/a.png", a})
	m, src := testPack("upd-pack", 2, pit{"bg/a.png", a})
	st, err := s.StagePack(m, src, OriginNet, strings.Repeat("cd", 32))
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("StagePack: %v %+v", err, st)
	}
	st.SetZipSize(6000)
	_, err = s.InstallPack(st, nil)
	st.Close()
	if err != nil || packBytes(s, "upd-pack") != 6000 {
		t.Fatalf("после обновления %d: %v", packBytes(s, "upd-pack"), err)
	}
	m, src = testPack("upd-pack", 3, pit{"bg/a.png", a})
	st, err = s.StagePack(m, src, OriginFile, strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.InstallPack(st, nil)
	st.Close()
	if err != nil || packBytes(s, "upd-pack") != int64(len(a)) {
		t.Fatalf("после файла %d, ждали %d: %v", packBytes(s, "upd-pack"), len(a), err)
	}
}

// Реестр: без поля — сумма предметов; с полем — оно; мусор (минус, огромное) зажимается; незнакомые
// поля пропускаются (читатель старее/новее); поле не числом — битый реестр, остальные наборы целы.
func TestPackZipSizeRegistry(t *testing.T) {
	_, cfg := openTemp(t)
	dir := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := `"items":[{"key":"my:0123456789abcdef","kind":"bg","theme":"basic","name":"a","bytes":1000},{"key":"my:fedcba9876543210","kind":"bg","theme":"basic","name":"b","bytes":500}]`
	files := map[string]string{
		"old-pack.json":  `{"v":1,"id":"old-pack","version":1,"origin":"net",` + item + `}`,
		"with-pack.json": `{"v":1,"id":"with-pack","version":1,"origin":"net","zipSize":1700,"future":{"x":1},` + item + `}`,
		"zero-pack.json": `{"v":1,"id":"zero-pack","version":1,"origin":"net","zipSize":0,` + item + `}`,
		"neg-pack.json":  `{"v":1,"id":"neg-pack","version":1,"origin":"net","zipSize":-5,` + item + `}`,
		"huge-pack.json": `{"v":1,"id":"huge-pack","version":1,"origin":"net","zipSize":9000000000000000000,` + item + `}`,
		"none-pack.json": `{"v":1,"id":"none-pack","version":1,"origin":"net","zipSize":0,"items":[]}`,
		"str-pack.json":  `{"v":1,"id":"str-pack","version":1,"zipSize":"много","items":[]}`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := Open(cfg)
	want := map[string]int64{"old-pack": 1500, "with-pack": 1700, "zero-pack": 1500, "neg-pack": 1500, "huge-pack": packZipMax, "none-pack": 0, "str-pack": -1}
	for id, w := range want {
		if got := packBytes(s, id); got != w {
			t.Errorf("%s: %d, ждали %d", id, got, w)
		}
	}
}
