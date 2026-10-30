package album_test

import (
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка по макету альбома (131 design §5, G5): серия, шаг 0 ApplyLookStep, CreateCoverLook.

// lkSide — сторона обложки макета: n мест в ряд (доли зоны), цвет страницы (пусто — фон листа).
func lkSide(n int, color string) *model.SpreadTpl {
	places := make([]model.Box, n)
	for i := range n {
		w := 0.9 / float64(n)
		places[i] = lkBox(0.05+w*float64(i), 0.2, w*0.9, 0.6)
	}
	return &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: places, Page: []model.TplBg{{Color: color}}}
}

// lkCoverTpl — макет lookB с обложкой: фон листа, на задней 2 места, на лицевой 3 и свой цвет.
func lkCoverTpl() model.AlbumTpl {
	t := lookB()
	t.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#334455"}, Back: lkSide(2, ""), Front: lkSide(3, "#998877")}
	return t
}

// lkCoverEditor — редактор с чистой обложкой.
func lkCoverEditor(t *testing.T) *album.Editor {
	t.Helper()
	e := lkEditor(t)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	return e
}

// lkSidePlaces — места сторонки scope обложки, мм.
func lkSidePlaces(e *album.Editor, scope string) (out []*model.Place) {
	p := edtest.P(e)
	for _, pl := range p.Cover.Sheet.Grid {
		if c := pl.X + pl.W/2; (scope == model.ScopeLeft) == (c < p.Geo(p.Cover.Sheet).W/2) {
			out = append(out, pl)
		}
	}
	return out
}

// lkZone — безопасная зона сторонки scope.
func lkZone(e *album.Editor, scope string) (x, y, w, h float64) {
	for _, sd := range edtest.P(e).CoverSides() {
		if sd.Scope == scope {
			return sd.Box.X, sd.Box.Y, sd.Box.W, sd.Box.H
		}
	}
	return
}

func lkNear(a, b float64) bool { return math.Abs(a-b) <= 0.011 }

// Серия на альбоме с обложкой: фон листа, места сторонок в безопасной зоне по LookBoxes, корешок цел;
// план равен итогу; повтор — холостая; Undo — один шаг; Save→Load цел.
func TestApplyLookCover(t *testing.T) {
	e := lkCoverEditor(t)
	tpl := lkCoverTpl()
	rev, undo := e.View().Rev, edtest.UndoLen(e)
	want, err := e.PlanLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || e.View().Rev != rev {
		t.Fatalf("PlanLook: %v", err)
	}
	_, plan, err := e.ApplyLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan != want || !plan.Cover {
		t.Fatalf("план %+v, итог %+v, %v", want, plan, err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали один", edtest.UndoLen(e)-undo)
	}
	c := edtest.P(e).Cover.Sheet
	if c.BgL != "#334455" || c.BgR != "#998877" || c.LookStep != 0 {
		t.Fatalf("фон/номер/корешок: %q %q %d", c.BgL, c.BgR, c.LookStep)
	}
	lkCheckSides(t, e, tpl)
	ids, data := lkSidePlaces(e, model.ScopeRight)[0].ID, lkJSON(e)
	rev = e.View().Rev
	if _, again, err := e.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 6}, nil); err != nil || again.Cover || e.View().Rev != rev ||
		lkSidePlaces(e, model.ScopeRight)[0].ID != ids || lkJSON(e) != data {
		t.Fatalf("повтор должен быть холостым: %+v %v", again, err)
	}
	e.Undo()
	if p := edtest.P(e); len(p.Spreads) != 0 || p.Look != nil || len(p.Cover.Sheet.Grid) != 0 || p.Cover.Sheet.BgL != "" {
		t.Fatal("Undo не вернул альбом одним шагом")
	}
}

// Save→Load: оформление обложки (места сторонок, фон) доходит до файла и обратно; повтор на
// загруженном альбоме — холостая правка.
func TestApplyLookCoverSaveLoad(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "cover.salbum")
	e := edtest.Open(t, model.New(), path)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	tpl := lkCoverTpl()
	if _, _, err := e.ApplyLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e2 := edtest.Open(t, p, path)
	if lkJSON(e2) != lkJSON(e) {
		t.Fatal("после Save→Load проект другой")
	}
	rev := e2.View().Rev
	if _, plan, err := e2.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Cover || e2.View().Rev != rev {
		t.Fatalf("повтор после Load: %+v %v", plan, err)
	}
}

// Обложки нет — серия её не создаёт; у макета нет обложки — обложка не трогается.
func TestApplyLookCoverAbsent(t *testing.T) {
	e := lkEditor(t)
	_, plan, err := e.ApplyLook(lkRefB, lkCoverTpl(), nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Cover || edtest.P(e).Cover != nil {
		t.Fatalf("обложки нет: план %+v, обложка %v, %v", plan, edtest.P(e).Cover, err)
	}
	e = lkCoverEditor(t)
	before := lkCoverJSON(e)
	if _, plan, err = e.ApplyLook(lkRefB, lookB(), nil, model.LookOpts{Spreads: 6}, nil); err != nil || plan.Cover ||
		lkCoverJSON(e) != before {
		t.Fatalf("у макета нет обложки: %+v %v", plan, err)
	}
}

// lkCoverJSON — JSON листа обложки.
func lkCoverJSON(e *album.Editor) string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	b, _ := json.Marshal(edtest.P(e).Cover)
	return string(b)
}

// lkCoverFull — обложка макета с предметами: звезда и заготовка на лицевой, узор листа.
func lkCoverFull() model.AlbumTpl {
	t := lkCoverTpl()
	t.Cover.Front.Stickers = []model.TplItem{{Anchor: model.Anchor{AX: 1, AY: 1, DX: -0.05, DY: -0.05}, W: 0.1, H: 0.1, Art: &model.Art{Kind: "star"}}}
	t.Cover.Front.Drafts = []model.TplText{{Anchor: model.Anchor{AY: 1, DY: -0.1}, W: 0.5, Text: "[Имена]", Style: model.PlainStyle()}}
	t.Cover.Bg.Pat = &model.Pattern{Kind: "dots"}
	return t
}

