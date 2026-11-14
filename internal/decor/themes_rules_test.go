package decor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// themeCase — случай файла: манифест (или сырой текст) и ожидаемый итог.
type themeCase struct {
	Name     string          `json:"name"`
	Manifest json.RawMessage `json:"manifest"`
	Raw      *string         `json:"raw"`
	Want     map[string]any  `json:"want"`
}

// casesFile — testdata/pack-manifest-cases.json: те же случаи проходят pack (TestManifestCases) и
// JS (pack-check.test.mjs); копия правила в decor (170-26) обязана дать тот же итог.
func casesFile(t *testing.T) (limits map[string]any, cases []themeCase) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pack-manifest-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Limits map[string]any `json:"limits"`
		Cases  []themeCase    `json:"cases"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f.Limits, f.Cases
}

// Копия правила decor даёт на сыром `themes` каждого случая файла то же, что pack и JS.
func TestThemeRulesCases(t *testing.T) {
	_, cases := casesFile(t)
	n := 0
	for _, c := range cases {
		_, hasT := c.Want["themes"]
		_, hasS := c.Want["themeSkips"]
		if !hasT && !hasS {
			continue
		}
		n++
		input := []byte(c.Manifest)
		if c.Raw != nil {
			input = []byte(*c.Raw)
		}
		var head struct {
			Themes json.RawMessage `json:"themes"`
		}
		if err := json.Unmarshal(input, &head); err != nil {
			t.Errorf("%s: манифест не читается: %v", c.Name, err)
			continue
		}
		got, sk := parseThemeDecl(head.Themes, maxThemeRaw)
		themes := make([]any, 0, len(got))
		for _, d := range got {
			themes = append(themes, map[string]any{"id": d.ID, "name": d.Name, "base": d.Base, "order": float64(d.Order)})
		}
		why := make([]any, 0, len(sk.why))
		for _, w := range sk.why {
			why = append(why, w)
		}
		have := map[string]any{"themes": themes, "themeSkips": why, "themeSkipN": float64(sk.n)}
		for k, want := range c.Want {
			if v, ok := have[k]; ok && !reflect.DeepEqual(v, want) {
				t.Errorf("%s: %s = %v, ждали %v", c.Name, k, v, want)
			}
		}
	}
	if n < 100 {
		t.Fatalf("случаев тем %d: файл урезан или фильтр сломан", n)
	}
}

// Пределы копии равны блоку limits файла случаев (его же сверяет pack: TestLimits).
func TestThemeRulesLimits(t *testing.T) {
	limits, _ := casesFile(t)
	for k, v := range map[string]int{
		"maxThemes": maxThemeDecls, "maxThemesRaw": maxThemeRaw, "maxThemeName": maxThemeName, "maxPackThemes": maxPackThemes,
		"themeOrderMin": themeOrderMin, "themeOrderMax": themeOrderMax, "themeOrderDef": themeOrderDef,
	} {
		if got, ok := limits[k].(float64); !ok || int(got) != v {
			t.Errorf("%s: файл %v, decor %d", k, limits[k], v)
		}
	}
}

// Реестр пишет объявления без предела байт (170-29): поле больше 4 КиБ при ≤ 16 записях читается,
// 17-я и дальше — нет, не-массив — пусто.
func TestThemeRulesRegistryNoByteLimit(t *testing.T) {
	var recs []themeDecl
	for i := range 16 {
		recs = append(recs, themeDecl{ID: string(rune('a'+i)) + "bcdefghijklmnop", Name: "Абвгдеёжзийклмнопр" + string(rune('а'+i)) + "я", Base: "holiday", Order: 10 + i})
	}
	raw, _ := json.MarshalIndent(recs, "          ", "          ") // как в реестре с отступами
	if len(raw) <= maxThemeRaw {
		t.Fatalf("поле %d Б не больше предела манифеста: тест ничего не проверяет", len(raw))
	}
	if _, sk := parseThemeDecl(raw, maxThemeRaw); sk.n != 1 || len(sk.why) != 1 || sk.why[0] != whySize {
		t.Errorf("манифест: %+v", sk)
	}
	if got, sk := parseThemeDecl(raw, 0); len(got) != 16 || sk.n != 0 {
		t.Errorf("реестр: %d тем, %+v", len(got), sk)
	}
	more := append(recs, themeDecl{ID: "extra", Name: "Лишняя", Base: "kids", Order: 5})
	raw, _ = json.Marshal(more)
	if got, sk := parseThemeDecl(raw, 0); len(got) != 16 || sk.n != 1 || sk.why[0] != whyLimit {
		t.Errorf("17 записей: %d тем, %+v", len(got), sk)
	}
	for _, bad := range []string{`"строка"`, `{"a":1}`, `7`} {
		if got, sk := parseThemeDecl([]byte(bad), 0); len(got) != 0 || sk.n != 1 || sk.why[0] != whyType {
			t.Errorf("%s: %d тем, %+v", bad, len(got), sk)
		}
	}
}

// Служебные имена и id: список в decor совпадает с темами таблицы — имена themeList заняты.
func TestThemeRulesReserved(t *testing.T) {
	names := reservedFolded()
	for _, th := range themeList {
		if !names[themeFold(th.name)] {
			t.Errorf("имя строки «%s» не занято для объявлений", th.name)
		}
		if th.id != ThemeRecent && th.id != ThemeAll && th.id != ThemeMine && th.id != ThemeBrand && th.id != ThemeLegacy && !isBaseTheme(th.id) {
			t.Errorf("тема %s не встроенная и не служебная", th.id)
		}
		if !isBaseTheme(th.id) && validThemeID(th.id) {
			t.Errorf("служебный id %s принят", th.id)
		}
	}
	for _, id := range []string{"new", "get", "constructor"} {
		if validThemeID(id) {
			t.Errorf("id %s принят", id)
		}
	}
}
