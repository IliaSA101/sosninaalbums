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
	"github.com/IliaSA101/sosninaalbums/internal/osx"
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
	a.backupDraft() // «Новый» и «Сохранить как» иначе удаляли бы несохранённый черновик без следа
	if err := os.Remove(a.draftPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("черновик не удалён: %v", paths.NoPath(err))
	}
}

// emitState рассылает вид нового проекта (как changed) и возвращает состояние целиком.
func (a *App) emitState() State {
	st := a.GetState()
	if a.ctx != nil {
		wr.EventsEmit(a.ctx, "view", st.View)
	}
	return st
}

// NewProject создаёт пустой альбом заданного изделия (чужое — album). Тип после
// создания не меняется.
func (a *App) NewProject(product string) State {
	a.flushSave()
	// Застрявший сломанный черновик не удаляем: сначала пробуем убрать его в .bad.
	// Не вышло — оставляем всё как есть, иначе новый проект пошёл бы поверх нечитаемого черновика.
	if !a.unlockDraft() {
		a.notice("Не удалось сохранить прежний черновик отдельно, новый проект не создан. Закройте программы, держащие файл «черновик" + projectExt + "», и повторите.")
		return a.GetState()
	}
	a.ed.Store(blankEditor(product))
	a.removeDraft()
	a.updateConfig(func(c *prefs.Config) { c.LastProject = "" })
	a.rescan()
	return a.emitState()
}

