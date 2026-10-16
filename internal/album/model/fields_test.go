package model

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Подстановка полей (030): регистр и «ё» не важны, неизвестное остаётся, значения
// не раскрываются повторно, пустое — по режиму, пустые строки в конце уходят.
func TestExpandFields(t *testing.T) {
	vars := Vars{Title: "Школа № 5", Year: "2026", Subtitle: "{Имя}"}
	ivanov := &Person{Last: "Иванов", First: "Петя"}
	cases := []struct {
		name, in, want string
		ctx            FieldCtx
		missing        []string
	}{
		{"без полей", "Просто текст {", "Просто текст {", FieldCtx{}, nil},
		{"регистр", "{школа} / {Школа} / {ШКОЛА}", "Школа № 5 / Школа № 5 / Школа № 5", FieldCtx{Vars: vars}, nil},
		{"пустое дважды", "{Учитель}{УЧИТЕЛЬ}{Руководитель}", "[Руководитель][Руководитель][Руководитель]", FieldCtx{Vars: vars}, []string{"Руководитель"}},
		{"неизвестное", "{Цвет} {Год}", "{Цвет} 2026", FieldCtx{Vars: vars}, nil},
		{"значение не раскрывается", "{Класс}", "{Имя}", FieldCtx{Vars: vars, Person: ivanov}, nil},
		{"имя", "{Фамилия}\n{Имя}", "Иванов\nПетя", FieldCtx{Person: ivanov}, nil},
		{"без отчества — без пустой строки", "{Фамилия}\n{Имя}\n{Отчество}", "Иванов\nПетя", FieldCtx{Person: ivanov, Mode: FieldPrint}, []string{"Отчество"}},
		{"холст", "Класс {Класс} {год}", "Класс [Подзаголовок] [Год]", FieldCtx{}, []string{"Подзаголовок", "Год"}},
		{"печать", "{Школа}, {Год}", ", ", FieldCtx{Mode: FieldPrint}, []string{"Заголовок", "Год"}},
		{"заглушка", "{Фамилия} {Год}", "[Фамилия] 2026", FieldCtx{Vars: vars, Person: ivanov, Mode: FieldStub}, []string{"Фамилия"}},
		{"пробелы в имени", "{ Школа }", "Школа № 5", FieldCtx{Vars: vars}, nil},
		{"ё = е", "{Учитёль}", "Мария Ивановна", FieldCtx{Vars: Vars{Lead: "Мария Ивановна"}}, nil},
		// Поля шапки листа (031 dialog §4): канон и прежние имена — одно поле.
		{"канон = синоним", "{Заголовок}|{заголовок}|{Школа}", "Школа № 5|Школа № 5|Школа № 5", FieldCtx{Vars: vars}, nil},
		{"синоним пуст — канон", "{Школа}{Заголовок}\n{Учитель}", "[Заголовок][Заголовок]\n[Руководитель]", FieldCtx{}, []string{"Заголовок", "Руководитель"}},
		{"шапка в печати", VigHeadText, "Школа № 5\n{Имя} · 2026", FieldCtx{Vars: vars, Mode: FieldPrint}, nil},
	}
	for _, c := range cases {
		got, miss := Expand(c.in, c.ctx)
		if got != c.want || !slices.Equal(miss, c.missing) {
			t.Errorf("%s: %q → %q %v, ждали %q %v", c.name, c.in, got, miss, c.want, c.missing)
		}
	}
	if !HasFields("{") || HasFields("без скобок") {
		t.Fatal("HasFields")
	}
}

