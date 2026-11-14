package spreads_test

// 105 «Разброс»: плитки в «Других вариантах» галереи (центр с боками n первым, затем по плитке на
// профиль), выбор одним шагом отмены, сгиб при «Только широкие» и поле foldHidden.

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// scatterEd — альбом из одного разворота с n вертикальными 2:3 при положении сгиба foldAny.
func scatterEd(t *testing.T, n int, foldAny bool) (*album.Editor, string) {
	t.Helper()
	v := [2]int{200, 300}
	wh := make([][2]int, n)
	for i := range wh {
		wh[i] = v
	}
	e := edtest.Open(t, model.New(), "")
	setFold(e, true, foldAny)
	return e, edtest.SpreadIDs(edtest.NewSpread(e, -1, whPhotos("z", wh...)))[0]
}

// scatterTiles — плитки «Разброса» в «Других вариантах» подборки.
func scatterTiles(ch model.ChoiceRows) []model.Choice {
	return slices.DeleteFunc(slices.Clone(ch.Extra), func(c model.Choice) bool {
		return !strings.Contains(c.Key, "Z(")
	})
}

// TestGalleryScatter: «Любые фото», 3–5 вертикальных — «Другие варианты» начинаются с центра, дальше
// 7 «Разброс» в порядке профилей; у плиток семья «Фигуры» и название «Разброс».
func TestGalleryScatter(t *testing.T) {
	for _, n := range []int{3, 4, 5} {
		e, id := scatterEd(t, n, true)
		ex := e.LayoutChoices(id).Whole.Extra
		tiles := scatterTiles(e.LayoutChoices(id).Whole)
		if len(tiles) != 7 || len(ex) < 8 {
			t.Fatalf("%d фото: плиток «Разброса» %d, других вариантов %d", n, len(tiles), len(ex))
		}
		if !strings.HasPrefix(ex[0].Key, "H(") || !strings.Contains(ex[0].Key, "n") || ex[0].FormName != "Мелкое — крупное" {
			t.Errorf("%d фото: первым не центр n: %q (%s)", n, ex[0].Key, ex[0].FormName)
		}
		for i, c := range tiles {
			if ex[1+i].Key != c.Key {
				t.Errorf("%d фото: плитка %d — %q, не подряд после центра (%q)", n, i, c.Key, ex[1+i].Key)
			}
			if c.Family != "figure" || c.FormName != "Разброс" || len(c.Rects) != n {
				t.Errorf("%d фото: %q — семья %q, название %q, фото %d", n, c.Key, c.Family, c.FormName, len(c.Rects))
			}
		}
	}
}

