package vigsheet_test

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Форма мест виньетки (спека 044, design §7).

// vigShapes — "роль:форма:скругление" мест листа i по порядку.
func vigShapes(e *album.Editor, i int) []string {
	var out []string
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		out = append(out, model.RoleOf(pl)+":"+pl.Shape+":"+map[float64]string{0: "0", 10: "10"}[pl.Round])
	}
	return out
}

// sameAll — у всех мест роли role форма want ("форма:скругление"); мест роли больше нуля.
func sameAll(t *testing.T, e *album.Editor, role, want string) {
	t.Helper()
	n := 0
	for _, s := range vigShapes(e, 0) {
		if r, rest, _ := strings.Cut(s, ":"); r == role {
			if n++; rest != want {
				t.Fatalf("%s: %s, ждали %s (%v)", role, rest, want, vigShapes(e, 0))
			}
		}
	}
	if n == 0 {
		t.Fatalf("мест роли %s нет", role)
	}
}

// Форма из окна — ученикам и учителям, групповое прямоугольное; новая виньетка — одним шагом.
func TestVignetteShape(t *testing.T) {
	e, sid := vigSheet(t, 25)
	undo := edtest.UndoLen(e)
	st := model.VigSetup{Vig: model.Vignette{Tpl: "t-top-g", Teachers: 2, Pupils: 12, Ratio: "1:1", Shape: model.ShapeOval}}
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 || vigOf(e, 0).Shape != model.ShapeOval {
		t.Fatalf("шагов %d, виньетка %+v", edtest.UndoLen(e)-undo, vigOf(e, 0))
	}
	sameAll(t, e, model.RolePupil, "oval:0")
	sameAll(t, e, model.RoleTeacher, "oval:0")
	sameAll(t, e, model.RoleGroup, ":0")
	// «Скруглённые» — 10 %; групповое по-прежнему прямоугольное.
	st.Vig.Shape, st.Vig.Round = "", 10
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	sameAll(t, e, model.RolePupil, ":10")
	sameAll(t, e, model.RoleTeacher, ":10")
	sameAll(t, e, model.RoleGroup, ":0")
	// Скругление у круга из окна зажимается в 0.
	st.Vig.Shape, st.Vig.Round = model.ShapeOval, 10
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil || vigOf(e, 0).Round != 0 {
		t.Fatalf("круг со скруглением: %v %+v", err, vigOf(e, 0))
	}
	sameAll(t, e, model.RolePupil, "oval:0")
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Смена только формы — без пересборки: сдвинутые места и фото стоят, один шаг; повтор —
// холостая; отмена — прежние формы.
func TestVignetteShapeOnlyKeepsPlaces(t *testing.T) {
	e, sid := vigSheet(t, 25)
	touchVig(t, e, 0, false)
	before := edtest.P(e).Spreads[0]
	boxes, photos := placeBoxes(before), slices.Clone(before.Photos)
	v := vigOf(e, 0)
	v.Shape = model.ShapeOval
	undo := edtest.UndoLen(e)
	if _, left, err := e.SetupVignette(sid, model.VigSetup{Vig: v}, nil); err != nil || left != 0 {
		t.Fatalf("применить: %v, left %d", err, left)
	}
	sp := edtest.P(e).Spreads[0]
	if !slices.Equal(placeBoxes(sp), boxes) || !slices.Equal(sp.Photos, photos) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("места или фото сдвинулись; шагов %d", edtest.UndoLen(e)-undo)
	}
	sameAll(t, e, model.RolePupil, "oval:0")
	sameAll(t, e, model.RoleTeacher, "oval:0")
	if !manual(e, sid) {
		t.Fatal("ручная правка мест потерялась")
	}
	rev := edtest.Rev(e)
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: v}, nil); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo+1, "повтор формы")
	e.Undo()
	sameAll(t, e, model.RolePupil, ":0")
	if vigOf(e, 0).Shape != "" || !slices.Equal(placeBoxes(edtest.P(e).Spreads[0]), boxes) {
		t.Fatalf("отмена: %+v", vigOf(e, 0))
	}
}

