package names_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// 071 G4: окно «Имена» на листе с местами — строка на каждое место (и пустое), имя из списка
// ложится табличкой места; «Имена файлов» и столбец «Файл» — в фото, табличка места снимается.

// seen — видимое имя фото id (табличка его места, своё или из названия файла), как в карточке.
func seen(e *album.Editor, id string) string {
	pv, _ := e.Person(id)
	return strings.TrimSpace(pv.Last + " " + pv.First + " " + pv.Middle)
}

// emptyRows — строки плана у пустых мест.
func emptyRows(plan model.NamesPlan) []model.NamesRow {
	var out []model.NamesRow
	for _, row := range plan.Rows {
		if row.Photo == "" {
			out = append(out, row)
		}
	}
	return out
}

// seenSubject —видимый предмет фото id.
func seenSubject(e *album.Editor, id string) string {
	pv, _ := e.Person(id)
	return pv.Subject
}

// emptyVig —виньетка head-t-left (1 учитель, 3 ученика) без фото; места по порядку сетки.
func emptyVig(t *testing.T) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 3}, nil); err != nil {
		t.Fatal(err)
	}
	return e, sid
}

// roleSeats — таблички мест листа 0 по ролям: учитель и ученики по порядку сетки.
func roleSeats(e *album.Editor) (teacher model.Person, pupils []model.Person) {
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if model.RoleOf(pl) == model.RoleTeacher {
			teacher = pl.Person
		} else {
			pupils = append(pupils, pl.Person)
		}
	}
	return teacher, pupils
}

// Пустая виньетка (сц. 1): строка на каждое место, учитель из списка — на место учителя, ученики
// — на места учеников; имена — таблички, на холсте вместо «[Фамилия]»; один шаг, повтор холостой,
// отмена снимает все таблички; круг Save → Load их хранит.
func TestPasteNamesEmptySeats(t *testing.T) {
	e, sid := emptyVig(t)
	o := model.NamesOpts{Text: "Фамилия\tИмя\tПредмет\nИванов\tПетя\nСидорова\tОльга\tМатематика\nПетров\tВаня\nКозлов\tКоля"}
	pre, err := e.PreviewNames(sid, o)
	if err != nil || len(pre.Rows) != 4 || pre.Seats != 4 || pre.Named != 4 || pre.Missing != 0 || pre.Extra != 0 || pre.NeedCaption {
		t.Fatalf("предпросмотр: %+v, %v", pre, err)
	}
	for _, row := range pre.Rows {
		if row.Place == "" || row.Photo != "" || !row.Seat || row.Who == nil {
			t.Fatalf("строка пустого места: %+v", row)
		}
	}
	undo := edtest.UndoLen(e)
	v, plan, err := e.PasteNames(sid, o)
	if err != nil || !reflect.DeepEqual(pre, plan) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("вставка: %v, шагов %d, план %+v", err, edtest.UndoLen(e)-undo, plan)
	}
	teacher, pupils := roleSeats(e)
	want := []model.Person{{Last: "Иванов", First: "Петя"}, {Last: "Петров", First: "Ваня"}, {Last: "Козлов", First: "Коля"}}
	if teacher != (model.Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}) || !reflect.DeepEqual(pupils, want) {
		t.Fatalf("таблички: учитель %+v, ученики %+v", teacher, pupils)
	}
	if got := stubAt(v, edtest.P(e).Spreads[0].Grid[0].ID); !strings.Contains(got, "Иванов") {
		t.Fatalf("заглушка первого места: %q", got)
	}
	if len(edtest.P(e).Photos) != 0 {
		t.Fatalf("фото появились: %v", edtest.P(e).Photos)
	}
	rev := edtest.Rev(e)
	e.PasteNames(sid, o)
	edtest.Same(t, e, rev, undo+1, "повтор вставки")
	// Круг Save → Load: таблички те же.
	q := edtest.SaveLoad(t, e)
	for i, pl := range q.Spreads[0].Grid {
		if pl.Person != edtest.P(e).Spreads[0].Grid[i].Person {
			t.Fatalf("место %d после чтения: %+v", i, pl.Person)
		}
	}
	e.Undo()
	if teacher, pupils = roleSeats(e); teacher != (model.Person{}) || !reflect.DeepEqual(pupils, make([]model.Person, 3)) {
		t.Fatalf("отмена: %+v %+v", teacher, pupils)
	}
}

