package app

// Открытие альбома по просьбе агента (152, design §4): ключ из list_albums или выбор файла в окне ОС,
// всегда с вопросом (класс Q, тихого открытия нет — 0.35-152-6). Путь агент не получает и не присылает:
// ключ служба находит среди недавних, фасад — тем же ключом (agents.RecentKey) и теми же проверками, что
// «Файл ▸ Последние» (recentCheck). Прежний альбом уходит в свой файл или закреплённую копию (pinLeaving,
// как при create_album); «Открыть прежний» в тосте возвращает его.

import (
	"errors"
	"log"
	"path/filepath"
	"strings"
	"unicode/utf8"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// askKindOpen — вид вопроса «Открыть альбом» (AgentAsk.Kind).
const askKindOpen = "open"

const (
	askAltOpen    = "Выбрать другой…" // кнопка вопроса с показанным альбомом: окно ОС вместо него
	askOKOpen     = "Открыть"
	askOKOpenPick = "Выбрать файл…"
)

// recentPath — путь недавнего альбома с этим ключом; false — такого нет.
func (a *App) recentPath(key string) (string, bool) {
	for _, p := range a.loadConfig().Recent {
		if agents.RecentKey(p) == key {
			return p, true
		}
	}
	return "", false
}

// openCheckErr переводит отказ recentCheck в ошибку для агента: без пути и без текста ОС.
func openCheckErr(err error) error {
	switch {
	case errors.Is(err, errNotRecent):
		return agents.ErrNoRecent
	case errors.Is(err, paths.ErrNotLocal):
		return agents.ErrOpenRemote
	}
	return agents.ErrOpenFailed
}

// exportRunning — шов тестов: выгрузка идёт (флаг службы поднят до окон выгрузки и держится до конца фона).
var exportRunning = func(a *App) bool {
	if a.exp == nil {
		return false
	}
	running, _ := a.exp.Busy()
	return running
}

// openBusy — программа занята выгрузкой (окна выгрузки или фон): альбом не открывается и не закрывается.
func (a *App) openBusy() bool { return exportRunning(a) }

// askOpenAlbum — вопрос «Открыть альбом» (Host.Ask, вид open): проверки до окна, затем запись вопроса.
// rec == nil — вопроса нет, res и err — итог вызывающему.
func (a *App) askOpenAlbum(req agents.AskReq) (rec *askRec, res agents.AskRes, err error) {
	if a.openBusy() {
		return nil, res, agents.ErrExporting // как у create_album: «идёт выгрузка» (0.35-152-25)
	}
	cur := a.editor()
	who := clientQuoted(req.Client)
	doc := docLine(a.askAlbumName(cur), a.askDocOf(cur))
	body := askBody{
		Title: "Открыть альбом по просьбе агента?",
		Lines: []string{who + " просит открыть сохранённый альбом. Файл выберете вы.", doc, askWho},
		OK:    askOKOpenPick,
	}
	var plan folderPlan
	if req.Key != "" {
		path, ok := a.recentPath(req.Key)
		if !ok {
			return nil, res, agents.ErrNoRecent
		}
		if err := a.recentCheck(path); err != nil {
			return nil, res, openCheckErr(err)
		}
		plan.path = path
		title := "Открыть альбом"
		if name := openName(path); name != "" {
			title += " «" + name + "»"
		}
		lines := []string{who + " просит открыть альбом:"}
		if h := strings.TrimSpace(req.Hint); h != "" && !paths.BadRunes(h) && utf8.RuneCountInString(h) <= askNameMax {
			lines = append(lines, "Агент назвал: «"+h+"»") // человек сверяет, того ли нашла программа
		}
		body = askBody{Title: title + "?", Lines: append(lines, doc, askWho), OK: askOKOpen}
	}
	return a.askPost(req, cur, plan, body)
}

// openName — название альбома по файлу для вопроса и тоста; с плохими знаками пусто (чужой текст не должен
// подменить заголовок).
func openName(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), projectExt)
	if name == "" || name == "." || paths.BadRunes(name) {
		return ""
	}
	return shortName(name)
}

