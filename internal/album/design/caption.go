package design

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Подписи к фото: набор, оформление и образец стиля (спека 006).

// SetCaption меняет подпись к фото (новая — со стилем соседней подписи, иначе по формату страницы); пустая —
// подпись снимается. txt — уже очищенный (CleanText).
func SetCaption(p *model.Project, photoID, txt string) {
	r, ok := p.Photos[photoID]
	if !ok {
		return
	}
	switch {
	case strings.TrimSpace(txt) == "":
		r.Caption = nil
	case r.Caption == nil:
		r.Caption = newCaption(p, photoID, txt)
	default:
		c := *r.Caption
		c.Text = txt
		r.Caption = &c
	}
	p.Photos[photoID] = r
}

// SetCaptionStyle — оформление подписи (st — уже зажатый, ClampStyle), её место
// (over — на фото) и отступ pad (030; уже зажат ClampCaptionPad, nil — 1 мм, свой
// указатель). Без подписи — холостая правка.
func SetCaptionStyle(p *model.Project, photoID string, st model.TextStyle, over bool, pad *float64) {
	r, ok := p.Photos[photoID]
	if !ok || r.Caption == nil {
		return
	}
	r.Caption = &model.Caption{Text: r.Caption.Text, Style: st, Over: over, Pad: pad}
	p.Photos[photoID] = r
}

// newCaption — подпись к фото id: оформление (стиль и «на фото») берёт у подписи
// на том же развороте, иначе у ближайшей по ленте, иначе — стиль по формату. Отступ (030) — тоже.
// Так поправленный размер не повторяют на каждом фото. Стиль хранится в подписи
// целиком, поэтому формат файла и старые подписи не меняются.
func newCaption(p *model.Project, id, txt string) *model.Caption {
	c := &model.Caption{Text: txt, Style: model.NewCaptionStyle(p.Settings.PageH)}
	if src := captionSource(p, id); src != nil {
		c.Style, c.Over = src.Style, src.Over
		c.Pad = model.ClampCaptionPad(src.Pad) // своя копия: указатель не общий
	}
	return c
}

// captionSource — подпись-образец для фото id: сначала с его разворота, потом
// из всего альбома; ближайшая по ленте, при равенстве — предыдущая. Фото вне
// разворотов — первая подпись ленты.
func captionSource(p *model.Project, id string) *model.Caption {
	type slot struct {
		spread int
		photo  string
	}
	var ribbon []slot
	at := -1
	for i, s := range p.Sheets() {
		for _, pid := range s.Photos {
			if pid == id {
				at = len(ribbon)
			}
			ribbon = append(ribbon, slot{i, pid})
		}
	}
	pick := func(sameSpread bool) *model.Caption {
		var best *model.Caption
		bestD := 0
		for k, sl := range ribbon {
			c := p.Photos[sl.photo].Caption
			if sl.photo == id || c == nil || (sameSpread && sl.spread != ribbon[at].spread) {
				continue
			}
			d := k - at
			if at < 0 {
				d = k
			}
			if d < 0 {
				d = -d
			}
			if best == nil || d < bestD {
				best, bestD = c, d
			}
		}
		return best
	}
	if at >= 0 {
		if c := pick(true); c != nil {
			return c
		}
	}
	return pick(false)
}
