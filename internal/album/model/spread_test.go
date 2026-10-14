package model

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// Без обложки зоны нулевые, а список листов — сами развороты.
func TestCoverGeoNoCover(t *testing.T) {
	p := New()
	p.Spreads = []*Spread{{ID: "a"}, {ID: "b"}}
	if g := p.CoverGeo(); g != (CoverGeo{}) {
		t.Fatalf("зоны без обложки: %+v", g)
	}
	if got := p.Sheets(); !slices.Equal(got, p.Spreads) {
		t.Fatalf("листы без обложки: %d", len(got))
	}
}

// LoadOrder не режет полный стек: MaxPlaces мест и 64 текста (спека 026), все тексты под местами.
func TestLoadOrderFullStack(t *testing.T) {
	s := &Spread{ID: "s"}
	var order []string
	for range 64 {
		s.Texts = append(s.Texts, &TextBlock{ID: NewID(), W: 50, Text: "т"})
		order = append(order, s.Texts[len(s.Texts)-1].ID)
	}
	for i := range MaxPlaces {
		s.Grid = append(s.Grid, &Place{ID: NewID(), X: float64(i * 10), W: 10, H: 10})
		order = append(order, s.Grid[i].ID)
	}
	s.Order = slices.Clone(order)
	p := &Project{Spreads: []*Spread{s}}
	p.LoadOrder()
	if !slices.Equal(s.Order, order) {
		t.Fatalf("стек %d из %d", len(s.Order), len(order))
	}
}

// Страж empty(): любое поле разворота, кроме производных, делает его непустым.
// Новое поле с данными пользователя без строки в empty() этот тест уронит.
func TestSpreadEmptyCoversFields(t *testing.T) {
	// Order — производное от мест и текстов: в empty() его нет, иначе устаревший
	// Order (normalize раньше syncOrder) не дал бы удалить опустевший разворот.
	derived := map[string]bool{"ID": true, "Layout": true, "Variant": true, "Order": true}
	if !(&Spread{ID: "s", Layout: "H(0,1)", Variant: 2}).Empty() {
		t.Fatal("разворот без данных не пустой")
	}
	typ := reflect.TypeFor[Spread]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if derived[f.Name] {
			continue
		}
		var s Spread
		v := reflect.ValueOf(&s).Elem().Field(i)
		switch v.Kind() {
		case reflect.String:
			v.SetString("x")
		case reflect.Slice:
			v.Set(reflect.MakeSlice(f.Type, 1, 1))
		case reflect.Int:
			v.SetInt(1)
		case reflect.Pointer:
			v.Set(reflect.New(f.Type.Elem()))
		default:
			t.Fatalf("поле %s: тип %s — дописать в тест и в empty()", f.Name, f.Type)
		}
		if s.Empty() {
			t.Errorf("поле %s не учтено в empty()", f.Name)
		}
	}
}

// coverProject — альбом 281×284 с n пустыми разворотами и обложкой пресета id.
func coverProject(t *testing.T, n int, id string) *Project {
	t.Helper()
	p := New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	for range n {
		p.Spreads = append(p.Spreads, &Spread{ID: NewID()})
	}
	pr, ok := CoverPresetOf(id)
	if !ok {
		t.Fatalf("нет пресета %s", id)
	}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID}, Spec: pr.Spec}
	return p
}

