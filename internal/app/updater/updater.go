// Package updater — служба обновления программы (спека 111, разрез 126 Ш1): состояние для блока
// «О программе», проверка подписанного манифеста выпуска, скачивание и запуск установщика
// (internal/update), события update:* и расписание проверки при запуске. Сеть — только через
// update. Правка проекта не участвует.
//
// Зовёт её internal/app (update.go — фасад с методами для интерфейса). Wails и App службе
// недоступны: события, диалоги, выход и фон приходят через Host (поля-функции, замыкания над App —
// позднее связывание, подмены тестов действуют и после NewApp).
package updater

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Фазы Info.Phase.
const (
	phaseIdle        = "idle"
	phaseChecking    = "checking"
	phaseLatest      = "latest"
	phaseAvailable   = "available"
	phaseDownloading = "downloading"
	phaseReady       = "ready"
	phaseError       = "error"
)

// FailWindow — сколько после запуска установщика итог считается «не установилось»: позже — тихо
// (человек мог запустить программу много времени спустя).
const FailWindow = time.Hour

// progressGap — update:progress не чаще 4 раз в секунду (свой, как у library:progress в app: служба
// app не импортирует).
const progressGap = 250 * time.Millisecond

// Info — состояние обновления для блока «О программе» и тостов. Тексты собирает интерфейс
// по phase и mode; Go отдаёт слова только в error. App объявляет его определённым типом
// (`type UpdateInfo updater.Info`), поэтому в models.ts он остаётся типом app.
type Info struct {
	Phase   string   `json:"phase"`   // idle | checking | latest | available | downloading | ready | error
	Mode    string   `json:"mode"`    // install (установленная Windows) | setup (переносная Windows) | page (Mac, Linux, minAuto)
	Version string   `json:"version"` // предлагаемая; "" — нет
	Notes   []string `json:"notes"`   // «Что нового» из манифеста, ≤ 8 строк, очищены
	Done    int64    `json:"done"`
	Total   int64    `json:"total"`
	Error   string   `json:"error"`   // словами, без адресов и путей
	ErrorOf string   `json:"errorOf"` // что не удалось при Phase == error: check | download | install; иначе ""
	AtStart bool     `json:"atStart"` // флажок «Проверять при запуске»
	Checked int64    `json:"checked"` // мс, последняя удачная проверка; 0 — не было
	Just    string   `json:"just"`    // версия, до которой только что обновились (один раз)
	Failed  string   `json:"failed"`  // версия, которая не поставилась (один раз)
}

// Progress — данные события update:progress.
type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// Значения Info.ErrorOf: какая операция закончилась ошибкой. Интерфейс по ним выбирает «Повторить»,
// а не гадает по наличию предложенной версии.
const (
	ErrOfCheck    = "check"
	ErrOfDownload = "download"
	ErrOfInstall  = "install"
)

// Ошибки для интерфейса (тексты — как были).
var (
	ErrWindows  = errors.New("установщик запускается только в Windows")
	ErrNotReady = errors.New("обновление ещё не скачано")
	ErrStart    = errors.New("не удалось запустить установщик — скачайте его со страницы выпуска")
)

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем
// пустого). App отдаёт настоящие значения; тест пересобирает службу с подменой до первого вызова.
type Host struct {
	CfgDir  func() string                  // папка config.json
	Version func() string                  // версия программы (wails.json); "dev" — сравнить нечем
	Emit    func(name string, data ...any) // событие окну
	GoBG    func(fn func()) bool           // фон под WaitGroup App; false — App закрывается
	StopCtx func() context.Context         // контекст выхода программы
	Quit    func()                         // закрыть программу (shutdown сохранит альбом)
	OpenURL func(url string)               // адрес в браузере
	// StartTimes — задержка и период проверки при запуске; общие с проверкой библиотеки (app).
	StartTimes func() (delay, every time.Duration)
	Client     func() (c *update.Client, auto bool) // клиент и разрешена ли проверка при запуске (в dev без стенда — нет)
	GOOS       func() string
	Exe        func() (string, error)            // путь запущенного exe
	Dir        func() string                     // папка скачанного (update.Dir); "" — кэша нет
	Free       func(dir string) (uint64, error)  // свободное место на томе
	Start      func(setup, cmdline string) error // запуск установщика
}

