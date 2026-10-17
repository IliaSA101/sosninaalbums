package vigsheet_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Тела правок 049 G3: подпись предметника по шаблону, кегль шапки при смене шаблона, подсказки
// превью (вид, круг, шапка) и пересборка новых шаблонов.

// setTpl — «Применить» с шаблоном tpl и подписью учителя ttext (как шлёт окно: подпись листа).
func setTpl(t *testing.T, e *album.Editor, sid string, i int, tpl, ttext string) {
	t.Helper()
	v := vigOf(e, i)
	v.Tpl, v.TText = tpl, ttext
	if _, _, err := e.SetVignette(sid, v, nil); err != nil {
		t.Fatalf("%s: %v", tpl, err)
	}
}

// vigHead — шапка листа i.
func vigHead(t *testing.T, e *album.Editor, i int) model.TextBlock {
	t.Helper()
	s := edtest.P(e).Spreads[i]
	if h := s.VigHead(s.Vig.Head); h != nil {
		return *h
	}
	t.Fatalf("у листа %d нет шапки", i)
	return model.TextBlock{}
}

// choiceOf — превью шаблона id.
func choiceOf(t *testing.T, cs []model.VigChoice, id string) model.VigChoice {
	t.Helper()
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("нет превью %s", id)
	return model.VigChoice{}
}

// Сц. 6: «Руководитель…» ставит подпись предметника вместо умолчания, уход — прежнюю, своя не
// трогается; каждая смена — один шаг, повтор — холостой; переживает Save→Load.
func TestVignetteStaffCaption(t *testing.T) {
	e, sid := vigSheet(t, 25)
	tt := func() string { return vigOf(e, 0).TText }
	undo := edtest.UndoLen(e)
	setTpl(t, e, sid, 0, "head-lead-left", tt())
	if tt() != model.StaffCaption || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Руководитель»: подпись %q, шагов %d", tt(), edtest.UndoLen(e)-undo)
	}
	rev := edtest.Rev(e)
	setTpl(t, e, sid, 0, "head-lead-left", tt())
	edtest.Same(t, e, rev, undo+1, "повтор «Руководитель»")
	if q := saveLoad(t, e); q.Spreads[0].Vig.TText != model.StaffCaption {
		t.Fatalf("после Save→Load подпись %q", q.Spreads[0].Vig.TText)
	}
	setTpl(t, e, sid, 0, "lead-left", tt()) // между «Руководитель…» подпись та же
	if tt() != model.StaffCaption {
		t.Fatalf("lead → lead: %q", tt())
	}
	setTpl(t, e, sid, 0, "t-left", tt())
	if tt() != model.TeacherCaption || edtest.UndoLen(e) != undo+3 {
		t.Fatalf("уход с «Руководитель»: подпись %q, шагов %d", tt(), edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if tt() != model.StaffCaption {
		t.Fatalf("отмена ухода: %q", tt())
	}
	e.Redo()
	// Своя подпись: ни приход, ни уход её не меняют.
	const own = "{Фамилия} {Имя}"
	setTpl(t, e, sid, 0, "t-left", own)
	for _, tpl := range []string{"lead-left", "t-center", "split-lead"} {
		setTpl(t, e, sid, 0, tpl, tt())
		if tt() != own {
			t.Fatalf("%s: своя подпись стала %q", tpl, tt())
		}
	}
}

// Окно и «Сделать виньеткой»: подмена — у SetupVignette и PreviewVignette так же; новая
// виньетка из обычного разворота с пустой подписью получает подпись предметника; соседние
// листы не трогаются.
func TestVignetteStaffCaptionWindow(t *testing.T) {
	e := vigAlbum(t)
	p := edtest.P(e)
	other := *p.Spreads[3].Vig
	setup := model.VigSetup{Vig: model.Vignette{Tpl: "split-lead", Teachers: 5, Pupils: 0, TText: p.Spreads[1].Vig.TText}}
	pr, err := e.PreviewVignette(p.Spreads[1].ID, setup, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Spread.Vig == nil || pr.Spread.Vig.TText != model.StaffCaption {
		t.Fatalf("предпросмотр: %+v", pr.Spread.Vig)
	}
	if _, _, err := e.SetupVignette(p.Spreads[1].ID, setup, nil); err != nil {
		t.Fatal(err)
	}
	p = edtest.P(e)
	if p.Spreads[1].Vig.TText != model.StaffCaption {
		t.Fatalf("«Применить» окна: %q", p.Spreads[1].Vig.TText)
	}
	if !p.Spreads[3].Vig.Same(other) {
		t.Fatal("соседняя виньетка изменилась")
	}
	// Обычный разворот с фото → «Руководитель»: подписи не было — сразу предметника.
	sid := p.Spreads[0].ID
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "lead-left", Teachers: 2, Pupils: 3}, nil); err != nil {
		t.Fatal(err)
	}
	if got := edtest.P(e).Spreads[0].Vig.TText; got != model.StaffCaption {
		t.Fatalf("новая виньетка «Руководитель»: %q", got)
	}
}

