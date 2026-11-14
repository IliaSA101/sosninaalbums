package spreads

// «Весь альбом заново» (107): что голое, что якорь, как свободные встают в пул и сдвигают стены,
// и что режим без якорей считает план побитово как обычная сборка тех же фото.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// albumPhotos — n фото p0… с проектом, где они уже записаны (разные пропорции).
func albumPhotos(n int) (*model.Project, []model.PhotoInfo) {
	p, ph := model.New(), make([]model.PhotoInfo, n)
	for i := range ph {
		w, h := 300, 200
		switch {
		case i%7 == 3:
			w, h = 250, 250
		case i%3 == 0:
			w, h = 200, 300
		}
		id := fmt.Sprint("p", i)
		ph[i] = model.PhotoInfo{ID: id, Path: "/t/" + id + ".jpg", W: w, H: h}
		p.Photos[id] = model.PhotoRef{Path: ph[i].Path, W: w, H: h}
	}
	return p, ph
}

func idsOf(ph []model.PhotoInfo) []string {
	out := make([]string, len(ph))
	for i, f := range ph {
		out[i] = f.ID
	}
	return out
}

// TestAlbumBare: голый — «Авто» с фото и ничего своего (раскладка и номер варианта не в счёт);
// всё остальное — якорь своего вида. Каждое поле разворота с данными пользователя делает якорем.
func TestAlbumBare(t *testing.T) {
	type kind int
	const (
		isBare kind = iota
		manual
		vignette
		decor
		blank
	)
	mk := func(f func(s *model.Spread)) *model.Spread {
		s := &model.Spread{ID: "x", Photos: []string{"p0", "p1"}}
		f(s)
		return s
	}
	cases := []struct {
		name string
		s    *model.Spread
		want kind
	}{
		{"голый", mk(func(*model.Spread) {}), isBare},
		{"с раскладкой", mk(func(s *model.Spread) { s.Layout = "m1" }), isBare},
		{"с номером варианта", mk(func(s *model.Spread) { s.Variant = 3 }), isBare},
		{"сетка", mk(func(s *model.Spread) { s.Grid = []*model.Place{{ID: "g"}} }), manual},
		{"виньетка", mk(func(s *model.Spread) { s.Kind = model.KindVignette }), vignette},
		{"виньетка с сеткой", mk(func(s *model.Spread) { s.Kind, s.Grid = model.KindVignette, []*model.Place{{ID: "g"}} }), vignette},
		{"текст", mk(func(s *model.Spread) { s.Texts = []*model.TextBlock{{ID: "t"}} }), decor},
		{"стикер", mk(func(s *model.Spread) { s.Stickers = []*model.Sticker{{ID: "k"}} }), decor},
		{"цвет слева", mk(func(s *model.Spread) { s.BgL = "#ffffff" }), decor},
		{"цвет справа", mk(func(s *model.Spread) { s.BgR = "#ffffff" }), decor},
		{"название", mk(func(s *model.Spread) { s.Name = "Сбор" }), decor},
		{"фон-фото", mk(func(s *model.Spread) { s.BgPhoto = &model.Backdrop{} }), decor},
		{"фон-фото слева", mk(func(s *model.Spread) { s.BgPhotoL = &model.Backdrop{} }), decor},
		{"фон-фото справа", mk(func(s *model.Spread) { s.BgPhotoR = &model.Backdrop{} }), decor},
		{"фон корешка", mk(func(s *model.Spread) { s.BgPhotoS = &model.Backdrop{} }), decor},
		{"узор", mk(func(s *model.Spread) { s.Pat = &model.Pattern{} }), decor},
		{"узор слева", mk(func(s *model.Spread) { s.PatL = &model.Pattern{} }), decor},
		{"узор справа", mk(func(s *model.Spread) { s.PatR = &model.Pattern{} }), decor},
		{"стиль подписей", mk(func(s *model.Spread) { s.Caps = &model.SheetCaps{} }), decor},
		{"поля шапки", mk(func(s *model.Spread) { s.Vars = &model.Vars{} }), decor},
		{"фон альбома скрыт", mk(func(s *model.Spread) { s.NoAlbumBg = true }), decor},
		{"пустой", &model.Spread{ID: "x"}, blank},
		{"без фото, но с названием", &model.Spread{ID: "x", Name: "Титул"}, decor},
	}
	for _, c := range cases {
		p, _ := albumPhotos(8)
		// Контроль: два голых разворота рядом всегда есть, пул не пуст.
		p.Spreads = []*model.Spread{{ID: "b0", Photos: []string{"p2", "p3"}}, c.s, {ID: "b1", Photos: []string{"p4", "p5"}}}
		ap, err := AlbumOf(p, nil, nil).Plan(model.AutoOpts{Spreads: 2, MaxPer: 8, Album: true}) // ленты нет: свободных нет
		if err != nil || ap.Album == nil {
			t.Fatalf("%s: %v %+v", c.name, err, ap)
		}
		want := model.AlbumPlan{Spreads: 2, Photos: 4}
		switch c.want {
		case isBare:
			want.Spreads, want.Photos = 3, 6
		case manual:
			want.Manual = 1
		case vignette:
			want.Vignettes = 1
		case decor:
			want.Decor = 1
		case blank:
			want.Blank = 1
		}
		if *ap.Album != want {
			t.Errorf("%s: состав %+v, ждали %+v", c.name, *ap.Album, want)
		}
	}
}

