package export

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func TestBlankSpreads(t *testing.T) {
	v := model.View{Spreads: []model.SpreadView{
		{},                   // 1 пустой
		{Name: "Вступление"}, // 2 только название — не печатается
		{Texts: []model.TextView{{ID: "t"}}},
		{Backdrops: []model.BackdropView{{Scope: "spread", Photo: "f"}}},
		{Places: []model.PlaceView{{ID: "m"}}},
		{BgSet: [2]string{"", "#102030"}},
		{Photos: []string{"a"}},
		{Captions: []model.CaptionView{{}}},
		{}, // 9 пустой
		{Backdrops: []model.BackdropView{{Scope: "left", Photo: "g", Fade: 1}}}, // 10 фон не виден
		{Backdrops: []model.BackdropView{{Scope: "left", Photo: "g", Fade: 1}, {Scope: "right", Photo: "h", Fade: 0.99}}},
		{Backdrops: []model.BackdropView{{Scope: "left", Photo: "g", Fade: 1}}, BgSet: [2]string{"#fff", ""}},
	}}
	n := len(v.Spreads)
	all := NewPlan(nil, n, false)
	if got := BlankSpreads(v, &all); !slices.Equal(got, []int{1, 2, 9, 10}) {
		t.Fatalf("весь альбом: %v", got)
	}
	pages := NewPlan(nil, n, true)
	if got := BlankSpreads(v, &pages); !slices.Equal(got, []int{1, 2, 9, 10}) {
		t.Fatalf("по страницам: %v", got)
	}
	for _, tc := range []struct {
		rng   string
		pages bool
		want  []int
	}{
		{"2-8", false, []int{2}},
		{"3-8", false, nil},
		{"1п, 9", true, []int{1, 9}},
		{"5-9", true, []int{9}},
		{"10-12", false, []int{10}},
	} {
		units, err := ParseRange(tc.rng, n)
		if err != nil {
			t.Fatal(err)
		}
		pl := NewPlan(units, n, tc.pages)
		if got := BlankSpreads(v, &pl); !slices.Equal(got, tc.want) {
			t.Errorf("%q pages=%v: %v, ждали %v", tc.rng, tc.pages, got, tc.want)
		}
	}
}

// Узор (048) — печать, если виден и известен; прозрачный на 100 % и неизвестный генератор
// (файл из новой версии, рисуется пусто) лист не заполняют. Обложка — так же.
func TestBlankPatterns(t *testing.T) {
	pat := func(fade float64, known bool) model.PatternView {
		return model.PatternView{Scope: "left", Pat: model.Pattern{Kind: "dots", Fade: fade}, Known: known}
	}
	v := model.View{Spreads: []model.SpreadView{
		{Patterns: []model.PatternView{pat(0, true)}},
		{Patterns: []model.PatternView{pat(1, true)}},                  // 2 не виден
		{Patterns: []model.PatternView{pat(0, false)}},                 // 3 неизвестный
		{Patterns: []model.PatternView{pat(1, true), pat(0.99, true)}}, // правая видна
		{Patterns: []model.PatternView{}},                              // 5 пустой список
	}}
	pl := NewPlan(nil, len(v.Spreads), false)
	if got := BlankSpreads(v, &pl); !slices.Equal(got, []int{2, 3, 5}) {
		t.Fatalf("пустые: %v", got)
	}
	for i, sv := range v.Spreads {
		c := model.View{Cover: &model.CoverView{Sheet: sv}}
		if got, want := BlankCover(c), slices.Contains([]int{2, 3, 5}, i+1); got != want {
			t.Errorf("обложка с узорами листа %d: пустая %v, ждали %v", i+1, got, want)
		}
	}
}

