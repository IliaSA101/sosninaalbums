package cover_test

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка по шаблону формата Фотосферы (047 §2): пресет и новая обложка, смена формата,
// старая обложка и «Исправить».

// spec — числа обложки редактора.
func spec(e *album.Editor) model.CoverSpec { return edtest.P(e).Cover.Spec }

// page — смена страницы через «Настройки альбома».
func page(e *album.Editor, w, h float64) model.View {
	st := edtest.P(e).Settings
	st.PageW, st.PageH = w, h
	return e.SetSettings(st)
}

// TestApplyPresetFormat — новая обложка и пресет «Фотосферы» на формате из таблицы берут
// сторонку и заворот шаблона, а не пользователя (решение 52); повтор холостой; Нетпринт держит
// сторонку пользователя; на 21×30 заворот 16 — «Фотосфера», 21 — «Свои размеры».
func TestApplyPresetFormat(t *testing.T) {
	e := coverAlbum(t, 2)
	if c := spec(e); c.Preset != model.CoverDefault || c.W != 286 || c.H != 286 || c.Wrap != 20 || c.Safe != 8 || c.SpineSafe != 12 {
		t.Fatalf("новая обложка на 30×30: %+v", c)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyCoverPreset(model.CoverDefault); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "тот же пресет на обложке по шаблону")
	sp := spec(e)
	sp.W, sp.H, sp.Wrap = 300, 290, 15
	if _, err := e.SetCoverSpec(sp); err != nil || spec(e).Preset != model.CoverCustom {
		t.Fatalf("свои числа: %v %+v", err, spec(e))
	}
	if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil || spec(e).W != 300 || spec(e).H != 290 {
		t.Fatalf("Нетпринт тронул сторонку: %v %+v", err, spec(e))
	}
	undo = edtest.UndoLen(e)
	if _, err := e.ApplyCoverPreset("fsfera-card"); err != nil || spec(e).W != 286 || spec(e).H != 286 || spec(e).Preset != "fsfera-card" {
		t.Fatalf("Картон на 30×30: %v %+v", err, spec(e))
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatal("пресет — не один шаг отмены")
	}
	if e.Undo(); spec(e).Preset != "netprint-premium" || spec(e).W != 300 {
		t.Fatalf("Undo пресета: %+v", spec(e))
	}

	b := edtest.Open(t, model.New(), "")
	page(b, 208, 296)
	if _, err := b.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	sp = spec(b)
	if sp.W != 211 || sp.H != 296 || sp.Wrap != 16 {
		t.Fatalf("21×30: %+v", sp)
	}
	sp.Safe, sp.SpineSafe = 0, 0 // линии карточка не правит: остаются прежние
	if _, err := b.SetCoverSpec(sp); err != nil || spec(b).Preset != model.CoverDefault || spec(b).Safe != 8 {
		t.Fatalf("21×30 с заворотом 16: %v %+v", err, spec(b))
	}
	sp.Wrap = 21
	if _, err := b.SetCoverSpec(sp); err != nil || spec(b).Preset != model.CoverCustom || spec(b).Safe != 8 {
		t.Fatalf("21×30 с заворотом 21: %v %+v", err, spec(b))
	}
	presetDropsTemplateSide(t)
}

// presetDropsTemplateSide — часть TestApplyPresetFormat: сторонка шаблона Фотосферы — не своя,
// Нетпринт уходит на страницу (0) одним шагом, повтор холостой, Save → Load держит 0, Undo —
// снова шаблон. Те же 286×286 в «Своих размерах» — своя сторонка: Нетпринт её оставляет.
func presetDropsTemplateSide(t *testing.T) {
	t.Helper()
	c := coverAlbum(t, 2)
	undo := edtest.UndoLen(c)
	if _, err := c.ApplyCoverPreset("netprint-premium"); err != nil || spec(c).W != 0 || spec(c).H != 0 || edtest.UndoLen(c) != undo+1 {
		t.Fatalf("Нетпринт после Фотосферы: %v %+v", err, spec(c))
	}
	rev, undo := edtest.Rev(c), edtest.UndoLen(c)
	if _, err := c.ApplyCoverPreset("netprint-premium"); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, c, rev, undo, "тот же Нетпринт")
	path := filepath.Join(testkit.TempDir(t), "нетпринт.salbum")
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	if q, err := album.LoadBase(path, filepath.Dir(path)); err != nil || q.Cover.Spec.W != 0 || q.Cover.Spec.H != 0 || q.Cover.Spec.Preset != "netprint-premium" {
		t.Fatalf("Save → Load: %v", err)
	}
	if c.Undo(); spec(c).W != 286 || spec(c).H != 286 || spec(c).Preset != model.CoverDefault {
		t.Fatalf("Undo Нетпринта: %+v", spec(c))
	}
	sp := spec(c)
	sp.Wrap = 15
	if _, err := c.SetCoverSpec(sp); err != nil || spec(c).Preset != model.CoverCustom {
		t.Fatalf("свои 286×286: %v %+v", err, spec(c))
	}
	if _, err := c.ApplyCoverPreset("netprint-premium"); err != nil || spec(c).W != 286 || spec(c).H != 286 {
		t.Fatalf("Нетпринт тронул свои 286×286: %v %+v", err, spec(c))
	}
}

