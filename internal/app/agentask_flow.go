package app

// Обёртка над машиной вопроса: окна ОС, правила, черновик; сама машина — пакет agentask.
//
// Машина вопроса агента человеку (149, design §3, 0.33-149-24): одна запись под одним mutex,
// состояния asked (срок идёт) → choosing (окно ОС открыто, срок стоит) → done. Счёт отказов,
// паузы и тишина — в службе агентов; сюда они не заходят. Применение ответа — agentask_apply.go.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentask"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentasktext"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// errAskClosed — ответ на вопрос, которого нет: закрыт, чужой или другого вида.
var errAskClosed = agentask.ErrClosed

// Сроки машины по умолчанию (design §2.6).
const (
	askLifeDefault     = 120 * time.Second // AskLife: жизнь вопроса в состоянии asked
	askExitWaitDefault = 3 * time.Second   // сколько выход ждёт закрытия окна ОС
)

// askPay — нагрузка вопроса для машины agentask: пишется до Post и не меняется.
type askPay struct {
	req  agents.AskReq
	ed   *album.Editor // add: редактор, чей ключ и ревизию агент назвал
	dir  string        // подсказка агента, годная для окна ОС ("" — нет)
	path string        // 150: настоящий путь показанной подсказки; "" — вопрос без пути
	exp  *exportAsk    // export: настройки и показанная папка (agentexport.go)
}

// askRec — открытый вопрос; ID и Kind (agentasktext.KindCreate | KindAdd | askKindEdit | askKindOpen |
// askKindExport) — в самой записи, остальное — в нагрузке.
type askRec = agentask.Rec[askPay]

// askMachine — состояние вопросов одного App: сама машина (agentask) и то, что ей знать нельзя —
// окна ОС, правила, черновик и прежний файл; окна ОС и сроки подменяются тестами.
type askMachine struct {
	q        atomic.Pointer[agentask.Machine[askPay]] // тесты подменяют целиком (newAskQ): сроки задаёт только конструктор
	mu       sync.Mutex                               // draft и prev; с записью вопроса за один захват не читаются
	front    func()                                   // окно программы вперёд
	pickDir  func(title, def string) (string, error)  // окно ОС «Выбрать папку»
	rules    func() *agentplaces.Rules                // правила выбранной папки (тесты — мягкие)
	elevated func() bool                              // процесс от администратора; тесты: раннер Windows сам администратор
	draft    agentDraft                               // 150: альбом, созданный агентом и не тронутый человеком
	prev     string                                   // 150: файл для «Открыть прежний» после тихого нового альбома
}

// newAskQ — машина вопросов со сроками; итог уходит в окно событием agents:askdone (паника отправки не
// роняет процесс: итог зовёт и таймер срока из своей горутины).
func (a *App) newAskQ(life, exitWait time.Duration) *agentask.Machine[askPay] {
	return agentask.New[askPay](life, exitWait, func(id, why string) {
		defer guard("agents:askdone")
		a.agentAskDone(AgentAskDone{ID: id, Why: why})
	})
}

// askQ — машина вопроса этого App.
func (a *App) askQ() *agentask.Machine[askPay] {
	return a.askM().q.Load()
}

