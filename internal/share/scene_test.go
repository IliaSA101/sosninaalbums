package share

import (
	"crypto/sha256"
	"image/color"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

var specA = export.WebSpec{DPI: 100, PageW: 100, PageH: 60, BleedX: 3, BleedY: 3, LongSide: 1600, Quality: 82}

func mustHash(t *testing.T, sp []render.Spread, cover *export.CoverJob, spec export.WebSpec, st FileStat) string {
	t.Helper()
	h, err := SceneHash(sp, cover, spec, st)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 64 {
		t.Fatalf("хэш %q", h)
	}
	return h
}

// Метки и подборки в сцену не входят (хэш прежний), кадр, текст, файл и параметры — входят.
func TestShareSceneHash(t *testing.T) {
	st := text.Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}
	p := model.New()
	pa, pb := testkit.AbsPath("a.jpg"), testkit.AbsPath("b.jpg")
	p.Photos["a"] = model.PhotoRef{Path: pa, W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: pb, W: 200, H: 300}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Texts: []*model.TextBlock{{ID: "t1", X: 20, Y: 30, W: 80, Text: "Привет", Style: st}}},
		{ID: "s2", Photos: []string{"b"}},
	}
	e, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	scene := func() []render.Spread {
		pp, v := e.Snapshot()
		return export.FromView(pp, v)
	}
	files := map[string][2]int64{pa: {1000, 5}, pb: {2000, 6}}
	stat := func(path string) (int64, int64, bool) {
		f, ok := files[path]
		return f[0], f[1], ok
	}
	base := mustHash(t, scene(), nil, specA, stat)
	if base != mustHash(t, scene(), nil, specA, stat) {
		t.Fatal("хэш одной сцены различается")
	}

	// Метки: оценка, цвет, флаг — хэш тот же.
	five, red, flag := 5, "red", 1
	if _, err := e.SetMarks([]model.PhotoInfo{{ID: "a", Path: pa}, {ID: "b", Path: pb}}, model.MarkPatch{Rating: &five, Label: &red, Flag: &flag}); err != nil {
		t.Fatal(err)
	}
	if got := mustHash(t, scene(), nil, specA, stat); got != base {
		t.Error("метки изменили хэш сцены")
	}
	// Поколение шрифтов (Gen) — в другом запуске другое, на хэш не влияет.
	pinned := scene()
	export.PinFonts(pinned, 99)
	if got := mustHash(t, pinned, nil, specA, stat); got != base {
		t.Error("поколение шрифтов изменило хэш")
	}

	// Кадр фото — другая сцена.
	e.SetFrame("a", 2, -1, -1)
	framed := mustHash(t, scene(), nil, specA, stat)
	if framed == base {
		t.Error("кадр не изменил хэш")
	}
	// Файл фото: изменился размер или время; пропал — тоже другая сцена.
	files[pa] = [2]int64{1001, 5}
	sz := mustHash(t, scene(), nil, specA, stat)
	files[pa] = [2]int64{1000, 6}
	mt := mustHash(t, scene(), nil, specA, stat)
	delete(files, pa)
	gone := mustHash(t, scene(), nil, specA, stat)
	if len(map[string]bool{framed: true, sz: true, mt: true, gone: true}) != 4 {
		t.Error("размер, время и пропажа файла должны давать разные хэши")
	}
	files[pa] = [2]int64{1000, 5}
	e.Undo()
	if got := mustHash(t, scene(), nil, specA, stat); got != base {
		t.Error("возврат правки не вернул хэш")
	}
	// Без stat файлы не учитываются, но пути в хэше.
	if mustHash(t, scene(), nil, specA, nil) == base {
		t.Error("хэш без stat совпал с хэшем со stat")
	}

	// Параметры страниц и обложка.
	other := specA
	other.LongSide = 1200
	if mustHash(t, scene(), nil, other, stat) == base {
		t.Error("длинная сторона не влияет на хэш")
	}
	other = specA
	other.BleedX = 0
	if mustHash(t, scene(), nil, other, stat) == base {
		t.Error("обрез не влияет на хэш")
	}
	cj := &export.CoverJob{Spread: render.Spread{Bg: [2]color.RGBA{{1, 2, 3, 255}, {1, 2, 3, 255}}}, Opt: render.Options{PageW: 50, PageH: 60, DPI: 100}}
	withCover := mustHash(t, scene(), cj, specA, stat)
	cj2 := &export.CoverJob{Spread: cj.Spread, Opt: cj.Opt}
	cj2.Spread.Bg[0].R = 9
	if withCover == base || withCover == mustHash(t, scene(), cj2, specA, stat) {
		t.Error("обложка не влияет на хэш")
	}
}

