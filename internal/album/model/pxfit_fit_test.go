package model

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// pxSnapshot — встроенные профили, у Фабрики форматы с px: 30×20 — 7180×2397 (на 1 px меньше
// таблицы, пример из спеки 123 §5), остальные — по мм страницы, минус px по ширине (dw).
func pxSnapshot(t *testing.T, dw int) {
	t.Helper()
	base := printprof.Builtin().Profiles()
	fab := base[2]
	fab.Formats = slices.Clone(fab.Formats)
	for i, f := range fab.Formats {
		fab.Formats[i].Px = &printprof.PxPair{Px(2*f.Page[0], PrintDPI) - dw, Px(f.Page[1], PrintDPI)}
	}
	fab.Formats[3].Px = &printprof.PxPair{7180, 2397}
	snap, err := printprof.NewSnapshot([]printprof.Profile{base[0], base[1], fab})
	if err != nil {
		t.Fatal(err)
	}
	printprof.UseForTest(t, snap)
}

// TestFitWithinTol: формат с px совпадает со страницей, пока файл страницы в FitTolPx от цели по
// каждой оси; FitPx называет цель, когда файл не точен. Перебор страницы от 302,5 до 305,5 мм
// шагом 0,01 против прямого счёта; в таблицах без px FormatOf прежний (по мм).
func TestFitWithinTol(t *testing.T) {
	if got := FitTolPx(PrintDPI); got != 6 {
		t.Fatalf("FitTolPx(300) = %d, ждали 6", got)
	}
	pxSnapshot(t, 0)
	hit, fit := 0, 0
	for w := 302.5; w <= 305.5; w += 0.01 {
		s := Settings{PageW: w, PageH: 202.95}
		nw, nh := Px(2*w, PrintDPI), Px(202.95, PrintDPI) // высота ровно 2397
		f, ok := FormatOf(CoverGroupFabrika, s)
		want := abs(nw-7180) <= 6 && abs(nh-2397) <= 6
		if want != (ok && f.Name == "30×20") {
			t.Fatalf("%.2f мм (%d px): FormatOf %q %v, в допуске %v", w, nw, f.Name, ok, want)
		}
		fw, fh, fok := f.FitPx(w, 202.95)
		if wantFit := want && (nw != 7180 || nh != 2397); fok != wantFit || fok && (fw != 7180 || fh != 2397) {
			t.Fatalf("%.2f мм: FitPx %d×%d %v, ждали подгонку %v", w, fw, fh, fok, wantFit)
		}
		hit += btoi(want)
		fit += btoi(fok)
	}
	if hit < 40 || fit < 30 || fit >= hit {
		t.Fatalf("перебор вырожден: совпало %d, подогнано %d", hit, fit)
	}
	// Граница: 7186 px — ровно допуск (6), 7188 — за ним.
	for _, c := range []struct {
		w  float64
		ok bool
	}{{304.2, true}, {304.3, false}} {
		if _, ok := FormatOf(CoverGroupFabrika, Settings{PageW: c.w, PageH: 203}); ok != c.ok {
			t.Errorf("%v мм (%d px): FormatOf = %v", c.w, Px(2*c.w, PrintDPI), ok)
		}
	}
	// Высота тоже по допуску, а 0 px цели — «нет цели».
	if _, _, ok := (PrintFormat{}).FitPx(304, 203); ok {
		t.Error("формат без px подгоняется")
	}
	if _, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 304, PageH: 203.6}); ok {
		t.Error("высота на 7 px больше цели принята")
	}
	if f, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 304, PageH: 203.4}); !ok || f.Name != "30×20" {
		t.Errorf("высота на 5 px больше цели: %v %v", f.Name, ok)
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestFitRejectsForeign: чужие форматы (201/200, 251/250 — разница 1 мм, 24 px по ширине) не
// принимаются и не подгоняются, даже когда у формата есть px (подгонка — до 3 px на сторону).
func TestFitRejectsForeign(t *testing.T) {
	pxSnapshot(t, 1)
	for _, c := range []struct {
		name string
		s    Settings
	}{
		{"20×20", Settings{PageW: 201, PageH: 203}},
		{"20×20", Settings{PageW: 199, PageH: 203}},
		{"25×25", Settings{PageW: 251, PageH: 254}},
		{"25×25", Settings{PageW: 250, PageH: 255}},
		{"30×30", Settings{PageW: 300, PageH: 306}},
	} {
		if f, ok := FormatOf(CoverGroupFabrika, c.s); ok {
			t.Errorf("%v: принята как %s", c.s, f.Name)
		}
		for _, f := range FabrikaFormats() {
			if f.Name != c.name {
				continue
			}
			if w, h, ok := f.FitPx(c.s.PageW, c.s.PageH); ok {
				t.Errorf("%s подогнан под %v: %d×%d", c.name, c.s, w, h)
			}
		}
	}
	// Свой формат: страница на 1 px меньше цели по ширине (dw=1) — подгонка есть, FormatNear тот же.
	if f, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 200, PageH: 203}); !ok || f.Name != "20×20" {
		t.Fatalf("20×20 с px: %v %v", f.Name, ok)
	}
	if f := FormatNear(CoverGroupFabrika, Settings{PageW: 200, PageH: 203}); f.Name != "20×20" || f.Px[0] == 0 {
		t.Errorf("FormatNear: %+v", f)
	}
}

// Строка Px формата доходит до таблицы; у встроенных её нет.
func TestFormatPxFromProfile(t *testing.T) {
	for _, f := range FabrikaFormats() {
		if f.Px != ([2]int{}) {
			t.Errorf("встроенный %s с px %v", f.Name, f.Px)
		}
	}
	pxSnapshot(t, 0)
	if f := FabrikaFormats()[3]; f.Px != [2]int{7180, 2397} {
		t.Errorf("px 30×20 = %v", f.Px)
	}
}
