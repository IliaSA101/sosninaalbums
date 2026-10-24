package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ApplyCoverGrid (127): плитка из CoverGridChoices ставится одним шагом, Left и named в JSON —
// как у остальных правок сетки; отказ — ошибка с текущим видом и Left = 0, без шага.
func TestAppApplyCoverGrid(t *testing.T) {
	a := gridApp(t)
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	ch := a.CoverGridChoices(model.ScopeRight, 3)
	if ch.Places != 0 || len(ch.Builtin) == 0 {
		t.Fatalf("галерея: %+v", ch)
	}
	r, err := a.ApplyCoverGrid(model.ScopeRight, ch.Builtin[0].Boxes)
	if err != nil || r.Left != 0 || r.View.Cover == nil || r.View.Cover.Sides[1].Places != 3 || !r.View.Cover.Sides[1].Known {
		t.Fatalf("ApplyCoverGrid: err %v, %+v", err, r.View.Cover)
	}
	if data, _ := json.Marshal(r); !strings.Contains(string(data), `"named":[]`) {
		t.Errorf("named не список: %.200s", data)
	}
	rev := r.View.Rev
	for name, call := range map[string]func() (GridResult, error){
		"корешок": func() (GridResult, error) { return a.ApplyCoverGrid(model.ScopeSpine, ch.Builtin[0].Boxes) },
		"пусто":   func() (GridResult, error) { return a.ApplyCoverGrid(model.ScopeRight, nil) },
	} {
		r, err := call()
		if err == nil || r.Left != 0 || r.View.Rev != rev || r.View.Cover == nil {
			t.Errorf("%s: err %v, left %d, rev %d (было %d)", name, err, r.Left, r.View.Rev, rev)
		}
	}
	// Шаг ‹ › идёт через встроенный Methods и отдаёт вид тем же путём.
	if v := a.StepCoverGrid(model.ScopeRight, 1); v.Rev < rev || v.Cover == nil {
		t.Errorf("StepCoverGrid: rev %d, было %d", v.Rev, rev)
	}
}
