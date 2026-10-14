package edtest

import (
	"errors"
	"fmt"
	"math"
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
	placeIDs, inAlbum := map[string]bool{}, map[string]bool{}
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
		if err := errors.Join(OrderInvariant(s), VigInvariant(s)); err != nil {
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

// OrderInvariant — Order: nil у «Авто» и при порядке по умолчанию, иначе перестановка
// мест и текстов; Grid и Texts — в его относительном порядке.
func OrderInvariant(s *model.Spread) error {
	if s.Order == nil {
		return nil
	}
	if len(s.Grid) == 0 {
		return fmt.Errorf("Order у «Авто»: %v", s.Order)
	}
	var grid, texts []string
	for _, pl := range s.Grid {
		grid = append(grid, pl.ID)
	}
	for _, t := range s.Texts {
		texts = append(texts, t.ID)
	}
	if slices.Equal(s.Order, append(slices.Clone(grid), texts...)) {
		return fmt.Errorf("Order по умолчанию не nil: %v", s.Order)
	}
	kind := s.Kinds()
	var og, ot []string
	for _, id := range s.Order {
		switch kind[id] {
		case model.LayerPlace:
			og = append(og, id)
		case model.LayerText:
			ot = append(ot, id)
		}
	}
	if len(s.Order) != len(grid)+len(texts) || !slices.Equal(og, grid) || !slices.Equal(ot, texts) {
		return fmt.Errorf("Order %v не перестановка мест %v и текстов %v в их порядке", s.Order, grid, texts)
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
