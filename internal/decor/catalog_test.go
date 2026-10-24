package decor

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestDecorPresets — встроенные фоны: ID по грамматике и без повторов, темы из списка, цвет
// страниц #rrggbb, параметры уже зажаты (в альбом идут как есть), генераторы есть (текстуры —
// после O1), 24–30 пресетов.
func TestDecorPresets(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range bgPresets {
		rest, ok := strings.CutPrefix(p.id, presetPrefix)
		if !ok || !gen.ValidKind(rest) || seen[p.id] {
			t.Errorf("ID %q", p.id)
		}
		seen[p.id] = true
		if !isTheme(p.theme) || p.theme == ThemeMine {
			t.Errorf("%s: тема %q", p.id, p.theme)
		}
		if len(p.color) != 7 || p.color[0] != '#' || strings.ToLower(p.color) != p.color {
			t.Errorf("%s: цвет %q", p.id, p.color)
		}
		if !gen.Known(p.spec.Kind) && !strings.HasPrefix(p.spec.Kind, "paper-") {
			t.Errorf("%s: нет генератора %q", p.id, p.spec.Kind)
		}
		if gen.Known(p.spec.Kind) && !reflect.DeepEqual(gen.Clamp(p.spec), p.spec) {
			t.Errorf("%s: не зажат: %+v → %+v", p.id, p.spec, gen.Clamp(p.spec))
		}
	}
	if n := len(bgPresets); n < 24 || n > 30 {
		t.Errorf("пресетов %d, ждали 24–30", n)
	}
}

