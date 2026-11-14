package grid_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Оформление страницы при узоре и фото на весь лист (ревью 049 Ф2): узор листа виден на
// каждой странице — едет с неё и у цели уступает странице место (SplitPat, как dropPatterns
// 048); фото листа не режется — не едет, у цели-страницы снимается (как dropPhotos 048).

// copyPage — «Копировать оформление» (decor) или «Копировать раскладку» с текстом и фоном.
type copyPage func(e *album.Editor, from string, to []model.GridTarget) error

func viaDecor(e *album.Editor, from string, to []model.GridTarget) error {
	_, err := e.CopyDecor("s1", from, to, true, false)
	return err
}

func viaGrid(e *album.Editor, from string, to []model.GridTarget) error {
	_, _, err := e.CopyGrid("s1", from, to, true, false)
	return err
}

// pageEd — s1: места на левой странице; s2 — «Авто» с фото d; s3 — пустой.
func pageEd(t *testing.T) *album.Editor {
	t.Helper()
	return edtest.GridEd(t, edtest.Calendar()[:2], []string{"d"}, []string{})
}

// А: у источника узор на весь лист — со страницы он едет узором страницы на несколько целей
// одной правкой; та же страница своего листа уже такая же — холостая; отмена — шаг.
func TestCopyPageSheetPatternFrom(t *testing.T) {
	for name, run := range map[string]copyPage{"decor": viaDecor, "grid": viaGrid} {
		t.Run(name, func(t *testing.T) {
			e := pageEd(t)
			mustPattern(t, e, "s1", model.ScopeSpread, "dots")
			undo := edtest.UndoLen(e)
			to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}, {Spread: "s3", Scope: model.ScopeRight}}
			if err := run(e, model.ScopeLeft, to); err != nil {
				t.Fatal(err)
			}
			edtest.MustGrid(t, e)
			if a, b := patKinds(e, 1), patKinds(e, 2); a != [3]string{"", "dots", ""} || b != [3]string{"", "", "dots"} || edtest.UndoLen(e) != undo+1 {
				t.Fatalf("s2 %v, s3 %v, шагов %d", a, b, edtest.UndoLen(e)-undo)
			}
			ps := edtest.P(e).Spreads
			if ps[1].PatL == ps[0].Pat || ps[2].PatR == ps[0].Pat {
				t.Fatal("узор общий с источником")
			}
			q := edtest.SaveLoad(t, e)
			for i, s := range q.Spreads {
				if got, want := kindsOf(s), patKinds(e, i); got != want {
					t.Fatalf("лист %d после записи и чтения: %v, ждали %v", i, got, want)
				}
			}
			e.Undo()
			if a, b := patKinds(e, 1), patKinds(e, 2); a != [3]string{} || b != [3]string{} {
				t.Fatalf("отмена: %v %v", a, b)
			}
		})
	}
	// Левая своего листа на правую: правая и так в том же узоре — копия оформления холостая.
	e := pageEd(t)
	mustPattern(t, e, "s1", model.ScopeSpread, "dots")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if err := viaDecor(e, model.ScopeLeft, []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}}); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "узор листа на свою же страницу")
}

// Б: у цели узор или фото на весь лист, у страницы источника их нет — страница цели из них
// выходит (узор остаётся другой странице, фото снимается), а не считается «уже такой же».
// Повтор — холостой; фото источника-страницы на месте фото листа цели встаёт.
func TestCopyPageSheetBackdropTo(t *testing.T) {
	for name, run := range map[string]copyPage{"decor": viaDecor, "grid": viaGrid} {
		t.Run(name, func(t *testing.T) { sheetBackdropOut(t, name, run) })
	}
	sheetPhotoReplaced(t)
	for name, run := range map[string]copyPage{"decor": viaDecor, "grid": viaGrid} {
		t.Run("свой лист/"+name, func(t *testing.T) { sheetPhotoOwn(t, name, run) })
	}
}

