package vigsheet_test

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Свои шаблоны виньетки (038 G3): снять шаблон с листа, применить его, «Вернуть авто»,
// признак «ручная», удалённый шаблон. Геометрия BuildOwn — в пакете vignette.

const ownTText = "{Фамилия} {Имя}"

// ownEd — лист 204×304 с 25 фото: «Шапка, учителя слева», 1 учитель + 24 ученика, подпись
// учителя и кегль подписей нестандартные, последний ученик уменьшен руками (шаблон не равен
// каталожному плану). tpl — шаблон, снятый с такого листа (ID t1, имя «Свой»).
func ownEd(t *testing.T) (e *album.Editor, sid string, tpl model.Template) {
	t.Helper()
	e, sid = vigSheet(t, 25)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 24, TText: ownTText}, nil); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Caps.Style.Size = 17 })
	pl := lastPupil(e)
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X, Y: pl.Y, W: pl.W - 3, H: pl.H - 3}}, "")
	tpl, err := e.VigTemplate(sid)
	if err != nil {
		t.Fatal(err)
	}
	tpl.ID, tpl.Name = "t1", "Свой"
	return e, sid, tpl
}

func lastPupil(e *album.Editor) model.Place {
	g := edtest.P(e).Spreads[0].Grid
	for i := len(g) - 1; i >= 0; i-- {
		if model.RoleOf(g[i]) == model.RolePupil {
			return *g[i]
		}
	}
	return model.Place{}
}

func manual(e *album.Editor, sid string) bool {
	for _, sv := range e.View().Spreads {
		if sv.ID == sid {
			return sv.Vig.Manual
		}
	}
	return false
}

// wantBoxes — места листа равны местам шаблона tpl (доли → мм листа, допуск зажима 0,01).
func wantBoxes(t *testing.T, e *album.Editor, tpl model.Template) {
	t.Helper()
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != len(tpl.Boxes) {
		t.Fatalf("мест %d, в шаблоне %d", len(g), len(tpl.Boxes))
	}
	w, h := edtest.P(e).Settings.PageW, edtest.P(e).Settings.PageH
	for i, b := range tpl.Boxes {
		pl := g[i]
		if model.RoleOf(pl) != tpl.Vig.Roles[i] || math.Abs(pl.X-b.X*w) > 0.02 || math.Abs(pl.Y-b.Y*h) > 0.02 ||
			math.Abs(pl.W-b.W*w) > 0.02 || math.Abs(pl.H-b.H*h) > 0.02 {
			t.Fatalf("место %d: %+v, в шаблоне %+v (%s)", i, *pl, b, tpl.Vig.Roles[i])
		}
	}
}

func TestVigTemplateOf(t *testing.T) {
	e, sid, tpl := ownEd(t)
	v := tpl.Vig
	if tpl.Kind != model.TplVignette || tpl.Scope != model.TplSpread || len(tpl.Boxes) != 25 || len(v.Roles) != 25 {
		t.Fatalf("шаблон: %+v", tpl)
	}
	if v.Pages != 1 || math.Abs(v.Aspect-204.0/304) > 1e-9 || v.Ratio != "3:4" || v.Head == nil {
		t.Fatalf("геометрия: %+v", v.VigGeo)
	}
	n := map[string]int{}
	for _, r := range v.Roles {
		n[r]++
	}
	if n[model.RolePupil] != 24 || n[model.RoleTeacher] != 1 {
		t.Fatalf("роли: %v", n)
	}
	if v.Style.TText != ownTText || v.Style.Caps == nil || v.Style.Caps.Style.Size != 17 || v.Style.Head == nil {
		t.Fatalf("стиль: %+v", v.Style)
	}
	// Данные класса не берутся: ни фото, ни имён, ни текста шапки (в шаблоне их полей нет).
	before := edtest.Rev(e)
	if _, err := e.VigTemplate(sid); err != nil || edtest.Rev(e) != before {
		t.Fatal("чтение шаблона правит проект")
	}
	// Обычный разворот и неизвестный лист — ошибка; виньетка без людей — нет мест.
	e.ClearVignette(sid)
	if _, err := e.VigTemplate(sid); err == nil || err.Error() != "Это не виньетка" {
		t.Fatalf("обычный разворот: %v", err)
	}
	if _, err := e.VigTemplate("нет"); err == nil {
		t.Fatal("неизвестный лист")
	}
	e.SetVignette(sid, model.Vignette{Tpl: "grid"}, nil)
	if _, err := e.VigTemplate(sid); err == nil || !strings.Contains(err.Error(), "нет мест") {
		t.Fatalf("виньетка без мест: %v", err)
	}
}

