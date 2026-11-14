package export

import (
	"cmp"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Рамочки мест в задании экспорта (079 design §4): фото садится в окно рамочки (FrameView.Win,
// посчитано в View), рамочка рисуется отдельным слоем над своим фото. Срез картинки берётся из
// альбома, как его зажал ClampBorder, — тот же, что у окна фото и у /media/f.

// placedPlaces — места листа с фото, по порядку s.Photos (Rects и Frames идут в нём же). nil — места
// не соответствуют фото (лист «Авто» или несовпадение): рамочек нет.
func placedPlaces(s model.SpreadView) []model.PlaceView {
	if !s.Grid {
		return nil
	}
	out := make([]model.PlaceView, 0, len(s.Photos))
	for _, pv := range s.Places {
		if pv.Photo != "" {
			out = append(out, pv)
		}
	}
	if len(out) != len(s.Photos) {
		return nil
	}
	for i, pv := range out {
		if pv.Photo != s.Photos[i] {
			return nil
		}
	}
	return out
}

// borderCut — срез и размер картинки-рамочки места из альбома: View его не несёт. false — места
// в альбоме нет (вид и проект разошлись): рамочку не рисуем.
func borderCut(p *model.Project, sheet, placeID string) (*model.Border, bool) {
	sp := p.Sheet(sheet)
	if sp == nil {
		return nil, false
	}
	for _, pl := range sp.Grid {
		if pl.ID == placeID && pl.Border != nil {
			return pl.Border, true
		}
	}
	return nil, false
}

// applyBorder переводит рамочку места pv в задание: окно фото (Cell, форма, Pivot — рамка места,
// вокруг центра которой поворот), маска окна и сама рамочка. rect — рамка места, f — кадр фото.
func applyBorder(p *model.Project, sheet string, pv model.PlaceView, f model.FrameView, ph *render.Photo, es *render.Spread) {
	b := pv.Border
	if b == nil || !b.Known {
		return // нет рамочки или вид новее программы: окно = место, рисовать нечего
	}
	box := layout.Rect{X: pv.X, Y: pv.Y, W: pv.W, H: pv.H}
	ph.Cell = f.Win
	ph.Outline = geom.Outline{Oval: f.WinShape == model.ShapeOval, Radius: f.WinRadius}
	ph.Pivot = box
	oval := pv.Shape == model.ShapeOval
	bd := render.BorderDraw{Box: box, Rot: pv.Rot, Z: f.Z}
	switch {
	case b.Pic:
		src, ok := borderCut(p, sheet, pv.ID)
		if !ok {
			return
		}
		// Файл из вида печати (079-40): у картинки, загруженной и в «Фоны», запись общая с фоном.
		bd.Pic, bd.Path, bd.Scale, bd.Cut = true, cmp.Or(b.File, p.Photos[b.Photo].Path), b.Scale, src.Cut
		if !src.Open {
			ph.Win = &render.WinMask{Box: box, Rot: pv.Rot, Path: bd.Path, Scale: b.Scale, Cut: src.Cut}
		}
	case render.FrameHasMask(b.Kind):
		ph.Win = &render.WinMask{Box: box, Rot: pv.Rot, Kind: b.Kind, Width: b.Width, Oval: oval, Radius: pv.Radius}
		return // мягкий край ничего не рисует поверх
	default:
		bd.Kind, bd.Inks, bd.Width, bd.Oval, bd.Radius = b.Kind, b.Inks, b.Width, oval, pv.Radius
	}
	es.Borders = append(es.Borders, bd)
}

// borderPhotos — записи Photos, на которые ссылаются рамочки-картинки мест листа с фото (пустое
// место рамочки не печатает, и его файл экспорт не держит).
func borderPhotos(s model.SpreadView, out map[string]bool) {
	for _, pv := range s.Places {
		if pv.Photo != "" && pv.Border != nil && pv.Border.Pic && pv.Border.Photo != "" {
			out[pv.Border.Photo] = true
		}
	}
}

// borderAABB — описанная рамка места pv под поворотом, мм листа.
func borderAABB(pv model.PlaceView) layout.Rect {
	return geom.RotRect{X: pv.X, Y: pv.Y, W: pv.W, H: pv.H, Rot: pv.Rot}.AABB()
}

// borderPhotoWants — файлы рамочек-картинок листа i (с 1), которые попадут в печать по плану.
func (pl Plan) borderPhotoWants(s model.SpreadView, i int, pageW float64, out map[string]bool) {
	for _, pv := range s.Places {
		if pv.Photo != "" && pv.Border != nil && pv.Border.Pic && pv.Border.Photo != "" && pl.wants(i, borderAABB(pv), pageW) {
			out[pv.Border.Photo] = true
		}
	}
}

// MissingBorderFiles — ошибка, если файл печати рамочки-картинки (BorderView.File: копия из
// «Рамочек», а не путь записи Photos) не на месте или его том не отвечает. Записи Photos проверяет
// MissingPhotos; копия лежит в папке данных и там не видна. Без проверки render открыл бы её сам и
// бросил весь холст (на сетевом томе — после таймаута). exists — как у MissingPhotos (том и файл).
func (pl Plan) MissingBorderFiles(v model.View, pageW float64, exists func(string) bool) error {
	seen := map[string]bool{}
	var gone []string
	check := func(s model.SpreadView, want func(model.PlaceView) bool) {
		for _, pv := range s.Places {
			b := pv.Border
			if pv.Photo == "" || b == nil || !b.Pic || b.File == "" || !want(pv) || seen[b.File] {
				continue
			}
			seen[b.File] = true
			if !exists(b.File) {
				gone = append(gone, filepath.Base(b.File))
			}
		}
	}
	if pl.Cover != nil && v.Cover != nil {
		check(v.Cover.Sheet, func(model.PlaceView) bool { return true })
	}
	for i, s := range v.Spreads {
		check(s, func(pv model.PlaceView) bool { return pl.wants(i+1, borderAABB(pv), pageW) })
	}
	if len(gone) == 0 {
		return nil
	}
	list := strings.Join(gone[:min(len(gone), 3)], ", ")
	if len(gone) > 3 {
		list += fmt.Sprintf(" …и ещё %d", len(gone)-3)
	}
	return fmt.Errorf("не найдены картинки рамочек: %d (%s); загрузите их заново в «Рамочки» — и повторите выгрузку", len(gone), list)
}

// ownBorders — ID записей Photos, на которые ссылаются рамочки мест проекта (с обложкой): пропавшее
// своё у них называется «своя рамочка».
func ownBorders(p *model.Project) map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.BorderPhotos() {
			out[id] = true
		}
	}
	return out
}
