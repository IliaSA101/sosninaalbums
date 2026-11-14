package agentasktext

// Тексты вопроса «Выгрузить альбом» (152, design §3.5): шаблоны Go, интерфейс только рисует. Строки о
// выгрузке даёт exporter.Summary.Lines (те же слова, что окно «Экспорт»), строки о папке — здесь.

import (
	"fmt"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

const (
	OKExport     = "Выгрузить"
	OKExportPick = "Выбрать папку…"
)

// ExportDir — итог разбора папки выгрузки, как его видит текст вопроса (поля exportPlan из app).
type ExportDir struct {
	Shown   string // настоящий путь для вопроса; "" — окно ОС
	Create  bool   // папки ещё нет: создаётся после вопросов
	NextTo  bool   // путь был в папке фото альбома — показана соседняя
	Net     bool   // сетевой диск
	Cloud   bool   // синхронизируется с облаком
	Foreign bool   // в папке чужие файлы с номерами: выгрузить нельзя
	Ours    int    // сколько своих файлов заменится
}

// Approved — путь показан и годится: «Выгрузить» пишет в него без окна ОС.
func (d ExportDir) Approved() bool { return d.Shown != "" && !d.Foreign }

// ExportIn — что вопрос выгрузки знает: клиент, альбом, строки Check и итог разбора папки.
type ExportIn struct {
	Client, Album string
	Lines         []string // exporter.Summary.Lines
	Dir           ExportDir
}

// ExportText — заголовок, абзацы и кнопка вопроса. Путь рисует интерфейс сразу за первым абзацем
// (AgentAsk.Path), поэтому первый абзац кончается двоеточием, когда путь есть.
func ExportText(in ExportIn) Body {
	who := ClientQuoted(in.Client) + " просит выгрузить альбом «" + ShortName(in.Album) + "» в JPEG"
	p := in.Dir
	body := Body{Title: "Выгрузить альбом по просьбе агента?", OK: OKExportPick}
	var lines []string
	if p.Shown == "" {
		lines = append(lines, who+". Папку выберете вы.")
	} else {
		lines = append(lines, who+":")
		body.Title = "Выгрузить альбом" + exportDirName(p.Shown) + "?"
		if p.Approved() {
			body.OK = OKExport
		}
	}
	lines = append(lines, in.Lines...)
	lines = append(lines, p.lines()...)
	body.Lines = append(lines, Who)
	return body
}

// exportDirName — « в папку «Петровы»» по последнему элементу пути; с плохими знаками имени в заголовке нет
// (чужой текст не должен подменить заголовок).
func exportDirName(dir string) string {
	el := dir[strings.LastIndexAny(dir, `/\`)+1:]
	if el == "" || paths.BadRunes(el) {
		return ""
	}
	return " в папку «" + ShortName(el) + "»"
}

// lines — строки о папке: создана ли, рядом ли с фото, сеть, облако, что заменится или помешает.
func (p ExportDir) lines() []string {
	var out []string
	if p.Shown == "" {
		return out
	}
	if p.Create {
		out = append(out, "Папка будет создана.")
	}
	if p.NextTo {
		out = append(out, "Рядом с папкой фото альбома — как в меню.")
	}
	if p.Net {
		out = append(out, "Сетевой диск.")
	}
	if p.Cloud {
		out = append(out, "Папка синхронизируется с облаком.")
	}
	switch {
	case p.Foreign:
		out = append(out, "Сюда выгрузить нельзя: в папке чужие файлы с номерами разворотов. Выберите другую папку.")
	case p.Ours > 0:
		out = append(out, fmt.Sprintf("В папке уже есть выгрузка (%d %s) — заменится.", p.Ours, filesWord(p.Ours)))
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