// AgentAnswerOpen — «Открыть» / «Выбрать файл…» в вопросе «Открыть альбом»: показанный альбом
// (other = false) или файл из окна ОС (other, «Выбрать другой…»). Интерфейс зовёт внутри newDoc.replace;
// отказ идёт через AnswerAgent(id, false). Не открылось — State.Cancelled, вопрос закрыт.
func (a *App) AgentAnswerOpen(id string, other bool) (st State, err error) {
	defer guardErr("AgentAnswerOpen", &err)
	rec, err := a.askClaim(id, askKindOpen, true)
	if err != nil || rec == nil {
		return a.cancelledState(), err
	}
	defer close(rec.idle)
	path, ok := a.openPick(rec, other)
	if !ok {
		return a.cancelledState(), nil
	}
	return a.openApply(rec, path)
}

// openPick — какой файл открывать. Показанный альбом проверяется заново (его могли убрать из последних);
// иначе окно ОС «Открыть» в папке альбомов. Отмена окна — «человек остановил», без счёта отказов.
func (a *App) openPick(rec *askRec, other bool) (path string, ok bool) {
	if rec.path != "" && !other {
		if !a.askBegin(rec) {
			return "", false
		}
		if err := a.recentCheck(rec.path); err != nil {
			a.askFail(rec, openCheckErr(err))
			return "", false
		}
		return rec.path, true
	}
	def := a.dialogDir(prefs.DirProject)
	if other && rec.path != "" {
		def = filepath.Dir(rec.path)
	}
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Открыть альбом — просит " + clientName(rec.req.Client),
		DefaultDirectory: def,
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil {
		log.Printf("окно открытия альбома по просьбе агента: %v", paths.NoPath(err))
	}
	if err != nil || path == "" {
		why := agents.WhyStopped
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: why}}, why)
		return "", false
	}
	if !a.askBegin(rec) {
		return "", false
	}
	return path, true
}

// openApply открывает path так же, как меню («Файл ▸ Открыть»): битый файл — ошибка человеку, файл новее
// программы — окно «Альбом из новой версии»; агенту в обоих случаях отказ без пути. Прежний альбом
// закрепляется (pinLeaving), «Открыть прежний» — в тосте.
func (a *App) openApply(rec *askRec, path string) (State, error) {
	if !a.askAllowed(rec.kind) { // разрешение сняли, пока вопрос висел
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: agents.WhyOff}}, agents.WhyOff)
		return a.cancelledState(), nil
	}
	if a.openBusy() {
		a.askFail(rec, agents.ErrExporting)
		return a.cancelledState(), nil
	}
	cur := a.editor()
	if cur != rec.ed { // человек открыл другой альбом, пока шёл вопрос: текст о судьбе прежнего устарел
		a.askFail(rec, agents.ErrAlbumChanged)
		return a.cancelledState(), nil
	}
	lv, err := a.pinLeaving(cur)
	if err != nil {
		a.askFail(rec, agents.ErrBackup)
		return a.cancelledState(), nil
	}
	defer lv.release()
	st, err := a.openPath(path)
	if err != nil {
		log.Printf("альбом по просьбе агента не открыт: %v", paths.NoPath(err))
		a.askFail(rec, agents.ErrOpenFailed)
		return st, err
	}
	if st.Cancelled { // файл новее программы: окно у человека, альбом прежний
		a.askFail(rec, agents.ErrOpenNewer)
		return st, nil
	}
	prev := lv.prev
	if prev != "" && paths.SamePath(prev, path) {
		prev = ""
	}
	a.setPrev(prev)
	ed := a.editor()
	res := agents.AskRes{Why: agents.WhyAnswered, Album: agents.AlbumRef{Key: a.askKey(ed), Rev: ed.View().Rev}}
	a.askFinish(rec, askOutcome{res: res}, agents.WhyAnswered)
	text := "Открыт альбом"
	if name := openName(path); name != "" {
		text += " «" + name + "»"
	}
	// Rev -1: «Вернуть» не предлагается, открытие не отменяется Ctrl+Z; вернуть прежний — «Открыть прежний».
	a.agentEdited(AgentEdited{Client: agentToastClient(rec.req.Client), Text: text, Rev: -1, Prev: prev != "", Quiet: true})
	return st, nil
}
