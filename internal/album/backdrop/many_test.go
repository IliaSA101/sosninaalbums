package backdrop_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Фон на набор листов (077 G1): ApplyBackgroundTo, PlanBackgroundTo.

// threeEd — два разворота NewTwo и третий пустой (пустые входят в набор, 077-6).
func threeEd(t *testing.T) (e *album.Editor, ids []string) {
	t.Helper()
	e, s1, s2 := edtest.BdEd(t)
	v := edtest.NewSpread(e, -1, nil)
	if len(v.Spreads) != 3 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	return e, []string{s1, s2, v.Spreads[2].ID}
}

func targetsOf(scope string, ids ...string) []model.GridTarget {
	out := make([]model.GridTarget, len(ids))
	for i, id := range ids {
		out[i] = model.GridTarget{Spread: id, Scope: scope}
	}
	return out
}

func preset(kind string) model.BgSource {
	return model.BgSource{Pattern: &model.Pattern{Kind: kind, Preset: "sa:" + kind}, Color: "#F0E0D0"}
}

func applyTo(t *testing.T, e *album.Editor, ts []model.GridTarget, src model.BgSource) int {
	t.Helper()
	_, n, err := e.ApplyBackgroundTo(ts, src)
	if err != nil {
		t.Fatal(err)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	return n
}

// Узор на все развороты: на каждом узор и цвет, прежние фото и узоры сняты, пустой разворот
// тоже с фоном; одна правка — один шаг, отмена и возврат — всех сразу; повтор — холостая; файл
// помнит узор на всех.
func TestApplyBackgroundToPreset(t *testing.T) {
	e, ids := threeEd(t)
	edtest.SetBg(t, e, ids[0], "left", "bg")
	apply(t, e, ids[1], "right", model.Pattern{Kind: "stripes"}, "")
	undo := edtest.UndoLen(e)

	if n := applyTo(t, e, targetsOf("spread", ids...), preset("dots")); n != 3 {
		t.Fatalf("изменено листов %d, ждали 3", n)
	}
	for _, id := range ids {
		s := sheetOf(e, id)
		if s.Pat == nil || s.Pat.Kind != "dots" || s.Pat.Preset != "sa:dots" || s.PatL != nil || s.PatR != nil ||
			len(s.Backdrops()) != 0 || s.BgL != "#f0e0d0" || s.BgR != "#f0e0d0" {
			t.Fatalf("%s: узор %+v L=%v R=%v фото=%v цвет %q/%q", id, s.Pat, s.PatL, s.PatR, s.Backdrops(), s.BgL, s.BgR)
		}
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали один", edtest.UndoLen(e)-undo)
	}
	rev := edtest.Rev(e)
	if n := applyTo(t, e, targetsOf("spread", ids...), preset("dots")); n != 0 {
		t.Fatalf("повтор: изменено %d", n)
	}
	edtest.Same(t, e, rev, undo+1, "повтор той же плитки")

	q := edtest.SaveLoad(t, e)
	for i, s := range q.Spreads {
		if s.Pat == nil || s.Pat.Kind != "dots" || s.BgL != "#f0e0d0" {
			t.Fatalf("после Save → Load, разворот %d: %+v %q", i, s.Pat, s.BgL)
		}
	}

	e.Undo()
	if s := sheetOf(e, ids[0]); s.Pat != nil || s.BgPhotoL == nil || s.BgPhotoL.Photo != "bg" {
		t.Fatalf("отмена, разворот 1: %+v %+v", s.Pat, s.BgPhotoL)
	}
	if s := sheetOf(e, ids[1]); s.Pat != nil || s.PatR == nil || s.PatR.Kind != "stripes" {
		t.Fatalf("отмена, разворот 2: %+v %+v", s.Pat, s.PatR)
	}
	if s := sheetOf(e, ids[2]); len(s.Patterns()) != 0 || s.BgL != "" {
		t.Fatalf("отмена, пустой разворот: %+v %q", s.Patterns(), s.BgL)
	}
	e.Redo()
	for _, id := range ids {
		if s := sheetOf(e, id); s.Pat == nil || s.Pat.Kind != "dots" {
			t.Fatalf("возврат, %s: %+v", id, s.Pat)
		}
	}
}

// Своё фото на левые страницы: одно фото с ключом на всех, узоры страницы сняты (узор листа —
// правой), правые не тронуты; то же фото повторно — холостая, кадр не сброшен; в файле — ключ.
func TestApplyBackgroundToOwn(t *testing.T) {
	const key = "my:0123456789abcdef"
	lib := filepath.Join(testkit.TempDir(t), "Фоны", "0123456789abcdef.jpg")
	edtest.PutFile(t, lib, 10)
	edtest.LibFinder(t, map[string]string{key: lib})
	e, ids := threeEd(t)
	apply(t, e, ids[0], "spread", model.Pattern{Kind: "stripes"}, "")
	edtest.SetBg(t, e, ids[1], "right", "bg")
	ph := libPhoto(key, lib)
	src := model.BgSource{Photo: ph}
	undo := edtest.UndoLen(e)

	if n := applyTo(t, e, targetsOf("left", ids...), src); n != 3 {
		t.Fatalf("изменено %d, ждали 3", n)
	}
	for _, id := range ids {
		if s := sheetOf(e, id); s.BgPhotoL == nil || s.BgPhotoL.Photo != ph.ID || s.PatL != nil || s.Pat != nil {
			t.Fatalf("%s: фото %+v узоры S=%v L=%v", id, s.BgPhotoL, s.Pat, s.PatL)
		}
	}
	if s := sheetOf(e, ids[0]); s.PatR == nil || s.PatR.Kind != "stripes" {
		t.Fatalf("узор листа не ушёл правой странице: %+v", s.PatR)
	}
	if s := sheetOf(e, ids[1]); s.BgPhotoR == nil || s.BgPhotoR.Photo != "bg" {
		t.Fatalf("правая задета: %+v", s.BgPhotoR)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали один", edtest.UndoLen(e)-undo)
	}

	e.SetBackdropFrame(ids[1], "left", 2, 0.5, 0.5)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if n := applyTo(t, e, targetsOf("left", ids...), src); n != 0 {
		t.Fatalf("то же своё: изменено %d", n)
	}
	edtest.Same(t, e, rev, undo, "то же своё")
	if b := edtest.BdOf(e, ids[1], "left"); b.Frame == nil || b.Frame.S != 2 {
		t.Fatalf("кадр сброшен повтором: %+v", b.Frame)
	}

	q := edtest.SaveLoad(t, e)
	if r, ok := q.Photos[ph.ID]; !ok || r.Lib != key || r.Path != lib {
		t.Fatalf("после Save → Load: %+v %v", r, ok)
	}
	for i, s := range q.Spreads {
		if s.BgPhotoL == nil || s.BgPhotoL.Photo != ph.ID {
			t.Fatalf("после Save → Load, разворот %d: %+v", i, s.BgPhotoL)
		}
	}
}

// Цели: пусто и nil — холостая; обложка, неизвестный ID и дубли пропускаются; обе страницы листа —
// один лист в счёте; неверная область, источник, страница листа 031 и превышение предела — отказ
// всей правки без шага, даже если до плохой цели правка уже шла.
func TestApplyBackgroundToTargets(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(0, 0, 100, 100, "a")})
	sid := e.View().Spreads[0].ID
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, ts := range [][]model.GridTarget{nil, {}, {{Spread: model.CoverID, Scope: "spread"}}, {{Spread: "нет", Scope: "left"}}} {
		if n := applyTo(t, e, ts, preset("dots")); n != 0 {
			t.Fatalf("%v: изменено %d", ts, n)
		}
		edtest.Same(t, e, rev, undo, "пустые и чужие цели")
	}

	bad := []struct {
		name string
		ts   []model.GridTarget
		src  model.BgSource
		want error
	}{
		{"корешок", append(targetsOf("spread", sid), model.GridTarget{Spread: model.CoverID, Scope: "spine"}), preset("dots"), model.ErrScope},
		{"мусор", append(targetsOf("left", sid), model.GridTarget{Spread: sid, Scope: "top"}), preset("dots"), model.ErrScope},
		{"пустая область", targetsOf("", sid), preset("dots"), model.ErrScope},
		{"нет источника", targetsOf("spread", sid), model.BgSource{}, model.ErrPattern},
		{"два источника", targetsOf("spread", sid), model.BgSource{Pattern: &model.Pattern{Kind: "dots"}, Photo: edtest.Photo("x")}, model.ErrPattern},
		{"плохой узор", targetsOf("spread", sid), model.BgSource{Pattern: &model.Pattern{Kind: "Не так"}}, model.ErrPattern},
		{"плохой ключ", targetsOf("spread", sid), model.BgSource{Photo: libPhoto("my:XYZ", edtest.Abs("x.jpg"))}, model.ErrLibKey},
	}
	for _, c := range bad {
		if _, n, err := e.ApplyBackgroundTo(c.ts, c.src); !errors.Is(err, c.want) || n != 0 {
			t.Fatalf("%s: ошибка %v, изменено %d", c.name, err, n)
		}
		if plan, err := e.PlanBackgroundTo(c.ts, c.src); !errors.Is(err, c.want) || plan != (model.BgPlan{}) {
			t.Fatalf("%s, план: %+v %v", c.name, plan, err)
		}
		edtest.Same(t, e, rev, undo, c.name)
	}
	many := make([]model.GridTarget, 3*model.MaxSpreads+1)
	for i := range many {
		many[i] = model.GridTarget{Spread: sid, Scope: "spread"}
	}
	if _, _, err := e.ApplyBackgroundTo(many, preset("dots")); err == nil {
		t.Fatal("предел целей не сработал")
	}
	edtest.Same(t, e, rev, undo, "предел целей")

	ts := append(targetsOf("left", sid, sid, "нет", model.CoverID), targetsOf("right", sid)...)
	if n := applyTo(t, e, ts, preset("dots")); n != 1 {
		t.Fatalf("обе страницы одного листа: изменено %d, ждали 1", n)
	}
	if s := sheetOf(e, sid); s.PatL == nil || s.PatR == nil || s.Pat != nil {
		t.Fatalf("страницы: L=%v R=%v S=%v", s.PatL, s.PatR, s.Pat)
	}
	if c := sheetOf(e, model.CoverID); len(c.Patterns()) != 0 || c.BgL != "" {
		t.Fatalf("обложка задета: %+v %q", c.Patterns(), c.BgL)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали один", edtest.UndoLen(e)-undo)
	}

	// Лист 031: страниц нет — страница на любом листе набора отменяет всю правку.
	sh := edtest.Open(t, model.NewSheet(200, 300), "")
	edtest.NewSpread(sh, -1, edtest.Photos("a"))
	v := edtest.NewSpread(sh, -1, edtest.Photos("b"))
	a, b := v.Spreads[0].ID, v.Spreads[1].ID
	rev, undo = edtest.Rev(sh), edtest.UndoLen(sh)
	if _, _, err := sh.ApplyBackgroundTo(append(targetsOf("spread", a), targetsOf("left", b)...), preset("dots")); !errors.Is(err, model.ErrScope) {
		t.Fatalf("страница листа 031: %v", err)
	}
	edtest.Same(t, sh, rev, undo, "лист 031")
	if s := sheetOf(sh, a); s.Pat != nil {
		t.Fatalf("отказ не откатил первую цель: %+v", s.Pat)
	}
	if n := applyTo(t, sh, targetsOf("spread", a, b), preset("dots")); n != 2 {
		t.Fatalf("лист 031, весь лист: изменено %d", n)
	}
}

