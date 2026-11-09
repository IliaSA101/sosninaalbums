package paths

import (
	"regexp"
	"strings"
)

// Конец пути в тексте — кавычка «"», «`», угловая скобка или перевод строки; пробел путь НЕ
// заканчивает (в «Фото свадьбы» пробелы бывают). Скобки внутри пути — сбалансированные группы
// «(июнь)», «[VIP]», «IMG (2).jpg»; одиночные «(» и «[» — обычные знаки, одиночные «)» и «]»
// закрывают путь («at f (C:\x\a.js:1:2)»). Апостроф — обычный знак («Анна' Пётр», «Мария's»):
// путь в апострофах снимает reMaskQuoted раньше. Вложенность групп — один уровень «((Анна))».
// Лучше скрыть лишнее, чем оставить имя.
const textPathTail = `(?:\(` + groupBody + `\)|\[` + groupBody + `\]|[^"` + "`" + `)\]<>\r\n])*`

// groupBody — внутренность скобочной группы: знаки без скобок или одна вложенная группа.
const groupBody = `(?:` + groupChar + `|\(` + groupChar + `*\)|\[` + groupChar + `*\])*`

const groupChar = `[^()\[\]"` + "`" + `<>\r\n]`

// pathStart — начала путей всех видов (для пути в апострофах): file:, UNC через «\\» и «//»,
// диск, закодированный диск, корень без буквы диска, любой абсолютный путь Unix из двух частей
// и больше (наши /media/… отсеивает quotedKeep) и «~/».
const pathStart = `(?:file:|\\\\|//[^/\s"'` + "`" + `<>()\[\]]+/|\\[^\\\s"'` + "`" + `<>()\[\]]+\\|[A-Za-z]:[\\/]|` +
	`[A-Za-z]%3A(?:%5C|%2F)|/[^/\s\\"'` + "`" + `<>()\[\]]+/|~/)`

// Перед путём без буквы диска — начало текста, пробел, кавычка, «(», «=» или «[»: так
// «http://…» (перед «//» двоеточие) и «a\n» в тексте не задеваются.
const plainLead = `(^|[\s"'(=\[])`

