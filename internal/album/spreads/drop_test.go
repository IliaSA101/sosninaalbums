package spreads_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Бросок фото на ручной разворот (042, design §2, §6).

// dropRef — фото w×h в проекте.
func dropRef(p *model.Project, id string, w, h int) {
	p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: w, H: h}
}

// rowPlaces — n мест 50×33 рядами по 10 (шаг 55×45 от угла 5, 5), фото f0… у первых full.
func rowPlaces(n, full int) []*model.Place {
	out := make([]*model.Place, n)
	for i := range out {
		out[i] = edtest.Gp(5+float64(i%10)*55, 5+float64(i/10)*45, 50, 33, "")
		if i < full {
			out[i].Photo = fmt.Sprintf("f%d", i)
		}
	}
	return out
}

// dropOpen — лист 0 проекта p — сетка places (фото 3:2), ID мест 000…001 по порядку.
func dropOpen(t *testing.T, p *model.Project, places []*model.Place) *album.Editor {
	t.Helper()
	s := p.Spreads[0]
	s.Grid, s.Photos = places, []string{}
	for i, pl := range places {
		pl.ID = fmt.Sprintf("%012x", i+1)
		if pl.Photo != "" {
			dropRef(p, pl.Photo, 300, 200)
			s.Photos = append(s.Photos, pl.Photo)
		}
	}
	e := edtest.Open(t, p, "")
	edtest.MustGrid(t, e)
	return e
}

// dropEd — разворот-сетка s1 (страницы 281×284) с местами places и «Авто»-разворот s2 (x1, x2).
func dropEd(t *testing.T, places []*model.Place) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	p.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2", Photos: []string{"x1", "x2"}}}
	dropRef(p, "x1", 300, 200)
	dropRef(p, "x2", 300, 200)
	return dropOpen(t, p, places)
}

// portrait — фото ленты 2:3.
func portrait(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: 200, H: 300}
}

// drop — бросок без ошибки: Ask — проект и rev прежние, без шага; иначе ровно один шаг отмены,
// Undo возвращает проект байт в байт, Redo — итог байт в байт.
func drop(t *testing.T, e *album.Editor, sid, place string, ph []model.PhotoInfo, at float64, rest string) (model.View, spreads.DropOut) {
	t.Helper()
	before, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
	v, out, err := e.DropPhotos(sid, place, ph, at, rest)
	if err != nil {
		t.Fatalf("бросок: %v", err)
	}
	edtest.MustGrid(t, e)
	after := model.Marshal(edtest.P(e))
	if out.Ask || bytes.Equal(before, after) {
		edtest.Same(t, e, rev, undo, "вопрос или холостой бросок")
		if !bytes.Equal(before, after) {
			t.Fatal("вопрос изменил проект")
		}
		return v, out
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("Undo не вернул проект байт в байт")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("Redo не вернул итог")
	}
	return v, out
}

func wantPhotos(t *testing.T, e *album.Editor, i int, want ...string) {
	t.Helper()
	if got := edtest.PhotosOf(e, i); !slices.Equal(got, want) {
		t.Fatalf("места листа %d: %q, ждали %q", i, got, want)
	}
}

func TestDropFill(t *testing.T) {
	e := dropEd(t, rowPlaces(4, 1))
	_, out := drop(t, e, "s1", "", edtest.Photos("n1", "n2"), -1, "")
	wantPhotos(t, e, 0, "f0", "n1", "n2", "")
	if out.Ask || out.Fit != 2 || out.Total != 2 || len(out.Fresh) != 0 || out.Left != 0 {
		t.Fatalf("итог %+v", out)
	}
	// Фото другого разворота переезжают в места (как AddPhotos), остаток мест — ему же.
	_, out = drop(t, e, "s1", "", edtest.Photos("x2"), 0.9, "")
	wantPhotos(t, e, 0, "f0", "n1", "n2", "x2")
	if out.Fit != 1 || !slices.Equal(edtest.P(e).Spreads[1].Photos, []string{"x1"}) {
		t.Fatalf("x2 не переехал: %+v, %v", out, edtest.P(e).Spreads[1].Photos)
	}
}

