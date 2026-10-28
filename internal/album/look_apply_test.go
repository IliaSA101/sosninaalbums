package album_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Постановка макета альбома серией (131 design §4, G4a): новый альбом, повтор, отмена, отказ,
// Save→Load. Режимы собранного альбома (add, bg) — G4b.

const (
	lkKeyA = "my:0123456789abcdef"
	lkKeyB = "my:fedcba9876543210"
)

var (
	lkRefA = model.LookRef{Key: lkKeyA, Name: "Классика"}
	lkRefB = model.LookRef{Key: lkKeyB, Name: "Свежий"}
)

func lkBox(x, y, w, h float64) model.Box { return model.Box{X: x, Y: y, W: w, H: h} }

// lkPlaces — n мест в ряд на разворот-ratio 2: ширина делится поровну.
func lkPlaces(n int) []model.Box {
	out := make([]model.Box, n)
	for i := range n {
		w := 0.9 / float64(n)
		out[i] = lkBox(0.05+w*float64(i), 0.1, w*0.9, 0.8)
	}
	return out
}

// lkStep — шаг-разворот (Pages 2, ratio 2 — страницы квадратные): n мест и цвет страниц.
func lkStep(role, name, color string, n int) model.LookStep {
	return model.LookStep{Role: role, Name: name, Tpl: model.SpreadTpl{
		V: model.TplV, Pages: 2, Ratio: 2, Places: lkPlaces(n), Page: []model.TplBg{{Color: color}, {Color: color}}}}
}

// lkDraft добавляет шагу заготовку надписи.
func lkDraft(st model.LookStep) model.LookStep {
	st.Tpl.Drafts = []model.TplText{{Anchor: model.Anchor{AY: 1, DY: -0.1}, W: 0.5, Text: "[Имена]", Style: model.PlainStyle()}}
	return st
}

func lkVig(name string, pupils int) model.LookStep {
	return model.LookStep{Role: model.RoleVignette, Name: name,
		Tpl: model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#f5f0e8"}}},
		Vig: &model.LookVig{Tpl: "head-t-left", Teachers: 1, Pupils: pupils, Ratio: "3:4"}}
}

// lookA: 0 титул (заготовка), 1 виньетка, 2 сборы, 3–4 повтор, 5 финал, 6 виньетка, 7 конец.
// Series(A, 9): 0 1 2 | 3 4 3 | 5 6 7 — шаги 1 2 3 4 5 4 6 7 8.
func lookA() model.AlbumTpl {
	return model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{
		lkDraft(lkStep("", "Титул", "#ffffff", 1)), lkVig("Класс", 4), lkStep("", "Сборы", "#fff0f0", 2),
		lkStep(model.RoleRepeat, "", "#f0fff0", 1), lkStep(model.RoleRepeat, "", "#f0f0ff", 3),
		lkStep("", "Финал", "#eeeeee", 2), lkVig("Учителя", 6), lkStep("", "Конец", "#dddddd", 1),
	}}
}

// lookB: 0 титул, 1 виньетка, 2 повтор, 3 финал. Series(B, 6): 0 1 | 2 2 2 | 3.
func lookB() model.AlbumTpl {
	return model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{
		lkStep("", "Начало", "#ffeedd", 2), lkVig("Класс", 8), lkStep(model.RoleRepeat, "", "#ddeeff", 4), lkStep("", "Финал", "#ccbbaa", 1),
	}}
}

func lkEditor(t *testing.T) *album.Editor {
	t.Helper()
	return edtest.Open(t, model.New(), "")
}

func lkApply(t *testing.T, e *album.Editor, ref model.LookRef, tpl model.AlbumTpl, find album.LookFinder, n int) model.LookPlan {
	t.Helper()
	_, plan, err := e.ApplyLook(ref, tpl, find, model.LookOpts{Spreads: n}, nil)
	if err != nil {
		t.Fatalf("ApplyLook: %v", err)
	}
	return plan
}

// lkRow — развороты проекта для сверки: ID, шаг, имя, ID мест.
type lkRow struct {
	ID, Name string
	Step     int
	Places   []string
	Vig      bool
}

