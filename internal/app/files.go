package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// removeDraft удаляет черновик после перехода на новый проект (кроме черновика новой версии); сбой только логируем:
// LastProject в конфиге всё равно указывает на нужный файл.
func (a *App) removeDraft() {
	if a.draftNewer.Load() {
		return // чужой версии: это данные пользователя, не наш мусор
	}
	if a.draftPath() == "" {
		return
	}
	a.backupDraft() // «Новый» и «Сохранить как» иначе удаляли бы несохранённый черновик без следа
	if err := os.Remove(a.draftPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("черновик не удалён: %v", paths.NoPath(err))
	}
}

// emitState рассылает вид нового проекта (как changed) и возвращает состояние целиком. Вид —
// полный и уже в цепочке (GetState → fullLocked).
func (a *App) emitState() State {
	st := a.GetState()
	a.emit("view", st.View)
	return st
}

// NewProject создаёт пустой альбом заданного изделия (чужое — album). Тип после
// создания не меняется.
func (a *App) NewProject(product string) State {
	return a.startNew(blankEditor(product))
}

// startNew — общий путь «Новый» (альбом и лист 031): прежний проект сохраняется, черновик
// убирается, редактор ed становится текущим.
func (a *App) startNew(ed *album.Editor) State {
	// Застрявший сломанный черновик не удаляем: сначала пробуем убрать его в .bad.
	// Не вышло — оставляем всё как есть, иначе новый проект пошёл бы поверх нечитаемого черновика.
	if !a.unlockDraft() {
		a.notice("Не удалось сохранить прежний черновик отдельно, новый проект не создан. Закройте программы, держащие файл «черновик" + projectExt + "», и повторите.")
		return a.GetState()
	}
	// Запись прежнего — в retire: после разблокировки правки пустого альбома попадут в черновик и
	// его копию (removeDraft).
	a.retire(ed, nil)
	a.removeDraft()
	a.updateConfig(func(c *prefs.Config) { c.LastProject = "" })
	a.rescan()
	return a.emitState()
}

func (a *App) OpenProject() (State, error) {
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Открыть альбом",
		DefaultDirectory: a.dialogDir(prefs.DirProject),
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil || path == "" {
		st := a.GetState()
		st.Cancelled = err == nil // сбой диалога — не отмена: фронт покажет ошибку
		return st, err
	}
	return a.openPath(path)
}

// openPath — общий путь открытия (диалог и список последних).
func (a *App) openPath(path string) (State, error) {
	if !reachable(path) {
		// Отключённый сетевой диск: чтение файла ждало бы таймаута SMB.
		return a.GetState(), fmt.Errorf("диск %s недоступен", filepath.VolumeName(path))
	}
	if a.backups.Inside(path) {
		return a.openBackup(path)
	}
	p, af, err := a.loadAlbum(path)
	if errors.Is(err, salbum.ErrNewer) {
		// Файл новее программы — окно «Альбом из новой версии» вместо тоста; прежний альбом остаётся.
		info := a.setNewer(path, false, err)
		st := a.GetState()
		st.Cancelled, st.Newer = true, &info
		return st, nil
	}
	if err != nil {
		return a.GetState(), openErr(err)
	}
	ed, err := album.NewEditor(p, path)
	if err != nil {
		return a.GetState(), err
	}
	a.retire(ed, nil)
	a.rememberProject(path)
	a.rescan()
	a.keepAlbumFonts(af) // тост и новые начертания — в ответе (State.Info, State.FreshFonts)
	return a.emitState(), nil
}

// SaveProject сохраняет в текущий файл; если файла ещё нет — спрашивает куда.
func (a *App) SaveProject() (model.View, error) {
	ed := a.editor()
	path := ed.Path()
	if path == "" {
		return a.SaveProjectAs()
	}
	a.backupBefore(path, backupKey(path), false) // и без правок: см. save
	if err := ed.Save(""); err != nil {
		return ed.View(), err
	}
	a.noticeBackup(ed)
	return a.out(ed.View()), nil
}

