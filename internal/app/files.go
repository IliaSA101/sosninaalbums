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
	st, _ := a.startNewIf(nil, ed)
	return st
}

// startNewIf — startNew, но при from != nil уход разрешён, только пока текущий — from: сверка под
// saveMu, в одном шаге со сменой (иначе человек, открывший другой альбом за время пробы агента,
// был бы уведён без вопроса). Не совпало — changed, ничего не тронуто.
func (a *App) startNewIf(from, ed *album.Editor) (st State, changed bool) {
	// Застрявший сломанный черновик не удаляем: сначала пробуем убрать его в .bad.
	// Не вышло — оставляем всё как есть, иначе новый проект пошёл бы поверх нечитаемого черновика.
	if !a.unlockDraft() {
		a.notice("Не удалось сохранить прежний черновик отдельно, новый проект не создан. Закройте программы, держащие файл «черновик" + projectExt + "», и повторите.")
		return a.GetState(), false
	}
	// Запись прежнего — в retire: после разблокировки правки пустого альбома попадут в черновик и
	// его копию (removeDraft).
	if !a.retireIf(from, ed, nil) {
		return a.GetState(), true
	}
	a.removeDraft()
	a.updateConfig(func(c *prefs.Config) { c.LastProject = "" })
	a.rescan()
	return a.emitState(), false
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
	st, _, err := a.openParsed(path, p, af, nil)
	return st, err
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
	cur := ed.Path()
	otherPath := cur == "" || !paths.SamePath(cur, path) // по этому пути записывается другой файл
	a.backupBefore(path, backupKey(path), otherPath)
	// Решение «тот же проект или другой» и запись — под одним замком Editor (SaveOver): поверх файла
	// с тем же отпечатком отпечаток и привязки к выгруженным альбомам сохраняются (167-25), иначе
	// новый отпечаток без привязок: запись о ссылке прежнего проекта к нему не подойдёт (167-1).
	if _, err := ed.SaveOver(path); err != nil {
		return ed.View(), err
	}
	a.noticeBackup(ed)
	a.shr.OnSaved(path, otherPath) // идущая выгрузка и правка ссылки этого пути останавливаются (167-10)
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
		a.armSaveLocked()
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

// armSaveLocked (под a.mu) перезапускает таймер автосохранения: через секунду тишины проект пишется.
func (a *App) armSaveLocked() {
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.saveTimer = time.AfterFunc(time.Second, a.autosave)
}

// armSave — armSaveLocked для тех, кто не держит a.mu (привязка к ссылке — не правка, 167-15).
func (a *App) armSave() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.armSaveLocked()
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
func (a *App) retire(next *album.Editor, between func()) { a.retireIf(nil, next, between) }

// retireIf — retire, который при from != nil ничего не делает (false), если текущий уже не from.
func (a *App) retireIf(from, next *album.Editor, between func()) bool {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if from != nil && a.editor() != from {
		return false
	}
	defer a.setEditor(next) // до Unlock: автосохранение после смены пишет уже next
	if ed := a.editor(); ed != nil {
		ed.Close()
	}
	a.save()
	if ed := a.editor(); ed != nil {
		if f := a.onRetire.Load(); f != nil {
			(*f)(ed) // копия правок человека, дошедших до Close (agentquiet_pin.go)
		}
	}
	if between != nil {
		between()
	}
	return true
}

// save пишет проект или черновик; вызывать под saveMu.
func (a *App) save() {
	defer guard("автосохранение")
	ed := a.editor()
	if ed == nil || ed == a.holdSave.Load() {
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
	// «Только что добавлено» — состояние альбома (160-M1), чужому не передаётся: смена редактора и сброс набора
	// под одним замком (addLoose проверяет текущий редактор под ним же).
	a.fresh.switchTo(func() { a.ed.Store(ed) })
}