// Злой ввод в Expand: длина результата ограничена, вложенные и незакрытые скобки
// копируются, не-UTF8 не ломает строку.
func TestExpandHostile(t *testing.T) {
	long := Vars{Title: strings.Repeat("ш", MaxVarRunes)}
	in := strings.Repeat("{Школа}", 1000)
	out, _ := Expand(in, FieldCtx{Vars: long})
	if n := utf8.RuneCountInString(out); n != MaxTextRunes {
		t.Fatalf("длина %d, ждали %d", n, MaxTextRunes)
	}
	out, _ = Expand(strings.Repeat("я{", 5000), FieldCtx{})
	if utf8.RuneCountInString(out) != MaxTextRunes || !utf8.ValidString(out) {
		t.Fatalf("длина без полей %d", utf8.RuneCountInString(out))
	}
	// Подпись и заглушка — не длиннее подписи (decisions 79), пустые строки в конце уходят.
	if got, _ := ShownCaption(&long, in, nil, FieldCanvas); utf8.RuneCountInString(got) != MaxCaptionRunes {
		t.Fatalf("подпись длиной %d, ждали %d", utf8.RuneCountInString(got), MaxCaptionRunes)
	}
	if got, _ := ShownCaption(&long, "{Год}"+strings.Repeat("\n", 400)+"x", nil, FieldPrint); got != "" {
		t.Fatalf("хвост из пустых строк: %q", got)
	}
	if got, _ := ShownCaption(&long, "{Школа}", nil, FieldCanvas); got != long.Title {
		t.Fatalf("короткая подпись изменилась: %q", got)
	}
	cases := map[string]string{
		"{{Год}}":                           "{2026}",
		"{Го{Год}д}":                        "{Го2026д}",
		"{Год":                              "{Год",
		"{}":                                "{}",
		"{Год1}":                            "{Год1}",
		"{" + strings.Repeat("а", 21) + "}": "{" + strings.Repeat("а", 21) + "}",
		"\xff{Год}\xfe":                     "\uFFFD2026\uFFFD",
		"{Г\xffод}":                         "{Г\uFFFDод}",
	}
	for in, want := range cases {
		if got, _ := Expand(in, FieldCtx{Vars: Vars{Year: "2026"}}); got != want {
			t.Errorf("%q → %q, ждали %q", in, got, want)
		}
	}
}

// Имя из имени файла: 2–3 слова из букв с заглавной; номера и подписи кадров — не имена.
// Номер в начале и отметка копии в конце снимаются (043 сц. 7): «(2)» с 043 — имя.
func TestPersonFromFile(t *testing.T) {
	ivanov := &Person{Last: "Иванов", First: "Петя"}
	cases := map[string]*Person{
		`D:\x\01_Иванов_Петя.jpg`:                ivanov,
		`D:\x\1. Иванов Петя.jpg`:                ivanov,
		`D:\x\12) Иванов Петя.jpg`:               ivanov,
		`D:\x\3-Иванов Петя.jpg`:                 ivanov,
		`D:\x\Иванов Петя 2.jpg`:                 ivanov,
		`D:\x\Иванов_Петя_2.jpg`:                 ivanov,
		`D:\x\Иванов Петя-2.jpg`:                 ivanov,
		`D:\x\01_Иванов_Петя (12).jpg`:           ivanov,
		`D:\x\Иванова-Петрова Анна (2).jpg`:      {Last: "Иванова-Петрова", First: "Анна"},
		`D:\x\Иванов Петя 2025.jpg`:              nil, // 4 цифры — не отметка копии
		`D:\x\12345 Иванов Петя.jpg`:             nil, // 5 цифр — не номер
		`D:\x\01_иванов_петя.jpg`:                nil, // строчные — только «Имена файлов»
		`D:\x\Иванов.Петя.jpg`:                   nil,
		`D:\x\Иванов Петя (2)(3).jpg`:            nil,
		`D:\x\(2).jpg`:                           nil,
		`D:\x\1.jpg`:                             nil,
		`C:\Фото\Иванов Петя.jpg`:                {Last: "Иванов", First: "Петя"},
		"/фото/Петрова Анна Сергеевна.JPEG":      {Last: "Петрова", First: "Анна", Middle: "Сергеевна"},
		`D:\x\Иванова-Петрова_Мария.jpg`:         {Last: "Иванова-Петрова", First: "Мария"},
		`D:\x\Д'Артаньян Шарль.jpg`:              {Last: "Д'Артаньян", First: "Шарль"},
		`D:\x\Smith John.jpg`:                    {Last: "Smith", First: "John"},
		`D:\x\DSC_0012.jpg`:                      nil,
		`D:\x\001.jpg`:                           nil,
		`D:\x\Свадьба 2025.jpg`:                  nil,
		`D:\x\Иванов.jpg`:                        nil,
		`D:\x\Иванов петя.jpg`:                   nil,
		`D:\x\Иванов Петя Сергеевич Младший.jpg`: nil,
		`D:\x\Иванов- Петя.jpg`:                  nil,
		`D:\x\Иванов--Петров Петя.jpg`:           nil,
		`D:\x\Иванов Петя (2).jpg`:               ivanov,
		"":                                       nil,
	}
	for path, want := range cases {
		got := PersonFromFile(path)
		if (got == nil) != (want == nil) || got != nil && *got != *want {
			t.Errorf("%q → %+v, ждали %+v", path, got, want)
		}
	}
	r := PhotoRef{Path: `C:\Иванов Петя.jpg`}
	if p, file := r.Name(); p == nil || !file || p.First != "Петя" {
		t.Fatalf("Name из файла: %+v %v", p, file)
	}
	r.Person = &Person{First: "Пётр"}
	if p, file := r.Name(); p.First != "Пётр" || file {
		t.Fatalf("своё имя: %+v %v", p, file)
	}
}