// sheetBackdropOut — страница цели выходит из узора листа и из фото листа; повтор и отмена.
func sheetBackdropOut(t *testing.T, name string, run copyPage) {
	e := pageEd(t)
	mustPattern(t, e, "s2", model.ScopeSpread, "bands")
	edtest.SetBg(t, e, "s3", model.ScopeSpread, "bg")
	undo := edtest.UndoLen(e)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}, {Spread: "s3", Scope: model.ScopeLeft}}
	if err := run(e, model.ScopeLeft, to); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	s2, s3 := sheetOf(e, "s2"), sheetOf(e, "s3")
	if patKinds(e, 1) != [3]string{"", "", "bands"} || s3.BgPhoto != nil || s3.BgPhotoL != nil || s3.BgPhotoR != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("s2 %v, s3 фото %+v %+v %+v", patKinds(e, 1), s3.BgPhoto, s3.BgPhotoL, s3.BgPhotoR)
	}
	if s2.Pat != nil || s2.PatL != nil {
		t.Fatalf("левая s2 в узоре: %+v %+v", s2.Pat, s2.PatL)
	}
	q := edtest.SaveLoad(t, e)
	if kindsOf(q.Spreads[1]) != [3]string{"", "", "bands"} || q.Spreads[2].BgPhoto != nil {
		t.Fatalf("после записи и чтения: %v %+v", kindsOf(q.Spreads[1]), q.Spreads[2].BgPhoto)
	}
	if name == "decor" { // у «раскладки» повтор — новые ID надписей и мест, как и прежде
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		if err := run(e, model.ScopeLeft, to); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "повтор")
	}
	e.Undo()
	if patKinds(e, 1) != [3]string{"bands", "", ""} || sheetOf(e, "s3").BgPhoto == nil {
		t.Fatalf("отмена: %v", patKinds(e, 1))
	}
}

// sheetPhotoReplaced — фото страницы источника встаёт вместо фото листа цели (фоны страниц и листа взаимоисключающие).
func sheetPhotoReplaced(t *testing.T) {
	e := pageEd(t)
	edtest.SetBg(t, e, "s1", model.ScopeLeft, "bg")
	edtest.SetBg(t, e, "s3", model.ScopeSpread, "d")
	if err := viaDecor(e, model.ScopeLeft, []model.GridTarget{{Spread: "s3", Scope: model.ScopeRight}}); err != nil {
		t.Fatal(err)
	}
	if s3 := sheetOf(e, "s3"); s3.BgPhoto != nil || s3.BgPhotoR == nil || s3.BgPhotoL != nil {
		t.Fatalf("фото s3: %+v %+v %+v", s3.BgPhoto, s3.BgPhotoL, s3.BgPhotoR)
	}
}

// sheetPhotoOwn — фото на весь лист у источника: копия страницы на соседнюю страницу того же
// листа его не снимает (оно и есть фон источника), у цели другого листа фото листа снимается
// (№217 п.1) — обе цели одной правкой; повтор и копия «как есть» — холостые; отмена — шаг.
func sheetPhotoOwn(t *testing.T, name string, run copyPage) {
	e := pageEd(t)
	edtest.SetBg(t, e, "s1", model.ScopeSpread, "bg")
	edtest.SetBg(t, e, "s3", model.ScopeSpread, "d")
	if name == "decor" { // правая и так в фото листа, стикеров и надписей нет — уже такая же
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		if err := run(e, model.ScopeLeft, []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}}); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "фото листа на свою же страницу")
	}
	putStickers(e, "s1", plate(20, 30, 40, 20, 0))
	undo := edtest.UndoLen(e)
	to := []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeLeft}}
	if err := run(e, model.ScopeLeft, to); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	s1, s3 := sheetOf(e, "s1"), sheetOf(e, "s3")
	if s1.BgPhoto == nil || s1.BgPhotoL != nil || s1.BgPhotoR != nil || len(onPage(s1, 1)) != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("s1 фото %+v %+v %+v, стикеров справа %d, шагов %d", s1.BgPhoto, s1.BgPhotoL, s1.BgPhotoR, len(onPage(s1, 1)), edtest.UndoLen(e)-undo)
	}
	if s3.BgPhoto != nil || s3.BgPhotoL != nil || s3.BgPhotoR != nil || len(onPage(s3, 0)) != 1 {
		t.Fatalf("s3 фото %+v %+v %+v", s3.BgPhoto, s3.BgPhotoL, s3.BgPhotoR)
	}
	q := edtest.SaveLoad(t, e)
	if q.Spreads[0].BgPhoto == nil || q.Spreads[2].BgPhoto != nil {
		t.Fatalf("после записи и чтения: %+v %+v", q.Spreads[0].BgPhoto, q.Spreads[2].BgPhoto)
	}
	if name == "decor" {
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		if err := run(e, model.ScopeLeft, to); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "повтор")
	}
	e.Undo()
	if s1, s3 := sheetOf(e, "s1"), sheetOf(e, "s3"); s1.BgPhoto == nil || len(onPage(s1, 1)) != 0 || s3.BgPhoto == nil {
		t.Fatalf("отмена: s1 %+v, стикеров справа %d, s3 %+v", s1.BgPhoto, len(onPage(s1, 1)), s3.BgPhoto)
	}
}

