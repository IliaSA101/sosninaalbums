package album_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// SizeBasis — чтение: настройки и число разворотов проекта, без шага отмены и rev (0.37, оценка объёма выгрузки).
func TestSizeBasis(t *testing.T) {
	p, _ := usedProject()
	p.Settings.PageW, p.Settings.PageH, p.Settings.DPI = 210, 297, 150
	e := edtest.Open(t, p, "")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	s, n := e.SizeBasis()
	if n != len(p.Spreads) || s.PageW != 210 || s.PageH != 297 || s.DPI != 150 {
		t.Errorf("SizeBasis = %+v, %d", s, n)
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Error("чтение изменило rev или отмену")
	}
	// пустой альбом: нет разворотов — 0
	e0 := edtest.Open(t, model.New(), "")
	if _, n := e0.SizeBasis(); n != 0 {
		t.Errorf("пустой альбом: %d разворотов", n)
	}
}