// TestDecorCatalog — темы с числами (пустые скрыты, «Все» и «Мои» всегда), свои — новыми
// первыми, скрытые не видны, недавние отфильтрованы, плитки узора — в пропорции листа.
func TestDecorCatalog(t *testing.T) {
	src := testkit.TempDir(t)
	s, _ := openTemp(t)
	empty, err := s.Catalog(KindBg, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Kind != KindBg || len(empty.Recent) != 0 || empty.ReadOnly {
		t.Fatalf("пустой: %+v", empty)
	}
	if ids := themeIDs(empty); ids[0] != ThemeAll || ids[len(ids)-1] != ThemeMine || slices.Contains(ids, ThemeRecent) {
		t.Errorf("темы без своих и недавних: %v", ids)
	}

	clock := int64(0)
	nowMs = func() int64 { clock++; return clock }
	t.Cleanup(func() { nowMs = func() int64 { return time.Now().UnixMilli() } })
	a := importOK(t, s, "wedding", testkit.JPEG(t, src, "a.jpg", 30, 20))
	b := importOK(t, s, "mine", testkit.JPEG(t, src, "b.jpg", 31, 20), testkit.JPEG(t, src, "c.jpg", 32, 20))
	if err := s.Hide(KindBg, b[1].ID); err != nil {
		t.Fatal(err)
	}
	recent := []string{b[1].ID, "sa:gold-dots", "мусор", a[0].ID, "sa:gold-dots", "sa:нет", "my:ffffffffffffffff"}
	for i := range 20 {
		recent = append(recent, bgPresets[i].id)
	}
	cat, err := s.Catalog(KindBg, recent, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cat.Recent[:2], []string{"sa:gold-dots", a[0].ID}) || len(cat.Recent) != MaxRecent {
		t.Errorf("недавние %v", cat.Recent)
	}
	var own []string
	for _, it := range cat.Items {
		if it.Own {
			own = append(own, it.ID)
			if !strings.HasPrefix(it.URL, LibPath+"t/") || it.Aspect <= 1 || it.Color != "" {
				t.Errorf("своя плитка %+v", it)
			}
			continue
		}
		req, ok := gen.Parse(strings.Split(it.URL, "?")[0], strings.Split(it.URL, "?")[1]+"&px=64")
		if !ok || req.W != 600 || req.H != 300 || it.Color == "" || it.Name == "" {
			t.Errorf("встроенная плитка %+v", it)
		}
	}
	if !slices.Equal(own, []string{b[0].ID, a[0].ID}) {
		t.Errorf("свои %v, ждали новые первыми без скрытого", own)
	}
	count := map[string]int{}
	for _, th := range cat.Themes {
		count[th.ID] = th.Count
	}
	if count[ThemeAll] != len(cat.Items) || count[ThemeMine] != 1 || count[ThemeRecent] != MaxRecent ||
		count["wedding"] != countTheme(cat.Items, "wedding") || cat.Themes[0].ID != ThemeRecent {
		t.Errorf("темы %+v", cat.Themes)
	}

	// Стикеры (Ф2): встроенные плитки всех тем; недавние фонов в них не попадают.
	st, err := s.Catalog(KindSticker, recent, 600, 300)
	wantThemes := []string{ThemeAll, "basic", "wedding", "kids", "kindergarten", "school", "grad", "holiday", ThemeBrand, ThemeMine}
	if err != nil || len(st.Items) != len(stickerPresets) || len(st.Recent) != 0 || !slices.Equal(themeIDs(st), wantThemes) {
		t.Errorf("стикеры: %+v, %v", st, err)
	}
	if _, err := s.Catalog("set", nil, 1, 1); !errors.Is(err, ErrKind) {
		t.Errorf("чужой вид: %v", err)
	}
}

func themeIDs(c Catalog) []string {
	var out []string
	for _, th := range c.Themes {
		out = append(out, th.ID)
	}
	return out
}

func countTheme(items []Item, theme string) int {
	n := 0
	for _, it := range items {
		if it.Theme == theme {
			n++
		}
	}
	return n
}

// TestDecorBackground — встроенный: узор (параметры генератора, отметка плитки — решение №201)
// и цвет страниц; мусор, свой без файла и скрытый пресет-неизвестный — ErrNotFound. Отметка
// каждого пресета проходит зажим модели.
func TestDecorBackground(t *testing.T) {
	s, _ := openTemp(t)
	bg, err := s.Background("sa:big-dots")
	if err != nil || bg.Pattern == nil || bg.Pattern.Kind != "dots" || bg.Pattern.Scale != 1.6 ||
		!slices.Equal(bg.Pattern.Inks, []string{"#f2b134"}) || bg.Color != "#fffaf0" || bg.Photo.Path != "" ||
		bg.Pattern.Preset != "sa:big-dots" {
		t.Fatalf("пресет: %+v, %v", bg, err)
	}
	for _, p := range bgPresets {
		if b, err := s.Background(p.id); err == nil && model.ClampPattern(b.Pattern).Preset != p.id {
			t.Errorf("%s: отметка %q не проходит зажим", p.id, b.Pattern.Preset)
		}
	}
	bg.Pattern.Inks[0] = "#000000" // копия: каталог не меняется
	if again, _ := s.Background("sa:big-dots"); again.Pattern.Inks[0] != "#f2b134" {
		t.Error("узор пресета общий с каталогом")
	}
	for _, id := range []string{"", "sa:", "sa:нет", "my:0123456789abcdef", "../Фоны/x.jpg", `C:\x.jpg`} {
		if _, err := s.Background(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Background(%q) = %v", id, err)
		}
	}
}

// TestDecorCatalogFast — 500 своих: каталог без обхода диска быстрее Limit(150 ms).
func TestDecorCatalogFast(t *testing.T) {
	s, _ := openTemp(t)
	s.files[KindBg] = map[string]string{}
	for i := range 500 {
		key := fmt.Sprintf("my:%016x", i+1)
		s.entries = append(s.entries, &entry{Key: key, Kind: KindBg, Theme: "school", Name: "Фон", W: 300, H: 200, Added: int64(i)})
		s.files[KindBg][key] = "x"
	}
	recent := make([]string, 0, 12)
	for i := range 12 {
		recent = append(recent, fmt.Sprintf("my:%016x", i+1))
	}
	var cat Catalog
	d := testkit.Fastest(5, func() { cat, _ = s.Catalog(KindBg, recent, 600, 300) })
	if len(cat.Items) != 500+countBuiltin() {
		t.Fatalf("плиток %d", len(cat.Items))
	}
	if limit := testkit.Limit(150 * time.Millisecond); d > limit {
		t.Errorf("каталог %v, предел %v", d, limit)
	}
}

// Свои с равным Added (сохранены в одну мс) встают устойчиво: позже сохранённый — первым.
func TestCatalogOwnTieOrder(t *testing.T) {
	s, _ := openTemp(t)
	s.files[KindBg] = map[string]string{}
	var keys []string
	for i, added := range []int64{5, 7, 7, 7, 3} {
		key := fmt.Sprintf("my:%016x", 0xf0-i*0x10) // ключи не по порядку: порядок не от Key
		keys = append(keys, key)
		s.entries = append(s.entries, &entry{Key: key, Kind: KindBg, Theme: "school", Name: "Фон", W: 300, H: 200, Added: added})
		s.files[KindBg][key] = "x"
	}
	want := []string{keys[3], keys[2], keys[1], keys[0], keys[4]}
	for range 20 {
		cat, err := s.Catalog(KindBg, nil, 600, 300)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, it := range cat.Items {
			if it.Own {
				got = append(got, it.ID)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("порядок %v, ждали %v", got, want)
		}
	}
}

func countBuiltin() int {
	n := 0
	for _, p := range bgPresets {
		if gen.Known(p.spec.Kind) {
			n++
		}
	}
	return n
}
