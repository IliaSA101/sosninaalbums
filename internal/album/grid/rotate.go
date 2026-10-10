package grid

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Поворот места (спека 010, design-rotation §2.1). Опора — центр места: код мест
// читает X…X+W как протяжённость и X+W/2 как центр, поэтому resize, inArea,
// areaFrac, mirrorFrac и формула группы верны без правок.

// SetPlaceAngle — угол места ref (ID места или фото), градусы по часовой вокруг
// центра; X, Y, W, H стоят (hold от угла не зависит). У «Авто» — закрепление той же
// правкой (withGrid). NaN/Inf и тот же угол — холостая.
func SetPlaceAngle(p *model.Project, spreadID, ref string, deg float64) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	_ = p.WithGrid(s, func() error {
		if pl := s.PlaceRef(ref); pl != nil {
			pl.Rot = geom.NormDeg(deg)
			p.ClampPlace(pl)
		}
		return nil
	})
}