// TestCoverFollowsFormat — обложка по шаблону переходит на шаблон нового формата тем же шагом
// (решение 53): одно Undo возвращает и страницу, и обложку; своя обложка и старая (заворот 23,5)
// остаются; формат → не формат — шаблон ближайшего формата (070 Е), обратно — шаблон нового.
func TestCoverFollowsFormat(t *testing.T) {
	e := coverAlbum(t, 2)
	x0 := sheet(e).BgPhotoR.X
	undo := edtest.UndoLen(e)
	page(e, 201, 203)
	if c := spec(e); c.W != 206 || c.H != 206 || c.Wrap != 20 || c.Preset != model.CoverDefault {
		t.Fatalf("30×30 → 20×20: %+v", c)
	}
	if sheet(e).BgPhotoR.X == x0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("фон лицевой не поехал (%v) или шагов отмены %d", sheet(e).BgPhotoR.X, edtest.UndoLen(e)-undo)
	}
	if e.Undo(); edtest.P(e).Settings.PageW != 281 || spec(e).W != 286 || sheet(e).BgPhotoR.X != x0 {
		t.Fatalf("Undo: страница %v, обложка %+v", edtest.P(e).Settings.PageW, spec(e))
	}
	page(e, 300, 300)
	if c := spec(e); c.W != 286 || c.H != 286 || c.Wrap != 20 {
		t.Fatalf("формат → не формат: %+v", c)
	}
	page(e, 208, 296)
	if c := spec(e); c.W != 211 || c.H != 296 || c.Wrap != 16 {
		t.Fatalf("не формат → 21×30: %+v", c)
	}
	// Свои числа: смена формата их не трогает.
	sp := spec(e)
	sp.W = 250
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	own := spec(e)
	page(e, 281, 284)
	if c := spec(e); c.W != 250 || c.H != own.H || c.Wrap != 16 || c.Preset != model.CoverCustom {
		t.Fatalf("своя обложка сменилась: %+v", c)
	}
	// Старая «Фотосфера» с заворотом 23,5 — не шаблон: тоже остаётся.
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec = legacy() })
	page(e, 201, 203)
	if c := spec(e); c.Wrap != 23.5 || c.W != 0 {
		t.Fatalf("старая обложка сменилась: %+v", c)
	}
}

// legacy — обложка «Тонкие листы» по таблице до 047: заворот 23,5, сторонка — страница,
// без безопасных линий.
func legacy() model.CoverSpec {
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	s := pr.Spec
	s.Wrap, s.Safe, s.SpineSafe = 23.5, 0, 0
	return s
}

// TestCoverFixLegacy — старая обложка после Save → Load прежняя (числа — истина, решение 52),
// в виде строка «Не по шаблону…»; «Исправить» (ApplyCoverPreset(cv.preset)) — один шаг: строка
// пуста, фото фона лицевой переехало (Reflow); Undo возвращает старые числа.
func TestCoverFixLegacy(t *testing.T) {
	src := coverAlbum(t, 20)
	edtest.Change(src, func(p *model.Project) { p.Cover.Spec = legacy() })
	path := filepath.Join(testkit.TempDir(t), "старый.salbum")
	if err := src.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if q.Cover.Spec.Wrap != 23.5 || q.Cover.Spec.W != 0 || q.Cover.Spec.Safe != 0 {
		t.Fatalf("Load тронул старую обложку: %+v", q.Cover.Spec)
	}
	e := edtest.Open(t, q, path)
	cv := e.View().Cover
	if cv.Fix != "Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326" {
		t.Fatalf("строка: %q", cv.Fix)
	}
	x0, undo := sheet(e).BgPhotoR.X, edtest.UndoLen(e)
	v, err := e.ApplyCoverPreset(cv.Preset)
	if err != nil || v.Cover.Fix != "" || spec(e).W != 286 || spec(e).Wrap != 20 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Исправить»: %v, строка %q, %+v", err, v.Cover.Fix, spec(e))
	}
	if x := sheet(e).BgPhotoR.X; x == x0 || x != v.Cover.Sheet.W-sheet(e).BgPhotoR.W {
		t.Fatalf("фон лицевой не по новой зоне: %v (было %v)", x, x0)
	}
	if e.Undo(); spec(e).Wrap != 23.5 || e.View().Cover.Fix == "" {
		t.Fatalf("Undo «Исправить»: %+v", spec(e))
	}
}

// TestSetSpecSidePage — сторонка 0 в своих числах значит «страница альбома» и в счёт пресета
// идёт как страница. На 21×30 высота шаблона (296) равна странице, поэтому H=0 — по-прежнему
// «Фотосфера», а на 30×30 (286 при странице 284) 0 — уже «Свои размеры». Неизвестный id
// пресета — тоже «Свои размеры», без ошибки; всё — одним шагом отмены.
func TestSetSpecSidePage(t *testing.T) {
	b := edtest.Open(t, model.New(), "")
	page(b, 208, 296)
	if _, err := b.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	sp := spec(b)
	sp.H = 0
	rev, undo := edtest.Rev(b), edtest.UndoLen(b)
	if _, err := b.SetCoverSpec(sp); err != nil || spec(b).Preset != model.CoverDefault || spec(b).H != 0 {
		t.Fatalf("H=0 при странице 296: %v %+v", err, spec(b))
	}
	if edtest.Rev(b) == rev || edtest.UndoLen(b) != undo+1 {
		t.Fatal("H=0 — не один шаг отмены")
	}
	sp = spec(b)
	sp.Preset = "нет-такого"
	if _, err := b.SetCoverSpec(sp); err != nil || spec(b).Preset != model.CoverCustom || spec(b).W != 211 {
		t.Fatalf("неизвестный пресет: %v %+v", err, spec(b))
	}
	if b.Undo(); spec(b).Preset != model.CoverDefault {
		t.Fatalf("Undo: %+v", spec(b))
	}

	e := coverAlbum(t, 2)
	sp = spec(e)
	sp.H = 0
	if _, err := e.SetCoverSpec(sp); err != nil || spec(e).Preset != model.CoverCustom || spec(e).H != 0 {
		t.Fatalf("H=0 при странице 284: %v %+v", err, spec(e))
	}
}