func (a *App) SaveProjectAs() (model.View, error) {
	ed := a.editor()
	path, err := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{
		Title:            "Сохранить альбом",
		DefaultDirectory: a.dialogDir(prefs.DirProject),
		DefaultFilename:  "Альбом" + projectExt,
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil {
		return ed.View(), err
	}
	if path == "" {
		return a.out(ed.View()), nil
	}
	if !strings.HasSuffix(strings.ToLower(path), projectExt) {
		path += projectExt
	}
	return a.saveAs(ed, path)
}

// saveAs — «Сохранить как» в выбранный path (после диалога).
func (a *App) saveAs(ed *album.Editor, path string) (model.View, error) {
	if err := a.checkSaveTarget(path); err != nil {
		return ed.View(), err
	}
	// Файл, поверх которого пишут, — в копию: «Сохранить как» мог промахнуться мимо имени.
	if cur := ed.Path(); cur == "" || !paths.SamePath(cur, path) {
		a.backupBefore(path, backupKey(path), true)
	} else {
		a.backupBefore(path, backupKey(path), false)
	}
	if err := ed.Save(path); err != nil {
		return ed.View(), err
	}
	a.noticeBackup(ed)
	a.rememberProject(path)
	// Застрявший черновик (draftLocked) не трогаем: он нечитаем, но это данные пользователя.
	if !a.draftLocked.Load() {
		a.removeDraft()
	}
	return a.out(ed.View()), nil
}

// draftPath — черновик в папке настроек; "" — папки нет (настройки не пишутся), черновика нет.
func (a *App) draftPath() string {
	if a.cfgDir == "" {
		return ""
	}
	return filepath.Join(a.cfgDir, "черновик"+projectExt)
}

// blankEditor — редактор нового пустого альбома изделия product.
func blankEditor(product string) *album.Editor {
	p := model.New()
	p.Product = model.NormProduct(product)
	return freshEditor(p)
}

// freshEditor — редактор нового проекта без файла. В новом проекте путей нет: проверка
// NewEditor здесь не откажет, отказ — ошибка программы.
func freshEditor(p *model.Project) *album.Editor {
	ed, err := album.NewEditor(p, "")
	if err != nil {
		panic(err)
	}
	return ed
}

// badDraftPath — куда уходит сломанный черновик.
func (a *App) badDraftPath(t time.Time) string {
	if a.cfgDir == "" {
		return ""
	}
	return filepath.Join(a.cfgDir, "черновик.bad-"+t.Format("20060102-150405")+projectExt)
}

// unlockDraft повторяет перенос застрявшего черновика в .bad; true — писать в черновик снова можно.
// Черновик новой версии не переносим (он не сломан): true, но писать в него всё равно нельзя
// (flushSave проверяет draftNewer), а removeDraft его не удалит.
func (a *App) unlockDraft() bool {
	if a.draftNewer.Load() {
		return true
	}
	if !a.draftLocked.Load() {
		return true
	}
	bad := a.badDraftPath(time.Now())
	if err := a.rename(a.draftPath(), bad); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("черновик по-прежнему не удалось сохранить отдельно: %v", paths.NoPath(err))
		return false
	}
	a.draftLocked.Store(false)
	return true
}

// changed — после каждой правки: через секунду тишины проект сохраняется сам; список
// недостающих следует за набором фото альбома (убрали битое фото — полоса уходит сразу).
func (a *App) changed(v model.View) (w model.View) {
	var old bool
	func() {
		a.mu.Lock()
		defer a.mu.Unlock() // и при панике рассылки: guardEdit затем читает вид
		if a.saveTimer != nil {
			a.saveTimer.Stop()
		}
		a.saveTimer = time.AfterFunc(time.Second, a.autosave)
		old = foreign(v, a.editor())
		// Другие открытые клиенты (окно и браузер в режиме разработки) тоже обновятся; холостой
		// и обогнанной правке событие не нужно (075-3).
		var send bool
		if w, send = a.outLocked(v); send {
			a.emit("view", w)
		}
	}()
	if !old { // rev прежнего альбома сбил бы проверку недостающих нынешнего
		a.syncMissing(v.Rev)
	}
	return w
}