// Service — состояние обновления, под mu.
type Service struct {
	h       Host
	mu      sync.Mutex
	phase   string
	offer   *update.Offer
	path    string // проверенный установщик в фазе ready
	done    int64
	total   int64
	err     string
	errOf   string // операция, давшая err (ErrOf*)
	just    string
	failed  string
	cancel  context.CancelFunc // идущая проверка или скачивание
	busy    bool
	started atomic.Bool // проверка при запуске — раз за сеанс
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и colorpick.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("updater.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h}
}

func (s *Service) mode(o *update.Offer) update.Mode {
	if o != nil && o.Page {
		return update.ModePage
	}
	return update.ModeOf(s.h.GOOS(), s.exe())
}

// exe — путь запущенного exe ("" — не определился).
func (s *Service) exe() string {
	exe, _ := s.h.Exe()
	return exe
}

// snap — текущее состояние без Just/Failed (их отдаёт только Info, один раз).
func (s *Service) snap() Info {
	u := prefs.UpdateOf(s.h.CfgDir())
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapLocked(u)
}

func (s *Service) snapLocked(u prefs.Update) Info {
	info := Info{Phase: s.phase, Notes: []string{}, Done: s.done, Total: s.total, Error: s.err, ErrorOf: s.errOf,
		AtStart: !u.Off, Checked: u.Checked}
	if info.Phase == "" {
		info.Phase = phaseIdle
	}
	if o := s.offer; o != nil {
		info.Version, info.Notes = o.Version, append(info.Notes, o.Notes...)
	}
	info.Mode = string(s.mode(s.offer))
	return info
}

// emitState — update:state со снимком (блок «О программе» перерисовывается без опроса).
func (s *Service) emitState() { s.h.Emit("update:state", s.snap()) }

// Info — текущее состояние без сети. Just и Failed отдаются один раз: интерфейс зовёт
// при запуске и показывает тост.
func (s *Service) Info() (info Info) {
	defer appkit.Guard("UpdateInfo")
	info = s.snap()
	s.mu.Lock()
	info.Just, info.Failed = s.just, s.failed
	s.just, s.failed = "", ""
	s.mu.Unlock()
	return info
}

// Startup — итог прошлого запуска установщика (startup, до первого Info): версия
// в Pending уже стоит (или новее) — Just; иначе недавно — Failed; старше FailWindow — молча.
// Pending снимается в любом случае.
func (s *Service) Startup() {
	cfg := s.h.CfgDir()
	u := prefs.UpdateOf(cfg)
	if u.Pending == "" {
		return
	}
	prefs.UpdateUpdate(cfg, func(n *prefs.Update) { n.Pending = "" })
	var just, failed string
	cur := s.h.Version()
	_, curOK := update.ParseVersion(cur)
	switch ago := time.Since(time.UnixMilli(u.PendingAt)); {
	case !curOK: // dev: сравнить нечем, итог молчит
	case !update.Newer(u.Pending, cur):
		just = u.Pending
	case ago >= 0 && ago < FailWindow:
		failed = u.Pending
	}
	log.Printf("обновление: итог запуска установщика %s — обновлено %q, не установилось %q", u.Pending, just, failed)
	s.mu.Lock()
	s.just, s.failed = just, failed
	s.mu.Unlock()
}

// begin — занять операцию (проверка, скачивание); false — другая уже идёт. Контекст — от
// выхода программы, Cancel его отменяет. Пустая phase — тихая проверка при запуске: она
// не меняет фазу, поэтому и не идёт, если фаза уже есть (человек успел нажать «Проверить» —
// её итог, вплоть до ready, тихая проверка не затрёт).
func (s *Service) begin(phase string) (context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy || phase == "" && s.phase != "" {
		return nil, false
	}
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	s.busy, s.cancel = true, cancel
	if phase != "" {
		s.phase, s.err, s.errOf = phase, "", ""
	}
	return ctx, true
}

// end — операция закончена.
func (s *Service) end() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.busy, s.cancel = false, nil
	s.mu.Unlock()
}

// Check — «Проверить обновления»: всегда в сеть (флажок не мешает — это действие
// человека); итог — в phase: latest, available или error. Идёт другая операция — её состояние.
func (s *Service) Check() (info Info, err error) {
	defer appkit.GuardErr("CheckUpdate", &err)
	s.check(false)
	return s.snap(), nil
}

