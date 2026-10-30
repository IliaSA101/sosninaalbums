package agents

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Правки агентом (149, design §1–2): общий вход правящих инструментов serveEdit, место правки,
// темп и записи отмены. Службa альбом не правит: каждое действие — замыкание Host (hostedit.go).

// Сроки правок по умолчанию (design §2.6); поля Options с теми же именами перекрывают.
const (
	askWait     = 10 * time.Second // сколько вызов ждёт ответ человека, затем pending
	askLife     = 120 * time.Second
	askCooldown = 30 * time.Second // пауза после первого отказа
	askMute     = 5 * time.Minute  // тишина после второго отказа подряд
	busyWait    = 2 * time.Second  // сколько правка ждёт, пока человек занят
	autoTimeout = 30 * time.Second // срок вызова autobuild
	scanSlack   = 5 * time.Second  // запас срока autobuild после ожидания ленты
	editRate    = 20               // правок в минуту на программу
	rateWindow  = time.Minute
)

// editRec — что служба вернула подключению последней правкой: отмена трогает только её.
type editRec struct {
	key     string
	rev     int
	created bool // последним было create_album: оно не отменяется
	spent   bool // правка была и снята (отмена, сдвиг rev): повторная отмена — «не своя», а не «правок не было»
}

// editState — состояние правок службы. Живёт со Service, а не с запуском: Close/Start счёт отказов
// и темп не обнуляют (измор переподключением — тот же измор, 0.33-149-20).
type editState struct {
	slot chan struct{} // одно место правки на программу
	now  func() time.Time
	rate int
	cool time.Duration
	mute time.Duration
	life time.Duration

	mu      sync.Mutex
	times   []time.Time                               // правки, дошедшие до места, за rateWindow
	recs    map[*mcp.ServerSession]editRec            // последняя правка подключения
	asks    map[*mcp.ServerSession]map[*pend]struct{} // открытые вопросы подключения; одна горутина ss.Wait на него
	strikes int                                       // закрытий вопроса без «да» подряд
	until   time.Time                                 // до этого момента вопросов нет
	long    bool                                      // until — пятиминутная тишина, а не пауза
	forever bool                                      // третья ступень: до перезапуска или AllowAsks
	cur     *pend                                     // последний вопрос (ждёт итога или отдан не весь)
	last    time.Time                                 // когда был последний отказ: ступени считаются в окне strikeWindow
	quiet   []time.Time                               // тихие папки и альбомы за quietWindow (perms.go)
	denied  map[string]bool                           // инструменты, о запрете которых уже сообщили (agents:denied)
}

func newEditState(o Options) *editState {
	return &editState{slot: make(chan struct{}, 1), now: o.Now, rate: o.Rate, cool: o.Cooldown, mute: o.Mute, life: o.AskLife,
		recs: map[*mcp.ServerSession]editRec{}, asks: map[*mcp.ServerSession]map[*pend]struct{}{}, denied: map[string]bool{}}
}

// takeSlot занимает место правки; нет — false. release безвреден при повторе: место отдаёт и тело
// вызова, и вопрос, которому оно передано.
func (e *editState) takeSlot() (release func(), ok bool) {
	select {
	case e.slot <- struct{}{}:
	default:
		return nil, false
	}
	var once sync.Once
	return func() { once.Do(func() { <-e.slot }) }, true
}

// countRate записывает правку, дошедшую до места, и говорит, укладывается ли программа в темп.
// Отказанные тоже записаны: агент, бьющийся в отказ, остаётся за пределом (М9).
func (e *editState) countRate() bool {
	now := e.now()
	cut := now.Add(-rateWindow)
	e.mu.Lock()
	defer e.mu.Unlock()
	i := 0
	for i < len(e.times) && !e.times[i].After(cut) {
		i++
	}
	e.times = append(e.times[i:], now)
	if over := len(e.times) - 4*e.rate; over > 0 { // память под залпом ограничена
		e.times = e.times[over:]
	}
	return len(e.times) <= e.rate
}

func (e *editState) setRec(ss *mcp.ServerSession, r editRec) {
	e.mu.Lock()
	e.recs[ss] = r
	e.mu.Unlock()
}

func (e *editState) getRec(ss *mcp.ServerSession) (editRec, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.recs[ss]
	return r, ok
}

