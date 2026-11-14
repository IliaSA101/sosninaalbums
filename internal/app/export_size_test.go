package app

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// ExportSize берёт настройки и число разворотов из проекта, а не от интерфейса; проект не меняется.
func TestExportSizeFromProject(t *testing.T) {
	e := newExportEnv(t, 3)
	rev := e.a.editor().View().Rev
	in := ExportSizeIn{Scope: "all", DPI: 300, Quality: 95}
	got := e.a.ExportSize(in)
	s, n := e.a.editor().SizeBasis()
	if n != 3 || s.PageW != 60 {
		t.Fatalf("основа: %v, %d", s, n)
	}
	want := export.SizeSummary(s, 3, export.SizeIn(in))
	if got.Summary != want.Summary || len(got.Per) != 3 {
		t.Fatalf("ответ %+v, ждали %+v", got, want)
	}
	if !strings.Contains(got.Summary, " на разворот, всего ") {
		t.Errorf("три разворота — есть «всего»: %q", got.Summary)
	}
	// «Текущий» — один файл, без «всего»
	if cur := e.a.ExportSize(ExportSizeIn{Scope: "current", DPI: 300, Quality: 95}).Summary; strings.Contains(cur, "всего") {
		t.Errorf("текущий: %q", cur)
	}
	if e.a.editor().View().Rev != rev {
		t.Error("оценка объёма изменила проект")
	}
}

// Лист-виньетка и пустой альбом: слова и ноль разворотов без паники.
func TestExportSizeSheetAndEmpty(t *testing.T) {
	a := newApp(t)
	p := model.New()
	p.Settings.Sheet = true
	a.ed.Store(mustEditor(t, p, ""))
	out := a.ExportSize(ExportSizeIn{Scope: "all", Quality: 95})
	if !strings.HasSuffix(out.Summary, " на лист") { // разворотов нет: итога «всего» нет
		t.Errorf("пустой лист: %q", out.Summary)
	}
}
