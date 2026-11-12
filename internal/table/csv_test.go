package table

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
)

func cp1251(t testing.TB, s string) []byte {
	t.Helper()
	b, err := charmap.Windows1251.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// utf16Text — «Текст Юникод» из Excel: BOM и UTF-16 (le — порядок байт).
func utf16Text(s string, le bool) []byte {
	out := []byte{0xFE, 0xFF}
	if le {
		out = []byte{0xFF, 0xFE}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		if le {
			out = append(out, byte(u), byte(u>>8))
		} else {
			out = append(out, byte(u>>8), byte(u))
		}
	}
	return out
}

func TestReadCSV(t *testing.T) {
	const semi = "Фамилия;Имя;Отчество\r\nИванов;Пётр;\r\n\"Петрова; Анна\";Анна;Сергеевна\r\n"
	semiRows := [][]string{{"Фамилия", "Имя", "Отчество"}, {"Иванов", "Пётр"}, {"Петрова; Анна", "Анна", "Сергеевна"}}
	const tab = "Фамилия\tИмя\tПредмет\nИванов\tПётр\t\nСидорова\t\"Ольга; Ивановна\"\tМатематика\n"
	tabRows := [][]string{{"Фамилия", "Имя", "Предмет"}, {"Иванов", "Пётр"}, {"Сидорова", "Ольга; Ивановна", "Математика"}}
	for _, tc := range []struct {
		name string
		data []byte
		enc  string
		rows [][]string
	}{
		{"cp1251, «;»", cp1251(t, semi), "Windows-1251", semiRows},
		{"UTF-8 с BOM", []byte("\xEF\xBB\xBF" + semi), "UTF-8", semiRows},
		{"UTF-8 без BOM", []byte(semi), "UTF-8", semiRows},
		{"UTF-16LE, Tab", utf16Text(tab, true), "UTF-16", tabRows},
		{"UTF-16BE, Tab", utf16Text(tab, false), "UTF-16", tabRows},
		{"запятая", []byte("Фамилия,Имя\nИванов,Пётр\n"), "UTF-8", [][]string{{"Фамилия", "Имя"}, {"Иванов", "Пётр"}}},
		{"«;» в кавычках при Tab", []byte("Имя\tОпись\nПётр\t\"а;б,в\"\n"), "UTF-8", [][]string{{"Имя", "Опись"}, {"Пётр", "а;б,в"}}},
		{"Tab раньше «;» при равенстве", []byte("а\tб;в\nг\tд;е\n"), "UTF-8", [][]string{{"а", "б;в"}, {"г", "д;е"}}},
		{"пустые строки, пробелы, хвост", []byte("\n  Иванов ; Пётр ;;\n\n;;\n  ;\nПетрова;Анна\n\n"), "UTF-8", [][]string{{"Иванов", "Пётр"}, {"Петрова", "Анна"}}},
		{"одно слово без разделителя", []byte("Иванов Пётр\nПетрова Анна"), "UTF-8", [][]string{{"Иванов Пётр"}, {"Петрова Анна"}}},
		{"кривые кавычки", []byte("Иван\"ов;Пётр\n\"Петр;ова"), "UTF-8", [][]string{{"Иван\"ов", "Пётр"}, {"Петр;ова"}}},
		{"нечётный хвост UTF-16", append(utf16Text("а;б", true), 0x41), "UTF-16", [][]string{{"а", "б"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Read(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if s.Enc != tc.enc || s.Name != "" || s.Sheets != 0 || !reflect.DeepEqual(s.Rows, tc.rows) {
				t.Fatalf("Enc %q, лист %q/%d, строки %q", s.Enc, s.Name, s.Sheets, s.Rows)
			}
			checkSheet(t, s)
		})
	}
}

func TestReadCSVLimits(t *testing.T) {
	line := func(n int) string { return strings.Repeat("Иванов;Пётр\n", n) }
	if s, err := Read([]byte(line(MaxRows))); err != nil || len(s.Rows) != MaxRows {
		t.Fatalf("%d строк: %v", MaxRows, err)
	}
	if _, err := Read([]byte(line(MaxRows + 1))); !errors.Is(err, ErrTooBig) {
		t.Fatalf("%d строк: %v", MaxRows+1, err)
	}
	// Столбцы за MaxCols и знаки за MaxCell отбрасываются без ошибки.
	s, err := Read([]byte(strings.Repeat("а;", MaxCols+10) + "\n" + strings.Repeat("ж", 5000) + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Rows[0]) != MaxCols || len([]rune(s.Rows[1][0])) != MaxCell {
		t.Fatalf("столбцов %d, знаков %d", len(s.Rows[0]), len([]rune(s.Rows[1][0])))
	}
	checkSheet(t, s)
	// Мегабайтная ячейка читается; миллион пустых столбцов — значение за MaxCols отброшено, строка пуста.
	if _, err := Read([]byte(strings.Repeat("ж", 1<<20))); err != nil {
		t.Fatal(err)
	}
	if _, err := Read([]byte(strings.Repeat(";", 1<<20) + "я")); !errors.Is(err, ErrEmpty) {
		t.Fatalf("столбцы за пределом: %v", err)
	}
	// Двоичное: NUL, UTF-16 без метки (чередование нулей) — не таблица.
	for _, b := range [][]byte{{'a', 0, ';', 0, 'b', 0}, {0xFF, 0xFD, 0}} {
		if _, err := Read(b); !errors.Is(err, ErrBroken) {
			t.Errorf("%q: %v", b, err)
		}
	}
}
