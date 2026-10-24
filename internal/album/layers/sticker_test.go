package layers_test

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Стикеры (спека 048 Ф2): правки, отмена, стек трёх видов, группа, фото стикера.

const (
	keyA = "my:0123456789abcdef"
	keyB = "my:fedcba9876543210"
)

// own — своё из библиотеки w×h px (ID — от ключа, как у App).
func own(key string, w, h int) model.StickerSrc {
	return model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs(key[3:] + ".png"), Lib: key, W: w, H: h}}
}

// art — процедурный рисунок; plate — растяжимая плашка (Aspect 3), zz-future — ключ новой версии.
func art(kind string, inks ...string) model.StickerSrc {
	return model.StickerSrc{Art: &model.Art{Kind: kind, Inks: inks}}
}

func addSticker(t *testing.T, e *album.Editor, sid string, page int, src model.StickerSrc) string {
	t.Helper()
	_, id, err := e.AddSticker(sid, page, src)
	if err != nil || id == "" {
		t.Fatalf("AddSticker: %q, %v", id, err)
	}
	edtest.MustGrid(t, e)
	return id
}

// stOf — копия стикера по ID (nil — нет).
func stOf(e *album.Editor, id string) *model.Sticker {
	s, i := edtest.P(e).FindSticker(id)
	if s == nil {
		return nil
	}
	return s.Stickers[i].Clone()
}

func center(st *model.Sticker) (float64, float64) { return st.X + st.W/2, st.Y + st.H/2 }

func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

// TestStickerEdits — добавление по центру страницы (¼ ширины, не выше ½), ошибки без правки,
// рамка, зеркало, прозрачность, цвета, замена, удаление; несколько стикеров на двух листах.
func TestStickerEdits(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t) // страница 300×300, лист 600×300
	a := addSticker(t, e, s1, 0, own(keyA, 400, 200))
	if st := stOf(e, a); *st != (model.Sticker{ID: a, Photo: model.LibPhotoID(keyA), X: 112.5, Y: 131.25, W: 75, H: 37.5}) {
		t.Fatalf("своё на левой: %+v", *st)
	}
	if r := edtest.P(e).Photos[model.LibPhotoID(keyA)]; r.Lib != keyA || r.W != 400 {
		t.Fatalf("запись своего: %+v", r)
	}
	b := addSticker(t, e, s1, 1, art("plate"))
	if st := stOf(e, b); st.Art.Kind != "plate" || st.W != 75 || st.H != 25 || !near(st.X+st.W/2, 450) {
		t.Fatalf("плашка на правой: %+v", *st)
	}
	tall := addSticker(t, e, s2, 7, own(keyB, 100, 400)) // страница зажата в 1; высокое — не выше ½
	if st := stOf(e, tall); st.H != 150 || st.W != 37.5 || !near(st.X+st.W/2, 450) {
		t.Fatalf("высокое: %+v", *st)
	}
	future := addSticker(t, e, s2, 0, art("zz-future", "#ABCDEF"))
	if st := stOf(e, future); st.Art.Kind != "zz-future" || st.W != st.H || st.Art.Inks[0] != "#abcdef" {
		t.Fatalf("неизвестный ключ хранится: %+v %+v", *st, *st.Art)
	}

	// Ошибки — без шага и без правки.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	bad := own(keyA, 10, 10)
	bad.Photo.ID = "чужой"
	for _, c := range []struct {
		src  model.StickerSrc
		want error
	}{{art("BAD!"), model.ErrSticker}, {art("dots"), model.ErrSticker}, {art(""), model.ErrSticker},
		{bad, model.ErrLibKey}, {model.StickerSrc{}, model.ErrLibKey}} {
		if _, id, err := e.AddSticker(s1, 0, c.src); !errors.Is(err, c.want) || id != "" {
			t.Errorf("%+v: %q, %v", c.src, id, err)
		}
	}
	// Неизвестный лист — холостая с пустым ID (048 ревью №208: прежде ID отдавался без правки,
	// и App сравнивал rev вне замка правки; теперь Editor сам говорит, что стикер не встал).
	if _, id, err := e.AddSticker("нет", 0, art("plate")); err != nil || id != "" {
		t.Errorf("неизвестный лист: %q, %v", id, err)
	}
	const keyC = "my:00000000000000cc" // нигде не стоит: запись своего не должна остаться
	if _, id, err := e.AddSticker("нет", 0, own(keyC, 10, 10)); err != nil || id != "" || edtest.HasRef(edtest.P(e), model.LibPhotoID(keyC)) {
		t.Errorf("своё на неизвестный лист: %q, %v", id, err)
	}
	edtest.Same(t, e, rev, undo, "отказы и неизвестный лист")

	// Рамка: своя пропорция держится вокруг центра, поворот — шаг 0.1.
	e.SetStickerBox(s1, a, 10, 20, 100, 10, 375.04, "")
	if st := stOf(e, a); st.W != 100 || st.H != 50 || st.X != 10 || st.Y != 0 || st.Rot != 15 {
		t.Fatalf("рамка своего: %+v", *st)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetStickerBox(s1, a, math.NaN(), 0, 1, 1, 0, "")
	e.SetStickerBox(s2, a, 0, 0, 50, 50, 0, "") // стикер другого листа
	e.SetStickerBox(s1, "нет", 0, 0, 50, 50, 0, "")
	e.SetStickerBox(s1, a, 10, 0, 100, 50, 15, "") // то же
	edtest.Same(t, e, rev, undo, "NaN, чужой лист, неизвестный, то же")
	stickerLook(t, e, s1, a, b)
	// Удаление — через DeleteSelection (ID стикера там, где ID текста); фото без дела уходит.
	e.DeleteSelection(s2, nil, nil, []string{tall, future})
	if s, _ := edtest.P(e).FindSticker(tall); s != nil || edtest.HasRef(edtest.P(e), model.LibPhotoID(keyB)) ||
		edtest.HasRef(edtest.P(e), model.LibPhotoID(keyA)) {
		t.Fatalf("после удаления: %s", model.Marshal(edtest.P(e)))
	}
	edtest.MustGrid(t, e)
}

