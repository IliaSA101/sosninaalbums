package model

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// psdGuide — числа PSD-шаблонов Фабрики (docs/research/fabrika-requirements.md, «PSD-шаблоны», 24.10),
// записанные по документу независимо от profile JSON. Загиб — от края листа обложки; «до корешка» —
// от линии загиба до края корешка по X, «между» — расстояние между линиями загиба по Y.
var psdGuide = map[string]struct {
	bleedX, bleedY, foldX, foldY, toSpine, between float64
}{
	"20×20": {3, 3, 20, 19, 203, 202}, // при корешке от 18 мм в PSD 18,5: направляющая даёт 19 (решение 0.26-123-50)
	"21×30": {3, 3, 18, 17, 210, 296},
	"25×25": {3, 3.5, 19, 18.5, 253, 253},
	"30×20": {3, 3, 20.5, 18.5, 306.5, 203},
	"30×30": {3, 4, 21.5, 13.5, 302.5, 303},
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestFabrikaGuidesPsd (0.26-123-35, Б5): направляющие всех пяти форматов Фабрики равны PSD, а линия
// загиба на листе CoverGeo при каждом корешке PSD (testdata/fabrika-psd.json) даёт те же «до корешка»
// и «между линиями», что шаблон. Лист и px от направляющих не зависят (их держит TestFabrikaPsdPx).
func TestFabrikaGuidesPsd(t *testing.T) {
	data, err := os.ReadFile("testdata/fabrika-psd.json")
	if err != nil {
		t.Fatal(err)
	}
	var psd psdPxFile
	if err := json.Unmarshal(data, &psd); err != nil {
		t.Fatal(err)
	}
	formats := FabrikaFormats()
	if len(formats) != len(psdGuide) || len(psd.Formats) != len(psdGuide) {
		t.Fatalf("форматов %d, PSD %d, таблица %d", len(formats), len(psd.Formats), len(psdGuide))
	}
	for _, f := range formats {
		w, ok := psdGuide[f.Name]
		if !ok {
			t.Fatalf("формата %s нет в таблице PSD", f.Name)
		}
		g := f.Guide
		if g.BleedX != w.bleedX || g.BleedY != w.bleedY || g.FoldX != w.foldX || g.FoldY != w.foldY || g.FoldSafe != 3 || g.SpineSafe != 12 {
			t.Errorf("%s: направляющие %+v, PSD обрез %v/%v загиб %v/%v риск 3, корешок 12", f.Name, g, w.bleedX, w.bleedY, w.foldX, w.foldY)
		}
		// Лист: сторонка и заворот формата, корешок каждого PSD-файла.
		rows := 0
		for _, pf := range psd.Formats {
			if pf.Name != f.Name {
				continue
			}
			for _, c := range pf.Covers {
				spec := CoverSpec{W: f.SideW, H: f.SideH, Wrap: f.Wrap, Steps: []SpineStep{{Upto: 1, Mm: c.Spine}}}
				cg := fsferaProject(f.PageW, f.PageH, 1, spec).CoverGeo()
				toSpine, between := cg.X[2]-g.FoldX, cg.Y[3]-2*g.FoldY
				if !near(toSpine, w.toSpine) || !near(between, w.between) {
					t.Errorf("%s, корешок %v: до корешка %v (PSD %v), между загибами %v (PSD %v)", f.Name, c.Spine, toSpine, w.toSpine, between, w.between)
				}
				rows++
			}
		}
		if rows < 13 {
			t.Errorf("%s: проверено корешков %d", f.Name, rows)
		}
	}
}

// TestBleedAxes: полоса обреза по осям — только у страницы, совпавшей с кнопкой формата Фабрики с её
// зоной и обрезом (25×25 — 3 / 3,5, 30×30 — 3 / 4); правка обреза, зоны, чужой размер, лист 031 и
// нулевой обрез — прежнее одно число на обе оси.
func TestBleedAxes(t *testing.T) {
	for _, f := range FabrikaFormats() {
		w := psdGuide[f.Name]
		s := Settings{PageW: f.PageW, PageH: f.PageH, DPI: 300, Layflat: true, Safe: f.Safe, Bleed: f.Bleed}
		if x, y := BleedAxes(s); x != w.bleedX || y != w.bleedY {
			t.Errorf("%s: обрез %v / %v, PSD %v / %v", f.Name, x, y, w.bleedX, w.bleedY)
		}
		// Допуск страницы PageTol: файл из px отстоит от таблицы на ≤ 1 px.
		s.PageW -= 0.05
		if x, y := BleedAxes(s); x != w.bleedX || y != w.bleedY {
			t.Errorf("%s в допуске: обрез %v / %v", f.Name, x, y)
		}
	}
	base := Settings{PageW: 250, PageH: 254, DPI: 300, Layflat: true, Safe: 10, Bleed: 3}
	for name, c := range map[string]func(*Settings){
		"обрез задан рукой": func(s *Settings) { s.Bleed = 5 },
		"зона другая":       func(s *Settings) { s.Safe = 8 },
		"чужой размер":      func(s *Settings) { s.PageH = 250 },
		"лист 031":          func(s *Settings) { s.Sheet = true },
		"обрез 0":           func(s *Settings) { s.Bleed = 0 },
	} {
		s := base
		c(&s)
		if x, y := BleedAxes(s); x != s.Bleed || y != s.Bleed {
			t.Errorf("%s: обрез %v / %v, ждали %v по обеим осям", name, x, y, s.Bleed)
		}
	}
}

// TestCoverGuide: направляющие обложки нужны только по шаблону Фабрики и без своих безопасных линий;
// пользовательские Safe/SpineSafe, чужой размер листа, Фотосфера и «Свои размеры» — прежний расчёт.
func TestCoverGuide(t *testing.T) {
	s := Settings{PageW: 304, PageH: 203, DPI: 300, Layflat: true, Safe: 10, Bleed: 3}
	pr, _ := CoverPresetFor("fabrika-photo", s)
	mk := func(spec CoverSpec) *Project {
		p := fsferaProject(304, 203, 2, spec)
		p.Settings = s
		return p
	}
	if g, ok := mk(pr.Spec).CoverGuide(); !ok || g.FoldX != 20.5 || g.FoldY != 18.5 || g.FoldSafe != 3 || g.SpineSafe != 12 {
		t.Fatalf("по шаблону: %+v %v", g, ok)
	}
	for name, mod := range map[string]func(*CoverSpec){
		"Safe задан":          func(c *CoverSpec) { c.Safe = 5 },
		"SpineSafe задан":     func(c *CoverSpec) { c.SpineSafe = 5 },
		"заворот не шаблона":  func(c *CoverSpec) { c.Wrap = 15 },
		"сторонка не шаблона": func(c *CoverSpec) { c.W = 300 },
		"таблица не шаблона":  func(c *CoverSpec) { c.Steps = []SpineStep{{Upto: 99, Mm: 5}} },
		"свои размеры":        func(c *CoverSpec) { *c = CoverSpec{Preset: CoverCustom, Wrap: 15, A: 10} },
		"Фотосфера":           func(c *CoverSpec) { fp, _ := CoverPresetFor(CoverDefault, s); *c = fp.Spec },
	} {
		spec := pr.Spec
		mod(&spec)
		if g, ok := mk(spec).CoverGuide(); ok || g != (PrintGuide{}) {
			t.Errorf("%s: направляющие %+v", name, g)
		}
	}
	if _, ok := New().CoverGuide(); ok {
		t.Error("без обложки направляющие есть")
	}
}