// Стикер (048) — печать, если виден и рисуется: свой или известного генератора; прозрачный на
// 100 % и неизвестный генератор лист не заполняют. Обложка — так же.
func TestExportStickerNotBlank(t *testing.T) {
	st := func(fade float64, known bool, photo string) model.StickerView {
		return model.StickerView{ID: "k", Photo: photo, Fade: fade, Known: known, W: 10, H: 10}
	}
	v := model.View{Spreads: []model.SpreadView{
		{Stickers: []model.StickerView{st(0, true, "")}},
		{Stickers: []model.StickerView{st(1, true, "")}},    // 2 не виден
		{Stickers: []model.StickerView{st(0, false, "")}},   // 3 неизвестный
		{Stickers: []model.StickerView{st(0.5, true, "f")}}, // свой
		{Stickers: []model.StickerView{st(1, true, "f"), st(0, false, ""), st(0.99, true, "")}},
		{Stickers: []model.StickerView{}}, // 6 пустой список
	}}
	pl := NewPlan(nil, len(v.Spreads), false)
	if got := BlankSpreads(v, &pl); !slices.Equal(got, []int{2, 3, 6}) {
		t.Fatalf("пустые: %v", got)
	}
	for i, sv := range v.Spreads {
		c := model.View{Cover: &model.CoverView{Sheet: sv}}
		if got, want := BlankCover(c), slices.Contains([]int{2, 3, 6}, i+1); got != want {
			t.Errorf("обложка со стикерами листа %d: пустая %v, ждали %v", i+1, got, want)
		}
	}
}

func TestBlankWarning(t *testing.T) {
	for _, tc := range []struct {
		nums  []int
		white bool
		want  string
	}{
		{nil, true, ""},
		{[]int{5}, true, "Разворот 5 пустой — выгрузить белым?"},
		{[]int{5, 7}, true, "Развороты 5, 7 пустые — выгрузить белыми?"},
		{[]int{5}, false, "Разворот 5 пустой — выгрузить с фоном альбома?"},
		{[]int{5, 7}, false, "Развороты 5, 7 пустые — выгрузить с фоном альбома?"},
		{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, true,
			"Развороты 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 …и ещё 2 пустые — выгрузить белыми?"},
	} {
		if got := BlankWarning(tc.nums, tc.white); got != tc.want {
			t.Errorf("%v white=%v: %q, ждали %q", tc.nums, tc.white, got, tc.want)
		}
	}
}

func TestFieldsWarning(t *testing.T) {
	cases := []struct {
		unfilled []string
		unnamed  int
		want     string
	}{
		{nil, 0, ""},
		{[]string{"Год", "Школа"}, 3, "Не заполнены поля: Год, Школа; фото без имени: 3. В выгрузке они будут пустыми. Продолжить?"},
		{[]string{"Класс"}, 0, "Не заполнены поля: Класс. В выгрузке они будут пустыми. Продолжить?"},
		{nil, 2, "Фото без имени: 2. В выгрузке они будут пустыми. Продолжить?"},
	}
	for _, c := range cases {
		if got := FieldsWarning(c.unfilled, c.unnamed); got != c.want {
			t.Errorf("%v, %d: %q", c.unfilled, c.unnamed, got)
		}
	}
}

// PlanFields: только листы плана (развороты по маске, обложка — если в плане); повторы полей
// сливаются; пустой план и вид без листов — ничего.
func TestPlanFields(t *testing.T) {
	if u, n := PlanFields(model.View{}, &Plan{}); len(u) != 0 || n != 0 {
		t.Fatalf("пусто: %v %d", u, n)
	}
	v := model.View{
		Spreads: []model.SpreadView{{Unfilled: []string{"Год"}}, {Unfilled: []string{"Год", "Заголовок"}, Unnamed: 2}, {Unnamed: 1}},
		Cover:   &model.CoverView{Sheet: model.SpreadView{Unfilled: []string{"Руководитель"}, Unnamed: 4}},
	}
	all := NewPlan(nil, 3, false)
	if u, n := PlanFields(v, &all); !slices.Equal(u, []string{"Год", "Заголовок"}) || n != 3 {
		t.Fatalf("все развороты: %v %d", u, n)
	}
	part := NewPlan([]Unit{{3, 1}}, 3, false).WithCover(&CoverJob{})
	if u, n := PlanFields(v, &part); !slices.Equal(u, []string{"Руководитель"}) || n != 5 {
		t.Fatalf("страница 3 и обложка: %v %d", u, n)
	}
	only := Plan{}.WithCover(&CoverJob{})
	if u, n := PlanFields(model.View{Spreads: v.Spreads}, &only); len(u) != 0 || n != 0 {
		t.Fatalf("обложка в плане, а в виде её нет: %v %d", u, n)
	}
}
