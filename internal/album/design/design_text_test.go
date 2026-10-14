package design_test

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

func TestSpreadNameSet(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	v := e.SetSpreadName(s1, "Сборы")
	if v.Spreads[0].Name != "Сборы" || v.Spreads[1].Name != "" {
		t.Fatalf("название: %q %q", v.Spreads[0].Name, v.Spreads[1].Name)
	}
	if v = e.SetSpreadName(s1, "  "); v.Spreads[0].Name != "" {
		t.Fatalf("пустое не сняло: %q", v.Spreads[0].Name)
	}
	rev := v.Rev
	if v = e.SetSpreadName("нет", "x"); v.Rev != rev {
		t.Fatal("неизвестный разворот поменял проект")
	}
	_ = s2
}

func TestSpreadNameClean(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Сборы\nневесты", "Сборы невесты"},
		{"\u202eЗагс\u0000\t", "Загс"},
		{"  Первый\r\nтанец  ", "Первый танец"},
	} {
		if got := model.CleanName(c.in); got != c.want {
			t.Errorf("CleanName(%q) = %q, ждали %q", c.in, got, c.want)
		}
	}
	long := model.CleanName(strings.Repeat("а", 100))
	if utf8.RuneCountInString(long) != model.MaxNameRunes {
		t.Fatalf("длина %d", utf8.RuneCountInString(long))
	}
	if got := model.CleanName(strings.Repeat("а", 59) + " б"); got != strings.Repeat("а", 59) {
		t.Fatalf("пробел на обрезе: %q", got)
	}
}

func TestSpreadNameNoop(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetSpreadName(s1, "Сборы")
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	if v := e.SetSpreadName(s1, " Сборы\n"); v.Rev != rev || edtest.UndoLen(e) != undo {
		t.Fatal("то же название — не холостая")
	}
}

func TestSpreadNameUndo(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetSpreadName(s1, "Сборы")
	e.SetSpreadName(s1, "Загс")
	if v := e.Undo(); v.Spreads[0].Name != "Сборы" {
		t.Fatalf("отмена: %q", v.Spreads[0].Name)
	}
	if v := e.Undo(); v.Spreads[0].Name != "" {
		t.Fatalf("вторая отмена: %q", v.Spreads[0].Name)
	}
}

func TestSpreadNameMoveSplitMerge(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	e.SetSpreadName(s1, "Сборы")
	v := e.MoveSpread(s1, 1)
	if v.Spreads[1].ID != s1 || v.Spreads[1].Name != "Сборы" {
		t.Fatalf("перенос: %+v", v.Spreads[1])
	}
	e.MoveSpread(s1, 0)
	v = e.SplitSpread(s1, 1)
	if v.Spreads[0].Name != "Сборы" || v.Spreads[1].Name != "" {
		t.Fatalf("разрез: %q %q", v.Spreads[0].Name, v.Spreads[1].Name)
	}
	// Склейка: название первого; у первого нет — второго.
	v = e.MergeSpreads(s1)
	if len(v.Spreads) != 2 || v.Spreads[0].Name != "Сборы" {
		t.Fatalf("склейка: %+v", v.Spreads)
	}
	e.SetSpreadName(s1, "")
	e.SetSpreadName(s2, "Загс")
	if v = e.MergeSpreads(s1); len(v.Spreads) != 1 || v.Spreads[0].Name != "Загс" {
		t.Fatalf("склейка без своего: %+v", v.Spreads)
	}
	// Автосборка: новые без названия, прежние — с ним.
	v, _ = e.AutoBuild(edtest.Photos("x", "y", "z"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if v.Spreads[0].Name != "Загс" || v.Spreads[len(v.Spreads)-1].Name != "" {
		t.Fatalf("автосборка: %q %q", v.Spreads[0].Name, v.Spreads[len(v.Spreads)-1].Name)
	}
}

func TestDuplicateGridNoName(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar(), []string{"d"})
	e.SetSpreadName("s1", "Календарь")
	e.SetSpreadName("s2", "Свой")
	if _, err := e.DuplicateGrid("s1", 2, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, true, false); err != nil {
		t.Fatal(err)
	}
	for _, s := range edtest.P(e).Spreads {
		want := map[string]string{"s1": "Календарь", "s2": "Свой"}[s.ID]
		if s.Name != want {
			t.Fatalf("разворот %s: %q, ждали %q", s.ID, s.Name, want)
		}
	}
}

func TestAddTextAuto(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 1)
	tb := edtest.Block(t, e, id)
	tv := e.View().Spreads[0].Texts[0]
	st := edtest.P(e).Settings
	want := model.PlainStyle()
	want.Size = model.StartSize(st.PageH)
	nat := text.NaturalWidth("Текст", want)
	if !tb.Auto || !tv.Auto || tv.Text != "Текст" || tv.W < nat || tv.W > nat+0.02+1e-9 || len(tv.Lines) != 1 {
		t.Fatalf("новый блок: %+v (естественная %.2f)", tv, nat)
	}
	if math.Abs(tv.X+tv.W/2-1.5*st.PageW) > 0.01 || math.Abs(tv.Y+tv.H/2-st.PageH/2) > 0.01 || tv.Style != want {
		t.Fatalf("не по центру страницы: %+v", tv)
	}
	if v := e.View(); v.TextMaxW != 284 || tv.Rot != 0 {
		t.Fatalf("textMaxW %v, rot %v", v.TextMaxW, tv.Rot)
	}
}

