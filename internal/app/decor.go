package app

import (
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Библиотека фонов (спека 048 Ф1, design §8): окно «Фоны», «Загрузить своё…», ✎ и ×
// своего, щелчок по плитке и правки узора. kind — "bg" | "sticker" (стикеры — Ф2) | "frame"
// (рамочки мест, 079).
// Пути к файлам библиотеки от интерфейса не принимаются: ID плитки → индекс decor.Store.

// Типы биндингов — копии decor.* (решение №178): namespace Wails = имя пакета.

// DecorItem — плитка каталога (decor.Item).
type DecorItem struct {
	ID     string  `json:"id"` // sa:<пресет> | my:<16 hex>
	Theme  string  `json:"theme"`
	Name   string  `json:"name"`
	URL    string  `json:"url"` // без px: UI дописывает &px=
	Own    bool    `json:"own"`
	Aspect float64 `json:"aspect"` // ширина/высота картинки; узор — 0
	W      int     `json:"w"`
	H      int     `json:"h"`
	Color  string  `json:"color,omitempty"` // цвет страниц пресета: подложка плитки
	Dark   bool    `json:"dark,omitempty"`  // светлый стикер (116): плитке — тёмная шашечка
	// Pages, Places — у разворота (067): 1 — страница, 2 — разворот; мест в шаблоне.
	Pages  int    `json:"pages,omitempty"`
	Places int    `json:"places,omitempty"`
	Pack   string `json:"pack,omitempty"` // название набора: подсказка «Набор «…»»
}

// DecorTheme — строка списка тем слева.
type DecorTheme struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DecorCatalog — содержимое окна одного вида.
type DecorCatalog struct {
	Kind     string       `json:"kind"`
	Themes   []DecorTheme `json:"themes"`
	Items    []DecorItem  `json:"items"`
	Recent   []string     `json:"recent"` // ID плиток, новые первыми
	ReadOnly bool         `json:"readOnly"`
}

// DecorSkip — пропущенный при загрузке файл и причина.
type DecorSkip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// DecorSvg — проверенный SVG стикера: окно рисует Data (base64) холстом W×H и присылает PNG
// в AddDecorPNG с Token (design §5.4).
type DecorSvg struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Data  string `json:"data"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// DecorImport — итог «Загрузить своё…»: добавленные (и возвращённые из скрытых), пропущенные
// и задания окну на SVG (только стикеры).
type DecorImport struct {
	Added   []DecorItem `json:"added"`
	Skipped []DecorSkip `json:"skipped"`
	Svg     []DecorSvg  `json:"svg"`
}

// newDecorImport — пустой итог: срезы не nil, интерфейс не проверяет null.
func newDecorImport() DecorImport {
	return DecorImport{Added: []DecorItem{}, Skipped: []DecorSkip{}, Svg: []DecorSvg{}}
}

// errNoLibrary — библиотека не открылась (startup не дошёл до неё).
var errNoLibrary = errors.New("библиотека фонов недоступна")

// Своя последняя папка у диалога «Загрузить своё…»: фоны, стикеры и рамочки обычно лежат в
// разных папках.
const (
	dirDecor   = "decor"
	dirSticker = "sticker"
	dirFrame   = "frame"
)

// openFilesDialog — нативный выбор нескольких файлов; в тестах подменяется.
var openFilesDialog = wr.OpenMultipleFilesDialog

// openDecor открывает библиотеку и ставит искатель своих для salbum — до первого Load (startup).
func (a *App) openDecor() {
	s := decor.Open(a.cfgDir)
	s.SetPrintCheck(ownprint.CheckData)
	a.decorLib.Store(s)
	salbum.SetLibFinder(s.Find)
	salbum.SetFrameFinder(s.FindFrame)
}

func (a *App) decorStore() (*decor.Store, error) {
	if s := a.decorLib.Load(); s != nil {
		return s, nil
	}
	return nil, errNoLibrary
}

// serveDecor — /media/gen/* и /media/lib/* (design §5.5); false — запрос не к библиотеке.
func (a *App) serveDecor(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, decor.GenPath) && !strings.HasPrefix(r.URL.Path, decor.LibPath) {
		return false
	}
	s := a.decorLib.Load()
	if s == nil {
		http.NotFound(w, r)
		return true
	}
	s.Handler(a.cache).ServeHTTP(w, r)
	return true
}

// DecorCatalog — плитки вида kind: встроенные, свои, недавние (config.json); плитки узоров —
// в пропорции разворота альбома.
func (a *App) DecorCatalog(kind string) (c DecorCatalog, err error) {
	defer guardErr("DecorCatalog", &err)
	s, err := a.decorStore()
	if err != nil {
		return DecorCatalog{Kind: kind, Themes: []DecorTheme{}, Items: []DecorItem{}, Recent: []string{}}, err
	}
	w, h := a.editor().SheetSize()
	cat, err := s.Catalog(kind, prefs.RecentDecor(a.cfgDir, kind), w, h)
	if err != nil {
		return DecorCatalog{Kind: kind, Themes: []DecorTheme{}, Items: []DecorItem{}, Recent: []string{}}, err
	}
	c = DecorCatalog{Kind: cat.Kind, Themes: make([]DecorTheme, len(cat.Themes)), Items: decorItems(cat.Items),
		Recent: cat.Recent, ReadOnly: cat.ReadOnly}
	for i, t := range cat.Themes {
		c.Themes[i] = DecorTheme(t)
	}
	return c, nil
}

func decorItems(in []decor.Item) []DecorItem {
	out := make([]DecorItem, len(in))
	for i, it := range in {
		out[i] = DecorItem(it)
	}
	return out
}

// ImportDecor — «Загрузить своё…»: диалог в Go, как PickBackdropFile. Отказ от диалога —
// пустой итог без ошибки. Добавленное сразу находит свои пропавшие в альбоме (RelinkLib).
// SVG стикеров приходят заданиями Svg: запись появится после AddDecorPNG.
func (a *App) ImportDecor(kind, theme string) (res DecorImport, err error) {
	defer guardErr("ImportDecor", &err)
	res = newDecorImport()
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	// Отказ — до диалога: выбранные файлы иначе пропали бы зря.
	if kind != decor.KindBg && kind != decor.KindSticker && kind != decor.KindFrame {
		return res, decor.ErrKind
	}
	if s.ReadOnly() {
		return res, decor.ErrReadOnly
	}
	opts := wr.OpenDialogOptions{
		Title:            "Свои фоны",
		DefaultDirectory: a.dialogDir(dirDecor),
		Filters:          []wr.FileFilter{{DisplayName: "Картинки (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"}},
	}
	dir := dirDecor
	switch kind {
	case decor.KindSticker:
		dir = dirSticker
		opts.Title, opts.DefaultDirectory = "Свои стикеры", a.dialogDir(dirSticker)
		opts.Filters = []wr.FileFilter{{DisplayName: "Картинки (*.png, *.jpg, *.jpeg, *.svg)", Pattern: "*.png;*.jpg;*.jpeg;*.svg"}}
	case decor.KindFrame: // рамочке нужна прозрачность окна: JPEG не предлагаем (импорт его всё равно отклонит)
		dir = dirFrame
		opts.Title, opts.DefaultDirectory = "Свои рамочки", a.dialogDir(dirFrame)
		opts.Filters = []wr.FileFilter{{DisplayName: "Картинки (*.png, *.svg)", Pattern: "*.png;*.svg"}}
	}
	files, err := openFilesDialog(a.ctx, opts)
	if err != nil || len(files) == 0 {
		return res, err
	}
	res, err = a.ImportDecorPaths(kind, theme, files)
	if err == nil {
		a.rememberDir(dir, filepath.Dir(files[0]))
	}
	return res, err
}

// ImportDecorPaths — загрузка выбранных файлов (как AddFolderPath у ChooseFolder: путь для
// проверки из браузера). Каждый файл проверяется строго, как «Свой файл…» (decor.Store.Import).
func (a *App) ImportDecorPaths(kind, theme string, files []string) (res DecorImport, err error) {
	defer guardErr("ImportDecorPaths", &err)
	res = newDecorImport()
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	r, err := s.Import(kind, theme, files)
	if err != nil {
		return res, err
	}
	res.Added = decorItems(r.Added)
	for _, k := range r.Skipped {
		res.Skipped = append(res.Skipped, DecorSkip(k))
	}
	for _, j := range r.Svg {
		res.Svg = append(res.Svg, DecorSvg(j))
	}
	a.relinkLib(s)
	return res, nil
}

// AddDecorPNG — PNG, нарисованный окном из проверенного SVG (base64): запись в библиотеку под
// ключом SVG по одноразовому токену задания. Как загрузка — находит свои пропавшие.
func (a *App) AddDecorPNG(token, data string) (it DecorItem, err error) {
	defer guardErr("AddDecorPNG", &err)
	s, err := a.decorStore()
	if err != nil {
		return it, err
	}
	got, err := s.AddPNG(token, data)
	if err != nil {
		return it, err
	}
	a.relinkLib(s)
	return DecorItem(got), nil
}

// relinkLib — после любой удачной загрузки, а не только с новыми: тот же файл «уже есть», а своё
// альбома без пути (отменили прежний RelinkLib, альбом с чужого компьютера) — находится само;
// нечего привязать — холостая правка без шага. Своё, чей файл стёрли и загрузили заново, путь
// не меняет — его снимает с пропавших полный пересчёт.
func (a *App) relinkLib(s *decor.Store) {
	ed := a.editor()
	a.changed(ed.RelinkLib(s.Find))
	refs := ed.Refs()
	a.mu.Lock()
	own := slices.ContainsFunc(a.missing, func(id string) bool { return refs[id].Lib != "" })
	a.mu.Unlock()
	if own {
		a.refreshMissing()
	}
}

// RenameDecor — ✎ своего: новое имя для показа. Встроенные не меняются.
func (a *App) RenameDecor(kind, id, name string) (err error) {
	defer guardErr("RenameDecor", &err)
	s, err := a.decorStore()
	if err != nil {
		return err
	}
	return s.Rename(kind, id, name)
}

// HideDecor — × своего: плитка скрывается, файл остаётся — альбомы, где он стоит, не меняются
// (решение №80), поэтому RelinkLib не нужен.
func (a *App) HideDecor(kind, id string) (err error) {
	defer guardErr("HideDecor", &err)
	s, err := a.decorStore()
	if err != nil {
		return err
	}
	return s.Hide(kind, id)
}

// ApplyBackground — щелчок по плитке фона: встроенный (sa:) — узор и цвет страниц пресета,
// фото области снимается; свой (my:) — фото фона по ключу, узор области снимается. Один шаг
// отмены; плитка — в недавние. Цвет у встроенных задан всегда (решение №170: пустой — не трогать).
func (a *App) ApplyBackground(spreadID, scope, itemID string) (v model.View, err error) {
	defer a.guardEdit("ApplyBackground", &v, &err)
	ed := a.editor()
	if !model.ValidBdScope(scope) {
		return ed.View(), errBackdropScope
	}
	s, err := a.decorStore()
	if err != nil {
		return ed.View(), err
	}
	bg, err := s.Background(itemID)
	if err != nil {
		return ed.View(), err
	}
	if bg.Pattern != nil && !model.ValidScope(scope) { // на корешок — только своё фото (061)
		return ed.View(), errBackdropScope
	}
	if bg.Pattern != nil {
		v, err = ed.ApplyPattern(spreadID, scope, *bg.Pattern, bg.Color)
	} else {
		v, err = ed.ApplyBackdrop(spreadID, scope, bg.Photo)
	}
	if err != nil {
		return v, err
	}
	prefs.RememberDecor(a.cfgDir, decor.KindBg, itemID)
	return a.changed(v), nil
}

// SetPattern — цвета, масштаб, seed, прозрачность узора области (ключ генератора не меняется,
// решение №172); серия с одним merge — один шаг отмены.
func (a *App) SetPattern(spreadID, scope string, p model.Pattern, merge string) (v model.View) {
	defer a.guardEdit("SetPattern", &v, nil)
	return a.changed(a.editor().SetPattern(spreadID, scope, p, merge))
}

// ClearPattern — убрать узор области.
func (a *App) ClearPattern(spreadID, scope string) (v model.View) {
	defer a.guardEdit("ClearPattern", &v, nil)
	return a.changed(a.editor().ClearPattern(spreadID, scope))
}

// BgResult — итог ApplyBackgroundTo: вид и сколько листов изменилось (тост «Фон поставлен на
// N разворотов»; 0 — у всех уже этот фон, шага нет).
type BgResult struct {
	View    model.View `json:"view"`
	Spreads int        `json:"spreads"`
}

// ApplyBackgroundTo — щелчок по плитке «Фоны» с набором листов (077): тот же фон на области
// targets одним шагом отмены; обложка не цель, пустые развороты — цель, стоящий фон заменяется,
// как у ApplyBackground. Плитка — в недавние один раз.
func (a *App) ApplyBackgroundTo(targets []model.GridTarget, itemID string) (r BgResult, err error) {
	defer a.guardEdit("ApplyBackgroundTo", &r.View, &err)
	ed := a.editor()
	src, err := a.bgSource(targets, itemID)
	if err != nil {
		return BgResult{View: ed.View()}, err
	}
	v, n, err := ed.ApplyBackgroundTo(targets, src)
	if err != nil {
		return BgResult{View: v}, err
	}
	prefs.RememberDecor(a.cfgDir, decor.KindBg, itemID)
	return BgResult{View: a.changed(v), Spreads: n}, nil
}

// PlanBackgroundTo — вопрос до ApplyBackgroundTo: сколько листов изменится и сколько фото фона
// заменится (кадр и поворот пропадут); без правки.
func (a *App) PlanBackgroundTo(targets []model.GridTarget, itemID string) (plan model.BgPlan, err error) {
	defer guardErr("PlanBackgroundTo", &err)
	src, err := a.bgSource(targets, itemID)
	if err != nil {
		return plan, err
	}
	return a.editor().PlanBackgroundTo(targets, src)
}

// bgSource — источник плитки для фона на набор: область каждой цели — только left/right/spread
// (корешок — у обложки, а она не цель), плитка — из библиотеки.
func (a *App) bgSource(targets []model.GridTarget, itemID string) (model.BgSource, error) {
	if slices.ContainsFunc(targets, func(t model.GridTarget) bool { return !model.ValidScope(t.Scope) }) {
		return model.BgSource{}, errBackdropScope
	}
	s, err := a.decorStore()
	if err != nil {
		return model.BgSource{}, err
	}
	bg, err := s.Background(itemID)
	if err != nil {
		return model.BgSource{}, err
	}
	return model.BgSource{Pattern: bg.Pattern, Color: bg.Color, Photo: bg.Photo}, nil
}

// SetAlbumBg — щелчок по плитке окна «Фоны» в режиме «Фон альбома» (098 §3): встроенный узор или
// своё фото на все развороты без своего фона — на весь разворот или на каждую страницу (pages).
// Цвет страниц пресета не ставится: цвет альбома красит и обложку. Один шаг отмены; плитка — в
// недавние; тот же фон — холостая без шага.
func (a *App) SetAlbumBg(itemID string, pages bool) (v model.View, err error) {
	defer a.guardEdit("SetAlbumBg", &v, &err)
	return a.setAlbumBg(itemID, pages, false)
}

// SetAlbumBgToAll — то же, что SetAlbumBg, и «Как в альбоме» у разворотов со своим фоном (115): их
// узоры, фото фона и «Не показывать здесь» сняты тем же шагом отмены; «скрыт» без своего фона и
// обложка остаются. Ошибки и недавние — как у SetAlbumBg; тот же фон при своих фонах — шаг из снятия.
func (a *App) SetAlbumBgToAll(itemID string, pages bool) (v model.View, err error) {
	defer a.guardEdit("SetAlbumBgToAll", &v, &err)
	return a.setAlbumBg(itemID, pages, true)
}

// setAlbumBg — общее тело SetAlbumBg и SetAlbumBgToAll; all — снять свои фоны разворотов.
func (a *App) setAlbumBg(itemID string, pages, all bool) (model.View, error) {
	ed := a.editor()
	s, err := a.decorStore()
	if err != nil {
		return ed.View(), err
	}
	bg, err := s.Background(itemID)
	if err != nil {
		return ed.View(), err
	}
	set := ed.SetAlbumBg
	if all {
		set = ed.SetAlbumBgToAll
	}
	v, err := set(model.BgSource{Pattern: bg.Pattern, Photo: bg.Photo}, pages)
	if err != nil {
		return v, err
	}
	prefs.RememberDecor(a.cfgDir, decor.KindBg, itemID)
	return a.changed(v), nil
}

// StickerResult — итог AddSticker: вид и ID нового стикера (интерфейс его выделяет).
type StickerResult struct {
	View model.View `json:"view"`
	ID   string     `json:"id"`
}

// AddSticker — щелчок по плитке стикера: по центру страницы page листа, ширина ¼ страницы, не
// выше ½. Встроенный (sa:st-…) — рисунок генератора, свой (my:) — фото по ключу из индекса
// библиотеки. Один шаг отмены; плитка — в недавние. Нет листа — холостая с пустым ID.
func (a *App) AddSticker(spreadID string, page int, itemID string) (r StickerResult, err error) {
	defer a.guardEdit("AddSticker", &r.View, &err)
	ed := a.editor()
	src, err := a.stickerSrc(itemID)
	if err != nil {
		return StickerResult{View: ed.View()}, err
	}
	// Пустой ID — стикер не встал (неизвестный лист): выделять нечего, в недавние не кладём.
	v, id, err := ed.AddSticker(spreadID, page, src)
	if err != nil {
		return StickerResult{View: v}, err
	}
	if id == "" {
		return StickerResult{View: a.out(v)}, nil
	}
	prefs.RememberDecor(a.cfgDir, decor.KindSticker, itemID)
	return StickerResult{View: a.changed(v), ID: id}, nil
}

// ReplaceSticker — «Заменить…»: другой рисунок или своё; центр, ширина и поворот остаются,
// высота — по новой пропорции. Тот же — холостая без шага и без недавних. Плитка — в недавние
// только при настоящей правке, как у AddSticker.
func (a *App) ReplaceSticker(spreadID, id, itemID string) (v model.View, err error) {
	defer a.guardEdit("ReplaceSticker", &v, &err)
	ed := a.editor()
	src, err := a.stickerSrc(itemID)
	if err != nil {
		return ed.View(), err
	}
	v, changed, err := ed.ReplaceSticker(spreadID, id, src)
	if err != nil {
		return v, err
	}
	if !changed {
		return a.out(v), nil
	}
	prefs.RememberDecor(a.cfgDir, decor.KindSticker, itemID)
	return a.changed(v), nil
}

// stickerSrc — плитка стикера → источник правки: встроенный — Art из параметров генератора
// (пропорцию рамки считает Editor), свой — фото с ключом библиотеки.
func (a *App) stickerSrc(itemID string) (model.StickerSrc, error) {
	s, err := a.decorStore()
	if err != nil {
		return model.StickerSrc{}, err
	}
	st, err := s.Sticker(itemID)
	if err != nil {
		return model.StickerSrc{}, err
	}
	if st.Art != nil {
		return model.StickerSrc{Art: &model.Art{Kind: st.Art.Kind, Inks: st.Art.Inks, Seed: st.Art.Seed}}, nil
	}
	return model.StickerSrc{Photo: st.Photo}, nil
}

// ApplyBorder — щелчок по плитке рамочки (079 §8): встроенная (sa:fr-…) или своя (my:…) местам
// ids листа (ссылки мест или фото), "" — снять. Один шаг отмены (у «Авто» — с закреплением); та
// же рамочка — холостая без шага и без недавних, плитка — в недавние только при правке.
func (a *App) ApplyBorder(spreadID string, ids []string, itemID string) (v model.View, err error) {
	defer a.guardEdit("ApplyBorder", &v, &err)
	ed := a.editor()
	var src model.BorderSrc
	if itemID != "" {
		s, err := a.decorStore()
		if err != nil {
			return ed.View(), err
		}
		fr, err := s.Frame(itemID)
		if err != nil {
			return ed.View(), err
		}
		src = model.BorderSrc(fr) // те же поля: разойдутся — не соберётся
	}
	v, changed, err := ed.ApplyBorderStep(spreadID, ids, src)
	if err != nil {
		return v, err
	}
	if !changed {
		return a.out(v), nil
	}
	if itemID != "" {
		prefs.RememberDecor(a.cfgDir, decor.KindFrame, itemID)
	}
	return a.changed(v), nil
}

// Фото на фоне разворота или страницы (спека 009, design-backdrop §5, §7).
// scope: "spread" (весь разворот) | "left" | "right" | "spine" (корешок обложки, 061).

// dirBackdrop — своя последняя папка у диалога «Свой файл…».
const dirBackdrop = "backdrop"

// openFileDialog — нативный диалог выбора файла; в тестах подменяется.
var openFileDialog = wr.OpenFileDialog

// PickBackdropFile — «Свой файл…»: диалог выбора в Go, путь от интерфейса не принимается.
// Отказ от диалога — вид без изменений и без ошибки (фона не появилось).
func (a *App) PickBackdropFile(spreadID, scope string) (v model.View, err error) {
	defer a.guardEdit("PickBackdropFile", &v, &err)
	if !model.ValidBdScope(scope) {
		return a.editor().View(), errBackdropScope
	}
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Фото для фона",
		DefaultDirectory: a.dialogDir(dirBackdrop),
		Filters:          []wr.FileFilter{{DisplayName: "Фото (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"}},
	})
	if err != nil || path == "" {
		return a.editor().View(), err
	}
	v, err = a.setBackdropFile(spreadID, scope, path)
	if err == nil {
		a.rememberDir(dirBackdrop, filepath.Dir(path))
	}
	return v, err
}

// setBackdropFile проверяет выбранный файл так же строго, как фото папки, и ставит его на фон.
func (a *App) setBackdropFile(spreadID, scope, path string) (model.View, error) {
	if !model.ValidBdScope(scope) {
		return a.editor().View(), errBackdropScope
	}
	ph, err := library.BackdropFile(path)
	if err != nil {
		return a.editor().View(), err
	}
	return a.changedOrErr(a.editor().SetBackdrop(spreadID, scope, model.PhotoInfo{ID: ph.ID, Path: ph.Path, W: ph.W, H: ph.H, Size: ph.Size}))
}
