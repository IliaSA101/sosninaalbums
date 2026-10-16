package namesfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/table"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const (
	nsMain = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// book — книга .xlsx из частей parts (имя части, содержимое).
func book(t *testing.T, parts [][2]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, p := range parts {
		f, err := w.Create(p[0])
		if err == nil {
			_, err = f.Write([]byte(p[1]))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// classBook — книга: первый лист скрыт, второй «Класс» — заголовок и одна строка
// (строки прямо в ячейках, inlineStr).
func classBook(t *testing.T) []byte {
	t.Helper()
	cell := func(ref, s string) string { return `<c r="` + ref + `" t="inlineStr"><is><t>` + s + `</t></is></c>` }
	return book(t, [][2]string{
		{"xl/workbook.xml", `<workbook ` + nsMain + ` xmlns:r="` + nsRel + `"><sheets>` +
			`<sheet name="Черновик" sheetId="1" state="hidden" r:id="rId1"/><sheet name="Класс" sheetId="2" r:id="rId2"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + nsRel + `/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="` + nsRel + `/worksheet" Target="/xl/worksheets/sheet2.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<worksheet ` + nsMain + `><sheetData><row r="1">` + cell("A1", "Скрыто") + `</row></sheetData></worksheet>`},
		{"xl/worksheets/sheet2.xml", `<worksheet ` + nsMain + `><sheetData><row r="1">` + cell("A1", "Фамилия") + cell("B1", "Имя") +
			`</row><row r="2">` + cell("A2", "Иванов") + cell("B2", "Пётр") + `</row></sheetData></worksheet>`},
	})
}

func putFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// xlsx и csv — текст и сведения; ошибки файла — словами для пользователя, без пути.
func TestRead(t *testing.T) {
	dir := testkit.TempDir(t)
	f, err := Read(putFile(t, dir, "Класс.xlsx", classBook(t)))
	want := File{Text: "Фамилия\tИмя\nИванов\tПётр", Name: "Класс.xlsx", Sheet: "Класс", Sheets: 2, Rows: 2}
	if err != nil || f != want {
		t.Fatalf("xlsx: %+v, %v", f, err)
	}
	// CSV из Excel под Windows: cp1251 и «;».
	csv := putFile(t, dir, "Класс.csv", []byte("\xd4\xe0\xec\xe8\xeb\xe8\xff;\xc8\xec\xff\r\n\xcf\xe5\xf2\xf0\xee\xe2\xe0;\xc0\xed\xed\xe0\r\n"))
	if f, err = Read(csv); err != nil || f != (File{Text: "Фамилия\tИмя\nПетрова\tАнна", Name: "Класс.csv", Rows: 2}) {
		t.Fatalf("csv: %+v, %v", f, err)
	}
	big := putFile(t, dir, "Большой.csv", bytes.Repeat([]byte("Иванов;Петя\n"), table.MaxFile/20)) // 9 МБ
	cases := map[string]error{
		big: table.ErrTooBig,
		putFile(t, dir, "Старый.xls", append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 512)...)): table.ErrOld,
		putFile(t, dir, "Пустой.csv", []byte(" \r\n\r\n")):                                                                  table.ErrEmpty,
		filepath.Join(dir, "Нет.xlsx"):                                                                                      ErrGone,
	}
	for path, want := range cases {
		if f, err = Read(path); !errors.Is(err, want) || f != (File{}) || strings.Contains(err.Error(), dir) {
			t.Errorf("%s: %+v, %v; ждали %v", filepath.Base(path), f, err, want)
		}
	}
}

// Таблица в пределах ячеек (1001×64×256 знаков), но с текстом в десятки МБ — одна общая строка
// на все ячейки, поэтому файл мал: окну её не отдаём (поле списка — 256 КБ), как и любую другую
// ошибку чтения — без текста, своей ошибкой ErrText (причина — не число строк и не размер файла).
func TestReadTextTooBig(t *testing.T) {
	dir := testkit.TempDir(t)
	row := `<row>` + strings.Repeat(`<c t="s"><v>0</v></c>`, table.MaxCols) + `</row>`
	bomb := book(t, [][2]string{
		{"xl/workbook.xml", `<workbook ` + nsMain + ` xmlns:r="` + nsRel + `"><sheets><sheet name="А" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/sharedStrings.xml", `<sst ` + nsMain + `><si><t>` + strings.Repeat("Ж", table.MaxCell) + `</t></si></sst>`},
		{"xl/worksheets/sheet1.xml", `<worksheet ` + nsMain + `><sheetData>` + strings.Repeat(row, table.MaxRows) + `</sheetData></worksheet>`},
	})
	if len(bomb) > table.MaxFile {
		t.Fatalf("книга %d байт: больше MaxFile, тест не о том", len(bomb))
	}
	f, err := Read(putFile(t, dir, "Бомба.xlsx", bomb))
	if !errors.Is(err, ErrText) || f != (File{}) {
		t.Fatalf("1001×64 длинных ячеек: %+v, %v", f, err)
	}
	// Причина названа верно: не «1000 строк и 8 МБ» (table.ErrTooBig), а размер текста.
	if errors.Is(err, table.ErrTooBig) || err.Error() != "в таблице слишком много текста: не больше 256 КБ" {
		t.Fatalf("текст ошибки: %q", err)
	}
	// Край: 1000 строк по 256 знаков (≈ 251 КБ) — ещё пролезает, две ячейки в строке — уже нет.
	cell := strings.Repeat("a", table.MaxCell)
	f, err = Read(putFile(t, dir, "Влезает.csv", []byte(strings.Repeat(cell+"\n", 1000))))
	if err != nil || f.Rows != 1000 || len(f.Text) > model.MaxNamesBytes {
		t.Fatalf("1000×256: %v, строк %d, байт %d", err, f.Rows, len(f.Text))
	}
	if f, err = Read(putFile(t, dir, "Не влезает.csv", []byte(strings.Repeat(cell+";"+cell+"\n", 1000)))); !errors.Is(err, ErrText) || f != (File{}) {
		t.Fatalf("1000×2×256: %+v, %v", f, err)
	}
}

// Ошибка системы с путём (ошибка открытия файла) уходит в лог без пути: в log.txt имя
// пользователя и названия альбомов не нужны; окно получает общие слова.
func TestUserErrLogHidesPath(t *testing.T) {
	const secret = `C:\Users\Имя\Классы\11А.xlsx`
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for _, err := range []error{
		&fs.PathError{Op: "read", Path: secret, Err: errors.New("устройство не готово")},
		fmt.Errorf("чтение: %w", &fs.PathError{Op: "open", Path: secret, Err: fs.ErrPermission}),
	} {
		buf.Reset()
		got := userErr(err)
		if got == nil || strings.Contains(got.Error(), "Имя") {
			t.Errorf("окно получило %v", got)
		}
		if out := buf.String(); out == "" || strings.Contains(out, "Имя") || strings.Contains(out, "11А") {
			t.Errorf("лог с путём или пустой: %q", out)
		}
	}
	// Известная ошибка table — без записи в лог.
	buf.Reset()
	if err := userErr(table.ErrBroken); !errors.Is(err, table.ErrBroken) || buf.Len() != 0 {
		t.Errorf("ErrBroken: %v, лог %q", err, buf.String())
	}
}
