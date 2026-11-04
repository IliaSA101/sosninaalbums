package names_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// 071: табличка места — правка (SetPlacePerson, SetPerson на месте с табличкой), карточка и
// действия с фото, которые таблички не трогают (страж: кода в них нет).

// seatID — ID места i листа s1.
func seatID(e *album.Editor, i int) string { return edtest.P(e).Spreads[0].Grid[i].ID }

// seatOf — табличка места i листа s1.
func seatOf(e *album.Editor, i int) model.Person { return edtest.P(e).Spreads[0].Grid[i].Person }

// setSeat — табличка места i листа s1 отдельным шагом.
func setSeat(e *album.Editor, i int, who model.Person) model.View {
	return e.SetPlacePerson("s1", seatID(e, i), who, "")
}

// shown — что видно под фото id листа s1 (подпись или заглушка); "" — ничего.
func shown(v model.View, id string) string {
	for _, cv := range v.Spreads[0].Captions {
		if cv.Photo == id {
			return cv.Text
		}
	}
	for _, st := range v.Spreads[0].Stubs {
		if st.Photo == id {
			return st.Text
		}
	}
	return ""
}

// stubAt — текст заглушки пустого места placeID листа s1; "" — заглушки нет.
func stubAt(v model.View, placeID string) string {
	for _, st := range v.Spreads[0].Stubs {
		if st.Place == placeID && st.Photo == "" {
			return st.Text
		}
	}
	return ""
}

var (
	petrov  = model.Person{Last: "Петров", First: "Ваня"}
	sidorov = model.Person{Last: "Сидоров", First: "Коля"}
)

// Табличка места: набор в поле — одна сессия, один шаг; другая сессия — новый шаг; то же имя
// — холостая; пусто — снять; чужое место и лист — холостая; зажим как у имени фото.
func TestSetPlacePersonSession(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	undo := edtest.UndoLen(e)
	e.SetPlacePerson("s1", seatID(e, 0), model.Person{Last: "Пет"}, "n1")
	v := e.SetPlacePerson("s1", seatID(e, 0), petrov, "n1")
	if seatOf(e, 0) != petrov || edtest.UndoLen(e) != undo+1 || stubAt(v, seatID(e, 0)) != "Петров Ваня" {
		t.Fatalf("сессия: %+v, шагов %d, заглушка %q", seatOf(e, 0), edtest.UndoLen(e)-undo, stubAt(v, seatID(e, 0)))
	}
	// Несколько мест: у каждого своя табличка, соседние не задеты.
	e.SetPlacePerson("s1", seatID(e, 2), sidorov, "n2")
	if seatOf(e, 2) != sidorov || seatOf(e, 1) != (model.Person{}) || seatOf(e, 0) != petrov || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("второе место: %+v %+v %+v", seatOf(e, 0), seatOf(e, 1), seatOf(e, 2))
	}
	rev, undo2 := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlacePerson("s1", seatID(e, 0), petrov, "n3")
	e.SetPlacePerson("s1", "нет", petrov, "")
	e.SetPlacePerson("нет", seatID(e, 0), sidorov, "")
	e.SetPlacePerson("s1", "", sidorov, "")
	edtest.Same(t, e, rev, undo2, "холостые")
	// Зажим: управляющие символы — пробел, длина — MaxPersonRunes; пробелы — пусто.
	e.SetPlacePerson("s1", seatID(e, 1), model.Person{Last: strings.Repeat("Ж", 100), First: "А\x00\nБ", Middle: "  "}, "")
	if got := seatOf(e, 1); utf8.RuneCountInString(got.Last) != model.MaxPersonRunes || got.First != "А Б" || got.Middle != "" {
		t.Fatalf("зажим: %+v", got)
	}
	// Все поля пусты — табличка снята, снова «[Фамилия]».
	v = e.SetPlacePerson("s1", seatID(e, 0), model.Person{Last: " "}, "")
	if seatOf(e, 0) != (model.Person{}) || stubAt(v, seatID(e, 0)) != "[Фамилия] [Имя]" {
		t.Fatalf("снять: %+v, %q", seatOf(e, 0), stubAt(v, seatID(e, 0)))
	}
	// Отмена — по шагу назад: табличка вернулась.
	e.Undo()
	if seatOf(e, 0) != petrov {
		t.Fatalf("отмена снятия: %+v", seatOf(e, 0))
	}
}

