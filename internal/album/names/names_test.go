package names_test

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// paste — вставка списка (030) через окно 043: итог в прежнем виде (лишних, без имени).
func paste(e *album.Editor, spreadID, text string, firstFirst bool) (model.View, int, int, error) {
	v, plan, err := e.PasteNames(spreadID, model.NamesOpts{Text: text, FirstFirst: firstFirst})
	return v, plan.Extra, plan.Missing, err
}

func TestPasteNames(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 50, "a"), edtest.Gp(70, 10, 50, 50, ""),
		edtest.Gp(130, 10, 50, 50, "b"), edtest.Gp(10, 70, 50, 50, "c"), edtest.Gp(70, 70, 50, 50, "d")})
	undo := edtest.UndoLen(e)
	v, extra, missing, err := paste(e, "s1", "1. Сидоров Иван\r\n\n2) Петрова\tАнна\tСергеевна\nКозлов Пётр", false)
	if err != nil || extra != 0 || missing != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("вставка: лишних %d, без имени %d, err %v, шагов %d", extra, missing, err, edtest.UndoLen(e)-undo)
	}
	// Порядок мест, и пустое (071): таблички мест a, пустого и b; c и d — без имени. Имена — на
	// местах, своих имён фото не получают.
	want := []model.Person{{Last: "Сидоров", First: "Иван"}, {Last: "Петрова", First: "Анна", Middle: "Сергеевна"},
		{Last: "Козлов", First: "Пётр"}, {}, {}}
	for i, w := range want {
		if got := seatOf(e, i); got != w {
			t.Fatalf("место %d: %+v, ждали %+v", i, got, w)
		}
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if edtest.P(e).Photos[id].Person != nil {
			t.Fatalf("%s получило своё имя: %+v", id, edtest.P(e).Photos[id].Person)
		}
	}
	if seen(e, "a") != "Сидоров Иван" || seen(e, "b") != "Козлов Пётр" || seen(e, "d") != "" || v.Rev != edtest.Rev(e) {
		t.Fatalf("видно: a %q, b %q, d %q", seen(e, "a"), seen(e, "b"), seen(e, "d"))
	}
	// Та же вставка — холостая: ни шага, ни rev.
	rev := edtest.Rev(e)
	if _, _, _, err := paste(e, "s1", "Сидоров Иван\nПетрова Анна Сергеевна\nКозлов Пётр", false); err != nil || edtest.Rev(e) != rev {
		t.Fatalf("повтор: err %v, rev %d → %d", err, rev, edtest.Rev(e))
	}
	// Имя первым и лишние строки: пять мест — пять строк, шестая лишняя.
	_, extra, missing, _ = paste(e, "s1", "Иван Сидоров\nАнна Сергеевна Петрова\nПётр Козлов\nОля Иванова\nМаша Орлова\nЛишний Человек", true)
	if extra != 1 || missing != 0 || seatOf(e, 1) != want[1] || seen(e, "d") != "Орлова Маша" {
		t.Fatalf("имя первым: лишних %d, без имени %d, место 1 %+v, d %q", extra, missing, seatOf(e, 1), seen(e, "d"))
	}
	// Отмена — одним шагом.
	e.Undo()
	if seatOf(e, 4) != (model.Person{}) || seatOf(e, 0) != want[0] {
		t.Fatal("отмена вставки")
	}
	// Слишком длинный список и чужой разворот — ошибка без правки.
	rev = edtest.Rev(e)
	if _, _, _, err := paste(e, "s1", strings.Repeat("Иванов Петя\n", 1001), false); err == nil || edtest.Rev(e) != rev {
		t.Fatal("1001 строка принята")
	}
	if _, _, _, err := paste(e, "нет", "Иванов Петя", false); err == nil || edtest.Rev(e) != rev {
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

// caption — подпись фото id в виде листа: текст и поля имени; нет подписи — ok = false.
func caption(v model.View, id string) (text string, who []string, ok bool) {
	for _, c := range v.Spreads[0].Captions {
		if c.Photo == id {
			return c.Text, c.Who, true
		}
	}
	return "", nil, false
}

// 049: предмет учителя — карточка (Teacher по месту виньетки), набор (сессия — один шаг,
// повтор — холостой, отмена), подпись StaffCaption, столбец «Предмет» окна «Имена».
func TestPersonSubjectCard(t *testing.T) {
	e, sid, teacher, pupils := vigEd(t)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 3, TText: model.StaffCaption}, nil); err != nil {
		t.Fatal(err)
	}
	if pv, _ := e.Person(teacher); !pv.Teacher || pv.Subject != "" {
		t.Fatalf("учитель: %+v", pv)
	}
	for _, id := range pupils {
		if pv, _ := e.Person(id); pv.Teacher {
			t.Fatalf("ученик %s на месте учителя", id)
		}
	}
	undo := edtest.UndoLen(e)
	e.SetPerson(teacher, model.Person{Last: "Сидорова", First: "Ольга", Subject: "Мат"}, "s1")
	e.SetPerson(teacher, model.Person{Last: "Сидорова", First: "Ольга", Subject: " Математика\n"}, "s1")
	if pv, _ := e.Person(teacher); pv.Subject != "Математика" || !pv.Teacher || pv.FromFile || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("набор предмета: %+v, шагов %d", pv, edtest.UndoLen(e)-undo)
	}
	rev := edtest.Rev(e)
	if e.SetPerson(teacher, model.Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}, "s2"); edtest.Rev(e) != rev || edtest.UndoLen(e) != undo+1 {
		t.Fatal("тот же предмет — не холостая правка")
	}
	if txt, who, ok := caption(e.View(), teacher); !ok || !strings.HasSuffix(txt, "\nМатематика") || len(who) != 4 || who[3] != "subject" {
		t.Fatalf("подпись учителя: %q %v %v", txt, who, ok)
	}
	if e.Undo(); edtest.P(e).Photos[teacher].Person != nil {
		t.Fatalf("отмена: %+v", edtest.P(e).Photos[teacher].Person)
	}

	// Свой только предмет: имя из названия файла остаётся, предмет — свой.
	e2, _ := fileEd(t, "Иванова Мария.jpg")
	e2.SetPerson("p1", model.Person{Subject: "Химия"}, "x")
	if pv, _ := e2.Person("p1"); pv.Last != "Иванова" || !pv.FromFile || pv.Subject != "Химия" || pv.Teacher {
		t.Fatalf("предмет к имени из файла: %+v", pv)
	}
}