// «Поставить на обложку» (ApplyLookStep, шаг 0): места, стикер и заготовка сторонки, узор листа цел,
// план равен итогу, повтор — холостая (ID те же), макет становится макетом альбома.
func TestApplyLookStepCover(t *testing.T) {
	e := lkCoverEditor(t)
	tpl := lkCoverFull()
	rev, data := e.View().Rev, lkJSON(e)
	want, err := e.PlanLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || e.View().Rev != rev || lkJSON(e) != data {
		t.Fatalf("PlanLookStep: %v, проект изменён", err)
	}
	_, got, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || got != want || miss != 0 {
		t.Fatalf("итог %+v (%d, %v), план %+v", got, miss, err, want)
	}
	p := edtest.P(e)
	c := p.Cover.Sheet
	if len(c.Grid) != 5 || len(c.Stickers) != 1 || len(c.Texts) != 1 || !c.Texts[0].Draft || c.Pat == nil || c.PatL != nil || c.PatR != nil {
		t.Fatalf("места %d, стикеры %d, надписи %d, узор листа %v / %v %v", len(c.Grid), len(c.Stickers), len(c.Texts), c.Pat, c.PatL, c.PatR)
	}
	if x, _, w, _ := lkZone(e, model.ScopeRight); c.Stickers[0].X < x || c.Stickers[0].X+c.Stickers[0].W > x+w {
		t.Fatalf("стикер вне зоны: %.1f", c.Stickers[0].X)
	}
	if p.Look == nil || p.Look.Key != lkKeyB || c.LookStep != 0 {
		t.Fatalf("макет альбома %+v, номер у обложки %d", p.Look, c.LookStep)
	}
	ids, sid, rev, data := lkSidePlaces(e, model.ScopeRight)[0].ID, c.Stickers[0].ID, e.View().Rev, lkJSON(e)
	if _, again, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil || again.Left != 0 ||
		e.View().Rev != rev || lkJSON(e) != data || lkSidePlaces(e, model.ScopeRight)[0].ID != ids || edtest.P(e).Cover.Sheet.Stickers[0].ID != sid {
		t.Fatalf("повтор: %+v %v", again, err)
	}
}

// Шаг 0 и отказы: обложки нет — холостая; у макета нет обложки, шаг на обложку и шаг 0 на развороте —
// ошибка без следа; чужой макет не становится макетом альбома.
func TestApplyLookStepCoverRefusals(t *testing.T) {
	e := lkEditor(t)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverTpl(), nil); err != nil || edtest.P(e).Cover != nil || edtest.P(e).Look != nil {
		t.Fatalf("обложки нет: %v, обложка %v", err, edtest.P(e).Cover)
	}
	e = lkCoverEditor(t)
	sid := edtest.NewSpread(e, 0, nil).Spreads[0].ID
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	for name, call := range map[string]func() error{
		"у макета нет обложки": func() error { _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lookB(), nil); return err },
		"шаг 1 на обложку":     func() error { _, _, _, err := e.ApplyLookStep(model.CoverID, 1, lkRefB, lkCoverTpl(), nil); return err },
		"шаг 0 на развороте":   func() error { _, _, _, err := e.ApplyLookStep(sid, 0, lkRefB, lkCoverTpl(), nil); return err },
	} {
		if err := call(); err == nil || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
			t.Fatalf("%s: ошибка %v, проект изменён", name, err)
		}
	}
	if _, err := e.PlanLookStep(model.CoverID, 0, lkRefB, lookB(), nil); err == nil {
		t.Fatal("PlanLookStep: у макета нет обложки — ждали ошибку")
	}
	// Чужой макет: у альбома уже есть свой — он остаётся.
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverTpl(), nil); err != nil || edtest.P(e).Look.Key != lkKeyA {
		t.Fatalf("макет альбома %+v, %v", edtest.P(e).Look, err)
	}
}

// Фото обложки садятся в места по порядку, лишние уходят в ленту (Left); шаг и план сходятся; Undo
// возвращает фото в места одним шагом.
func TestApplyLookStepCoverPhotos(t *testing.T) {
	e := lkCoverEditor(t)
	frac := []model.Box{lkBox(0, 0, 0.5, 0.5), lkBox(0.5, 0, 0.5, 0.5), lkBox(0, 0.5, 0.5, 0.5), lkBox(0.5, 0.5, 0.5, 0.5)}
	if _, _, err := e.ApplyCoverGrid(model.ScopeRight, frac); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { // фото в места лицевой по порядку
		c := p.Cover.Sheet
		for i, pl := range lkSidePlaces(e, model.ScopeRight) {
			id := string(rune('a' + i))
			p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: 300, H: 200}
			pl.Photo = id
			c.Photos = append(c.Photos, id)
		}
	})
	if n := len(lkSidePlaces(e, model.ScopeRight)); n != 4 {
		t.Fatalf("мест до шага %d", n)
	}
	tpl := lkCoverTpl()
	tpl.Cover.Front = lkSide(2, "")
	want, err := e.PlanLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	_, got, _, err2 := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || err2 != nil || got != want || got.Left != 2 {
		t.Fatalf("итог %+v (%v), план %+v (%v), ждали 2 фото в ленту", got, err2, want, err)
	}
	pl := lkSidePlaces(e, model.ScopeRight)
	if len(pl) != 2 || pl[0].Photo != "a" || pl[1].Photo != "b" {
		t.Fatalf("фото по порядку: %+v", pl)
	}
	e.Undo()
	if n := len(lkSidePlaces(e, model.ScopeRight)); n != 4 {
		t.Fatalf("после Undo мест %d, ждали 4", n)
	}
}

