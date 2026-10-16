package export

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Фото фона участвует в проверках экспорта: разворот — всегда, страница — если фон заходит на неё.
func TestSelectedPhotosBackdrops(t *testing.T) {
	const pageW = 300.0
	area := func(scope string) layout.Rect {
		switch scope {
		case "left":
			return layout.Rect{W: pageW, H: 300}
		case "right":
			return layout.Rect{X: pageW, W: pageW, H: 300}
		}
		return layout.Rect{W: 2 * pageW, H: 300}
	}
	bd := func(scope, id string, x, w, rot float64) model.BackdropView {
		return model.BackdropView{Scope: scope, Photo: id, Area: area(scope), Box: model.BackdropBox{X: x, Y: 0, W: w, H: 100, Rot: rot}}
	}
	v := model.View{Spreads: []model.SpreadView{
		{Photos: []string{"p1"}, Rects: []layout.Rect{{X: 10, W: 50, H: 50}}, Backdrops: []model.BackdropView{bd("spread", "wide", 100, 150, 0)}},
		{Photos: []string{"p2"}, Rects: []layout.Rect{{X: 10, W: 50, H: 50}}, Backdrops: []model.BackdropView{bd("right", "r", 400, 100, 0)}},
		// 90° вокруг угла: рамка уходит влево от x=320 до 220, область правой страницы режет её по 300.
		{Backdrops: []model.BackdropView{bd("right", "rot", 320, 200, 90)}},
		// 90° вокруг x=350: описанный прямоугольник 250..350 — обе страницы.
		{Backdrops: []model.BackdropView{bd("spread", "cross", 350, 100, 90)}},
	}}
	pl := func(spec string) Plan {
		u, err := ParseRange(spec, 4)
		if err != nil {
			t.Fatal(err)
		}
		return NewPlan(u, 4, true)
	}
	has := func(m map[string]bool, ids ...string) bool {
		if len(m) != len(ids) {
			return false
		}
		for _, id := range ids {
			if !m[id] {
				return false
			}
		}
		return true
	}

	if got := NewPlan(nil, 4, false).SelectedPhotos(v, pageW); !has(got, "p1", "wide", "p2", "r", "rot", "cross") {
		t.Errorf("все развороты: %v", got)
	}
	if got := pl("1").SelectedPhotos(v, pageW); !has(got, "p1", "wide") {
		t.Errorf("разворот 1: %v", got)
	}
	// Страница 1л: место p1 (x=10) и фон «wide» (100..250 < 300).
	if got := pl("1л").SelectedPhotos(v, pageW); !has(got, "p1", "wide") {
		t.Errorf("1л: %v", got)
	}
	// Страница 1п: фон «wide» кончается на 250 — на правую не заходит.
	if got := pl("1п").SelectedPhotos(v, pageW); len(got) != 0 {
		t.Errorf("1п: %v", got)
	}
	// 2л: место p2 (x=10), фон правой страницы не заходит; 2п: только фон «r».
	if got := pl("2л").SelectedPhotos(v, pageW); !has(got, "p2") {
		t.Errorf("2л: %v", got)
	}
	if got := pl("2п").SelectedPhotos(v, pageW); !has(got, "r") {
		t.Errorf("2п: %v", got)
	}
	// 3: фон правой страницы с поворотом, обрезанный областью, левую не задевает.
	if got := pl("3л").SelectedPhotos(v, pageW); len(got) != 0 {
		t.Errorf("3л: %v", got)
	}
	if got := pl("3п").SelectedPhotos(v, pageW); !has(got, "rot") {
		t.Errorf("3п: %v", got)
	}
	for _, s := range []string{"4л", "4п"} {
		if got := pl(s).SelectedPhotos(v, pageW); !has(got, "cross") {
			t.Errorf("%s: %v", s, got)
		}
	}
}

// Фон с прозрачностью 100 % не печатается и не читается — в выборку не входит; 99 % — входит.
func TestSelectedPhotosFadedBackdrop(t *testing.T) {
	bd := func(id string, fade float64) model.BackdropView {
		return model.BackdropView{Scope: "spread", Photo: id, Fade: fade,
			Area: layout.Rect{W: 600, H: 300}, Box: model.BackdropBox{W: 600, H: 300}}
	}
	v := model.View{Spreads: []model.SpreadView{
		{Backdrops: []model.BackdropView{bd("hidden", 1), bd("seen", 0.99)}},
	}}
	got := NewPlan(nil, 1, false).SelectedPhotos(v, 300)
	if got["hidden"] || !got["seen"] || len(got) != 1 {
		t.Fatalf("выборка: %v", got)
	}
}

// «По страницам»: холст 2·px(PageW) на пиксель шире обычного px(2·PageW); на границе лимита
// стороны (65535) отказ должен прийти до выбора папки, а не из RunJobs.
func TestCheckCanvasesSplit(t *testing.T) {
	opt := render.Options{PageW: 2774.32, PageH: 100, DPI: 300} // px(2·PageW) = 65535, 2·px(PageW) = 65536
	if err := CheckCanvases(opt, false); err != nil {
		t.Fatalf("обычный холст: %v", err)
	}
	if err := CheckCanvases(opt, true); err == nil {
		t.Fatal("по страницам: ждали отказ (холст 65536 px)")
	}
}

