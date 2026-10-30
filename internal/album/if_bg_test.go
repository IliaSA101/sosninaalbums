package album_test

// Фон и стикеры агента (151, design §1.4, §1.5): ApplyBackgroundToIf, AddStickersIf, AddStickerAt.

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const ifLibKey = "my:00000000000000ee"

// ifBgProject — n разворотов по два фото; ph(i) = i-е фото фона клиента, lib — фото набора (Lib).
func ifBgProject(n int) (p *model.Project, client func(i int) string, lib string) {
	p = model.New()
	lib = model.LibPhotoID(ifLibKey)
	p.Photos[lib] = model.PhotoRef{Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: ifLibKey}
	client = func(i int) string {
		return library.IDFor(edtest.Abs("cl" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".jpg"))
	}
	for i := range n {
		a, b, c := library.IDFor(edtest.Abs("p"+string(rune('a'+i%26))+string(rune('a'+i/26))+".jpg")), model.NewID(), client(i)
		p.Photos[a] = model.PhotoRef{Path: edtest.Abs(a + ".jpg"), W: 300, H: 200}
		p.Photos[b] = model.PhotoRef{Path: edtest.Abs(b + ".jpg"), W: 300, H: 200}
		p.Photos[c] = model.PhotoRef{Path: edtest.Abs("c" + c + ".jpg"), W: 300, H: 200}
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{a, b}})
	}
	return p, client, lib
}

func ifTargets(scope string, ids ...string) []model.GridTarget {
	out := make([]model.GridTarget, len(ids))
	for i, id := range ids {
		out[i] = model.GridTarget{Spread: id, Scope: scope}
	}
	return out
}

func ifDots(kind string) model.BgSource {
	return model.BgSource{Pattern: &model.Pattern{Kind: kind, Preset: "sa:" + kind}, Color: "#F0E0D0"}
}

// TestBackgroundIf: узор на три разворота одним шагом, итог равен ApplyBackgroundTo на двойнике;
// повтор, чужой ID и обложка — холостые; плохая область и пустой источник — отказ без следа;
// круг Save → Load; отмена по rev; отказы сверки.
func TestBackgroundIf(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "bg.salbum")
	p, _, _ := ifBgProject(4)
	e, twin := edtest.Open(t, p, path), edtest.Open(t, p.Clone(), "")
	ids := make([]string, 0, 3)
	for _, s := range e.View().Spreads[:3] {
		ids = append(ids, s.ID)
	}
	ts := ifTargets("spread", ids...)
	if _, _, err := twin.ApplyBackgroundTo(ts, ifDots("dots")); err != nil {
		t.Fatal(err)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, out, err := e.ApplyBackgroundToIf(ts, ifDots("dots"), false, album.Expect{Rev: rev})
	if err != nil || out.Changed != 3 || len(out.Skipped) != 0 || out.Replaced != 0 || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("ApplyBackgroundToIf: %+v, rev %d → %d, шагов %d, %v", out, rev, v.Rev, edtest.UndoLen(e)-undo, err)
	}
	for i := range ids {
		if edtest.P(e).Spreads[i].Pat == nil || edtest.P(e).Spreads[i].Pat.Kind != edtest.P(twin).Spreads[i].Pat.Kind {
			t.Fatalf("разворот %d: узор не как у ApplyBackgroundTo", i)
		}
	}
	// Холостые: повтор, чужой ID, обложка (ни шага, ни rev).
	s := take(e)
	for name, tg := range map[string][]model.GridTarget{
		"повтор": ts, "чужой ID": ifTargets("spread", "нет-такого"), "обложка": ifTargets("spread", model.CoverID), "пусто": nil,
	} {
		if v, out, err := e.ApplyBackgroundToIf(tg, ifDots("dots"), false, album.Expect{Rev: s.rev}); err != nil || out.Changed != 0 || v.Rev != s.rev {
			t.Fatalf("%s: %+v, rev %d, %v", name, out, v.Rev, err)
		}
		s.same(t, e, name)
	}
	// Отказы без следа.
	if _, _, err := e.ApplyBackgroundToIf(ifTargets("spine", ids[0]), ifDots("dots"), false, album.Expect{Rev: s.rev}); !errors.Is(err, model.ErrScope) {
		t.Fatalf("область: %v", err)
	}
	if _, _, err := e.ApplyBackgroundToIf(ts, model.BgSource{}, false, album.Expect{Rev: s.rev}); !errors.Is(err, model.ErrPattern) {
		t.Fatalf("источник: %v", err)
	}
	s.same(t, e, "отказы")
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || q.Spreads[0].Pat == nil || q.Spreads[3].Pat != nil {
		t.Fatalf("после Save → Load: %v", err)
	}
	if _, err := e.UndoIfRev(album.Expect{Rev: s.rev}); err != nil || edtest.P(e).Spreads[0].Pat != nil {
		t.Fatalf("отмена: %v", err)
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, err := e.ApplyBackgroundToIf(ts, ifDots("stripes"), false, x)
		return err
	})
}

