package spreads_test

// 104 «Подрезка, чтобы закрыть лист»: настройка альбома, ключи «Авто», отмена, «Сделать ручным».

import (
	"bytes"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// ownerKey — случай владельца (спека 104): шесть вертикальных 2:3 на 300×305, поля 3/3.
const ownerKey = "H(0,V(1,2),3,V(4,5))"

// fitEd — «Фабрика» 300×305, поля 3/3; три листа: шесть вертикальных с ключом ownerKey (подрезка
// 5 % закрывает лист), два горизонтальных (закрывать нечего) и три вертикальных + горизонтальное.
func fitEd(t *testing.T) (*album.Editor, []string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Margin, st.Gap, st.FoldAny = 300, 305, 3, 3, true // «Любые фото»: узкое на сгибе допустимо
	e.SetSettings(st)
	v, h := [2]int{200, 300}, [2]int{300, 200}
	edtest.NewSpread(e, -1, whPhotos("v", v, v, v, v, v, v))
	edtest.NewSpread(e, -1, whPhotos("h", h, h))
	ids := edtest.SpreadIDs(edtest.NewSpread(e, -1, whPhotos("k", v, v, v, h)))
	e.SetLayout(ids[0], ownerKey)
	// Лучшая в ряду хранится как "": смотрим действующий ключ.
	if cur, _, ok := edtest.P(e).Layout(edtest.P(e).Spreads[0]); !ok || cur.Key != ownerKey {
		t.Fatalf("ключ владельца не встал: %q", cur.Key)
	}
	return e, ids
}

func setCrop(e *album.Editor, c int) model.View {
	st := edtest.P(e).Settings
	st.Crop = c
	return e.SetSettings(st)
}

// TestSetSettingsCropKeeps — «Нет» → «до 5 %» одним шагом отмены: выбранные ключи остаются теми же,
// блок, которому не хватает меньше предела, растягивается и закрывает лист (рамка = область в полях),
// остальные листы — побитно прежние. Повтор — холостая правка; значение вне шагов зажимается; Save →
// Load держит настройку и ключи; Ctrl+Z возвращает проект и вид побитно.
func TestSetSettingsCropKeeps(t *testing.T) {
	e, _ := fitEd(t)
	before, keys, bytesBefore := rectsOf(e.View()), layoutsOf(edtest.P(e)), model.Marshal(edtest.P(e))
	if edtest.P(e).Settings.Crop != 0 {
		t.Fatalf("новый альбом: crop %d", edtest.P(e).Settings.Crop)
	}

	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v := setCrop(e, 5)
	if edtest.P(e).Settings.Crop != 5 || edtest.UndoLen(e) != undo+1 || v.Rev == rev {
		t.Fatalf("crop %d, шагов %d", edtest.P(e).Settings.Crop, edtest.UndoLen(e)-undo)
	}
	if got := layoutsOf(edtest.P(e)); !slices.Equal(got, keys) {
		t.Fatalf("ключи сменились: %q → %q", keys, got)
	}
	got := rectsOf(v)
	// Первый лист: то же дерево, но растянутое — рамка закрывает область в полях.
	if slices.EqualFunc(got[0], before[0], func(a, b layout.Rect) bool { return a == b }) {
		t.Fatal("шесть вертикальных не растянулись")
	}
	p := edtest.P(e)
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range got[0] {
		x0, y0, x1, y1 = min(x0, r.X), min(y0, r.Y), max(x1, r.X+r.W), max(y1, r.Y+r.H)
	}
	m, w, h := p.Settings.Margin, 2*p.Settings.PageW, p.Settings.PageH
	for name, d := range map[string]float64{"слева": x0 - m, "сверху": y0 - m, "справа": w - m - x1, "снизу": h - m - y1} {
		if math.Abs(d) > 0.01 {
			t.Errorf("рамка не закрывает лист %s на %.3f мм", name, d)
		}
	}
	// Остальные листы: что считает Build с подрезкой, то и показано; ключ тот же.
	for i := 1; i < 3; i++ {
		asp, prm := p.LayoutIn(p.Spreads[i].Photos)
		cur, _, ok := p.Layout(p.Spreads[i])
		r, built := layout.Build(cur.Key, asp, prm)
		if !ok || !built || !slices.Equal(got[i], r.Rects) {
			t.Errorf("лист %d: показано не то, что строит Build с подрезкой", i)
		}
	}
	if !slices.Equal(got[1], before[1]) {
		t.Errorf("два горизонтальных закрыть нечем — должны остаться прежними:\n%v\n%v", got[1], before[1])
	}

	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	setCrop(e, 5)
	edtest.Same(t, e, rev, undo, "повтор «до 5 %»")
	for in, want := range map[int]int{7: 5, 8: 10, 99: 15, -4: 0, 2: 0, 13: 15} { // край, середина, предел
		setCrop(e, in)
		if g := edtest.P(e).Settings.Crop; g != want {
			t.Fatalf("crop %d → %d, ждали %d", in, g, want)
		}
	}
	setCrop(e, 5)

	q := edtest.SaveLoad(t, e)
	if q.Settings.Crop != 5 || !slices.Equal(layoutsOf(q), keys) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("Save → Load: crop %d, ключи %q, from %d", q.Settings.Crop, layoutsOf(q), edtest.From(q))
	}

	// Отмена: все шаги подрезки назад — проект и вид побитно прежние.
	for edtest.P(e).Settings.Crop != 0 && edtest.UndoLen(e) > 0 {
		e.Undo()
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), bytesBefore) || !slices.EqualFunc(rectsOf(e.View()), before, slices.Equal) {
		t.Fatal("отмена не вернула проект и вид побитно")
	}
}

// TestCropFreezeSameLook — «Сделать ручным» берёт растянутые места (с округлением места до 0,01 мм): вид тот же, что у «Авто» с подрезкой,
// а после отмены закрепления и подрезки места и «Авто» прежние. Несколько листов: закрепляется один.
func TestCropFreezeSameLook(t *testing.T) {
	e, _ := fitEd(t)
	v := setCrop(e, 10)
	auto := rectsOf(v)
	edtest.FreezeAt(t, e, 0)
	grid := edtest.P(e).Spreads[0].Grid
	if len(grid) != 6 {
		t.Fatalf("мест %d", len(grid))
	}
	for i, pl := range grid {
		r := auto[0][i]
		if math.Abs(pl.X-r.X) > 0.006 || math.Abs(pl.Y-r.Y) > 0.006 || math.Abs(pl.W-r.W) > 0.006 || math.Abs(pl.H-r.H) > 0.006 {
			t.Errorf("место %d %+v ≠ растянутый %+v", i, *pl, r)
		}
	}
	if got := rectsOf(e.View()); !slices.Equal(got[1], auto[1]) || !slices.Equal(got[2], auto[2]) {
		t.Error("закрепление первого листа сдвинуло соседей")
	}
}