// freshPlaces — новые места листа 0 по ID из out.Fresh.
func freshPlaces(t *testing.T, e *album.Editor, out spreads.DropOut) []*model.Place {
	t.Helper()
	s := edtest.P(e).Spreads[0]
	var got []*model.Place
	for _, id := range out.Fresh {
		_, pl := s.FindPlace(id)
		if pl == nil {
			t.Fatalf("нового места %s нет в сетке", id)
		}
		got = append(got, pl)
	}
	return got
}

func TestDropTileWhenFull(t *testing.T) {
	e := dropEd(t, rowPlaces(2, 2))
	ph := []model.PhotoInfo{edtest.Photo("n1"), portrait("n2"), edtest.Photo("n3")}
	v, out := drop(t, e, "s1", "", ph, 0.75, "")
	if out.Ask || out.Fit != 0 || out.Total != 3 || len(out.Fresh) != 3 || out.Small {
		t.Fatalf("итог %+v", out)
	}
	wantPhotos(t, e, 0, "f0", "f1", "n1", "n2", "n3")
	fresh := freshPlaces(t, e, out)
	limit := 281.0 / 3
	for i, pl := range fresh {
		if pl.X < 281+12-0.01 || pl.X+pl.W > 562-12+0.01 || pl.Y < 12-0.01 || pl.Y+pl.H > 284-12+0.01 {
			t.Fatalf("место %d не на правой странице без полей: %+v", i, *pl)
		}
		a := float64(edtest.P(e).Photos[pl.Photo].W) / float64(edtest.P(e).Photos[pl.Photo].H)
		if math.Abs(pl.W/pl.H-a) > 0.01 || max(pl.W, pl.H) > limit+0.01 || pl.Photo != ph[i].ID {
			t.Fatalf("место %d: %+v, пропорции фото %.3f, предел %.2f", i, *pl, a, limit)
		}
		for _, o := range fresh[:i] {
			if a, b := model.PlaceAABB(pl), model.PlaceAABB(o); a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Fatalf("новые места налезают: %+v %+v", *pl, *o)
			}
		}
	}
	// Новые места — сверху (в конце стека) и в виде.
	if stack := v.Spreads[0].Places; stack[len(stack)-1].ID != out.Fresh[2] {
		t.Fatalf("последнее место вида %s, ждали %s", stack[len(stack)-1].ID, out.Fresh[2])
	}
	// Save → Load: новые места — обычные Place, круг записи их не меняет.
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	// ID фото Load строит заново (Rekey): места сверяются по геометрии, ID места и файлу фото.
	seat := func(p *model.Project) []string {
		var out []string
		for _, pl := range p.Spreads[0].Grid {
			out = append(out, fmt.Sprintf("%s %v %v %v %v %s", pl.ID, pl.X, pl.Y, pl.W, pl.H, p.Photos[pl.Photo].Path))
		}
		return out
	}
	if x, y := seat(q), seat(edtest.P(e)); !slices.Equal(x, y) {
		t.Fatalf("Save → Load изменил места после плитки:\n%v\n%v", x, y)
	}
}

