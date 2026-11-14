package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Правки стикера на листе (спека 048 Ф2, design §8). Постановка и замена рисунка — в App
// (AddSticker, ReplaceSticker): им нужна библиотека decor.Store.

// SetStickerBox — перенос, размер, поворот стикера (мм листа, угол до поворота); серия с
// одним merge (жест, стрелки) — один шаг отмены.
func (m *Methods) SetStickerBox(spreadID, id string, x, y, w, h, rot float64, merge string) (v model.View) {
	defer m.guardEdit("SetStickerBox", &v, nil)
	return m.changed(m.editor().SetStickerBox(spreadID, id, x, y, w, h, rot, merge))
}

// FlipSticker — зеркало стикера по оси "h" | "v"; один шаг.
func (m *Methods) FlipSticker(spreadID, id, axis string) (v model.View) {
	defer m.guardEdit("FlipSticker", &v, nil)
	return m.changed(m.editor().FlipSticker(spreadID, id, axis))
}

// SetStickerFade — прозрачность стикера 0..1; серия ползунка с одним merge — один шаг.
func (m *Methods) SetStickerFade(spreadID, id string, fade float64, merge string) (v model.View) {
	defer m.guardEdit("SetStickerFade", &v, nil)
	return m.changed(m.editor().SetStickerFade(spreadID, id, fade, merge))
}

// SetStickerInks — цвета процедурного стикера; у своего — холостая. Серия палитры — один шаг.
func (m *Methods) SetStickerInks(spreadID, id string, inks []string, merge string) (v model.View) {
	defer m.guardEdit("SetStickerInks", &v, nil)
	return m.changed(m.editor().SetStickerInks(spreadID, id, inks, merge))
}