// Корешок не трогается: надпись, стикер и фото корешка — как были.
func TestApplyLookCoverKeepsSpine(t *testing.T) {
	e := lkCoverEditor(t)
	if _, err := e.SetCoverLayout(model.CoverBoth); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetBackdrop(model.CoverID, model.ScopeSpine, edtest.Photo("sp")); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) {
		a := p.BdArea(p.Cover.Sheet, model.ScopeSpine)
		p.Cover.Sheet.Texts = append(p.Cover.Sheet.Texts, &model.TextBlock{ID: "c0ffee0000a1", X: a.X + a.W/2 - 5, Y: 50, W: 10, Text: "корешок", Style: model.PlainStyle()})
		p.Cover.Sheet.Stickers = append(p.Cover.Sheet.Stickers, &model.Sticker{ID: "c0ffee0000a2", X: a.X + a.W/2 - 2, Y: 120, W: 4, H: 4, Art: &model.Art{Kind: "star"}})
	})
	spine := func() string {
		txt := lkTextJSON(e, "c0ffee0000a1")
		edtest.Mu(e).Lock()
		defer edtest.Mu(e).Unlock()
		c := edtest.P(e).Cover.Sheet
		b, _ := json.Marshal([]any{c.BgPhotoS, c.Stickers[0]})
		return txt + string(b)
	}
	before := spine()
	if edtest.P(e).Cover.Sheet.BgPhotoS == nil {
		t.Fatal("фото корешка не встало")
	}
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverFull(), nil); err != nil {
		t.Fatal(err)
	}
	if after := spine(); after != before {
		t.Fatalf("корешок изменился:\n%s\n%s", before, after)
	}
}

func lkTextJSON(e *album.Editor, id string) string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	for _, tx := range edtest.P(e).Cover.Sheet.Texts {
		if tx.ID == id {
			b, _ := json.Marshal(tx)
			return string(b)
		}
	}
	return ""
}

// CreateCoverLook: у альбома найден макет — обложка получает его оформление, затем фото броска (оно
// главнее макета: встаёт в первое место лицевой, 145; фоном его нет); один шаг отмены; повтор
// Undo+CreateCover без макета — чистая, фото фоном лицевой.
func TestCreateCoverWithLook(t *testing.T) {
	tpl := lkCoverFull()
	e := lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	// Обложки не было: серия её не создала.
	if edtest.P(e).Cover != nil {
		t.Fatal("серия создала обложку")
	}
	ph := edtest.Photo("bg1")
	lkHost(e, tpl)
	undo := edtest.UndoLen(e)
	_, styled, err := e.CreateCoverLook("", &ph, nil, e.Look())
	if err != nil || styled == nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("styled %v, шагов %d, %v", styled != nil, edtest.UndoLen(e)-undo, err)
	}
	c := edtest.P(e).Cover.Sheet
	if len(c.Grid) != 5 || c.BgL != "#334455" || c.BgR != "#998877" || len(c.Stickers) != 1 || c.BgPhotoR != nil {
		t.Fatalf("оформление: мест %d, фон %q %q, стикеров %d, фото фоном лицевой %v", len(c.Grid), c.BgL, c.BgR, len(c.Stickers), c.BgPhotoR)
	}
	if seats := edtest.P(e).CoverSeats(); seats[0].Photo != "bg1" {
		t.Fatalf("фото не в первом месте лицевой: %q", seats[0].Photo)
	}
	// «Без оформления»: Undo + обычная CreateCover.
	e.Undo()
	if edtest.P(e).Cover != nil {
		t.Fatal("Undo не снял обложку")
	}
	if _, err := e.CreateCover("", &ph); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Sheet; len(c.Grid) != 0 || c.BgL != "" || c.BgPhotoR == nil {
		t.Fatalf("без оформления: %+v", c)
	}
}

// CreateCoverLook без оформления: макета у альбома нет, не найден, нет обложки у макета, поиск не подключён —
// обычная обложка, styled = false.
func TestCreateCoverWithLookNone(t *testing.T) {
	notFound := func(model.LookRef) *model.AlbumTpl { return nil }
	cases := map[string]func(*testing.T) (*album.Editor, album.LookFinder){
		"макета нет у альбома": func(t *testing.T) (*album.Editor, album.LookFinder) { return lkEditor(t), lkPrev(lkCoverFull()) },
		"макет не найден": func(t *testing.T) (*album.Editor, album.LookFinder) {
			e := lkEditor(t)
			lkApply(t, e, lkRefB, lkCoverFull(), nil, 6)
			return e, notFound
		},
		"у макета нет обложки": func(t *testing.T) (*album.Editor, album.LookFinder) {
			e := lkEditor(t)
			lkApply(t, e, lkRefB, lookB(), nil, 6)
			return e, lkPrev(lookB())
		},
		"поиск не подключён": func(t *testing.T) (*album.Editor, album.LookFinder) {
			e := lkEditor(t)
			lkApply(t, e, lkRefB, lkCoverFull(), nil, 6)
			return e, nil
		},
	}
	for name, mk := range cases {
		e, find := mk(t)
		if find != nil {
			e.SetLookHost(album.LookHost{Find: find})
		}
		if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled != nil {
			t.Fatalf("%s: styled %v, %v", name, styled != nil, err)
		}
		if c := edtest.P(e).Cover; c == nil || len(c.Sheet.Grid) != 0 || c.Sheet.BgL != "" {
			t.Fatalf("%s: обложка не чистая: %+v", name, c)
		}
	}
}

// Лист 031 и отказы: у проекта-листа обложки нет — отказ без следа; обложка уже есть, неизвестный
// пресет — то же; ApplyLookStep на листе 031 — холостая.
func TestApplyLookSheet031(t *testing.T) {
	tpl := lkCoverFull()
	e := edtest.Open(t, model.NewSheet(204, 304), "")
	lkHost(e, tpl)
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err == nil || styled != nil {
		t.Fatalf("лист 031: ждали отказ, styled %v", styled != nil)
	}
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
		t.Fatalf("шаг 0 на листе 031: %v", err)
	}
	if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data || edtest.P(e).Cover != nil {
		t.Fatal("лист 031: проект изменился")
	}
	e = lkCoverEditor(t)
	lkHost(e, tpl)
	rev, data = e.View().Rev, lkJSON(e)
	if _, _, err := e.CreateCoverLook("", nil, nil, e.Look()); err == nil || e.View().Rev != rev || lkJSON(e) != data {
		t.Fatalf("обложка уже есть: ошибка %v", err)
	}
	if _, _, err := e.CreateCoverLook("нет-такого", nil, nil, e.Look()); err == nil || lkJSON(e) != data {
		t.Fatalf("неизвестный пресет: ошибка %v", err)
	}
}

