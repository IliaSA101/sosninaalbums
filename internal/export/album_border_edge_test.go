package export

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Края рамочек в экспорте (079 G4): вид и проект разошлись, места не соответствуют фото листа,
// склейка названий пропавшего своего.

// Какие места листа считаются «с фото в порядке s.Photos»: только сетка, и только если порядок и
// число совпадают с фото листа; иначе рамочек нет вовсе (лучше без рамочки, чем чужая).
func TestPlacedPlaces(t *testing.T) {
	pl := func(id, photo string) model.PlaceView { return model.PlaceView{ID: id, Photo: photo} }
	sv := model.SpreadView{Grid: true, Photos: []string{"a", "b"},
		Places: []model.PlaceView{pl("p1", "a"), pl("p0", ""), pl("p2", "b")}}
	got := placedPlaces(sv)
	if len(got) != 2 || got[0].ID != "p1" || got[1].ID != "p2" {
		t.Fatalf("годный лист: %+v", got)
	}
	for name, mut := range map[string]func(*model.SpreadView){
		"не сетка":           func(s *model.SpreadView) { s.Grid = false },
		"лишнее фото в виде": func(s *model.SpreadView) { s.Photos = []string{"a", "b", "c"} },
		"меньше фото":        func(s *model.SpreadView) { s.Photos = []string{"a"} },
		"порядок другой":     func(s *model.SpreadView) { s.Photos = []string{"b", "a"} },
		"другое фото":        func(s *model.SpreadView) { s.Photos = []string{"a", "z"} },
		"нет мест":           func(s *model.SpreadView) { s.Places = nil },
		"нет фото":           func(s *model.SpreadView) { s.Photos = nil },
	} {
		s := sv
		s.Places = append([]model.PlaceView(nil), sv.Places...)
		mut(&s)
		if got := placedPlaces(s); got != nil {
			t.Errorf("%s: %+v, ждали nil", name, got)
		}
	}
}

// Вид и проект разошлись (место или разворот пропали из проекта после снимка): рамочка-картинка не
// рисуется и не маскирует фото; остальное задание цело. Расходится сразу несколько мест листа.
func TestExportBorderViewProjectDiverged(t *testing.T) {
	file := filepath.Join(testkit.TempDir(t), "fr.png")
	p := borderAlbum(0)
	e := mustEditor(t, p, "")
	if _, err := e.ApplyBorder(brSheet, []string{placeA, placeB}, model.BorderSrc{Photo: ownFrame(file), Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}); err != nil {
		t.Fatal(err)
	}
	pp, v := e.Snapshot()
	if got := FromView(pp, v)[0]; len(got.Borders) != 1 || got.Photos[0].Win == nil {
		t.Fatalf("исходное задание без рамочки: %+v", got)
	}
	place := layout.Rect{X: 20, Y: 10, W: 60, H: 40}

	// Места нет в проекте.
	gone := *pp
	gone.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{"a"}}}
	es := FromView(&gone, v)[0]
	if len(es.Borders) != 0 || es.Photos[0].Win != nil || es.Photos[0].Pivot != place {
		t.Errorf("места нет в проекте: слоёв %d, маска %+v", len(es.Borders), es.Photos[0].Win)
	}
	// Места нет в проекте, а рамочка у соседнего есть: чужое место не подходит по ID.
	other := *pp
	other.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{"a"}, Grid: []*model.Place{
		{ID: "0000000000ff", X: 20, Y: 10, W: 60, H: 40, Photo: "a", Border: &model.Border{Photo: model.LibPhotoID(brKey)}}}}}
	if es := FromView(&other, v)[0]; len(es.Borders) != 0 {
		t.Errorf("чужое место: слоёв %d", len(es.Borders))
	}
	// В проекте у места рамочки уже нет.
	bare := borderAlbum(0)
	if es := FromView(bare, v)[0]; len(es.Borders) != 0 || es.Photos[0].Win != nil {
		t.Errorf("рамочки в проекте нет: слоёв %d", len(es.Borders))
	}
	// Разворота нет в проекте (другой ID).
	moved := *pp
	moved.Spreads = []*model.Spread{{ID: "zz", Photos: []string{"a"}}}
	if es := FromView(&moved, v)[0]; len(es.Borders) != 0 {
		t.Errorf("разворота нет: слоёв %d", len(es.Borders))
	}

	// Места листа разошлись с фото: ни окон, ни слоёв, фото на месте как без рамочек.
	vv := v
	vv.Spreads = append([]model.SpreadView(nil), v.Spreads...)
	vv.Spreads[0].Places = nil
	es = FromView(pp, vv)[0]
	if len(es.Borders) != 0 || es.Photos[0].Win != nil || es.Photos[0].Pivot != (layout.Rect{}) {
		t.Errorf("места не совпали с фото: %+v", es.Photos[0])
	}
}

func TestJoinRu(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"фоны"}, "фоны"},
		{[]string{"фоны", "стикеры"}, "фоны и стикеры"},
		{[]string{"фоны", "стикеры", "рамочки"}, "фоны, стикеры и рамочки"},
	} {
		if got := joinRu(c.in); got != c.want {
			t.Errorf("%v: %q, ждали %q", c.in, got, c.want)
		}
	}
}