// Имя фото на месте с табличкой правит табличку (её и видно): фото своего имени не получает;
// фото на месте без таблички и на «Авто» — как 030, имя у фото.
func TestSetPersonRoutesToSeat(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	setSeat(e, 0, petrov)
	undo := edtest.UndoLen(e)
	e.SetPerson("a", model.Person{Last: "Орлов"}, "p1")
	v := e.SetPerson("a", model.Person{Last: "Орлов", First: "Олег"}, "p1")
	if seatOf(e, 0) != (model.Person{Last: "Орлов", First: "Олег"}) || edtest.P(e).Photos["a"].Person != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("на месте с табличкой: табличка %+v, у фото %+v, шагов %d", seatOf(e, 0), edtest.P(e).Photos["a"].Person, edtest.UndoLen(e)-undo)
	}
	if shown(v, "a") != "Орлов Олег" {
		t.Fatalf("подпись: %q", shown(v, "a"))
	}
	e.SetPerson("b", sidorov, "")
	if seatOf(e, 1) != (model.Person{}) || person(e, "b") != "Сидоров Коля" {
		t.Fatalf("без таблички: табличка %+v, у фото %q", seatOf(e, 1), person(e, "b"))
	}
	e.SetPerson("x", sidorov, "") // «Авто» s2
	if person(e, "x") != "Сидоров Коля" {
		t.Fatalf("«Авто»: %q", person(e, "x"))
	}
	// То же имя ещё раз — холостая.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPerson("a", model.Person{Last: "Орлов", First: "Олег"}, "")
	edtest.Same(t, e, rev, undo, "то же имя")
}

// Все поля очищены — табличка снята: снова своё имя фото, а без него — из имени файла.
func TestSetPersonClearsSeat(t *testing.T) {
	e := capsEd(t, 2, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "Иванов Петя.jpg"), named("b", "DSC_2.jpg")}, "")
	e.SetPerson("b", model.Person{Last: "Козлов", First: "Олег", Subject: "Химия"}, "")
	setSeat(e, 0, petrov)
	setSeat(e, 1, sidorov)
	v := e.View()
	if shown(v, "a") != "Петров Ваня" || shown(v, "b") != "Сидоров Коля" {
		t.Fatalf("таблички: %q, %q", shown(v, "a"), shown(v, "b"))
	}
	e.SetPerson("a", model.Person{}, "")
	v = e.SetPerson("b", model.Person{}, "")
	if seatOf(e, 0) != (model.Person{}) || seatOf(e, 1) != (model.Person{}) {
		t.Fatalf("не сняты: %+v, %+v", seatOf(e, 0), seatOf(e, 1))
	}
	if shown(v, "a") != "Иванов Петя" || shown(v, "b") != "Козлов Олег" || person(e, "b") != "Козлов Олег" {
		t.Fatalf("после снятия: %q, %q", shown(v, "a"), shown(v, "b"))
	}
	if pv, _ := e.Person("a"); !pv.FromFile || pv.Place != "" {
		t.Fatalf("карточка: %+v", pv)
	}
}

// Карточка фото: табличка с именем — Place места, FromFile нет; табличка только с предметом —
// имя фото с её предметом, из файла — с пометкой (071-16); без таблички — как 030.
func TestPersonViewSeat(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "Иванов Петя.jpg"), named("b", "Козлов Олег.jpg"), named("c", "Орлов Иван.jpg")}, "")
	setSeat(e, 0, petrov)
	setSeat(e, 1, model.Person{Subject: "Физика"})
	want := map[string]model.PersonView{
		"a": {Last: "Петров", First: "Ваня", Place: seatID(e, 0), SeatNamed: true},
		"b": {Last: "Козлов", First: "Олег", Subject: "Физика", FromFile: true, Place: seatID(e, 1)},
		"c": {Last: "Орлов", First: "Иван", FromFile: true},
	}
	for id, w := range want {
		if pv, ok := e.Person(id); !ok || pv != w {
			t.Errorf("%s: %+v, ждали %+v", id, pv, w)
		}
	}
	if _, ok := e.Person("нет"); ok {
		t.Fatal("фото не из альбома")
	}
}