// Отказ в середине оформления (стикеров больше предела) откатывает всё: ни фона, ни мест, ни шага.
func TestApplyLookCoverRefusalAtomic(t *testing.T) {
	e := lkCoverEditor(t)
	edtest.Change(e, func(p *model.Project) { // стикеры на корешке: оформление сторонок их не снимает
		a := p.BdArea(p.Cover.Sheet, model.ScopeSpine)
		for i := range model.MaxStickers {
			p.Cover.Sheet.Stickers = append(p.Cover.Sheet.Stickers, &model.Sticker{ID: model.NewID(), X: a.X + a.W/2 - 1, Y: 10 + float64(i), W: 2, H: 2, Art: &model.Art{Kind: "star"}})
		}
	})
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverFull(), nil); err == nil {
		t.Fatal("ждали отказ: стикеров больше предела")
	}
	if _, _, err := e.ApplyLook(lkRefB, lkCoverFull(), nil, model.LookOpts{Spreads: 6}, nil); err == nil {
		t.Fatal("серия: ждали отказ")
	}
	if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatal("отказ оставил след")
	}
}

// Серия и свой фон обложки: заменяется — Replaced (вопрос), фон прежнего макета — нет.
func TestApplyLookCoverReplaced(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverFull()
	e := lkCoverEditor(t)
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	plan, err := e.PlanLook(lkRefB, a, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Replaced != 1 || !plan.Cover {
		t.Fatalf("свой фон: %+v %v", plan, err)
	}
	lkApply(t, e, lkRefB, a, nil, 6)
	plan, err = e.PlanLook(lkRefA, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Replaced != 0 || !plan.Cover {
		t.Fatalf("фон прежнего макета: %+v %v", plan, err)
	}
}

// Картинка библиотеки в обложке макета (фон листа и стикер лицевой): файла на ПК нет — слой пропущен и
// посчитан (Missing в шаге 0), файл есть — встала; серия считает ключи в plan.Missing.
func TestApplyLookCoverLibMissing(t *testing.T) {
	const key = "my:00000000000000bb"
	tpl := lkCoverFull()
	tpl.Cover.Bg.Lib = key
	tpl.Cover.Front.Stickers = append(tpl.Cover.Front.Stickers, model.TplItem{W: 0.1, H: 0.1, Lib: key})
	e := lkCoverEditor(t)
	plan, err := e.PlanLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Missing != 1 || !plan.Cover {
		t.Fatalf("серия без файла: %+v %v", plan, err)
	}
	_, _, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || miss != 1 || edtest.P(e).Cover.Sheet.BgPhoto != nil {
		t.Fatalf("шаг 0 без файла: %d, %v", miss, err)
	}
	libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}}
	if _, _, miss, err = e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, libs); err != nil || miss != 0 {
		t.Fatalf("шаг 0 с файлом: %d, %v", miss, err)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgPhoto == nil || len(c.Stickers) != 2 {
		t.Fatalf("фото фона %v, стикеров %d", c.BgPhoto, len(c.Stickers))
	}
}

// lkNoTrace — вызов call отказал, а проект, rev и отмена прежние.
func lkNoTrace(t *testing.T, e *album.Editor, name string, call func() error) error {
	t.Helper()
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	err := call()
	if err == nil || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatalf("%s: ошибка %v, проект изменён", name, err)
	}
	return err
}

// Поиск прежнего макета вернул nil на живом Project.Look: все макетные развороты тронуты, ошибки нет;
// негодный макет в CreateCoverLook (пустой) — обложка без оформления.
func TestApplyLookFinderNil(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	gone := func(model.LookRef) *model.AlbumTpl { return nil }
	plan, err := e.PlanLook(lkRefB, lookB(), gone, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 9 {
		t.Fatalf("тронутых %d, ждали 9 (%v)", plan.Touched, err)
	}
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &model.AlbumTpl{} }})
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled != nil || len(edtest.P(e).Cover.Sheet.Grid) != 0 {
		t.Fatalf("пустой макет: styled %v, %v", styled != nil, err)
	}
}

// Фон обложки — только картинка библиотеки (без узора): без файла слой пропущен и посчитан, с файлом встал.
func TestApplyLookCoverBgLib(t *testing.T) {
	const key = "my:00000000000000cc"
	tpl := lkCoverTpl()
	tpl.Cover.Bg = &model.TplBg{Lib: key}
	e := lkCoverEditor(t)
	if _, _, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil || miss != 1 || edtest.P(e).Cover.Sheet.BgPhoto != nil {
		t.Fatalf("без файла: %d %v", miss, err)
	}
	libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("l.jpg"), W: 300, H: 200, Lib: key}}
	if _, _, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, libs); err != nil || miss != 0 || edtest.P(e).Cover.Sheet.BgPhoto == nil {
		t.Fatalf("с файлом: %d %v", miss, err)
	}
	// Фото листа остаётся под сторонками: повтор холостой, ID мест прежние.
	id, rev := lkSidePlaces(e, model.ScopeRight)[0].ID, e.View().Rev
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, libs); err != nil || e.View().Rev != rev || lkSidePlaces(e, model.ScopeRight)[0].ID != id || edtest.P(e).Cover.Sheet.BgPhoto == nil {
		t.Fatalf("повтор с фото листа: %v, rev %d → %d", err, rev, e.View().Rev)
	}
}

