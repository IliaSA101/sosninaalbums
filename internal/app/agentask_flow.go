package app

// Машина вопроса агента человеку (149, design §3, 0.33-149-24): одна запись под одним mutex,
// состояния asked (срок идёт) → choosing (окно ОС открыто, срок стоит) → done. Счёт отказов,
// паузы и тишина — в службе агентов; сюда они не заходят. Применение ответа — agentask_apply.go.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// errAskClosed — ответ на вопрос, которого нет: закрыт, чужой или другого вида.
var errAskClosed = errors.New("вопрос закрыт")

// askState — состояние записи вопроса.
type askState int

const (
	askAsked    askState = iota // окно показано, срок идёт
	askChoosing                 // окно ОС открыто (или применяется выбор), срок стоит
	askDone                     // итог отдан
)

// askAny — askFinishIf из любого состояния, кроме done.
const askAny askState = -1

// Сроки машины по умолчанию (design §2.6).
const (
	askLifeDefault     = 120 * time.Second // AskLife: жизнь вопроса в состоянии asked
	askExitWaitDefault = 3 * time.Second   // сколько выход ждёт закрытия окна ОС
)

// askOutcome — итог вопроса для ждущего вызова Host.Ask.
type askOutcome struct {
	res agents.AskRes
	err error
}

// askRec — открытый вопрос.
type askRec struct {
	id      string
	kind    string // askKindCreate | askKindAdd | askKindEdit | askKindOpen | askKindExport
	req     agents.AskReq
	ed      *album.Editor // add: редактор, чей ключ и ревизию агент назвал
	dir     string        // подсказка агента, годная для окна ОС ("" — нет)
	path    string        // 150: настоящий путь показанной подсказки; "" — вопрос без пути
	exp     *exportAsk    // export: настройки и показанная папка (agentexport.go)
	state   askState
	closing bool            // выход начался: выбор не применять
	timer   *time.Timer     // срок asked
	out     chan askOutcome // итог (буфер 1)
	idle    chan struct{}   // закрыт, когда обработчик ответа вернулся (только после choosing)
}

// askMachine — состояние вопросов одного App; окна ОС и сроки подменяются тестами.
type askMachine struct {
	mu       sync.Mutex
	cur      *askRec
	life     time.Duration
	exitWait time.Duration
	front    func()                                  // окно программы вперёд
	pickDir  func(title, def string) (string, error) // окно ОС «Выбрать папку»
	rules    func() *agentplaces.Rules               // правила выбранной папки (тесты — мягкие)
	elevated func() bool                             // процесс от администратора; тесты: раннер Windows сам администратор
	draft    agentDraft                              // 150: альбом, созданный агентом и не тронутый человеком
	prev     string                                  // 150: файл для «Открыть прежний» после тихого нового альбома
	doneEmit chan struct{}                           // закрыт, когда agents:askdone последнего итога ушёл в окно (nil — итогов не было)
}

// askM — машина вопросов этого App (поле askMach, лениво: и у &App{} в тестах).
func (a *App) askM() *askMachine {
	if m := a.askMach.Load(); m != nil {
		return m
	}
	m := &askMachine{life: askLifeDefault, exitWait: askExitWaitDefault, elevated: osx.Elevated}
	m.front = func() {
		if a.ctx != nil {
			wr.WindowUnminimise(a.ctx)
			wr.WindowShow(a.ctx)
		}
	}
	m.pickDir = func(title, def string) (string, error) {
		if a.ctx == nil {
			return "", nil
		}
		return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{Title: title, DefaultDirectory: def})
	}
	m.rules = func() *agentplaces.Rules {
		_, thumbs := dataDirs(a.data)
		return agentplaces.Default(a.cfgDir, thumbs)
	}
	a.askMach.CompareAndSwap(nil, m)
	return a.askMach.Load()
}

// agentAsk — Host.Ask: вопрос человеку в окне программы. Ждёт итог или конец ctx (ctx отменён —
// gone, кроме выбора папки: там итог виден в get_album). Путь агенту не сообщается.
func (a *App) agentAsk(ctx context.Context, req agents.AskReq) (res agents.AskRes, err error) {
	defer guardErr("agentAsk", &err)
	rec, res, err := a.askOpen(req)
	if rec == nil {
		return res, err
	}
	select {
	case o := <-rec.out:
		return o.res, o.err
	case <-ctx.Done():
	}
	a.askFinishIf(rec, askAsked, askOutcome{res: agents.AskRes{Why: agents.WhyGone}}, agents.WhyGone)
	o := <-rec.out // в choosing итог придёт от ответа человека или от выхода
	return o.res, o.err
}