// 049 (пересмотр 0.12 №8): столбец «Предмет» пишет предмет учителю, ученику — пусто;
// повтор — холостой; список без столбца предмет снимает (список — источник имени целиком).
func TestNamesSubject(t *testing.T) {
	e, sid, teacher, pupils := vigEd(t)
	// Предмет без имени (фото «v1.jpg») — место без имени: Missing его считает.
	e.SetPerson(teacher, model.Person{Subject: "Музыка"}, "x")
	if plan, err := e.PreviewNames(sid, model.NamesOpts{}); err != nil || plan.Named != 0 || plan.Missing != plan.Seats {
		t.Fatalf("предмет без имени считается именем: %+v %v", plan, err)
	}
	text := "Фамилия\tИмя\tПредмет\nСидорова\tОльга\tФизика\nКозлов\tИван\nОрлов\tОлег\nПетрова\tАнна\n"
	if _, _, err := e.PasteNames(sid, model.NamesOpts{Text: text}); err != nil {
		t.Fatal(err)
	}
	// Имена на виньетке — таблички мест (071): видно табличку, свой предмет фото снят.
	tv, _ := e.Person(teacher)
	if pv, _ := e.Person(pupils[0]); tv.Subject != "Физика" || tv.Place == "" || pv.Subject != "" || pv.Place == "" ||
		edtest.P(e).Photos[teacher].Person != nil {
		t.Fatalf("из списка: учитель %+v, ученик %+v, своё %+v", tv, pv, edtest.P(e).Photos[teacher].Person)
	}
	rev := edtest.Rev(e)
	if _, _, _ = e.PasteNames(sid, model.NamesOpts{Text: text}); edtest.Rev(e) != rev {
		t.Fatal("повтор списка с предметом — не холостой")
	}
	if pv, _ := e.Person(teacher); pv.Subject != "Физика" || !pv.Teacher {
		t.Fatalf("карточка: %+v", pv)
	}
	plain := "Фамилия\tИмя\nСидорова\tОльга\nКозлов\tИван\nОрлов\tОлег\nПетрова\tАнна\n"
	if _, _, _ = e.PasteNames(sid, model.NamesOpts{Text: plain}); seenSubject(e, teacher) != "" {
		t.Fatalf("список без предмета: %q", seenSubject(e, teacher))
	}
	if e.Undo(); seenSubject(e, teacher) != "Физика" {
		t.Fatal("отмена вставки не вернула предмет")
	}
}
