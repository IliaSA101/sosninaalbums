package grid_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// «Сохранить альбом как макет…» (131 G6b): grid.AlbumTplOf и Editor.AlbumTplOf — снимок
// разворотов и обложки в макет. Слияние и предел шагов в чистом виде — model.TestMergeLookSteps.

func saveOpts(ids ...string) model.SaveLookOpts {
	return model.SaveLookOpts{Ids: ids, RepFrom: 1, RepTo: 0, Places: true, Drafts: true}
}

func ids(e *album.Editor) []string {
	var out []string
	for _, s := range edtest.P(e).Spreads {
		out = append(out, s.ID)
	}
	return out
}

// TestAlbumTplOfSnapshot: богатый разворот и «Авто» — места, фон, узор, стикеры (свой — с ключом
// библиотеки), надпись заготовкой; альбом не меняется, фото в макет не попадают.
func TestAlbumTplOfSnapshot(t *testing.T) {
	e := tplEd(t)
	before, rev := snapshot(e), edtest.Rev(e)
	tpl, err := e.AlbumTplOf(saveOpts("s1", "s2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tpl.Steps) != 2 || tpl.V != model.LookV || tpl.Cover != nil {
		t.Fatalf("макет: шагов %d, V %d, обложка %v", len(tpl.Steps), tpl.V, tpl.Cover)
	}
	a, b := tpl.Steps[0].Tpl, tpl.Steps[1].Tpl
	if len(a.Places) != 3 || len(b.Places) != 2 {
		t.Errorf("мест: s1 %d (ждали 3), «Авто» s2 %d (ждали 2)", len(a.Places), len(b.Places))
	}
	if a.Pages != 2 || len(a.Page) != 2 || a.Page[0].Color != "#aabbcc" || a.Page[1].Color != "#ddeeff" || a.Page[1].Pat == nil || a.Page[0].Lib != kBg {
		t.Errorf("фон s1: %+v", a.Page)
	}
	if len(a.Stickers) != 3 || len(a.Drafts) != 1 || len(a.Texts) != 0 || a.Drafts[0].Text != "Наша свадьба" {
		t.Errorf("оформление s1: стикеров %d, заготовок %d, надписей %d", len(a.Stickers), len(a.Drafts), len(a.Texts))
	}
	own := slices.IndexFunc(a.Stickers, func(it model.TplItem) bool { return it.Lib == kSt })
	if own < 0 {
		t.Error("свой стикер из библиотеки не снят")
	}
	if tpl.Steps[0].Role != "" || tpl.Steps[0].Name != "" {
		t.Errorf("роль/имя шага: %q %q", tpl.Steps[0].Role, tpl.Steps[0].Name)
	}
	data, _ := json.Marshal(tpl)
	for _, bad := range []string{`"photo"`, ".jpg", `"a"`, `"d"`} { // фото и пути не входят
		if strings.Contains(string(data), bad) {
			t.Errorf("в макете %s", bad)
		}
	}
	if !slices.Equal(snapshot(e), before) || edtest.Rev(e) != rev {
		t.Error("проект изменился при снятии")
	}
}

// TestAlbumTplOfFlags: «Места для фото» и «Надписи — заготовками» выключены — мест и надписей нет;
// заготовки включены — обычная надпись тоже заготовка.
func TestAlbumTplOfFlags(t *testing.T) {
	e := tplEd(t)
	o := saveOpts("s1")
	o.Places, o.Drafts = false, false
	tpl, err := e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	if st := tpl.Steps[0].Tpl; len(st.Places) != 0 || len(st.Drafts)+len(st.Texts) != 0 || len(st.Stickers) != 3 {
		t.Errorf("без мест и надписей: %d мест, %d надписей, %d стикеров", len(st.Places), len(st.Drafts)+len(st.Texts), len(st.Stickers))
	}
	o.Places, o.Drafts = true, true
	if tpl, err = e.AlbumTplOf(o); err != nil || len(tpl.Steps[0].Tpl.Drafts) != 1 || len(tpl.Steps[0].Tpl.Places) != 3 {
		t.Errorf("с флажками: %v, %+v", err, tpl.Steps[0].Tpl)
	}
}

// TestAlbumTplOfRepeatMerge: повторяемые сливаются по TplKey, начало и конец остаются как есть;
// пустой разворот в серию не идёт, позиции повторяемых считаются по выбранным id.
func TestAlbumTplOfRepeatMerge(t *testing.T) {
	e := tplEd(t)
	tpl0, err := e.AlbumTplOf(saveOpts("s1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"s3", "s4"} { // s3 и s4 — копии s1
		if _, _, _, err := e.ApplySpreadTpl(id, model.ScopeSpread, tpl0.Steps[0].Tpl, tplLibs()); err != nil {
			t.Fatal(err)
		}
	}
	o := saveOpts("s1", "s2", "s3", "s4")
	o.RepFrom, o.RepTo = 2, 4
	tpl, err := e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	roles := make([]string, len(tpl.Steps))
	for i, st := range tpl.Steps {
		roles[i] = st.Role
	}
	if !slices.Equal(roles, []string{"", model.RoleRepeat, model.RoleRepeat}) { // s3 = s4: слиты в один
		t.Fatalf("роли после слияния: %q", roles)
	}
	// Тот же выбор с пустой серединой: пустой s5 пропускается, позиции — по выбранным id.
	edtest.Change(e, func(p *model.Project) { p.Spreads = append(p.Spreads, &model.Spread{ID: "s5"}) })
	o = saveOpts("s1", "s5", "s2")
	o.RepFrom, o.RepTo = 3, 3
	tpl, err = e.AlbumTplOf(o)
	if err != nil || len(tpl.Steps) != 2 || tpl.Steps[0].Role != "" || tpl.Steps[1].Role != model.RoleRepeat {
		t.Errorf("пустой разворот: %v, %+v", err, tpl.Steps)
	}
	// Без повторяемых одинаковые развороты остаются шагами: автор выбрал серию из них.
	o = saveOpts("s3", "s4")
	if tpl, err = e.AlbumTplOf(o); err != nil || len(tpl.Steps) != 2 {
		t.Errorf("без повтора: %v, шагов %d", err, len(tpl.Steps))
	}
	// Диапазон за краями зажимается; перевёрнутый — повторяемых нет.
	o.RepFrom, o.RepTo = -5, 99
	if tpl, err = e.AlbumTplOf(o); err != nil || len(tpl.Steps) != 1 || tpl.Steps[0].Role != model.RoleRepeat {
		t.Errorf("диапазон шире выбора: %v, %+v", err, tpl.Steps)
	}
}

// TestAlbumTplOfErrors: пустой выбор, чужой id, обложка, повтор id, слишком длинный список, нечего снять.
func TestAlbumTplOfErrors(t *testing.T) {
	e := tplEd(t)
	for name, c := range map[string]struct {
		o    model.SaveLookOpts
		want error
	}{
		"пусто":       {saveOpts(), grid.ErrLookNoSpreads},
		"чужой":       {saveOpts("s1", "zz"), grid.ErrLookIDs},
		"обложка":     {saveOpts(model.CoverID), grid.ErrLookIDs},
		"дубль":       {saveOpts("s1", "s1"), grid.ErrLookIDs},
		"пустые":      {saveOpts("s3", "s4"), nil},
		"без снятого": {model.SaveLookOpts{Ids: []string{"s3"}}, nil},
	} {
		_, err := e.AlbumTplOf(c.o)
		if c.want != nil && !errors.Is(err, c.want) || c.want == nil && !errors.Is(err, grid.ErrLookNothing) {
			t.Errorf("%s: ошибка %v, ждали %v", name, err, c.want)
		}
	}
}

// TestAlbumTplOfIDsLimit: MaxSpreads+1 разных id (в альбоме их нет) — ErrLookIDs до проверки каждого
// по альбому; в альбоме больше MaxSpreads разворотов не бывает, так что предел ловит только такой список.
func TestAlbumTplOfIDsLimit(t *testing.T) {
	e := tplEd(t)
	many := make([]string, model.MaxSpreads+1)
	for i := range many {
		many[i] = fmt.Sprintf("нет%03d", i)
	}
	if _, err := e.AlbumTplOf(saveOpts(many...)); !errors.Is(err, grid.ErrLookIDs) {
		t.Errorf("%d id: %v", len(many), err)
	}
}

// TestAlbumTplOfLimit24: 24 разных шага — макет, 25 — ErrLookMany с текстом для тоста; одинаковые
// повторяемые считаются один раз.
func TestAlbumTplOfLimit24(t *testing.T) {
	e := tplEd(t)
	for i := range 25 {
		if _, err := e.NewSpread(len(edtest.P(e).Spreads), nil); err != nil {
			t.Fatal(err)
		}
		sid := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
		e.SetBackground(sid, model.ScopeLeft, fmt.Sprintf("#%02x%02x%02x", 10+i, 20, 30))
	}
	all := ids(e)[4:] // 25 новых, у каждого свой цвет
	if _, err := e.AlbumTplOf(model.SaveLookOpts{Ids: all[:24]}); err != nil {
		t.Errorf("24 разных: %v", err)
	}
	if _, err := e.AlbumTplOf(model.SaveLookOpts{Ids: all}); !errors.Is(err, model.ErrLookMany) || !strings.Contains(err.Error(), "Не больше 24 разных разворотов") {
		t.Errorf("25 разных: %v", err)
	}
	same := slices.Concat(all[:1], all[:1]) // тот же разворот дважды — id повторён
	if _, err := e.AlbumTplOf(model.SaveLookOpts{Ids: same}); !errors.Is(err, grid.ErrLookIDs) {
		t.Errorf("дубль id: %v", err)
	}
}

// TestAlbumTplOfVignette: лист-виньетка — шаг vignette с параметрами листа и без мест; среди
// повторяемых — ErrLookVigRep, на краю диапазона — шаг «один раз».
func TestAlbumTplOfVignette(t *testing.T) {
	e := tplEd(t)
	if _, err := e.AddVignette(1); err != nil {
		t.Fatal(err)
	}
	vid := ids(e)[1]
	o := saveOpts("s1", vid, "s2")
	tpl, err := e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	st := tpl.Steps[1]
	v := edtest.P(e).Spreads[1].Vig
	if st.Role != model.RoleVignette || st.Vig == nil || st.Vig.Tpl != v.Tpl || st.Vig.Pupils != v.Pupils || len(st.Tpl.Places) != 0 {
		t.Errorf("шаг виньетки: %+v", st)
	}
	o.RepFrom, o.RepTo = 1, 3
	if _, err := e.AlbumTplOf(o); !errors.Is(err, model.ErrLookVigRep) {
		t.Errorf("виньетка среди повторяемых: %v", err)
	}
	o.RepFrom, o.RepTo = 2, 3 // виньетка на краю: сама «один раз», повторяемый — s2
	if tpl, err = e.AlbumTplOf(o); err != nil || len(tpl.Steps) != 3 || tpl.Steps[1].Role != model.RoleVignette || tpl.Steps[2].Role != model.RoleRepeat {
		t.Errorf("виньетка на краю: %v, %+v", err, tpl.Steps)
	}
	// Одна виньетка — не макет: нужен хотя бы один шаг-не-виньетка.
	if _, err := e.AlbumTplOf(saveOpts(vid)); !errors.Is(err, grid.ErrLookNothing) {
		t.Errorf("одна виньетка: %v", err)
	}
}

// TestAlbumTplOfCover: стороны обложки — страницы на безопасную зону (доли зоны), цвет в странице;
// корешок и фото не входят; Cover = false или обложки нет — без обложки.
func TestAlbumTplOfCover(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeRight, "h1", "v2")
	c.e.SetBackground(model.CoverID, model.ScopeLeft, "#112233")
	c.e.SetBackground(model.CoverID, model.ScopeRight, "#445566")
	mustPattern(t, c.e, model.CoverID, model.ScopeSpread, "dots") // узор на весь лист — в Bg
	c.e.SetBackground(ids(c.e)[0], model.ScopeLeft, "#abcdef")    // шаг не пустой и без мест
	before, rev := snapshot(c.e), edtest.Rev(c.e)
	o := saveOpts(ids(c.e)[0])
	o.Cover = true
	tpl, err := c.e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	cv := tpl.Cover
	if cv == nil || cv.Back == nil || cv.Front == nil {
		t.Fatalf("обложка макета: %+v", cv)
	}
	if cv.Back.Pages != 1 || cv.Back.Page[0].Color != "#112233" || cv.Front.Page[0].Color != "#445566" || len(cv.Front.Places) != 2 || len(cv.Back.Places) != 0 {
		t.Errorf("стороны: задняя %+v, лицевая %+v", cv.Back, cv.Front)
	}
	for _, b := range cv.Front.Places {
		if b.X < 0 || b.Y < 0 || b.X+b.W > 1.000001 || b.Y+b.H > 1.000001 {
			t.Errorf("место вне зоны: %+v", b)
		}
	}
	if cv.Bg == nil || cv.Bg.Pat == nil || cv.Bg.Color != "" {
		t.Errorf("фон листа обложки: %+v", cv.Bg)
	}
	if len(cv.Front.Drafts) != 1 { // надпись «Обложка» — заготовкой
		t.Errorf("заготовок лицевой %d", len(cv.Front.Drafts))
	}
	o.Cover = false
	if tpl, _ = c.e.AlbumTplOf(o); tpl.Cover != nil {
		t.Error("Cover = false, а обложка в макете")
	}
	o.Cover, o.Places, o.Drafts = true, false, false
	if tpl, err = c.e.AlbumTplOf(o); err != nil || tpl.Cover == nil || len(tpl.Cover.Front.Places) != 0 {
		t.Errorf("обложка без мест: %v %+v", err, tpl.Cover)
	}
	if !slices.Equal(snapshot(c.e), before) || edtest.Rev(c.e) != rev {
		t.Error("проект изменился при снятии обложки")
	}
	// Альбом без обложки: флажок есть, обложки в макете нет.
	if tpl, _ = tplEd(t).AlbumTplOf(model.SaveLookOpts{Ids: []string{"s1"}, Cover: true}); tpl.Cover != nil {
		t.Error("обложки в проекте нет, а в макете есть")
	}
}

// TestAlbumTplOfCoverEmpty: обложка без цвета, мест и надписей, а также без зоны сторонки — в макет
// не входит (пустая обложка не пишется).
func TestAlbumTplOfCoverEmpty(t *testing.T) {
	c := fsfera(t)
	c.clear(model.ScopeLeft)
	c.clear(model.ScopeRight)
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Sheet.Texts = nil })
	c.e.SetBackground(ids(c.e)[0], model.ScopeLeft, "#abcdef")
	o := model.SaveLookOpts{Ids: ids(c.e)[:1], Places: true, Drafts: true, Cover: true}
	tpl, err := c.e.AlbumTplOf(o)
	if err != nil || tpl.Cover != nil {
		t.Errorf("пустая обложка: %v, %+v", err, tpl.Cover)
	}
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 5000, 5000 }) // зоны нет
	if tpl, err = c.e.AlbumTplOf(o); err != nil || tpl.Cover != nil {
		t.Errorf("обложка без зоны: %v, %+v", err, tpl.Cover)
	}
}

