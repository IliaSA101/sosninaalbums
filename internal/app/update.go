package app

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Обновление программы (спека 111, design §3–4): проверка подписанного манифеста выпуска,
// скачивание и запуск установщика — internal/update; здесь состояние для «О программы», события
// и расписание. Сеть — только decor/fetch через update. Правка проекта не участвует.

// Фазы UpdateInfo.Phase.
const (
	updIdle        = "idle"
	updChecking    = "checking"
	updLatest      = "latest"
	updAvailable   = "available"
	updDownloading = "downloading"
	updReady       = "ready"
	updError       = "error"
)

// Сколько после запуска установщика итог считается «не установилось»: позже — тихо (человек
// мог запустить программу много времени спустя).
const updFailWindow = time.Hour

// UpdateInfo — состояние обновления для блока «О программе» и тостов. Тексты собирает интерфейс
// по phase и mode; Go отдаёт слова только в error.
type UpdateInfo struct {
	Phase   string   `json:"phase"`   // idle | checking | latest | available | downloading | ready | error
	Mode    string   `json:"mode"`    // install (установленная Windows) | setup (переносная Windows) | page (Mac, Linux, minAuto)
	Version string   `json:"version"` // предлагаемая; "" — нет
	Notes   []string `json:"notes"`   // «Что нового» из манифеста, ≤ 8 строк, очищены
	Done    int64    `json:"done"`
	Total   int64    `json:"total"`
	Error   string   `json:"error"`   // словами, без адресов и путей
	AtStart bool     `json:"atStart"` // флажок «Проверять при запуске»
	Checked int64    `json:"checked"` // мс, последняя удачная проверка; 0 — не было
	Just    string   `json:"just"`    // версия, до которой только что обновились (один раз)
	Failed  string   `json:"failed"`  // версия, которая не поставилась (один раз)
}

// updProgressEvent — данные события update:progress.
type updProgressEvent struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

var (
	errUpdWindows = errors.New("установщик запускается только в Windows")
	errUpdNotRdy  = errors.New("обновление ещё не скачано")
	errUpdStart   = errors.New("не удалось запустить установщик — скачайте его со страницы выпуска")
)

// updState — состояние обновления, под mu. Подменяемое (client, version, goos, …) — крючки тестов.
type updState struct {
	mu      sync.Mutex
	phase   string
	offer   *update.Offer
	path    string // проверенный установщик в фазе ready
	done    int64
	total   int64
	err     string
	just    string
	failed  string
	cancel  context.CancelFunc // идущая проверка или скачивание
	busy    bool
	started atomic.Bool // проверка при запуске — раз за сеанс

	client  func() (c *update.Client, auto bool) // nil — update.Default
	version string                               // "" — из wails.json
	goos    string                               // "" — runtime.GOOS
	exe     func() (string, error)               // nil — os.Executable
	dir     string                               // "" — update.Dir
	free    func(dir string) (uint64, error)     // nil — osx.FreeBytes
	start   func(setup, cmdline string) error    // nil — update.Start
	quit    func()                               // nil — выход через Wails
}

func (a *App) updVersion() string {
	if a.upd.version != "" {
		return a.upd.version
	}
	return appVersion(wailsJSON)
}

func (a *App) updGOOS() string {
	if a.upd.goos != "" {
		return a.upd.goos
	}
	return runtime.GOOS
}

func (a *App) updDir() string {
	if a.upd.dir != "" {
		return a.upd.dir
	}
	return update.Dir(a.data)
}

// updSource — клиент и разрешена ли проверка при запуске (в dev без стенда — нет).
func (a *App) updSource() (*update.Client, bool) {
	if a.upd.client != nil {
		return a.upd.client()
	}
	src, auto := update.Default()
	return update.New(src), auto
}

// updExe — путь запущенного exe ("" — не определился).
func (a *App) updExe() string {
	f := os.Executable
	if a.upd.exe != nil {
		f = a.upd.exe
	}
	exe, _ := f()
	return exe
}

// updMode — как обновляется эта копия; предложение «только страница» — page.
func (a *App) updMode(o *update.Offer) update.Mode {
	if o != nil && o.Page {
		return update.ModePage
	}
	return update.ModeOf(a.updGOOS(), a.updExe())
}

