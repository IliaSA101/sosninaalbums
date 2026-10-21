package spreads

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// AddLoose — операция над проектом (097 §4): files — новое добавление отдельных фото со временем
// at. Файлы уже проверены App (локальные обычные JPEG/PNG); путь проверяется и здесь (защита в
// глубину, как AddFolder). Внутри папки списка — InFolder, уже отдельное или повтор в списке —
// Dup, сверх MaxLooseFiles — Over; остальные — одним добавлением в конец. Нечего добавить —
// проект не трогается (холостая правка).
func AddLoose(p *model.Project, files []string, at int64) (model.LooseAdded, error) {
	var r model.LooseAdded
	for _, f := range files {
		if err := paths.CheckLocalPath(f); err != nil {
			return model.LooseAdded{}, err
		}
	}
	in := paths.NewCover(p.Folders, paths.Linked) // как FoldSources: куда дойдёт сканер
	seen := map[string]bool{}
	for _, f := range p.LooseFiles() {
		seen[model.LooseKey(f)] = true
	}
	add := model.LooseAdd{ID: model.NewID(), At: max(at, 0)}
	for _, f := range files {
		f = filepath.Clean(f)
		k := model.LooseKey(f)
		switch {
		case in.HasFile(f):
			if r.InFolder == 0 {
				r.Folder = filepath.Dir(f)
			}
			r.InFolder++
		case seen[k]:
			r.Dup++
		case len(seen) >= model.MaxLooseFiles:
			r.Over++
		default:
			seen[k] = true
			add.Files = append(add.Files, model.LooseFile{Path: f})
		}
	}
	if len(add.Files) == 0 {
		return r, nil
	}
	if len(p.Loose) >= model.MaxLooseAdds {
		return model.LooseAdded{}, fmt.Errorf("добавлений отдельных фото не больше %d", model.MaxLooseAdds)
	}
	p.Loose = append(p.Loose, add)
	r.Batch, r.Added = add.ID, len(add.Files)
	return r, nil
}

// RemoveLoose — операция над проектом: file == "" — всё добавление id, иначе один его файл
// (paths.SamePath); опустевшее добавление уходит. Неизвестное — холостая.
func RemoveLoose(p *model.Project, id, file string) {
	i := slices.IndexFunc(p.Loose, func(a model.LooseAdd) bool { return a.ID == id })
	if i < 0 {
		return
	}
	if file != "" {
		a := &p.Loose[i]
		a.Files = slices.DeleteFunc(a.Files, func(f model.LooseFile) bool { return paths.SamePath(f.Path, file) })
		if len(a.Files) > 0 {
			return
		}
	}
	p.Loose = slices.Delete(p.Loose, i, i+1)
	if len(p.Loose) == 0 {
		p.Loose = nil
	}
}