func (a *App) OpenProject() (State, error) {
	path, err := wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Открыть альбом",
		DefaultDirectory: a.dialogDir(prefs.DirProject),
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil || path == "" {
		return a.GetState(), err
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
	p, err := salbum.Load(path)
	if err != nil {
		return a.GetState(), err
	}
	ed, err := album.NewEditor(p, path)
	if err != nil {
		return a.GetState(), err
	}
	a.flushSave()
	a.ed.Store(ed)
	a.rememberProject(path)
	a.rescan()
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
	err := ed.Save("")
	return ed.View(), err
}

func (a *App) SaveProjectAs() (model.View, error) {
	ed := a.editor()
	path, err := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{
		Title:            "Сохранить альбом",
		DefaultDirectory: a.dialogDir(prefs.DirProject),
		DefaultFilename:  "Альбом" + projectExt,
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil || path == "" {
		return ed.View(), err
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
	if cur := ed.Path(); cur == "" || !strings.EqualFold(filepath.Clean(cur), filepath.Clean(path)) {
		a.backupBefore(path, backupKey(path), true)
	} else {
		a.backupBefore(path, backupKey(path), false)
	}
	if err := ed.Save(path); err != nil {
		return ed.View(), err
	}
	a.rememberProject(path)
	// Застрявший черновик (draftLocked) не трогаем: он нечитаем, но это данные пользователя.
	if !a.draftLocked.Load() {
		a.removeDraft()
	}
	return ed.View(), nil
}

func (a *App) draftPath() string { return filepath.Join(a.cfgDir, "черновик"+projectExt) }

// loadStartup открывает последний проект, а если его нет — черновик; иначе пустой.
// Файл, который есть, но не открылся, не должен быть затёрт пустым проектом:
// чужой .salbum остаётся как есть (path пуст, автосохранение пишет только черновик),
// а сломанный черновик уходит в «черновик.bad-<время>.salbum» рядом. Черновик более новой
// версии (ErrNewer) не сломан: его не трогаем, а автосохранение черновика на сессию выключаем.
func (a *App) loadStartup(last string) (p *model.Project, path, notice string) {
	var notes []string
	for _, cand := range []string{last, a.draftPath()} {
		if cand == "" {
			continue
		}
		if cand == last && !reachable(cand) {
			// Отключённый сетевой диск: Load ждал бы таймаута, окно — пустое. LastProject не трогаем:
			// со следующим запуском при сети альбом откроется.
			notes = append(notes, "Диск "+filepath.VolumeName(cand)+" с последним альбомом недоступен. Открыт черновик.")
			continue
		}
		loaded, err := salbum.Load(cand)
		if err == nil {
			if cand == a.draftPath() {
				cand = ""
			}
			return loaded, cand, strings.Join(notes, " ")
		}
		if errors.Is(err, os.ErrNotExist) {
			if cand == last {
				notes = append(notes, "Последний проект не найден: "+cand+". Открыт черновик.")
			}
			continue
		}
		log.Printf("не открылся проект %s: %v", filepath.Base(cand), paths.NoPath(err))
		if cand != a.draftPath() {
			notes = append(notes, "Не открылся проект: "+err.Error()+".")
			continue
		}
		if errors.Is(err, salbum.ErrNewer) {
			a.draftNewer.Store(true)
			notes = append(notes, "Черновик сохранён более новой версией программы, здесь он не открывается и не изменится.")
			continue
		}
		bad := a.badDraftPath(time.Now())
		if rerr := a.rename(cand, bad); rerr != nil {
			log.Printf("черновик не удалось сохранить в %s: %v", filepath.Base(bad), paths.NoPath(rerr))
			a.draftLocked.Store(true)
			notes = append(notes, "Не открылся черновик ("+err.Error()+") и не сохранён отдельно: автосохранение отключено до перезапуска.")
		} else {
			notes = append(notes, "Не открылся черновик ("+err.Error()+"). Копия сохранена: "+bad+".")
		}
	}
	return model.New(), "", strings.Join(notes, " ")
}

// blankEditor — редактор нового пустого альбома изделия product. В model.New() путей нет:
// проверка NewEditor здесь не откажет, отказ — ошибка программы.
func blankEditor(product string) *album.Editor {
	p := model.New()
	p.Product = model.NormProduct(product)
	ed, err := album.NewEditor(p, "")
	if err != nil {
		panic(err)
	}
	return ed
}

// startEditor — редактор для проекта loadStartup; возвращает путь и уведомление. Load отдаёт
// проверенный проект, и отказ NewEditor сюда не доходит (Load ⊆ Check, FuzzLoad). Если всё же —
// пустой альбом, а черновик до перезапуска не пишется (draftLocked). Замок ставится и когда
// отвергнут LastProject при целом черновике: иначе автосохранение пустого альбома затёрло бы
// черновик; «Новый» потом переименует черновик в .bad.
// В log.txt — без текста ошибки: в нём путь.
func (a *App) startEditor(p *model.Project, path, notice string) (string, string) {
	ed, err := album.NewEditor(p, path)
	if err == nil {
		a.ed.Store(ed)
		return path, notice
	}
	log.Printf("проект при запуске не прошёл проверку путей")
	a.draftLocked.Store(true)
	a.ed.Store(blankEditor(""))
	return "", strings.TrimSpace(notice + " Не открылся проект: " + err.Error() + ". Автосохранение отключено до перезапуска.")
}

// badDraftPath — куда уходит сломанный черновик.
func (a *App) badDraftPath(t time.Time) string {
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
		log.Printf("черновик по-прежнему не удалось сохранить в %s: %v", filepath.Base(bad), paths.NoPath(err))
		return false
	}
	a.draftLocked.Store(false)
	return true
}

// changed — после каждой правки: через секунду тишины проект сохраняется сам.
func (a *App) changed(v model.View) model.View {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.saveTimer = time.AfterFunc(time.Second, a.autosave)
	// Другие открытые клиенты (окно и браузер в режиме разработки) тоже обновятся.
	a.emit("view", v)
	return v
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
		if a.draftLocked.Load() || a.draftNewer.Load() {
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
	if a.ctx != nil {
		wr.EventsEmit(a.ctx, "saved", ed.View())
	}
}

// ShowPhotoInExplorer показывает файл фото альбома в Проводнике. Путь берётся из
// проекта по ID, от интерфейса не принимается (как ShowInExplorer). Файла нет —
// открывается его папка; нет и её — ошибка.
func (a *App) ShowPhotoInExplorer(id string) error {
	r, ok := a.editor().Ref(id)
	if !ok {
		return errors.New("фото нет в альбоме")
	}
	path := filepath.Clean(r.Path)
	if err := paths.CheckLocalPath(path); err != nil {
		return errors.New("путь к фото недоступен")
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return revealPath(path, true)
	}
	if dir := filepath.Dir(path); isDir(dir) {
		return revealPath(dir, false)
	}
	return errors.New("файл не найден")
}

// revealPath — запуск файлового менеджера; в тестах подменяется, чтобы не стартовать процесс.
// file — выделить файл в его папке, иначе открыть папку. Ждать завершения не нужно.
var revealPath = osx.Reveal

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
