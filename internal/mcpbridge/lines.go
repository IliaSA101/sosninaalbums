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
// (json.Decoder.Token), не читая строку целиком. Нашёлся только годный id (goodID). needMethod —
// строка клиента: ошибкой отвечаем лишь запросу, а у запроса в начале строки должен оказаться и
// верхний "method"; иначе это ответ клиента программе, и от её имени мы не отвечаем.
func extractID(head []byte, needMethod bool) (id json.RawMessage, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(head))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, false
	}
	sawMethod := false
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			break
		}
		key, isKey := t.(string)
		if !isKey {
			break
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			break // значение оборвано пределом головы: дальше не прочесть
		}
		switch key {
		case "id":
			if !goodID(v) {
				return nil, false
			}
			id = v
		case "method":
			sawMethod = true
		}
		if id != nil && (!needMethod || sawMethod) {
			return id, true
		}
	}
	return nil, false
}
