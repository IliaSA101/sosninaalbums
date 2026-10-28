package table

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// classSheet — лист класса «как у Excel»: общие строки (rich text, фонетика), inlineStr,
// число в столбце «Фото», формула, логическое, ошибка, пустые ячейки и строки, ячейка без r.
const classSheet = `
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="inlineStr"><is><t>Предмет</t></is></c><c r="D1" t="s"><v>2</v></c></row>
<row r="2"><c r="A2" t="s"><v>3</v></c><c r="B2" t="str"><f>"Пётр"</f><v>Пётр</v></c><c r="C2"/><c r="D2"><v>14</v></c></row>
<row r="3"><c r="A3"/></row>
<row r="4"><c r="C4" t="inlineStr"><is><r><t>Мате</t></r><r><t xml:space="preserve">матика </t></r><rPh sb="0" eb="1"><t>ﾏﾃ</t></rPh></is></c></row>
<row r="5"><c t="s"><v>4</v></c><c t="b"><v>1</v></c><c t="b"><v>0</v></c><c t="e"><v>#N/A</v></c><c t="s"><v>99</v></c><c r="G5"><v>  1.5E-2 </v></c></row>
<row r="6"><c r="A6" t="s"><v>5</v></c></row>`

var classSST = sstXML(
	`<si><t>Фамилия</t></si>`,
	`<si><t>Имя</t></si>`,
	`<si><t>Фото</t></si>`,
	// rich text: куски склеиваются, фонетика пропадает
	`<si><r><rPr><b/></rPr><t>Иван</t></r><r><t>ов</t></r><rPh sb="0" eb="2"><t>イワノフ</t></rPh><phoneticPr fontId="1"/></si>`,
	`<si><t>  Петрова  </t></si>`,
	`<si><t xml:space="preserve">Строка
с переводом</t></si>`,
)

func TestReadXLSX(t *testing.T) {
	want := [][]string{
		{"Фамилия", "Имя", "Предмет", "Фото"},
		{"Иванов", "Пётр", "", "14"},
		{"", "", "Математика"},
		{"Петрова", "ИСТИНА", "ЛОЖЬ", "", "", "", "1.5E-2"},
		{"Строка\nс переводом"},
	}
	got, err := Read(book(t, sheetXML(classSheet), classSST))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Rows, want) || got.Name != "Класс" || got.Sheets != 1 || got.Enc != "" {
		t.Fatalf("получено %q, лист %q из %d, Enc %q", got.Rows, got.Name, got.Sheets, got.Enc)
	}
	checkSheet(t, got)
	// Text() не ломает таблицу: перевод строки внутри ячейки — пробел.
	if want := "Фамилия\tИмя\tПредмет\tФото\nИванов\tПётр\t\t14\n\t\tМатематика\nПетрова\tИСТИНА\tЛОЖЬ\t\t\t\t1.5E-2\nСтрока с переводом"; got.Text() != want {
		t.Fatalf("Text() = %q", got.Text())
	}
}

// Скрытый первый лист пропускается, но входит в число листов; Target бывает любым.
func TestReadXLSXSheetTargets(t *testing.T) {
	wb := workbookXML([3]string{"Служебный", "hidden", "rId1"}, [3]string{"Тайный", "veryHidden", "rId2"}, [3]string{"Список", "", "rId3"}, [3]string{"Ещё", "", "rId4"})
	other := sheetXML(`<row><c t="inlineStr"><is><t>не тот лист</t></is></c></row>`)
	main := sheetXML(`<row><c t="inlineStr"><is><t>Иванов Пётр</t></is></c></row>`)
	for _, tc := range []struct{ name, target, part string }{
		{"относительный", "worksheets/sheet3.xml", "xl/worksheets/sheet3.xml"},
		{"абсолютный", "/xl/worksheets/sheet3.xml", "xl/worksheets/sheet3.xml"},
		{"с точками", "../xl/worksheets/./sheet3.xml", "xl/worksheets/sheet3.xml"},
		{"регистр и ведущий слэш в zip", "worksheets/Sheet3.XML", "/XL/Worksheets/sheet3.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := zipOf(t,
				part{"xl/workbook.xml", wb},
				part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"}, [2]string{"rId2", "worksheets/sheet2.xml"}, [2]string{"rId3", tc.target}, [2]string{"rId4", "worksheets/sheet4.xml"})},
				part{"xl/worksheets/sheet1.xml", other}, part{"xl/worksheets/sheet2.xml", other}, part{tc.part, main}, part{"xl/worksheets/sheet4.xml", other})
			s, err := Read(data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.Rows, [][]string{{"Иванов Пётр"}}) || s.Name != "Список" || s.Sheets != 4 {
				t.Fatalf("получено %q, лист %q из %d", s.Rows, s.Name, s.Sheets)
			}
		})
	}
}