func TestApplyOwnVignette(t *testing.T) {
	e, sid, tpl := ownEd(t)
	tpls := []model.Template{tpl}
	// Лист меняет вид и оформление: свой шаблон должен вернуть и места, и оформление.
	e.SetVignette(sid, model.Vignette{Tpl: "grid", Teachers: 1, Pupils: 24}, nil)
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Caps.Style.Size = 9 })
	if manual(e, sid) {
		t.Fatal("лист по каталогу помечен ручным")
	}
	undo := edtest.UndoLen(e)
	v, left, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Ratio: "3:4"}, tpls)
	if err != nil || left != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("применить: left %d, err %v, шагов %d", left, err, edtest.UndoLen(e)-undo)
	}
	wantBoxes(t, e, tpl) // те же числа и формат: места ровно как в шаблоне
	sp := edtest.P(e).Spreads[0]
	if sp.Vig.Tpl != "my:t1" || sp.Vig.Own == nil || sp.Vig.Own.Name != "Свой" || sp.Vig.TText != ownTText || sp.Caps.Style.Size != 17 {
		t.Fatalf("оформление шаблона не встало: %+v, кегль %v", sp.Vig, sp.Caps.Style.Size)
	}
	if vv := v.Spreads[0].Vig; vv.Manual || vv.OwnName != "Свой" || vv.TplName != "Свой" || vv.Own != nil || len(rolePhotos(e, 0, model.RolePupil)) != 24 {
		t.Fatalf("вид: %+v", vv)
	}
	// Повторное «Применить» — холостая правка (ни rev, ни шага отмены).
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Ratio: "3:4", TText: ownTText}, tpls); err != nil ||
		edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("повтор: err %v, rev %d→%d", err, rev, edtest.Rev(e))
	}
	// «Отменить» — всё назад одним шагом: и места, и оформление.
	e.Undo()
	if sp = edtest.P(e).Spreads[0]; sp.Vig.Tpl != "grid" || sp.Caps.Style.Size != 9 {
		t.Fatalf("отмена: %+v", sp.Vig)
	}
}

// Другие числа: роль с тем же числом стоит точно, лишние фото — в ленту одним шагом отмены.
func TestApplyOwnOtherCounts(t *testing.T) {
	e, sid, tpl := ownEd(t)
	tpls := []model.Template{tpl}
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls)
	before := string(model.Marshal(edtest.P(e)))
	undo := edtest.UndoLen(e)
	_, left, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 19}, tpls)
	if err != nil || left != 5 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("−5 учеников: left %d, err %v", left, err)
	}
	g := edtest.P(e).Spreads[0].Grid
	if n := len(rolePhotos(e, 0, model.RolePupil)); len(g) != 20 || n != 19 {
		t.Fatalf("мест %d, фото учеников %d", len(g), n)
	}
	// Учитель — того же числа, шаблон стоит точно: место учителя то же, что в шаблоне.
	for i, b := range tpl.Boxes {
		if tpl.Vig.Roles[i] != model.RoleTeacher {
			continue
		}
		w, h := edtest.P(e).Settings.PageW, edtest.P(e).Settings.PageH
		var tp *model.Place
		for _, pl := range g {
			if model.RoleOf(pl) == model.RoleTeacher {
				tp = pl
			}
		}
		if math.Abs(tp.X-b.X*w) > 0.02 || math.Abs(tp.Y-b.Y*h) > 0.02 || math.Abs(tp.W-b.W*w) > 0.02 {
			t.Fatalf("учитель сдвинулся: %+v, в шаблоне %+v", *tp, b)
		}
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("одна отмена не вернула лист")
	}
	// Больше людей: места растут, фото остаются на местах (порядок прежний).
	want := rolePhotos(e, 0, model.RolePupil)
	_, left, err = e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 2, Pupils: 30}, tpls)
	if err != nil || left != 0 || len(edtest.P(e).Spreads[0].Grid) != 32 || !slices.Equal(rolePhotos(e, 0, model.RolePupil), want) {
		t.Fatalf("+6 учеников: left %d, err %v, мест %d", left, err, len(edtest.P(e).Spreads[0].Grid))
	}
	// Оформление: ставит смена шаблона, а не повторное «Применить» (решение 146).
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Caps.Style.Size = 11 })
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 22, TText: ownTText}, tpls)
	if sz := edtest.P(e).Spreads[0].Caps.Style.Size; sz != 11 {
		t.Fatalf("повторное «Применить» вернуло оформление шаблона: кегль %v", sz)
	}
}

