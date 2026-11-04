package app

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// shareEstimate (T5-13): оценка идёт по площади фото, а не по всей странице. Эталон — QA-альбом
// (8 разворотов 600×300 мм, 3 фото, доля 0,59…0,85, страниц 16): рендер дал 4 034 381 Б; прежняя
// оценка 0,2 Б/px по всей странице давала 8 256 630 Б (вдвое выше).
func TestShareEstimate(t *testing.T) {
	spec, err := export.NewWebSpec(model.Settings{PageW: 300, PageH: 300}, model.View{})
	if err != nil {
		t.Fatal(err)
	}
	// Разворот доли c: одно фото на всю ширину и c·300 мм высоты.
	view := func(covs ...float64) model.View {
		var v model.View
		for _, c := range covs {
			v.Spreads = append(v.Spreads, model.SpreadView{W: 600, H: 300, Rects: []layout.Rect{{W: 600, H: 300 * c}}})
		}
		return v
	}
	qa := view(0.773, 0.826, 0.774, 0.661, 0.846, 0.773, 0.594, 0.828)
	const rendered = 4034381.0
	if got := float64(shareEstimate(spec, qa, 16)); got < rendered*0.7 || got > rendered*1.3 {
		t.Errorf("QA-альбом: оценка %.0f, рендер %.0f (допуск ±30 %%)", got, rendered)
	}
	// Края: нет разворотов, развороты без фото.
	if n := shareEstimate(spec, model.View{}, 0); n != 0 {
		t.Errorf("нет разворотов: %d", n)
	}
	if n := shareEstimate(spec, view(0, 0), 4); n != 0 {
		t.Errorf("развороты без фото: %d", n)
	}
	// Несколько разворотов: оценка растёт с долей и складывается по страницам.
	one, full, two := shareEstimate(spec, view(0.5), 2), shareEstimate(spec, view(1), 2), shareEstimate(spec, view(0.5, 0.5), 4)
	near := func(a, b int64) bool { return a-b <= 1 && b-a <= 1 } // округление до байта
	if one <= 0 || !near(full, 2*one) || !near(two, 2*one) {
		t.Errorf("доля 0,5: %d, доля 1: %d, два разворота: %d", one, full, two)
	}
	// Перекрытия (сетка) не дают больше полной страницы.
	over := model.View{Spreads: []model.SpreadView{{W: 600, H: 300, Rects: []layout.Rect{{W: 600, H: 300}, {W: 600, H: 300}}}}}
	if n := shareEstimate(spec, over, 2); !near(n, full) {
		t.Errorf("перекрытия: %d, ждали %d", n, full)
	}
	// Место за краем листа считается только внутри листа; место целиком снаружи — ноль.
	clip := model.View{Spreads: []model.SpreadView{{W: 600, H: 300, Rects: []layout.Rect{{X: -100, Y: -50, W: 700, H: 200}, {X: 500, W: 400, H: 300}, {X: 900, W: 50, H: 50}}}}}
	// внутри листа: 600×150 + 100×300 = 120000 из 180000 (две первые не пересекаются)
	if n, want := shareEstimate(spec, clip, 2), shareEstimate(spec, view(120000.0/180000), 2); !near(n, want) {
		t.Errorf("обрезка по листу: %d, ждали %d", n, want)
	}
	// Обложка — ещё две страницы средней доли (здесь 0,5: как один разворот этой доли).
	if n := shareEstimate(spec, view(0.5, 0.5), 6); n < two+one-1 || n > two+one+1 {
		t.Errorf("с обложкой: %d, ждали около %d", n, two+one)
	}
}
