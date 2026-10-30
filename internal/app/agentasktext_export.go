package app

// Тексты вопроса «Выгрузить альбом» (152, design §3.5): шаблоны Go, интерфейс только рисует. Строки о
// выгрузке даёт exporter.Summary.Lines (те же слова, что окно «Экспорт»), строки о папке — здесь.

import (
	"fmt"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

const (
	askOKExport     = "Выгрузить"
	askOKExportPick = "Выбрать папку…"
)

// exportAskIn — что вопрос выгрузки знает: клиент, альбом, строки Check и итог разбора папки.
type exportAskIn struct {
	Client, Album string
	Lines         []string // exporter.Summary.Lines
	Plan          exportPlan
}

// exportAskText — заголовок, абзацы и кнопка вопроса. Путь рисует интерфейс сразу за первым абзацем
// (AgentAsk.Path), поэтому первый абзац кончается двоеточием, когда путь есть.
func exportAskText(in exportAskIn) askBody {
	who := clientQuoted(in.Client) + " просит выгрузить альбом «" + shortName(in.Album) + "» в JPEG"
	p := in.Plan
	body := askBody{Title: "Выгрузить альбом по просьбе агента?", OK: askOKExportPick}
	var lines []string
	if p.shown == "" {
		lines = append(lines, who+". Папку выберете вы.")
	} else {
		lines = append(lines, who+":")
		body.Title = "Выгрузить альбом" + exportDirName(p.shown) + "?"
		if p.approved() {
			body.OK = askOKExport
		}
	}
	lines = append(lines, in.Lines...)
	lines = append(lines, p.lines()...)
	body.Lines = append(lines, askWho)
	return body
}

// exportDirName — « в папку «Петровы»» по последнему элементу пути; с плохими знаками имени в заголовке нет
// (чужой текст не должен подменить заголовок).
func exportDirName(dir string) string {
	el := dir[strings.LastIndexAny(dir, `/\`)+1:]
	if el == "" || paths.BadRunes(el) {
		return ""
	}
	return " в папку «" + shortName(el) + "»"
}

// lines — строки о папке: создана ли, рядом ли с фото, сеть, облако, что заменится или помешает.
func (p exportPlan) lines() []string {
	var out []string
	if p.shown == "" {
		return out
	}
	if p.create {
		out = append(out, "Папка будет создана.")
	}
	if p.nextTo {
		out = append(out, "Рядом с папкой фото альбома — как в меню.")
	}
	if p.net {
		out = append(out, "Сетевой диск.")
	}
	if p.cloud {
		out = append(out, "Папка синхронизируется с облаком.")
	}
	switch {
	case p.foreign:
		out = append(out, "Сюда выгрузить нельзя: в папке чужие файлы с номерами разворотов. Выберите другую папку.")
	case len(p.ours) > 0:
		out = append(out, fmt.Sprintf("В папке уже есть выгрузка (%d %s) — заменится.", len(p.ours), filesWord(len(p.ours))))
	}
	return out
}

// filesWord — «файл», «файла», «файлов».
func filesWord(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return "файлов"
	case n%10 == 1:
		return "файл"
	case n%10 >= 2 && n%10 <= 4:
		return "файла"
	}
	return "файлов"
}
