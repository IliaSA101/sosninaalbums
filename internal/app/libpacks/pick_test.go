package libpacks

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// ent — строка каталога для тестов выбора: id, донор (needs), размер.
func ent(id, needs string, size int64) fetch.Entry {
	return fetch.Entry{ID: id, Version: 1, MinApp: "0.1.0", Title: "T-" + id, Needs: needs, Size: size,
		Counts: fetch.Counts{Bg: 2, Sticker: 1}}
}

func catOf(es ...fetch.Entry) fetch.Catalog { return fetch.Catalog{Seq: 1, Packs: es} }

func stands(ids ...string) map[string]decor.PackInfo {
	m := map[string]decor.PackInfo{}
	for _, id := range ids {
		m[id] = decor.PackInfo{ID: id, Title: "Стоит-" + id, Version: 1, Origin: decor.OriginNet}
	}
	return m
}

func ids(es []fetch.Entry) []string {
	out := []string{}
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

// Состав «Скачать»: донор с дополнениями; дополнение тянет нестоящего донора вверх; цикл needs
// не зацикливает; чужой id даёт только себя; стоящее, удалённое пользователем и «нужна новая
// программа» вниз не идут (кроме нажатого).
func TestPickClosure(t *testing.T) {
	cat := catOf(ent("don", "", 10), ent("add-a", "don", 5), ent("add-b", "don", 7), ent("loner", "", 3),
		ent("ca", "cb", 1), ent("cb", "ca", 1), ent("nest", "add-a", 2))
	ix := indexOf(cat)
	cases := []struct {
		name string
		id   string
		have map[string]decor.PackInfo
		skip []string
		want []string
	}{
		{"донор и все дополнения, включая вложенные", "don", nil, nil, []string{"don", "add-a", "add-b", "nest"}},
		{"дополнение тянет донора и его дополнения", "add-b", nil, nil, []string{"don", "add-a", "add-b", "nest"}},
		{"донор стоит — только дополнение", "add-b", stands("don"), nil, []string{"add-b"}},
		{"стоящее дополнение вниз не идёт", "don", stands("add-a"), nil, []string{"don", "add-b"}},
		{"удалённое вами дополнение вниз не идёт", "don", nil, []string{"add-a"}, []string{"don", "add-b"}},
		{"нажатое удалённое — идёт", "add-a", stands("don"), []string{"add-a"}, []string{"add-a", "nest"}},
		{"цикл needs", "ca", nil, nil, []string{"ca", "cb"}},
		{"одиночка", "loner", nil, nil, []string{"loner"}},
		{"чужой id", "нет-такого", nil, nil, []string{}},
	}
	for _, c := range cases {
		got := ix.closure(cat, c.id, c.have, c.skip, "0.29.0")
		if len(got.ents) != len(c.want) || len(c.want) > 0 && !slices.Equal(ids(got.ents), c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, ids(got.ents), c.want)
		}
	}
	// Дополнение, просящее программу новее, вниз не идёт (серая кнопка у своей строки).
	late := ent("late", "don", 9)
	late.MinApp = "9.0.0"
	cat2 := catOf(ent("don", "", 10), late)
	if got := indexOf(cat2).closure(cat2, "don", nil, nil, "0.29.0"); !slices.Equal(ids(got.ents), []string{"don"}) {
		t.Errorf("minApp вниз: %v", ids(got.ents))
	}
}