// Края: пустая сцена, nil и пустые списки, -0 и 0, NaN, порядок разворотов, вложенность.
func TestShareSceneHashEdges(t *testing.T) {
	empty := mustHash(t, nil, nil, specA, nil)
	if empty != mustHash(t, []render.Spread{}, nil, specA, nil) {
		t.Error("nil и пустой список различаются")
	}
	one := []render.Spread{{Bg: [2]color.RGBA{{1, 1, 1, 255}, {2, 2, 2, 255}}}}
	if mustHash(t, one, nil, specA, nil) == empty {
		t.Error("один разворот не отличается от пустой сцены")
	}
	// Порядок разворотов и их число.
	a, b := render.Spread{Bg: [2]color.RGBA{{1, 0, 0, 255}, {1, 0, 0, 255}}}, render.Spread{Bg: [2]color.RGBA{{0, 1, 0, 255}, {0, 1, 0, 255}}}
	if mustHash(t, []render.Spread{a, b}, nil, specA, nil) == mustHash(t, []render.Spread{b, a}, nil, specA, nil) {
		t.Error("порядок разворотов не влияет")
	}
	if mustHash(t, []render.Spread{a}, nil, specA, nil) == mustHash(t, []render.Spread{a, a}, nil, specA, nil) {
		t.Error("число разворотов не влияет")
	}
	// -0 = 0; NaN считается без паники и устойчиво.
	neg := math.Copysign(0, -1)
	p0 := []render.Spread{{Plates: []render.Plate{{Rect: layout.Rect{X: 0}}}}}
	pn := []render.Spread{{Plates: []render.Plate{{Rect: layout.Rect{X: neg}}}}}
	if mustHash(t, p0, nil, specA, nil) != mustHash(t, pn, nil, specA, nil) {
		t.Error("-0 и 0 различаются")
	}
	nan := []render.Spread{{Plates: []render.Plate{{Rect: layout.Rect{X: math.NaN()}}}}}
	nanA, nanB := mustHash(t, nan, nil, specA, nil), mustHash(t, nan, nil, specA, nil)
	if nanA != nanB {
		t.Error("NaN нестабилен")
	}
	if mustHash(t, nan, nil, specA, nil) == mustHash(t, p0, nil, specA, nil) {
		t.Error("NaN совпал с нулём")
	}
	// Фото: путь без stat учитывается, пустой путь — нет паники.
	ph := []render.Spread{{Photos: []render.Photo{{Path: ""}, {Path: testkit.AbsPath("x.jpg")}}}}
	mustHash(t, ph, nil, specA, func(string) (int64, int64, bool) { return 0, 0, false })
}

// Обход устойчив к циклам и картам: цикл — ошибка, а не зависание; карта не зависит от порядка.
func TestShareSceneWalker(t *testing.T) {
	type node struct{ Next *node }
	n := &node{}
	n.Next = n
	w := &sceneWriter{h: sha256.New()}
	w.value(reflect.ValueOf(n), 0)
	if w.err == nil {
		t.Error("цикл не замечен")
	}
	m := map[string]int{}
	for i := range 50 {
		m[string(rune('a'+i%26))+string(rune('A'+i/2%26))] = i
	}
	sum := func() string {
		w := &sceneWriter{h: sha256.New()}
		w.value(reflect.ValueOf(m), 0)
		if w.err != nil {
			t.Fatal(w.err)
		}
		return string(w.h.Sum(nil))
	}
	first := sum()
	for range 20 {
		if sum() != first {
			t.Fatal("хэш карты зависит от порядка обхода")
		}
	}
}