// Превью «Руководитель…» считает полосу подписи с той же подменой: плитка равна листу после
// «Применить» (иначе полоса двух строк, а встанет трёх).
func TestVigChoicesStaffMatchesApply(t *testing.T) {
	for _, id := range []string{"lead-left", "head-lead-left"} {
		e, sid := vigSheet(t, 0)
		c := choiceOf(t, e.VignetteChoices(sid, 3, 20, "3:4", nil), id)
		setTpl(t, e, sid, 0, id, vigOf(e, 0).TText)
		v := vigOf(e, 0)
		v.Teachers, v.Pupils = 3, 20
		if _, _, err := e.SetVignette(sid, v, nil); err != nil {
			t.Fatal(err)
		}
		g := edtest.P(e).Geo(edtest.P(e).Spreads[0])
		grid := edtest.P(e).Spreads[0].Grid
		if len(grid) != len(c.Boxes) {
			t.Fatalf("%s: мест %d, превью %d", id, len(grid), len(c.Boxes))
		}
		for k, pl := range grid {
			b := c.Boxes[k]
			if math.Abs(b.X*g.W-pl.X) > 0.05 || math.Abs(b.Y*g.H-pl.Y) > 0.05 || math.Abs(b.W*g.W-pl.W) > 0.05 || math.Abs(b.H*g.H-pl.H) > 0.05 {
				t.Fatalf("%s: место %d %+v, превью %+v", id, k, *pl, b)
			}
		}
	}
}

