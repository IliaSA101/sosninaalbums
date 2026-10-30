package model

import "testing"

// TestPageFitsGroup (123 §1.5): страница совпала с кнопкой ряда целиком — размер с допуском, dpi,
// layflat, зона и обрез. 21×30 Фотосферы и Фабрики одного размера, их различают зона и обрез.
func TestPageFitsGroup(t *testing.T) {
	on := func(group, label string) Settings {
		for _, pr := range PagePresets() {
			if pr.Group == group && pr.Label == label {
				return Settings{PageW: pr.W, PageH: pr.H, DPI: pr.DPI, Layflat: pr.Layflat, Safe: pr.Safe, Bleed: pr.Bleed}
			}
		}
		t.Fatalf("кнопки %s %q нет", group, label)
		return Settings{}
	}
	fs21, fab21 := on(CoverGroupFsfera, "21×30"), on(CoverGroupFabrika, "21×30")
	if fs21.PageW != fab21.PageW || fs21.PageH != fab21.PageH {
		t.Fatal("21×30 должны совпасть размером — иначе тест ничего не различает")
	}
	near := on(CoverGroupFabrika, "30×20")
	near.PageW, near.PageH = near.PageW-0.05, near.PageH-0.05
	dpi, sheet, zone := on(CoverGroupFabrika, "30×20"), on(CoverGroupFabrika, "30×20"), on(CoverGroupFabrika, "30×20")
	dpi.DPI = 150
	sheet.Sheet = true
	zone.Safe = 6
	for _, c := range []struct {
		name  string
		s     Settings
		group string
		want  bool
	}{
		{"21×30 Фотосферы — Фотосфера", fs21, CoverGroupFsfera, true},
		{"21×30 Фотосферы — не Фабрика", fs21, CoverGroupFabrika, false},
		{"21×30 Фабрики — Фабрика", fab21, CoverGroupFabrika, true},
		{"21×30 Фабрики — не Фотосфера", fab21, CoverGroupFsfera, false},
		{"303,95 × 202,95 в допуске", near, CoverGroupFabrika, true},
		{"другой dpi", dpi, CoverGroupFabrika, false},
		{"другая зона", zone, CoverGroupFabrika, false},
		{"лист 031", sheet, CoverGroupFabrika, false},
		{"общий ряд", fab21, "", false},
		{"неизвестный ряд", fab21, "Нет такой", false},
		{"пустые настройки", Settings{}, CoverGroupFabrika, false},
	} {
		if got := PageFitsGroup(c.s, c.group); got != c.want {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}
