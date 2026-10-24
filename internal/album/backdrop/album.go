package backdrop

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Фон альбома (098 design §4.4): поле проекта, а не слоты листа — рисуется на каждом листе без
// своего фона (Project.AlbumBgOn), обложки не касается (098-10). Геометрии нет: заполняет область.

// SetAlbumBg — тело Editor.SetAlbumBg: щелчок по плитке окна «Фоны» в режиме «Фон альбома».
// Узор или своё фото из библиотеки (ровно одно, иначе ErrPattern); pages — на каждую страницу,
// иначе на весь разворот. Прежние кадр и зеркало сбрасываются (у нового снимка свой горизонт),
// прозрачность остаётся — она в строке настроек, а не у плитки. Цвет страниц пресета не
// ставится: Settings.Bg красит и обложку, а её фон альбома не касается. Прозрачность узора — у
// фона альбома (Fade), своя у узора обнуляется. Тот же фон повторно — холостая (сравнение JSON).
func SetAlbumBg(p *model.Project, src model.BgSource, pages bool) error {
	if (src.Pattern == nil) == (src.Photo.ID == "") {
		return model.ErrPattern
	}
	b := model.AlbumBg{Pages: pages}
	if p.AlbumBg != nil {
		b.Fade = p.AlbumBg.Fade
	}
	if src.Pattern != nil {
		np := model.ClampPattern(src.Pattern)
		if np == nil {
			return model.ErrPattern
		}
		np.Fade = 0
		b.Pat = np
	} else {
		ph := src.Photo
		if ph.Lib != "" && (!model.ValidLibKey(ph.Lib) || ph.ID != model.LibPhotoID(ph.Lib)) {
			return model.ErrLibKey // иначе Rekey при открытии развёл бы запись и фон
		}
		model.Remember(p, []model.PhotoInfo{ph})
		b.Photo = ph.ID
	}
	p.AlbumBg = &b
	p.ClampAlbumBg()
	return nil
}

// ClearAlbumBg — тело Editor.ClearAlbumBg: «Убрать». Отметки «Не показывать здесь» листов
// остаются — новый фон альбома их не вернёт. Запись фото уходит сама (PruneRefs конвейера).
func ClearAlbumBg(p *model.Project) { p.AlbumBg = nil }

// SetAlbumBgFade — тело Editor.SetAlbumBgFade: прозрачность фона альбома (зажим 0…1, шаг 0,01
// — ClampAlbumBg). Фона нет — холостая.
func SetAlbumBgFade(p *model.Project, fade float64) {
	if p.AlbumBg == nil {
		return
	}
	b := *p.AlbumBg
	b.Fade = fade
	p.AlbumBg = &b
	p.ClampAlbumBg()
}

// SetNoAlbumBg — тело Editor.SetNoAlbumBg: «Не показывать здесь» (off) и «Показать» на развороте
// spreadID. Обложка и неизвестный ID — холостая: у обложки фона альбома нет.
func SetNoAlbumBg(p *model.Project, spreadID string, off bool) {
	i := slices.IndexFunc(p.Spreads, func(s *model.Spread) bool { return s.ID == spreadID })
	if i < 0 || p.IsCover(p.Spreads[i]) {
		return
	}
	p.Spreads[i].NoAlbumBg = off
}

// ownBg — у листа есть свой фон: узор или фото фона на любой области (фон альбома на нём не виден).
func ownBg(s *model.Spread) bool { return len(s.Backdrops()) > 0 || len(s.Patterns()) > 0 }

// toAlbum снимает свой фон листа и отметку «Не показывать здесь»: дальше лист идёт за фоном
// альбома. Цвет страниц не трогается (q115-4): узор из каталога ставит пару, «Как в альбоме»
// для цвета — отдельная правка. Запись фото фона уйдёт сама (PruneRefs конвейера).
func toAlbum(s *model.Spread) {
	s.Pat, s.PatL, s.PatR = nil, nil, nil
	s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.BgPhotoS = nil, nil, nil, nil
	s.NoAlbumBg = false
}

// SetAlbumBgAll — тело Editor.SetAlbumBgToAll (115): SetAlbumBg, затем AllToAlbum. Ошибка
// источника — до снятия, проект не тронут.
func SetAlbumBgAll(p *model.Project, src model.BgSource, pages bool) error {
	if err := SetAlbumBg(p, src, pages); err != nil {
		return err
	}
	AllToAlbum(p)
	return nil
}

// AllToAlbum — часть SetAlbumBgAll (115): у каждого разворота со своим фоном снимает
// его вместе с «Не показывать здесь». «Скрыт» без своего фона остаётся (q115-5), обложка —
// вне p.Spreads. Фона альбома нет — холостая.
func AllToAlbum(p *model.Project) {
	if p.AlbumBg == nil {
		return
	}
	for _, s := range p.Spreads {
		if ownBg(s) {
			toAlbum(s)
		}
	}
}

// ToAlbum — тело Editor.SpreadBgToAlbum (115): один разворот «как в альбоме»; «Не показывать
// здесь» снимается и без своего фона. Фона альбома нет, обложка и неизвестный ID — холостая.
func ToAlbum(p *model.Project, spreadID string) {
	if p.AlbumBg == nil {
		return
	}
	i := slices.IndexFunc(p.Spreads, func(s *model.Spread) bool { return s.ID == spreadID })
	if i < 0 || p.IsCover(p.Spreads[i]) {
		return
	}
	toAlbum(p.Spreads[i])
}