// Тонкие листы Фотосферы, 20 разворотов: корешок 14 мм, лист 623 × 331 (032, критерий 1).
func TestCoverGeoFsferaThin(t *testing.T) {
	p := coverProject(t, 20, "fsfera-thin")
	cg := p.CoverGeo()
	if cg.Spine != 14 || cg.Over || cg.N != 20 {
		t.Fatalf("корешок %v over %v n %d", cg.Spine, cg.Over, cg.N)
	}
	if cg.X != [8]float64{0, 23.5, 304.5, 304.5, 318.5, 318.5, 599.5, 623} || cg.Y != [4]float64{0, 23.5, 307.5, 331} {
		t.Fatalf("зоны X %v Y %v", cg.X, cg.Y)
	}
	g := p.Geo(p.Cover.Sheet)
	if g.W != 623 || g.H != 331 || g.Mid != 311.5 || !g.Layflat || len(g.Pages) != 2 {
		t.Fatalf("Geo обложки %+v", g)
	}
	if l, _ := p.AreaOf(p.Cover.Sheet, ScopeLeft); l != (layout.Rect{W: 304.5, H: 331}) {
		t.Fatalf("задняя %+v", l)
	}
	if r, _ := p.AreaOf(p.Cover.Sheet, ScopeRight); r != (layout.Rect{X: 318.5, W: 304.5, H: 331}) {
		t.Fatalf("лицевая %+v", r)
	}
	if s, _ := p.AreaOf(p.Cover.Sheet, ScopeSpread); s != (layout.Rect{W: 623, H: 331}) {
		t.Fatalf("весь лист %+v", s)
	}
	// Разворот — как раньше: 2·pageW, шов посередине.
	if g := p.Geo(p.Spreads[0]); g.W != 562 || g.H != 284 || g.Mid != 281 || len(g.Pages) != 2 || g.Pages[1].X != 281 {
		t.Fatalf("Geo разворота %+v", g)
	}
	// Своя сторонка — вместо страницы.
	p.Cover.Spec.W, p.Cover.Spec.H = 300, 200
	if g := p.Geo(p.Cover.Sheet); g.W != 23.5*2+300*2+14 || g.H != 247 {
		t.Fatalf("сторонка 300×200: %+v", g)
	}
}

// Нетпринт Премиум, 20 разворотов: 5,4 + 0,39·20 = 13,2 мм; расстав с двух сторон корешка.
func TestCoverSpineFormula(t *testing.T) {
	p := coverProject(t, 20, "netprint-premium")
	cg := p.CoverGeo()
	if cg.Spine != 13.2 || cg.Over {
		t.Fatalf("корешок %v", cg.Spine)
	}
	if want := 15 + 281 + 9 + 13.2 + 9 + 281 + 15.0; math.Abs(cg.X[7]-want) > 1e-9 || cg.X[3] != 305 || cg.X[4] != 318.2 {
		t.Fatalf("X %v, ждали ширину %v", cg.X, want)
	}
	if mm, over := (CoverSpec{A: 10}).Spine(500); mm != 10 || over {
		t.Fatalf("постоянный корешок: %v %v", mm, over)
	}
}

// Разворотов больше таблицы — последняя строка и over; граница таблицы — ещё в ней.
func TestCoverSpineOver(t *testing.T) {
	for _, c := range []struct {
		n    int
		mm   float64
		over bool
	}{{0, 6, false}, {5, 6, false}, {6, 8, false}, {57, 30, false}, {58, 30, true}, {200, 30, true}} {
		p := coverProject(t, c.n, "fsfera-thin")
		if cg := p.CoverGeo(); cg.Spine != c.mm || cg.Over != c.over {
			t.Errorf("n=%d: %v over %v, ждали %v %v", c.n, cg.Spine, cg.Over, c.mm, c.over)
		}
	}
}

