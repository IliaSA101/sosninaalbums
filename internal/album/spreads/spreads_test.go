package spreads_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// bothPages — постраничный ключ, где на каждой странице не меньше 2 фото.
func bothPages(k string) bool {
	if !strings.HasPrefix(k, "P[") {
		return false
	}
	parts := strings.Split(strings.Trim(k[2:], "]"), "|")
	return len(parts) == 2 && strings.Contains(parts[0], ",") && strings.Contains(parts[1], ",")
}

func TestStepLayoutCoalesce(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d"))
	v := edtest.NewSpread(e, -1, edtest.Photos("x", "y"))
	s1, s2 := v.Spreads[0].ID, v.Spreads[1].ID
	orig := v.Spreads[0].Layout
	undo := edtest.UndoLen(e)

	e.StepLayout(s1, "spread", 1)
	e.SetLayout(s1, edtest.PickKey(t, e, 0, bothPages))
	v = e.StepLayout(s1, "left", 1)
	if v.Spreads[0].Pages == nil {
		t.Fatalf("после шага страницы раскладка не постраничная: %s", v.Spreads[0].Layout)
	}
	e.RemoveFolder("нет такой") // холостая — цепочку не рвёт
	v = e.StepLayout(s1, "right", -1)
	if v.Spreads[0].Layout == orig {
		t.Fatalf("раскладка не менялась")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("снимков отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	v = e.Undo()
	if v.Spreads[0].Layout != orig {
		t.Fatalf("после отмены %s, ждали %s", v.Spreads[0].Layout, orig)
	}

	// Другая правка или другой разворот начинают новый снимок.
	e.StepLayout(s1, "spread", 1)
	edtest.AddFolder(e, testkit.TempDir(t))
	e.StepLayout(s1, "spread", 1)
	e.StepLayout(s2, "spread", 1)
	if edtest.UndoLen(e) != undo+4 {
		t.Fatalf("снимков отмены %d, ждали %d", edtest.UndoLen(e), undo+4)
	}
}

func TestStepLayoutCycle(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	id := v.Spreads[0].ID
	n := v.Spreads[0].Variants
	if v.Spreads[0].Variant != 0 || n < 2 {
		t.Fatalf("начало: %d / %d", v.Spreads[0].Variant, n)
	}
	v = e.StepLayout(id, "spread", -1)
	if v.Spreads[0].Variant != n-1 {
		t.Fatalf("назад с первой: %d, ждали %d", v.Spreads[0].Variant, n-1)
	}
	for range n - 1 {
		v = e.StepLayout(id, "spread", -1)
	}
	if v.Spreads[0].Variant != 0 || edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("круг не замкнулся: %d, Layout %q", v.Spreads[0].Variant, edtest.P(e).Spreads[0].Layout)
	}
	// Страница при непостраничной раскладке — no-op.
	if !strings.HasPrefix(v.Spreads[0].Layout, "P[") {
		rev := v.Rev
		if e.StepLayout(id, "left", 1).Rev != rev {
			t.Fatalf("шаг страницы при непостраничной раскладке что-то изменил")
		}
	}
}

func TestSetLayoutNoop(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	v := e.View()
	undo := edtest.UndoLen(e)
	for _, c := range []struct{ id, key string }{
		{s1, v.Spreads[0].Layout},
		{s1, "H(0,"},
		{s1, "H(0,1,2)"},
		{"нет", v.Spreads[0].Layout},
	} {
		if got := e.SetLayout(c.id, c.key); got.Rev != v.Rev || edtest.UndoLen(e) != undo {
			t.Fatalf("SetLayout(%q, %q) не холостой", c.id, c.key)
		}
	}
	key := edtest.PickKey(t, e, 0, edtest.AnyKey)
	e.SetLayout(s1, key)
	rev := e.View().Rev
	if e.SetLayout(s1, key).Rev != rev {
		t.Fatalf("повторный SetLayout того же ключа не холостой")
	}
}

func TestSplitSpread(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d", "e"))
	id := v.Spreads[0].ID
	e.SetLayout(id, edtest.PickKey(t, e, 0, edtest.AnyKey))
	before := model.Marshal(edtest.P(e))
	rev, undo := e.View().Rev, edtest.UndoLen(e)

	v, err := e.SplitSpread(id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 2 || v.Spreads[0].ID != id || v.Spreads[1].ID == id {
		t.Fatalf("развороты после разреза: %+v", v.Spreads)
	}
	if !slices.Equal(v.Spreads[0].Photos, []string{"a", "b"}) || !slices.Equal(v.Spreads[1].Photos, []string{"c", "d", "e"}) {
		t.Fatalf("состав: %v %v", v.Spreads[0].Photos, v.Spreads[1].Photos)
	}
	for i, s := range edtest.P(e).Spreads {
		if s.Layout != "" {
			t.Errorf("разворот %d: Layout = %q", i, s.Layout)
		}
		if len(v.Spreads[i].Rects) != len(v.Spreads[i].Photos) {
			t.Errorf("разворот %d: %d прямоугольников на %d фото", i, len(v.Spreads[i].Rects), len(v.Spreads[i].Photos))
		}
	}
	if v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d (ждали %d), undo %d (ждали %d)", v.Rev, rev+1, edtest.UndoLen(e), undo+1)
	}
	after := model.Marshal(edtest.P(e))
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("отмена вернула не тот проект")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatalf("возврат не повторил разрез")
	}
}