// Макет без фона листа, но с цветом лицевой: фон обложки целиком задаёт макет (цвет задней без
// своего — пустой), свой прежний цвет — Replaced; повтор холостой.
func TestApplyLookCoverSideColorOnly(t *testing.T) {
	tpl := lkCoverTpl()
	tpl.Cover.Bg = nil
	e := lkCoverEditor(t)
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	plan, err := e.PlanLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Replaced != 1 || !plan.Cover {
		t.Fatalf("план %+v %v", plan, err)
	}
	lkApply(t, e, lkRefB, tpl, nil, 6)
	if c := edtest.P(e).Cover.Sheet; c.BgR != "#998877" || c.BgL != "" {
		t.Fatalf("цвета: задняя %q, лицевая %q", c.BgL, c.BgR)
	}
	rev := e.View().Rev
	if _, again, err := e.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 6}, nil); err != nil || again.Cover || e.View().Rev != rev {
		t.Fatalf("повтор: %+v %v", again, err)
	}
}

// Мест на другой сторонке столько, что новые не влезают в MaxPlaces: ошибка из applyGrid, след — никакого.
func TestApplyLookCoverPlacesOverflow(t *testing.T) {
	e := lkCoverEditor(t)
	edtest.Change(e, func(p *model.Project) {
		a := p.BdArea(p.Cover.Sheet, model.ScopeSpine) // места на корешке: сторонки их не заменяют
		for i := range model.MaxPlaces - 1 {
			p.Cover.Sheet.Grid = append(p.Cover.Sheet.Grid, &model.Place{ID: model.NewPlaceID(map[string]bool{}), X: a.X + a.W/2 - 2, Y: 5 + float64(i)*4, W: 4, H: 3})
		}
	})
	lkNoTrace(t, e, "места", func() error { _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverTpl(), nil); return err })
}

// Стикеры вне сторонок (на корешке) заполняют предел: стикер лицевой не ставится, ошибка — ErrStickers.
func TestApplyLookCoverStickersOverflow(t *testing.T) {
	e := lkCoverEditor(t)
	edtest.Change(e, func(p *model.Project) {
		a := p.BdArea(p.Cover.Sheet, model.ScopeSpine)
		for i := range model.MaxStickers {
			p.Cover.Sheet.Stickers = append(p.Cover.Sheet.Stickers, &model.Sticker{ID: model.NewID(), X: a.X + a.W/2 - 5, Y: 10 + float64(i), W: 10, H: 10, Art: &model.Art{Kind: "star"}})
		}
	})
	if n := len(edtest.P(e).Cover.Sheet.Stickers); n != model.MaxStickers {
		t.Fatalf("стикеров на корешке %d", n)
	}
	err := lkNoTrace(t, e, "стикеры", func() error {
		_, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverFull(), nil)
		return err
	})
	if !errors.Is(err, model.ErrStickers) {
		t.Fatalf("ждали ErrStickers, получили %v", err)
	}
}

// Тронутые места (поворот/«Вписать»/рамочка) делают повтор правкой: места ставятся заново.
func TestApplyLookCoverTouchedPlaces(t *testing.T) {
	tpl := lkCoverTpl()
	for name, touch := range map[string]func(pl *model.Place){
		"Вписать": func(pl *model.Place) { pl.Fit = model.FitContain },
		"рамочка": func(pl *model.Place) { pl.Border = &model.Border{Kind: "line", Width: 1} },
		"поворот": func(pl *model.Place) { pl.Rot = 5 },
	} {
		e := lkCoverEditor(t)
		if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
			t.Fatalf("%s: первый шаг: %v", name, err)
		}
		edtest.Change(e, func(p *model.Project) { touch(lkSidePlaces(e, model.ScopeRight)[0]) })
		rev := e.View().Rev
		if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil || e.View().Rev == rev {
			t.Fatalf("%s: повтор должен вернуть места (%v)", name, err)
		}
		if pl := lkSidePlaces(e, model.ScopeRight)[0]; pl.Fit != "" || pl.Border != nil || pl.Rot != 0 {
			t.Fatalf("%s: место не вернулось: %+v", name, pl)
		}
	}
}

// Вырожденная зона: поля больше сторонки — зоны нет, оформление мест не ставит и не падает.
func TestApplyLookCoverNoZone(t *testing.T) {
	e := lkCoverEditor(t)
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 1000, 1000 })
	if n := len(edtest.P(e).CoverSides()); n != 0 {
		t.Fatalf("зоны не вырождены: %d", n)
	}
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverTpl(), nil); err != nil || len(edtest.P(e).Cover.Sheet.Grid) != 0 {
		t.Fatalf("без зоны: %v, мест %d", err, len(edtest.P(e).Cover.Sheet.Grid))
	}
}

// lkHost подключает к редактору поиск макета: любая запись находит tpl.
func lkHost(e *album.Editor, tpl model.AlbumTpl) {
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &tpl }})
}

// lkCheckSides — места обеих сторонок равны LookBoxes макета в безопасной зоне ± 0,01 мм.
func lkCheckSides(t *testing.T, e *album.Editor, tpl model.AlbumTpl) {
	t.Helper()
	for scope, side := range map[string]*model.SpreadTpl{model.ScopeLeft: tpl.Cover.Back, model.ScopeRight: tpl.Cover.Front} {
		x, y, w, h := lkZone(e, scope)
		got := lkSidePlaces(e, scope)
		boxes := model.LookBoxes(*side, w/h)
		if len(got) != len(boxes) {
			t.Fatalf("%s: мест %d, ждали %d", scope, len(got), len(boxes))
		}
		for i, b := range boxes {
			if pl := got[i]; !lkNear(pl.X, x+b.X*w) || !lkNear(pl.Y, y+b.Y*h) || !lkNear(pl.W, b.W*w) || !lkNear(pl.H, b.H*h) {
				t.Fatalf("%s место %d: %.2f %.2f %.2f %.2f, ждали в зоне %.2f %.2f %.2f %.2f", scope, i, pl.X, pl.Y, pl.W, pl.H,
					x+b.X*w, y+b.Y*h, b.W*w, b.H*h)
			}
		}
	}
}