// autosave — сработал таймер. Проверка stopped под saveMu: запись не может начаться после
// барьера в stopBackground.
func (a *App) autosave() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if a.stopped.Load() {
		return
	}
	a.save()
}

func (a *App) flushSave() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.save()
}

// retire — смена альбома (087-5): прежний закрывается (Close ждёт правки, уже вставшие к его
// замку, поздние отклоняет), пишется и только потом next становится текущим; between — после
// записи прежнего, до смены (копия черновика перед openBackup). Всё под saveMu: две смены подряд
// не разминутся (вторая закроет и запишет next, а не снова прежний), автосохранение — между ними.
// Закрытый Editor текущим не остаётся: смена — в defer, и паника записи её не отменит.
func (a *App) retire(next *album.Editor, between func()) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	defer a.setEditor(next) // до Unlock: автосохранение после смены пишет уже next
	if ed := a.editor(); ed != nil {
		ed.Close()
	}
	a.save()
	if between != nil {
		between()
	}
}

// save пишет проект или черновик; вызывать под saveMu.
func (a *App) save() {
	defer guard("автосохранение")
	ed := a.editor()
	if ed == nil {
		return
	}
	var err error
	if path := ed.Path(); path != "" {
		// Без условия «есть правки»: Save перепишет и файл, который decode поправил при загрузке.
		// Дубль отсекает backup (совпал с последней копией), частоту — Every.
		a.backupBefore(path, backupKey(path), false)
		err = ed.Save("")
	} else {
		// Без файла проекта пишем черновик, чтобы ничего не потерялось.
		// Если старый черновик не открылся и не ушёл в .bad, поверх него не пишем.
		if a.draftLocked.Load() || a.draftNewer.Load() || a.draftPath() == "" {
			return
		}
		if backup.HasContent(a.draftPath()) { // пустой черновик копировать незачем
			a.backupBefore(a.draftPath(), backupKey(""), false)
		}
		err = ed.WriteDraft(a.draftPath())
	}
	if err != nil {
		log.Printf("автосохранение: %v", paths.NoPath(err))
		return
	}
	a.noticeBackup(ed)
	v := ed.View() // dirty и path — после записи: правка в промежутке снова ставит dirty
	a.emit("saved", Saved{Dirty: v.Dirty, Path: v.Path})
}

// Saved — событие saved (075-5): фронту нужны только эти два поля, а не вид в 1,2 МБ.
type Saved struct {
	Dirty bool   `json:"dirty"`
	Path  string `json:"path"`
}

// Настройки и «Последние» — рядом с открытием файлов: OpenRecent идёт через openPath.

// config.json лежит в папке данных App (cfgDir); логика и замок — в internal/prefs.
func (a *App) loadConfig() prefs.Config { return prefs.LoadConfig(a.cfgDir) }

func (a *App) updateConfig(fn func(c *prefs.Config)) { prefs.UpdateConfig(a.cfgDir, fn) }

func (a *App) rememberDir(kind, dir string) { prefs.RememberDir(a.cfgDir, kind, dir) }

func (a *App) dialogDir(kind string) string { return prefs.DialogDir(a.cfgDir, kind) }

// rememberRecent записывает альбом в «последние». Черновик (его путь внутренний) не пишем.
func (a *App) rememberRecent(path string) {
	if path == "" || paths.FoldEqual(path, a.draftPath()) {
		return
	}
	a.updateConfig(func(c *prefs.Config) { c.Recent = prefs.PushRecent(c.Recent, path) })
}

// OpenRecent открывает альбом из списка последних. Принимается только путь, который
// есть в config.recent: интерфейс не может открыть произвольный файл. Нет файла —
// ошибка, запись остаётся (её убирает ForgetRecent).
func (a *App) OpenRecent(path string) (State, error) {
	if prefs.RecentIndex(a.loadConfig().Recent, path) < 0 {
		return a.GetState(), fmt.Errorf("альбома нет в списке последних")
	}
	if err := paths.CheckLocalPath(path); err != nil {
		return a.GetState(), err
	}
	if !strings.HasSuffix(strings.ToLower(path), projectExt) {
		return a.GetState(), fmt.Errorf("это не альбом (%s)", projectExt)
	}
	return a.openPath(path)
}

