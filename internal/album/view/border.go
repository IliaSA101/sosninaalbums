package view

import (
	"net/url"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

// framePicURL — накладка и маска рамочки-картинки: /media/f/<ID записи>, как свой стикер по
// /media/p/<id> (079-22); путь к файлу даёт Editor.BorderPath, адрес путей не несёт.
const framePicURL = "/media/f/"

// borderView — рамочка места pl для холста и экспорта (079 design §7); нет рамочки — nil.
// Встроенная — как её нарисует gen: цвета дополнены цветами вида, толщина 0 — толщина вида;
// неизвестный вид (файл новее) — как в файле, Known false, без имени и адресов. Картинка — запись
// Photos (Lib — для имени из каталога), размер 0 — 1. Адреса — без px (UI дописывает &px=); строку
// запроса строит gen (079-34), срез и open — из альбома, как их зажал ClampBorder.
func borderView(p *model.Project, pl *model.Place) *model.BorderView {
	b := pl.Border
	if b == nil {
		return nil
	}
	bv := &model.BorderView{Kind: b.Kind, Photo: b.Photo, Preset: b.Preset, Ins: p.BorderIns(pl), Inks: []string{}}
	if b.Photo != "" {
		bv.Pic, bv.Open, bv.Known, bv.Lib, bv.Scale = true, b.Open, true, p.Photos[b.Photo].Lib, b.Scale
		if bv.Scale == 0 {
			bv.Scale = 1
		}
		req := gen.FramePicRequest{W: pl.W, H: pl.H, Scale: b.Scale, Cut: b.Cut, Open: b.Open}
		base := framePicURL + url.PathEscape(b.Photo) + "?"
		bv.URL = base + gen.FramePicQuery(req)
		if !b.Open { // без окна маски нет: фото на всё место
			req.Mask = true
			bv.MaskURL = base + gen.FramePicQuery(req)
		}
		return bv
	}
	bv.Inks, bv.Width = append(bv.Inks, b.Inks...), b.Width
	m, ok := gen.FrameLookup(b.Kind)
	if !ok {
		return bv
	}
	bv.Known, bv.Name, bv.DefWidth = true, m.Name, m.Width
	bv.Inks = append(bv.Inks, m.Inks[min(len(bv.Inks), len(m.Inks)):]...)
	if bv.Width == 0 {
		bv.Width = m.Width
	}
	o := pl.Outline()
	spec := gen.FrameSpec{Kind: b.Kind, Inks: b.Inks, Width: b.Width, W: pl.W, H: pl.H, Oval: o.Oval, Radius: o.Radius}
	if m.Mask { // мягкий край меняет маску фото и поверх ничего не рисует (как печать, render.FrameHasMask)
		bv.MaskURL = gen.FrameQuery(spec, true)
	} else {
		bv.URL = gen.FrameQuery(spec, false)
	}
	return bv
}
