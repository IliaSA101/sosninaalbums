package names

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Роли столбцов — как в NamesPlan.Cols (их же показывает окно: «Столбцы: Фамилия, Имя»).
const (
	colLast    = "Фамилия"
	colFirst   = "Имя"
	colMiddle  = "Отчество"
	colFIO     = "ФИО"
	colSubject = "Предмет"
	colFile    = "Файл"
)

// roleOf — роль столбца по заголовку (design 043 §3.1); "" — столбец не используется.
func roleOf(title string) string {
	key := headerKey(title)
	if _, ok := fioOrder(key); ok {
		return colFIO
	}
	switch key {
	case "фамилия", "last name", "surname":
		return colLast
	case "имя", "first name":
		return colFirst
	case "отчество", "middle name", "patronymic":
		return colMiddle
	case "фио", "ученик", "учащийся", "full name", "name":
		return colFIO
	case "предмет", "должность", "subject":
		return colSubject
	case "файл", "фото", "имя файла", "file", "photo", "filename":
		return colFile
	}
	return ""
}

// fioOrder — синонимы «ФИО», которые сами называют порядок слов: «Имя первым» на них не
// действует (иначе «Фамилия Имя Отчество» при включённом флажке читалось бы задом наперёд).
// key — headerKey заголовка; ok = false — порядок не задан; firstFirst — имя первым.
func fioOrder(key string) (firstFirst, ok bool) {
	switch key {
	case "фамилия имя", "фамилия и имя", "фамилия имя отчество":
		return false, true
	case "имя фамилия", "имя и фамилия", "имя отчество фамилия":
		return true, true
	}
	return false, false
}

// keyReplacer — один на пакет: строить Replacer на каждую ячейку заголовка дорого, а строка
// заголовка из вставленного текста может быть в сотни тысяч ячеек.
var keyReplacer = strings.NewReplacer("ё", "е", ":", "", ",", "", "*", "", ".", "")

// maxCols — столбцов заголовка, дальше не читаем (table.MaxCols: names не импортирует table).
// Без предела строка заголовка в 256 КБ дала бы десятки тысяч столбцов в Cols и Ignored.
const maxCols = 64

// headerKey — заголовок для сравнения: регистр, «ё», «:», «*», точки и запятые не важны («Ф.И.О.:» → «фио»).
func headerKey(s string) string {
	s = keyReplacer.Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// header — столбцы заголовка: номер столбца каждой роли (−1 — нет).
type header struct {
	cols                                    []string
	ignored                                 []string
	last, first, middle, fio, subject, file int
	fixed, fixedFirst                       bool // ячейка ФИО сама задаёт порядок (fioOrder)
}

// parseHeader — заголовок, если в строке есть столбец «Фамилия», «Имя» или «ФИО». Повтор роли
// не используется (берётся первый столбец); при Фамилии/Имени «ФИО» тоже не используется.
// Строка без Tab и «;» (spaced — ячейки по пробелам) — заголовок, только если все её слова —
// Фамилия, Имя или Отчество: иначе «Сидоров Имя» из простого списка стал бы заголовком и пропал,
// а с ФИО, Предметом или Файлом слова имени уехали бы в чужие столбцы («ФИО Предмет» +
// «Иванов Пётр Сергеевич» — учитель «Иванов»).
func parseHeader(c []string, spaced bool) (header, bool) {
	c = c[:min(len(c), maxCols)]
	h := header{last: -1, first: -1, middle: -1, fio: -1, subject: -1, file: -1}
	slot := map[string]*int{colLast: &h.last, colFirst: &h.first, colMiddle: &h.middle,
		colFIO: &h.fio, colSubject: &h.subject, colFile: &h.file}
	roles := make([]string, len(c))
	for i, title := range c {
		role := roleOf(title)
		if spaced && role != colLast && role != colFirst && role != colMiddle {
			return header{}, false
		}
		if p := slot[role]; p != nil && *p < 0 {
			*p, roles[i] = i, role
		}
	}
	if h.last < 0 && h.first < 0 && h.fio < 0 {
		return header{}, false
	}
	if h.fio >= 0 && (h.last >= 0 || h.first >= 0) {
		roles[h.fio], h.fio = "", -1
	}
	if h.fio >= 0 {
		h.fixedFirst, h.fixed = fioOrder(headerKey(c[h.fio]))
	}
	h.cols, h.ignored = roles, []string{}
	for i, title := range c {
		if roles[i] == "" && title != "" {
			h.ignored = append(h.ignored, title)
		}
	}
	return h, true
}

// order — имя берётся из слов ячейки «ФИО», и «Имя первым» на него влияет: порядок не задан ни
// столбцами Фамилия/Имя, ни самим заголовком ФИО.
func (h header) order() bool { return h.last < 0 && h.first < 0 && !h.fixed }

// entry — человек из ячеек строки; ok = false — имени в строке нет (строка пропускается).
func (h header) entry(c []string, firstFirst bool) (Entry, bool) {
	at := func(i int) string {
		if i >= 0 && i < len(c) {
			return c[i]
		}
		return ""
	}
	var who *model.Person
	if h.fio >= 0 {
		if w := strings.Fields(at(h.fio)); len(w) > 0 {
			if h.fixed {
				firstFirst = h.fixedFirst
			}
			p := person(w, firstFirst)
			if p.Middle == "" {
				p.Middle = at(h.middle)
			}
			who = model.ClampPerson(p)
		}
	} else {
		who = model.ClampPerson(model.Person{Last: at(h.last), First: at(h.first), Middle: at(h.middle)})
	}
	if who == nil {
		return Entry{}, false
	}
	// Предмет ложится в имя (049); учитель — по непустой ячейке, как в 043.
	who.Subject = at(h.subject)
	return Entry{Who: *model.ClampPerson(*who), Teacher: at(h.subject) != "", File: at(h.file)}, true
}