func lkRows(e *album.Editor) []lkRow {
	var out []lkRow
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	for _, s := range edtest.P(e).Spreads {
		r := lkRow{ID: s.ID, Name: s.Name, Step: s.LookStep, Vig: s.IsVignette()}
		for _, pl := range s.Grid {
			r.Places = append(r.Places, pl.ID)
		}
		out = append(out, r)
	}
	return out
}

func lkSteps(rows []lkRow) []int {
	var out []int
	for _, r := range rows {
		out = append(out, r.Step)
	}
	return out
}

func lkJSON(e *album.Editor) string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	return string(model.Marshal(edtest.P(e)))
}

// Новый альбом: серия по Series, имена и номера шагов, места по LookBoxes, заготовки, виньетки.
func TestApplyLookNew(t *testing.T) {
	e := lkEditor(t)
	a := lookA()
	plan := lkApply(t, e, lkRefA, a, nil, 9)
	rows := lkRows(e)
	if want := []int{1, 2, 3, 4, 5, 4, 6, 7, 8}; !slices.Equal(lkSteps(rows), want) {
		t.Fatalf("шаги: %v, ждали %v", lkSteps(rows), want)
	}
	for i, st := range []string{"Титул", "Класс", "Сборы", "", "", "", "Финал", "Учителя", "Конец"} {
		if rows[i].Name != st {
			t.Errorf("разворот %d: имя %q, ждали %q", i, rows[i].Name, st)
		}
	}
	for i, vig := range []bool{false, true, false, false, false, false, false, true, false} {
		if rows[i].Vig != vig {
			t.Errorf("разворот %d: виньетка %v", i, rows[i].Vig)
		}
	}
	// Места обычных шагов — по числу мест шага; пустые (фото нет).
	for i, n := range map[int]int{0: 1, 2: 2, 3: 1, 4: 3, 5: 1, 6: 2, 8: 1} {
		if len(rows[i].Places) != n {
			t.Errorf("разворот %d: мест %d, ждали %d", i, len(rows[i].Places), n)
		}
	}
	if len(rows[1].Places) != 5 || len(rows[7].Places) != 7 { // учитель + ученики
		t.Errorf("места виньеток: %d и %d", len(rows[1].Places), len(rows[7].Places))
	}
	p := edtest.P(e)
	if p.Look == nil || p.Look.Key != lkKeyA || p.Look.Name != "Классика" {
		t.Fatalf("Project.Look = %+v", p.Look)
	}
	if p.Spreads[2].BgL != "#fff0f0" || p.Spreads[0].BgR != "#ffffff" {
		t.Errorf("фон страниц шага не встал: %q %q", p.Spreads[2].BgL, p.Spreads[0].BgR)
	}
	drafts := 0
	for _, tx := range p.Spreads[0].Texts {
		if tx.Draft {
			drafts++
		}
	}
	if drafts != 1 || plan.Drafts != 1 {
		t.Errorf("заготовок на титуле %d, в плане %d", drafts, plan.Drafts)
	}
	for _, s := range p.Spreads {
		for _, pl := range s.Grid {
			if pl.Photo != "" {
				t.Fatal("места макета должны быть пустыми")
			}
		}
	}
	want := model.LookPlan{Mode: model.LookSeries, Spreads: 9, Places: 1 + 5 + 2 + 1 + 3 + 1 + 2 + 7 + 1, Drafts: 1}
	if plan != want {
		t.Errorf("план %+v, ждали %+v", plan, want)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Серия без указанного числа — все шаги по разу; повтор из-за зажима n не меньше начала и конца.
func TestApplyLookDefaultCount(t *testing.T) {
	e := lkEditor(t)
	if plan := lkApply(t, e, lkRefA, lookA(), nil, 0); plan.Spreads != 8 {
		t.Fatalf("умолчание: %d разворотов, ждали 8", plan.Spreads)
	}
	for n, want := range map[int]int{1: 7, -5: 8, 1 << 30: model.MaxSpreads} { // 3 + 1 повтор + 3; умолчание; предел
		e = lkEditor(t)
		if plan := lkApply(t, e, lkRefA, lookA(), nil, n); plan.Spreads != want || len(edtest.P(e).Spreads) != want {
			t.Fatalf("n = %d: %d разворотов, ждали %d", n, plan.Spreads, want)
		}
	}
}

// Шаг vignette: лист 031 с параметрами шага и оформлением; повтор холостой, смена числа учеников
// обновляет тот же лист, сдвинутые места возвращаются «Вернуть авто» тем же ID.
func TestApplyLookVignette(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	v := edtest.P(e).Spreads[1]
	if !v.IsVignette() || v.Vig.Tpl != "head-t-left" || v.Vig.Teachers != 1 || v.Vig.Pupils != 4 || v.Vig.Ratio != "3:4" {
		t.Fatalf("параметры виньетки: %+v", v.Vig)
	}
	if v.BgL != "#f5f0e8" || v.Name != "Класс" || v.LookStep != 2 || len(v.Grid) != 5 || v.VigHead(v.Vig.Head) == nil {
		t.Fatalf("лист виньетки: фон %q, имя %q, шаг %d, мест %d", v.BgL, v.Name, v.LookStep, len(v.Grid))
	}
	id := v.ID
	// Свой макет с тем же положением виньетки и другим числом учеников: лист тот же, мест больше.
	b := lookA()
	b.Steps[1] = lkVig("Класс", 9)
	lkApply(t, e, lkRefB, b, lkPrev(lookA()), 9)
	if v = edtest.P(e).Spreads[1]; v.ID != id || v.Vig.Pupils != 9 || len(v.Grid) != 10 {
		t.Fatalf("после смены: ID %s (был %s), учеников %d, мест %d", v.ID, id, v.Vig.Pupils, len(v.Grid))
	}
	// Сдвинутое место виньетки возвращается при повторе (тронутая заменяется), лист тот же.
	x0 := edtest.P(e).Spreads[1].Grid[0].X
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Grid[0].X += 6 })
	if plan, _ := e.PlanLook(lkRefB, b, lkPrev(b), model.LookOpts{Spreads: 9}, nil); plan.Touched != 1 {
		t.Fatalf("тронутых %d, ждали 1", plan.Touched)
	}
	lkApply(t, e, lkRefB, b, lkPrev(b), 9)
	if v = edtest.P(e).Spreads[1]; v.ID != id || v.Grid[0].X != x0 {
		t.Fatalf("место виньетки не вернулось: %+v", v.Grid[0])
	}
	if plan := lkApply(t, e, lkRefB, b, lkPrev(b), 9); plan.Touched != 0 {
		t.Fatalf("после возврата тронутых %d", plan.Touched)
	}
}

