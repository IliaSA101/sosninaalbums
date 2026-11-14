package packprep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func order(n int) *int { return &n }

// themedRecipe — два листа с двумя темами на разных базах (как holiday-birthday, 170-18) и лист без темы набора.
func themedRecipe() *Recipe {
	rec := baseRecipe(
		Item{Src: "s.png", Mode: ModeSheet, Out: "st/a-", Names: []string{"Шар", "Торт"}, Grid: "2x1", Theme: "birthday"},
		Item{Src: "s2.png", Mode: ModeSheet, Out: "st/b-", Names: []string{"Корона", "Леденец"}, Grid: "2x1", Theme: "kidsparty"},
		Item{Src: "s3.png", Mode: ModeSheet, Out: "st/c-", Names: []string{"Диск", "Квадрат"}, Grid: "2x1"},
	)
	rec.Themes = []ThemeDecl{
		{ID: "birthday", Name: "День рождения", Base: "holiday", Order: order(10)},
		{ID: "kidsparty", Name: "Детский праздник", Base: "kids"},
	}
	return rec
}

func themedSrc(t *testing.T) string {
	t.Helper()
	src := tempDir(t)
	for _, f := range []string{"s.png", "s2.png", "s3.png"} {
		writePNG(t, filepath.Join(src, f), stickerSheet())
	}
	return src
}

// Объявленная тема: в theme предмета идёт база, в topic — id; манифест получает themes с умолчаниями;
// предмет со встроенной темой набора topic не получает; отчёт считает предметы по темам.
func TestRecipeThemes(t *testing.T) {
	rep, out := run(t, themedSrc(t), themedRecipe())
	if !rep.OK {
		t.Fatalf("ошибки: %v", rep.Errors)
	}
	var m struct {
		Themes []struct {
			ID, Name, Base string
			Order          int
		}
		Items []struct{ File, Theme, Topic string }
	}
	if err := json.Unmarshal(readOut(t, out, "manifest.src.json"), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Themes) != 2 || m.Themes[0].ID != "birthday" || m.Themes[0].Base != "holiday" || m.Themes[0].Order != 10 ||
		m.Themes[1].Base != "kids" || m.Themes[1].Order != themeOrderDef {
		t.Fatalf("themes в манифесте: %+v", m.Themes)
	}
	want := map[string][2]string{"st/a-01.png": {"holiday", "birthday"}, "st/b-01.png": {"kids", "kidsparty"}, "st/c-01.png": {"school", ""}}
	got := map[string][2]string{}
	for _, it := range m.Items {
		got[it.File] = [2]string{it.Theme, it.Topic}
	}
	for file, w := range want {
		if got[file] != w {
			t.Errorf("%s: theme/topic %v, ждали %v", file, got[file], w)
		}
	}
	if rep.Themes["birthday"] != 2 || rep.Themes["kidsparty"] != 2 || len(rep.Themes) != 2 {
		t.Errorf("счёт по темам: %v", rep.Themes)
	}
	if !strings.Contains(rep.Text(), "тема набора «birthday»: предметов 2") {
		t.Errorf("в тексте отчёта нет строки темы:\n%s", rep.Text())
	}
	// Прогон повторяется байт в байт: порядок объявлений — как в рецепте.
	_, out2 := run(t, themedSrc(t), themedRecipe())
	if string(readOut(t, out, "manifest.src.json")) != string(readOut(t, out2, "manifest.src.json")) {
		t.Error("manifest.src.json разный между прогонами")
	}
}

// Без объявлений манифест как раньше: нет ни themes, ни topic.
func TestRecipeNoThemes(t *testing.T) {
	src := tempDir(t)
	writePNG(t, filepath.Join(src, "s.png"), stickerSheet())
	_, out := run(t, src, baseRecipe(Item{Src: "s.png", Mode: ModeSheet, Out: "st/a-", Names: []string{"Шар", "Торт"}, Grid: "2x1"}))
	man := string(readOut(t, out, "manifest.src.json"))
	if strings.Contains(man, `"themes"`) || strings.Contains(man, `"topic"`) {
		t.Errorf("поля тем без объявлений:\n%s", man)
	}
}

