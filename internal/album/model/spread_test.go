package model

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
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
		case reflect.Bool:
			v.SetBool(true)
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
	pr, ok := CoverPresetFor(id, p.Settings)
	if !ok {
		t.Fatalf("нет пресета %s", id)
	}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID}, Spec: pr.Spec}
	return p
}

// Тонкие листы Фотосферы 30×30 (281×284), 20 разворотов: шаблон 047 — сторонка 286×286,
// заворот 20, корешок 14 мм, лист 626 × 326 (CoverPresetFor; 7394 × 3850 px при 300 dpi).
func TestCoverGeoFsferaThin(t *testing.T) {
	p := coverProject(t, 20, "fsfera-thin")
	cg := p.CoverGeo()
	if cg.Spine != 14 || cg.Over || cg.N != 20 {
		t.Fatalf("корешок %v over %v n %d", cg.Spine, cg.Over, cg.N)
	}
	if cg.X != [8]float64{0, 20, 306, 306, 320, 320, 606, 626} || cg.Y != [4]float64{0, 20, 306, 326} {
		t.Fatalf("зоны X %v Y %v", cg.X, cg.Y)
	}
	g := p.Geo(p.Cover.Sheet)
	if g.W != 626 || g.H != 326 || g.Mid != 313 || !g.Layflat || len(g.Pages) != 2 {
		t.Fatalf("Geo обложки %+v", g)
	}
	if l, _ := p.AreaOf(p.Cover.Sheet, ScopeLeft); l != (layout.Rect{W: 306, H: 326}) {
		t.Fatalf("задняя %+v", l)
	}
	if r, _ := p.AreaOf(p.Cover.Sheet, ScopeRight); r != (layout.Rect{X: 320, W: 306, H: 326}) {
		t.Fatalf("лицевая %+v", r)
	}
	if s, _ := p.AreaOf(p.Cover.Sheet, ScopeSpread); s != (layout.Rect{W: 626, H: 326}) {
		t.Fatalf("весь лист %+v", s)
	}
	// Разворот — как раньше: 2·pageW, шов посередине.
	if g := p.Geo(p.Spreads[0]); g.W != 562 || g.H != 284 || g.Mid != 281 || len(g.Pages) != 2 || g.Pages[1].X != 281 {
		t.Fatalf("Geo разворота %+v", g)
	}
	// Своя сторонка — вместо шаблона; 0 — страница.
	p.Cover.Spec.W, p.Cover.Spec.H = 300, 200
	if g := p.Geo(p.Cover.Sheet); g.W != 20*2+300*2+14 || g.H != 240 {
		t.Fatalf("сторонка 300×200: %+v", g)
	}
	p.Cover.Spec.W, p.Cover.Spec.H = 0, 0
	if g := p.Geo(p.Cover.Sheet); g.W != 20*2+281*2+14 || g.H != 324 {
		t.Fatalf("сторонка — страница: %+v", g)
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
	if !p.Placed("c") || !p.OnDecor()["bg"] || !p.PlaceIDs()[cv.Grid[0].ID] {
		t.Fatal("Placed/OnDecor/PlaceIDs без обложки")
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

// VigHead: пустой ID и ссылка на несуществующую надпись — nil, своя надпись — именно она, не первая.
func TestVigHeadLookup(t *testing.T) {
	a, b := &TextBlock{ID: "aaaaaaaaaaaa"}, &TextBlock{ID: "bbbbbbbbbbbb"}
	s := &Spread{Texts: []*TextBlock{a, b}}
	// Надпись без ID (старый файл) не должна находиться по пустой ссылке на шапку.
	noID := &Spread{Texts: []*TextBlock{{}, a}}
	if noID.VigHead("") != nil || noID.VigHead(a.ID) != a {
		t.Fatal("пустая ссылка нашла надпись без ID")
	}
	if s.VigHead("") != nil || s.VigHead("cccccccccccc") != nil || (&Spread{}).VigHead(a.ID) != nil {
		t.Fatal("пустой ID или чужая ссылка дали надпись")
	}
	if s.VigHead(a.ID) != a || s.VigHead(b.ID) != b {
		t.Fatal("надпись найдена не по ID")
	}
}

// fsferaProject — альбом на странице w×h с n пустыми разворотами и обложкой spec.
func fsferaProject(w, h float64, n int, spec CoverSpec) *Project {
	p := New()
	p.Settings.PageW, p.Settings.PageH = w, h
	for range n {
		p.Spreads = append(p.Spreads, &Spread{ID: NewID()})
	}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID}, Spec: spec}
	return p
}

// Семь форматов Фотосферы (047): файл разворота и обложка «Тонкие листы» при корешке 6 мм
// в px при 300 dpi совпадают с PSD (fsfera-pro-research.md §2); ID уникальны, зона
// безопасности 10 (у 25×25 и 21×30 — 8) и не больше четверти меньшей стороны (ClampSettings).
func TestFsferaFormats(t *testing.T) {
	px := func(mm float64) int { return int(math.Round(mm / 25.4 * 300)) }
	want := map[string][4]int{
		"20×15": {4748, 1795, 5409, 2315}, "20×20": {4748, 2398, 5409, 2906}, "20×30": {4748, 3224, 5409, 3732},
		"30×20": {6756, 2398, 7370, 2906}, "25×25": {5929, 3000, 6591, 3496}, "30×30": {6638, 3354, 7299, 3850},
		"21×30": {4913, 3496, 5433, 3874},
	}
	all := FsferaFormats()
	if len(all) != len(want) {
		t.Fatalf("форматов %d", len(all))
	}
	ids := map[string]bool{}
	for _, f := range all {
		w, ok := want[f.Name]
		if !ok || ids[f.ID] {
			t.Fatalf("лишний формат или повтор ID: %+v", f)
		}
		ids[f.ID] = true
		pr, _ := CoverPresetFor("fsfera-thin", Settings{PageW: f.PageW, PageH: f.PageH})
		p := fsferaProject(f.PageW, f.PageH, 1, pr.Spec)
		sg, cg := p.Geo(p.Spreads[0]), p.Geo(p.Cover.Sheet)
		if got := [4]int{px(sg.W), px(sg.H), px(cg.W), px(cg.H)}; got != w {
			t.Errorf("%s: разворот и обложка %v px, ждали %v", f.Name, got, w)
		}
		safe := 10.0
		if f.Name == "25×25" || f.Name == "21×30" {
			safe = 8
		}
		if f.Safe != safe || f.Safe > min(f.PageW, f.PageH)/4 {
			t.Errorf("%s: безопасная зона %v", f.Name, f.Safe)
		}
		if got, ok := FsferaFormatOf(ClampSettings(Settings{PageW: f.PageW, PageH: f.PageH})); !ok || got != f {
			t.Errorf("%s: формат по странице не найден: %+v", f.Name, got)
		}
	}
	// Лист 031 того же размера и страница не из таблицы — не формат.
	if _, ok := FsferaFormatOf(Settings{PageW: 281, PageH: 284, Sheet: true}); ok {
		t.Error("лист 031 принят за фотокнигу")
	}
	if _, ok := FsferaFormatOf(Settings{PageW: 281.01, PageH: 284}); ok {
		t.Error("281,01 × 284 принят за 30×30")
	}
}

// Пресеты страницы (047, решение 54): общий ряд без «Фотосферы» и ряд Фотосферы из таблицы —
// 300 dpi, layflat, зона формата, подсказка с числом зоны.
func TestPagePresets(t *testing.T) {
	all, fs := PagePresets(), FsferaFormats()
	if len(all) != 6+len(fs)+8+len(FabrikaFormats()) {
		t.Fatalf("пресетов %d", len(all))
	}
	for i, f := range fs {
		pp := all[6+i]
		if pp.Label != f.Name || pp.Group != CoverGroupFsfera || pp.W != f.PageW || pp.H != f.PageH || pp.DPI != 300 || !pp.Layflat || pp.Safe != f.Safe {
			t.Errorf("пресет %d: %+v", i, pp)
		}
	}
	if h := all[6+4].Hint; h != "Фотосфера Pro: JPEG sRGB 300 dpi, 00.jpg — обложка, 01.jpg… — развороты; важное — не ближе 8 мм к краю" {
		t.Errorf("подсказка 25×25: %q", h)
	}
	for _, pp := range all[:6] {
		if pp.Group != "" || pp.DPI != 0 || pp.Layflat || pp.Safe != 0 || pp.Hint != "" {
			t.Errorf("общий пресет задаёт лишнее: %+v", pp)
		}
	}
}

// Группы пресетов (058, 081): "", «Фотосфера», «Нетпринт», «Фабрика Фотокниги» подряд в этом порядке;
// ряд Нетпринта — 8 форматов «Премиум» (058 п. 5), 300 dpi, layflat, важное 10 мм; у Фабрики 5 строк.
func TestPagePresetsGroups(t *testing.T) {
	var groups []string
	var np []string
	fabrika := 0
	for _, pp := range PagePresets() {
		if len(groups) == 0 || groups[len(groups)-1] != pp.Group {
			groups = append(groups, pp.Group)
		}
		if pp.Group == CoverGroupFabrika {
			fabrika++
		}
		if pp.Group == CoverGroupNetprint {
			if pp.DPI != 300 || !pp.Layflat || pp.Safe != 10 || pp.Hint != "Нетпринт Премиум: JPEG 300 dpi, разворотами; важное — не ближе 10 мм к краю" {
				t.Errorf("%s: %+v", pp.Label, pp)
			}
			np = append(np, fmt.Sprintf("%s %v×%v", pp.Label, pp.W, pp.H))
		}
	}
	if want := []string{"", CoverGroupFsfera, "Нетпринт", CoverGroupFabrika}; !slices.Equal(groups, want) {
		t.Fatalf("группы %q, ждали %q", groups, want)
	}
	want := []string{"15×20 153×203", "20×15 200×156", "18×13 182.5×127", "20×20 200×203", "20×28 200×270", "28×20 285×203",
		"25×25 250×254", "30×30 300×305"}
	if !slices.Equal(np, want) {
		t.Fatalf("Нетпринт %q", np)
	}
	if fabrika != 5 {
		t.Errorf("кнопок Фабрики %d, ждали 5", fabrika)
	}
	if pr, _ := CoverPresetOf("netprint-premium"); pr.Group != CoverGroupNetprint {
		t.Fatalf("обложка Нетпринта в группе %q", pr.Group)
	}
}

// CoverPresetFor (047, 070 Е): Фотосфера — сторонка и заворот шаблона формата страницы, вне
// таблицы и на листе 031 — ближайшего формата (не «из страницы»: W/H > 0 всегда); безопасные
// линии 8/12; прочие пресеты — как CoverPresetOf; неизвестный — false.
func TestCoverPresetFor(t *testing.T) {
	for _, c := range []struct {
		st         Settings
		w, h, wrap float64
	}{
		{Settings{PageW: 281, PageH: 284}, 286, 286, 20},
		{Settings{PageW: 208, PageH: 296}, 211, 296, 16},
		{Settings{PageW: 300, PageH: 300}, 286, 286, 20},
		{Settings{PageW: 281, PageH: 284, Sheet: true}, 286, 286, 20},
		{Settings{PageW: 200, PageH: 300}, 211, 296, 16}, // ближайший 21×30 (12 мм), не 20×30 (28 мм)
	} {
		for _, id := range []string{"fsfera-thin", "fsfera-card", "fsfera-double"} {
			pr, ok := CoverPresetFor(id, c.st)
			if sp := pr.Spec; !ok || sp.Preset != id || sp.W != c.w || sp.H != c.h || sp.Wrap != c.wrap || sp.Safe != 8 || sp.SpineSafe != 12 {
				t.Errorf("%s на %v×%v (лист %v): %+v", id, c.st.PageW, c.st.PageH, c.st.Sheet, sp)
			}
		}
	}
	np, _ := CoverPresetOf("netprint-premium")
	if got, ok := CoverPresetFor("netprint-premium", Settings{PageW: 281, PageH: 284}); !ok || !reflect.DeepEqual(got, np) {
		t.Errorf("Нетпринт на 30×30 изменился: %+v", got)
	}
	if _, ok := CoverPresetFor("нет такого", Settings{PageW: 281, PageH: 284}); ok {
		t.Error("неизвестный пресет найден")
	}
}

// CoverOffTemplate и CoverFixText (047, 070 Е): старая обложка (заворот 23,5, сторонка — страница)
// на 30×30 — размеры листа; вне таблицы — размеры листа ближайшего формата; обложка «из страницы»
// на нетабличной странице — тоже не шаблон; другая таблица корешка при том же листе —
// «корешок или расстав»; шаблон, Нетпринт, своя и нет обложки — "".
func TestCoverFixText(t *testing.T) {
	thin, _ := CoverPresetOf("fsfera-thin")
	old := thin.Spec
	old.Wrap, old.Safe, old.SpineSafe = 23.5, 0, 0
	tpl, _ := CoverPresetFor("fsfera-thin", Settings{PageW: 281, PageH: 284})
	steps := tpl.Spec
	steps.Steps = slices.Clone(steps.Steps)
	steps.Steps[len(steps.Steps)-1].Mm = 31
	// Заворот 23 при сторонке, уменьшенной ровно на него: лист тот же, не так — заворот.
	wrap := tpl.Spec
	wrap.Wrap, wrap.W, wrap.H = 23, wrap.W-3, wrap.H-6
	np, _ := CoverPresetOf("netprint-premium")
	cu, _ := CoverPresetOf(CoverCustom)
	for _, c := range []struct {
		w, h float64
		spec CoverSpec
		want string
	}{
		{281, 284, old, "Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326"},
		{300, 300, old, "Не по шаблону Фотосферы 30×30: 661 × 347 мм, нужно 626 × 326"},
		{300, 300, thin.Spec, "Не по шаблону Фотосферы 30×30: 654 × 340 мм, нужно 626 × 326"},
		{281, 284, wrap, "Не по шаблону Фотосферы 30×30: заворот 23 мм, нужно 20"},
		{281, 284, steps, "Не по шаблону Фотосферы 30×30: корешок или расстав не по таблице"},
		{281, 284, tpl.Spec, ""},
		{300, 300, tpl.Spec, ""},
		{281, 284, np.Spec, ""},
		{281, 284, cu.Spec, ""},
	} {
		p := fsferaProject(c.w, c.h, 20, c.spec)
		if got := CoverFixText(p); got != c.want {
			t.Errorf("%v×%v %+v: %q, ждали %q", c.w, c.h, c.spec, got, c.want)
		}
	}
	p := fsferaProject(281, 284, 20, old)
	if have, want, f, off := p.CoverOffTemplate(); !off || have != [2]float64{623, 331} || want != [2]float64{626, 326} || f.Name != "30×30" {
		t.Errorf("CoverOffTemplate: %v %v %+v %v", have, want, f, off)
	}
	p.Cover = nil
	if CoverFixText(p) != "" {
		t.Error("без обложки есть строка")
	}
}

// Узоры фона (048). Лист 031: узор страницы переходит на весь лист, левая важнее; узор листа
// остаётся, страницы снимаются. Не лист — узоры страниц как были.
func TestFoldPagesPattern(t *testing.T) {
	pat := func(k string) *Pattern { return &Pattern{Kind: k} }
	kind := func(pt *Pattern) string {
		if pt == nil {
			return ""
		}
		return pt.Kind
	}
	p := NewSheet(200, 300)
	p.Spreads = []*Spread{
		{ID: "l", PatL: pat("left")},
		{ID: "r", PatR: pat("right")},
		{ID: "lr", PatL: pat("left"), PatR: pat("right")},
		{ID: "all", Pat: pat("sheet"), PatL: pat("left"), PatR: pat("right")},
		{ID: "none"},
	}
	p.FoldPages()
	want := []string{"left", "right", "left", "sheet", ""}
	for i, s := range p.Spreads {
		if kind(s.Pat) != want[i] || s.PatL != nil || s.PatR != nil {
			t.Errorf("%s: pat %q, patL %v, patR %v; ждали %q и пустые страницы", s.ID, kind(s.Pat), s.PatL, s.PatR, want[i])
		}
	}
	a := New()
	a.Spreads = []*Spread{{ID: "a", PatL: pat("left"), PatR: pat("right")}}
	a.FoldPages()
	if s := a.Spreads[0]; s.Pat != nil || kind(s.PatL) != "left" || kind(s.PatR) != "right" {
		t.Fatalf("альбом: узоры страниц сведены: %+v", s)
	}
}

// Слоты узора: области spread/left/right, чужая — nil; Patterns — по порядку, без пустых.
func TestPatSlot(t *testing.T) {
	s := &Spread{}
	for _, scope := range PatScopes() {
		*s.PatSlot(scope) = &Pattern{Kind: scope}
	}
	if s.Pat.Kind != ScopeSpread || s.PatL.Kind != ScopeLeft || s.PatR.Kind != ScopeRight || s.PatSlot("cover") != nil {
		t.Fatalf("слоты: %+v %+v %+v", s.Pat, s.PatL, s.PatR)
	}
	s.PatL = nil
	if got := s.Patterns(); len(got) != 2 || got[0] != s.Pat || got[1] != s.PatR {
		t.Fatalf("Patterns: %v", got)
	}
	if (&Spread{}).Patterns() != nil {
		t.Fatal("Patterns пустого листа не nil")
	}
}

// Ключ своего: грамматика my:<16 hex> нижним регистром; ID записи — 16 hex от ключа, разный
// у разных ключей и не равный самому hex ключа.
func TestLibPhotoID(t *testing.T) {
	good := "my:0123456789abcdef"
	for key, want := range map[string]bool{
		good: true, "my:ffffffffffffffff": true,
		"": false, "my:": false, "my:0123456789ABCDEF": false, "my:0123456789abcde": false,
		"my:0123456789abcdef0": false, "sa:0123456789abcdef": false, "my:0123456789abcdeg": false,
		"My:0123456789abcdef": false, "my:0123456789abc/ef": false,
	} {
		if ValidLibKey(key) != want {
			t.Errorf("ValidLibKey(%q) = %v", key, !want)
		}
	}
	id := LibPhotoID(good)
	if len(id) != 16 || strings.Trim(id, "0123456789abcdef") != "" || id == good[3:] {
		t.Fatalf("ID %q", id)
	}
	if LibPhotoID(good) != id || LibPhotoID("my:ffffffffffffffff") == id {
		t.Fatal("ID не от ключа")
	}
}

// TeacherSeat — только место учителя виньетки; ученик, групповое, обычный лист и пустой ID — нет.
func TestTeacherSeat(t *testing.T) {
	p := New()
	vig := &Spread{ID: "v", Kind: KindVignette, Vig: &Vignette{Teachers: 2, Pupils: 1}, Grid: []*Place{
		{ID: "1", Role: RoleTeacher, Photo: "t1"}, {ID: "2", Role: RoleTeacher, Photo: "t2"},
		{ID: "3", Role: RolePupil, Photo: "u"}, {ID: "4", Role: RoleGroup, Photo: "g"}, {ID: "5", Role: RoleTeacher}}}
	plain := &Spread{ID: "s", Grid: []*Place{{ID: "6", Role: RoleTeacher, Photo: "x"}}}
	p.Spreads = []*Spread{plain, vig}
	for id, want := range map[string]bool{"t1": true, "t2": true, "u": false, "g": false, "x": false, "": false, "нет": false} {
		if p.TeacherSeat(id) != want {
			t.Errorf("%q: %v, ждали %v", id, !want, want)
		}
	}
}

// Индексы полей: каждое имя — свой индекс, прежние имена шапки (030) — синонимы полей шапки,
// а не имени; «Предмет» — поле имени, а не шапки; ключи UI — по полям имени.
func TestFieldIndexSynonyms(t *testing.T) {
	for i, n := range fieldNames {
		if fieldIndex(n) != i {
			t.Errorf("%s: индекс %d, ждали %d", n, fieldIndex(n), i)
		}
	}
	for syn, want := range map[string]string{"Школа": "Заголовок", "класс": "Подзаголовок", "УЧИТЕЛЬ": "Руководитель"} {
		if i := fieldIndex(syn); i < 0 || fieldNames[i] != want || IsNameField(syn) {
			t.Errorf("%s → %d, ждали %s", syn, i, want)
		}
	}
	if fieldNames[fSubject] != "Предмет" || fieldNames[fTitle] != "Заголовок" || fieldNames[fSubtitle] != "Подзаголовок" || fieldNames[fLead] != "Руководитель" {
		t.Fatal("именованные индексы разошлись с fieldNames")
	}
	if !IsNameField("предмет") || IsNameField("Год") || HasHeadFields("{Предмет}") || !HasHeadFields("{Предмет}{Год}") {
		t.Fatal("Предмет — поле имени, Год — шапки")
	}
	if len(nameKeys) != nameFields || nameKeys[fSubject] != "subject" {
		t.Fatalf("ключи UI: %v", nameKeys)
	}
}

// Поля имени шаблона с предметом: ключ subject по порядку появления, без повторов.
func TestNameFieldsOfSubject(t *testing.T) {
	for in, want := range map[string][]string{
		StaffCaption:               {"last", "first", "middle", "subject"},
		"{Предмет} {Фамилия}":      {"subject", "last"},
		"{предмет}{ПРЕДМЕТ} {Год}": {"subject"},
		TeacherCaption:             {"last", "first", "middle"},
	} {
		if got := NameFieldsOf(in); !slices.Equal(got, want) {
			t.Errorf("%q: %v, ждали %v", in, got, want)
		}
	}
	// Меню «Вставить поле»: предмет — у виньетки и листа с полями, свадебному листу — нет.
	vig := &Spread{Kind: KindVignette, Vig: &Vignette{}}
	if got := FieldsOf(vig, nil); len(got) != len(fieldNames) || got[3] != "Предмет" {
		t.Fatalf("виньетка: %v", got)
	}
	if got := FieldsOf(&Spread{}, nil); slices.Contains(got, "Предмет") {
		t.Fatalf("свадебный лист: %v", got)
	}
}

// 114: ShiftGridPhoto — фото встаёт в щель порядка GridPhotos, места стоят, фото идут по
// непустым местам; пустые места остаются пустыми.
func TestShiftGridPhoto(t *testing.T) {
	mk := func(ids ...string) *Spread {
		s := &Spread{ID: "s"}
		for i, id := range ids {
			s.Grid = append(s.Grid, &Place{ID: fmt.Sprintf("p%d", i), X: float64(i), W: 1, H: 1, Photo: id})
		}
		s.Photos = GridPhotos(s)
		return s
	}
	place := func(s *Spread) []string {
		var out []string
		for _, pl := range s.Grid {
			out = append(out, pl.Photo)
		}
		return out
	}
	cases := []struct {
		name  string
		from  []string
		id    string
		index int
		want  []string // nil — холостая
	}{
		{"a в щель перед d", []string{"a", "b", "", "c", "d"}, "a", 3, []string{"b", "c", "", "a", "d"}},
		{"d в начало", []string{"a", "b", "", "c", "d"}, "d", 0, []string{"d", "a", "", "b", "c"}},
		{"-1 — в конец", []string{"a", "b", "", "c", "d"}, "a", -1, []string{"b", "c", "", "d", "a"}},
		{"99 — в конец", []string{"a", "b", "", "c", "d"}, "b", 99, []string{"a", "c", "", "d", "b"}},
		{"без пустых", []string{"a", "b", "c"}, "c", 1, []string{"a", "c", "b"}},
		{"соседние — обмен", []string{"a", "b", "c"}, "a", 2, []string{"b", "a", "c"}},
		{"щель до своего", []string{"a", "b", "c"}, "b", 1, nil},
		{"щель после своего", []string{"a", "b", "c"}, "b", 2, nil},
		{"последнее в конец", []string{"a", "b"}, "b", -1, nil},
		{"единственное", []string{"a"}, "a", 0, nil},
		{"чужое фото", []string{"a", "b"}, "x", 0, nil},
		{"пустой id", []string{"a", "", "b"}, "", 0, nil},
		{"нет фото", []string{"", ""}, "a", 0, nil},
	}
	for _, c := range cases {
		s := mk(c.from...)
		geo := make([]Place, len(s.Grid))
		for i, pl := range s.Grid {
			geo[i] = *pl
			geo[i].Photo = ""
		}
		got := s.ShiftGridPhoto(c.id, c.index)
		want := c.want
		if want == nil {
			want = c.from
		}
		if got != (c.want != nil) || !slices.Equal(place(s), want) {
			t.Errorf("%s: изменено %v, места %v, ждали %v", c.name, got, place(s), want)
		}
		if !slices.Equal(s.Photos, GridPhotos(s)) {
			t.Errorf("%s: Photos %v ≠ GridPhotos %v", c.name, s.Photos, GridPhotos(s))
		}
		for i, pl := range s.Grid {
			g := *pl
			g.Photo = ""
			if g != geo[i] {
				t.Errorf("%s: место %d изменилось: %+v", c.name, i, g)
			}
		}
	}
	// Лист без мест («Авто») — ничего.
	if (&Spread{Photos: []string{"a", "b"}}).ShiftGridPhoto("a", -1) {
		t.Error("«Авто» не сетка: сдвиг невозможен")
	}
}