func TestDropPagePick(t *testing.T) {
	page := func(t *testing.T, e *album.Editor, out spreads.DropOut) int {
		t.Helper()
		pl := freshPlaces(t, e, out)[0]
		return edtest.P(e).Geo(edtest.P(e).Spreads[0]).PageOf(pl.X + pl.W/2)
	}
	for _, c := range []struct {
		name  string
		at    float64
		heavy int // страница, где места крупнее (−1 — поровну)
		want  int
	}{
		{"курсор слева", 0.2, 0, 0},
		{"курсор справа", 0.8, 0, 1},
		{"без точки, левая занята больше", -1, 0, 1},
		{"без точки, правая занята больше", -1, 1, 0},
		{"без точки, поровну", -1, -1, 0},
		{"NaN — как без точки", math.NaN(), 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			big := edtest.Gp(20, 20, 200, 150, "f0")
			small := edtest.Gp(300, 20, 60, 40, "f1")
			if c.heavy == 1 {
				big.X, small.X = 300, 20
			}
			if c.heavy < 0 {
				small.W, small.H = 200, 150
			}
			e := dropEd(t, []*model.Place{big, small})
			_, out := drop(t, e, "s1", "", edtest.Photos("n1"), c.at, "")
			if got := page(t, e, out); got != c.want {
				t.Fatalf("страница %d, ждали %d", got, c.want)
			}
		})
	}
	t.Run("бросок на место — его страница", func(t *testing.T) {
		e := dropEd(t, []*model.Place{edtest.Gp(300, 20, 60, 40, "f0"), edtest.Gp(20, 20, 200, 150, "f1")})
		_, out := drop(t, e, "s1", "000000000001", edtest.Photos("n1", "n2"), -1, spreads.RestAdd)
		if !out.Out || page(t, e, out) != 1 {
			t.Fatalf("итог %+v, страница %d", out, page(t, e, out))
		}
	})
	t.Run("лист 031 — одна страница", func(t *testing.T) {
		e := dropOpen(t, model.NewSheet(200, 300), []*model.Place{edtest.Gp(10, 10, 60, 40, "f0")})
		sid := edtest.P(e).Spreads[0].ID
		_, out := drop(t, e, sid, "", edtest.Photos("n1", "n2"), 0.9, "")
		for _, pl := range freshPlaces(t, e, out) {
			if pl.X < 8-0.01 || pl.X+pl.W > 192+0.01 {
				t.Fatalf("место вне листа без полей: %+v", *pl)
			}
		}
	})
}