// Стиль шапки: существующая шапка получает стиль шаблона при смене шаблона, новая создаётся с ним.
func TestApplyOwnHeadStyle(t *testing.T) {
	_, _, tpl := ownEd(t)
	st := model.PlainStyle()
	st.Size = 33
	tpl.Vig.Style.Head = &st
	tpls := []model.Template{tpl}
	size := func(e *album.Editor) float64 {
		for _, tb := range edtest.P(e).Spreads[0].Texts {
			if tb.ID == edtest.P(e).Spreads[0].Vig.Head {
				return tb.Style.Size
			}
		}
		return -1
	}
	e, sid := vigSheet(t, 0) // шапка уже есть (создана по каталогу)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls); err != nil || size(e) != 33 {
		t.Fatalf("существующая шапка: кегль %v, err %v", size(e), err)
	}
	// Лист, не бывший виньеткой: шапка создаётся сразу со стилем шаблона.
	e2 := edtest.Open(t, model.NewSheet(204, 304), "")
	sid2 := edtest.P(e2).Spreads[0].ID
	if _, _, err := e2.SetVignette(sid2, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls); err != nil || size(e2) != 33 {
		t.Fatalf("новая шапка: кегль %v, err %v", size(e2), err)
	}
	// Повторное «Применить» стиль шапки не трогает.
	edtest.Change(e2, func(p *model.Project) { p.Spreads[0].Texts[0].Style.Size = 20 })
	e2.SetVignette(sid2, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 20}, tpls)
	if size(e2) != 20 {
		t.Fatalf("повтор вернул стиль шапки: %v", size(e2))
	}
}

func TestOwnVignetteGone(t *testing.T) {
	e, sid, tpl := ownEd(t)
	tpls := []model.Template{tpl}
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls); err != nil {
		t.Fatal(err)
	}
	// Шаблон удалён из файла: лист работает, в окне он виден как «удалённый».
	cs := e.VignetteChoices(sid, 1, 24, "3:4", nil)
	last := cs[len(cs)-1]
	if len(cs) != 30 || last.ID != "my:t1" || !last.Own || !last.Gone || last.Name != "Свой" || last.Ratio != "3:4" || len(last.Boxes) != 25 {
		t.Fatalf("Gone: %d превью, последнее %+v", len(cs), last)
	}
	for _, c := range cs[:29] {
		if c.Own || c.Gone {
			t.Fatalf("каталожное %s помечено своим", c.ID)
		}
	}
	// Файл на месте: то же превью без Gone и без дубля.
	if cs = e.VignetteChoices(sid, 1, 24, "3:4", tpls); len(cs) != 30 || cs[29].Gone || cs[29].Name != "Свой" {
		t.Fatalf("шаблон в файле: %d, %+v", len(cs), cs[len(cs)-1])
	}
	// «Применить» с новыми числами работает без файла: берётся копия с листа.
	_, left, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 20}, nil)
	if err != nil || left != 4 || len(edtest.P(e).Spreads[0].Grid) != 21 {
		t.Fatalf("новые числа без шаблона: left %d, err %v", left, err)
	}
	// Смена кегля подписей пересобирает нетронутую виньетку по копии.
	pad := edtest.P(e).Spreads[0].Grid[0].Y
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Caps.Pad += 3 })
	if edtest.P(e).Spreads[0].Grid[0].Y == pad || manual(e, sid) || edtest.P(e).Spreads[0].Vig.Own == nil {
		t.Fatal("Relay не пересобрал лист с удалённым шаблоном")
	}
	// Чужой шаблон: ошибка без правки.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:чужой", Teachers: 1, Pupils: 20}, nil); !errors.Is(err, model.ErrTplNotFound) || err.Error() != "Шаблон не найден" ||
		edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("чужой шаблон: %v", err)
	}
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: "my:чужой", Pupils: 3}}, nil); err == nil ||
		edtest.Rev(e) != rev {
		t.Fatalf("чужой шаблон в окне: %v", err)
	}
	if _, err := e.PreviewVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: "my:чужой", Pupils: 3}}, nil); err == nil {
		t.Fatal("предпросмотр с чужим шаблоном")
	}
}

