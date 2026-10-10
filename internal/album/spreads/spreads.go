package spreads

// Правки состава альбома: фото и развороты, настройки, папки, автосборка,
// разрезание и слияние разворотов. Все идут через Editor.change/try.

import (
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// AddPhotos — операция над проектом; photos уже без повторов.
func AddPhotos(p *model.Project, spreadID string, photos []model.PhotoInfo, index int) error {
	ids := model.Remember(p, photos)
	_, s := p.Find(spreadID)
	if s == nil {
		return nil
	}
	if len(s.Grid) > 0 {
		return addToGrid(p, s, ids)
	}
	// Позиция считается до удаления: поправим её, если фото уже были выше в этом же развороте.
	for _, id := range ids {
		if j := slices.Index(s.Photos, id); j >= 0 && index >= 0 && j < index {
			index--
		}
	}
	p.Detach(ids)
	s.Insert(ids, index)
	if len(s.Photos) > model.MaxPhotos {
		return model.ErrTooMany()
	}
	return nil
}

// NewSpread — операция над проектом; photos уже без повторов.
func NewSpread(p *model.Project, at int, photos []model.PhotoInfo) error {
	ids := model.Remember(p, photos)
	if len(ids) > model.MaxPhotos {
		return model.ErrTooMany()
	}
	p.Detach(ids)
	s := &model.Spread{ID: model.NewID(), Photos: ids}
	if at < 0 || at > len(p.Spreads) {
		at = len(p.Spreads)
	}
	p.Spreads = slices.Insert(p.Spreads, at, s)
	return nil
}

// MovePhoto — операция над проектом; неизвестное фото — холостая правка.
func MovePhoto(p *model.Project, photoID, toSpread string, index int) error {
	ref, ok := p.Photos[photoID]
	if !ok {
		return nil
	}
	return AddPhotos(p, toSpread, []model.PhotoInfo{{ID: photoID, Path: ref.Path, W: ref.W, H: ref.H, Size: ref.Size}}, index)
}

// SwapPhotos — операция над проектом.
func SwapPhotos(p *model.Project, a, b string) {
	if p.Placed(a) && p.Placed(b) {
		p.Swap(a, b)
	}
}

// RemovePhotos — операция над проектом.
func RemovePhotos(p *model.Project, ids []string) { p.Detach(ids) }

// DeleteSpread — операция над проектом.
func DeleteSpread(p *model.Project, id string) { DeleteSpreads(p, []string{id}) }

// DeleteSpreads — операция над проектом.
func DeleteSpreads(p *model.Project, ids []string) {
	ids = ids[:min(len(ids), model.MaxSpreads)]
	del := make(map[string]bool, len(ids))
	for _, id := range ids {
		del[id] = true
	}
	p.Spreads = slices.DeleteFunc(p.Spreads, func(s *model.Spread) bool { return del[s.ID] })
}

// MoveSpread — операция над проектом.
func MoveSpread(p *model.Project, id string, to int) {
	i, s := p.Find(id)
	if s == nil {
		return
	}
	p.Spreads = slices.Delete(p.Spreads, i, i+1)
	to = max(0, min(to, len(p.Spreads)))
	p.Spreads = slices.Insert(p.Spreads, to, s)
}

// SetSettings — операция над проектом: зажим размеров и цвет фона.
func SetSettings(p *model.Project, s model.Settings) {
	s = model.ClampSettings(s)
	s.Bg = model.BgColor(s.Bg)
	p.Settings = s
}

// AddFolder — операция над проектом. Путь проверяет и обёртка, но тело экспортное: проверка
// повторена здесь (защита в глубину), иначе вызов мимо Editor положил бы в проект UNC-путь.
func AddFolder(p *model.Project, dir string) error {
	if err := paths.CheckLocalPath(dir); err != nil {
		return err
	}
	switch {
	case slices.Contains(p.Folders, dir):
	case len(p.Folders) >= model.MaxFolders:
		return fmt.Errorf("в проекте не может быть больше %d папок", model.MaxFolders)
	default:
		p.Folders = append(p.Folders, dir)
	}
	return nil
}

// RemoveFolder — операция над проектом.
func RemoveFolder(p *model.Project, dir string) {
	p.Folders = slices.DeleteFunc(p.Folders, func(f string) bool { return f == dir })
}

// AutoBuild — операция над проектом; photos уже без повторов. Возвращает число новых разворотов.
func AutoBuild(p *model.Project, photos []model.PhotoInfo, at, perSpread int) (made int, err error) {
	inGrid, onBg := gridPhotoSet(p), p.OnBackdrops()
	ids := model.Remember(p, slices.DeleteFunc(slices.Clone(photos), func(ph model.PhotoInfo) bool { return inGrid[ph.ID] || onBg[ph.ID] }))
	p.Detach(ids)
	groups := layout.Group(p.Aspects(ids), p.Params(), min(perSpread, model.MaxSpreadPhotos))
	if at < 0 || at > len(p.Spreads) {
		at = len(p.Spreads)
	}
	var spreads []*model.Spread
	for _, g := range groups {
		// Group сам не даёт больше 7 на разворот; режем и здесь, чтобы склейка
		// и автосборка держались одной границы.
		for i := g[0]; i < g[1]; i += model.MaxSpreadPhotos {
			spreads = append(spreads, &model.Spread{ID: model.NewID(), Photos: slices.Clone(ids[i:min(g[1], i+model.MaxSpreadPhotos)])})
		}
	}
	if len(spreads) == 0 {
		return 0, nil
	}
	if len(p.Spreads)+len(spreads) > model.MaxSpreads {
		return 0, fmt.Errorf("автосборка даст больше %d разворотов: в альбоме их не может быть больше", model.MaxSpreads)
	}
	p.Spreads = slices.Insert(p.Spreads, at, spreads...)
	return len(spreads), nil
}

// SplitSpread — операция над проектом.
func SplitSpread(p *model.Project, id string, at int) {
	i, s := p.Find(id)
	if s == nil || at < 1 || at >= len(s.Photos) || len(s.Grid) > 0 {
		return
	}
	tail := &model.Spread{ID: model.NewID(), Photos: slices.Clone(s.Photos[at:])}
	s.Photos = slices.Clone(s.Photos[:at])
	s.Layout = ""
	p.Spreads = slices.Insert(p.Spreads, i+1, tail)
}

// MergeSpreads — операция над проектом.
func MergeSpreads(p *model.Project, id string) {
	i, s := p.Find(id)
	if s == nil || i+1 >= len(p.Spreads) {
		return
	}
	next := p.Spreads[i+1]
	if len(s.Grid) > 0 || len(next.Grid) > 0 ||
		len(s.Photos)+len(next.Photos) > model.MaxSpreadPhotos || len(s.Texts)+len(next.Texts) > model.MaxTexts {
		return
	}
	s.Photos = append(slices.Clone(s.Photos), next.Photos...)
	s.Texts = append(s.Texts, next.Texts...)
	if s.Name == "" {
		s.Name = next.Name
	}
	// Фон по сторонам: нет своего цвета у первого — берётся второго.
	if s.BgL == "" {
		s.BgL = next.BgL
	}
	if s.BgR == "" {
		s.BgR = next.BgR
	}
	if len(s.Backdrops()) == 0 { // фоны первого; без них — второго
		s.BgPhoto, s.BgPhotoL, s.BgPhotoR = next.BgPhoto, next.BgPhotoL, next.BgPhotoR
	}
	s.Layout = ""
	p.Spreads = slices.Delete(p.Spreads, i+1, i+2)
}

// addToGrid — AddPhotos (и MovePhoto) в разворот-сетку: всё или ничего, каждое
// фото — в первое пустое место; фото этой сетки остаются где стоят (перестановка
// чипа ленты — холостая: порядок задают места). Мест не хватило — errNoPlace.
func addToGrid(p *model.Project, s *model.Spread, ids []string) error {
	for _, id := range ids {
		if slices.Contains(model.GridPhotos(s), id) {
			continue
		}
		i := slices.IndexFunc(s.Grid, func(pl *model.Place) bool { return pl.Photo == "" })
		if i < 0 {
			return model.ErrNoPlace
		}
		p.Put(s, s.Grid[i], id)
	}
	return nil
}

// gridPhotoSet — фото, стоящие в сетках: автосборка их не трогает.
func gridPhotoSet(p *model.Project) map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Spreads {
		for _, pl := range s.Grid {
			if pl.Photo != "" {
				out[pl.Photo] = true
			}
		}
	}
	return out
}

// DeleteSelection — операция над проектом (Editor.DeleteSelection; порядок описан там).
func DeleteSelection(p *model.Project, spreadID string, photos, places, texts []string) {
	for _, id := range texts {
		p.DeleteText(id)
	}
	p.Detach(photos)
	p.DeletePlaces(spreadID, places)
}