// 071-16: табличка только с предметом — имя в карточке от фото (из файла — с пометкой), правка
// пишет части имени в фото, предмет — в табличку; обмен уносит имена с фото, предметы стоят на
// местах; очистка снимает табличку и своё имя фото. Скрытый свой предмет фото не всплывает.
func TestSubjectSeatNameToPhoto(t *testing.T) {
	e := capsEd(t, 2, nameTpl+"\n{Предмет}")
	e.FillPlaces([]model.PhotoInfo{named("a", "Сидорова Ольга.jpg"), named("b", "Иванов Пётр.jpg")}, "")
	e.SetPerson("a", model.Person{Subject: "Музыка"}, "") // свой предмет — скроется под табличкой
	setSeat(e, 0, model.Person{Subject: "Математика"})
	setSeat(e, 1, model.Person{Subject: "Физика"})
	if pv, _ := e.Person("a"); pv != (model.PersonView{Last: "Сидорова", First: "Ольга", Subject: "Математика", FromFile: true, Place: seatID(e, 0)}) {
		t.Fatalf("карточка: %+v", pv)
	}
	undo := edtest.UndoLen(e)
	e.SetPerson("a", model.Person{Subject: "Алг"}, "s1") // имя из файла: карточка шлёт только предмет
	e.SetPerson("a", model.Person{Subject: "Алгебра"}, "s1")
	e.SetPerson("b", model.Person{Last: "Иванов", First: "Пётр", Subject: "Химия"}, "s2") // всё видимое
	if seatOf(e, 0) != (model.Person{Subject: "Алгебра"}) || seatOf(e, 1) != (model.Person{Subject: "Химия"}) || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("таблички: %+v, %+v, шагов %d", seatOf(e, 0), seatOf(e, 1), edtest.UndoLen(e)-undo)
	}
	pa, pb := edtest.P(e).Photos["a"].Person, edtest.P(e).Photos["b"].Person
	if pa != nil || pb == nil || *pb != (model.Person{Last: "Иванов", First: "Пётр"}) {
		t.Fatalf("имена фото: a %+v, b %+v", pa, pb)
	}
	if pv, _ := e.Person("a"); !pv.FromFile || pv.Subject != "Алгебра" {
		t.Fatalf("карточка a после правки: %+v", pv)
	}
	if pv, _ := e.Person("b"); pv.FromFile || pv.Last != "Иванов" || pv.Subject != "Химия" {
		t.Fatalf("карточка b: %+v", pv)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPerson("a", model.Person{Subject: "Алгебра"}, "")
	e.SetPerson("b", model.Person{Last: "Иванов", First: "Пётр", Subject: "Химия"}, "")
	edtest.Same(t, e, rev, undo, "то же имя")
	// Круг Save → Load: предмет — у места, имя — у фото.
	q := edtest.SaveLoad(t, e)
	if g := q.Spreads[0].Grid; g[0].Person != (model.Person{Subject: "Алгебра"}) || q.Photos[g[0].Photo].Person != nil ||
		g[1].Person != (model.Person{Subject: "Химия"}) || q.Photos[g[1].Photo].Person == nil || q.Photos[g[1].Photo].Person.Last != "Иванов" {
		t.Fatalf("Save → Load: %+v %+v", *g[0], *g[1])
	}
	// Обмен: имя из файла не застряло на месте учителя.
	v := e.SwapPhotos("a", "b")
	if shown(v, "b") != "Иванов Пётр\nАлгебра" || shown(v, "a") != "Сидорова Ольга\nХимия" {
		t.Fatalf("после обмена: b %q, a %q", shown(v, "b"), shown(v, "a"))
	}
	e.Undo()
	// Очистка: табличка снята, имя снова из файла, предмета нет (свой «Музыка» не всплыл).
	v = e.SetPerson("a", model.Person{}, "")
	if seatOf(e, 0) != (model.Person{}) || edtest.P(e).Photos["a"].Person != nil || shown(v, "a") != "Сидорова Ольга\n[Предмет]" {
		t.Fatalf("очистка: табличка %+v, подпись %q", seatOf(e, 0), shown(v, "a"))
	}
	if pv, _ := e.Person("a"); pv != (model.PersonView{Last: "Сидорова", First: "Ольга", FromFile: true}) {
		t.Fatalf("карточка после очистки: %+v", pv)
	}
	if e.Undo(); seatOf(e, 0) != (model.Person{Subject: "Алгебра"}) {
		t.Fatalf("отмена очистки: %+v", seatOf(e, 0))
	}
}