// Повтор того же макета — холостая правка: rev и шаг отмены те же, ID разворотов и мест прежние.
func TestApplyLookTwiceNoop(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	before, rev, undo, data := lkRows(e), e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	plan := lkApply(t, e, lkRefA, lookA(), nil, 9)
	if e.View().Rev != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("повтор не холостой: rev %d → %d, шагов %d → %d", rev, e.View().Rev, undo, edtest.UndoLen(e))
	}
	if !slices.EqualFunc(before, lkRows(e), func(a, b lkRow) bool { return a.ID == b.ID && slices.Equal(a.Places, b.Places) }) || lkJSON(e) != data {
		t.Fatal("ID разворотов или мест сменились")
	}
	if !plan.Trial || plan.Touched != 0 || plan.Prev != "Классика" {
		t.Errorf("повтор: пробный %v, тронутых %d, прежний %q", plan.Trial, plan.Touched, plan.Prev)
	}
}

// Один шаг отмены: Undo возвращает пустой альбом, Redo — макет.
func TestApplyLookUndoOneStep(t *testing.T) {
	e := lkEditor(t)
	if edtest.UndoLen(e) != 0 {
		t.Fatal("в чистом редакторе есть шаги отмены")
	}
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	if edtest.UndoLen(e) != 1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e))
	}
	done := lkJSON(e)
	e.Undo()
	if p := edtest.P(e); len(p.Spreads) != 0 || p.Look != nil {
		t.Fatalf("после Undo: %d разворотов, макет %v", len(p.Spreads), p.Look)
	}
	e.Redo()
	if lkJSON(e) != done {
		t.Fatal("Redo не вернул макет")
	}
	// Смена макета — тоже один шаг, и Undo возвращает прежний.
	lkApply(t, e, lkRefB, lookB(), lkPrev(lookA()), 6)
	if edtest.UndoLen(e) != 2 {
		t.Fatalf("после смены шагов отмены %d, ждали 2", edtest.UndoLen(e))
	}
	e.Undo()
	if lkJSON(e) != done {
		t.Fatal("Undo смены макета не вернул прежний")
	}
}

