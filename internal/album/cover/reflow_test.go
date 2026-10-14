package cover_test

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Пересчёт содержимого обложки при смене геометрии (design 032 §3): кусочно-линейно по зонам,
// в том же шаге отмены, что и вызвавшая правка.

// near — равенство до сотой мм (шаг зажима).
func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

// frontPlace — место обложки на лицевой: x от левого края лицевой, y от верха сторонки.
func frontPlace(t *testing.T, e *album.Editor, dx, dy, w, h float64) string {
	t.Helper()
	if _, err := e.AddPlace(model.CoverID, 1); err != nil {
		t.Fatal(err)
	}
	pl := sheet(e).Grid[len(sheet(e).Grid)-1]
	cg := edtest.P(e).CoverGeo()
	e.SetPlaces(model.CoverID, []model.PlaceBox{{ID: pl.ID, X: cg.X[5] + dx, Y: cg.Y[1] + dy, W: w, H: h}}, "")
	return pl.ID
}

func placeOf(e *album.Editor, id string) model.Place {
	_, pl := sheet(e).FindPlace(id)
	return *pl
}

// reflowAlbum — 22 разворота (корешок 14 мм), «лицевая + задняя» с фото на обеих, надпись
// корешка и место на лицевой (его ID).
func reflowAlbum(t *testing.T) (*album.Editor, string) {
	t.Helper()
	e := coverAlbum(t, 22)
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	edtest.SetBg(t, e, model.CoverID, model.ScopeLeft, pid(1))
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	id := frontPlace(t, e, 40, 30, 120, 90)
	if sp := edtest.P(e).CoverGeo().Spine; sp != 14 {
		t.Fatalf("корешок при 22 разворотах %v", sp)
	}
	return e, id
}

// TestCoverReflowOnNewSpread — разворот добавили, корешок 14 → 16 мм: лицевая (фон, место)
// сдвинулась на +2, задняя на месте, надпись корешка по центру того же размера.
func TestCoverReflowOnNewSpread(t *testing.T) {
	e, id := reflowAlbum(t)
	front, back, pl := *sheet(e).BgPhotoR, *sheet(e).BgPhotoL, placeOf(e, id)
	tb := *sheet(e).Texts[0]
	edtest.NewSpread(e, -1, []model.PhotoInfo{info("x")})
	if sp := edtest.P(e).CoverGeo().Spine; sp != 16 {
		t.Fatalf("корешок при 23 разворотах %v", sp)
	}
	if f := sheet(e).BgPhotoR; !near(f.X, front.X+2) || f.W != front.W {
		t.Fatalf("фон лицевой %+v, был %+v", *f, front)
	}
	if b := sheet(e).BgPhotoL; *b != back {
		t.Fatalf("фон задней сдвинулся: %+v, был %+v", *b, back)
	}
	if got := placeOf(e, id); !near(got.X, pl.X+2) || got.W != pl.W || got.Y != pl.Y {
		t.Fatalf("место лицевой %+v, было %+v", got, pl)
	}
	nt := sheet(e).Texts[0]
	cx, _ := model.BoxOf(nt).Center()
	if g := edtest.P(e).Geo(sheet(e)); !near(cx, g.W/2) || nt.W != tb.W || nt.Style.Size != tb.Style.Size {
		t.Fatalf("надпись корешка %+v, центр %v, лист %v", *nt, cx, g.W)
	}
	edtest.MustGrid(t, e)
}

// TestCoverReflowUndo — пересчёт обложки — в шаге отмены вызвавшей правки: один Undo
// возвращает и разворот, и обложку байт в байт.
func TestCoverReflowUndo(t *testing.T) {
	e, _ := reflowAlbum(t)
	before, undo := string(model.Marshal(edtest.P(e))), edtest.UndoLen(e)
	edtest.NewSpread(e, -1, []model.PhotoInfo{info("x")})
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, было %d", edtest.UndoLen(e), undo)
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("Undo не вернул обложку одним шагом")
	}
	e.Redo()
	if sp := edtest.P(e).CoverGeo().Spine; sp != 16 {
		t.Fatalf("Redo: корешок %v", sp)
	}
}

// TestCoverReflowPanelScale — сторонка 300 → 280 мм: место на лицевой масштабируется в её
// пределах (ровное — по краям, повёрнутое — по центру), за лист не выходит.
func TestCoverReflowPanelScale(t *testing.T) {
	e := coverAlbum(t, 4)
	sp := edtest.P(e).Cover.Spec
	sp.W = 300
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	flat := frontPlace(t, e, 30, 20, 210, 150)
	turned := frontPlace(t, e, 60, 180, 100, 60)
	e.SetPlaceAngle(model.CoverID, turned, 30)
	old, was, wasT := edtest.P(e).CoverGeo(), placeOf(e, flat), placeOf(e, turned)
	sp.W = 280
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	cg, k := edtest.P(e).CoverGeo(), 280.0/300
	got := placeOf(e, flat)
	if !near(got.X, cg.X[5]+(was.X-old.X[5])*k) || !near(got.W, was.W*k) || got.H != was.H {
		t.Fatalf("ровное место %+v, было %+v", got, was)
	}
	if got.X < cg.X[5] || got.X+got.W > cg.X[6] {
		t.Fatalf("место вышло за лицевую: %+v, лицевая %v…%v", got, cg.X[5], cg.X[6])
	}
	gt := placeOf(e, turned)
	if c := gt.X + gt.W/2; !near(c, cg.X[5]+(wasT.X+wasT.W/2-old.X[5])*k) || !near(gt.W, wasT.W*k) || gt.Rot != 30 {
		t.Fatalf("повёрнутое место %+v, было %+v", gt, wasT)
	}
	edtest.MustGrid(t, e)
}

