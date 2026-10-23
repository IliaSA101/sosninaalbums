package model

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обрез и разметка типографии (066).

// TestPagePresetsBleed — обрез у пресетов: Фотосфера 2 (все форматы), Нетпринт 3 (гипотеза,
// decisions-066 №5), Фабрика Фотокниги 3 (081 №2, гипотеза), общий ряд 0 — обрез пользователя он не трогает, как и безопасную зону.
func TestPagePresetsBleed(t *testing.T) {
	want := map[string]float64{"": 0, CoverGroupFsfera: 2, CoverGroupNetprint: 3, CoverGroupFabrika: 3}
	n := map[string]int{}
	for _, pp := range PagePresets() {
		if pp.Bleed != want[pp.Group] {
			t.Errorf("%s %q: обрез %v, ждали %v", pp.Group, pp.Label, pp.Bleed, want[pp.Group])
		}
		n[pp.Group]++
	}
	if n[""] == 0 || n[CoverGroupFsfera] != len(FsferaFormats()) || n[CoverGroupNetprint] == 0 || n[CoverGroupFabrika] != len(FabrikaFormats()) {
		t.Fatalf("ряды пресетов: %v", n)
	}
	for _, f := range FsferaFormats() {
		if f.Bleed != 2 {
			t.Errorf("%s: обрез формата %v", f.Name, f.Bleed)
		}
	}
}

// TestClampSettingsBleed — обрез 0…min(10, четверть меньшей стороны: при странице от 50 мм это
// всегда 10); NaN, ±Inf и меньше нуля — 0.
func TestClampSettingsBleed(t *testing.T) {
	for _, c := range []struct {
		pw, ph, in, want float64
	}{
		{300, 300, 2, 2}, {300, 300, 2.5, 2.5}, {300, 300, 0, 0}, {300, 300, -1, 0},
		{300, 300, math.NaN(), 0}, {300, 300, math.Inf(1), 0}, {300, 300, math.Inf(-1), 0},
		{300, 300, 11, 10}, {300, 300, 10, 10}, {60, 300, 30, 10}, {50, 50, 20, 10},
	} {
		s := DefaultSettings()
		s.PageW, s.PageH, s.Bleed = c.pw, c.ph, c.in
		if got := ClampSettings(s).Bleed; got != c.want {
			t.Errorf("%v×%v, обрез %v: %v, ждали %v", c.pw, c.ph, c.in, got, c.want)
		}
	}
	if DefaultSettings().Bleed != 0 {
		t.Fatal("обрез по умолчанию не 0")
	}
}

// pxAt300 — сторона файла в px при 300 dpi (как экспорт).
func pxAt300(mm float64) int { return int(math.Round(mm / 25.4 * 300)) }

// TestFsferaPSDNumbers — числа PSD Фотосферы (research §2) как векторы, без самих файлов:
// размер файла разворота и обложки в мм и px, безопасная зона, обрез, корешок по корзине,
// заворот и безопасные линии обложки сходятся со встроенными таблицами.
func TestFsferaPSDNumbers(t *testing.T) {
	formats := map[string]PrintFormat{}
	for _, f := range FsferaFormats() {
		formats[f.Name] = f
	}
	for _, c := range []struct {
		name         string
		pxW, pxH     int
		safe, bleed  float64
		fileW, fileH float64
	}{
		{"20×30", 4748, 3224, 10, 2, 402, 273},
		{"30×30", 6638, 3354, 10, 2, 562, 284},
		{"25×25", 5929, 3000, 8, 2, 502, 254},
	} {
		f := formats[c.name]
		if 2*f.PageW != c.fileW || f.PageH != c.fileH || pxAt300(2*f.PageW) != c.pxW || pxAt300(f.PageH) != c.pxH ||
			f.Safe != c.safe || f.Bleed != c.bleed {
			t.Errorf("разворот %s: %+v", c.name, f)
		}
	}
	for _, c := range []struct {
		format, preset   string
		n                int
		w, h, spine, wrp float64
		pxW, pxH         int
	}{
		{"20×30", "fsfera-thin", 34, 472, 316, 20, 20, 5575, 3732},
		{"30×30", "fsfera-thin", 22, 626, 326, 14, 20, 7394, 3850},
		{"30×30", "fsfera-card", 12, 626, 326, 14, 20, 7394, 3850},
		{"30×30", "fsfera-double", 10, 630, 326, 18, 20, 7441, 3850},
		{"21×30", "fsfera-thin", 22, 468, 328, 14, 16, 5528, 3874},
	} {
		f := formats[c.format]
		p := New()
		p.Settings.PageW, p.Settings.PageH = f.PageW, f.PageH
		for range c.n {
			p.Spreads = append(p.Spreads, &Spread{ID: NewID()})
		}
		pr, ok := CoverPresetFor(c.preset, p.Settings)
		if !ok {
			t.Fatalf("нет пресета %s", c.preset)
		}
		p.Cover = &Cover{Sheet: &Spread{ID: CoverID}, Spec: pr.Spec}
		cg := p.CoverGeo()
		w, h := cg.X[7], cg.Y[3]
		if w != c.w || h != c.h || cg.Spine != c.spine || cg.X[1] != c.wrp || cg.Y[1] != c.wrp || cg.Over ||
			pxAt300(w) != c.pxW || pxAt300(h) != c.pxH || pr.Spec.Safe != 8 || pr.Spec.SpineSafe != 12 {
			t.Errorf("обложка %s %s n=%d: %v×%v корешок %v заворот %v/%v, px %d×%d, линии %v/%v",
				c.format, c.preset, c.n, w, h, cg.Spine, cg.X[1], cg.Y[1], pxAt300(w), pxAt300(h), pr.Spec.Safe, pr.Spec.SpineSafe)
		}
	}
}

