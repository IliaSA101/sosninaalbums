package album_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Заполнить места» не меняется (0.27-131-10, 131 G4b): фото подряд во все пустые места, в том
// числе виньеток. Эталон снят с FillPlaces до правок 131 (grid/ops.go в них не входит).
func TestFillPlacesUnchanged(t *testing.T) {
	seated := func(e *album.Editor) (out [][]string) {
		for _, s := range edtest.P(e).Spreads {
			var ph []string
			for _, pl := range s.Grid {
				ph = append(ph, pl.Photo)
			}
			out = append(out, ph)
		}
		return out
	}
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("p%02d", i)
		}
		return out
	}
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	if _, left, _ := e.FillPlaces(edtest.Photos(ids(12)...), ""); left != 0 {
		t.Fatalf("не встало %d", left)
	}
	want := [][]string{{"p00"}, {"p01", "p02", "p03", "p04", "p05"}, {"p06", "p07"}, {"p08"}, {"p09", "p10", "p11"},
		{""}, {"", ""}, {"", "", "", "", "", "", ""}, {""}}
	if got := seated(e); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("посадка: %q, ждали %q", got, want)
	}
	// Фото больше мест (23): остаток — left.
	e2 := lkEditor(t)
	lkApply(t, e2, lkRefA, lookA(), nil, 9)
	if _, left, _ := e2.FillPlaces(edtest.Photos(ids(25)...), ""); left != 2 {
		t.Fatalf("left %d, ждали 2", left)
	}
	// Пустой альбом без мест и пустой список фото.
	e3 := lkEditor(t)
	if _, left, _ := e3.FillPlaces(edtest.Photos(ids(3)...), ""); left != 3 {
		t.Fatalf("left %d, ждали 3", left)
	}
	if _, left, _ := e2.FillPlaces(nil, ""); left != 0 {
		t.Fatalf("пустой список: left %d", left)
	}
}

// lkGeoms — места не-виньеток по разворотам: x, y, w, h.
func lkGeoms(e *album.Editor) [][][4]float64 {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	var out [][][4]float64
	for _, s := range edtest.P(e).Spreads {
		var g [][4]float64
		if !s.IsVignette() {
			for _, pl := range s.Grid {
				g = append(g, [4]float64{pl.X, pl.Y, pl.W, pl.H})
			}
		}
		out = append(out, g)
	}
	return out
}

// lkSameGeoms — места двух альбомов совпадают с точностью 0,01 мм; skip — развороты, которые не сверяются.
func lkSameGeoms(a, b [][][4]float64, skip ...int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if slices.Contains(skip, i) {
			continue
		}
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			for k := range a[i][j] {
				if abs(a[i][j][k]-b[i][j][k]) > 0.0101 {
					return false
				}
			}
		}
	}
	return true
}

// lkFormat — альбом нового формата pw×300 мм с макетом B из 6 разворотов (оракул смены формата).
func lkFormat(t *testing.T, pw float64) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW = pw
	e := edtest.Open(t, p, "")
	lkApply(t, e, lkRefB, lookB(), nil, 6)
	return e
}

func lkResize(e *album.Editor, pw float64, look *model.AlbumTpl) {
	edtest.Mu(e).Lock()
	s := edtest.P(e).Settings
	edtest.Mu(e).Unlock()
	s.PageW = pw
	e.SetSettingsLook(s, look)
}

// Смена формата с макетом: нетронутые места вписываются заново (как поставила бы серия на новом
// формате), тронутое тянется Rescale; обратно — прежние места; без макета (nil) места тянутся по
// осям и композиция теряет пропорции.
func TestLookPlacesKeepAspectOnResize(t *testing.T) {
	b := lookB()
	e, twin, plain := lkFormat(t, 300), lkFormat(t, 300), lkFormat(t, 300)
	orig := lkGeoms(e)
	const touched = 3 // повтор: место сдвинуто руками — тронутый разворот
	for _, x := range []*album.Editor{e, twin, plain} {
		edtest.Mu(x).Lock()
		edtest.P(x).Spreads[touched].Grid[0].X += 7
		edtest.Mu(x).Unlock()
	}
	json0, rev, undo := lkJSON(e), e.View().Rev, edtest.UndoLen(e)
	for _, pw := range []float64{200, 400, 300} {
		lkResize(e, pw, &b)
		lkResize(twin, pw, nil)
		lkResize(plain, pw, nil)
		fresh := lkFormat(t, pw)
		if !lkSameGeoms(lkGeoms(e), lkGeoms(fresh), touched) {
			t.Fatalf("ширина %v: нетронутые места не равны свежей серии на новом формате", pw)
		}
		if !lkSameGeoms(lkGeoms(e)[touched:touched+1], lkGeoms(twin)[touched:touched+1]) {
			t.Fatalf("ширина %v: тронутое место должно идти по Rescale", pw)
		}
		if pw != 300 && lkSameGeoms(lkGeoms(plain), lkGeoms(fresh), touched) {
			t.Fatalf("ширина %v: без макета места не должны совпасть со свежей серией (тест ничего не проверяет)", pw)
		}
	}
	// Круг 30×30 → 20×30 → 40×30 → 30×30: нетронутые места те же, что были.
	if !lkSameGeoms(lkGeoms(e), orig, touched) {
		t.Fatal("после круга форматов места не вернулись")
	}
	if !lkSameGeoms(lkGeoms(e)[touched:touched+1], lkGeoms(twin)[touched:touched+1]) {
		t.Fatal("тронутое место ушло от Rescale")
	}
	if edtest.UndoLen(e) != undo+3 || e.View().Rev != rev+3 {
		t.Fatalf("шагов отмены %d (ждали %d), rev %d", edtest.UndoLen(e), undo+3, e.View().Rev)
	}
	for range 3 {
		e.Undo()
	}
	if lkJSON(e) != json0 {
		t.Fatal("Undo не вернул альбом")
	}
}

// Края: макета нет на ПК (nil) и негодный макет — как SetSettings; шаг макета вне его длины и
// место с «Вписать» — тронуты; тот же формат — холостая правка.
func TestLookPlacesResizeEdges(t *testing.T) {
	b := lookB()
	e := lkFormat(t, 300)
	rev, data := e.View().Rev, lkJSON(e)
	lkResize(e, 300, &b)
	if e.View().Rev != rev || lkJSON(e) != data {
		t.Fatal("тот же формат: правка должна быть холостой")
	}
	short := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Один", "#ffffff", 1)}}
	for _, look := range []*model.AlbumTpl{nil, {}, &short} {
		x, ref := lkFormat(t, 300), lkFormat(t, 300)
		lkResize(x, 200, look)
		lkResize(ref, 200, nil)
		if !lkSameGeoms(lkGeoms(x), lkGeoms(ref), 0) {
			t.Fatalf("макет %v: ждали Rescale", look)
		}
	}
	// Вписать — тронутое место: тянется по осям.
	x, ref := lkFormat(t, 300), lkFormat(t, 300)
	for _, y := range []*album.Editor{x, ref} {
		edtest.Mu(y).Lock()
		edtest.P(y).Spreads[2].Grid[0].Fit = "contain"
		edtest.Mu(y).Unlock()
	}
	lkResize(x, 200, &b)
	lkResize(ref, 200, nil)
	if !lkSameGeoms(lkGeoms(x)[2:3], lkGeoms(ref)[2:3]) {
		t.Fatal("место с «Вписать» должно тянуться Rescale")
	}
}