// Сц. 9: смена шаблона подстраивает кегль шапки под новую полосу (гарнитура, цвет, текст —
// прежние), одним шагом с местами; повтор — холостой; свой кегль только уменьшается.
func TestVignetteHeadRefit(t *testing.T) {
	e, sid := vigSheet(t, 25)
	h0 := vigHead(t, e, 0)
	auto := h0.Style.Size
	st := h0.Style
	st.Font, st.Color = "ptserif", "#112233"
	e.SetTextStyle(h0.ID, st)
	e.SetText(h0.ID, "Школа № 1", "")
	// Полоса сверху → полоса сверху: кегль прежний.
	setTpl(t, e, sid, 0, "head-t-right", vigOf(e, 0).TText)
	if h := vigHead(t, e, 0); h.Style.Size != auto {
		t.Fatalf("полоса → полоса: кегль %v, был %v", h.Style.Size, auto)
	}
	// В ряд (класс плотный — ячейки узкие): кегль по ширине ячеек, рамка — по ним.
	v := vigOf(e, 0)
	v.Pupils = 40
	if _, _, err := e.SetVignette(sid, v, nil); err != nil {
		t.Fatal(err)
	}
	if h := vigHead(t, e, 0); h.Style.Size != auto {
		t.Fatalf("тот же шаблон, другие числа: кегль %v, был %v", h.Style.Size, auto)
	}
	c := choiceOf(t, e.VignetteChoices(sid, v.Teachers, v.Pupils, v.Ratio, nil), "t-left-cell")
	if c.Head == nil {
		t.Fatal("у превью «Учителя слева, шапка в ряду» нет шапки")
	}
	g := edtest.P(e).Geo(edtest.P(e).Spreads[0])
	hw, hh := c.Head.W*g.W, c.Head.H*g.H
	undo := edtest.UndoLen(e)
	setTpl(t, e, sid, 0, "t-left-cell", vigOf(e, 0).TText)
	h := vigHead(t, e, 0)
	// Кегль по замеру гарнитурой шапки (ptserif): шаблонный текст не ниже полосы, кегль не больше
	// полосы на две строки (наибольший и целые слова — TestHeadFit ядра).
	want, top := h.Style.Size, math.Round(min(max(hh/2.6/0.3528, 12), 48))
	if want > top || headH(st, want, hw) > hh {
		t.Fatalf("в ряд: кегль %v (предел %v) — текст %.1f мм ниже полосы %.1f×%.1f", want, top, headH(st, want, hw), hw, hh)
	}
	// Наибольший: следующий кегль в полосу целыми словами уже не влезает — иначе прошёл бы и
	// пересчёт «всегда 12».
	if want < top && headFits(st, want+1, hw, hh) {
		t.Fatalf("в ряд: кегль %v не наибольший — %v тоже влезает в полосу %.1f×%.1f", want, want+1, hw, hh)
	}
	if !(want < auto) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("в ряд: кегль %v, ждали %v (был %v), шагов %d", h.Style.Size, want, auto, edtest.UndoLen(e)-undo)
	}
	if h.Style.Font != "ptserif" || h.Style.Color != "#112233" || h.Text != "Школа № 1" || math.Abs(h.W-hw) > 0.02 {
		t.Fatalf("в ряд: шапка %+v, ширина ждали %v", h, hw)
	}
	rev := edtest.Rev(e)
	setTpl(t, e, sid, 0, "t-left-cell", vigOf(e, 0).TText)
	edtest.Same(t, e, rev, undo+1, "повтор «шапка в ряду»")
	if q := saveLoad(t, e); q.Spreads[0].VigHead(q.Spreads[0].Vig.Head).Style.Size != want {
		t.Fatal("кегль шапки не пережил Save→Load")
	}
	// Обратно на полосу: кегль «по ячейкам» снова по полосе.
	setTpl(t, e, sid, 0, "head-t-left", vigOf(e, 0).TText)
	if h := vigHead(t, e, 0); h.Style.Size != auto {
		t.Fatalf("обратно на полосу: кегль %v, ждали %v", h.Style.Size, auto)
	}
	e.Undo()
	if h := vigHead(t, e, 0); h.Style.Size != want {
		t.Fatalf("отмена: кегль %v, ждали %v", h.Style.Size, want)
	}
	e.Redo()
	// Свой кегль: крупный на той же полосе остаётся, в ряду — уменьшается; мелкий — остаётся.
	st = vigHead(t, e, 0).Style
	st.Size = auto + 6
	e.SetTextStyle(h0.ID, st)
	setTpl(t, e, sid, 0, "head-g-top", vigOf(e, 0).TText)
	if h := vigHead(t, e, 0); h.Style.Size != auto+6 {
		t.Fatalf("свой крупный на полосе: %v", h.Style.Size)
	}
	setTpl(t, e, sid, 0, "t-left-cell", vigOf(e, 0).TText)
	if h := vigHead(t, e, 0); h.Style.Size != want {
		t.Fatalf("свой крупный в ряду: %v, ждали %v", h.Style.Size, want)
	}
	st = vigHead(t, e, 0).Style
	st.Size = 12
	e.SetTextStyle(h0.ID, st)
	for _, tpl := range []string{"head-t-left", "t-left-cell"} {
		setTpl(t, e, sid, 0, tpl, vigOf(e, 0).TText)
		if h := vigHead(t, e, 0); h.Style.Size != 12 {
			t.Fatalf("%s: свой мелкий кегль стал %v", tpl, h.Style.Size)
		}
	}
}