// Список на виньетку с фото (сц. 8, 071-4): имена — таблички мест, свои имена фото на этих местах
// сняты целиком (и предмет: список — источник имени, 0.12 №175), видно имя из списка; места,
// которым строк не хватило, не тронуты — ни табличка, ни своё имя.
func TestPasteNamesOccupiedSeats(t *testing.T) {
	e, sid, teacher, pupils := vigEd(t)
	e.SetPerson(teacher, model.Person{Last: "Свой", First: "Учитель", Subject: "Пение"}, "")
	e.SetPerson(pupils[0], model.Person{Last: "Свой", First: "Ученик"}, "")
	e.SetPerson(pupils[2], model.Person{Last: "Третий", First: "Свой"}, "")
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: "Козлов Иван\nОрлов Олег"})
	if err != nil || plan.Seats != 4 || plan.Named != 4 || plan.Missing != 0 || plan.Extra != 0 {
		t.Fatalf("вставка: %+v, %v", plan, err)
	}
	if seen(e, pupils[0]) != "Козлов Иван" || seen(e, pupils[1]) != "Орлов Олег" || person(e, pupils[0]) != "" ||
		edtest.P(e).Photos[pupils[0]].Person != nil {
		t.Fatalf("ученики: %q %q, своё %+v", seen(e, pupils[0]), seen(e, pupils[1]), edtest.P(e).Photos[pupils[0]].Person)
	}
	// Третьему ученику и учителю строк не хватило: своё имя и предмет на месте, табличек нет.
	if seen(e, pupils[2]) != "Третий Свой" || seen(e, teacher) != "Свой Учитель" {
		t.Fatalf("без строки: %q, учитель %q", seen(e, pupils[2]), seen(e, teacher))
	}
	if _, pl := edtest.P(e).PlaceOf(teacher); pl == nil || pl.Person != (model.Person{}) {
		t.Fatalf("табличка учителя: %+v", pl)
	}
	// Список без столбца «Предмет», учитель — четвёртой строкой по порядку мест: своё имя и
	// предмет учителя сняты и под табличкой без предмета не проступают.
	e.PasteNames(sid, model.NamesOpts{Text: "Козлов Иван\nОрлов Олег\nПетров Ваня\nСидорова Ольга"})
	if pv, _ := e.Person(teacher); pv.Last != "Сидорова" || pv.Subject != "" || pv.Place == "" || edtest.P(e).Photos[teacher].Person != nil {
		t.Fatalf("учитель: %+v, своё %+v", pv, edtest.P(e).Photos[teacher].Person)
	}
}

// «Имена файлов» на листе с местами (сц. 9, 071-5): имя из названия — в фото, табличка его места
// снята; фото без имени в названии и пустое место таблички хранят; строки — не таблички.
func TestPasteNamesFilesClearsSeat(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "Иванов Петя.jpg"), named("b", "DSC_1.jpg")}, "")
	for i, who := range []model.Person{petrov, sidorov, {Last: "Орлова", First: "Оля"}} {
		setSeat(e, i, who)
	}
	_, plan, err := e.PasteNames("s1", model.NamesOpts{From: "files"})
	if err != nil || len(plan.Rows) != 3 || plan.Seats != 3 || plan.Named != 3 || plan.Missing != 0 {
		t.Fatalf("вставка: %+v, %v", plan, err)
	}
	for _, row := range plan.Rows {
		if row.Seat || row.Place == "" {
			t.Fatalf("строка «Имена файлов» — табличка: %+v", row)
		}
	}
	if seatOf(e, 0) != (model.Person{}) || person(e, "a") != "Иванов Петя" || seen(e, "a") != "Иванов Петя" {
		t.Fatalf("фото с именем в названии: табличка %+v, имя %q", seatOf(e, 0), person(e, "a"))
	}
	if seatOf(e, 1) != sidorov || person(e, "b") != "" || seatOf(e, 2).Last != "Орлова" {
		t.Fatalf("без имени в названии и пустое: %+v %q %+v", seatOf(e, 1), person(e, "b"), seatOf(e, 2))
	}
}

