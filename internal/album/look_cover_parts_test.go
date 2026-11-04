package album_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Доля обложки в плане (0.28-131-174): CoverTouched / CoverReplaced входят в Touched / Replaced
// (те остаются суммой), текст вопроса берётся по ним.
func TestLookPlanCoverParts(t *testing.T) {
	o := model.LookOpts{Spreads: 6}
	// Тронута только обложка: Touched 1 — она одна.
	tpl := lkCoverFull()
	e := lkCoverEditor(t)
	lkOwnFront(e)
	plan, err := e.PlanLook(lkRefB, tpl, nil, o, nil)
	if err != nil || plan.Touched != 1 || !plan.CoverTouched || plan.CoverReplaced || plan.Replaced != 0 {
		t.Fatalf("только обложка тронута: %+v %v", plan, err)
	}
	// Тронуты только развороты (обложки в альбоме нет): CoverTouched не выставлен.
	e = lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	gone := func(model.LookRef) *model.AlbumTpl { return nil }
	if plan, err = e.PlanLook(lkRefB, lookB(), gone, o, nil); err != nil || plan.Touched != 9 || plan.CoverTouched || plan.CoverReplaced {
		t.Fatalf("развороты без обложки: %+v %v", plan, err)
	}
	// Чистая обложка и обложка прежнего макета — не тронуты.
	e = lkCoverEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	if plan, err = e.PlanLook(lkRefB, tpl, lkPrev(tpl), o, nil); err != nil || plan.CoverTouched || plan.CoverReplaced {
		t.Fatalf("обложка прежнего макета: %+v %v", plan, err)
	}
	// Свой фон обложки — CoverReplaced; Replaced считает его же; тронутого нет.
	a := lkCoverTpl()
	e = lkCoverEditor(t)
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	if plan, err = e.PlanLook(lkRefB, a, nil, o, nil); err != nil || plan.Replaced != 1 || !plan.CoverReplaced || plan.CoverTouched {
		t.Fatalf("свой фон обложки: %+v %v", plan, err)
	}
}

// Режим bg: свой фон обложки — CoverReplaced (путь ops_look_full), без своего — нет.
func TestLookPlanCoverReplacedBg(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverTpl()
	a.Cover.Bg, a.Cover.Back, a.Cover.Front = nil, lkSide(2, ""), lkSide(3, "")
	e := lkBuilt(t, 2)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	o := model.LookOpts{Mode: model.LookBg}
	base, err := e.PlanLook(lkRefB, b, lkPrev(a), o, nil)
	if err != nil || base.CoverReplaced {
		t.Fatalf("чистая обложка: %+v %v", base, err)
	}
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), o, nil)
	if err != nil || !plan.CoverReplaced || plan.Replaced != base.Replaced+1 {
		t.Fatalf("свой фон: %+v %v", plan, err)
	}
}

// CreateCoverLook отдаёт запись макета (имя для тоста) из того же прохода; смена макета между
// чтением App и правкой — ErrLookChanged без шага и следа; ожидали «нет макета», а он есть — то же.
func TestCreateCoverLookRecord(t *testing.T) {
	tpl := lkCoverFull()
	e := lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	want := e.Look()
	other := model.LookRef{Key: "my:0000000000000001", Name: "Чужой"}
	for name, w := range map[string]*model.LookRef{"другая запись": &other, "ждали без макета": nil} {
		lkNoTrace(t, e, name, func() error {
			_, look, err := e.CreateCoverLook("", nil, nil, w)
			if look != nil {
				t.Errorf("%s: запись при ошибке", name)
			}
			if !errors.Is(err, album.ErrLookChanged) {
				t.Errorf("%s: %v", name, err)
			}
			return err
		})
	}
	if edtest.P(e).Cover != nil {
		t.Fatal("обложка осталась после отказа")
	}
	_, look, err := e.CreateCoverLook("", nil, nil, want)
	if err != nil || look == nil || look.Name != want.Name || look.Key != want.Key {
		t.Fatalf("запись макета: %+v %v", look, err)
	}
	// Альбом без макета, ждали без макета — обычная обложка, записи нет.
	e = lkEditor(t)
	lkHost(e, tpl)
	if _, look, err = e.CreateCoverLook("", nil, nil, nil); err != nil || look != nil || edtest.P(e).Cover == nil {
		t.Fatalf("без макета: %+v %v", look, err)
	}
}