// TestGalleryScatterPick: щелчок по плитке — один шаг отмены, ключ и геометрия плитки, круг Save → Load;
// Ctrl+Z возвращает «Авто»; повтор того же ключа — холостая правка (ни rev, ни шага).
func TestGalleryScatterPick(t *testing.T) {
	e, id := scatterEd(t, 3, true)
	tiles := scatterTiles(e.LayoutChoices(id).Whole)
	if len(tiles) == 0 {
		t.Fatal("нет плиток «Разброса»")
	}
	want := tiles[2]
	undo := edtest.UndoLen(e)
	v := e.SetLayout(id, want.Key)
	if edtest.UndoLen(e) != undo+1 || v.Spreads[0].Layout != want.Key || !slices.Equal(v.Spreads[0].Rects, want.Rects) {
		t.Fatalf("после щелчка: ключ %q, шагов %d, геометрия как у плитки %v", v.Spreads[0].Layout, edtest.UndoLen(e)-undo,
			slices.Equal(v.Spreads[0].Rects, want.Rects))
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetLayout(id, want.Key)
	edtest.Same(t, e, rev, undo, "повтор плитки «Разброса»")

	q := edtest.SaveLoad(t, e)
	if q.Spreads[0].Layout != want.Key || edtest.From(q) != model.FormatVersion {
		t.Fatalf("Save → Load: ключ %q (ждали %q), формат %d", q.Spreads[0].Layout, want.Key, edtest.From(q))
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("отмена: ключ %q, ждали «Авто»", edtest.P(e).Spreads[0].Layout)
	}
}

// TestGalleryScatterFold: «Только широкие» и 3 вертикальных — все 7 плиток по страницам (2 + 1), на
// сгибе ничего, foldHidden = 8 (7 Z и центр); «Любые фото» — на разворот, foldHidden нет; 4 фото —
// Z на разворот, скрыты только оба центра; 6 фото и 2 фото — нечего скрывать.
func TestGalleryScatterFold(t *testing.T) {
	e, id := scatterEd(t, 3, false)
	tiles := scatterTiles(e.LayoutChoices(id).Whole)
	if len(tiles) != 7 {
		t.Fatalf("широкие, 3 фото: плиток %d", len(tiles))
	}
	for _, c := range tiles {
		if !strings.HasPrefix(c.Key, "P[Z(") {
			t.Errorf("широкие: %q не по страницам", c.Key)
		}
	}
	if got := e.View().Spreads[0].FoldHidden; got != 8 {
		t.Errorf("широкие, 3 фото: foldHidden %d, ждали 8", got)
	}
	for _, c := range []struct {
		n       int
		foldAny bool
		want    int
	}{{3, true, 0}, {4, false, 2}, {4, true, 0}, {5, false, 8}, {2, false, 0}, {6, false, 0}} {
		e, _ := scatterEd(t, c.n, c.foldAny)
		if got := e.View().Spreads[0].FoldHidden; got != c.want {
			t.Errorf("%d фото, любые %v: foldHidden %d, ждали %d", c.n, c.foldAny, got, c.want)
		}
	}
}

// TestScatterFoldHiddenReal: реальные пропорции стока (QA 23.10) — foldHidden считает раскладки,
// отказанные только сгибом: 4 и 5 вертикальных с одним шире (1708×2000) дают 7 (центра нет), почти
// 2:3 — 2 и 8.
func TestScatterFoldHiddenReal(t *testing.T) {
	a, b, w := [2]int{1331, 2000}, [2]int{1333, 2000}, [2]int{1708, 2000}
	for _, c := range []struct {
		wh   [][2]int
		want int
	}{
		{[][2]int{a, b, b}, 8}, {[][2]int{a, b, b, b}, 2}, {[][2]int{a, b, b, b, b}, 8},
		{[][2]int{a, b, b, w}, 7}, {[][2]int{a, b, b, w, b}, 7},
	} {
		e := edtest.Open(t, model.New(), "")
		setFold(e, true, false)
		v := edtest.NewSpread(e, -1, whPhotos("r", c.wh...))
		if got := v.Spreads[0].FoldHidden; got != c.want {
			t.Errorf("%d фото %v: foldHidden %d, ждали %d", len(c.wh), c.wh, got, c.want)
		}
	}
}

// TestScatterPickedStaysOnWide:выбранная при «Любых фото» плитка с фото на сгибе (Z и центр n) после
// смены на «Только широкие» стоит, как любой выбранный руками ключ (владелец 17.10: «уже свёрстанное
// не трогается»; 105-qa-2): ключ и геометрия те же, variant = -1 (в ряд «Широких» ключа нет), смена —
// один шаг отмены, повтор холостой, Save → Load и отмена возвращают как было. Два разворота сразу.
func TestScatterPickedStaysOnWide(t *testing.T) {
	e, id := scatterEd(t, 3, true)
	wh := [2]int{200, 300}
	edtest.NewSpread(e, -1, whPhotos("y", wh, wh, wh))
	ids := edtest.SpreadIDs(e.View())
	ex := e.LayoutChoices(id).Whole.Extra
	z := scatterTiles(e.LayoutChoices(id).Whole)[2]
	center := ex[0]
	e.SetLayout(ids[0], z.Key)
	e.SetLayout(ids[1], center.Key)
	before := rectsOf(e.View())

	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v := setFold(e, true, false)
	if edtest.UndoLen(e) != undo+1 || v.Rev == rev {
		t.Fatalf("смена настройки: шагов %d", edtest.UndoLen(e)-undo)
	}
	for i, key := range []string{z.Key, center.Key} {
		sv := v.Spreads[i]
		if sv.Layout != key || sv.Variant != -1 {
			t.Errorf("разворот %d: ключ %q (ждали %q), variant %d (ждали -1)", i, sv.Layout, key, sv.Variant)
		}
	}
	if got := rectsOf(v); !slices.EqualFunc(got, before, slices.Equal) {
		t.Fatal("выбранные раскладки сдвинулись")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	setFold(e, true, false)
	edtest.Same(t, e, rev, undo, "повтор «Только широкие»")

	if got := layoutsOf(edtest.SaveLoad(t, e)); !slices.Equal(got, []string{z.Key, center.Key}) {
		t.Fatalf("Save → Load: ключи %q", got)
	}
	e.Undo()
	if p := edtest.P(e); !p.Settings.FoldAny || !slices.Equal(layoutsOf(p), []string{z.Key, center.Key}) {
		t.Fatalf("отмена: foldAny %v, ключи %q", p.Settings.FoldAny, layoutsOf(p))
	}
}

// TestPageChoicesScatter: у P[Z…|Z…] подборка страницы меняет одну страницу, вторая остаётся.
func TestPageChoicesScatter(t *testing.T) {
	e, id := scatterEd(t, 3, false)
	var key string
	for _, c := range scatterTiles(e.LayoutChoices(id).Whole) {
		if strings.Count(c.Key, "Z(") == 2 {
			key = c.Key
			break
		}
	}
	if key == "" {
		t.Fatal("нет плитки P[Z|Z]")
	}
	e.SetLayout(id, key)
	ch := e.LayoutChoices(id)
	parts := func(k string) [2]string {
		in := strings.TrimSuffix(strings.TrimPrefix(k, "P["), "]")
		l, r, _ := strings.Cut(in, "|")
		return [2]string{l, r}
	}
	cur := parts(key)
	for side, rows := range []*model.ChoiceRows{ch.Left, ch.Right} {
		if rows == nil || len(rows.InOrder) == 0 {
			t.Fatalf("страница %d: подборки нет", side)
		}
		for _, c := range append(slices.Clone(rows.InOrder), rows.Rearranged...) {
			if !strings.HasPrefix(c.Key, "P[") {
				t.Errorf("страница %d: %q — не постраничный ключ", side, c.Key)
				continue
			}
			if p := parts(c.Key); p[1-side] != cur[1-side] {
				t.Errorf("страница %d: %q меняет и вторую страницу (%q → %q)", side, c.Key, cur[1-side], p[1-side])
			}
		}
	}
}
