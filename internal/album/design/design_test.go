package design_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// frameOf — кадр фото id в виде.
func frameOf(t *testing.T, v model.View, id string) model.FrameView {
	t.Helper()
	for _, s := range v.Spreads {
		if i := slices.Index(s.Photos, id); i >= 0 {
			return s.Frames[i]
		}
	}
	t.Fatalf("фото %s нет в виде", id)
	return model.FrameView{}
}

func TestFrameFollowsPhoto(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetFrame("a", 2, 0.5, -0.25)
	want := model.Frame{S: 2, X: 0.5, Y: -0.25}
	check := func(when string, v model.View) {
		t.Helper()
		f := frameOf(t, v, "a")
		if (model.Frame{S: f.S, X: f.X, Y: f.Y}) != want {
			t.Fatalf("%s: кадр %+v", when, f)
		}
	}
	check("SetFrame", e.View())
	check("раскладка", e.StepLayout(s1, "spread", 1))
	check("обмен", e.SwapPhotos("a", "c"))
	v, err := e.MovePhoto("a", s1, 0)
	if err != nil {
		t.Fatal(err)
	}
	check("перенос", v)
	e.RemovePhotos([]string{"a"})
	if _, ok := e.Ref("a"); ok {
		t.Fatal("удалённое фото осталось в проекте")
	}
	check("отмена удаления", e.Undo())
}

func TestSetFrameNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	e.ResetFrame("a")
	e.SetFrame("a", 1.0000001, 0, 0) // округление до 0.001 — тот же кадр
	e.SetFrame("нет", 2, 0, 0)
	if v := e.View(); v.Rev != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостой сброс попал в историю: rev %d→%d", rev, v.Rev)
	}
	e.SetFrame("a", 2, 0, 0)
	e.SetFrame("a", 2, 0, 0)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("снимков %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	if r, _ := e.Ref("a"); r.Frame == nil || r.Frame.S != 2 {
		t.Fatalf("кадр: %+v", r.Frame)
	}
	e.ResetFrame("a")
	if r, _ := e.Ref("a"); r.Frame != nil {
		t.Fatalf("сброс оставил кадр %+v", r.Frame)
	}
}

func TestViewFrames(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: "a", Path: "/a.jpg", W: 3000, H: 2000}})
	cell, f := v.Spreads[0].Rects[0], v.Spreads[0].Frames[0]
	if len(v.Spreads[0].Frames) != 1 || !near(f.Img, cell) || f.S != 1 {
		t.Fatalf("кадр по умолчанию %+v, ячейка %+v", f, cell)
	}
	if want := int(math.Round(3000 / (cell.W / 25.4))); f.DPI != want {
		t.Fatalf("dpi %d, ждали %d", f.DPI, want)
	}
	if v.MinDPI != 200 || v.LowDPI != low(f.DPI, 200) {
		t.Fatalf("порог %d, мало точек %d при %d dpi", v.MinDPI, v.LowDPI, f.DPI)
	}
	v = e.SetFrame("a", 2, -1, 1)
	f = v.Spreads[0].Frames[0]
	if math.Abs(f.Img.W-2*cell.W) > 1e-6 || f.Img.X != cell.X || math.Abs(f.Img.Y+f.Img.H-cell.Y-cell.H) > 1e-6 {
		t.Fatalf("увеличенный кадр %+v в ячейке %+v", f.Img, cell)
	}
	if want := int(math.Round(3000 / (f.Img.W / 25.4))); f.DPI != want || v.LowDPI != 1 {
		t.Fatalf("dpi %d (ждали %d), мало точек %d", f.DPI, want, v.LowDPI)
	}
	// Порог — не выше dpi настроек; фото без размера не считается.
	st := v.Settings
	st.DPI = 100
	e.SetSettings(st)
	v = edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: "z", Path: "/z.jpg"}})
	if f := v.Spreads[1].Frames[0]; f.DPI != 0 {
		t.Fatalf("dpi фото без размера: %d", f.DPI)
	}
	if f = v.Spreads[0].Frames[0]; v.MinDPI != 100 || v.LowDPI != low(f.DPI, 100) {
		t.Fatalf("порог %d, мало точек %d при %d dpi", v.MinDPI, v.LowDPI, f.DPI)
	}
}

func low(dpi, limit int) int {
	if dpi > 0 && dpi < limit {
		return 1
	}
	return 0
}

