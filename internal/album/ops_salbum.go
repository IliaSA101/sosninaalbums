package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
)

// Обёртки области salbum (023): Relink и RelinkDrive — исключение из «тело ≤ 3 операторов»,
// отметки переезжают в том же шаге через e.edit и relinkMarks (design §2).

// Refs — глубокая копия сведений обо всех фото альбома (ID → PhotoRef): App проверяет,
// какие файлы пропали, и ищет их по имени.
// Договор: только чтение под замком, проверять нечего.
func (e *Editor) Refs() (refs map[string]model.PhotoRef) {
	e.read(func(p *model.Project) { refs = salbum.Refs(p) })
	return refs
}

// Relink — перепривязка пропавших фото: moves (ID → новый путь) меняет пути,
// затем ID пересчитываются по новым путям (rekey: лента узнаёт фото как
// использованные). Папки переезжают следом: по паре «старый → новый путь» каждого
// фото paths.PrefixMap отрезает общий хвост, самая частая пара применяется к папкам
// проекта. Если после этого ни одна папка не содержит новых путей, добавляется
// addFolder (выбранная пользователем). Одна правка — один шаг отмены; свои отметки
// переезжают по тем же парам путей в том же шаге.
// Договор: до правки ничего не проверяет — salbum.Relink сам пропускает неизвестный ID,
// нелокальный путь (paths.CheckLocalPath) и чужую или нелокальную addFolder.
func (e *Editor) Relink(moves map[string]string, addFolder string) model.View {
	var files, dirs []model.PathPair
	v, _ := e.edit("", false, func(p *model.Project) error {
		files, dirs = salbum.Relink(p, moves, addFolder)
		return nil
	}, func() []MarkChange { return e.relinkMarks(files, dirs) })
	return v
}

// RelinkDrive — смена буквы диска «from → to» (одна латинская буква) для всего,
// что на from не нашлось, а на to есть: пропавшие фото и папки. Одна правка вместе
// со своими отметками.
// Договор: до правки ничего не проверяет — буквы проверяет paths.SwapDrive в salbum.RelinkDrive.
func (e *Editor) RelinkDrive(from, to string) model.View {
	var files, dirs []model.PathPair
	v, _ := e.edit("", false, func(p *model.Project) error {
		files, dirs = salbum.RelinkDrive(p, from, to)
		return nil
	}, func() []MarkChange { return e.relinkMarks(files, dirs) })
	return v
}

// RelinkLib — свои фоны и стикеры, которые нашлись в библиотеке (App зовёт после загрузки):
// путь от искателя find (nil — поставленный salbum.SetLibFinder). Шаг отмены, как Relink;
// ничего нового — холостая.
// Договор: до правки ничего не проверяет — ключ и путь искателя проверяет salbum.RelinkLib.
func (e *Editor) RelinkLib(find salbum.LibFinder) model.View {
	return e.change(func(p *model.Project) { salbum.RelinkLib(p, find) })
}

// RefitFonts — набор своих шрифтов сменился (загрузка, откат окна; App зовёт после
// text.SetCustom): ширины «по тексту» и названия в Fonts пересчитывают FitTexts и SyncFonts
// конвейера, тело пустое. Шаг отмены, если что-то сдвинулось; иначе холостая (как RelinkLib).
func (e *Editor) RefitFonts() model.View {
	return e.change(func(*model.Project) {})
}

// Обёртка Editor над сборкой вида (internal/album/view). Вызывается под e.mu: View, edit,
// Undo/Redo и правки отметок берут замок раньше. Всё остальное чтение вида идёт через
// публичный Editor.View (editor.go).

// view — вид проекта с состоянием редактора (путь, dirty, глубина отмены, rev).
// Договор: e.mu уже взят вызывающим; проект только читается.
func (e *Editor) view() model.View {
	v := view.BuildView(e.p, e.state(false))
	markVigs(e.p, &v) // имя шаблона и «Ручная» виньеток: план мест знает Editor (038)
	v.Again = e.againAlive()
	// Подборки — всегда список, а не nil: шапка дельты (sameHead) сравнивает их DeepEqual.
	v.Bins = append([]model.Bin{}, e.bins...)
	v.SetSrc(e.id)
	return v
}

// printView — вид печати для экспорта (Snapshot): пустые поля — пусто, заглушек нет (030).
// Договор: как у view.
func (e *Editor) printView() model.View {
	v := view.BuildView(e.p, e.state(true))
	borderFiles(e.p, &v)
	return v
}

// borderFiles — рамочкам-картинкам вида печати файл печати BorderView.File (079-40): копия из
// «Рамочек», а не путь записи, общей с фоном той же картинки.
func borderFiles(p *model.Project, v *model.View) {
	sheets := make([]*model.SpreadView, 0, len(v.Spreads)+1)
	for i := range v.Spreads {
		sheets = append(sheets, &v.Spreads[i])
	}
	if v.Cover != nil {
		sheets = append(sheets, &v.Cover.Sheet)
	}
	for _, s := range sheets {
		for _, pv := range s.Places {
			if b := pv.Border; b != nil && b.Pic {
				b.File = salbum.FramePath(p.Photos[b.Photo])
			}
		}
	}
}

func (e *Editor) state(forPrint bool) view.State {
	st := view.State{Path: e.path, Dirty: e.dirty, CanUndo: len(e.undo) > 0,
		CanRedo: len(e.redo) > 0, Rev: e.rev, MarksRev: e.marksRev, Print: forPrint}
	if !forPrint {
		e.markupState(&st)
	}
	return st
}
