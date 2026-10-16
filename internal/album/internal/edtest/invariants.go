package edtest

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// GridInvariants — инварианты сетки (design 007 §1) по всем листам (обложка — лист «-1»):
// после Load и после каждой правки. Фото стоит в одном месте на весь проект, место — в листе.
func GridInvariants(p *model.Project) error {
	placeIDs, inAlbum, layerIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	on2 := func(v float64) bool { return math.Abs(v*100-math.Round(v*100)) < 1e-6 }
	off := len(p.Sheets()) - len(p.Spreads)
	for k, s := range p.Sheets() {
		i := k - off
		g := p.Geo(s)
		sw, ph := g.W, g.H
		if p.Cover != nil && s == p.Cover.Sheet && (s.ID != model.CoverID || len(s.Grid) == 0 && len(s.Photos) > 0) {
			return fmt.Errorf("обложка: ID %q, «Авто» с фото %v", s.ID, s.Photos)
		}
		for _, id := range s.Photos {
			if inAlbum[id] {
				return fmt.Errorf("разворот %d: фото %s стоит дважды", i, id)
			}
			inAlbum[id] = true
		}
		if err := errors.Join(OrderInvariant(s), VigInvariant(s), PatInvariant(s), StickerInvariant(p, s, layerIDs)); err != nil {
			return fmt.Errorf("разворот %d: %w", i, err)
		}
		if s.Grid == nil {
			continue
		}
		if len(s.Grid) == 0 || len(s.Grid) > model.MaxPlaces {
			return fmt.Errorf("разворот %d: мест %d", i, len(s.Grid))
		}
		if !slices.Equal(s.Photos, model.GridPhotos(s)) {
			return fmt.Errorf("разворот %d: Photos %v не по местам %v", i, s.Photos, model.GridPhotos(s))
		}
		for _, pl := range s.Grid {
			switch {
			case pl == nil:
				return fmt.Errorf("разворот %d: nil-место", i)
			case !model.IsTextID(pl.ID) || placeIDs[pl.ID]:
				return fmt.Errorf("разворот %d: ID места %q", i, pl.ID)
			case pl.Fit != "" && pl.Fit != model.FitContain:
				return fmt.Errorf("разворот %d: fit %q", i, pl.Fit)
			case !sameShape(pl):
				return fmt.Errorf("разворот %d: форма %q, скругление %v", i, pl.Shape, pl.Round)
			case pl.Photo != "" && !HasRef(p, pl.Photo):
				return fmt.Errorf("разворот %d: фото места без записи %s", i, pl.Photo)
			case pl.W < geom.MinPlace || pl.H < geom.MinPlace || !HoldInside(pl, sw, ph):
				return fmt.Errorf("разворот %d: место вне разворота %+v", i, *pl)
			case !on2(pl.X) || !on2(pl.Y) || !on2(pl.W) || !on2(pl.H) || geom.NormDeg(pl.Rot) != pl.Rot:
				return fmt.Errorf("разворот %d: место не округлено %+v", i, *pl)
			}
			placeIDs[pl.ID] = true
		}
	}
	return nil
}

// sameShape — форма места уже зажата (044): правка и Load не оставляют чужого.
func sameShape(pl *model.Place) bool {
	s, r := model.ClampShape(pl.Shape, pl.Round)
	return s == pl.Shape && r == pl.Round
}

// VigInvariant — виньетка (031): Kind — "" или vignette, Vig ровно у виньетки, мест по
// параметрам ≤ MaxPlaces, шапка — надпись этого листа; роли мест известные и только у виньетки;
// своя форма (Own, 038) — ровно у шаблона "my:…".
func VigInvariant(s *model.Spread) error {
	if s.Kind != "" && s.Kind != model.KindVignette || (s.Kind == "") != (s.Vig == nil) {
		return fmt.Errorf("вид %q, параметры %+v", s.Kind, s.Vig)
	}
	if v := s.Vig; v != nil {
		head := v.Head == "" || slices.ContainsFunc(s.Texts, func(t *model.TextBlock) bool { return t.ID == v.Head })
		own := strings.HasPrefix(v.Tpl, model.VigOwnPrefix)
		if model.VigPlaces(*v) > model.MaxPlaces || v.Teachers > model.MaxTeachers || v.Pupils < 0 || !head || own != (v.Own != nil) {
			return fmt.Errorf("виньетка %+v", *v)
		}
	}
	for _, pl := range s.Grid {
		known := pl.Role == "" || pl.Role == model.RolePupil || pl.Role == model.RoleTeacher || pl.Role == model.RoleGroup
		if !known || s.Kind == "" && pl.Role != "" {
			return fmt.Errorf("роль места %q у вида %q", pl.Role, s.Kind)
		}
	}
	return nil
}

