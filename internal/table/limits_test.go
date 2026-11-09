package table

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// rowsXML — n строк по одной ячейке-числу.
func rowsXML(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, `<row><c><v>%d</v></c></row>`, i+1)
	}
	return b.String()
}

// within проверяет, что Read уложился в предел и вернул ожидаемую ошибку.
func within(t *testing.T, base time.Duration, data []byte, want error) {
	t.Helper()
	// Лучший из трёх: всплеск нагрузки портит один прогон, а взрывной разбор медленен в каждом.
	var err error
	d := testkit.Fastest(3, func() { _, err = Read(data) })
	if lim := testkit.Limit(base); d > lim {
		t.Errorf("Read занял %v, предел %v", d, lim)
	}
	if !errors.Is(err, want) {
		t.Fatalf("ошибка %v, ждали %v", err, want)
	}
}

// padding — n байт пробелов, разбитых пустыми тегами: пробельный текст длиннее MaxToken
// был бы одним токеном, а тесту нужен ровный размер части, а не проверка токена.
func padding(n int) string {
	const chunk = MaxToken / 2
	var b strings.Builder
	for n > chunk+4 {
		b.WriteString(strings.Repeat(" ", chunk) + "<a/>")
		n -= chunk + 4
	}
	b.WriteString(strings.Repeat(" ", n))
	return b.String()
}

// bombBook — книга, где лист из total байт пробелов: в zip это сотни КБ.
func bombBook(t *testing.T, total int) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	add := func(name string, body string) {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("xl/workbook.xml", workbookXML([3]string{"A", "", "rId1"}))
	add("xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"}))
	f, err := w.Create("xl/worksheets/sheet1.xml") // кусками: 200 МБ в памяти тесту не нужны
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte(" "), 1<<20)
	if _, err := f.Write([]byte("<worksheet><sheetData>")); err != nil {
		t.Fatal(err)
	}
	for range total >> 20 {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// skipBig — большие наборы пропускаются на быстром уровне (-short: precheck и цикл исполнителя).
func skipBig(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("большой набор: полный прогон")
	}
}