// updSnap — текущее состояние без Just/Failed (их отдаёт только UpdateInfo, один раз).
func (a *App) updSnap() UpdateInfo {
	u := prefs.UpdateOf(a.cfgDir)
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	return a.updSnapLocked(u)
}

func (a *App) updSnapLocked(u prefs.Update) UpdateInfo {
	info := UpdateInfo{Phase: a.upd.phase, Notes: []string{}, Done: a.upd.done, Total: a.upd.total, Error: a.upd.err,
		AtStart: !u.Off, Checked: u.Checked}
	if info.Phase == "" {
		info.Phase = updIdle
	}
	if o := a.upd.offer; o != nil {
		info.Version, info.Notes = o.Version, append(info.Notes, o.Notes...)
	}
	info.Mode = string(a.updMode(a.upd.offer))
	return info
}

// emitState — update:state со снимком (блок «О программе» перерисовывается без опроса).
func (a *App) emitUpdState() { a.emit("update:state", a.updSnap()) }

// UpdateInfo — текущее состояние без сети. Just и Failed отдаются один раз: интерфейс зовёт
// при запуске и показывает тост.
func (a *App) UpdateInfo() (info UpdateInfo) {
	defer guard("UpdateInfo")
	info = a.updSnap()
	a.upd.mu.Lock()
	info.Just, info.Failed = a.upd.just, a.upd.failed
	a.upd.just, a.upd.failed = "", ""
	a.upd.mu.Unlock()
	return info
}

// updateStartup — итог прошлого запуска установщика (startup, до первого UpdateInfo): версия
// в Pending уже стоит (или новее) — Just; иначе недавно — Failed; старше updFailWindow — молча.
// Pending снимается в любом случае.
func (a *App) updateStartup() {
	u := prefs.UpdateOf(a.cfgDir)
	if u.Pending == "" {
		return
	}
	prefs.UpdateUpdate(a.cfgDir, func(n *prefs.Update) { n.Pending = "" })
	var just, failed string
	cur := a.updVersion()
	_, curOK := update.ParseVersion(cur)
	switch ago := time.Since(time.UnixMilli(u.PendingAt)); {
	case !curOK: // dev: сравнить нечем, итог молчит
	case !update.Newer(u.Pending, cur):
		just = u.Pending
	case ago >= 0 && ago < updFailWindow:
		failed = u.Pending
	}
	log.Printf("обновление: итог запуска установщика %s — обновлено %q, не установилось %q", u.Pending, just, failed)
	a.upd.mu.Lock()
	a.upd.just, a.upd.failed = just, failed
	a.upd.mu.Unlock()
}

// updBegin — занять операцию (проверка, скачивание); false — другая уже идёт. Контекст — от
// выхода программы, CancelUpdate его отменяет. Пустая phase — тихая проверка при запуске: она
// не меняет фазу, поэтому и не идёт, если фаза уже есть (человек успел нажать «Проверить» —
// её итог, вплоть до ready, тихая проверка не затрёт).
func (a *App) updBegin(phase string) (context.Context, bool) {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	if a.upd.busy || phase == "" && a.upd.phase != "" {
		return nil, false
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	a.upd.busy, a.upd.cancel = true, cancel
	if phase != "" {
		a.upd.phase, a.upd.err = phase, ""
	}
	return ctx, true
}

// updEnd — операция закончена.
func (a *App) updEnd() {
	a.upd.mu.Lock()
	if a.upd.cancel != nil {
		a.upd.cancel()
	}
	a.upd.busy, a.upd.cancel = false, nil
	a.upd.mu.Unlock()
}

// CheckUpdate — «Проверить обновления»: всегда в сеть (флажок не мешает — это действие
// человека); итог — в phase: latest, available или error. Идёт другая операция — её состояние.
func (a *App) CheckUpdate() (info UpdateInfo, err error) {
	defer guardErr("CheckUpdate", &err)
	a.checkUpdate(false)
	return a.updSnap(), nil
}

// checkUpdate — проверка; silent (при запуске) не меняет фазу до результата и ошибки пишет
// только в log.txt. Найденное новее при silent — update:offer.
func (a *App) checkUpdate(silent bool) {
	phase := updChecking
	if silent {
		phase = ""
	}
	ctx, ok := a.updBegin(phase)
	if !ok {
		return
	}
	defer a.updEnd()
	if !silent {
		a.emitUpdState()
	}
	c, _ := a.updSource()
	seen := prefs.UpdateOf(a.cfgDir).Seen
	if c.Stand() {
		seen = "" // версии ленты стенда не сверяются с настоящими выпусками
	}
	res, err := update.Check(ctx, c, a.updVersion(), seen)
	if err != nil {
		log.Printf("обновление: проверка не удалась: %s", update.Detail(err))
		if !silent {
			a.updFail(err)
		}
		return
	}
	if c.Stand() { // ленту стенда QA в config.json не пишем: Seen и Checked настоящих выпусков остаются
		log.Printf("обновление: источник — стенд, Seen и Checked не сохраняются")
	} else {
		now := time.Now().UnixMilli()
		prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Checked, u.Seen = now, res.Seen })
	}
	if res.Rollback {
		log.Printf("обновление: подписанный файл старее виденной версии — отклонён")
	}
	a.upd.mu.Lock()
	a.upd.offer, a.upd.phase, a.upd.err = res.Offer, updLatest, ""
	if res.Offer != nil {
		a.upd.phase = updAvailable
	}
	a.upd.mu.Unlock()
	if res.Offer != nil {
		log.Printf("обновление: найдена версия %s", res.Offer.Version)
	}
	a.emitUpdState()
	if silent && res.Offer != nil {
		a.emit("update:offer", a.updSnap())
	}
}