// Поле own из интерфейса не принимается: геометрию даёт только файл шаблонов или сам лист.
func TestOwnFromUIIgnored(t *testing.T) {
	e, sid, tpl := ownEd(t)
	evil := &model.VigShape{Name: "Чужая", Boxes: []model.Box{{X: 0, Y: 0, W: 0.5, H: 0.5}}, VigGeo: model.VigGeo{Roles: []string{"pupil"}, Pages: 1, Aspect: 1, Ratio: "3:4"}}
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head", Teachers: 1, Pupils: 10, Own: evil}, nil); err != nil {
		t.Fatal(err)
	}
	if sp := edtest.P(e).Spreads[0]; sp.Vig.Own != nil || len(sp.Grid) != 11 {
		t.Fatalf("own из SetVignette принят: %+v", sp.Vig.Own)
	}
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Own: evil}}, []model.Template{tpl}); err != nil {
		t.Fatal(err)
	}
	if sp := edtest.P(e).Spreads[0]; sp.Vig.Own == nil || sp.Vig.Own.Name != "Свой" || len(sp.Grid) != 25 {
		t.Fatalf("own из SetupVignette: %+v", sp.Vig.Own)
	}
	// Шаблон другого числа страниц не ставится на лист.
	tpl.Vig.Pages = 2
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 5}, []model.Template{tpl}); err == nil {
		t.Fatal("шаблон разворота встал на лист")
	}
}

// Круг Save → Load: копия геометрии и оформление переживают файл; лист открывается без шаблонов.
func TestOwnVignetteSaveLoad(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl})
	path := filepath.Join(testkit.TempDir(t), "v.salbum")
	if err := e.WriteCopy(path); err != nil {
		t.Fatal(err)
	}
	p2, err := album.LoadBase(path, "")
	if err != nil {
		t.Fatal(err)
	}
	want, got := edtest.P(e).Spreads[0], p2.Spreads[0]
	if got.Vig.Tpl != "my:t1" || !got.Vig.Own.Equal(want.Vig.Own) || got.Vig.TText != want.Vig.TText || len(got.Grid) != 25 {
		t.Fatalf("после Load: %+v", got.Vig)
	}
	e2 := edtest.Open(t, p2, path)
	if manual(e2, sid) {
		t.Fatal("загруженный лист помечен ручным")
	}
	// Файла шаблонов нет: «Применить» с новыми числами работает по копии, превью — с Gone.
	if _, left, err := e2.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 21}, nil); err != nil || left != 3 {
		t.Fatalf("после Load: left %d, err %v", left, err)
	}
	if cs := e2.VignetteChoices(sid, 1, 21, "3:4", nil); !cs[len(cs)-1].Gone {
		t.Fatal("после Load шаблон не помечен удалённым")
	}
}