// Превью каталога несёт вид (таблица design §1.2; пакет генератора отсюда не виден — depguard)
// и шапку в долях листа: у шаблонов «head…» и «…-cell»; у своих вида нет. Подсказки формы и
// портрета у каталога нет (056 №5): Shape, Round, Ratio и формы коробок пустые, и у «Россыпи».
func TestVigChoicesHints(t *testing.T) {
	fam := map[string]string{
		"grid": "inrow", "head": "inrow", "head-low": "inrow", "t-center": "inrow", "head-t-center": "inrow", "cell-head": "inrow",
		"t-left": "side", "head-t-left": "side", "t-right": "side", "head-t-right": "side", "t-top": "side", "head-t-top": "side",
		"t-bottom": "side", "head-t-bottom": "side", "t-left-cell": "side",
		"g-top": "group", "head-g-top": "group", "g-bottom": "group", "head-g-bottom": "group", "head-t-left-g": "group", "t-top-g": "group",
		"t-left-big": "big", "head-t-left-big": "big", "t-left-big-cell": "big",
		"lead-left": "staff", "head-lead-left": "staff",
		"round": "round", "head-round": "round", "head-t-left-round": "round",
	}
	e, sid, tpl := ownEd(t)
	cs := e.VignetteChoices(sid, 2, 24, "3:4", []model.Template{tpl})
	if len(cs) != len(fam)+1 { // лист: 29 шаблонов без Split и свой
		t.Fatalf("превью %d, ждали %d + свой", len(cs), len(fam))
	}
	for _, c := range cs[:len(fam)] {
		if c.Family == "" || c.Family != fam[c.ID] || c.Shape != "" || c.Round != 0 || c.Own {
			t.Fatalf("%s: вид %q (ждали %q), форма %q %v", c.ID, c.Family, fam[c.ID], c.Shape, c.Round)
		}
		if c.Ratio != "" {
			t.Fatalf("%s: портрет-подсказка %q", c.ID, c.Ratio)
		}
		for k, b := range c.Boxes {
			if b.Shape != "" || b.Round != 0 {
				t.Fatalf("%s: коробка %d (%s) форма %q %v", c.ID, k, c.Roles[k], b.Shape, b.Round)
			}
		}
		if want := strings.Contains(c.ID, "head") || strings.HasSuffix(c.ID, "-cell"); (c.Head != nil) != want {
			t.Fatalf("%s: шапка %+v, ждали %v", c.ID, c.Head, want)
		}
		if h := c.Head; h != nil && (h.X < 0 || h.Y < 0 || h.W <= 0 || h.H <= 0 || h.X+h.W > 1+1e-9 || h.Y+h.H > 1+1e-9) {
			t.Fatalf("%s: шапка вне листа %+v", c.ID, *h)
		}
	}
	if own := cs[len(cs)-1]; !own.Own || own.Family != "" {
		t.Fatalf("свой шаблон: %+v", own)
	}
}

// Нетронутые виньетки новых видов пересобираются при смене полей листа (Relay) и остаются
// нетронутыми: план детерминирован.
func TestRelayNewTpls(t *testing.T) {
	for _, id := range []string{"t-center", "head-t-center", "cell-head", "lead-left", "round", "t-left-big"} {
		e, sid := vigSheet(t, 25)
		setTpl(t, e, sid, 0, id, vigOf(e, 0).TText)
		manual := func() bool { return e.View().Spreads[0].Vig.Manual }
		if manual() {
			t.Fatalf("%s: сразу после «Применить» — «Ручная»", id)
		}
		ids := edtest.PlaceIDsOf(e)
		x0 := edtest.P(e).Spreads[0].Grid[0].X
		st := edtest.P(e).Settings
		st.Margin += 4
		e.SetSettings(st)
		if manual() || !slices.Equal(edtest.PlaceIDsOf(e), ids) || edtest.P(e).Spreads[0].Grid[0].X == x0 {
			t.Fatalf("%s: после смены полей «Ручная» %v, ID %v → %v, x %v", id, manual(), ids, edtest.PlaceIDsOf(e), x0)
		}
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		setTpl(t, e, sid, 0, id, vigOf(e, 0).TText)
		edtest.Same(t, e, rev, undo, id+": повтор после пересборки")
	}
}

