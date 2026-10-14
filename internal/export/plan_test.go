package export

import (
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
