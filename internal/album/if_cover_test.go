package album_test

// Кадр обложки агента (151, design §1.3, решение 0.34-151-16): CreateCoverLookIf и CoverPhotoIf.
// Бросок человека (CreateCoverLook) не меняется: замена кадра в занятом месте — только у агента.

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// icSeats — макет с n местами на лицевой (задняя — два места) и оформленная обложка без фото.
func icSeats(t *testing.T, e *album.Editor, n int) {
	t.Helper()
	tpl := lkCoverFull()
	tpl.Cover.Front = lkSide(n, "#998877")
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled == nil {
		t.Fatalf("обложка без фото: %v", err)
	}
}

// icPut — CoverPhotoIf с текущим rev; ошибка — провал.
func icPut(t *testing.T, e *album.Editor, ph model.PhotoInfo) (place string, changed bool) {
	t.Helper()
	_, place, changed, err := e.CoverPhotoIf(ph, album.Expect{Rev: edtest.Rev(e)})
	if err != nil {
		t.Fatalf("CoverPhotoIf(%s): %v", ph.ID, err)
	}
	return place, changed
}

// TestCreateCoverLookIf: та же обложка, что у броска человека (CreateCoverLook на двойнике), один
// шаг; place — место; Undo убирает обложку; круг Save → Load держит кадр; отказы сверки без следа.
func TestCreateCoverLookIf(t *testing.T) {
	tpl := lkCoverFull()
	path := filepath.Join(testkit.TempDir(t), "c.salbum")
	e := edtest.Open(t, model.New(), path)
	twin := lkEditor(t)
	for _, ed := range []*album.Editor{e, twin} {
		lkApply(t, ed, lkRefB, tpl, nil, 6)
		lkHost(ed, tpl)
	}
	ph := edtest.Lib(edtest.Abs("new1.jpg"), model.XmpMarks{})
	if _, _, err := twin.CreateCoverLook("", &ph, nil, twin.Look()); err != nil {
		t.Fatal(err)
	}
	before, undo, rev := model.Marshal(edtest.P(e)), edtest.UndoLen(e), edtest.Rev(e)

	v, styled, place, err := e.CreateCoverLookIf("", &ph, nil, e.Look(), album.Expect{Rev: rev})
	if err != nil || styled == nil || place != album.CoverPlaceSeat || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("CreateCoverLookIf: styled %v, place %q, rev %d → %d, шагов %d → %d, %v",
			styled != nil, place, rev, v.Rev, undo, edtest.UndoLen(e), err)
	}
	if got, want := csFilled(edtest.P(e)), csFilled(edtest.P(twin)); !reflect.DeepEqual(got, want) || got[0] != ph.ID {
		t.Fatalf("места %q, как у броска %q", got, want)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || csFilled(q)[0] != ph.ID {
		t.Fatalf("после Save → Load: %v, %q", err, csFilled(q))
	}
	if _, err := e.UndoIfRev(album.Expect{Rev: v.Rev}); err != nil || edtest.P(e).Cover != nil || !reflect.DeepEqual(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("отмена: %v, обложка %v", err, edtest.P(e).Cover != nil)
	}
}

// Вторая обложка и сменившийся макет — отказ без следа; сверка — на живом редакторе.
func TestCreateCoverLookIfRefuse(t *testing.T) {
	tpl := lkCoverFull()
	e := lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	ph := edtest.Photo("late")
	s := take(e)
	if _, _, _, err := e.CreateCoverLookIf("", &ph, nil, nil, album.Expect{Rev: s.rev}); !errors.Is(err, album.ErrLookChanged) {
		t.Fatalf("чужой макет: %v", err)
	}
	s.same(t, e, "чужой макет")
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, _, err := e.CreateCoverLookIf("", &ph, nil, e.Look(), x)
		return err
	})
}