func selectProject() *model.Project {
	return &model.Project{Photos: map[string]model.PhotoRef{
		"a": {Path: "/x/a.jpg", W: 10, H: 10},
		"b": {Path: "/x/b.jpg"},
		"c": {Path: "/x/c.jpg"},
		"d": {Path: "/x/d.jpg"},
		"e": {Path: "/x/e.jpg"},
	}}
}

// PickPhotos берёт только выбранные и известные проекту ID; сам проект не трогает.
func TestPickPhotos(t *testing.T) {
	p := selectProject()
	sub := PickPhotos(p, map[string]bool{"a": true, "c": true, "нет": true})
	if len(sub.Photos) != 2 || sub.Photos["a"].W != 10 || sub.Photos["c"].Path != "/x/c.jpg" {
		t.Errorf("выбор: %+v", sub.Photos)
	}
	if len(p.Photos) != 5 {
		t.Errorf("исходный проект изменён: %d фото", len(p.Photos))
	}
	if sub := PickPhotos(p, nil); sub.Photos == nil || len(sub.Photos) != 0 {
		t.Errorf("пустой выбор: %+v", sub.Photos)
	}
}

// MissingPhotos: все на месте — nil; пропавших до трёх — по именам; больше — с «и ещё N»;
// пропавшее вне выбора не мешает.
func TestMissingPhotos(t *testing.T) {
	p := selectProject()
	all := map[string]bool{"a": true, "b": true, "c": true, "d": true, "e": true}

	if err := MissingPhotos(p, all, func(string) bool { return true }); err != nil {
		t.Errorf("всё на месте: %v", err)
	}
	only := func(gone ...string) func(string) bool {
		return func(path string) bool {
			for _, g := range gone {
				if path == "/x/"+g+".jpg" {
					return false
				}
			}
			return true
		}
	}
	err := MissingPhotos(p, all, only("b"))
	if err == nil || !strings.Contains(err.Error(), "фото: 1 (b.jpg)") || strings.Contains(err.Error(), "ещё") {
		t.Errorf("одно пропавшее: %v", err)
	}
	err = MissingPhotos(p, all, only("a", "b", "c", "d", "e"))
	if err == nil || !strings.Contains(err.Error(), "фото: 5 (a.jpg, b.jpg, c.jpg …и ещё 2)") {
		t.Errorf("пять пропавших: %v", err)
	}
	if err := MissingPhotos(p, map[string]bool{"a": true}, only("b")); err != nil {
		t.Errorf("пропавшее вне выбора: %v", err)
	}
}

// Ошибка пропавших фото называет реальный пункт меню (U21).
func TestMissingPhotosNamesMenu(t *testing.T) {
	p := selectProject()
	all := map[string]bool{"a": true, "b": true}
	err := MissingPhotos(p, all, func(string) bool { return false })
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	// Подписи — как пункт «Файл» → «Найти пропавшие фото…» в frontend/src/shell/menu/menu.ts.
	for _, want := range []string{"Файл ▸ Найти пропавшие фото…", "повторите выгрузку"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в ошибке нет %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Недостающие") {
		t.Errorf("устаревший пункт меню: %v", err)
	}
}

// Свои из библиотеки (048): «Найти пропавшие фото…» их не ищет — отдельная строка без имени
// файла (путь пуст), с подсказкой загрузить те же файлы; вместе с обычными — обе строки.
func TestMissingPhotosLib(t *testing.T) {
	p := selectProject()
	p.Photos["k1"] = model.PhotoRef{Lib: "my:0123456789abcdef", W: 4, H: 3}
	p.Photos["k2"] = model.PhotoRef{Lib: "my:fedcba9876543210", Path: "/lib/k2.jpg", W: 4, H: 3}
	// exists("") — true: пустой путь своего пропавшим делает MissingPhotos, а не проверка файла.
	gone := func(paths ...string) func(string) bool {
		return func(path string) bool { return !slices.Contains(paths, path) }
	}

	err := MissingPhotos(p, map[string]bool{"a": true, "k1": true}, gone())
	if err == nil || !strings.Contains(err.Error(), "нет в библиотеке этого компьютера: 1 (свой фон)") ||
		strings.Contains(err.Error(), "не найдены файлы") || strings.Contains(err.Error(), "Найти пропавшие") ||
		!strings.HasSuffix(err.Error(), "повторите выгрузку") {
		t.Fatalf("один свой: %v", err)
	}
	err = MissingPhotos(p, map[string]bool{"b": true, "k1": true, "k2": true}, gone("/x/b.jpg", "/lib/k2.jpg"))
	if err == nil || !strings.Contains(err.Error(), "не найдены файлы фото: 1 (b.jpg); найдите их: Файл ▸ Найти пропавшие фото…") ||
		!strings.Contains(err.Error(), "нет в библиотеке этого компьютера: 2 (свои фоны)") || strings.Contains(err.Error(), "k2.jpg") {
		t.Fatalf("обычное и два своих: %v", err)
	}
	if err := MissingPhotos(p, map[string]bool{"a": true, "k2": true}, gone()); err != nil {
		t.Fatalf("свой на месте: %v", err)
	}
}

