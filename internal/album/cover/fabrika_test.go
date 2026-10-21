package cover_test

import (
	"fmt"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// fabrikaAlbum — n разворотов на странице w×h и пустая обложка пресета preset.
func fabrikaAlbum(t *testing.T, w, h float64, n int, preset string) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = w, h
	for i := range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("%012x", i+1), Photos: []string{}})
	}
	e := edtest.Open(t, p, "")
	if _, err := e.CreateCover(preset, nil); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestCoverFollowsFormatFabrika (081 сц. 4, 11): обложка Фабрики ровно по шаблону переходит на
// шаблон нового формата тем же шагом отмены (20×20 → 30×30: заворот 17 → 12); Undo возвращает
// и страницу, и обложку одним шагом; тот же формат — холостая правка; свои числа не трогаются.
func TestCoverFollowsFormatFabrika(t *testing.T) {
	e := fabrikaAlbum(t, 200, 203, 3, "fabrika-layflat-card")
	if c := spec(e); c.Preset != "fabrika-layflat-card" || c.W != 206 || c.H != 206 || c.Wrap != 17 {
		t.Fatalf("новая обложка на 20×20: %+v", c)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	page(e, 200, 203)
	edtest.Same(t, e, rev, undo, "тот же формат")
	page(e, 300, 305)
	if c := spec(e); c.Preset != "fabrika-layflat-card" || c.W != 312 || c.H != 306 || c.Wrap != 12 {
		t.Fatalf("20×20 → 30×30: %+v", c)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e)-undo)
	}
	if e.Undo(); edtest.P(e).Settings.PageW != 200 || spec(e).W != 206 || spec(e).Wrap != 17 {
		t.Fatalf("Undo: страница %v, обложка %+v", edtest.P(e).Settings.PageW, spec(e))
	}
	// Две правки подряд — два шага; страница вне таблицы — шаблон ближайшего формата.
	page(e, 300, 300)
	if c := spec(e); c.W != 312 || c.H != 306 || c.Wrap != 12 {
		t.Fatalf("300×300: %+v", c)
	}
	// Свои числа: смена формата их не трогает.
	sp := spec(e)
	sp.W = 250
	if _, err := e.SetCoverSpec(sp); err != nil {
		t.Fatal(err)
	}
	own := spec(e)
	page(e, 250, 254)
	if c := spec(e); c.W != 250 || c.H != own.H || c.Wrap != own.Wrap || c.Preset != model.CoverCustom {
		t.Fatalf("своя обложка сменилась: %+v", c)
	}
}

// TestApplyPresetFabrika (081 сц. 3, 5, 11): пресет Фабрики берёт сторонку формата, после Нетпринта
// сторонка пользователя не теряется, переход к пресету другой типографии сбрасывает сторонку шаблона;
// «Исправить» — один шаг, строка «Не по шаблону Фабрики Фотокниги…» пропадает, Undo возвращает числа;
// неизвестный пресет — ошибка без шага.
func TestApplyPresetFabrika(t *testing.T) {
	e := fabrikaAlbum(t, 300, 305, 20, "fabrika-photo")
	v := e.View()
	if c := spec(e); c.W != 312 || c.Wrap != 12 || v.Cover.Fix != "" || v.Cover.Sheet.W != 662 || v.Cover.Sheet.H != 330 {
		t.Fatalf("30×30, 20 разворотов: %+v, лист %v × %v, строка %q", c, v.Cover.Sheet.W, v.Cover.Sheet.H, v.Cover.Fix)
	}
	// Руками сторонка 310: «Своя», строка не показывается (пресет сброшен в «Свои размеры»).
	sp := spec(e)
	sp.W = 310
	if _, err := e.SetCoverSpec(sp); err != nil || spec(e).Preset != model.CoverCustom {
		t.Fatalf("свои числа: %v %+v", err, spec(e))
	}
	// Тот же id пресета с неверными числами (как после правки таблицы) — «Не по шаблону» и «Исправить».
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Preset = "fabrika-photo" })
	if got := e.View().Cover.Fix; got != "Не по шаблону Фабрики Фотокниги 30×30: 658 × 330 мм, нужно 662 × 330" {
		t.Fatalf("строка: %q", got)
	}
	undo := edtest.UndoLen(e)
	v, err := e.ApplyCoverPreset(v.Cover.Preset)
	if err != nil || v.Cover.Fix != "" || spec(e).W != 312 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Исправить»: %v, строка %q, %+v", err, v.Cover.Fix, spec(e))
	}
	if e.Undo(); spec(e).W != 310 || e.View().Cover.Fix == "" {
		t.Fatalf("Undo «Исправить»: %+v", spec(e))
	}
	e.Redo()
	// Нетпринт после Фабрики: сторонка шаблона не уносится как «своя».
	if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil || spec(e).W != 0 || spec(e).H != 0 {
		t.Fatalf("Нетпринт после Фабрики: %v %+v", err, spec(e))
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyCoverPreset("fabrika-нет"); err == nil {
		t.Fatal("неизвестный пресет принят")
	}
	edtest.Same(t, e, rev, undo, "неизвестный пресет")
}
