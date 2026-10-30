package sharer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

const uploadWorkers = 3 // страниц одновременно (design §3.3 п. 6)

// runShare — фон выгрузки и итог share:done (error, stopped). Событие — без ссылки и id (С3):
// ссылка содержит id, ключ доступа; окно по событию перечитывает State. Флаг снимается до события,
// чтобы повтор сразу не получил «выгрузка уже идёт».
func (s *Service) runShare(ctx context.Context, cancel context.CancelFunc, run *runState, j *job) {
	defer cancel()
	restore := s.h.LiftMemory()
	var note string
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("выгрузка: паника: %v", r)
				err = fail("Внутренняя ошибка выгрузки — подробности в журнале")
			}
		}()
		note, err = s.execute(ctx, j)
		return err
	}()
	restore()
	removeDir(j.dir)
	switch {
	case errors.Is(err, albums.ErrToken):
		s.tokenLost(j.origin) // записи shares.json целы; ход остановлен как при «Остановить»
	case errors.Is(err, albums.ErrGone):
		s.markGone(j.Path, j.UID, s.own(ctx, run))
	}
	msg, stopped := note, false
	switch {
	case err == nil:
		log.Printf("выгрузка: готово")
	case errors.Is(err, context.Canceled):
		msg, stopped = "", true
		log.Printf("выгрузка: остановлена")
	default:
		msg = errText(err)
	}
	s.end(run)
	s.h.Emit("share:done", doneEvent{Err: msg, Stopped: stopped})
}

// execute — ход выгрузки: пределы сервиса, страницы, манифест, создание или новая редакция,
// загрузка недостающего, публикация, правка срока и пароля, запись. note — предупреждение при
// успехе (ссылка готова, но запись не легла на диск).
func (s *Service) execute(ctx context.Context, j *job) (note string, err error) {
	c, err := s.client(j.origin, j.token)
	if err != nil {
		return "", err
	}
	h, err := c.Health(ctx)
	if err != nil {
		logFail("пределы сервиса", err)
		return "", noService(err)
	}
	lim := h.Limits.OrDefault()
	s.setLimits(j.origin, lim)
	if j.Pages > lim.Pages {
		return "", fail(fmt.Sprintf("В альбоме %d страниц, сервис принимает не больше %d", j.Pages, lim.Pages))
	}
	pages, err := export.RunWeb(ctx, j.Spreads, j.Cover, j.Spec, j.dir, func(d, t int) { s.progress("pages", d, t, 0, 0) })
	if err != nil {
		logFail("страницы", err)
		return "", err
	}
	gen := strings.TrimSpace("Sosnina Albums " + strings.TrimSpace(s.h.Version))
	m, err := share.BuildManifest(ctx, j.dir, pages, j.Cover != nil, j.Spec, j.title, gen, lim)
	if err != nil {
		logFail("манифест", err)
		return "", err
	}
	cur, missing, err := s.open(ctx, c, j, m)
	if err != nil {
		return "", err
	}
	if err := s.upload(ctx, c, j, cur.ID, m, pages, missing); err != nil {
		return "", err
	}
	s.progress("publish", 0, 0, 0, 0)
	// Новый пароль ставится до публикации: упавший Patch после неё оставил бы новую редакцию открытой по старому.
	if cur, err = s.setPassword(ctx, c, j, cur); err != nil {
		return "", err
	}
	pub, err := s.publish(ctx, c, j, cur.ID, m, pages)
	if err != nil {
		return "", err
	}
	if err := s.checkLink(pub.URL); err != nil {
		log.Printf("выгрузка: ссылка из ответа сервиса не прошла проверку")
		return "", fail("Сервис вернул ссылку, которую программа не принимает. Запись о ссылке не сохранена")
	}
	exp, err := s.adjust(ctx, c, j, cur, pub)
	if err != nil {
		return "", err
	}
	rec := cur
	rec.URL, rec.ExpiresAt, rec.PublishedAt = pub.URL, exp, s.now().UTC()
	rec.Scene, rec.Title, rec.Password, rec.Gone = j.Scene, m.Title, j.password, false
	wrote, perr := s.putIf(j.Path, rec, s.own(ctx, j.run))
	switch {
	case !wrote: // путь занят другим проектом или ход остановлен: прежнюю запись не возвращаем
		return "", context.Canceled
	case perr != nil:
		log.Printf("выгрузка: запись о ссылке не сохранена: %v", perr)
		return "Ссылка готова, но записать её в программе не удалось: " + perr.Error() + ". Она показана в окне до выхода из программы — скопируйте её сейчас.", nil
	}
	return "", nil
}

