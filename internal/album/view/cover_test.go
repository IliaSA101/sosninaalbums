package view_test

import (
	"encoding/json"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestSpreadViewSize — размер листа в виде из Geo (032 §6): разворот — 2·pageW × pageH со
// швом pageW, лист 031 — pageW × pageH без шва. UI считает проценты от них.
func TestSpreadViewSize(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar(), []string{"x"})
	v := e.View()
	for _, sv := range v.Spreads {
		if sv.W != 562 || sv.H != 284 || sv.Mid != 281 {
			t.Errorf("разворот %s: %v×%v шов %v, хочу 562×284 шов 281", sv.ID, sv.W, sv.H, sv.Mid)
		}
	}
	sh := edtest.Open(t, model.NewSheet(204, 304), "").View()
	if len(sh.Spreads) != 1 {
		t.Fatalf("листов %d", len(sh.Spreads))
	}
	if sv := sh.Spreads[0]; sv.W != 204 || sv.H != 304 || sv.Mid != 0 {
		t.Errorf("лист 031: %v×%v шов %v, хочу 204×304 шов 0", sv.W, sv.H, sv.Mid)
	}
	if sh.Cover != nil {
		t.Error("у листа 031 обложки нет")
	}
}

// TestCoverView — вид обложки (032 §6): лист из Geo с швом в центре корешка, числа, зоны
// с подписями, пустая зона броска, размер 00.jpg при dpi альбома; разворотов столько же.
func TestCoverView(t *testing.T) {
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	v := e.View()
	if v.Cover == nil || len(v.Spreads) != 2 {
		t.Fatalf("обложка %v, разворотов %d", v.Cover, len(v.Spreads))
	}
	c := v.Cover
	// Тонкие листы, 2 разворота: корешок 6; лист 20 + 281 + 6 + 281 + 20 × 284 + 40 (заворот 20, 047).
	if s := c.Sheet; s.ID != model.CoverID || s.W != 608 || s.H != 324 || s.Mid != 304 || !s.Grid || len(s.Places) != 1 {
		t.Fatalf("лист обложки %s %v×%v шов %v, мест %d", s.ID, s.W, s.H, s.Mid, len(s.Places))
	}
	if c.Spine != 6 || c.N != 2 || c.Over || c.Layout != "front" || c.Preset != model.CoverDefault {
		t.Fatalf("числа обложки %+v", *c)
	}
	dpi := float64(v.Settings.DPI)
	if c.PxW != int(math.Round(608/25.4*dpi)) || c.PxH != int(math.Round(324/25.4*dpi)) {
		t.Fatalf("00.jpg %d×%d при %v dpi", c.PxW, c.PxH, dpi)
	}
	labels := map[string]string{}
	for _, z := range c.Zones {
		labels[z.Kind] = z.Label
	}
	want := map[string]string{"back": "Задняя", "front": "Лицевая", "spine": "Корешок 6 мм", "wrap": "Заворот", "trim": "", "safe": ""}
	for k, l := range want {
		if got, ok := labels[k]; !ok || got != l {
			t.Errorf("зона %s: %q, хочу %q", k, got, l)
		}
	}
	if _, ok := labels["hinge"]; ok {
		t.Error("расстав 0 — зоны расстава нет")
	}
	if len(c.Drop) != 1 || c.Drop[0].Kind != "front" || c.Drop[0].Rect.X != 304+3 || c.Drop[0].Rect.W != 608-307 {
		t.Fatalf("пустая зона броска %+v", c.Drop)
	}
	if len(c.Safe) != 2 || c.TextMaxW != 301-2*v.Settings.Safe {
		t.Fatalf("безопасная зона %+v, предел ширины %v", c.Safe, c.TextMaxW)
	}
	if v.EmptyPlaces != 1 || slices.Contains(v.BackdropUsed, "k") {
		t.Fatalf("итоги вида: пустых мест %d, фоны %v", v.EmptyPlaces, v.BackdropUsed)
	}
}

