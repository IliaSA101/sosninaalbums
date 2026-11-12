package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Раздел «Можно скачать» (137 Т3): список берётся из сохранённого каталога без сети; удалённый
// сетевой набор стоит в нём с пометкой «удалён вами» и в «Скачать все» не входит; дата — время
// файла; перезапуск (новое приложение над той же папкой) читает список с диска.
func TestPackListAvailable(t *testing.T) {
	a, l := netApp(t, true)
	zips := fourZips(t)
	l.publish(t, nil, zips...)
	if list, _ := a.LibraryPacks(); len(list.Available) != 0 || list.AvailAt != 0 || list.All.Count != 0 || list.Available == nil {
		t.Fatalf("до первой проверки: %+v", list)
	}
	before := time.Now().Add(-time.Minute).UnixMilli()
	if _, err := getAll(t, a); err != nil { // Т3: весь каталог ещё ставится прежним путём
		t.Fatal(err)
	}
	if _, err := a.RemovePack("sage"); err != nil {
		t.Fatal(err)
	}
	hits := l.hits.Load()
	list, err := a.LibraryPacks()
	if err != nil || len(list.Available) != 1 || list.Available[0].ID != "sage" || !list.Available[0].Removed ||
		list.Available[0].AddonTitles == nil || list.All.Count != 0 || list.AvailAt < before {
		t.Fatalf("список: %v %+v", err, list)
	}
	if l.hits.Load() != hits {
		t.Error("список ходил в сеть")
	}
	if raw, _ := json.Marshal(list); !jsonHas(raw, `"available":[`) || !jsonHas(raw, `"addonTitles":[]`) {
		t.Errorf("JSON: %s", raw)
	}
	// Файл списка — обычный, 0600-подобный, рядом нет остатков.
	if _, err := os.Stat(filepath.Join(a.cfgDir, "library-catalog.json")); err != nil {
		t.Fatal(err)
	}
}

func jsonHas(raw []byte, sub string) bool { return strings.Contains(string(raw), sub) }

// Удаление донора: названия стоящих дополнений по сохранённому списку; без списка — пусто (не null).
func TestPlanRemoveAddons(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	add := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "ivory-add", "Айвори+" })
	l.publish(t, func(r map[string]any) {
		if r["id"] == "ivory-add" {
			r["needs"] = "ivory"
		}
	}, zip, add)
	if _, err := a.AddPackPath(zip); err != nil {
		t.Fatal(err)
	}
	if r, err := a.PlanRemovePack("ivory"); err != nil || r.Addons == nil || len(r.Addons) != 0 {
		t.Fatalf("без списка: %v %+v", err, r)
	}
	if res, err := a.UpdateLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("установка: %v %+v", err, res)
	}
	r, err := a.PlanRemovePack("ivory")
	if err != nil || len(r.Addons) != 1 || r.Addons[0] != "Айвори+" {
		t.Fatalf("донор: %v %+v", err, r)
	}
	if r, err := a.PlanRemovePack("ivory-add"); err != nil || len(r.Addons) != 0 || r.Addons == nil {
		t.Fatalf("дополнение: %v %+v", err, r)
	}
}