// TestCoverPhotoIf: места лицевой берутся по очереди, при полных — кадр в наибольшем (копией, прежний
// кадр уходит в ленту), повтор того же файла — холостая; по шагу на каждую правку; круг Save → Load.
func TestCoverPhotoIf(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "p.salbum")
	e := edtest.Open(t, model.New(), path)
	icSeats(t, e, 3)
	ph := make([]model.PhotoInfo, 6)
	for i := range ph {
		ph[i] = edtest.Lib(edtest.Abs("cp"+string(rune('a'+i))+".jpg"), model.XmpMarks{})
	}
	for i := range 3 { // пустые места — по порядку
		undo := edtest.UndoLen(e)
		if place, changed := icPut(t, e, ph[i]); place != album.CoverPlaceSeat || !changed || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("фото %d: place %q, changed %v, шагов %d", i, place, changed, edtest.UndoLen(e)-undo)
		}
	}
	if got := csFilled(edtest.P(e)); !reflect.DeepEqual(got[:3], []string{ph[0].ID, ph[1].ID, ph[2].ID}) {
		t.Fatalf("места %q", got)
	}
	// Пустых нет, места равны: заменяется первое; прежний кадр — в ленту (не в альбоме, записи нет).
	undo := edtest.UndoLen(e)
	if place, changed := icPut(t, e, ph[3]); place != album.CoverPlaceSeat || !changed || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("замена: place %q, changed %v", place, changed)
	}
	p := edtest.P(e)
	if got := csFilled(p); !reflect.DeepEqual(got[:3], []string{ph[3].ID, ph[1].ID, ph[2].ID}) || p.Placed(ph[0].ID) {
		t.Fatalf("после замены %q, прежний в альбоме %v", got, p.Placed(ph[0].ID))
	}
	if _, ok := p.Photos[ph[0].ID]; ok {
		t.Fatal("запись заменённого кадра осталась в проекте")
	}
	// Повтор того же файла — ни шага, ни rev.
	s := take(e)
	if place, changed := icPut(t, e, ph[3]); place != album.CoverPlaceSeat || changed {
		t.Fatalf("повтор: place %q, changed %v", place, changed)
	}
	s.same(t, e, "повтор кадра")
	// Наибольшее место: второе вырастает — заменяется оно.
	edtest.Change(e, func(p *model.Project) { pl := p.CoverSeats()[1]; pl.W *= 1.2; pl.H *= 1.2 })
	if icPut(t, e, ph[4]); csFilled(edtest.P(e))[1] != ph[4].ID || csFilled(edtest.P(e))[0] != ph[3].ID {
		t.Fatalf("наибольшее место не заменено: %q", csFilled(edtest.P(e)))
	}
	// Круг Save → Load.
	want := csFilled(edtest.P(e))
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || !reflect.DeepEqual(csFilled(q), want) {
		t.Fatalf("после Save → Load: %v, %q (было %q)", err, csFilled(q), want)
	}
	// Отмена возвращает прежний кадр одним шагом.
	if _, err := e.UndoIfRev(album.Expect{Rev: edtest.Rev(e)}); err != nil || csFilled(edtest.P(e))[1] != ph[1].ID {
		t.Fatalf("отмена: %v, места %q", err, csFilled(edtest.P(e)))
	}
}

// Отказы CoverPhotoIf: обложки нет; сверка. Ни следа.
func TestCoverPhotoIfRefuse(t *testing.T) {
	e := lkEditor(t)
	s := take(e)
	if _, _, _, err := e.CoverPhotoIf(edtest.Photo("x"), album.Expect{Rev: s.rev}); !errors.Is(err, cover.ErrNone) {
		t.Fatalf("без обложки: %v", err)
	}
	s.same(t, e, "без обложки")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, _, err := e.CoverPhotoIf(edtest.Photo("x"), x)
		return err
	})
}

