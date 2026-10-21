package model

import (
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// LooseAdd — одно добавление отдельных фото (097, v16).
type LooseAdd struct {
	ID    string      `json:"id"`    // 12 hex, NewID
	At    int64       `json:"at"`    // время добавления, Unix-секунды; подпись — местное время в UI
	Files []LooseFile `json:"files"` // порядок выбора; ≥ 1 (пустое добавление убирает FoldSources)
}

// LooseFile — файл добавления. Path в памяти абсолютный; Alt — запасной путь из файла, когда
// файла нет ни там, ни там (как PhotoRef.Alt, FolderAlt): encode пишет его обратно как был.
type LooseFile struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

// LooseAdded — итог добавления отдельных фото (097 §4): Batch — ID нового добавления ("" — ничего
// не добавлено); InFolder — уже в папке альбома (Folder — папка первого такого файла); Dup — уже
// добавлены отдельно или повтор в списке; Over — сверх MaxLooseFiles. Skipped (не фото) и Bad
// (не локальный путь) заполняет App до правки.
type LooseAdded struct {
	Batch    string `json:"batch"`
	Added    int    `json:"added"`
	InFolder int    `json:"inFolder"`
	Folder   string `json:"folder"`
	Dup      int    `json:"dup"`
	Skipped  int    `json:"skipped"`
	Bad      int    `json:"bad"`
	Over     int    `json:"over"`
}

// FolderAdd — итог добавления папки (093): Within — папка списка, в которую dir уже входит (или
// она сама; проект не менялся), "" — добавлена; Merged — сколько папок списка вошли в dir;
// LooseIn — сколько отдельных фото теперь видны через неё.
type FolderAdd struct {
	Within  string `json:"within"`
	Merged  int    `json:"merged"`
	LooseIn int    `json:"looseIn"`
}

// LooseView — добавление для интерфейса: файлы с ID фото ленты (Photo = library.IDFor пути) —
// UI по нему узнаёт, в альбоме ли фото и пришло ли оно из чтения. Запасной путь в вид не идёт.
type LooseView struct {
	ID    string          `json:"id"`
	At    int64           `json:"at"`
	Files []LooseFileView `json:"files"`
}

// LooseFileView — файл добавления в виде.
type LooseFileView struct {
	Path  string `json:"path"`
	Photo string `json:"photo"`
}

// LooseKey — ключ пути отдельного фото: один файл — один ключ (нижний регистр, Clean).
func LooseKey(path string) string { return strings.ToLower(filepath.Clean(path)) }

// ValidID — ID записи вида NewID: 12 строчных hex.
func ValidID(id string) bool {
	if len(id) != 12 {
		return false
	}
	for i := range len(id) {
		if c := id[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// LooseFiles — пути всех отдельных фото по порядку добавлений и выбора.
func (p *Project) LooseFiles() []string {
	out := []string{}
	for _, a := range p.Loose {
		for _, f := range a.Files {
			out = append(out, f.Path)
		}
	}
	return out
}

// FoldSources — инвариант «источники не пересекаются» (093, 097 §2): повтор папки и папка внутри
// другой папки списка уходят (порядок оставшихся прежний, их FolderAlt снимается); отдельное фото
// внутри папки списка уходит (looseIn), повтор пути — позднейший, пустое добавление — целиком;
// сверх MaxLooseAdds — хвост добавлений, затем сверх MaxLooseFiles — хвост файлов (только Load:
// операции предел не превышают). merged — сколько папок ушло. Второй вызов ничего не меняет.
// «Внутри» — куда дойдёт сканер (paths.Reaches, 093-4): подпапка на «.», устройство или через
// ссылку остаётся отдельной записью. linked — проверка ссылок (paths.Linked; модель без IO —
// функцию даёт вызывающий), nil — не проверять.
func (p *Project) FoldSources(linked func(dir string) bool) (merged, looseIn int) {
	merged = p.foldFolders(linked)
	return merged, p.foldLoose(linked)
}

// foldFolders — папки без повторов и вложенных; FolderAlt — только у оставшихся.
func (p *Project) foldFolders(linked func(dir string) bool) int {
	c := paths.NewCover(p.Folders, linked)
	seen := map[string]bool{}
	kept := make([]string, 0, len(p.Folders))
	for _, f := range p.Folders {
		k := paths.DirKey(f)
		if _, in := c.Root(f, false); !in && !seen[k] {
			seen[k] = true
			kept = append(kept, f)
		}
	}
	merged := len(p.Folders) - len(kept)
	if merged > 0 {
		p.Folders = kept
	}
	if len(p.FolderAlt) > 0 {
		have := make(map[string]bool, len(p.Folders))
		for _, f := range p.Folders {
			have[f] = true
		}
		for dir := range p.FolderAlt {
			if !have[dir] {
				delete(p.FolderAlt, dir)
			}
		}
	}
	if len(p.FolderAlt) == 0 {
		p.FolderAlt = nil
	}
	return merged
}

// foldLoose — отдельные фото вне папок, без повторов, без пустых добавлений и в пределах.
func (p *Project) foldLoose(linked func(dir string) bool) (looseIn int) {
	c := paths.NewCover(p.Folders, linked)
	seen := map[string]bool{}
	var out []LooseAdd
	for _, a := range p.Loose {
		if len(out) >= MaxLooseAdds {
			break
		}
		files := make([]LooseFile, 0, len(a.Files))
		for _, f := range a.Files {
			k := LooseKey(f.Path)
			switch {
			case f.Path == "":
			case c.HasFile(f.Path):
				looseIn++
			case seen[k] || len(seen) >= MaxLooseFiles:
			default:
				seen[k] = true
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			a.Files = files
			out = append(out, a)
		}
	}
	p.Loose = out
	return looseIn
}