// headH — высота шаблонного текста шапки в стиле st кеглем pt по ширине w (как рамка BoxOf).
// Наибольший кегль и целые слова проверяют TestHeadFit ядра и этот тест через headFits: пакет text отсюда не виден.
func headH(st model.TextStyle, pt, w float64) float64 {
	st.Size = pt
	return model.BoxOf(&model.TextBlock{Text: model.VigHeadText, Style: st, W: w}).H
}

// headFits — шаблонный текст шапки кеглем pt влезает в полосу w×h целыми словами (мерка headFit
// ядра; области доступна только model, поэтому слово «целое», если в ширину w оно встаёт в ту же
// высоту, что и без предела ширины).
func headFits(st model.TextStyle, pt, w, h float64) bool {
	if headH(st, pt, w) > h {
		return false
	}
	for _, word := range strings.Fields(model.VigHeadText) {
		st.Size = pt
		box := func(w float64) float64 { return model.BoxOf(&model.TextBlock{Text: word, Style: st, W: w}).H }
		if box(w) > box(1e4) {
			return false
		}
	}
	return true
}

// Шаблон без шапки (решение №44 0.10: «убрать насовсем — шаблон без шапки»): «Применить» снимает
// шаблонную шапку одним шагом, поля шапки остаются, отмена возвращает; обратно на шаблон с
// шапкой — новая. Повтор — холостой.
func TestVignetteNoHeadTplDropsHead(t *testing.T) {
	e, sid := vigSheet(t, 25)
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: vigOf(e, 0), Vars: model.Vars{Title: "Школа № 1", Year: "2026"}}, nil); err != nil {
		t.Fatal(err)
	}
	s0 := edtest.P(e).Spreads[0]
	before, vars := len(s0.Texts), *s0.Vars
	undo := edtest.UndoLen(e)
	setTpl(t, e, sid, 0, "grid", vigOf(e, 0).TText)
	s := edtest.P(e).Spreads[0]
	if s.Vig.Head != "" || len(s.Texts) != before-1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шаблон без шапки: Head %q, надписей %d (было %d), шагов %d", s.Vig.Head, len(s.Texts), before, edtest.UndoLen(e)-undo)
	}
	if s.Vars == nil || *s.Vars != vars {
		t.Fatalf("поля шапки потерялись: %+v, были %+v", s.Vars, vars)
	}
	rev := edtest.Rev(e)
	setTpl(t, e, sid, 0, "grid", vigOf(e, 0).TText)
	edtest.Same(t, e, rev, undo+1, "повтор шаблона без шапки")
	if q := saveLoad(t, e); q.Spreads[0].Vig.Head != "" || len(q.Spreads[0].Texts) != before-1 {
		t.Fatal("снятая шапка не пережила Save→Load")
	}
	e.Undo()
	if vigHead(t, e, 0).Text != model.VigHeadText {
		t.Fatal("отмена не вернула шапку")
	}
	e.Redo()
	setTpl(t, e, sid, 0, "head", vigOf(e, 0).TText)
	if vigHead(t, e, 0).Text != model.VigHeadText || len(edtest.P(e).Spreads[0].Texts) != before {
		t.Fatal("шаблон с шапкой не вернул её")
	}
}

