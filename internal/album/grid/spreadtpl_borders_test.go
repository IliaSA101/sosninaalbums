package grid_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочки мест и заготовки в шаблоне разворота (131 G1, design §1): постановка, повтор, снятие.

const kFr = "my:00000000000000d1" // рамочка-картинка из набора

// frameLibs — файлы «этого ПК»: фоны и стикеры tplLibs плюс рамочка kFr.
func frameLibs() map[string]model.PhotoInfo {
	libs := tplLibs()
	libs[kFr] = model.PhotoInfo{Path: edtest.Abs("lib-frame.png"), W: 400, H: 300}
	return libs
}

// borderTpl — разворот из трёх мест: встроенная рамочка на первом, картинка на третьем,
// второе без рамочки; надпись и две заготовки.
func borderTpl() model.SpreadTpl {
	return model.SpreadTpl{V: model.TplV, Pages: 2, Page: []model.TplBg{{Color: "#101010"}, {Color: "#202020"}},
		Places: []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.2, H: 0.3}, {X: 0.55, Y: 0.5, W: 0.2, H: 0.3}},
		Borders: []model.TplBorder{
			{I: 0, Kind: "line", Inks: []string{"#112233"}, Width: 2.5},
			{I: 2, Lib: kFr, Scale: 1.5, Cut: [4]float64{0.1, 0.2, 0.1, 0.2}, Open: true},
		},
		Texts:  []model.TplText{{Anchor: model.Anchor{AY: -1, DY: 0.05}, W: 0.5, Text: "Наша свадьба", Style: model.PlainStyle()}},
		Drafts: []model.TplText{{Anchor: model.Anchor{AY: 1, DY: -0.05}, W: 0.4, Text: "[Имена молодожёнов]", Style: model.PlainStyle()}, {Anchor: model.Anchor{AX: 1, AY: 1, DX: -0.05, DY: -0.05}, W: 0.2, Text: "[Дата]", Style: model.PlainStyle()}},
	}
}

func gridIDs(s *model.Spread) []string {
	var out []string
	for _, pl := range s.Grid {
		out = append(out, pl.ID)
	}
	return out
}

