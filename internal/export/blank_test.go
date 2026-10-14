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