// CheckSilent — проверка при запуске: фазу до результата не меняет, ошибки пишет только в log.txt.
// Найденное новее — update:offer.
func (s *Service) CheckSilent() { s.check(true) }

// check — проверка; silent (при запуске) не меняет фазу до результата и ошибки пишет
// только в log.txt. Найденное новее при silent — update:offer.
func (s *Service) check(silent bool) {
	phase := phaseChecking
	if silent {
		phase = ""
	}
	ctx, ok := s.begin(phase)
	if !ok {
		return
	}
	defer s.end()
	if !silent {
		s.emitState()
	}
	c, _ := s.h.Client()
	cfg := s.h.CfgDir()
	seen := prefs.UpdateOf(cfg).Seen
	if c.Stand() {
		seen = "" // версии ленты стенда не сверяются с настоящими выпусками
	}
	res, err := update.Check(ctx, c, s.h.Version(), seen)
	if err != nil {
		log.Printf("обновление: проверка не удалась: %s", update.Detail(err))
		if !silent {
			s.fail(ErrOfCheck, err)
		}
		return
	}
	if c.Stand() { // ленту стенда QA в config.json не пишем: Seen и Checked настоящих выпусков остаются
		log.Printf("обновление: источник — стенд, Seen и Checked не сохраняются")
	} else {
		now := time.Now().UnixMilli()
		prefs.UpdateUpdate(cfg, func(u *prefs.Update) { u.Checked, u.Seen = now, res.Seen })
	}
	if res.Rollback {
		log.Printf("обновление: подписанный файл старее виденной версии — отклонён")
	}
	s.mu.Lock()
	s.offer, s.phase, s.err, s.errOf = res.Offer, phaseLatest, "", ""
	if res.Offer != nil {
		s.phase = phaseAvailable
	}
	s.mu.Unlock()
	if res.Offer != nil {
		log.Printf("обновление: найдена версия %s", res.Offer.Version)
	}
	s.emitState()
	if silent && res.Offer != nil {
		s.h.Emit("update:offer", s.snap())
	}
}

// fail — фаза error с текстом ошибки операции of (ErrOf*); отмена — не ошибка (фаза возвращается к предложению).
func (s *Service) fail(of string, err error) {
	s.mu.Lock()
	if errors.Is(err, update.ErrCancelled) {
		s.phase = phaseAvailable
		if s.offer == nil {
			s.phase = phaseIdle
		}
	} else {
		s.phase, s.err, s.errOf = phaseError, err.Error(), of
	}
	s.mu.Unlock()
	s.emitState()
}

// Download — скачать и проверить предложенное: ход — update:progress; уже скачанное и
// сверенное — сразу ready. Только для install и setup; для page и без предложения —
// ничего не делает, отдаёт состояние.
func (s *Service) Download() (info Info, err error) {
	defer appkit.GuardErr("DownloadUpdate", &err)
	s.mu.Lock()
	o := s.offer
	busy := s.busy
	s.mu.Unlock()
	// Нет предложения или ставят вручную — не отказ: «Повторить» по кругу не нужен, снимок сам
	// скажет интерфейсу mode=page («Скачать» со страницы) или фазу без предложения («Проверить»).
	if busy || o == nil || o.Setup == nil || s.mode(o) == update.ModePage {
		return s.snap(), nil
	}
	ctx, ok := s.begin(phaseDownloading)
	if !ok {
		return s.snap(), nil
	}
	defer s.end()
	s.mu.Lock()
	s.done, s.total = 0, o.Setup.Size
	s.mu.Unlock()
	s.emitState()
	c, _ := s.h.Client()
	path, err := update.Fetch(ctx, c, *o, s.h.Dir(), s.h.Free, s.progress(o.Setup.Size))
	if err != nil {
		log.Printf("обновление: скачивание не удалось: %s", update.Detail(err))
		s.fail(ErrOfDownload, err)
		return s.snap(), nil
	}
	s.mu.Lock()
	s.phase, s.path, s.done = phaseReady, path, o.Setup.Size
	s.mu.Unlock()
	log.Printf("обновление: версия %s скачана и сверена", o.Version)
	s.emitState()
	return s.snap(), nil
}

// progress — колбэк хода: update:progress не чаще progressGap, конец — сразу.
func (s *Service) progress(total int64) func(done int64) {
	var last time.Time
	return func(done int64) {
		s.mu.Lock()
		s.done = done
		s.mu.Unlock()
		if now := time.Now(); done >= total || now.Sub(last) >= progressGap {
			last = now
			s.h.Emit("update:progress", Progress{Done: done, Total: total})
		}
	}
}