func TestDropAsk(t *testing.T) {
	e := dropEd(t, rowPlaces(4, 1))
	before := model.Marshal(edtest.P(e))
	ph := edtest.Photos("n1", "n2", "n3", "n4", "n5", "n6", "n7")
	for _, rest := range []string{"", "что-то"} {
		_, out := drop(t, e, "s1", "", ph, -1, rest)
		if !out.Ask || out.Fit != 3 || out.Total != 7 || len(out.Fresh) != 0 {
			t.Fatalf("rest %q: итог %+v", rest, out)
		}
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != 0 {
		t.Fatal("вопрос оставил след")
	}
}

func TestDropRestAdd(t *testing.T) {
	e := dropEd(t, rowPlaces(4, 1))
	ph := edtest.Photos("n1", "n2", "n3", "n4", "n5", "n6", "x1")
	_, out := drop(t, e, "s1", "", ph, 0.75, spreads.RestAdd)
	if out.Ask || out.Fit != 3 || out.Total != 7 || len(out.Fresh) != 4 || out.Left != 0 {
		t.Fatalf("итог %+v", out)
	}
	wantPhotos(t, e, 0, "f0", "n1", "n2", "n3", "n4", "n5", "n6", "x1")
	if s2 := edtest.P(e).Spreads[1].Photos; !slices.Equal(s2, []string{"x2"}) {
		t.Fatalf("x1 остался на s2: %v", s2)
	}
}

func TestDropRestSkip(t *testing.T) {
	e := dropEd(t, rowPlaces(4, 1))
	ph := edtest.Photos("n1", "x1", "n3", "n4", "x2", "n6", "n7")
	_, out := drop(t, e, "s1", "", ph, -1, spreads.RestSkip)
	if out.Ask || out.Fit != 3 || out.Left != 4 || len(out.Fresh) != 0 {
		t.Fatalf("итог %+v", out)
	}
	wantPhotos(t, e, 0, "f0", "n1", "x1", "n3")
	p := edtest.P(e)
	if !slices.Equal(p.Spreads[1].Photos, []string{"x2"}) {
		t.Fatalf("фото другого разворота: %v", p.Spreads[1].Photos)
	}
	for _, id := range []string{"n4", "n6", "n7"} {
		if _, ok := p.Photos[id]; ok {
			t.Fatalf("фото ленты %s осталось в проекте", id)
		}
	}
	// «Не добавлять», когда мест нет вовсе, — ничего (холостая).
	full := dropEd(t, rowPlaces(2, 2))
	if _, out := drop(t, full, "s1", "", edtest.Photos("n1"), -1, spreads.RestSkip); out.Left != 1 {
		t.Fatalf("без мест: %+v", out)
	}
}

func TestDropPlaceSwapReplace(t *testing.T) {
	// Из ленты на занятое — замена, прежнее фото — в ленту; остальные — в следующие пустые.
	e := dropEd(t, rowPlaces(4, 2))
	_, out := drop(t, e, "s1", "000000000002", edtest.Photos("n1", "n2", "n3"), -1, "")
	if !out.Out || out.Ask || out.Fit != 3 || out.Total != 3 {
		t.Fatalf("замена: %+v", out)
	}
	wantPhotos(t, e, 0, "f0", "n1", "n2", "n3")
	// Из альбома на занятое — обмен.
	_, out = drop(t, e, "s1", "000000000001", edtest.Photos("x1"), -1, "")
	if out.Out || out.Fit != 1 {
		t.Fatalf("обмен: %+v", out)
	}
	wantPhotos(t, e, 0, "x1", "n1", "n2", "n3")
	if s2 := edtest.P(e).Spreads[1].Photos; !slices.Contains(s2, "f0") {
		t.Fatalf("f0 не ушёл на место x1: %v", s2)
	}
	// Фото этой сетки на пустое место — перенос; его место освобождается для следующего.
	e = dropEd(t, rowPlaces(3, 2))
	_, out = drop(t, e, "s1", "000000000003", edtest.Photos("f0", "n1"), -1, "")
	if out.Ask || out.Fit != 2 || out.Total != 2 {
		t.Fatalf("перенос: %+v", out)
	}
	wantPhotos(t, e, 0, "n1", "f1", "f0")
	// Пачка с хозяином места (как PlacePhotos): замена, хозяин — в следующее пустое, не в ленту,
	// фото после него — в следующее.
	e = dropEd(t, rowPlaces(4, 2))
	_, out = drop(t, e, "s1", "000000000001", edtest.Photos("n1", "f0", "n2"), -1, "")
	if out.Out || out.Ask || out.Fit != 3 || out.Total != 3 {
		t.Fatalf("пачка с хозяином: %+v", out)
	}
	wantPhotos(t, e, 0, "n1", "f1", "f0", "n2")
	// Обмен с фото другого листа: хозяин ушёл на тот лист и садится в пустое этого.
	e = dropEd(t, rowPlaces(3, 2))
	_, out = drop(t, e, "s1", "000000000001", edtest.Photos("x1", "f0"), -1, "")
	if out.Out || out.Ask || out.Fit != 2 || out.Total != 2 {
		t.Fatalf("обмен с хозяином: %+v", out)
	}
	wantPhotos(t, e, 0, "x1", "f1", "f0")
	if s2 := edtest.P(e).Spreads[1].Photos; !slices.Equal(s2, []string{"x2"}) {
		t.Fatalf("другой лист после обмена: %v", s2)
	}
	// Мест для хозяина нет — вопрос; «Не добавлять» — хозяин в ленте, он и есть не вставшее.
	e = dropEd(t, rowPlaces(2, 2))
	if _, out = drop(t, e, "s1", "000000000001", edtest.Photos("n1", "f0"), -1, ""); !out.Ask || out.Fit != 1 || out.Total != 2 {
		t.Fatalf("хозяину нет места: %+v", out)
	}
	_, out = drop(t, e, "s1", "000000000001", edtest.Photos("n1", "f0"), -1, spreads.RestSkip)
	if out.Out || out.Left != 1 {
		t.Fatalf("хозяин не встал: %+v", out)
	}
	wantPhotos(t, e, 0, "n1", "f1")
}

func TestDropPlaceAsk(t *testing.T) {
	e := dropEd(t, rowPlaces(3, 3))
	ph := edtest.Photos("n1", "n2", "n3")
	_, out := drop(t, e, "s1", "000000000002", ph, -1, "")
	if !out.Ask || out.Fit != 1 || out.Total != 3 {
		t.Fatalf("вопрос: %+v", out)
	}
	_, out = drop(t, e, "s1", "000000000002", ph, -1, spreads.RestAdd)
	if !out.Out || len(out.Fresh) != 2 {
		t.Fatalf("плитка: %+v", out)
	}
	wantPhotos(t, e, 0, "f0", "n1", "f2", "n2", "n3")
}

func TestDropAlreadyInGrid(t *testing.T) {
	e := dropEd(t, rowPlaces(4, 2))
	for _, c := range []struct{ place, what string }{{"", "на лист"}, {"000000000001", "на своё место"}} {
		_, out := drop(t, e, "s1", c.place, edtest.Photos("f0", "f1"), -1, "")
		if out.Ask || len(out.Fresh) != 0 {
			t.Fatalf("%s: %+v", c.what, out)
		}
		wantPhotos(t, e, 0, "f0", "f1", "", "")
	}
	// Неизвестный лист, место, пустой список — холостая.
	_, _ = drop(t, e, "нет", "", edtest.Photos("n1"), -1, "")
	_, _ = drop(t, e, "s1", "нет места", edtest.Photos("n1"), -1, "")
	_, _ = drop(t, e, "s1", "", nil, -1, "")
	if edtest.UndoLen(e) != 0 {
		t.Fatal("холостой бросок оставил шаг")
	}
}

func TestDropAutoSpread(t *testing.T) {
	a := dropEd(t, rowPlaces(2, 2))
	b := dropEd(t, rowPlaces(2, 2))
	ph := edtest.Photos("n1", "n2", "f1")
	_, out := drop(t, a, "s2", "", ph, 0.2, "")
	edtest.AddPhotos(b, "s2", ph, -1)
	if out.Fit != 3 || out.Total != 3 || out.Ask {
		t.Fatalf("итог %+v", out)
	}
	if x, y := edtest.P(a).Spreads[1].Photos, edtest.P(b).Spreads[1].Photos; !slices.Equal(x, y) {
		t.Fatalf("«Авто»: %v, AddPhotos: %v", x, y)
	}
	// Предел фото «Авто» — прежняя ошибка, проект прежний.
	many := make([]string, model.MaxPhotos)
	for i := range many {
		many[i] = fmt.Sprintf("m%02d", i)
	}
	before := model.Marshal(edtest.P(a))
	if _, _, err := a.DropPhotos("s2", "", edtest.Photos(many...), -1, ""); err == nil || !bytes.Equal(model.Marshal(edtest.P(a)), before) {
		t.Fatalf("предел «Авто»: %v", err)
	}
}

func TestDropCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", rowPlaces(2, 2))
	_, out := drop(t, e, model.CoverID, edtest.CoverPlace, edtest.Photos("n1", "n2", "n3"), -1, "")
	if out.Ask || !out.Out || out.Left != 2 || len(out.Fresh) != 0 {
		t.Fatalf("обложка: %+v", out)
	}
	if got := edtest.P(e).Cover.Sheet.Photos; !slices.Equal(got, []string{"n1"}) {
		t.Fatalf("обложка: %v", got)
	}
}