// TestCoverReflowEdges — предметы на краях зон: левый край в правом завороте (последняя зона),
// предмет у границы нулевого заворота (зона нулевой ширины не растягивается) и повёрнутый фон
// (его угол считается от центра).
func TestCoverReflowEdges(t *testing.T) {
	e := coverAlbum(t, 4)
	cg := edtest.P(e).CoverGeo()
	wrapPl := frontPlace(t, e, cg.X[6]-cg.X[5]+1, 20, 8, 20) // левый край правее лицевой — в завороте
	edtest.NewSpread(e, -1, []model.PhotoInfo{info("x")})
	if got := placeOf(e, wrapPl); got.W <= 0 || got.X < edtest.P(e).CoverGeo().X[6] {
		t.Fatalf("место в завороте %+v, лицевая до %v", got, edtest.P(e).CoverGeo().X[6])
	}

	// Повёрнутый фон лицевой: после роста корешка центр съезжает вместе с лицевой.
	e.SetBackdropBox(model.CoverID, model.ScopeRight, 200, 20, 100, 60, 20, "")
	center := func(b *model.Backdrop) (float64, float64) { return b.TBox().Center() }
	b0 := *sheet(e).BgPhotoR
	cx0, cy0 := center(&b0)
	before := edtest.P(e).CoverGeo()
	edtest.NewSpread(e, -1, edtest.Photos("y", "z"))
	edtest.NewSpread(e, -1, edtest.Photos("u"))
	shift := edtest.P(e).CoverGeo().X[5] - before.X[5]
	b1 := *sheet(e).BgPhotoR
	cx1, cy1 := center(&b1)
	if shift <= 0 || b1.Rot != 20 || !near(b1.W, b0.W) || !near(cx1, cx0+shift) || !near(cy1, cy0) {
		t.Fatalf("повёрнутый фон: центр (%v, %v) → (%v, %v), сдвиг %v, %+v → %+v", cx0, cy0, cx1, cy1, shift, b0, b1)
	}
	edtest.MustGrid(t, e)

	// Нулевой заворот: предмет вплотную к краю листа остаётся целым при пересчёте.
	sp := edtest.P(e).Cover.Spec
	sp.Wrap = 0
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	cg = edtest.P(e).CoverGeo()
	flush := frontPlace(t, e, cg.X[6]-cg.X[5]-30, 10, 30, 30)
	sp.Hinge = 2
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	if got := placeOf(e, flush); got.W <= 0 || got.H <= 0 {
		t.Fatalf("место у края при нулевом завороте %+v", got)
	}
	edtest.MustGrid(t, e)
}

// TestEditorReplayCover — сценарий с обложкой (critic §4): обмен, «Заполнить», автосборка,
// перепривязка, удаление текста и разворотов (Reflow), отмена — инварианты сетки по всем
// листам после каждого шага; автосборка и «Заполнить» не берут фото с места обложки.
func TestEditorReplayCover(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(20, 20, 100, 80, "a"), edtest.Gp(300, 20, 100, 80, "")}, []string{"b", "c"})
	coverPhoto := func() string { return placeOf(e, edtest.CoverPlace).Photo }
	step := func(name string, ok bool) {
		t.Helper()
		if err := edtest.GridInvariants(edtest.P(e)); err != nil || !ok {
			t.Fatalf("%s: %v (фото места обложки %q)", name, err, coverPhoto())
		}
	}
	e.SwapPhotos("k", "b")
	step("SwapPhotos", coverPhoto() == "b" && slices.Contains(edtest.P(e).Spreads[1].Photos, "k"))
	e.SwapPhotos("k", "b")
	step("SwapPhotos:назад", coverPhoto() == "k")
	_, left := e.FillPlaces(edtest.Photos("k", "x"), "")
	step("FillPlaces", left == 0 && coverPhoto() == "k" && slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "x"}))
	_, err := e.AutoBuild(edtest.Photos("k", "y", "z"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].Photos
	step("AutoBuild", err == nil && coverPhoto() == "k" && slices.Equal(last, []string{"y", "z"}))
	moved := edtest.Abs(`перенос\k.jpg`)
	e.Relink(map[string]string{"k": moved}, "")
	// ID фото — от пути (Rekey): место обложки получает новый ID, запись — новый путь.
	k := coverPhoto()
	step("Relink", k != "k" && edtest.P(e).Photos[k].Path == moved && !edtest.HasRef(edtest.P(e), "k"))
	e.DeleteText(sheet(e).Texts[0].ID)
	step("DeleteText", len(sheet(e).Texts) == 0)
	w := edtest.P(e).Geo(sheet(e)).W
	e.DeleteSpread(edtest.P(e).Spreads[1].ID)
	step("DeleteSpread", edtest.P(e).Geo(sheet(e)).W <= w && coverPhoto() == k)
	for range 7 {
		e.Undo()
		step("Undo", true)
	}
	if coverPhoto() != "k" || len(sheet(e).Texts) != 1 || len(edtest.P(e).Spreads) != 2 {
		t.Fatalf("после отмены: место %q, текстов %d, разворотов %d", coverPhoto(), len(sheet(e).Texts), len(edtest.P(e).Spreads))
	}
}