// TestAlbumPoolOrder: пул — фото голых разворотов в порядке альбома (а не ленты), свободные из
// photos встают после ближайшего предыдущего фото пула, без такого — в начало; несколько подряд —
// в порядке ленты; стоящие в якорях, на фоне и копией файла не берутся; стена сдвигается на
// вставленные перед ней, якорь в самом начале остаётся первым.
func TestAlbumPoolOrder(t *testing.T) {
	p, ph := albumPhotos(14)
	by := map[string]model.PhotoInfo{}
	for _, f := range ph {
		by[f.ID] = f
	}
	feed := func(ids ...string) []model.PhotoInfo {
		var out []model.PhotoInfo
		for _, id := range ids {
			out = append(out, by[id])
		}
		return out
	}
	// Альбом переставлен: голый b1 (p3 p4) стоит раньше b0 (p0 p1); между ними — якорь a1.
	p.Spreads = []*model.Spread{
		{ID: "a0", Name: "Титул"},
		{ID: "b1", Photos: []string{"p3", "p4"}},
		{ID: "a1", Name: "Середина"},
		{ID: "b0", Photos: []string{"p0", "p1"}},
		{ID: "a2", Photos: []string{"p10", "p12-2"}, Name: "Оформленный", BgPhoto: &model.Backdrop{Photo: "p11"}},
	}
	// p10 стоит в якоре, p11 — фоном якоря, p12 — копией файла в якоре: свободными не считаются.
	p.Photos["p12-2"] = model.PhotoRef{Path: by["p12"].Path, W: 300, H: 200}
	got := idsOf(AlbumOf(p, feed("p2", "p0", "p1", "p5", "p6", "p3", "p10", "p4", "p7", "p11", "p12", "p13"), nil).infos)
	// Порядок альбома: b1 (p3 p4), b0 (p0 p1); свободные: p2 — в начало (перед ним в ленте
	// фото пула нет), p5 p6 — после p1, p7 — после p4, p13 — после p4 (последнее пул-фото в ленте).
	j := AlbumOf(p, feed("p2", "p0", "p1", "p5", "p6", "p3", "p10", "p4", "p7", "p11", "p12", "p13"), nil)
	want := []string{"p2", "p3", "p4", "p7", "p13", "p0", "p1", "p5", "p6"}
	if !slices.Equal(got, want) {
		t.Fatalf("пул %v, ждали %v", got, want)
	}
	if j.stat.Free != 5 || j.stat.Photos != 4 || j.stat.Spreads != 2 || j.stat.Decor != 3 {
		t.Fatalf("состав %+v", j.stat)
	}
	// Стены: a0 — в начале (0, остаётся первым), a1 — после b1 со своими свободными p7 p13, a2 — в конце.
	var walls []int
	for _, a := range j.anchors {
		walls = append(walls, a.wall)
	}
	if !slices.Equal(walls, []int{0, 5, len(want)}) {
		t.Fatalf("стены %v, ждали [0 5 %d]", walls, len(want))
	}
	if !slices.Equal(BareIDs(p), []string{"b1", "b0"}) {
		t.Fatalf("BareIDs %v", BareIDs(p))
	}
	// Края: лента пуста — пул только альбомный.
	if j0 := AlbumOf(p, nil, nil); len(j0.infos) != 4 || j0.stat.Free != 0 {
		t.Fatalf("без ленты: %d фото, свободных %d", len(j0.infos), j0.stat.Free)
	}
}

