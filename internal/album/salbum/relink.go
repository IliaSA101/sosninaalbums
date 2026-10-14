package salbum

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Refs — тело Editor.Refs над проектом: глубокая копия (PhotoRef.Copy), указатели внутрь
// проекта наружу не уходят.
func Refs(p *model.Project) map[string]model.PhotoRef {
	refs := make(map[string]model.PhotoRef, len(p.Photos))
	for id, r := range p.Photos {
		refs[id] = r.Copy()
	}
	return refs
}

// Relink — тело Editor.Relink над проектом: переехавшие файлы и папки (для своих отметок).
func Relink(p *model.Project, moves map[string]string, addFolder string) (files, dirs []model.PathPair) {
	files = relink(p, moves)
	if len(files) == 0 {
		return files, nil
	}
	dirs = moveFolders(p, files)
	addFolderFor(p, files, addFolder)
	return files, dirs
}

// RelinkDrive — тело Editor.RelinkDrive над проектом: переехавшие файлы и папки.
func RelinkDrive(p *model.Project, from, to string) (files, dirs []model.PathPair) {
	moves := map[string]string{}
	for id, r := range p.Photos {
		if _, here := fileSize(r.Path); here {
			continue
		}
		if np, ok := paths.SwapDrive(r.Path, from, to); ok {
			if _, there := fileSize(np); there {
				moves[id] = np
			}
		}
	}
	files = relink(p, moves)
	for i, f := range p.Folders {
		if isDir(f) {
			continue
		}
		if np, ok := paths.SwapDrive(f, from, to); ok && isDir(np) {
			dirs = append(dirs, model.PathPair{Old: f, New: np})
			setFolder(p, i, np)
		}
	}
	p.Folders = uniquePaths(p.Folders)
	relinkGuide(p, from, to)
	return files, dirs
}

// relink меняет пути фото из moves и пересчитывает ID. Пропускает: неизвестный ID,
// нелокальный путь, путь, уже занятый другим фото (повтор файла в альбоме).
func relink(p *model.Project, moves map[string]string) []model.PathPair {
	used := make(map[string]string, len(p.Photos))
	for id, r := range p.Photos {
		used[strings.ToLower(filepath.Clean(r.Path))] = id
	}
	placed := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			placed[id] = true
		}
	}
	var pairs []model.PathPair
	for _, id := range slices.Sorted(maps.Keys(moves)) {
		ref, ok := p.Photos[id]
		if !ok || paths.CheckLocalPath(moves[id]) != nil {
			continue
		}
		np := filepath.Clean(moves[id])
		key := strings.ToLower(np)
		if other, taken := used[key]; taken && other != id {
			continue
		}
		pairs = append(pairs, model.PathPair{Old: ref.Path, New: np, BgOnly: !placed[id]})
		delete(used, strings.ToLower(filepath.Clean(ref.Path)))
		used[key] = id
		ref.Path, ref.Alt = np, ""
		// Файл на месте, но другой длины (перепривязали «по W×H»): размеры — из заголовка.
		if size, ok := fileSize(np); ok {
			refresh(&ref, size)
		}
		p.Photos[id] = ref
	}
	if len(pairs) > 0 {
		Rekey(p, library.IDFor)
	}
	return pairs
}

// moveFolders применяет к папкам самую частую пару префиксов из pairs; уходят
// только папки, которых нет на месте, а новая должна существовать. При равенстве берётся меньшая по тексту (результат
// не зависит от порядка обхода карты). Возвращает переехавшие папки (для своих отметок).
func moveFolders(p *model.Project, pairs []model.PathPair) []model.PathPair {
	type pre struct{ from, to string }
	count := map[pre]int{}
	for _, pr := range pairs {
		if pr.BgOnly {
			continue
		}
		if from, to, ok := paths.PrefixMap(pr.Old, pr.New); ok {
			count[pre{from, to}]++
		}
	}
	var best pre
	n := 0
	for k, c := range count {
		if c > n || c == n && (k.from < best.from || k.from == best.from && k.to < best.to) {
			best, n = k, c
		}
	}
	if n == 0 {
		return nil
	}
	var moved []model.PathPair
	for i, f := range p.Folders {
		if isDir(f) {
			continue
		}
		if np, ok := paths.ReplacePrefix(f, best.from, best.to); ok && isDir(np) {
			moved = append(moved, model.PathPair{Old: f, New: np})
			setFolder(p, i, np)
		}
	}
	p.Folders = uniquePaths(p.Folders)
	return moved
}

// addFolderFor добавляет выбранную папку, если ни одна из папок проекта не
// содержит перепривязанных фото: иначе лента их не увидит.
func addFolderFor(p *model.Project, pairs []model.PathPair, dir string) {
	if dir == "" || paths.CheckLocalPath(dir) != nil || len(p.Folders) >= model.MaxFolders {
		return
	}
	pairs = slices.DeleteFunc(slices.Clone(pairs), func(pr model.PathPair) bool { return pr.BgOnly })
	if len(pairs) == 0 {
		return
	}
	for _, pr := range pairs {
		if slices.ContainsFunc(p.Folders, func(f string) bool { return library.Under(pr.New, f) }) {
			return
		}
	}
	dir = filepath.Clean(dir)
	if !slices.ContainsFunc(p.Folders, func(f string) bool { return paths.SamePath(f, dir) }) {
		p.Folders = append(p.Folders, dir)
	}
}

// setFolder заменяет папку i; её запасной путь (FolderAlt) больше не нужен.
func setFolder(p *model.Project, i int, dir string) {
	delete(p.FolderAlt, p.Folders[i])
	p.Folders[i] = dir
}

// uniquePaths убирает повторы путей (без учёта регистра), порядок сохраняется.
func uniquePaths(list []string) []string {
	out := list[:0]
	for _, f := range list {
		if !slices.ContainsFunc(out, func(o string) bool { return paths.SamePath(o, f) }) {
			out = append(out, f)
		}
	}
	return out
}
