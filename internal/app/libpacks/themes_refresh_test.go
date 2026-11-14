package libpacks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Перекачка тем для наборов, поставленных до 0.40 (спека 170-9, 170-19): решение — в плане, дальше
// StagePack/InstallPack ведут её без пересмотра, tv пишется всегда.

// объявления тем для строки каталога (клиенту нужно только их число)
const declsRow = `[{"id":"newyear","name":"Новый год","base":"holiday"},{"id":"snow","name":"Снег","base":"basic"}]`

// rejectedDecl — манифест с объявлением, которое 0.40 отвергает (служебный id): принятых тем нет.
var rejectedDecl = []map[string]any{{"id": "recent", "name": "Служебная", "base": "holiday", "order": 1}}

// as039 — реестры наборов ids глазами 0.39: без tv и themes; службе отдаётся заново открытая
// библиотека (реестры читаются при Open).
func (e *demoEnv) as039(ids ...string) {
	e.t.Helper()
	for _, id := range ids {
		path := filepath.Join(e.dir, decor.RootName, "Наборы", id+".json") // packsDir — папка реестров наборов
		data, err := os.ReadFile(path)
		if err != nil {
			e.t.Fatal(err)
		}
		var reg map[string]json.RawMessage
		if err := json.Unmarshal(data, &reg); err != nil {
			e.t.Fatal(err)
		}
		if _, ok := reg["tv"]; !ok {
			e.t.Fatalf("%s: в реестре нет tv", id)
		}
		delete(reg, "tv")
		delete(reg, "themes")
		if data, err = json.Marshal(reg); err != nil {
			e.t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	e.reopen()
}

// reopen — библиотека открывается заново (перезапуск программы).
func (e *demoEnv) reopen() {
	e.t.Helper()
	st := decor.Open(e.dir)
	e.svc.h.Store = func() (*decor.Store, error) { return st, nil }
	e.st = st
}

// themesRead — реестр набора id писала программа с темами (tv ≥ 1).
func (e *demoEnv) themesRead(id string) bool {
	e.t.Helper()
	for _, p := range e.st.Packs() {
		if p.ID == id {
			return p.ThemesRead
		}
	}
	e.t.Fatalf("%s не стоит", id)
	return false
}

// Условия плана: та же версия берётся только из сети, при реестре без tv и объявлениях в строке,
// если программа набору не старая; остальное — как раньше (без пропусков с причиной).
func TestThemesRefreshPlan(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.srv.publish(row{id: "net-old", version: 1, zip: packZip(t, "net-old", 1, 31)}, row{id: "net-new", version: 1, zip: packZip(t, "net-new", 1, 32)})
	for _, id := range []string{"net-old", "net-new"} {
		if res, err := e.svc.Download(id); err != nil || !res.Picked {
			t.Fatalf("Download %s: %v %+v", id, err, res)
		}
	}
	e.addFile(packZip(t, "file-old", 1, 33), "file-old")
	e.as039("net-old", "file-old") // net-new остаётся с tv
	if !e.themesRead("net-new") || e.themesRead("net-old") || e.themesRead("file-old") {
		t.Fatalf("исходные реестры: %v %v %v", e.themesRead("net-new"), e.themesRead("net-old"), e.themesRead("file-old"))
	}
	ent := func(id string, ver, themes int, minApp string) fetch.Entry {
		return fetch.Entry{ID: id, Version: ver, MinApp: minApp, Title: id, Size: 10, Themes: themes}
	}
	cases := []struct {
		name string
		e    fetch.Entry
		skip []string
		want bool
	}{
		{"сеть, та же версия, нет tv, темы в строке", ent("net-old", 1, 2, "0.1.0"), nil, true},
		{"предел тем", ent("net-old", 1, 16, "0.1.0"), nil, true},
		{"в строке нет тем", ent("net-old", 1, 0, "0.1.0"), nil, false},
		{"tv есть", ent("net-new", 1, 2, "0.1.0"), nil, false},
		{"стоит из файла", ent("file-old", 1, 2, "0.1.0"), nil, false},
		{"программа набору старая", ent("net-old", 1, 2, "99.0.0"), nil, false},
		{"в пропуске «удалён вами»", ent("net-old", 1, 2, "0.1.0"), []string{"net-old"}, false},
		{"в каталоге версия старее стоящей", ent("net-old", 0, 2, "0.1.0"), nil, false},
		{"обычное обновление без тем", ent("net-old", 2, 0, "0.1.0"), nil, true},
		{"обычное обновление с tv", ent("net-new", 2, 0, "0.1.0"), nil, true},
		{"нового набора с темами строка — обычная установка", ent("fresh", 1, 2, "0.1.0"), nil, true},
	}
	for _, c := range cases {
		prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.Skip = c.skip })
		cat := fetch.Catalog{Packs: []fetch.Entry{c.e}}
		var res Update
		todo, need := e.svc.netPlan(e.st, cat, wantAll(cat), &res)
		if got := len(todo) == 1 && todo[0].ID == c.e.ID; got != c.want || need != map[bool]int64{true: 10}[c.want] {
			t.Errorf("%s: в плане %v (%d байт), ждали %v", c.name, todo, need, c.want)
		}
		if len(res.Skipped) != 0 {
			t.Errorf("%s: пропуск с причиной: %+v", c.name, res.Skipped)
		}
	}
}

