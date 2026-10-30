package agents

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Вопросы агента человеку (149, design §3, служебная часть): жетон и pending, await_answer, счёт
// отказов на программу. Окно, выбор папки и состояния asked → choosing → done — у фасада (Host.Ask).

// pend — вопрос, ушедший в Host.Ask. Живёт, пока Ask не вернулся, и ещё AskLife после: итог ждёт
// await_answer (один раз).
type pend struct {
	token   string
	sess    *mcp.ServerSession
	kind    string
	tool    string        // инструмент, чей это вопрос: смена разрешения на «Запретить» закрывает вопрос (PermsChanged)
	off     bool          // закрыт запретом: итог off (под ed.mu)
	done    chan struct{} // закрыт, когда итог готов
	cancel  context.CancelFunc
	res     AskRes // итог (после done)
	err     error
	fin     time.Time // когда готов итог
	waiting bool      // кто-то уже ждёт по этому жетону (под ed.mu)
	took    bool      // итог отдан агенту
}

func (p *pend) finished() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// pendingText — что агент говорит человеку, пока вопрос открыт в окне программы.
const pendingText = "Вопрос открыт в окне Sosnina Albums. Попросите человека ответить там " +
	"(окно могло не подняться — оно на панели задач), затем вызовите await_answer"

type pendingOut struct {
	Ask  string `json:"ask" jsonschema:"жетон вопроса: передайте в await_answer"`
	Text string `json:"text" jsonschema:"что сказать человеку"`
}

type folderRes struct {
	N      int  `json:"n" jsonschema:"номер папки в альбоме с 1"`
	Merged bool `json:"merged" jsonschema:"папка уже была в альбоме: правки нет"`
}

type scanRes struct {
	Active bool `json:"active" jsonschema:"программа ещё читает фото папки"`
}

// askOut — ответ create_album, add_folder и await_answer: либо итог, либо pending.
type askOut struct {
	Album   *AlbumRef   `json:"album,omitempty"`
	Folder  *folderRes  `json:"folder,omitempty"`
	Scan    *scanRes    `json:"scan,omitempty"`
	Pending *pendingOut `json:"pending,omitempty"`
}

// admit — можно ли задавать вопрос: пауза после отказа, тишина, третья ступень.
func (e *editState) admit() error {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case e.forever:
		return errAskOff // третья ступень: сами не вернутся
	case now.Before(e.until) && e.long:
		return errAskMuted
	case now.Before(e.until):
		return errAskCooldown
	}
	return nil
}

// strikeWindow — отказ старше этого срока начинает счёт ступеней заново.
const strikeWindow = 30 * time.Minute

// strike — вопрос закрыт без «да»: пауза 30 с, второй подряд — 5 минут, третий — до перезапуска
// или AllowAsks. Возвращает, наступила ли третья ступень (интерфейсу надо обновиться).
func (e *editState) strike() (forever bool) {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	// Ступени считаются в окне strikeWindow (0.34-150-29): при «Всегда» «да» почти не бывает, и отказы
	// без окна копились бы часами. Залп окно не ослабляет; третья ступень (forever) окном не снимается.
	if !e.forever && e.strikes > 0 && now.Sub(e.last) > strikeWindow {
		e.strikes = 0
	}
	e.last = now
	e.strikes++
	switch e.strikes {
	case 1:
		e.until, e.long = now.Add(e.cool), false
	case 2:
		e.until, e.long = now.Add(e.mute), true
	default:
		forever = !e.forever
		e.forever = true
	}
	return forever
}

// yes — человек ответил «да»: счёт с начала.
func (e *editState) yes() {
	e.mu.Lock()
	e.strikes, e.until, e.long = 0, time.Time{}, false
	e.mu.Unlock()
}

// AsksMuted — вопросы отключены третьей ступенью (окно «ИИ-агенты»: «Разрешить вопросы»).
func (s *Service) AsksMuted() bool {
	s.ed.mu.Lock()
	defer s.ed.mu.Unlock()
	return s.ed.forever
}

// AllowAsks снимает тишину и счёт отказов («Разрешить вопросы», 0.33-149-20).
func (s *Service) AllowAsks() {
	s.ed.mu.Lock()
	was := s.ed.forever
	s.ed.strikes, s.ed.until, s.ed.long, s.ed.forever = 0, time.Time{}, false, false
	s.ed.mu.Unlock()
	if was {
		s.changed()
	}
}