// grow — набор одной строки в блок «по тексту»: точка выравнивания стоит.
func TestAutoWidthGrow(t *testing.T) {
	for _, rot := range []float64{0, 30} {
		for _, al := range []string{"left", "center", "right"} {
			e, s1, _ := edtest.NewTwo(t)
			id := edtest.AddText(t, e, s1, 0)
			st := model.PlainStyle()
			st.Align = al
			e.SetTextStyle(id, st)
			e.SetTextBox(id, 100, 100, edtest.Block(t, e, id).W)
			e.SetTextAngle(id, rot)
			ax, ay := model.BoxOf(edtest.Block(t, e, id)).Anchor(al)
			w0 := edtest.Block(t, e, id).W
			e.SetText(id, "Иван и Мария приглашают вас", "")
			tb := edtest.Block(t, e, id)
			bx, by := model.BoxOf(tb).Anchor(al)
			if !tb.Auto || tb.W <= w0 || math.Abs(bx-ax) > 0.01 || math.Abs(by-ay) > 0.01 {
				t.Fatalf("%v° %s: якорь (%.3f, %.3f) → (%.3f, %.3f), w %.2f → %.2f", rot, al, ax, ay, bx, by, w0, tb.W)
			}
		}
	}
}

func TestAutoWidthCap(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	long := strings.Repeat("Иван и Мария приглашают вас на свадьбу ", 6)
	v := e.SetText(id, long, "")
	tv := v.Spreads[0].Texts[0]
	if tv.W != v.TextMaxW || len(tv.Lines) < 2 || !tv.Auto {
		t.Fatalf("длинный: w %.2f, предел %.2f, строк %d", tv.W, v.TextMaxW, len(tv.Lines))
	}
}

func TestAutoWidthStyle(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	before := model.BoxOf(edtest.Block(t, e, id))
	cx, cy := before.Anchor("center")
	st := model.PlainStyle()
	st.Size, st.Font = 90, "cormorant"
	e.SetTextStyle(id, st)
	tb := edtest.Block(t, e, id)
	if want := edtest.P(e).SheetAutoW(nil, tb); tb.W != want || tb.W <= before.W {
		t.Fatalf("ширина %.2f, ждали %.2f (была %.2f)", tb.W, want, before.W)
	}
	if x, y := model.BoxOf(tb).Anchor("center"); math.Abs(x-cx) > 0.01 || math.Abs(y-cy) > 0.01 {
		t.Fatalf("якорь сдвинулся: (%.3f, %.3f) → (%.3f, %.3f)", cx, cy, x, y)
	}
}

func TestSetTextBoxFixes(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	w := edtest.Block(t, e, id).W
	e.SetTextBox(id, 50, 60, w) // перенос: ширина та же
	if tb := edtest.Block(t, e, id); !tb.Auto || tb.X != 50 || tb.Y != 60 {
		t.Fatalf("перенос: %+v", *tb)
	}
	v := e.SetTextBox(id, 50, 60, 120) // ручка
	if tb := edtest.Block(t, e, id); tb.Auto || tb.W != 120 || v.Spreads[0].Texts[0].Auto {
		t.Fatalf("ручка: %+v", *tb)
	}
	// Закреплённая ширина при наборе не меняется.
	e.SetText(id, "Иван и Мария приглашают вас на свадьбу", "")
	if tb := edtest.Block(t, e, id); tb.W != 120 {
		t.Fatalf("закреплённая поменялась: %v", tb.W)
	}
}

