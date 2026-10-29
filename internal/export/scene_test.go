package export

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// SheetScene (кадр пипетки, 109) даёт ту же сцену, что экспорт листа: у каждого листа вида,
// в том числе с несколькими фото и пустого.
func TestSheetSceneSameAsFromView(t *testing.T) {
	p := model.New()
	p.Settings.Bg = "#f6f1e7"
	for _, id := range []string{"a", "b", "c"} {
		p.Photos[id] = model.PhotoRef{Path: testkit.AbsPath(id + ".jpg"), W: 300, H: 200}
	}
	grid := func(ids ...string) []*model.Place {
		var out []*model.Place
		for i, id := range ids {
			out = append(out, &model.Place{ID: model.NewID(), Photo: id, X: float64(5 + i*60), Y: 5, W: 55, H: 40})
		}
		return out
	}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a", "b"}, Grid: grid("a", "b")},
		{ID: "s2", Photos: []string{"c"}, Grid: grid("c")},
		{ID: "s3"}, // пустой лист — один фон
	}
	pp, v := mustEditor(t, p, "").Snapshot()
	want := FromView(pp, v)
	if len(want) != 3 || len(v.Spreads) != 3 {
		t.Fatalf("листов %d, сцен %d", len(v.Spreads), len(want))
	}
	for i, s := range v.Spreads {
		if got := SheetScene(pp, s); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("лист %s: сцена отличается от FromView", s.ID)
		}
	}
	if len(want[0].Photos) != 2 || want[0].Photos[0].Path == "" {
		t.Errorf("в сцене первого листа нет фото: %+v", want[0].Photos)
	}
}