// lkOwnFront кладёт на лицевую свой стикер и свою надпись (вне корешка, центр в зоне лицевой).
func lkOwnFront(e *album.Editor) {
	edtest.Change(e, func(p *model.Project) {
		x, y, w, h := lkZone(e, model.ScopeRight)
		c := p.Cover.Sheet
		c.Stickers = append(c.Stickers, &model.Sticker{ID: model.NewID(), X: x + w/2, Y: y + 5, W: 10, H: 10, Art: &model.Art{Kind: "star"}})
		c.Texts = append(c.Texts, &model.TextBlock{ID: "c0ffee0000b1", X: x + 5, Y: y + h/2, W: 40, Text: "своя", Style: model.PlainStyle()})
	})
}

// «Поставить на обложку»: план называет заменяемые надпись и стикер лицевой (вопрос до правки), итог тот же.
func TestApplyLookStepCoverPlanReplaced(t *testing.T) {
	e := lkCoverEditor(t)
	lkOwnFront(e)
	tpl := lkCoverFull()
	want, err := e.PlanLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || want.Texts != 1 || want.Stickers != 1 {
		t.Fatalf("план %+v (%v), ждали надпись 1 и стикер 1", want, err)
	}
	_, got, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || got != want {
		t.Fatalf("итог %+v (%v), план %+v", got, err, want)
	}
	if c := edtest.P(e).Cover.Sheet; len(c.Texts) != 1 || !c.Texts[0].Draft || len(c.Stickers) != 1 {
		t.Fatalf("свои не заменились: надписи %d, стикеры %d", len(c.Texts), len(c.Stickers))
	}
}

// Фон обложки в Replaced — только поставленный не прежним макетом: сдвинутое место фон «своим» не делает.
func TestApplyLookCoverOwnBgOnly(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverTpl()
	b.Cover.Bg = &model.TplBg{Color: "#112233"}
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefA, a, nil, 6)
	edtest.Change(e, func(p *model.Project) { lkSidePlaces(e, model.ScopeRight)[0].X += 7 })
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Replaced != 0 || plan.Touched != 1 || !plan.Cover {
		t.Fatalf("фон от прежнего макета, место сдвинуто: %+v %v", plan, err)
	}
}

// Свои места, надписи и стикеры сторонок, которые заменятся, — тронутое; обложка прежнего макета — нет.
func TestApplyLookCoverTouched(t *testing.T) {
	tpl := lkCoverFull()
	e := lkCoverEditor(t)
	lkOwnFront(e)
	plan, err := e.PlanLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 1 {
		t.Fatalf("своя обложка: %+v %v", plan, err)
	}
	e = lkCoverEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	if plan, err = e.PlanLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 0 || plan.Cover {
		t.Fatalf("обложка прежнего макета: %+v %v", plan, err)
	}
	next := lkCoverFull()
	next.Cover.Bg.Pat = &model.Pattern{Kind: "stripes"}
	if plan, err = e.PlanLook(lkRefA, next, lkPrev(tpl), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 0 || !plan.Cover {
		t.Fatalf("смена макета: %+v %v", plan, err)
	}
	e = lkCoverEditor(t) // чистая обложка без своего
	if plan, err = e.PlanLook(lkRefB, tpl, nil, model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 0 {
		t.Fatalf("чистая обложка: %+v %v", plan, err)
	}
}

// Только фон листа различается — обложка меняется (plan.Cover), цвета сторон идут за новым фоном.
func TestApplyLookCoverBgOnlyDiff(t *testing.T) {
	a, b := lookB(), lookB()
	a.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#334455"}}
	b.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#445566"}}
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefA, a, nil, 6)
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || !plan.Cover {
		t.Fatalf("план %+v %v", plan, err)
	}
	if _, _, err = e.ApplyLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgL != "#445566" || c.BgR != "#445566" {
		t.Fatalf("фон %q %q", c.BgL, c.BgR)
	}
}

// Сторона без мест (одни предметы) места не трогает: свои остаются, повтор холостой.
func TestApplyLookCoverSideWithoutPlaces(t *testing.T) {
	tpl := lkCoverFull()
	tpl.Cover.Front.Places, tpl.Cover.Front.Borders = nil, nil
	e := lkCoverEditor(t)
	if _, _, err := e.ApplyCoverGrid(model.ScopeRight, []model.Box{lkBox(0.1, 0.1, 0.4, 0.4), lkBox(0.5, 0.5, 0.4, 0.4)}); err != nil {
		t.Fatal(err)
	}
	ids := []string{lkSidePlaces(e, model.ScopeRight)[0].ID, lkSidePlaces(e, model.ScopeRight)[1].ID}
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
		t.Fatal(err)
	}
	pl := lkSidePlaces(e, model.ScopeRight)
	if len(pl) != 2 || pl[0].ID != ids[0] || pl[1].ID != ids[1] || len(edtest.P(e).Cover.Sheet.Stickers) != 1 {
		t.Fatalf("места сторонки изменились: %+v", pl)
	}
	rev := e.View().Rev
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil || e.View().Rev != rev {
		t.Fatalf("повтор: %v, rev %d → %d", err, rev, e.View().Rev)
	}
}

// Стикер «под местами» шаблона лицевой встаёт под местами, второй — над ними, заготовка — сверху.
func TestApplyLookCoverLayerOrder(t *testing.T) {
	tpl := lkCoverFull()
	tpl.Cover.Back = nil
	under := model.TplItem{W: 0.2, H: 0.2, Under: true, Art: &model.Art{Kind: "star"}}
	tpl.Cover.Front.Stickers = append([]model.TplItem{under}, tpl.Cover.Front.Stickers...)
	e := lkCoverEditor(t)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
		t.Fatal(err)
	}
	c := edtest.P(e).Cover.Sheet
	kinds := c.Kinds()
	var got []string
	for _, id := range c.Stack() {
		got = append(got, kinds[id])
	}
	want := []string{model.LayerSticker, model.LayerPlace, model.LayerPlace, model.LayerPlace, model.LayerSticker, model.LayerText}
	if len(got) != len(want) {
		t.Fatalf("стек %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("стек %v, ждали %v", got, want)
		}
	}
}

