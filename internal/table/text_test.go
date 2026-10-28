package table

import (
	"reflect"
	"strings"
	"testing"
)

func TestSheetText(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows [][]string
		want string
	}{
		{"нет строк", nil, ""},
		{"одна ячейка", [][]string{{"Иванов"}}, "Иванов"},
		{"строки и ячейки", [][]string{{"Фамилия", "Имя"}, {"Иванов", "Пётр"}}, "Фамилия\tИмя\nИванов\tПётр"},
		{"пустая ячейка внутри остаётся", [][]string{{"а", "", "в"}}, "а\t\tв"},
		{"пустой хвост снят", [][]string{{"а", "б", "", ""}, {"в"}}, "а\tб\nв"},
		{"\\t \\r \\n в ячейке — пробел", [][]string{{"а\tб", "в\r\nг", "д\nе"}}, "а б\tв  г\tд е"},
		{"много строк", [][]string{{"1"}, {"2"}, {"3"}}, "1\n2\n3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Sheet{Rows: tc.rows}).Text(); got != tc.want {
				t.Fatalf("Text() = %q, ждали %q", got, tc.want)
			}
		})
	}
	// Text() не меняет таблицу (значения-копии, но строки общие).
	rows := [][]string{{"а", ""}}
	_ = Sheet{Rows: rows}.Text()
	if !reflect.DeepEqual(rows, [][]string{{"а", ""}}) {
		t.Fatalf("Text() изменил строки: %q", rows)
	}
	// Круг: Text() режется обратно по "\n" и "\t" в те же строки (names делит именно так).
	s, err := Read([]byte("Фамилия;Имя;Предмет\nИванов;Пётр;\nСидорова;Ольга;Математика\n"))
	if err != nil {
		t.Fatal(err)
	}
	var back [][]string
	for l := range strings.SplitSeq(s.Text(), "\n") {
		back = append(back, strings.Split(l, "\t"))
	}
	if want := [][]string{{"Фамилия", "Имя", "Предмет"}, {"Иванов", "Пётр"}, {"Сидорова", "Ольга", "Математика"}}; !reflect.DeepEqual(back, want) {
		t.Fatalf("круг Text(): %q", back)
	}
}
