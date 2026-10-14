package view_test

import (
	"encoding/json"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
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
	// Тонкие листы, 2 разворота: корешок 6; лист 23,5 + 281 + 6 + 281 + 23,5 × 284 + 47.
	if s := c.Sheet; s.ID != model.CoverID || s.W != 615 || s.H != 331 || s.Mid != 307.5 || !s.Grid || len(s.Places) != 1 {
		t.Fatalf("лист обложки %s %v×%v шов %v, мест %d", s.ID, s.W, s.H, s.Mid, len(s.Places))
	}
	if c.Spine != 6 || c.N != 2 || c.Over || c.Layout != "front" || c.Preset != model.CoverDefault {
		t.Fatalf("числа обложки %+v", *c)
	}
	dpi := float64(v.Settings.DPI)
	if c.PxW != int(math.Round(615/25.4*dpi)) || c.PxH != int(math.Round(331/25.4*dpi)) {
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
	if len(c.Drop) != 1 || c.Drop[0].Kind != "front" || c.Drop[0].Rect.X != 307.5+3 || c.Drop[0].Rect.W != 615-310.5 {
		t.Fatalf("пустая зона броска %+v", c.Drop)
	}
	if len(c.Safe) != 2 || c.TextMaxW != 304.5-2*v.Settings.Safe {
		t.Fatalf("безопасная зона %+v, предел ширины %v", c.Safe, c.TextMaxW)
	}
	if v.EmptyPlaces != 1 || slices.Contains(v.BackdropUsed, "k") {
		t.Fatalf("итоги вида: пустых мест %d, фоны %v", v.EmptyPlaces, v.BackdropUsed)
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
	if got := v.Cover.Sheet; got.Texts[0].Text != "Школа №5" || got.Vars != (model.Vars{}) || len(got.Fields) != 7 {
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
