// Package table — чтение таблицы с именами (книга .xlsx, CSV/TSV, «Текст Юникод») в строки текста.
// Файл недоверенный (его принёс пользователь), поэтому у всего есть предел, а превышение —
// ошибка, а не обрезка: «правдоподобная» неполная таблица хуже отказа. Пакет ничего не пишет
// на диск и не знает проекта: имена и порядок разбирает names.
package table

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Пределы. Список класса на 1000 строк — десятки КБ, остальное — запас на картинки в книге.
const (
	MaxFile    = 8 << 20  // файл целиком
	MaxText    = 2 << 20  // CSV/TSV: на 1000 строк хватает ~100 КБ
	MaxEntries = 5000     // частей в zip: обычная книга — 10–30
	MaxPart    = 16 << 20 // распакованная часть (лист, общие строки): разбор ≤ ~0,2 с
	MaxMeta    = 1 << 20  // workbook.xml и rels
	MaxStrings = 200000   // общих строк
	MaxRows    = 1001     // непустых строк: заголовок + names.MaxPasteLines
	MaxCols    = 64       // столбцов, дальше не читаем
	MaxCell    = 256      // знаков в ячейке: имя ≤ 60, остальное — запас
)

// Тексты показываются пользователю как есть.
var (
	ErrOld     = errors.New("это файл .xls или файл с паролем: сохраните его в Excel как «Книга Excel (.xlsx)» без пароля или как CSV")
	ErrTooBig  = errors.New("таблица слишком большая: не больше 1000 строк и 8 МБ")
	ErrBroken  = errors.New("файл не похож на таблицу Excel или CSV")
	ErrNoSheet = errors.New("в книге нет видимых листов")
	ErrEmpty   = errors.New("в таблице нет строк")
)

// Сигнатуры: формат определяется по содержимому, а не по расширению (файл часто переименован).
var (
	sigZip = []byte("PK\x03\x04")
	sigOLE = []byte{0xD0, 0xCF, 0x11, 0xE0} // .xls и книга xlsx с паролем
)

// Sheet — первый видимый лист книги или весь текст CSV.
type Sheet struct {
	Rows   [][]string // непустые строки; ячейки без краевых пробелов, ≤ MaxCols, ≤ MaxCell знаков
	Name   string     // лист xlsx; "" — текст
	Sheets int        // листов в книге (со скрытыми); 0 — текст
	Enc    string     // "UTF-8" | "UTF-16" | "Windows-1251"; "" — xlsx
}

// ReadFile читает файл с диска. Размер проверяется до чтения, а читается не больше MaxFile+1:
// файл мог вырасти между проверкой и чтением, а в память целиком он не нужен.
func ReadFile(path string) (Sheet, error) {
	f, err := os.Open(path)
	if err != nil {
		return Sheet{}, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	st, err := f.Stat()
	if err != nil {
		return Sheet{}, err
	}
	if st.Size() > MaxFile {
		return Sheet{}, ErrTooBig
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFile+1))
	if err != nil {
		return Sheet{}, err
	}
	return Read(data)
}

// Read разбирает содержимое файла; формат — по сигнатуре.
func Read(data []byte) (Sheet, error) {
	if len(data) > MaxFile {
		return Sheet{}, ErrTooBig
	}
	var (
		s   Sheet
		err error
	)
	switch {
	case bytes.HasPrefix(data, sigZip):
		s, err = readXLSX(data)
	case bytes.HasPrefix(data, sigOLE):
		return Sheet{}, ErrOld
	case len(data) > MaxText:
		return Sheet{}, ErrTooBig
	default:
		s, err = readText(data)
	}
	if err != nil {
		return Sheet{}, err
	}
	if len(s.Rows) == 0 {
		return Sheet{}, ErrEmpty
	}
	return s, nil
}

// Text — таблица текстом для поля списка: строки через "\n", ячейки через "\t" (names делит
// по Tab, поэтому \t \r \n внутри ячейки становятся пробелом); хвостовые пустые ячейки сняты.
func (s Sheet) Text() string {
	var b strings.Builder
	for i, row := range s.Rows {
		n := len(row)
		for n > 0 && row[n-1] == "" {
			n--
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, c := range row[:n] {
			if j > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(oneLine.Replace(c))
		}
	}
	return b.String()
}

var oneLine = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")

// clean — ячейка без краевых пробелов и не длиннее MaxCell знаков. Сначала грубо по байтам,
// чтобы не гонять по мегабайтной ячейке; обрезка по байтам могла разрезать знак — хвост снимается.
func clean(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > MaxCell*utf8.UTFMax {
		s = s[:MaxCell*utf8.UTFMax]
	}
	s = strings.ToValidUTF8(s, "")
	n := 0
	for i := range s {
		if n == MaxCell {
			s = s[:i]
			break
		}
		n++
	}
	return strings.TrimSpace(s)
}

// trimRow снимает хвостовые пустые ячейки; пустая строка — nil.
func trimRow(row []string) []string {
	n := len(row)
	for n > 0 && row[n-1] == "" {
		n--
	}
	if n == 0 {
		return nil
	}
	return row[:n]
}
