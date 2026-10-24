package spreads_test

// «Весь альбом заново» (107) на уровне Editor: голые развороты заменяются новыми, якоря (ручные,
// виньетки, оформленные, пустые) и обложка остаются байт в байт между теми же фото, свойства фото
// и ID едут, похожие уходят в ленту, план = предпросмотр = сборка.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// albumInfo — фото id с пропорциями по номеру i (как mixedPhotos).
func albumInfo(id string, i int) model.PhotoInfo {
	w, h := 300, 200
	switch {
	case i%7 == 3:
		w, h = 250, 250
	case i%3 == 0:
		w, h = 200, 300
	}
	return model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: w, H: h}
}

// albumFix — альбом с якорями: голые s0 (p0–p3), s2 (p4–p9), s4 (p10–p15), s7 (p16–p21); ручной
// man (m0, m1), оформленный dec (d0, d1, надпись), виньетка, пустой blank; обложка с фото cv.
// Лента feed: f0 до всех, p0…p9, m0, m1, f1, f2, p10…p15, d0, d1, p16…p21, f3, cv.
type albumFix struct {
	e    *album.Editor
	feed []model.PhotoInfo
	vig  string // ID виньетки
}

func newAlbumFix(t testing.TB) albumFix {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	var feed []model.PhotoInfo
	n := 0
	add := func(id string, inFeed bool) string {
		ph := albumInfo(id, n)
		n++
		p.Photos[id] = model.PhotoRef{Path: ph.Path, W: ph.W, H: ph.H}
		if inFeed {
			feed = append(feed, ph)
		}
		return id
	}
	run := func(prefix string, from, to int) []string {
		var out []string
		for i := from; i < to; i++ {
			out = append(out, add(fmt.Sprint(prefix, i), true))
		}
		return out
	}
	add("f0", true)
	bare0 := run("p", 0, 4)
	bare1 := run("p", 4, 10)
	manual := []string{add("m0", true), add("m1", true)}
	add("f1", true)
	add("f2", true)
	bare2 := run("p", 10, 16)
	decor := []string{add("d0", true), add("d1", true)}
	bare3 := run("p", 16, 22)
	add("f3", true)
	p.Spreads = []*model.Spread{
		{ID: "bare0", Photos: bare0},
		{ID: "man", Photos: manual, Grid: []*model.Place{edtest.Gp(10, 10, 100, 80, "m0"), edtest.Gp(130, 10, 100, 80, "m1")}},
		{ID: "bare1", Photos: bare1},
		{ID: "dec", Photos: decor, Name: "Банкет", Texts: []*model.TextBlock{{ID: "0a0b0c0d0e01", X: 20, Y: 50, W: 100, Text: "Анна и Павел", Style: model.PlainStyle()}}},
		{ID: "bare2", Photos: bare2},
		{ID: "blank"},
		{ID: "bare3", Photos: bare3},
	}
	for i, pl := range p.Spreads[1].Grid {
		pl.ID = fmt.Sprintf("%012x", i+1)
	}
	e := edtest.Open(t, p, "")
	if _, err := e.AddVignette(5); err != nil {
		t.Fatal(err)
	}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	edtest.Change(e, func(p *model.Project) {
		p.Photos["cv"] = model.PhotoRef{Path: edtest.Abs("cv.jpg"), W: 300, H: 200}
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Photos: []string{"cv"},
			Grid: []*model.Place{{ID: edtest.CoverPlace, X: 400, Y: 100, W: 120, H: 80, Photo: "cv"}}}}
	})
	f := albumFix{e: e, feed: append(feed, albumInfo("cv", n))}
	for _, s := range edtest.P(e).Spreads {
		if s.Kind == model.KindVignette {
			f.vig = s.ID
		}
	}
	if f.vig == "" {
		t.Fatal("виньетка не создана")
	}
	return f
}

// albumOpts — настройки пересборки альбома на n новых разворотов.
func albumOpts(n int) model.AutoOpts {
	return model.AutoOpts{Spreads: n, Auto: true, Variety: model.VarietyNormal, Album: true}
}

// anchorIDs — ID якорей фикстуры по порядку альбома.
func (f albumFix) anchorIDs() []string { return []string{"man", "dec", f.vig, "blank"} }