// TestAlbumTplOfAutoSameAsSnapshot: «Авто»-разворот в макете (закреплён на общей копии) даёт тот же
// шаблон, что отдельный снимок разворота (копия на каждый).
func TestAlbumTplOfAutoSameAsSnapshot(t *testing.T) {
	e := tplEd(t)
	tpl, err := e.AlbumTplOf(saveOpts("s2"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := grid.SpreadTplOfAs(edtest.P(e), "s2", model.ScopeSpread, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := tpl.Steps[0].Tpl; model.TplKey(got) != model.TplKey(want) || len(got.Places) != 2 {
		t.Errorf("макет %+v, снимок %+v", got, want)
	}
}

// TestAlbumTplOfSkipsPrivate: skip(ключ) убирает свой стикер из шага и из шаблона разворота, остальные остаются.
func TestAlbumTplOfSkipsPrivate(t *testing.T) {
	e := tplEd(t)
	skip := func(lib string) bool { return lib == kSt }
	tpl, err := grid.AlbumTplOf(edtest.P(e), saveOpts("s1"), skip)
	if err != nil || len(tpl.Steps[0].Tpl.Stickers) != 2 {
		t.Fatalf("макет: %v, стикеров %d", err, len(tpl.Steps[0].Tpl.Stickers))
	}
	for _, it := range tpl.Steps[0].Tpl.Stickers {
		if it.Lib == kSt {
			t.Error("пропущенный стикер в макете")
		}
	}
	one, err := grid.SpreadTplOfSkip(edtest.P(e), "s1", model.ScopeSpread, true, true, skip)
	if err != nil || len(one.Stickers) != 2 {
		t.Fatalf("шаблон разворота: %v, стикеров %d", err, len(one.Stickers))
	}
	if all, _ := grid.SpreadTplOfSkip(edtest.P(e), "s1", model.ScopeSpread, true, true, nil); len(all.Stickers) != 3 {
		t.Errorf("без skip стикеров %d", len(all.Stickers))
	}
}

// TestAlbumTplOfCoverPlaceOutsideZone: места лицевой, выступающие за безопасную зону (на заворот),
// ложатся в макет целиком внутри зоны (holdInZone): прямое обрезается по ней, повёрнутое сдвигается внутрь.
func TestAlbumTplOfCoverPlaceOutsideZone(t *testing.T) {
	c := fsfera(t)
	c.fill(model.ScopeRight, "h1", "v2")
	var z layout.Rect
	edtest.Change(c.e, func(p *model.Project) {
		for _, sd := range p.CoverSides() {
			if sd.Scope == model.ScopeRight {
				z = sd.Box
			}
		}
		k := 0
		for _, pl := range p.Cover.Sheet.Grid {
			if !grid.InArea(p, p.Cover.Sheet, pl.X, pl.W, model.ScopeRight) {
				continue
			}
			if k == 0 { // прямое: правый край на 30 % зоны за ней
				pl.X, pl.W, pl.Y, pl.H = z.X+0.5*z.W, 0.8*z.W, z.Y+0.1*z.H, 0.3*z.H
			} else { // повёрнутое: угол выступает за правый край
				pl.X, pl.W, pl.Y, pl.H, pl.Rot = z.X+0.75*z.W, 0.4*z.W, z.Y+0.5*z.H, 0.3*z.H, 40
			}
			k++
		}
	})
	c.e.SetBackground(ids(c.e)[0], model.ScopeLeft, "#abcdef")
	o := saveOpts(ids(c.e)[0])
	o.Cover = true
	tpl, err := c.e.AlbumTplOf(o)
	if err != nil || tpl.Cover == nil || tpl.Cover.Front == nil || len(tpl.Cover.Front.Places) != 2 {
		t.Fatalf("макет: %v %+v", err, tpl.Cover)
	}
	places := tpl.Cover.Front.Places
	if b := places[0]; math.Abs(b.X-0.5) > 0.001 || math.Abs(b.W-0.5) > 0.001 {
		t.Errorf("прямое место не обрезано по зоне: %+v", b)
	}
	b := places[1]
	r := geom.RotRect{X: b.X * z.W, Y: b.Y * z.H, W: b.W * z.W, H: b.H * z.H, Rot: b.Rot}
	if bb := r.AABB(); bb.X < -0.01 || bb.Y < -0.01 || bb.X+bb.W > z.W+0.01 || bb.Y+bb.H > z.H+0.01 {
		t.Errorf("повёрнутое место вне зоны: %+v (AABB %+v)", b, bb)
	}
}

// TestEditorPrivateLib: LookHost.Private убирает стикер из шаблона разворота и из макета у Editor;
// без хозяина берутся все.
func TestEditorPrivateLib(t *testing.T) {
	e := tplEd(t)
	one, err := e.SpreadTplOf("s1", model.ScopeSpread, true, true)
	if err != nil || len(one.Stickers) != 3 {
		t.Fatalf("без хозяина: %v, стикеров %d", err, len(one.Stickers))
	}
	e.SetLookHost(album.LookHost{Private: func(lib string) bool { return lib == kSt }})
	if one, err = e.SpreadTplOf("s1", model.ScopeSpread, true, true); err != nil || len(one.Stickers) != 2 {
		t.Errorf("шаблон разворота: %v, стикеров %d", err, len(one.Stickers))
	}
	tpl, err := e.AlbumTplOf(saveOpts("s1"))
	if err != nil || len(tpl.Steps[0].Tpl.Stickers) != 2 {
		t.Errorf("макет: %v", err)
	}
	e.SetLookHost(album.LookHost{}) // хозяин есть, Private нет
	if one, err = e.SpreadTplOf("s1", model.ScopeSpread, true, true); err != nil || len(one.Stickers) != 3 {
		t.Errorf("Private nil: %v, стикеров %d", err, len(one.Stickers))
	}
}
