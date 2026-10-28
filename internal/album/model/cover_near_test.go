package model

import (
	"math"
	"testing"
)

// TestCoverPresetForNearest (070 Е): страница вне таблицы — шаблон ближайшего формата по
// |ΔW|+|ΔH| (300×300 → 30×30, 200×300 → 21×30, 400×300 → 30×30); равное расстояние — первый
// в таблице; формат таблицы — он сам; нули, отрицательные, NaN и огромные — какой-то формат,
// без паники и без сторонки «из страницы»; у всех трёх пресетов Фотосферы одно и то же.
func TestCoverPresetForNearest(t *testing.T) {
	for _, c := range []struct {
		w, h float64
		name string
	}{
		{300, 300, "30×30"}, {200, 300, "21×30"}, {400, 300, "30×30"},
		{250, 250, "25×25"}, {201, 152, "20×15"}, {0, 0, "20×15"}, {-5, -5, "20×15"},
		{1e6, 1e6, "30×30"}, {math.NaN(), 300, "20×15"},
	} {
		s := Settings{PageW: c.w, PageH: c.h}
		f := FsferaFormatNear(s)
		if f.Name != c.name {
			t.Errorf("%v×%v: формат %s, ждали %s", c.w, c.h, f.Name, c.name)
		}
		for _, id := range []string{"fsfera-thin", "fsfera-card", "fsfera-double"} {
			pr, ok := CoverPresetFor(id, s)
			if sp := pr.Spec; !ok || sp.W != f.SideW || sp.H != f.SideH || sp.Wrap != f.Wrap {
				t.Errorf("%s на %v×%v: %+v, ждали сторонку %v×%v заворот %v", id, c.w, c.h, sp, f.SideW, f.SideH, f.Wrap)
			}
		}
	}
	// Каждый формат таблицы — сам себе ближайший; лист 031 считается по размеру страницы.
	for _, f := range FsferaFormats() {
		if got := FsferaFormatNear(Settings{PageW: f.PageW, PageH: f.PageH}); got != f {
			t.Errorf("%s: ближайший %s", f.Name, got.Name)
		}
		if got := FsferaFormatNear(Settings{PageW: f.PageW, PageH: f.PageH, Sheet: true}); got != f {
			t.Errorf("лист %s: ближайший %s", f.Name, got.Name)
		}
	}
	// Ничья: страница ровно посередине двух форматов с одной шириной выбирает раньше стоящий.
	a, b := FsferaFormats()[0], FsferaFormats()[1] // 20×15 и 20×20: ширина одна
	mid := Settings{PageW: a.PageW, PageH: (a.PageH + b.PageH) / 2}
	if got := FsferaFormatNear(mid); got != a {
		t.Errorf("ничья: %s, ждали %s", got.Name, a.Name)
	}
}
