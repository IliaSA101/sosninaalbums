package names

import (
	"errors"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Пределы поля списка (design 043 §3.1): больше — ошибка без правки. Строк поля на одну больше,
// чем людей: таблица из файла приходит с заголовком.
const (
	MaxPasteBytes = model.MaxNamesBytes // тот же предел app проверяет у текста из таблицы (PickNamesFile)
	MaxPasteLines = 1001
	maxPeople     = MaxPasteLines - 1
)

var errTooLong = errors.New("список имён слишком длинный: не больше 1000 строк и 256 КБ")

// Entry — человек из строки списка.
type Entry struct {
	Who     model.Person // зажато ClampPerson, не пусто
	Teacher bool         // непустой «Предмет»
	File    string       // столбец «Файл»; "" — нет
	Line    int          // строка поля, с 1 (заголовок и пустые строки считаются)
}

// bom — U+FEFF в UTF-8.
const bom = "\xef\xbb\xbf"

// Roster — разобранное поле окна «Имена»: люди по порядку строк и что узнано о столбцах.
// Files — режим «Имена файлов»: списка нет, имена берёт Plan из названий фото.
type Roster struct {
	Files      bool
	FirstFirst bool
	People     []Entry
	Cols       []string // роли столбцов заголовка (NamesPlan.Cols); nil — без заголовка
	Ignored    []string
	ByFile     bool // есть столбец «Файл»
	Order      bool // «Имя первым» действует: нет отдельных столбцов Фамилия/Имя
	// и заголовок ФИО сам не задаёт порядок («Фамилия Имя…», решение №47)
}

// Read разбирает поле окна: строка — человек. Первая непустая строка с «Фамилия», «Имя» или
// «ФИО» — заголовок (header.go), тогда имя берётся по столбцам. Без заголовка — правила 030:
// пустые строки пропускаются, нумерация «1.» и «2)» снимается, слов 1 — Фамилия, 2 — Фамилия
// Имя, 3+ — остаток в Отчество; FirstFirst — Имя [Отчество] Фамилия. Чтение до замка
// редактора: проект не нужен.
func Read(o model.NamesOpts) (Roster, error) {
	r := Roster{Files: o.From == model.NamesFiles, FirstFirst: o.FirstFirst, Order: true}
	if r.Files {
		return r, nil
	}
	if len(o.Text) > MaxPasteBytes {
		return Roster{}, errTooLong
	}
	// BOM из Excel/«Блокнота» в начале вставки: иначе первая ячейка заголовка не узнаётся.
	o.Text = strings.TrimPrefix(o.Text, bom)
	lines := strings.Split(strings.TrimRight(o.Text, "\r\n"), "\n")
	if len(lines) > MaxPasteLines {
		return Roster{}, errTooLong
	}
	start, h, hasHead, sep := 0, header{}, false, sepSpace
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			c, s := headCells(line)
			if h, hasHead = parseHeader(c, s == sepSpace); hasHead {
				start, sep = i+1, s
				r.Cols, r.Ignored, r.ByFile, r.Order = h.cols, h.ignored, h.file >= 0, h.order()
			}
			break
		}
	}
	for i := start; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		var (
			e  Entry
			ok bool
		)
		if hasHead {
			if sep == sepNone || sep == sepSpace {
				line = unnumber(line) // простой список с заголовком: нумерация «1.» снимается, как без него
			}
			e, ok = h.entry(cellsBy(line, sep), o.FirstFirst)
		} else if words := split(unnumber(line)); len(words) > 0 {
			who := model.ClampPerson(person(words, o.FirstFirst))
			if ok = who != nil; ok {
				e.Who = *who
			}
		}
		if !ok {
			continue
		}
		if len(r.People) == maxPeople {
			return Roster{}, errTooLong
		}
		e.Line = i + 1
		r.People = append(r.People, e)
	}
	return r, nil
}

// unnumber снимает нумерацию «  12.» или «3)» в начале строки.
func unnumber(line string) string {
	t := strings.TrimLeftFunc(line, unicode.IsSpace)
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i > 0 && i < len(t) && (t[i] == '.' || t[i] == ')') {
		return t[i+1:]
	}
	return line
}

// Разделитель ячеек строки.
type sepKind int

const (
	sepSpace sepKind = iota // по пробелам
	sepTab
	sepSemi
	sepNone // вся строка — одна ячейка; есть Tab — по Tab (в имени Tab не бывает: это таблица)
)

// sepOf — разделитель строки: Tab, иначе «;» (в ячейке CSV бывает «;», в Tab-таблице — нет), иначе пробелы.
func sepOf(line string) sepKind {
	switch {
	case strings.Contains(line, "\t"):
		return sepTab
	case strings.Contains(line, ";"):
		return sepSemi
	}
	return sepSpace
}

// cells — ячейки строки по местам: есть Tab — только по Tab, иначе есть «;» — по «;», иначе по
// пробелам. Пустые ячейки остаются: по ним считаются столбцы.
func cells(line string) []string { return cellsBy(line, sepOf(line)) }

// cellsBy — ячейки строки по заданному разделителю. Строки данных делятся разделителем заголовка,
// а не своим: Sheet.Text снимает пустые хвостовые ячейки, и у строки с пустым «Предмет» Tab пропадает.
func cellsBy(line string, sep sepKind) []string {
	var parts []string
	switch sep {
	case sepTab:
		parts = strings.Split(line, "\t")
	case sepSemi:
		parts = strings.Split(line, ";")
	case sepNone:
		if !strings.Contains(line, "\t") {
			return []string{strings.TrimSpace(line)}
		}
		parts = strings.Split(line, "\t")
	default:
		return strings.Fields(line)
	}
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// headCells — ячейки возможной строки заголовка и их разделитель. Без Tab и «;» заголовок из
// одного столбца («ФИО», «Фамилия имя») — одна ячейка: строки данных тогда не делятся по пробелам
// (только по Tab, если он в строке есть — cellsBy, sepNone).
func headCells(line string) ([]string, sepKind) {
	sep := sepOf(line)
	c := cellsBy(line, sep)
	if sep == sepSpace && (len(c) == 1 || roleOf(line) == colFIO) {
		return []string{strings.TrimSpace(line)}, sepNone
	}
	return c, sep
}

// split — слова строки без заголовка: ячейки без пустых.
func split(line string) []string {
	var out []string
	for _, w := range cells(line) {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// person — слова в имя по порядку «Фамилия Имя Отчество» или, при firstFirst,
// «Имя Отчество Фамилия» (одно слово — тогда Имя).
func person(w []string, firstFirst bool) model.Person {
	if firstFirst {
		switch len(w) {
		case 1:
			return model.Person{First: w[0]}
		case 2:
			return model.Person{First: w[0], Last: w[1]}
		}
		n := len(w) - 1
		return model.Person{First: w[0], Middle: strings.Join(w[1:n], " "), Last: w[n]}
	}
	switch len(w) {
	case 1:
		return model.Person{Last: w[0]}
	case 2:
		return model.Person{Last: w[0], First: w[1]}
	}
	return model.Person{Last: w[0], First: w[1], Middle: strings.Join(w[2:], " ")}
}