func TestSplitSpreadNoop(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	e.Save(filepath.Join(testkit.TempDir(t), "x.salbum")) // dirty = false
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	for _, c := range []struct {
		id string
		at int
	}{{s1, 0}, {s1, -1}, {s1, 2}, {s1, 5}, {"нет", 1}, {s2, 1}, {s2, 0}} {
		v, err := e.SplitSpread(c.id, c.at)
		if err != nil || v.Rev != rev || edtest.UndoLen(e) != undo || v.Dirty {
			t.Fatalf("SplitSpread(%q, %d) не холостой", c.id, c.at)
		}
	}
}

func TestMergeSpreads(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	v := edtest.NewSpread(e, -1, edtest.Photos("d", "e"))
	s1, s2 := v.Spreads[0].ID, v.Spreads[1].ID
	e.SetLayout(s1, edtest.PickKey(t, e, 0, edtest.AnyKey))
	e.SetLayout(s2, edtest.PickKey(t, e, 1, edtest.AnyKey))
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)

	v = e.MergeSpreads(s1)
	if len(v.Spreads) != 1 || v.Spreads[0].ID != s1 || !slices.Equal(v.Spreads[0].Photos, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("после склейки: %+v", v.Spreads)
	}
	if edtest.P(e).Spreads[0].Layout != "" || v.Spreads[0].Layout != edtest.RowsOf(e, 0)[0].Key {
		t.Fatalf("раскладка не лучшая: %q", edtest.P(e).Spreads[0].Layout)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("undo %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("отмена не вернула оба разворота с раскладками")
	}
}

// Склейка и узоры (048) — как фото фона: свои у первого, без них — второго; один шаг.
func TestMergeSpreadsPattern(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	for _, ids := range [][]string{{"a"}, {"b"}, {"c"}, {"d"}} {
		edtest.NewSpread(e, -1, edtest.Photos(ids...))
	}
	sp := e.View().Spreads
	for _, c := range []struct{ sp, scope, kind string }{
		{sp[1].ID, model.ScopeLeft, "dots"}, {sp[2].ID, model.ScopeSpread, "stripes"}, {sp[3].ID, model.ScopeRight, "bands"},
	} {
		if _, err := e.ApplyPattern(c.sp, c.scope, model.Pattern{Kind: c.kind}, ""); err != nil {
			t.Fatal(err)
		}
	}
	undo := edtest.UndoLen(e)
	e.MergeSpreads(sp[0].ID) // у первого узоров нет — берутся второго
	e.MergeSpreads(sp[2].ID) // у обоих есть — остаются первого
	ps := edtest.P(e).Spreads
	if len(ps) != 2 || ps[0].PatL == nil || ps[0].PatL.Kind != "dots" || ps[0].Pat != nil ||
		ps[1].Pat == nil || ps[1].Pat.Kind != "stripes" || ps[1].PatR != nil || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("после склеек: %+v | %+v %+v", ps[0], ps[1].Pat, ps[1].PatR)
	}
	e.Undo()
	if ps = edtest.P(e).Spreads; len(ps) != 3 || ps[2].PatR == nil || ps[2].PatR.Kind != "bands" {
		t.Fatal("отмена не вернула узор второго")
	}
}