// Фон обложки в режиме «Фоны по серии» знает цвет стороны макета: лицевая получает свой цвет, повтор
// холостой, фон прежнего макета «своим» не считается.
func TestApplyLookBgCoverSideColor(t *testing.T) {
	a := lkCoverTpl()
	e := lkBuilt(t, 2)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	o := model.LookOpts{Mode: model.LookBg}
	plan := lkPlanned(t, e, lkRefA, a, nil, o)
	if !plan.Cover {
		t.Fatalf("план %+v", plan)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgL != "#334455" || c.BgR != "#998877" || len(c.Grid) != 0 {
		t.Fatalf("фон %q %q, мест %d (режим bg места не ставит)", c.BgL, c.BgR, len(c.Grid))
	}
	rev := e.View().Rev
	if _, again, err := e.ApplyLook(lkRefA, a, lkPrev(a), o, nil); err != nil || again.Cover || again.Replaced != 0 || e.View().Rev != rev {
		t.Fatalf("повтор: %+v %v", again, err)
	}
	b := lkCoverTpl()
	b.Cover.Front = lkSide(1, "#aa0000")
	if plan, _ = e.PlanLook(lkRefB, b, lkPrev(a), o, nil); plan.Replaced != 0 || !plan.Cover {
		t.Fatalf("смена цвета лицевой: %+v", plan)
	}
}

// Узор и фото стороны: если у фона листа их нет — встают на сторону, если есть — сторона добавляет
// только цвет, ключ её картинки не считается (решение 135).
func TestApplyLookCoverSideBgLib(t *testing.T) {
	const key = "my:00000000000000dd"
	libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("s.jpg"), W: 300, H: 200, Lib: key}}
	tpl := lkCoverTpl()
	tpl.Cover.Front.Page[0] = model.TplBg{Color: "#998877", Lib: key}
	e := lkCoverEditor(t)
	_, _, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil)
	if err != nil || miss != 1 || edtest.P(e).Cover.Sheet.BgPhotoR != nil {
		t.Fatalf("без файла: %d %v", miss, err)
	}
	if _, _, miss, err = e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, libs); err != nil || miss != 0 {
		t.Fatalf("с файлом: %d %v", miss, err)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgPhotoR == nil || c.BgPhotoL != nil {
		t.Fatalf("фото стороны: лицевая %v, задняя %v", c.BgPhotoR, c.BgPhotoL)
	}
	rev := e.View().Rev
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, libs); err != nil || e.View().Rev != rev {
		t.Fatalf("повтор: %v", err)
	}
	// Узор у фона листа есть: картинка стороны не ставится и не считается.
	tpl.Cover.Bg.Pat = &model.Pattern{Kind: "dots"}
	e = lkCoverEditor(t)
	if _, _, miss, err = e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil || miss != 0 || edtest.P(e).Cover.Sheet.BgPhotoR != nil {
		t.Fatalf("фон листа с узором: miss %d, %v", miss, err)
	}
}

// Серия 6 → 9 при оформленной обложке: корешок вырос, места сторонок по-прежнему по макету,
// обложка не меняется, повтор холостой.
func TestApplyLookCoverSeriesGrows(t *testing.T) {
	tpl := lkCoverTpl()
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	plan, err := e.PlanLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 9}, nil)
	if err != nil || plan.Cover || plan.Touched != 0 || plan.Replaced != 0 {
		t.Fatalf("план %+v %v", plan, err)
	}
	if _, _, err = e.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 9}, nil); err != nil {
		t.Fatal(err)
	}
	lkCheckSides(t, e, tpl)
	rev := e.View().Rev
	if _, again, err := e.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Spreads: 9}, nil); err != nil || again.Cover || e.View().Rev != rev {
		t.Fatalf("повтор: %+v %v", again, err)
	}
}

// lkSideTexts — лицевая шаблона: места и (если draft) заготовка.
func lkSideTexts(places int, draft bool) *model.SpreadTpl {
	t := lkSide(places, "")
	if draft {
		t.Drafts = []model.TplText{{Anchor: model.Anchor{AY: 1, DY: -0.1}, W: 0.5, Text: "[Имена]", Style: model.PlainStyle()}}
	}
	return t
}

// lkOwnText кладёт на сторонку scope свою надпись.
func lkOwnText(e *album.Editor, scope string) {
	edtest.Change(e, func(p *model.Project) {
		x, y, w, h := lkZone(e, scope)
		p.Cover.Sheet.Texts = append(p.Cover.Sheet.Texts, &model.TextBlock{ID: "c0ffee0000b2", X: x + w/4, Y: y + h/2, W: 40, Text: "моя", Style: model.PlainStyle()})
	})
}

// Н1: прежний макет не ставил надписей на сторонку, фотограф добавил свою, новый макет с заготовкой её
// снимет — это тронутое (вопрос); без своей надписи вопроса нет. То же для своих мест, которых у
// прежнего шаблона не было. Своя надпись на стороне без надписей в шаблоне остаётся (повтор).
func TestApplyLookCoverExtrasTouched(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverTpl()
	a.Cover.Front = lkSideTexts(3, false)
	b.Cover.Front = lkSideTexts(3, true)
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefA, a, nil, 6)
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 0 || !plan.Cover {
		t.Fatalf("без своего: %+v %v", plan, err)
	}
	lkOwnText(e, model.ScopeRight)
	if plan, err = e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 1 {
		t.Fatalf("своя надпись: %+v %v", plan, err)
	}
	// Повтор прежнего макета надпись не трогает.
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefA, a, nil); err != nil || lkTextJSON(e, "c0ffee0000b2") == "" {
		t.Fatalf("своя надпись должна остаться: %v", err)
	}
	// Места: у прежнего шаблона лицевой мест не было, у нового есть.
	a2, b2 := lkCoverTpl(), lkCoverTpl()
	a2.Cover.Front = lkSideTexts(0, true)
	b2.Cover.Front = lkSideTexts(2, true)
	e = lkCoverEditor(t)
	lkApply(t, e, lkRefA, a2, nil, 6)
	if _, _, err := e.ApplyCoverGrid(model.ScopeRight, []model.Box{lkBox(0.1, 0.1, 0.4, 0.4)}); err != nil {
		t.Fatal(err)
	}
	if plan, err = e.PlanLook(lkRefB, b2, lkPrev(a2), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 1 {
		t.Fatalf("свои места: %+v %v", plan, err)
	}
}