// open создаёт альбом на сервисе или присылает новую редакцию прежнего (ссылка та же). Запись с id
// ложится сразу после Create: обрыв дальше оставляет id, и следующий «Выгрузить» продолжит через
// Update (design §3.3 п. 8). 404 на Update — запись «нет на сервере» и вопрос о новой ссылке.
func (s *Service) open(ctx context.Context, c *albums.Client, j *job, m *albums.Manifest) (prefs.ShareRecord, []int, error) {
	if j.base != nil {
		up, err := c.Update(ctx, j.base.ID, m)
		switch {
		case err == nil:
			b := *j.base
			b.Gone = false // сервис альбом знает: пометка «нет на сервере» снимается
			return b, up.Missing, nil
		case !errors.Is(err, albums.ErrGone):
			logFail("новая редакция", err)
			return prefs.ShareRecord{}, nil, err
		}
		logFail("новая редакция", err)
		s.markGone(j.Path, j.UID, s.own(ctx, j.run))
		if ok, aerr := s.h.Ask(goneQuestion); aerr != nil || !ok {
			return prefs.ShareRecord{}, nil, context.Canceled // «Нет»: остановка без ошибки
		}
		j.base = nil
		if j.days == 0 {
			j.days = 30
		}
	}
	cr, err := c.Create(ctx, &albums.CreateRequest{Manifest: *m, ExpiresAt: s.now().AddDate(0, 0, j.days).UTC(), Password: j.password})
	if err != nil {
		logFail("создание", err)
		return prefs.ShareRecord{}, nil, noService(err)
	}
	rec := prefs.ShareRecord{Origin: j.origin, ID: cr.ID, Title: m.Title, Password: j.password, ExpiresAt: cr.ExpiresAt, UID: j.UID}
	wrote, perr := s.putIf(j.Path, rec, s.own(ctx, j.run))
	if !wrote {
		return prefs.ShareRecord{}, nil, context.Canceled
	}
	if perr != nil {
		log.Printf("выгрузка: запись с id не сохранена: %v", perr)
	}
	return rec, cr.Missing, nil
}

// publish публикует редакцию. Сервис потерял часть страниц (409 pages_missing) — они догружаются
// один раз и публикация повторяется.
func (s *Service) publish(ctx context.Context, c *albums.Client, j *job, id string, m *albums.Manifest, pages []export.WebPage) (*albums.Published, error) {
	pub, err := c.Publish(ctx, id)
	var api *albums.APIError
	if errors.As(err, &api) && api.Code == albums.CodePagesMissing && albums.ValidateMissing(m, api.Missing) == nil && len(api.Missing) > 0 {
		if err := s.upload(ctx, c, j, id, m, pages, api.Missing); err != nil {
			return nil, err
		}
		pub, err = c.Publish(ctx, id)
	}
	if err != nil {
		logFail("публикация", err)
		return nil, err
	}
	return pub, nil
}

// setPassword меняет пароль прежней ссылки (Patch с revoke_sessions) до публикации новой редакции;
// запись получает новый пароль сразу: обрыв дальше оставит её равной тому, что у сервиса. Пароль не
// менялся (и новая ссылка, где пароль ушёл в Create) — ничего не делает.
func (s *Service) setPassword(ctx context.Context, c *albums.Client, j *job, cur prefs.ShareRecord) (prefs.ShareRecord, error) {
	if j.password == cur.Password {
		return cur, nil
	}
	pw := j.password
	if _, err := c.Patch(ctx, cur.ID, &albums.Patch{Password: &pw, RevokeSessions: true}); err != nil {
		logFail("пароль", err)
		return cur, err
	}
	cur.Password = j.password
	wrote, perr := s.putIf(j.Path, cur, s.own(ctx, j.run))
	if !wrote {
		return cur, context.Canceled
	}
	if perr != nil {
		log.Printf("выгрузка: запись с новым паролем не сохранена: %v", perr)
	}
	return cur, nil
}

// adjust правит срок, если он отличается от того, что у сервиса (Patch; пароль — setPassword до
// публикации). Возвращает итоговый срок.
func (s *Service) adjust(ctx context.Context, c *albums.Client, j *job, cur prefs.ShareRecord, pub *albums.Published) (time.Time, error) {
	if j.base == nil || j.days <= 0 {
		return pub.ExpiresAt, nil
	}
	t := s.now().AddDate(0, 0, j.days).UTC()
	al, err := c.Patch(ctx, cur.ID, &albums.Patch{ExpiresAt: &t})
	if err != nil {
		logFail("срок", err)
		return time.Time{}, err
	}
	return al.ExpiresAt, nil
}

// markGone помечает запись пути «нет на сервере» (404 на запрос по id альбома); запись другого
// проекта (отпечаток не uid) не трогается. alive — сверка
// putIf; nil — без неё.
func (s *Service) markGone(path, uid string, alive func() bool) {
	rec, ok, _ := s.get(path, uid)
	if !ok || rec.Gone {
		return
	}
	rec.Gone = true
	if _, err := s.putIf(path, rec, alive); err != nil {
		log.Printf("выгрузка: пометка «нет на сервере» не сохранена: %v", err)
	}
}

