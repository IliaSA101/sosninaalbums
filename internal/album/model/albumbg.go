package model

import "github.com/IliaSA101/sosninaalbums/internal/geom"

// Фон альбома (098 design §4): поле проекта, а не Settings — настройки интерфейс шлёт целиком.
// Рисуется на каждом листе без своего фона (AlbumBgOn); геометрии нет — заполняет область.

// AlbumBg — фон альбома: ровно одно из Pat и Photo.
type AlbumBg struct {
	Pages bool     `json:"pages,omitempty"` // на каждую страницу; false — на весь разворот
	Pat   *Pattern `json:"pat,omitempty"`
	Photo string   `json:"photo,omitempty"` // ID записи Photos (своё из библиотеки)
	Frame *Frame   `json:"frame,omitempty"` // кадр фото в области; nil — S = 1; R — только наклон
	FlipH bool     `json:"flipH,omitempty"`
	FlipV bool     `json:"flipV,omitempty"`
	Fade  float64  `json:"fade,omitempty"` // 0…1, шаг 0,01, как Backdrop.Fade
}

// AlbumBgView — фон альбома для строки «Фон альбома» в настройках (View.AlbumBg).
type AlbumBgView struct {
	Name  string  `json:"name"`          // имя плитки
	Lib   string  `json:"lib,omitempty"` // ключ своего фото из библиотеки; "" — узор
	Fade  float64 `json:"fade"`
	Pages bool    `json:"pages"`
	// Preset — плитка каталога узора (как PatternView.Preset): образец и отметка в окне «Фоны».
	Preset string `json:"preset,omitempty"`
	// Photo — ID записи фото фона (образец /media/p/<id>, когда ключа библиотеки уже нет); "" — узор.
	Photo string `json:"photo,omitempty"`
}

// ClampAlbumBg — фон альбома к допустимому (Load и правки): и узор, и фото, ни того ни другого
// или фото без записи — фон снимается; узор — ClampPattern; кадр — как у фона (ClampFrame,
// только наклон ±44,9); у узора кадра и зеркала нет; Fade 0…1, шаг 0,01; у проекта-листа (031)
// страниц нет — Pages false. Значение заменяется копией.
func (p *Project) ClampAlbumBg() {
	b := p.AlbumBg
	if b == nil {
		return
	}
	if b.Pat != nil && b.Photo != "" {
		p.AlbumBg = nil
		return
	}
	c := AlbumBg{Pages: b.Pages && !p.Settings.Sheet, Fade: geom.Round2(geom.ClampF(Finite(b.Fade, 0), 0, 1))}
	if _, ok := p.Photos[b.Photo]; ok && b.Photo != "" {
		c.Photo, c.FlipH, c.FlipV = b.Photo, b.FlipH, b.FlipV
		if f := ClampFrame(b.Frame); f != nil {
			_, f.R = SplitR(f.R) // у фона только наклон (045)
			c.Frame = ClampFrame(f)
		}
	} else {
		c.Pat = ClampPattern(b.Pat)
	}
	if c.Pat == nil && c.Photo == "" {
		p.AlbumBg = nil
		return
	}
	p.AlbumBg = &c
}

// AlbumBgOn — фон альбома рисуется на листе s: он есть, s не обложка, на листе его не скрыли и
// своего фона нет — ни фото фона, ни узора на любой области (decisions 098-11).
func (p *Project) AlbumBgOn(s *Spread) bool {
	return p.AlbumBg != nil && s != nil && !p.IsCover(s) && !s.NoAlbumBg &&
		len(s.Backdrops()) == 0 && len(s.Patterns()) == 0
}

// albumBgPhoto — ID записи фото фона альбома; "" — его нет. Для обходов ссылок (OnDecor).
func (p *Project) albumBgPhoto() string {
	if p.AlbumBg == nil {
		return ""
	}
	return p.AlbumBg.Photo
}
