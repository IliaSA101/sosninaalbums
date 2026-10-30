package view_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Узоры и своё из библиотеки в View (048 G5): PatternView, BackdropView.Lib, View.LibPhotos.

func applyPat(t *testing.T, e *album.Editor, sp, scope string, pt model.Pattern) {
	t.Helper()
	if _, err := e.ApplyPattern(sp, scope, pt, ""); err != nil {
		t.Fatal(err)
	}
}

// Пусто — [] (UI не проверяет null): у листа без узоров и у альбома без своих.
func TestPatternViewsEmpty(t *testing.T) {
	e, _, _ := edtest.BdEd(t)
	v := e.View()
	for _, sv := range v.Spreads {
		if sv.Patterns == nil || len(sv.Patterns) != 0 {
			t.Fatalf("узоры пустого листа: %#v", sv.Patterns)
		}
	}
	if v.LibPhotos == nil || len(v.LibPhotos) != 0 {
		t.Fatalf("свои: %#v", v.LibPhotos)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"patterns":[]`, `"libPhotos":[]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("в JSON нет %s", want)
		}
	}
}

// Узор листа и узоры страниц: область — та же, что у фото фона; цвета дополнены цветами
// генератора, масштаб 0 → 1, имя, пикеры и URL — от генератора; неизвестный ключ — Known
// false, без имени и URL. Порядок — лист, левая, правая. Круг Save → Load вид не меняет;
// снятие узора — шаг, отмена его возвращает, снятие пустого — холостая.
func TestPatternViews(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	bands, _ := gen.Lookup("bands")
	applyPat(t, e, s1, "spread", model.Pattern{Kind: "bands", Inks: []string{"#102030"}, Fade: 0.25})
	edtest.SetBg(t, e, s1, "spread", "bg") // фото фона поверх узора: узор остаётся
	applyPat(t, e, s2, "right", model.Pattern{Kind: "future-kind", Inks: []string{"#010203", "#040506"}, Scale: 2})
	applyPat(t, e, s2, "left", model.Pattern{Kind: "confetti", Seed: 77, Scale: 0.5})
	v := e.View()

	sv := v.Spreads[0]
	if len(sv.Patterns) != 1 || len(sv.Backdrops) != 1 {
		t.Fatalf("лист 1: узоры %+v, фото фона %+v", sv.Patterns, sv.Backdrops)
	}
	pv := sv.Patterns[0]
	if pv.Scope != "spread" || pv.Area != sv.Backdrops[0].Area || pv.Area != (layout.Rect{W: sv.W, H: sv.H}) {
		t.Fatalf("область узора %+v, фото фона %+v, лист %v×%v", pv.Area, sv.Backdrops[0].Area, sv.W, sv.H)
	}
	if !slices.Equal(pv.Pat.Inks, []string{"#102030", bands.Inks[1]}) || pv.Pat.Scale != 1 || pv.Pat.Fade != 0.25 || pv.Pat.Kind != "bands" {
		t.Fatalf("узор листа: %+v", pv.Pat)
	}
	if !pv.Known || pv.Name != bands.Name || pv.Inks != 2 || pv.Random {
		t.Fatalf("карточка: %+v", pv)
	}
	if !strings.HasPrefix(pv.URL, "/media/gen/pat?k=bands&") || strings.Contains(pv.URL, "px=") ||
		!strings.Contains(pv.URL, "&w=600&h=300&") {
		t.Fatalf("URL %q", pv.URL)
	}
	path, query, _ := strings.Cut(pv.URL+"&px=64", "?")
	if req, ok := gen.Parse(path, query); !ok || req.W != 600 || req.H != 300 {
		t.Fatalf("URL не разбирается /media/gen: %q (%+v)", pv.URL, req)
	}

	sv = v.Spreads[1]
	if len(sv.Patterns) != 2 || sv.Patterns[0].Scope != "left" || sv.Patterns[1].Scope != "right" {
		t.Fatalf("лист 2: %+v", sv.Patterns)
	}
	l, r := sv.Patterns[0], sv.Patterns[1]
	if l.Area != (layout.Rect{W: sv.Mid, H: sv.H}) || r.Area != (layout.Rect{X: sv.Mid, W: sv.W - sv.Mid, H: sv.H}) {
		t.Fatalf("области страниц: %+v %+v (шов %v)", l.Area, r.Area, sv.Mid)
	}
	if !l.Known || !l.Random || l.Pat.Seed != 77 || l.Pat.Scale != 0.5 || l.Inks != 4 || len(l.Pat.Inks) != 4 {
		t.Fatalf("конфетти: %+v", l)
	}
	if r.Known || r.Name != "" || r.URL != "" || r.Inks != 0 || r.Pat.Kind != "future-kind" || r.Pat.Scale != 2 ||
		!slices.Equal(r.Pat.Inks, []string{"#010203", "#040506"}) {
		t.Fatalf("неизвестный генератор: %+v", r)
	}

	q := edtest.SaveLoad(t, e)
	v2 := edtest.Open(t, q, "").View()
	for i := range v.Spreads {
		if !reflect.DeepEqual(v.Spreads[i].Patterns, v2.Spreads[i].Patterns) {
			t.Fatalf("лист %d после Save → Load:\n%+v\n%+v", i, v.Spreads[i].Patterns, v2.Spreads[i].Patterns)
		}
	}

	undo := edtest.UndoLen(e)
	if got := e.ClearPattern(s2, "left").Spreads[1].Patterns; len(got) != 1 || got[0].Scope != "right" {
		t.Fatalf("после снятия левого: %+v", got)
	}
	rev := edtest.Rev(e)
	e.ClearPattern(s2, "left")
	edtest.Same(t, e, rev, undo+1, "снятие пустого")
	if got := e.Undo().Spreads[1].Patterns; !reflect.DeepEqual(got, v.Spreads[1].Patterns) {
		t.Fatalf("после отмены: %+v", got)
	}
}

