package view_test

import (
	"errors"
	"log"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Вид обложки с корешком (061): области раскладки, зоны броска, фото корешка в виде.

func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

// spineEd — обложка CoverEd в раскладке kind с пресетом preset.
func spineEd(t *testing.T, preset, kind string) *album.Editor {
	t.Helper()
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	if _, err := e.ApplyCoverPreset(preset); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetCoverLayout(kind); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestCoverScopes — области фото фона и подписи по раскладке отдаёт Go, в порядке карточки.
func TestCoverScopes(t *testing.T) {
	type sc = model.CoverScope
	for _, c := range []struct {
		kind string
		want []sc
	}{
		{"front", []sc{{Scope: "right", Label: "Лицевая"}}},
		{"wrap", []sc{{Scope: "spread", Label: "Вся обложка"}}},
		{"both", []sc{{Scope: "left", Label: "Задняя"}, {Scope: "spine", Label: "Корешок"}, {Scope: "right", Label: "Лицевая"}}},
	} {
		v := spineEd(t, model.CoverDefault, c.kind).View()
		if got := v.Cover.Scopes; !slices.Equal(got, c.want) {
			t.Errorf("%s: %+v, ждали %+v", c.kind, got, c.want)
		}
	}
}

// TestDropZonesBoth — в both три зоны броска без перекрытия и на всю ширину листа: задняя
// [0, X2], корешок [X2, X5], лицевая [X5, W] — и при расставе 9 (Нетпринт), и при 0;
// занятая зона пропадает; области фона сторонок прежние (с расставом).
func TestDropZonesBoth(t *testing.T) {
	for _, preset := range []string{model.CoverDefault, "netprint-premium"} {
		e := spineEd(t, preset, "both")
		v := e.View()
		cg, d := edtest.P(e).CoverGeo(), v.Cover.Drop
		if len(d) != 3 || d[0].Kind != "back" || d[1].Kind != "spine" || d[2].Kind != "front" || d[1].Label != "Корешок" {
			t.Fatalf("%s: зоны %+v", preset, d)
		}
		x := cg.X
		for i, want := range [][2]float64{{0, x[2]}, {x[2], x[5]}, {x[5], x[7]}} {
			r := d[i].Rect
			if !near(r.X, want[0]) || !near(r.X+r.W, want[1]) || r.Y != 0 || r.H != cg.Y[3] {
				t.Errorf("%s: зона %s %+v, ждали %v", preset, d[i].Kind, r, want)
			}
		}
		if preset != model.CoverDefault && x[3]-x[2] != 9 {
			t.Fatalf("расстав %v", x[3]-x[2])
		}
		edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, "x")
		if d := e.View().Cover.Drop; len(d) != 2 || d[0].Kind != "back" || d[1].Kind != "front" {
			t.Fatalf("%s: занятый корешок в зонах: %+v", preset, d)
		}
		sv := e.View().Cover.Sheet
		i := slices.IndexFunc(sv.Backdrops, func(b model.BackdropView) bool { return b.Scope == model.ScopeSpine })
		if i < 0 || i != len(sv.Backdrops)-1 || sv.Backdrops[i].Area.X != x[2] || !near(sv.Backdrops[i].Area.W, x[5]-x[2]) {
			t.Fatalf("%s: фото корешка в виде %+v", preset, sv.Backdrops)
		}
		if a, _ := edtest.P(e).AreaOf(edtest.P(e).Cover.Sheet, model.ScopeLeft); !near(a.W, x[3]) {
			t.Fatalf("%s: область задней изменилась: %+v", preset, a)
		}
	}
}

// TestDropZonesNarrowSpine — корешок уже 1 мм (свои размеры A=B=0, расстав 0) зону броска не
// даёт: в неё не попасть; задняя и лицевая сходятся на корешке.
func TestDropZonesNarrowSpine(t *testing.T) {
	e := spineEd(t, model.CoverDefault, "both")
	sp := edtest.P(e).Cover.Spec
	sp.Preset, sp.Steps, sp.A, sp.B, sp.Hinge = model.CoverCustom, nil, 0.5, 0, 0
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	cg := edtest.P(e).CoverGeo()
	d := e.View().Cover.Drop
	if cg.Spine >= 1 || len(d) != 2 || d[0].Kind != "back" || d[1].Kind != "front" {
		t.Fatalf("корешок %v: зоны %+v", cg.Spine, d)
	}
}

// narrowSpec — «Свои размеры» с корешком a мм без расставов (область корешка = a).
func narrowSpec(t *testing.T, e *album.Editor, a float64) {
	t.Helper()
	sp := edtest.P(e).Cover.Spec
	sp.Preset, sp.Steps, sp.A, sp.B, sp.Hinge = model.CoverCustom, nil, a, 0, 0
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	if got := edtest.P(e).BdArea(edtest.P(e).Cover.Sheet, model.ScopeSpine).W; got != a {
		t.Fatalf("область корешка %v, ждали %v", got, a)
	}
}

// TestNarrowSpineRule — корешок уже 1 мм — одно правило (decisions 31): нет «Корешка» в
// областях и зоны броска, SetBackdrop(spine) — ErrScope без шага; фото с прежнего широкого
// корешка в правке остаётся с рамкой > 0 (зажим идемпотентен), Load снимает его со строкой
// в log.txt, и Load∘Save∘Load = Load. Ровно 1 мм — корешок доступен.
func TestNarrowSpineRule(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	for _, c := range []struct {
		a  float64
		ok bool
	}{{0, false}, {0.99, false}, {1, true}} {
		e := spineEd(t, model.CoverDefault, "both")
		edtest.SetBg(t, e, model.CoverID, model.ScopeSpine, "x")
		narrowSpec(t, e, c.a)
		p := edtest.P(e)
		s := p.Cover.Sheet
		b := s.BgPhotoS
		if b == nil || b.W <= 0 || b.H <= 0 || !reflect.DeepEqual(p.ClampBackdrop(s, b, model.ScopeSpine), b) {
			t.Fatalf("%v мм: фото корешка в правке %+v", c.a, b)
		}
		// Нулевая рамка из файла (1e-9 → 0 при округлении) не ломает повторный зажим.
		tiny := p.ClampBackdrop(s, &model.Backdrop{Photo: "x", X: b.X, W: 1e-9, H: 1e-9, Rot: 30}, model.ScopeSpine)
		if tiny.W < model.MinSpine || tiny.H < model.MinSpine || !reflect.DeepEqual(p.ClampBackdrop(s, tiny, model.ScopeSpine), tiny) {
			t.Fatalf("%v мм: зажим крошечной рамки %+v", c.a, tiny)
		}
		// Рамка 0 × 0: при корешке уже MinSpine зажим не ставит её по области (меньше MinSpine) и идемпотентен.
		zero := p.ClampBackdrop(s, &model.Backdrop{Photo: "x"}, model.ScopeSpine)
		if zero.W <= 0 || zero.H <= 0 || (c.a < 1 && (zero.W < model.MinSpine || zero.H < model.MinSpine)) || !reflect.DeepEqual(p.ClampBackdrop(s, zero, model.ScopeSpine), zero) {
			t.Fatalf("%v мм: зажим нулевой рамки %+v", c.a, zero)
		}
		cv := e.View().Cover
		has := slices.ContainsFunc(cv.Scopes, func(x model.CoverScope) bool { return x.Scope == model.ScopeSpine })
		if has != c.ok || len(cv.Scopes) != map[bool]int{true: 3, false: 2}[c.ok] {
			t.Fatalf("%v мм: области %+v", c.a, cv.Scopes)
		}
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		_, err := e.SetBackdrop(model.CoverID, model.ScopeSpine, edtest.Photo("y"))
		switch {
		case c.ok && (err != nil || edtest.UndoLen(e) != undo+1):
			t.Fatalf("%v мм: фото на корешок: %v", c.a, err)
		case !c.ok && (!errors.Is(err, model.ErrScope) || !errors.Is(err, model.ErrSpineNarrow) || errors.Is(err, model.ErrSpineLayout)):
			t.Fatalf("%v мм: отказ корешка: %v", c.a, err)
		case !c.ok:
			edtest.Same(t, e, rev, undo, "отказ узкого корешка")
		}
		buf.Reset()
		q := edtest.SaveLoad(t, e)
		if got := q.Cover.Sheet.BgPhotoS != nil; got != c.ok {
			t.Fatalf("%v мм: после Load фото корешка %v, ждали %v", c.a, got, c.ok)
		}
		if logged := strings.Contains(buf.String(), "фото корешка снято: корешок меньше 1 мм"); logged == c.ok {
			t.Fatalf("%v мм: строка в log.txt %v:\n%s", c.a, logged, buf.String())
		}
		if q2 := edtest.SaveLoad(t, edtest.Open(t, q, "")); !reflect.DeepEqual(model.Marshal(q), model.Marshal(q2)) {
			t.Fatalf("%v мм: Load∘Save∘Load ≠ Load:\n%s\n%s", c.a, model.Marshal(q), model.Marshal(q2))
		}
		// Пустой узкий корешок: зоны броска нет, у 1 мм — есть.
		f := spineEd(t, model.CoverDefault, "both")
		narrowSpec(t, f, c.a)
		drop := slices.ContainsFunc(f.View().Cover.Drop, func(z model.CoverZone) bool { return z.Kind == "spine" })
		if drop != c.ok {
			t.Fatalf("%v мм: зона броска корешка %v", c.a, drop)
		}
	}
}
