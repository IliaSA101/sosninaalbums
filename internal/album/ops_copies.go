package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
)

// Копии фото (098 design §2): чтение для окна «Фото уже в альбоме» и для App. Постановка копий —
// запросом «<ID файла>-new» в прежних правках (model.ResolveCopies), отдельной правки нет.

// WhereUsed — «где стоит» каждое фото ids (ID ленты, запись или запрос копии): вхождения всех
// записей семьи, число мест и запись единственного места. Договор: только чтение под замком —
// без шага отмены и rev; не nil, по порядку ids.
func (e *Editor) WhereUsed(ids []string) (out []model.UsedAt) {
	e.read(func(p *model.Project) { out = view.WhereUsed(p, ids) })
	return out
}

// FileRef — сведения о файле id (запись, ID файла или запрос копии): запись семьи с наименьшим
// номером; копия без общих указателей. Нет ни одной записи семьи — ok = false.
func (e *Editor) FileRef(id string) (r model.PhotoRef, ok bool) {
	e.read(func(p *model.Project) {
		if fam := p.Family(model.FileOf(id)); len(fam) > 0 {
			r, ok = p.Photos[fam[0]].Copy(), true
		}
	})
	return r, ok
}
