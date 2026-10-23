package edit

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Статичные списки: пресеты типографий и размеры страницы не пустые и не требуют проекта.
func TestPresetsStatic(t *testing.T) {
	m := twoSpreads(t).Methods
	cp := m.CoverPresets()
	if len(cp) == 0 || cp[0].ID == "" {
		t.Errorf("пресеты обложки: %+v", cp)
	}
	if pp := m.PagePresets(); len(pp) == 0 || pp[0].W == 0 {
		t.Errorf("размеры страницы: %+v", pp)
	}
}

// Жизнь обложки через методы: каждая правка доходит до Changed ровно раз и виден в виде;
// отказ Editor (обложка уже есть, неизвестный пресет и раскладка) возвращается как есть и
// до Changed не доходит; холостая правка вид не меняет.
func TestCoverWiring(t *testing.T) {
	k := twoSpreads(t)
	r0 := k.rev()

	v, err := k.CreateCover("", "")
	if err != nil || v.Cover == nil || v.Rev <= r0 || k.calls != 1 {
		t.Fatalf("CreateCover: err %v, cover %v, rev %d→%d, Changed %d", err, v.Cover != nil, r0, v.Rev, k.calls)
	}
	// Обложка уже есть: ошибка, вид тот же, события нет.
	if v2, err := k.CreateCover("", ""); err == nil || v2.Rev != v.Rev || k.calls != 1 {
		t.Errorf("повторный CreateCover: err %v, rev %d, Changed %d", err, v2.Rev, k.calls)
	}

	presets := k.CoverPresets()
	last := presets[len(presets)-1]
	if v, err = k.ApplyCoverPreset(last.ID); err != nil || v.Cover.Preset != last.ID || k.calls != 2 {
		t.Errorf("ApplyCoverPreset: err %v, пресет %q (ждали %q), Changed %d", err, v.Cover.Preset, last.ID, k.calls)
	}
	if _, err = k.ApplyCoverPreset("нет-такого"); err == nil || k.calls != 2 {
		t.Errorf("неизвестный пресет: err %v, Changed %d", err, k.calls)
	}

	if v, err = k.SetCoverSpec(model.CoverSpec{Preset: model.CoverCustom, Wrap: 12}); err != nil ||
		v.Cover.Preset != model.CoverCustom || v.Cover.Spec.Wrap != 12 || k.calls != 3 {
		t.Errorf("SetCoverSpec: err %v, пресет %q, wrap %v, Changed %d", err, v.Cover.Preset, v.Cover.Spec.Wrap, k.calls)
	}

	if v, err = k.SetCoverLayout("wrap"); err != nil || v.Cover.Layout != "wrap" || k.calls != 4 {
		t.Errorf("SetCoverLayout: err %v, раскладка %q, Changed %d", err, v.Cover.Layout, k.calls)
	}
	if _, err = k.SetCoverLayout("кривая"); err == nil || k.calls != 4 {
		t.Errorf("неизвестная раскладка: err %v, Changed %d", err, k.calls)
	}

	if v = k.SetCoverSpineBg("#112233"); v.Cover.SpineBg != "#112233" {
		t.Errorf("плашка корешка: %q", v.Cover.SpineBg)
	}
	bg := v.Rev
	if v = k.SetCoverSpineBg("не цвет"); v.Rev != bg {
		t.Errorf("неверный цвет плашки изменил проект: rev %d → %d", bg, v.Rev)
	}
	if v = k.SetCoverSpineBg(""); v.Cover.SpineBg != "" {
		t.Errorf("плашка не снята: %q", v.Cover.SpineBg)
	}

	// Разметки нет: ClearCoverGuide холостая.
	before := k.rev()
	if v = k.ClearCoverGuide(); v.Rev != before {
		t.Errorf("ClearCoverGuide без разметки: rev %d → %d", before, v.Rev)
	}
	// Разметки разворотов нет: ClearSpreadGuide холостая; есть — шаг, Changed зовётся (066).
	calls := k.calls
	if v = k.ClearSpreadGuide(); v.Rev != before || k.calls != calls+1 {
		t.Errorf("ClearSpreadGuide без разметки: rev %d → %d, Changed %d", before, v.Rev, k.calls-calls)
	}
	if _, err := k.ed.SetSpreadGuide(testkit.AbsPath("Шаблоны/spread.psd")); err != nil {
		t.Fatal(err)
	}
	if v = k.ClearSpreadGuide(); v.SpreadMarkup != nil || v.Rev != before+2 {
		t.Errorf("ClearSpreadGuide: rev %d → %d, %+v", before, v.Rev, v.SpreadMarkup)
	}

	// Снять обложку: вид без неё; повторно — холостая. Undo возвращает обложку.
	if v = k.DeleteCover(); v.Cover != nil {
		t.Fatal("обложка не снята")
	}
	gone := v.Rev
	if v = k.DeleteCover(); v.Rev != gone {
		t.Errorf("повторное DeleteCover: rev %d → %d", gone, v.Rev)
	}
	if v = k.ed.Undo(); v.Cover == nil {
		t.Error("Undo не вернул обложку: холостая правка заняла шаг отмены?")
	}
}