// bytesOf — JSON разворота по ID ("" — нет такого).
func bytesOf(p *model.Project, id string) string {
	if _, s := p.Find(id); s != nil {
		b, _ := json.Marshal(s)
		return string(b)
	}
	return ""
}

// photosBefore — множество ID фото разворотов «Авто» перед якорем id.
func photosBefore(p *model.Project, id string) []string {
	var out []string
	for _, s := range p.Spreads {
		if s.ID == id {
			break
		}
		if len(s.Grid) == 0 {
			out = append(out, s.Photos...)
		}
	}
	slices.Sort(out)
	return out
}

// bareSpreads — развороты «Авто» с фото в порядке альбома (без якорей).
func bareSpreads(p *model.Project, f albumFix) []*model.Spread {
	anchors := f.anchorIDs()
	var out []*model.Spread
	for _, s := range p.Spreads {
		if !slices.Contains(anchors, s.ID) {
			out = append(out, s)
		}
	}
	return out
}

// flat — фото данных разворотов подряд.
func flat(ss []*model.Spread) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Photos...)
	}
	return out
}

// TestAlbumAnchorsKept: якоря (ручной, оформленный, виньетка, пустой) и обложка после пересборки
// байт в байт; стоят в том же порядке; при разных N (меньше, столько же, больше голых).
func TestAlbumAnchorsKept(t *testing.T) {
	// Стен четыре отрезка (якоря подряд — одна стена): меньше четырёх разворотов не соберётся.
	for _, n := range []int{2, 4, 9} {
		f := newAlbumFix(t)
		before := edtest.P(f.e).Clone()
		coverBefore, _ := json.Marshal(before.Cover)
		if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(n)); err != nil {
			t.Fatalf("N=%d: %v", n, err)
		}
		p := edtest.P(f.e)
		for _, id := range f.anchorIDs() {
			if got, want := bytesOf(p, id), bytesOf(before, id); got == "" || got != want {
				t.Errorf("N=%d: якорь %s изменился:\n%s\n%s", n, id, got, want)
			}
		}
		var order []string
		for _, s := range p.Spreads {
			if slices.Contains(f.anchorIDs(), s.ID) {
				order = append(order, s.ID)
			}
		}
		if !slices.Equal(order, f.anchorIDs()) {
			t.Errorf("N=%d: порядок якорей %v", n, order)
		}
		if got := len(bareSpreads(p, f)); got != max(n, 4) {
			t.Errorf("N=%d: новых разворотов %d", n, got)
		}
		// Корешок обложки зависит от числа разворотов: при том же числе обложка не двигается.
		if n == 4 {
			if got, _ := json.Marshal(p.Cover); !bytes.Equal(got, coverBefore) {
				t.Errorf("обложка изменилась:\n%s\n%s", got, coverBefore)
			}
		}
	}
}

// TestAlbumAnchorsInPlace: якорь стоит между теми же фото: перед ним — фото «Авто», что были перед
// ним, и свободные, чей предшественник по ленте среди них; после — остальные.
func TestAlbumAnchorsInPlace(t *testing.T) {
	for _, n := range []int{4, 5, 9} {
		f := newAlbumFix(t)
		old := edtest.P(f.e).Clone()
		free := map[string]string{"f1": "p9", "f2": "p9", "f3": "p21"} // свободное → предшественник; f0 — в начало
		want := map[string][]string{}
		for _, id := range f.anchorIDs() {
			set := photosBefore(old, id)
			set = slices.DeleteFunc(set, func(x string) bool { return x == "d0" || x == "d1" })
			if len(set) > 0 { // f0 встаёт в начало пула, но не перед якорем самого начала
				set = append(set, "f0")
			}
			for fr, pred := range free {
				if slices.Contains(set, pred) {
					set = append(set, fr)
				}
			}
			slices.Sort(set)
			want[id] = set
		}
		if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(n)); err != nil {
			t.Fatal(err)
		}
		p := edtest.P(f.e)
		for id, w := range want {
			got := slices.DeleteFunc(photosBefore(p, id), func(x string) bool { return x == "d0" || x == "d1" })
			if !slices.Equal(got, w) {
				t.Errorf("N=%d: перед %s %v, ждали %v", n, id, got, w)
			}
		}
	}
}