// TestAlbumPoolNoBare: голых нет — пул пуст, даже если свободные пришли; план пустой, состав
// якорей виден (окно объясняет «пересобирать нечего»).
func TestAlbumPoolNoBare(t *testing.T) {
	p, ph := albumPhotos(6)
	p.Spreads = []*model.Spread{{ID: "a0", Name: "Титул"}, {ID: "a1"}, {ID: "m", Photos: []string{"p0"}, Grid: []*model.Place{{ID: "g", Photo: "p0"}}}}
	for _, feed := range [][]model.PhotoInfo{nil, ph} {
		ap, err := AlbumOf(p, feed, nil).Plan(model.AutoOpts{Spreads: 3, Auto: true, Album: true})
		want := model.AlbumPlan{Decor: 1, Blank: 1, Manual: 1}
		if err != nil || ap.Photos != 0 || ap.Spreads != 0 || ap.Album == nil || *ap.Album != want {
			t.Fatalf("лента %d: %+v %v, ждали состав %+v", len(feed), ap, err, want)
		}
	}
	if ap, err := AlbumOf(model.New(), nil, nil).Plan(model.AutoOpts{Album: true}); err != nil || ap.Photos != 0 || ap.Album == nil || *ap.Album != (model.AlbumPlan{}) {
		t.Fatalf("пустой проект: %+v %v", ap, err)
	}
}

// TestAlbumPlanNoAnchorsSame: все развороты голые — якорей нет, план побитово как у сборки тех же
// фото (все пути: «ровно», диапазон, «Авто», серии), и подсказка та же, кроме состава.
func TestAlbumPlanNoAnchorsSame(t *testing.T) {
	p, ph := albumPhotos(40)
	for i := 0; i < 40; i += 5 {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprint("b", i), Photos: idsOf(ph[i : i+5])})
	}
	q := model.New()
	for _, f := range ph {
		q.Photos[f.ID] = p.Photos[f.ID]
	}
	for _, o := range []model.AutoOpts{{Spreads: 6, MaxPer: 8}, {Spreads: 7, MinPer: 3, MaxPer: 9}, {Spreads: 5, Auto: true}, {Spreads: 9, Auto: true, Series: true}} {
		want, wantPl, err1 := PlanOf(q, ph, nil).plan(o)
		o.Album = true
		got, gotPl, err2 := AlbumOf(p, ph, nil).plan(o)
		if err1 != nil || err2 != nil || fmt.Sprint(gotPl) != fmt.Sprint(wantPl) {
			t.Fatalf("%+v: план %v, ждали %v (%v %v)", o, gotPl, wantPl, err1, err2)
		}
		if got.Album == nil || got.Album.Spreads != 8 || got.Album.Photos != 40 {
			t.Fatalf("%+v: состав %+v", o, got.Album)
		}
		got.Album = nil
		if got != want {
			t.Fatalf("%+v: подсказка %+v, ждали %+v", o, got, want)
		}
	}
}

// TestAlbumWithoutWalls: Without пересчитывает стены по оставшимся фото; стена не уходит за край
// пула и не обгоняет другую; состав считается до отбора.
func TestAlbumWithoutWalls(t *testing.T) {
	p, ph := albumPhotos(10)
	p.Spreads = []*model.Spread{
		{ID: "b0", Photos: idsOf(ph[:3])}, {ID: "a0", Name: "A"}, {ID: "b1", Photos: idsOf(ph[3:6])}, {ID: "a1", Name: "B"},
		{ID: "a2", Name: "C"}, {ID: "b2", Photos: idsOf(ph[6:])},
	}
	j := AlbumOf(p, ph, nil).Without(map[string]bool{"p1": true, "p4": true, "p5": true, "p9": true})
	var walls []int
	for _, a := range j.anchors {
		walls = append(walls, a.wall)
	}
	if len(j.infos) != 6 || !slices.Equal(walls, []int{2, 3, 3}) {
		t.Fatalf("фото %d, стены %v, ждали 6 и [2 3 3]", len(j.infos), walls)
	}
	if j.stat.Photos != 10 || j.stat.Spreads != 3 {
		t.Fatalf("состав считается до отбора: %+v", j.stat)
	}
}