func TestTextSessionOneUndo(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	undo := edtest.UndoLen(e)
	v, err := e.AddText(s1, 0, "s-1")
	if err != nil {
		t.Fatal(err)
	}
	id := v.Spreads[0].Texts[0].ID
	e.SetText(id, "Иван", "s-1")
	e.SetFrame("нет", 2, 0, 0) // холостая — цепочку не рвёт
	v = e.SetText(id, "Иван и Мария", "s-1")
	if edtest.UndoLen(e) != undo+1 || v.Spreads[0].Texts[0].Text != "Иван и Мария" {
		t.Fatalf("снимков %d, ждали %d; текст %q", edtest.UndoLen(e), undo+1, v.Spreads[0].Texts[0].Text)
	}
	if v = e.Undo(); len(v.Spreads[0].Texts) != 0 {
		t.Fatal("отмена сессии не убрала блок")
	}
	e.Redo()
	// Новая сессия, другая правка и сессия не по формату — отдельные шаги.
	undo = edtest.UndoLen(e)
	e.SetText(id, "1", "s-2")
	e.SetTextBox(id, 10, 10, 100)
	e.SetText(id, "2", "s-2")
	e.SetText(id, "3", "плохая сессия")
	e.SetText(id, "4", "плохая сессия")
	if edtest.UndoLen(e) != undo+5 {
		t.Fatalf("снимков %d, ждали %d", edtest.UndoLen(e), undo+5)
	}
	// Подпись в той же сессии сливается так же.
	undo = edtest.UndoLen(e)
	e.SetCaption("a", "П", "c-1")
	e.SetCaption("a", "Подпись", "c-1")
	if r, _ := e.Ref("a"); edtest.UndoLen(e) != undo+1 || r.Caption == nil || r.Caption.Text != "Подпись" {
		t.Fatalf("подпись: снимков %d, %+v", edtest.UndoLen(e)-undo, r.Caption)
	}
}

func TestTextEmptyDeletes(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), e.View().Rev
	v, _ := e.AddText(s1, 1, "s-1")
	id := v.Spreads[0].Texts[0].ID
	v = e.SetText(id, " \n\t", "s-1")
	if len(v.Spreads[0].Texts) != 0 {
		t.Fatal("пустой текст не удалил блок")
	}
	// Блок добавили и стёрли в одной сессии — пустого шага отмены нет.
	if edtest.UndoLen(e) != undo || v.Rev == rev {
		t.Fatalf("снимков %d, ждали %d", edtest.UndoLen(e), undo)
	}
	v, _ = e.AddText(s1, 1, "")
	id = v.Spreads[0].Texts[0].ID
	e.SetText(id, "", "s-2")
	if v = e.Undo(); len(v.Spreads[0].Texts) != 1 {
		t.Fatal("отмена удаления пустым текстом не вернула блок")
	}
	e.SetCaption("a", "Подпись", "")
	e.SetCaption("a", "  ", "")
	if r, _ := e.Ref("a"); r.Caption != nil {
		t.Fatalf("пустая подпись осталась: %+v", r.Caption)
	}
}

func TestTextFitsPage(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	v, _ := e.AddText(s1, 0, "")
	id := v.Spreads[0].Texts[0].ID
	st := v.Settings
	check := func(when string, v model.View) model.TextView {
		t.Helper()
		tv := v.Spreads[0].Texts[0]
		if tv.W < model.MinTextW || tv.W > 2*st.PageW || tv.X < 0 || tv.X+tv.W > 2*st.PageW+1e-9 ||
			tv.Y < 0 || tv.Y+tv.H > st.PageH+1e-9 {
			t.Fatalf("%s: блок за разворотом %+v", when, tv)
		}
		return tv
	}
	tv := check("большой", e.SetTextBox(id, -50, 1e6, 1e6))
	if tv.W != 2*st.PageW || tv.X != 0 || math.Abs(tv.Y+tv.H-st.PageH) > 1e-6 {
		t.Fatalf("зажим: %+v", tv)
	}
	tv = check("маленький", e.SetTextBox(id, 2*st.PageW-1, 5, 1))
	if tv.W != model.MinTextW || tv.X != 2*st.PageW-model.MinTextW || tv.Y != 5 {
		t.Fatalf("зажим: %+v", tv)
	}
	rev := e.View().Rev
	if v = e.SetTextBox(id, math.NaN(), math.Inf(1), math.Inf(-1)); v.Rev != rev {
		t.Fatal("NaN/Inf должны оставить рамку как была")
	}
	// Длинный текст растит высоту — блок поднимается, чтобы не выйти за низ.
	e.SetTextBox(id, 0, st.PageH-20, 50)
	check("много строк", e.SetText(id, "раз\nдва\nтри\nчетыре\nпять\nшесть\nсемь\nвосемь", ""))
	if lines := e.View().Spreads[0].Texts[0].Lines; len(lines) != 8 || lines[0].Y <= e.View().Spreads[0].Texts[0].Y {
		t.Fatalf("строки: %+v", lines)
	}
}