// TestAlbumPoolOrder: фото идут в порядке альбома, свободные — после своего предшественника
// по ленте; если альбом переставлен руками, порядок альбома побеждает.
func TestAlbumPoolOrder(t *testing.T) {
	f := newAlbumFix(t)
	if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(5)); err != nil {
		t.Fatal(err)
	}
	var want []string
	want = append(want, "f0")
	for i := range 10 {
		want = append(want, fmt.Sprint("p", i))
	}
	want = append(want, "f1", "f2")
	for i := 10; i < 22; i++ {
		want = append(want, fmt.Sprint("p", i))
	}
	want = append(want, "f3")
	if got := flat(bareSpreads(edtest.P(f.e), f)); !slices.Equal(got, want) {
		t.Fatalf("порядок %v, ждали %v", got, want)
	}

	// Альбом переставлен: голый bare3 (p16–p21) перенесён в начало; свободные идут по ленте
	// за своими предшественниками (f3 — за p21, f0 — в начало).
	f = newAlbumFix(t)
	f.e.MoveSpread("bare3", 0)
	if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(5)); err != nil {
		t.Fatal(err)
	}
	want = []string{"f0"}
	for i := 16; i < 22; i++ {
		want = append(want, fmt.Sprint("p", i))
	}
	want = append(want, "f3")
	for i := range 10 {
		want = append(want, fmt.Sprint("p", i))
	}
	want = append(want, "f1", "f2")
	for i := 10; i < 16; i++ {
		want = append(want, fmt.Sprint("p", i))
	}
	if got := flat(bareSpreads(edtest.P(f.e), f)); !slices.Equal(got, want) {
		t.Fatalf("после переноса: %v, ждали %v", got, want)
	}

	// Без свободных в ленте (галочка выключена) пул — только фото альбома.
	f = newAlbumFix(t)
	var inAlbum []model.PhotoInfo
	for _, ph := range f.feed {
		if ph.ID[0] != 'f' {
			inAlbum = append(inAlbum, ph)
		}
	}
	if _, err := f.e.AutoBuild(inAlbum, -1, albumOpts(4)); err != nil {
		t.Fatal(err)
	}
	if got := flat(bareSpreads(edtest.P(f.e), f)); len(got) != 22 || slices.Contains(got, "f0") || slices.Contains(got, "f3") {
		t.Fatalf("без свободных: %v", got)
	}
}

// TestAlbumKeepsRefs: свой кадр, кадр, подпись, зеркало, тон, имя и копия фото (098) едут с фото —
// записи проекта у стоящих в пуле фото не меняются.
func TestAlbumKeepsRefs(t *testing.T) {
	f := newAlbumFix(t)
	pad := 2.0
	edtest.Change(f.e, func(p *model.Project) {
		set := func(id string, fn func(r *model.PhotoRef)) {
			r := p.Photos[id]
			fn(&r)
			p.Photos[id] = r
		}
		set("p1", func(r *model.PhotoRef) { r.Asp = 1.5; r.Frame = &model.Frame{S: 2, X: 0.5, Y: -0.5} })
		set("p2", func(r *model.PhotoRef) { r.Adj = &model.Tone{Gray: true, Bright: 10} })
		set("p3", func(r *model.PhotoRef) {
			r.Caption = &model.Caption{Text: "Подпись", Style: model.PlainStyle(), Pad: &pad}
			r.FlipH = true
		})
		set("p4", func(r *model.PhotoRef) { r.FlipV, r.Fade = true, 0.5 })
		// Копия p5 стоит рядом с ним: запись только в проекте, в ленте её нет.
		cp := p.Photos["p5"]
		cp.Asp = 1.2
		p.Photos["p5-2"] = cp
		p.Spreads[2].Photos = append(p.Spreads[2].Photos, "p5-2")
	})
	before := map[string]model.PhotoRef{}
	for id, r := range edtest.P(f.e).Photos {
		before[id] = r
	}
	if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(4)); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(f.e)
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5", "p5-2", "cv"} {
		if !reflect.DeepEqual(p.Photos[id], before[id]) {
			t.Errorf("%s: запись изменилась: %+v, была %+v", id, p.Photos[id], before[id])
		}
	}
	if !slices.Contains(flat(bareSpreads(p, f)), "p5-2") {
		t.Error("копия p5-2 пропала из альбома")
	}
}

