package model

import (
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Фото корешка обложки (061): область, слоты, зажим и инвариант при чтении.

// Область корешка — [X2, X5] × [0, H] только у обложки: на развороте и листе 031 её нет;
// области сторонок прежние (Geo.Pages) — старые альбомы выглядят как раньше. Расстав 0 и 9.
func TestSpineArea(t *testing.T) {
	for _, id := range []string{"fsfera-thin", "netprint-premium"} {
		p := coverProject(t, 20, id)
		cg, s := p.CoverGeo(), p.Cover.Sheet
		a, ok := p.AreaOf(s, ScopeSpine)
		want := layout.Rect{X: cg.X[2], W: cg.X[5] - cg.X[2], H: cg.Y[3]}
		if !ok || !near2(a, want) {
			t.Fatalf("%s: корешок %+v %v, ждали %+v", id, a, ok, want)
		}
		if l, _ := p.AreaOf(s, ScopeLeft); l != p.Geo(s).Pages[0] {
			t.Errorf("%s: задняя изменилась: %+v", id, l)
		}
		if r, _ := p.AreaOf(s, ScopeRight); r != p.Geo(s).Pages[1] {
			t.Errorf("%s: лицевая изменилась: %+v", id, r)
		}
		if _, ok := p.AreaOf(p.Spreads[0], ScopeSpine); ok {
			t.Errorf("%s: корешок у разворота", id)
		}
	}
	sh := NewSheet(200, 300)
	if _, ok := sh.AreaOf(sh.Spreads[0], ScopeSpine); ok {
		t.Error("корешок у листа 031")
	}
	if _, ok := New().AreaOf(nil, ScopeSpine); ok {
		t.Error("корешок без листа")
	}
}

func near2(a, b layout.Rect) bool {
	d := func(x, y float64) bool { return x-y < 0.006 && y-x < 0.006 }
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}

// Страж обходов: у каждой области BdScopes есть слот фото, у каждой PatScopes — слот узора;
// корешок — только фото (узора на нём нет: PatSlot nil). Backdrops видит фото корешка.
func TestBdScopesSlots(t *testing.T) {
	s := &Spread{}
	for _, scope := range BdScopes() {
		slot := s.Slot(scope)
		if slot == nil || !ValidBdScope(scope) {
			t.Fatalf("область фото %q без слота", scope)
		}
		*slot = &Backdrop{Photo: scope}
	}
	for _, scope := range PatScopes() {
		if s.PatSlot(scope) == nil || !ValidScope(scope) {
			t.Fatalf("область узора %q без слота", scope)
		}
	}
	if s.PatSlot(ScopeSpine) != nil || ValidScope(ScopeSpine) || !ValidBdScope(ScopeSpine) || ValidBdScope("cover") || ValidBdScope("") {
		t.Fatal("корешок: узора и цвета нет, фото есть; чужое — нет")
	}
	if got := s.Backdrops(); len(got) != 4 || got[3].Photo != ScopeSpine || s.BgPhotoS.Photo != ScopeSpine {
		t.Fatalf("Backdrops: %v", got)
	}
	if s.Empty() {
		t.Fatal("лист с фото корешка пустой")
	}
	if !(&Spread{BgPhotoS: nil}).Empty() || (&Spread{BgPhotoS: &Backdrop{Photo: "x"}}).Empty() {
		t.Fatal("Empty не видит фото корешка")
	}
	if !errors.Is(ErrSpineLayout, ErrScope) || !strings.Contains(ErrSpineLayout.Error(), "Задняя + корешок + лицевая") {
		t.Fatalf("ошибка корешка: %v", ErrSpineLayout)
	}
}

// «Поворот» 90° на корешке: рамка поперёк узкой полосы не сжимается до 3 ширин корешка
// (предел — 3 большие стороны по обеим осям); у сторонок предел прежний. Пустая рамка — по области.
func TestSpineClampRotate(t *testing.T) {
	p := coverProject(t, 5, "fsfera-thin")
	s := p.Cover.Sheet
	p.Photos["a"] = PhotoRef{Path: "C:/a.jpg", W: 300, H: 200}
	a := p.BdArea(s, ScopeSpine)
	b := p.ClampBackdrop(s, &Backdrop{Photo: "a", X: a.X, Y: a.Y, W: a.H, H: a.W, Rot: 90}, ScopeSpine)
	if b == nil || b.W != a.H || b.H != a.W || b.Rot != 90 { // корешок 6 мм: меньше 10 мм можно
		t.Fatalf("повёрнутая рамка корешка %+v, область %+v", b, a)
	}
	if b := p.ClampBackdrop(s, &Backdrop{Photo: "a", X: a.X, W: 1e6, H: 1e6}, ScopeSpine); b.W != 3*a.H || b.H != 3*a.H {
		t.Fatalf("предел корешка %+v", b)
	}
	r := p.BdArea(s, ScopeRight)
	if b := p.ClampBackdrop(s, &Backdrop{Photo: "a", X: r.X, W: 1e6, H: 1e6}, ScopeRight); b.W != 3*r.W || b.H != 3*r.H {
		t.Fatalf("предел лицевой изменился: %+v", b)
	}
	if b := p.ClampBackdrop(s, &Backdrop{Photo: "a"}, ScopeSpine); b.X != a.X || b.W != a.W || b.H != a.H {
		t.Fatalf("пустая рамка корешка не по области: %+v", b)
	}
	if b := p.ClampBackdrop(s, &Backdrop{Photo: "a", X: a.X, W: 1, H: 1}, ScopeSpine); b.W != a.W || b.H != a.W {
		t.Fatalf("нижний предел корешка — его ширина: %+v", b)
	}
	if b := p.ClampBackdrop(s, &Backdrop{Photo: "a", X: r.X, W: 1, H: 1}, ScopeRight); b.W != minBackdrop || b.H != minBackdrop {
		t.Fatalf("нижний предел лицевой изменился: %+v", b)
	}
}

// Load: фото корешка живёт только у обложки в CoverBoth — на развороте, у обложки «лицевая»
// и «через корешок» снимается со строкой в log.txt; одно фото через лист снимает и корешок.
func TestSpineOnlyOnCover(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	bd := func() *Backdrop { return &Backdrop{Photo: "a", W: 5, H: 300} }
	for _, c := range []struct {
		layout string
		keep   bool
	}{{CoverBoth, true}, {CoverFront, false}, {CoverWrap, false}} {
		buf.Reset()
		p := coverProject(t, 3, "fsfera-thin")
		p.Photos["a"] = PhotoRef{Path: "C:/a.jpg", W: 300, H: 200}
		p.Cover.Layout = c.layout
		p.Cover.Sheet.BgPhotoS = bd()
		p.Spreads[0].BgPhotoS, p.Spreads[1].BgPhotoS = bd(), bd()
		p.ClampDesign()
		if got := p.Cover.Sheet.BgPhotoS != nil; got != c.keep {
			t.Errorf("%q: фото корешка у обложки %v, ждали %v", c.layout, got, c.keep)
		}
		if p.Spreads[0].BgPhotoS != nil || p.Spreads[1].BgPhotoS != nil {
			t.Errorf("%q: фото корешка у разворота осталось", c.layout)
		}
		want := 2
		if !c.keep {
			want = 3
		}
		if n := strings.Count(buf.String(), "фото корешка снято"); n != want {
			t.Errorf("%q: строк в log.txt %d, ждали %d", c.layout, n, want)
		}
	}
	p := coverProject(t, 3, "fsfera-thin")
	p.Photos["a"] = PhotoRef{Path: "C:/a.jpg", W: 300, H: 200}
	p.Cover.Layout = CoverBoth
	s := p.Cover.Sheet
	s.BgPhoto, s.BgPhotoS = bd(), bd()
	p.ClampDesign()
	if s.BgPhoto == nil || s.BgPhotoS != nil {
		t.Fatalf("одно фото через лист и корешок: %+v %+v", s.BgPhoto, s.BgPhotoS)
	}
}