func TestTextRescaleOnSettings(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	v, _ := e.AddText(s1, 1, "")
	id := v.Spreads[0].Texts[0].ID
	v = e.SetTextBox(id, 330, 150, 120)
	st := v.Settings
	st.PageW, st.PageH = 200, 250
	tv := e.SetSettings(st).Spreads[0].Texts[0]
	if math.Abs(tv.X-220) > 1e-6 || math.Abs(tv.W-80) > 1e-6 || math.Abs(tv.Y-125) > 1e-6 {
		t.Fatalf("после смены формата %+v", tv)
	}
	if tv = e.Undo().Spreads[0].Texts[0]; tv.X != 330 || tv.W != 120 || tv.Y != 150 {
		t.Fatalf("отмена: %+v", tv)
	}
	// Цвет альбома раскладки и тексты не трогает.
	st = e.View().Settings
	st.Bg = "#000000"
	if tv = e.SetSettings(st).Spreads[0].Texts[0]; tv.X != 330 || tv.Y != 150 {
		t.Fatalf("цвет сдвинул текст: %+v", tv)
	}
}

func TestCaptionSurvivesLayoutChange(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	e.SetCaption("a", "Подпись", "")
	v := e.StepLayout(s1, "spread", 1)
	at := func(v model.View) (model.CaptionView, layout.Rect) {
		for i, s := range v.Spreads {
			if len(s.Captions) == 1 {
				return s.Captions[0], edtest.RectOf(v, i, "a")
			}
		}
		t.Fatal("подпись пропала")
		return model.CaptionView{}, layout.Rect{}
	}
	cv, r := at(v)
	if cv.X != r.X || cv.W != r.W || cv.Y < r.Y+r.H {
		t.Fatalf("подпись %+v под ячейкой %+v", cv, r)
	}
	v, _ = e.MovePhoto("a", s2, -1)
	if cv, r = at(v); cv.X != r.X || cv.Text != "Подпись" {
		t.Fatalf("после переноса %+v, ячейка %+v", cv, r)
	}
}