// Свои стикеры (048) — в выгрузке, как фото фона: страница — если описанная рамка заходит на
// неё (поворот вокруг центра), прозрачный на 100 % не в счёт, процедурный фото не даёт;
// обложка — только при обложке в плане.
func TestSelectedPhotosStickers(t *testing.T) {
	const pageW = 300.0
	own := func(id string, x, w, rot, fade float64) model.StickerView {
		return model.StickerView{ID: "k" + id, Photo: id, X: x, Y: 10, W: w, H: 40, Rot: rot, Fade: fade, Known: true}
	}
	v := model.View{Spreads: []model.SpreadView{
		{Stickers: []model.StickerView{own("left", 10, 50, 0, 0), own("hidden", 400, 50, 0, 1),
			{ID: "art", Art: &model.Art{Kind: "star"}, Gen: true, Known: true, X: 400, W: 50, H: 50}}},
		{Stickers: []model.StickerView{own("cross", 280, 40, 0, 0)}}, // 280..320 — через шов
		// Рамка 280..330 через шов, под 90° вокруг центра (305) описанная — 285..325: обе страницы;
		// рамка 250..299 под 90° — 254,5..294,5: только левая.
		{Stickers: []model.StickerView{own("rot", 280, 50, 90, 0), own("rotl", 250, 49, 90, 0)}},
	}, Cover: &model.CoverView{Sheet: model.SpreadView{Stickers: []model.StickerView{own("cv", 10, 20, 0, 0), own("cvh", 10, 20, 0, 1)}}}}
	pl := func(spec string) Plan {
		u, err := ParseRange(spec, 3)
		if err != nil {
			t.Fatal(err)
		}
		return NewPlan(u, 3, true)
	}
	keys := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	for _, c := range []struct {
		name string
		plan Plan
		want []string
	}{
		{"все", NewPlan(nil, 3, false), []string{"cross", "left", "rot", "rotl"}},
		{"1п", pl("1п"), nil},
		{"1л", pl("1л"), []string{"left"}},
		{"2л", pl("2л"), []string{"cross"}},
		{"2п", pl("2п"), []string{"cross"}},
		{"3л", pl("3л"), []string{"rot", "rotl"}},
		{"3п", pl("3п"), []string{"rot"}},
		{"с обложкой", NewPlan(nil, 3, false).WithCover(&CoverJob{}), []string{"cross", "cv", "left", "rot", "rotl"}},
	} {
		if got := keys(c.plan.SelectedPhotos(v, pageW)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}

// Пропавший свой стикер: строка про библиотеку называет стикер и галерею стикеров; со своим
// фоном — оба вида и обе галереи; стикер обложки — тоже стикер.
func TestMissingPhotosLibSticker(t *testing.T) {
	p := selectProject()
	p.Photos["k1"] = model.PhotoRef{Lib: "my:0123456789abcdef", W: 4, H: 3}
	p.Photos["k2"] = model.PhotoRef{Lib: "my:fedcba9876543210", W: 4, H: 3}
	p.Photos["k3"] = model.PhotoRef{Lib: "my:00000000000000aa", W: 4, H: 3}
	p.Spreads = []*model.Spread{{ID: "s1", BgPhoto: &model.Backdrop{Photo: "k1"},
		Stickers: []*model.Sticker{{ID: "st1", Photo: "k2"}, {ID: "st2", Art: &model.Art{Kind: "star"}}}}}
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Stickers: []*model.Sticker{{ID: "st3", Photo: "k3"}}}}
	for _, c := range []struct {
		ids  []string
		want string
	}{
		{[]string{"k2"}, "нет в библиотеке этого компьютера: 1 (свой стикер); загрузите те же файлы в галерею стикеров, они найдутся сами"},
		{[]string{"k2", "k3"}, "нет в библиотеке этого компьютера: 2 (свои стикеры); загрузите те же файлы в галерею стикеров"},
		{[]string{"k1", "k2", "a"}, "нет в библиотеке этого компьютера: 2 (свои фоны и стикеры); загрузите те же файлы в галереи фонов и стикеров"},
		{[]string{"k1"}, "нет в библиотеке этого компьютера: 1 (свой фон); загрузите те же файлы в галерею фонов"},
	} {
		sel := map[string]bool{}
		for _, id := range c.ids {
			sel[id] = true
		}
		if err := MissingPhotos(p, sel, func(string) bool { return true }); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v", c.ids, err)
		}
	}
}