// TestAlbumReuseIDs: новые развороты берут ID прежних голых по порядку; лишним — новые; при
// меньшем N остаются первые.
func TestAlbumReuseIDs(t *testing.T) {
	old := []string{"bare0", "bare1", "bare2", "bare3"}
	for _, n := range []int{4, 5, 6} {
		f := newAlbumFix(t)
		if _, err := f.e.AutoBuild(f.feed, -1, albumOpts(n)); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, s := range bareSpreads(edtest.P(f.e), f) {
			ids = append(ids, s.ID)
		}
		k := min(n, len(old))
		if len(ids) != n || !slices.Equal(ids[:k], old[:k]) {
			t.Errorf("N=%d: ID %v, ждали начало %v", n, ids, old[:k])
		}
		for _, id := range ids[k:] {
			if slices.Contains(old, id) {
				t.Errorf("N=%d: ID %s повторён", n, id)
			}
		}
	}
}

// TestAlbumNothingToRebuild: голых нет — холостая правка (rev, шаги отмены и сам проект те же),
// и с пустой лентой, и со свободными; AutoPlan объясняет «пересобирать нечего». Пустой альбом — то же.
func TestAlbumNothingToRebuild(t *testing.T) {
	f := newAlbumFix(t)
	edtest.Change(f.e, func(p *model.Project) {
		p.Spreads = slices.DeleteFunc(p.Spreads, func(s *model.Spread) bool { return !slices.Contains(f.anchorIDs(), s.ID) })
	})
	for _, feed := range [][]model.PhotoInfo{nil, f.feed} {
		rev, undo, data := edtest.Rev(f.e), edtest.UndoLen(f.e), model.Marshal(edtest.P(f.e))
		ap, err := f.e.AutoPlan(feed, albumOpts(3))
		if err != nil || ap.Photos != 0 || ap.Spreads != 0 || ap.Album == nil || ap.Album.Spreads != 0 || ap.Album.Manual != 1 || ap.Album.Decor != 1 || ap.Album.Blank != 1 || ap.Album.Vignettes != 1 {
			t.Fatalf("план: %+v %v", ap, err)
		}
		v, err := f.e.AutoBuild(feed, -1, albumOpts(3))
		if err != nil || edtest.Rev(f.e) != rev || edtest.UndoLen(f.e) != undo || v.Again || !bytes.Equal(model.Marshal(edtest.P(f.e)), data) {
			t.Fatalf("сборка не холостая: rev %d→%d, шагов %d→%d, again %v (%v)", rev, edtest.Rev(f.e), undo, edtest.UndoLen(f.e), v.Again, err)
		}
	}
	empty := edtest.Open(t, model.New(), "")
	if _, err := empty.AutoBuild(f.feed, -1, albumOpts(3)); err != nil || edtest.UndoLen(empty) != 0 || len(edtest.P(empty).Spreads) != 0 {
		t.Fatalf("пустой альбом: шагов %d, разворотов %d (%v)", edtest.UndoLen(empty), len(edtest.P(empty).Spreads), err)
	}
}

