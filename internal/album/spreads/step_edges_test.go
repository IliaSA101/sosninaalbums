package spreads

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// step: ключ вне обоих рядов — вперёд к началу, назад к концу первого ряда; пустой ряд — "" (раскладка не меняется).
func TestStepKeyOutsideRows(t *testing.T) {
	in := []layout.Result{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	re := []layout.Result{{Key: "r"}}
	if got := step(in, re, "чужой", 1); got != "a" {
		t.Errorf("вперёд: %q", got)
	}
	if got := step(in, re, "чужой", -1); got != "c" {
		t.Errorf("назад: %q", got)
	}
	if got := step(nil, re, "чужой", 1); got != "" {
		t.Errorf("пустой ряд: %q", got)
	}
	if got := step(nil, nil, "чужой", -1); got != "" {
		t.Errorf("пустые ряды: %q", got)
	}
}

// stepMore на коротком ряду (меньше ManyFrom фото досчитывать нечего): к началу ряда, а не пустой ключ.
func TestStepMoreNothingToAdd(t *testing.T) {
	prm := model.New().Params()
	asp := []float64{1.5, 0.7, 1}
	row, _, _ := model.AutoRow(asp, prm, "")
	if len(row) == 0 {
		t.Fatal("нет ряда")
	}
	if got := stepMore(asp, prm, row); got != row[0].Key {
		t.Errorf("stepMore = %q, ждали начало ряда %q", got, row[0].Key)
	}
}
