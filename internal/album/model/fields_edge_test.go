package model

import (
	"slices"
	"strings"
	"testing"
)

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
