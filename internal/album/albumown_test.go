package album_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Как в альбоме» (115): цвет и фон альбома против своих у разворотов — SetSettingsToAll,
// SetAlbumBgToAll, SpreadBgToAlbum. Один шаг отмены, обложка не тронута.

const ownCoverPhoto = "cvbg"

// ownColorEd — обложка со своим цветом левой сторонки и четыре разворота: свой цвет у s1 только
// слева, у s2 на обеих страницах, у s3 только справа, у s4 нет.
func ownColorEd(t *testing.T) *album.Editor {
	t.Helper()
	e := albumBgEd(t)
	e.SetBackground("s1", model.ScopeLeft, "#112233")
	e.SetBackground("s2", model.ScopeSpread, "#445566")
	e.SetBackground("s3", model.ScopeRight, "#778899")
	edtest.Change(e, func(p *model.Project) { p.Cover.Sheet.BgL = "#aabbcc" })
	return e
}

// bgsOf — свои цвета страниц разворотов s1…s4 и обложки подряд.
func bgsOf(e *album.Editor) []string {
	p := edtest.P(e)
	var out []string
	for _, s := range append(slices.Clone(p.Spreads), p.Cover.Sheet) {
		out = append(out, s.BgL+"|"+s.BgR)
	}
	return out
}

func TestSetSettingsToAll(t *testing.T) {
	e := ownColorEd(t)
	if want := []string{"#112233|", "#445566|#445566", "|#778899", "|", "#aabbcc|"}; !slices.Equal(bgsOf(e), want) {
		t.Fatalf("исходные цвета: %v, ждали %v", bgsOf(e), want)
	}
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	st := edtest.P(e).Settings
	st.Bg = "#eef2e6"
	v := e.SetSettingsToAll(st)

	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("не один шаг: %d → %d", undo, edtest.UndoLen(e))
	}
	if want := []string{"|", "|", "|", "|", "#aabbcc|"}; !slices.Equal(bgsOf(e), want) {
		t.Fatalf("свои цвета: %v, ждали %v", bgsOf(e), want)
	}
	if v.Settings.Bg != "#eef2e6" {
		t.Fatalf("цвет альбома: %q", v.Settings.Bg)
	}
	for _, sv := range v.Spreads {
		if sv.BgSet != [2]string{} || sv.Bg != [2]string{"#eef2e6", "#eef2e6"} {
			t.Fatalf("%s: bgSet %v, bg %v", sv.ID, sv.BgSet, sv.Bg)
		}
	}

	// Круг Save → Load: развороты без своего цвета, обложка со своим.
	q := edtest.SaveLoad(t, e)
	if q.Settings.Bg != "#eef2e6" || q.Cover.Sheet.BgL != "#aabbcc" {
		t.Fatalf("после Save→Load: %q, обложка %q", q.Settings.Bg, q.Cover.Sheet.BgL)
	}
	for _, s := range q.Spreads {
		if s.BgL != "" || s.BgR != "" {
			t.Fatalf("после Save→Load %s: %q|%q", s.ID, s.BgL, s.BgR)
		}
	}

	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена вернула не всё")
	}
	if v = e.Redo(); v.Settings.Bg != "#eef2e6" {
		t.Fatalf("повтор: %q", v.Settings.Bg)
	}
}

