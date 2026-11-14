package album_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Фон альбома (098 §3, design §4): правки Editor, вид листов, лента. Круг Save→Load — у salbum
// (G1b) и стража обходов (G5).

const albumBgKey = "my:00000000000000ab"

var albumBgPat = model.Pattern{Kind: "stripes", Inks: []string{"#203040"}, Preset: "sa:stripes-test"}

// albumBgLib — своё фото из библиотеки для фона альбома.
func albumBgLib() model.PhotoInfo {
	return model.PhotoInfo{ID: model.LibPhotoID(albumBgKey), Path: edtest.Abs("albumbg.jpg"), W: 400, H: 300, Lib: albumBgKey}
}

func setAlbumBg(t *testing.T, e *album.Editor, src model.BgSource, pages bool) model.View {
	t.Helper()
	v, err := e.SetAlbumBg(src, pages)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// albumParts — узоры и фото фона альбома на листе (Album: true) по областям.
func albumParts(sv model.SpreadView) (pats, bds []string) {
	for _, pv := range sv.Patterns {
		if pv.Album {
			pats = append(pats, pv.Scope)
		}
	}
	for _, b := range sv.Backdrops {
		if b.Album {
			bds = append(bds, b.Scope)
		}
	}
	return pats, bds
}

// countOf — сколько раз id в списке.
func countOf(list []string, id string) int {
	n := 0
	for _, x := range list {
		if x == id {
			n++
		}
	}
	return n
}

// albumBgEd — обложка и четыре разворота: сетка s1 и «Авто» s2…s4.
func albumBgEd(t *testing.T) *album.Editor {
	t.Helper()
	return edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 60, "a")}, []string{"b"}, []string{"c"}, []string{"d"})
}

// TestAlbumBgView — фон альбома рисуется на каждом развороте без своего фона, не на обложке и не
// на листе с «Не показывать здесь»; свой фон на любой области его убирает, снятый — возвращает.
func TestAlbumBgView(t *testing.T) {
	e := albumBgEd(t)
	v := setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	if len(v.Spreads) != 4 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	for _, sv := range v.Spreads {
		pats, bds := albumParts(sv)
		if !slices.Equal(pats, []string{model.ScopeSpread}) || len(bds) != 0 || sv.NoAlbumBg {
			t.Fatalf("%s: узоры альбома %v, фото %v", sv.ID, pats, bds)
		}
		if pv := sv.Patterns[0]; pv.Area.X != 0 || pv.Area.W != sv.W || pv.Area.H != sv.H || !pv.Known || pv.URL == "" ||
			pv.Preset != albumBgPat.Preset || pv.Pat.Inks[0] != "#203040" {
			t.Fatalf("%s: узор альбома %+v", sv.ID, pv)
		}
	}
	if pats, bds := albumParts(v.Cover.Sheet); len(pats)+len(bds) != 0 || len(v.Cover.Sheet.Patterns) != 0 {
		t.Fatalf("обложка с фоном альбома: %v %v", pats, bds)
	}
	if a := v.AlbumBg; a == nil || a.Name == "" || a.Preset != albumBgPat.Preset || a.Lib != "" || a.Photo != "" || a.Pages || a.Fade != 0 {
		t.Fatalf("строка настроек: %+v", a)
	}

	// Свой узор на левой s2, своё фото на правой s3, скрытый на s4: остаётся только s1.
	if _, err := e.ApplyPattern("s2", model.ScopeLeft, model.Pattern{Kind: "dots"}, ""); err != nil {
		t.Fatal(err)
	}
	edtest.SetBg(t, e, "s3", model.ScopeRight, "x")
	v = e.SetNoAlbumBg("s4", true)
	want := map[string]bool{"s1": true}
	for _, sv := range v.Spreads {
		pats, bds := albumParts(sv)
		if got := len(pats)+len(bds) > 0; got != want[sv.ID] {
			t.Fatalf("%s: фон альбома %v, ждали %v", sv.ID, got, want[sv.ID])
		}
		if sv.NoAlbumBg != (sv.ID == "s4") {
			t.Fatalf("%s: скрыт %v", sv.ID, sv.NoAlbumBg)
		}
	}
	if n := len(v.Spreads[1].Patterns); n != 1 || v.Spreads[1].Patterns[0].Scope != model.ScopeLeft {
		t.Fatalf("s2: свой узор и фон альбома вместе: %+v", v.Spreads[1].Patterns)
	}
	// Свой сняли и «Показать» — снова виден.
	e.ClearPattern("s2", model.ScopeLeft)
	e.ClearBackdrop("s3", model.ScopeRight)
	v = e.SetNoAlbumBg("s4", false)
	for _, sv := range v.Spreads {
		if pats, _ := albumParts(sv); len(pats) != 1 {
			t.Fatalf("%s: после снятия своего %v", sv.ID, pats)
		}
	}
}