// Нарушения правил объявления и ссылок — отказ рецепту, все сразу.
func TestRecipeThemesRefused(t *testing.T) {
	cases := []struct {
		name string
		mut  func(r *Recipe)
		want string
	}{
		{"тема без предметов", func(r *Recipe) { r.Themes = append(r.Themes, ThemeDecl{ID: "extra", Name: "Лишняя"}) }, `"extra" объявлена, но ни один`},
		{"предмет с неизвестной темой", func(r *Recipe) { r.Items[0].Theme = "ghost" }, `theme "ghost"`},
		{"17 объявлений", func(r *Recipe) {
			for i := range 15 {
				r.Themes = append(r.Themes, ThemeDecl{ID: strings.Repeat("a", i+1), Name: "Тема " + strings.Repeat("я", i+1)})
			}
		}, "не больше 16"},
		{"встроенный id", func(r *Recipe) { r.Themes[0].ID = "holiday" }, "не встроенная и не служебная"},
		{"служебный id", func(r *Recipe) { r.Themes[0].ID = "constructor" }, "не встроенная и не служебная"},
		{"id с цифрой", func(r *Recipe) { r.Themes[0].ID = "year2" }, "^[a-z]{1,16}$"},
		{"повтор id", func(r *Recipe) { r.Themes[1].ID = "birthday" }, "повторяется"},
		{"имя длиннее 20", func(r *Recipe) { r.Themes[0].Name = strings.Repeat("я", 21) }, "имя"},
		{"гомоглиф", func(r *Recipe) { r.Themes[0].Name = "Пpаздник" }, "имя"},
		{"имя встроенной строки", func(r *Recipe) { r.Themes[0].Name = "праздник" }, "совпадает"},
		{"имя-двойник", func(r *Recipe) { r.Themes[1].Name = "ДЕНЬ РОЖДЕНИЯ" }, "совпадает"},
		{"база не встроенная", func(r *Recipe) { r.Themes[0].Base = "newyear" }, "base"},
		{"order 0", func(r *Recipe) { r.Themes[0].Order = order(0) }, "order 0"},
		{"order 100", func(r *Recipe) { r.Themes[0].Order = order(100) }, "order 100"},
	}
	for _, c := range cases {
		rec := themedRecipe()
		c.mut(rec)
		data, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseRecipe(data); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ждали ошибку с %q, получили %v", c.name, c.want, err)
		}
	}
	if _, err := ParseRecipe(mustJSON(t, themedRecipe())); err != nil {
		t.Fatalf("годный рецепт отвергнут: %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Тема набора может быть и темой по умолчанию рецепта; неизвестное поле в объявлении — отказ.
func TestRecipeThemeAsDefault(t *testing.T) {
	rec := themedRecipe()
	rec.Theme = "kidsparty"
	rec.Items[2].Theme = ""
	rec.Items[0].Theme, rec.Items[1].Theme = "birthday", ""
	rep, _ := run(t, themedSrc(t), rec)
	if !rep.OK || rep.Themes["kidsparty"] != 4 {
		t.Fatalf("тема по умолчанию: %v %v", rep.Themes, rep.Errors)
	}
	bad := strings.Replace(string(mustJSON(t, themedRecipe())), `"name":"День рождения"`, `"name":"День рождения","color":"red"`, 1)
	if _, err := ParseRecipe([]byte(bad)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("неизвестное поле объявления: %v", err)
	}
}

// skip убрал все предметы темы — ошибка отчёта, независимые предметы собраны, манифест записан.
func TestRecipeThemeSkippedAll(t *testing.T) {
	rec := themedRecipe()
	rec.Skip = map[string]string{"s2.png#1": "брак", "s2.png#2": "брак"}
	rep, out := run(t, themedSrc(t), rec)
	if rep.OK || !hasError(rep, `тема "kidsparty" объявлена, но в наборе нет`) || len(rep.Items) != 4 {
		t.Fatalf("ждали одну ошибку и 4 предмета: %v, предметов %d", rep.Errors, len(rep.Items))
	}
	readOut(t, out, "manifest.src.json")
}

// TestThemeDeclParity: копия правила объявления здесь и pack.ParseThemes дают одно и то же на общем
// векторе testdata/pack-manifest-cases.json. Инструмент строже: принял список — программа приняла все записи
// с теми же значениями; программа что-то пропустила — инструмент отказывает.
func TestThemeDeclParity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pack-manifest-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Cases []struct {
			Name     string `json:"name"`
			Manifest struct {
				Themes json.RawMessage `json:"themes"`
			} `json:"manifest"`
			Want struct {
				OK     bool `json:"ok"`
				Themes []struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Base  string `json:"base"`
					Order int    `json:"order"`
				} `json:"themes"`
				ThemeSkipN int `json:"themeSkipN"`
			} `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	checked, accepted := 0, 0
	for _, c := range v.Cases {
		var decls []ThemeDecl
		dec := json.NewDecoder(strings.NewReader(string(c.Manifest.Themes)))
		dec.DisallowUnknownFields()
		if len(c.Manifest.Themes) == 0 || dec.Decode(&decls) != nil || !c.Want.OK {
			continue // нет поля, не массив записей нужного вида или набор отвергнут по другой причине
		}
		checked++
		errs := checkThemeDecls(decls)
		same := c.Want.ThemeSkipN == 0 && len(c.Want.Themes) == len(decls)
		if (len(errs) == 0) != same {
			t.Errorf("%s: инструмент: ошибок %d, программа приняла %d из %d (пропущено %d)", c.Name, len(errs), len(c.Want.Themes), len(decls), c.Want.ThemeSkipN)
			continue
		}
		if len(errs) > 0 {
			continue
		}
		accepted++
		for i, d := range decls {
			r, w := d.resolved(), c.Want.Themes[i]
			if r.ID != w.ID || r.Name != w.Name || r.Base != w.Base || *r.Order != w.Order {
				t.Errorf("%s: запись %d: %+v, программа %+v", c.Name, i, r, w)
			}
		}
	}
	if checked < 20 || accepted < 5 {
		t.Fatalf("вектор почти не использован: проверено %d, принято %d", checked, accepted)
	}
}
