package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Album — копия проекта p для собранной папки dir после успешного Copy (design §4): у записей
// плана — путь в папке сбора, без запасного пути и без ключа библиотеки (своё — обычный файл,
// decisions №13); пропавшие — как были. Папка альбома — dir/Фото (лента читает подпапки),
// разметки — новые пути. Отметки: фото в плане — новый путь, пропавшее фото — как было,
// остальные (фото вне сбора) не пишутся, а с ними и их членство в подборках (083-12); подборки —
// копией. Отдельные фото (097 §6): в плане — новый путь в «Фото отдельно», пропавшие — как были,
// не скопированные (режим «только альбом») — выпадают. ID — по новым путям (Rekey), отпечаток —
// новый. p и side не меняются.
func Album(p *model.Project, side model.Side, dir string, pl Plan) (*model.Project, model.Side) {
	q := p.Clone()
	moved := map[string]string{} // ключ пути источника → путь в папке сбора
	photos := false
	for _, it := range pl.Items {
		np := filepath.Join(dir, filepath.FromSlash(it.Rel))
		moved[key(it.Src)] = np
		// Только отдельные фото — папки «Фото» нет: пропавшая папка в списке была бы ложной
		photos = photos || it.Kind == KindPhoto && !strings.HasPrefix(it.Rel, DirLoose+"/")
		retarget(q, it, np)
	}
	q.Folders, q.FolderAlt = nil, nil
	if photos {
		q.Folders = []string{filepath.Join(dir, DirPhotos)}
	}
	q.Loose = looseMoved(p, moved, pl)
	q.FoldSources(paths.Linked)
	out := model.Side{Marks: moveMarks(p, side.Marks, moved, pl.Missing), Bins: slices.Clone(side.Bins)}
	salbum.Rekey(q, library.IDFor)
	q.UID = model.NewUID() // собранная копия — другой проект: ссылка прежнего к ней не переходит (146 T4-9)
	return q, out
}

// looseMoved — отдельные фото собранного альбома: файл в плане — его новый путь; пропавший (фото
// альбома из pl.Missing или pl.LooseLost) — как был; остальные выпадают.
func looseMoved(p *model.Project, moved map[string]string, pl Plan) []model.LooseAdd {
	lost := map[string]bool{}
	for _, k := range pl.LooseLost {
		lost[k] = true
	}
	for _, id := range pl.Missing {
		if ref, ok := p.Photos[id]; ok && ref.Path != "" {
			lost[key(ref.Path)] = true
		}
	}
	var out []model.LooseAdd
	for _, a := range p.Loose {
		files := make([]model.LooseFile, 0, len(a.Files))
		for _, f := range a.Files {
			if np, ok := moved[key(f.Path)]; ok {
				files = append(files, model.LooseFile{Path: np})
			} else if lost[key(f.Path)] {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			a.Files = files
			out = append(out, a)
		}
	}
	return out
}

// retarget — записи элемента плана it в копии q смотрят на его файл np в папке сбора.
func retarget(q *model.Project, it Item, np string) {
	for _, r := range it.Refs {
		switch {
		case it.Kind == KindGuide && r == RefCover && q.Cover != nil:
			q.Cover.Guide, q.Cover.GuideAlt = np, ""
		case it.Kind == KindGuide && r == RefSpreads:
			q.SpreadGuide, q.SpreadGuideAlt = np, ""
		case it.Kind == KindPhoto || it.Kind == KindOwn:
			if ref, ok := q.Photos[r]; ok {
				ref.Path, ref.Alt, ref.Lib, ref.Size = np, "", "", it.Size
				q.Photos[r] = ref
			}
		}
	}
}

// moveMarks — отметки собранного альбома: файл в плане (moved) — новый путь; пропавшее фото
// альбома (missing) — прежний; остальные не пишутся. Ключ — ID по пути.
func moveMarks(p *model.Project, marks map[string]model.Mark, moved map[string]string, missing []string) map[string]model.Mark {
	lost := map[string]bool{}
	for _, id := range missing {
		if ref, ok := p.Photos[id]; ok && ref.Path != "" {
			lost[key(ref.Path)] = true
		}
	}
	out := make(map[string]model.Mark, len(marks))
	for _, m := range marks {
		if np, ok := moved[key(m.Path)]; ok {
			m.Path = np
			out[library.IDFor(np)] = m.WithAlt("")
		} else if lost[key(m.Path)] {
			out[library.IDFor(m.Path)] = m
		}
	}
	return out
}

// Save пишет собранный альбом q в path только относительными путями от его папки (WriteRel,
// атомарно) и открывает записанное заново: пропавших больше, чем pl.Missing, — ошибка плана
// (альбом уже записан, но об этом нужно сказать). Резервную копию прежнего файла делает App.
// Печать отпечатка — пути собранного альбома: сбор выдаёт ему отпечаток (Album, 146 T4-9).
func Save(q *model.Project, side model.Side, path string, pl Plan) error {
	q.SetUIDAt(model.UIDTag(q.UID, path))
	if err := salbum.WriteRel(q, path, filepath.Dir(path), side); err != nil {
		return diskFull(err) // альбом пишется последним: место могло кончиться и на нём
	}
	lp, err := salbum.Load(path)
	if err != nil {
		return fmt.Errorf("собранный альбом не открылся: %w", err)
	}
	lost := 0
	for _, r := range lp.Photos {
		if st, err := os.Stat(r.Path); r.Path == "" || err != nil || !st.Mode().IsRegular() {
			lost++
		}
	}
	if lost > len(pl.Missing) {
		return fmt.Errorf("в собранном альбоме не нашлось фото: %d", lost-len(pl.Missing))
	}
	return nil
}
