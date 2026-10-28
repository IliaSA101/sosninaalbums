package libpacks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Задание «докачать листы образцов» (спека 144, design §4, security Н3, Н4, Н12, требования 3–5):
// отдельно от «Скачать» наборов — своя блокировка, своя отмена, свои паузы. Замок наборов (s.mu)
// задание не берёт: идущее «Скачать» и образцы друг другу не мешают.

const (
	sheetTimeout = time.Minute      // предел одного листа (килобайты); пол Download — 15 мин пакета
	prevPause    = 10 * time.Minute // пауза после неудачи связи или диска
	prevBatch    = 50               // листов за одно задание
)

// PreviewsReady — данные события library:previews: сколько листов появилось за задание.
type PreviewsReady struct {
	Ready int `json:"ready"`
}

// prevState — память задания: пауза, плохие листы сеанса, выключенная папка, счётчик журнала.
type prevState struct {
	mu      sync.Mutex
	retryAt time.Time
	bad     map[string]bool // sha листа: 404 или не тот размер/sha — до конца сеанса не запрашивается
	off     atomic.Bool     // папка листов не обычная папка: образцы выключены на сеанс
	logN    atomic.Int64
}

func (p *prevState) paused(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return now.Before(p.retryAt)
}

func (p *prevState) pause(until time.Time) {
	p.mu.Lock()
	p.retryAt = until
	p.mu.Unlock()
}

func (p *prevState) isBad(sha string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bad[sha]
}

func (p *prevState) markBad(sha string) {
	p.mu.Lock()
	if p.bad == nil {
		p.bad = map[string]bool{}
	}
	p.bad[sha] = true
	p.mu.Unlock()
}

// prevBegin — контекст задания от StopCtx, отмена записана в prevCancel под netMu и согласие
// перечитано уже после записи (тот же порядок, что beginNet: отзыв между первой проверкой и
// записью отмены иначе не остановил бы задание). Без согласия — ok == false, ни одного запроса.
func (s *Service) prevBegin() (ctx context.Context, done func(), ok bool) {
	s.h.BeforeBegin()
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	s.netMu.Lock()
	s.prevCancel = cancel
	s.netMu.Unlock()
	done = func() {
		s.netMu.Lock()
		s.prevCancel = nil
		s.netMu.Unlock()
		cancel()
	}
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		done()
		return nil, nil, false
	}
	return ctx, done, true
}

// stopPreviews — отзыв согласия обрывает задание образцов. «Отменить» окна «Наборы» (Cancel) его
// не трогает: у образцов своя отмена.
func (s *Service) stopPreviews() {
	s.netMu.Lock()
	if s.prevCancel != nil {
		s.prevCancel()
	}
	s.netMu.Unlock()
}

// previewWanted — строки каталога, чьи листы нужны плиткам: набора нет в библиотеке, у строки есть
// лист, ни она, ни донор вверх не «удалены вами», программа подходит всему составу «Скачать»
// (closure). Без дополнений-серых: их плитки не показываются. Только чтение каталога.
func previewWanted(cat fetch.Catalog, have map[string]decor.PackInfo, skip []string, app string) []fetch.Entry {
	ix := indexOf(cat)
	var out []fetch.Entry
	for _, e := range cat.Packs {
		if e.Preview == nil || slices.Contains(skip, e.ID) {
			continue
		}
		if _, stands := have[e.ID]; stands {
			continue
		}
		if e.Needs != "" { // дополнение без донора: ни в библиотеке, ни в каталоге — closure обрывает подъём молча
			_, stands := have[e.Needs]
			if _, inCat := ix.byID[e.Needs]; !stands && !inCat {
				continue
			}
		}
		c := ix.closure(cat, e.ID, have, skip, app)
		ok := !e.NeedsApp(app)
		for _, m := range c.ents {
			ok = ok && !m.NeedsApp(app)
		}
		for _, up := range c.up {
			ok = ok && !slices.Contains(skip, up)
		}
		if ok {
			out = append(out, e)
		}
	}
	return out
}

// FetchPreviews — докачивает листы образцов для строк, которые дали бы плитки (previewWanted), пока
// их нет на диске: по одному, не больше prevBatch за задание. Вызывает окно при открытии через
// Host.GoBG. Условия: папка настроек, согласие, сохранённый список, библиотека не «только чтение»,
// пауза прошла; второе задание параллельно — молча выход. Итог — сколько листов появилось (событие
// library:previews при ненулевом). Неудача связи или диска — пауза prevPause; 404 и «не тот
// размер/sha» — лист в плохих до конца сеанса, ход дальше; отзыв или выход — конец без паузы.
func (s *Service) FetchPreviews() (ready int) {
	defer appkit.Guard("libraryPreviews")
	if !s.prevMu.TryLock() {
		return 0
	}
	defer s.prevMu.Unlock()
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed || s.prev.paused(s.h.Now()) {
		return 0
	}
	cat, _, _ := s.cached()
	st, err := s.h.Store()
	if cat == nil || err != nil || st.ReadOnly() {
		return 0
	}
	// Остатки .dl-*.tmp после аварии убираются до выхода «качать нечего» (под prevMu).
	if dir := s.previewDir(); dir != "" {
		cleanTmp(dir)
	}
	rows := previewWanted(*cat, haveOf(st), prefs.LibraryNetOf(s.h.CfgDir()).Skip, s.h.Version())
	if len(rows) == 0 {
		return 0
	}
	dir := s.ensureDir()
	if dir == "" {
		return 0
	}
	todo := s.missingSheets(rows)
	if len(todo) == 0 {
		return 0
	}
	ctx, done, ok := s.prevBegin()
	if !ok {
		return 0
	}
	defer done()
	c := s.h.Fetcher()
	if c == nil {
		return 0
	}
	defer func() {
		if ready > 0 {
			s.h.Emit("library:previews", PreviewsReady{Ready: ready})
		}
	}()
	return s.fetchSheets(ctx, c, dir, todo)
}