// TestSetSettingsToAllNoop — тот же цвет и своих нет: ни rev, ни шага; свой цвет при тех же
// настройках — правка (так «Как в альбоме» работает и без смены цвета).
func TestSetSettingsToAllNoop(t *testing.T) {
	e := albumBgEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetSettingsToAll(edtest.P(e).Settings)
	edtest.Same(t, e, rev, undo, "своих нет, настройки те же")

	e.SetBackground("s2", model.ScopeLeft, "#445566")
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetSettingsToAll(edtest.P(e).Settings)
	if edtest.Rev(e) == rev || edtest.UndoLen(e) != undo+1 || edtest.P(e).Spreads[1].BgL != "" {
		t.Fatal("свой цвет при тех же настройках: нужен один шаг со снятием")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetSettingsToAll(edtest.P(e).Settings)
	edtest.Same(t, e, rev, undo, "повтор")
}

// ownBgEd — фон альбома (узор), разворот s1 с узором и цветом, s2 с фото на левой странице,
// s3 только скрыт, s4 скрыт со своим фото; на обложке своё фото фона и цвет.
func ownBgEd(t *testing.T) *album.Editor {
	t.Helper()
	e := albumBgEd(t)
	setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	if _, err := e.ApplyPattern("s1", model.ScopeSpread, model.Pattern{Kind: "dots"}, "#f5efe6"); err != nil {
		t.Fatal(err)
	}
	edtest.SetBg(t, e, "s2", model.ScopeLeft, "x")
	e.SetNoAlbumBg("s3", true)
	edtest.SetBg(t, e, "s4", model.ScopeRight, "y")
	e.SetNoAlbumBg("s4", true)
	edtest.Change(e, func(p *model.Project) {
		p.Photos[ownCoverPhoto] = model.PhotoRef{Path: edtest.Abs(ownCoverPhoto + ".jpg"), W: 300, H: 200}
		p.Cover.Sheet.BgPhoto = &model.Backdrop{Photo: ownCoverPhoto}
		p.Cover.Sheet.BgL = "#aabbcc"
	})
	return e
}

func TestSetAlbumBgToAll(t *testing.T) {
	e := ownBgEd(t)
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	v, err := e.SetAlbumBgToAll(model.BgSource{Photo: albumBgLib()}, true)
	if err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("не один шаг: %d → %d", undo, edtest.UndoLen(e))
	}
	p := edtest.P(e)
	for i, s := range p.Spreads {
		if len(s.Patterns()) != 0 || len(s.Backdrops()) != 0 {
			t.Fatalf("s%d: свой фон остался", i+1)
		}
		// Скрыт остаётся только у s3: своего фона у него не было (q115-5); s4 скрыт со своим — снят.
		if s.NoAlbumBg != (i == 2) {
			t.Fatalf("s%d: NoAlbumBg %v", i+1, s.NoAlbumBg)
		}
	}
	if p.Spreads[0].BgL != "#f5efe6" {
		t.Fatalf("цвет разворота с узором должен остаться: %q", p.Spreads[0].BgL)
	}
	if p.Cover.Sheet.BgPhoto == nil || p.Cover.Sheet.BgL != "#aabbcc" {
		t.Fatal("обложка тронута")
	}
	for _, id := range []string{"x", "y"} {
		if _, ok := p.Photos[id]; ok {
			t.Fatalf("запись фото %s фона осталась", id)
		}
	}
	if v.AlbumBg == nil || v.AlbumBg.Photo == "" || !v.AlbumBg.Pages {
		t.Fatalf("фон альбома: %+v", v.AlbumBg)
	}
	for _, sv := range v.Spreads {
		if sv.NoAlbumBg != (sv.ID == "s3") {
			t.Fatalf("%s: скрыт %v", sv.ID, sv.NoAlbumBg)
		}
	}

	// Круг Save → Load: фон альбома — фото, своих фонов нет.
	edtest.LibFinder(t, map[string]string{albumBgKey: edtest.Abs("albumbg.jpg")})
	q := edtest.SaveLoad(t, e)
	if q.AlbumBg == nil || q.Spreads[0].Pat != nil || q.Spreads[1].BgPhotoL != nil || q.Spreads[3].NoAlbumBg || !q.Spreads[2].NoAlbumBg || q.Cover.Sheet.BgPhoto == nil {
		t.Fatalf("после Save→Load: %+v", q.AlbumBg)
	}

	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена вернула не всё")
	}
}