// Зажим полей 030 для правок и Load; Copy не оставляет общих указателей.
func TestClampCaptionFields(t *testing.T) {
	if ClampVars(Vars{Title: "  \n ", Lead: "\t"}) != nil || ClampPerson(Person{Middle: "\t"}) != nil {
		t.Fatal("пустые поля должны давать nil")
	}
	v := ClampVars(Vars{Title: "Школа\n№ 5", Year: strings.Repeat("9", 500)})
	if v.Title != "Школа № 5" || utf8.RuneCountInString(v.Year) != MaxVarRunes {
		t.Fatalf("переменные: %+v", v)
	}
	p := ClampPerson(Person{Last: strings.Repeat("я", 10000), First: "Петя\u202e"})
	if utf8.RuneCountInString(p.Last) != MaxPersonRunes || p.First != "Петя" {
		t.Fatalf("имя: %+v", p)
	}
	f := func(v float64) *float64 { return &v }
	for in, want := range map[float64]float64{4: 4, -3: 0, 99: MaxPad, 2.345: 2.3} {
		if got := ClampCaptionPad(f(in)); got == nil || *got != want {
			t.Errorf("отступ %v → %v", in, got)
		}
	}
	for _, in := range []float64{math.NaN(), math.Inf(1), DefaultPad, 1.04} {
		if got := ClampCaptionPad(f(in)); got != nil {
			t.Errorf("отступ %v → %v, ждали nil", in, *got)
		}
	}
	if (Caption{}).PadOf() != DefaultPad || (Caption{Pad: f(3)}).PadOf() != 3 {
		t.Fatal("PadOf")
	}
	c := ClampSheetCaps(SheetCaps{Pad: math.NaN(), Text: "  ", Style: TextStyle{Size: 1e9, Color: "#ABCDEF"}})
	if c.Pad != DefaultPad || c.Text != "" || c.Style.Color != "#abcdef" || c.Style.Size > 1000 {
		t.Fatalf("стиль листа: %+v", c)
	}

	r := PhotoRef{Caption: &Caption{Text: "т", Pad: f(3)}, Person: &Person{Last: "Иванов"}}
	cp := r.Copy()
	*cp.Caption.Pad, cp.Person.Last = 5, "Петров"
	if *r.Caption.Pad != 3 || r.Person.Last != "Иванов" {
		t.Fatal("Copy делит указатели Pad или Person")
	}
}

// Load и CheckTotals считают знаки полей 030 одинаково: правка, которую Load обрежет, не проходит.
func TestFieldsTextBudget(t *testing.T) {
	p := New()
	long := strings.Repeat("я", MaxPersonRunes)
	for i := range MaxTotalRunes/(3*MaxPersonRunes) + 1 {
		p.Photos[NewID()+string(rune('a'+i%26))] = PhotoRef{Person: &Person{Last: long, First: long, Middle: long}}
	}
	if err := p.CheckTotals(New()); err == nil {
		t.Fatal("CheckTotals пропустил имена сверх запаса знаков")
	}
	s := &Spread{ID: NewID(), Photos: []string{}, Vars: &Vars{Title: "Школа"}}
	p.Spreads = []*Spread{s}
	p.ClampDesign()
	n, kept := 0, 0
	for _, r := range p.Photos {
		if r.Person != nil {
			n += r.Person.runes()
			kept++
		}
	}
	if s.Vars == nil || n+s.Vars.runes() > MaxTotalRunes || kept != len(p.Photos)-1 {
		t.Fatalf("после зажима: знаков %d, имён %d из %d, vars %v", n, kept, len(p.Photos), s.Vars)
	}
	if err := p.CheckTotals(New()); err != nil {
		t.Fatalf("зажатый проект не проходит CheckTotals: %v", err)
	}
}