// ifSkipEd — пять разворотов: s0 слева фото клиента, s1 на весь лист фото набора, s2 пуст, s3 справа
// фото клиента, s4 слева фото набора.
func ifSkipEd(t *testing.T) (e *album.Editor, ids []string) {
	t.Helper()
	p, client, lib := ifBgProject(5)
	p.Spreads[0].BgPhotoL = &model.Backdrop{Photo: client(0), W: 100, H: 100}
	p.Spreads[1].BgPhoto = &model.Backdrop{Photo: lib, W: 100, H: 100}
	p.Spreads[3].BgPhotoR = &model.Backdrop{Photo: client(3), W: 100, H: 100}
	p.Spreads[4].BgPhotoL = &model.Backdrop{Photo: lib, W: 100, H: 100}
	e = edtest.Open(t, p, "")
	for _, s := range e.View().Spreads {
		ids = append(ids, s.ID)
	}
	return e, ids
}

// TestBackgroundSkipPhoto: без replace листы с фото клиента не тронуты (ID в Skipped), фото набора
// заменяются без вопроса; с replace — заменено всё; все пропущены — холостая; один шаг на вызов.
func TestBackgroundSkipPhoto(t *testing.T) {
	e, ids := ifSkipEd(t)
	ts := append(ifTargets("left", ids[0], ids[4]), ifTargets("spread", ids[1], ids[2], ids[3])...)
	ts = append(ts, model.GridTarget{Spread: ids[3], Scope: "left"}) // другая сторона листа с фото: не пропуск
	undo := edtest.UndoLen(e)
	_, out, err := e.ApplyBackgroundToIf(ts, ifDots("dots"), false, album.Expect{Rev: edtest.Rev(e)})
	if err != nil || out.Changed != 4 || out.Replaced != 0 || len(out.Skipped) != 2 || out.Skipped[0] != ids[0] || out.Skipped[1] != ids[3] {
		t.Fatalf("без replace: %+v, %v", out, err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали 1", edtest.UndoLen(e)-undo)
	}
	sp := edtest.P(e).Spreads
	if sp[0].BgPhotoL == nil || sp[0].PatL != nil || sp[3].BgPhotoR == nil || sp[3].PatL == nil {
		t.Fatal("фото клиента тронуто или у пропущенного листа появился узор")
	}
	if sp[1].BgPhoto != nil || sp[1].Pat == nil || sp[4].BgPhotoL != nil || sp[4].PatL == nil || sp[2].Pat == nil {
		t.Fatal("фото набора и пустой лист не получили фон")
	}
	// Все цели пропущены — холостая.
	s := take(e)
	if v, out, err := e.ApplyBackgroundToIf(ifTargets("spread", ids[0], ids[3]), ifDots("stripes"), false, album.Expect{Rev: s.rev}); err != nil || out.Changed != 0 || len(out.Skipped) != 2 || v.Rev != s.rev {
		t.Fatalf("все пропущены: %+v, rev %d, %v", out, v.Rev, err)
	}
	s.same(t, e, "все пропущены")
	// С replace: фото клиента уходят, Skipped пуст, Replaced — снятые фото фона.
	e2, ids2 := ifSkipEd(t)
	ts2 := append(ifTargets("left", ids2[0], ids2[4]), ifTargets("spread", ids2[1], ids2[2], ids2[3])...)
	_, out, err = e2.ApplyBackgroundToIf(ts2, ifDots("dots"), true, album.Expect{Rev: edtest.Rev(e2)})
	if err != nil || out.Changed != 5 || out.Replaced != 4 || len(out.Skipped) != 0 {
		t.Fatalf("replace: %+v, %v", out, err)
	}
	if sp := edtest.P(e2).Spreads; sp[0].BgPhotoL != nil || sp[3].BgPhotoR != nil {
		t.Fatal("с replace фото клиента осталось")
	}
}

// 200 целей, половина с фото клиента: пропуск идёт одним проходом по проекту (не копия на цель),
// время в пределах порога; итог определён.
func TestBackgroundSkipPhotoMany(t *testing.T) {
	p, client, _ := ifBgProject(200)
	for i, s := range p.Spreads {
		if i%2 == 0 {
			s.BgPhotoL = &model.Backdrop{Photo: client(i), W: 100, H: 100}
		}
	}
	e := edtest.Open(t, p, "")
	ids := make([]string, 0, 200)
	for _, s := range e.View().Spreads {
		ids = append(ids, s.ID)
	}
	ts := ifTargets("spread", ids...)
	kinds := []string{"dots", "stripes"}
	var out album.BgOutcome
	k := 0
	d := testkit.Fastest(5, func() {
		_, out, _ = e.ApplyBackgroundToIf(ts, ifDots(kinds[k%2]), false, album.Expect{Rev: edtest.Rev(e)})
		k++
	})
	if out.Changed != 100 || len(out.Skipped) != 100 {
		t.Fatalf("изменено %d, пропущено %d, ждали по 100", out.Changed, len(out.Skipped))
	}
	t.Logf("200 целей: %v", d)
	testkit.CostTest(t)
	if lim := testkit.Limit(150 * time.Millisecond); d > lim {
		t.Fatalf("фон на 200 целей %v, порог %v", d, lim)
	}
}

// ifStickerEd — альбом из двух разворотов; art — рисунок звезда.
func ifStickerEd(t *testing.T, path string) (*album.Editor, []string) {
	t.Helper()
	e := edtest.Open(t, model.New(), path)
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	edtest.NewSpread(e, -1, edtest.Photos("c"))
	return e, edtest.SpreadIDs(e.View())
}

var ifStar = model.StickerSrc{Art: &model.Art{Kind: "star"}}

// TestAddStickersIf: пачка из нескольких стикеров на разных листах и углах одним шагом; углы — в
// пределах страницы с отступом, размеры s < m < l; круг Save → Load; отмена по rev; повтор пачки
// добавляет снова (стикер не «холостой»); пустая пачка — холостая.
func TestAddStickersIf(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "st.salbum")
	e, ids := ifStickerEd(t, path)
	puts := []album.StickerPut{
		{Spread: ids[0], Page: 1, Src: ifStar, Anchor: layers.AnchorTopRight, Size: "s"},
		{Spread: ids[0], Page: 0, Src: ifStar, Anchor: layers.AnchorBottomLeft, Size: "l"},
		{Spread: ids[1], Page: 1, Src: ifStar, Anchor: layers.AnchorCenter},
		{Spread: ids[1], Page: 0, Src: ifStar, Anchor: layers.AnchorTopLeft, Size: "m"},
		{Spread: ids[1], Page: 1, Src: ifStar, Anchor: layers.AnchorBottomRight, Size: "m"},
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, got, err := e.AddStickersIf(puts, album.Expect{Rev: rev})
	if err != nil || len(got) != 5 || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("AddStickersIf: ids %d, rev %d → %d, шагов %d, %v", len(got), rev, v.Rev, edtest.UndoLen(e)-undo, err)
	}
	p := edtest.P(e)
	s0, s1 := p.Spreads[0], p.Spreads[1]
	if len(s0.Stickers) != 2 || len(s1.Stickers) != 3 {
		t.Fatalf("стикеров %d и %d", len(s0.Stickers), len(s1.Stickers))
	}
	pg := p.Geo(s0)
	tr, bl := s0.Stickers[0], s0.Stickers[1]
	if tr.W >= bl.W || tr.X+tr.W > pg.Page(1).X+pg.Page(1).W || tr.Y < 0 || tr.X+tr.W/2 < pg.Page(1).X+pg.Page(1).W/2 || tr.Y+tr.H/2 > pg.H/2 {
		t.Fatalf("правый верхний s %+v не в углу правой страницы (страница %+v)", tr, pg.Page(1))
	}
	if bl.X < 0 || bl.Y+bl.H > pg.H || bl.X+bl.W/2 > pg.Page(0).W/2+pg.Page(0).X+bl.W || bl.Y+bl.H/2 < pg.H/2 {
		t.Fatalf("левый нижний l %+v не в углу левой страницы", bl)
	}
	// Угол стоит ровно на отступе max(Safe, Margin) от краёв страницы (не вплотную и не дальше).
	inset := max(p.Settings.Safe, p.Settings.Margin)
	if inset <= 0 {
		t.Fatalf("отступ %v: проверять нечего", inset)
	}
	p0, p1 := pg.Page(0), pg.Page(1)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	if !near(tr.X+tr.W, p1.X+p1.W-inset) || !near(tr.Y, p1.Y+inset) {
		t.Fatalf("правый верхний: (%v;%v) правый край %v, ждали отступ %v от страницы %+v", tr.X, tr.Y, tr.X+tr.W, inset, p1)
	}
	if !near(bl.X, p0.X+inset) || !near(bl.Y+bl.H, p0.Y+p0.H-inset) {
		t.Fatalf("левый нижний: (%v;%v) низ %v, ждали отступ %v от страницы %+v", bl.X, bl.Y, bl.Y+bl.H, inset, p0)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || len(q.Spreads[0].Stickers) != 2 || len(q.Spreads[1].Stickers) != 3 {
		t.Fatalf("после Save → Load: %v", err)
	}
	s := take(e)
	if v, ids2, err := e.AddStickersIf(nil, album.Expect{Rev: s.rev}); err != nil || len(ids2) != 0 || v.Rev != s.rev {
		t.Fatalf("пустая пачка: %v, rev %d", err, v.Rev)
	}
	s.same(t, e, "пустая пачка")
	if _, err := e.UndoIfRev(album.Expect{Rev: s.rev}); err != nil || len(edtest.P(e).Spreads[0].Stickers) != 0 {
		t.Fatalf("отмена: %v", err)
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, err := e.AddStickersIf(puts, x)
		return err
	})
}