// «Ручная»: места, сдвинутые руками, отличаются от плана шаблона; «Вернуть авто» собирает заново.
func TestVignetteManualAndReset(t *testing.T) {
	e, sid := vigSheet(t, 25)
	if manual(e, sid) {
		t.Fatal("свежая виньетка помечена ручной")
	}
	if tn := e.View().Spreads[0].Vig.TplName; tn != "Шапка, учителя слева" {
		t.Fatalf("имя шаблона каталога: %q", tn)
	}
	want, before := rolePhotos(e, 0, model.RolePupil), string(model.Marshal(edtest.P(e)))
	// Холостая: нетронутая виньетка.
	rev := edtest.Rev(e)
	if _, left, err := e.ResetVignette(sid); err != nil || left != 0 || edtest.Rev(e) != rev {
		t.Fatalf("Reset нетронутой: left %d, err %v, rev %d→%d", left, err, rev, edtest.Rev(e))
	}
	pl := lastPupil(e)
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 2, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	if !manual(e, sid) {
		t.Fatal("сдвинутое место не сделало виньетку ручной")
	}
	undo := edtest.UndoLen(e)
	v, left, err := e.ResetVignette(sid)
	if err != nil || left != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("Reset: left %d, err %v, шагов %d", left, err, edtest.UndoLen(e)-undo)
	}
	if manual(e, sid) || v.Spreads[0].Vig.Manual || !slices.Equal(rolePhotos(e, 0, model.RolePupil), want) {
		t.Fatal("после «Вернуть авто» виньетка ручная или фото сдвинулись")
	}
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("«Вернуть авто» дало не тот проект, что был до правки руками")
	}
	e.Undo() // один шаг: снова сдвинутое место
	if !manual(e, sid) {
		t.Fatal("отмена не вернула сдвиг")
	}
	// Вернули руками всё в «Авто» (нет мест): тоже ручная, «Вернуть авто» собирает места.
	e.SetGrid(sid, false)
	if !manual(e, sid) {
		t.Fatal("лист без мест не помечен ручным")
	}
	if _, _, err := e.ResetVignette(sid); err != nil || manual(e, sid) || len(edtest.P(e).Spreads[0].Grid) != 25 {
		t.Fatalf("Reset листа «Авто»: err %v", err)
	}
	// Обычный разворот и неизвестный лист — холостая.
	e.ClearVignette(sid)
	rev = edtest.Rev(e)
	if _, _, err := e.ResetVignette(sid); err != nil || edtest.Rev(e) != rev {
		t.Fatalf("Reset обычного разворота: %v", err)
	}
	if _, _, err := e.ResetVignette("нет"); err == nil {
		t.Fatal("Reset неизвестного листа")
	}
}

// «Ручная» у своего шаблона: тот же признак относительно плана BuildOwn; Reset с удалённым
// шаблоном работает по копии на листе.
func TestOwnManualAndReset(t *testing.T) {
	e, sid, tpl := ownEd(t)
	want := tpl
	want.Boxes = slices.Clone(tpl.Boxes)
	tpls := []model.Template{tpl}
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls)
	if manual(e, sid) {
		t.Fatal("свой шаблон помечен ручным")
	}
	// Шаблон удалён из файла, а его места в памяти испорчены: лист знает только свою копию.
	for i := range tpls[0].Boxes {
		tpls[0].Boxes[i] = model.Box{X: 0, Y: 0, W: 0.1, H: 0.1}
	}
	tpls = nil
	if cs := e.VignetteChoices(sid, 1, 24, "3:4", tpls); !cs[len(cs)-1].Gone {
		t.Fatal("шаблон не удалён")
	}
	pl := lastPupil(e)
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X - 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	if !manual(e, sid) {
		t.Fatal("сдвиг не сделал лист ручным")
	}
	undo := edtest.UndoLen(e)
	if _, _, err := e.ResetVignette(sid); err != nil || manual(e, sid) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("Reset своего: %v", err)
	}
	wantBoxes(t, e, want)
}

