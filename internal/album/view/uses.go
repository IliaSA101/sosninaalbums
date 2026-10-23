package view

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Uses — где стоит фото id (084): по ряду листов (обложка первой), внутри листа — место, фоны,
// свои стикеры, рамочки-картинки мест; каждое вхождение отдельно. Считаются все записи семьи
// копий id (098): Ref — запись вхождения, если это не сам id. Фон альбома — первым, Sheet = −1.
// Только чтение; не nil.
func Uses(p *model.Project, id string) []model.PhotoUse {
	out := []model.PhotoUse{}
	if id == "" {
		return out
	}
	base := model.BaseID(id)
	use := func(sheet int, label, x string) {
		u := model.PhotoUse{Sheet: sheet, Label: label}
		if x != id {
			u.Ref = x
		}
		out = append(out, u)
	}
	if b := p.AlbumBg; b != nil && b.Photo != "" && model.BaseID(b.Photo) == base {
		use(-1, "Фон альбома", b.Photo)
	}
	n := 0 // номер разворота или листа, без обложки
	for i, s := range p.Sheets() {
		name := "Обложка"
		if !p.IsCover(s) {
			n++
			name = fmt.Sprintf("Разворот %d", n)
			if p.Settings.Sheet {
				name = fmt.Sprintf("Лист %d", n)
			}
		}
		add := func(kind string, ids ...string) {
			for _, x := range ids {
				if x != "" && model.BaseID(x) == base {
					use(i, name+" — "+kind, x)
				}
			}
		}
		add("место", s.Photos...)
		for _, b := range s.Backdrops() {
			add("фон", b.Photo)
		}
		for _, st := range s.Stickers {
			add("стикер", st.Photo)
		}
		add("рамочка", s.BorderPhotos()...)
	}
	return out
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