// Ручная форма места переживает пересборку (смена формата листа, другие числа, «Вернуть
// авто»): переносится по ID, как «Целиком»; новые места — форма виньетки.
func TestVignetteShapeRelayKeepsOwn(t *testing.T) {
	e, sid := vigSheet(t, 25)
	g := edtest.P(e).Spreads[0].Grid
	own := g[3].ID
	e.SetPlaceShape(sid, []string{own}, model.ShapeOval)
	shapeOf := func() string {
		_, pl := edtest.P(e).Spreads[0].FindPlace(own)
		if pl == nil {
			return "нет места"
		}
		return pl.Shape
	}
	w0 := g[3].W
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	e.SetSettings(st)
	if _, pl := edtest.P(e).Spreads[0].FindPlace(own); shapeOf() != model.ShapeOval || !(pl.W > w0) {
		t.Fatalf("смена формата: форма %q, ширина %v → %v", shapeOf(), w0, pl.W)
	}
	v := vigOf(e, 0)
	v.Pupils++
	if _, _, err := e.SetVignette(sid, v, nil); err != nil || shapeOf() != model.ShapeOval {
		t.Fatalf("другие числа: %v, форма %q", err, shapeOf())
	}
	if _, _, err := e.ResetVignette(sid); err != nil || shapeOf() != model.ShapeOval {
		t.Fatalf("«Вернуть авто»: %v, форма %q", err, shapeOf())
	}
	if n := slices.Index(vigShapes(e, 0), model.RolePupil+":oval:0"); n < 0 {
		t.Fatalf("формы: %v", vigShapes(e, 0))
	}
	// Выбор формы в окне заново — всем людям форма виньетки (ручная тоже).
	v = vigOf(e, 0)
	v.Round = 10
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: v}, nil); err != nil || shapeOf() != "" {
		t.Fatalf("форма из окна: %v, %q", err, shapeOf())
	}
	sameAll(t, e, model.RolePupil, ":10")
}

// Ручное ▢ (076) у места виньетки — как ручной круг: переживает смену формата и числа людей;
// соседи остаются прямоугольными.
func TestVignetteRoundedOwnKept(t *testing.T) {
	e, sid := vigSheet(t, 25)
	g := edtest.P(e).Spreads[0].Grid
	own := g[3].ID
	sameAll(t, e, model.RolePupil, ":0")
	undo := edtest.UndoLen(e)
	e.SetPlaceShape(sid, []string{own}, model.ShapeRounded)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d", edtest.UndoLen(e)-undo)
	}
	formOf := func() string {
		_, pl := edtest.P(e).Spreads[0].FindPlace(own)
		if pl == nil {
			return "нет места"
		}
		return pl.Shape + ":" + map[float64]string{0: "0", 10: "10"}[pl.Round]
	}
	count := func() int {
		return len(slices.DeleteFunc(vigShapes(e, 0), func(s string) bool { return s != model.RolePupil+"::10" }))
	}
	if formOf() != ":10" || count() != 1 {
		t.Fatalf("▢: %q, %v", formOf(), vigShapes(e, 0))
	}
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	e.SetSettings(st)
	if formOf() != ":10" || count() != 1 {
		t.Fatalf("смена формата: %q, %v", formOf(), vigShapes(e, 0))
	}
	v := vigOf(e, 0)
	v.Pupils++
	if _, _, err := e.SetVignette(sid, v, nil); err != nil || formOf() != ":10" || count() != 1 {
		t.Fatalf("другие числа: %v, %q, %v", err, formOf(), vigShapes(e, 0))
	}
}