// TestAlbumMaxSpreads: якоря занимают место под предел 1000 разворотов: N зажимается так, чтобы
// всего было не больше; плотность, при которой не хватает места, — ошибка, альбом цел.
func TestAlbumMaxSpreads(t *testing.T) {
	p := model.New()
	var ph []model.PhotoInfo
	for i := range 30 {
		x := albumInfo(fmt.Sprint("q", i), i)
		ph = append(ph, x)
		p.Photos[x.ID] = model.PhotoRef{Path: x.Path, W: x.W, H: x.H}
	}
	for i := range model.MaxSpreads - 5 {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprint("a", i), Name: fmt.Sprint("A", i)})
	}
	for i := range 5 {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprint("b", i), Photos: []string{fmt.Sprint("q", 6*i), fmt.Sprint("q", 6*i+1), fmt.Sprint("q", 6*i+2), fmt.Sprint("q", 6*i+3), fmt.Sprint("q", 6*i+4), fmt.Sprint("q", 6*i+5)}})
	}
	e := edtest.Open(t, p, "")
	ap, err := e.AutoPlan(ph, model.AutoOpts{Spreads: 999, MaxPer: 16, Album: true})
	if err != nil || ap.Spreads != 5 || ap.MaxSpreads != 5 || ap.Album.Spreads != 5 {
		t.Fatalf("N зажат: %+v %v", ap, err)
	}
	if _, err := e.AutoBuild(ph, -1, model.AutoOpts{Spreads: 999, MaxPer: 16, Album: true}); err != nil || len(edtest.P(e).Spreads) != model.MaxSpreads {
		t.Fatalf("сборка на предел: %d разворотов (%v)", len(edtest.P(e).Spreads), err)
	}
	// Не помещается: 30 фото по 2 на разворот нужно 15 разворотов, места 5.
	rev, data := edtest.Rev(e), model.Marshal(edtest.P(e))
	if _, err := e.AutoPlan(ph, model.AutoOpts{Spreads: 15, MinPer: 1, MaxPer: 2, Album: true}); err == nil {
		t.Fatal("AutoPlan: ждали ошибку про предел разворотов")
	}
	if _, err := e.AutoBuild(ph, -1, model.AutoOpts{Spreads: 15, MinPer: 1, MaxPer: 2, Album: true}); err == nil || edtest.Rev(e) != rev || !bytes.Equal(model.Marshal(edtest.P(e)), data) {
		t.Fatalf("AutoBuild: ждали ошибку и целый альбом (%v)", err)
	}
}

// TestAlbumSimilarToRibbon: «Убирать похожие кадры» в режиме альбома: отброшенные фото альбома
// уходят в ленту (из проекта и разворотов), оставленные — на месте; AutoSimilar, план и сборка
// видят один отбор; Ctrl+Z возвращает.
func TestAlbumSimilarToRibbon(t *testing.T) {
	ph, first := scenes("s", 60)
	e := edtest.Open(t, model.New(), "")
	build(t, e, ph, -1, model.AutoOpts{Spreads: 8, Auto: true})
	if len(placedIDs(edtest.P(e))) != 60 {
		t.Fatal("сборка взяла не все фото")
	}
	before := model.Marshal(edtest.P(e))
	o := model.AutoOpts{Spreads: 6, Auto: true, Similar: 1, Album: true}
	drops, err := e.AutoSimilar(ph, o)
	if err != nil || len(drops) != 60-len(first) {
		t.Fatalf("отброшено %d, ждали %d (%v)", len(drops), 60-len(first), err)
	}
	ap, err := e.AutoPlan(ph, o)
	if err != nil || ap.Similar != len(drops) || ap.Photos != len(first) || ap.Album == nil || ap.Album.Photos != 60 {
		t.Fatalf("план %+v (%v)", ap, err)
	}
	build(t, e, ph, -1, o)
	p := edtest.P(e)
	var kept []string
	for _, i := range first {
		kept = append(kept, ph[i].ID)
	}
	if got := placedIDs(p); !slices.Equal(got, kept) {
		t.Fatalf("в альбоме %v, ждали %v", got, kept)
	}
	for _, d := range drops {
		if _, ok := p.Photos[d.ID]; ok || p.Placed(d.ID) {
			t.Errorf("отброшенный %s остался в проекте", d.ID)
		}
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("Ctrl+Z: альбом не вернулся")
	}
}