// Столбец «Файл» на листе с местами (сц. 9): найденное фото получает имя, табличка его места
// снята; соседние таблички и пустое место не тронуты.
func TestPasteNamesByFileClearsSeat(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	for i, who := range []model.Person{petrov, sidorov, {Last: "Орлова"}} {
		setSeat(e, i, who)
	}
	_, plan, err := e.PasteNames("s1", model.NamesOpts{Text: "Файл\tФИО\nDSC_2.jpg\tКозлов Иван\nDSC_9.jpg\tНет Такого"})
	if err != nil || plan.Extra != 1 || plan.Left[0].Reason != "nofile" || plan.Seats != 3 || plan.Named != 3 {
		t.Fatalf("вставка: %+v, %v", plan, err)
	}
	if seatOf(e, 1) != (model.Person{}) || person(e, "b") != "Козлов Иван" || seen(e, "b") != "Козлов Иван" {
		t.Fatalf("найденное фото: табличка %+v, имя %q", seatOf(e, 1), person(e, "b"))
	}
	if seatOf(e, 0) != petrov || person(e, "a") != "" || seatOf(e, 2).Last != "Орлова" {
		t.Fatalf("соседи: %+v %q %+v", seatOf(e, 0), person(e, "a"), seatOf(e, 2))
	}
}

// Лист «Авто» (как 043): строки по фото без мест, имена — в фото, табличек нет.
func TestPasteNamesAutoSheet(t *testing.T) {
	e, sid := fileEd(t, "a.jpg", "b.jpg", "c.jpg")
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: "Козлов Иван\nОрлов Олег"})
	if err != nil || len(plan.Rows) != 3 || plan.Seats != 3 || plan.Named != 2 || plan.Missing != 1 || !plan.NeedCaption {
		t.Fatalf("вставка: %+v, %v", plan, err)
	}
	for _, row := range plan.Rows {
		if row.Place != "" || row.Seat || row.Photo == "" {
			t.Fatalf("строка «Авто»: %+v", row)
		}
	}
	if person(e, "p1") != "Козлов Иван" || person(e, "p2") != "Орлов Олег" || person(e, "p3") != "" {
		t.Fatalf("имена: %q %q %q", person(e, "p1"), person(e, "p2"), person(e, "p3"))
	}
}

// «Авто» без фото и мест (071-11): строк нет, весь список — лишние, флажок подписи не нужен;
// вставка — холостая и с флажком (подписывать нечего).
func TestPlanNoPlaces(t *testing.T) {
	e := edtest.Open(t, model.NewSheet(204, 304), "")
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 0 || len(s.Photos) != 0 {
		t.Fatalf("лист не пустой: %+v", s)
	}
	o := model.NamesOpts{Text: "Иванов Петя\nПетров Ваня", Caption: true}
	pre, err := e.PreviewNames(s.ID, o)
	if err != nil || len(pre.Rows) != 0 || pre.Seats != 0 || pre.Named != 0 || pre.Extra != 2 || len(pre.Left) != 2 ||
		pre.Left[0].Reason != "extra" || pre.NeedCaption {
		t.Fatalf("предпросмотр: %+v, %v", pre, err)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err = e.PasteNames(s.ID, o); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "вставка на лист без мест")
	if edtest.P(e).Spreads[0].Caps != nil {
		t.Fatal("шаблон подписи на листе без мест")
	}
}
