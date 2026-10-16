package names

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// people — имена разобранного списка по порядку.
func people(t *testing.T, text string, firstFirst bool) []model.Person {
	t.Helper()
	r, err := Read(model.NamesOpts{Text: text, FirstFirst: firstFirst})
	if err != nil {
		t.Fatalf("%q: %v", text, err)
	}
	var out []model.Person
	for _, e := range r.People {
		out = append(out, e.Who)
	}
	return out
}

// Без заголовка — правила 030: нумерация, пустые строки, Tab и «;», «Имя первым», пределы.
func TestParse(t *testing.T) {
	got := people(t, " 1. Сидоров Иван\n\n12) Петрова Анна Сергеевна Мария\r\nКозлов;Пётр\n\tОдин\t\n3.", false)
	want := []model.Person{
		{Last: "Сидоров", First: "Иван"},
		{Last: "Петрова", First: "Анна", Middle: "Сергеевна Мария"},
		{Last: "Козлов", First: "Пётр"},
		{Last: "Один"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("разбор: %+v", got)
	}
	got = people(t, "Иван Сидоров\nАнна Сергеевна Петрова\nОля", true)
	want = []model.Person{{First: "Иван", Last: "Сидоров"}, {First: "Анна", Middle: "Сергеевна", Last: "Петрова"}, {First: "Оля"}}
	if !slices.Equal(got, want) {
		t.Fatalf("имя первым: %+v", got)
	}
	// Номера строк поля — с 1, пустые строки считаются.
	r, _ := Read(model.NamesOpts{Text: "\nИванов\n\nПетров"})
	if len(r.People) != 2 || r.People[0].Line != 2 || r.People[1].Line != 4 || r.Cols != nil || !r.Order {
		t.Fatalf("строки: %+v", r)
	}
	// Ровно 1000 человек (с переводом строки в конце) и 1000 с заголовком — можно; 1001 человек,
	// 1002 строки поля и больше 256 КБ — нет.
	if _, err := Read(model.NamesOpts{Text: strings.Repeat("А Б\n", MaxPasteLines-1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(model.NamesOpts{Text: "Фамилия\tИмя\n" + strings.Repeat("А\tБ\n", MaxPasteLines-1)}); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"1001 человек":  strings.Repeat("А Б\n", MaxPasteLines),
		"1002 строки":   strings.Repeat("\n", MaxPasteLines) + "А",
		"больше 256 КБ": strings.Repeat("я", MaxPasteBytes),
	} {
		if _, err := Read(model.NamesOpts{Text: text}); err == nil || err.Error() != errTooLong.Error() {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Длинное имя режется ClampPerson.
	got = people(t, strings.Repeat("Ф", 500)+" Имя", false)
	if len(got) != 1 || len([]rune(got[0].Last)) != model.MaxPersonRunes {
		t.Fatalf("длинное: %d знаков", len([]rune(got[0].Last)))
	}
	// «Имена файлов» поле не читает: даже слишком длинное.
	if r, err := Read(model.NamesOpts{From: model.NamesFiles, Text: strings.Repeat("я", MaxPasteBytes+1)}); err != nil || !r.Files || r.People != nil {
		t.Fatalf("имена файлов: %+v, %v", r, err)
	}
}

// Заголовки таблицы (043 §3.1): синонимы, регистр, «ё», знаки, любой порядок столбцов, «ФИО»,
// «№» не используется; строка с Tab делится только по Tab.
func TestParseHeader(t *testing.T) {
	r, err := Read(model.NamesOpts{Text: "\n№\tИМЯ:\tФамилия*\tОтчество\tДолжность\tФото\tКласс\n" +
		"1\tПётр\tИванов\t\t\t0012\t5А\n" +
		"2\tМария\tСидорова\tИвановна\tМатематика\n" +
		"3\t\t\t\tФизика\n" + // без имени — пропуск
		"4\tОля\tКозлова;Петрова", FirstFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	wantCols := []string{"", "Имя", "Фамилия", "Отчество", "Предмет", "Файл", ""}
	if !slices.Equal(r.Cols, wantCols) || !slices.Equal(r.Ignored, []string{"№", "Класс"}) || !r.ByFile || r.Order {
		t.Fatalf("столбцы %q, не используются %q, по файлу %v, порядок %v", r.Cols, r.Ignored, r.ByFile, r.Order)
	}
	want := []Entry{
		{Who: model.Person{Last: "Иванов", First: "Пётр"}, File: "0012", Line: 3},
		{Who: model.Person{Last: "Сидорова", First: "Мария", Middle: "Ивановна", Subject: "Математика"}, Teacher: true, Line: 4},
		{Who: model.Person{Last: "Козлова;Петрова", First: "Оля"}, Line: 6},
	}
	if !slices.Equal(r.People, want) {
		t.Fatalf("люди: %+v", r.People)
	}
	// «Ф.И.О.» — одна ячейка по правилам 030, «Имя первым» действует; заголовки через «;».
	r, _ = Read(model.NamesOpts{Text: "Ф.И.О.;Предмет\nПетя Иванов;\nМария Ивановна Сидорова;Музыка", FirstFirst: true})
	want = []Entry{
		{Who: model.Person{Last: "Иванов", First: "Петя"}, Line: 2},
		{Who: model.Person{Last: "Сидорова", First: "Мария", Middle: "Ивановна", Subject: "Музыка"}, Teacher: true, Line: 3},
	}
	if !slices.Equal(r.People, want) || !r.Order || !slices.Equal(r.Cols, []string{"ФИО", "Предмет"}) || r.ByFile {
		t.Fatalf("ФИО: %+v", r)
	}
	// Фамилия и ФИО вместе — ФИО не используется; повтор роли — первый столбец; английские синонимы.
	r, _ = Read(model.NamesOpts{Text: "Full name\tSurname\tFirst Name\tLast name\nX Y\tIvanov\tPetr\tZ"})
	if !slices.Equal(r.Cols, []string{"", "Фамилия", "Имя", ""}) || !slices.Equal(r.Ignored, []string{"Full name", "Last name"}) ||
		len(r.People) != 1 || r.People[0].Who != (model.Person{Last: "Ivanov", First: "Petr"}) {
		t.Fatalf("синонимы: %+v", r)
	}
	// Строка без «Фамилия», «Имя», «ФИО» — не заголовок: правила 030.
	r, _ = Read(model.NamesOpts{Text: "Предмет\tКласс\nИванов\tПетя"})
	if r.Cols != nil || len(r.People) != 2 || r.People[0].Who != (model.Person{Last: "Предмет", First: "Класс"}) {
		t.Fatalf("без заголовка: %+v", r)
	}
	// Через пробелы — заголовок, только если все слова — Фамилия, Имя, Отчество: «Сидоров Имя» — человек.
	r, _ = Read(model.NamesOpts{Text: "имя  фамилия  отчество\nПетя Иванов\nМария Сидорова Ивановна"})
	if !slices.Equal(r.Cols, []string{"Имя", "Фамилия", "Отчество"}) || r.Order || len(r.People) != 2 ||
		r.People[1].Who != (model.Person{Last: "Сидорова", First: "Мария", Middle: "Ивановна"}) {
		t.Fatalf("заголовок через пробелы: %+v", r)
	}
	r, _ = Read(model.NamesOpts{Text: "Сидоров Имя\nИванов Петя"})
	if r.Cols != nil || len(r.People) != 2 || r.People[0].Who != (model.Person{Last: "Сидоров", First: "Имя"}) {
		t.Fatalf("не заголовок: %+v", r)
	}
	// Ячейки: Tab — только по Tab, «;» внутри ячейки остаётся; без Tab — по «;», иначе по пробелам.
	for line, want := range map[string][]string{
		"Козлов;Пётр\tИван": {"Козлов;Пётр", "Иван"},
		" А ; ;Б":           {"А", "", "Б"},
		"  А   Б ":          {"А", "Б"},
		"\tА\t":             {"", "А", ""},
	} {
		if got := cells(line); !slices.Equal(got, want) {
			t.Errorf("%q → %q, ждали %q", line, got, want)
		}
	}
}

// Строки данных делятся разделителем заголовка (043 ревью): Sheet.Text снимает пустые хвостовые
// ячейки, и у ученика с пустым «Предмет» Tab пропадает — «Иванов Пётр Сергеевич» остаётся ФИО.
func TestParseRowSeparatorFromHeader(t *testing.T) {
	ivanov := model.Person{Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}
	for name, text := range map[string]string{
		"Tab, пустой предмет без Tab": "ФИО\tПредмет\nСидорова Ольга Петровна\tМатематика\nИванов Пётр Сергеевич",
		"Tab, пустой предмет с Tab":   "ФИО\tПредмет\nСидорова Ольга Петровна\tМатематика\nИванов Пётр Сергеевич\t",
		"CSV «;», пустой хвост":       "ФИО;Предмет\nСидорова Ольга Петровна;Математика\nИванов Пётр Сергеевич;",
		"CSV «;», хвоста нет":         "ФИО;Предмет\nСидорова Ольга Петровна;Математика\nИванов Пётр Сергеевич",
	} {
		r, err := Read(model.NamesOpts{Text: text})
		if err != nil || len(r.People) != 2 {
			t.Fatalf("%s: %+v, %v", name, r.People, err)
		}
		if p := r.People[1]; p.Who != ivanov || p.Teacher || !r.People[0].Teacher || r.People[0].Who.Last != "Сидорова" {
			t.Errorf("%s: %+v", name, r.People)
		}
	}
	// Один столбец «ФИО»: заголовок без Tab и «;» — строки не делятся по пробелам как столбцы.
	got := people(t, "ФИО\nИванов Пётр\nСидорова Ольга Петровна", false)
	if want := []model.Person{{Last: "Иванов", First: "Пётр"}, {Last: "Сидорова", First: "Ольга", Middle: "Петровна"}}; !slices.Equal(got, want) {
		t.Errorf("один столбец ФИО: %+v", got)
	}
	// Один столбец «Фамилия» — вся строка в Фамилии (ячейка одна).
	if got = people(t, "Фамилия\nИванов Пётр", false); !slices.Equal(got, []model.Person{{Last: "Иванов Пётр"}}) {
		t.Errorf("один столбец Фамилия: %+v", got)
	}
	// Заголовок Tab-таблицы: «;» внутри ячейки не делит; строка без Tab — одна ячейка.
	if got = people(t, "Фамилия\tИмя\nКозлов;Пётр\tИван\nОдин", false); !slices.Equal(got, []model.Person{{Last: "Козлов;Пётр", First: "Иван"}, {Last: "Один"}}) {
		t.Errorf("Tab-заголовок: %+v", got)
	}
	// Без заголовка поведение прежнее: строка делится по своему разделителю.
	if got = people(t, "Иванов Пётр\nКозлов;Пётр", false); !slices.Equal(got, []model.Person{{Last: "Иванов", First: "Пётр"}, {Last: "Козлов", First: "Пётр"}}) {
		t.Errorf("без заголовка: %+v", got)
	}
	// Число без точки — не нумерация (2025 — не «2025.»).
	if got = people(t, "2025 Год", false); !slices.Equal(got, []model.Person{{Last: "2025", First: "Год"}}) {
		t.Errorf("число без точки: %+v", got)
	}
}

// Заголовок одной ячейкой «Фамилия Имя» и его записи — синоним «ФИО» (одна ячейка). Порядок слов
// задаёт сам заголовок: «Имя первым» не действует (Order = false) ни выключенным, ни включённым.
func TestParseHeaderOneCellSynonyms(t *testing.T) {
	ivanov := model.Person{Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}
	petrova := model.Person{Last: "Петрова", First: "Анна"}
	for _, ff := range []bool{false, true} {
		for head, rows := range map[string]string{
			"Фамилия Имя":            "Иванов Пётр Сергеевич\nПетрова Анна",
			"Фамилия, имя":           "Иванов Пётр Сергеевич\nПетрова Анна",
			"Фамилия и имя":          "Иванов Пётр Сергеевич\nПетрова Анна",
			"ФАМИЛИЯ ИМЯ ОТЧЕСТВО":   "Иванов Пётр Сергеевич\nПетрова Анна",
			"Фамилия имя отчество:":  "Иванов Пётр Сергеевич\nПетрова Анна",
			"Имя Фамилия":            "Пётр Сергеевич Иванов\nАнна Петрова",
			"имя и фамилия":          "Пётр Сергеевич Иванов\nАнна Петрова",
			"Имя, отчество, фамилия": "Пётр Сергеевич Иванов\nАнна Петрова",
		} {
			r, err := Read(model.NamesOpts{Text: head + "\n" + rows, FirstFirst: ff})
			if err != nil || !slices.Equal(r.Cols, []string{"ФИО"}) || r.Order || len(r.People) != 2 ||
				r.People[0].Who != ivanov || r.People[1].Who != petrova {
				t.Errorf("%q, имя первым %v: %+v, %v", head, ff, r, err)
			}
		}
	}
	// Синоним в таблице с Tab — тот же фиксированный порядок; «ФИО» порядок не задаёт.
	r, _ := Read(model.NamesOpts{Text: "Фамилия Имя Отчество\tПредмет\nИванов Пётр Сергеевич\tФизика", FirstFirst: true})
	physics := ivanov
	physics.Subject = "Физика" // 049: предмет ложится в имя
	if r.Order || len(r.People) != 1 || r.People[0].Who != physics || !r.People[0].Teacher {
		t.Errorf("синоним в таблице: %+v", r)
	}
	if r, _ = Read(model.NamesOpts{Text: "ФИО\tПредмет\nПётр Иванов", FirstFirst: true}); !r.Order ||
		len(r.People) != 1 || r.People[0].Who != (model.Person{Last: "Иванов", First: "Пётр"}) {
		t.Errorf("ФИО: %+v", r)
	}
	// Синоним рядом со столбцом «Фамилия» не используется — и порядок не фиксирует.
	if r, _ = Read(model.NamesOpts{Text: "Имя Фамилия\tФамилия\tИмя\nX Y\tИванов\tПётр"}); r.Order ||
		!slices.Equal(r.Cols, []string{"", "Фамилия", "Имя"}) || len(r.People) != 1 || r.People[0].Who != (model.Person{Last: "Иванов", First: "Пётр"}) {
		t.Errorf("синоним и Фамилия: %+v", r)
	}
	// Те же слова в Tab-заголовке остаются отдельными столбцами.
	r, _ = Read(model.NamesOpts{Text: "Фамилия\tИмя\nИванов\tПётр"})
	if !slices.Equal(r.Cols, []string{"Фамилия", "Имя"}) || r.Order {
		t.Errorf("столбцы: %+v", r)
	}
}

// Заголовок одной ячейкой (043 ревью 2): строка данных с Tab — таблица с заголовком только в A1,
// делится по Tab; без Tab — одна ячейка. Нумерация простого списка снимается и под заголовком.
func TestParseOneCellHeaderRows(t *testing.T) {
	cases := []struct {
		name, text string
		want       []model.Person
	}{
		{"ФИО и предмет за Tab", "ФИО\nСидорова Ольга\tМатематика\nИванов Пётр Сергеевич",
			[]model.Person{{Last: "Сидорова", First: "Ольга"}, {Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}}},
		{"Фамилия и имя за Tab", "Фамилия\nИванов\tПётр\nКозлов Пётр",
			[]model.Person{{Last: "Иванов"}, {Last: "Козлов Пётр"}}},
		{"пустая первая ячейка", "ФИО\n\tИванов\nПетров", []model.Person{{Last: "Петров"}}},
		{"нумерация под ФИО", "ФИО\n1. Иванов Пётр\n2) Петрова Анна\n2025 Год",
			[]model.Person{{Last: "Иванов", First: "Пётр"}, {Last: "Петрова", First: "Анна"}, {Last: "2025", First: "Год"}}},
		{"нумерация под синонимом", "Фамилия Имя\n 12. Иванов Пётр",
			[]model.Person{{Last: "Иванов", First: "Пётр"}}},
		{"нумерация под пробельным заголовком", "Имя Отчество\n1. Пётр Сергеевич",
			[]model.Person{{First: "Пётр", Middle: "Сергеевич"}}},
		{"нумерация и Tab", "ФИО\n3. Иванов Пётр\tФизика", []model.Person{{Last: "Иванов", First: "Пётр"}}},
	}
	for _, c := range cases {
		if got := people(t, c.text, false); !slices.Equal(got, c.want) {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
	// В Tab-таблице номер — свой столбец: «1.» в ячейке имени не снимается (это не простой список).
	if got := people(t, "ФИО\tПредмет\n1. Иванов\t", false); !slices.Equal(got, []model.Person{{Last: "1.", First: "Иванов"}}) {
		t.Errorf("Tab-таблица: %+v", got)
	}
}

// Пробельный заголовок (043 ревью 4): только из Фамилия/Имя/Отчество или одной ячейки-синонима ФИО;
// с ФИО, Предметом или Файлом строка — не заголовок, а человек по правилам 030.
func TestParseSpacedHeaderRoles(t *testing.T) {
	for _, text := range []string{"ФИО Предмет\nИванов Пётр Сергеевич", "Фамилия Имя Предмет\nИванов Пётр Сергеевич",
		"Фамилия Файл\nИванов Пётр Сергеевич", "Имя Фото\nИванов Пётр Сергеевич"} {
		r, err := Read(model.NamesOpts{Text: text})
		if err != nil || r.Cols != nil || !r.Order || len(r.People) != 2 || r.People[1].Teacher || r.People[1].File != "" ||
			r.People[1].Who != (model.Person{Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}) {
			t.Errorf("%q: %+v, %v", text, r, err)
		}
	}
	// Те же роли через «;» — заголовок.
	r, _ := Read(model.NamesOpts{Text: "ФИО;Предмет\nИванов Пётр Сергеевич;Физика"})
	if !slices.Equal(r.Cols, []string{"ФИО", "Предмет"}) || len(r.People) != 1 || !r.People[0].Teacher {
		t.Errorf("через «;»: %+v", r)
	}
}

// BOM в начале вставки (Excel, «Блокнот») не прячет первую ячейку заголовка.
func TestReadStripsBOM(t *testing.T) {
	for _, text := range []string{bom + "Фамилия;Имя\nИванов;Пётр\nПетрова;Анна", bom + "Фамилия\tИмя\nИванов\tПётр\nПетрова\tАнна"} {
		r, err := Read(model.NamesOpts{Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(r.Cols, []string{"Фамилия", "Имя"}) || len(r.People) != 2 || r.People[0].Who.Last != "Иванов" || r.People[1].Who.First != "Анна" {
			t.Fatalf("%q: cols %v, people %v", text, r.Cols, r.People)
		}
	}
	// Пустое поле и один BOM — не паника, людей нет.
	for _, text := range []string{"", bom} {
		if r, err := Read(model.NamesOpts{Text: text}); err != nil || len(r.People) != 0 {
			t.Fatalf("%q: %v %v", text, r.People, err)
		}
	}
}
