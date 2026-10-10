package app

import (
	"log"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetState() State {
	// Ждём startup: иначе снимок до загрузки проекта затёр бы в окне уже пришедшие события.
	select {
	case <-a.ready:
	case <-a.stopCtx.Done():
	}
	// View — под тем же замком, что photos/scan: снимок согласован с событиями library/scan,
	// а медленное чтение диска (prefsAndRecent) — уже после.
	a.mu.Lock()
	view := a.editor().View()
	photos, scan := a.photos, a.scan
	notice := a.startNotice
	a.startNotice = ""
	missing := append([]string{}, a.missing...)
	a.mu.Unlock()
	if photos == nil {
		photos = []library.Photo{}
	}
	p, recent := a.prefsAndRecent()
	return State{View: view, Photos: photos, Scan: scan, Notice: notice, Missing: missing,
		Version: appVersion(wailsJSON), Fonts: text.Families(), Prefs: p, Recent: recent}
}

// notice показывает пользователю текст ошибки тостом (событие `notice`); без окна — только лог.
func (a *App) notice(text string) {
	log.Print(text)
	a.emit("notice", text)
}

// ChooseFolder спрашивает папку с фото и добавляет её в проект.
func (a *App) ChooseFolder() (model.View, error) {
	dir, err := wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Папка с фотографиями",
		DefaultDirectory: a.dialogDir(prefs.DirPhotos),
	})
	if err != nil || dir == "" {
		return a.editor().View(), err
	}
	v, err := a.AddFolderPath(dir)
	if err == nil {
		a.rememberDir(prefs.DirPhotos, filepath.Dir(dir)) // соседняя съёмка обычно лежит рядом
	}
	return v, err
}

// AddFolderPath: недопустимый путь — ошибка (отклонённый промис), проект не меняется.
func (a *App) AddFolderPath(dir string) (v model.View, err error) {
	defer a.guardEdit("AddFolderPath", &v, &err)
	v, err = a.editor().AddFolder(dir)
	if err != nil {
		return v, err
	}
	a.rescan()
	return a.changed(v), nil
}

func (a *App) RemoveFolder(dir string) (v model.View) {
	defer a.guardEdit("RemoveFolder", &v, nil)
	v = a.changed(a.editor().RemoveFolder(dir))
	a.rescan()
	return v
}

func (a *App) Rescan() { a.rescan() }

// AddPhotos, NewSpread, MovePhoto: отказ (лимит фото на развороте) — ошибка, проект не меняется.
func (a *App) AddPhotos(spreadID string, ids []string, index int) (v model.View, err error) {
	defer a.guardEdit("AddPhotos", &v, &err)
	return a.changedOrErr(a.editor().AddPhotos(spreadID, a.infos(ids), index))
}

func (a *App) NewSpread(at int, ids []string) (v model.View, err error) {
	defer a.guardEdit("NewSpread", &v, &err)
	infos := a.infos(ids)
	if len(ids) > 0 && len(infos) == 0 {
		// Фото пропали при пересканировании во время переноса: пустой разворот здесь не просили.
		return a.editor().View(), nil
	}
	return a.changedOrErr(a.editor().NewSpread(at, infos))
}

func (a *App) MovePhoto(photoID, toSpread string, index int) (v model.View, err error) {
	defer a.guardEdit("MovePhoto", &v, &err)
	return a.changedOrErr(a.editor().MovePhoto(photoID, toSpread, index))
}

// changedOrErr рассылает вид, только если правка прошла.
func (a *App) changedOrErr(v model.View, err error) (model.View, error) {
	if err != nil {
		return v, err
	}
	return a.changed(v), nil
}

func (a *App) SwapPhotos(x, y string) (v model.View) {
	defer a.guardEdit("SwapPhotos", &v, nil)
	return a.changed(a.editor().SwapPhotos(x, y))
}

func (a *App) RemovePhotos(ids []string) (v model.View) {
	defer a.guardEdit("RemovePhotos", &v, nil)
	return a.changed(a.editor().RemovePhotos(ids))
}

func (a *App) DeleteSpread(id string) (v model.View) {
	defer a.guardEdit("DeleteSpread", &v, nil)
	return a.changed(a.editor().DeleteSpread(id))
}

