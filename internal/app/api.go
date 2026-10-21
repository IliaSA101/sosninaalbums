package app

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/namesfile"
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
	// Холодные раскладки большого альбома — секунды (073): считаются до замка, с ходом open:progress.
	// View под замком берёт их из кэша — тот же снимок, но лента и события не стоят.
	a.warmEditor(a.editor())
	// View — под тем же замком, что photos/scan: снимок согласован с событиями library/scan,
	// а медленное чтение диска (prefsAndRecent) — уже после.
	a.mu.Lock()
	view := a.fullLocked(a.editor().View())
	photos, scan := a.photos, a.scan
	notice, info, fresh := a.startNotice, a.startInfo, a.startFresh
	a.startNotice, a.startInfo, a.startFresh = "", "", nil
	missing := append([]string{}, a.missing...)
	a.mu.Unlock()
	if photos == nil {
		photos = []library.Photo{}
	}
	p, recent := a.prefsAndRecent()
	return State{View: view, Photos: photos, Scan: scan, Notice: notice, Info: info, FreshFonts: fresh, Missing: missing,
		Version: appVersion(wailsJSON), Fonts: text.Families(), Prefs: p, Recent: recent, Keymap: a.Keymap()}
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
	if err != nil {
		return a.editor().View(), err
	}
	if dir == "" {
		return a.out(a.editor().View()), nil
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

// changedOrErr рассылает вид, только если правка прошла.
func (a *App) changedOrErr(v model.View, err error) (model.View, error) {
	if err != nil {
		return v, err
	}
	return a.changed(v), nil
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

// Разметка типографии обложки и разворотов (выбор файла, раздача) — markup.go.

// Виньетка (031 G5): панель, лента и меню «Разворот».

// SetVignette — «Сделать виньеткой» и «Применить»; Left — фото, ушедшие в ленту (тост).
func (a *App) SetVignette(spreadID string, v model.Vignette) (r GridResult, err error) {
	defer a.guardEdit("SetVignette", &r.View, &err)
	view, left, err := a.editor().SetVignette(spreadID, v, a.templateList())
	r.View, err = a.changedOrErr(view, err)
	r.Left = left
	return r, err
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

// Поля подписей (спека 030): переменные альбома, имя на фото, список имён, стиль листа.

// NamesResult — вид после вставки имён: Extra — лишних строк, Missing — фото листа без имени,
// Named из Seats — подписано мест (окно и тост: «Подписано: 23 из 25, лишних строк: 1»).
type NamesResult struct {
	View    model.View `json:"view"`
	Extra   int        `json:"extra"`
	Missing int        `json:"missing"`
	Named   int        `json:"named"`
	Seats   int        `json:"seats"`
}

// Person — имя фото для карточки: своё или из имени файла (fromFile). Фото ещё не в
// альбоме — из имени файла ленты.
func (a *App) Person(photoID string) model.PersonView {
	defer guard("Person")
	if pv, ok := a.editor().Person(photoID); ok {
		return pv
	}
	a.mu.Lock()
	ph, ok := a.byID[photoID]
	a.mu.Unlock()
	if !ok {
		return model.PersonView{}
	}
	return model.NameView(model.PhotoRef{Path: ph.Path})
}

// PlacePerson — табличка места placeID листа spreadID (071) для карточки «Место» и окошка «Имя»
// пустого места; teacher — место учителя виньетки. Места нет — пустой.
func (a *App) PlacePerson(spreadID, placeID string) model.PersonView {
	defer guard("PlacePerson")
	pv, _ := a.editor().PlacePerson(spreadID, placeID)
	return pv
}

// PasteNames — окно «Имена» (043): имена из списка (строка таблицы — человек; заголовки,
// учителя, столбец «Файл») или из названий фото (o.From = "files") на фото листа, один шаг
// отмены; повтор того же — без шага. Список больше 1000 строк — ошибка без правки.
func (a *App) PasteNames(spreadID string, o model.NamesOpts) (r NamesResult, err error) {
	defer a.guardEdit("PasteNames", &r.View, &err)
	v, plan, err := a.editor().PasteNames(spreadID, o)
	r.View, err = a.changedOrErr(v, err)
	r.Extra, r.Missing, r.Named, r.Seats = plan.Extra, plan.Missing, plan.Named, plan.Seats
	return r, err
}

// NamesFile — загруженная таблица: Text кладётся в поле списка, остальное — строка сведений
// «Класс.xlsx, лист «Класс» (листов: 2), строк: 26».
type NamesFile struct {
	Text      string `json:"text"`
	File      string `json:"file"`   // имя файла без пути
	Sheet     string `json:"sheet"`  // лист xlsx; "" — csv/txt
	Sheets    int    `json:"sheets"` // листов в книге; 0 — csv/txt
	Rows      int    `json:"rows"`   // непустых строк, с заголовком
	Cancelled bool   `json:"cancelled"`
}

// dirNames — своя последняя папка у диалога «Таблица с именами».
const dirNames = "names"

// PickNamesFile — «Загрузить таблицу…»: диалог в Go (путь от интерфейса не принимается),
// таблица (.xlsx, CSV, «Текст Юникод») становится текстом поля списка. Проект не меняется.
// Отказ от диалога — Cancelled без ошибки.
func (a *App) PickNamesFile() (f NamesFile, err error) {
	defer guardErr("PickNamesFile", &err)
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Таблица с именами",
		DefaultDirectory: a.dialogDir(dirNames),
		Filters:          []wr.FileFilter{{DisplayName: "Таблица (*.xlsx, *.csv, *.txt)", Pattern: "*.xlsx;*.csv;*.txt"}},
	})
	if err != nil || path == "" {
		return NamesFile{Cancelled: err == nil}, err // сбой диалога — не отмена: окно покажет ошибку
	}
	if !reachable(path) {
		// Отключённый сетевой диск: чтение ждало бы таймаута SMB.
		return NamesFile{}, fmt.Errorf("диск %s недоступен", filepath.VolumeName(path))
	}
	f0, err := namesfile.Read(path)
	if err != nil {
		return NamesFile{}, err
	}
	a.rememberDir(dirNames, filepath.Dir(path))
	return NamesFile{Text: f0.Text, File: f0.Name, Sheet: f0.Sheet, Sheets: f0.Sheets, Rows: f0.Rows}, nil
}

// Режим «Сетка» (спека 007, design §4.2): правки мест под guardEdit.

// GridResult — вид после правки и сколько фото не встало или ушло в ленту
// (интерфейс показывает тост с «Вернуть» = Undo). Named — только у PlacePhotos и FillPlaces.
type GridResult struct {
	View  model.View `json:"view"`
	Left  int        `json:"left"`
	Named []string   `json:"named"` // фото, получившие имя из названия файла этим шагом (060; тост «Убрать имена»)
}

// MarshalJSON — Named всегда список: правки сетки без броска его не задают, а UI не проверяет
// на null.
func (r GridResult) MarshalJSON() ([]byte, error) {
	type plain GridResult // без метода: иначе рекурсия
	q := plain(r)
	q.Named = orEmpty(q.Named)
	return json.Marshal(q)
}

// PlacePhotos — бросок фото (из ленты или альбома) на место.
func (a *App) PlacePhotos(spreadID, placeID string, ids []string) (r GridResult, err error) {
	defer a.guardEdit("PlacePhotos", &r.View, &err)
	v, left, named, err := a.editor().PlacePhotos(spreadID, placeID, a.infosOrRefs(ids))
	r.View, err = a.changedOrErr(v, err)
	r.Left, r.Named = left, named
	return r, err
}

// orEmpty — nil как пустой список: в JSON — [], а не null (UI не проверяет на null).
func orEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// DropResult — итог DropPhotos (042). Ask: правки не было (rev тот же) — UI задаёт вопрос
// и зовёт DropPhotos ещё раз с rest "add" | "skip".
type DropResult struct {
	View  model.View `json:"view"`
	Ask   bool       `json:"ask"`
	Fit   int        `json:"fit"`   // сколько встанет (встало) в имеющиеся места, с целевым
	Total int        `json:"total"` // сколько фото пришло на лист (без тех, что уже в его местах)
	Fresh []string   `json:"fresh"` // ID новых мест плитки: UI выделяет их, как после «Место»
	Grown int        `json:"grown"` // на сколько выросло «Учеников» нетронутой виньетки
	Out   bool       `json:"out"`   // прежнее фото целевого места ушло в ленту (замена)
	Left  int        `json:"left"`  // не встало: «Не добавлять» или обложка
	Small bool       `json:"small"` // место плитки мельче 1/10 меньшей стороны страницы
	Named []string   `json:"named"` // получили имя из названия файла этим шагом (060; тост «Убрать имена»)
}

// DropPhotos — бросок фото на лист spreadID (042): placeID "" — на лист, иначе на место.
// at — доля ширины листа под курсором 0…1, < 0 — точки нет. rest: "" — спросить, если
// 0 < Fit < Total; "add" — остальные плиткой (нетронутая виньетка — ученики); "skip" — не
// добавлять. «Авто» — как AddPhotos в конец; обложка — как PlacePhotos. Отказ — ошибка,
// проект прежний.
func (a *App) DropPhotos(spreadID, placeID string, ids []string, at float64, rest string) (r DropResult, err error) {
	defer a.guardEdit("DropPhotos", &r.View, &err)
	v, o, err := a.editor().DropPhotos(spreadID, placeID, a.infosOrRefs(ids), at, rest)
	if o.Ask && err == nil { // правки нет (v — нынешний вид): не рассылать, автосохранение не трогать
		r.View = a.out(v)
	} else if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	return DropResult{View: r.View, Ask: o.Ask, Fit: o.Fit, Total: o.Total, Fresh: orEmpty(o.Fresh), Grown: o.Grown,
		Out: o.Out, Left: o.Left, Small: o.Small, Named: orEmpty(o.Named)}, nil
}

// FillPlaces — «Заполнить пустые места» фото из ленты, начиная с разворота fromSpread.
func (a *App) FillPlaces(ids []string, fromSpread string) (r GridResult) {
	defer a.guardEdit("FillPlaces", &r.View, nil)
	v, left, named := a.editor().FillPlaces(a.infos(ids), fromSpread)
	return GridResult{View: a.changed(v), Left: left, Named: named}
}

// CopyGrid копирует места области from на цели; design — с текстом и фоном,
// mirror — на страницы другой стороны зеркально.
func (a *App) CopyGrid(spreadID, from string, to []model.GridTarget, design, mirror bool) (r GridResult, err error) {
	defer a.guardEdit("CopyGrid", &r.View, &err)
	v, left, err := a.editor().CopyGrid(spreadID, from, to, design, mirror)
	r.View, err = a.changedOrErr(v, err)
	r.Left = left
	return r, err
}

// CopyDecor — «Копировать оформление» (049 Ф2): фон, узор, фото фона и стикеры области from
// (texts — и надписи) на цели; места и фото не трогаются; mirror — стикеры на странице другой
// стороны отражаются. Left всегда 0: тип результата — как у CopyGrid (одно окно копирования).
func (a *App) CopyDecor(spreadID, from string, to []model.GridTarget, texts, mirror bool) (r GridResult, err error) {
	defer a.guardEdit("CopyDecor", &r.View, &err)
	v, err := a.editor().CopyDecor(spreadID, from, to, texts, mirror)
	r.View, err = a.changedOrErr(v, err)
	return r, err
}

// ApplyGrid ставит сетку шаблона или встроенную: boxes — доли области tplScope
// (page | spread) с углом места, scope — куда ставить (left | right | spread).
func (a *App) ApplyGrid(spreadID, scope, tplScope string, boxes []model.Box) (r GridResult, err error) {
	defer a.guardEdit("ApplyGrid", &r.View, &err)
	v, left, err := a.editor().ApplyGrid(spreadID, scope, tplScope, boxes)
	r.View, err = a.changedOrErr(v, err)
	r.Left = left
	return r, err
}

// infosOrRefs — сведения о фото из ленты, а для фото альбома, чьей папки нет в
// ленте, — из проекта (как MovePhoto).
func (a *App) infosOrRefs(ids []string) []model.PhotoInfo {
	known := a.infos(ids)
	byID := make(map[string]model.PhotoInfo, len(known))
	for _, p := range known {
		byID[p.ID] = p
	}
	out := make([]model.PhotoInfo, 0, len(ids))
	ed := a.editor()
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		} else if r, ok := ed.Ref(id); ok {
			out = append(out, model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H, Size: r.Size})
		}
	}
	return out
}