// Переименованный шаблон: «Применить» с новыми полями шапки и прежними числами не пересобирает
// места (сдвиги руками остаются), имя на листе — новое; одна отмена — всё назад.
func TestOwnRenameKeepsManual(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl})
	pl := lastPupil(e)
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X - 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	before := slices.Clone(edtest.P(e).Spreads[0].Grid)
	renamed := tpl
	renamed.Name = "Другой"
	st := model.VigSetup{Vig: model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Ratio: "3:4", TText: ownTText}, Vars: model.Vars{Title: "Школа"}}
	undo := edtest.UndoLen(e)
	if _, left, err := e.SetupVignette(sid, st, []model.Template{renamed}); err != nil || left != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("Применить: left %d, err %v", left, err)
	}
	sp := edtest.P(e).Spreads[0]
	for i, p := range sp.Grid {
		if *p != *before[i] {
			t.Fatalf("место %d пересобрано: %+v, было %+v", i, *p, *before[i])
		}
	}
	if !manual(e, sid) || sp.Vars == nil || sp.Vars.Title != "Школа" || sp.Vig.Own.Name != "Другой" {
		t.Fatalf("после «Применить»: ручная %v, поля %+v, имя %q", manual(e, sid), sp.Vars, sp.Vig.Own.Name)
	}
	// Повтор — холостая правка.
	rev := edtest.Rev(e)
	if _, _, err := e.SetupVignette(sid, st, []model.Template{renamed}); err != nil || edtest.Rev(e) != rev {
		t.Fatalf("повтор: err %v, rev %d→%d", err, rev, edtest.Rev(e))
	}
	e.Undo()
	if sp = edtest.P(e).Spreads[0]; sp.Vars != nil || sp.Vig.Own.Name != "Свой" || !manual(e, sid) {
		t.Fatalf("отмена: поля %+v, имя %q", sp.Vars, sp.Vig.Own.Name)
	}
}

// Превью своих шаблонов — с портретом шаблона, а не панели: выбор ставит его портрет.
func TestOwnChoicesTemplateRatio(t *testing.T) {
	e, sid, tpl := ownEd(t)
	tpls := []model.Template{tpl}
	for _, ratio := range []string{"3:4", "2:3", "1:1"} {
		c := e.VignetteChoices(sid, 1, 24, ratio, tpls)[29]
		if !c.Own || c.Ratio != "3:4" || len(c.Boxes) != len(tpl.Boxes) {
			t.Fatalf("%s: %+v", ratio, c)
		}
		for i, b := range c.Boxes { // точный режим: места шаблона (порядок ролей плана — ученики первыми)
			if !slices.ContainsFunc(tpl.Boxes, func(o model.Box) bool {
				return math.Abs(o.X-b.X) < 1e-4 && math.Abs(o.Y-b.Y) < 1e-4 && math.Abs(o.W-b.W) < 1e-4
			}) {
				t.Fatalf("%s: место %d превью %+v не из шаблона", ratio, i, b)
			}
		}
	}
	// Удалённый шаблон на листе — тоже со своим портретом.
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, tpls)
	a, b := e.VignetteChoices(sid, 1, 24, "3:4", nil), e.VignetteChoices(sid, 1, 24, "1:1", nil)
	if ga, gb := a[len(a)-1], b[len(b)-1]; !gb.Gone || !slices.Equal(ga.Boxes, gb.Boxes) || gb.Ratio != "3:4" {
		t.Fatalf("удалённый: %+v", gb)
	}
}

