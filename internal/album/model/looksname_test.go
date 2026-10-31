package model

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// 060 (А): таблица «да/нет» разборов A §5 и B §4 и края правила.
func TestLooksLikeName(t *testing.T) {
	yes := map[string]Person{
		`D:\x\01_иванов_петя.jpg`:                  {Last: "Иванов", First: "Петя"},
		`D:\x\ИВАНОВ ПЕТЯ.jpg`:                     {Last: "Иванов", First: "Петя"},
		`D:\x\ИВАНОВ ПЕТЯ (2).jpg`:                 {Last: "Иванов", First: "Петя"},
		`D:\x\12. Сидоров Ваня (2).jpg`:            {Last: "Сидоров", First: "Ваня"},
		`D:\x\ivanov_petya.JPG`:                    {Last: "Ivanov", First: "Petya"},
		`D:\x\ivanov petya.jpg`:                    {Last: "Ivanov", First: "Petya"},
		`D:\x\Иванова-Петрова Анна Сергеевна.jpg`:  {Last: "Иванова-Петрова", First: "Анна", Middle: "Сергеевна"},
		`D:\x\Иванова-Петрова Анна.jpg`:            {Last: "Иванова-Петрова", First: "Анна"},
		`D:\x\Иванов.Петя.jpg`:                     {Last: "Иванов", First: "Петя"},
		`/home/x/O'Brien Sean.png`:                 {Last: "O'Brien", First: "Sean"},
		`D:\x\Иванов Петя.jpg`:                     {Last: "Иванов", First: "Петя"},
		`D:\x\Ёлкина Алёна-2.jpg`:                  {Last: "Ёлкина", First: "Алёна"},
		`Иванов Ив`:                                {Last: "Иванов", First: "Ив"}, // слово из 2 букв — край
		`D:\x\003 - Петров Пётр Петрович (12).jpg`: {Last: "Петров", First: "Пётр", Middle: "Петрович"},
	}
	for path, want := range yes {
		got := LooksLikeName(path)
		if got == nil || *got != want {
			t.Errorf("%q: %+v, ждали %+v", path, got, want)
		}
	}
	no := []string{
		`D:\x\IMG_1234.jpg`, `D:\x\P1038054.JPG`, `D:\x\DSC_0012.jpg`, `D:\x\_MG_5678.CR2`, `D:\x\DSC.jpg`,
		`D:\x\Свадьба Ивановых.jpg`, `D:\x\Первый танец.jpg`, `D:\x\первый танец.jpg`, `D:\x\wedding day.jpg`,
		`D:\x\Иванов.jpg`, `D:\x\Ivanов Петя.jpg`, `D:\x\Иванов Petya.jpg`, `D:\x\Санкт-Петербург.jpg`,
		`D:\x\Иванов П.jpg`, `D:\x\001.jpg`, `D:\x\Свадьба 2025.jpg`, `D:\x\Я.jpg`, `D:\x\Ab Cd1.jpg`,
		`D:\x\Иванов Петя Сергеевич Младший.jpg`, `D:\x\Фото Иванова.jpg`, `D:\x\IMG Иванов.jpg`,
		`D:\x\Букет невесты.jpg`, `D:\x\11 класс.jpg`, `D:\x\Выпускной 2025.jpg`, `D:\x\Общее фото.jpg`,
		`D:\x\Фото-Иванов Петя.jpg`, `D:\x\иВанов петя.jpg`, `D:\x\Иванов--Петя Анна.jpg`,
		`D:\x\Ἀλέξανδρος Παπαδόπουλος.jpg`, `D:\x\Иванов 🙂.jpg`, ``, `D:\x\.jpg`, `D:\x\ _ .jpg`,
		`D:\x\Иванов` + "\xff" + ` Петя.jpg`, `D:\x\` + strings.Repeat("Ы", 3*MaxPersonRunes+3) + ` Петя.jpg`,
		`D:\x\` + strings.Repeat("ы", MaxPersonRunes+1) + ` Петя.jpg`, // слово длиннее части имени
	}
	for _, path := range no {
		if got := LooksLikeName(path); got != nil {
			t.Errorf("%q: %+v, ждали nil", path, *got)
		}
	}
}

// Стоп-список — одно место правила: каждое слово, в любом регистре и частью через дефис, гасит
// имя, которое без него прошло бы.
func TestLooksStop(t *testing.T) {
	if len(looksStop) < 40 {
		t.Fatalf("стоп-список: %d слов, ждали ≥ 40 (A §5 + B §4)", len(looksStop))
	}
	for w := range looksStop {
		if w != strings.ToLower(w) || strings.ContainsAny(w, " -_") {
			t.Errorf("слово стоп-списка %q: только строчные, одно слово", w)
		}
		// Пара в письменности слова: без стоп-слова такое имя прошло бы.
		pair := "петров"
		if scriptOf([]rune(w)[0]) == scriptOf('a') {
			pair = "petrov"
		}
		if LooksLikeName(`D:\x\`+pair+" "+pair+".jpg") == nil {
			t.Fatalf("%q: основа без стоп-слова не прошла", pair)
		}
		for _, path := range []string{
			`D:\x\` + w + "_" + pair + ".jpg",
			`D:\x\01 ` + strings.ToUpper(w) + " " + strings.ToUpper(pair) + ".jpg",
			`D:\x\` + pair + " " + w + "-" + pair + ".jpg",
			`D:\x\` + pair + " " + pair + " " + w + ".jpg",
		} {
			if got := LooksLikeName(path); got != nil {
				t.Errorf("%q: %+v, стоп-слово %q не сработало", path, *got, w)
			}
		}
	}
}

// Что бы ни пришло: без паники; имя — 2–3 слова по правилу nameWord, одна письменность, без
// стоп-слов, и вольный разбор 043 (PersonFromFileLoose) даёт то же — LooksLikeName только строже.
func FuzzLooksLikeName(f *testing.F) {
	for _, s := range []string{
		`D:\x\01_иванов_петя.jpg`, `IMG_1234`, `Иванова-Петрова Анна Сергеевна`, `ivanov.petya (2)`,
		`Ivanов Петя`, `Санкт-Петербург`, `12. Сидоров Ваня (2)`, "\xff\xfe", `Иванов--Петя`, `O'Brien Sean`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		got := LooksLikeName(path)
		if got == nil {
			return
		}
		if again := LooksLikeName(path); again == nil || *again != *got {
			t.Fatalf("%q: разный итог %+v / %+v", path, got, again)
		}
		loose := PersonFromFileLoose(path, false)
		if loose == nil || *loose != *got {
			t.Fatalf("%q: %+v, а вольный разбор — %+v", path, *got, loose)
		}
		if got.Subject != "" {
			t.Fatalf("%q: предмет %q", path, got.Subject)
		}
		words := []string{got.Last, got.First}
		if got.Middle != "" {
			words = append(words, got.Middle)
		}
		if bad := badWords(words); bad != "" {
			t.Fatalf("%q: %s в %+v", path, bad, *got)
		}
	})
}