// newToken — жетон вопроса: 16 байт crypto/rand. Не id окна: по нему вопрос ищет только подключение,
// которое его задало.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ask — вопрос человеку: проверки службы, затем Host.Ask в своей горутине. Вызов ждёт итог AskWait,
// не дождался — pending и await_answer. Место правки (c.rel) переходит к вопросу и держится до итога.
func (c *call) ask(req AskReq) (askOut, error) {
	s := c.s
	if err := s.ed.admit(); err != nil {
		return askOut{}, err
	}
	tok, err := newToken()
	if err != nil {
		return askOut{}, errInternal
	}
	// «Всегда» у папки: после admit() (пауза и тишина гасят и тихий путь) резервируем тихий слот под
	// замком службы и просим фасад решить, можно ли без вопроса (design §3).
	var stamp time.Time
	quiet := false
	if c.mode == prefs.PermAllow {
		if stamp, quiet = s.ed.takeQuiet(); quiet {
			req.Always = true
		}
	}
	actx, cancel := context.WithCancel(c.env.ctx)
	p := &pend{token: tok, sess: c.sess, kind: req.Kind, tool: c.spec.name, done: make(chan struct{}), cancel: cancel}
	s.ed.mu.Lock()
	s.ed.cur = p
	s.ed.mu.Unlock()
	s.ed.watchAsk(c.sess, p) // подключение закроется — агент ушёл: вопрос «gone»
	c.keep = true            // место правки теперь у вопроса
	c.env.r.wg.Add(1)
	go s.runAsk(actx, askRun{r: c.env.r, runCtx: c.env.ctx, p: p, req: req, release: c.rel, quiet: quiet, stamp: stamp})
	return c.waitAsk(p, true)
}

// watchAsk берёт вопрос p под присмотр подключения ss: когда оно закроется, вопрос отменяется
// («gone»). Горутина ss.Wait одна на подключение, а не на вопрос: агент, засыпающий вопросами,
// горутин не плодит.
func (e *editState) watchAsk(ss *mcp.ServerSession, p *pend) {
	if ss == nil {
		return
	}
	e.mu.Lock()
	set := e.asks[ss]
	first := set == nil
	if first {
		set = map[*pend]struct{}{}
		e.asks[ss] = set
	}
	set[p] = struct{}{}
	e.mu.Unlock()
	if first {
		go func() {
			defer guardPanic("watchAsk")
			_ = ss.Wait()
			e.mu.Lock()
			set := e.asks[ss]
			delete(e.asks, ss)
			e.mu.Unlock()
			for p := range set {
				p.cancel()
			}
		}()
	}
}

// unwatchAsk снимает вопрос с присмотра: итог готов.
func (e *editState) unwatchAsk(p *pend) {
	e.mu.Lock()
	delete(e.asks[p.sess], p)
	e.mu.Unlock()
}

// askRun — что тело вопроса берёт у вызова: запуск, жизнь запуска службы, вопрос и место правки.
type askRun struct {
	r       *run
	runCtx  context.Context
	p       *pend
	req     AskReq
	release func()
	quiet   bool      // зарезервирован тихий слот (AskReq.Always)
	stamp   time.Time // его метка
}

// runAsk — тело вопроса: Host.Ask, итог, счёт отказов, освобождение места правки.
func (s *Service) runAsk(actx context.Context, a askRun) {
	defer a.r.wg.Done()
	defer a.release()
	defer a.p.cancel()
	defer s.ed.unwatchAsk(a.p)
	var res AskRes
	var err error
	func() {
		defer func() {
			if v := recover(); v != nil {
				logPanic("ask", v)
				res, err = AskRes{}, errInternal
			}
		}()
		res, err = s.h.Ask(actx, a.req)
	}()
	if a.quiet && (err != nil || res.Why != WhyQuiet) { // фасад вернул вопрос, а не тишину: слот обратно
		s.ed.returnQuiet(a.stamp)
	}
	s.finishAsk(a, res, err, a.runCtx.Err() != nil)
}

// finishAsk записывает итог и считает отказы. closing — программа закрывается: это не отказ человека.
// Место правки и присмотр снимаются до close(done): иначе агент, получив итог, сразу шлёт следующую
// просьбу и ловит ложное «занято» (release в defer runAsk срабатывал уже после ответа).
func (s *Service) finishAsk(a askRun, res AskRes, err error, closing bool) {
	p := a.p
	why := res.Why
	if err != nil && errors.Is(err, context.Canceled) {
		err, why = nil, WhyGone
	}
	s.ed.mu.Lock()
	off := p.off
	s.ed.mu.Unlock()
	if off && why != WhyAnswered && why != WhyQuiet { // инструмент запретили, пока вопрос висел
		err, why = nil, WhyOff
	}
	if closing && why != WhyAnswered && why != WhyQuiet {
		why = WhyClosed
	}
	var forever bool
	switch why {
	case WhyAnswered:
		s.ed.yes()
	case WhyQuiet: // успех без вопроса: ни yes() (агент обнулял бы ступени тихими вызовами), ни strike()
	case WhyDeclined, WhyTimeout, WhyGone, WhyTooWide, WhyOurs:
		forever = s.ed.strike()
	}
	if err == nil && why != WhyAnswered && why != WhyQuiet {
		err = askWhyErr(why)
	}
	if err == nil { // «да»: запись для отмены ставит только настоящая правка
		s.ed.afterAsk(p, res)
	}
	s.ed.mu.Lock()
	p.res, p.err, p.fin = res, err, s.ed.now()
	s.ed.mu.Unlock()
	s.ed.unwatchAsk(p)
	a.release() // повтор в defer runAsk безвреден (once)
	close(p.done)
	if forever {
		s.changed() // asksMuted сменился
	}
}

