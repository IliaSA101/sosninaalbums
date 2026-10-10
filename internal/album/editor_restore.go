package album

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// Восстановление из резервной копии (спека 019 E).

// LoadBase — Load, только относительные пути считаются от base, а не от папки файла: копия
// лежит в папке копий, а фото — рядом с самим альбомом. base == "" — только абсолютные пути
// (черновик; относительный путь берёт запасной abs).
func LoadBase(file, base string) (*model.Project, error) {
	data, err := salbum.ReadLimited(file)
	if err != nil {
		return nil, err
	}
	return salbum.Decode(data, base)
}

// Restore заменяет альбом проектом q (из LoadBase) одним шагом отмены; совпало — холостая,
// не прошли пределы правки — ошибка, альбом прежний.
// Заменяется всё, что пишет encode (Product, Folders, FolderAlt, Settings, Spreads, Photos):
// это поля Project с JSON-тегами, и свои отметки (q.loaded; у копии до 004 их нет — после
// замены их нет и в альбоме, Ctrl+Z вернёт). from остаётся текущим — копия старой версии файла
// (Editor.backup) нужна для файла альбома, а не для копии. q проверяется заново, как в
// NewEditor (own): не прошёл — ошибка, альбом прежний.
func (e *Editor) Restore(q *model.Project) (model.View, error) {
	src, marks, err := own(q)
	if err != nil {
		return e.View(), err
	}
	return e.edit("", true, func(p *model.Project) error {
		from := p.From()
		*p = *src
		p.SetFrom(from)
		return nil
	}, func() []MarkChange { return e.replaceMarks(marks) })
}

// Черновик.

// WriteDraft — WriteCopy для черновика: если проект прочитан из файла старой версии (черновик
// от прежней программы), первая запись поверх него сначала копирует исходник в <путь>.v<N>.bak,
// как Save делает для обычного файла: откат на старую программу не теряет черновик.
// Копия уже есть или файл уже новой версии — копировать нечего (salbum.BackupOld).
func (e *Editor) WriteDraft(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.from != 0 && e.from < model.FormatVersion {
		if err := salbum.BackupOld(path, e.from); err != nil {
			return fmt.Errorf("копия черновика старой версии: %w", err)
		}
	}
	return salbum.Write(e.p, path, "", e.marks)
}
