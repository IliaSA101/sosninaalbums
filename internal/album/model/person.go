package model

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Имя на фото (030, 043): своё или из имени файла (строгое и вольное правило), вид для карточки.

// Name — имя на фото: своё, иначе из имени файла (fromFile). nil — имени нет. Свой только
// предмет (049) — имя из файла с этим предметом: учителю «Иванова Мария.jpg» вписали
// предмет, имя из файла не пропадает.
func (r PhotoRef) Name() (p *Person, fromFile bool) {
	if r.Person != nil && r.Person.Named() {
		return r.Person, false
	}
	if p = PersonFromFile(r.Path); p != nil {
		if r.Person != nil {
			p.Subject = r.Person.Subject
		}
		return p, true
	}
	return r.Person, false
}

// PersonFromFile — имя из имени файла: «Иванов Петя.jpg» → Иванов / Петя, три слова —
// с отчеством. Слово — буквы (внутри дефис и апостроф), первая заглавная; «_» — пробел;
// номер в начале («01_», «1. ») и отметка копии в конце (« (2)», « 2», «-2») не мешают.
// Иначе nil: «DSC_0012», «001», «Свадьба 2025» именем не считаются. Правило строгое:
// имя берётся неявно, без предпросмотра (043 Р10).
func PersonFromFile(path string) *Person { return personFromBase(fileBase(path), false, false) }

// PersonFromFileLoose — имя для режима «Имена файлов» (043): вдобавок к PersonFromFile
// «.» — пробел, одно слово через дефис делится («Иванов-Петя»), регистр исправляется
// («ИВАНОВ», «иванов» → «Иванов»); firstFirst — «Имя [Отчество] Фамилия». Вольные правила
// действуют только по явному «Вставить» с предпросмотром.
func PersonFromFileLoose(path string, firstFirst bool) *Person {
	return personFromBase(fileBase(path), true, firstFirst)
}