// Шапка в ряду (049 §1.3.4, §3.2) по всем шаблонам, числам класса и учителей: рамка не ниже
// своей полосы и нижнего поля листа, слова не рвутся. Отказ шаблона или превью без шапки
// пропускаются, но у каждого шаблона проверенные случаи есть — иначе тест молча пуст.
func TestVignetteHeadCellFitsBand(t *testing.T) {
	for _, tpl := range []string{"cell-head", "t-left-cell", "t-left-big-cell"} {
		checked := 0
		for n := 1; n <= 60; n++ {
			for _, tc := range []int{0, 1, 2, 3} {
				e, sid := vigSheet(t, 0)
				v := vigOf(e, 0)
				c := choiceOf(t, e.VignetteChoices(sid, tc, n, v.Ratio, nil), tpl)
				v.Tpl, v.Pupils, v.Teachers = tpl, n, tc
				if _, _, err := e.SetVignette(sid, v, nil); err != nil || c.Head == nil {
					continue
				}
				s := edtest.P(e).Spreads[0]
				h := s.VigHead(s.Vig.Head)
				if h == nil {
					t.Fatalf("%s %d/%d: нет шапки", tpl, tc, n)
				}
				g := edtest.P(e).Geo(s)
				b := model.BoxOf(h)
				band := (c.Head.Y + c.Head.H) * g.H
				if b.Y+b.H > band+0.01 || b.Y+b.H > g.H-g.Margin+0.01 {
					t.Fatalf("%s %d/%d: шапка %v пт до %.1f мм, полоса до %.1f, поле листа с %.1f", tpl, tc, n, h.Style.Size, b.Y+b.H, band, g.H-g.Margin)
				}
				checked++
			}
		}
		if checked == 0 {
			t.Fatalf("%s: ни одного проверенного случая (все — отказ или превью без шапки)", tpl)
		}
	}
}

// «Копировать раскладку» с оформлением с виньетки на две виньетки: шапка каждой цели — копия
// шапки источника (как у «Дублировать»), после «Применить» шапка одна; копия — один шаг отмены.
func TestCopyGridVigHeadFollows(t *testing.T) {
	e, sid := vigSheet(t, 0)
	var dst []model.GridTarget
	for i := 1; i <= 2; i++ {
		if _, err := e.AddVignette(-1); err != nil {
			t.Fatal(err)
		}
		v := vigOf(e, i)
		v.Tpl, v.Pupils, v.Teachers = "cell-head", 25, 1
		sp := edtest.P(e).Spreads[i].ID
		if _, _, err := e.SetVignette(sp, v, nil); err != nil {
			t.Fatal(err)
		}
		dst = append(dst, model.GridTarget{Spread: sp, Scope: model.ScopeSpread})
	}
	undo := edtest.UndoLen(e)
	if _, _, err := e.CopyGrid(sid, model.ScopeSpread, dst, true, false); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("копия — шагов %d", edtest.UndoLen(e)-undo)
	}
	src := vigHead(t, e, 0)
	for i := 1; i <= 2; i++ {
		s := edtest.P(e).Spreads[i]
		h := s.VigHead(s.Vig.Head)
		if h == nil || h.ID == src.ID || h.Text != src.Text {
			t.Fatalf("лист %d: после копии шапка %+v, надписей %d", i, h, len(s.Texts))
		}
	}
	if q := saveLoad(t, e); q.Spreads[1].VigHead(q.Spreads[1].Vig.Head) == nil {
		t.Fatal("связь шапки не пережила Save→Load")
	}
	for i := 1; i <= 2; i++ {
		v := vigOf(e, i)
		v.Pupils = 26
		if _, _, err := e.SetVignette(edtest.P(e).Spreads[i].ID, v, nil); err != nil {
			t.Fatal(err)
		}
		if n := len(edtest.P(e).Spreads[i].Texts); n != 1 {
			t.Fatalf("лист %d: после «Применить» надписей %d, ждали одну шапку", i, n)
		}
	}
	for range 3 {
		e.Undo()
	}
	if s := edtest.P(e).Spreads[1]; s.Vig.Tpl != "cell-head" || len(s.Texts) != 1 || s.VigHead(s.Vig.Head) == nil {
		t.Fatal("отмена копии не вернула шапку цели")
	}
}
