package view

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Uses — где стоит фото id (084): по ряду листов (обложка первой), внутри листа — место, фоны,
// свои стикеры, рамочки-картинки мест; каждое вхождение отдельно. Считаются все записи семьи
// копий id (098); запись вхождения — в Pick. Фон альбома — первым, Sheet = −1.
// Вид, лист, части подписи и Pick — для перехода без разбора Label (106). Только чтение; не nil.
func Uses(p *model.Project, id string) []model.PhotoUse {
	out := []model.PhotoUse{}
	if id == "" {
		return out
	}
	base := model.BaseID(id)
	use := func(u model.PhotoUse) {
		if u.What == "" {
			u.Label = u.Page
		} else {
			u.Label = u.Page + " — " + u.What
		}
		out = append(out, u)
	}
	mine := func(x string) bool { return x != "" && model.BaseID(x) == base }
	if b := p.AlbumBg; b != nil && mine(b.Photo) {
		use(model.PhotoUse{Sheet: -1, Kind: model.UseAlbumBg, Page: "Фон альбома"})
	}
	n := 0 // номер разворота или листа, без обложки
	for i, s := range p.Sheets() {
		page := "Обложка"
		if !p.IsCover(s) {
			n++
			page = fmt.Sprintf("Разворот %d", n)
			if p.Settings.Sheet {
				page = fmt.Sprintf("Лист %d", n)
			}
		}
		sheetUses(s, func(kind, what, pick, x string) {
			if mine(x) {
				use(model.PhotoUse{Sheet: i, Kind: kind, Spread: s.ID, Page: page, What: what, Pick: pick})
			}
		})
	}
	return out
}

// sheetUses — фото листа по порядку: место, фоны областей, стикеры, рамочки-картинки мест; для
// каждого — вид, «что» подписи, Pick (что выделить при переходе) и запись фото (106).
func sheetUses(s *model.Spread, add func(kind, what, pick, photo string)) {
	for _, x := range s.Photos {
		add(model.UsePlace, "место", x, x)
	}
	for _, scope := range model.BdScopes() { // порядок как у Backdrops()
		if b := *s.Slot(scope); b != nil {
			add(model.UseBackdrop, "фон", scope, b.Photo)
		}
	}
	for _, st := range s.Stickers {
		add(model.UseSticker, "стикер", st.ID, st.Photo)
	}
	for _, pl := range s.Grid {
		if b := pl.Border; b != nil {
			ref := pl.Photo // запись фото в месте; пустое место — само место
			if ref == "" {
				ref = pl.ID
			}
			add(model.UseBorder, "рамочка", ref, b.Photo)
		}
	}
}

// WhereUsed — «где стоит» для окна «Фото уже в альбоме» (098 design §2.4): по ID файла каждого
// id (запись, ID ленты или запрос копии) — вхождения семьи, число мест и запись единственного
// места (Move; мест 0 или больше одного — ""). Только чтение; не nil, по порядку ids.
func WhereUsed(p *model.Project, ids []string) []model.UsedAt {
	out := make([]model.UsedAt, 0, len(ids))
	for _, id := range ids {
		base := model.FileOf(id)
		u := model.UsedAt{ID: base, Uses: Uses(p, base)}
		for _, s := range p.Sheets() {
			for _, x := range s.Photos {
				if model.BaseID(x) == base {
					u.Places++
					u.Move = x
				}
			}
		}
		if u.Places != 1 {
			u.Move = ""
		}
		out = append(out, u)
	}
	return out
}

// UsedText — строка «Использовано N раз» по числу вхождений; 0 — «В альбоме не использовано».
// У слова «раз» своя форма только у 2–4 («раза»), кроме 12–14.
func UsedText(n int) string {
	if n <= 0 {
		return "В альбоме не использовано"
	}
	word := "раз"
	if d := n % 10; d >= 2 && d <= 4 && (n%100 < 12 || n%100 > 14) {
		word = "раза"
	}
	return fmt.Sprintf("Использовано %d %s", n, word)
}
