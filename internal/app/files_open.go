package app

// Общий хвост открытия альбома: меню, «Последние» и ответ на вопрос агента (openPath) и тихое открытие (160,
// openQuiet) приходят сюда с уже разобранным проектом.

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// openParsed делает разобранный проект p (файл path) текущим альбомом: Editor, запись прежнего (retire), список
// «Последние», лента, шрифты папки (af) и событие состояния. from != nil — смена только пока текущий альбом
// всё ещё from (сверка под saveMu); иначе changed = true и ничего не тронуто: альбом человека не уводим.
func (a *App) openParsed(path string, p *model.Project, af albumFonts, from *album.Editor) (st State, changed bool, err error) {
	ed, err := album.NewEditor(p, path)
	if err != nil {
		return a.GetState(), false, err
	}
	a.keepOwnRef(ed, path)
	if !a.retireIf(from, ed, nil) {
		return a.GetState(), true, nil
	}
	_, uid := ed.Ident()
	a.shr.Settled(path, uid) // регистр пути записи — когда альбом уже текущий
	a.rememberProject(path)
	a.rescan()
	a.keepAlbumFonts(af) // тост и новые начертания — в ответе (State.Info, State.FreshFonts)
	return a.emitState(), false, nil
}

// keepOwnRef — привязка своей записи о ссылке в проект при открытии (167-24): запись пути есть, а
// её ref в файле нет (файл записан 0.37 или привязку отбросили) — Editor запомнит её без шага
// отмены, файл получит с ближайшей записью. Чужой путь (копия, шаблон) ничего не получает.
func (a *App) keepOwnRef(ed *album.Editor, path string) {
	_, uid := ed.Ident()
	ref := a.shr.Opened(path, uid)
	if ref != "" && !slices.Contains(ed.ShareRefs(), ref) {
		ed.KeepRefs(uid, ref)
	}
}

// NewFromTemplate — «Новый альбом из шаблона…» (167-11): вёрстка другого альбома как основа, но
// без его ссылки — новый отпечаток, привязок нет, файла нет (черновик). Диалог и разбор — как у
// «Открыть…» (UNC в путях фото, предел, шрифты папки); файл новее программы — окно «Альбом из новой
// версии», прежний альбом остаётся. Шаблон читается до записи текущего альбома — только чтение.
func (a *App) NewFromTemplate() (res State, err error) {
	defer guardErr("NewFromTemplate", &err)
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Новый альбом из шаблона",
		DefaultDirectory: a.dialogDir(prefs.DirProject),
		Filters:          []wr.FileFilter{{DisplayName: "Альбом (*" + projectExt + ")", Pattern: "*" + projectExt}},
	})
	if err != nil || path == "" {
		st := a.GetState()
		st.Cancelled = err == nil
		return st, err
	}
	return a.newFromTemplate(path)
}

// newFromTemplate — тело NewFromTemplate после диалога (тесты идут сюда).
func (a *App) newFromTemplate(path string) (State, error) {
	if !reachable(path) {
		return a.GetState(), fmt.Errorf("диск %s недоступен", filepath.VolumeName(path))
	}
	p, af, err := a.loadAlbum(path)
	if errors.Is(err, salbum.ErrNewer) {
		info := a.setNewer(path, false, err)
		st := a.GetState()
		st.Cancelled, st.Newer = true, &info
		return st, nil
	}
	if err != nil {
		return a.GetState(), openErr(err)
	}
	ed, err := album.NewTemplateEditor(p)
	if err != nil {
		return a.GetState(), err
	}
	st := a.startNew(ed)
	if a.editor() != ed { // отказ (прежний черновик не убран): альбом прежний, тост шрифтов не от него
		return st, nil
	}
	a.keepAlbumFonts(af)
	return a.emitState(), nil // тост и новые начертания — в ответе, как у openParsed
}
