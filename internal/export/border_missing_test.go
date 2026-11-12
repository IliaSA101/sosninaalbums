package export

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Файлы печати рамочек-картинок: проверка до рендера (том не отвечает — как пропавший файл).

func borderPlace(photo, file string) model.PlaceView {
	return model.PlaceView{Photo: photo, X: 10, Y: 10, W: 20, H: 20,
		Border: &model.BorderView{Pic: true, Known: true, File: file}}
}

// existsOnly — exists, для которого «на месте» только перечисленные файлы; calls считает вызовы.
func existsOnly(calls *int, ok ...string) func(string) bool {
	return func(p string) bool {
		*calls++
		for _, o := range ok {
			if o == p {
				return true
			}
		}
		return false
	}
}

func TestMissingBorderFiles(t *testing.T) {
	v := model.View{Spreads: []model.SpreadView{
		{Places: []model.PlaceView{borderPlace("a", "/lib/f1.png"), borderPlace("b", "/lib/f2.png"), borderPlace("c", "/lib/f2.png")}},
		{Places: []model.PlaceView{borderPlace("d", "/lib/f3.png"), borderPlace("", "/lib/empty.png")}},
	}}
	full := NewPlan(nil, 2, false)

	// Все на месте — тишина; общий файл двух мест проверяется один раз, пустое место — не файл.
	calls := 0
	if err := full.MissingBorderFiles(v, 100, existsOnly(&calls, "/lib/f1.png", "/lib/f2.png", "/lib/f3.png")); err != nil {
		t.Fatalf("все на месте: %v", err)
	}
	if calls != 3 {
		t.Errorf("проб файлов %d, ждали 3", calls)
	}

	// Два пропавших из трёх (том не отвечает) — оба в одной ошибке.
	calls = 0
	err := full.MissingBorderFiles(v, 100, existsOnly(&calls, "/lib/f2.png"))
	if err == nil || !strings.Contains(err.Error(), "f1.png") || !strings.Contains(err.Error(), "f3.png") ||
		strings.Contains(err.Error(), "f2.png") || strings.Contains(err.Error(), "empty.png") || !strings.Contains(err.Error(), ": 2 ") {
		t.Errorf("два пропавших: %v", err)
	}

	// Выгружается только второй разворот: файл первого не проверяется.
	only2 := NewPlan([]Unit{{2, 0}}, 2, false)
	calls = 0
	if err := only2.MissingBorderFiles(v, 100, existsOnly(&calls, "/lib/f3.png")); err != nil {
		t.Errorf("первый разворот не выгружается: %v", err)
	}
	if calls != 1 {
		t.Errorf("проб файлов %d, ждали 1", calls)
	}
}

func TestMissingBorderFilesEdges(t *testing.T) {
	never := func(string) bool { t.Error("проба файла"); return false }
	// Пустой вид, лист без рамочек, рамочка не картинка, рамочка без файла (запись пропала — это дело MissingPhotos).
	plain := model.PlaceView{Photo: "a", Border: &model.BorderView{Known: true}}
	v := model.View{Spreads: []model.SpreadView{{}, {Places: []model.PlaceView{plain, borderPlace("b", ""), {Photo: "c"}}}}}
	if err := NewPlan(nil, 2, false).MissingBorderFiles(v, 100, never); err != nil {
		t.Errorf("нечего проверять: %v", err)
	}
	if err := NewPlan(nil, 0, false).MissingBorderFiles(model.View{}, 100, never); err != nil {
		t.Errorf("пустой вид: %v", err)
	}

	// Обложка проверяется, только если она в плане; больше трёх — «…и ещё».
	cover := model.SpreadView{}
	for _, f := range []string{"/l/1.png", "/l/2.png", "/l/3.png", "/l/4.png", "/l/5.png"} {
		cover.Places = append(cover.Places, borderPlace("p", f))
	}
	vc := model.View{Cover: &model.CoverView{Sheet: cover}}
	calls := 0
	if err := NewPlan(nil, 0, false).MissingBorderFiles(vc, 100, existsOnly(&calls)); err != nil || calls != 0 {
		t.Errorf("обложки нет в плане: %v, проб %d", err, calls)
	}
	err := NewPlan(nil, 0, false).WithCover(&CoverJob{}).MissingBorderFiles(vc, 100, existsOnly(&calls))
	if err == nil || !strings.Contains(err.Error(), "…и ещё 2") || !strings.Contains(err.Error(), ": 5 ") {
		t.Errorf("обложка, пять пропавших: %v", err)
	}
}