var (
	// Путь в апострофах целиком. Апостроф внутри — часть пути, если за ним не пробел и не знак
	// препинания («Мария's») или до следующего апострофа ещё есть разделитель папок («'C:\Анна'
	// Пётр\a.jpg'»); иначе он закрывает путь: «'C:\a.jpg' и 'D:\b.jpg'» — два пути.
	reMaskQuoted = regexp.MustCompile(`(?i)'` + pathStart +
		"(?:[^'\"`<>\\r\\n]|'[^'\"`<>\\r\\n\\\\/]*[\\\\/]|'[^\\s'\"`<>,.;:)\\]\\r\\n])*'")
	// Начало — не буква и не цифра (или начало текста): иначе «profile:» и «p:/» из http задели бы.
	reMaskFile     = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])file:` + textPathTail)
	reMaskUNC      = regexp.MustCompile(`\\\\` + textPathTail)
	reMaskUNCSlash = regexp.MustCompile(plainLead + `//[^/\s"'` + "`" + `<>()\[\]]+/` + textPathTail)
	reMaskDrive    = regexp.MustCompile(`(^|[^A-Za-z0-9])[A-Za-z]:[\\/]` + textPathTail)
	reMaskEncoded  = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])[A-Za-z]%3A(?:%5C|%2F)` + textPathTail)
	// Корень без буквы диска «\Users\…» — не меньше двух разделителей: одиночное «\t» не путь.
	reMaskRooted = regexp.MustCompile(plainLead + `\\[^\\\s"'` + "`" + `<>()\[\]]+\\` + textPathTail)
	// Unix: любой абсолютный путь из двух и более частей («/run/user/1000/gvfs/smb-share:…/Свадьба»,
	// «/data/…», «/opt/…») и «~/…» — только в начале текста или после пробела, кавычки, «(», «[»,
	// «=» и «:»: так «…localhost/home…» внутри адреса не задевается. Наши /media/… (ID, не пути)
	// отсеивает код по первой части (группа 2); тома /media/<имя>/ — раньше, reMaskMedia.
	reMaskUnix = regexp.MustCompile(`(?i)(^|[\s"'(\[=:])(?:/([^/\s"'` + "`" + `<>()\[\]\\]+)/|~/)` + textPathTail)
	// /media/<имя>/…: тома Linux (/media/ilya/ДИСК). Сегмент короче двух знаков и наши адреса
	// (mediaOurs) — не путь; в RE2 нет отрицательного просмотра, отсев — в коде.
	reMaskMedia = regexp.MustCompile(`(?i)(^|[\s"'(\[=:])/media/([^/\s"'` + "`" + `<>()\[\]\\]{2,})/` + textPathTail)
)

// mediaOurs — первые сегменты наших адресов /media/…, длиннее одного знака (t, p, f — короче).
var mediaOurs = map[string]bool{"font": true, "gen": true, "lib": true, "tpl": true, "cover-guide": true, "spread-guide": true}

// reasonSep — «: » между путём и причиной в тексте ошибки Go («open <путь>: no such file»).
const reasonSep = ": "

// maskKeep заменяет совпадения re на «<путь>», сохраняя перед ними группу 1 (если есть) и
// причину ошибки после пути: хвост пути жадный, а причина — часть после первого «: », где уже нет
// разделителей папок («open C:\x\IMG (2).jpg: no such file» → «open <путь>: no such file»).
// Двоеточие с пробелом внутри самого пути («Проект: Иванов\a.jpg») разделителем не считается:
// после него есть «\» или «/». skip — не путь (по группе 2), текст не меняется.
func maskKeep(re *regexp.Regexp, s string, skip func(seg string) bool) string {
	idx := re.FindAllStringSubmatchIndex(s, -1)
	if idx == nil {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range idx {
		start := m[0]
		if re.NumSubexp() > 0 && m[2] >= 0 {
			start = m[3] // группа 1 — начало совпадения; остаётся как есть
		}
		if skip != nil && m[4] >= 0 && skip(s[m[4]:m[5]]) {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString("<путь>")
		b.WriteString(reasonOf(s[start:m[1]]))
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// reasonOf — «: причина» в конце совпадения (с разделителем), если она есть; иначе пусто.
// Причина — английский errno Linux/Mac: первое её звено до следующего «: » целиком ASCII, иначе
// это кусок имени папки («Свадьба: Анна и Пётр: permission denied») и скрывается вместе с путём.
func reasonOf(match string) string {
	for i := 0; ; {
		j := strings.Index(match[i:], reasonSep)
		if j < 0 {
			return ""
		}
		at := i + j
		if isReason(match[at+len(reasonSep):]) {
			return match[at:]
		}
		i = at + 1
	}
}

// isReason — хвост после «: » похож на причину, а не на продолжение пути: непустой, без «\»,
// первое звено — текст errno (reErrnoLike), а «/» — только между словами («input/output error»).
func isReason(rest string) bool {
	if rest == "" || strings.Contains(rest, `\`) {
		return false
	}
	first, _, _ := strings.Cut(rest, reasonSep)
	if !reErrnoLike.MatchString(first) {
		return false
	}
	for _, w := range strings.Fields(rest) {
		if strings.Contains(w, "/") && !reSlashWord.MatchString(w) {
			return false
		}
	}
	return true
}

// reErrnoLike — вид текста errno: два слова и больше строчными латинскими буквами. Однословное
// «Smith.jpg» или «Ivanov: wedding» — кусок имени папки, не причина. Списка errno нет: в тестах
// и логах есть сокращённые и виндовые тексты.
var reErrnoLike = regexp.MustCompile(`^[a-z/'-]+(?: [a-z/'-]+)+[.,;]?$`)

// reSlashWord — слово с «/» внутри причины: «input/output», не путь и не «a/b.jpg».
var reSlashWord = regexp.MustCompile(`^[A-Za-z]+(?:/[A-Za-z]+)+[,;.]?$`)

// MaskText заменяет пути файлов в произвольном тексте на «<путь>» (журнал без личных имён,
// 073-7): диск, UNC (\\, // и \\?\), корень без диска, file:, закодированный диск, домашние
// папки и тома Mac/Unix. Адреса http(s) и наши /media/… остаются как есть.
func MaskText(s string) string {
	s = reMaskQuoted.ReplaceAllStringFunc(s, quotedMask) // первым: апостроф внутри пути — обычный знак
	s = maskKeep(reMaskFile, s, nil)                     // до диска: file:///C:/x иначе дал бы «file:///<путь>»
	s = maskKeep(reMaskUNC, s, nil)
	s = maskKeep(reMaskUNCSlash, s, nil)
	s = maskKeep(reMaskDrive, s, nil)
	s = maskKeep(reMaskEncoded, s, nil)
	s = maskKeep(reMaskRooted, s, nil)
	s = maskKeep(reMaskMedia, s, func(seg string) bool { return mediaOurs[strings.ToLower(seg)] })
	return maskKeep(reMaskUnix, s, func(seg string) bool { return strings.EqualFold(seg, "media") })
}

// quotedMask — замена пути в апострофах; наш адрес '/media/<ID>/…' и короткий сегмент
// ('/media/x/…') остаются как есть (то же, что отсев в reMaskMedia).
func quotedMask(m string) string {
	if len(m) > 8 && strings.EqualFold(m[1:8], "/media/") {
		seg, _, _ := strings.Cut(m[8:], "/")
		if mediaOurs[strings.ToLower(seg)] || len([]rune(seg)) < 2 {
			return m
		}
	}
	return "'<путь>'"
}