func TestSetTextAuto(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetTextBox(id, 50, 60, 120)
	undo := edtest.UndoLen(e)
	e.SetTextAuto(id)
	tb := edtest.Block(t, e, id)
	if !tb.Auto || tb.W != edtest.P(e).SheetAutoW(nil, tb) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("по тексту: %+v", *tb)
	}
	rev := e.View().Rev
	if v := e.SetTextAuto(id); v.Rev != rev {
		t.Fatal("повтор — не холостая")
	}
}

func TestSetTextAngle(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	cx, cy := model.BoxOf(edtest.Block(t, e, id)).Center()
	for _, deg := range []float64{30, 90, -135, 0} {
		v := e.SetTextAngle(id, deg)
		tb := edtest.Block(t, e, id)
		x, y := model.BoxOf(tb).Center()
		if tb.Rot != deg || v.Spreads[0].Texts[0].Rot != deg || math.Abs(x-cx) > 0.01 || math.Abs(y-cy) > 0.01 {
			t.Fatalf("%v°: %+v, центр (%.3f, %.3f), ждали (%.3f, %.3f)", deg, *tb, x, y, cx, cy)
		}
	}
	// Тот же угол — холостая; каждый поворот — шаг отмены.
	e.SetTextAngle(id, 45)
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	if v := e.SetTextAngle(id, 45.04); v.Rev != rev || edtest.UndoLen(e) != undo {
		t.Fatal("тот же угол — не холостая")
	}
	if v := e.Undo(); v.Spreads[0].Texts[0].Rot != 0 {
		t.Fatalf("отмена: %v", v.Spreads[0].Texts[0].Rot)
	}
}

func TestSetTextAngleNorm(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	for _, c := range [][2]float64{{190, -170}, {-180, 180}, {360.04, 0}} {
		if v := e.SetTextAngle(id, c[0]); v.Spreads[0].Texts[0].Rot != c[1] {
			t.Fatalf("%v → %v, ждали %v", c[0], v.Spreads[0].Texts[0].Rot, c[1])
		}
	}
}

func TestSetTextAngleNaN(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetTextAngle(id, 30)
	rev := e.View().Rev
	for _, d := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if v := e.SetTextAngle(id, d); v.Rev != rev {
			t.Fatalf("%v поменял проект", d)
		}
	}
	e.Snapshot() // паникует, если в проект попало NaN
}

func TestCopyGridRotatedText(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar()[:2], []string{"d"})
	// Блок у корешка: угол P на правой странице, а повёрнутый на 90° он лежит
	// левее P — центр описанного прямоугольника слева.
	edtest.Change(e, func(p *model.Project) {
		p.Spreads[0].Texts = []*model.TextBlock{{ID: model.NewID(), X: 283, Y: 100, W: 60, Rot: 90, Text: "Текст", Style: model.PlainStyle()}}
	})
	tb := *edtest.P(e).Spreads[0].Texts[0]
	if a := model.BoxOf(&tb).AABB(); tb.X < edtest.P(e).Settings.PageW || a.X+a.W/2 >= edtest.P(e).Settings.PageW {
		t.Fatalf("заготовка: %+v, aabb %+v", tb, a)
	}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, true, false); err != nil {
		t.Fatal(err)
	}
	if ts := edtest.P(e).Spreads[1].Texts; len(ts) != 1 || ts[0].Rot != 90 || ts[0].X != tb.X {
		t.Fatalf("повёрнутый блок не скопирован с левой страницы: %+v", ts)
	}
}

func TestStartSize(t *testing.T) {
	for h, want := range map[float64]float64{300: 34, 200: 23, 10: 6, 5000: 200} {
		if got := model.ClampStyle(model.TextStyle{Size: model.StartSize(h)}).Size; got != want {
			t.Errorf("высота %v: %v пт, ждали %v", h, got, want)
		}
	}
	e, s1, _ := edtest.NewTwo(t)
	e.SetSettings(model.Settings{PageW: 200, PageH: 200, Margin: 5, Gap: 2, DPI: 300})
	if sz := edtest.Block(t, e, edtest.AddText(t, e, s1, 0)).Style.Size; sz != 23 {
		t.Fatalf("блок на странице 200 мм: %v пт", sz)
	}
}