// TestCoverZonesSpineSafe — безопасные линии обложки (047): Safe от линии загиба, SpineSafe от
// края корешка (у Нетпринта — от края расстава); 0 — Settings.Safe со всех сторон, как до 047.
func TestCoverZonesSpineSafe(t *testing.T) {
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	safe := func() []layout.Rect { return e.View().Cover.Safe }
	// Тонкие листы, 2 разворота: X 0 20 301 301 307 307 588 608, Y 0 20 304 324; линии 8 и 12.
	if got := safe(); !slices.Equal(got, []layout.Rect{{X: 28, Y: 28, W: 261, H: 268}, {X: 319, Y: 28, W: 261, H: 268}}) {
		t.Fatalf("линии 8/12: %+v", got)
	}
	st := edtest.P(e).Settings.Safe
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 0, 0 })
	if got := safe(); !slices.Equal(got, []layout.Rect{{X: 20 + st, Y: 20 + st, W: 281 - 2*st, H: 284 - 2*st}, {X: 307 + st, Y: 20 + st, W: 281 - 2*st, H: 284 - 2*st}}) {
		t.Fatalf("0 — зона альбома %v: %+v", st, got)
	}
	// Нетпринт Премиум: расстав 9 по обе стороны корешка, линия корешка — от края расстава.
	if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 5, 10 })
	cg := edtest.P(e).CoverGeo()
	x, y := cg.X, cg.Y
	want := []layout.Rect{
		{X: x[1] + 5, Y: y[1] + 5, W: x[2] - 10 - x[1] - 5, H: y[2] - y[1] - 10},
		{X: x[5] + 10, Y: y[1] + 5, W: x[6] - 5 - x[5] - 10, H: y[2] - y[1] - 10},
	}
	near := func(a, b layout.Rect) bool {
		return math.Abs(a.X-b.X)+math.Abs(a.Y-b.Y)+math.Abs(a.W-b.W)+math.Abs(a.H-b.H) < 1e-9
	}
	if got := safe(); x[3]-x[2] != 9 || len(got) != 2 || !near(got[0], want[0]) || !near(got[1], want[1]) {
		t.Fatalf("расстав: %+v, ждали %+v (X %v)", got, want, x)
	}
}

// TestCoverCaptionBottom — подпись под фото у нижнего края лицевой упирается в линию обреза
// обложки (CoverGeo().Y[2] = 307,5 без обрезной зоны), а не в высоту страницы альбома (284):
// под местом 200…280 места хватает, «!» нет; под местом 230…300 — уже за обрезом.
func TestCoverCaptionBottom(t *testing.T) {
	e := edtest.CoverEd(t, "k", nil, []string{"x"})
	place := func(y float64) model.View {
		return e.SetPlaces(model.CoverID, []model.PlaceBox{{ID: edtest.CoverPlace, X: 400, Y: y, W: 120, H: 80}}, "")
	}
	place(200)
	only := func(v model.View) model.CaptionView {
		t.Helper()
		if len(v.Cover.Sheet.Captions) != 1 {
			t.Fatalf("подписей на обложке %d", len(v.Cover.Sheet.Captions))
		}
		return v.Cover.Sheet.Captions[0]
	}
	if c := only(e.SetCaption("k", "Иванов Иван", "")); c.Overflow != 0 {
		t.Fatalf("подпись под местом 200…280: «!» %v мм, низ %v", c.Overflow, c.Y+c.H)
	}
	if c := only(place(230)); c.Overflow <= 0 {
		t.Fatalf("подпись под местом 230…300 за линией обреза без «!»: низ %v", c.Y+c.H)
	}
}