// afterAsk — после «да»: add с настоящей папкой — запись отмены {key, rev}; create отметка «не отменяется».
func (e *editState) afterAsk(p *pend, res AskRes) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case p.kind == AskEdit: // правку запишет и отметит сама правка
	case p.kind == AskCreate:
		e.recs[p.sess] = editRec{created: true}
	case !res.Merged && res.Album.Key != "":
		e.recs[p.sess] = editRec{key: res.Album.Key, rev: res.Album.Rev}
	}
}

// askWhyErr — итог вопроса не «да» словами для агента.
func askWhyErr(why string) error {
	switch why {
	case WhyDeclined:
		return errDeclined
	case WhyTimeout:
		return errAskTimeout
	case WhyGone:
		return errAskGone
	case WhyClosed:
		return errClosing
	case WhyOff:
		return errNoEdits
	case WhyTooWide:
		return errFolderTooWide
	case WhyOurs:
		return errFolderOurs
	case WhyDraftLocked:
		return errDraftLocked
	}
	log.Printf("agents: вопрос закончился неизвестным итогом")
	return errInternal
}

// waitAsk ждёт итог вопроса не дольше AskWait. owner — вызов, задавший вопрос: его отмена закрывает
// вопрос (gone); отмена await_answer вопрос не трогает.
func (c *call) waitAsk(p *pend, owner bool) (askOut, error) {
	t := time.NewTimer(c.s.o.AskWait)
	defer t.Stop()
	select {
	case <-p.done:
		return c.askResult(p)
	case <-t.C:
		return askOut{Pending: &pendingOut{Ask: p.token, Text: pendingText}}, nil
	case <-c.ctx.Done():
		if owner {
			p.cancel()
		}
		return askOut{}, errTimeout
	}
}

// askResult отдаёт итог готового вопроса и помечает его отданным.
func (c *call) askResult(p *pend) (askOut, error) {
	s := c.s
	s.ed.mu.Lock()
	p.took = true
	res, err := p.res, p.err
	s.ed.mu.Unlock()
	if err != nil {
		return askOut{}, err
	}
	switch res.Why {
	case WhyQuiet:
		c.verdict = noteQuiet
	case WhyAnswered:
		c.verdict = noteYes
	}
	out := askOut{Album: &res.Album, Folder: &folderRes{N: res.Folder, Merged: res.Merged}, Scan: &scanRes{Active: res.Scanning}}
	return out, nil
}

// awaitAnswer — await_answer: итог вопроса по жетону. Жетон работает только в подключении, задавшем
// вопрос; второе ожидание по тому же жетону сразу получает pending.
func (c *call) awaitAnswer(in awaitIn) (askOut, error) {
	e := c.s.ed
	e.mu.Lock()
	p := e.cur
	switch {
	case p == nil || p.sess != c.sess || subtle.ConstantTimeCompare([]byte(in.Ask), []byte(p.token)) != 1:
		e.mu.Unlock()
		return askOut{}, errAskGone
	case p.took:
		e.mu.Unlock()
		return askOut{}, errAskClosed
	case p.finished() && e.now().Sub(p.fin) > c.s.o.AskLife:
		e.cur = nil // итог пролежал дольше срока
		e.mu.Unlock()
		return askOut{}, errAskGone
	case p.waiting:
		e.mu.Unlock()
		return askOut{Pending: &pendingOut{Ask: p.token, Text: pendingText}}, nil
	}
	p.waiting = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		p.waiting = false
		e.mu.Unlock()
	}()
	return c.waitAsk(p, false)
}

// askClient — имя клиента для текста вопроса: пусто для неизвестного и для похожего на саму
// программу (М2: «Sosnina Albums просит…» от агента — обман).
func askClient(name string) string {
	if Lookalike(name) {
		return ""
	}
	return name
}
