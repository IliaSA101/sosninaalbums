package spreads_test

// Спека 035 U1: автосборка в конец занимает пустые развороты хвоста (Spread.Empty) вместо
// новых — не больше N, лишние пустые остаются после собранных; один шаг отмены.
// AutoPlan.Reuse и верх ползунка считают хвост свободным местом.

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// blanksEd — редактор с разворотами по шаблону: "∅" — пустой («+ Новый разворот»),
// иначе — разворот с этим фото. Возвращает ID разворотов по порядку.
func blanksEd(t *testing.T, form ...string) (*album.Editor, []string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	for _, f := range form {
		var ph []model.PhotoInfo
		if f != "∅" {
			ph = edtest.Photos(f)
		}
		edtest.NewSpread(e, -1, ph)
	}
	ids := edtest.SpreadIDs(e.View())
	if len(ids) != len(form) {
		t.Fatalf("развороты %v, ждали %v", ids, form)
	}
	return e, ids
}

// formOf — вид альбома для сравнения: старый ID — его индекс в was ("0", "1"…), пустой
// новый — "∅", новый с фото — "+".
func formOf(v model.View, was []string) string {
	out := make([]string, len(v.Spreads))
	for i, s := range v.Spreads {
		switch k := slices.Index(was, s.ID); {
		case k >= 0:
			out[i] = string(rune('0' + k))
		case len(s.Photos) == 0:
			out[i] = "∅"
		default:
			out[i] = "+"
		}
	}
	return strings.Join(out, " ")
}

// TestAutoBuildFillsBlankTail: сборка в конец занимает пустые хвоста (сначала ближние),
// лишние пустые остаются после новых; ID новых — свежие; Ctrl+Z одним шагом возвращает
// пустые с прежними ID; результат переживает Save→Load.
func TestAutoBuildFillsBlankTail(t *testing.T) {
	for _, c := range []struct {
		form    []string
		photos  int
		n, at   int
		want    string
		builtAt int // индекс первого нового
	}{
		{[]string{"∅"}, 10, 10, -1, "+ + + + + + + + + +", 0},
		{[]string{"∅"}, 6, 3, -1, "+ + +", 0},
		{[]string{"A", "∅", "∅"}, 4, 1, -1, "0 + 2", 1},
		{[]string{"A", "∅", "∅"}, 4, 1, 3, "0 + 2", 1},  // at = длина — как UI
		{[]string{"A", "∅", "∅"}, 4, 1, 2, "0 + 2", 1},  // at внутри хвоста — тоже в конец
		{[]string{"A", "∅", "∅"}, 4, 1, 99, "0 + 2", 1}, // за концом
		{[]string{"∅", "∅", "∅"}, 4, 2, -1, "+ + 2", 0}, // все пустые
		{[]string{"A", "∅", "∅"}, 6, 4, -1, "0 + + + +", 1},
		{[]string{"A", "B"}, 4, 2, -1, "0 1 + +", 2}, // без пустых в конце — как раньше
	} {
		e, was := blanksEd(t, c.form...)
		before, undo := edtest.P(e).Clone(), edtest.UndoLen(e)
		ap, err := e.AutoPlan(mixedPhotos("p", c.photos), model.AutoOpts{Spreads: c.n, MaxPer: 64})
		if err != nil {
			t.Fatal(err)
		}
		v, err := e.AutoBuild(mixedPhotos("p", c.photos), c.at, model.AutoOpts{Spreads: c.n, MaxPer: 64})
		if err != nil {
			t.Fatal(err)
		}
		if got := formOf(v, was); got != c.want {
			t.Fatalf("%v + N=%d at=%d: %q, ждали %q", c.form, c.n, c.at, got, c.want)
		}
		if made := strings.Count(c.want, "+"); ap.Spreads != made || ap.Reuse != min(made, strings.Count(strings.Join(c.form, ""), "∅")) {
			t.Fatalf("%v + N=%d: план %+v", c.form, c.n, ap)
		}
		if len(v.Spreads[c.builtAt].Photos) == 0 || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%v: первый новый %d пуст или шагов отмены %d → %d", c.form, c.builtAt, undo, edtest.UndoLen(e))
		}
		// Save→Load: оставшиеся пустые и новые на своих местах.
		file := filepath.Join(testkit.TempDir(t), "a.salbum")
		if err := e.Save(file); err != nil {
			t.Fatal(err)
		}
		q, err := album.LoadBase(file, filepath.Dir(file))
		if err != nil {
			t.Fatal(err)
		}
		if got := edtest.SpreadIDs(edtest.Open(t, q, "").View()); !slices.Equal(got, edtest.SpreadIDs(v)) {
			t.Fatalf("%v: после Load %v, было %v", c.form, got, edtest.SpreadIDs(v))
		}
		v = e.Undo()
		if !slices.Equal(edtest.SpreadIDs(v), was) || !slices.EqualFunc(edtest.P(e).Spreads, before.Spreads, func(a, b *model.Spread) bool { return a.ID == b.ID && len(a.Photos) == len(b.Photos) }) {
			t.Fatalf("%v: после отмены %v, ждали %v", c.form, edtest.SpreadIDs(v), was)
		}
	}
}