// TestAlbumBgViewPhoto — «на каждую страницу» — две области на каждом развороте; фото — как
// своё фото фона той же плитки на весь разворот (рамка = область), с прозрачностью фона альбома.
func TestAlbumBgViewPhoto(t *testing.T) {
	e := albumBgEd(t)
	v := setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, true)
	for _, sv := range v.Spreads {
		if pats, _ := albumParts(sv); !slices.Equal(pats, []string{model.ScopeLeft, model.ScopeRight}) {
			t.Fatalf("%s: по страницам %v", sv.ID, pats)
		}
	}
	if !v.AlbumBg.Pages {
		t.Fatal("строка настроек: не по страницам")
	}

	// Фото: вид — как у своего фото фона той же плитки на весь разворот (рамка = область).
	ph := albumBgLib()
	v = setAlbumBg(t, e, model.BgSource{Photo: ph}, false)
	v = e.SetAlbumBgFade(0.333, "")
	var album model.BackdropView
	for _, sv := range v.Spreads {
		pats, bds := albumParts(sv)
		if len(pats) != 0 || !slices.Equal(bds, []string{model.ScopeSpread}) {
			t.Fatalf("%s: фото альбома %v, узоры %v", sv.ID, bds, pats)
		}
		album = sv.Backdrops[0]
	}
	if album.Photo != ph.ID || album.Lib != albumBgKey || album.Fade != 0.33 || album.DPI == 0 {
		t.Fatalf("фото альбома: %+v", album)
	}
	if a := v.AlbumBg; a.Lib != albumBgKey || a.Photo != ph.ID || a.Fade != 0.33 || a.Preset != "" {
		t.Fatalf("строка настроек фото: %+v", a)
	}
	if _, err := e.ApplyBackdrop("s4", model.ScopeSpread, ph); err != nil {
		t.Fatal(err)
	}
	v = e.SetBackdropFade("s4", model.ScopeSpread, 0.33, "")
	own := v.Spreads[3].Backdrops[0]
	if own.Album || len(v.Spreads[3].Backdrops) != 1 {
		t.Fatalf("s4: свой фон %+v", v.Spreads[3].Backdrops)
	}
	own.Album = true
	if own != album {
		t.Fatalf("фото альбома %+v\nсвоё той же плитки %+v", album, own)
	}
}

// TestAlbumBgViewEdge — фото без ключа библиотеки (после «Собрать в папку»): имя — по файлу;
// кадр из файла — в виде листа; узор неизвестного генератора (файл новой версии) — масштаб 1,
// без имени и превью.
func TestAlbumBgViewEdge(t *testing.T) {
	e := albumBgEd(t)
	v := setAlbumBg(t, e, model.BgSource{Photo: edtest.Photo("x")}, false)
	if a := v.AlbumBg; a.Name != "x" || a.Lib != "" || a.Photo != "x" {
		t.Fatalf("строка настроек фото без ключа: %+v", a)
	}
	edtest.Change(e, func(p *model.Project) { p.AlbumBg.Frame = &model.Frame{S: 2} })
	v = e.View()
	for _, sv := range v.Spreads {
		b := sv.Backdrops[0]
		if b.Frame.S != 2 || b.Img.W <= b.Area.W {
			t.Fatalf("%s: кадр %+v, фото %+v, область %+v", sv.ID, b.Frame, b.Img, b.Area)
		}
	}
	v = setAlbumBg(t, e, model.BgSource{Pattern: &model.Pattern{Kind: "future-kind"}}, false)
	if a := v.AlbumBg; a.Name != "" || a.Photo != "" {
		t.Fatalf("строка настроек неизвестного узора: %+v", a)
	}
	for _, sv := range v.Spreads {
		pv := sv.Patterns[0]
		if !pv.Album || pv.Known || pv.Pat.Scale != 1 || pv.Name != "" || pv.URL != "" {
			t.Fatalf("%s: неизвестный узор %+v", sv.ID, pv)
		}
	}
}

