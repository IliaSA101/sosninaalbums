package decor

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// style из файла макета (163): в плитку идёт зажатым — длина и управляющие знаки; нет поля — "";
// свой макет со стилем переживает круг SaveLook → Open.
func TestLookStyleField(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 40, 30)
	files := map[string]string{
		"a": `"style":"  Акварель "`,
		"b": `"style":"` + strings.Repeat("Ж", 100) + `"`,
		"c": "\"style\":\"Мусор\\u0000\\u202e\\n\\tТекст\"",
		"e": "",
	}
	its := []pit{{"bg/a.png", bg}}
	for k, extra := range files {
		l := lookOf(libKey(bg), "")
		l.Steps[0].Name = "Шаг " + k // разные байты — разные ключи
		its = append(its, pit{"looks/" + k + ".json", lookBytes(t, l, extra)})
	}
	m, src := testPack("styled", 1, its...)
	install(t, s, m, src, nil)
	cat, err := s.Catalog(KindAlbum, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range cat.Items {
		if it.PackID == "styled" {
			got = append(got, it.Style)
		}
	}
	slices.Sort(got)
	want := []string{"", "Акварель", strings.Repeat("Ж", model.MaxLookStyleRunes), "Мусор  Текст"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("styles %q, ждали %q", got, want)
	}
	// Свой макет со стилем: круг SaveLook → перезапуск → каталог.
	l := lookOf(libKey(bg), "")
	l.Style = "Графика"
	it, err := s.SaveLook("mine", "Свой", l)
	if err != nil {
		t.Fatal(err)
	}
	if it.Style != "Графика" {
		t.Errorf("SaveLook: style %q", it.Style)
	}
	if got, ok := findItem(t, Open(cfg), KindAlbum, it.ID); !ok || got.Style != "Графика" {
		t.Errorf("после перезапуска: %+v", got)
	}
}