// askOpen проверяет вопрос, заводит запись и показывает окно. rec == nil — вопрос не задан: res и
// err уже итог для вызывающего.
func (a *App) askOpen(req agents.AskReq) (rec *askRec, res agents.AskRes, err error) {
	select {
	case <-a.ready:
	default:
		return nil, res, agents.ErrNotReady
	}
	if a.stopped.Load() {
		return nil, agents.AskRes{Why: agents.WhyClosed}, nil
	}
	if req.Kind != agents.AskCreate && req.Kind != agents.AskAdd && req.Kind != agents.AskEdit && req.Kind != agents.AskOpen && req.Kind != agents.AskExport {
		return nil, res, errors.New("неизвестный вид вопроса")
	}
	if !a.askAllowedTool(askToolOf(req)) {
		return nil, agents.AskRes{Why: agents.WhyOff}, nil
	}
	if req.Kind == agents.AskOpen { // 152: ключ или выбор файла, своё применение — agentopen.go
		return a.askOpenAlbum(req)
	}
	if req.Kind == agents.AskExport { // 152: выгрузка с полным путём, применение — agentexport.go
		return a.askExportAlbum(req)
	}
	if req.Kind == agents.AskCreate && a.openBusy() { // выгрузка идёт (окна или фон): картина не меняется (0.35-152-27)
		return nil, res, agents.ErrExporting
	}
	ed, err := a.askTarget(req)
	if err != nil {
		return nil, res, err
	}
	var plan folderPlan
	var body askBody
	if req.Kind == agents.AskEdit { // правка и показ: окна ОС и папки нет
		if body, err = a.editAskText(req, ed); err != nil {
			return nil, res, err
		}
	} else {
		always := a.askMode(req.Kind) == prefs.PermAllow
		plan = a.planFolder(req, ed, always)
		if req.Always && plan.quiet() {
			qres, why, qerr := a.quietApply(req, ed, plan)
			if why == "" {
				return nil, qres, qerr
			}
			if plan.why = why; why == quietAsk {
				plan.why = ""
			}
		}
		ok := false
		body, ok = askText(req.Kind, req.Client, a.askAlbumName(ed), a.askDocOf(ed))
		if plan.path != "" {
			body, ok = askTextPath(req.Kind, req.Client, a.askAlbumName(ed), a.askDocOf(ed), plan.label)
		}
		if !ok {
			return nil, res, errors.New("нет текста вопроса")
		}
		if plan.note != "" {
			body.Lines = append(slices.Clone(body.Lines), plan.note) // «Место … — на диске …»
		}
	}
	return a.askPost(req, ed, plan, body)
}

// askPost заводит запись вопроса и показывает окно. Один вопрос за раз: занято — ErrAskBusy.
func (a *App) askPost(req agents.AskReq, ed *album.Editor, plan folderPlan, body askBody) (rec *askRec, res agents.AskRes, err error) {
	return a.askPostWith(req, ed, plan, body, nil)
}