// Поля шапки — свойство листа (031 dialog §4): свои; у обложки и обычного разворота без
// своих — первой виньетки с полями; у виньетки — только свои. Меню полей — 3 или 7 имён.
func TestVarsOfSheet(t *testing.T) {
	vig := func(v *Vars) *Spread {
		return &Spread{ID: NewID(), Kind: KindVignette, Vig: &Vignette{Tpl: "x"}, Vars: v}
	}
	a, b := &Vars{Title: "Гимназия № 1"}, &Vars{Title: "Лицей"}
	plain, own := &Spread{ID: NewID()}, &Spread{ID: NewID(), Vars: &Vars{Year: "2026"}}
	empty, v1, v2 := vig(nil), vig(a), vig(b)
	p := New()
	p.Spreads = []*Spread{plain, empty, own, v1, v2}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID}}
	for name, c := range map[string]struct {
		s    *Spread
		want *Vars
	}{
		"обложка — первой виньетки с полями": {p.Cover.Sheet, a},
		"разворот без своих":                 {plain, a},
		"свои":                               {own, own.Vars},
		"виньетка без своих — пусто":         {empty, nil},
		"виньетка — свои":                    {v2, b},
		"nil":                                {nil, nil},
	} {
		if got := p.VarsOf(c.s); got != c.want {
			t.Errorf("%s: %+v, ждали %+v", name, got, c.want)
		}
		if got := p.SheetVars()(c.s); got != c.want { // обход листов — то же, что VarsOf
			t.Errorf("SheetVars, %s: %+v, ждали %+v", name, got, c.want)
		}
	}
	if got := FieldsOf(plain, p.VarsOf(plain)); len(got) != 8 || got[3] != "Предмет" || got[4] != "Заголовок" || got[7] != "Руководитель" {
		t.Fatalf("меню полей с полями: %v", got)
	}
	if got := FieldsOf(empty, p.VarsOf(empty)); len(got) != 8 {
		t.Fatalf("у виньетки без полей меню шапки есть: %v", got)
	}
	// Свадебный альбом без виньеток: полей нет, меню — только имя, пустое поле — «[Заголовок]».
	w := New()
	w.Spreads = []*Spread{plain}
	if w.VarsOf(plain) != nil || w.SheetVars()(plain) != nil || !slices.Equal(FieldsOf(plain, nil), []string{"Фамилия", "Имя", "Отчество"}) {
		t.Fatalf("без виньеток: %+v %v", w.VarsOf(plain), FieldsOf(plain, nil))
	}
	if got, _ := Shown(w.VarsOf(plain), "{Школа}", nil, FieldCanvas); got != "[Заголовок]" {
		t.Fatalf("пустое поле: %q", got)
	}
	if FieldsOf(nil, a) == nil || len(FieldsOf(nil, a)) != 3 {
		t.Fatal("листа нет — только имя")
	}
	// Меню отдаёт копию: правка списка вызывающим не портит имена полей.
	FieldsOf(plain, nil)[0] = "x"
	if FieldsOf(plain, nil)[0] != "Фамилия" {
		t.Fatal("FieldsOf отдаёт общий массив")
	}
}

// HasHeadFields — поля шапки и прежние имена; имя на фото, неизвестное и сломанное — нет.
func TestHasHeadFields(t *testing.T) {
	for s, want := range map[string]bool{
		"{Заголовок}": true, "x {год} y": true, "{ Школа }": true, "{{Класс}}": true, "{Учитель": false,
		"{Фамилия}\n{Имя}": false, "{Цвет}": false, "": false, "{}{": false, "{Го{Руководитель}": true,
	} {
		if HasHeadFields(s) != want {
			t.Errorf("%q: ждали %v", s, want)
		}
	}
}