func TestMergeSpreadsTexts(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	e.AddText(s1, 0, "")
	v, _ := e.AddText(s2, 1, "")
	second := v.Spreads[1].Texts[0]
	e.SetBackground(s1, "left", "#111111")
	e.SetBackground(s2, "right", "#222222")
	v = e.MergeSpreads(s1)
	if len(v.Spreads) != 1 || len(v.Spreads[0].Texts) != 2 {
		t.Fatalf("склейка: %+v", v.Spreads)
	}
	if got := v.Spreads[0].Texts[1]; got.ID != second.ID || got.X != second.X || got.Y != second.Y {
		t.Fatalf("текст второго разворота сдвинулся: %+v → %+v", second, got)
	}
	if v.Spreads[0].BgSet != [2]string{"#111111", "#222222"} {
		t.Fatalf("фон склейки: %v", v.Spreads[0].BgSet)
	}
	// Сумма текстов больше MaxTexts — склейки нет.
	e.Undo()
	for range model.MaxTexts - 1 {
		if _, err := e.AddText(s1, 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.AddText(s1, 0, ""); err == nil {
		t.Fatal("блок сверх MaxTexts должен отказывать")
	}
	rev := e.View().Rev
	if v = e.MergeSpreads(s1); len(v.Spreads) != 2 || v.Rev != rev {
		t.Fatalf("склейка сверх MaxTexts прошла: %d разворотов", len(v.Spreads))
	}
}

// Предел 64 надписи (спека 026): AddText до 64 проходит, 65-я — отказ с числом в тексте;
// склейка 32 + 32 проходит, 33 + 32 — нет.
func TestTexts64(t *testing.T) {
	if model.MaxTexts != 64 {
		t.Fatalf("MaxTexts = %d, спека 026 — 64", model.MaxTexts)
	}
	e, s1, _ := edtest.NewTwo(t)
	for i := range 64 {
		if _, err := e.AddText(s1, 0, ""); err != nil {
			t.Fatalf("блок %d: %v", i+1, err)
		}
	}
	rev := e.View().Rev
	_, err := e.AddText(s1, 0, "")
	if err == nil || !strings.Contains(err.Error(), "больше 64 текстов") || e.View().Rev != rev {
		t.Fatalf("65-й блок: %v", err)
	}
	if v := e.View(); len(v.Spreads[0].Texts) != 64 {
		t.Fatalf("в виде %d блоков", len(v.Spreads[0].Texts))
	}
	merge := func(n1, n2 int) model.View {
		edtest.Change(e, func(p *model.Project) {
			p.Spreads[0].Texts, p.Spreads[1].Texts = p.Spreads[0].Texts[:n1], nil
			for range n2 {
				p.Spreads[1].Texts = append(p.Spreads[1].Texts, &model.TextBlock{ID: model.NewID(), X: 300, Y: 20, W: 50, Text: "т", Style: model.PlainStyle()})
			}
		})
		return e.MergeSpreads(s1)
	}
	if v := merge(33, 32); len(v.Spreads) != 2 {
		t.Fatal("склейка 33 + 32 прошла")
	}
	if v := merge(32, 32); len(v.Spreads) != 1 || len(v.Spreads[0].Texts) != 64 {
		t.Fatalf("склейка 32 + 32: %d разворотов", len(v.Spreads))
	}
}

func TestSplitKeepsTexts(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.AddText(s1, 0, "")
	e.SetBackground(s1, "spread", "#f6f1e7")
	v, err := e.SplitSpread(s1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 3 || len(v.Spreads[0].Texts) != 1 || len(v.Spreads[1].Texts) != 0 {
		t.Fatalf("разрез: %+v", v.Spreads)
	}
	if v.Spreads[0].BgSet != [2]string{"#f6f1e7", "#f6f1e7"} || v.Spreads[1].BgSet != [2]string{} {
		t.Fatalf("фон после разреза: %v %v", v.Spreads[0].BgSet, v.Spreads[1].BgSet)
	}
}

func TestNormalizeKeepsTextOnlySpread(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	v, _ := e.AddText(s1, 0, "")
	id := v.Spreads[0].Texts[0].ID
	v = e.RemovePhotos([]string{"a", "b"})
	if len(v.Spreads) != 2 || v.Spreads[0].ID != s1 || len(v.Spreads[0].Photos) != 0 || len(v.Spreads[0].Texts) != 1 {
		t.Fatalf("разворот с текстом удалён: %+v", v.Spreads)
	}
	if sv := v.Spreads[0]; sv.Layout != "" || len(sv.Rects) != 0 || len(sv.Frames) != 0 {
		t.Fatalf("пустой разворот: %+v", sv)
	}
	// Без текста опустевший разворот удаляется, как раньше.
	if v = e.RemovePhotos([]string{"c"}); len(v.Spreads) != 1 || v.Spreads[0].ID != s1 {
		t.Fatalf("разворот без текста остался: %+v", v.Spreads)
	}
	// Фото в разворот с одним текстом встаёт как обычно.
	v = edtest.AddPhotos(e, s1, edtest.Photos("x"), -1)
	if len(v.Spreads[0].Rects) != 1 || v.Spreads[0].Texts[0].ID != id {
		t.Fatalf("фото в разворот с текстом: %+v", v.Spreads[0])
	}
}

func TestBackground(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	if bg := e.View().Spreads[0].Bg; bg != [2]string{"#ffffff", "#ffffff"} {
		t.Fatalf("по умолчанию: %v", bg)
	}
	st := e.View().Settings
	st.Bg = "#ABCDEF"
	v := e.SetSettings(st)
	if v.Settings.Bg != "#abcdef" || v.Spreads[0].Bg != [2]string{"#abcdef", "#abcdef"} {
		t.Fatalf("цвет альбома: %q %v", v.Settings.Bg, v.Spreads[0].Bg)
	}
	v = e.SetBackground(s1, "left", "#000000")
	if v.Spreads[0].Bg != [2]string{"#000000", "#abcdef"} || v.Spreads[0].BgSet != [2]string{"#000000", ""} {
		t.Fatalf("левая: %v %v", v.Spreads[0].Bg, v.Spreads[0].BgSet)
	}
	v = e.SetBackground(s1, "spread", "#1C1C1C")
	if v.Spreads[0].BgSet != [2]string{"#1c1c1c", "#1c1c1c"} || v.Spreads[1].BgSet != [2]string{} {
		t.Fatalf("весь разворот: %v", v.Spreads[0].BgSet)
	}
	v = e.SetBackground(s1, "right", "")
	if v.Spreads[0].Bg != [2]string{"#1c1c1c", "#abcdef"} {
		t.Fatalf("снять правую: %v", v.Spreads[0].Bg)
	}
	rev := v.Rev
	for _, c := range [][2]string{{"spread", "red"}, {"spread", "#12345g"}, {"top", "#ffffff"}, {"left", "#1c1c1c"}} {
		if v = e.SetBackground(s1, c[0], c[1]); v.Rev != rev {
			t.Fatalf("%v должно быть холостой правкой", c)
		}
	}
}

func TestAutoBuildDedupe(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v, _ := e.AutoBuild(edtest.Photos("x", "y", "x", "y", "z"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	var all []string
	for _, s := range v.Spreads {
		all = append(all, s.Photos...)
	}
	slices.Sort(all)
	if !slices.Equal(all, []string{"x", "y", "z"}) {
		t.Fatalf("фото в альбоме: %v", all)
	}
}

func TestSwapOutsideAlbumNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	rev := e.View().Rev
	for _, pair := range [][2]string{{"a", "нет"}, {"нет", "c"}, {"a", "a"}} {
		if v := e.SwapPhotos(pair[0], pair[1]); v.Rev != rev || !slices.Equal(v.Spreads[0].Photos, []string{"a", "b"}) {
			t.Fatalf("обмен %v: %+v", pair, v.Spreads)
		}
	}
	if v := e.SwapPhotos("a", "c"); !slices.Equal(v.Spreads[1].Photos, []string{"a"}) {
		t.Fatalf("обычный обмен: %+v", v.Spreads)
	}
}

func TestSettersRejectNaN(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	nan, inf := math.NaN(), math.Inf(1)
	v, _ := e.AddText(s1, 0, "")
	id := v.Spreads[0].Texts[0].ID
	e.SetFrame("a", nan, inf, -inf)
	e.SetFrame("b", inf, nan, 0.5)
	e.SetTextBox(id, nan, inf, nan)
	e.SetTextStyle(id, model.TextStyle{Font: "montserrat", Size: inf, Color: "#zzzzzz"})
	e.SetCaptionStyle("a", model.TextStyle{Size: nan}, false, 1)
	st := e.View().Settings
	st.PageW, st.Margin = nan, inf
	e.SetSettings(st)
	p, v := e.Snapshot() // паникует, если в проект попало NaN/Inf
	if len(p.Spreads) != 2 || len(p.Photos) != 3 {
		t.Fatalf("проект пострадал: %d разворотов, %d фото", len(p.Spreads), len(p.Photos))
	}
	if f := p.Photos["b"].Frame; f == nil || *f != (model.Frame{S: 1, X: 0, Y: 0.5}) {
		t.Fatalf("кадр b: %+v", f)
	}
	if tv := v.Spreads[0].Texts[0]; tv.Style.Size != 14 || tv.Style.Color != "#222222" {
		t.Fatalf("стиль: %+v", tv.Style)
	}
	if v.Settings.PageW != model.DefaultSettings().PageW {
		t.Fatalf("настройки: %+v", v.Settings)
	}
}

// BenchmarkView300 — View большого альбома на тёплых кэшах: 300 разворотов,
// 300 блоков текста, 100 подписей. Цель — ≤ 20 мс.
func BenchmarkView300(b *testing.B) {
	p := model.New()
	for i := range 300 {
		s := &model.Spread{ID: model.NewID()}
		for j := range 3 {
			id := string(rune('A'+j)) + model.NewID()
			w, h := 3000, 2000
			if (i>>j)&1 == 1 {
				w, h = h, w
			}
			p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		s.Texts = []*model.TextBlock{{ID: model.NewID(), X: 30, Y: 20, W: 200, Text: "Иван и Мария, глава " + string(rune('0'+i%10)),
			Style: model.PlainStyle()}}
		if i%3 == 0 {
			r := p.Photos[s.Photos[0]]
			r.Caption = &model.Caption{Text: "Подпись к фото", Style: model.DefaultCaptionStyle()}
			p.Photos[s.Photos[0]] = r
		}
		p.Spreads = append(p.Spreads, s)
	}
	e := edtest.Open(b, p, "")
	e.View()
	b.ResetTimer()
	for b.Loop() {
		e.View()
	}
}

// BenchmarkView64Texts — View разворота с 64 надписями (спека 026) после правки
// текста: кэш вёрстки тёплый, стек и рамки считаются заново.
func BenchmarkView64Texts(b *testing.B) {
	e := edtest.Open(b, model.New(), "")
	s1 := edtest.NewSpread(e, -1, edtest.Photos("a", "b")).Spreads[0].ID
	for range 64 {
		if _, err := e.AddText(s1, 0, ""); err != nil {
			b.Fatal(err)
		}
	}
	id := e.View().Spreads[0].Texts[0].ID
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		e.SetTextBox(id, float64(10+i%2), 10, 120)
		e.View()
	}
}

func near(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}
