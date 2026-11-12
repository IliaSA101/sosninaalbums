// Package agentasktext — тексты вопросов агента («агент просит…»): чистые функции над строками и числами,
// без Editor, Wails и служб (162 Ш2а). App собирает входы, пакет — слова.
package agentasktext

// Тексты вопроса «агент просит…» (149, design §3, решение 0.33-149-5): шаблоны Go, интерфейс только
// рисует. Своего текста агент не вносит: из чужих рук приходит лишь имя клиента, да и оно — в «…»
// после фильтра знаков. Фразу «Агент предлагает папку «…»» рисует интерфейс сам из AgentAsk.Hint
// в <bdi> (0.33-149-35), здесь её нет.

import (
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Виды вопроса (AgentAsk.Kind).
const (
	KindCreate = "create"
	KindAdd    = "add"
)

// Doc — что с открытым альбомом, когда агент просит создать новый.
type Doc int

const (
	DocSaved Doc = iota // есть файл, несохранённого нет: сохранится в свой файл
	DocDraft            // несохранённый черновик: уйдёт в резервные копии
	DocEmpty            // пустой альбом: закроется
	DocDirty            // есть файл и несохранённые правки: уйдут копией, файл не переписывается
)

const (
	okPick       = "Выбрать папку…"
	unknown      = "неизвестный клиент"
	noName       = "без названия"
	Who          = "Агент называет себя сам: если вы ничего не просили — «Отмена»."
	NameMax      = 60 // рун имени альбома и клиента в тексте
	FolderReject = "Выберите папку съёмки: эта слишком широкая или служебная — корень диска, домашняя папка, «Документы» целиком или системные папки не подойдут."
)

// Body — текст вопроса без идентификатора и подсказки (их добавляет вопрос).
type Body struct {
	Title string
	Lines []string
	OK    string
}

// Text — заголовок, абзацы и кнопка вопроса вида kind; false — вид неизвестен. client — имя
// клиента, как его назвала служба, album — название открытого альбома (его же файл или черновик),
// doc — что с ним станет (только у create). Первый абзац — «кто просит и чего»: интерфейс
// вставляет подсказку папки сразу за ним.
func Text(kind, client, album string, doc Doc) (Body, bool) {
	who := ClientQuoted(client)
	switch kind {
	case KindCreate:
		return Body{
			Title: "Создать альбом по просьбе агента?",
			Lines: []string{
				who + " просит создать новый альбом из папки с фотографиями. Папку выберете вы.",
				DocLine(album, doc),
				Who,
			},
			OK: okPick,
		}, true
	case KindAdd:
		return Body{
			Title: "Добавить папку по просьбе агента?",
			Lines: []string{
				who + " просит добавить в открытый альбом «" + ShortName(album) + "» папку с фотографиями. Папку выберете вы.",
				Who,
			},
			OK: okPick,
		}, true
	}
	return Body{}, false
}

// Alt — подпись кнопки «Выбрать другую…» вопроса с показанным путём.
const Alt = "Выбрать другую…"

// TextPath — вопрос с показанным настоящим путём (150, design §5.1): заголовок называет папку,
// кнопки «Добавить» / «Создать»; сам путь рисует интерфейс сразу за первым абзацем. label — последний
// элемент пути; с плохими знаками имени в заголовке нет (чужой текст не должен подменить заголовок).
func TextPath(kind, client, album string, doc Doc, label string) (Body, bool) {
	who := ClientQuoted(client)
	name := ""
	if !paths.BadRunes(label) && label != "" {
		name = " «" + ShortName(label) + "»"
	}
	switch kind {
	case KindCreate:
		return Body{
			Title: "Создать альбом из папки" + name + "?",
			Lines: []string{who + " просит создать новый альбом из папки:", DocLine(album, doc), Who},
			OK:    "Создать",
		}, true
	case KindAdd:
		return Body{
			Title: "Добавить папку" + name + "?",
			Lines: []string{who + " просит добавить в открытый альбом «" + ShortName(album) + "» папку:", Who},
			OK:    "Добавить",
		}, true
	}
	return Body{}, false
}

// DocLine — что станет с открытым альбомом.
func DocLine(album string, doc Doc) string {
	switch doc {
	case DocSaved:
		return "Открытый «" + ShortName(album) + "» сохранится в свой файл."
	case DocDirty:
		return "Несохранённые правки «" + ShortName(album) + "» сохранятся копией."
	case DocDraft:
		if ShortName(album) == noName {
			return "Несохранённый черновик уйдёт в резервные копии."
		}
		return "Несохранённый черновик «" + ShortName(album) + "» уйдёт в резервные копии."
	}
	return "Пустой альбом закроется."
}

// DialogTitle — заголовок окна ОС «Выбрать папку»: просит клиент, а не программа.
func DialogTitle(client string) string {
	return "Папка с фотографиями — просит " + ClientName(client)
}

// ClientName — имя клиента для показа: плохие знаки, пустое, слишком длинное и похожее на
// программу («Sosnina», «Сосн»: подделка под окно программы) — «неизвестный клиент».
func ClientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || paths.BadRunes(name) || utf8.RuneCountInString(name) > NameMax || agents.Lookalike(name) {
		return unknown
	}
	return name
}

// ClientQuoted — имя клиента для абзаца: в «…», чтобы чужие слова не сливались с нашими;
// неизвестный клиент без кавычек.
func ClientQuoted(name string) string {
	n := ClientName(name)
	if n == unknown {
		return "Неизвестный клиент"
	}
	return "Клиент «" + n + "»"
}

// ShortName — название альбома для текста: свои файлы тоже могут быть чужими по происхождению,
// поэтому тот же фильтр знаков; пустое или плохое — «без названия», длинное — с «…».
func ShortName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || paths.BadRunes(s) {
		return noName
	}
	if utf8.RuneCountInString(s) > NameMax {
		return string([]rune(s)[:NameMax-1]) + "…"
	}
	return s
}
