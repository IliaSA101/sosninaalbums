package app

import (
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

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

// MergeSpreads: больше MaxPhotos фото или MaxTexts текстов вместе — ничего не меняет.
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

// MoreLayouts — плитка «Ещё» галереи: ряд «Авто» (17+ фото) на порцию длиннее have. Только чтение.
func (a *App) MoreLayouts(spreadID string, have int) model.Choices {
	defer guard("MoreLayouts")
	return a.editor().MoreLayouts(spreadID, have)
}

func (a *App) SetSettings(s model.Settings) (v model.View) {
	defer a.guardEdit("SetSettings", &v, nil)
	return a.changed(a.editor().SetSettings(s))
}

// AutoPlan — подсказка окна автосборки на o.Spreads разворотов по не больше o.MaxPer фото
// (оба зажимаются; MaxPer — в 1…64; Variety не влияет). Только чтение. Фото нет — пустой
// план; альбом почти полон (MaxSpreads) — ошибка.
func (a *App) AutoPlan(ids []string, o model.AutoOpts) (pl model.AutoPlan, err error) {
	defer guardErr("AutoPlan", &err)
	return a.editor().AutoPlan(a.infos(ids), o)
}

// AutoBuild собирает ровно те развороты, что показал AutoPlan с теми же аргументами.
// Сверх MaxSpreads разворотов — ошибка, проект не меняется. Variety зажимается в 0…2.
func (a *App) AutoBuild(ids []string, at int, o model.AutoOpts) (v model.View, err error) {
	defer a.guardEdit("AutoBuild", &v, &err)
	o.Variety = o.Variety.Clamp()
	return a.changedOrErr(a.editor().AutoBuild(a.infos(ids), at, o))
}

// VaryLayouts — «Разнообразить раскладки» (041) для выделенных разворотов, одним шагом
// отмены; variety зажимается в 0…2. Нечего менять — холостая: rev тот же, UI покажет тост.
// Оценки из XMP берутся до замка Editor: замки не вкладываются.
func (a *App) VaryLayouts(spreadIDs []string, variety int) (v model.View, err error) {
	defer a.guardEdit("VaryLayouts", &v, &err)
	ed := a.editor()
	infos := a.infos(ed.PhotosOf(spreadIDs))
	return a.changed(ed.VaryLayouts(spreadIDs, model.Variety(variety).Clamp(), infos)), nil
}

func (a *App) Undo() model.View { return a.step((*album.Editor).Undo) }
func (a *App) Redo() model.View { return a.step((*album.Editor).Redo) }

// step выполняет отмену или возврат; если при этом поменялся список папок — перечитывает библиотеку.
func (a *App) step(do func(*album.Editor) model.View) (v model.View) {
	defer a.guardEdit("Undo/Redo", &v, nil)
	ed := a.editor()
	before := ed.Folders()
	v = a.changed(do(ed))
	// Недостающие при отмене перепривязки (меняет пути, а не папки) пересчитывает changed.
	if !slices.Equal(before, v.Folders) {
		a.rescan()
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

// Листы особого вида: виньетка (031) и обложка (032). Методы App над ними — здесь,
// чтобы в папке не плодить файлы по спекам (decisions 0.10 №63).

// NewVignetteSheet создаёт проект-лист w×h мм без корешка (031: 204×304 или 304×404):
// Settings.Sheet, лист — одна страница, экспорт — файл на лист. Размеры зажимаются
// как у страницы. Тип изделия — album: режим задаёт Settings.Sheet. На листе — виньетка
// по умолчанию (031 G5).
func (a *App) NewVignetteSheet(w, h float64) State {
	return a.startNew(freshEditor(album.NewVignetteSheet(w, h)))
}

// Обложка (032). Места, тексты, фоны и слои на ней — общие методы с spreadID "cover".

// dirCoverGuide — своя последняя папка у диалога «Разметка типографии».
const dirCoverGuide = "coverGuide"

// CoverPresets — пресеты типографий для списка «Типография» (статично).
func (a *App) CoverPresets() []model.CoverPreset { return model.CoverPresets() }

// CreateCover создаёт обложку: preset "" — пресет по умолчанию. photoID непуст — фото из
// библиотеки или альбома встаёт фоном лицевой (бросок на плитку «Обложка»), тем же шагом
// отмены; фото остаётся в своём развороте. Обложка уже есть или проект-лист — ошибка.
func (a *App) CreateCover(preset, photoID string) (v model.View, err error) {
	defer a.guardEdit("CreateCover", &v, &err)
	var ph *model.PhotoInfo
	if photoID != "" {
		infos := a.infosOrRefs([]string{photoID})
		if len(infos) == 0 {
			return a.editor().View(), errNoPhoto
		}
		ph = &infos[0]
	}
	return a.changedOrErr(a.editor().CreateCover(preset, ph))
}

// DeleteCover снимает обложку; фото её мест уходят из альбома, как при удалении разворота.
func (a *App) DeleteCover() (v model.View) {
	defer a.guardEdit("DeleteCover", &v, nil)
	return a.changed(a.editor().DeleteCover())
}

// ApplyCoverPreset — числа пресета типографии; сторонка пользователя остаётся.
func (a *App) ApplyCoverPreset(id string) (v model.View, err error) {
	defer a.guardEdit("ApplyCoverPreset", &v, &err)
	return a.changedOrErr(a.editor().ApplyCoverPreset(id))
}

// SetCoverSpec — свои числа обложки (зажим в Go); не совпали с пресетом — «Свои размеры».
func (a *App) SetCoverSpec(s model.CoverSpec) (v model.View, err error) {
	defer a.guardEdit("SetCoverSpec", &v, &err)
	return a.changedOrErr(a.editor().SetCoverSpec(s))
}

// SetCoverLayout — раскладка обложки: front | both | wrap.
func (a *App) SetCoverLayout(kind string) (v model.View, err error) {
	defer a.guardEdit("SetCoverLayout", &v, &err)
	return a.changedOrErr(a.editor().SetCoverLayout(kind))
}

// CoverSpineText — «Надпись на корешке» с плашкой; уже есть — без изменений.
func (a *App) CoverSpineText() (v model.View, err error) {
	defer a.guardEdit("CoverSpineText", &v, &err)
	return a.changedOrErr(a.editor().CoverSpineText())
}

// SetCoverSpineBg — цвет плашки корешка #rrggbb; "" — без плашки.
func (a *App) SetCoverSpineBg(color string) (v model.View) {
	defer a.guardEdit("SetCoverSpineBg", &v, nil)
	return a.changed(a.editor().SetCoverSpineBg(color))
}

// PickCoverGuide — «Выбрать файл…» разметки: диалог в Go, путь от интерфейса не принимается;
// файл проверяется так же строго, как свой фон. Отказ от диалога — вид без изменений.
func (a *App) PickCoverGuide() (v model.View, err error) {
	defer a.guardEdit("PickCoverGuide", &v, &err)
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Разметка типографии",
		DefaultDirectory: a.dialogDir(dirCoverGuide),
		Filters:          []wr.FileFilter{{DisplayName: "Изображение (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"}},
	})
	if err != nil || path == "" {
		return a.editor().View(), err
	}
	return a.setCoverGuide(path)
}

// setCoverGuide — проверка выбранного файла разметки и правка.
func (a *App) setCoverGuide(path string) (model.View, error) {
	ph, err := library.BackdropFile(path)
	if err != nil {
		return a.editor().View(), err
	}
	v, err := a.changedOrErr(a.editor().SetCoverGuide(ph.Path))
	if err == nil {
		a.rememberDir(dirCoverGuide, filepath.Dir(ph.Path))
	}
	return v, err
}

// ClearCoverGuide убирает разметку обложки.
func (a *App) ClearCoverGuide() (v model.View) {
	defer a.guardEdit("ClearCoverGuide", &v, nil)
	return a.changed(a.editor().ClearCoverGuide())
}

// maxGuideBytes — предел файла разметки для раздачи: WebView декодирует его сам.
const maxGuideBytes = 100 << 20

// serveCoverGuide отдаёт /media/cover-guide — файл-разметку обложки. Путь берётся только из
// проекта (из запроса его нет: ?r=<rev> нужен лишь сбросу кэша), заново проверяется так же, как
// при выборе (локальный путь, JPEG/PNG, обычный файл), том должен отвечать.
func (a *App) serveCoverGuide(w http.ResponseWriter, r *http.Request) {
	path := a.editor().CoverGuidePath()
	if path == "" || !reachable(path) {
		http.NotFound(w, r)
		return
	}
	ph, err := library.BackdropFile(path)
	if err != nil || ph.Size > maxGuideBytes {
		http.NotFound(w, r)
		return
	}
	ct := "image/jpeg"
	if strings.EqualFold(filepath.Ext(ph.Path), ".png") {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache") // файл на диске могут заменить под тем же путём
	http.ServeFile(w, r, ph.Path)               //nolint:gosec // ph.Path — путь разметки из проекта после BackdropFile, не из запроса
}

// Виньетка (031 G5): панель, лента и меню «Разворот».

// VignetteChoices — превью шаблонов виньетки для чисел и пропорции панели (без правки).
func (a *App) VignetteChoices(spreadID string, teachers, pupils int, ratio string) []model.VigChoice {
	defer guard("VignetteChoices")
	return a.editor().VignetteChoices(spreadID, teachers, pupils, ratio, a.templateList())
}

// SetVignette — «Сделать виньеткой» и «Применить»; Left — фото, ушедшие в ленту (тост).
func (a *App) SetVignette(spreadID string, v model.Vignette) (r GridResult, err error) {
	defer a.guardEdit("SetVignette", &r.View, &err)
	view, left, err := a.editor().SetVignette(spreadID, v, a.templateList())
	r.View, err = a.changedOrErr(view, err)
	r.Left = left
	return r, err
}

// PreviewVignette — предпросмотр окна «Настроить виньетку»: лист после «Применить» с теми же
// параметрами и полями и сколько фото уйдёт в ленту. Проект не меняется, события нет.
func (a *App) PreviewVignette(spreadID string, s model.VigSetup) (pr model.VigPreview, err error) {
	defer guardErr("PreviewVignette", &err)
	return a.editor().PreviewVignette(spreadID, s, a.templateList())
}

// SetupVignette — «Применить» окна: параметры и поля шапки одним шагом отмены; Left — тост.
func (a *App) SetupVignette(spreadID string, s model.VigSetup) (r GridResult, err error) {
	defer a.guardEdit("SetupVignette", &r.View, &err)
	view, left, err := a.editor().SetupVignette(spreadID, s, a.templateList())
	r.View, err = a.changedOrErr(view, err)
	r.Left = left
	return r, err
}

// ResetVignette — «Вернуть авто» на виньетке: места по плану текущего шаблона, фото и имена
// остаются; Left — фото, ушедшие в ленту. Один шаг отмены; нетронутая виньетка — без правки.
func (a *App) ResetVignette(spreadID string) (r GridResult, err error) {
	defer a.guardEdit("ResetVignette", &r.View, &err)
	view, left, err := a.editor().ResetVignette(spreadID)
	r.View, err = a.changedOrErr(view, err)
	r.Left = left
	return r, err
}

// AddVignette — «Виньетка» в колонке «+ Новый разворот» и меню: разворот по умолчанию на at.
func (a *App) AddVignette(at int) (v model.View, err error) {
	defer a.guardEdit("AddVignette", &v, &err)
	return a.changedOrErr(a.editor().AddVignette(at))
}

// ClearVignette — «Обычный разворот»: места и фото остаются.
func (a *App) ClearVignette(spreadID string) (v model.View) {
	defer a.guardEdit("ClearVignette", &v, nil)
	return a.changed(a.editor().ClearVignette(spreadID))
}

// SortVignette — «Сортировать по фамилии».
func (a *App) SortVignette(spreadID string) (v model.View) {
	defer a.guardEdit("SortVignette", &v, nil)
	return a.changed(a.editor().SortVignette(spreadID))
}