// Хэш сцены после Save→Load тот же: опубликованный альбом, закрытый и открытый заново, не
// становится «изменённым после выгрузки». Сравнение от загруженного альбома (nil и пустые списки
// после чтения не те же, что в памяти). Холостая правка (кадр в прежнее значение) хэш не трогает.
func TestShareSceneSaveLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	st := text.Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Bleed = 120, 90, 3
	pa, pb := testkit.AbsPath("a.jpg"), testkit.AbsPath("b.jpg")
	p.Photos["a"] = model.PhotoRef{Path: pa, W: 300, H: 200, Frame: &model.Frame{S: 2, X: -1, Y: -1}, Caption: &model.Caption{Text: "Подпись", Style: model.DefaultCaptionStyle()}}
	p.Photos["b"] = model.PhotoRef{Path: pb, W: 200, H: 300, FlipH: true}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Texts: []*model.TextBlock{{ID: "t1", X: 20, Y: 30, W: 80, Text: "Привет", Style: st}}},
		{ID: "s2", Photos: []string{"b"}, BgR: "#1c1c1c"},
		{ID: "s3"},
	}
	e, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	e.SetCoverSpineBg("#102030")
	stat := func(path string) (int64, int64, bool) { return 100, 7, true }
	hashOf := func(e *album.Editor) string {
		pp, v := e.Snapshot()
		spec, err := export.NewWebSpec(pp.Settings, v)
		if err != nil {
			t.Fatal(err)
		}
		return mustHash(t, export.FromView(pp, v), export.NewCoverJob(pp, v, spec.DPI, 82), spec, stat)
	}
	file := filepath.Join(dir, "a.salbum")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	loaded := func() *album.Editor {
		q, err := album.LoadBase(file, dir)
		if err != nil {
			t.Fatal(err)
		}
		le, err := album.NewEditor(q, file)
		if err != nil {
			t.Fatal(err)
		}
		return le
	}
	l1 := loaded()
	h1 := hashOf(l1)
	if h1 != hashOf(loaded()) {
		t.Fatal("хэш двух загрузок одного файла различается")
	}
	// Сохранить загруженный и открыть снова — хэш не плывёт на втором круге.
	file2 := filepath.Join(dir, "b.salbum")
	if err := l1.Save(file2); err != nil {
		t.Fatal(err)
	}
	file = file2
	if h2 := hashOf(loaded()); h2 != h1 {
		t.Error("второй круг Save→Load изменил хэш")
	}
	if hashOf(e) != h1 {
		t.Error("хэш исходного альбома в памяти и загруженного различается")
	}
	// Холостая правка: кадр в то же значение — хэш и ревизия прежние. После загрузки ID фото
	// другие (по пути), поэтому ищем по пути.
	pl, v0 := l1.Snapshot()
	id := ""
	for k, ph := range pl.Photos {
		if ph.Path == pa {
			id = k
		}
	}
	if id == "" {
		t.Fatal("фото не нашлось после загрузки")
	}
	l1.SetFrame(id, 2, -1, -1)
	if _, v1 := l1.Snapshot(); v1.Rev != v0.Rev || hashOf(l1) != h1 {
		t.Errorf("холостая правка: rev %d → %d, хэш изменился: %v", v0.Rev, v1.Rev, hashOf(l1) != h1)
	}
	// Настоящая правка — другой хэш, отмена возвращает прежний.
	l1.SetFrame(id, 1.5, 0, 0)
	if hashOf(l1) == h1 {
		t.Error("правка кадра не изменила хэш")
	}
	l1.Undo()
	if hashOf(l1) != h1 {
		t.Error("отмена не вернула хэш")
	}
}

// Нулевые поля в хэш не пишутся: поле, добавленное в модель сцены позже, пока не задано, не
// меняет хэши уже опубликованных альбомов.
func TestShareSceneZeroFields(t *testing.T) {
	type old struct {
		A int
		S string
	}
	type grown struct {
		A int
		S string
		B float64
		L []int
		P *int
		N struct{ X [2]float64 }
	}
	sum := func(v any) string {
		w := &sceneWriter{h: sha256.New()}
		w.value(reflect.ValueOf(v), 0)
		if w.err != nil {
			t.Fatal(w.err)
		}
		return string(w.h.Sum(nil))
	}
	if sum(old{A: 1, S: "x"}) != sum(grown{A: 1, S: "x", B: math.Copysign(0, -1), L: []int{}}) {
		t.Error("нулевое новое поле изменило хэш")
	}
	one := 1
	if sum(old{A: 1, S: "x"}) == sum(grown{A: 1, S: "x", B: 0.5}) || sum(old{A: 1}) == sum(grown{A: 1, P: &one}) {
		t.Error("заданное новое поле не меняет хэш")
	}
	zeroPtr := 0
	if sum(grown{}) == sum(grown{P: &zeroPtr}) {
		t.Error("nil и указатель на ноль неотличимы")
	}
	// Граница вложенной структуры: без метки конца ключи внутреннего и внешнего поля склеились бы.
	type in struct{ X, Y float64 }
	type out struct {
		A in
		Y float64
	}
	if sum(out{A: in{X: 1}, Y: 2}) == sum(out{A: in{X: 1, Y: 2}}) {
		t.Error("граница вложенной структуры неоднозначна")
	}
	o1, o2 := out{A: in{X: 1}, Y: 2}, out{A: in{X: 1}, Y: 2}
	if sum(o1) != sum(o2) {
		t.Error("хэш одной структуры нестабилен")
	}
	// Gen пропускается только у render.TextDraw, одноимённое поле чужой структуры — нет.
	type other struct{ Gen uint64 }
	if sum(other{Gen: 1}) == sum(other{Gen: 2}) {
		t.Error("Gen чужой структуры пропущен")
	}
	if sum(render.TextDraw{Text: "a", Gen: 1}) != sum(render.TextDraw{Text: "a", Gen: 2}) {
		t.Error("Gen у TextDraw не пропущен")
	}
}