// updFail — фаза error с текстом ошибки; отмена — не ошибка (фаза возвращается к предложению).
func (a *App) updFail(err error) {
	a.upd.mu.Lock()
	if errors.Is(err, update.ErrCancelled) {
		a.upd.phase = updAvailable
		if a.upd.offer == nil {
			a.upd.phase = updIdle
		}
	} else {
		a.upd.phase, a.upd.err = updError, err.Error()
	}
	a.upd.mu.Unlock()
	a.emitUpdState()
}

// DownloadUpdate — скачать и проверить предложенное: ход — update:progress; уже скачанное и
// сверенное — сразу ready. Только для install и setup; для page и без предложения —
// ничего не делает, отдаёт состояние.
func (a *App) DownloadUpdate() (info UpdateInfo, err error) {
	defer guardErr("DownloadUpdate", &err)
	a.upd.mu.Lock()
	o := a.upd.offer
	busy := a.upd.busy
	a.upd.mu.Unlock()
	// Нет предложения или ставят вручную — не отказ: «Повторить» по кругу не нужен, снимок сам
	// скажет интерфейсу mode=page («Скачать» со страницы) или фазу без предложения («Проверить»).
	if busy || o == nil || o.Setup == nil || a.updMode(o) == update.ModePage {
		return a.updSnap(), nil
	}
	ctx, ok := a.updBegin(updDownloading)
	if !ok {
		return a.updSnap(), nil
	}
	defer a.updEnd()
	a.upd.mu.Lock()
	a.upd.done, a.upd.total = 0, o.Setup.Size
	a.upd.mu.Unlock()
	a.emitUpdState()
	c, _ := a.updSource()
	path, err := update.Fetch(ctx, c, *o, a.updDir(), a.upd.free, a.updProgress(o.Setup.Size))
	if err != nil {
		log.Printf("обновление: скачивание не удалось: %s", update.Detail(err))
		a.updFail(err)
		return a.updSnap(), nil
	}
	a.upd.mu.Lock()
	a.upd.phase, a.upd.path, a.upd.done = updReady, path, o.Setup.Size
	a.upd.mu.Unlock()
	log.Printf("обновление: версия %s скачана и сверена", o.Version)
	a.emitUpdState()
	return a.updSnap(), nil
}

// updProgress — колбэк хода: update:progress не чаще progressGap, конец — сразу.
func (a *App) updProgress(total int64) func(done int64) {
	var last time.Time
	return func(done int64) {
		a.upd.mu.Lock()
		a.upd.done = done
		a.upd.mu.Unlock()
		if now := time.Now(); done >= total || now.Sub(last) >= progressGap {
			last = now
			a.emit("update:progress", updProgressEvent{Done: done, Total: total})
		}
	}
}

// CancelUpdate — «Отменить»: идущее скачивание (или проверка) обрывается, .part удаляется.
func (a *App) CancelUpdate() {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	if a.upd.cancel != nil {
		a.upd.cancel()
	}
}

