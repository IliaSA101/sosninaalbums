package app

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// 0.26-123-57: две свои, чьи подписи в списках («<название> (своя)») совпадают, не сохраняются.

// Своя «QA тест 1» и «QA тест 1 (своя)» обе показываются «QA тест 1 (своя)» — в любом порядке отказ.
func TestSaveOwnLabelClash(t *testing.T) {
	for _, order := range [][2]string{{"QA тест 1", "QA тест 1 (своя)"}, {"QA тест 1 (своя)", "QA тест 1"}} {
		a, ev := printApp(t)
		saved(t, a, editOf(t, a, "fabrika", order[0]))
		e := editOf(t, a, "fabrika", order[1])
		if c := a.CheckPrintProfile(e); c.OK || c.Errs["name"] != errPrintDupLabel.Error() {
			t.Errorf("%q после %q: расчёт %+v", order[1], order[0], c.Errs)
		}
		if _, err := a.SavePrintProfile(e, false); err == nil || err.Error() != errPrintDupLabel.Error() {
			t.Errorf("%q после %q: %v", order[1], order[0], err)
		}
		if ownCount(a) != 1 || ev.count("print:changed") != 1 || len(a.mustStore(t).Prints()) != 1 {
			t.Errorf("после отказа свои %d, событий %d", ownCount(a), ev.count("print:changed"))
		}
	}
}

// Подписи разные — можно: «X (своя)» и «X (своя 2)» (и «X (своя 3)» следом); у встроенной пометки нет.
func TestSaveOwnLabelAllowed(t *testing.T) {
	a, _ := printApp(t)
	for _, name := range []string{"X (своя)", "X (своя 2)", "X (своя 3)", "Фотосфера (своя)"} {
		saved(t, a, editOf(t, a, "fabrika", name))
	}
	if ownCount(a) != 4 {
		t.Fatalf("своих %d, ждали 4", ownCount(a))
	}
	// Круг Save→Load: после перечитывания хранилища все четыре на месте.
	a.reloadPrints()
	if ownCount(a) != 4 {
		t.Errorf("после перезагрузки реестра своих %d", ownCount(a))
	}
	// Само название встроенной остаётся отказом — это двойник, а не спор подписей.
	if _, err := a.SavePrintProfile(editOf(t, a, "fabrika", "Фотосфера"), false); err == nil || err.Error() != errPrintDup.Error() {
		t.Errorf("название встроенной: %v", err)
	}
}

// Прежние данные: пара с одинаковой подписью, уже лежащая в хранилище, не пропадает при загрузке.
func TestReloadKeepsStoredLabelPair(t *testing.T) {
	a, _ := printApp(t)
	for i, name := range []string{"Пара", "Пара (своя)"} {
		p := ownFile(t, a, "own-abcd000"+string(rune('1'+i)), name, 1)
		a.prints.mu.Lock()
		err := a.storeOwn(a.mustStore(t), "", p, nil)
		a.prints.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
	a.reloadPrints()
	if ownCount(a) != 2 {
		t.Errorf("своих %d, ждали 2", ownCount(a))
	}
}

// freeName не предлагает название, чья подпись совпала бы с подписью существующей своей.
func TestFreeNameAvoidsLabelClash(t *testing.T) {
	a, _ := printApp(t)
	one := saved(t, a, editOf(t, a, "fabrika", "Бета"))
	// «Бета (своя)» совпало бы подписью с «Бета» — следующее «(своя 2)».
	if e, err := a.PrintProfileEdit(one.ID); err != nil || e.Name != "Бета (своя 2)" {
		t.Fatalf("основа «Бета»: %q %v", e.Name, err)
	}
	// Несколько подряд: занято «Бета» и «Бета (своя 2)» — всё равно «Бета (своя 3)».
	saved(t, a, editOf(t, a, "fabrika", "Бета (своя 2)"))
	if e, _ := a.PrintProfileEdit(one.ID); e.Name != "Бета (своя 3)" {
		t.Errorf("с занятой «(своя 2)»: %q", e.Name)
	}
	// Выбранное предложение сохраняется без отказа.
	e, _ := a.PrintProfileEdit(one.ID)
	if _, err := a.SavePrintProfile(e, false); err != nil {
		t.Errorf("предложение %q отвергнуто: %v", e.Name, err)
	}
}

// Файл .saprint: название, чья подпись совпала с подписью другой своей, — отказ, ничего не записано;
// копия под новым id (чужой id) отказ не обходит.
func TestImportOwnLabelClash(t *testing.T) {
	a, ev := printApp(t)
	saved(t, a, editOf(t, a, "fabrika", "QA тест 1"))
	for i, name := range []string{"QA тест 1 (своя)", "QA\u00a0тест 1  (своя)"} {
		p := ownFile(t, a, "own-eeee000"+string(rune('1'+i)), "Временная", 1)
		p.Name, p.Gen = name, name
		if _, err := a.importPrintPath(profileFile(t, "x.saprint", encoded(t, p))); err != errPrintDupLabel {
			t.Errorf("%q: %v", name, err)
		}
	}
	bi := builtinCopy(t, "fabrika", "QA тест 1 (своя)") // id встроенной → импорт делает копию
	if _, err := a.importPrintPath(profileFile(t, "b.saprint", encoded(t, bi))); err != errPrintDupLabel {
		t.Errorf("копия: %v", err)
	}
	if ownCount(a) != 1 || ev.count("print:changed") != 1 {
		t.Errorf("своих %d, событий %d", ownCount(a), ev.count("print:changed"))
	}
	// Подпись другая — принимается.
	ok := ownFile(t, a, "own-eeee0009", "QA тест 1 (своя 2)", 1)
	if _, err := a.importPrintPath(profileFile(t, "ok.saprint", encoded(t, ok))); err != nil || ownCount(a) != 2 {
		t.Errorf("«(своя 2)»: %v, своих %d", err, ownCount(a))
	}
}

// Замена по id: новое название файла спорит подписью с другой своей — отказ и при вопросе, и при ответе,
// если название заняли за это время.
func TestImportReplaceLabelClash(t *testing.T) {
	a, _ := printApp(t)
	v1 := ownFile(t, a, "own-aaaa0001", "Общая", 1)
	if _, err := a.importPrintPath(profileFile(t, "1.saprint", encoded(t, v1))); err != nil {
		t.Fatal(err)
	}
	saved(t, a, editOf(t, a, "fabrika", "Игрек"))
	v2 := v1
	v2.Version, v2.Name, v2.Gen = 2, "Игрек (своя)", "Игрек (своя)"
	if _, err := a.importPrintPath(profileFile(t, "2.saprint", encoded(t, v2))); err != errPrintDupLabel {
		t.Errorf("вопрос о замене: %v", err)
	}
	// Название свободно при вопросе, затем занято сохранением другой своей.
	v3 := v1
	v3.Version, v3.Name, v3.Gen = 3, "Зет", "Зет"
	res, err := a.importPrintPath(profileFile(t, "3.saprint", encoded(t, v3)))
	if err != nil || res.Ask != "replace" {
		t.Fatalf("вопрос: %+v %v", res, err)
	}
	saved(t, a, editOf(t, a, "fabrika", "Зет (своя)"))
	if _, err := a.ConfirmImportPrintProfile(res.Token); err != errPrintDupLabel {
		t.Errorf("ответ: %v", err)
	}
	if cur, _ := printprof.Current().ByID("own-aaaa0001"); cur.Name != "Общая" || cur.Version != 1 {
		t.Errorf("типография изменилась: %q v%d", cur.Name, cur.Version)
	}
}
