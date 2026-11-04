package salbum

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestSalbumFabrikaPreset (081): пресет обложки Фабрики и страница 30×30 переживают Save → Load →
// Save (тот же JSON, числа и id на месте); формат файла не менялся — id пресета строка, числа
// истина: id, которого эта версия не знает (старая программа / будущий пресет), открывается как
// «Свои размеры» с теми же числами, а не теряет обложку.
func TestSalbumFabrikaPreset(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe, p.Settings.Bleed = 300, 305, 10, 3
	for _, id := range []string{"fabrika-photo", "fabrika-photo-2", "fabrika-layflat-card-1"} {
		pr, _ := model.CoverPresetFor(id, p.Settings)
		p.Cover.Spec = pr.Spec
		path := filepath.Join(dir, id+".salbum")
		first, q := saveLoad(t, p, path)
		if !reflect.DeepEqual(q.Cover.Spec, pr.Spec) || q.Settings.PageW != 300 || q.Settings.Bleed != 3 {
			t.Fatalf("%s: после Save → Load %+v, страница %+v", id, q.Cover.Spec, q.Settings)
		}
		if second, _ := saveLoad(t, q, path); !bytes.Equal(first, second) {
			t.Errorf("%s: Save → Load → Save дал другой JSON", id)
		}
		if got := model.CoverFixText(q); got != "" {
			t.Errorf("%s: после Load обложка не по шаблону: %q", id, got)
		}
		// Предел вернулся из таблицы; у изделий, которых нет на 30×30, строки нет (081 С2).
		if lim, ok := model.LimitOf(q); ok != (id == "fabrika-photo") || (ok && lim.Max != 40) {
			t.Errorf("%s: предел после Load %+v (%v)", id, lim, ok)
		}
	}
	// 30×20 / 2 мм: десять разворотов, ступень «10 — 24 мм» переживает Save → Load, обложка по шаблону.
	p.Settings.PageW, p.Settings.PageH = 304, 203
	pr, _ := model.CoverPresetFor("fabrika-photo-2", p.Settings)
	p.Cover.Spec = pr.Spec
	_, q := saveLoad(t, p, filepath.Join(dir, "30x20.salbum"))
	if st := q.Cover.Spec.Steps; len(st) != 9 || st[8] != (model.SpineStep{Upto: 10, Mm: 24}) || model.CoverFixText(q) != "" {
		t.Errorf("30×20 / 2 мм после Load: %+v, %q", q.Cover.Spec.Steps, model.CoverFixText(q))
	}
	if lim, ok := model.LimitOf(q); !ok || lim.Max != 10 {
		t.Errorf("30×20 / 2 мм: предел %+v (%v)", lim, ok)
	}
	// Незнакомый id (своя типография с другого компьютера, 123 §4) остаётся пометкой изделия,
	// числа те же: обложка ведёт себя как «Свои размеры». Мусорный id — «Свои размеры».
	cover := `,"cover":{"spec":{"preset":"fabrika-future","w":312,"h":306,"wrap":12,"steps":[{"upto":40,"mm":26}]}}`
	r := mustDecode(t, coverJSON(dir, `[{"id":"s1","photos":["p"]}]`, cover), dir)
	if s := r.Cover.Spec; s.Preset != "fabrika-future" || model.CoverMissing(s.Preset) == "" || s.W != 312 || s.H != 306 || s.Wrap != 12 || len(s.Steps) != 1 || s.Steps[0].Mm != 26 {
		t.Errorf("незнакомый пресет: %+v", s)
	}
	junk := mustDecode(t, coverJSON(dir, `[{"id":"s1","photos":["p"]}]`, `,"cover":{"spec":{"preset":"../Чужой","w":312,"h":306,"wrap":12}}`), dir)
	if s := junk.Cover.Spec; s.Preset != model.CoverCustom || s.W != 312 {
		t.Errorf("мусорный id: %+v", s)
	}
}

// TestUnknownPresetKept (123 §4): обложка со своей типографией открывается на компьютере, где её
// нет, не теряя id изделия, и возвращается в тот же JSON; когда типография появилась, id снова
// пресет. Карточка пишет «Типография не найдена».
func TestUnknownPresetKept(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	p.Cover.Spec.Preset = "own-0a1b2c3d-1"
	path := filepath.Join(dir, "own.salbum")
	first, q := saveLoad(t, p, path)
	if q.Cover.Spec.Preset != "own-0a1b2c3d-1" || model.CoverMissing(q.Cover.Spec.Preset) != "Типография не найдена: own-0a1b2c3d-1" {
		t.Fatalf("id изделия потерян: %+v", q.Cover.Spec)
	}
	if second, _ := saveLoad(t, q, path); !bytes.Equal(first, second) {
		t.Error("Save → Load → Save дал другой JSON")
	}
	if _, ok := model.CoverPresetFor(q.Cover.Spec.Preset, q.Settings); ok {
		t.Error("неизвестное изделие нашлось")
	}
	// Холостая обложка: геометрия по числам Spec, как у «Свои размеры».
	if cg := q.CoverGeo(); cg.X[7] <= 0 || cg.Y[3] <= 0 {
		t.Errorf("лист обложки без размера: %+v", cg)
	}
}