// Cancel — «Отменить»: идущее скачивание (или проверка) обрывается, .part удаляется.
func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// Install — «Перезапустить и обновить»: только ready и Windows. Сверка sha256 заново (файл
// мог измениться на диске) → pending в config.json → запуск установщика → выход. Установленная
// копия: тихо, на месте, с перезапуском; переносная — обычный мастер.
func (s *Service) Install() (err error) {
	defer appkit.GuardErr("InstallUpdate", &err)
	s.mu.Lock()
	o, path, phase := s.offer, s.path, s.phase
	s.mu.Unlock()
	switch {
	case s.h.GOOS() != "windows":
		return ErrWindows
	case phase != phaseReady || o == nil || o.Setup == nil || path == "":
		return ErrNotReady
	}
	if err := update.Verify(path, *o.Setup); err != nil {
		log.Printf("обновление: файл не прошёл повторную проверку: %s", update.Detail(err))
		s.mu.Lock()
		s.phase, s.err, s.errOf, s.path = phaseError, err.Error(), ErrOfInstall, ""
		s.mu.Unlock()
		s.emitState()
		return err
	}
	mode := s.mode(o)
	cmdline, err := update.CmdLine(path, mode, filepath.Dir(s.exe()))
	if err != nil {
		log.Printf("обновление: командная строка установщика не собрана: %v", err) // ErrPath — без пути
		return ErrStart
	}
	cfg := s.h.CfgDir()
	now := time.Now().UnixMilli()
	prefs.UpdateUpdate(cfg, func(u *prefs.Update) { u.Pending, u.PendingAt = o.Version, now })
	if err := s.h.Start(path, cmdline); err != nil {
		log.Printf("обновление: установщик не запустился: %v", paths.NoPath(err)) // путь и имя пользователя — не в лог
		prefs.UpdateUpdate(cfg, func(u *prefs.Update) { u.Pending, u.PendingAt = "", 0 })
		return ErrStart
	}
	log.Printf("обновление: запущен установщик версии %s (%s)", o.Version, mode)
	s.h.Quit() // shutdown сохранит альбом; установщик ждёт выхода программы
	return nil
}

// OpenPage — страница выпуска в браузере: проверенной версии, а без предложения — последнего
// выпуска. Адрес строится здесь; от интерфейса его нет.
func (s *Service) OpenPage() (err error) {
	defer appkit.GuardErr("OpenUpdatePage", &err)
	u := appkit.RepoURL + "/releases/latest"
	s.mu.Lock()
	if o := s.offer; o != nil && update.SetupName(o.Version) != "" {
		u = appkit.RepoURL + "/releases/tag/v" + o.Version
	}
	s.mu.Unlock()
	s.h.OpenURL(u)
	return nil
}

// SetCheck — флажок «Проверять при запуске»; снятие отменяет ещё не начатую проверку.
func (s *Service) SetCheck(atStart bool) (info Info) {
	defer appkit.Guard("SetUpdateCheck")
	prefs.UpdateUpdate(s.h.CfgDir(), func(u *prefs.Update) { u.Off = !atStart })
	return s.snap()
}

// Schedule — при запуске (domReady): чистка папки скачанного и проверка обновления.
// Проверка — если не выключена, источник разрешён (в dev — только стенд), прошло ≥ every
// (часы ушли назад — тоже пора), раз за сеанс; через delay — окно уже открыто. Флажок
// перечитывается после задержки: снятие за это время отменяет проверку.
func (s *Service) Schedule() {
	update.Clean(s.h.Dir(), s.h.Version())
	_, auto := s.h.Client()
	_, every := s.h.StartTimes()
	u := prefs.UpdateOf(s.h.CfgDir())
	since := time.Now().UnixMilli() - u.Checked
	if u.Off || !auto || since >= 0 && since < every.Milliseconds() {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	if !s.h.GoBG(func() {
		defer appkit.Guard("updateCheck")
		delay, _ := s.h.StartTimes()
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-s.h.StopCtx().Done():
			return
		}
		if !prefs.UpdateOf(s.h.CfgDir()).Off {
			s.check(true)
		}
	}) {
		s.started.Store(false) // фон закрыт: проверка не запущена, раз за сеанс не потрачен
	}
}