// Н2: у прежнего макета фона обложки нет — любой фон на ней свой, замена его Replaced.
func TestApplyLookCoverPrevWithoutBg(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverTpl()
	a.Cover.Bg, a.Cover.Back, a.Cover.Front = nil, lkSide(2, ""), lkSide(3, "")
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefA, a, nil, 6)
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Replaced != 1 {
		t.Fatalf("серия: %+v %v", plan, err)
	}
	// То же в режиме bg на собранном альбоме.
	e = lkBuilt(t, 2)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	o := model.LookOpts{Mode: model.LookBg}
	base, err := e.PlanLook(lkRefB, b, lkPrev(a), o, nil) // Replaced разворотов без своего фона обложки
	if err != nil {
		t.Fatal(err)
	}
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	if plan, err = e.PlanLook(lkRefB, b, lkPrev(a), o, nil); err != nil || plan.Replaced != base.Replaced+1 {
		t.Fatalf("bg: %+v %v, без своего фона обложки %d", plan, err, base.Replaced)
	}
}

// CreateCoverLook: макет, который ничего не меняет, — styled false; отказ после оформления (фото броска с
// негодным ключом) откатывает всё, styled false, обложки нет.
func TestCreateCoverLookStyled(t *testing.T) {
	const key = "my:00000000000000ff"
	nothing := lkCoverTpl()
	nothing.Cover = &model.CoverTpl{Front: &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}},
		Stickers: []model.TplItem{{W: 0.1, H: 0.1, Lib: key}}}} // картинки нет на ПК — предмета нет
	e := lkEditor(t)
	lkApply(t, e, lkRefB, nothing, nil, 6)
	lkHost(e, nothing)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled != nil || edtest.P(e).Cover == nil {
		t.Fatalf("макет без изменений: styled %v, %v", styled != nil, err)
	}
	tpl := lkCoverTpl()
	e = lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	bad := model.PhotoInfo{ID: "bad", Path: edtest.Abs("bad.jpg"), W: 300, H: 200, Lib: "плохой ключ"}
	lkNoTrace(t, e, "фото броска", func() error {
		_, styled, err := e.CreateCoverLook("", &bad, nil, e.Look())
		if styled != nil {
			t.Fatal("styled при ошибке")
		}
		return err
	})
	if edtest.P(e).Cover != nil {
		t.Fatal("обложка осталась после отказа")
	}
}

// Режим bg: макет без Bg, но с цветом стороны, задаёт фон обложки; свой цвет задней сам не уходит
// на лицевую (индексы сторон верны).
func TestApplyLookBgCoverSideColorOnly(t *testing.T) {
	tpl := lkCoverTpl()
	tpl.Cover.Bg, tpl.Cover.Front = nil, lkSide(1, "")
	tpl.Cover.Back = lkSide(1, "#112233")
	e := lkBuilt(t, 2)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	plan := lkPlanned(t, e, lkRefA, tpl, nil, model.LookOpts{Mode: model.LookBg})
	if c := edtest.P(e).Cover.Sheet; !plan.Cover || c.BgL != "#112233" || c.BgR != "" {
		t.Fatalf("план %+v, фон %q %q", plan, c.BgL, c.BgR)
	}
}

// Макет без фона обложки её фон не трогает (серия и шаг 0): свой цвет цел.
func TestApplyLookCoverNoBgKeepsBg(t *testing.T) {
	tpl := lkCoverTpl()
	tpl.Cover.Bg, tpl.Cover.Back, tpl.Cover.Front = nil, lkSide(2, ""), lkSide(3, "")
	e := lkCoverEditor(t)
	e.SetBackground(model.CoverID, "spread", "#abcdef")
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgL != "#abcdef" || c.BgR != "#abcdef" || len(c.Grid) != 5 {
		t.Fatalf("фон %q %q, мест %d", c.BgL, c.BgR, len(c.Grid))
	}
	// Своя надпись на стороне без надписей в шаблоне остаётся.
	lkOwnText(e, model.ScopeLeft)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, lkCoverTpl(), nil); err != nil || lkTextJSON(e, "c0ffee0000b2") == "" {
		t.Fatalf("своя надпись пропала: %v", err)
	}
}

// Смена макета: обложка прежнего макета, новый ставит другие места — не тронутое.
func TestApplyLookCoverNextPlaces(t *testing.T) {
	a, b := lkCoverTpl(), lkCoverTpl()
	b.Cover.Front = lkSide(2, "#998877")
	e := lkCoverEditor(t)
	lkApply(t, e, lkRefA, a, nil, 6)
	plan, err := e.PlanLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 0 || !plan.Cover {
		t.Fatalf("план %+v %v", plan, err)
	}
}

// Свой цвет задней без Bg: ставится на заднюю, лицевая не меняется.
func TestApplyLookCoverBackColorOnly(t *testing.T) {
	tpl := lkCoverTpl()
	tpl.Cover.Bg, tpl.Cover.Back, tpl.Cover.Front = nil, lkSide(1, "#445566"), lkSide(1, "")
	e := lkCoverEditor(t)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, tpl, nil); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Sheet; c.BgL != "#445566" || c.BgR != "" {
		t.Fatalf("фон %q %q", c.BgL, c.BgR)
	}
}
