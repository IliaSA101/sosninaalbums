package design_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
)

// Правки подписей и текстов с чужим ID (элемент удалили, пока окно было открыто) —
// холостые: ни шага отмены, ни rev, ни ошибки.
func TestUnknownIDsNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetCaption("нет", "Подпись", "")
	e.SetText("нет", "Текст", "")
	e.SetTextBox("нет", 10, 10, 50)
	e.SetBackground("нет", "spread", "#ff0000")
	if _, err := e.AddText("нет", 0, ""); err != nil {
		t.Fatalf("текст на чужом развороте: %v", err)
	}
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
}