// stickerLook — вторая половина TestStickerEdits: зеркало, прозрачность, цвета, замена у своего
// a и плашки b листа s1.
func stickerLook(t *testing.T, e *album.Editor, s1, a, b string) {
	t.Helper()
	// Зеркало, прозрачность.
	e.FlipSticker(s1, a, "h")
	e.FlipSticker(s1, a, "v")
	if st := stOf(e, a); !st.FlipH || !st.FlipV {
		t.Fatalf("зеркало: %+v", *st)
	}
	e.FlipSticker(s1, a, "x")
	e.SetStickerFade(s1, a, 0.333, "")
	if st := stOf(e, a); st.Fade != 0.33 {
		t.Fatalf("прозрачность: %v", st.Fade)
	}
	e.SetStickerFade(s1, a, 5, "")
	if st := stOf(e, a); st.Fade != 1 {
		t.Fatalf("прозрачность сверх: %v", st.Fade)
	}

	// Цвета: у рисунка — нижним регистром, хвост из цветов генератора не хранится; у своего — холостая.
	e.SetStickerInks(s1, b, []string{"#FF0000"}, "")
	if st := stOf(e, b); !slices.Equal(st.Art.Inks, []string{"#ff0000"}) {
		t.Fatalf("цвета: %v", st.Art.Inks)
	}
	// У плашки два цвета (#fffaf0, #d9b779): цвета генератора и хвост, равный ему, не хранятся.
	e.SetStickerInks(s1, b, []string{"#111111", "#d9b779"}, "")
	if st := stOf(e, b); !slices.Equal(st.Art.Inks, []string{"#111111"}) {
		t.Fatalf("хвост цвета генератора хранится: %v", st.Art.Inks)
	}
	e.SetStickerInks(s1, b, []string{"#FFFAF0", "#222222", "#333333"}, "") // третий — сверх двух
	if st := stOf(e, b); !slices.Equal(st.Art.Inks, []string{"#fffaf0", "#222222"}) {
		t.Fatalf("два цвета, лишний отброшен: %v", st.Art.Inks)
	}
	e.SetStickerInks(s1, b, []string{"#fffaf0", "#d9b779"}, "")
	if st := stOf(e, b); st.Art.Inks != nil {
		t.Fatalf("цвета генератора хранятся: %v", st.Art.Inks)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetStickerInks(s1, a, []string{"#ff0000"}, "")
	e.SetStickerInks(s1, b, nil, "")
	edtest.Same(t, e, rev, undo, "цвета своего и те же цвета")

	// Замена: центр, ширина, поворот, зеркало, прозрачность стоят; высота — по новой пропорции.
	was := stOf(e, a)
	if _, changed, err := e.ReplaceSticker(s1, a, own(keyB, 100, 400)); err != nil || !changed {
		t.Fatal(changed, err)
	}
	cx, cy := center(was)
	if st := stOf(e, a); st.W != was.W || st.H != 400 || st.Rot != was.Rot || !st.FlipH || st.Fade != 1 ||
		!near(st.X+st.W/2, cx) || !near(st.Y+st.H/2, cy) || st.Photo != model.LibPhotoID(keyB) {
		t.Fatalf("замена на высокое: %+v, было %+v", *st, *was)
	}
	if _, _, err := e.ReplaceSticker(s1, a, art("plate")); err != nil { // растяжимая рамку не меняет
		t.Fatal(err)
	}
	if st := stOf(e, a); st.Photo != "" || st.Art.Kind != "plate" || st.W != 100 || st.H != 400 {
		t.Fatalf("замена на плашку: %+v", *st)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	// Холостая (тот же, неизвестный стикер, чужой лист) — changed=false: App не кладёт в недавние.
	for _, c := range []struct{ sid, id string }{{s1, a}, {s1, "нет"}, {"нет", a}} {
		if _, changed, err := e.ReplaceSticker(c.sid, c.id, art("plate")); err != nil || changed {
			t.Fatalf("замена тем же %v: %v, %v", c, changed, err)
		}
	}
	if _, changed, err := e.ReplaceSticker(s1, a, art("dots")); !errors.Is(err, model.ErrSticker) || changed {
		t.Fatalf("замена на узор: %v, %v", changed, err)
	}
	edtest.Same(t, e, rev, undo, "замена тем же, неизвестный, чужой лист и на узор")
}

// TestStickerLimit — не больше MaxStickers на лист: 65-й — ошибка без шага; на другом листе — можно.
func TestStickerLimit(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	edtest.Change(e, func(p *model.Project) {
		_, s := p.Find(s1)
		for range model.MaxStickers {
			s.Stickers = append(s.Stickers, &model.Sticker{ID: model.NewID(), Art: &model.Art{Kind: "plate"}, X: 10, Y: 10, W: 30, H: 10})
		}
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.AddSticker(s1, 0, art("plate")); !errors.Is(err, model.ErrStickers) {
		t.Fatalf("65-й: %v", err)
	}
	edtest.Same(t, e, rev, undo, "65-й стикер")
	addSticker(t, e, s2, 0, art("plate"))
}

// TestStickerUndo — каждая правка — шаг; серия рамки, ползунка и палитры — один шаг; Undo
// возвращает и запись своего фото; холостые — без шага.
func TestStickerUndo(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	empty := string(model.Marshal(edtest.P(e)))
	undo := edtest.UndoLen(e)
	a := addSticker(t, e, s1, 0, own(keyA, 400, 200))
	b := addSticker(t, e, s1, 1, art("plate"))
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов %d, ждали %d", edtest.UndoLen(e), undo+2)
	}
	undo = edtest.UndoLen(e)
	for i := range 3 {
		e.SetStickerBox(s1, a, 10+float64(i), 10, 75, 37.5, 0, "drag-1")
	}
	for i := range 3 {
		e.SetStickerFade(s1, b, 0.1*float64(i+1), "fade-1")
	}
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("серии: шагов %d, ждали %d", edtest.UndoLen(e)-undo, 2)
	}
	e.SetStickerInks(s1, b, []string{"#111111"}, "ink-1")
	e.SetStickerInks(s1, b, []string{"#222222"}, "ink-1")
	e.FlipSticker(s1, a, "h")
	if edtest.UndoLen(e) != undo+4 {
		t.Fatalf("палитра и зеркало: шагов %d, ждали 4", edtest.UndoLen(e)-undo)
	}
	rev, n := edtest.Rev(e), edtest.UndoLen(e)
	e.SetStickerFade(s1, b, 0.3, "fade-2")
	e.SetStickerBox(s1, a, 12, 10, 75, 37.5, 0, "drag-2")
	edtest.Same(t, e, rev, n, "те же значения")
	e.Undo() // зеркало
	if stOf(e, a).FlipH {
		t.Fatal("Undo зеркала")
	}
	e.Undo() // палитра одним шагом
	if st := stOf(e, b); st.Art.Inks != nil {
		t.Fatalf("Undo палитры: %v", st.Art.Inks)
	}
	for edtest.UndoLen(e) > 0 && stOf(e, a) != nil {
		e.Undo()
	}
	if got := string(model.Marshal(edtest.P(e))); got != empty {
		t.Fatalf("Undo до начала:\n%s\n%s", got, empty)
	}
	e.Redo()
	if stOf(e, a) == nil || !edtest.HasRef(edtest.P(e), model.LibPhotoID(keyA)) {
		t.Fatal("Redo добавления")
	}
}

// TestStackStickers — по умолчанию места → стикеры → тексты, Order только при отличии; новый
// стикер — над местами и стикерами (при стикере над текстом — над ним, design §3.1);
// «На передний план» — над текстом.
func TestStackStickers(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(60, 60, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	t1 := edtest.TextAt(t, e, "s1", 50, 50)
	s1 := addSticker(t, e, "s1", 0, art("plate"))
	s2 := addSticker(t, e, "s1", 0, own(keyA, 300, 300))
	if got := edtest.StackOf(e); edtest.P(e).Spreads[0].Order != nil || !slices.Equal(got, []string{g[0], g[1], s1, s2, t1}) {
		t.Fatalf("по умолчанию: Order %v, стек %v", edtest.P(e).Spreads[0].Order, got)
	}
	if bytes.Contains(model.Marshal(edtest.P(e)), []byte(`"order"`)) {
		t.Fatal("order без отличия от порядка по умолчанию")
	}
	items := edtest.P(e).Spreads[0].LayerItems()
	if items[2].Kind != model.LayerSticker || items[2].ID != s1 || items[4].Kind != model.LayerText {
		t.Fatalf("виды: %+v", items)
	}
	e.ArrangeLayers("s1", "front", []string{s1})
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{g[0], g[1], s2, t1, s1}) {
		t.Fatalf("стикер над текстом: %v", got)
	}
	s3 := addSticker(t, e, "s1", 0, art("plate"))
	if got := edtest.StackOf(e); !slices.Equal(got, []string{g[0], g[1], s2, t1, s1, s3}) {
		t.Fatalf("новый при Order: %v", got)
	}
	e.ArrangeLayers("s1", "back", []string{s3})
	if got := edtest.StackOf(e); !slices.Equal(got, []string{s3, g[0], g[1], s2, t1, s1}) || stOf(e, s3) == nil {
		t.Fatalf("стикер под фото: %v", got)
	}
	edtest.MustGrid(t, e)
	// Ушли стикеры не на своём месте — Order = nil.
	e.DeleteSelection("s1", nil, nil, []string{s1, s3})
	if s := edtest.P(e).Spreads[0]; s.Order != nil || !slices.Equal(edtest.StackOf(e), []string{g[0], g[1], s2, t1}) {
		t.Fatalf("после удаления: Order %v, стек %v", s.Order, edtest.StackOf(e))
	}

	// Починка напрямую: недостающий стикер — над верхним местом или стикером; нет их — вниз.
	pl := func(id string) *model.Place { return &model.Place{ID: id} }
	st := func(id string) *model.Sticker { return &model.Sticker{ID: id} }
	s := &model.Spread{Grid: []*model.Place{pl("p1"), pl("p2")}, Stickers: []*model.Sticker{st("k1"), st("k2")},
		Texts: []*model.TextBlock{{ID: "t1"}}, Order: []string{"t1", "p1", "нет", "k1", "p2"}}
	if got, want := s.Stack(), []string{"t1", "p1", "k1", "p2", "k2"}; !slices.Equal(got, want) {
		t.Fatalf("stack %v, ждали %v", got, want)
	}
	s = &model.Spread{Stickers: []*model.Sticker{st("k1"), st("k2")}, Texts: []*model.TextBlock{{ID: "t1"}},
		Order: []string{"t1", "k2"}}
	if got, want := s.Stack(), []string{"t1", "k2", "k1"}; !slices.Equal(got, want) {
		t.Fatalf("stack «Авто» %v, ждали %v", got, want)
	}
	s.Order = []string{"k2"} // стикеров в Order нет выше k2 — k1 над ним, текст наверх
	if got, want := s.Stack(), []string{"k2", "k1", "t1"}; !slices.Equal(got, want) {
		t.Fatalf("stack %v, ждали %v", got, want)
	}
	if s.SetStack([]string{"k1", "k2", "t1"}); s.Order != nil || s.Stickers[1].ID != "k2" {
		t.Fatalf("порядок по умолчанию: Order %v", s.Order)
	}
}

// TestSyncOrderAutoStickers — на «Авто» Order живёт, если есть стикеры (только стикеры и тексты),
// переживает Save → Load; без стикеров — nil; фото раскладки в стек не входят.
func TestSyncOrderAutoStickers(t *testing.T) {
	e, sid := edtest.AutoEd(t)
	t1 := edtest.AddText(t, e, sid, 0)
	k := addSticker(t, e, sid, 0, art("plate"))
	if s := edtest.P(e).Spreads[0]; s.Order != nil || !slices.Equal(s.Stack(), []string{k, t1}) {
		t.Fatalf("«Авто» по умолчанию: Order %v, стек %v", s.Order, s.Stack())
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ArrangeLayers(sid, "front", []string{"a", "b"}) // фото «Авто» — не слои
	edtest.Same(t, e, rev, undo, "фото «Авто» на передний план")
	e.ArrangeLayers(sid, "front", []string{k})
	if s := edtest.P(e).Spreads[0]; !slices.Equal(s.Order, []string{t1, k}) || s.Grid != nil {
		t.Fatalf("стикер над текстом на «Авто»: Order %v, сетка %v", s.Order, s.Grid)
	}
	edtest.MustGrid(t, e)
	if q := edtest.SaveLoad(t, e); !slices.Equal(q.Spreads[0].Order, []string{t1, k}) {
		t.Fatalf("Save → Load: %v", q.Spreads[0].Order)
	}
	e.DeleteSelection(sid, nil, nil, []string{k})
	if s := edtest.P(e).Spreads[0]; s.Order != nil {
		t.Fatalf("без стикеров: %v", s.Order)
	}
	edtest.MustGrid(t, e)
}

// TestStickerGroup — стикер в группе — как место: перенос и масштаб, поворот по дуге, масштаб по
// осям (своя пропорция возвращается), выравнивание; у «Авто» группа из стикеров не закрепляет.
func TestStickerGroup(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(20, 20, 100, 100, "a")})
	pid := edtest.PlaceIDsOf(e)[0]
	k := addSticker(t, e, "s1", 0, own(keyA, 400, 200))
	e.SetStickerBox("s1", k, 150, 40, 60, 30, 0, "")
	f := addSticker(t, e, "s1", 1, art("zz-future"))
	e.SetStickerBox("s1", f, 300, 40, 40, 20, 0, "")
	ids := []string{pid, k, f}

	e.TransformGroup("s1", ids, 1.5, 20, 20, 0, 0, "")
	if st := stOf(e, k); st.X != 215 || st.Y != 50 || st.W != 90 || st.H != 45 {
		t.Fatalf("масштаб группы: %+v", *st)
	}
	if st := stOf(e, f); st.X != 440 || st.W != 60 || st.H != 30 {
		t.Fatalf("масштаб свободной пропорции: %+v", *st)
	}
	edtest.MustGrid(t, e)
	// Поворот на 180° вокруг центра места и стикера: стикер — по дуге, угол — его.
	pair := []string{pid, k}
	e.RotateGroup("s1", pair, 180, 162.5, 95)
	if st := stOf(e, k); st.Rot != 180 || st.W != 90 || !near(st.X+st.W/2, 65) || !near(st.Y+st.H/2, 117.5) {
		t.Fatalf("поворот группы: %+v", *st)
	}
	edtest.MustGrid(t, e)
	e.ScaleGroupAxes("s1", pair, 1.5, 1, 0, 20, 20)
	if st := stOf(e, k); st.W <= 90 || math.Abs(st.W/st.H-2) > 0.01 {
		t.Fatalf("масштаб по осям: пропорция %+v", *st)
	}
	edtest.MustGrid(t, e)
	e.AlignItems("s1", []string{pid, f}, "top", "sel")
	items := edtest.P(e).Spreads[0].LayerItems()
	top := func(id string) float64 {
		i := slices.IndexFunc(items, func(it model.LayerItem) bool { return it.ID == id })
		return items[i].Box.AABB().Y
	}
	if !near(top(pid), top(f)) {
		t.Fatalf("выравнивание по верху: место %v, стикер %v", top(pid), top(f))
	}
	edtest.MustGrid(t, e)

	ea, sid := edtest.AutoEd(t)
	g1 := addSticker(t, ea, sid, 0, art("plate"))
	g2 := addSticker(t, ea, sid, 1, art("plate"))
	ea.TransformGroup(sid, []string{g1, g2}, 1, 0, 0, 5, 5, "")
	if s := edtest.P(ea).Spreads[0]; s.Grid != nil || stOf(ea, g1).Y != stOf(ea, g2).Y {
		t.Fatalf("«Авто»: сетка %v", s.Grid)
	}
	edtest.MustGrid(t, ea)
}

// TestStickerPhotoRefs — фото своего стикера — в PhotoIDs и BackdropUsed, не «в альбоме»:
// автосборка и «Заполнить пустые» его не берут; PruneRefs держит запись, пока стикер стоит.
func TestStickerPhotoRefs(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, ""), edtest.Gp(150, 10, 100, 100, "")})
	k := addSticker(t, e, "s1", 0, own(keyA, 400, 200))
	k2 := addSticker(t, e, "s1", 1, own(keyA, 400, 200)) // то же фото дважды
	id := model.LibPhotoID(keyA)
	s := edtest.P(e).Spreads[0]
	if !slices.Contains(s.PhotoIDs(), id) || slices.Contains(s.Photos, id) || !edtest.P(e).OnDecor()[id] {
		t.Fatalf("PhotoIDs %v, Photos %v", s.PhotoIDs(), s.Photos)
	}
	if v := e.View(); !slices.Equal(v.BackdropUsed, []string{id}) {
		t.Fatalf("BackdropUsed %v", v.BackdropUsed)
	}
	v, left, _ := e.FillPlaces([]model.PhotoInfo{own(keyA, 400, 200).Photo, edtest.Photo("x")}, "s1")
	if left != 0 || slices.Contains(edtest.PhotosOf(e, 0), id) || !slices.Contains(edtest.PhotosOf(e, 0), "x") {
		t.Fatalf("«Заполнить пустые»: %v, осталось %d", edtest.PhotosOf(e, 0), left)
	}
	n := len(v.Spreads)
	if v, err := e.AutoBuild([]model.PhotoInfo{own(keyA, 400, 200).Photo}, -1, model.AutoOpts{}); err != nil || len(v.Spreads) != n {
		t.Fatalf("автосборка взяла фото стикера: %v, листов %d", err, len(v.Spreads))
	}
	e.DeleteSelection("s1", nil, nil, []string{k})
	if !edtest.HasRef(edtest.P(e), id) {
		t.Fatal("запись ушла, пока второй стикер стоит")
	}
	e.DeleteSelection("s1", nil, nil, []string{k2})
	if edtest.HasRef(edtest.P(e), id) || len(e.View().BackdropUsed) != 0 {
		t.Fatal("запись без стикера осталась")
	}
	edtest.MustGrid(t, e)
}
