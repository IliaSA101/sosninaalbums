package view

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Uses — где стоит фото id (084): по ряду листов (обложка первой), внутри листа — место, фоны,
// свои стикеры, рамочки-картинки мест; каждое вхождение отдельно. Только чтение; не nil.
func Uses(p *model.Project, id string) []model.PhotoUse {
	out := []model.PhotoUse{}
	if id == "" {
		return out
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
				if x == id {
					out = append(out, model.PhotoUse{Sheet: i, Label: name + " — " + kind})
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
