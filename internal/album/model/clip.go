package model

import "github.com/IliaSA101/sosninaalbums/internal/layout"

// Буфер «Копировать и вставить» (099, design §2). Снимок живёт в App и в .salbum не пишется:
// FormatVersion прежний. ClipSel, ClipInfo, PasteReq и Pt — биндинги; Clip и PasteOut несут
// записи фото с путями и наружу не уходят.

// ClipSel — выделенное на листе (от UI): ссылки мест или фото (как у DeleteSelection), ID надписей, ID стикеров.
type ClipSel struct {
	Refs     []string `json:"refs"`
	Texts    []string `json:"texts"`
	Stickers []string `json:"stickers"`
}

// ClipInfo — сводка буфера для UI, без путей. Пусто — все нули и Sheet "".
type ClipInfo struct {
	Photos   int    `json:"photos"`
	Places   int    `json:"places"` // пустые места
	Texts    int    `json:"texts"`
	Stickers int    `json:"stickers"`
	Sheet    string `json:"sheet"` // ID листа-источника
}

// Pt — точка листа, мм.
type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Повтор фото при вставке (PasteReq.Repeat): кнопки окна 098 «Фото уже в альбоме».
const (
	RepeatCopy = "copy" // «Поставить копию»
	RepeatMove = "move" // «Перенести сюда»
)

// PasteReq — куда вставлять. Hover — курсор над листом (мм; nil — не над листом), At — точка щелчка
// меню (центр вставленного; nil — по правилу). Repeat: "" — фото в местах альбома вернуть вопросом,
// RepeatCopy — ставить копии, RepeatMove — перенести (кнопки окна 098).
type PasteReq struct {
	Sheet  string `json:"sheet"`
	Hover  *Pt    `json:"hover"`
	At     *Pt    `json:"at"`
	Repeat string `json:"repeat"`
}

// Clean — запрос от UI к допустимому (design §10): точка с NaN/Inf — «точки нет», Repeat вне
// белого списка — "". Указатели новые: запрос UI не делит память с проектом.
func (r PasteReq) Clean() PasteReq {
	pt := func(p *Pt) *Pt {
		if p == nil || !finite(p.X) || !finite(p.Y) {
			return nil
		}
		c := *p
		return &c
	}
	r.Hover, r.At = pt(r.Hover), pt(r.At)
	if r.Repeat != RepeatCopy && r.Repeat != RepeatMove {
		r.Repeat = ""
	}
	return r
}

// Clip — снимок выделенного (не биндинг: несёт PhotoRef с путями). Всё — копии: правка и удаление
// оригинала снимок не меняют, вставка его не трогает (один снимок — много вставок).
type Clip struct {
	Src      string              // ID листа-источника
	Anchor   ClipAnchor          // точка привязки (design §3)
	Box      layout.Rect         // описанный прямоугольник группы, мм источника
	Places   []Place             // копии мест «Ручной» (ID и Photo — как у источника)
	Auto     []ClipPhoto         // фото «Авто»: ID и прямоугольник ячейки
	Texts    []TextBlock         // копии надписей
	Stickers []Sticker           // копии стикеров
	Refs     map[string]PhotoRef // записи фото мест и «Авто», картинок стикеров и рамочек (PhotoRef.Copy)
	Seq      []string            // ID элементов по стеку источника снизу вверх; фото «Авто» — первыми
}

// ClipAnchor — точка привязки снимка: центр группы в долях страницы-якоря или листа.
type ClipAnchor struct {
	Page   int     // страница-якорь; −1 — весь лист
	Fx, Fy float64 // центр группы в долях страницы (или листа)
}

// ClipPhoto — фото «Авто» в снимке: ID и ячейка раскладки источника, мм.
type ClipPhoto struct {
	ID  string
	Box layout.Rect
}

// Empty — в снимке нечего вставлять (nil тоже пуст).
func (c *Clip) Empty() bool {
	return c == nil || len(c.Places)+len(c.Auto)+len(c.Texts)+len(c.Stickers) == 0
}

// Info — сводка снимка для UI: числа и ID листа, без путей; nil — пустая.
func (c *Clip) Info() ClipInfo {
	if c.Empty() {
		return ClipInfo{}
	}
	in := ClipInfo{Photos: len(c.Auto), Texts: len(c.Texts), Stickers: len(c.Stickers), Sheet: c.Src}
	for _, pl := range c.Places {
		if pl.Photo != "" {
			in.Photos++
		} else {
			in.Places++
		}
	}
	return in
}

// PhotoIDs — фото снимка (места с фото, затем «Авто») по порядку: их вставка спрашивает о повторе.
func (c *Clip) PhotoIDs() []string {
	if c == nil {
		return nil
	}
	var ids []string
	for _, pl := range c.Places {
		if pl.Photo != "" {
			ids = append(ids, pl.Photo)
		}
	}
	for _, a := range c.Auto {
		ids = append(ids, a.ID)
	}
	return ids
}

// PasteOut — итог вставки: что выделить (ссылки мест или фото «Авто», ID надписей и стикеров),
// Repeat — фото, уже стоящие в альбоме (правки не было), Froze — «Авто» стал «Ручной».
type PasteOut struct {
	Refs, Texts, Stickers, Repeat []string
	Froze                         bool
}