// Мест на лицевой нет: фото фоном правой сторонки; другое фото заменяет его (рамка стоит, кадр
// сброшен), то же — холостая; через круг Save → Load фон на месте.
func TestCoverPhotoIfBackdrop(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "b.salbum")
	e := edtest.Open(t, model.New(), path)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	a, b := edtest.Lib(edtest.Abs("bga.jpg"), model.XmpMarks{}), edtest.Lib(edtest.Abs("bgb.jpg"), model.XmpMarks{})
	if place, changed := icPut(t, e, a); place != album.CoverPlaceBackground || !changed {
		t.Fatalf("первое: %q %v", place, changed)
	}
	first := *edtest.P(e).Cover.Sheet.BgPhotoR
	if first.Photo != a.ID {
		t.Fatalf("фон %+v", first)
	}
	s := take(e)
	if _, changed := icPut(t, e, a); changed {
		t.Fatal("то же фото фоном снова — не холостая")
	}
	s.same(t, e, "повтор фона")
	if place, changed := icPut(t, e, b); place != album.CoverPlaceBackground || !changed {
		t.Fatalf("второе: %q %v", place, changed)
	}
	got := edtest.P(e).Cover.Sheet.BgPhotoR
	if got.Photo != b.ID || got.W != first.W || got.H != first.H {
		t.Fatalf("фон %+v, ждали фото %s в той же рамке %+v", got, b.ID, first)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || q.Cover.Sheet.BgPhotoR == nil || q.Cover.Sheet.BgPhotoR.Photo != b.ID {
		t.Fatalf("после Save → Load: %v, %+v, %v", err, q.Cover.Sheet.BgPhotoR, q.Photos)
	}
}

// wrap: фото лицевой — BgPhoto через корешок; замена не опустошает заднюю и корешок (ScopeRight
// снял бы BgPhoto), повтор — холостая.
func TestCoverPhotoIfWrap(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetCoverLayout(model.CoverWrap); err != nil {
		t.Fatal(err)
	}
	a, b := edtest.Photo("wa"), edtest.Photo("wb")
	if _, err := e.SetBackdrop(model.CoverID, model.ScopeSpread, a); err != nil {
		t.Fatal(err)
	}
	if place, changed := icPut(t, e, b); place != album.CoverPlaceBackground || !changed {
		t.Fatalf("замена: %q %v", place, changed)
	}
	c := edtest.P(e).Cover.Sheet
	if c.BgPhoto == nil || c.BgPhoto.Photo != b.ID || c.BgPhotoL != nil || c.BgPhotoR != nil || c.BgPhotoS != nil {
		t.Fatalf("wrap: BgPhoto %+v, L %v R %v S %v", c.BgPhoto, c.BgPhotoL, c.BgPhotoR, c.BgPhotoS)
	}
	s := take(e)
	if place, changed := icPut(t, e, b); place != album.CoverPlaceBackground || changed {
		t.Fatalf("повтор: %q %v", place, changed)
	}
	s.same(t, e, "повтор wrap")
	// Пустой wrap: фото встаёт через весь лист, а не на правую сторонку.
	e2 := edtest.Open(t, model.New(), "")
	if _, err := e2.CreateCover("", nil); err != nil {
		t.Fatalf("CreateCover: %v", err)
	}
	if _, err := e2.SetCoverLayout(model.CoverWrap); err != nil {
		t.Fatalf("SetCoverLayout: %v", err)
	}
	icPut(t, e2, a)
	if c := edtest.P(e2).Cover.Sheet; c.BgPhoto == nil || c.BgPhotoR != nil {
		t.Fatalf("пустой wrap: BgPhoto %v R %v", c.BgPhoto, c.BgPhotoR)
	}
}

