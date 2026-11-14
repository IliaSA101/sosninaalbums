package cover_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Правки обложки через Editor (032 §2, §5): создание, пресеты и свои числа, раскладки,
// надпись на корешке; холостые правки без шага отмены.

// pid — ID фото разворота i в coverAlbum.
func pid(i int) string { return fmt.Sprintf("p%02d", i) }

// info — сведения о фото для правок (как из библиотеки).
func info(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: 300, H: 200}
}

// coverAlbum — n разворотов по одному фото (страница 281×284, поле безопасности 8) и обложка
// «тонкие листы» с фото разворота 3 на лицевой.
func coverAlbum(t *testing.T, n int) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = 281, 284, 8
	for i := range n {
		p.Photos[pid(i)] = model.PhotoRef{Path: edtest.Abs(pid(i) + ".jpg"), W: 300, H: 200}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("%012x", i+1), Photos: []string{pid(i)}})
	}
	e := edtest.Open(t, p, "")
	ph := info(pid(3))
	if _, err := e.CreateCover("", &ph); err != nil {
		t.Fatal(err)
	}
	return e
}

func sheet(e *album.Editor) *model.Spread { return edtest.P(e).Cover.Sheet }

// TestCoverBackdropKeepsSpread — бросок на плитку создаёт обложку с фото фона лицевой одним
// шагом; фото остаётся в своём развороте (у фона свой кадр, decisions 50).
func TestCoverBackdropKeepsSpread(t *testing.T) {
	e := coverAlbum(t, 6)
	p := edtest.P(e)
	if p.Cover.Spec.Preset != model.CoverDefault || p.Cover.Layout != model.CoverFront || edtest.UndoLen(e) != 1 {
		t.Fatalf("обложка %+v, шагов отмены %d", p.Cover, edtest.UndoLen(e))
	}
	b := sheet(e).BgPhotoR
	if b == nil || b.Photo != pid(3) || !slices.Equal(p.Spreads[3].Photos, []string{pid(3)}) {
		t.Fatalf("фон лицевой %+v, разворот 3 %v", b, p.Spreads[3].Photos)
	}
	if a := p.BdArea(sheet(e), model.ScopeRight); b.X != a.X || b.W != a.W || b.H != p.Geo(sheet(e)).H {
		t.Fatalf("фон не по зоне лицевой с заворотом: %+v, зона %+v", b, a)
	}
	v := e.View()
	if v.Cover == nil || len(v.Cover.Drop) != 0 || !slices.Contains(v.BackdropUsed, pid(3)) || len(v.Spreads) != 6 {
		t.Fatalf("вид: обложка %v, фоны %v, разворотов %d", v.Cover, v.BackdropUsed, len(v.Spreads))
	}
	e.Undo()
	if edtest.P(e).Cover != nil {
		t.Fatal("Undo не снял обложку")
	}
}

// TestCoverPlaceTakesPhoto — место на обложке подчиняется «одно фото — одно место»: фото
// уходит из разворота.
func TestCoverPlaceTakesPhoto(t *testing.T) {
	e := coverAlbum(t, 6)
	if _, err := e.AddPlace(model.CoverID, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.PlacePhotos(model.CoverID, sheet(e).Grid[0].ID, []model.PhotoInfo{info(pid(5))}); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	if sheet(e).Grid[0].Photo != pid(5) || len(p.Spreads) != 5 || slices.ContainsFunc(p.Spreads, func(s *model.Spread) bool {
		return slices.Contains(s.Photos, pid(5))
	}) {
		t.Fatalf("фото в месте обложки %q, разворотов %d", sheet(e).Grid[0].Photo, len(p.Spreads))
	}
	edtest.MustGrid(t, e)
}

// TestCreateCoverDefaultByPage: пустой пресет — изделие типографии страницы (Фабрика 30×20 →
// «Фотобумага», сторонка и заворот из её формата), не «Фотосфера · Тонкие листы»; один шаг отмены.
func TestCreateCoverDefaultByPage(t *testing.T) {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe, p.Settings.Bleed = 304, 203, 10, 3
	e := edtest.Open(t, p, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	c := edtest.P(e).Cover
	if c == nil || c.Spec.Preset != "fabrika-photo" || c.Spec.W != 310 || c.Spec.H != 206 || c.Spec.Wrap != 17 {
		t.Fatalf("обложка на 304×203: %+v", c)
	}
	if edtest.UndoLen(e) != 1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e))
	}
	if e.Undo(); edtest.P(e).Cover != nil {
		t.Fatal("отмена не сняла обложку")
	}
}