// CheckTotals на границе запаса знаков: поля шапки (480 знаков на лист) и подписи учителей виньеток
// (300 знаков на лист) — ровно в запас проходят, на один лист больше — отказ; Load (ClampDesign)
// оставляет ровно то, что пропускает CheckTotals.
func TestTextBudgetVarsAndTeacher(t *testing.T) {
	full := strings.Repeat("я", MaxVarRunes)
	vars := func() *Vars { return &Vars{Title: full, Subtitle: full, Year: full, Lead: full} }
	teacher := func() *Spread {
		return &Spread{ID: NewID(), Kind: KindVignette, Vig: &Vignette{Tpl: "grid", TText: strings.Repeat("я", MaxCaptionRunes)}}
	}
	plain := func() *Spread { return &Spread{ID: NewID(), Photos: []string{}, Vars: vars()} }
	for name, tc := range map[string]struct {
		mk      func() *Spread
		perItem int
	}{
		"поля шапки":      {plain, 4 * MaxVarRunes},
		"подпись учителя": {teacher, MaxCaptionRunes},
	} {
		fit := MaxTotalRunes / tc.perItem // столько листов умещается
		build := func(n int) *Project {
			p := New()
			for range n {
				p.Spreads = append(p.Spreads, tc.mk())
			}
			return p
		}
		if err := build(fit).CheckTotals(New()); err != nil {
			t.Errorf("%s: %d листов в запас не проходят: %v", name, fit, err)
		}
		over := build(fit + 1)
		if err := over.CheckTotals(New()); err == nil {
			t.Errorf("%s: %d листов сверх запаса прошли", name, fit+1)
		}
		over.ClampDesign()
		if err := over.CheckTotals(New()); err != nil {
			t.Errorf("%s: после зажима CheckTotals отказывает: %v", name, err)
		}
		left := 0
		for _, s := range over.Spreads {
			if s.Vars != nil || s.Vig != nil && s.Vig.TText != "" {
				left++
			}
		}
		if left != fit {
			t.Errorf("%s: после зажима осталось %d листов с текстом, ждали %d", name, left, fit)
		}
	}
}

// Поля имени шаблона: порядок появления, синонимы и регистр, без повторов, без полей — nil.
func TestNameFieldsOf(t *testing.T) {
	for in, want := range map[string][]string{
		"{Фамилия}\n{Имя}":           {"last", "first"},
		"{Имя} {ФАМИЛИЯ} {имя}":      {"first", "last"},
		"{Фамилия} {Имя} {Отчество}": {"last", "first", "middle"},
		"{Отчество}":                 {"middle"},
		"{Год} {Школа} {Заголовок}":  nil,
		"Просто текст":               nil,
		"{Неизвестное} { Имя":        nil,
		"":                           nil,
		"{Год} и {Фамилия} «{Имя}»":  {"last", "first"},
		"{{Имя}}":                    {"first"},
	} {
		if got := NameFieldsOf(in); !slices.Equal(got, want) || (want == nil && got != nil) {
			t.Errorf("%q: %v, ждали %v", in, got, want)
		}
	}
}

// Без «{» Expand отдаёт текст как есть и ничего не считает пустым.
func TestExpandNoBraces(t *testing.T) {
	out, missing := Expand("Просто текст", FieldCtx{})
	if out != "Просто текст" || missing != nil {
		t.Fatalf("%q %v", out, missing)
	}
	if HasFields("без скобок") || !HasFields("{") {
		t.Fatal("HasFields")
	}
	if out, miss := Shown(nil, "{Год}", nil, FieldPrint); out != "" || !slices.Equal(miss, []string{"Год"}) {
		t.Fatalf("Shown без проекта: %q %v", out, miss)
	}
}

// Обычный текст длиннее предела обрывает подстановку до первого поля.
func TestExpandPlainOverflow(t *testing.T) {
	in := strings.Repeat("я", MaxTextRunes+10) + "{Год}"
	out, missing := Expand(in, FieldCtx{Vars: Vars{Year: "2026"}})
	if []rune(out)[MaxTextRunes-1] != 'я' || len([]rune(out)) != MaxTextRunes || len(missing) != 0 {
		t.Fatalf("длина %d, пустых %v", len([]rune(out)), missing)
	}
}

