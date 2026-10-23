package album

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Фон альбома (098 design §4.4): рисуется на листах без своего фона; тело — backdrop/album.go.
// Смена фона альбома меняет вид всех разворотов: дельта вида полная.

// albumBgMerge — ключ слияния серии ползунка прозрачности фона альбома в один шаг отмены.
func albumBgMerge(session string) string {
	if !model.ValidSession(session) {
		return ""
	}
	return "albumbg:" + session
}

// SetAlbumBg ставит фон альбома: узор src.Pattern или своё фото src.Photo (ключ библиотеки) —
// на весь разворот или на каждую страницу (pages). Один шаг отмены; тот же — холостая.
// Договор: до правки ничего; источник и ключ проверяет тело (ErrPattern, ErrLibKey — без шага).
func (e *Editor) SetAlbumBg(src model.BgSource, pages bool) (model.View, error) {
	return e.try("", func(p *model.Project) error { return backdrop.SetAlbumBg(p, src, pages) })
}

// ClearAlbumBg снимает фон альбома; свои фоны листов и отметки «Не показывать здесь» остаются.
// Договор: до правки ничего; фона нет — холостая.
func (e *Editor) ClearAlbumBg() model.View {
	return e.change(backdrop.ClearAlbumBg)
}

// SetAlbumBgFade ставит прозрачность фона альбома 0…1 (зажим, шаг 0,01). merge — сессия
// ползунка: серия — один шаг отмены.
// Договор: fade — конечное число, иначе холостая; ключ слияния — albumBgMerge.
func (e *Editor) SetAlbumBgFade(fade float64, merge string) model.View {
	if math.IsNaN(fade) || math.IsInf(fade, 0) {
		return e.View()
	}
	return e.apply(albumBgMerge(merge), func(p *model.Project) { backdrop.SetAlbumBgFade(p, fade) })
}

// SetNoAlbumBg — «Не показывать здесь» (off = true) и «Показать» фона альбома на развороте.
// Договор: до правки ничего; обложка и неизвестный ID — холостая (проверяет тело).
func (e *Editor) SetNoAlbumBg(spreadID string, off bool) model.View {
	return e.change(func(p *model.Project) { backdrop.SetNoAlbumBg(p, spreadID, off) })
}