// TestCoverPhotoTwice: на лицевой одно место; фото из альбома дважды подряд — второе встаёт копией
// на место первого, развороты не тронуты (обмена нет), два шага Ctrl+Z.
func TestCoverPhotoTwice(t *testing.T) {
	e := lkBuilt(t, 0)
	pha, phb := edtest.Lib(edtest.Abs("pha.jpg"), model.XmpMarks{}), edtest.Lib(edtest.Abs("phb.jpg"), model.XmpMarks{})
	if _, left, _ := e.FillPlaces([]model.PhotoInfo{pha, phb}, ""); left != 0 { // ID от пути: копия считает основу по нему
		t.Fatalf("фото не встали в развороты: %d", left)
	}
	tpl := lkCoverFull()
	tpl.Cover.Front = lkSide(1, "#998877")
	lkHost(e, tpl)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled == nil {
		t.Fatalf("обложка: %v", err)
	}
	spreads, undo := lkSnap(e), edtest.UndoLen(e)
	icPut(t, e, pha)
	first := csFilled(edtest.P(e))[0]
	icPut(t, e, phb)
	p := edtest.P(e)
	second := csFilled(p)[0]
	if model.BaseID(first) != pha.ID || model.CopyNo(first) < 2 || model.BaseID(second) != phb.ID || model.CopyNo(second) < 2 {
		t.Fatalf("в месте %q, затем %q: ждали копии pha и phb", first, second)
	}
	if !reflect.DeepEqual(spreads, lkSnap(e)) || !p.Placed(pha.ID) || !p.Placed(phb.ID) {
		t.Fatal("развороты изменились (обмен вместо копии)")
	}
	if _, ok := p.Photos[first]; ok {
		t.Fatalf("копия первого кадра осталась в проекте: %q", first)
	}
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов %d, ждали 2", edtest.UndoLen(e)-undo)
	}
	// Тот же файл по копии и по основе — повтор: места не меняются.
	s := take(e)
	if _, changed := icPut(t, e, phb); changed {
		t.Fatal("повтор кадра создал копию")
	}
	s.same(t, e, "повтор по файлу")
	e.Undo()
	if got := csFilled(edtest.P(e))[0]; got != first {
		t.Fatalf("первая отмена: %q, ждали %q", got, first)
	}
	e.Undo()
	if got := csFilled(edtest.P(e))[0]; got != "" || !reflect.DeepEqual(spreads, lkSnap(e)) {
		t.Fatalf("вторая отмена: %q", got)
	}
}

// TestCoverPhotoLibPlaced: своё из библиотеки, уже стоящее в месте разворота, встаёт фоном лицевой
// даже при пустых местах (как у броска); развороты не тронуты. Не стоящее в альбоме — в место.
func TestCoverPhotoLibPlaced(t *testing.T) {
	e := lkBuilt(t, 1)
	const key = "my:00000000000000ff"
	lib := model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}
	if _, left, _ := e.FillPlaces([]model.PhotoInfo{lib}, ""); left != 0 {
		t.Fatalf("не встало в разворот: %d", left)
	}
	tpl := lkCoverFull()
	lkHost(e, tpl)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled == nil {
		t.Fatalf("обложка: %v", err)
	}
	spreads := lkSnap(e)
	if place, changed := icPut(t, e, lib); place != album.CoverPlaceBackground || !changed {
		t.Fatalf("place %q, changed %v", place, changed)
	}
	p := edtest.P(e)
	if b := p.Cover.Sheet.BgPhotoR; b == nil || b.Photo != lib.ID || !p.Placed(lib.ID) || !reflect.DeepEqual(spreads, lkSnap(e)) {
		t.Fatalf("фон %+v, кадр на развороте %v", b, p.Placed(lib.ID))
	}
	if got := csFilled(p); got[0] != "" {
		t.Fatalf("место занято: %q", got)
	}
	free := model.PhotoInfo{ID: model.LibPhotoID("my:0000000000000fff"), Path: edtest.Abs("lib2.jpg"), W: 300, H: 200, Lib: "my:0000000000000fff"}
	if place, _ := icPut(t, e, free); place != album.CoverPlaceSeat || csFilled(edtest.P(e))[0] != free.ID {
		t.Fatalf("свободное своё: place %q, места %q", place, csFilled(edtest.P(e)))
	}
}