// Вольное правило «Имён файлов» (043 сц. 6): регистр, «.» и дефис между словами, «Имя первым».
func TestPersonFromFileLoose(t *testing.T) {
	ivanov := &Person{Last: "Иванов", First: "Петя"}
	cases := []struct {
		path       string
		firstFirst bool
		want       *Person
	}{
		{`D:\x\иванов_петя.jpg`, false, ivanov},
		{`D:\x\ИВАНОВ ПЕТЯ.JPG`, false, ivanov},
		{`D:\x\Иванов-Петя.jpg`, false, ivanov},
		{`D:\x\Иванов.Петя.jpg`, false, ivanov},
		{`D:\x\01_иванов.петя (2).jpg`, false, ivanov},
		{`D:\x\петя иванов.jpg`, true, ivanov},
		{`D:\x\Petya Ivanov.jpg`, true, &Person{Last: "Ivanov", First: "Petya"}},
		{`D:\x\анна сергеевна петрова.jpg`, true, &Person{Last: "Петрова", First: "Анна", Middle: "Сергеевна"}},
		{`D:\x\Пётр Иванович Сидоров.jpg`, true, &Person{Last: "Сидоров", First: "Пётр", Middle: "Иванович"}}, // 3 слова, «Имя первым»
		{`D:\x\Петя-Иванов.jpg`, true, ivanov}, // через дефис, «Имя первым»
		{`D:\x\пётр-иванович-сидоров.jpg`, true, &Person{Last: "Сидоров", First: "Пётр", Middle: "Иванович"}},
		{`D:\x\Иванов-Пётр-Сергеевич.jpg`, false, &Person{Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}},
		{`D:\x\иванова-петрова анна.jpg`, false, &Person{Last: "Иванова-Петрова", First: "Анна"}},
		{`D:\x\МакКартни Пол.jpg`, false, &Person{Last: "МакКартни", First: "Пол"}},
		{`D:\x\Иванов-Петров-Сидоров-Козлов.jpg`, false, nil}, // 4 части — не делится
		{`D:\x\DSC_0012.jpg`, false, nil},
		{`D:\x\iPhone фото.jpg`, false, nil},
		{`D:\x\Свадьба 2025.jpg`, false, nil},
		{"", true, nil},
	}
	for _, c := range cases {
		got := PersonFromFileLoose(c.path, c.firstFirst)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("%q (%v) → %+v, ждали %+v", c.path, c.firstFirst, got, c.want)
		}
	}
	// Строгое правило вольные случаи не принимает: имя без окна не появляется само.
	for _, path := range []string{`D:\x\иванов_петя.jpg`, `D:\x\Иванов.Петя.jpg`, `D:\x\Иванов-Петя.jpg`} {
		if p := PersonFromFile(path); p != nil {
			t.Errorf("строгое %q → %+v", path, p)
		}
	}
}

