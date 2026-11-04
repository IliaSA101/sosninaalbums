package export

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Шрифты, которых нет на этом компьютере (082 п. 8): по листам плана, непустым текстам и подписям,
// с названиями из альбома; встроенные и зарегистрированные свои вопроса не дают.
func TestPlanFonts(t *testing.T) {
	lato, bodoni, anon, quiet := text.FamilyID("Lato"), text.FamilyID("Bodoni"), text.FamilyID("x"), text.FamilyID("y")
	tv := func(font, s string) model.TextView {
		return model.TextView{Text: s, Style: model.TextStyle{Font: font}}
	}
	v := model.View{
		Spreads: []model.SpreadView{
			{Texts: []model.TextView{tv(lato, "Анна"), tv("montserrat", "Павел"), tv(quiet, "  ")}},
			{Captions: []model.CaptionView{{TextView: tv(bodoni, "Подпись")}}, Texts: []model.TextView{tv(lato, "ещё")}},
			{Texts: []model.TextView{tv(anon, "без названия")}},
		},
		Cover:      &model.CoverView{Sheet: model.SpreadView{Texts: []model.TextView{tv(bodoni, "Обложка")}}},
		AlbumFonts: []model.FontUse{{ID: lato, Name: "Lato"}, {ID: bodoni, Name: "Bodoni"}, {ID: anon}},
	}
	full := NewPlan(nil, 3, false)
	if got := PlanFonts(v, &full); !slices.Equal(got, []string{"Bodoni", "Lato", "Свой шрифт"}) {
		t.Fatalf("все листы: %q", got)
	}
	units, err := ParseRange("1", 3)
	if err != nil {
		t.Fatal(err)
	}
	one := NewPlan(units, 3, false)
	if got := PlanFonts(v, &one); !slices.Equal(got, []string{"Lato"}) {
		t.Fatalf("лист 1: %q", got)
	}
	cover := Plan{mask: map[int]uint8{}}.WithCover(&CoverJob{})
	if got := PlanFonts(v, &cover); !slices.Equal(got, []string{"Bodoni"}) {
		t.Fatalf("только обложка: %q", got)
	}
	if got := PlanFonts(model.View{}, &Plan{}); len(got) != 0 {
		t.Fatalf("пустой вид: %q", got)
	}
}

func TestFontsWarning(t *testing.T) {
	cases := []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{"Lato"}, "В выгрузке нет шрифтов: „Lato“. Текст будет шрифтом Montserrat. Выгрузить так?"},
		{[]string{"Bodoni Moda", "Lato"}, "В выгрузке нет шрифтов: „Bodoni Moda“, „Lato“. Текст будет шрифтом Montserrat. Выгрузить так?"},
	}
	for _, c := range cases {
		if got := FontsWarning(c.names); got != c.want {
			t.Errorf("%q: %q", c.names, got)
		}
	}
}
