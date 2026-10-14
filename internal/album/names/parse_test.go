package names

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func TestParse(t *testing.T) {
	got, err := Parse(" 1. Сидоров Иван\n\n12) Петрова Анна Сергеевна Мария\r\nКозлов;Пётр\n\tОдин\t\n3.", false)
	want := []model.Person{
		{Last: "Сидоров", First: "Иван"},
		{Last: "Петрова", First: "Анна", Middle: "Сергеевна Мария"},
		{Last: "Козлов", First: "Пётр"},
		{Last: "Один"},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("разбор: %+v, %v", got, err)
	}
	got, _ = Parse("Иван Сидоров\nАнна Сергеевна Петрова\nОля", true)
	want = []model.Person{{First: "Иван", Last: "Сидоров"}, {First: "Анна", Middle: "Сергеевна", Last: "Петрова"}, {First: "Оля"}}
	if !slices.Equal(got, want) {
		t.Fatalf("имя первым: %+v", got)
	}
	// Ровно 1000 строк (с переводом строки в конце) — можно, 1001 и 64 КБ+ — нет.
	if _, err := Parse(strings.Repeat("А Б\n", MaxPasteLines), false); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(strings.Repeat("А Б\n", MaxPasteLines+1), false); err == nil {
		t.Fatal("1001 строка")
	}
	if _, err := Parse(strings.Repeat("я", MaxPasteBytes), false); err == nil {
		t.Fatal("больше 64 КБ")
	}
	// Длинное имя режется ClampPerson.
	got, _ = Parse(strings.Repeat("Ф", 500)+" Имя", false)
	if len(got) != 1 || len([]rune(got[0].Last)) != model.MaxPersonRunes {
		t.Fatalf("длинное: %d знаков", len([]rune(got[0].Last)))
	}
}
