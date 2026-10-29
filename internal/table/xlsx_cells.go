package table

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// text собирает все <t> внутри элемента end (si, is): куски rich text <r> идут подряд,
// а фонетика <rPh> (подсказка чтения для японского) в имя не входит.
func text(d *stream, end string) (string, error) {
	var b strings.Builder
	in, ph := false, 0
	for {
		t, err := d.Token()
		if err != nil {
			return "", wrap(err)
		}
		switch v := t.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "t":
				in = true
			case "rPh":
				ph++
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "t":
				in = false
			case "rPh":
				ph--
			case end:
				return b.String(), nil
			}
		case xml.CharData:
			// Дальше MaxCell знаков строка всё равно обрежется: не копим мегабайты.
			if room := MaxCell*4 - b.Len(); in && ph == 0 && room > 0 {
				b.Write(v[:min(len(v), room)])
			}
		}
	}
}

// sharedStrings читает общие строки книги (в ячейках типа s стоит индекс в этом списке).
func sharedStrings(f *zip.File) ([]string, error) {
	d, c, err := openXML(f, MaxPart)
	if err != nil {
		return nil, err
	}
	defer c.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	var out []string
	for {
		t, e := d.Token()
		if e == io.EOF {
			return out, nil
		}
		if e != nil {
			return nil, wrap(e)
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == "si" {
			if len(out) >= MaxStrings {
				return nil, ErrTooBig
			}
			s, err := text(d, "si")
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
	}
}

// col — номер столбца из ссылки «AB12» (0 — A); -1 — ссылки нет. Буквы без предела дали бы
// переполнение, а столбцы за MaxCols всё равно отбрасываются.
func col(ref string) int {
	n, i := 0, 0
	for ; i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z'; i++ {
		n = n*26 + int(ref[i]-'A'+1)
		if n > 1<<20 {
			return 1 << 20
		}
	}
	if i == 0 {
		return -1
	}
	return n - 1
}

// sheetReader копит строки листа; next — столбец ячейки без ссылки r (следующий за предыдущей).
type sheetReader struct {
	d    *stream
	sst  []string
	rows [][]string
	row  []string
	next int
}

// addCell читает ячейку <c> и кладёт значение в текущую строку; за MaxCols и пустое — отбрасывает.
func (r *sheetReader) addCell(se xml.StartElement) error {
	ci := col(attr(se, "r"))
	if ci < 0 {
		ci = r.next
	}
	r.next = ci + 1
	val, err := cell(r.d, attr(se, "t"), r.sst)
	if err != nil {
		return err
	}
	if val = clean(val); ci < MaxCols && val != "" {
		for len(r.row) <= ci {
			r.row = append(r.row, "")
		}
		r.row[ci] = val
	}
	return nil
}

// endRow закрывает строку: пустые не считаются, 1002-я непустая — ErrTooBig.
func (r *sheetReader) endRow() error {
	if len(r.row) == 0 {
		return nil
	}
	if len(r.rows) >= MaxRows {
		return ErrTooBig
	}
	r.rows = append(r.rows, r.row)
	r.row = nil
	return nil
}

// sheetRows читает строки листа.
func sheetRows(f *zip.File, sst []string) ([][]string, error) {
	d, c, err := openXML(f, MaxPart)
	if err != nil {
		return nil, err
	}
	defer c.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	r := &sheetReader{d: d, sst: sst}
	for {
		t, e := d.Token()
		if e == io.EOF {
			return r.rows, nil
		}
		if e != nil {
			return nil, wrap(e)
		}
		switch v := t.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "row":
				r.row, r.next = nil, 0
			case "c":
				err = r.addCell(v)
			}
		case xml.EndElement:
			if v.Name.Local == "row" {
				err = r.endRow()
			}
		}
		if err != nil {
			return nil, err
		}
	}
}

// cell читает ячейку до её конца. Формулы — сохранённый результат <v>, сама формула не нужна.
func cell(d *stream, typ string, sst []string) (string, error) {
	var v strings.Builder
	inV := false
	for {
		t, err := d.Token()
		if err != nil {
			return "", wrap(err)
		}
		switch x := t.(type) {
		case xml.StartElement:
			switch x.Name.Local {
			case "is": // inlineStr: текст прямо в ячейке, устроен как <si>
				s, err := text(d, "is")
				if err != nil {
					return "", err
				}
				return s, skipTo(d, "c")
			case "v":
				inV = true
			}
		case xml.CharData:
			if room := MaxCell*4 - v.Len(); inV && room > 0 {
				v.Write(x[:min(len(x), room)])
			}
		case xml.EndElement:
			switch x.Name.Local {
			case "v":
				inV = false
			case "c":
				return value(typ, v.String(), sst), nil
			}
		}
	}
}

// skipTo читает поток до закрывающего тега end.
func skipTo(d *stream, end string) error {
	for {
		t, err := d.Token()
		if err != nil {
			return wrap(err)
		}
		if e, ok := t.(xml.EndElement); ok && e.Name.Local == end {
			return nil
		}
	}
}

// value — значение ячейки по типу t: s — общая строка, b — логическое, e — ошибка формулы
// (в имени она ни к чему), остальное (n, d, str, без типа) — текст <v> как есть.
func value(typ, v string, sst []string) string {
	switch typ {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || i < 0 || i >= len(sst) {
			return ""
		}
		return sst[i]
	case "b":
		if strings.TrimSpace(v) == "1" {
			return "ИСТИНА"
		}
		return "ЛОЖЬ"
	case "e":
		return ""
	}
	return v
}
