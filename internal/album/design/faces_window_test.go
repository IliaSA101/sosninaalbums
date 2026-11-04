package design_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Окно силуэта под лицо (silhouette.md): тот же отбор мест, что у подгона.

// TestFaceWindow: виньетка — окно места; рамочка — окно рамочки; групповое, «Целиком», пустое
// место, чужое фото, пустой ID, чужой лист и лист не виньетка — ok = false. Чтение ничего не правит.
func TestFaceWindow(t *testing.T) {
	e, sid := facesEd(t, 6)
	p, _ := e.Snapshot()
	pl := p.Sheet(sid).Grid
	var seated []*model.Place
	for _, q := range pl {
		if q.Photo != "" {
			seated = append(seated, q)
		}
	}
	if len(seated) != 6 {
		t.Fatalf("мест с фото %d, ждали 6", len(seated))
	}

	// Каждое место: размеры окна равны размерам места (без рамочки), у разных мест они свои.
	for _, q := range seated {
		w, h, ok := e.FaceWindow(sid, q.Photo)
		if !ok || math.Abs(w-q.W) > 1e-9 || math.Abs(h-q.H) > 1e-9 || w <= 0 || h <= 0 {
			t.Fatalf("место %s: окно %v×%v ok=%v, ждали %v×%v", q.ID, w, h, ok, q.W, q.H)
		}
	}
	// Круглое место: окно то же (форму режет интерфейс), не вписанный квадрат.
	e.SetPlaceShape(sid, []string{seated[0].ID}, model.ShapeOval)
	if w, h, ok := e.FaceWindow(sid, seated[0].Photo); !ok || math.Abs(w-seated[0].W) > 1e-9 || math.Abs(h-seated[0].H) > 1e-9 {
		t.Fatalf("овальное место: окно %v×%v ok=%v", w, h, ok)
	}

	// Рамочка сужает окно: берётся BorderWindow, то же, по которому ставит кадр FaceCrop.
	if _, err := e.ApplyBorder(sid, []string{seated[1].ID}, model.BorderSrc{Kind: "line", Preset: "sa:line"}); err != nil {
		t.Fatal(err)
	}
	e.SetBorder(sid, []string{seated[1].ID}, model.BorderPatch{Width: 8}, "")
	p, _ = e.Snapshot()
	win, _ := p.BorderWindow(p.Sheet(sid).PlaceRef(seated[1].ID))
	w, h, ok := e.FaceWindow(sid, seated[1].Photo)
	if !ok || !(w < seated[1].W-1) || math.Abs(w-win.W) > 1e-9 || math.Abs(h-win.H) > 1e-9 {
		t.Fatalf("рамочка: окно %v×%v ok=%v, ждали %v×%v (место %v×%v)", w, h, ok, win.W, win.H, seated[1].W, seated[1].H)
	}

	// Групповое и «Целиком» — силуэта нет, соседи не затронуты.
	e.SetPlaceFit(sid, []string{seated[2].ID}, model.FitContain)
	edtest.Change(e, func(p *model.Project) { p.Sheet(sid).PlaceRef(seated[3].ID).Role = model.RoleGroup })
	for i, q := range seated[2:4] {
		if _, _, ok := e.FaceWindow(sid, q.Photo); ok {
			t.Errorf("место %d (целиком/групповое): окно есть", i+2)
		}
	}
	if _, _, ok := e.FaceWindow(sid, seated[4].Photo); !ok {
		t.Error("соседнее место потеряло окно")
	}

	// Чужое фото, пустой ID (пустое место), чужой лист.
	for _, c := range []struct{ name, sid, photo string }{
		{"чужое фото", sid, "нет-такого"},
		{"пустой ID", sid, ""},
		{"чужой лист", "нет-такого", seated[4].Photo},
		{"пустой лист", "", seated[4].Photo},
	} {
		if w, h, ok := e.FaceWindow(c.sid, c.photo); ok || w != 0 || h != 0 {
			t.Errorf("%s: окно %v×%v ok=%v", c.name, w, h, ok)
		}
	}

	// Лист не виньетка: те же места, фото те же — силуэта нет.
	edtest.Change(e, func(p *model.Project) { p.Sheet(sid).Kind, p.Sheet(sid).Vig = "", nil })
	if _, _, ok := e.FaceWindow(sid, seated[4].Photo); ok {
		t.Error("лист не виньетка: окно есть")
	}

	// Чтение ничего не правит.
	rev1, undo1, js := edtest.Rev(e), edtest.UndoLen(e), jsonOf(e)
	e.FaceWindow(sid, seated[4].Photo)
	e.FaceWindow(sid, seated[0].Photo)
	if edtest.Rev(e) != rev1 || edtest.UndoLen(e) != undo1 || jsonOf(e) != js {
		t.Error("FaceWindow изменил проект")
	}
}