// Фото фона и узор — один «фон»: у первого фото фона, у второго узор — остаётся только фото
// первого; у первого ничего — второй целиком (и фото, и узор).
func TestMergeSpreadsBackgroundWhole(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	for _, ids := range [][]string{{"a"}, {"b"}, {"c"}, {"d"}} {
		edtest.NewSpread(e, -1, edtest.Photos(ids...))
	}
	sp := e.View().Spreads
	edtest.SetBg(t, e, sp[0].ID, model.ScopeLeft, "bg1")
	edtest.SetBg(t, e, sp[3].ID, model.ScopeLeft, "bg2")
	for _, c := range []struct{ sp, scope string }{{sp[1].ID, model.ScopeRight}, {sp[3].ID, model.ScopeRight}} {
		if _, err := e.ApplyPattern(c.sp, c.scope, model.Pattern{Kind: "dots"}, ""); err != nil {
			t.Fatal(err)
		}
	}
	e.MergeSpreads(sp[0].ID) // фото первого против узора второго
	e.MergeSpreads(sp[2].ID) // пустой первый против фото и узора второго
	ps := edtest.P(e).Spreads
	if len(ps) != 2 || ps[0].BgPhotoL == nil || len(ps[0].Patterns()) != 0 {
		t.Fatalf("смесь фонов первого и второго: фото %+v, узоры %+v", ps[0].BgPhotoL, ps[0].Patterns())
	}
	if ps[1].BgPhotoL == nil || ps[1].PatR == nil || ps[1].PatR.Kind != "dots" {
		t.Fatalf("пустой первый не взял фон второго: фото %+v, узор %+v", ps[1].BgPhotoL, ps[1].PatR)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

func TestMergeSpreadsLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d"))
	edtest.NewSpread(e, -1, edtest.Photos("e", "f", "g"))
	v := edtest.NewSpread(e, -1, edtest.Photos("h", "i", "j", "k"))
	s1, s3 := v.Spreads[0].ID, v.Spreads[2].ID
	if v.MaxSpreadPhotos != 64 || model.MaxPhotos != 64 {
		t.Fatalf("MaxSpreadPhotos = %d", v.MaxSpreadPhotos)
	}

	v = e.MergeSpreads(s1) // 4 + 3 = 7
	if len(v.Spreads) != 2 || len(v.Spreads[0].Photos) != 7 {
		t.Fatalf("4 + 3 не склеились: %+v", v.Spreads)
	}
	edtest.AddPhotos(e, s3, manyPhotos(54), -1) // 7 + 58 = 65
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, id := range []string{s1, s3, "нет"} { // сверх 64; последний; неизвестный
		if v := e.MergeSpreads(id); v.Rev != rev || edtest.UndoLen(e) != undo {
			t.Fatalf("MergeSpreads(%q) не холостой", id)
		}
	}

	e = edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, manyPhotos(32))
	v = edtest.NewSpread(e, -1, edtest.Photos("e", "f", "g", "h"))
	edtest.AddPhotos(e, v.Spreads[1].ID, manyPhotos(60)[32:], -1) // 32 + 32 = 64 — ровно предел
	if v2 := e.MergeSpreads(v.Spreads[0].ID); len(v2.Spreads) != 1 || len(v2.Spreads[0].Photos) != 64 {
		t.Fatalf("32 + 32 не склеились: %d разворотов", len(v2.Spreads))
	}
}

