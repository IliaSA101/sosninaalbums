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