// Сухой прогон: число листов и фото фона, которые уйдут или заменятся; проект не меняется.
// Своё фото там, где оно уже стоит, не в счёт (там холостая); фото, поставленное и снятое той же
// правкой, — тоже; фото всего листа при цели-странице — в счёт.
func TestPlanBackgroundTo(t *testing.T) {
	e, ids := threeEd(t)
	edtest.SetBg(t, e, ids[0], "spread", "bg")
	edtest.SetBg(t, e, ids[1], "left", "bg2")
	edtest.SetBg(t, e, ids[1], "right", "bg3")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)

	ts := append(targetsOf("left", ids...), targetsOf("right", ids[1])...)
	plan, err := e.PlanBackgroundTo(ts, preset("dots"))
	if err != nil || plan != (model.BgPlan{Spreads: 3, Photos: 3}) {
		t.Fatalf("узор: %+v %v", plan, err)
	}
	edtest.Same(t, e, rev, undo, "сухой прогон")
	if edtest.BdOf(e, ids[0], "spread") == nil || edtest.BdOf(e, ids[1], "left") == nil {
		t.Fatal("сухой прогон снял фото")
	}

	own := model.BgSource{Photo: edtest.Photo("bg2")}
	if plan, _ = e.PlanBackgroundTo(targetsOf("left", ids[1]), own); plan != (model.BgPlan{}) {
		t.Fatalf("то же фото на месте: %+v", plan)
	}
	if plan, _ = e.PlanBackgroundTo(targetsOf("right", ids[1], ids[2]), own); plan != (model.BgPlan{Spreads: 2, Photos: 1}) {
		t.Fatalf("другое фото на правой: %+v", plan)
	}
	ts = append(targetsOf("left", ids[2]), targetsOf("spread", ids[2])...)
	if plan, _ = e.PlanBackgroundTo(ts, model.BgSource{Photo: edtest.Photo("new")}); plan != (model.BgPlan{Spreads: 1}) {
		t.Fatalf("левая, затем весь лист пустого разворота: %+v", plan)
	}
	if plan, _ = e.PlanBackgroundTo(nil, preset("dots")); plan != (model.BgPlan{}) {
		t.Fatalf("без целей: %+v", plan)
	}

	// План сходится с правкой: столько же листов.
	if n := applyTo(t, e, targetsOf("spread", ids...), preset("dots")); n != 3 {
		t.Fatalf("правка: %d", n)
	}
	if plan, _ = e.PlanBackgroundTo(targetsOf("spread", ids...), preset("dots")); plan != (model.BgPlan{}) {
		t.Fatalf("после правки: %+v", plan)
	}
}