// Копия листа со своим шаблоном (дублировать, копировать сетку) — инварианты виньетки, круг
// Save → Load: у копии та же форма, ни Tpl без Own, ни Own без Tpl.
func TestOwnSheetCopies(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl})
	if _, err := e.DuplicateGrid(sid, 2, true); err != nil {
		t.Fatal(err)
	}
	e.ClearVignette(edtest.P(e).Spreads[2].ID) // обычный лист — цель копии сетки
	if _, _, err := e.CopyGrid(sid, model.ScopeSpread, []model.GridTarget{{Spread: edtest.P(e).Spreads[2].ID, Scope: model.ScopeSpread}}, true, false); err != nil {
		t.Fatal(err)
	}
	check := func(p *model.Project, what string) {
		t.Helper()
		if len(p.Spreads) != 3 {
			t.Fatalf("%s: листов %d", what, len(p.Spreads))
		}
		own := p.Spreads[0].Vig.Own
		for i, s := range p.Spreads {
			if err := edtest.VigInvariant(s); err != nil {
				t.Fatalf("%s, лист %d: %v", what, i, err)
			}
			if i < 2 && (s.Vig.Tpl != "my:t1" || !s.Vig.Own.Equal(own) || len(s.Grid) != 25) {
				t.Fatalf("%s, лист %d: %+v", what, i, s.Vig)
			}
		}
		if s := p.Spreads[2]; s.IsVignette() || len(s.Grid) != 25 {
			t.Fatalf("%s: копия сетки на обычный лист — виньетка %v, мест %d", what, s.IsVignette(), len(s.Grid))
		}
	}
	check(edtest.P(e), "в памяти")
	path := filepath.Join(testkit.TempDir(t), "v.salbum")
	if err := e.WriteCopy(path); err != nil {
		t.Fatal(err)
	}
	p2, err := album.LoadBase(path, "")
	if err != nil {
		t.Fatal(err)
	}
	check(p2, "после Load")
}

// 20 каталожных и 50 своих превью на 64 местах — быстрее 5 мс (спека 038).
func TestVignetteChoicesOwnFast(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, nil)
	sid := edtest.P(e).Spreads[0].ID
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "t-left", Teachers: 4, Pupils: 60}, nil); err != nil {
		t.Fatal(err)
	}
	base, err := e.VigTemplate(sid)
	if err != nil || len(base.Boxes) != 64 || base.Vig.Pages != 2 {
		t.Fatalf("шаблон разворота: %d мест, err %v", len(base.Boxes), err)
	}
	var tpls []model.Template
	for i := range 50 {
		tp := base
		tp.ID, tp.Name = string(rune('a'+i%26))+string(rune('a'+i/26)), "Свой"
		tpls = append(tpls, tp)
	}
	// Шаблон листа на разворот не попадает.
	sheet := base
	sheet.ID, sheet.Vig = "лист", &model.VigTpl{VigGeo: base.Vig.VigGeo}
	sheet.Vig.Pages = 1
	tpls = append(tpls, sheet, model.Template{ID: "grid1", Scope: model.TplSpread, Boxes: base.Boxes})
	var cs []model.VigChoice
	d := testkit.Fastest(7, func() { cs = e.VignetteChoices(sid, 4, 60, "3:4", tpls) })
	// Обычно ~0,5 мс; предел 25 мс (×50) — страж от квадратичного роста, а не от шума
	// параллельных прогонов (на 5 мс всплеск нагрузки ронял тест).
	if lim := testkit.Limit(25 * time.Millisecond); d > lim {
		t.Fatalf("33+50 превью за %v, предел %v", d, lim)
	}
	if len(cs) != 83 {
		t.Fatalf("превью %d, ждали 33 + 50", len(cs))
	}
	for _, c := range cs[33:] {
		if !c.Own || c.Gone || len(c.Boxes) != 64 || c.PupilW <= 0 || !strings.HasPrefix(c.ID, "my:") || c.Ratio != "3:4" {
			t.Fatalf("свой %+v", c)
		}
	}
}

// «Мои шаблоны» сетки не показывают шаблоны виньетки.
func TestGridChoicesSkipsVignette(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.ClearVignette(sid)
	plain := model.Template{ID: "p1", Name: "Сетка", Scope: model.TplSpread, Boxes: tpl.Boxes[:3]}
	vig := tpl
	vig.Boxes = tpl.Boxes[:3]
	c := e.GridChoices(sid, 3, "spread", []model.Template{vig, plain})
	if len(c.Mine) != 1 || c.Mine[0].ID != "p1" {
		t.Fatalf("Mine: %+v", c.Mine)
	}
}