func TestDropTooManyPlaces(t *testing.T) {
	e := dropEd(t, rowPlaces(60, 60))
	before := model.Marshal(edtest.P(e))
	_, _, err := e.DropPhotos("s1", "", edtest.Photos("n1", "n2", "n3", "n4", "n5"), -1, "")
	if err == nil || !strings.Contains(err.Error(), "64 мест") || !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != 0 {
		t.Fatalf("ждали отказ про 64 места без правки: %v", err)
	}
	// Ровно 64 — можно.
	_, out := drop(t, e, "s1", "", edtest.Photos("n1", "n2", "n3", "n4"), -1, "")
	if len(out.Fresh) != 4 || len(edtest.P(e).Spreads[0].Grid) != model.MaxPlaces {
		t.Fatalf("до предела: %+v", out)
	}
}

func TestDropTileTooSmall(t *testing.T) {
	p := model.New()
	p.Settings = model.ClampSettings(model.Settings{PageW: 50, PageH: 50, Margin: 3, Gap: 2, DPI: 300})
	p.Spreads = []*model.Spread{{ID: "s1"}}
	e := dropOpen(t, p, []*model.Place{edtest.Gp(5, 5, 20, 15, "")})
	if g := edtest.P(e).Settings; g.PageW != 50 {
		t.Fatalf("страница %v", g.PageW)
	}
	ids := make([]string, 64)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%02d", i)
	}
	before := model.Marshal(edtest.P(e))
	_, _, err := e.DropPhotos("s1", "", edtest.Photos(ids...), -1, spreads.RestAdd)
	if !errors.Is(err, spreads.ErrTileSmall) || !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != 0 {
		t.Fatalf("ждали ErrTileSmall без правки: %v", err)
	}
}