func TestCoverCreateErrors(t *testing.T) {
	e := coverAlbum(t, 2)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CreateCover("", nil); err != cover.ErrExists {
		t.Fatalf("повторная обложка: %v", err)
	}
	edtest.Same(t, e, rev, undo, "повторная обложка")
	e.DeleteCover()
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CreateCover("нет такой", nil); err != cover.ErrPreset || edtest.P(e).Cover != nil {
		t.Fatalf("неизвестный пресет: %v", err)
	}
	edtest.Same(t, e, rev, undo, "неизвестный пресет")
	if e.Undo(); edtest.P(e).Cover == nil || edtest.UndoLen(e) != 1 {
		t.Fatal("удаление обложки — не один шаг отмены")
	}
	sh := edtest.Open(t, model.NewSheet(204, 304), "")
	if _, err := sh.CreateCover("", nil); err != cover.ErrSheet || edtest.P(sh).Cover != nil {
		t.Fatalf("обложка в проекте-листе: %v", err)
	}
	for _, f := range []func() error{
		func() error { _, err := sh.ApplyCoverPreset(model.CoverDefault); return err },
		func() error { _, err := sh.SetCoverSpec(model.CoverSpec{}); return err },
		func() error { _, err := sh.SetCoverLayout("wrap"); return err },
		func() error { _, err := sh.CoverSpineText(); return err },
	} {
		if err := f(); err != cover.ErrNone {
			t.Fatalf("правка без обложки: %v", err)
		}
	}
}

// TestCoverSpecPreset — пресет остаётся подписью, пока числа его; любое своё число —
// «Свои размеры» с теми же числами. Сторонка в счёт, только если её задаёт шаблон (Фотосфера:
// формат страницы или ближайший, 070 Е); Нетпринт сторонку пользователя не трогает.
func TestCoverSpecPreset(t *testing.T) {
	e := coverAlbum(t, 2)
	sp := edtest.P(e).Cover.Spec
	sp.W, sp.H = 300, 290
	if _, err := e.SetCoverSpec(sp); err != nil || edtest.P(e).Cover.Spec.Preset != model.CoverCustom || edtest.P(e).Cover.Spec.W != 300 {
		t.Fatalf("своя сторонка на формате Фотосферы: %v %+v", err, edtest.P(e).Cover.Spec)
	}
	// Страница не из таблицы: сторонка шаблона — ближайший формат (30×30: 286×286), а не страница;
	// своя сторонка 300×290 — уже не шаблон, пресет «Своих размеров».
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 300, 300
	e.SetSettings(st)
	if _, err := e.ApplyCoverPreset(model.CoverDefault); err != nil || edtest.P(e).Cover.Spec.W != 286 || edtest.P(e).Cover.Spec.H != 286 {
		t.Fatalf("пресет вне таблицы не по ближайшему формату: %v %+v", err, edtest.P(e).Cover.Spec)
	}
	if _, err := e.SetCoverSpec(sp); err != nil || edtest.P(e).Cover.Spec.Preset != model.CoverCustom || edtest.P(e).Cover.Spec.W != 300 {
		t.Fatalf("своя сторонка вне таблицы осталась «Фотосферой»: %v %+v", err, edtest.P(e).Cover.Spec)
	}
	sp.Wrap = 21
	if _, _ = e.SetCoverSpec(sp); edtest.P(e).Cover.Spec.Preset != model.CoverCustom || edtest.P(e).Cover.Spec.Wrap != 21 {
		t.Fatalf("свой заворот: %+v", edtest.P(e).Cover.Spec)
	}
	if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Spec; c.Preset != "netprint-premium" || c.W != 300 || c.H != 290 || c.A != 5.4 || c.Steps != nil {
		t.Fatalf("пресет Нетпринта: %+v", c)
	}
	sp.Wrap = math.NaN()
	if _, err := e.SetCoverSpec(sp); err != nil || edtest.P(e).Cover.Spec.Wrap != 15 {
		t.Fatalf("NaN не зажат: %v %+v", err, edtest.P(e).Cover.Spec)
	}
}

