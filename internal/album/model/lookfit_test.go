package model

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгонка шага по страницам (165 Т1): поле Fit, зажим, LookBoxes, PageFrame.

func fitSample(fit string, pages int) SpreadTpl {
	t := SpreadTpl{V: TplV, Pages: pages, Ratio: 2, Fit: fit, Page: make([]TplBg, pages),
		Places: []Box{{X: 0.04, Y: 0.08, W: 0.4, H: 0.84}, {X: 0.56, Y: 0.1, W: 0.4, H: 0.4}}}
	t.Page[0].Color = "#ffffff"
	return t
}

func TestClampSpreadTplFit(t *testing.T) {
	cases := []struct {
		name  string
		fit   string
		pages int
		want  string
	}{
		{"page у разворота", FitPage, 2, FitPage},
		{"пусто", "", 2, ""},
		{"неизвестное", "spread", 2, ""},
		{"регистр", "Page", 2, ""},
		{"страница", FitPage, 1, ""},
	}
	for _, c := range cases {
		got, ok := ClampSpreadTpl(fitSample(c.fit, c.pages))
		if !ok || got.Fit != c.want {
			t.Errorf("%s: fit %q (%v), ждали %q", c.name, got.Fit, ok, c.want)
		}
	}
	// Круг JSON: поле переживает запись и чтение шаблона; без поля читается как "".
	in, _ := ClampSpreadTpl(fitSample(FitPage, 2))
	data, err := json.Marshal(in)
	if err != nil || !strings.Contains(string(data), `"fit":"page"`) {
		t.Fatalf("запись: %s %v", data, err)
	}
	back, err := ParseSpreadTpl(data)
	if err != nil || !reflect.DeepEqual(back, in) {
		t.Errorf("круг: %+v %v", back, err)
	}
	old, _ := ClampSpreadTpl(fitSample("", 2))
	data, _ = json.Marshal(old)
	if strings.Contains(string(data), "fit") {
		t.Errorf("пустой fit попал в файл: %s", data)
	}
	if back, err = ParseSpreadTpl(data); err != nil || back.Fit != "" {
		t.Errorf("без поля: %+v %v", back, err)
	}
	// Ключ старого шаблона в библиотеке прежний (эталон до правки).
	if got := TplKey(old); got != "my:d0f3b83986b24551" {
		t.Errorf("ключ старого шаблона изменился: %s", got)
	}
	if TplKey(in) == TplKey(old) {
		t.Error("шаблон с fit и без — один ключ")
	}
}

func TestLookBoxesFitPage(t *testing.T) {
	for _, r := range []float64{0.5, 1, 1.5, 2, 3} {
		fit, old := fitSample(FitPage, 2), fitSample("", 2)
		if got := LookBoxes(fit, r); !reflect.DeepEqual(got, fit.Places) {
			t.Errorf("r=%v: места fit изменились: %v", r, got)
		}
		if r != 2 && reflect.DeepEqual(LookBoxes(old, r), old.Places) {
			t.Errorf("r=%v: места без fit не подогнаны", r)
		}
	}
	// Копия, не тот же срез.
	fit := fitSample(FitPage, 2)
	if got := LookBoxes(fit, 1); &got[0] == &fit.Places[0] {
		t.Error("LookBoxes отдал срез шаблона")
	}
	// Панорама: место через сгиб — подгонка по-старому.
	pano := fitSample(FitPage, 2)
	pano.Places = []Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}
	if pano.ByPage() || reflect.DeepEqual(LookBoxes(pano, 1), pano.Places) {
		t.Error("панорама должна подгоняться по-старому")
	}
	// Страница не подгоняется по страницам.
	if one := fitSample(FitPage, 1); one.ByPage() {
		t.Error("ByPage у шаблона страницы")
	}
}

func TestPageFrame(t *testing.T) {
	tp := SpreadTpl{Ratio: 2}
	for _, c := range []struct {
		page  layout.Rect
		want  layout.Rect
		wantK float64
		name  string
	}{
		{page: layout.Rect{X: 0, Y: 0, W: 200, H: 300}, want: layout.Rect{X: 0, Y: 50, W: 200, H: 200}, wantK: 200, name: "20×30"},
		{page: layout.Rect{X: 400, Y: 0, W: 400, H: 300}, want: layout.Rect{X: 450, Y: 0, W: 300, H: 300}, wantK: 300, name: "правая 40×30"},
		{page: layout.Rect{X: 0, Y: 0, W: 300, H: 200}, want: layout.Rect{X: 50, Y: 0, W: 200, H: 200}, wantK: 200, name: "30×20 слева"},
		{page: layout.Rect{X: 0, Y: 0, W: 250, H: 250}, want: layout.Rect{X: 0, Y: 0, W: 250, H: 250}, wantK: 250, name: "квадрат"},
	} {
		fr, k := PageFrame(tp, c.page)
		if math.Abs(k-c.wantK) > 1e-9 || math.Abs(fr.X-c.want.X) > 1e-9 || math.Abs(fr.Y-c.want.Y) > 1e-9 ||
			math.Abs(fr.W-c.want.W) > 1e-9 || math.Abs(fr.H-c.want.H) > 1e-9 {
			t.Errorf("%s: %+v k=%v, ждали %+v k=%v", c.name, fr, k, c.want, c.wantK)
		}
	}
	// Край: Ratio не число, ≤ 0, пустая страница — кадр равен странице, без паники.
	pg := layout.Rect{X: 5, Y: 6, W: 100, H: 80}
	for _, r := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if fr, k := PageFrame(SpreadTpl{Ratio: r}, pg); fr != pg || k != 80 {
			t.Errorf("Ratio %v: %+v %v", r, fr, k)
		}
	}
	if fr, k := PageFrame(tp, layout.Rect{}); fr != (layout.Rect{}) || k != 0 {
		t.Errorf("пустая страница: %+v %v", fr, k)
	}
}

// TestLookTpl: решение ByPage принимается по исходным местам — панорама теряет Fit (в том числе
// заход чуть больше допуска, который старая подгонка сжимает под допуск), обычный шаг остаётся
// как есть; шаг-страница и пустые места не падают.
func TestLookTpl(t *testing.T) {
	pan := fitSample(FitPage, 2)
	pan.Places = []Box{{X: 0.3, Y: 0.1, W: 0.2 + 1.5*FoldEps, H: 0.5}}
	if pan.ByPage() {
		t.Fatal("заход больше допуска — панорама")
	}
	if got := LookTpl(pan, 3); got.Fit != "" || !reflect.DeepEqual(got.Places, LookBoxes(pan, 3)) {
		t.Errorf("панорама: fit %q, места %+v", got.Fit, got.Places)
	}
	by := fitSample(FitPage, 2)
	if got := LookTpl(by, 3); got.Fit != FitPage || !reflect.DeepEqual(got.Places, by.Places) {
		t.Errorf("по страницам: fit %q, места %+v", got.Fit, got.Places)
	}
	if got := LookTpl(fitSample("", 2), 3); got.Fit != "" || reflect.DeepEqual(got.Places, by.Places) {
		t.Errorf("по-старому: fit %q, места %+v (ждали подгонку LookBoxes)", got.Fit, got.Places)
	}
	empty := fitSample(FitPage, 2)
	empty.Places = nil
	if got := LookTpl(empty, 3); got.Fit != FitPage || len(got.Places) != 0 {
		t.Errorf("без мест: %+v", got)
	}
	if got := LookTpl(fitSample(FitPage, 1), 3); got.Fit != "" {
		t.Errorf("страница: fit %q", got.Fit)
	}
}