func TestReadXLSXLimits(t *testing.T) {
	t.Run("бомба 200 МБ", func(t *testing.T) {
		skipBig(t)
		data := bombBook(t, 200<<20)
		if len(data) > MaxFile {
			t.Fatalf("бомба не сжалась: %d байт", len(data))
		}
		within(t, time.Second, data, ErrTooBig)
	})
	t.Run("часть ровно MaxPart", func(t *testing.T) {
		skipBig(t)
		// Предел — «не больше», ровно на пределе (пробелы после корня) ещё не ошибка.
		sheet := sheetXML(`<row><c t="inlineStr"><is><t>Иванов</t></is></c></row>`)
		sheet += padding(MaxPart - len(sheet))
		s, err := Read(book(t, sheet, ""))
		if err != nil || len(s.Rows) != 1 {
			t.Fatalf("на пределе: %v, %q", err, s.Rows)
		}
		if _, err := Read(book(t, sheet+" ", "")); !errors.Is(err, ErrTooBig) {
			t.Fatalf("предел+1: %v", err)
		}
	})
	t.Run("строки", func(t *testing.T) {
		s, err := Read(book(t, sheetXML(rowsXML(MaxRows)), ""))
		if err != nil || len(s.Rows) != MaxRows {
			t.Fatalf("%d строк: %v", MaxRows, err)
		}
		within(t, time.Second, book(t, sheetXML(rowsXML(MaxRows+1)), ""), ErrTooBig)
		within(t, time.Second, book(t, sheetXML(rowsXML(100000)), ""), ErrTooBig)
	})
	t.Run("части zip", func(t *testing.T) {
		parts := func(n int) []part {
			ps := []part{
				{"xl/workbook.xml", workbookXML([3]string{"A", "", "rId1"})},
				{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})},
				{"xl/worksheets/sheet1.xml", sheetXML(`<row><c><v>1</v></c></row>`)},
			}
			for i := len(ps); i < n; i++ {
				ps = append(ps, part{fmt.Sprintf("docProps/p%d.xml", i), ""})
			}
			return ps
		}
		if _, err := Read(zipOf(t, parts(MaxEntries)...)); err != nil {
			t.Fatalf("%d частей: %v", MaxEntries, err)
		}
		within(t, time.Second, zipOf(t, parts(MaxEntries+1)...), ErrTooBig)
	})
	t.Run("общие строки", func(t *testing.T) {
		skipBig(t)
		within(t, time.Second, book(t, sheetXML(`<row><c t="s"><v>0</v></c></row>`), sstXML(strings.Repeat(`<si/>`, MaxStrings+1))), ErrTooBig)
		s, err := Read(book(t, sheetXML(`<row><c t="s"><v>0</v></c></row>`), sstXML(`<si><t>Иванов</t></si>`+strings.Repeat(`<si/>`, MaxStrings-1))))
		if err != nil || s.Rows[0][0] != "Иванов" {
			t.Fatalf("%d общих строк: %v", MaxStrings, err)
		}
	})
	t.Run("служебная часть больше MaxMeta", func(t *testing.T) {
		wb := workbookXML([3]string{"A", "", "rId1"}) + strings.Repeat(" ", MaxMeta)
		data := zipOf(t, part{"xl/workbook.xml", wb}, part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})}, part{"xl/worksheets/sheet1.xml", sheetXML(rowsXML(1))})
		within(t, time.Second, data, ErrTooBig)
	})
	t.Run("300 000 ячеек за столбцом XFD", func(t *testing.T) {
		skipBig(t)
		cells := strings.Repeat(`<c r="XFD1"><v>1</v></c>`, 300000)
		within(t, 2*time.Second, book(t, sheetXML(`<row>`+cells+`</row>`), ""), ErrEmpty)
	})
	t.Run("вложенность 10^6", func(t *testing.T) {
		// Глубже MaxDepth — не лист Excel (см. TestXMLStreamLimits), а не «пустая таблица».
		deep := strings.Repeat("<x>", 1000000) + strings.Repeat("</x>", 1000000)
		within(t, 3*time.Second, book(t, sheetXML(deep), ""), ErrBroken)
	})
	t.Run("сигнатуры и мусор", func(t *testing.T) {
		bin := make([]byte, 4096)
		for i := range bin {
			bin[i] = byte(i * 7)
		}
		for _, tc := range []struct {
			name string
			data []byte
			want error
		}{
			{"OLE: .xls и книга с паролем", append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, bin...), ErrOld},
			{"PK без архива", append([]byte("PK\x03\x04"), bin...), ErrBroken},
			{"двоичный мусор", bin, ErrBroken},
			{"пустой файл", nil, ErrEmpty},
			{"только пробелы", []byte(" \r\n\t\r\n"), ErrEmpty},
			{"> MaxFile", make([]byte, MaxFile+1), ErrTooBig},
			{"CSV > MaxText", bytes.Repeat([]byte("а;б\n"), MaxText/4+1), ErrTooBig},
		} {
			if _, err := Read(tc.data); !errors.Is(err, tc.want) {
				t.Errorf("%s: ошибка %v, ждали %v", tc.name, err, tc.want)
			}
		}
	})
}

func TestReadFile(t *testing.T) {
	dir := testkit.TempDir(t)
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Круг «файл → таблица»: то, что записали, читается теми же строками.
	s, err := ReadFile(write("класс.csv", []byte("Фамилия;Имя\nИванов;Пётр\n")))
	if err != nil || s.Text() != "Фамилия\tИмя\nИванов\tПётр" {
		t.Fatalf("csv: %v, %q", err, s.Text())
	}
	if s, err = ReadFile(write("класс.xlsx", book(t, sheetXML(`<row><c t="inlineStr"><is><t>Иванов</t></is></c></row>`), ""))); err != nil || s.Name != "Класс" {
		t.Fatalf("xlsx: %v, %q", err, s.Name)
	}
	// Расширение ничего не решает: xlsx под именем .txt читается как xlsx.
	if s, err = ReadFile(write("переименован.txt", book(t, sheetXML(rowsXML(2)), ""))); err != nil || s.Sheets != 1 {
		t.Fatalf("переименованный xlsx: %v", err)
	}
	// Файл больше MaxFile отклоняется по размеру, без чтения в память.
	if _, err := ReadFile(write("большой.xlsx", make([]byte, MaxFile+1<<20))); !errors.Is(err, ErrTooBig) {
		t.Fatalf("9 МБ: %v", err)
	}
	if _, err := ReadFile(write("пустой.csv", nil)); !errors.Is(err, ErrEmpty) {
		t.Fatalf("пустой файл: %v", err)
	}
	if _, err := ReadFile(filepath.Join(dir, "нет.csv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("нет файла: %v", err)
	}
	if _, err := ReadFile(dir); err == nil {
		t.Fatal("папка прочиталась как таблица")
	}
}
