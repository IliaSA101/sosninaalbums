package view_test

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// 071: табличка места в подписи — главнее своего имени фото и имени из файла.

// stubOf — заглушка пустого места id; nil — нет.
func stubOf(sv model.SpreadView, id string) *model.StubView {
	for i := range sv.Stubs {
		if sv.Stubs[i].Place == id && sv.Stubs[i].Photo == "" {
			return &sv.Stubs[i]
		}
	}
	return nil
}

// Табличка → своё имя → строгое из файла; предмет — таблички, без него — свой предмет фото;
// табличка только с предметом — имя фото с этим предметом. Холст и печать — одно и то же.
func TestSeatNameWins(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a"), edtest.Gp(100, 10, 80, 100, "b"),
		edtest.Gp(190, 10, 80, 100, "c"), edtest.Gp(10, 130, 80, 100, "d")})
	rename(e, "a", "Иванов Петя.jpg")
	rename(e, "b", "Козлов Олег.jpg")
	rename(e, "c", "Орлов Иван.jpg")
	rename(e, "d", "Смирнов Лев.jpg")
	e.SetPerson("b", model.Person{Last: "Сидорова", First: "Анна", Subject: "Химия"}, "")
	ids := edtest.PlaceIDsOf(e)
	e.SetPlacePerson("s1", ids[0], model.Person{Last: "Петров", First: "Ваня", Subject: "Математика"}, "")
	e.SetPlacePerson("s1", ids[1], model.Person{Last: "Орлова", First: "Маша"}, "")
	e.SetPlacePerson("s1", ids[2], model.Person{Subject: "Физика"}, "")
	v := e.SetSheetCaps("s1", true, sheetTpl("{Фамилия} {Имя}\n{Предмет}", 1))
	_, pv := e.Snapshot()
	want := map[string]string{
		"a": "Петров Ваня\nМатематика", // табличка целиком, имя файла не видно
		"b": "Орлова Маша\nХимия",      // табличка без предмета — свой предмет фото
		"c": "Орлов Иван\nФизика",      // только предмет — имя из файла с ним
	}
	for id, w := range want {
		for _, sv := range []model.SpreadView{v.Spreads[0], pv.Spreads[0]} {
			if cv := capOfPhoto(sv, id); cv == nil || cv.Text != w {
				t.Errorf("%s: %+v, ждали %q", id, cv, w)
			}
		}
	}
	// Без таблички — как 030: строгое имя из файла, предмета нет (на холсте — «[Предмет]»).
	if cv := capOfPhoto(v.Spreads[0], "d"); cv == nil || cv.Text != "Смирнов Лев\n[Предмет]" {
		t.Fatalf("без таблички: %+v", cv)
	}
	// Своё имя b не потеряно: табличку сняли — снова оно.
	v = e.SetPlacePerson("s1", ids[1], model.Person{}, "")
	if cv := capOfPhoto(v.Spreads[0], "b"); cv == nil || cv.Text != "Сидорова Анна\nХимия" {
		t.Fatalf("табличка снята: %+v", cv)
	}
	// «Авто»: мест нет — имена у фото, как раньше.
	ea, id := edtest.AutoEd(t)
	av := ea.SetSheetCaps(id, true, sheetTpl("{Фамилия}", 1)).Spreads[0]
	ea.SetPerson(av.Photos[0], model.Person{Last: "Зайцев"}, "")
	if cv := capOfPhoto(ea.View().Spreads[0], av.Photos[0]); cv == nil || cv.Text != "Зайцев" {
		t.Fatalf("«Авто»: %+v", cv)
	}
}

// Заглушка пустого места: текст — табличка (как подпись на холсте), без таблички — «[Фамилия]»;
// Who — поля имени шаблона у каждого пустого места; в печати заглушек нет, но Waiting —
// пустые места с именем на табличке — и там.
func TestEmptySeatStub(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a"), edtest.Gp(100, 10, 80, 100, ""),
		edtest.Gp(190, 10, 80, 100, ""), edtest.Gp(10, 130, 80, 100, ""), edtest.Gp(100, 130, 80, 100, "")})
	ids := edtest.PlaceIDsOf(e)
	e.SetPlacePerson("s1", ids[0], model.Person{Last: "Петров", First: "Ваня"}, "") // место с фото — не ждёт
	e.SetPlacePerson("s1", ids[1], model.Person{Last: "Сидоров", First: "Коля"}, "")
	e.SetPlacePerson("s1", ids[2], model.Person{Last: "Орлова"}, "")
	e.SetPlacePerson("s1", ids[3], model.Person{Subject: "Химия"}, "") // предмет — не имя
	v := e.SetSheetCaps("s1", true, sheetTpl("{Фамилия} {Имя}", 1))
	sv := v.Spreads[0]
	want := map[string]string{ids[1]: "Сидоров Коля", ids[2]: "Орлова [Имя]", ids[3]: "[Фамилия] [Имя]", ids[4]: "[Фамилия] [Имя]"}
	for pid, w := range want {
		st := stubOf(sv, pid)
		if st == nil || st.Text != w || !slices.Equal(st.Who, []string{"last", "first"}) {
			t.Errorf("место %s: %+v, ждали %q", pid, st, w)
		}
	}
	if stubOf(sv, ids[0]) != nil || len(sv.Stubs) != 4 || sv.Waiting != 2 {
		t.Fatalf("заглушек %d, ждут %d", len(sv.Stubs), sv.Waiting)
	}
	// Таблички пустых мест не считаются пустыми полями и фото без имени.
	if len(v.Unfilled) != 0 || v.Unnamed != 0 {
		t.Fatalf("пустые поля %v, без имени %d", v.Unfilled, v.Unnamed)
	}
	_, pv := e.Snapshot()
	if ps := pv.Spreads[0]; len(ps.Stubs) != 0 || ps.Waiting != 2 || len(ps.Captions) != 1 {
		t.Fatalf("печать: заглушек %d, ждут %d, подписей %d", len(ps.Stubs), ps.Waiting, len(ps.Captions))
	}
	// Без шаблона подписи заглушек нет, а имена всё равно ждут фото.
	v = e.SetSheetCaps("s1", false, model.SheetCaps{})
	if sv := v.Spreads[0]; len(sv.Stubs) != 0 || sv.Waiting != 2 {
		t.Fatalf("без шаблона: заглушек %d, ждут %d", len(sv.Stubs), sv.Waiting)
	}
	// Шаблон без полей имени — Who пусто, текст шаблона.
	v = e.SetSheetCaps("s1", true, sheetTpl("Класс", 1))
	if st := stubOf(v.Spreads[0], ids[1]); st == nil || st.Who != nil || st.Text != "Класс" {
		t.Fatalf("шаблон без имени: %+v", st)
	}
}
