package view

import (
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Фон альбома в виде листа (098 design §4.2): узор или фото с Album: true по областям листа без
// своего фона. Геометрия — как у своего фона, поставленного щелчком по плитке (рамка = область,
// кадр от неё), поэтому холст и выгрузка рисуют его теми же путями. Тон к фону не применяется
// (098-23).

// albumBgScopes — области фона альбома на листе s: весь разворот или каждая страница; у листа
// с одной страницей (031) — весь лист.
func albumBgScopes(p *model.Project, s *model.Spread) []string {
	if p.AlbumBg.Pages {
		if _, ok := p.AreaOf(s, model.ScopeLeft); ok {
			return []string{model.ScopeLeft, model.ScopeRight}
		}
	}
	return []string{model.ScopeSpread}
}

// albumBgSheet — узоры или фото фона альбома в вид листа sv; лист со своим фоном, скрытым фоном
// альбома и обложка — без них (AlbumBgOn).
func albumBgSheet(p *model.Project, s *model.Spread, sv *model.SpreadView) {
	sv.NoAlbumBg = s.NoAlbumBg
	if !p.AlbumBgOn(s) {
		return
	}
	b := p.AlbumBg
	for _, scope := range albumBgScopes(p, s) {
		area := p.BdArea(s, scope)
		if b.Pat != nil {
			sv.Patterns = append(sv.Patterns, albumPattern(*b.Pat, scope, area, b.Fade))
			continue
		}
		ref := p.Photos[b.Photo]
		fr := model.Frame{S: 1}
		if b.Frame != nil {
			fr = *b.Frame
		}
		img := model.PlaceImage(layout.Rect{W: area.W, H: area.H}, p.RawAspect(b.Photo), fr)
		sv.Backdrops = append(sv.Backdrops, model.BackdropView{Scope: scope, Photo: b.Photo, Area: area,
			Box: model.BackdropBox{X: area.X, Y: area.Y, W: area.W, H: area.H}, Img: img, Frame: fr,
			FlipH: b.FlipH, FlipV: b.FlipV, DPI: geom.FrameDPI(ref.W, img), Fade: b.Fade, Lib: ref.Lib, Album: true})
	}
}

// albumPattern — вид узора фона альбома, как patternViews у своего; прозрачность — фона альбома.
func albumPattern(pt model.Pattern, scope string, area layout.Rect, fade float64) model.PatternView {
	spec, known := gen.Resolve(gen.Spec{Kind: pt.Kind, Inks: pt.Inks, Scale: pt.Scale, Seed: pt.Seed})
	if spec.Scale == 0 { // неизвестный генератор Resolve не дополняет
		spec.Scale = 1
	}
	pv := model.PatternView{Scope: scope, Area: area, Known: known, Preset: pt.Preset, Album: true,
		Pat: model.Pattern{Kind: pt.Kind, Inks: spec.Inks, Scale: spec.Scale, Seed: spec.Seed, Fade: fade}}
	if m, ok := gen.Lookup(pt.Kind); ok {
		pv.Name, pv.Inks, pv.Random = m.Name, len(m.Inks), m.Random
		pv.URL = gen.Query(spec, area.W, area.H)
	}
	return pv
}

// albumBgView — строка «Фон альбома» настроек; nil — фона нет. Имя своего из библиотеки UI
// берёт из каталога по Lib (как у фото фона); фото без ключа — по имени файла.
func albumBgView(p *model.Project) *model.AlbumBgView {
	b := p.AlbumBg
	if b == nil {
		return nil
	}
	v := &model.AlbumBgView{Fade: b.Fade, Pages: b.Pages}
	if b.Pat != nil {
		v.Preset = b.Pat.Preset
		if m, ok := gen.Lookup(b.Pat.Kind); ok {
			v.Name = m.Name
		}
		return v
	}
	ref := p.Photos[b.Photo]
	v.Photo, v.Lib = b.Photo, ref.Lib
	if ref.Lib == "" && ref.Path != "" {
		base := filepath.Base(ref.Path)
		v.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return v
}