// Поля и промежуток по 1/4 страницы (максимум зажима): 64 места не помещаются ни в одну
// сетку — Tile отдаёт пусто, а не мелкие места; проект и шаг отмены те же.
func TestDropTileNoRoom(t *testing.T) {
	p := model.New()
	p.Settings = model.ClampSettings(model.Settings{PageW: 50, PageH: 50, Margin: 100, Gap: 100, DPI: 300})
	if p.Settings.Margin != 12.5 || p.Settings.Gap != 12.5 {
		t.Fatalf("зажим: поля %v, промежуток %v", p.Settings.Margin, p.Settings.Gap)
	}
	p.Spreads = []*model.Spread{{ID: "s1"}}
	e := dropOpen(t, p, []*model.Place{edtest.Gp(5, 5, 20, 15, "")})
	ids := make([]string, 62)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%02d", i)
	}
	before, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
	_, _, err := e.DropPhotos("s1", "", edtest.Photos(ids...), -1, spreads.RestAdd)
	if !errors.Is(err, spreads.ErrTileSmall) || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("ждали ErrTileSmall без правки: %v", err)
	}
	edtest.Same(t, e, rev, undo, "отказ плитки")
	// Одно фото влезает в то же поле: край «ровно одно место» не отказ.
	if _, out := drop(t, e, "s1", "", edtest.Photos("n1", "n2"), -1, spreads.RestAdd); len(out.Fresh) != 1 {
		t.Fatalf("пара фото при тех же полях: %+v", out)
	}
}

// Обложка без мест: «Авто» у неё не бывает, сажать некуда — холостая, без вопроса и шага.
func TestDropCoverNoPlaces(t *testing.T) {
	e := edtest.CoverEd(t, "cv", rowPlaces(2, 2))
	edtest.Change(e, func(p *model.Project) { p.Cover.Sheet.Grid, p.Cover.Sheet.Photos = nil, nil })
	_, out := drop(t, e, model.CoverID, "", edtest.Photos("n1", "n2"), -1, spreads.RestAdd)
	if !reflect.DeepEqual(out, spreads.DropOut{}) {
		t.Fatalf("обложка без мест: %+v", out)
	}
}

func TestDropSmallFlag(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}}
	e := dropOpen(t, p, []*model.Place{edtest.Gp(5, 5, 50, 33, "f0")})
	_, out := drop(t, e, "s1", "", edtest.Photos("n1", "n2"), 0.2, "")
	if out.Small {
		t.Fatalf("два фото на 300×300 — не мелко: %+v", out)
	}
	// 61 фото 3:2 на 300×300 (поля 12) — места около 37×25: мельче 1/10 страницы (30 мм).
	ids := make([]string, 61)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%02d", i)
	}
	_, out = drop(t, e, "s1", "", edtest.Photos(ids...), 0.8, "")
	short := 300.0
	for _, pl := range freshPlaces(t, e, out) {
		short = min(short, pl.W, pl.H)
	}
	if !out.Small || short >= 30 || short < geom.MinPlace {
		t.Fatalf("61 фото на 300×300: Small %v, короткая сторона %.2f", out.Small, short)
	}
}