// InstallUpdate — «Перезапустить и обновить»: только ready и Windows. Сверка sha256 заново (файл
// мог измениться на диске) → pending в config.json → запуск установщика → выход. Установленная
// копия: тихо, на месте, с перезапуском; переносная — обычный мастер.
func (a *App) InstallUpdate() (err error) {
	defer guardErr("InstallUpdate", &err)
	a.upd.mu.Lock()
	o, path, phase := a.upd.offer, a.upd.path, a.upd.phase
	a.upd.mu.Unlock()
	switch {
	case a.updGOOS() != "windows":
		return errUpdWindows
	case phase != updReady || o == nil || o.Setup == nil || path == "":
		return errUpdNotRdy
	}
	if err := update.Verify(path, *o.Setup); err != nil {
		log.Printf("обновление: файл не прошёл повторную проверку: %s", update.Detail(err))
		a.upd.mu.Lock()
		a.upd.phase, a.upd.err, a.upd.path = updError, err.Error(), ""
		a.upd.mu.Unlock()
		a.emitUpdState()
		return err
	}
	mode := a.updMode(o)
	cmdline, err := update.CmdLine(path, mode, filepath.Dir(a.updExe()))
	if err != nil {
		log.Printf("обновление: командная строка установщика не собрана: %v", err) // ErrPath — без пути
		return errUpdStart
	}
	now := time.Now().UnixMilli()
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Pending, u.PendingAt = o.Version, now })
	start := update.Start
	if a.upd.start != nil {
		start = a.upd.start
	}
	if err := start(path, cmdline); err != nil {
		log.Printf("обновление: установщик не запустился: %v", paths.NoPath(err)) // путь и имя пользователя — не в лог
		prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Pending, u.PendingAt = "", 0 })
		return errUpdStart
	}
	log.Printf("обновление: запущен установщик версии %s (%s)", o.Version, mode)
	a.updQuit()
	return nil
}

// updQuit — закрыть программу: shutdown сохранит альбом. Установщик ждёт её выхода.
func (a *App) updQuit() {
	switch {
	case a.upd.quit != nil:
		a.upd.quit()
	case a.ctx != nil:
		wr.Quit(a.ctx)
	}
}

// OpenUpdatePage — страница выпуска в браузере: проверенной версии, а без предложения — последнего
// выпуска. Адрес строится здесь; от интерфейса его нет.
func (a *App) OpenUpdatePage() (err error) {
	defer guardErr("OpenUpdatePage", &err)
	u := repoURL + "/releases/latest"
	a.upd.mu.Lock()
	if o := a.upd.offer; o != nil && update.SetupName(o.Version) != "" {
		u = repoURL + "/releases/tag/v" + o.Version
	}
	a.upd.mu.Unlock()
	openURL(a, u)
	return nil
}

// SetUpdateCheck — флажок «Проверять при запуске»; снятие отменяет ещё не начатую проверку.
func (a *App) SetUpdateCheck(atStart bool) (info UpdateInfo) {
	defer guard("SetUpdateCheck")
	prefs.UpdateUpdate(a.cfgDir, func(u *prefs.Update) { u.Off = !atStart })
	return a.updSnap()
}

// scheduleUpdateCheck — при запуске (domReady): чистка папки скачанного и проверка обновления.
// Проверка — если не выключена, источник разрешён (в dev — только стенд), прошло ≥ startEvery
// (часы ушли назад — тоже пора), раз за сеанс; через startDelay — окно уже открыто. Флажок
// перечитывается после задержки: снятие за это время отменяет проверку.
func (a *App) scheduleUpdateCheck() {
	update.Clean(a.updDir(), a.updVersion())
	_, auto := a.updSource()
	u := prefs.UpdateOf(a.cfgDir)
	since := time.Now().UnixMilli() - u.Checked
	if u.Off || !auto || since >= 0 && since < startEvery.Milliseconds() {
		return
	}
	if !a.upd.started.CompareAndSwap(false, true) {
		return
	}
	if !a.goBG(func() {
		defer guard("updateCheck")
		t := time.NewTimer(startDelay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-a.stopCtx.Done():
			return
		}
		if !prefs.UpdateOf(a.cfgDir).Off {
			a.checkUpdate(true)
		}
	}) {
		a.upd.started.Store(false) // фон закрыт: проверка не запущена, раз за сеанс не потрачен
	}
}
