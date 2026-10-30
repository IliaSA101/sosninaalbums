package table

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// readText разбирает CSV, TSV и «Текст Юникод» (так Excel сохраняет Tab-список в UTF-16).
func readText(b []byte) (Sheet, error) {
	s, enc := decode(b)
	// NUL в тексте — признак бинарного файла (или UTF-16 без метки): имён там не найти.
	if strings.IndexByte(s, 0) >= 0 {
		return Sheet{}, ErrBroken
	}
	r := csv.NewReader(strings.NewReader(s))
	// Разделитель знает только пользователь; кавычки Excel ставит не всегда ровно.
	r.Comma, r.LazyQuotes, r.FieldsPerRecord = sniff(s), true, -1
	sh := Sheet{Enc: enc}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return sh, nil
		}
		if err != nil {
			return Sheet{}, ErrBroken
		}
		row := make([]string, 0, min(len(rec), MaxCols))
		for i, c := range rec {
			if i >= MaxCols {
				break
			}
			row = append(row, clean(c))
		}
		if row = trimRow(row); row == nil {
			continue
		}
		if len(sh.Rows) >= MaxRows {
			return Sheet{}, ErrTooBig
		}
		sh.Rows = append(sh.Rows, row)
	}
}

// decode переводит байты в UTF-8: метка (BOM) UTF-8/UTF-16, иначе валидный UTF-8, иначе
// Windows-1251 — так Excel под русской Windows сохраняет «CSV (разделители — запятые)».
func decode(b []byte) (string, string) {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return strings.ToValidUTF8(string(b[3:]), "�"), "UTF-8"
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}), bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		le := b[0] == 0xFF
		b = b[2:]
		u := make([]uint16, len(b)/2) // нечётный хвост — обрыв файла, отбрасывается
		for i := range u {
			if le {
				u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
			} else {
				u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
			}
		}
		return string(utf16.Decode(u)), "UTF-16"
	case utf8.Valid(b):
		return string(b), "UTF-8"
	}
	s, err := charmap.Windows1251.NewDecoder().Bytes(b)
	if err != nil { // cp1251 в Go декодирует любой байт; страховка на будущее
		return strings.ToValidUTF8(string(b), "�"), "UTF-8"
	}
	return string(s), "Windows-1251"
}

// sniff — разделитель по первой непустой строке: Tab, «;» или «,» (чаще всех; при равенстве
// — в этом порядке, Tab надёжнее: запятая и «;» бывают внутри имён и должностей).
func sniff(s string) rune {
	line := ""
	for l := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(l) != "" {
			line = l
			break
		}
	}
	best, n := '\t', strings.Count(line, "\t")
	for _, c := range []rune{';', ','} {
		if k := strings.Count(line, string(c)); k > n {
			best, n = c, k
		}
	}
	return best
}
