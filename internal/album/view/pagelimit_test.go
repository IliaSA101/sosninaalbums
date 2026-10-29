package view_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestViewPageLimit — View.PageLimit (085): обложка Фотосферы называет изделие; правка числа
// разворотов (добавить много разом, удалить несколько разом) меняет warn, отмена — обратно,
// холостая правка — без шага; Save→Load — то же; без типографии поля в JSON нет.
func TestViewPageLimit(t *testing.T) {
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	lim := func() model.PageLimitView {
		t.Helper()
		l := e.View().PageLimit
		if l == nil {
			t.Fatal("предела нет")
		}
		return *l
	}
	in := model.PageLimitView{Who: "Фотосфера «Тонкие листы»", Range: "до 57 разворотов", Count: 2, Max: 57}
	if l := lim(); l != in {
		t.Fatalf("2 разворота: %+v", l)
	}
	edtest.Change(e, func(p *model.Project) { // 56 новых разворотов одним шагом: 58
		for range 56 {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
		}
	})
	over := in
	over.Count, over.Warn = 58, "Разворотов 58 — у Фотосферы («Тонкие листы») не больше 57"
	if l := lim(); l != over {
		t.Fatalf("58 разворотов: %+v", l)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if l := edtest.Open(t, q, path).View().PageLimit; l == nil || *l != over {
		t.Fatalf("после Save→Load %+v", l)
	}
	// Холостая правка (те же настройки) — ни шага, ни смены предела.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetSettings(edtest.P(e).Settings)
	edtest.Same(t, e, rev, undo, "те же настройки")
	if l := lim(); l != over {
		t.Fatalf("после холостой правки %+v", l)
	}
	// Удалить два разворота одним шагом — 56, в пределах.
	ids := []string{edtest.P(e).Spreads[2].ID, edtest.P(e).Spreads[3].ID}
	e.DeleteSpreads(ids)
	back := in
	back.Count = 56
	if l := lim(); l != back || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("56 разворотов: %+v, шагов %d", l, edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if l := lim(); l != over {
		t.Fatalf("после отмены %+v", l)
	}
	e.Redo()
	if l := lim(); l != back {
		t.Fatalf("после повтора %+v", l)
	}
	// Без обложки на общем формате — типография неизвестна: поля нет и в JSON.
	edtest.Change(e, func(p *model.Project) { p.Cover = nil })
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 300, 300
	v := e.SetSettings(st)
	if v.PageLimit != nil {
		t.Fatalf("общий 30×30: %+v", *v.PageLimit)
	}
	if b, err := json.Marshal(v); err != nil || bytes.Contains(b, []byte(`"pageLimit"`)) {
		t.Fatalf("pageLimit в JSON без типографии (%v)", err)
	}
	if v := e.Undo(); v.PageLimit == nil || v.PageLimit.Who != model.CoverGroupFsfera || v.PageLimit.Count != 56 {
		t.Fatalf("отмена формата: %+v", v.PageLimit)
	}
}