// Обложка с фото: фото из ленты и из альбома встают фоном лицевой одним шагом; фото, которого
// нет ни там ни там, — ErrNoPhoto без обложки и без события.
func TestCreateCoverWithPhoto(t *testing.T) {
	k := twoSpreads(t)
	if _, err := k.CreateCover("", "нет"); !errors.Is(err, ErrNoPhoto) || k.calls != 0 || k.ed.View().Cover != nil {
		t.Fatalf("неизвестное фото: err %v, Changed %d", err, k.calls)
	}
	k.lentPhoto("L")
	v, err := k.CreateCover("", "L")
	if err != nil || v.Cover == nil || len(v.Cover.Sheet.Backdrops) != 1 || v.Cover.Sheet.Backdrops[0].Photo != "L" {
		t.Fatalf("фото ленты: err %v, cover %+v", err, v.Cover)
	}
	if k.ed.Undo(); k.ed.View().Cover != nil {
		t.Error("обложка с фото — больше одного шага отмены")
	}

	// Фото альбома (его нет в ленте) тоже годится: InfosOrRefs берёт сведения из проекта.
	if v, err = k.CreateCover("", "a"); err != nil || len(v.Cover.Sheet.Backdrops) != 1 || v.Cover.Sheet.Backdrops[0].Photo != "a" {
		t.Errorf("фото альбома: err %v", err)
	}
	if len(v.Spreads[0].Photos) != 2 {
		t.Errorf("фото ушло из своего разворота: %v", v.Spreads[0].Photos)
	}
}

// Виньетка: лист добавляется, снимается обратно в обычный; чтение и предпросмотр проект не
// трогают и берут шаблоны из Host.Templates.
func TestVignetteWiring(t *testing.T) {
	k := twoSpreads(t)
	tplCalls := 0
	k.h.Templates = func() []model.Template { tplCalls++; return nil }

	v, err := k.AddVignette(1)
	if err != nil || len(v.Spreads) != 3 || v.Spreads[1].Vig == nil || k.calls != 1 {
		t.Fatalf("AddVignette: err %v, листов %d, Changed %d", err, len(v.Spreads), k.calls)
	}
	id := v.Spreads[1].ID

	rev := k.rev()
	choices := k.VignetteChoices(id, 1, 4, "2:3")
	if len(choices) == 0 || tplCalls != 1 || k.rev() != rev || k.calls != 1 {
		t.Errorf("VignetteChoices: %d вариантов, шаблоны %d раз, Changed %d", len(choices), tplCalls, k.calls)
	}
	pr, err := k.PreviewVignette(id, model.VigSetup{Vig: v.Spreads[1].Vig.Vignette})
	if err != nil || pr.Spread.ID != id || tplCalls != 2 || k.rev() != rev || k.calls != 1 {
		t.Errorf("PreviewVignette: err %v, лист %q, шаблоны %d раз, rev %d→%d, Changed %d", err, pr.Spread.ID, tplCalls, rev, k.rev(), k.calls)
	}
	if _, err = k.PreviewVignette("нет", model.VigSetup{}); err == nil {
		t.Error("предпросмотр неизвестного листа без ошибки")
	}

	if v = k.SortVignette(id); v.Spreads[1].Vig == nil || k.calls != 2 {
		t.Errorf("SortVignette: Changed %d", k.calls)
	}
	if v = k.ClearVignette(id); v.Spreads[1].Vig != nil || k.calls != 3 {
		t.Errorf("ClearVignette: вид %v, Changed %d", v.Spreads[1].Vig, k.calls)
	}
	// Уже обычный: холостая, rev тот же.
	cleared := v.Rev
	if v = k.ClearVignette(id); v.Rev != cleared {
		t.Errorf("повторный ClearVignette: rev %d → %d", cleared, v.Rev)
	}
}

// Надпись на корешке: первый раз — правка, повторный вызов — холостая (rev тот же, шага отмены нет).
func TestCoverSpineTextWiring(t *testing.T) {
	k := twoSpreads(t)
	if _, err := k.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	v, err := k.CoverSpineText()
	if err != nil || len(v.Cover.Sheet.Texts) != 1 || k.calls != 2 {
		t.Fatalf("CoverSpineText: err %v, текстов %d, Changed %d", err, len(v.Cover.Sheet.Texts), k.calls)
	}
	withText := v.Rev
	if v, err = k.CoverSpineText(); err != nil || v.Rev != withText || len(v.Cover.Sheet.Texts) != 1 {
		t.Errorf("повторная надпись: err %v, rev %d → %d, текстов %d", err, withText, v.Rev, len(v.Cover.Sheet.Texts))
	}
	if v = k.ed.Undo(); len(v.Cover.Sheet.Texts) != 0 {
		t.Error("Undo не снял надпись: холостая правка заняла шаг?")
	}
}