// Отказ: пределы и негодный вход откатывают без правки, без шага и без rev.
func TestApplyLookRefusedNoStep(t *testing.T) {
	e := lkEditor(t)
	edtest.Change(e, func(p *model.Project) { // 195 своих разворотов с фоном: серия в 9 не влезает
		for range 195 {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#abcdef"})
		}
	})
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	check := func(name string, ref model.LookRef, tpl model.AlbumTpl, n int) {
		t.Helper()
		v, plan, err := e.ApplyLook(ref, tpl, nil, model.LookOpts{Spreads: n}, nil)
		if err == nil {
			t.Fatalf("%s: ждали отказ", name)
		}
		if plan != (model.LookPlan{}) || v.Rev != rev || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
			t.Fatalf("%s: отказ оставил след (rev %d, шагов %d)", name, e.View().Rev, edtest.UndoLen(e))
		}
	}
	check("предел MaxSpreads", lkRefA, lookA(), 9)
	check("без ключа", model.LookRef{Name: "x"}, lookA(), 9)
	check("плохой ключ", model.LookRef{Key: "my:XYZ"}, lookA(), 9)
	check("пустой макет", lkRefA, model.AlbumTpl{V: model.LookV}, 9)
	check("макет новее", lkRefA, model.AlbumTpl{V: model.LookV + 1, Steps: lookA().Steps}, 9)
	if _, err := e.PlanLook(lkRefA, lookA(), nil, model.LookOpts{Spreads: 9}, nil); err == nil {
		t.Fatal("PlanLook должен отказать там же, где ApplyLook")
	}
	// То же число влезает, когда своих разворотов меньше.
	e2 := lkEditor(t)
	edtest.Change(e2, func(p *model.Project) {
		for range 191 {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#abcdef"})
		}
	})
	if plan := lkApply(t, e2, lkRefA, lookA(), nil, 9); plan.Spreads != 9 || len(edtest.P(e2).Spreads) != model.MaxSpreads {
		t.Fatalf("на пределе: %d разворотов серии, всего %d", plan.Spreads, len(edtest.P(e2).Spreads))
	}
}

// Пустые развороты уходят, свои с делом остаются на местах; макет ставится рядом.
func TestApplyLookDropsEmptyKeepsOwn(t *testing.T) {
	e := lkEditor(t)
	edtest.Change(e, func(p *model.Project) {
		p.Spreads = append(p.Spreads,
			&model.Spread{ID: "empty000001", Photos: []string{}},
			&model.Spread{ID: "own000000001", Photos: []string{}, BgL: "#123456"})
	})
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	rows := lkRows(e)
	if len(rows) != 10 {
		t.Fatalf("разворотов %d, ждали 10 (9 + свой)", len(rows))
	}
	var own *lkRow
	for i := range rows {
		if rows[i].ID == "empty000001" {
			t.Fatal("пустой разворот остался")
		}
		if rows[i].ID == "own000000001" {
			own = &rows[i]
		}
	}
	if own == nil || own.Step != 0 {
		t.Fatalf("свой разворот потерян или получил шаг: %+v", own)
	}
	if plan := lkApply(t, e, lkRefA, lookA(), nil, 9); !plan.Trial || plan.Touched != 0 {
		t.Fatalf("повтор рядом со своим: %+v", plan)
	}
}

