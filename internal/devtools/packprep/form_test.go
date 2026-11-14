package packprep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFormParity: копия правил вида здесь и formOf в internal/decor дают одно и то же на общем
// векторе testdata/parity/decorform.json (144 предмета пилота и края).
func TestFormParity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "parity", "decorform.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Cases []struct {
			Name string `json:"name"`
			W    int    `json:"w"`
			H    int    `json:"h"`
			Form string `json:"form"`
			Note string `json:"note"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &v); err != nil || len(v.Cases) < 144 {
		t.Fatalf("вектор: %v, случаев %d", err, len(v.Cases))
	}
	for _, c := range v.Cases {
		if got := formOf(c.Name, c.W, c.H); got != c.Form {
			t.Errorf("%q %d×%d (%s): вид %q, ждали %q", c.Name, c.W, c.H, c.Note, got, c.Form)
		}
	}
}

// TestCheckFormFlag: вид предмета рецепта (kind) не совпал с признаком программы — флаг предмета в
// отчёте, упаковка идёт (ошибок нет, отчёт OK); совпавшие и обычные стикеры — без флага.
func TestCheckFormFlag(t *testing.T) {
	src := tempDir(t)
	writePNG(t, filepath.Join(src, "s.png"), stickerSheet())
	names := func(a, b string) []string { return []string{a, b} }
	rec := baseRecipe(
		// рецепт: уголки, признак: стикеры (имена не про уголок, форма квадратная)
		Item{Src: "s.png", Mode: ModeSheet, Out: "corners/a-", Kind: KindCorner, Names: names("Диск", "Квадрат"), Grid: "2x1"},
		// рецепт: стикеры, у одного в имени «уголок»; второй — обычный
		Item{Src: "s.png", Mode: ModeSheet, Out: "stickers/b-", Names: names("Золотой уголок", "Круг"), Grid: "2x1"},
		// рецепт: уголки, признак тот же — без флага
		Item{Src: "s.png", Mode: ModeSheet, Out: "corners/c-", Kind: KindCorner, Names: names("Уголок листья", "Уголки цветы"), Grid: "2x1"},
	)
	rep, out := run(t, src, rec)
	if !rep.OK || len(rep.Errors) != 0 {
		t.Fatalf("упаковка должна идти: %v", rep.Errors)
	}
	want := map[string]string{
		"Диск": "вид рецепта: уголок, признак: стикер", "Квадрат": "вид рецепта: уголок, признак: стикер",
		"Золотой уголок": "вид рецепта: стикер, признак: уголок",
	}
	for _, o := range rep.Items {
		w, flagged := want[o.Name]
		if flagged != hasFlag(o, "вид рецепта") || flagged && !hasFlag(o, w) {
			t.Errorf("%q: флаги %v, ждали %q", o.Name, o.Flags, w)
		}
	}
	if len(readOut(t, out, "_check/report.json")) == 0 || len(rep.Flags) < 3 {
		t.Errorf("флаги в сводке отчёта: %v", rep.Flags)
	}
}