// Свой шаблон запоминает форму (стиль и коробки), файл шаблонов с ним — v5; выбор шаблона
// приносит форму, как подпись учителя.
func TestVigTemplateShape(t *testing.T) {
	e, sid, _ := ownEd(t)
	v := vigOf(e, 0)
	v.Shape = model.ShapeOval
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: v}, nil); err != nil {
		t.Fatal(err)
	}
	tpl, err := e.VigTemplate(sid)
	if err != nil || tpl.Vig.Style.Shape != model.ShapeOval || tpl.Vig.Style.Round != 0 {
		t.Fatalf("стиль: %v %+v", err, tpl.Vig.Style)
	}
	for i, b := range tpl.Boxes {
		if b.Shape != model.ShapeOval {
			t.Fatalf("коробка %d (%s) без формы: %+v", i, tpl.Vig.Roles[i], b)
		}
	}
	tpl.ID, tpl.Name = "t2", "Круги"
	data, err := model.MarshalTemplates([]model.Template{tpl})
	if err != nil || !bytes.HasPrefix(data, []byte(`{"version":5,`)) {
		t.Fatalf("файл: %v %.20s", err, data)
	}
	back, _, err := model.ParseTemplates(data)
	if err != nil || len(back) != 1 || back[0].Vig.Style.Shape != model.ShapeOval {
		t.Fatalf("круг файла: %v %+v", err, back)
	}
	// Другой лист: каталог с прямоугольниками → свой шаблон с кругами.
	e2, sid2 := vigSheet(t, 25)
	sameAll(t, e2, model.RolePupil, ":0")
	if _, _, err := e2.SetVignette(sid2, model.Vignette{Tpl: model.VigOwnPrefix + back[0].ID, Teachers: 1, Pupils: 24, Ratio: "3:4"}, back); err != nil {
		t.Fatal(err)
	}
	if vigOf(e2, 0).Shape != model.ShapeOval {
		t.Fatalf("виньетка: %+v", vigOf(e2, 0))
	}
	sameAll(t, e2, model.RolePupil, "oval:0")
	sameAll(t, e2, model.RoleTeacher, "oval:0")
}

// formOf — "форма:скругление" для сравнения превью.
func formOf(shape string, round float64) string {
	return shape + ":" + map[float64]string{0: "0", 10: "10"}[round]
}

// Превью своего шаблона с формой несёт её (окно переключает «Форму», как портрет): поля
// Shape/Round и формы коробок людей; групповое и весь каталог — прямоугольники (056 №5);
// удалённый шаблон листа — форма листа.
func TestVigChoicesShape(t *testing.T) {
	e, sid, tpl := ownEd(t)
	styled := func(id, shape string, round float64) model.Template {
		c := tpl
		c.ID, c.Vig = id, &model.VigTpl{VigGeo: tpl.Vig.VigGeo, Style: tpl.Vig.Style}
		c.Vig.Style.Shape, c.Vig.Style.Round = shape, round
		return c
	}
	tpls := []model.Template{styled("t1", model.ShapeOval, 0), styled("t2", "", 10), styled("t3", "", 0)}
	check := func(cs []model.VigChoice, want map[string]string) {
		t.Helper()
		own := 0
		for _, c := range cs {
			w := ":0" // каталог — прямоугольники, и «Россыпь» (056 №5: подсказки формы нет)
			if c.Own {
				own++
				w = want[c.ID]
			}
			if got := formOf(c.Shape, c.Round); got != w {
				t.Fatalf("%s: форма %s, ждали %s", c.ID, got, w)
			}
			for k, b := range c.Boxes {
				bw := w
				if c.Roles[k] == model.RoleGroup {
					bw = ":0"
				}
				if got := formOf(b.Shape, b.Round); got != bw {
					t.Fatalf("%s: коробка %d (%s) %s, ждали %s", c.ID, k, c.Roles[k], got, bw)
				}
			}
		}
		if own != len(want) || len(cs) <= own {
			t.Fatalf("своих превью %d из %d, всего %d", own, len(want), len(cs))
		}
	}
	check(e.VignetteChoices(sid, 2, 24, "3:4", tpls), map[string]string{"my:t1": "oval:0", "my:t2": ":10", "my:t3": ":0"})
	// На листе шаблон с кругами, из файла его удалили — превью «удалённого» с формой листа.
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls); err != nil {
		t.Fatal(err)
	}
	cs := e.VignetteChoices(sid, 1, 24, "3:4", nil)
	if g := cs[len(cs)-1]; !g.Gone {
		t.Fatalf("последнее превью не удалённое: %+v", g)
	}
	check(cs, map[string]string{"my:t1": "oval:0"})
}

