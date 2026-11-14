package spreads_test

// 104 G3: подрезка в плитках галереи и «Что соберётся».

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// wantCrop — подрезка раскладки key разворота s в целых процентах вверх, как у плитки.
func wantCrop(t *testing.T, p *model.Project, s *model.Spread, key string) int {
	t.Helper()
	asp, prm := p.LayoutIn(s.Photos)
	r, ok := layout.Build(key, asp, prm)
	if !ok {
		t.Fatalf("ключ %q не строится", key)
	}
	return int(math.Ceil(100*r.Crop - 1e-9))
}

// choiceRowsOf — все ряды плиток разворота: целый и страниц.
func choiceRowsOf(c model.Choices) [][]model.Choice {
	rows := [][]model.Choice{c.Whole.InOrder, c.Whole.Rearranged, c.Whole.Extra}
	for _, side := range []*model.ChoiceRows{c.Left, c.Right} {
		if side != nil {
			rows = append(rows, side.InOrder, side.Rearranged)
		}
	}
	return rows
}

// cropTiles — проверка плиток листа id: подрезка в 0…limit, ряд «Авто» совпадает с Build;
// возвращает число плиток с подрезкой.
func cropTiles(t *testing.T, e *album.Editor, id string, limit int) (stretched int) {
	t.Helper()
	p := edtest.P(e)
	_, s := p.Find(id)
	c := e.LayoutChoices(id)
	for _, row := range choiceRowsOf(c) {
		for _, ch := range row {
			if ch.Crop < 0 || ch.Crop > limit {
				t.Errorf("%s %s: подрезка %d вне 0…%d", id, ch.Key, ch.Crop, limit)
			}
			if ch.Crop > 0 {
				stretched++
			}
		}
	}
	for _, ch := range c.Whole.InOrder {
		if got := wantCrop(t, p, s, ch.Key); ch.Crop != got {
			t.Errorf("%s %s: подрезка %d, Build даёт %d", id, ch.Key, ch.Crop, got)
		}
	}
	return stretched
}

// TestChoiceCrop: плитка несёт подрезку целым процентом вверх; без подрезки в настройках — 0;
// предел не превышен. Ряд «Авто», «Другие варианты» и ряды страниц, несколько листов, Ctrl+Z
// возвращает нули.
func TestChoiceCrop(t *testing.T) {
	e, ids := fitEd(t)
	check := func(limit int) (n int) {
		for _, id := range ids {
			n += cropTiles(t, e, id, limit)
		}
		return n
	}
	if n := check(0); n != 0 {
		t.Fatalf("crop 0: растянутых плиток %d", n)
	}
	setCrop(e, 5)
	if n := check(5); n == 0 {
		t.Fatal("crop 5: ни одной плитки с подрезкой")
	}
	setCrop(e, 15)
	if n := check(15); n == 0 {
		t.Fatal("crop 15: ни одной плитки с подрезкой")
	}
	e.Undo()
	e.Undo()
	if n := check(0); n != 0 {
		t.Fatalf("после отмены: растянутых плиток %d", n)
	}
}

// TestPreviewCrop: «Что соберётся» несёт подрезку каждого разворота, как собранная раскладка;
// без подрезки в настройках — нули.
func TestPreviewCrop(t *testing.T) {
	stretched := 0
	for _, crop := range []int{0, 15} {
		e, ph := previewAlbum(t)
		setCrop(e, crop)
		o := model.AutoOpts{Spreads: 8, MinPer: 3, MaxPer: 9, Variety: model.VarietyNormal}
		pv, err := e.AutoPreview(ph, -1, o)
		if err != nil {
			t.Fatal(err)
		}
		was := map[string]bool{}
		for _, s := range edtest.P(e).Spreads {
			was[s.ID] = true
		}
		build(t, e, ph, -1, o)
		got := built(edtest.P(e), was)
		if len(got) != len(pv.Spreads) {
			t.Fatalf("crop %d: собрано %d, показано %d", crop, len(got), len(pv.Spreads))
		}
		for i, s := range got {
			p := edtest.P(e)
			r, _, _ := p.Layout(s)
			want := wantCrop(t, p, s, r.Key)
			if pv.Spreads[i].Crop != want || (crop == 0 && want != 0) {
				t.Errorf("crop %d, разворот %d: показано %d, собрано %d", crop, i, pv.Spreads[i].Crop, want)
			}
			if want > 0 {
				stretched++
			}
		}
	}
	t.Logf("растянутых разворотов в сборке: %d", stretched)
}
