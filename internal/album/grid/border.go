package grid

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочки мест (079 design §1, §8, §10). У каждого места свой *Border (079-23): правка — только
// заменой указателя новой зажатой рамочкой, иначе общий указатель после «Разделить» менял бы оба
// места, а WithGrid (==) правку на месте не увидел бы.

// ApplyBorder — тело Editor.ApplyBorder: рамочка src местам ids листа spreadID (ссылки placeRef);
// пустой src — снять. Своя картинка — запись Photos по ключу библиотеки (Remember) тем же шагом.
// У «Авто» постановка закрепляет разворот (WithGrid), снятие — холостая. Каждому месту — своя
// копия; у места с той же рамочкой указатель стоит (повтор — холостая). Вид не по грамматике,
// неверный ключ своей или заданы оба (вид и картинка) — ErrBorder; неизвестный лист — холостая.
func ApplyBorder(p *model.Project, spreadID string, ids []string, src model.BorderSrc) error {
	ph := src.Photo
	if src.Kind != "" && ph.ID != "" {
		return model.ErrBorder
	}
	nb := src.Border()
	if nb != nil && nb.Photo != "" && (!model.ValidLibKey(ph.Lib) || ph.ID != model.LibPhotoID(ph.Lib)) {
		return model.ErrBorder
	}
	s := p.Sheet(spreadID)
	switch {
	case s == nil, nb == nil && len(s.Grid) == 0:
		return nil
	case nb != nil && nb.Photo != "" && !p.OnBackdrop(ph.ID): // запись фона — к исходнику, не переписывать копией
		model.Remember(p, []model.PhotoInfo{ph})
	}
	if nb != nil {
		if nb = model.ClampBorder(p, nb); nb == nil {
			return model.ErrBorder
		}
	}
	return p.WithGrid(s, func() error {
		for _, ref := range ids {
			if pl := s.PlaceRef(ref); pl != nil && !sameBorder(pl.Border, nb) {
				pl.Border = nb.Clone()
			}
		}
		return nil
	})
}

// SetBorder — тело Editor.SetBorder: ручная правка рамочек мест ids (ссылки placeRef) листа.
// pt.Inks (не nil) — цвета встроенной (пустой список — цвета вида), pt.Width > 0 — её толщина,
// pt.Scale > 0 — размер картинки; поля чужого вида не действуют. Изменилось — новая рамочка без
// отметки плитки Preset (как SetPattern), те же числа — указатель и плитка стоят. Места без
// рамочки, «Авто» и неизвестный лист — холостая.
func SetBorder(p *model.Project, spreadID string, ids []string, pt model.BorderPatch) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	for _, ref := range ids {
		pl := s.PlaceRef(ref)
		if pl == nil || pl.Border == nil {
			continue
		}
		nb := pl.Border.Clone()
		switch {
		case nb.Kind != "":
			if pt.Inks != nil {
				nb.Inks = slices.Clone(pt.Inks)
			}
			if pt.Width > 0 {
				nb.Width = pt.Width
			}
		case pt.Scale > 0:
			nb.Scale = pt.Scale
		}
		nb.Preset = ""
		if nb = model.ClampBorder(p, nb); nb != nil && !sameParams(nb, pl.Border) {
			pl.Border = nb
		}
	}
}

// sameBorder — рамочки равны по значению (nil — только nil).
func sameBorder(a, b *model.Border) bool {
	if a == nil || b == nil {
		return a == b
	}
	return sameParams(a, b) && a.Preset == b.Preset
}

// sameParams — у рамочек один вид или картинка и те же числа; отметка плитки не сравнивается.
func sameParams(a, b *model.Border) bool {
	return a.Kind == b.Kind && a.Photo == b.Photo && slices.Equal(a.Inks, b.Inks) && a.Width == b.Width &&
		a.Scale == b.Scale && a.Cut == b.Cut && a.Open == b.Open
}