// SetVignette ставит форму, что прислали (056 №5): плитка ▦ и окно шлют форму целиком,
// подсказки «кругов» нет — ни Круга, ни 1:1 сами не появляются. Смена шаблона и формы — один шаг,
// повтор холостой, отмена — прежняя форма; переживает Save→Load; мусор формы зажимается.
func TestSetVignetteFormFromUI(t *testing.T) {
	e, sid := vigSheet(t, 25)
	set := func(tpl, ratio, shape string, round float64) {
		t.Helper()
		v := model.Vignette{Tpl: tpl, Teachers: 1, Pupils: 24, Ratio: ratio, Shape: shape, Round: round}
		if _, _, err := e.SetVignette(sid, v, nil); err != nil {
			t.Fatalf("%s %q %v: %v", tpl, shape, round, err)
		}
	}
	people := func(want string) {
		t.Helper()
		sameAll(t, e, model.RolePupil, want)
		sameAll(t, e, model.RoleTeacher, want)
	}
	set("head-t-left", "3:4", model.ShapeOval, 0)
	people("oval:0")
	undo := edtest.UndoLen(e)
	// «Россыпью ▭» с листа-кругов: прямоугольник и портрет как прислали.
	set("round", "3:4", "", 0)
	if v := vigOf(e, 0); v.Tpl != "round" || v.Shape != "" || v.Round != 0 || v.Ratio != "3:4" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Россыпью ▭»: %+v, шагов %d", v, edtest.UndoLen(e)-undo)
	}
	people(":0")
	rev := edtest.Rev(e)
	set("round", "3:4", "", 0)
	edtest.Same(t, e, rev, undo+1, "повтор той же плитки")
	// Тот же шаблон, другая форма: только форма, шаг.
	set("round", "3:4", "", 10)
	if v := vigOf(e, 0); v.Shape != "" || v.Round != 10 || v.Ratio != "3:4" || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("«Россыпью ▢»: %+v, шагов %d", v, edtest.UndoLen(e)-undo)
	}
	people(":10")
	set("head-t-left-g", "3:4", model.ShapeOval, 10) // у круга скругления нет
	if v := vigOf(e, 0); v.Shape != model.ShapeOval || v.Round != 0 || v.Ratio != "3:4" {
		t.Fatalf("«…групповое снизу ◯»: %+v", v)
	}
	people("oval:0")
	sameAll(t, e, model.RoleGroup, ":0") // групповое — всегда прямоугольник
	q := saveLoad(t, e)
	if v := q.Spreads[0].Vig; v.Tpl != "head-t-left-g" || v.Shape != model.ShapeOval || v.Round != 0 || v.Ratio != "3:4" {
		t.Fatalf("после Save→Load: %+v", *v)
	}
	for _, pl := range q.Spreads[0].Grid {
		if r := model.RoleOf(pl); r != model.RoleGroup && (pl.Shape != model.ShapeOval || pl.Round != 0) {
			t.Fatalf("после Save→Load место %s (%s): %q %v", pl.ID, r, pl.Shape, pl.Round)
		}
	}
	e.Undo()
	if v := vigOf(e, 0); v.Tpl != "round" || v.Shape != "" || v.Round != 10 {
		t.Fatalf("отмена: %+v", v)
	}
	people(":10")
	e.Undo()
	people(":0")
	// Мусор формы: незнакомая форма — прямоугольник, скругление — до предела, NaN — 0.
	set("round", "3:4", "нет такой", 1e9)
	if v := vigOf(e, 0); v.Shape != "" || v.Round != model.MaxRound {
		t.Fatalf("зажим: %+v", v)
	}
	set("round", "3:4", "", math.NaN())
	if v := vigOf(e, 0); v.Shape != "" || v.Round != 0 {
		t.Fatalf("NaN: %+v", v)
	}
}

// «Сделать виньеткой» на обычном листе — тоже форма и портрет как прислали, и у шаблонов «Россыпи».
func TestSetVignetteFormNew(t *testing.T) {
	for _, c := range []struct {
		tpl, shape string
		round      float64
	}{{"head-round", "", 0}, {"round", "", 10}, {"head-t-left-round", model.ShapeOval, 0}, {"head-t-left", model.ShapeOval, 0}} {
		e := edtest.Open(t, model.NewSheet(204, 304), "")
		sid := edtest.P(e).Spreads[0].ID
		v := model.Vignette{Tpl: c.tpl, Teachers: 1, Pupils: 24, Ratio: "3:4", Shape: c.shape, Round: c.round}
		if _, _, err := e.SetVignette(sid, v, nil); err != nil {
			t.Fatal(err)
		}
		if g := vigOf(e, 0); g.Shape != c.shape || g.Round != c.round || g.Ratio != "3:4" {
			t.Fatalf("%s/%q: %+v", c.tpl, c.shape, g)
		}
	}
}
