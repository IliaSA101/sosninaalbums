package prefs

import (
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 113-1: pending → done; негодный id или state — карта как была; предел maxTours; чужие ключи целы.
func TestSetTourState(t *testing.T) {
	m := SetTourState(nil, "first", "pending")
	if len(m) != 1 || m["first"] != "pending" {
		t.Fatalf("первая запись: %v", m)
	}
	m2 := SetTourState(m, "first", "done")
	if m2["first"] != "done" || m["first"] != "pending" {
		t.Fatalf("pending → done: %v, исходная %v (должна остаться прежней)", m2, m)
	}
	// Негодное — без изменений; nil остаётся nil (в config.json ключ не появится).
	for _, c := range [][2]string{
		{"", "done"}, {"first", ""}, {"first", "later"}, {"first", "Done"}, {"bad id", "done"},
		{"first1", "done"}, {"первый", "done"}, {strings.Repeat("a", 33), "done"},
	} {
		if got := SetTourState(nil, c[0], c[1]); got != nil {
			t.Errorf("SetTourState(nil, %q, %q) = %v", c[0], c[1], got)
		}
		if got := SetTourState(m2, c[0], c[1]); len(got) != 1 || got["first"] != "done" {
			t.Errorf("SetTourState(%q, %q) изменил карту: %v", c[0], c[1], got)
		}
	}
	// Граница длины id: 32 буквы годятся.
	if got := SetTourState(nil, strings.Repeat("a", 32), "done"); len(got) != 1 {
		t.Errorf("id из 32 букв: %v", got)
	}
	// Устаревший клиент не откатывает пройденный тур.
	if got := SetTourState(m2, "first", "pending"); got["first"] != "done" {
		t.Errorf("done откатился в pending: %v", got)
	}
	// Несколько id сразу; чужие id и чужие значения остаются.
	m3 := SetTourState(SetTourState(map[string]string{"future": "later"}, "first", "done"), "spread", "pending")
	if len(m3) != 3 || m3["future"] != "later" || m3["first"] != "done" || m3["spread"] != "pending" {
		t.Errorf("несколько id: %v", m3)
	}
	// Предел: восемь ключей — новый не добавляется, существующий меняется.
	full := map[string]string{}
	for i := range maxTours {
		full[fmt.Sprintf("t%c", 'a'+i)] = "pending"
	}
	if got := SetTourState(full, "extra", "done"); len(got) != maxTours || got["extra"] != "" {
		t.Errorf("сверх предела: %v", got)
	}
	if got := SetTourState(full, "ta", "done"); len(got) != maxTours || got["ta"] != "done" {
		t.Errorf("существующий при полной карте: %v", got)
	}
}

// 113-2: флаги TourDue/TourMore по таблице.
func TestPrefsTourFlags(t *testing.T) {
	cases := []struct {
		name      string
		level     string
		tours     map[string]string
		due, more bool
	}{
		{"первый запуск", "", nil, true, false},
		{"первый запуск, пустая карта", "", map[string]string{}, true, false},
		{"обновился: уровень есть, тура нет", "full", nil, false, false},
		{"закрыли посреди тура", "full", map[string]string{"first": "pending"}, true, false},
		{"закрыли посреди тура, уровня нет", "", map[string]string{"first": "pending"}, true, false},
		{"прошли", "full", map[string]string{"first": "done"}, false, true},
		{"прошли, уровня нет (файл правили)", "", map[string]string{"first": "done"}, false, true},
		{"прошли и продолжение", "full", map[string]string{"first": "done", "spread": "done"}, false, false},
		{"продолжение в процессе", "full", map[string]string{"first": "done", "spread": "pending"}, false, true},
		{"spread без first", "full", map[string]string{"spread": "done"}, false, false},
		{"чужое значение — как нет", "full", map[string]string{"first": "later"}, false, false},
		{"чужое значение на первом запуске", "", map[string]string{"first": "later"}, true, false},
		{"чужой id не мешает", "full", map[string]string{"future": "done", "first": "done"}, false, true},
		{"негодный id", "full", map[string]string{"bad id": "pending", "first": "done", "spread": "done"}, false, false},
	}
	for _, c := range cases {
		p := Of(Config{Level: c.level, Tours: c.tours})
		if p.TourDue != c.due || p.TourMore != c.more {
			t.Errorf("%s: TourDue=%v TourMore=%v, ждали %v %v", c.name, p.TourDue, p.TourMore, c.due, c.more)
		}
	}
	// Флаги сравнимы: Prefs остаётся структурой без срезов и карт.
	if Of(Config{Level: "full", Tours: map[string]string{"first": "done"}}) == Of(Config{Level: "full"}) {
		t.Error("флаги не попали в Prefs")
	}
}

// 113-3: чужие id (и чужие значения) в tours переживают правку другого поля и SetTourState;
// пустая карта в файл не пишется.
func TestConfigKeepsUnknownTours(t *testing.T) {
	dir := testkit.TempDir(t)
	raw := `{"level":"full","tours":{"future":"later","first":"pending"}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	UpdateConfig(dir, func(c *Config) { c.HideStart = true })
	UpdateConfig(dir, func(c *Config) { c.Tours = SetTourState(c.Tours, "first", "done") })
	want := map[string]string{"future": "later", "first": "done"}
	if got := LoadConfig(dir).Tours; !maps.Equal(got, want) {
		t.Errorf("tours после записи: %v, ждали %v", got, want)
	}
	// Круг Save → Load: тот же файл читается тем же значением.
	if p := Of(LoadConfig(dir)); p.TourDue || !p.TourMore || p.Level != LevelFull {
		t.Errorf("Prefs после круга: %+v", p)
	}
	UpdateConfig(dir, func(c *Config) { c.Tours = nil })
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), "tours") {
		t.Errorf("пустые tours остались в файле: %s", data)
	}
}