// TestMarkupView — вид разметки: нет пути — nil; файл не прочитан — только имя; линии — мм
// листа от размера файла (картинка растянута на лист); Warn у PSD с DPI — по мм с допуском
// 1 мм и числами, без DPI — по пропорциям с допуском 1 %, слова по листу.
func TestMarkupView(t *testing.T) {
	if MarkupView("", library.MarkupInfo{W: 1, H: 1}, true, 10, 10, MarkupCover) != nil {
		t.Fatal("вид без файла")
	}
	gv := MarkupView(testkit.AbsPath("Шаблоны/spread.psd"), library.MarkupInfo{W: 600, H: 300}, false, 600, 300, MarkupSpread)
	if gv == nil || gv.Name != "spread.psd" || gv.OK || gv.Image || gv.Warn != "" || gv.Lines != nil {
		t.Fatalf("не прочитан: %+v", gv)
	}
	// Обложка 30×30 из 20 тонких (626 × 326 мм) и её шаблон 7394 × 3850 px при 300 dpi.
	cov := library.MarkupInfo{W: 7394, H: 3850, DPI: 300, Image: true,
		Guides: []library.MarkupGuide{{Pos: 7394.0 * 20 / 626}, {Pos: 7394.0 * 306 / 626}, {Pos: 3850.0 * 20 / 326, Horz: true}, {Pos: 7394}}}
	gv = MarkupView(testkit.AbsPath("a/Cover.psd"), cov, true, 626, 326, MarkupCover)
	want := []GuideLine{{V: true, At: 20}, {V: true, At: 306}, {At: 20}, {V: true, At: 626}}
	if !gv.OK || !gv.Image || gv.Warn != "" || len(gv.Lines) != len(want) {
		t.Fatalf("обложка по размеру: %+v", gv)
	}
	for i, l := range gv.Lines {
		if l.V != want[i].V || math.Abs(l.At-want[i].At) > 0.01 {
			t.Errorf("линия %d: %+v, ждали %+v", i, l, want[i])
		}
	}
	// Тот же шаблон на обложке из 23 разворотов (корешок 16: 628 мм) — строка с числами; линии
	// всё равно по файлу, растянутому на лист.
	gv = MarkupView(testkit.AbsPath("a/Cover.psd"), cov, true, 628, 326, MarkupCover)
	if gv.Warn != "Размер разметки 626 × 326 мм — у обложки 628 × 326 мм" || len(gv.Lines) != 4 || gv.Lines[3].At != 628 {
		t.Fatalf("обложка не по размеру: %+v", gv)
	}
	if gv = MarkupView("s.psd", cov, true, 626.9, 325.1, MarkupCover); gv.Warn != "" {
		t.Fatalf("в допуске 1 мм: %q", gv.Warn)
	}
	sp := library.MarkupInfo{W: 4748, H: 3224, DPI: 300}
	if gv = MarkupView("s.psd", sp, true, 562, 284, MarkupSpread); gv.Warn != "Размер разметки 402 × 273 мм — у разворота 562 × 284 мм" || gv.Image {
		t.Fatalf("разворот: %+v", gv)
	}
	if gv = MarkupView("s.psd", sp, true, 300, 400, MarkupSheet); gv.Warn != "Размер разметки 402 × 273 мм — у листа 300 × 400 мм" {
		t.Fatalf("лист: %q", gv.Warn)
	}
	// Без DPI (JPEG/PNG): пропорции, 1 %.
	jp := library.MarkupInfo{W: 1000, H: 500, Image: true}
	for _, c := range []struct {
		w, h        float64
		where, warn string
	}{
		{600, 300, MarkupSpread, ""}, {600, 302, MarkupSpread, ""}, {600, 310, MarkupSpread, "Пропорции разметки не совпадают с разворотом"},
		{300, 300, MarkupCover, "Пропорции разметки не совпадают с обложкой"}, {300, 300, MarkupSheet, "Пропорции разметки не совпадают с листом"},
	} {
		if gv = MarkupView("g.jpg", jp, true, c.w, c.h, c.where); gv.Warn != c.warn || !gv.Image {
			t.Errorf("%v×%v %s: %q, ждали %q", c.w, c.h, c.where, gv.Warn, c.warn)
		}
	}
	// Не больше maxGuideLines линий в виде; пустой размер файла — как не прочитан.
	many := library.MarkupInfo{W: 10, H: 10, Guides: make([]library.MarkupGuide, 1000)}
	if gv = MarkupView("m.psd", many, true, 10, 10, MarkupSpread); len(gv.Lines) != maxGuideLines {
		t.Fatalf("линий %d", len(gv.Lines))
	}
	if gv = MarkupView("z.psd", library.MarkupInfo{}, true, 10, 10, MarkupSpread); gv.OK || gv.Lines != nil {
		t.Fatalf("нулевой размер: %+v", gv)
	}
	if gv = MarkupView("note.psd", library.MarkupInfo{W: 10, H: 10, Note: "без картинки"}, true, 10, 10, MarkupSpread); gv.Note != "без картинки" || gv.Image {
		t.Fatalf("причина: %+v", gv)
	}
}