// TestMerge64: склейка держится предела MaxPhotos = 64 (было 7): 7 + 9 склеиваются, 40 + 30 нет.
func TestMerge64(t *testing.T) {
	for _, c := range []struct {
		a, b int
		ok   bool
	}{{7, 9, true}, {40, 30, false}} {
		e := edtest.Open(t, model.New(), "")
		ph := manyPhotos(c.a + c.b)
		edtest.NewSpread(e, -1, ph[:c.a])
		v := edtest.NewSpread(e, -1, ph[c.a:])
		v2 := e.MergeSpreads(v.Spreads[0].ID)
		if got := len(v2.Spreads) == 1; got != c.ok || (!c.ok && v2.Rev != v.Rev) {
			t.Fatalf("%d + %d: склеились %v, ждали %v", c.a, c.b, got, c.ok)
		}
	}
}

func TestAutoBuildLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	var ids []string
	for i := range 30 {
		ids = append(ids, string(rune('A'+i)))
	}
	// Один разворот при «не больше 20» зажимается до ⌈30/20⌉ = 2.
	v, _ := e.AutoBuild(edtest.Photos(ids...), -1, model.AutoOpts{Spreads: 1, MaxPer: 20})
	if len(v.Spreads) != 2 {
		t.Fatalf("разворотов %d, ждали 2", len(v.Spreads))
	}
	for _, s := range v.Spreads {
		if len(s.Photos) > 20 {
			t.Fatalf("автосборка дала %d фото на разворот", len(s.Photos))
		}
	}
}

func TestSplitMergeRoundTrip(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d", "e"))
	id := v.Spreads[0].ID
	before := model.Marshal(edtest.P(e))
	e.SplitSpread(id, 3)
	v = e.MergeSpreads(id)
	if len(v.Spreads) != 1 || !slices.Equal(v.Spreads[0].Photos, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("разрез и склейка: %+v", v.Spreads)
	}
	e.Undo()
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatalf("два Undo не вернули исходный проект")
	}
}

func TestSplitBreaksLayoutChain(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d", "e"))
	id := v.Spreads[0].ID
	undo := edtest.UndoLen(e)
	e.SetLayout(id, edtest.PickKey(t, e, 0, edtest.AnyKey))
	e.SplitSpread(id, 3)
	e.SetLayout(id, edtest.PickKey(t, e, 0, edtest.AnyKey))
	if edtest.UndoLen(e) != undo+3 {
		t.Fatalf("снимков отмены %d, ждали %d", edtest.UndoLen(e), undo+3)
	}
}

func TestLayoutChoices(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	pv := func(id string) model.PhotoInfo { return model.PhotoInfo{ID: id, Path: "/" + id, W: 200, H: 300} }
	v := edtest.NewSpread(e, -1, []model.PhotoInfo{pv("a"), edtest.Photo("b"), pv("c"), edtest.Photo("d")})
	id := v.Spreads[0].ID
	c := e.LayoutChoices(id)
	if c.Spread != id || c.Rev != v.Rev || len(c.Whole.InOrder) == 0 || len(c.Whole.Rearranged) == 0 {
		t.Fatalf("галерея: rev %d, рядов %d + %d", c.Rev, len(c.Whole.InOrder), len(c.Whole.Rearranged))
	}
	if e.View().Rev != v.Rev {
		t.Fatalf("LayoutChoices изменил проект")
	}
	// Перестановка из галереи: порядок фото тот же, ряд листания — с перестановкой.
	r := c.Whole.Rearranged[1]
	v = e.SetLayout(id, r.Key)
	sv := v.Spreads[0]
	if !slices.Equal(sv.Photos, []string{"a", "b", "c", "d"}) || sv.Layout != r.Key || !sv.Rearranged || sv.Variant != 1 {
		t.Fatalf("после перестановки: %+v", sv)
	}
	if !slices.Equal(sv.Rects, r.Rects) {
		t.Fatalf("прямоугольники не совпали с карточкой")
	}
	v = e.StepLayout(id, "spread", 1)
	if v.Spreads[0].Layout != c.Whole.Rearranged[2%len(c.Whole.Rearranged)].Key {
		t.Fatalf("шаг по ряду перестановок: %s", v.Spreads[0].Layout)
	}

	// Постраничная раскладка: ряды страниц есть, у страниц из 2 фото.
	key := edtest.PickKey(t, e, 0, bothPages)
	v = e.SetLayout(id, key)
	if len(v.Spreads[0].Pages) != 2 || v.Spreads[0].Pages[0].Photos != 2 || v.Spreads[0].Pages[0].Variant < 0 {
		t.Fatalf("страницы: %+v", v.Spreads[0].Pages)
	}
	c = e.LayoutChoices(id)
	if c.Left == nil || c.Right == nil || len(c.Left.InOrder) == 0 {
		t.Fatalf("нет рядов страниц")
	}
	for _, ch := range c.Left.InOrder {
		if !strings.HasPrefix(ch.Key, "P[") || len(ch.Rects) != 4 {
			t.Fatalf("карточка страницы %s", ch.Key)
		}
	}
}

