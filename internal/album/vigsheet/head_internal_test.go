package vigsheet

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Кегль шапки (049 §3.2): у полос сверху и снизу — прежняя формула h/2,6 (кегль старых листов
// не меняется), ширина полосы и замер не участвуют; в ряду — замер ядра, без него — ещё и не
// больше ширины/9; 12…48 пт.
func TestHeadStyleBandsSame(t *testing.T) {
	for _, h := range []float64{0, 5, 14, 20, 31.5, 40, 200} {
		want := math.Round(geom.ClampF(h/2.6/0.3528, 12, 48))
		for _, w := range []float64{0, 10, 100, 1000} {
			if got := headStyle(layout.Rect{W: w, H: h}, false, 0).Size; got != want {
				t.Fatalf("полоса %v×%v: кегль %v, ждали прежний %v", w, h, got, want)
			}
			if got := headStyle(layout.Rect{W: w, H: h}, false, 20).Size; got != want {
				t.Fatalf("полоса %v×%v с замером: кегль %v, ждали прежний %v", w, h, got, want)
			}
		}
		if got := headStyle(layout.Rect{W: 1000, H: h}, true, 0).Size; got != want {
			t.Fatalf("широкая шапка в ряду %v: кегль %v, ждали %v", h, got, want)
		}
	}
	for _, c := range []struct{ w, h, want float64 }{
		{45, 60, math.Round(45.0 / 9 / 0.3528)}, // узкая высокая ячейка — по ширине
		{0, 60, 12},                             // нижний предел
		{900, 60, 48},                           // верхний предел
	} {
		if got := headStyle(layout.Rect{W: c.w, H: c.h}, true, 0).Size; got != c.want {
			t.Fatalf("шапка в ряду %v×%v: кегль %v, ждали %v", c.w, c.h, got, c.want)
		}
	}
	for _, fit := range []float64{12, 17, 48} { // замер ядра (HeadFit) — кегль шапки в ряду как есть
		if got := headStyle(layout.Rect{W: 45, H: 60}, true, fit).Size; got != fit {
			t.Fatalf("шапка в ряду с замером %v: кегль %v", fit, got)
		}
	}
	if st := headStyle(layout.Rect{W: 45, H: 60}, true, 17); st.Font != model.PlainStyle().Font || st.Align != model.PlainStyle().Align {
		t.Fatalf("стиль шапки в ряду не умолчание: %+v", st)
	}
}

// refitHead: кегль «по полосе» идёт за полосой, свой только уменьшается и только на мельчающей
// полосе; без новой полосы — не трогается.
func TestRefitHeadRules(t *testing.T) {
	band := &layout.Rect{W: 180, H: 31}   // полоса сверху: 34 пт
	cell := &layout.Rect{W: 80, H: 60}    // ячейки ряда: 25 пт
	wide := &layout.Rect{W: 180, H: 39.5} // полоса крупнее: 43 пт
	auto := headStyle(*band, false, 0).Size
	small := headStyle(*cell, true, 0).Size
	big := headStyle(*wide, false, 0).Size
	if !(small < auto && auto < big) {
		t.Fatalf("кегли полос %v, %v, %v — нужен порядок по возрастанию", small, auto, big)
	}
	plan := func(r *layout.Rect, c bool) VigPlan { return VigPlan{Head: r, HeadCell: c} }
	fitted := func(r *layout.Rect, fit float64) VigPlan { return VigPlan{Head: r, HeadCell: true, HeadFit: fit} }
	for _, c := range []struct {
		name     string
		size     float64
		was, cur VigPlan
		wantSize float64
	}{
		{"по полосе → та же полоса", auto, plan(band, false), plan(band, false), auto},
		{"по полосе → в ряд", auto, plan(band, false), plan(cell, true), small},
		{"по полосе → крупнее", auto, plan(band, false), plan(wide, false), big},
		{"свой крупный → та же полоса", 46, plan(band, false), plan(band, false), 46},
		{"свой крупный → в ряд", 46, plan(band, false), plan(cell, true), small},
		{"свой мелкий → в ряд", 12, plan(band, false), plan(cell, true), 12},
		{"свой → крупнее", 20, plan(band, false), plan(wide, false), 20},
		{"полосы не было", 46, plan(nil, false), plan(cell, true), small},
		{"полосы не было, мелкий", 12, plan(nil, false), plan(band, false), 12},
		{"новой полосы нет", 46, plan(band, false), plan(nil, false), 46},
		{"по полосе → в ряд по замеру", auto, plan(band, false), fitted(cell, 19), 19},
		{"по замеру → полоса", 19, fitted(cell, 19), plan(band, false), auto},
		{"свой → в ряд по замеру", 22, plan(band, false), fitted(cell, 19), 19},
	} {
		h := &model.TextBlock{Style: model.TextStyle{Font: "ptserif", Size: c.size, Color: "#112233", Align: "left"}}
		refitHead(h, c.was, c.cur)
		if h.Style.Size != c.wantSize {
			t.Fatalf("%s: кегль %v, ждали %v", c.name, h.Style.Size, c.wantSize)
		}
		if h.Style.Font != "ptserif" || h.Style.Color != "#112233" || h.Style.Align != "left" {
			t.Fatalf("%s: стиль сменился %+v", c.name, h.Style)
		}
	}
}

// SwapTText: умолчание ↔ подпись предметника, своя и пустая без шаблона — как есть.
func TestSwapTText(t *testing.T) {
	staff, teach := model.StaffCaption, model.TeacherCaption
	for _, c := range []struct{ cur, tpl, want string }{
		{"", staff, staff},
		{teach, staff, staff},
		{staff, staff, staff},
		{"{Фамилия}", staff, "{Фамилия}"},
		{staff, "", teach},
		{teach, "", teach},
		{"", "", ""},
		{"{Фамилия}", "", "{Фамилия}"},
	} {
		if got := SwapTText(c.cur, c.tpl); got != c.want {
			t.Fatalf("SwapTText(%q, %q) = %q, ждали %q", c.cur, c.tpl, got, c.want)
		}
	}
}