// askPostWith — askPost с данными выгрузки (nil у прочих видов): они лежат в записи вопроса до щелчка, а в событие
// идут число файлов и «папка будет создана». Путь с чужими файлами показан, но «Выбрать другую…» нет.
func (a *App) askPostWith(req agents.AskReq, ed *album.Editor, plan folderPlan, body askBody, ex *exportAsk) (rec *askRec, res agents.AskRes, err error) {
	id, err := newAskID()
	if err != nil {
		return nil, res, err
	}
	m := a.askM()
	rec = &askRec{id: id, kind: req.Kind, req: req, ed: ed, dir: plan.dir, path: plan.path, exp: ex, out: make(chan askOutcome, 1)}
	m.mu.Lock()
	if m.cur != nil {
		m.mu.Unlock()
		return nil, res, agents.ErrAskBusy
	}
	m.cur = rec
	prevDone := m.doneEmit // askdone прежнего вопроса, который ещё может быть в пути
	rec.timer = time.AfterFunc(m.life, func() {
		defer guard("срок вопроса агента")
		a.askFinishIf(rec, askAsked, askOutcome{res: agents.AskRes{Why: agents.WhyTimeout}}, agents.WhyTimeout)
	})
	m.mu.Unlock()
	tool := askToolOf(req)
	if req.Kind == agents.AskOpen || req.Kind == agents.AskExport {
		tool = "" // у выгрузки и открытия ссылки «Не спрашивать» нет (0.35-152-21)
	}
	ask := AgentAsk{ID: id, Kind: req.Kind, Title: body.Title, Lines: body.Lines, Hint: plan.label, OK: body.OK,
		Path: plan.path, Link: plan.link, Tool: tool, Why: plan.why}
	if plan.path != "" {
		ask.Alt = askAlt
		if req.Kind == agents.AskOpen {
			ask.Alt = askAltOpen
		}
	}
	if ex != nil {
		ask.Files, ask.Create = ex.files, ex.plan.create
		if !ex.plan.approved() {
			ask.Alt = "" // выбор один — окно ОС («Выбрать папку…»)
		}
	}
	// agents:ask не обгоняет agents:askdone прежнего вопроса: окно иначе проигнорирует чужой askdone
	// и потеряет тост «не дождался ответа». Ждём только событие, не итог агента.
	if prevDone != nil {
		var stop <-chan struct{} // у &App{} в тестах stopCtx нет
		if a.stopCtx != nil {
			stop = a.stopCtx.Done()
		}
		select {
		case <-prevDone:
		case <-stop:
		}
	}
	a.agentAsked(ask)
	m.front()
	return rec, res, nil
}

// askTarget — add: открытый альбом того же ключа и ревизии, что видел агент; edit: альбом того же ключа
// (ревизию службы сверяет сама, до вопроса и после «да»; у show_spread её нет); create: текущий альбом
// (для текста о его судьбе). Ошибки — те, что служба скажет агенту.
func (a *App) askTarget(req agents.AskReq) (*album.Editor, error) {
	if req.Kind == agents.AskCreate {
		return a.editor(), nil
	}
	ed, err := a.agentEditor(req.Key)
	if err != nil {
		return nil, agentHostErr(err)
	}
	if req.Kind == agents.AskEdit {
		return ed, nil
	}
	if ed.View().Rev != req.Rev {
		return nil, agents.ErrAlbumEdited
	}
	return ed, nil
}

// askAlbumName — название открытого альбома для текста: имя файла без расширения; у черновика пусто.
func (a *App) askAlbumName(ed *album.Editor) string {
	if ed == nil || ed.Path() == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(ed.Path()), projectExt)
}

// askDocOf — что станет с открытым альбомом при создании нового.
func (a *App) askDocOf(ed *album.Editor) askDoc {
	if ed == nil {
		return askDocEmpty
	}
	v := ed.View()
	switch {
	case v.Path != "" && v.Dirty:
		return askDocDirty
	case v.Path != "":
		return askDocSaved
	case len(v.Spreads) == 0 && len(v.Folders) == 0 && v.Cover == nil:
		return askDocEmpty
	}
	return askDocDraft
}

// newAskID — 16 байт crypto/rand, hex.
func newAskID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// askFinish закрывает вопрос из любого состояния: итог ждущему вызову и agents:askdone окну.
// false — уже закрыт.
func (a *App) askFinish(rec *askRec, o askOutcome, why string) bool {
	return a.askFinishIf(rec, askAny, o, why)
}

// askFinishIf — askFinish только из состояния from: срок и gone не прерывают выбор папки
// (0.33-149-24).
func (a *App) askFinishIf(rec *askRec, from askState, o askOutcome, why string) bool {
	m := a.askM()
	m.mu.Lock()
	if rec.state == askDone || (from != askAny && rec.state != from) {
		m.mu.Unlock()
		return false
	}
	rec.state = askDone
	if rec.timer != nil {
		rec.timer.Stop()
	}
	// Запись снимается до итога: получив его, агент может сразу спросить снова, а не поймать ErrAskBusy.
	if m.cur == rec {
		m.cur = nil
	}
	emitted := make(chan struct{})
	m.doneEmit = emitted
	m.mu.Unlock()
	defer close(emitted) // следующий agents:ask уходит только после askdone (askPostWith)
	rec.out <- o         // итог не ждёт события: зависшая отправка в окно не держит агента
	a.agentAskDone(AgentAskDone{ID: rec.id, Why: why})
	return true
}

