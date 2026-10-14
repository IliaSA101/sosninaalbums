package names_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/names"
)

// Чужое фото и чужой разворот — холостые правки: ни шага отмены, ни rev.
func TestNamesUnknownIDsNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetPerson("нет", model.Person{Last: "Иванов"}, "p")
	e.SetSheetCaps("нет", true, model.SheetCaps{Style: model.DefaultCaptionStyle()})
	e.SetSheetCaps("нет", false, model.SheetCaps{})
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
}

// Фото листа, которого нет в списке фото проекта (битый файл), вставка пропускает без паники
// и в «без имени» не считает.
func TestPasteSkipsMissingPhoto(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s", Photos: []string{"ghost", "a"}}}
	p.Photos["a"] = model.PhotoRef{Path: `D:\x\DSC_0001.jpg`}
	extra, missing, err := names.Paste(p, "s", []model.Person{{Last: "Иванов"}, {Last: "Петров"}, {Last: "Сидоров"}})
	if err != nil || extra != 1 || missing != 0 {
		t.Fatalf("лишних %d, без имени %d, err %v", extra, missing, err)
	}
	if got := p.Photos["a"].Person; got == nil || got.Last != "Петров" {
		t.Fatalf("имя по индексу в ленте: %+v", got)
	}
	if _, ok := p.Photos["ghost"]; ok {
		t.Fatal("чужое фото появилось в проекте")
	}
	if _, _, err := names.Paste(p, "нет", nil); err == nil {
		t.Fatal("чужой разворот принят")
	}
}
