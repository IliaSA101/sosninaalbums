package backdrop_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// SetPattern с областью, где ключ узора не проходит зажим (в живом проекте такого не бывает —
// конвейер правки и Load зажимают узоры): тело не должно записывать nil поверх слота; соседние
// области и другой разворот правятся как обычно.
func TestSetPatternBadSlotKind(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	apply(t, e, s1, "right", model.Pattern{Kind: "dots"}, "")
	apply(t, e, s2, "left", model.Pattern{Kind: "dots"}, "")
	p := edtest.P(e).Clone()
	bad := &model.Pattern{Kind: "BAD!", Fade: 0.2}
	p.Sheet(s1).PatL = bad

	backdrop.SetPattern(p, s1, "left", model.Pattern{Fade: 0.8})
	if got := p.Sheet(s1).PatL; got != bad || got.Fade != 0.2 || got.Kind != "BAD!" {
		t.Fatalf("слот с плохим ключом изменён: %+v", got)
	}
	backdrop.SetPattern(p, s1, "right", model.Pattern{Fade: 0.8})
	backdrop.SetPattern(p, s2, "left", model.Pattern{Fade: 0.6})
	if p.Sheet(s1).PatR.Fade != 0.8 || p.Sheet(s2).PatL.Fade != 0.6 {
		t.Fatalf("соседние области: %+v %+v", p.Sheet(s1).PatR, p.Sheet(s2).PatL)
	}
}