// PatInvariant — узор (048): на весь лист (Pat) или по страницам (PatL/PatR), не вместе — как
// фото фона; зажатый (ClampPattern — неподвижная точка).
func PatInvariant(s *model.Spread) error {
	if s.Pat != nil && (s.PatL != nil || s.PatR != nil) {
		return fmt.Errorf("узор листа %+v вместе с узором страницы %+v | %+v", *s.Pat, s.PatL, s.PatR)
	}
	for _, pt := range []*model.Pattern{s.Pat, s.PatL, s.PatR} {
		if pt != nil && !reflect.DeepEqual(model.ClampPattern(pt), pt) {
			return fmt.Errorf("узор не зажат: %+v", *pt)
		}
	}
	return nil
}

// OrderInvariant — Order: nil без мест и стикеров (у «Авто» только с текстами) и при порядке по
// умолчанию (места, стикеры, тексты), иначе перестановка мест, стикеров и текстов без чужих;
// Grid, Stickers и Texts — в его относительном порядке.
func OrderInvariant(s *model.Spread) error {
	if s.Order == nil {
		return nil
	}
	if len(s.Grid)+len(s.Stickers) == 0 {
		return fmt.Errorf("Order без мест и стикеров: %v", s.Order)
	}
	var grid, sts, texts []string
	for _, pl := range s.Grid {
		grid = append(grid, pl.ID)
	}
	for _, st := range s.Stickers {
		sts = append(sts, st.ID)
	}
	for _, t := range s.Texts {
		texts = append(texts, t.ID)
	}
	if slices.Equal(s.Order, slices.Concat(grid, sts, texts)) {
		return fmt.Errorf("Order по умолчанию не nil: %v", s.Order)
	}
	kind := s.Kinds()
	var og, ost, ot []string
	for _, id := range s.Order {
		switch kind[id] {
		case model.LayerPlace:
			og = append(og, id)
		case model.LayerSticker:
			ost = append(ost, id)
		case model.LayerText:
			ot = append(ot, id)
		}
	}
	if len(s.Order) != len(grid)+len(sts)+len(texts) || !slices.Equal(og, grid) || !slices.Equal(ost, sts) || !slices.Equal(ot, texts) {
		return fmt.Errorf("Order %v не перестановка мест %v, стикеров %v и текстов %v в их порядке", s.Order, grid, sts, texts)
	}
	return nil
}

// StickerInvariant — стикеры листа s (048): не больше MaxStickers, ID уникальны в проекте (и с
// местами и текстами — seen общий на проект), фото есть в Photos, зажатые (ClampSticker —
// неподвижная точка), своя пропорция держится (FitStickers ничего не меняет).
func StickerInvariant(p *model.Project, s *model.Spread, seen map[string]bool) error {
	if len(s.Stickers) > model.MaxStickers {
		return fmt.Errorf("стикеров %d", len(s.Stickers))
	}
	for _, pl := range s.Grid {
		seen[pl.ID] = true
	}
	for _, t := range s.Texts {
		seen[t.ID] = true
	}
	fit := p
	if len(s.Stickers) > 0 {
		fit = p.Clone()
		fit.FitStickers()
	}
	for _, st := range s.Stickers {
		switch {
		case st == nil:
			return fmt.Errorf("nil-стикер")
		case !model.IsTextID(st.ID) || seen[st.ID]:
			return fmt.Errorf("ID стикера %q", st.ID)
		case st.Photo != "" && !HasRef(p, st.Photo):
			return fmt.Errorf("фото стикера без записи %s", st.Photo)
		case !reflect.DeepEqual(p.ClampSticker(s, st), st):
			return fmt.Errorf("стикер не зажат: %+v", *st)
		}
		seen[st.ID] = true
		if cs, i := fit.FindSticker(st.ID); cs == nil || !reflect.DeepEqual(cs.Stickers[i], st) {
			return fmt.Errorf("пропорция стикера не держится: %+v", *st)
		}
	}
	return nil
}

// HoldInside — hold места внутри разворота sw×ph (у ровного — само место).
func HoldInside(pl *model.Place, sw, ph float64) bool {
	h := model.Hold(pl)
	return h.X >= -1e-6 && h.Y >= -1e-6 && h.X+h.W <= sw+1e-6 && h.Y+h.H <= ph+1e-6
}

func HasRef(p *model.Project, id string) bool { _, ok := p.Photos[id]; return ok }

func MustGrid(t *testing.T, e *album.Editor) {
	t.Helper()
	if err := GridInvariants(P(e)); err != nil {
		t.Fatal(err)
	}
}

// Same — правка холостая: rev и undo прежние.
func Same(t *testing.T, e *album.Editor, rev, undo int, what string) {
	t.Helper()
	if Rev(e) != rev || UndoLen(e) != undo {
		t.Fatalf("%s: rev %d → %d, undo %d → %d", what, rev, Rev(e), undo, UndoLen(e))
	}
}