// Обходы «по всем листам» видят обложку, операции над порядком альбома — нет (032 §1).
func TestCoverSheetsWalks(t *testing.T) {
	p := coverProject(t, 0, "fsfera-thin")
	cv := p.Cover.Sheet
	cv.Grid = []*Place{{ID: NewID(), X: 400, Y: 100, W: 100, H: 100, Photo: "c"}}
	cv.Photos = []string{"c"}
	cv.Texts = []*TextBlock{{ID: NewID(), X: 400, Y: 20, W: 50, Text: "т"}}
	cv.BgPhotoR = &Backdrop{Photo: "bg", X: 318.5, W: 304.5, H: 331}
	sp := &Spread{ID: NewID(), Photos: []string{"a", "b"}}
	p.Spreads = []*Spread{sp}
	for _, id := range []string{"a", "b", "c", "bg"} {
		p.Photos[id] = PhotoRef{Path: `C:\ф\` + id + ".jpg", W: 3, H: 2}
	}
	if sh := p.Sheets(); len(sh) != 2 || sh[0] != cv || sh[1] != sp {
		t.Fatalf("Sheets: обложка первой")
	}
	if p.Sheet(CoverID) != cv || p.Sheet(sp.ID) != sp || p.GridSpread(CoverID) != cv {
		t.Fatal("Sheet/GridSpread не находят обложку")
	}
	if _, s := p.Find(CoverID); s != nil {
		t.Fatal("Find нашёл обложку: операции над порядком альбома её не видят")
	}
	if !p.Placed("c") || !p.OnBackdrops()["bg"] || !p.PlaceIDs()[cv.Grid[0].ID] {
		t.Fatal("Placed/OnBackdrops/PlaceIDs без обложки")
	}
	if s, i := p.FindText(cv.Texts[0].ID); s != cv || i != 0 {
		t.Fatal("FindText без обложки")
	}
	p.Swap("a", "c")
	if cv.Grid[0].Photo != "a" || sp.Photos[0] != "c" {
		t.Fatalf("Swap через обложку: %v %v", cv.Grid[0].Photo, sp.Photos)
	}
	p.PruneRefs()
	if len(p.Photos) != 4 {
		t.Fatalf("PruneRefs потерял фото обложки: %v", p.Photos)
	}
	p.Detach([]string{"a"})
	if cv.Grid[0].Photo != "" || len(cv.Photos) != 0 {
		t.Fatal("Detach не снял фото с места обложки")
	}
	// Без обложки ID «cover» не находится.
	p.Cover = nil
	if p.Sheet(CoverID) != nil || len(p.Sheets()) != 1 || p.IsCover(cv) {
		t.Fatal("обложки нет, а лист находится")
	}
}

// TestCoverPruneRefs — фото мест и фона обложки остаются в Photos, фото только на фоне
// теряет свойства места (как на развороте), неиспользуемое уходит (032, критерий 9).
func TestCoverPruneRefs(t *testing.T) {
	p := coverProject(t, 0, "fsfera-thin")
	cv := p.Cover.Sheet
	cv.Grid = []*Place{{ID: NewID(), W: 50, H: 50, Photo: "pl"}}
	cv.Photos = []string{"pl"}
	cv.BgPhoto = &Backdrop{Photo: "bg", W: 100, H: 100}
	f := &Frame{S: 2}
	p.Photos = map[string]PhotoRef{"pl": {Frame: f}, "bg": {Frame: f}, "gone": {}}
	p.PruneRefs()
	if _, ok := p.Photos["gone"]; ok || p.Photos["pl"].Frame == nil || p.Photos["bg"].Frame != nil || len(p.Photos) != 2 {
		t.Fatalf("PruneRefs с обложкой: %+v", p.Photos)
	}
}

// WarmLayouts греет не только ряд «Авто», но и «Другие варианты» (счётчик «i / N» в View берёт
// их под замком): у обоих разворотов; с выключенным параллелизмом — не греет ничего.
func TestWarmLayoutsExtra(t *testing.T) {
	p := New()
	p.Photos = map[string]PhotoRef{}
	var ss []*Spread
	for _, n := range []int{4, 6} {
		s := &Spread{ID: NewID()}
		for i := range n {
			id := NewID()
			w, h := 300, 200
			if i%3 == 0 {
				w, h = 200, 300
			}
			p.Photos[id] = PhotoRef{W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		ss = append(ss, s)
	}
	warm := func() []bool {
		var out []bool
		for _, s := range ss {
			out = append(out, variety.Cached(p.Aspects(s.Photos), p.Params(), RowLen(len(s.Photos))))
		}
		return out
	}
	t.Cleanup(func() { SetParallelLayouts(true) })
	for _, par := range []bool{false, true} {
		SetParallelLayouts(par)
		variety.ResetCache()
		p.WarmLayouts(ss)
		if got := warm(); got[0] != par || got[1] != par {
			t.Fatalf("параллельно=%v: прогрето %v", par, got)
		}
	}
	p.WarmLayouts(nil) // пусто — не падает
}
