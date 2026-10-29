package view_test

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Вид, лист, части подписи и Pick вхождения (106 design §2): фронт переходит по ним, подпись не
// разбирает.

// Все виды сразу, несколько объектов одного вида: два стикера, фон левой и правой страницы,
// рамочка на месте с фото и на пустом месте — у каждого вхождения свой Pick.
func TestPhotoUsesPick(t *testing.T) {
	e := usesEd(t)
	edtest.Change(e, func(p *model.Project) {
		s1 := p.Spreads[0]
		s1.Stickers = append(s1.Stickers, &model.Sticker{ID: "c0ffee0000a2", Photo: "x", X: 50, Y: 10, W: 30, H: 30})
		s1.BgPhotoR = &model.Backdrop{Photo: "x", W: 100, H: 100}
		s1.Grid = append(s1.Grid, &model.Place{ID: "c0ffee0000b1", X: 10, Y: 150, W: 60, H: 60, Border: &model.Border{Photo: "x"}})
	})
	p := edtest.P(e)
	cv, s1, s2 := p.Cover.Sheet.ID, p.Spreads[0].ID, p.Spreads[1].ID
	if cv != model.CoverID {
		t.Fatalf("ID обложки: %q", cv)
	}
	uses, _ := e.PhotoUses("x")
	type row struct {
		sheet                              int
		kind, spread, page, what, pick, lb string
	}
	var got []row
	for _, u := range uses {
		got = append(got, row{u.Sheet, u.Kind, u.Spread, u.Page, u.What, u.Pick, u.Label})
	}
	want := []row{
		{0, model.UseBackdrop, cv, "Обложка", "фон", model.ScopeSpread, "Обложка — фон"},
		{1, model.UsePlace, s1, "Разворот 1", "место", "x", "Разворот 1 — место"},
		{1, model.UseBackdrop, s1, "Разворот 1", "фон слева", model.ScopeLeft, "Разворот 1 — фон слева"},
		{1, model.UseBackdrop, s1, "Разворот 1", "фон справа", model.ScopeRight, "Разворот 1 — фон справа"},
		{1, model.UseSticker, s1, "Разворот 1", "стикер 1", "c0ffee0000a1", "Разворот 1 — стикер 1"},
		{1, model.UseSticker, s1, "Разворот 1", "стикер 2", "c0ffee0000a2", "Разворот 1 — стикер 2"},
		{1, model.UseBorder, s1, "Разворот 1", "рамочка 1", "y", "Разворот 1 — рамочка 1"},            // место с фото: его запись
		{1, model.UseBorder, s1, "Разворот 1", "рамочка 2", "c0ffee0000b1", "Разворот 1 — рамочка 2"}, // пустое место: само место
		{2, model.UseBackdrop, s2, "Разворот 2", "фон", model.ScopeSpread, "Разворот 2 — фон"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("вхождения x:\n%+v\nждали\n%+v", got, want)
	}
}

// Копия на обложке и на развороте: Pick — запись вхождения (своя у каждой копии).
func TestPhotoUsesPickCopies(t *testing.T) {
	e := usesEd(t)
	c2 := model.CopyID("c", 2)
	edtest.Change(e, func(p *model.Project) {
		p.Photos[c2] = p.Photos["c"]
		p.Spreads[1].Photos = append(p.Spreads[1].Photos, c2)
	})
	uses, _ := e.PhotoUses("c")
	s2 := edtest.P(e).Spreads[1].ID
	if len(uses) != 2 || uses[0].Pick != "c" || uses[0].Spread != model.CoverID ||
		uses[1].Pick != c2 || uses[1].Spread != s2 || uses[1].Page != "Разворот 2" {
		t.Fatalf("семья c: %+v", uses)
	}
	uses, _ = e.PhotoUses(c2) // спросили копию — те же вхождения
	if len(uses) != 2 || uses[0].Pick != "c" || uses[1].Pick != c2 {
		t.Fatalf("семья c-2: %+v", uses)
	}
}

// Фон альбома: ни листа, ни частей «что», ни Pick; Label — одна страница.
func TestPhotoUsesPickAlbumBg(t *testing.T) {
	e := usesEd(t)
	edtest.Change(e, func(p *model.Project) { p.AlbumBg = &model.AlbumBg{Photo: "z"} })
	uses, _ := e.PhotoUses("z")
	if len(uses) != 2 {
		t.Fatalf("вхождения z: %+v", uses)
	}
	bg := uses[0]
	if bg.Sheet != -1 || bg.Kind != model.UseAlbumBg || bg.Spread != "" || bg.What != "" || bg.Pick != "" || bg.Page != "Фон альбома" || bg.Label != "Фон альбома" {
		t.Fatalf("фон альбома: %+v", bg)
	}
}

// Label == Page + " — " + What у каждого вхождения (фон альбома — Page); пустой список не nil.
func TestPhotoUsesLabelParts(t *testing.T) {
	e := usesEd(t)
	edtest.Change(e, func(p *model.Project) { p.AlbumBg = &model.AlbumBg{Photo: "x"} })
	uses, _ := e.PhotoUses("x")
	if len(uses) != 7 {
		t.Fatalf("вхождений %d: %+v", len(uses), uses)
	}
	for _, u := range uses {
		want := u.Page + " — " + u.What
		if u.What == "" {
			want = u.Page
		}
		if u.Label != want || u.Kind == "" || u.Page == "" {
			t.Errorf("%+v: Label %q, ждали %q", u, u.Label, want)
		}
	}
	if none, _ := e.PhotoUses("нет"); none == nil || len(none) != 0 {
		t.Fatalf("нет вхождений: %#v", none)
	}
}