// TestCoverTextExpands — поля в тексте обложки: на холсте пустое поле — «[Заголовок]» (прежнее
// {Школа} — синоним) и оно в Unfilled; в виде печати (экспорт, Snapshot) — пусто. Своих полей
// у обложки нет — берутся у первой виньетки с полями (031 dialog §4).
func TestCoverTextExpands(t *testing.T) {
	e := edtest.CoverEd(t, "k", nil, []string{"x"})
	id := edtest.P(e).Cover.Sheet.Texts[0].ID
	v := e.SetText(id, "{Школа}", "")
	if got := v.Cover.Sheet.Texts[0]; got.Text != "[Заголовок]" || got.Src != "{Школа}" || !slices.Contains(v.Unfilled, "Заголовок") {
		t.Fatalf("холст: %q (исходник %q), незаполненные %v", got.Text, got.Src, v.Unfilled)
	}
	if _, pv := e.Snapshot(); pv.Cover == nil || pv.Cover.Sheet.Texts[0].Text != "" {
		t.Fatalf("печать: %+v", pv.Cover)
	}
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	v = edtest.Change(e, func(p *model.Project) { p.Spreads[len(p.Spreads)-1].Vars = &model.Vars{Title: "Школа №5"} })
	if got := v.Cover.Sheet; got.Texts[0].Text != "Школа №5" || got.Vars != (model.Vars{}) || len(got.Fields) != 8 { // 8: с «Предметом» (049)
		t.Fatalf("с полями виньетки: %q, свои %+v, меню %v", got.Texts[0].Text, got.Vars, got.Fields)
	}
	if slices.Contains(v.Unfilled, "Заголовок") {
		t.Fatalf("заполненный заголовок в незаполненных: %v", v.Unfilled)
	}
}

// TestSpreadOfMatchesView — вид одного листа (сухой прогон окна виньетки) равен листу полного
// вида холста: сетка и виньетка с полями; неизвестный лист — ошибка и пустой вид.
func TestSpreadOfMatchesView(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar())
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Vars = &model.Vars{Title: "Гимназия"} })
	p, v := edtest.P(e), e.View()
	if len(v.Spreads) != 2 {
		t.Fatalf("листов %d", len(v.Spreads))
	}
	for i, want := range v.Spreads {
		got, err := view.SpreadOf(p, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		if want.Vig != nil { // TplName и Manual виньетки ставит Editor (план мест — не в view, 038 №14)
			if want.Vig.TplName == "" {
				t.Fatalf("лист %d: Editor не назвал шаблон виньетки", i)
			}
			vv := *want.Vig
			vv.TplName, vv.Manual = "", false
			want.Vig = &vv
		}
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(want)
		if string(a) != string(b) {
			t.Fatalf("лист %d:\n%s\n%s", i, a, b)
		}
	}
	if sv, err := view.SpreadOf(p, "нет"); err == nil || sv.ID != "" {
		t.Fatalf("неизвестный лист: %v, %q", err, sv.ID)
	}
}

// TestCoverViewSpineText — CoverView.SpineText (058 №19): ID надписи на корешке по условию
// «Надписи на корешке»; прямой текст и повёрнутый вне корешка — не она; повтор кнопки — холостой
// и ID тот же; Save→Load — тот же ID; отмена и удаление — "".
func TestCoverViewSpineText(t *testing.T) {
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	spine := func() string { t.Helper(); return e.View().Cover.SpineText }
	if s := spine(); s != "" {
		t.Fatalf("без надписи: %q", s)
	}
	if _, err := e.AddText(model.CoverID, 1, ""); err != nil { // прямой текст на лицевой
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { // повёрнутый — на лицевой, не в корешке
		tb := *p.Cover.Sheet.Texts[0]
		tb.ID, tb.Rot = "tilted", 10
		p.Cover.Sheet.Texts = append(p.Cover.Sheet.Texts, &tb)
	})
	if s := spine(); s != "" {
		t.Fatalf("прямой и повёрнутый вне корешка: %q", s)
	}
	undo, n := edtest.UndoLen(e), len(edtest.P(e).Cover.Sheet.Texts)
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	ts := edtest.P(e).Cover.Sheet.Texts
	id := ts[len(ts)-1].ID
	if s := spine(); s == "" || s != id || len(ts) != n+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("надпись %q, ждали %q; текстов %d, шагов %d", s, id, len(ts), edtest.UndoLen(e)-undo)
	}
	rev := edtest.Rev(e)
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo+1, "повтор «Надписи»")
	if s := spine(); s != id {
		t.Fatalf("после повтора %q", s)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if s := edtest.Open(t, q, path).View().Cover.SpineText; s != id {
		t.Fatalf("после Save→Load %q, ждали %q", s, id)
	}
	e.Undo()
	if s := spine(); s != "" {
		t.Fatalf("после отмены %q", s)
	}
	e.Redo()
	e.DeleteText(id)
	if s := spine(); s != "" {
		t.Fatalf("после удаления %q", s)
	}
}