// askClaim сверяет ответ человека с открытым вопросом: id, вид и состояние asked. Отказ (yes =
// false) закрывает вопрос любого вида; «да» на add/create переводит в choosing (срок стоит) — rec
// возвращается, обработчик обязан закрыть rec.idle. rec == nil без ошибки — вопрос закрыт этим
// вызовом (отказ, флажок выключен). Id чужой, вопрос закрыт или вид «да» не совпал — errAskClosed,
// вопрос остаётся как был.
func (a *App) askClaim(id, kind string, yes bool) (*askRec, error) {
	on := a.askAllowed(kind) // разрешение могли снять, пока вопрос висел; диск — вне замка
	m := a.askM()
	m.mu.Lock()
	rec := m.cur
	if rec == nil || rec.state != askAsked || subtle.ConstantTimeCompare([]byte(id), []byte(rec.id)) != 1 ||
		(yes && rec.kind != kind) {
		m.mu.Unlock()
		return nil, errAskClosed
	}
	why := agents.WhyDeclined
	if yes && !on {
		why = agents.WhyOff
	}
	if !yes || !on {
		m.mu.Unlock()
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: why}}, why)
		return nil, nil
	}
	rec.state, rec.idle = askChoosing, make(chan struct{})
	rec.timer.Stop()
	m.mu.Unlock()
	return rec, nil
}

// askPick — выбор папки для применения. Вопрос с показанным путём и без «Выбрать другую…» применяет
// этот путь (повтор правила, сверка настоящего пути: подмена — too_wide); иначе открывает окно ОС
// (other — в родителе показанного пути). false — выбора нет и вопрос уже закрыт (отмена окна — declined).
func (a *App) askPick(rec *askRec, other bool) (dir string, ok bool) {
	if rec.path != "" && !other {
		if !a.askBegin(rec) {
			return "", false
		}
		r := a.askM().rules() // один раз: Default бывает nil, Resolve на nil отказывает
		_, resolved, err := r.Resolve(rec.path)
		if err != nil || r.Key(resolved) != r.Key(rec.path) {
			a.askReject(rec, agentplaces.ErrTooWide)
			return "", false
		}
		return resolved, true
	}
	def := rec.dir
	if other && rec.path != "" {
		def = filepath.Dir(rec.path)
	}
	if def == "" {
		def = a.dialogDir(prefs.DirPhotos)
	}
	dir, err := a.askM().pickDir(askDialogTitle(rec.req.Client), def)
	if err != nil {
		log.Printf("окно выбора папки по просьбе агента: %v", err) // путь в ошибке окна не бывает
	}
	if err != nil || dir == "" {
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: agents.WhyDeclined}}, agents.WhyDeclined)
		return "", false
	}
	if !a.askBegin(rec) {
		return "", false
	}
	return dir, true
}

// askBegin — выбор можно применять: вопрос не закрыт и выход не начался.
func (a *App) askBegin(rec *askRec) bool {
	m := a.askM()
	m.mu.Lock()
	defer m.mu.Unlock()
	return rec.state == askChoosing && !rec.closing
}

// closeAgentAsk — выход: вопрос снимается до остановки службы (done{closed}), ждущий вызов получает
// closed. В choosing выход ждёт возврата окна ОС и применения (не дольше exitWait: зависшее окно
// выход не держит); выбор после начала выхода не применяется.
func (a *App) closeAgentAsk() {
	m := a.askM()
	m.mu.Lock()
	rec := m.cur
	if rec == nil || rec.state == askDone {
		m.mu.Unlock()
		return
	}
	rec.closing = true
	idle := rec.idle
	m.mu.Unlock()
	if idle != nil {
		select {
		case <-idle:
		case <-time.After(m.exitWait):
		}
	}
	a.askFinish(rec, askOutcome{res: agents.AskRes{Why: agents.WhyClosed}}, agents.WhyClosed)
}