// Табличка места для карточки «Место» и окошка «Имя»: учитель виньетки — Teacher (поле
// «Предмет»), предмет без имени допустим; место без таблички — пусто без Place; нет места — ok false.
func TestPlacePersonTeacher(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		s.Kind, s.Vig = model.KindVignette, &model.Vignette{TText: "{Фамилия} {Имя}\n{Предмет}"}
		s.Grid[0].Role = model.RoleTeacher
	})
	tid := seatID(e, 0)
	v := e.SetPlacePerson("s1", tid, model.Person{Subject: "Математика"}, "")
	if pv, ok := e.PlacePerson("s1", tid); !ok || pv != (model.PersonView{Subject: "Математика", Teacher: true, Place: tid}) {
		t.Fatalf("учитель: %+v %v", pv, ok)
	}
	if got := stubAt(v, tid); got != "[Фамилия] [Имя]\nМатематика" {
		t.Fatalf("заглушка учителя с предметом: %q", got)
	}
	// Окошко «Имя» пустого места учителя открывает и поле «Предмет»: {Предмет} есть в шаблоне.
	i := slices.IndexFunc(v.Spreads[0].Stubs, func(st model.StubView) bool { return st.Place == tid && st.Photo == "" })
	if i < 0 || !slices.Equal(v.Spreads[0].Stubs[i].Who, []string{"last", "first", "subject"}) {
		t.Fatalf("поля заглушки учителя: %d %+v", i, v.Spreads[0].Stubs)
	}
	setSeat(e, 1, petrov)
	if pv, ok := e.PlacePerson("s1", seatID(e, 1)); !ok || pv != (model.PersonView{Last: "Петров", First: "Ваня", Place: seatID(e, 1), SeatNamed: true}) {
		t.Fatalf("ученик: %+v", pv)
	}
	if pv, ok := e.PlacePerson("s1", seatID(e, 2)); !ok || pv != (model.PersonView{}) {
		t.Fatalf("без таблички: %+v", pv)
	}
	for _, c := range [][2]string{{"s1", "нет"}, {"нет", tid}, {"s2", tid}, {"s1", ""}} {
		if pv, ok := e.PlacePerson(c[0], c[1]); ok || pv != (model.PersonView{}) {
			t.Fatalf("%v: %+v %v", c, pv, ok)
		}
	}
}