// Страницу из одного фото можно листать: ряд страницы есть, шаг меняет только её.
func TestStepLayoutSinglePage(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a"))
	id := v.Spreads[0].ID
	e.SetLayout(id, "P[0|-]")
	v = e.View()
	sv := v.Spreads[0]
	if sv.Layout != "P[0|-]" || sv.Pages == nil {
		t.Fatalf("раскладка %s, страницы %v", sv.Layout, sv.Pages)
	}
	if pv := sv.Pages[0]; pv.Photos != 1 || pv.Variant < 0 || pv.Variants < 2 {
		t.Fatalf("страница из одного фото: %+v", pv)
	}
	if sv.Pages[1].Variants != 0 {
		t.Fatalf("у пустой страницы ряд: %+v", sv.Pages[1])
	}
	seen := map[string]bool{sv.Layout: true}
	for range sv.Pages[0].Variants - 1 {
		v = e.StepLayout(id, "left", 1)
		k := v.Spreads[0].Layout
		if seen[k] || v.Spreads[0].Pages == nil || v.Spreads[0].Pages[0].Variant < 0 {
			t.Fatalf("шаг страницы дал %s (%+v)", k, v.Spreads[0].Pages)
		}
		seen[k] = true
	}
	if v = e.StepLayout(id, "left", 1); v.Spreads[0].Layout != "P[0|-]" {
		t.Fatalf("круг страницы не замкнулся: %s", v.Spreads[0].Layout)
	}
	c := e.LayoutChoices(id)
	if c.Left == nil || len(c.Left.InOrder) != sv.Pages[0].Variants || c.Right != nil {
		t.Fatalf("галерея страниц: left %v, right %v", c.Left, c.Right)
	}
}

// textIDs — ID текстов разворота i.
func textIDs(e *album.Editor, i int) []string {
	var out []string
	for _, t := range edtest.P(e).Spreads[i].Texts {
		out = append(out, t.ID)
	}
	return out
}

func TestDeleteSelectionOneUndo(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(142, 12, 100, 100, ""), edtest.Gp(12, 150, 100, 100, "b")})
	var ids []string
	for range 2 {
		v, err := e.AddText("s1", 0, "")
		if err != nil {
			t.Fatal(err)
		}
		ts := v.Spreads[0].Texts
		ids = append(ids, ts[len(ts)-1].ID)
	}
	empty := edtest.P(e).Spreads[0].Grid[1].ID
	before := edtest.UndoLen(e)
	e.DeleteSelection("s1", []string{"a"}, []string{empty}, ids[:1])
	if edtest.UndoLen(e) != before+1 {
		t.Fatalf("шагов %d, ждали 1", edtest.UndoLen(e)-before)
	}
	edtest.MustGrid(t, e)
	// Текст ушёл, фото ушло, его место пустое, пустое место удалено, «b» цело.
	if !slices.Equal(textIDs(e, 0), ids[1:]) || !slices.Equal(edtest.PhotosOf(e, 0), []string{"", "b"}) || edtest.HasRef(edtest.P(e), "a") {
		t.Fatalf("тексты %v, места %v", textIDs(e, 0), edtest.PhotosOf(e, 0))
	}
	e.Undo()
	if len(edtest.P(e).Spreads[0].Texts) != 2 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "", "b"}) {
		t.Fatalf("Undo: тексты %d, места %v", len(edtest.P(e).Spreads[0].Texts), edtest.PhotosOf(e, 0))
	}
}

func TestDeleteSelectionNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.DeleteSelection("s1", nil, nil, nil)
	edtest.Same(t, e, rev, undo, "пустой выбор")
	e.DeleteSelection("s1", []string{"нет"}, []string{"нет", ""}, []string{"нет"})
	e.DeleteSelection("нет", []string{"нет"}, []string{"a"}, nil)
	edtest.Same(t, e, rev, undo, "мусорные id")
}

// Тексты и фото убираются и у «Авто»; места у него холостые и разворот не закрепляют.
func TestDeleteSelectionAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	v, err := e.AddText(id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	txt := v.Spreads[0].Texts[0].ID
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.DeleteSelection(id, nil, []string{"a", "b"}, nil)
	edtest.Same(t, e, rev, undo, "места у «Авто»")
	e.DeleteSelection(id, []string{"a"}, []string{"b"}, []string{txt})
	if edtest.UndoLen(e) != undo+1 || edtest.P(e).Spreads[0].Grid != nil {
		t.Fatalf("шагов %d, сетка %v", edtest.UndoLen(e)-undo, edtest.P(e).Spreads[0].Grid)
	}
	if edtest.HasRef(edtest.P(e), "a") || !edtest.HasRef(edtest.P(e), "b") || len(edtest.P(e).Spreads[0].Texts) != 0 {
		t.Fatalf("фото %v, текстов %d", edtest.P(e).Spreads[0].Photos, len(edtest.P(e).Spreads[0].Texts))
	}
}

// fiveSpreads — пять разворотов по одному фото: a, b, c, d, e.
func fiveSpreads(t *testing.T) (*album.Editor, []string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	var v model.View
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		v = edtest.NewSpread(e, -1, edtest.Photos(id))
	}
	return e, edtest.SpreadIDs(v)
}

// Три разворота из пяти — один шаг: Undo возвращает все три на свои места, Redo снова убирает.
func TestDeleteSpreadsOneUndo(t *testing.T) {
	e, ids := fiveSpreads(t)
	before := model.Marshal(edtest.P(e))
	rev, undo := e.View().Rev, edtest.UndoLen(e)

	v := e.DeleteSpreads([]string{ids[3], ids[0], ids[1]})
	if want := []string{ids[2], ids[4]}; !slices.Equal(edtest.SpreadIDs(v), want) {
		t.Fatalf("остались %v, ждали %v", edtest.SpreadIDs(v), want)
	}
	if v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d (ждали %d), undo %d (ждали %d)", v.Rev, rev+1, edtest.UndoLen(e), undo+1)
	}
	after := model.Marshal(edtest.P(e))
	if v := e.Undo(); !slices.Equal(edtest.SpreadIDs(v), ids) {
		t.Fatalf("отмена: %v, ждали %v", edtest.SpreadIDs(v), ids)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена вернула не тот проект")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("возврат не повторил удаление")
	}
}