// fetchSheets — листы по одному; итог — сколько скачано. Паузы и плохие листы — по классу ошибки.
func (s *Service) fetchSheets(ctx context.Context, c *fetch.Client, dir string, todo []fetch.Entry) (ready int) {
	broken := 0 // ErrBroken подряд: обрыв тела и страница прокси с 200 выглядят как битый лист
	for _, e := range todo {
		// Перед каждым листом: отзыв согласия мог прийти между ними.
		if ctx.Err() != nil || !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
			return ready
		}
		err := s.fetchSheet(ctx, c, dir, e.ID, e.Preview)
		switch {
		case err == nil:
			broken = 0
			ready++
		case errors.Is(err, fetch.ErrCancelled):
			return ready
		case errors.Is(err, errBadRow):
			s.prev.markBad(e.Preview.SHA256) // строка каталога негодна: связь ни при чём
			s.prevLog("%s: %s", e.ID, prevWord(err))
		case errors.Is(err, fetch.ErrBroken):
			if broken++; broken >= 2 {
				// Второй подряд: это связь, не лист. Второй в bad не кладём, пауза и выход (144 Н4).
				s.prevLog("%s: %s", e.ID, prevWord(err))
				s.prev.pause(s.h.Now().Add(prevPause))
				return ready
			}
			s.prev.markBad(e.Preview.SHA256)
			s.prevLog("%s: %s", e.ID, prevWord(err))
		case errors.Is(err, fetch.ErrNotFound):
			broken = 0
			s.prev.markBad(e.Preview.SHA256)
			s.prevLog("%s: %s", e.ID, prevWord(err))
		default:
			s.prevLog("%s: %s", e.ID, prevWord(err))
			s.prev.pause(s.h.Now().Add(prevPause))
			return ready
		}
	}
	return ready
}

// errBadRow — в строке каталога размер или sha листа негодны: качать нечего, лист в плохих.
var errBadRow = errors.New("строка листа негодна")

// prevWord — слова журнала по классу ошибки: тексты fetch (названия наборов, GitHub) в log.txt не идут.
func prevWord(err error) string {
	var se *fetch.StatusError
	switch {
	case errors.Is(err, errBadRow), errors.Is(err, fetch.ErrBroken):
		return "лист не сошёлся с каталогом"
	case errors.Is(err, fetch.ErrNotFound):
		return "лист не найден на сервере"
	case errors.Is(err, fetch.ErrDisk):
		return "лист не записан на диск"
	case errors.As(err, &se):
		return "ответ " + strconv.Itoa(se.Code)
	}
	return "нет связи"
}

// missingSheets — строки с листом, которого нет на диске и который не в плохих; один sha — один
// лист (общий лист двух наборов качается раз); не больше prevBatch.
func (s *Service) missingSheets(rows []fetch.Entry) []fetch.Entry {
	var out []fetch.Entry
	seen := map[string]bool{}
	for _, e := range rows {
		p := e.Preview
		if seen[p.SHA256] || s.prev.isBad(p.SHA256) || s.sheetPresent(p) {
			continue
		}
		seen[p.SHA256] = true
		if out = append(out, e); len(out) == prevBatch {
			break
		}
	}
	return out
}

// fetchSheet — один лист: предел размера и sha до вызова, временный файл (дескриптор закрыт до
// Client.Download), срок sheetTimeout, Rename в <sha>.jpg; при любой ошибке временный удаляется.
// Образец — guidevideo.fetchOne. Журнал пишет вызывающий: тут ни адресов, ни путей.
func (s *Service) fetchSheet(ctx context.Context, c *fetch.Client, dir, id string, p *fetch.Preview) error {
	if p == nil || p.Size < 1 || p.Size > fetch.MaxPreview || !validSHA(p.SHA256) {
		return errBadRow
	}
	final := filepath.Join(dir, p.SHA256+previewExt)
	_ = os.Remove(final) // лист не тот по размеру или не обычный файл: заменяем (папка непустая — Rename скажет)
	f, err := os.CreateTemp(dir, tmpPattern)
	if err != nil {
		return fetch.ErrDisk
	}
	tmp := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fetch.ErrDisk
	}
	defer func() { _ = os.Remove(tmp) }() // после удачного Rename файла уже нет
	cctx, cancel := context.WithTimeout(ctx, sheetTimeout)
	defer cancel()
	e := fetch.Entry{Title: id, Size: p.Size, SHA256: p.SHA256, URL: p.URL}
	if err := c.Download(cctx, e, tmp, nil); err != nil {
		return err
	}
	if err := s.h.Rename(tmp, final); err != nil {
		return fetch.ErrDisk
	}
	return nil
}