// badWords — чем слова имени нарушают правило LooksLikeName; "" — ничем.
func badWords(words []string) string {
	var script *unicode.RangeTable
	for _, w := range words {
		if !utf8.ValidString(w) || !nameWord(w) || stopWord(w) || utf8.RuneCountInString(w) < 2 {
			return "слово " + w + " не по правилу"
		}
		for _, r := range w {
			s := scriptOf(r)
			if s == nil && unicode.IsLetter(r) || s != nil && script != nil && s != script {
				return "смесь письменностей"
			}
			if s != nil {
				script = s
			}
		}
	}
	return ""
}

// Имена файлов с macOS (NFD: «й» = «и» + U+0306, «ё» = «е» + U+0308) разбираются как набранные
// с клавиатуры (decisions 31): основа имени приводится к NFC — во всех трёх разборах.
func TestNamesNFC(t *testing.T) {
	const breve, diaer = "\u0306", "\u0308"
	cases := map[string]Person{
		`D:\x\Иванов Андреи` + breve + `.jpg`:                 {Last: "Иванов", First: "Андрей"},
		`/Users/x/Але` + diaer + `на Петрова.jpg`:             {Last: "Алёна", First: "Петрова"},
		`D:\x\Е` + diaer + `лкина Але` + diaer + `на (2).jpg`: {Last: "Ёлкина", First: "Алёна"},
		`D:\x\Сергеи` + breve + ` Петров Сергеевич.jpg`:       {Last: "Сергей", First: "Петров", Middle: "Сергеевич"},
		`D:\x\И` + breve + `орданов Пётр.jpg`:                 {Last: "Йорданов", First: "Пётр"},
		`D:\x\І` + diaer + `жакевич Максим.jpg`:               {Last: "Їжакевич", First: "Максим"},
		`D:\x\Кавалеу` + breve + `ски Иван.jpg`:               {Last: "Кавалеўски", First: "Иван"},
		`D:\x\У` + breve + `ладзімір Іваноу` + breve + `.jpg`: {Last: "Ўладзімір", First: "Іваноў"},
	}
	for path, want := range cases {
		if got := LooksLikeName(path); got == nil || *got != want {
			t.Errorf("LooksLikeName %q: %+v, ждали %+v", path, got, want)
		}
		if got := PersonFromFile(path); got == nil || *got != want {
			t.Errorf("PersonFromFile %q: %+v, ждали %+v", path, got, want)
		}
		if got := PersonFromFileLoose(path, false); got == nil || *got != want {
			t.Errorf("PersonFromFileLoose %q: %+v, ждали %+v", path, got, want)
		}
	}
	// Знак без буквы перед ним — не буква и после NFC: такое не имя.
	if got := LooksLikeName(`D:\x\Иванов ` + breve + `Петя.jpg`); got != nil {
		t.Errorf("знак без буквы: %+v", *got)
	}
}
