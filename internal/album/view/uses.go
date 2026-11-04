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
		var here []model.PhotoUse // вхождения листа: подписи различаем после обхода
		var seats []int           // номер места в ряду мест листа (у остальных видов 0)
		sheetUses(s, func(kind, what, pick, x string, seat int) {
			if mine(x) {
				here = append(here, model.PhotoUse{Sheet: i, Kind: kind, Spread: s.ID, Page: page, What: what, Pick: pick})
				seats = append(seats, seat)
			}
		})
		numberUses(here, seats)
		for _, u := range here {
			use(u)
		}
	}
	return out
}

// scopeWord — область фона в подписи при повторах: слева, справа, корешок, целиком.
var scopeWord = map[string]string{
	model.ScopeSpread: "целиком", model.ScopeLeft: "слева", model.ScopeRight: "справа", model.ScopeSpine: "на корешке",
}

// numberUses — подпись «место 3» вместо «место», если вхождений одного вида на листе два и более
// (иначе две копии фото на одном развороте неразличимы). Место — по номеру в ряду мест листа
// (seats), фон — областью (scopeWord), остальные виды — по порядку вхождений; одиночное вхождение подпись не меняет.
func numberUses(here []model.PhotoUse, seats []int) {
	count := map[string]int{}
	for _, u := range here {
		count[u.Kind]++
	}
	next := map[string]int{}
	for i := range here {
		u := &here[i]
		next[u.Kind]++
		if count[u.Kind] < 2 {
			continue
		}
		n := next[u.Kind]
		if u.Kind == model.UsePlace {
			n = seats[i]
		}
		if u.Kind == model.UseBackdrop { // у фона различает область, а не номер
			u.What += " " + scopeWord[u.Pick]
			continue
		}
		u.What = fmt.Sprintf("%s %d", u.What, n)
	}
}

// sheetUses — фото листа по порядку: место, фоны областей, стикеры, рамочки-картинки мест; для
// каждого — вид, «что» подписи, Pick (что выделить при переходе), запись фото (106) и номер
// места в ряду мест листа с 1 (у остальных видов 0).
func sheetUses(s *model.Spread, add func(kind, what, pick, photo string, seat int)) {
	for i, x := range s.Photos {
		add(model.UsePlace, "место", x, x, i+1)
	}
	for _, scope := range model.BdScopes() { // порядок как у Backdrops()
		if b := *s.Slot(scope); b != nil {
			add(model.UseBackdrop, "фон", scope, b.Photo, 0)
		}
	}
	for _, st := range s.Stickers {
		add(model.UseSticker, "стикер", st.ID, st.Photo, 0)
	}
	for _, pl := range s.Grid {
		if b := pl.Border; b != nil {
			ref := pl.Photo // запись фото в месте; пустое место — само место
			if ref == "" {
				ref = pl.ID
			}
			add(model.UseBorder, "рамочка", ref, b.Photo, 0)
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