// TestAlbumPlanMatchesBuild: AutoPlan, «Что соберётся» и сборка видят одно: число разворотов,
// фото на каждом (по порядку), ч/б, раскладки, запасные; уровни разнообразия, ч/б и варианты.
func TestAlbumPlanMatchesBuild(t *testing.T) {
	for _, o := range []model.AutoOpts{
		{Spreads: 4, Auto: true},
		{Spreads: 5, Auto: true, Variety: model.VarietyMore, Gray: 2, Variant: 3},
		{Spreads: 6, MinPer: 2, MaxPer: 9, Variety: model.VarietyNormal, Variant: 2, Series: true},
		{Spreads: 4, MaxPer: 16, Families: []string{"hero", "dense"}, Variety: model.VarietyNormal, Gray: 1},
	} {
		o.Album = true
		name := fmt.Sprintf("%+v", o)
		f := newAlbumFix(t)
		ap, err := f.e.AutoPlan(f.feed, o)
		if err != nil || ap.Album == nil || ap.Album.Spreads != 4 || ap.Album.Free != 4 || ap.Album.Manual != 1 || ap.Album.Vignettes != 1 || ap.Album.Decor != 1 || ap.Album.Blank != 1 {
			t.Fatalf("%s: план %+v (%v)", name, ap, err)
		}
		rev := edtest.Rev(f.e)
		pv, err := f.e.AutoPreview(f.feed, -1, o)
		if err != nil || edtest.Rev(f.e) != rev || len(pv.Spreads) != ap.Spreads {
			t.Fatalf("%s: предпросмотр %d разворотов, план %d (%v)", name, len(pv.Spreads), ap.Spreads, err)
		}
		if _, err := f.e.AutoBuild(f.feed, -1, o); err != nil {
			t.Fatal(err)
		}
		checkAlbumBuilt(t, name, edtest.P(f.e), f, ap, pv)
	}
}

// checkAlbumBuilt — собранные развороты против плана и предпросмотра: число, фото, раскладка, ч/б.
func checkAlbumBuilt(t *testing.T, name string, p *model.Project, f albumFix, ap model.AutoPlan, pv model.AutoPreview) {
	t.Helper()
	built := bareSpreads(p, f)
	if len(built) != ap.Spreads {
		t.Fatalf("%s: собрано %d, план %d", name, len(built), ap.Spreads)
	}
	if n := len(flat(built)); n != ap.Photos {
		t.Errorf("%s: фото %d, план %d", name, n, ap.Photos)
	}
	gray := 0
	for i, s := range built {
		r, _, _ := p.Layout(s)
		if len(s.Photos) != pv.Spreads[i].Photos || r.Key != pv.Spreads[i].Key {
			t.Errorf("%s: разворот %d: %d фото, раскладка %q; предпросмотр %d, %q", name, i, len(s.Photos), r.Key, pv.Spreads[i].Photos, pv.Spreads[i].Key)
		}
		allGray := true
		for _, id := range s.Photos {
			allGray = allGray && p.Photos[id].Adj != nil && p.Photos[id].Adj.Gray
		}
		if pv.Spreads[i].Gray {
			gray++
			if !allGray {
				t.Errorf("%s: разворот %d обещан ч/б, фото цветные", name, i)
			}
		}
	}
	if gray != ap.Gray {
		t.Errorf("%s: ч/б в предпросмотре %d, в плане %d", name, gray, ap.Gray)
	}
}

// TestAlbumPreviewMatchesBuild: тот же предпросмотр после смены ID и без свободных — совпадает со
// сборкой, и предпросмотр проект не меняет.
func TestAlbumPreviewMatchesBuild(t *testing.T) {
	f := newAlbumFix(t)
	o := model.AutoOpts{Spreads: 5, Auto: true, Variety: model.VarietyMore, Gray: 3, Album: true}
	data, rev, undo := model.Marshal(edtest.P(f.e)), edtest.Rev(f.e), edtest.UndoLen(f.e)
	pv, err := f.e.AutoPreview(f.feed, 99, o) // at не действует
	if err != nil || !bytes.Equal(model.Marshal(edtest.P(f.e)), data) || edtest.Rev(f.e) != rev || edtest.UndoLen(f.e) != undo {
		t.Fatalf("предпросмотр изменил проект (%v)", err)
	}
	if _, err := f.e.AutoBuild(f.feed, -7, o); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(f.e)
	built := bareSpreads(p, f)
	if len(built) != len(pv.Spreads) || len(pv.Samples) == 0 {
		t.Fatalf("собрано %d, предпросмотр %d (образцов %d)", len(built), len(pv.Spreads), len(pv.Samples))
	}
	for i, s := range built {
		if r, _, _ := p.Layout(s); r.Key != pv.Spreads[i].Key || len(s.Photos) != pv.Spreads[i].Photos {
			t.Errorf("разворот %d: %q/%d, предпросмотр %q/%d", i, r.Key, len(s.Photos), pv.Spreads[i].Key, pv.Spreads[i].Photos)
		}
	}
}
