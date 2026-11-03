package salbum

import (
	"maps"
	"path/filepath"
	"slices"

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
// Отдельные фото (097 §5): запись следует за фото альбома, папка ради него не добавляется;
// пропавшие отдельные не из альбома из той же папки переезжают следом (их пары — в files).
func Relink(p *model.Project, moves map[string]string, addFolder string) (files, dirs []model.PathPair) {
	files = relink(p, moves)
	if len(files) == 0 {
		return files, nil
	}
	loose := followLoose(p, files)
	dirs = moveFolders(p, files)
	addFolderFor(p, slices.DeleteFunc(slices.Clone(files), func(pr model.PathPair) bool { return loose[model.LooseKey(pr.Old)] }), addFolder)
	files = append(files, sameDirLoose(p, files)...)
	p.FoldSources(paths.Linked)
	return files, dirs
}

// RelinkDrive — тело Editor.RelinkDrive над проектом: переехавшие файлы и папки.
func RelinkDrive(p *model.Project, from, to string) (files, dirs []model.PathPair) {
	moves := map[string]string{}
	seen := map[string]bool{} // ID файлов: копии того же файла — одна проверка диска (relink тянет семью)
	for id, r := range p.Photos {
		if r.Lib != "" || seen[model.BaseID(id)] { // своё из библиотеки не лежит на диске фотографа (RelinkLib)
			continue
		}
		seen[model.BaseID(id)] = true
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
	relinkGuides(p, from, to)
	files = append(files, swapLoose(p, from, to)...)
	p.FoldSources(paths.Linked)
	return files, dirs
}

// followLoose — записи отдельных фото с прежним путём пары переезжают на новый (097 §5);
// ответ — ключи LooseKey прежних путей, которые были отдельными.
func followLoose(p *model.Project, pairs []model.PathPair) map[string]bool {
	if len(p.Loose) == 0 {
		return nil
	}
	to := make(map[string]string, len(pairs))
	for _, pr := range pairs {
		to[model.LooseKey(pr.Old)] = pr.New
	}
	was := map[string]bool{}
	eachLoose(p, func(f *model.LooseFile) {
		k := model.LooseKey(f.Path)
		if np, ok := to[k]; ok {
			was[k] = true
			f.Path, f.Alt = np, ""
		}
	})
	return was
}

// sameDirLoose — пропавшие отдельные фото не из альбома, лежавшие в той же папке, что прежний
// путь пары: файл с тем же именем в новой папке пары — переезд (пара — для своих отметок).
func sameDirLoose(p *model.Project, pairs []model.PathPair) []model.PathPair {
	if len(p.Loose) == 0 {
		return nil
	}
	dirs := make(map[string]string, len(pairs))
	for _, pr := range pairs {
		dirs[model.LooseKey(filepath.Dir(pr.Old))] = filepath.Dir(pr.New)
	}
	album := make(map[string]bool, len(p.Photos))
	for _, r := range p.Photos {
		album[model.LooseKey(r.Path)] = true
	}
	var out []model.PathPair
	eachLoose(p, func(f *model.LooseFile) {
		nd, ok := dirs[model.LooseKey(filepath.Dir(f.Path))]
		if !ok || album[model.LooseKey(f.Path)] || isFile(f.Path) {
			return
		}
		if np := filepath.Join(nd, filepath.Base(f.Path)); paths.CheckLocalPath(np) == nil && isFile(np) {
			out = append(out, model.PathPair{Old: f.Path, New: np})
			f.Path, f.Alt = np, ""
		}
	})
	return out
}

// swapLoose — смена буквы диска у пропавших отдельных фото, которые есть на новом месте.
func swapLoose(p *model.Project, from, to string) []model.PathPair {
	var out []model.PathPair
	eachLoose(p, func(f *model.LooseFile) {
		if isFile(f.Path) {
			return
		}
		if np, ok := paths.SwapDrive(f.Path, from, to); ok && isFile(np) {
			out = append(out, model.PathPair{Old: f.Path, New: np})
			f.Path, f.Alt = np, ""
		}
	})
	return out
}

// eachLoose — fn по каждому файлу отдельных фото (по указателю: правка на месте).
func eachLoose(p *model.Project, fn func(f *model.LooseFile)) {
	for i := range p.Loose {
		for j := range p.Loose[i].Files {
			fn(&p.Loose[i].Files[j])
		}
	}
}

// relink меняет пути фото из moves и пересчитывает ID. Перепривязка — по файлу (098-20): запись
// из moves тянет за собой всю семью копий (свойства файла — SetFile), пара — одна на файл.
// Пропускает: неизвестный ID, своё из библиотеки (путь от искателя, RelinkLib), нелокальный путь,
// путь, уже занятый файлом другой семьи (повтор файла в альбоме), повтор семьи в moves.
func relink(p *model.Project, moves map[string]string) []model.PathPair {
	used := make(map[string]string, len(p.Photos)) // ключ пути → ID файла (BaseID)
	for id, r := range p.Photos {
		used[paths.Fold(filepath.Clean(r.Path))] = model.BaseID(id)
	}
	files := p.PlacedFiles()
	done := map[string]bool{}
	var pairs []model.PathPair
	for _, id := range slices.Sorted(maps.Keys(moves)) {
		ref, ok := p.Photos[id]
		base := model.BaseID(id)
		if !ok || ref.Lib != "" || done[base] || paths.CheckLocalPath(moves[id]) != nil {
			continue
		}
		np := filepath.Clean(moves[id])
		key := paths.Fold(np)
		if other, taken := used[key]; taken && other != base {
			continue
		}
		done[base] = true
		pairs = append(pairs, model.PathPair{Old: ref.Path, New: np, BgOnly: !files[base]})
		delete(used, paths.Fold(filepath.Clean(ref.Path)))
		used[key] = base
		ref.Path, ref.Alt = np, ""
		// Файл на месте, но другой длины (перепривязали «по W×H»): размеры — из заголовка.
		if size, ok := fileSize(np); ok {
			refresh(&ref, size)
		}
		p.Photos[id] = ref
		p.SetFile(id, ref)
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
	in := paths.NewCover(p.Folders, paths.Linked) // лента видит то, куда дойдёт сканер (093-4)
	for _, pr := range pairs {
		if in.HasFile(pr.New) {
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
