package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// AlbumTplOf — «Сохранить альбом как макет…» (131 design §8): макет из разворотов o.Ids и обложки.
// Договор: только чтение под замком — проект, rev и отмена не меняются («Авто» закрепляется на
// копии). Отказ — ошибка без макета (пустой выбор, чужой id, больше MaxLookSteps разных шагов).
func (e *Editor) AlbumTplOf(o model.SaveLookOpts) (t model.AlbumTpl, err error) {
	e.read(func(p *model.Project) { t, err = grid.AlbumTplOf(p, o, e.privateLib()) })
	return t, err
}

// privateLib — какие свои стикеры не уходят в шаблон и макет (LookHost.Private, код ссылки клиента); nil — любые.
func (e *Editor) privateLib() func(string) bool {
	if h := e.lookHost.Load(); h != nil {
		return h.Private
	}
	return nil
}
