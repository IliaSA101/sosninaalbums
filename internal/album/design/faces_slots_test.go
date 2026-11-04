package design_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestFaceSlots — снимок мест для поиска лиц: места с фото, кроме группового, с четвертями и
// кадром на сейчас; чтение не правит проект; чужой лист — false.
func TestFaceSlots(t *testing.T) {
	e, sid := facesEd(t, 6)
	was := jsonOf(e)
	hits, ok := e.FaceSlots(sid)
	if !ok || len(hits) != 6 {
		t.Fatalf("мест %d (ok=%v), ждали 6", len(hits), ok)
	}
	for _, h := range hits {
		if h.Place == "" || h.Photo == "" || h.Box.H != 0 || h.Missing || h.Many {
			t.Fatalf("снимок с лицом или без места: %+v", h)
		}
	}
	if jsonOf(e) != was {
		t.Fatal("чтение снимка изменило проект")
	}
	if _, ok := e.FaceSlots("нет-такого"); ok {
		t.Fatal("чужой лист принят")
	}

	// Поворот на четверть и наклон: Turn — четверти, Frame — кадр целиком; групповое выпадает.
	p := edtest.P(e)
	pl := p.Sheet(sid).Grid
	var withPhoto []*model.Place
	for _, q := range pl {
		if q.Photo != "" {
			withPhoto = append(withPhoto, q)
		}
	}
	r := p.Photos[withPhoto[0].Photo]
	r.Frame = &model.Frame{S: 1.5, X: 0.2, Y: -0.1, R: 100}
	p.Photos[withPhoto[0].Photo] = r
	withPhoto[1].Role = model.RoleGroup
	hits, _ = e.FaceSlots(sid)
	if len(hits) != 5 {
		t.Fatalf("с групповым мест %d, ждали 5", len(hits))
	}
	for _, h := range hits {
		if h.Place == withPhoto[1].ID {
			t.Fatal("групповое место попало в снимок")
		}
		if h.Place == withPhoto[0].ID && (h.Turn != 1 || h.Frame != *r.Frame) {
			t.Fatalf("поворот и кадр: %+v", h)
		}
	}

	// Лист без фото: пустой снимок, не ошибка.
	for _, q := range pl {
		q.Photo = ""
	}
	if hits, ok := e.FaceSlots(sid); !ok || len(hits) != 0 {
		t.Fatalf("пустой лист: %d мест, ok=%v", len(hits), ok)
	}
}