// «Копировать раскладку» с текстом и фоном и «Зеркально»: стикеры на странице другой стороны
// отражаются, как в «Копировать оформление» (x, FlipH, угол −Rot); на той же стороне и без
// флажка — только сдвиг.
func TestCopyGridMirrorStickers(t *testing.T) {
	e := pageEd(t)
	k := plate(20, 30, 40, 20, 30)
	putStickers(e, "s1", k)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeLeft}}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, to, true, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	if m := onPage(sheetOf(e, "s2"), 1); len(m) != 1 || m[0].X != 562-k.X-k.W || !m[0].FlipH || m[0].Rot != -30 || m[0].Y != k.Y {
		t.Fatalf("отражённый: %s", jsonOf(t, m))
	}
	if s := onPage(sheetOf(e, "s3"), 0); len(s) != 1 || s[0].X != k.X || s[0].FlipH || s[0].Rot != 30 {
		t.Fatalf("та же сторона: %s", jsonOf(t, s))
	}
	// Без «Зеркально» — сдвиг на страницу.
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, to[:1], true, false); err != nil {
		t.Fatal(err)
	}
	if m := onPage(sheetOf(e, "s2"), 1); len(m) != 1 || m[0].X != k.X+281 || m[0].FlipH || m[0].Rot != 30 {
		t.Fatalf("без зеркала: %s", jsonOf(t, m))
	}
}

// PlanCopyGrid с текстом и фоном считает заменяемые стикеры цели (по области), как CopyDecor;
// без оформления — ноль; у листа без стикеров — ноль.
func TestPlanCopyGridStickers(t *testing.T) {
	e := pageEd(t)
	putStickers(e, "s1", plate(20, 30, 40, 20, 0))
	putStickers(e, "s2", plate(20, 40, 30, 30, 0), plate(300, 40, 30, 30, 0), plate(320, 90, 30, 30, 0))
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeLeft}}
	plan := e.PlanCopyGrid("s1", model.ScopeLeft, to, true, false)
	if plan.Stickers != 2 {
		t.Fatalf("план %+v", plan)
	}
	if p := e.PlanCopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, true, false); p.Stickers != 3 {
		t.Fatalf("обе страницы: %+v", p)
	}
	if p := e.PlanCopyGrid("s1", model.ScopeLeft, to, false, false); p.Stickers != 0 {
		t.Fatalf("без оформления: %+v", p)
	}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, to, true, false); err != nil || len(onPage(sheetOf(e, "s2"), 1)) != 1 || len(onPage(sheetOf(e, "s2"), 0)) != 1 {
		t.Fatalf("копия по плану: %v", err)
	}
}

// Цель «свой лист целиком» при копии страницы: expandTargets раскрывает её в обе страницы,
// своя пропускается, соседняя получает копию — фото на весь лист источника остаётся (это его
// фон); повтор — холостой (у «раскладки» новые ID, как и прежде); Save→Load; отмена — шаг.
func TestCopyPageOwnSpreadTarget(t *testing.T) {
	for name, run := range map[string]copyPage{"decor": viaDecor, "grid": viaGrid} {
		t.Run(name, func(t *testing.T) {
			e := pageEd(t)
			edtest.SetBg(t, e, "s1", model.ScopeSpread, "bg")
			putStickers(e, "s1", plate(20, 30, 40, 20, 0))
			undo := edtest.UndoLen(e)
			to := []model.GridTarget{{Spread: "s1", Scope: model.ScopeSpread}}
			if err := run(e, model.ScopeLeft, to); err != nil {
				t.Fatal(err)
			}
			edtest.MustGrid(t, e)
			s1 := sheetOf(e, "s1")
			if s1.BgPhoto == nil || s1.BgPhotoL != nil || s1.BgPhotoR != nil || len(onPage(s1, 0)) != 1 || len(onPage(s1, 1)) != 1 || edtest.UndoLen(e) != undo+1 {
				t.Fatalf("фото %+v %+v %+v, стикеров %d+%d, шагов %d", s1.BgPhoto, s1.BgPhotoL, s1.BgPhotoR, len(onPage(s1, 0)), len(onPage(s1, 1)), edtest.UndoLen(e)-undo)
			}
			q := edtest.SaveLoad(t, e)
			if q.Spreads[0].BgPhoto == nil || len(onPage(q.Spreads[0], 1)) != 1 {
				t.Fatalf("после записи и чтения: %+v, справа %d", q.Spreads[0].BgPhoto, len(onPage(q.Spreads[0], 1)))
			}
			if name == "decor" {
				rev, undo := edtest.Rev(e), edtest.UndoLen(e)
				if err := run(e, model.ScopeLeft, to); err != nil {
					t.Fatal(err)
				}
				edtest.Same(t, e, rev, undo, "повтор на свой лист")
			}
			e.Undo()
			if s1 := sheetOf(e, "s1"); s1.BgPhoto == nil || len(onPage(s1, 1)) != 0 || len(onPage(s1, 0)) != 1 {
				t.Fatalf("отмена: фото %+v, справа %d", s1.BgPhoto, len(onPage(s1, 1)))
			}
		})
	}
}
