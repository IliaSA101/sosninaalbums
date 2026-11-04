package looks

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestLookTileFitPage (165, ревью 2): плитка шага с fit = "page" — живой рендер lookTile →
// PreviewTplWith — совпадает с листом после «Применить шаг» (места и предметы, ±0,1 мм) на шести
// форматах страницы, а украшение стоит в центре страницы (±1 мм).
func TestLookTileFitPage(t *testing.T) {
	tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Fit: model.FitPage, Page: []model.TplBg{{Color: "#ffffff"}, {Color: "#ffffff"}},
		Places: []model.Box{{X: 0.56, Y: 0.1, W: 0.4, H: 0.8}, {X: 0, Y: 0, W: 0.5, H: 0.4}},
		Stickers: []model.TplItem{
			{Anchor: model.Anchor{AX: -1, DX: 0.5, AY: 0, DY: 0.3}, W: 0.3, H: 0.3, Art: &model.Art{Kind: "star"}},
			{Anchor: model.Anchor{AX: 1, DX: -0.5, AY: 0, DY: 0.3}, W: 0.2, H: 0.2, Art: &model.Art{Kind: "star"}},
		}}
	look := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{{Name: "Титул", Tpl: tpl}}}
	for _, pre := range model.PagePresets()[:6] {
		pw, ph := pre.W, pre.H
		p := model.New()
		p.Settings.PageW, p.Settings.PageH = pw, ph
		p.Settings = model.ClampSettings(p.Settings)
		p.Spreads = []*model.Spread{{ID: "s1"}}
		ed := mustEditor(t, p)
		if _, _, _, err := ed.ApplyLookStep("s1", 1, model.LookRef{}, look, nil); err != nil {
			t.Fatalf("%s: %v", pre.Label, err)
		}
		sheet, _ := ed.Snapshot()
		tiles, ok := lookTile(look, 1, false, false, 2*pw/ph)
		if !ok || len(tiles) != 1 {
			t.Fatalf("%s: плитка %v %d", pre.Label, ok, len(tiles))
		}
		tp, _, err := album.PreviewTplWith(tiles[0], 2*pw, ph, nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", pre.Label, err)
		}
		a, b := sheet.Spreads[0], tp.Spreads[0]
		if len(a.Grid) != 2 || len(b.Grid) != 2 || len(a.Stickers) != 2 || len(b.Stickers) != 2 {
			t.Fatalf("%s: мест %d/%d, стикеров %d/%d", pre.Label, len(a.Grid), len(b.Grid), len(a.Stickers), len(b.Stickers))
		}
		fitSameBoxes(t, pre.Label, a, b)
		// Абсолютно: место {0, 0, 0,5 × 0,4} — навылет на всю ширину страницы (двойная подгонка
		// сдвинула бы его); лист и плитка сверены между собой выше.
		for name, sp := range map[string]*model.Spread{"лист": a, "плитка": b} {
			if g := sp.Grid[1]; math.Abs(g.X) > 0.1 || math.Abs(g.Y) > 0.1 || math.Abs(g.W-pw) > 0.1 {
				t.Errorf("%s: %s: место навылет %v, ждали X = 0, Y = 0, W = %.0f", pre.Label, name, g, pw)
			}
		}
		fitCentered(t, pre.Label, a.Stickers, pw)
	}
}

func fitNear(x, y float64) bool { return math.Abs(x-y) <= 0.1 }

// fitSameBoxes — места и стикеры листа a и плитки b совпадают по порядку (±0,1 мм).
func fitSameBoxes(t *testing.T, label string, a, b *model.Spread) {
	t.Helper()
	for i := range a.Grid {
		g, h := a.Grid[i], b.Grid[i]
		if !fitNear(g.X, h.X) || !fitNear(g.Y, h.Y) || !fitNear(g.W, h.W) || !fitNear(g.H, h.H) {
			t.Errorf("%s: место %d: лист %v, плитка %v", label, i, g, h)
		}
	}
	for i := range a.Stickers {
		g, h := a.Stickers[i], b.Stickers[i]
		if !fitNear(g.X, h.X) || !fitNear(g.Y, h.Y) || !fitNear(g.W, h.W) || !fitNear(g.H, h.H) {
			t.Errorf("%s: стикер %d: лист %v, плитка %v", label, i, g, h)
		}
	}
}

// fitCentered — украшения по центру своей страницы шириной pw (по горизонтали), ±1 мм.
func fitCentered(t *testing.T, label string, sts []*model.Sticker, pw float64) {
	t.Helper()
	for _, st := range sts {
		cx, want := st.X+st.W/2, pw/2
		if cx > pw {
			want = pw + pw/2
		}
		if math.Abs(cx-want) > 1 {
			t.Errorf("%s: центр стикера %.2f, центр страницы %.2f", label, cx, want)
		}
	}
}

// TestLookTileFitPageEdgePanorama: место заходит за сгиб чуть больше допуска — плитка и лист
// подгоняют шаг по-старому, ровно как тот же шаг без fit (без второй подгонки).
func TestLookTileFitPageEdgePanorama(t *testing.T) {
	mk := func(fit string) model.AlbumTpl {
		tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Fit: fit, Page: []model.TplBg{{Color: "#ffffff"}, {Color: "#ffffff"}},
			Places: []model.Box{{X: 0.3, Y: 0.1, W: 0.2015, H: 0.6}, {X: 0.6, Y: 0.1, W: 0.3, H: 0.3}}}
		return model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{{Name: "Титул", Tpl: tpl}}}
	}
	for _, sz := range [][2]float64{{300, 200}, {400, 200}, {200, 300}} {
		var got [2]*model.Spread
		for i, fit := range []string{"", model.FitPage} {
			tiles, ok := lookTile(mk(fit), 1, false, false, 2*sz[0]/sz[1])
			if !ok || len(tiles) != 1 {
				t.Fatalf("%v: плитка %v", sz, ok)
			}
			tp, _, err := album.PreviewTplWith(tiles[0], 2*sz[0], sz[1], nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got[i] = tp.Spreads[0]
		}
		if len(got[0].Grid) != 2 || len(got[1].Grid) != 2 {
			t.Fatalf("%v: мест %d/%d", sz, len(got[0].Grid), len(got[1].Grid))
		}
		fitSameBoxes(t, "плитка fit/без fit", got[0], got[1])
	}
}

func mustEditor(t testing.TB, p *model.Project) *album.Editor {
	t.Helper()
	e, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	return e
}