func TestSetTextScale(t *testing.T) {
	for _, rot := range []float64{0, 30} {
		e, s1, _ := edtest.NewTwo(t)
		id := edtest.AddText(t, e, s1, 0)
		e.SetTextBox(id, 100, 100, 80) // закреплённая ширина
		e.SetTextAngle(id, rot)
		b := edtest.Block(t, e, id)
		x0, y0, w0, s0 := b.X, b.Y, b.W, b.Style.Size
		e.SetTextScale(id, s0*2, x0-7.5, y0-3.25)
		b = edtest.Block(t, e, id)
		if b.Style.Size != s0*2 || b.Auto || math.Abs(b.W-2*w0) > 0.01 || b.X != x0-7.5 || b.Y != y0-3.25 || b.Rot != rot {
			t.Fatalf("%v°: %+v (было x %v y %v w %v)", rot, b, x0, y0, w0)
		}
		// Один шаг undo: и кегль, и угол.
		e.Undo()
		if b = edtest.Block(t, e, id); b.Style.Size != s0 || b.W != w0 || b.X != x0 || b.Y != y0 {
			t.Fatalf("undo: %+v", b)
		}
	}
}

func TestSetTextScaleAuto(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetTextBox(id, 100, 100, edtest.Block(t, e, id).W)
	e.SetTextAuto(id)
	b := edtest.Block(t, e, id)
	x0, y0, w0 := b.X, b.Y, b.W
	e.SetTextScale(id, b.Style.Size+10, x0, y0)
	b = edtest.Block(t, e, id)
	if !b.Auto || b.W <= w0 || b.W != edtest.P(e).SheetAutoW(nil, b) || b.X != x0 || b.Y != y0 {
		t.Fatalf("auto: %+v (было w %v)", b, w0)
	}
}

// NaN/Inf в x, y — прежний угол; тот же кегль с другим углом — сдвиг; без изменений — холостая.
func TestSetTextScaleXY(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetTextBox(id, 100, 100, 80)
	b := edtest.Block(t, e, id)
	x0, y0, w0, s0 := b.X, b.Y, b.W, b.Style.Size
	e.SetTextScale(id, s0+4, math.NaN(), math.Inf(1))
	b = edtest.Block(t, e, id)
	if b.Style.Size != s0+4 || b.X != x0 || b.Y != y0 || b.W <= w0 {
		t.Fatalf("NaN-угол: %+v", b)
	}
	rev := e.View().Rev
	if e.SetTextScale(id, s0+4, x0, y0).Rev != rev || e.SetTextScale(id, s0+4, x0+0.001, y0).Rev != rev {
		t.Fatal("тот же кегль и угол — холостая правка")
	}
	e.SetTextScale(id, s0+4, x0+5, y0+6)
	if b = edtest.Block(t, e, id); b.X != x0+5 || b.Y != y0+6 {
		t.Fatalf("сдвиг без смены кегля: %+v", b)
	}
}

func TestSetTextScaleNoop(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	b := edtest.Block(t, e, id)
	rev := e.View().Rev
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -5, b.Style.Size} {
		if got := e.SetTextScale(id, v, b.X, b.Y).Rev; got != rev {
			t.Fatalf("size %v поменял проект", v)
		}
	}
	if e.SetTextScale("нет", 20, 0, 0).Rev != rev {
		t.Fatal("неизвестный блок")
	}
	e.SetTextScale(id, 5000, b.X, b.Y)
	if sz := edtest.Block(t, e, id).Style.Size; sz != text.MaxSize {
		t.Fatalf("зажим сверху: %v", sz)
	}
	e.SetTextScale(id, 0.5, b.X, b.Y)
	if sz := edtest.Block(t, e, id).Style.Size; sz != text.MinSize {
		t.Fatalf("зажим снизу: %v", sz)
	}
}