// TestSetAlbumBgToAllSame — тот же фон альбома: при своих фонах шаг только из снятия, без них —
// холостая; ошибка источника не снимает ничего.
func TestSetAlbumBgToAllSame(t *testing.T) {
	e := ownBgEd(t)
	src := model.BgSource{Pattern: &albumBgPat}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	before := model.Marshal(edtest.P(e))

	if _, err := e.SetAlbumBgToAll(model.BgSource{}, false); !errors.Is(err, model.ErrPattern) {
		t.Fatalf("пустой источник: %v", err)
	}
	if _, err := e.SetAlbumBgToAll(model.BgSource{Pattern: &albumBgPat, Photo: albumBgLib()}, false); !errors.Is(err, model.ErrPattern) {
		t.Fatalf("два источника: %v", err)
	}
	edtest.Same(t, e, rev, undo, "ошибка источника")
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("ошибка источника тронула проект")
	}

	if _, err := e.SetAlbumBgToAll(src, false); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 || len(edtest.P(e).Spreads[0].Patterns()) != 0 {
		t.Fatal("тот же фон при своих: нужен один шаг со снятием")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SetAlbumBgToAll(src, false); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "тот же фон, своих нет")
}

func TestSpreadBgToAlbum(t *testing.T) {
	e := ownBgEd(t)
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)

	v := e.SpreadBgToAlbum("s4") // скрыт со своим фото: оба сняты
	s4 := edtest.P(e).Spreads[3]
	if edtest.UndoLen(e) != undo+1 || s4.NoAlbumBg || len(s4.Backdrops()) != 0 {
		t.Fatalf("s4: шаг %d, скрыт %v, фото %d", edtest.UndoLen(e)-undo, s4.NoAlbumBg, len(s4.Backdrops()))
	}
	if len(edtest.P(e).Spreads[1].Backdrops()) != 1 {
		t.Fatal("тронут чужой разворот")
	}
	if v.Spreads[3].NoAlbumBg {
		t.Fatalf("вид: %+v", v.Spreads[3])
	}
	e.SpreadBgToAlbum("s3") // только скрыт: «здесь как в альбоме»
	if edtest.P(e).Spreads[2].NoAlbumBg {
		t.Fatal("s3 остался скрытым")
	}
	e.SpreadBgToAlbum("s1") // узор снят, цвет не тронут
	if s1 := edtest.P(e).Spreads[0]; len(s1.Patterns()) != 0 || s1.BgL != "#f5efe6" {
		t.Fatalf("s1: узоры %d, цвет %q", len(s1.Patterns()), s1.BgL)
	}

	// Холостые: повтор, обложка, неизвестный ID.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SpreadBgToAlbum("s1")
	e.SpreadBgToAlbum(model.CoverID)
	e.SpreadBgToAlbum("нет-такого")
	edtest.Same(t, e, rev, undo, "холостые")
	if edtest.P(e).Cover.Sheet.BgPhoto == nil {
		t.Fatal("обложка тронута")
	}

	q := edtest.SaveLoad(t, e)
	if q.Spreads[0].Pat != nil || q.Spreads[2].NoAlbumBg || q.Spreads[3].BgPhotoR != nil || q.Spreads[1].BgPhotoL == nil {
		t.Fatal("после Save→Load")
	}
	e.Undo()
	e.Undo()
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("три шага отмены не вернули проект")
	}
}

// TestSpreadBgToAlbumNoAlbumBg — фона альбома нет: правка холостая (свой фон и «скрыт» остаются).
func TestSpreadBgToAlbumNoAlbumBg(t *testing.T) {
	e := albumBgEd(t)
	edtest.SetBg(t, e, "s2", model.ScopeLeft, "x")
	e.SetNoAlbumBg("s3", true)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SpreadBgToAlbum("s2")
	e.SpreadBgToAlbum("s3")
	edtest.Same(t, e, rev, undo, "фона альбома нет")
}