// Отказ пачки целиком, ни шага: лист неизвестный или обложка, неизвестный якорь и размер, рисунок
// негодный, сверх MaxStickers на листе и MaxTotalStickers в альбоме — типизированные ошибки.
func TestAddStickersIfRefuse(t *testing.T) {
	e, ids := ifStickerEd(t, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	ok := album.StickerPut{Spread: ids[0], Src: ifStar, Anchor: layers.AnchorCenter}
	bad := func(f func(*album.StickerPut)) album.StickerPut { c := ok; f(&c); return c }
	cases := map[string]struct {
		puts []album.StickerPut
		want error
	}{
		"нет листа": {[]album.StickerPut{ok, bad(func(p *album.StickerPut) { p.Spread = "нет" })}, album.ErrNoSheet},
		"обложка":   {[]album.StickerPut{bad(func(p *album.StickerPut) { p.Spread = model.CoverID })}, album.ErrNoSheet},
		"якорь":     {[]album.StickerPut{ok, bad(func(p *album.StickerPut) { p.Anchor = "левее" })}, layers.ErrAnchor},
		"размер":    {[]album.StickerPut{bad(func(p *album.StickerPut) { p.Size = "xl" })}, layers.ErrAnchor},
		"рисунок":   {[]album.StickerPut{bad(func(p *album.StickerPut) { p.Src = model.StickerSrc{Art: &model.Art{Kind: "Не Ключ!"}} })}, model.ErrSticker},
		"плохой ключ": {[]album.StickerPut{bad(func(p *album.StickerPut) {
			p.Src = model.StickerSrc{Photo: model.PhotoInfo{ID: "x", Lib: "плохой"}}
		})}, model.ErrLibKey},
	}
	for name, c := range cases {
		s := take(e)
		if _, ids, err := e.AddStickersIf(c.puts, album.Expect{Rev: s.rev}); !errors.Is(err, c.want) || ids != nil {
			t.Fatalf("%s: %v (ждали %v), ids %v", name, err, c.want, ids)
		}
		s.same(t, e, name)
	}
	// Предел листа: 64 стикера — ещё один отказывает всей пачке.
	full := make([]album.StickerPut, model.MaxStickers)
	for i := range full {
		full[i] = ok
	}
	if _, _, err := e.AddStickersIf(full, album.Expect{Rev: edtest.Rev(e)}); err != nil {
		t.Fatal(err)
	}
	s := take(e)
	if _, _, err := e.AddStickersIf([]album.StickerPut{ok}, album.Expect{Rev: s.rev}); !errors.Is(err, model.ErrStickers) {
		t.Fatalf("65-й стикер листа: %v", err)
	}
	s.same(t, e, "предел листа")
	// Предел альбома: пачка, из-за которой общее число превысит MaxTotalStickers.
	edtest.Change(e, func(p *model.Project) {
		for i := range p.Spreads {
			for len(p.Spreads[i].Stickers) < model.MaxStickers {
				p.Spreads[i].Stickers = append(p.Spreads[i].Stickers, p.Spreads[0].Stickers[0])
			}
		}
	})
	s = take(e)
	over := make([]album.StickerPut, model.MaxTotalStickers)
	for i := range over {
		over[i] = bad(func(p *album.StickerPut) { p.Spread = ids[1] })
	}
	if _, _, err := e.AddStickersIf(over, album.Expect{Rev: s.rev}); !errors.Is(err, model.ErrStickersTotal) {
		t.Fatalf("сверх альбома: %v", err)
	}
	s.same(t, e, "предел альбома")
}

// TestAddStickerAtCenterSame: AddSticker и AddStickerAt(центр, ¼) ставят побайтно один и тот же
// стикер (с точностью до ID); якорь и доля вне допустимого — ErrAnchor без правки.
func TestAddStickerAtCenterSame(t *testing.T) {
	mk := func() (*model.Project, string) {
		p := model.New()
		s := &model.Spread{ID: model.NewID(), Photos: nil}
		p.Spreads = append(p.Spreads, s)
		return p, s.ID
	}
	for page := range 2 {
		a, ida := mk()
		b, idb := mk()
		if err := layers.AddSticker(a, ida, page, "st", ifStar); err != nil {
			t.Fatal(err)
		}
		if err := layers.AddStickerAt(b, idb, page, "st", ifStar, layers.AnchorCenter, layers.FracM); err != nil {
			t.Fatal(err)
		}
		sa, sb := a.Spreads[0].Stickers, b.Spreads[0].Stickers
		ja, _ := json.Marshal(sa)
		jb, _ := json.Marshal(sb)
		if len(sa) != 1 || len(sb) != 1 || !bytes.Equal(ja, jb) {
			t.Fatalf("страница %d: %+v и %+v", page, sa, sb)
		}
	}
	p, id := mk()
	for _, c := range []struct {
		anchor string
		frac   float64
	}{{"", 0.25}, {"center", 0}, {"center", -1}, {"center", 1.5}, {"center", nan()}} {
		if err := layers.AddStickerAt(p, id, 0, "x", ifStar, c.anchor, c.frac); !errors.Is(err, layers.ErrAnchor) {
			t.Fatalf("%q %v: %v", c.anchor, c.frac, err)
		}
	}
	if len(p.Spreads[0].Stickers) != 0 {
		t.Fatal("отказ оставил стикер")
	}
}

func nan() float64 { var z float64; return z / z }