// Ни одно объявление не принято (служебный id) — всё равно одна перекачка: два хода «Обновить» и
// «после перезапуска» качают архив один раз; tv записан.
func TestThemesRefreshOnce(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.srv.publish(row{id: "ny-set", version: 1, zip: packZip(t, "ny-set", 1, 41)}) // как 0.39: без тем
	if res, err := e.svc.Download("ny-set"); err != nil || res.Added != 1 || e.srv.assetHits.Load() != 1 {
		t.Fatalf("первая установка: %v %+v", err, res)
	}
	e.as039("ny-set")
	if e.themesRead("ny-set") {
		t.Fatal("tv не снят")
	}
	// Каталог 0.40: строка объявляет темы, а манифест объявление отвергает; архив — другой.
	e.srv.publish(row{id: "ny-set", version: 1, zip: packZipThemes(t, "ny-set", 1, rejectedDecl, 41), themes: declsRow})
	res, err := e.svc.Update()
	if err != nil || res.Updated != 1 || res.Added != 0 || len(res.Failed) != 0 || e.srv.assetHits.Load() != 2 {
		t.Fatalf("перекачка: %v %+v, скачиваний %d\n%s", err, res, e.srv.assetHits.Load(), e.log.String())
	}
	if !e.themesRead("ny-set") {
		t.Fatal("после перекачки tv не записан")
	}
	if !strings.Contains(e.log.String(), `набор "ny-set": та же версия 1, другой архив`) {
		t.Errorf("нет строки о другом архиве:\n%s", e.log.String())
	}
	for when, reopen := range map[string]bool{"второй ход": false, "после перезапуска": true} {
		if reopen {
			e.reopen()
		}
		e.srv.publish(row{id: "ny-set", version: 1, zip: packZipThemes(t, "ny-set", 1, rejectedDecl, 41), themes: declsRow})
		if res, err := e.svc.Update(); err != nil || res.Updated != 0 || res.Added != 0 || e.srv.assetHits.Load() != 2 {
			t.Errorf("%s: %v %+v, скачиваний %d", when, err, res, e.srv.assetHits.Load())
		}
	}
}

// Набор уже с tv — «Обновить» его не перекачивает, как бы ни была полна строка.
func TestThemesRefreshIdle(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.srv.publish(row{id: "ny-set", version: 1, zip: packZipThemes(t, "ny-set", 1, nil, 42), themes: declsRow})
	if res, err := e.svc.Download("ny-set"); err != nil || res.Added != 1 || !e.themesRead("ny-set") {
		t.Fatalf("установка: %v %+v", err, res)
	}
	for range 2 {
		e.srv.publish(row{id: "ny-set", version: 1, zip: packZipThemes(t, "ny-set", 1, nil, 42), themes: declsRow})
		if res, err := e.svc.Update(); err != nil || res.Updated+res.Added != 0 || e.srv.assetHits.Load() != 1 {
			t.Fatalf("лишнее скачивание: %v %+v, скачиваний %d", err, res, e.srv.assetHits.Load())
		}
		e.reopen()
	}
}