// loadTemplates читает templates.json при старте (хранилище — internal/prefs).
func (a *App) loadTemplates() {
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	a.tpls = prefs.LoadTemplates(a.cfgDir)
}

// templateList — копия текущего списка шаблонов.
func (a *App) templateList() []model.Template {
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	return slices.Clone(a.tpls.List())
}

// storeTemplates пишет новый список в templates.json и принимает его в память. Вызывать под tplMu.
func (a *App) storeTemplates(list []model.Template) error {
	return a.tpls.Store(a.cfgDir, list, a.rename)
}

// tplReady — шаблоны прочитаны с диска (не открылись при старте — перечитываются здесь), чтобы
// лимиты, поиск по ID и сверка импорта шли по настоящему списку. Вызывать под tplMu.
func (a *App) tplReady() error { return a.tpls.Ensure(a.cfgDir) }

// SaveGridTemplate сохраняет места области scope ("left"/"right"/"spread") разворота
// как шаблон (страницы или разворота). Пустое имя — «Шаблон N». Нет мест — ошибка.
func (a *App) SaveGridTemplate(spreadID, scope, name string) (err error) {
	defer guardErr("SaveGridTemplate", &err)
	boxes, tplScope, err := a.editor().PlaceBoxes(spreadID, scope)
	if err != nil {
		return err
	}
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if err := a.tplReady(); err != nil {
		return err
	}
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	if len(cur) >= model.MaxTemplates {
		return model.ErrTooManyTpl
	}
	list := append(slices.Clone(cur), model.Template{
		ID: model.NewID(), Name: model.CleanTemplateName(name, len(cur)+1), Scope: tplScope, Boxes: boxes,
	})
	return a.storeTemplates(list)
}