// Strict OOXML: другие пространства имён и префиксы элементов читаются так же.
func TestReadXLSXStrict(t *testing.T) {
	const ns, rel = "http://purl.oclc.org/ooxml/spreadsheetml/main", "http://purl.oclc.org/ooxml/officeDocument/relationships"
	data := zipOf(t,
		part{"xl/workbook.xml", `<x:workbook xmlns:x="` + ns + `" xmlns:q="` + rel + `"><x:sheets><x:sheet name="Строгий" sheetId="1" q:id="rId7"/></x:sheets></x:workbook>`},
		part{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://purl.oclc.org/ooxml/package/relationships"><Relationship Id="rId7" Target="worksheets/s.xml"/></Relationships>`},
		part{"xl/worksheets/s.xml", `<x:worksheet xmlns:x="` + ns + `"><x:sheetData><x:row><x:c r="B1" t="s"><x:v>0</x:v></x:c><x:c r="C1"><x:v>7</x:v></x:c></x:row></x:sheetData></x:worksheet>`},
		part{"xl/sharedStrings.xml", `<x:sst xmlns:x="` + ns + `"><x:si><x:t>Петрова Анна</x:t></x:si></x:sst>`})
	s, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"", "Петрова Анна", "7"}}; !reflect.DeepEqual(s.Rows, want) || s.Name != "Строгий" {
		t.Fatalf("получено %q, лист %q", s.Rows, s.Name)
	}
}

// Края: только пустое, нет видимых листов, оборванные части, лист с битой ссылкой.
func TestReadXLSXEdges(t *testing.T) {
	rels := part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})}
	sheet := part{"xl/worksheets/sheet1.xml", sheetXML(`<row><c t="inlineStr"><is><t>Иванов</t></is></c></row>`)}
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"лист без значений", book(t, sheetXML(`<row r="1"><c r="A1"/><c r="B1" t="s"><v>5</v></c></row><row/>`), ""), ErrEmpty},
		{"только пробелы", book(t, sheetXML(`<row><c t="inlineStr"><is><t>   </t></is></c></row>`), ""), ErrEmpty},
		{"все листы скрыты", zipOf(t, part{"xl/workbook.xml", workbookXML([3]string{"A", "hidden", "rId1"})}, rels, sheet), ErrNoSheet},
		{"нет листов", zipOf(t, part{"xl/workbook.xml", workbookXML()}, rels, sheet), ErrNoSheet},
		{"rId без связи", zipOf(t, part{"xl/workbook.xml", workbookXML([3]string{"A", "", "rId9"})}, rels, sheet), ErrNoSheet},
		{"части листа нет", zipOf(t, part{"xl/workbook.xml", workbookXML([3]string{"A", "", "rId1"})}, rels), ErrBroken},
		{"нет workbook.xml", zipOf(t, rels, sheet), ErrBroken},
		{"zip не книга", zipOf(t, part{"word/document.xml", "<w/>"}), ErrBroken},
		{"оборванный лист", book(t, `<worksheet><sheetData><row><c t="inlineStr"><is><t>Ив`, ""), ErrBroken},
		{"оборванные общие строки", book(t, sheetXML(`<row><c t="s"><v>0</v></c></row>`), `<sst><si><t>Ив`), ErrBroken},
		{"не XML", book(t, "{json}", ""), ErrEmpty},
		{"пустой zip", zipOf(t), ErrBroken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Read(tc.data); !errors.Is(err, tc.want) {
				t.Fatalf("ошибка %v, ждали %v", err, tc.want)
			}
		})
	}
}

// Столбцы за MaxCols и длинные ячейки обрезаются без ошибки.
func TestReadXLSXClamps(t *testing.T) {
	var row strings.Builder
	for i := range MaxCols + 6 {
		row.WriteString(`<c t="inlineStr"><is><t>x` + strings.Repeat("я", i%3) + `</t></is></c>`)
	}
	long := `<c t="inlineStr"><is><t>` + strings.Repeat("ж", 100000) + `</t></is></c>`
	s, err := Read(book(t, sheetXML(`<row>`+row.String()+`</row><row>`+long+`</row>`), ""))
	if err != nil {
		t.Fatal(err)
	}
	checkSheet(t, s)
	if len(s.Rows[0]) != MaxCols || len([]rune(s.Rows[1][0])) != MaxCell {
		t.Fatalf("столбцов %d, знаков %d", len(s.Rows[0]), len([]rune(s.Rows[1][0])))
	}
	// Столбец XFD (16384) отбрасывается, как и всё за MaxCols.
	if _, err := Read(book(t, sheetXML(`<row><c r="XFD1"><v>1</v></c></row>`), "")); !errors.Is(err, ErrEmpty) {
		t.Fatalf("ячейка за пределом: %v", err)
	}
}
