package names_test

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func TestPasteNames(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 50, "a"), edtest.Gp(70, 10, 50, 50, ""),
		edtest.Gp(130, 10, 50, 50, "b"), edtest.Gp(10, 70, 50, 50, "c"), edtest.Gp(70, 70, 50, 50, "d")})
	undo := edtest.UndoLen(e)
	v, extra, missing, err := e.PasteNames("s1", "1. Сидоров Иван\r\n\n2) Петрова\tАнна\tСергеевна\nКозлов Пётр", false)
	if err != nil || extra != 0 || missing != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("вставка: лишних %d, без имени %d, err %v, шагов %d", extra, missing, err, edtest.UndoLen(e)-undo)
	}
	// Порядок мест, пустое место пропускается: a, b, c получили имена, d — нет.
	want := map[string]model.Person{"a": {Last: "Сидоров", First: "Иван"},
		"b": {Last: "Петрова", First: "Анна", Middle: "Сергеевна"}, "c": {Last: "Козлов", First: "Пётр"}}
	for id, w := range want {
		if got := edtest.P(e).Photos[id].Person; got == nil || *got != w {
			t.Fatalf("%s: %+v, ждали %+v", id, got, w)
		}
	}
	if edtest.P(e).Photos["d"].Person != nil || v.Rev != edtest.Rev(e) {
		t.Fatal("d без имени")
	}
	// Та же вставка — холостая: ни шага, ни rev.
	rev := edtest.Rev(e)
	if _, _, _, err := e.PasteNames("s1", "Сидоров Иван\nПетрова Анна Сергеевна\nКозлов Пётр", false); err != nil || edtest.Rev(e) != rev {
		t.Fatalf("повтор: err %v, rev %d → %d", err, rev, edtest.Rev(e))
	}
	// Имя первым и лишние строки.
	_, extra, missing, _ = e.PasteNames("s1", "Иван Сидоров\nАнна Сергеевна Петрова\nПётр Козлов\nОля Иванова\nЛишний Человек", true)
	if extra != 1 || missing != 0 || *edtest.P(e).Photos["b"].Person != want["b"] || edtest.P(e).Photos["d"].Person.Last != "Иванова" {
		t.Fatalf("имя первым: лишних %d, без имени %d, b %+v", extra, missing, edtest.P(e).Photos["b"].Person)
	}
	// Отмена — одним шагом.
	e.Undo()
	if edtest.P(e).Photos["d"].Person != nil {
		t.Fatal("отмена вставки")
	}
	// Слишком длинный список и чужой разворот — ошибка без правки.
	rev = edtest.Rev(e)
	if _, _, _, err := e.PasteNames("s1", strings.Repeat("Иванов Петя\n", 1001), false); err == nil || edtest.Rev(e) != rev {
		t.Fatal("1001 строка принята")
	}
	if _, _, _, err := e.PasteNames("нет", "Иванов Петя", false); err == nil || edtest.Rev(e) != rev {
		t.Fatal("чужой разворот принят")
	}
}

func TestSheetCapsOverride(t *testing.T) {
	e, s0, _ := edtest.NewTwo(t)
	st := model.DefaultCaptionStyle()
	e.SetCaption("a", "А", "")
	e.SetCaption("b", "Б", "")
	st.Size = 9
	e.SetCaptionStyle("a", st, false, 1)
	st.Size = 11
	e.SetCaptionStyle("b", st, true, 3)
	sizes := func() []float64 {
		var out []float64
		for _, c := range e.View().Spreads[0].Captions {
			out = append(out, c.Style.Size)
		}
		return out
	}
	undo := edtest.UndoLen(e)
	c := model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1}
	c.Style.Size = 14
	v := e.SetSheetCaps(s0, true, c)
	if got := sizes(); len(got) != 2 || got[0] != 14 || got[1] != 14 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("стиль листа: %v, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	if sv := v.Spreads[0]; !sv.Captions[1].Sheet || sv.Captions[1].Over || sv.Caps == nil || sv.Caps.Style.Size != 14 {
		t.Fatalf("вид листа: %+v", sv.Caps)
	}
	// Свои стили не стёрты, на другом листе не действует.
	if a := edtest.P(e).Photos["a"].Caption; a.Style.Size != 9 {
		t.Fatalf("свой стиль a: %+v", a.Style)
	}
	if edtest.P(e).Spreads[1].Caps != nil {
		t.Fatal("стиль ушёл на другой лист")
	}
	// Правка стиля листа заменяет Caps целиком (копии разворота делят указатель).
	old := edtest.P(e).Spreads[0].Caps
	c.Style.Size = 16
	e.SetSheetCaps(s0, true, c)
	if old.Style.Size != 14 || edtest.P(e).Spreads[0].Caps == old {
		t.Fatal("Caps правится на месте")
	}
	// Снять — прежние кегли; каждое действие — шаг отмены.
	e.SetSheetCaps(s0, false, model.SheetCaps{})
	if got := sizes(); got[0] != 9 || got[1] != 11 || edtest.UndoLen(e) != undo+3 {
		t.Fatalf("сняли: %v, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if got := sizes(); got[0] != 16 {
		t.Fatalf("отмена снятия: %v", got)
	}
	// Мусор зажимается: NaN-отступ — 1 мм, кегль — в пределах.
	c.Pad, c.Style.Size = math.NaN(), 1000
	e.SetSheetCaps(s0, true, c)
	if got := edtest.P(e).Spreads[0].Caps; got.Pad != model.DefaultPad || got.Style.Size > 200 {
		t.Fatalf("зажим: %+v", got)
	}
}

// Поля шапки правит окно виньетки (031 Т2, SetupVignette); здесь — имя на фото.
func TestSetPerson(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo := edtest.UndoLen(e)
	e.SetPerson("a", model.Person{Last: "И"}, "p1")
	e.SetPerson("a", model.Person{Last: " Иванов ", First: "Петя"}, "p1")
	if pv, ok := e.Person("a"); !ok || pv.Last != "Иванов" || pv.FromFile || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("имя: %+v, шагов %d", pv, edtest.UndoLen(e)-undo)
	}
	if e.SetPerson("a", model.Person{}, "p2"); edtest.P(e).Photos["a"].Person != nil {
		t.Fatal("пустое имя — снова из файла")
	}
	if _, ok := e.Person("нет"); ok {
		t.Fatal("чужое фото")
	}
}
