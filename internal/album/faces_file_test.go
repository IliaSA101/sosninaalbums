package album_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// facesSnap — снимок мест листа sid с лицами (как от детектора): размер лица зависит от места.
func facesSnap(e *album.Editor, sid string) []model.FaceHit {
	p, _ := e.Snapshot()
	var out []model.FaceHit
	for _, pl := range p.Sheet(sid).Grid {
		if pl.Photo == "" {
			continue
		}
		h := 0.18 + 0.01*float64(len(out)%8)
		box := layout.Rect{X: 0.5 - h*0.375, Y: 0.22, W: h * 0.75, H: h}
		out = append(out, model.FaceHit{Place: pl.ID, Photo: pl.Photo, Frame: p.Photos[pl.Photo].FrameOrDefault(),
			Box: box, Eye: geom.Pt{X: 0.5, Y: box.Y + 0.38*h}})
	}
	return out
}

// framesByPath — кадры по пути файла: ID фото при открытии могут смениться (Rekey).
func framesByPath(p *model.Project) map[string]model.Frame {
	out := map[string]model.Frame{}
	for _, r := range p.Photos {
		out[r.Path] = r.FrameOrDefault()
	}
	return out
}

// TestFitFacesFileOnlyFrames — подгон по лицам меняет в файле только frame; после «сохранить →
// открыть» кадры те же, а повтор холостой даже без памяти fitted.
func TestFitFacesFileOnlyFrames(t *testing.T) {
	e := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	ph := make([]model.PhotoInfo, 25)
	for i := range ph {
		ph[i] = model.PhotoInfo{ID: string(rune('a' + i)), Path: edtest.Abs(string(rune('a'+i)) + ".jpg"), W: 300, H: 400}
	}
	e.FillPlaces(ph, sid)
	p0, _ := e.Snapshot()
	was := string(model.Marshal(p0))

	_, rep, _ := e.FitFaces(sid, facesSnap(e, sid), "", "", nil, nil)
	if rep.Fitted != 25 {
		t.Fatalf("итог %+v", rep)
	}
	p1, _ := e.Snapshot()
	for id, r := range p1.Photos {
		r.Frame = nil
		p1.Photos[id] = r
	}
	if string(model.Marshal(p1)) != was {
		t.Fatal("подгон поменял в проекте что-то кроме кадров")
	}

	path := filepath.Join(testkit.TempDir(t), "faces.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	pe, _ := e.Snapshot()
	if !reflect.DeepEqual(framesByPath(q), framesByPath(pe)) {
		t.Fatal("кадры после «сохранить → открыть» не те")
	}
	e2 := edtest.Open(t, q, path)
	rev, undo := edtest.Rev(e2), edtest.UndoLen(e2)
	_, again, _ := e2.FitFaces(sid, facesSnap(e2, sid), "", "", nil, nil)
	edtest.Same(t, e2, rev, undo, "повтор после «сохранить → открыть»")
	if again.Asked || again.Fitted != 25 {
		t.Fatalf("после открытия: %+v", again)
	}
}
