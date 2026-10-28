package mcpbridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// headKeep — сколько начала длинной строки хранится для поиска id; остальное читается и
// отбрасывается без хранения.
const headKeep = 64 << 10

// lineReader читает строки с пределом: память не больше max + буфер, как бы ни была длинна
// строка. Предел проверяется до накопления, а не после.
type lineReader struct {
	br  *bufio.Reader
	max int
}

func newLineReader(r io.Reader, limit int) *lineReader {
	return &lineReader{br: bufio.NewReaderSize(r, 64<<10), max: limit}
}

// next возвращает очередную непустую строку без перевода строки. big — строка длиннее предела:
// line пуста, head — начало строки (до headKeep байт). Ошибка ввода (в том числе io.EOF)
// приходит после последней строки; неполная последняя строка без перевода отдаётся как строка.
func (lr *lineReader) next() (line []byte, big bool, head []byte, err error) {
	for {
		line, big, head, err = lr.one()
		if err != nil || big || len(line) > 0 {
			return line, big, head, err
		}
	}
}

func (lr *lineReader) one() (line []byte, big bool, head []byte, err error) {
	var buf []byte
	skipping := false
	for {
		chunk, e := lr.br.ReadSlice('\n')
		hasNL := len(chunk) > 0 && chunk[len(chunk)-1] == '\n'
		if !skipping {
			size := len(buf) + len(chunk)
			if hasNL {
				size--
			}
			if size > lr.max {
				skipping = true
				all := append(buf, chunk...)
				head = bytes.Clone(all[:min(headKeep, len(all))])
				buf = nil
			} else {
				buf = append(buf, chunk...)
			}
		}
		switch {
		case e == nil:
			if skipping {
				return nil, true, head, nil
			}
			return trimEOL(buf), false, nil, nil
		case errors.Is(e, bufio.ErrBufferFull):
			continue
		}
		// Конец ввода или ошибка: накопленное отдаём строкой, ошибка придёт со следующим вызовом.
		if skipping {
			return nil, true, head, nil
		}
		if len(buf) > 0 {
			return trimEOL(buf), false, nil, nil
		}
		return nil, false, nil, e
	}
}

func trimEOL(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	return bytes.TrimSuffix(b, []byte("\r"))
}

// extractID ищет значение верхнего ключа "id" в начале длинной строки потоковым разбором
// (json.Decoder.Token), не читая строку целиком. Нашёлся только годный id (строка или число
// короткой записи); иначе ok=false.
func extractID(head []byte) (id json.RawMessage, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(head))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, false
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, isKey := t.(string)
		if !isKey {
			return nil, false
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, false // значение оборвано пределом головы: id дальше не найти
		}
		if key == "id" {
			if _, ok := idKey(v); !ok {
				return nil, false
			}
			return v, true
		}
	}
	return nil, false
}