// askM — машина вопросов этого App (поле askMach, лениво: и у &App{} в тестах).
func (a *App) askM() *askMachine {
	if m := a.askMach.Load(); m != nil {
		return m
	}
	m := &askMachine{elevated: osx.Elevated}
	m.q.Store(a.newAskQ(askLifeDefault, askExitWaitDefault))
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
	o := a.askQ().Wait(ctx, rec) // в choosing итог придёт от ответа человека или от выхода
	return o.Res, o.Err
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
	var body agentasktext.Body
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
		body, ok = agentasktext.Text(req.Kind, req.Client, a.askAlbumName(ed), a.askDocOf(ed))
		if plan.path != "" {
			body, ok = agentasktext.TextPath(req.Kind, req.Client, a.askAlbumName(ed), a.askDocOf(ed), plan.label)
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
func (a *App) askPost(req agents.AskReq, ed *album.Editor, plan folderPlan, body agentasktext.Body) (rec *askRec, res agents.AskRes, err error) {
	return a.askPostWith(req, ed, plan, body, nil)
}

// askPostWith — askPost с данными выгрузки (nil у прочих видов): они лежат в записи вопроса до щелчка, а в событие
// идут число файлов и «папка будет создана». Путь с чужими файлами показан, но «Выбрать другую…» нет.
func (a *App) askPostWith(req agents.AskReq, ed *album.Editor, plan folderPlan, body agentasktext.Body, ex *exportAsk) (rec *askRec, res agents.AskRes, err error) {
	id, err := newAskID()
	if err != nil {
		return nil, res, err
	}
	rec = &askRec{ID: id, Kind: req.Kind, Payload: askPay{req: req, ed: ed, dir: plan.dir, path: plan.path, exp: ex}}
	prevDone, err := a.askQ().Post(rec) // askdone прежнего вопроса, который ещё может быть в пути
	if err != nil {
		return nil, res, err
	}
	tool := askToolOf(req)
	switch req.Kind {
	case agents.AskExport:
		tool = "" // у выгрузки ссылки «Не спрашивать» нет (0.35-152-21)
	case agents.AskOpen:
		tool = openAskTool(req, a.askMode(agents.AskOpen)) // 160-Т5-8: ссылка только там, где «Всегда» сделает вопрос тихим
	}
	ask := AgentAsk{ID: id, Kind: req.Kind, Title: body.Title, Lines: body.Lines, Hint: plan.label, OK: body.OK,
		Path: plan.path, Link: plan.link, Tool: tool, Why: plan.why}
	if plan.path != "" {
		ask.Alt = agentasktext.Alt
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
	a.askM().front()
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
func (a *App) askDocOf(ed *album.Editor) agentasktext.Doc {
	if ed == nil {
		return agentasktext.DocEmpty
	}
	v := ed.View()
	switch {
	case v.Path != "" && v.Dirty:
		return agentasktext.DocDirty
	case v.Path != "":
		return agentasktext.DocSaved
	case len(v.Spreads) == 0 && len(v.Folders) == 0 && v.Cover == nil:
		return agentasktext.DocEmpty
	}
	return agentasktext.DocDraft
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
func (a *App) askFinish(rec *askRec, o agentask.Outcome, why string) bool {
	return a.askQ().FinishIf(rec, agentask.Any, o, why)
}

// askClaim сверяет ответ человека с открытым вопросом (agentask.Claim); разрешение на момент ответа
// читается здесь: его могли снять, пока вопрос висел, а диск — вне замка машины. rec != nil — вопрос в
// choosing, обработчик обязан вызвать askQ().Release(rec).
func (a *App) askClaim(id, kind string, yes bool) (*askRec, error) {
	return a.askQ().Claim(id, kind, yes, a.askAllowed(kind))
}

// askPick — выбор папки для применения. Вопрос с показанным путём и без «Выбрать другую…» применяет
// этот путь (повтор правила, сверка настоящего пути: подмена — too_wide); иначе открывает окно ОС
// (other — в родителе показанного пути). false — выбора нет и вопрос уже закрыт (отмена окна — declined).
func (a *App) askPick(rec *askRec, other bool) (dir string, ok bool) {
	if rec.Payload.path != "" && !other {
		if !a.askQ().Begin(rec) {
			return "", false
		}
		r := a.askM().rules() // один раз: Default бывает nil, Resolve на nil отказывает
		_, resolved, err := r.Resolve(rec.Payload.path)
		if err != nil || r.Key(resolved) != r.Key(rec.Payload.path) {
			a.askReject(rec, agentplaces.ErrTooWide)
			return "", false
		}
		return resolved, true
	}
	def := rec.Payload.dir
	if other && rec.Payload.path != "" {
		def = filepath.Dir(rec.Payload.path)
	}
	if def == "" {
		def = a.dialogDir(prefs.DirPhotos)
	}
	dir, err := a.askM().pickDir(agentasktext.DialogTitle(rec.Payload.req.Client), def)
	if err != nil {
		log.Printf("окно выбора папки по просьбе агента: %v", paths.NoPath(err))
	}
	if err != nil || dir == "" {
		a.askFinish(rec, agentask.Outcome{Res: agents.AskRes{Why: agents.WhyDeclined}}, agents.WhyDeclined)
		return "", false
	}
	if !a.askQ().Begin(rec) {
		return "", false
	}
	return dir, true
}

// closeAgentAsk — выход: вопрос снимается до остановки службы (done{closed}), ждущий вызов получает
// closed. В choosing выход ждёт возврата окна ОС и применения (не дольше exitWait).
func (a *App) closeAgentAsk() { a.askQ().Close() }