// fileBase — имя файла без папки и расширения; «\» — разделитель и вне Windows.
func fileBase(path string) string {
	base := path[strings.LastIndexAny(path, `\/`)+1:]
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// maxFileBase — длина имени файла до снятия номера и отметки копии; после — 3 слова и 2 пробела.
const maxFileBase = 3*MaxPersonRunes + 12

func personFromBase(base string, loose, firstFirst bool) *Person {
	if !utf8.ValidString(base) || utf8.RuneCountInString(base) > maxFileBase {
		return nil
	}
	sep := "_"
	if loose {
		sep = "_."
	}
	base = strings.Map(func(r rune) rune {
		if strings.ContainsRune(sep, r) {
			return ' '
		}
		return r
	}, base)
	base = strings.TrimSpace(stripCopy(stripLead(strings.TrimSpace(base))))
	if utf8.RuneCountInString(base) > 3*MaxPersonRunes+2 {
		return nil
	}
	words := strings.Fields(base)
	if loose && len(words) == 1 {
		if parts := strings.Split(words[0], "-"); len(parts) == 2 || len(parts) == 3 {
			words = parts
		}
	}
	if len(words) < 2 || len(words) > 3 {
		return nil
	}
	for i, w := range words {
		if loose {
			w = fixCase(w)
			words[i] = w
		}
		if !nameWord(w) {
			return nil
		}
	}
	if firstFirst {
		// «Имя [Отчество] Фамилия», как names.person.
		n := len(words) - 1
		words = append([]string{words[n]}, words[:n]...)
	}
	p := Person{Last: words[0], First: words[1]}
	if len(words) == 3 {
		p.Middle = words[2]
	}
	return ClampPerson(p)
}

// stripLead снимает номер в начале: «01 », «1. », «2) », «3 - » или «1.» / «1-» перед буквой.
func stripLead(s string) string {
	i := 0
	for i < len(s) && i <= 4 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 4 {
		return s
	}
	j := skipSpaces(s, i)
	if j < len(s) && strings.IndexByte(".)-", s[j]) >= 0 {
		k := skipSpaces(s, j+1)
		r, _ := utf8.DecodeRuneInString(s[k:])
		if k > j+1 || j == i && unicode.IsLetter(r) {
			return s[k:]
		}
	}
	if j > i {
		return s[j:]
	}
	return s
}

func skipSpaces(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

// stripCopy снимает отметку копии в конце: « (2)», « 2», «-2» (1–3 цифры; «2025» — не отметка).
func stripCopy(s string) string {
	t, paren := s, strings.HasSuffix(s, ")")
	if paren {
		t = s[:len(s)-1]
	}
	i := len(t)
	for i > 0 && t[i-1] >= '0' && t[i-1] <= '9' {
		i--
	}
	if n := len(t) - i; n == 0 || n > 3 || i < 2 {
		return s
	}
	switch c := t[i-1]; {
	case paren && c == '(' && t[i-2] == ' ':
		return t[:i-2]
	case !paren && (c == ' ' || c == '-'):
		return t[:i-1]
	}
	return s
}

// fixCase — слово целиком строчными или заглавными → «Иванов», каждая часть через дефис
// отдельно («иванова-петрова» → «Иванова-Петрова»); смешанный регистр («МакКартни») — как есть.
func fixCase(w string) string {
	if strings.ToLower(w) != w && strings.ToUpper(w) != w {
		return w
	}
	parts := strings.Split(strings.ToLower(w), "-")
	for i, p := range parts {
		if r, n := utf8.DecodeRuneInString(p); n > 0 {
			parts[i] = string(unicode.ToUpper(r)) + p[n:]
		}
	}
	return strings.Join(parts, "-")
}

// nameWord — слово имени: заглавная буква, дальше буквы, дефис или апостроф
// (не два знака подряд и не в конце): «Иванова-Петрова», «Д'Артаньян».
func nameWord(w string) bool {
	prevSep := false
	for i, r := range w {
		switch {
		case i == 0:
			if !unicode.IsUpper(r) {
				return false
			}
		case unicode.IsLetter(r):
			prevSep = false
		case r == '-' || r == '\'' || r == '’':
			if prevSep {
				return false
			}
			prevSep = true
		default:
			return false
		}
	}
	return !prevSep
}

// PersonView — имя на фото для карточки: своё или из имени файла (FromFile — подсказка
// серым; предмет и тогда свой). Teacher — фото на месте учителя виньетки: карточка
// показывает поле «Предмет» (049).
type PersonView struct {
	Last     string `json:"last"`
	First    string `json:"first"`
	Middle   string `json:"middle"`
	Subject  string `json:"subject"`
	FromFile bool   `json:"fromFile"`
	Teacher  bool   `json:"teacher"`
}

// NameView — имя фото r для карточки; имени нет — пустые поля. Teacher ставит тот, кто
// знает проект (TeacherSeat).
func NameView(r PhotoRef) PersonView {
	who, file := r.Name()
	if who == nil {
		return PersonView{}
	}
	return PersonView{Last: who.Last, First: who.First, Middle: who.Middle, Subject: who.Subject, FromFile: file}
}

// Окно «Имена» (043): предпросмотр и вставка считаются одной функцией names.Plan,
// поэтому то, что показано в окне, и ляжет на фото.

// NamesFiles — NamesOpts.From: имена из названий фото листа, а не из списка.
const NamesFiles = "files"

// NamesOpts — окно «Имена»: откуда имена и как читать список.
type NamesOpts struct {
	From       string `json:"from"`       // "list" | "files"; иное — "list"
	Text       string `json:"text"`       // список (From = "list")
	FirstFirst bool   `json:"firstFirst"` // «Имя первым»
	Caption    bool   `json:"caption"`    // подписать фото {Фамилия} {Имя}, если у обычного листа нет шаблона подписи
}

// NamesRow — место листа, получающее имя, в порядке предпросмотра.
type NamesRow struct {
	Photo string  `json:"photo"`
	Tag   string  `json:"tag"`  // "учитель" | "ученик 3" | "групповое"; "" — не виньетка
	No    int     `json:"no"`   // номер среди получающих; 0 — групповое (пропуск)
	Line  int     `json:"line"` // строка поля (1 — первая, считая заголовок); 0 — из имени файла или нет
	Who   *Person `json:"who"`  // что ляжет; nil — имя не меняется
}

// NamesLeft — строка списка без фото.
type NamesLeft struct {
	Line   int    `json:"line"`
	Who    Person `json:"who"`
	Reason string `json:"reason"` // "extra" | "nofile" | "many" | "dup" | "noseat" (учитель без места учителя)
	File   string `json:"file"`   // значение столбца «Файл»; "" — нет
}

// NamesPlan — предпросмотр окна; он же план вставки.
type NamesPlan struct {
	Rows         []NamesRow  `json:"rows"`
	Left         []NamesLeft `json:"left"`         // не больше 50; полное число — Extra
	Extra        int         `json:"extra"`        // строк без фото
	Seats        int         `json:"seats"`        // мест, получающих имя (фото листа без группового)
	Named        int         `json:"named"`        // из них с именем после вставки
	Missing      int         `json:"missing"`      // Seats − Named
	Cols         []string    `json:"cols"`         // по столбцам: "Фамилия" | "Имя" | "Отчество" | "ФИО" | "Предмет" | "Файл" | "" (не используется); nil — без заголовка
	Ignored      []string    `json:"ignored"`      // заголовки неиспользуемых столбцов, как в тексте
	ByFile       bool        `json:"byFile"`       // сопоставление по столбцу «Файл»
	Order        bool        `json:"order"`        // «Имя первым» на что-то влияет (нет отдельных столбцов Фамилия/Имя или заголовок ФИО сам задаёт порядок («Фамилия Имя…», решение №47))
	Teachers     int         `json:"teachers"`     // строк-учителей в списке
	Pupils       int         `json:"pupils"`       // остальных строк
	TeacherSeats int         `json:"teacherSeats"` // мест учителей на виньетке, и пустых; не виньетка — 0
	PupilSeats   int         `json:"pupilSeats"`   // мест учеников на виньетке, и пустых; не виньетка — 0
	NeedCaption  bool        `json:"needCaption"`  // обычный лист без шаблона подписи: окно покажет флажок «Подписать фото»
}
