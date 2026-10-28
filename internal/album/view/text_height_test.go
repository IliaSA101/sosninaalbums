package view_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Завершающий \n в тексте не растит рамку, и блок у низа листа остаётся на месте
// (FitTexts его не поднимает): рамка = строки, видимые в редакторе.
func TestTextTrailingNewlineHeight(t *testing.T) {
	e, id := edtest.AutoEd(t)
	t1 := edtest.AddText(t, e, id, 0)
	t2 := edtest.AddText(t, e, id, 0)
	st := model.TextStyle{Font: "greatvibes", Size: 80, Color: "#222222", Align: "left"}
	find := func(v model.View, tid string) model.TextView {
		for _, s := range v.Spreads {
			for _, tv := range s.Texts {
				if tv.ID == tid {
					return tv
				}
			}
		}
		t.Fatalf("блока %s нет", tid)
		return model.TextView{}
	}
	var ph, h1 float64
	for _, tid := range []string{t1, t2} {
		e.SetTextStyle(tid, st)
		v := e.SetText(tid, "Сава и Вася", "")
		tv := find(v, tid)
		if len(tv.Lines) != 1 {
			t.Fatalf("исходный текст в %d строк", len(tv.Lines))
		}
		ph, h1 = v.Settings.PageH, tv.H
	}
	// оба блока у самого низа листа, строго по высоте одной строки
	y := math.Floor((ph-h1)*100) / 100
	e.SetTextBox(t1, 10, y, 200)
	e.SetTextBox(t2, 250, y, 200)
	e.SetText(t1, "Сава и Вася\n", "")
	v := e.SetText(t2, "Сава и Вася\n", "")
	for _, tid := range []string{t1, t2} {
		tv := find(v, tid)
		if tv.H != h1 || len(tv.Lines) != 1 {
			t.Errorf("%s: H %.3f (нужно %.3f), строк %d", tid, tv.H, h1, len(tv.Lines))
		}
		if tv.Y != y {
			t.Errorf("%s: блок поднят FitTexts: y %.2f → %.2f", tid, y, tv.Y)
		}
	}
	// пустая строка после Enter — видна в редакторе, значит растит рамку
	if tv := find(e.SetText(t1, "Сава и Вася\n\n", ""), t1); len(tv.Lines) != 2 || tv.H <= h1 {
		t.Errorf("два \\n: строк %d, H %.3f", len(tv.Lines), tv.H)
	}
}
