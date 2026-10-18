package collect

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Album — копия проекта p для собранной папки dir после успешного Copy (design §4): у записей
// плана — путь в папке сбора, без запасного пути и без ключа библиотеки (своё — обычный файл,
// decisions №13); пропавшие — как были. Папка альбома — dir/Фото (лента читает подпапки),
// разметки — новые пути. Отметки: фото в плане — новый путь, пропавшее фото — как было,
// остальные (фото вне сбора) не пишутся. ID — по новым путям (Rekey). p и marks не меняются.
func Album(p *model.Project, marks map[string]model.Mark, dir string, pl Plan) (*model.Project, map[string]model.Mark) {
	q := p.Clone()
	moved := map[string]string{} // ключ пути источника → путь в папке сбора
	photos := false
	for _, it := range pl.Items {
		np := filepath.Join(dir, filepath.FromSlash(it.Rel))
		moved[key(it.Src)] = np
		photos = photos || it.Kind == KindPhoto
		retarget(q, it, np)
	}
	q.Folders, q.FolderAlt = nil, nil
	if photos {
		q.Folders = []string{filepath.Join(dir, DirPhotos)}
	}
	out := moveMarks(p, marks, moved, pl.Missing)
	salbum.Rekey(q, library.IDFor)
	return q, out
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
func Save(q *model.Project, marks map[string]model.Mark, path string, pl Plan) error {
	if err := salbum.WriteRel(q, path, filepath.Dir(path), marks); err != nil {
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
