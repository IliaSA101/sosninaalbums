package table

import (
	"strings"
	"testing"
)

// fixedRels — связи книги для fuzz: workbook.xml, общие строки и лист мутируются по отдельности,
// иначе правка байта в сжатом zip ломала бы CRC и не доходила до разбора XML.
var fixedRels = relsXML([2]string{"rId1", "worksheets/sheet1.xml"})

// FuzzReadXLSX — три части книги мусором: без паники и зависания, а успешный разбор
// укладывается в пределы (checkSheet). В обычном прогоне — только затравки и корпус;
// искать руками: go test ./internal/table -fuzz FuzzReadXLSX -fuzztime 30s.
func FuzzReadXLSX(f *testing.F) {
	wb := workbookXML([3]string{"Скрытый", "hidden", "rId9"}, [3]string{"Класс", "", "rId1"})
	f.Add(wb, classSST, sheetXML(classSheet))
	f.Add(workbookXML([3]string{"Класс", "", "rId1"}), sstXML(`<si><t>Иванов</t></si>`), sheetXML(`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="inlineStr"><is><t>Пётр</t></is></c></row>`))
	f.Add(`<x:workbook xmlns:x="u" xmlns:q="v"><x:sheets><x:sheet name="С" q:id="rId1"/></x:sheets></x:workbook>`, `<x:sst xmlns:x="u"><x:si><x:t>Анна</x:t></x:si></x:sst>`, `<x:worksheet xmlns:x="u"><x:sheetData><x:row><x:c t="s"><x:v>0</x:v></x:c></x:row></x:sheetData></x:worksheet>`)
	f.Add(wb, "", `<row><c r="ZZZZZZZZ9"><v>1</v></c><c t="b"><v>1</v></c><c t="e"/><c t="s"><v>-1</v></c></row>`)
	f.Add("", "", "")
	// Бомбы: глубокая вложенность (с объявлениями пространств имён) и тысячи атрибутов в теге.
	deep := strings.Repeat("<x>", 100) + strings.Repeat(`<x xmlns:p="u">`, 100)
	attrs := strings.Repeat(` a="" xmlns:q=""`, 5000)
	f.Add(wb, sstXML(`<si>`+deep+`</si>`), sheetXML(deep))
	f.Add(wb, sstXML(`<si`+attrs+`/>`), sheetXML(`<row><c`+attrs+`/></row>`))
	f.Add(workbookXML([3]string{"Класс", "", "rId1"})+deep, "", sheetXML(`<row><c`+attrs+`><v>1</v></c></row>`))
	f.Fuzz(func(t *testing.T, workbook, sst, sheet string) {
		parts := []part{{"xl/workbook.xml", workbook}, {"xl/_rels/workbook.xml.rels", fixedRels}, {"xl/worksheets/sheet1.xml", sheet}}
		if sst != "" {
			parts = append(parts, part{"xl/sharedStrings.xml", sst})
		}
		if s, err := Read(zipOf(t, parts...)); err == nil {
			checkSheet(t, s)
		}
	})
}

// FuzzReadCSV — любые байты как файл: определение формата и кодировки, разбор CSV.
func FuzzReadCSV(f *testing.F) {
	for _, s := range []string{
		"Фамилия;Имя;Отчество\r\nИванов;Пётр;\r\n\"Петрова; Анна\";Анна;Сергеевна\r\n",
		"Фамилия\tИмя\tПредмет\nИванов\tПётр\t\n",
		"Фамилия,Имя\nИванов,Пётр\n\"кривая\n",
		"\n;;\n \n",
	} {
		f.Add([]byte(s))
		f.Add(append([]byte("\xEF\xBB\xBF"), s...))
		f.Add(cp1251(f, s))
		f.Add(utf16Text(s, true))
		f.Add(utf16Text(s, false))
	}
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte{0xD0, 0xCF, 0x11, 0xE0})
	f.Add(book(f, sheetXML(classSheet), classSST)) // zip тоже идёт через Read
	f.Fuzz(func(t *testing.T, data []byte) {
		if s, err := Read(data); err == nil {
			checkSheet(t, s)
		}
	})
}
