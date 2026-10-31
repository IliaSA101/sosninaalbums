package grid

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Края PutLookStep и LookUntouched, до которых не доходит ядро: чужой разворот, пустой шаблон.
func TestPutLookStepEdges(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{},
		Grid:  []*model.Place{{ID: "0a0000000001", X: 10, Y: 10, W: 50, H: 50, Person: model.Person{Last: "Иванов"}}},
		Texts: []*model.TextBlock{{ID: "7e0000000001", W: 50, Text: "своя", Style: model.PlainStyle()}}}}
	before := string(model.Marshal(p))
	st := model.LookStep{Name: "Глава", Tpl: model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Page: []model.TplBg{{}, {}}}}
	if plan, missing, err := PutLookStep(p, "нет", st, 1, nil); err != nil || missing != nil || plan != (model.GridPlan{}) {
		t.Fatalf("неизвестный разворот: %+v %v %v", plan, missing, err)
	}
	if _, _, err := PutLookStep(p, "s1", st, 1, nil); !errors.Is(err, model.ErrTplEmpty) {
		t.Fatalf("пустой шаг на обычном листе: %v", err)
	}
	if string(model.Marshal(p)) != before { // отказ — до снятия надписей, мест и табличек имён
		t.Fatal("отказ оставил след")
	}
	if LookUntouched(p, "нет", st, 1, nil) {
		t.Fatal("неизвестный разворот не может быть нетронутым")
	}
}
