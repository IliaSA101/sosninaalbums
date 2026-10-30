package app

// Тихое применение папки и нового альбома при «Всегда» (150, design §4.1 п. 8–10), черновик агента
// и «Открыть прежний». Решение «можно ли» — agentquiet.go; здесь перечитывание разрешения и правила
// перед записью, сама правка одним шагом отмены, тост. Места не пополняются: ни тихая, ни отвеченная
// папка места не создают (0.34-150-16).

import (
	"errors"
	"fmt"
	"log"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// quietAsk — итог quietApply: вместо тишины обычный вопрос без причины («Спрашивать»).
const quietAsk = "-"

// errNoPrev — «Открыть прежний» без прежнего: тост устарел.
var errNoPrev = errors.New("прежнего альбома нет")

// agentDraft — альбом, созданный create_album: «нетронутый», пока его ревизия равна ревизии последней
// правки агента (design §4.1 п. 8). Правил человек — ревизия ушла вперёд, тихо заменить нельзя.
type agentDraft struct {
	ed  *album.Editor
	rev int
}

// askMode — разрешение инструмента вида kind сейчас.
func (a *App) askMode(kind string) string {
	mode, _ := a.agentPerm(askTool(kind))
	return mode
}

// draftSet запоминает черновик агента после создания альбома.
func (a *App) draftSet(ed *album.Editor, rev int) {
	m := a.askM()
	m.mu.Lock()
	m.draft = agentDraft{ed: ed, rev: rev}
	m.mu.Unlock()
}

// setPrev запоминает файл для «Открыть прежний» ("" — прежнего нет).
func (a *App) setPrev(path string) {
	m := a.askM()
	m.mu.Lock()
	m.prev = path
	m.mu.Unlock()
}

// draftUntouched — ed создан агентом и с тех пор менялся только его правками.
func (a *App) draftUntouched(ed *album.Editor) bool {
	if ed == nil {
		return false
	}
	m := a.askM()
	m.mu.Lock()
	d := m.draft
	m.mu.Unlock()
	return d.ed == ed && ed.View().Rev == d.rev
}

// agentDraftStep — правка агента сдвинула ревизию с before на after: черновик остаётся нетронутым,
// если агент правил именно его и именно от «нетронутой» ревизии. Зовут переходники Host после
// каждой правки агента; правка, о которой фасад не знает, делает черновик «тронутым» — вопрос.
func (a *App) agentDraftStep(key string, before, after int) {
	ed := a.editor()
	kf := a.agentEd().opts.Key
	if ed == nil || kf == nil || key == "" || key != kf(ed.ID()) {
		return
	}
	m := a.askM()
	m.mu.Lock()
	if m.draft.ed == ed && m.draft.rev == before {
		m.draft.rev = after
	}
	m.mu.Unlock()
}

// quietApply применяет план без вопроса. Итог: res/err — окончательный ответ службе (WhyQuiet — успех);
// why != "" — тихо нельзя (разрешение «Спрашивать», правило или альбом изменились за время пробы):
// задавать обычный вопрос с этой причиной. Разрешение и правило перечитываются здесь, перед записью.
func (a *App) quietApply(req agents.AskReq, ed *album.Editor, plan folderPlan) (res agents.AskRes, why string, err error) {
	mode, blocked := a.agentPerm(askTool(req.Kind))
	if blocked != "" || mode == prefs.PermDeny {
		return agents.AskRes{Why: agents.WhyOff}, "", nil
	}
	if mode != prefs.PermAllow {
		return res, quietAsk, nil // стало «Спрашивать»: обычный вопрос без причины
	}
	r := a.askM().rules()
	_, resolved, rerr := r.resolve(plan.path)
	linked, done := within(r.timeout, func() bool { return hasLinkPart(resolved) })
	if rerr != nil || r.key(resolved) != r.key(plan.path) || !done || linked {
		return res, whyLink, nil // подмена между пробой и применением
	}
	if req.Kind == agents.AskCreate {
		return a.quietCreate(req, ed, resolved)
	}
	got, gerr := a.agentEditor(req.Key)
	if gerr != nil {
		return res, "", agentHostErr(gerr)
	}
	if got != ed {
		return res, "", agents.ErrAlbumChanged
	}
	if ed.View().Rev != req.Rev {
		return res, "", agents.ErrAlbumEdited
	}
	if berr := a.agentBackup(req.Key); berr != nil { // копии нет — правки нет
		return res, "", agentHostErr(berr)
	}
	res, err = a.quietAdd(ed, resolved)
	if err == nil && res.Why == agents.WhyQuiet && !res.Merged {
		a.agentEdited(AgentEdited{Client: agentToastClient(req.Client), Text: "Добавлена папка", Rev: res.Album.Rev,
			Quiet: true, Path: resolved})
	}
	return res, "", err
}

// quietCreate — тихий новый альбом: прежний без правок остаётся в своём файле, нетронутый черновик
// агента и файл с несохранёнными правками — в закреплённой копии (файл не переписывается);
// человеческий черновик заменять нельзя (draft).
func (a *App) quietCreate(req agents.AskReq, cur *album.Editor, resolved string) (res agents.AskRes, why string, err error) {
	if a.editor() != cur { // пока шла проба папки, человек открыл другой альбом
		return res, "", agents.ErrAlbumChanged
	}
	if a.askDocOf(cur) == askDocDraft && !a.draftUntouched(cur) {
		return res, whyDraft, nil
	}
	lv, perr := a.pinLeaving(cur)
	if perr != nil {
		return res, "", agents.ErrBackup
	}
	defer lv.release()
	next := blankEditor("album")
	// Сверка «текущий всё ещё cur» — под saveMu, в одном шаге со сменой: B человека не уводим.
	if _, changed := a.startNewIf(cur, next); changed {
		return res, "", agents.ErrAlbumChanged
	}
	if a.editor() != next {
		return agents.AskRes{Why: agents.WhyDraftLocked}, "", nil
	}
	a.setPrev(lv.prev)
	res, err = a.quietAdd(next, resolved)
	if err != nil || res.Why != agents.WhyQuiet {
		if a.editor() == next { // альбом создан, а папка не добавилась: человеку — тост с «Открыть прежний»
			a.agentEdited(AgentEdited{Client: agentToastClient(req.Client), Text: "Создан пустой альбом: папка не добавлена",
				Rev: next.View().Rev, Prev: lv.prev != "", Quiet: true})
		}
		return res, "", err
	}
	a.draftSet(next, res.Album.Rev)
	a.agentEdited(AgentEdited{Client: agentToastClient(req.Client), Text: "Новый альбом из", Rev: res.Album.Rev,
		Prev: lv.prev != "", Quiet: true, Path: resolved})
	return res, "", nil
}

// quietAddFolder — шов тестов: добавление папки в альбом.
var quietAddFolder = func(ed *album.Editor, dir string) (model.View, model.FolderAdd, error) { return ed.AddFolder(dir) }

// quietAdd добавляет папку в ed одним шагом отмены и перечитывает ленту; места не пополняются.
func (a *App) quietAdd(ed *album.Editor, dir string) (agents.AskRes, error) {
	v, r, err := quietAddFolder(ed, dir)
	if err != nil {
		log.Printf("папка по просьбе агента не добавлена: %v", paths.NoPath(err))
		return agents.AskRes{Why: agents.WhyTooWide}, nil
	}
	if a.editor() != ed { // альбом сменился: Editor закрыт, правка отклонена молча
		return agents.AskRes{}, agents.ErrAlbumChanged
	}
	merged := r.Within != ""
	if !merged {
		a.rescan()
	}
	v = a.changed(v)
	a.folderAdded(dir, r)
	return agents.AskRes{
		Why:      agents.WhyQuiet,
		Album:    agents.AlbumRef{Key: a.askKey(ed), Rev: v.Rev},
		Folder:   folderNumber(v.Folders, dir, r.Within),
		Merged:   merged,
		Scanning: a.agentsScanning(),
	}, nil
}

// AgentOpenPrev — «Открыть прежний» из тоста тихого нового альбома: любой несохранённый текущий
// закрепляется копией (черновик агента, черновик, который успел править человек, и свой файл с
// несохранёнными правками), затем открывается прежний файл или закреплённая копия. Нет копии —
// прежний не открывается. Ни прежний файл, ни файл с правками человека программа не переписывает.
func (a *App) AgentOpenPrev() (st State, err error) {
	defer guardErr("AgentOpenPrev", &err)
	m := a.askM()
	m.mu.Lock()
	prev := m.prev
	m.mu.Unlock()
	if prev == "" {
		return a.GetState(), errNoPrev
	}
	if cur := a.editor(); cur != nil {
		lv, perr := a.pinLeaving(cur)
		if perr != nil {
			return a.GetState(), fmt.Errorf("текущий альбом не удалось сохранить отдельно, прежний не открыт: %w", errAgentBackup)
		}
		defer lv.release()
	}
	st, err = a.openPath(prev)
	if err == nil && !st.Cancelled {
		a.setPrev("")
		if ed := a.editor(); ed != nil && ed.Path() == "" && a.backups.Inside(prev) {
			// Копия с правками, которых нет ни в каком файле: отметка «не сохранено», закрытие спросит.
			ed.MarkUnsaved()
			st = a.emitState()
		}
	}
	return st, err
}