// Слово имени с цифрой или знаком — не имя; слишком длинное имя файла — тоже.
func TestPersonFromFileRejects(t *testing.T) {
	for _, path := range []string{
		`D:\x\Иван1 Петя.jpg`,
		`D:\x\Иванов Пе#я.jpg`,
		`D:\x\` + strings.Repeat("Ы", 3*MaxPersonRunes+3) + ` Петя.jpg`,
		"D:\\x\\Иванов \xff\xfe.jpg",
	} {
		if p := PersonFromFile(path); p != nil {
			t.Errorf("%q → %+v, ждали nil", path, p)
		}
	}
	if p := PersonFromFile(`D:\x\Д’Артаньян Шарль.jpg`); p == nil || p.Last != "Д’Артаньян" {
		t.Fatalf("типографский апостроф: %+v", p)
	}
}

// NameView: имени нет вовсе — пустая карточка; своё и из файла различаются.
func TestNameView(t *testing.T) {
	if v := NameView(PhotoRef{Path: `D:\x\DSC_0001.jpg`}); v != (PersonView{}) {
		t.Fatalf("без имени: %+v", v)
	}
	if v := NameView(PhotoRef{Path: `D:\x\Иванов Петя.jpg`}); !v.FromFile || v.Last != "Иванов" {
		t.Fatalf("из файла: %+v", v)
	}
	if v := NameView(PhotoRef{Person: &Person{First: "Пётр"}}); v.FromFile || v.First != "Пётр" {
		t.Fatalf("своё: %+v", v)
	}
}

// Зажим при Load без запаса знаков: переменные и имя пропадают, у стиля листа теряется шаблон.
func TestClampFieldsNoBudget(t *testing.T) {
	b := textBudget{runes: 3}
	if clampVarsIn(&Vars{Title: "Школа № 5"}, &b) != nil || clampPersonIn(&Person{Last: "Иванов"}, &b) != nil {
		t.Fatal("сверх запаса переменные и имя должны пропасть")
	}
	if clampVarsIn(&Vars{}, &b) != nil || clampPersonIn(&Person{}, &b) != nil {
		t.Fatal("пустые — nil")
	}
	c := clampCapsIn(&SheetCaps{Text: "Шаблон {Фамилия}", Pad: 3}, func(s string) int { return runes(s) }, &b)
	if c == nil || c.Text != "" || c.Pad != 3 {
		t.Fatalf("стиль листа без запаса: %+v", c)
	}
	if clampCapsIn(nil, func(string) int { return 0 }, &b) != nil {
		t.Fatal("nil остаётся nil")
	}
}

// ClampArt: nil остаётся nil; ключ не по грамматике и ключ узора — nil; неизвестный, но верный
// ключ хранится; цвета копируются (правка копии не трогает исходный рисунок).
func TestClampArtEdges(t *testing.T) {
	if ClampArt(nil) != nil {
		t.Fatal("nil → nil")
	}
	for _, k := range []string{"", "BAD!", "dots"} {
		if ClampArt(&Art{Kind: k}) != nil {
			t.Errorf("ключ %q не рисунок", k)
		}
	}
	src := &Art{Kind: "zz-future", Inks: []string{"#ABCDEF"}}
	got := ClampArt(src)
	if got == nil || got.Kind != "zz-future" || got.Inks[0] != "#abcdef" || src.Inks[0] != "#ABCDEF" {
		t.Fatalf("неизвестный ключ: %+v, исходный %+v", got, src)
	}
}

// checkStickers: предел листа — отказ всегда (и у обложки), предел проекта — только если стало
// больше, чем было; переполненный файл не «замерзает». Несколько листов сразу.
func TestCheckStickersLimits(t *testing.T) {
	fill := func(s *Spread, n int) {
		for range n {
			s.Stickers = append(s.Stickers, &Sticker{ID: NewID(), Art: &Art{Kind: "plate"}})
		}
	}
	p := New()
	p.Spreads = []*Spread{{ID: "a"}, {ID: "b"}}
	fill(p.Spreads[0], MaxStickers)
	fill(p.Spreads[1], MaxStickers)
	if err := p.checkStickers(New()); err != nil {
		t.Fatalf("ровно предел на листе: %v", err)
	}
	fill(p.Spreads[1], 1)
	if err := p.checkStickers(New()); err != ErrStickers {
		t.Fatalf("лист сверх предела: %v", err)
	}
	p.Spreads[1].Stickers = p.Spreads[1].Stickers[:MaxStickers]
	p.Cover = &Cover{Sheet: &Spread{ID: "cover"}}
	fill(p.Cover.Sheet, MaxStickers+1)
	if err := p.checkStickers(New()); err != ErrStickers {
		t.Fatalf("обложка сверх предела: %v", err)
	}

	// Предел проекта: у каждого листа ≤ MaxStickers, всего > MaxTotalStickers.
	q := New()
	for i := 0; i <= MaxTotalStickers/MaxStickers; i++ {
		s := &Spread{ID: strconv.Itoa(i)}
		fill(s, MaxStickers)
		q.Spreads = append(q.Spreads, s)
	}
	if err := q.checkStickers(New()); err == nil || err == ErrStickers {
		t.Fatalf("сверх проекта: %v", err)
	}
	if err := q.checkStickers(q); err != nil {
		t.Fatalf("не больше, чем было: %v", err)
	}
}