// Обмен фото (перетаскивание на занятое место): таблички остаются на местах, оба фото
// подписаны табличками своих новых мест. Отмена — шагом.
func TestSwapKeepsSeatNames(t *testing.T) {
	e := capsEd(t, 2, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	setSeat(e, 0, petrov)
	setSeat(e, 1, sidorov)
	v := e.SwapPhotos("a", "b")
	g := edtest.P(e).Spreads[0].Grid
	if g[0].Photo != "b" || g[0].Person != petrov || g[1].Photo != "a" || g[1].Person != sidorov {
		t.Fatalf("обмен: %+v %+v", *g[0], *g[1])
	}
	if shown(v, "b") != "Петров Ваня" || shown(v, "a") != "Сидоров Коля" {
		t.Fatalf("подписи: b %q, a %q", shown(v, "b"), shown(v, "a"))
	}
	v = e.Undo()
	if shown(v, "a") != "Петров Ваня" {
		t.Fatalf("отмена: %q", shown(v, "a"))
	}
}

// «Убрать фото» и Delete: место пустое, табличка ждёт следующее фото (бледно на холсте).
func TestRemoveKeepsSeatName(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg"), named("c", "DSC_3.jpg")}, "")
	setSeat(e, 0, petrov)
	setSeat(e, 1, sidorov)
	e.RemovePhotos([]string{"a"})
	v := e.DeleteSelection("s1", []string{"b"}, nil, nil)
	if _, ok := edtest.P(e).Photos["b"]; ok {
		t.Fatal("Delete не удалил фото")
	}
	for i, w := range []model.Person{petrov, sidorov} {
		if pl := edtest.P(e).Spreads[0].Grid[i]; pl.Photo != "" || pl.Person != w {
			t.Fatalf("место %d: %+v", i, *pl)
		}
	}
	if stubAt(v, seatID(e, 0)) != "Петров Ваня" || stubAt(v, seatID(e, 1)) != "Сидоров Коля" || v.Spreads[0].Waiting != 2 {
		t.Fatalf("заглушки: %q, %q, ждут %d", stubAt(v, seatID(e, 0)), stubAt(v, seatID(e, 1)), v.Spreads[0].Waiting)
	}
	// Удалить место — его табличка пропадает вместе с ним.
	e.DeleteSelection("s1", nil, []string{seatID(e, 0)}, nil)
	if g := edtest.P(e).Spreads[0].Grid; len(g) != 2 || g[0].Person != sidorov {
		t.Fatalf("удаление места: %d мест, первое %+v", len(g), g[0].Person)
	}
}

// Бросок фото из ленты на занятое место с табличкой (замена): новое фото подписано табличкой,
// прежнее ушло в ленту без её имени.
func TestReplaceTakesSeatName(t *testing.T) {
	e := capsEd(t, 2, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "Иванов Петя.jpg")}, "")
	setSeat(e, 0, petrov)
	v, _, err := e.DropPhotos("s1", seatID(e, 0), []model.PhotoInfo{named("n", "DSC_9.jpg")}, -1, "")
	if err != nil || edtest.P(e).Spreads[0].Grid[0].Photo != "n" || shown(v, "n") != "Петров Ваня" {
		t.Fatalf("замена: %v, место %+v, подпись %q", err, *edtest.P(e).Spreads[0].Grid[0], shown(v, "n"))
	}
	if edtest.P(e).Placed("a") || seatOf(e, 0) != petrov {
		t.Fatalf("прежнее фото на листе %v, табличка %+v", edtest.P(e).Placed("a"), seatOf(e, 0))
	}
}

// «Заполнить» и бросок нескольких фото на лист: фото встают по порядку пустых мест и подписаны
// их табличками; место без таблички — именем фото.
func TestFillTakesSeatNames(t *testing.T) {
	e := capsEd(t, 4, nameTpl)
	setSeat(e, 0, petrov)
	setSeat(e, 1, sidorov)
	setSeat(e, 3, model.Person{Last: "Орлова", First: "Маша"})
	_, left, _ := e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	v, _, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("c", "Козлов Олег.jpg"), named("d", "DSC_4.jpg")}, -1, "")
	if left != 0 || err != nil {
		t.Fatalf("left %d, err %v", left, err)
	}
	want := map[string]string{"a": "Петров Ваня", "b": "Сидоров Коля", "c": "Козлов Олег", "d": "Орлова Маша"}
	for id, w := range want {
		if got := shown(v, id); got != w {
			t.Errorf("%s: %q, ждали %q", id, got, w)
		}
	}
	if v.Spreads[0].Waiting != 0 {
		t.Fatalf("ждут %d", v.Spreads[0].Waiting)
	}
}

// Порядок слоёв и движение мест группой: табличка едет с местом (по ID), подписи те же.
func TestStackKeepsSeatNames(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	setSeat(e, 0, petrov)
	setSeat(e, 2, sidorov)
	p0, p2 := seatID(e, 0), seatID(e, 2)
	e.ArrangeLayers("s1", "front", []string{p0})
	v := e.TransformGroup("s1", []string{p0, p2}, 1, 0, 0, 5, 7, "")
	byID := map[string]*model.Place{}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		byID[pl.ID] = pl
	}
	if byID[p0].Person != petrov || byID[p2].Person != sidorov || byID[p0].Photo != "a" {
		t.Fatalf("после слоёв и движения: %+v, %+v", *byID[p0], *byID[p2])
	}
	if edtest.P(e).Spreads[0].Grid[len(byID)-1].ID != p0 {
		t.Fatal("место не поднялось наверх")
	}
	if shown(v, "a") != "Петров Ваня" || stubAt(v, p2) != "Сидоров Коля" {
		t.Fatalf("подписи: %q, %q", shown(v, "a"), stubAt(v, p2))
	}
}

