package fetch

import (
	"encoding/json"
	"strings"
	"testing"
)

// objs — JSON-массив из n объявлений тем.
func objs(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = `{"id":"newyear","name":"Новый год","base":"holiday"}`
	}
	return "[" + strings.Join(items, ",") + "]"
}

// Поле themes строки (170): список → число ≤ 16, любой другой тип и негодный JSON внутри — 0; сам
// набор и каталог целы, сырое поле обнуляется, в Skipped ничего не попадает.
func TestParseCatalogThemes(t *testing.T) {
	src := newTestLib(t).source()
	cases := []struct {
		name string
		raw  string // "" — поля нет
		want int
	}{
		{"нет поля", "", 0},
		{"пустой список", `[]`, 0},
		{"одно", objs(1), 1},
		{"три", objs(3), 3},
		{"ровно предел", objs(16), 16},
		{"сверх предела", objs(17), 16},
		{"огромный список", objs(500), 16},
		{"элементы не объекты", `[1,"a",null]`, 3},
		{"строка", `"newyear"`, 0},
		{"объект", `{"id":"newyear"}`, 0},
		{"число", `5`, 0},
		{"логическое", `true`, 0},
		{"null", `null`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := []map[string]any{row("ivory", 1, src.Prefix+"ivory-v1/ivory-1.zip", []byte("zip a")), row("plain", 1, src.Prefix+"plain-v1/plain-1.zip", []byte("zip b"))}
			if c.raw != "" {
				rows[0]["themes"] = json.RawMessage(c.raw)
			}
			cat, err := ParseCatalog(bodyJSON(t, 5, rows...), src)
			if err != nil || len(cat.Packs) != 2 || len(cat.Skipped) != 0 {
				t.Fatalf("каталог: %v %+v", err, cat)
			}
			if cat.Packs[0].Themes != c.want || cat.Packs[1].Themes != 0 {
				t.Errorf("Themes %d и %d, ждали %d и 0", cat.Packs[0].Themes, cat.Packs[1].Themes, c.want)
			}
			if cat.Packs[0].ThemesRaw != nil || cat.Packs[0].ID != "ivory" || cat.Packs[0].Size != 5 {
				t.Errorf("сырое поле не обнулено или строка испорчена: %+v", cat.Packs[0])
			}
		})
	}
}

// Каталог целиком из набора с темами и без: порядок и число строк прежние (старые клиенты ключ
// пропускают — ключ только добавляется).
func TestParseCatalogThemesKeepsOthers(t *testing.T) {
	src := newTestLib(t).source()
	a := row("ivory", 1, src.Prefix+"ivory-v1/ivory-1.zip", []byte("zip a"))
	a["themes"] = json.RawMessage(objs(2))
	b := row("rose", 2, src.Prefix+"rose-v1/rose-2.zip", []byte("zip r"))
	b["themes"] = json.RawMessage(`"плохо"`)
	cat, err := ParseCatalog(bodyJSON(t, 9, a, b), src)
	if err != nil || cat.Seq != 9 || len(cat.Packs) != 2 || cat.Packs[0].ID != "ivory" || cat.Packs[1].ID != "rose" || cat.Packs[1].Version != 2 {
		t.Fatalf("%v %+v", err, cat)
	}
	if cat.Packs[0].Themes != 2 || cat.Packs[1].Themes != 0 {
		t.Errorf("Themes: %d, %d", cat.Packs[0].Themes, cat.Packs[1].Themes)
	}
}