// TestCoverLayouts — смена раскладки переносит фото фона (design §2); надпись на корешке —
// rot −90 по центру корешка, плашка — цвет лицевой, повтор — холостой.
func TestCoverLayouts(t *testing.T) {
	e := coverAlbum(t, 4)
	s, p := func() *model.Spread { return sheet(e) }, func() *model.Project { return edtest.P(e) } // отказ подменяет проект
	if _, err := e.SetCoverLayout("wrap"); err != nil || s().BgPhoto == nil || s().BgPhoto.Photo != pid(3) || s().BgPhotoR != nil {
		t.Fatalf("через корешок: %v %+v", err, s())
	}
	if b := s().BgPhoto; b.X != 0 || b.W != p().Geo(s()).W {
		t.Fatalf("фон через корешок не на весь лист: %+v", b)
	}
	if _, err := e.SetCoverLayout("front"); err != nil || s().BgPhotoR == nil || s().BgPhoto != nil || s().BgPhotoR.Photo != pid(3) {
		t.Fatalf("на лицевой: %v %+v", err, s())
	}
	e.SetBackdropFrame(model.CoverID, model.ScopeRight, 1.5, 0.2, 0)
	if _, err := e.SetCoverLayout("both"); err != nil || s().BgPhotoR.Frame == nil || s().BgPhotoL != nil {
		t.Fatalf("лицевая + задняя: кадр лицевой сброшен или задняя не пуста: %v %+v", err, s())
	}
	if d := e.View().Cover.Drop; len(d) != 2 || d[0].Kind != "back" || d[1].Kind != "spine" { // корешок пуст (061)
		t.Fatalf("пустые зоны «лицевая + задняя»: %+v", d)
	}
	edtest.SetBg(t, e, model.CoverID, model.ScopeLeft, pid(1))
	if _, err := e.SetCoverLayout("wrap"); err != nil || s().BgPhoto.Photo != pid(3) || s().BgPhotoL != nil || s().BgPhoto.Frame != nil {
		t.Fatalf("обе → через корешок: %v %+v", err, s())
	}
	if _, err := e.SetCoverLayout("bad"); err != cover.ErrLayout {
		t.Fatalf("неизвестная раскладка: %v", err)
	}
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	g, cg := p().Geo(s()), p().CoverGeo()
	tb := s().Texts[len(s().Texts)-1]
	cx, cy := model.BoxOf(tb).Center()
	if tb.Rot != -90 || math.Abs(cx-g.W/2) > 0.01 || math.Abs(cy-g.H/2) > 0.01 || tb.Style.Size != math.Round(cg.Spine*14)/10 {
		t.Fatalf("надпись на корешке %+v, центр (%v, %v), лист %v×%v", tb, cx, cy, g.W, g.H)
	}
	if p().Cover.SpineBg != "#ffffff" || tb.W != g.H-2*p().Cover.Spec.Wrap-16 {
		t.Fatalf("плашка %q, ширина переноса %v", p().Cover.SpineBg, tb.W)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, _ = e.CoverSpineText()
	edtest.Same(t, e, rev, undo, "повторная надпись на корешке")
	e.SetCoverSpineBg("#102030")
	if p().Cover.SpineBg != "#102030" {
		t.Fatalf("цвет плашки %q", p().Cover.SpineBg)
	}
	e.SetCoverSpineBg("")
	if p().Cover.SpineBg != "" {
		t.Fatal("плашка не снята")
	}
}

// TestCoverSpineTextDark — на тёмной плашке текст белый.
func TestCoverSpineTextDark(t *testing.T) {
	e := coverAlbum(t, 2)
	e.SetCoverSpineBg("#203040")
	if _, err := e.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	if tb := sheet(e).Texts[0]; tb.Style.Color != "#ffffff" {
		t.Fatalf("цвет надписи на тёмной плашке %q", tb.Style.Color)
	}
}

// TestCoverIdleEdits — холостые правки обложки: ни шага отмены, ни rev.
func TestCoverIdleEdits(t *testing.T) {
	e := coverAlbum(t, 3)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, _ = e.SetCoverLayout("front")
	_, _ = e.SetCoverLayout("")
	_, _ = e.SetCoverSpec(edtest.P(e).Cover.Spec)
	_, _ = e.ApplyCoverPreset(model.CoverDefault)
	e.SetCoverSpineBg("красный")
	e.ClearCoverGuide()
	_, _ = e.SetCoverGuide(`\\server\share\guide.png`)
	_, _ = e.SetCoverGuide(edtest.Abs("guide.gif"))
	e.SetSettings(edtest.P(e).Settings)
	edtest.Same(t, e, rev, undo, "холостые правки обложки")
	e.DeleteCover()
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.DeleteCover()
	e.SetCoverSpineBg("#ffffff")
	edtest.Same(t, e, rev, undo, "правки без обложки")
}

// markupOf — разметка обложки в виде; нет — нулевая (Name "").
func markupOf(v model.View) model.GuideView {
	if v.Cover == nil || v.Cover.Markup == nil {
		return model.GuideView{}
	}
	return *v.Cover.Markup
}

// TestCoverGuide — путь разметки: абсолютный PSD/JPEG/PNG; имя файла в виде, файла нет — OK false.
func TestCoverGuide(t *testing.T) {
	e := coverAlbum(t, 2)
	path := edtest.Abs("нет-такой-разметки.png")
	v, err := e.SetCoverGuide(path)
	if m := markupOf(v); err != nil || edtest.P(e).Cover.Guide != path || m.Name != "нет-такой-разметки.png" || m.OK {
		t.Fatalf("разметка: %v, вид %+v", err, m)
	}
	if v = e.ClearCoverGuide(); v.Cover.Markup != nil || edtest.P(e).Cover.Guide != "" {
		t.Fatal("разметка не снята")
	}
}

// TestCoverEmptyPhotosRoundTrip — у обложки без фото на местах Photos = nil, как после Load
// (FixCoverSheet): пустое место и его удаление переживают Save → Load тем же JSON листа
// (иначе "photos":[] после чтения стало бы null).
func TestCoverEmptyPhotosRoundTrip(t *testing.T) {
	e := coverAlbum(t, 2)
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	same := func(what string) {
		t.Helper()
		if err := e.Save(path); err != nil {
			t.Fatal(err)
		}
		q, err := album.LoadBase(path, filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		// Фото и места: ID фото Load выводит из пути заново, фон сравнивать нельзя.
		a, err := json.Marshal([]any{sheet(e).Photos, sheet(e).Grid})
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := json.Marshal([]any{q.Cover.Sheet.Photos, q.Cover.Sheet.Grid}); !bytes.Equal(a, b) {
			t.Fatalf("%s: лист после Save → Load другой:\n%s\n%s", what, a, b)
		}
	}
	if _, err := e.AddPlace(model.CoverID, 1, ""); err != nil {
		t.Fatal(err)
	}
	if sheet(e).Photos != nil {
		t.Fatalf("пустое место: Photos %#v", sheet(e).Photos)
	}
	same("пустое место")
	var ids []string
	for _, pl := range sheet(e).Grid {
		ids = append(ids, pl.ID)
	}
	e.DeletePlaces(model.CoverID, ids)
	if sheet(e).Photos != nil || sheet(e).Grid != nil {
		t.Fatalf("места удалены: Photos %#v, мест %d", sheet(e).Photos, len(sheet(e).Grid))
	}
	same("места удалены")
}

// writePNG — PNG w×h пикселей в каталоге теста, его путь.
func writePNG(t *testing.T, name string, w, h int) string {
	t.Helper()
	path := filepath.Join(testkit.TempDir(t), name)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCoverGuideReadable — разметка читается: OK; пропорции чужие (больше 1 % от листа) —
// Warn; файл не картинка — OK false без ошибки правки.
func TestCoverGuideReadable(t *testing.T) {
	e := coverAlbum(t, 2)
	g := edtest.P(e).Geo(sheet(e))
	exact := writePNG(t, "exact.png", int(math.Round(g.W*10)), int(math.Round(g.H*10)))
	v, err := e.SetCoverGuide(exact)
	if m := markupOf(v); err != nil || !m.OK || m.Warn != "" || !m.Image {
		t.Fatalf("разметка по листу: %v, %+v", err, m)
	}
	v, err = e.SetCoverGuide(writePNG(t, "square.png", 100, 100))
	if m := markupOf(v); err != nil || !m.OK || m.Warn != "Пропорции разметки не совпадают с обложкой" {
		t.Fatalf("квадратная разметка: %v, %+v", err, m)
	}
	junk := filepath.Join(testkit.TempDir(t), "junk.png")
	if err := os.WriteFile(junk, []byte("не картинка"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, err = e.SetCoverGuide(junk); err != nil || markupOf(v).OK || markupOf(v).Warn != "" {
		t.Fatalf("не картинка: %v, %+v", err, markupOf(v))
	}
}

// TestCoverSpineTextLimit — на обложке уже MaxTexts надписей: «Надпись на корешке» — отказ
// без правки (лимит общий для листа).
func TestCoverSpineTextLimit(t *testing.T) {
	e := coverAlbum(t, 2)
	for len(sheet(e).Texts) < model.MaxTexts {
		if _, err := e.AddText(model.CoverID, 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.CoverSpineText(); err == nil {
		t.Fatal("надпись сверх лимита принята")
	}
	edtest.Same(t, e, rev, undo, "надпись на корешке сверх лимита")
}

// TestCoverSpineTextOddFront — цвет лицевой берётся из настроек альбома; не #rrggbb (файл
// правили руками) считается светлым: плашка ставится, текст остаётся тёмным, без паники.
func TestCoverSpineTextOddFront(t *testing.T) {
	for _, bg := range []string{"#fff", "#zzzzzz"} {
		e := coverAlbum(t, 2)
		edtest.Change(e, func(p *model.Project) { p.Settings.Bg = bg })
		if _, err := e.CoverSpineText(); err != nil {
			t.Fatalf("%q: %v", bg, err)
		}
		if tb := sheet(e).Texts[0]; tb.Style.Color == "#ffffff" {
			t.Fatalf("фон %q принят за тёмный: текст %q", bg, tb.Style.Color)
		}
	}
}

// TestCoverViewSpineText — CoverView.SpineText («Выделить надпись», 058 №19): пусто, пока в
// корешке нет повёрнутой надписи (обычный текст обложки не в счёт); после «Надписи на корешке» —
// её ID, повтор холостой; Save → Load тот же ID; отмена снова пусто.
func TestCoverViewSpineText(t *testing.T) {
	e := coverAlbum(t, 3)
	if _, err := e.AddText(model.CoverID, 1, ""); err != nil {
		t.Fatal(err)
	}
	if got := e.View().Cover.SpineText; got != "" {
		t.Fatalf("только обычный текст: SpineText %q", got)
	}
	v, err := e.CoverSpineText()
	if err != nil {
		t.Fatal(err)
	}
	id := sheet(e).Texts[len(sheet(e).Texts)-1].ID
	if got := v.Cover.SpineText; got == "" || got != id {
		t.Fatalf("после надписи: SpineText %q, ждали %q", got, id)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if v, _ = e.CoverSpineText(); v.Cover.SpineText != id {
		t.Fatalf("повтор: SpineText %q", v.Cover.SpineText)
	}
	edtest.Same(t, e, rev, undo, "повторная надпись на корешке")
	if q := edtest.SaveLoad(t, e); q.SpineTextID() != id {
		t.Fatalf("после Save → Load: %q, ждали %q", q.SpineTextID(), id)
	}
	if got := e.Undo().Cover.SpineText; got != "" {
		t.Fatalf("после отмены: SpineText %q", got)
	}
}