// TestAlbumBgEmptyAlbum —фон альбома без разворотов: ставится и виден в строке настроек; лист
// без своего фона появляется уже с ним.
func TestAlbumBgEmptyAlbum(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, true)
	if v.AlbumBg == nil || len(v.Spreads) != 0 {
		t.Fatalf("пустой альбом: %+v, развороты %d", v.AlbumBg, len(v.Spreads))
	}
	v = edtest.NewSpread(e, -1, edtest.Photos("a"))
	if pats, _ := albumParts(v.Spreads[0]); len(pats) != 2 {
		t.Fatalf("новый разворот: %v", pats)
	}
}

// TestAlbumBgNewSpreads — развороты автосборки, «+ Новый разворот» и разреза получают фон альбома
// сами; фото фона альбома автосборка не берёт (оно не «в альбоме»).
func TestAlbumBgNewSpreads(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	ph := albumBgLib()
	setAlbumBg(t, e, model.BgSource{Photo: ph}, false)
	in := append([]model.PhotoInfo{ph}, edtest.Photos("p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10")...)
	v, err := e.AutoBuild(in, -1, model.AutoOpts{Spreads: 5, MaxPer: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) < 6 {
		t.Fatalf("автосборка: %d разворотов", len(v.Spreads))
	}
	v = edtest.NewSpread(e, -1, edtest.Photos("q1", "q2"))
	last := v.Spreads[len(v.Spreads)-1].ID
	v, err = e.SplitSpread(last, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, sv := range v.Spreads {
		if _, bds := albumParts(sv); !slices.Equal(bds, []string{model.ScopeSpread}) {
			t.Fatalf("%s: фон альбома %v", sv.ID, bds)
		}
		if slices.Contains(sv.Photos, ph.ID) {
			t.Fatalf("%s: фото фона альбома в местах", sv.ID)
		}
	}
	if edtest.P(e).Placed(ph.ID) {
		t.Fatal("автосборка посадила фото фона альбома")
	}
}

// TestAlbumBgUndo — каждая правка фона альбома — один шаг отмены; повтор, пустое и негодное —
// холостые без шага; серия ползунка с одной сессией — один шаг.
func TestAlbumBgUndo(t *testing.T) {
	e := albumBgEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ClearAlbumBg()
	edtest.Same(t, e, rev, undo, "снять без фона")
	e.SetAlbumBgFade(0.5, "")
	edtest.Same(t, e, rev, undo, "прозрачность без фона")
	for _, c := range []struct {
		name string
		src  model.BgSource
		want error
	}{
		{"пусто", model.BgSource{}, model.ErrPattern},
		{"оба", model.BgSource{Pattern: &albumBgPat, Photo: albumBgLib()}, model.ErrPattern},
		{"неверный узор", model.BgSource{Pattern: &model.Pattern{Kind: "Плохой ключ"}}, model.ErrPattern},
		{"ключ не от ID", model.BgSource{Photo: model.PhotoInfo{ID: "x", Path: edtest.Abs("x.jpg"), W: 3, H: 2, Lib: albumBgKey}}, model.ErrLibKey},
	} {
		if _, err := e.SetAlbumBg(c.src, false); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v, ждали %v", c.name, err, c.want)
		}
		edtest.Same(t, e, rev, undo, c.name)
	}

	setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("поставить: шагов %d", edtest.UndoLen(e)-undo)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	edtest.Same(t, e, rev, undo, "та же плитка")

	for _, f := range []float64{0.2, 0.35, 0.5} {
		e.SetAlbumBgFade(f, "s1")
	}
	if edtest.UndoLen(e) != undo+1 || edtest.P(e).AlbumBg.Fade != 0.5 {
		t.Fatalf("ползунок: шагов %d, прозрачность %v", edtest.UndoLen(e)-undo, edtest.P(e).AlbumBg.Fade)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetAlbumBgFade(math.NaN(), "s1")
	e.SetAlbumBgFade(0.5, "")
	edtest.Same(t, e, rev, undo, "NaN и та же прозрачность")
	e.SetAlbumBgFade(7, "")
	if edtest.P(e).AlbumBg.Fade != 1 {
		t.Fatalf("зажим: %v", edtest.P(e).AlbumBg.Fade)
	}
	e.Undo()

	e.SetNoAlbumBg("s2", true)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatal("«Не показывать здесь»: не один шаг")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetNoAlbumBg("s2", true)
	e.SetNoAlbumBg(model.CoverID, true)
	e.SetNoAlbumBg("нет-такого", true)
	edtest.Same(t, e, rev, undo, "повтор, обложка, неизвестный")

	e.ClearAlbumBg()
	if edtest.UndoLen(e) != undo+1 || edtest.P(e).AlbumBg != nil {
		t.Fatal("убрать: не один шаг")
	}
	v := e.Undo()
	if v.AlbumBg == nil || v.AlbumBg.Fade != 0.5 {
		t.Fatalf("отмена «Убрать»: %+v", v.AlbumBg)
	}
	e.Undo() // «Не показывать здесь»
	e.Undo() // серия ползунка
	if f := edtest.P(e).AlbumBg.Fade; f != 0 {
		t.Fatalf("отмена серии: %v", f)
	}
	v = e.Undo() // постановка
	if v.AlbumBg != nil {
		t.Fatal("отмена постановки")
	}
	for _, sv := range v.Spreads {
		if pats, bds := albumParts(sv); len(pats)+len(bds) != 0 {
			t.Fatalf("%s: после отмены фон альбома остался", sv.ID)
		}
	}
	if v = e.Redo(); v.AlbumBg == nil {
		t.Fatal("повтор постановки")
	}
}

// TestBackdropUsedAlbumBg — фото фона альбома в ленте «стоит на фоне» (BackdropUsed), даже когда
// ни на одном листе его не видно; снятый фон альбома уходит из набора и из записей.
func TestBackdropUsedAlbumBg(t *testing.T) {
	e := albumBgEd(t)
	ph := albumBgLib()
	v := setAlbumBg(t, e, model.BgSource{Photo: ph}, true)
	if n := countOf(v.BackdropUsed, ph.ID); n != 1 {
		t.Fatalf("на всех листах: %v", v.BackdropUsed)
	}
	for _, id := range []string{"s1", "s2", "s3", "s4"} {
		v = e.SetNoAlbumBg(id, true)
	}
	for _, sv := range v.Spreads {
		if len(sv.Backdrops) != 0 {
			t.Fatalf("%s: фон виден", sv.ID)
		}
	}
	if !slices.Contains(v.BackdropUsed, ph.ID) || !edtest.HasRef(edtest.P(e), ph.ID) {
		t.Fatalf("скрыт на всех листах: %v", v.BackdropUsed)
	}
	v = e.ClearAlbumBg()
	if slices.Contains(v.BackdropUsed, ph.ID) || edtest.HasRef(edtest.P(e), ph.ID) {
		t.Fatalf("снят: %v, запись осталась %v", v.BackdropUsed, edtest.HasRef(edtest.P(e), ph.ID))
	}
	v = setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	if len(v.BackdropUsed) != 0 {
		t.Fatalf("узор: %v", v.BackdropUsed)
	}
}
