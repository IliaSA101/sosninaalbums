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
	// Незнакомый id — «Свои размеры» с теми же числами.
	cover := `,"cover":{"spec":{"preset":"fabrika-future","w":312,"h":306,"wrap":12,"steps":[{"upto":40,"mm":26}]}}`
	r := mustDecode(t, coverJSON(dir, `[{"id":"s1","photos":["p"]}]`, cover), dir)
	if s := r.Cover.Spec; s.Preset != model.CoverCustom || s.W != 312 || s.H != 306 || s.Wrap != 12 || len(s.Steps) != 1 || s.Steps[0].Mm != 26 {
		t.Errorf("незнакомый пресет: %+v", s)
	}
}
