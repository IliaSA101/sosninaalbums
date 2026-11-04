package decor

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestDecorFramePresets — встроенные рамочки: ID sa:fr-…, уникальны, темы из списка 048, вид
// известен и спека зажата (ClampFrame ничего не меняет), Frame отдаёт вид, цвета, толщину и отметку.
func TestDecorFramePresets(t *testing.T) {
	s, _ := openTemp(t)
	if len(framePresets) != 12 {
		t.Errorf("встроенных %d, ждали 12 (design §3)", len(framePresets))
	}
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, p := range framePresets {
		if rest, ok := strings.CutPrefix(p.id, "sa:fr-"); !ok || rest == "" || seen[p.id] {
			t.Errorf("ID %q", p.id)
		}
		seen[p.id] = true
		kinds[p.kind] = true
		if !isTheme(p.theme) || p.theme == ThemeMine || cleanName(p.name) != p.name || p.name == "" {
			t.Errorf("%s: тема %q, имя %q", p.id, p.theme, p.name)
		}
		if !gen.FrameKnown(p.kind) {
			t.Fatalf("%s: нет вида %q", p.id, p.kind)
		}
		spec := gen.FrameSpec{Kind: p.kind, Inks: p.inks, Width: p.width, W: 60, H: 80}
		if got := gen.ClampFrame(spec); !reflect.DeepEqual(got, spec) {
			t.Errorf("%s: не зажат: %+v → %+v", p.id, spec, got)
		}
		fs, err := s.Frame(p.id)
		if err != nil || fs.Kind != p.kind || fs.Preset != p.id || fs.Width != p.width || !slices.Equal(fs.Inks, p.inks) || fs.Photo.ID != "" {
			t.Errorf("%s: Frame %+v %v", p.id, fs, err)
		}
		fs.Inks = append(fs.Inks, "#000000") // копия: правка снаружи таблицу не трогает
	}
	if len(kinds) != 9 {
		t.Errorf("видов %d, ждали 9", len(kinds))
	}
	if fs, _ := s.Frame("sa:fr-soft"); len(fs.Inks) != 0 {
		t.Errorf("мягкий край с цветами: %v", fs.Inks)
	}
}

func themeCounts(c Catalog) map[string]int {
	out := map[string]int{}
	for _, th := range c.Themes {
		out[th.ID] = th.Count
	}
	return out
}

// TestDecorCatalogFrames — каталог рамочек: 12 встроенных в порядке таблицы, квадратные плитки с
// разбираемым адресом, темы с числами; в других видах рамочек нет.
func TestDecorCatalogFrames(t *testing.T) {
	s, _ := openTemp(t)
	empty, err := s.Catalog(KindFrame, nil, 0, 0)
	if err != nil || empty.Kind != KindFrame || len(empty.Items) != 12 || len(empty.Recent) != 0 || empty.ReadOnly {
		t.Fatalf("без своих: %+v %v", empty, err)
	}
	for i, it := range empty.Items {
		p := framePresets[i]
		if it.ID != p.id || it.Name != p.name || it.Theme != p.theme || it.Own || it.Aspect != 1 || it.Pack != "" {
			t.Errorf("плитка %d: %+v", i, it)
		}
		_, q, _ := strings.Cut(it.URL, "?")
		req, ok := gen.ParseFrame(q + "&px=128")
		if !ok || !req.Tile || req.Spec.Kind != p.kind || req.Spec.Width != p.width || !strings.HasPrefix(it.URL, gen.FramePath+"?") {
			t.Errorf("%s: адрес плитки %s → %+v %v", p.id, it.URL, req, ok)
		}
	}
	wantCounts := map[string]int{ThemeAll: 12, "basic": 3, "wedding": 3, "grad": 2, "kids": 1, "kindergarten": 1, "school": 1, "holiday": 1, ThemeMine: 0}
	if got := themeCounts(empty); !reflect.DeepEqual(got, wantCounts) {
		t.Errorf("темы %v, ждали %v", got, wantCounts)
	}
	for _, kind := range []string{KindBg, KindSticker, KindSpread} {
		c, err := s.Catalog(kind, nil, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range c.Items {
			if strings.HasPrefix(it.ID, "sa:fr-") || strings.HasPrefix(it.URL, gen.FramePath) {
				t.Errorf("%s: чужая плитка %+v", kind, it)
			}
		}
	}
	if ps := s.Packs(); ps[0].Frame != 12 {
		t.Errorf("встроенных в строке наборов %d", ps[0].Frame)
	}
}

// TestDecorCatalogFramesOwn — своя рамочка после встроенных, с темой и пропорцией; недавние
// отфильтрованы; скрытая не видна; в каталогах других видов её нет.
func TestDecorCatalogFramesOwn(t *testing.T) {
	src := testkit.TempDir(t)
	s, _ := openTemp(t)
	res, err := s.Import(KindFrame, "kids", []string{writeFile(t, src, "Кольцо.png", pngBytes(t, ringImage(60, 80, 12, 16, 12, 16)))})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("импорт: %+v %v", res, err)
	}
	own := res.Added[0]
	cat, err := s.Catalog(KindFrame, []string{"sa:fr-mat", "sa:fr-nope", own.ID, "sa:fr-mat", "my:0000000000000000"}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if last := cat.Items[len(cat.Items)-1]; len(cat.Items) != 13 || last.ID != own.ID || !last.Own || last.Aspect != 0.75 || last.Theme != "kids" {
		t.Fatalf("со своей: %d плиток, последняя %+v", len(cat.Items), last)
	}
	if !slices.Equal(cat.Recent, []string{"sa:fr-mat", own.ID}) {
		t.Errorf("недавние %v", cat.Recent)
	}
	if c := themeCounts(cat); c[ThemeAll] != 13 || c["kids"] != 2 || c[ThemeRecent] != 2 {
		t.Errorf("темы со своей: %v", c)
	}
	if err := s.Hide(KindFrame, own.ID); err != nil {
		t.Fatal(err)
	}
	if cat, _ := s.Catalog(KindFrame, nil, 0, 0); len(cat.Items) != 12 {
		t.Errorf("скрытая видна: %d", len(cat.Items))
	}
	for _, kind := range []string{KindBg, KindSticker, KindSpread} {
		c, _ := s.Catalog(kind, nil, 0, 0)
		if slices.ContainsFunc(c.Items, func(it Item) bool { return it.ID == own.ID }) {
			t.Errorf("%s: чужая плитка", kind)
		}
	}
}
