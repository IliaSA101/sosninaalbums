package app

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
	askKindCreate = "create"
	askKindAdd    = "add"
)

// askDoc — что с открытым альбомом, когда агент просит создать новый.
type askDoc int

const (
	askDocSaved askDoc = iota // есть файл, несохранённого нет: сохранится в свой файл
	askDocDraft               // несохранённый черновик: уйдёт в резервные копии
	askDocEmpty               // пустой альбом: закроется
	askDocDirty               // есть файл и несохранённые правки: уйдут копией, файл не переписывается
)

const (
	askOK           = "Выбрать папку…"
	askUnknown      = "неизвестный клиент"
	askNoName       = "без названия"
	askWho          = "Агент называет себя сам: если вы ничего не просили — «Отмена»."
	askNameMax      = 60 // рун имени альбома и клиента в тексте
	askFolderReject = "Выберите папку съёмки: эта слишком широкая или служебная — корень диска, домашняя папка, «Документы» целиком или системные папки не подойдут."
)

// askBody — текст вопроса без идентификатора и подсказки (их добавляет вопрос).
type askBody struct {
	Title string
	Lines []string
	OK    string
}

// askText — заголовок, абзацы и кнопка вопроса вида kind; false — вид неизвестен. client — имя
// клиента, как его назвала служба, album — название открытого альбома (его же файл или черновик),
// doc — что с ним станет (только у create). Первый абзац — «кто просит и чего»: интерфейс
// вставляет подсказку папки сразу за ним.
func askText(kind, client, album string, doc askDoc) (askBody, bool) {
	who := clientQuoted(client)
	switch kind {
	case askKindCreate:
		return askBody{
			Title: "Создать альбом по просьбе агента?",
			Lines: []string{
				who + " просит создать новый альбом из папки с фотографиями. Папку выберете вы.",
				docLine(album, doc),
				askWho,
			},
			OK: askOK,
		}, true
	case askKindAdd:
		return askBody{
			Title: "Добавить папку по просьбе агента?",
			Lines: []string{
				who + " просит добавить в открытый альбом «" + shortName(album) + "» папку с фотографиями. Папку выберете вы.",
				askWho,
			},
			OK: askOK,
		}, true
	}
	return askBody{}, false
}

// askAlt — подпись кнопки «Выбрать другую…» вопроса с показанным путём.
const askAlt = "Выбрать другую…"

// askTextPath — вопрос с показанным настоящим путём (150, design §5.1): заголовок называет папку,
// кнопки «Добавить» / «Создать»; сам путь рисует интерфейс сразу за первым абзацем. label — последний
// элемент пути; с плохими знаками имени в заголовке нет (чужой текст не должен подменить заголовок).
func askTextPath(kind, client, album string, doc askDoc, label string) (askBody, bool) {
	who := clientQuoted(client)
	name := ""
	if !paths.BadRunes(label) && label != "" {
		name = " «" + shortName(label) + "»"
	}
	switch kind {
	case askKindCreate:
		return askBody{
			Title: "Создать альбом из папки" + name + "?",
			Lines: []string{who + " просит создать новый альбом из папки:", docLine(album, doc), askWho},
			OK:    "Создать",
		}, true
	case askKindAdd:
		return askBody{
			Title: "Добавить папку" + name + "?",
			Lines: []string{who + " просит добавить в открытый альбом «" + shortName(album) + "» папку:", askWho},
			OK:    "Добавить",
		}, true
	}
	return askBody{}, false
}

// docLine — что станет с открытым альбомом.
func docLine(album string, doc askDoc) string {
	switch doc {
	case askDocSaved:
		return "Открытый «" + shortName(album) + "» сохранится в свой файл."
	case askDocDirty:
		return "Несохранённые правки «" + shortName(album) + "» сохранятся копией."
	case askDocDraft:
		if shortName(album) == askNoName {
			return "Несохранённый черновик уйдёт в резервные копии."
		}
		return "Несохранённый черновик «" + shortName(album) + "» уйдёт в резервные копии."
	}
	return "Пустой альбом закроется."
}

// askDialogTitle — заголовок окна ОС «Выбрать папку»: просит клиент, а не программа.
func askDialogTitle(client string) string {
	return "Папка с фотографиями — просит " + clientName(client)
}

// clientName — имя клиента для показа: плохие знаки, пустое, слишком длинное и похожее на
// программу («Sosnina», «Сосн»: подделка под окно программы) — «неизвестный клиент».
func clientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || paths.BadRunes(name) || utf8.RuneCountInString(name) > askNameMax || agents.Lookalike(name) {
		return askUnknown
	}
	return name
}

// clientQuoted — имя клиента для абзаца: в «…», чтобы чужие слова не сливались с нашими;
// неизвестный клиент без кавычек.
func clientQuoted(name string) string {
	n := clientName(name)
	if n == askUnknown {
		return "Неизвестный клиент"
	}
	return "Клиент «" + n + "»"
}

// shortName — название альбома для текста: свои файлы тоже могут быть чужими по происхождению,
// поэтому тот же фильтр знаков; пустое или плохое — «без названия», длинное — с «…».
func shortName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || paths.BadRunes(s) {
		return askNoName
	}
	if utf8.RuneCountInString(s) > askNameMax {
		return string([]rune(s)[:askNameMax-1]) + "…"
	}
	return s
}