// dropRec снимает запись: вторая отмена подряд — не своя (0.33-149-28), но подключение уже правило
// (spent), в отличие от нового без правок. Создание остаётся
// помеченным, чтобы отмена сразу после него говорила «не отменяется».
func (e *editState) dropRec(ss *mcp.ServerSession) {
	e.mu.Lock()
	if r := e.recs[ss]; r.created {
		e.recs[ss] = editRec{created: true}
	} else {
		e.recs[ss] = editRec{spent: true}
	}
	e.mu.Unlock()
}

// forgetSession — подключение закрыто окончательно: его запись больше не нужна.
func (e *editState) forgetSession(ss *mcp.ServerSession) {
	if e == nil {
		return
	}
	e.mu.Lock()
	delete(e.recs, ss)
	delete(e.asks, ss)
	e.mu.Unlock()
}

// gate — общий вход правящих инструментов (serveEdit), после схемы и места чтения: флажок →
// процесс не повышен → место правки → темп → ключ (и rev) → «человек не занят» → копия. Возвращает
// release места правки; тело освобождает его (c.keep — передал вопросу). ref nil — альбома не
// спрашивают (create_album).
func (c *call) gate(ref *AlbumRef, needRev bool) (release func(), err error) {
	s, h, sp := c.s, c.s.h, c.spec
	if sp.edit && c.blocked == "elevated" { // «Запретить» отсечено в serve; здесь остаётся процесс от администратора
		return nil, errElevated
	}
	rel, ok := s.ed.takeSlot()
	if !ok {
		return nil, errEditBusy
	}
	done := false
	defer func() { // отказ и паника Host не оставляют место занятым
		if !done {
			rel()
		}
	}()
	if !s.ed.countRate() {
		return nil, errEditRate
	}
	if ref != nil {
		v, ed := h.View()
		switch {
		case ed == 0:
			return nil, errNoAlbum
		case ref.Key != albumKey(ed):
			return nil, errAlbumChanged
		case needRev && ref.Rev != v.Rev:
			return nil, errAlbumEdited
		}
	}
	bctx, cancel := context.WithTimeout(c.ctx, s.o.BusyWait)
	busy := h.Busy(bctx)
	cancel()
	if busy {
		return nil, errUserBusy
	}
	if c.mode == prefs.PermAsk && (sp.edit || sp.view) && !sp.ask { // «Спрашивать» у правки и показа (design §5.2); папка и альбом спрашивают сами
		if err := c.askEdit(ref, needRev); err != nil {
			return nil, err
		}
	}
	if sp.backup && ref != nil {
		if err := h.Backup(ref.Key); err != nil {
			log.Printf("agents: %s: резервная копия: %v", sp.name, bare(err))
			return nil, errBackup
		}
	}
	done = true
	c.rel = rel
	return rel, nil
}

// regEdit регистрирует правящий инструмент: тело run идёт после gate. ref достаёт альбом из входа
// (nil — альбома нет); needRev — сверять ревизию агента с текущей.
func regEdit[In, Out any](e *toolEnv, spec toolSpec, tweak func(props map[string]*jsonschema.Schema), ref func(In) *AlbumRef, needRev bool,
	run func(*call, In) (Out, error)) {
	reg(e, spec, tweak, func(c *call, in In) (Out, error) {
		var zero Out
		c.editReq = editAskReq(spec.name, in)
		rel, err := c.gate(ref(in), needRev)
		if err != nil {
			return zero, err
		}
		defer func() {
			if !c.keep {
				rel()
			}
		}()
		return run(c, in)
	})
}

// hostErr приводит ошибку Host к тому, что скажем агенту: срок вызова вышел — правка не применена.
func (c *call) hostErr(err error) error {
	switch {
	case err != nil && c.ctx.Err() != nil:
		return errLate
	case errors.Is(err, ErrNoSpread):
		return noSpreadFor(c.spec.view) // разворот исчез между проверкой и правкой
	}
	return err
}

// noteOf — итог вызова словами для журнала (Call.Note); "" — обычный вызов.
func noteOf(spec toolSpec, ae *agentErr) string {
	switch ae {
	case errDenied:
		return noteDenied // не отказ человека в вопросе: решение принято заранее
	case errDeclined, errFolderTooWide, errFolderOurs, errAskCooldown, errAskMuted, errAskOff:
		return "отказано"
	case errAskTimeout, errAskGone:
		return "без ответа"
	case errEditRate:
		return "лимит" // человек не занят: программа ограничила темп правок
	case errEditBusy, errUserBusy, errAskBusy:
		return "занят"
	case errBackup:
		return "копия"
	case errLate:
		return "поздно"
	case errTimeout:
		if spec.edit || spec.await || spec.view {
			return "поздно"
		}
	}
	return ""
}