// ForgetRecent убирает альбом из списка последних (сам файл не трогает).
func (a *App) ForgetRecent(path string) []prefs.RecentProject {
	a.updateConfig(func(c *prefs.Config) { c.Recent = prefs.DropRecent(c.Recent, path) })
	_, recent := a.prefsAndRecent()
	return recent
}

// Recent — список последних без записи config.json: «Сохранить как» кладёт альбом в список
// без события, и UI перечитывает его при открытии меню «Файл» и стартового окна.
func (a *App) Recent() []prefs.RecentProject {
	_, recent := a.prefsAndRecent()
	return recent
}

// ClearRecent очищает список последних одной записью config.json (файлы не трогает).
func (a *App) ClearRecent() []prefs.RecentProject {
	a.updateConfig(func(c *prefs.Config) { c.Recent = nil })
	return []prefs.RecentProject{}
}

// Резервные копии альбома (спека 019 E). a.backups == nil — копий нет, остальное работает.

// BackupInfo — строка «Резервных копий». Time — unix-миллисекунды (местное время из имени).
type BackupInfo struct {
	Name    string `json:"name"`
	Time    int64  `json:"time"`
	Size    int64  `json:"size"`
	Spreads int    `json:"spreads"`
	Photos  int    `json:"photos"`
}

const backupNotice = "Это резервная копия — сохраните её через «Сохранить как»"

// startBackups — хранилище копий и чистка старых в фоне (startup).
func (a *App) startBackups() {
	root := backup.DefaultRoot(a.data)
	if root == "" {
		return
	}
	a.backups = backup.New(root)
	a.pruneBackups()
}

// pruneBackups — общая чистка копий в фоне: при старте и каждые несколько записей за сеанс.
func (a *App) pruneBackups() {
	if a.stopped.Load() {
		return // выход: closeBackground уже ждёт фон
	}
	a.goBG(func() {
		defer guard("резервные копии")
		if err := a.backups.Prune(); err != nil {
			log.Printf("резервные копии, чистка: %v", paths.NoPath(err))
		}
	})
}

// backupKey — папка копий альбома path ("" — черновик).
func backupKey(path string) string {
	if path == "" {
		return backup.DraftKey
	}
	return backup.Key(path)
}

// backupBefore — копия прежнего файла перед его перезаписью: force — без учёта «раз в 10 минут».
// Сбой копии не мешает сохранению: только лог (с ключом, без пути).
func (a *App) backupBefore(path, key string, force bool) {
	a.backupKeep(path, key, force, "")
}

// backupKeep — backupBefore, только чистка не удаляет копию keep ("" — любую).
func (a *App) backupKeep(path, key string, force bool, keep string) {
	// Пропавший сетевой диск: чтение ждало бы таймаута SMB, а следом его ждёт и сама запись.
	if a.backups == nil || !reachable(path) {
		return
	}
	var err error
	switch {
	case keep != "":
		err = a.backups.ForceKeep(path, key, keep)
	case force:
		err = a.backups.Force(path, key)
	default:
		err = a.backups.Before(path, key)
	}
	if err != nil {
		log.Printf("резервная копия %s: %s", key, paths.NoPath(err))
	}
	if a.backups.PruneDue() {
		a.pruneBackups()
	}
}

// backupDraft — копия черновика перед тем, как его удалят или затрут чужим содержимым;
// пустой черновик не копируем.
func (a *App) backupDraft() { a.backupDraftKeep("") }

// backupDraftKeep — backupDraft, чистка не удаляет копию keep.
func (a *App) backupDraftKeep(keep string) {
	if a.backups == nil || a.draftNewer.Load() || a.draftPath() == "" || !backup.HasContent(a.draftPath()) {
		return
	}
	a.backupKeep(a.draftPath(), backup.DraftKey, true, keep)
}

