package album_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// TestOpenOldCustomKernNoStep (161): альбом 0.35 со своей Lora открывается без вопросов — «по
// тексту» пересчитан с кернингом GPOS (кириллица «То» теперь кернится), а шагов отмены нет,
// «изменён» не поднят, rev 0 (FitTexts идёт в Load, не в правке).
func TestOpenOldCustomKernNoStep(t *testing.T) {
	lora := edtest.UseFont(t, "Lora", "Lora-Regular.ttf")
	const stale = 0.35 // ширина, записанная 0.35 (до кернинга GPOS)
	const txt = "Тото Тото Тото"
	st := model.TextStyle{Font: lora, Size: 40, Color: "#222222", Align: "left"}
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Texts: []*model.TextBlock{
		{ID: "0a0b0c0d0e01", X: 20, Y: 50, W: stale, Auto: true, Text: txt, Style: st},
	}}}
	path := filepath.Join(testkit.TempDir(t), "old.salbum")
	e := edtest.Open(t, p, path)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q := edtest.LoadFile(t, path)
	w := q.Spreads[0].Texts[0].W
	box := text.Layout(txt, st, 1e5)
	want := box.Lines[0].W
	if len(box.Lines[0].DX) == 0 || math.Abs(w-want) > 0.05 {
		t.Fatalf("ширина после открытия %.4f, по тексту с кернингом %.4f (DX %v)", w, want, box.Lines[0].DX)
	}
	if w == stale {
		t.Fatal("ширина осталась прежней")
	}
	e2 := edtest.Open(t, q, path)
	if edtest.UndoLen(e2) != 0 || edtest.Dirty(e2) || edtest.Rev(e2) != 0 {
		t.Fatalf("открытие: шагов отмены %d, изменён %v, rev %d", edtest.UndoLen(e2), edtest.Dirty(e2), edtest.Rev(e2))
	}
}
