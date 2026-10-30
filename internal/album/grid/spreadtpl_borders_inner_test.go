package grid

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочка с индексом вне мест (зажим такие снимает, здесь — страховка) места не трогает и не паникует.
func TestTplBordersOutOfRange(t *testing.T) {
	tp := model.SpreadTpl{Places: []model.Box{{W: 0.5, H: 0.5}}, Borders: []model.TplBorder{{I: -1, Kind: "line"}, {I: 1, Kind: "line"}}}
	(&tplRes{}).borders(&tp)
	if tp.Places[0].Border != nil {
		t.Fatalf("рамочка вне мест села на место 0: %+v", tp.Places[0].Border)
	}
}