// Backups — копии текущего альбома (или черновика), новые первыми.
func (a *App) Backups() (list []BackupInfo, err error) {
	defer guardErr("Backups", &err)
	list = []BackupInfo{}
	if a.backups == nil {
		return list, nil
	}
	ents, err := a.backups.List(backupKey(a.editor().Path()))
	if err != nil {
		return list, err
	}
	for _, e := range ents {
		list = append(list, BackupInfo{Name: e.Name, Time: e.Time.UnixMilli(), Size: e.Size, Spreads: e.Spreads, Photos: e.Photos})
	}
	return list, nil
}

// RestoreBackup заменяет альбом содержимым копии name одним шагом отмены. Имя — только из
// Backups: путь строится по ключу текущего альбома. Перед заменой текущий файл уходит в копию,
// а сама name при этой чистке не удаляется: иначе после Ctrl+Z её версии нигде не было бы.
func (a *App) RestoreBackup(name string) (v model.View, err error) {
	defer a.guardEdit("RestoreBackup", &v, &err)
	ed := a.editor()
	path := ed.Path()
	key := backupKey(path)
	file, ferr := a.backups.File(key, name)
	if ferr != nil {
		return ed.View(), backup.ErrNotFound
	}
	base := "" // черновик: только абсолютные пути
	if path != "" {
		if abs, aerr := filepath.Abs(path); aerr == nil {
			base = filepath.Dir(abs) // альбом переехал вместе с фото — относительные пути найдут их
		}
	}
	q, lerr := album.LoadBase(file, base)
	switch {
	case errors.Is(lerr, salbum.ErrNewer):
		return ed.View(), errors.New("копия записана более новой версией программы")
	case errors.Is(lerr, os.ErrNotExist):
		return ed.View(), backup.ErrNotFound
	case lerr != nil:
		return ed.View(), lerr
	}
	if path != "" {
		a.backupKeep(path, key, true, name)
	} else {
		a.backupDraftKeep(name)
	}
	before := sourcesOf(ed)
	v, err = ed.Restore(q)
	if err != nil {
		return v, err
	}
	v = a.changed(v)
	// Недостающие при смене набора фото без смены папок пересчитывает changed.
	if before.differs(sourcesOf(ed)) {
		a.rescan()
	}
	return v, nil
}

// ShowBackups открывает папку копий текущего альбома в Проводнике; её нет — корень копий.
func (a *App) ShowBackups() error {
	if a.backups == nil {
		return errors.New("папка резервных копий недоступна")
	}
	dir := a.backups.Dir(backupKey(a.editor().Path()))
	if dir == "" || !isDir(dir) {
		dir = a.backups.Root
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return revealPath(dir, false)
}

// openBackup открывает копию несохранённым альбомом: папку копий программа сама не перезаписывает,
// LastProject и «Последние» не меняются. Прежний черновик — в копию: автосохранение его затрёт.
func (a *App) openBackup(path string) (State, error) {
	p, err := album.LoadBase(path, "")
	if err != nil {
		return a.GetState(), err
	}
	ed, err := album.NewEditor(p, "")
	if err != nil {
		return a.GetState(), err
	}
	a.retire(ed, a.backupDraft)
	a.rescan()
	st := a.emitState()
	a.notice(backupNotice)
	return st, nil
}

// checkSaveTarget — «Сохранить как» в папку копий запрещено: её чистит Prune.
func (a *App) checkSaveTarget(path string) error {
	if a.backups.Inside(path) {
		return fmt.Errorf("сохраните альбом в другую папку: здесь резервные копии")
	}
	return nil
}

// rememberProject — альбом стал текущим файлом: в конфиг и папку диалога.
func (a *App) rememberProject(path string) {
	a.updateConfig(func(c *prefs.Config) { c.LastProject = path; c.Recent = prefs.PushRecent(c.Recent, path) })
	a.rememberDir(prefs.DirProject, filepath.Dir(path))
}

// setEditor делает ed текущим и подключает к нему библиотеку макетов (131): смена формата сама
// вписывает макет альбома, View.look.found знает, есть ли он на этом ПК.
func (a *App) setEditor(ed *album.Editor) {
	ed.SetLookHost(a.lookHost())
	a.ed.Store(ed)
}