// TestGuideKey — отпечаток файла: тот же файл — тот же ключ; то же имя в другой папке, другой
// размер или время записи — другой; не прочитан — по одному пути; пути в ключе нет; без файла
// вида нет.
func TestGuideKey(t *testing.T) {
	info := library.MarkupInfo{W: 10, H: 10, Size: 500, Mod: 1000}
	key := func(path string, in library.MarkupInfo, ok bool) string {
		return MarkupView(path, in, ok, 10, 10, MarkupSpread).Key
	}
	a := key(testkit.AbsPath("A/s.psd"), info, true)
	if len(a) != 12 || strings.Contains(a, "psd") || a != key(testkit.AbsPath("A/s.psd"), info, true) {
		t.Fatalf("ключ %q пуст, содержит путь или нестабилен", a)
	}
	for name, other := range map[string]string{
		"папка":  key(testkit.AbsPath("B/s.psd"), info, true),
		"размер": key(testkit.AbsPath("A/s.psd"), library.MarkupInfo{W: 10, H: 10, Size: 501, Mod: 1000}, true),
		"время":  key(testkit.AbsPath("A/s.psd"), library.MarkupInfo{W: 10, H: 10, Size: 500, Mod: 1001}, true),
	} {
		if other == a {
			t.Errorf("%s не меняет ключ", name)
		}
	}
	// Не прочитан: размер и время из info не берутся, ключ зависит только от пути.
	n1, n2 := key(testkit.AbsPath("A/s.psd"), info, false), key(testkit.AbsPath("A/s.psd"), library.MarkupInfo{}, false)
	if n1 != n2 || n1 == a || n1 == key(testkit.AbsPath("B/s.psd"), info, false) {
		t.Fatalf("не прочитан: %q %q", n1, n2)
	}
	if MarkupView("", info, true, 10, 10, MarkupSpread) != nil {
		t.Fatal("вид без файла")
	}
}
