package packprep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Рецепт строгий: неизвестное поле, лишний объект, чужой режим, выход из -src, длинное about — отказ.
func TestRecipeStrict(t *testing.T) {
	if _, err := ParseRecipe(readFixture(t, "recipe-ok.json")); err != nil {
		t.Fatalf("рецепт из спеки не принят: %v", err)
	}
	bad := map[string]string{
		"recipe-unknown-field.json":      "unknown field",
		"recipe-unknown-item-field.json": "unknown field",
		"recipe-trailing.json":           "лишние данные",
		"recipe-src-escape.json":         "внутри -src",
		"recipe-src-abs.json":            "внутри -src",
		"recipe-mode-field.json":         `поле "grid" не годится для режима "bg"`,
		"recipe-license.json":            "CC0-1.0",
		"recipe-about-long.json":         "about: до 140 знаков",
		"recipe-bad-mode.json":           "mode",
		"recipe-skip-orphan.json":        "такого src в рецепте нет",
	}
	for file, want := range bad {
		_, err := ParseRecipe(readFixture(t, file))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: ждали ошибку с %q, получили %v", file, want, err)
		}
	}
}

// Каждое поле шапки и предмета проверяется: таблица нарушений, у каждого — своя ошибка.
func TestRecipeFields(t *testing.T) {
	good := Item{Src: "a/b.jpg", Mode: ModeBg, Out: "bg/a.jpg", Name: "Фон"}
	cases := []struct {
		name string
		mut  func(r *Recipe)
		want string
	}{
		{"id", func(r *Recipe) { r.ID = "Ab" }, "id"},
		{"version", func(r *Recipe) { r.Version = 0 }, "version"},
		{"minApp", func(r *Recipe) { r.MinApp = "0.26" }, "minApp"},
		{"theme", func(r *Recipe) { r.Theme = "zoo" }, "theme"},
		{"needs себя", func(r *Recipe) { r.Needs = r.ID }, "needs"},
		{"пустые items", func(r *Recipe) { r.Items = nil }, "items"},
		{"title пуст", func(r *Recipe) { r.Title = "" }, "title не задан"},
		{"пробелы по краям", func(r *Recipe) { r.Author = " Автор" }, "author"},
		{"управляющий знак", func(r *Recipe) { r.Source = "a\x00b" }, "source"},
		{"out без ext", func(r *Recipe) { r.Items[0].Out = "bg/a" }, "фон — .jpg или .png"},
		{"out с заглавными", func(r *Recipe) { r.Items[0].Out = "bg/A.jpg" }, "по правилам набора"},
		{"нет имени", func(r *Recipe) { r.Items[0].Name = "" }, "name не задан"},
		{"trim", func(r *Recipe) { r.Items[0].Trim = 9 }, "trim"},
		{"имя длиннее 60", func(r *Recipe) { r.Items[0].Name = strings.Repeat("я", 61) }, "до 60 знаков"},
		{"src не картинка", func(r *Recipe) { r.Items[0].Src = "a/b.gif" }, "PNG или JPEG"},
		{"thr", func(r *Recipe) {
			r.Items = []Item{{Src: "f.png", Mode: ModeFrame, Out: "frames/f.png", Name: "Р", Thr: 100}}
		}, "thr"},
		{"holes", func(r *Recipe) {
			r.Items = []Item{{Src: "f.png", Mode: ModeFrame, Out: "frames/f.png", Name: "Р", Holes: "most"}}
		}, "holes"},
		{"frame jpg", func(r *Recipe) { r.Items = []Item{{Src: "f.png", Mode: ModeFrame, Out: "frames/f.jpg", Name: "Р"}} }, "только .png"},
		{"sheet с name", func(r *Recipe) { r.Items = []Item{{Src: "s.png", Mode: ModeSheet, Out: "stickers/s-", Name: "Р"}} }, "names"},
		{"names и name", func(r *Recipe) {
			r.Items = []Item{{Src: "s.png", Mode: ModeRows, Out: "stickers/s-", Name: "Р", Names: []string{"А"}}}
		}, "name и names вместе"},
		{"пустое имя в names", func(r *Recipe) {
			r.Items = []Item{{Src: "s.png", Mode: ModeSheet, Out: "stickers/s-", Names: []string{"А", ""}}}
		}, "names[1] пусто"},
		{"kind у фона", func(r *Recipe) { r.Items[0].Kind = KindSticker }, `поле "kind"`},
		{"kind чужой", func(r *Recipe) {
			r.Items = []Item{{Src: "s.png", Mode: ModeRows, Out: "s-", Name: "Р", Kind: "frame"}}
		}, "kind"},
		{"svg не svg", func(r *Recipe) { r.Items = []Item{{Src: "s.png", Mode: ModeSvg, Out: "s-", Names: []string{"А"}}} }, "нужен .svg"},
		{"out префикс-папка", func(r *Recipe) {
			r.Items = []Item{{Src: "s.png", Mode: ModeSheet, Out: "stickers/", Names: []string{"А"}}}
		}, "префикс"},
		{"skip #0", func(r *Recipe) {
			r.Items = []Item{{Src: "s.png", Mode: ModeSheet, Out: "s-", Names: []string{"А"}}}
			r.Skip = map[string]string{"s.png#0": "брак"}
		}, "номер предмета"},
		{"skip без причины", func(r *Recipe) { r.Skip = map[string]string{"a/b.jpg": " "} }, "причина"},
	}
	for _, c := range cases {
		r := baseRecipe(good)
		c.mut(r)
		err := r.validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ждали %q, получили %v", c.name, c.want, err)
		}
	}
	if err := baseRecipe(good).validate(); err != nil {
		t.Fatalf("исходный рецепт теста негоден: %v", err)
	}
}

// Рецепт больше предела — отказ до разбора; пустой и мусор — ошибка, а не паника.
func TestRecipeBadInputs(t *testing.T) {
	for name, data := range map[string][]byte{
		"пустой":     nil,
		"мусор":      []byte("\x00\x01\x02"),
		"не объект":  []byte("[]"),
		"усечённый":  readFixture(t, "recipe-ok.json")[:200],
		"огромный":   append([]byte(`{"id":"`), make([]byte, MaxRecipeBytes)...),
		"null":       []byte("null"),
		"глубокий":   []byte(strings.Repeat("[", 5000)),
		"пустой объ": []byte("{}"),
	} {
		if _, err := ParseRecipe(data); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
	}
}

func FuzzParseRecipe(f *testing.F) {
	for _, n := range []string{"recipe-ok.json", "recipe-unknown-field.json", "recipe-trailing.json"} {
		b, err := os.ReadFile(filepath.Join("testdata", n))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte(`{"items":[{"mode":"svg","knock":[[1,2]],"split":[{"group":1,"x":2}]}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		r, err := ParseRecipe(data)
		if err == nil && r == nil {
			t.Fatal("ошибки нет, а рецепта нет")
		}
	})
}
