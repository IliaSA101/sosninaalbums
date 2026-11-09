package table

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	nsMain = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPkg  = "http://schemas.openxmlformats.org/package/2006/relationships"
)

type part struct{ name, body string }

// zipOf собирает zip в памяти; Deflate, как у Excel.
func zipOf(tb testing.TB, parts ...part) []byte {
	tb.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, p := range parts {
		f, err := w.Create(p.name)
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			tb.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		tb.Fatal(err)
	}
	return b.Bytes()
}

// workbookXML — книга из листов (имя, состояние, r:id); пространства имён обычные.
func workbookXML(sheets ...[3]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="` + nsMain + `" xmlns:r="` + nsRel + `"><sheets>`)
	for i, s := range sheets {
		b.WriteString(`<sheet name="` + s[0] + `" sheetId="` + string(rune('1'+i)) + `"`)
		if s[1] != "" {
			b.WriteString(` state="` + s[1] + `"`)
		}
		b.WriteString(` r:id="` + s[2] + `"/>`)
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}

// relsXML — связи книги: пары (Id, Target).
func relsXML(pairs ...[2]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="` + nsPkg + `">`)
	for _, p := range pairs {
		b.WriteString(`<Relationship Id="` + p[0] + `" Type="` + nsRel + `/worksheet" Target="` + p[1] + `"/>`)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

func sstXML(items ...string) string {
	return `<sst xmlns="` + nsMain + `">` + strings.Join(items, "") + `</sst>`
}

func sheetXML(rows string) string {
	return `<worksheet xmlns="` + nsMain + `"><sheetData>` + rows + `</sheetData></worksheet>`
}

// book — книга с одним видимым листом «Класс»; sst может быть пустым (части нет).
func book(tb testing.TB, sheet, sst string) []byte {
	tb.Helper()
	parts := []part{
		{"xl/workbook.xml", workbookXML([3]string{"Класс", "", "rId1"})},
		{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})},
		{"xl/worksheets/sheet1.xml", sheet},
	}
	if sst != "" {
		parts = append(parts, part{"xl/sharedStrings.xml", sst})
	}
	return zipOf(tb, parts...)
}

// checkSheet — инварианты любого успешного разбора, на них же стоят fuzz-тесты.
func checkSheet(tb testing.TB, s Sheet) {
	tb.Helper()
	if len(s.Rows) == 0 || len(s.Rows) > MaxRows {
		tb.Fatalf("строк %d вне 1…%d", len(s.Rows), MaxRows)
	}
	for i, row := range s.Rows {
		if len(row) == 0 || len(row) > MaxCols || row[len(row)-1] == "" {
			tb.Fatalf("строка %d: %q (пустая, длиннее MaxCols или с пустым хвостом)", i, row)
		}
		for _, c := range row {
			if !utf8.ValidString(c) || utf8.RuneCountInString(c) > MaxCell || c != strings.TrimSpace(c) {
				tb.Fatalf("строка %d: плохая ячейка %q", i, c)
			}
		}
	}
	if s.Sheets < 0 || (s.Sheets == 0) != (s.Enc != "") {
		tb.Fatalf("Sheets %d и Enc %q не сходятся", s.Sheets, s.Enc)
	}
	if t := s.Text(); strings.Count(t, "\n") != len(s.Rows)-1 || strings.Contains(t, "\r") {
		tb.Fatalf("Text() ломает строки: %q", t)
	}
}