// Счёт листов совпадает с холостой правкой Editor (сравнение JSON): цвета узора nil и пустой
// срез — одно и то же (в файле оба не пишутся), иначе тост сказал бы «поставлен», а шага нет.
func TestApplyBackgroundToInksNil(t *testing.T) {
	src := preset("dots")
	pat := model.ClampPattern(src.Pattern)
	if pat.Inks == nil {
		pat.Inks = []string{}
	} else {
		pat.Inks = nil
	}
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Pat: pat, BgL: "#f0e0d0", BgR: "#f0e0d0"}}
	before := string(model.Marshal(p))
	n, err := backdrop.ApplyTo(p, targetsOf("spread", "s1"), src)
	if err != nil || n != 0 {
		t.Fatalf("тот же узор с другим видом пустых цветов: изменено %d, %v", n, err)
	}
	if after := string(model.Marshal(p)); after != before {
		t.Fatalf("JSON изменился, а счёт 0:\n%s\n%s", before, after)
	}
}

// 200 разворотов × «весь разворот», узор: правка одним шагом (снимок, сравнение, View на весь
// альбом) — ~8 мс, сухой прогон (копия проекта) — ~3 мс (замер G1, 18.10). Пороги вшестеро
// выше: под нагрузкой ЦП втрое медленнее (как TestPreviewNamesFast), а квадратичный рост по
// листам — в сотни раз.
func TestApplyBackgroundToFast(t *testing.T) {
	p := model.New()
	for i := range 200 {
		a, b, c := model.NewID(), model.NewID(), model.NewID()
		for _, id := range []string{a, b, c} {
			p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: 300, H: 200}
		}
		s := &model.Spread{ID: model.NewID(), Photos: []string{a, b}}
		if i%2 == 0 { // половина — с фото фона: замена, а не только постановка
			s.BgPhotoL = &model.Backdrop{Photo: c, W: 100, H: 100}
		}
		p.Spreads = append(p.Spreads, s)
	}
	e := edtest.Open(t, p, "")
	ids := make([]string, len(p.Spreads))
	for i, s := range e.View().Spreads {
		ids[i] = s.ID
	}
	ts := targetsOf("spread", ids...)
	kinds := []string{"dots", "stripes"}
	var k, n int
	d := testkit.Fastest(5, func() {
		_, n, _ = e.ApplyBackgroundTo(ts, preset(kinds[k%2]))
		k++
	})
	if n != len(ids) {
		t.Fatalf("изменено %d, ждали %d", n, len(ids))
	}
	dp := testkit.Fastest(5, func() { _, _ = e.PlanBackgroundTo(ts, preset(kinds[k%2])) })
	t.Logf("правка %v, план %v", d, dp)
	if lim := testkit.Limit(50 * time.Millisecond); d > lim {
		t.Fatalf("фон на 200 разворотов %v, порог %v", d, lim)
	}
	if lim := testkit.Limit(20 * time.Millisecond); dp > lim {
		t.Fatalf("план на 200 разворотов %v, порог %v", dp, lim)
	}
}
