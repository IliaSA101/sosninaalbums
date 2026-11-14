package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// defaultOf — вид, отмеченный Default в PrintPlans(last); "" — «Обычная» (ни один план не отмечен).
func defaultOf(t *testing.T, e *exportEnv, last string) string {
	t.Helper()
	got := ""
	for _, pl := range e.a.PrintPlans(last) {
		if pl.Default {
			if got != "" {
				t.Fatalf("Default у двух планов при last=%q", last)
			}
			got = pl.Kind
		}
	}
	return got
}

// TestPrintPlansDefault (123 §1.5): окно открывается на пункте Go. Через App, с настоящим
// редактором: готовая Фабрика при запомненной Фотосфере, общая страница с запретом, «Обычная»;
// проект при запросе не меняется (ни rev, ни шаг отмены), а смена альбома меняет ответ.
func TestPrintPlansDefault(t *testing.T) {
	e, _ := printEnv(t, 12, 300, 305, "fabrika-photo")
	before := e.a.editor().View()
	for _, c := range []struct{ last, want string }{
		{"fsfera", "fabrika"}, {"fabrika", "fabrika"}, {"", "fabrika"}, {"xyz", "fabrika"},
	} {
		if got := defaultOf(t, e, c.last); got != c.want {
			t.Errorf("готовая Фабрика, last %q: %q, ждали %q", c.last, got, c.want)
		}
	}
	if v := e.a.editor().View(); v.Rev != before.Rev || v.CanUndo != before.CanUndo {
		t.Fatal("PrintPlans изменил проект")
	}
	// Общая страница 300×200 без обложки: запомненная Фабрика остаётся с запретом, иначе «Обычная».
	e.a.ed.Store(mustEditor(t, book(300, 200, 3, ""), ""))
	if got := defaultOf(t, e, "fabrika"); got != "fabrika" {
		t.Errorf("общая страница, last fabrika: %q", got)
	}
	if got := defaultOf(t, e, ""); got != "" {
		t.Errorf("общая страница, last пусто: %q", got)
	}
	plans := e.a.PrintPlans("fabrika")
	if len(plans[1].Block) != 1 || !strings.Contains(plans[1].Block[0], "разворот выйдет 7087 × 2362 px") {
		t.Errorf("запрет Фабрики с px: %q", plans[1].Block)
	}
	// Лист 031 — планов нет, выбирать нечего.
	e.a.ed.Store(mustEditor(t, model.NewSheet(204, 304), ""))
	if b, _ := json.Marshal(e.a.PrintPlans("fabrika")); string(b) != "[]" {
		t.Errorf("лист 031: %s", b)
	}
}