// Строки «Можно скачать»: порядок каталога; стоящие без строки; дополнение к стоящему донору —
// строка с donor, к нестоящему — нет; размер и названия дополнений — по составу; why, removed.
func TestPackListAvailable(t *testing.T) {
	late := ent("late", "", 4)
	late.MinApp = "9.0.0"
	cat := dropDevices(catOf(ent("have-it", "", 1), ent("don", "", 10), ent("add-a", "don", 5), ent("gone", "", 2), late,
		ent("add-st", "have-it", 3), ent("orph", "нет-донора", 1), ent("con", "", 1)))
	rows := availRows(cat, stands("have-it"), []string{"gone"}, "0.29.0")
	var got []string
	for _, r := range rows {
		got = append(got, r.ID)
	}
	if want := []string{"don", "gone", "late", "add-st"}; !slices.Equal(got, want) {
		t.Fatalf("строки %v, ждали %v (стоящий, дополнение нестоящего, сирота, «con» — без строки)", got, want)
	}
	byID := map[string]Avail{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if d := byID["don"]; d.Bytes != 15 || len(d.AddonTitles) != 1 || d.AddonTitles[0] != "T-add-a" || d.Donor != "" || d.Why != "" || d.Removed {
		t.Errorf("донор: %+v", d)
	}
	if g := byID["gone"]; !g.Removed || g.Why != "" || g.Bytes != 2 || g.AddonTitles == nil {
		t.Errorf("удалён вами: %+v", g)
	}
	if l := byID["late"]; l.Why != "нужна версия Sosnina Albums 9.0.0 или новее" || l.Removed {
		t.Errorf("новая программа: %+v", l)
	}
	if a := byID["add-st"]; a.Donor != "Стоит-have-it" || a.Bytes != 3 {
		t.Errorf("дополнение к стоящему: %+v", a)
	}
	if r := byID["don"]; r.Bg != 2 || r.Sticker != 1 || r.Version != 1 {
		t.Errorf("счётчики: %+v", r)
	}
	// Пустой список и пустой каталог — пустой срез, не nil.
	if r := availRows(catOf(), nil, nil, "0.29.0"); r == nil || len(r) != 0 {
		t.Errorf("пустой каталог: %#v", r)
	}
}

// «Можно скачать все» считает только строки без серой кнопки и без «удалён вами»
// (критика Д4): число и байты тоста совпадают с тем, что поставится.
func TestAvailableCount(t *testing.T) {
	late := ent("late", "", 4)
	late.MinApp = "9.0.0"
	cat := catOf(ent("don", "", 10), ent("add-a", "don", 5), ent("gone", "", 2), late, ent("plain", "", 1))
	rows := availRows(cat, nil, []string{"gone"}, "0.29.0")
	if len(rows) != 4 { // don, gone, late, plain: add-a идёт с донором
		t.Fatalf("строк %d", len(rows))
	}
	dl := downloadable(rows)
	if len(dl) != 2 || bytesOf(dl) != 16 {
		t.Errorf("к скачиванию %d, байт %d (ждали don 15 + plain 1)", len(dl), bytesOf(dl))
	}
	if len(downloadable(nil)) != 0 || bytesOf(nil) != 0 {
		t.Error("пусто")
	}
}

// updateWant: стоящие и дополнения к стоящим донорам; новые наборы и дополнения к нестоящим — нет.
func TestUpdateWant(t *testing.T) {
	cat := catOf(ent("old", "", 1), ent("new", "", 1), ent("add-old", "old", 1), ent("add-new", "new", 1))
	got := updateWant(cat, stands("old"))
	for id, want := range map[string]bool{"old": true, "add-old": true, "new": false, "add-new": false} {
		if got[id] != want {
			t.Errorf("%s: %v, ждали %v", id, got[id], want)
		}
	}
}

// shownPlan: подпись «Скачать все» и план — одна функция; серые и «удалён вами» не входят,
// байты — по составу без двойного счёта.
func TestShownPlan(t *testing.T) {
	late := ent("late", "", 4)
	late.MinApp = "9.0.0"
	cat := catOf(ent("don", "", 10), ent("add-a", "don", 5), ent("gone", "", 2), late, ent("plain", "", 1))
	got, all := shownPlan(cat, nil, []string{"gone"}, "0.29.0")
	if all.Count != 2 || all.Bytes != 16 || len(got) != 3 || !got["don"] || !got["add-a"] || !got["plain"] || got["gone"] || got["late"] {
		t.Errorf("план: %v %+v", got, all)
	}
	if ids, all := shownPlan(catOf(), nil, nil, "0.29.0"); len(ids) != 0 || all.Count != 0 {
		t.Errorf("пусто: %v %+v", ids, all)
	}
}