// TestApplySpreadTplBordersNoop — рамочки встают на места своего индекса (встроенная и картинка
// с записью в Photos), без файла картинки место остаётся без рамочки (missing), повтор — холостой:
// ни шага, ни rev, ID мест прежние; изменённый шаблон — один шаг, отмена его возвращает.
func TestApplySpreadTplBordersNoop(t *testing.T) {
	e := tplEd(t)
	tp := borderTpl()
	_, plan, missing, err := e.ApplySpreadTpl("s3", model.ScopeSpread, tp, frameLibs())
	if err != nil || missing != 0 || plan.Left != 0 {
		t.Fatalf("постановка: %+v, %d, %v", plan, missing, err)
	}
	edtest.MustGrid(t, e)
	s := sheetOf(e, "s3")
	if len(s.Grid) != 3 {
		t.Fatalf("мест %d", len(s.Grid))
	}
	b0, b1, b2 := s.Grid[0].Border, s.Grid[1].Border, s.Grid[2].Border
	if b0 == nil || b0.Kind != "line" || b0.Width != 2.5 || !slices.Equal(b0.Inks, []string{"#112233"}) || b1 != nil {
		t.Fatalf("рамочки 0 и 1: %+v, %+v", b0, b1)
	}
	if b2 == nil || b2.Photo != model.LibPhotoID(kFr) || b2.Scale != 1.5 || b2.Cut != [4]float64{0.1, 0.2, 0.1, 0.2} || !b2.Open {
		t.Fatalf("рамочка-картинка: %+v", b2)
	}
	if r, ok := edtest.P(e).Photos[b2.Photo]; !ok || r.Lib != kFr || r.W != 400 {
		t.Fatalf("запись картинки рамочки: %+v", r)
	}

	// Повтор: холостая правка.
	ids, rev, undo := gridIDs(s), edtest.Rev(e), edtest.UndoLen(e)
	if _, _, missing, err = e.ApplySpreadTpl("s3", model.ScopeSpread, tp, frameLibs()); err != nil || missing != 0 {
		t.Fatalf("повтор: %d, %v", missing, err)
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo || !slices.Equal(gridIDs(sheetOf(e, "s3")), ids) {
		t.Fatalf("повтор не холостой: rev %d→%d, шагов %d→%d, ID %v→%v", rev, edtest.Rev(e), undo, edtest.UndoLen(e), ids, gridIDs(sheetOf(e, "s3")))
	}

	// Другая рамочка — один шаг; отмена возвращает прежние рамочки.
	tp2 := borderTpl()
	tp2.Borders[0].Kind = "gold"
	tp2.Borders[0].Inks = nil
	if _, _, _, err = e.ApplySpreadTpl("s3", model.ScopeSpread, tp2, frameLibs()); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 || sheetOf(e, "s3").Grid[0].Border.Kind != "gold" {
		t.Fatalf("шагов %d (было %d), рамочка %+v", edtest.UndoLen(e), undo, sheetOf(e, "s3").Grid[0].Border)
	}
	e.Undo()
	if got := sheetOf(e, "s3").Grid[0].Border; got == nil || got.Kind != "line" {
		t.Fatalf("после отмены: %+v", got)
	}

	// Круг Save → Load: рамочки на месте, картинка находится по ключу.
	edtest.LibFinder(t, map[string]string{kBg: edtest.Abs("lib-bg.jpg"), kSt: edtest.Abs("lib-st.png"), kFr: edtest.Abs("lib-frame.png")})
	q := edtest.SaveLoad(t, e)
	g := q.Spreads[2].Grid
	if len(g) != 3 || g[0].Border == nil || g[0].Border.Kind != "line" || g[1].Border != nil || g[2].Border == nil || g[2].Border.Photo != model.LibPhotoID(kFr) || g[2].Border.Scale != 1.5 {
		t.Fatalf("после записи и чтения: %s", jsonOf(t, g))
	}
}

// TestApplySpreadTplBordersMissing — картинки рамочки нет на этом ПК: место без рамочки, в
// missing один ключ (на два места с одной картинкой — один), остальное ставится; повтор холостой.
func TestApplySpreadTplBordersMissing(t *testing.T) {
	e := tplEd(t)
	tp := borderTpl()
	tp.Borders = append(tp.Borders, model.TplBorder{I: 1, Lib: kFr})
	_, _, missing, err := e.ApplySpreadTpl("s3", model.ScopeSpread, tp, tplLibs())
	if err != nil || missing != 1 {
		t.Fatalf("missing %d, %v", missing, err)
	}
	s := sheetOf(e, "s3")
	if s.Grid[0].Border == nil || s.Grid[1].Border != nil || s.Grid[2].Border != nil {
		t.Fatalf("рамочки: %s", jsonOf(t, s.Grid))
	}
	if _, ok := edtest.P(e).Photos[model.LibPhotoID(kFr)]; ok {
		t.Error("запись пропавшей картинки попала в альбом")
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, _, err = e.ApplySpreadTpl("s3", model.ScopeSpread, tp, tplLibs()); err != nil || edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("повтор не холостой: %v", err)
	}
}

// TestApplySpreadTplBordersPage — шаблон страницы на весь разворот: рамочка едет на оба места,
// у каждого места свой указатель (079-23); на «Авто» места закрепляются, рамочки встают.
func TestApplySpreadTplBordersPage(t *testing.T) {
	e := tplEd(t)
	page := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{}},
		Places:  []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}},
		Borders: []model.TplBorder{{I: 0, Kind: "line"}}}
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, page, nil); err != nil {
		t.Fatal(err)
	}
	g := sheetOf(e, "s3").Grid
	if len(g) != 2 || g[0].Border == nil || g[1].Border == nil || g[0].Border == g[1].Border {
		t.Fatalf("рамочки: %s", jsonOf(t, g))
	}
	if _, plan, _, err := e.ApplySpreadTpl("s2", model.ScopeLeft, page, nil); err != nil || plan.Frozen != 1 {
		t.Fatalf("«Авто»: %+v, %v", plan, err)
	}
	if g := sheetOf(e, "s2").Grid; len(g) == 0 || g[0].Border == nil || g[0].Border.Kind != "line" {
		t.Fatalf("s2: %s", jsonOf(t, g))
	}
	// Шаблон без рамочек снимает прежние: места «те же», но рамочка другая — не холостая правка.
	page.Borders = nil
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, page, nil); err != nil {
		t.Fatal(err)
	}
	if g := sheetOf(e, "s3").Grid; g[0].Border != nil || g[1].Border != nil {
		t.Fatalf("рамочки не сняты: %s", jsonOf(t, g))
	}
}