// Узор обложки — в листе обложки, области сторонок как у фото фона.
func TestPatternViewCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	applyPat(t, e, model.CoverID, "right", model.Pattern{Kind: "dots"})
	edtest.SetBg(t, e, model.CoverID, "right", "bg")
	c := e.View().Cover
	if c == nil || len(c.Sheet.Patterns) != 1 || len(c.Sheet.Backdrops) != 1 {
		t.Fatalf("обложка: %+v", c)
	}
	if pv := c.Sheet.Patterns[0]; pv.Area != c.Sheet.Backdrops[0].Area || pv.Area.X <= 0 || !pv.Known {
		t.Fatalf("узор сторонки %+v, фото фона %+v", pv, c.Sheet.Backdrops[0].Area)
	}
}

// Свои из библиотеки: ключ в BackdropView.Lib, ID — в View.LibPhotos по возрастанию; у
// обычного фото фона lib в JSON нет. Пропавшее своё (путь пуст после Load) в списке остаётся.
func TestLibPhotosView(t *testing.T) {
	keys := []string{"my:0123456789abcdef", "my:fedcba9876543210"}
	dir := testkit.TempDir(t)
	files := map[string]string{}
	for _, k := range keys {
		files[k] = filepath.Join(dir, k[3:]+".jpg")
		edtest.PutFile(t, files[k], 10)
	}
	edtest.LibFinder(t, files)
	e, s1, s2 := edtest.BdEd(t)
	for i, at := range [][2]string{{s1, "left"}, {s2, "spread"}} {
		ph := model.PhotoInfo{ID: model.LibPhotoID(keys[i]), Path: files[keys[i]], Lib: keys[i], W: 400, H: 300, Size: 10}
		if _, err := e.ApplyBackdrop(at[0], at[1], ph); err != nil {
			t.Fatal(err)
		}
	}
	v := edtest.SetBg(t, e, s1, "right", "bg")
	want := []string{model.LibPhotoID(keys[0]), model.LibPhotoID(keys[1])}
	slices.Sort(want)
	if !slices.Equal(v.LibPhotos, want) {
		t.Fatalf("LibPhotos %v, ждали %v", v.LibPhotos, want)
	}
	bds := v.Spreads[0].Backdrops
	if len(bds) != 2 || bds[0].Lib != keys[0] || bds[1].Lib != "" || v.Spreads[1].Backdrops[0].Lib != keys[1] {
		t.Fatalf("Lib у фото фона: %+v / %+v", bds, v.Spreads[1].Backdrops)
	}
	data, err := json.Marshal(bds[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"lib"`) {
		t.Fatalf("у обычного фото фона есть lib: %s", data)
	}

	delete(files, keys[1]) // на «другом компьютере» второго нет
	v2 := edtest.Open(t, edtest.SaveLoad(t, e), "").View()
	if !slices.Equal(v2.LibPhotos, want) || v2.Spreads[1].Backdrops[0].Lib != keys[1] {
		t.Fatalf("после Save → Load: %v %+v", v2.LibPhotos, v2.Spreads[1].Backdrops)
	}
}

// Отметка плитки (решение №201): Preset из ApplyPattern — в PatternView.Preset (в Pat его нет);
// прозрачность её не снимает, цвета, масштаб и seed — снимают (шаг, отмена возвращает); та же
// плитка и та же правка — холостые; чужое значение отбрасывается, узор остаётся; круг
// Save → Load её хранит, узор без отметки читается с пустой. Несколько областей — у каждой своя.
func TestPatternPreset(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	applyPat(t, e, s1, "left", model.Pattern{Kind: "dots", Inks: []string{"#d4b26a"}, Preset: "sa:gold-dots"})
	applyPat(t, e, s1, "right", model.Pattern{Kind: "stripes", Preset: "sa:thin-stripes"})
	applyPat(t, e, s2, "spread", model.Pattern{Kind: "bands", Preset: "../x"})
	presets := func(v model.View) []string {
		var out []string
		for _, sv := range v.Spreads {
			for _, pv := range sv.Patterns {
				if pv.Pat.Preset != "" {
					t.Fatalf("Preset в Pat: %+v", pv.Pat)
				}
				out = append(out, pv.Preset)
			}
		}
		return out
	}
	v := e.View()
	if got := presets(v); !slices.Equal(got, []string{"sa:gold-dots", "sa:thin-stripes", ""}) {
		t.Fatalf("отметки %q", got)
	}
	if pt := edtest.P(e).Spreads[1].Pat; pt == nil || pt.Kind != "bands" || pt.Preset != "" {
		t.Fatalf("узор с чужой отметкой: %+v", pt)
	}

	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	applyPat(t, e, s1, "left", model.Pattern{Kind: "dots", Inks: []string{"#d4b26a"}, Preset: "sa:gold-dots"})
	edtest.Same(t, e, rev, undo, "та же плитка")
	pat := v.Spreads[0].Patterns[0].Pat
	pat.Fade = 0.4
	if got := presets(e.SetPattern(s1, "left", pat, "")); got[0] != "sa:gold-dots" {
		t.Fatalf("прозрачность сняла отметку: %q", got)
	}
	rev = edtest.Rev(e)
	e.SetPattern(s1, "left", pat, "")
	edtest.Same(t, e, rev, undo+1, "та же прозрачность")
	pat.Inks = []string{"#102030"}
	if got := presets(e.SetPattern(s1, "left", pat, "")); got[0] != "" || got[1] != "sa:thin-stripes" {
		t.Fatalf("цвет не снял отметку (или снял у соседа): %q", got)
	}
	pr := v.Spreads[0].Patterns[1].Pat
	pr.Scale = 2
	if got := presets(e.SetPattern(s1, "right", pr, "")); got[1] != "" {
		t.Fatalf("масштаб не снял отметку: %q", got)
	}
	if got := presets(e.Undo()); got[0] != "" || got[1] != "sa:thin-stripes" {
		t.Fatalf("после отмены масштаба: %q", got)
	}
	if got := presets(e.Undo()); got[0] != "sa:gold-dots" {
		t.Fatalf("после отмены цвета: %q", got)
	}

	q := edtest.SaveLoad(t, e)
	if pt := q.Spreads[0].PatL; pt == nil || pt.Preset != "sa:gold-dots" || q.Spreads[1].Pat.Preset != "" {
		t.Fatalf("после Save → Load: %+v %+v", pt, q.Spreads[1].Pat)
	}
	if got := presets(edtest.Open(t, q, "").View()); !slices.Equal(got, presets(e.View())) {
		t.Fatalf("вид после Save → Load: %q", got)
	}
}