// SaveVignetteTemplate сохраняет виньетку листа (лист или разворот) как свой шаблон (038):
// места с ролями, шапку, пропорции и стиль подписей — без фото, имён и текста шапки. Пустое
// имя — «Шаблон N». Не виньетка, нет мест людей, слишком много шаблонов или файл больше 1 МБ —
// ошибка. Шаблон общий для всех альбомов этого компьютера.
func (a *App) SaveVignetteTemplate(spreadID, name string) (err error) {
	defer guardErr("SaveVignetteTemplate", &err)
	t, err := a.editor().VigTemplate(spreadID)
	if err != nil {
		return err
	}
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if err := a.tplReady(); err != nil {
		return err
	}
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	if len(cur) >= model.MaxTemplates {
		return model.ErrTooManyTpl
	}
	t.ID, t.Name = model.NewID(), model.CleanTemplateName(name, len(cur)+1)
	return a.storeTemplates(append(slices.Clone(cur), t))
}

// RenameGridTemplate меняет имя шаблона (пустое — «Шаблон N»). id — как у карточки: у шаблона
// виньетки с префиксом «my:» (VigChoice.id).
func (a *App) RenameGridTemplate(id, name string) (err error) {
	defer guardErr("RenameGridTemplate", &err)
	err = a.editTemplates(id, func(list []model.Template, i int) []model.Template {
		list[i].Name = model.CleanTemplateName(name, i+1)
		return list
	})
	if err == nil {
		a.syncOwnNames()
	}
	return err
}