// Save→Load: макет, номера шагов, имена, места и заготовки доходят до файла v19 и обратно;
// повтор на загруженном — холостой.
func TestApplyLookSaveLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "look.salbum")
	e := edtest.Open(t, model.New(), path)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || p.Look == nil || p.Look.Key != lkKeyA || p.Look.Name != "Классика" {
		t.Fatalf("версия %d, макет %+v", p.Version, p.Look)
	}
	want := lkRows(e)
	e2 := edtest.Open(t, p, path)
	got := lkRows(e2)
	if len(got) != len(want) {
		t.Fatalf("разворотов %d, ждали %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Step != want[i].Step || got[i].Name != want[i].Name || len(got[i].Places) != len(want[i].Places) {
			t.Fatalf("разворот %d: %+v, ждали %+v", i, got[i], want[i])
		}
	}
	if len(p.Spreads[0].Texts) != 1 || !p.Spreads[0].Texts[0].Draft {
		t.Fatalf("заготовка не дошла: %+v", p.Spreads[0].Texts)
	}
	rev := e2.View().Rev
	if plan := lkApply(t, e2, lkRefA, lookA(), nil, 9); e2.View().Rev != rev || plan.Touched != 0 {
		t.Fatalf("повтор после Load: rev %d → %d, тронутых %d", rev, e2.View().Rev, plan.Touched)
	}
	data, _ := json.Marshal(p.Look)
	if strings.Contains(string(data), "path") {
		t.Fatalf("в записи макета лишнее: %s", data)
	}
}

// Стоимость: серия в 120 разворотов — новая и повтор (со сверкой пробного).
func TestApplyLookSeriesFast(t *testing.T) {
	testkit.CostTest(t)
	steps := []model.LookStep{lkStep("", "Начало", "#ffffff", 5), lkStep(model.RoleRepeat, "", "#eeeeee", 5), lkStep("", "Конец", "#dddddd", 5)}
	tpl := model.AlbumTpl{V: model.LookV, Steps: steps}
	fresh := testkit.Fastest(3, func() {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, tpl, nil, 120)
	})
	e := lkEditor(t)
	lkApply(t, e, lkRefA, tpl, nil, 120)
	again := testkit.Fastest(3, func() { lkApply(t, e, lkRefA, tpl, nil, 120) })
	if limit := testkit.Limit(60 * time.Millisecond); fresh > limit || again > limit {
		t.Fatalf("120 разворотов: новая %v, повтор %v, предел %v", fresh, again, limit)
	}
	if n := len(edtest.P(e).Spreads); n != 120 {
		t.Fatalf("разворотов %d", n)
	}
}

// Формат не квадратный: места шага встают композицией целиком (LookBoxes, 0.27-131-20), а не
// долями области; повтор на том же формате — холостой.
func TestApplyLookKeepsComposition(t *testing.T) {
	for _, f := range []struct{ w, h float64 }{{200, 300}, {400, 300}} { // 20×30 и 40×30
		e := lkEditor(t)
		st := edtest.P(e).Settings
		st.PageW, st.PageH = f.w, f.h
		e.SetSettings(st)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		p := edtest.P(e)
		s := p.Spreads[2] // «Сборы»: два места, нарисованы на области 2:1
		area := 2 * f.w / f.h
		want := model.LookBoxes(lookA().Steps[2].Tpl, area)
		if len(s.Grid) != 2 {
			t.Fatalf("мест %d", len(s.Grid))
		}
		for i, pl := range s.Grid {
			w, h := 2*f.w, f.h
			if d := max(abs(pl.X-want[i].X*w), abs(pl.Y-want[i].Y*h), abs(pl.W-want[i].W*w), abs(pl.H-want[i].H*h)); d > 0.05 {
				t.Errorf("%v: место %d отклонилось от LookBoxes на %.3f мм: %+v", f, i, d, pl)
			}
		}
		// Места одной строки остаются на общей высоте и с равным промежутком (композиция не искажена).
		if s.Grid[0].Y != s.Grid[1].Y || s.Grid[0].H != s.Grid[1].H {
			t.Errorf("%v: выравнивание мест потеряно", f)
		}
		rev := e.View().Rev
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		if e.View().Rev != rev {
			t.Errorf("%v: повтор не холостой", f)
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