// upload грузит страницы missing в uploadWorkers потоков с повторами; первая ошибка останавливает остальные.
func (s *Service) upload(ctx context.Context, c *albums.Client, j *job, id string, m *albums.Manifest, pages []export.WebPage, missing []int) error {
	if len(missing) == 0 {
		return nil
	}
	if albums.ValidateMissing(m, missing) != nil { // не доверяем списку сервиса: индекс по страницам только после сверки
		return albums.ErrResponse
	}
	var total int64
	for _, n := range missing {
		total += m.Pages[n-1].Bytes
	}
	pr := &progress{s: s, total: len(missing), totalBytes: total}
	pr.emit()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)
	queue := make(chan int)
	for range uploadWorkers {
		wg.Go(func() {
			for n := range queue {
				p := m.Pages[n-1]
				if err := s.putPage(ctx, c, id, p, filepath.Join(j.dir, pages[n-1].Name)); err != nil {
					once.Do(func() { first = err; cancel() })
					continue
				}
				pr.add(p.Bytes)
			}
		})
	}
feed:
	for _, n := range missing {
		select {
		case queue <- n:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	pr.flush() // последнее значение всегда уходит: при остановке или сбое окно не застревает на старом счёте
	if first != nil {
		return first
	}
	return ctx.Err()
}

// putPage грузит одну страницу; сеть, 5xx и 429 повторяются (retryDelay), остальное — сразу ошибка.
func (s *Service) putPage(ctx context.Context, c *albums.Client, id string, p albums.Page, path string) error {
	for attempt := 0; ; attempt++ {
		err := putOnce(ctx, c, id, p, path)
		if err == nil {
			return nil
		}
		d, again := retryDelay(err, attempt)
		if !again {
			logFail(fmt.Sprintf("страница %d", p.N), err)
			return err
		}
		logFail(fmt.Sprintf("страница %d, повтор %d", p.N, attempt+1), err)
		if werr := s.wait(ctx, d); werr != nil {
			return werr
		}
	}
}

func putOnce(ctx context.Context, c *albums.Client, id string, p albums.Page, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fail("Файл страницы пропал во временной папке")
	}
	defer func() { _ = f.Close() }()
	return c.PutPage(ctx, id, p.N, p.Type, f, p.Bytes)
}

// stopper — отложенная отправка; в тестах подменяется (Service.after).
type stopper interface{ Stop() bool }

func realAfter(d time.Duration, f func()) stopper { return time.AfterFunc(d, f) }

// progress — ход загрузки: не чаще одного события за gap; отброшенное из-за gap значение уходит
// по заднему фронту (таймер), последнее — всегда (flush).
type progress struct {
	s           *Service
	mu          sync.Mutex
	done, total int
	bytes       int64
	totalBytes  int64
	last        time.Time
	sent        int     // done в последнем отправленном событии
	timer       stopper // отложенная отправка; nil — не взведён
	seq         int     // поколение таймера: опоздавший fire погашенного таймера выходит
	closed      bool    // flush был: таймер больше не стреляет
}

func (p *progress) add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	p.bytes += n
	since := p.s.now().Sub(p.last)
	switch {
	case p.done == p.total || since >= p.s.gap:
		p.emit()
	case p.timer == nil && !p.closed:
		// Пачка ответов пришла разом: без таймера следующее событие было бы лишь по следующему ответу.
		p.seq++
		seq := p.seq
		p.timer = p.s.after(p.s.gap-since, func() { p.fire(seq) })
	}
	if p.s.acked != nil {
		p.s.acked(p.done)
	}
}

// disarm гасит таймер; fire уже погашенного (ждёт mu, Stop не успел) увидит новое поколение.
func (p *progress) disarm() {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	p.seq++
}

// fire — задний фронт: шлёт значение, отброшенное из-за gap. После flush молчит (closed под mu);
// таймер старого поколения (его погасил emit, а новый уже взведён) ничего не трогает.
func (p *progress) fire(seq int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if seq != p.seq {
		return
	}
	p.timer = nil
	if !p.closed && p.done != p.sent {
		p.emit()
	}
}

// flush шлёт итоговое значение, если оно не ушло из-за паузы gap, и гасит таймер; вызывают после
// остановки потоков, до share:done.
func (p *progress) flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.disarm()
	if p.done != p.sent {
		p.emit()
	}
}

// emit шлёт событие и гасит таймер; вызывающий держит mu (кроме первого вызова, пока других потоков нет).
func (p *progress) emit() {
	p.disarm()
	p.last, p.sent = p.s.now(), p.done
	p.s.progress("upload", p.done, p.total, p.bytes, p.totalBytes)
}

// progressEvent — данные share:progress одним объектом (0.32-146-T4-3); без ссылки и id.
type progressEvent struct {
	Phase      string `json:"phase"` // pages | upload | publish
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// doneEvent — данные share:done одним объектом: без url и id (С3); Err пуст при успехе и остановке.
type doneEvent struct {
	Err     string `json:"err"`
	Stopped bool   `json:"stopped"`
}

func (s *Service) progress(phase string, done, total int, bytes, totalBytes int64) {
	s.h.Emit("share:progress", progressEvent{phase, done, total, bytes, totalBytes})
}