// DeleteGridTemplate удаляет шаблон (id — как у RenameGridTemplate); вопрос «удалить?» задаёт
// интерфейс. Листы, где шаблон виньетки применён, остаются: геометрия не меняется (в Vig.Own),
// имя — последнее из файла.
func (a *App) DeleteGridTemplate(id string) (err error) {
	defer guardErr("DeleteGridTemplate", &err)
	// Имя копии на листах — последнее из файла: после удаления его больше неоткуда взять
	// (альбом мог быть закрыт, когда шаблон переименовали).
	a.syncOwnNames()
	return a.editTemplates(id, func(list []model.Template, i int) []model.Template {
		return slices.Delete(list, i, i+1)
	})
}

// syncOwnNames — имена своих шаблонов виньетки на листах открытого альбома — по файлу шаблонов,
// без шага отмены (Editor.SyncOwnNames); сменились — вид рассылается, альбом сохранится сам.
func (a *App) syncOwnNames() {
	if v, changed := a.editor().SyncOwnNames(a.templateList()); changed {
		a.changed(v)
	}
}

// editTemplates применяет fn к копии списка (i — индекс шаблона id) и записывает результат.
func (a *App) editTemplates(id string, fn func(list []model.Template, i int) []model.Template) error {
	id = strings.TrimPrefix(id, model.VigOwnPrefix)
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if err := a.tplReady(); err != nil {
		return err
	}
	if a.tpls.Newer() {
		return model.ErrTemplatesNewer
	}
	cur := a.tpls.List()
	i := slices.IndexFunc(cur, func(t model.Template) bool { return t.ID == id })
	if i < 0 {
		return model.ErrTplNotFound
	}
	return a.storeTemplates(fn(slices.Clone(cur), i))
}