// Холостая сборка (нет фото; все фото уже в сетке) пустой хвост не трогает: ни шага
// отмены, ни rev.
func TestAutoBuildBlankTailNoop(t *testing.T) {
	e, was := blanksEd(t, "∅", "∅")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, ph := range [][]model.PhotoInfo{nil, {}} {
		v, err := e.AutoBuild(ph, -1, model.AutoOpts{Spreads: 3, MaxPer: 64})
		if err != nil || !slices.Equal(edtest.SpreadIDs(v), was) {
			t.Fatalf("без фото: %v %v", edtest.SpreadIDs(v), err)
		}
		edtest.Same(t, e, rev, undo, "автосборка без фото")
	}
	// Все выбранные фото уже в сетке — отсеяны, хвост цел, шага нет.
	g := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(130, 12, 100, 100, "b")})
	edtest.NewSpread(g, -1, nil)
	edtest.NewSpread(g, -1, nil)
	gIDs, rev, undo := edtest.SpreadIDs(g.View()), edtest.Rev(g), edtest.UndoLen(g)
	ph := edtest.Photos("a", "b")
	if ap, err := g.AutoPlan(ph, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil || ap != (model.AutoPlan{}) {
		t.Fatalf("план, все в сетке: %+v %v", ap, err)
	}
	if v, err := g.AutoBuild(ph, -1, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil || !slices.Equal(edtest.SpreadIDs(v), gIDs) {
		t.Fatalf("все в сетке: %v %v", edtest.SpreadIDs(v), err)
	}
	edtest.Same(t, g, rev, undo, "автосборка, все фото в сетке")
	if ap, err := e.AutoPlan(nil, model.AutoOpts{Spreads: 3, MaxPer: 64}); err != nil || ap != (model.AutoPlan{}) {
		t.Fatalf("план без фото: %+v %v", ap, err)
	}
}

// TestAutoBuildMovesPlacedPhotoIntoTail: выбранное фото уже стоит в альбоме, за ним пустой
// хвост ([A(a), ∅]) — оно переезжает на место хвоста, не дублируется и не теряется;
// отмена возвращает его назад.
func TestAutoBuildMovesPlacedPhotoIntoTail(t *testing.T) {
	e, was := blanksEd(t, "a", "∅")
	v, err := e.AutoBuild(edtest.Photos("a", "b"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, s := range v.Spreads {
		for _, id := range s.Photos {
			seen[id]++
		}
	}
	if len(seen) != 2 || seen["a"] != 1 || seen["b"] != 1 {
		t.Fatalf("фото после сборки: %v, развороты %q", seen, formOf(v, was))
	}
	if v = e.Undo(); !slices.Equal(edtest.SpreadIDs(v), was) || !slices.Equal(v.Spreads[0].Photos, []string{"a"}) {
		t.Fatalf("после отмены %v, ждали %v с фото a на первом", edtest.SpreadIDs(v), was)
	}
}

// TestAutoBuildKeepsBlankMiddle: пустые в середине (не в хвосте) не занимаются.
func TestAutoBuildKeepsBlankMiddle(t *testing.T) {
	e, was := blanksEd(t, "∅", "A", "∅", "B")
	v, err := e.AutoBuild(mixedPhotos("p", 4), -1, model.AutoOpts{Spreads: 2, MaxPer: 64})
	if err != nil {
		t.Fatal(err)
	}
	if got := formOf(v, was); got != "0 1 2 3 + +" {
		t.Fatalf("развороты %q", got)
	}
	// Вставка в середину (at до хвоста) хвост не занимает.
	e, was = blanksEd(t, "A", "∅")
	if v, err = e.AutoBuild(mixedPhotos("p", 4), 0, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil || formOf(v, was) != "+ + 0 1" {
		t.Fatalf("вставка в начало: %q %v", formOf(v, was), err)
	}
}

// TestAutoBuildKeepsNamedEmpty: разворот без фото, но с названием, цветом, пустой сеткой,
// стилем подписей или виньетка — не пустой (Spread.Empty), сборка его не занимает.
func TestAutoBuildKeepsNamedEmpty(t *testing.T) {
	marks := map[string]func(t *testing.T) (*album.Editor, string){
		"название": func(t *testing.T) (*album.Editor, string) {
			e, ids := blanksEd(t, "∅")
			e.SetSpreadName(ids[0], "Банкет")
			return e, ids[0]
		},
		"цвет": func(t *testing.T) (*album.Editor, string) {
			e, ids := blanksEd(t, "∅")
			edtest.Change(e, func(p *model.Project) { p.Spreads[0].BgL = "#203040" })
			return e, ids[0]
		},
		"пустая сетка": func(t *testing.T) (*album.Editor, string) {
			return edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "")}), "s1"
		},
		"стиль подписей": func(t *testing.T) (*album.Editor, string) {
			e, ids := blanksEd(t, "∅")
			edtest.Change(e, func(p *model.Project) { p.Spreads[0].Caps = &model.SheetCaps{} })
			return e, ids[0]
		},
		"виньетка": func(t *testing.T) (*album.Editor, string) {
			e := edtest.Open(t, model.New(), "")
			v, err := e.AddVignette(-1)
			if err != nil {
				t.Fatal(err)
			}
			return e, v.Spreads[0].ID
		},
	}
	for what, mk := range marks {
		e, id := mk(t)
		if edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].Empty() {
			t.Fatalf("%s: заготовка пустая — тест ничего не проверит", what)
		}
		ap, err := e.AutoPlan(mixedPhotos("p", 4), model.AutoOpts{Spreads: 2, MaxPer: 64})
		if err != nil || ap.Reuse != 0 {
			t.Fatalf("%s: план %+v %v", what, ap, err)
		}
		v, err := e.AutoBuild(mixedPhotos("p", 4), -1, model.AutoOpts{Spreads: 2, MaxPer: 64})
		if err != nil || formOf(v, []string{id}) != "0 + +" {
			t.Fatalf("%s: развороты %q %v", what, formOf(v, []string{id}), err)
		}
	}
	// Не пустой перед пустым хвостом: занят только хвост.
	e, ids := blanksEd(t, "∅", "∅")
	e.SetSpreadName(ids[0], "Банкет")
	if v, err := e.AutoBuild(mixedPhotos("p", 4), -1, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil || formOf(v, ids) != "0 + +" {
		t.Fatalf("название перед хвостом: %q %v", formOf(v, ids), err)
	}
}

// TestAutoPlanReuse: Reuse = min(хвост, N); без хвоста и без фото — 0.
func TestAutoPlanReuse(t *testing.T) {
	e, _ := blanksEd(t, "A", "∅", "∅")
	for _, c := range []struct{ n, want int }{{1, 1}, {2, 2}, {5, 2}, {0, 1}} {
		if ap, err := e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: c.n, MaxPer: 64}); err != nil || ap.Reuse != c.want {
			t.Fatalf("N=%d: %+v %v, ждали Reuse %d", c.n, ap, err, c.want)
		}
	}
	e, _ = blanksEd(t, "A")
	if ap, err := e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: 5, MaxPer: 64}); err != nil || ap.Reuse != 0 {
		t.Fatalf("без хвоста: %+v %v", ap, err)
	}
	e = edtest.Open(t, model.New(), "")
	if ap, err := e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: 5, MaxPer: 64}); err != nil || ap.Reuse != 0 || ap.Spreads != 5 {
		t.Fatalf("пустой альбом: %+v %v", ap, err)
	}
}