// TestApplySpreadTplDrafts — заготовки встают надписями с Draft, обычная надпись — без; шаблон
// из одних заготовок заменяет надписи цели; повтор холостой; круг Save → Load сохраняет признак.
func TestApplySpreadTplDrafts(t *testing.T) {
	e := tplEd(t)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil {
		t.Fatal(err)
	}
	s := sheetOf(e, "s3")
	var plain, drafts []string
	for _, tx := range s.Texts {
		if tx.Draft {
			drafts = append(drafts, tx.Text)
		} else {
			plain = append(plain, tx.Text)
		}
	}
	if !slices.Equal(plain, []string{"Наша свадьба"}) || !slices.Equal(drafts, []string{"[Имена молодожёнов]", "[Дата]"}) {
		t.Fatalf("надписи %v, заготовки %v", plain, drafts)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil || edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("повтор не холостой: %v", err)
	}

	// Правка заготовки (смена строки без снятия признака на уровне модели G2) делает повтор не холостым.
	edtest.Change(e, func(p *model.Project) { p.Spreads[2].Texts[1].Text = "Анна и Михаил" })
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil || edtest.Rev(e) == rev {
		t.Fatalf("правленная заготовка не заменена: %v", err)
	}

	// Только заготовки: свои надписи цели заменяются (s1 держит «Наша свадьба»).
	only := model.SpreadTpl{V: model.TplV, Pages: 2, Page: []model.TplBg{{}, {}},
		Drafts: []model.TplText{{W: 0.4, Text: "[Имена]", Style: model.PlainStyle()}}}
	_, plan, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, only, nil)
	if err != nil || plan.Texts != 1 {
		t.Fatalf("план %+v, %v", plan, err)
	}
	if got := sheetOf(e, "s1").Texts; len(got) != 1 || !got[0].Draft || got[0].Text != "[Имена]" {
		t.Fatalf("s1: %s", jsonOf(t, got))
	}
	// Шаблон без надписей и заготовок надписей цели не трогает.
	pageOnly := model.SpreadTpl{V: model.TplV, Pages: 2, Page: []model.TplBg{{Color: "#abcdef"}, {}}}
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, pageOnly, nil); err != nil || len(sheetOf(e, "s1").Texts) != 1 {
		t.Fatalf("надписи тронуты: %v", err)
	}

	edtest.LibFinder(t, map[string]string{kBg: edtest.Abs("lib-bg.jpg"), kSt: edtest.Abs("lib-st.png"), kFr: edtest.Abs("lib-frame.png")})
	q := edtest.SaveLoad(t, e)
	n := 0
	for _, tx := range q.Spreads[0].Texts {
		if tx.Draft && tx.Text == "[Имена]" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("после записи и чтения: %s", jsonOf(t, q.Spreads[0].Texts))
	}
}

// TestSpreadTplOfDrafts — снятие: рамочки мест уходят в borders (встроенная как есть, картинка
// по ключу библиотеки), надписи с Draft — в drafts, «все заготовками» — все; без мест рамочек
// нет; снимок ставится снова теми же рамочками и заготовками (круг шаблон → лист → шаблон).
func TestSpreadTplOfDrafts(t *testing.T) {
	e := tplEd(t)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil {
		t.Fatal(err)
	}
	tp, err := e.SpreadTplOf("s3", model.ScopeSpread, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tp.Places) != 3 || len(tp.Borders) != 2 || len(tp.Texts) != 1 || len(tp.Drafts) != 2 {
		t.Fatalf("снимок: %s", jsonOf(t, tp))
	}
	for i, pl := range tp.Places {
		if pl.Border != nil {
			t.Errorf("место %d: рамочка осталась в коробке", i)
		}
	}
	if b := tp.Borders[0]; b.I != 0 || b.Kind != "line" || b.Width != 2.5 || b.Lib != "" {
		t.Errorf("borders[0] %+v", b)
	}
	if b := tp.Borders[1]; b.I != 2 || b.Lib != kFr || b.Kind != "" || b.Scale != 1.5 || !b.Open {
		t.Errorf("borders[1] %+v", b)
	}
	if tp.Drafts[0].Text != "[Имена молодожёнов]" || tp.Texts[0].Text != "Наша свадьба" {
		t.Errorf("надписи %+v, заготовки %+v", tp.Texts, tp.Drafts)
	}

	// Круг: ставим снимок на пустой лист, снимаем ещё раз — тот же шаблон.
	if _, _, _, err := e.ApplySpreadTpl("s4", model.ScopeSpread, tp, frameLibs()); err != nil {
		t.Fatal(err)
	}
	again, err := e.SpreadTplOf("s4", model.ScopeSpread, true, true)
	if err != nil || model.TplKey(again) != model.TplKey(tp) {
		t.Fatalf("круг изменил шаблон: %v\n%s\n%s", err, jsonOf(t, tp), jsonOf(t, again))
	}

	// «Надписи — заготовками»: обычная надпись тоже уходит в drafts.
	all, err := grid.SpreadTplOfAs(edtest.P(e), "s3", model.ScopeSpread, true, true, true)
	if err != nil || len(all.Texts) != 0 || len(all.Drafts) != 3 {
		t.Fatalf("все заготовками: %v\n%s", err, jsonOf(t, all))
	}
	// Без мест — без рамочек; без надписей — без заготовок.
	np, err := e.SpreadTplOf("s3", model.ScopeSpread, false, false)
	if err != nil || len(np.Places) != 0 || len(np.Borders) != 0 || len(np.Texts)+len(np.Drafts) != 0 {
		t.Fatalf("без мест и надписей: %v\n%s", err, jsonOf(t, np))
	}
}