// Блок во всю ширину поля, сужен боковой ручкой и возвращён «по тексту»: точка
// выравнивания держится, но блок не вылетает к краю разворота — встаёт в поле своей
// страницы, где и был.
func TestSetTextAutoBackToPage(t *testing.T) {
	long := strings.Repeat("слово ", 80)
	for page := 0; page < 2; page++ {
		e, s1, _ := edtest.NewTwo(t)
		id := edtest.AddText(t, e, s1, page)
		e.SetText(id, long, "")
		b := edtest.Block(t, e, id)
		maxW := edtest.P(e).TextMaxW(nil)
		if b.W != maxW {
			t.Fatalf("p%d: ширина %v, ждали %v", page, b.W, maxW)
		}
		x0, w0 := b.X, b.W
		e.SetTextBox(id, x0, b.Y, w0-90)
		e.SetTextAuto(id)
		b = edtest.Block(t, e, id)
		if !b.Auto || b.W != w0 || math.Abs(b.X-x0) > 0.01 {
			t.Fatalf("p%d: x %v w %v, было x %v w %v", page, b.X, b.W, x0, w0)
		}
	}
}

// Возврат к авто-ширине держит блок в безопасном поле его страницы при любом выравнивании.
func TestSetTextAutoPageFitTable(t *testing.T) {
	long := strings.Repeat("слово ", 80)
	for _, align := range []string{"left", "center", "right"} {
		for page := 0; page < 2; page++ {
			e, s1, _ := edtest.NewTwo(t)
			id := edtest.AddText(t, e, s1, page)
			e.SetText(id, long, "")
			edtest.StyleAlign(t, e, id, align)
			pw, maxW := edtest.P(e).Settings.PageW, edtest.P(e).TextMaxW(nil)
			l := float64(page)*pw + model.DefaultSafe
			e.SetTextBox(id, l, 20, maxW-90)
			e.SetTextAuto(id)
			b := edtest.Block(t, e, id)
			if !b.Auto || b.W != maxW || math.Abs(b.X-l) > 0.01 {
				t.Fatalf("%s p%d: x %v w %v, ждали x %v w %v", align, page, b.X, b.W, l, maxW)
			}
		}
	}
}

// Смена кегля у блока «по тексту» у поля не выталкивает его за поле страницы.
func TestSetTextStylePageFit(t *testing.T) {
	for page := 0; page < 2; page++ {
		e, s1, _ := edtest.NewTwo(t)
		id := edtest.AddText(t, e, s1, page)
		e.SetText(id, "Привет, мир", "")
		edtest.StyleAlign(t, e, id, "center")
		l := float64(page)*edtest.P(e).Settings.PageW + model.DefaultSafe
		e.SetTextBox(id, l, 20, 30)
		e.SetTextAuto(id)
		st := edtest.Block(t, e, id).Style
		st.Size += 20
		e.SetTextStyle(id, st)
		if b := edtest.Block(t, e, id); b.X < l-0.01 {
			t.Fatalf("p%d: x %v левее поля %v", page, b.X, l)
		}
	}
}

// Блок через корешок pageFitX не трогает: сдвиг только по точке выравнивания.
func TestSetTextAutoSpineUntouched(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetText(id, "Привет", "")
	edtest.StyleAlign(t, e, id, "center")
	pw := edtest.P(e).Settings.PageW
	e.SetTextBox(id, pw-30, 20, 60)
	e.SetTextAuto(id)
	b := edtest.Block(t, e, id)
	want := pw - 30 + (60-b.W)/2
	if !b.Auto || math.Abs(b.X-want) > 0.02 {
		t.Fatalf("x %v, ждали %v (w %v)", b.X, want, b.W)
	}
}

// Ввод текста не зажимает x в поле страницы: рамку растит редактор от якоря.
func TestSetTextKeepsAnchorAtMargin(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetText(id, "Привет", "")
	edtest.StyleAlign(t, e, id, "center")
	e.SetTextBox(id, 20, 20, 40)
	e.SetTextAuto(id)
	b := edtest.Block(t, e, id)
	x0, w0 := b.X, b.W
	e.SetText(id, "Привет, друг", "")
	b = edtest.Block(t, e, id)
	want := x0 + (w0-b.W)/2
	if b.W <= w0 || math.Abs(b.X-want) > 0.02 || b.X >= model.DefaultSafe {
		t.Fatalf("x %v, ждали %v у поля (w %v → %v)", b.X, want, w0, b.W)
	}
	// Левое выравнивание: x не меняется совсем.
	edtest.StyleAlign(t, e, id, "left")
	x1 := edtest.Block(t, e, id).X
	e.SetText(id, "Привет, друг!", "")
	if b = edtest.Block(t, e, id); b.X != x1 {
		t.Fatalf("left: x %v → %v", x1, b.X)
	}
}
