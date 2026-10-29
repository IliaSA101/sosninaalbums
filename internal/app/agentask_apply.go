package app

// Применение выбора человека в вопросе агента (149, design §3): проверка папки, флажок, ключ того
// же редактора, резервная копия и добавление папки одним шагом отмены. Путь выбранной папки
// наружу не идёт: ни агенту, ни в журнал, ни в log.txt (TestAgentFolderNoEcho).

import (
	"errors"
	"log"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// askApplyAdd — «Добавить папку»: выбранная папка в тот же открытый альбом. Возвращает вид после
// правки; false — вопрос закрыт без правки.
func (a *App) askApplyAdd(rec *askRec, dir string) (model.View, bool) {
	if !a.askCheckPicked(rec, dir) {
		return model.View{}, false
	}
	ed, err := a.agentEditor(rec.req.Key)
	if err == nil && ed != rec.ed { // тот же альбом, что при вопросе
		err = errAgentAlbumChanged
	}
	if err == nil {
		err = a.agentBackup(rec.req.Key)
	}
	if err != nil {
		a.askFail(rec, agentHostErr(err))
		return model.View{}, false
	}
	return a.askAddFolder(rec, ed, dir)
}

// askApplyCreate — «Создать альбом»: новый пустой альбом, в него выбранная папка. Прежний альбом
// сохраняет startNew; черновик занят — новый альбом не создан, папка в прежний не добавляется.
// Несохранённый черновик startNew удалил бы, а копия при пустом хранилище молча не делается, поэтому
// копия — до startNew и закреплена (новый черновик не вытеснит её чисткой); нет копии — отказ.
// Возвращает, создан ли новый альбом: нет — интерфейс видит отмену, а не новый проект.
func (a *App) askApplyCreate(rec *askRec, dir string) (created bool) {
	if !a.askCheckPicked(rec, dir) {
		return false
	}
	// Застрявший черновик startNew разблокирует сам и откажет, если не вышло (draft_locked) — копию
	// без разблокировки писать некуда, поэтому разблокировка здесь, до копии.
	if cur := a.editor(); a.askDocOf(cur) == askDocDraft && a.unlockDraft() {
		if name, err := a.agentPinned(cur); err != nil || name == "" {
			a.askFail(rec, agents.ErrBackup)
			return false
		}
	}
	next := blankEditor("album")
	a.startNew(next)
	if a.editor() != next {
		why := agents.WhyDraftLocked
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: why}}, why)
		return false
	}
	a.askAddFolder(rec, next, dir)
	return true
}

// askCheckPicked — выбранная папка годится и флажок правок ещё включён; иначе вопрос закрыт
// (too_wide / ours с тостом человеку, off) и false.
func (a *App) askCheckPicked(rec *askRec, dir string) bool {
	if err := a.askM().rules().folder(dir); err != nil {
		a.askReject(rec, err)
		return false
	}
	if on, _ := a.agentEditState(); !on {
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: agents.WhyOff}}, agents.WhyOff)
		return false
	}
	return true
}

// askReject — папка не подошла: тост человеку, агенту только причина без пути.
func (a *App) askReject(rec *askRec, err error) {
	why := agents.WhyTooWide
	if errors.Is(err, errFolderOurs) {
		why = agents.WhyOurs
	}
	a.notice(askFolderReject)
	a.askFinish(rec, askOutcome{res: agents.AskRes{Why: why}}, why)
}

// askFail закрывает вопрос ошибкой для агента; окну — gone (вопрос закончился без «да»).
func (a *App) askFail(rec *askRec, err error) {
	a.askFinish(rec, askOutcome{err: err}, agents.WhyGone)
}

// askAddFolder добавляет папку в ed одним шагом отмены, перечитывает ленту и отдаёт итог агенту.
func (a *App) askAddFolder(rec *askRec, ed *album.Editor, dir string) (model.View, bool) {
	v, r, err := ed.AddFolder(dir)
	if err != nil {
		log.Printf("папка по просьбе агента не добавлена: %v", paths.NoPath(err))
		a.askReject(rec, errFolderTooWide)
		return model.View{}, false
	}
	if a.editor() != ed { // альбом сменился: Editor закрыт, правка отклонена молча
		a.askFail(rec, agents.ErrAlbumChanged)
		return model.View{}, false
	}
	merged := r.Within != ""
	if !merged {
		a.rescan()
	}
	v = a.changed(v)
	a.folderAdded(dir, r)
	a.rememberDir(prefs.DirPhotos, filepath.Dir(dir)) // соседняя съёмка обычно лежит рядом
	a.mu.Lock()
	scanning := a.scanning || a.scan.Active
	a.mu.Unlock()
	res := agents.AskRes{
		Why:      agents.WhyAnswered,
		Album:    agents.AlbumRef{Key: a.askKey(ed), Rev: v.Rev},
		Folder:   folderNumber(v.Folders, dir, r.Within),
		Merged:   merged,
		Scanning: scanning,
	}
	a.askFinish(rec, askOutcome{res: res}, agents.WhyAnswered)
	return v, true
}

// askKey — ключ альбома для агента (тот же, что сверяет agentEditor).
func (a *App) askKey(ed *album.Editor) string {
	if kf := a.agentEd().opts.Key; kf != nil {
		return kf(ed.ID())
	}
	return agents.AlbumKey(ed.ID())
}

// folderNumber — номер добавленной (или уже содержащей её) папки в списке с 1.
func folderNumber(folders []string, dir, within string) int {
	want := dir
	if within != "" {
		want = within
	}
	i := slices.IndexFunc(folders, func(f string) bool { return paths.DirKey(f) == paths.DirKey(want) })
	if i < 0 {
		return len(folders)
	}
	return i + 1
}
