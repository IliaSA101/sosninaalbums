package grid_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// Подгонка шага по страницам (165 Т1, доправка ревью): края решения и предметы.

// TestLookFitPageEdgePanorama: место заходит за сгиб на δ чуть больше допуска — это панорама,
// шаг подгоняется по-старому один раз. Старая подгонка сжимает заход в Ratio/r раз, и повторная
// проверка на подогнанных местах находила «не панораму» (страницы 30×20, r = 3, заход 0,0015).
func TestLookFitPageEdgePanorama(t *testing.T) {
	for _, over := range []float64{0.0015, 0.002, 0.0029, 0.01} {
		for _, sz := range [][2]float64{{300, 200}, {400, 200}, {200, 300}} {
			var dumps [2]string
			for i, fit := range []string{"", looktpl.FitPage} {
				tp := fitTpl(fit)
				tp.Places = []model.Box{{X: 0.3, Y: 0.1, W: 0.2 + over, H: 0.6}, {X: 0.6, Y: 0.1, W: 0.3, H: 0.3}}
				tp = looktpl.LookTpl(tp, 2*sz[0]/sz[1])
				dumps[i] = fitDump(applyFit(t, tp, sz[0], sz[1]))
			}
			if dumps[0] != dumps[1] {
				t.Errorf("заход %.4f, %v: с fit\n%s\nбез fit\n%s", over, sz, dumps[1], dumps[0])
			}
		}
	}
}

// TestLookFitPageDrafts: заготовка надписи на левой половине при fit — одна штука, в центре
// левой страницы (не по одной на страницу).
func TestLookFitPageDrafts(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Drafts: []looktpl.TplText{{Anchor: looktpl.Anchor{AX: -1, DX: 0.5}, W: 0.4, Text: "[Текст]", Style: model.PlainStyle()}}}
	for _, sz := range fitPages {
		pw := sz[0]
		s := edtest.P(applyFit(t, tp, pw, sz[1])).Spreads[0]
		if len(s.Texts) != 1 || !s.Texts[0].Draft {
			t.Fatalf("%v: надписей %d, ждали одну заготовку", sz, len(s.Texts))
		}
		if tx := s.Texts[0]; math.Abs(tx.X+tx.W/2-pw/2) > 0.1 {
			t.Errorf("%v: центр заготовки %.2f, ждали %.2f", sz, tx.X+tx.W/2, pw/2)
		}
	}
}

// TestLookFitPageStretchArt: растяжимая лента W = 0,5 композиции у левого края при fit — ровно
// страница (X = 0, W = ширина страницы): доля композиции — две доли ширины страницы.
func TestLookFitPageStretchArt(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Stickers: []looktpl.TplItem{{Anchor: looktpl.Anchor{AX: -1, DX: 0.25, AY: 1, DY: -0.05}, W: 0.5, H: 0.1, Art: &model.Art{Kind: "plate"}}}}
	for _, sz := range [][2]float64{{200, 300}, {300, 200}} {
		s := edtest.P(applyFit(t, tp, sz[0], sz[1])).Spreads[0]
		if len(s.Stickers) != 1 {
			t.Fatalf("%v: стикеров %d", sz, len(s.Stickers))
		}
		if st := s.Stickers[0]; math.Abs(st.X) > 0.01 || math.Abs(st.W-sz[0]) > 0.01 {
			t.Errorf("%v: лента X %.2f W %.2f, ждали 0 и %.0f", sz, st.X, st.W, sz[0])
		}
	}
}

// TestLookFitPageFoldCenter: предмет с центром ровно на сгибе композиции относится к правой
// странице («>=»): на 300×200 его центр — у левого края кадра правой страницы.
func TestLookFitPageFoldCenter(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Stickers: []looktpl.TplItem{stickerAt(looktpl.Anchor{}, 0.2)}}
	s := edtest.P(applyFit(t, tp, 300, 200)).Spreads[0]
	if len(s.Stickers) != 1 {
		t.Fatalf("стикеров %d", len(s.Stickers))
	}
	// кадр правой страницы 200×200 со сдвигом 50: центр на его левом краю — 300 + 50
	if st := s.Stickers[0]; math.Abs(st.X+st.W/2-350) > 0.1 {
		t.Errorf("центр %.2f, ждали 350 (правая страница)", st.X+st.W/2)
	}
}

// TestLookPlaceRot (design §3, гипотеза до генератора): наклон места доходит до листа, а при
// нахлёсте верхнее — последнее место шаблона (порядок слоёв = порядок мест) — с fit и без.
func TestLookPlaceRot(t *testing.T) {
	rots := []float64{4, -3, 6}
	for _, fit := range []string{"", looktpl.FitPage} {
		for _, sz := range fitPages {
			tp := fitTpl(fit)
			tp.Stickers, tp.Texts = nil, nil
			tp.Places = []model.Box{
				{X: 0.06, Y: 0.1, W: 0.26, H: 0.7, Rot: rots[0]},
				{X: 0.2, Y: 0.2, W: 0.26, H: 0.7, Rot: rots[1]}, // нахлёст с первым
				{X: 0.54, Y: 0.15, W: 0.3, H: 0.6, Rot: rots[2]},
			}
			tp = looktpl.LookTpl(tp, 2*sz[0]/sz[1])
			s := edtest.P(applyFit(t, tp, sz[0], sz[1])).Spreads[0]
			if len(s.Grid) != 3 {
				t.Fatalf("%q %v: мест %d", fit, sz, len(s.Grid))
			}
			for i, pl := range s.Grid {
				if pl.Rot != rots[i] {
					t.Errorf("%q %v: у места %d наклон %v, ждали %v", fit, sz, i, pl.Rot, rots[i])
				}
			}
			stack := s.Stack()
			if len(stack) != 3 || stack[2] != s.Grid[2].ID || stack[1] != s.Grid[1].ID || stack[0] != s.Grid[0].ID {
				t.Errorf("%q %v: порядок слоёв %v не равен порядку мест", fit, sz, stack)
			}
			a, b := s.Grid[0], s.Grid[1]
			if a.X+a.W <= b.X || b.X+b.W <= a.X || a.Y+a.H <= b.Y || b.Y+b.H <= a.Y {
				t.Errorf("%q %v: места не нахлёстываются: %v %v", fit, sz, a, b)
			}
		}
	}
}
