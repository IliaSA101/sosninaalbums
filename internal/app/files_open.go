package app

// Общий хвост открытия альбома: меню, «Последние» и ответ на вопрос агента (openPath) и тихое открытие (160,
// openQuiet) приходят сюда с уже разобранным проектом.

import (
	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// openParsed делает разобранный проект p (файл path) текущим альбомом: Editor, запись прежнего (retire), список
// «Последние», лента, шрифты папки (af) и событие состояния. from != nil — смена только пока текущий альбом
// всё ещё from (сверка под saveMu); иначе changed = true и ничего не тронуто: альбом человека не уводим.
func (a *App) openParsed(path string, p *model.Project, af albumFonts, from *album.Editor) (st State, changed bool, err error) {
	ed, err := album.NewEditor(p, path)
	if err != nil {
		return a.GetState(), false, err
	}
	if !a.retireIf(from, ed, nil) {
		return a.GetState(), true, nil
	}
	a.rememberProject(path)
	a.rescan()
	a.keepAlbumFonts(af) // тост и новые начертания — в ответе (State.Info, State.FreshFonts)
	return a.emitState(), false, nil
}