// Неизвестные и повторные ID пропускаются; ни одного известного — холостая (rev и undo те же).
func TestDeleteSpreadsUnknownAndDuplicates(t *testing.T) {
	e, ids := fiveSpreads(t)
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	for _, list := range [][]string{nil, {}, {"нет"}, {"нет", "", "и-нет"}} {
		if v := e.DeleteSpreads(list); v.Rev != rev || len(v.Spreads) != 5 || edtest.UndoLen(e) != undo {
			t.Fatalf("%q: rev %d→%d, разворотов %d", list, rev, v.Rev, len(v.Spreads))
		}
	}
	v := e.DeleteSpreads([]string{ids[2], "нет", ids[2], ids[2]})
	if want := []string{ids[0], ids[1], ids[3], ids[4]}; !slices.Equal(edtest.SpreadIDs(v), want) {
		t.Fatalf("остались %v, ждали %v", edtest.SpreadIDs(v), want)
	}
	if v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("повторы дали не один шаг: rev %d, undo %d", v.Rev, edtest.UndoLen(e))
	}
	// DeleteSpread — тот же путь: уже удалённый ID холостой.
	if v2 := e.DeleteSpread(ids[2]); v2.Rev != v.Rev {
		t.Fatal("повторное удаление поменяло rev")
	}
}

// Список длиннее MaxSpreads режется до построения set: ID за пределом не учитываются.
func TestDeleteSpreadsCapsList(t *testing.T) {
	e, ids := fiveSpreads(t)
	list := make([]string, model.MaxSpreads, model.MaxSpreads+1)
	for i := range list {
		list[i] = "нет"
	}
	list = append(list, ids[0])
	if v := e.DeleteSpreads(list); len(v.Spreads) != 5 {
		t.Fatalf("удалён разворот за пределом списка: %v", edtest.SpreadIDs(v))
	}
}

func TestRememberKeepsFrameAndCaption(t *testing.T) {
	e, _, s2 := edtest.NewTwo(t)
	fr := &model.Frame{S: 2, X: 0.5}
	cpt := &model.Caption{Text: "Подпись", Style: model.ClampStyle(model.TextStyle{})}
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos["a"]
		r.Frame, r.Caption = fr, cpt
		p.Photos["a"] = r
	})
	check := func(when string) {
		t.Helper()
		r, ok := e.Ref("a")
		if !ok || r.Frame == nil || *r.Frame != *fr || r.Caption == nil || r.Caption.Text != cpt.Text {
			t.Fatalf("%s: %+v", when, r)
		}
	}
	if _, err := e.MovePhoto("a", s2, -1); err != nil {
		t.Fatal(err)
	}
	check("перенос")
	e.AutoBuild(edtest.Photos("a", "b", "c"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	check("автосборка")
	edtest.AddPhotos(e, s2, []model.PhotoInfo{{ID: "a", Path: "/a.jpg", W: 300, H: 200, Size: 42}}, -1)
	check("добавление")
	if r, _ := e.Ref("a"); r.Size != 42 {
		t.Fatalf("размер файла: %d", r.Size)
	}
}

// 114: на «Авто» MovePhoto внутри разворота даёт тот же порядок и раскладку, что AddPhotos.
func TestMovePhotoAutoSameAsAdd(t *testing.T) {
	for _, idx := range []int{0, 2, 4, -1} {
		mk := func() (*album.Editor, string) {
			e := edtest.Open(t, model.New(), "")
			v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c", "d"))
			return e, v.Spreads[0].ID
		}
		e1, s1 := mk()
		e2, s2 := mk()
		if _, err := e1.AddPhotos(s1, edtest.Photos("b"), idx); err != nil {
			t.Fatal(err)
		}
		if _, err := e2.MovePhoto("b", s2, idx); err != nil {
			t.Fatal(err)
		}
		a, b := edtest.P(e1).Spreads[0], edtest.P(e2).Spreads[0]
		if !slices.Equal(a.Photos, b.Photos) || a.Layout != b.Layout || len(b.Grid) != 0 {
			t.Fatalf("index %d: AddPhotos %v/%q, MovePhoto %v/%q", idx, a.Photos, a.Layout, b.Photos, b.Layout)
		}
		if edtest.Rev(e1) != edtest.Rev(e2) || edtest.UndoLen(e1) != edtest.UndoLen(e2) {
			t.Fatalf("index %d: rev %d/%d, undo %d/%d", idx, edtest.Rev(e1), edtest.Rev(e2), edtest.UndoLen(e1), edtest.UndoLen(e2))
		}
	}
}
