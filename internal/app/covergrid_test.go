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

// TestSaveGridTemplateCover (0.27-132): «Сохранить как шаблон…» со сторонки обложки — шаблон страницы
// «Шаблон N», виден в «Мои» обеих сторонок и страницы разворота, лежит в templates.json; без мест,
// на корешке и без обложки — текст для тоста и файл не тронут.
func TestSaveGridTemplateCover(t *testing.T) {
	a := gridApp(t)
	if err := a.SaveGridTemplate(model.CoverID, model.ScopeRight, ""); err == nil || err.Error() != "Обложки нет" {
		t.Fatalf("без обложки: %v", err)
	}
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	for scope, want := range map[string]string{model.ScopeRight: "На лицевой нет мест", model.ScopeLeft: "На задней нет мест",
		model.ScopeSpine: "неизвестная область", model.ScopeSpread: "неизвестная область"} {
		if ch := a.CoverGridChoices(scope, 0); (scope == model.ScopeRight || scope == model.ScopeLeft) && ch.Places != 0 {
			t.Fatalf("в новой обложке на %s %d мест", scope, ch.Places)
		}
		if err := a.SaveGridTemplate(model.CoverID, scope, ""); err == nil || err.Error() != want {
			t.Errorf("%s: %v, ждали %q", scope, err, want)
		}
	}
	if n := len(a.templateList()); n != 0 {
		t.Fatalf("отказы записали %d шаблонов", n)
	}

	ch := a.CoverGridChoices(model.ScopeRight, 3)
	if _, err := a.ApplyCoverGrid(model.ScopeRight, ch.Builtin[0].Boxes); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	if err := a.SaveGridTemplate(model.CoverID, model.ScopeRight, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGridTemplate(model.CoverID, model.ScopeRight, " Мой\n вид "); err != nil {
		t.Fatal(err)
	}
	list := a.templateList()
	if len(list) != 2 || list[0].Scope != model.TplPage || list[0].Name != "Шаблон 1" || list[1].Name != "Мой вид" || len(list[0].Boxes) != 3 {
		t.Fatalf("шаблоны: %+v", list)
	}
	if v := a.editor().View(); v.Rev != rev {
		t.Errorf("сохранение шаблона изменило проект: rev %d → %d", rev, v.Rev)
	}
	// Шаблон из файла виден в «Мои» обложки (обеих сторонок) и страницы разворота.
	a.loadTemplates()
	for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
		if mine := a.CoverGridChoices(scope, 3).Mine; len(mine) != 2 || mine[0].ID != list[0].ID || mine[0].Scope != model.TplPage {
			t.Errorf("Мои обложки %s: %+v", scope, mine)
		}
	}
	if mine := a.GridChoices("s1", 3, model.ScopeLeft).Mine; len(mine) != 2 || mine[0].ID != list[0].ID {
		t.Errorf("Мои страницы: %+v", mine)
	}
}
