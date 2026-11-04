package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Обёртки «Копировать и вставить» (099): тела — grid/clip.go и grid/paste.go. Буфер держит App
// (design §1): Editor отдаёт снимок и принимает его обратно, сам буфер не хранит.

// TakeClip — снимок выделенного на листе (Ctrl+C). Договор: только чтение под замком — без шага
// отмены и rev; false — нечего копировать (буфер App остаётся прежним).
func (e *Editor) TakeClip(sheet string, sel model.ClipSel) (c model.Clip, ok bool) {
	e.read(func(p *model.Project) { c, ok = grid.Take(p, sheet, sel) })
	return c, ok
}

// CutClip — снимок и удаление выделенного одним шагом отмены (Ctrl+X). Договор: до правки ничего;
// нечего вырезать — холостая, ok = false; отказ settle — откат без шага, ok = false.
func (e *Editor) CutClip(sheet string, sel model.ClipSel) (v model.View, c model.Clip, ok bool, err error) {
	v, err = e.try("", func(p *model.Project) error { c, ok = grid.Cut(p, sheet, sel); return nil })
	return v, c, ok && err == nil, err
}

// PasteClip — вставка снимка c на лист r.Sheet одним шагом отмены (Ctrl+V, «Вставить» меню).
// Договор: запрос чистится до правки (PasteReq.Clean: NaN/Inf — точки нет, Repeat — белый список);
// пустой снимок и неизвестный лист — холостая; out.Repeat — вопрос 098 без правки (шага и rev нет);
// пределы и копия фото — ошибка try, откат без шага, out пустой.
func (e *Editor) PasteClip(c *model.Clip, r model.PasteReq) (v model.View, out model.PasteOut, err error) {
	v, err = e.try("", func(p *model.Project) (err error) { out, err = grid.Paste(p, c, r.Clean(), clipTwins()); return err })
	return v, okOut(out, err), err
}

// DuplicateClip — дубль выделенного на том же листе со сдвигом 5 мм одним шагом (Ctrl+D); буфер
// App не трогается. Фото — копии сразу, без вопроса (0.23-q2). Договор: как у PasteClip.
func (e *Editor) DuplicateClip(sheet string, sel model.ClipSel) (v model.View, out model.PasteOut, err error) {
	v, err = e.try("", func(p *model.Project) (err error) { out, err = grid.Duplicate(p, sheet, sel, clipTwins()); return err })
	return v, okOut(out, err), err
}

// okOut — итог вставки только при удачной правке: отказ settle откатывает и то, что тело успело.
func okOut(out model.PasteOut, err error) model.PasteOut {
	if err != nil {
		return model.PasteOut{}
	}
	return out
}

// clipTwins — фото снимка, уже стоящие в местах альбома (design §5, контракт 098-24): стоит ли — по
// файлу (PlacedFile: любая запись семьи, фон и стикер не в счёт), «Поставить копию» — новая запись
// 098 по записи из снимка (AddCopy), «Перенести сюда» — убрать с мест все записи семьи.
func clipTwins() grid.Twins {
	return grid.Twins{Placed: (*model.Project).PlacedFile, Copy: model.AddCopy, Move: detachFile}
}

// detachFile — записи файла id (семья копий), стоящие в местах листов, уходят с мест: переносимое
// фото встаёт на новое место один раз, пустые места остаются (как у Detach).
func detachFile(p *model.Project, id string) {
	base := model.BaseID(id)
	var gone []string
	for _, s := range p.Sheets() {
		for _, x := range s.Photos {
			if model.BaseID(x) == base {
				gone = append(gone, x)
			}
		}
	}
	p.Detach(gone)
}