// fullWithTail — альбом на MaxSpreads − free разворотов: не пустые (с цветом) и blank
// пустых в хвосте.
func fullWithTail(t *testing.T, free, blank int) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for i := range model.MaxSpreads - free {
			s := &model.Spread{ID: model.NewID(), Photos: []string{}}
			if i < model.MaxSpreads-free-blank {
				s.BgL = "#ffffff"
			}
			p.Spreads = append(p.Spreads, s)
		}
	})
	return e
}

// TestAutoPlanRoomCountsBlankTail: верх ползунка — MaxSpreads − непустых; сборка на весь
// верх укладывается в предел, заняв хвост.
func TestAutoPlanRoomCountsBlankTail(t *testing.T) {
	e := fullWithTail(t, 1, 5)
	ap, err := e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: 99, MaxPer: 64})
	if err != nil || ap.MaxSpreads != 6 || ap.Spreads != 6 || ap.Reuse != 5 {
		t.Fatalf("план: %+v %v", ap, err)
	}
	v, err := e.AutoBuild(mixedPhotos("p", 10), -1, model.AutoOpts{Spreads: 99, MaxPer: 64})
	if err != nil || len(v.Spreads) != model.MaxSpreads || len(v.Spreads[model.MaxSpreads-6].Photos) == 0 {
		t.Fatalf("сборка: %d разворотов, %v", len(v.Spreads), err)
	}
	// Полный альбом, весь хвост пуст: место есть.
	e = fullWithTail(t, 0, 2)
	if ap, err = e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: 99, MaxPer: 64}); err != nil || ap.MaxSpreads != 2 || ap.Reuse != 2 {
		t.Fatalf("полный с хвостом: %+v %v", ap, err)
	}
	// Вставка в середину хвост не занимает: сверх предела — ошибка, проект как был.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.AutoBuild(mixedPhotos("p", 10), 0, model.AutoOpts{Spreads: 2, MaxPer: 64}); err == nil || !strings.Contains(err.Error(), "200 разворотов") {
		t.Fatalf("вставка в середину полного: %v", err)
	}
	edtest.Same(t, e, rev, undo, "отказ вставки в середину")
	// Полный без хвоста — ошибка, как раньше.
	e = fullWithTail(t, 0, 0)
	if _, err := e.AutoPlan(mixedPhotos("p", 10), model.AutoOpts{Spreads: 1, MaxPer: 64}); err == nil {
		t.Fatal("полный альбом без хвоста: ждали ошибку")
	}
}
