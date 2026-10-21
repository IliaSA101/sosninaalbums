package paths

import "regexp"

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
// диск, закодированный диск, корень без буквы диска, домашние папки и тома Mac/Unix.
const pathStart = `(?:file:|\\\\|//[^/\s"'` + "`" + `<>()\[\]]+/|\\[^\\\s"'` + "`" + `<>()\[\]]+\\|[A-Za-z]:[\\/]|` +
	`[A-Za-z]%3A(?:%5C|%2F)|/(?:users|home|volumes|private|var|mnt|tmp)/|~/)`

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
	// Unix-образцы — только в начале текста или после пробела, кавычки, «(», «[», «=» и «:»: так
	// «…localhost/home…» внутри адреса не задевается, а наши /media/… (ID, не пути) — не в списке.
	// «~/…» — домашняя папка в оболочечной записи.
	reMaskUnix = regexp.MustCompile(`(?i)(^|[\s"'(\[=:])(?:/(?:users|home|volumes|private|var|mnt|tmp)/|~/)` + textPathTail)
)

// MaskText заменяет пути файлов в произвольном тексте на «<путь>» (журнал без личных имён,
// 073-7): диск, UNC (\\, // и \\?\), корень без диска, file:, закодированный диск, домашние
// папки и тома Mac/Unix. Адреса http(s) и наши /media/… остаются как есть.
func MaskText(s string) string {
	s = reMaskQuoted.ReplaceAllString(s, "'<путь>'") // первым: апостроф внутри пути — обычный знак
	s = reMaskFile.ReplaceAllString(s, "${1}<путь>") // до диска: file:///C:/x иначе дал бы «file:///<путь>»
	s = reMaskUNC.ReplaceAllString(s, "<путь>")
	s = reMaskUNCSlash.ReplaceAllString(s, "${1}<путь>")
	s = reMaskDrive.ReplaceAllString(s, "${1}<путь>")
	s = reMaskEncoded.ReplaceAllString(s, "${1}<путь>")
	s = reMaskRooted.ReplaceAllString(s, "${1}<путь>")
	return reMaskUnix.ReplaceAllString(s, "${1}<путь>")
}