// TestSpreadTplOfBorderNotFromLib — рамочка-картинка не из библиотеки (запись Photos без Lib)
// в шаблон не идёт, соседняя встроенная остаётся с тем же индексом.
func TestSpreadTplOfBorderNotFromLib(t *testing.T) {
	e := tplEd(t)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) {
		id := model.LibPhotoID(kFr)
		r := p.Photos[id]
		r.Lib = ""
		p.Photos[id] = r
	})
	tp, err := e.SpreadTplOf("s3", model.ScopeSpread, true, false)
	if err != nil || len(tp.Borders) != 1 || tp.Borders[0].I != 0 {
		t.Fatalf("%v\n%s", err, jsonOf(t, tp))
	}
}

// TestSpreadTplOfOwnPresetNoop — рамочка, поставленная щелчком по плитке (Preset), входит в
// снимок; свой снимок на тот же разворот — холостая правка: rev, ID мест, шаги и Preset прежние;
// на другой лист отметка едет, круг Save → Load её хранит; один шаг и отмена на другой рамочке.
func TestSpreadTplOfOwnPresetNoop(t *testing.T) {
	e := tplEd(t)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, borderTpl(), frameLibs()); err != nil {
		t.Fatal(err)
	}
	s := sheetOf(e, "s3")
	// Несколько мест сразу: плитка на втором (без рамочки) и на первом (встроенная).
	src := model.BorderSrc{Kind: "line", Inks: []string{"#FFFFFF"}, Width: 2, Preset: "sa:line"}
	if _, err := e.ApplyBorder("s3", []string{s.Grid[0].ID, s.Grid[1].ID}, src); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if b := sheetOf(e, "s3").Grid[i].Border; b == nil || b.Preset != "sa:line" {
			t.Fatalf("место %d: рамочка %+v", i, b)
		}
	}
	tp, err := e.SpreadTplOf("s3", model.ScopeSpread, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tp.Borders) != 3 || tp.Borders[0].Preset != "sa:line" || tp.Borders[1].Preset != "sa:line" || tp.Borders[2].Preset != "" {
		t.Fatalf("снимок: %s", jsonOf(t, tp.Borders))
	}

	ids, rev, undo, before := gridIDs(sheetOf(e, "s3")), edtest.Rev(e), edtest.UndoLen(e), model.Marshal(edtest.P(e))
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, tp, frameLibs()); err != nil {
		t.Fatal(err)
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo || !slices.Equal(gridIDs(sheetOf(e, "s3")), ids) || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("свой снимок не холостой: rev %d→%d, шагов %d→%d, ID %v→%v", rev, edtest.Rev(e), undo, edtest.UndoLen(e), ids, gridIDs(sheetOf(e, "s3")))
	}
	if b := sheetOf(e, "s3").Grid[1].Border; b == nil || b.Preset != "sa:line" {
		t.Fatalf("отметка плитки пропала: %+v", b)
	}

	// Другой лист: отметка едет, ключ шаблона учитывает её.
	if _, _, _, err := e.ApplySpreadTpl("s4", model.ScopeSpread, tp, frameLibs()); err != nil {
		t.Fatal(err)
	}
	if b := sheetOf(e, "s4").Grid[0].Border; b == nil || b.Preset != "sa:line" {
		t.Fatalf("s4: %+v", b)
	}
	// Снимок без отметки (рамочка сменилась правкой) — другой шаблон: один шаг, отмена возвращает.
	e.SetBorder("s3", []string{sheetOf(e, "s3").Grid[0].ID}, model.BorderPatch{Width: 5}, "")
	plain, err := e.SpreadTplOf("s3", model.ScopeSpread, true, true)
	if err != nil || plain.Borders[0].Preset != "" || model.TplKey(plain) == model.TplKey(tp) {
		t.Fatalf("правка толщины должна снять отметку: %v\n%s", err, jsonOf(t, plain.Borders))
	}
	// Круг Save → Load.
	edtest.LibFinder(t, map[string]string{kBg: edtest.Abs("lib-bg.jpg"), kSt: edtest.Abs("lib-st.png"), kFr: edtest.Abs("lib-frame.png")})
	q := edtest.SaveLoad(t, e)
	if b := q.Spreads[3].Grid[1].Border; b == nil || b.Preset != "sa:line" {
		t.Fatalf("после записи и чтения: %+v", b)
	}
}