// DeleteSpreads удаляет выделенные развороты одним шагом отмены; неизвестные ID пропускаются.
func (a *App) DeleteSpreads(ids []string) (v model.View) {
	defer a.guardEdit("DeleteSpreads", &v, nil)
	return a.changed(a.editor().DeleteSpreads(ids))
}

func (a *App) MoveSpread(id string, to int) (v model.View) {
	defer a.guardEdit("MoveSpread", &v, nil)
	return a.changed(a.editor().MoveSpread(id, to))
}

func (a *App) SplitSpread(id string, at int) (v model.View) {
	defer a.guardEdit("SplitSpread", &v, nil)
	return a.changed(a.editor().SplitSpread(id, at))
}

// MergeSpreads: больше MaxSpreadPhotos фото или MaxTexts текстов вместе — ничего не меняет.
func (a *App) MergeSpreads(id string) (v model.View) {
	defer a.guardEdit("MergeSpreads", &v, nil)
	return a.changed(a.editor().MergeSpreads(id))
}

// SetLayout ставит раскладку из галереи; битый или уже стоящий ключ — ничего не меняет.
func (a *App) SetLayout(spreadID, key string) (v model.View) {
	defer a.guardEdit("SetLayout", &v, nil)
	return a.changed(a.editor().SetLayout(spreadID, key))
}

// StepLayout листает раскладки: scope "spread" | "left" | "right".
func (a *App) StepLayout(spreadID, scope string, delta int) (v model.View) {
	defer a.guardEdit("StepLayout", &v, nil)
	return a.changed(a.editor().StepLayout(spreadID, scope, delta))
}

// LayoutChoices — варианты для галереи. Только чтение: без правки и без события.
func (a *App) LayoutChoices(spreadID string) model.Choices {
	defer guard("LayoutChoices")
	return a.editor().LayoutChoices(spreadID)
}

func (a *App) SetSettings(s model.Settings) (v model.View) {
	defer a.guardEdit("SetSettings", &v, nil)
	return a.changed(a.editor().SetSettings(s))
}

// AutoBuild: сверх MaxSpreads разворотов — ошибка, проект не меняется.
func (a *App) AutoBuild(ids []string, at, perSpread int) (v model.View, err error) {
	defer a.guardEdit("AutoBuild", &v, &err)
	return a.changedOrErr(a.editor().AutoBuild(a.infos(ids), at, perSpread))
}

func (a *App) Undo() model.View { return a.step((*album.Editor).Undo) }
func (a *App) Redo() model.View { return a.step((*album.Editor).Redo) }

// step выполняет отмену или возврат; если при этом поменялся список папок — перечитывает библиотеку.
func (a *App) step(do func(*album.Editor) model.View) (v model.View) {
	defer a.guardEdit("Undo/Redo", &v, nil)
	ed := a.editor()
	before := ed.Folders()
	v = a.changed(do(ed))
	if !slices.Equal(before, v.Folders) {
		a.rescan()
	} else {
		a.refreshMissing() // отмена перепривязки меняет пути, а не папки
	}
	return v
}

// Свои отметки фото (спека 004). Путь и отметки из файла — из реестра сканера по ID, не от UI:
// Editor.SetMarks путь не проверяет, а чужой (например, UNC) попал бы в .salbum.

// SetMarks ставит свои отметки фото ленты; ID, которых нет в ленте, пропускаются.
// Ошибка — сверх MaxMarks своих отметок; отметки тогда прежние.
func (a *App) SetMarks(ids []string, patch model.MarkPatch) (v model.View, err error) {
	defer a.guardEdit("SetMarks", &v, &err)
	return a.changedOrErr(a.editor().SetMarks(a.infos(ids), patch))
}

// ResetMarks — «Как в Lightroom»: у этих фото снова действуют отметки из файлов.
func (a *App) ResetMarks(ids []string) (v model.View) {
	defer a.guardEdit("ResetMarks", &v, nil)
	return a.changed(a.editor().ResetMarks(ids))
}

// Marks — свои отметки проекта (ID → заданные поля); UI перечитывает при смене View.marksRev и после state.
func (a *App) Marks() map[string]model.MarkView {
	defer guard("Marks")
	return a.editor().Marks()
}