// Бросок на место с табличкой (060 и 071): строгое имя файла и вольное — подпись таблички,
// автоимени нет, в named фото нет; на место без таблички — автоимя, как 060.
func TestDropOnNamedPlace(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	setSeat(e, 0, petrov)
	setSeat(e, 1, petrov)
	v, out, err := e.DropPhotos("s1", seatID(e, 0), []model.PhotoInfo{named("s", "Иванов Петя.jpg")}, -1, "")
	if err != nil || shown(v, "s") != "Петров Ваня" || len(out.Named) != 0 {
		t.Fatalf("строгое: %v, %q, named %v", err, shown(v, "s"), out.Named)
	}
	v, out, err = e.DropPhotos("s1", seatID(e, 1), []model.PhotoInfo{named("l", "01_иванов_петя.jpg")}, -1, "")
	if err != nil || shown(v, "l") != "Петров Ваня" || len(out.Named) != 0 || edtest.P(e).Photos["l"].Person != nil {
		t.Fatalf("вольное: %v, %q, named %v, у фото %+v", err, shown(v, "l"), out.Named, edtest.P(e).Photos["l"].Person)
	}
	v, out, err = e.DropPhotos("s1", seatID(e, 2), []model.PhotoInfo{named("f", "02_сидоров_коля.jpg")}, -1, "")
	if err != nil || shown(v, "f") != "Сидоров Коля" || len(out.Named) != 1 || out.Named[0] != "f" {
		t.Fatalf("без таблички: %v, %q, named %v", err, shown(v, "f"), out.Named)
	}
}

// Круг Save → Load: таблички мест (и правленная через фото) и имена фото — те же (ID фото
// Load переводит в ключи от путей — фото ищется по файлу); вид тот же; повторный круг —
// тот же JSON.
func TestSeatNamesSaveLoad(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	e.FillPlaces([]model.PhotoInfo{named("a", "DSC_1.jpg"), named("b", "DSC_2.jpg")}, "")
	setSeat(e, 0, petrov)
	e.SetPerson("a", model.Person{Last: "Петров", First: "Иван", Subject: "Химия"}, "")
	e.SetPerson("b", sidorov, "")
	e.SetPlacePerson("s1", seatID(e, 2), model.Person{Last: "Орлова"}, "")
	p := edtest.SaveLoad(t, e)
	g := p.Spreads[0].Grid
	want := []model.Person{{Last: "Петров", First: "Иван", Subject: "Химия"}, {}, {Last: "Орлова"}}
	for i, w := range want {
		if g[i].Person != w {
			t.Errorf("место %d: %+v, ждали %+v", i, g[i].Person, w)
		}
	}
	a, b := p.Photos[g[0].Photo], p.Photos[g[1].Photo]
	if filepath.Base(a.Path) != "DSC_1.jpg" || a.Person != nil || b.Person == nil || *b.Person != sidorov {
		t.Fatalf("имена фото: a %+v, b %+v", a, b)
	}
	e2 := edtest.Open(t, p, "")
	v := e2.View()
	if shown(v, g[0].Photo) != "Петров Иван" || shown(v, g[1].Photo) != "Сидоров Коля" || stubAt(v, g[2].ID) != "Орлова [Имя]" {
		t.Fatalf("вид после чтения: %q, %q, %q", shown(v, g[0].Photo), shown(v, g[1].Photo), stubAt(v, g[2].ID))
	}
	j1 := model.Marshal(p)
	j2 := model.Marshal(edtest.SaveLoad(t, e2))
	if !bytes.Equal(j1, j2) {
		t.Fatal("второй круг Save → Load отличается")
	}
}
