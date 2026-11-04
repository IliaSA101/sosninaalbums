package decor

import (
	"image/color"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestLegacyNotOwn — «Прежние макеты» только для наборов, как «Фирменные»: свой стикер, разворот и
// макет с темой legacy уходят в «Мои»; сама тема для набора по-прежнему принимается.
func TestLegacyNotOwn(t *testing.T) {
	if ownTheme(ThemeLegacy) || ownTheme(ThemeBrand) || !ownTheme("school") || !ownTheme(ThemeMine) {
		t.Fatal("ownTheme")
	}
	s, cfg := openTemp(t)
	res, err := s.Import(KindSticker, ThemeLegacy, []string{testkit.PNG(t, testkit.TempDir(t), "my.png", 20, 10, color.NRGBA{200, 10, 10, 255})})
	if err != nil || len(res.Added) != 1 || res.Added[0].Theme != ThemeMine {
		t.Errorf("свой стикер: %+v %v", res, err)
	}
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	sp, err := s.SaveSpreadTpl(ThemeLegacy, "Свой", model.SpreadTpl{V: 1, Pages: 1, Ratio: 1, Places: box, Page: []model.TplBg{{Color: "#cccccc"}}})
	if err != nil || sp.Theme != ThemeMine {
		t.Errorf("свой разворот: %+v %v", sp, err)
	}
	lk, err := s.SaveLook(ThemeLegacy, "Свой макет", lookOf("", ""))
	if err != nil || lk.Theme != ThemeMine {
		t.Errorf("свой макет: %+v %v", lk, err)
	}
	// Реестр своих, записанный на диск с темой legacy (руками или иной версией), при чтении даёт «Мои».
	reg := regMap(t, cfg)
	for _, e := range reg["items"].([]any) {
		e.(map[string]any)["theme"] = ThemeLegacy
	}
	putRegMap(t, cfg, reg)
	cat, err := Open(cfg).Catalog(KindSticker, nil, 300, 300)
	n := 0
	for _, it := range cat.Items {
		if it.Own {
			n++
			if it.Theme != ThemeMine {
				t.Errorf("свой из реестра: тема %q", it.Theme)
			}
		}
	}
	if err != nil || n != 1 {
		t.Errorf("своих в реестре: %d %v", n, err)
	}
}

// TestThemeLegacy — «Прежние макеты»: тема признана, стоит после наборных тем и до «Фирменных»;
// предмет набора с этой темой ложится в неё (и после перезапуска), неизвестная тема (набор
// от более новой программы) по-прежнему уходит в «Универсальные».
func TestThemeLegacy(t *testing.T) {
	if !isTheme(ThemeLegacy) || packTheme(ThemeLegacy) != ThemeLegacy {
		t.Fatalf("isTheme/packTheme: %v %q", isTheme(ThemeLegacy), packTheme(ThemeLegacy))
	}
	for th, want := range map[string]string{"": "basic", "future-theme": "basic", ThemeMine: "basic", "school": "school"} {
		if got := packTheme(th); got != want {
			t.Errorf("packTheme(%q) = %q, ждали %q", th, got, want)
		}
	}
	s, cfg := openTemp(t)
	themes := []string{ThemeLegacy, "future-theme", "school", ThemeLegacy}
	items := make([]pit, len(themes))
	for i := range themes[:3] {
		items[i] = pit{"stickers/s" + string(rune('a'+i)) + ".png", renderPNG(t, 20+i, 20)}
	}
	items[3] = pit{"looks/main.json", lookBytes(t, lookOf("", ""), "")} // прежний макет — вид KindAlbum
	m, src := testPack("legacy-pack", 1, items...)
	for i := range m.Items {
		m.Items[i].Theme = themes[i]
	}
	install(t, s, m, src, nil)
	checkLegacyCatalog(t, s)
	checkLegacyCatalog(t, Open(cfg)) // реестр набора с диска: тема не потерялась
	// Без предметов тема скрыта, как прочие пустые.
	empty, _ := openTemp(t)
	cat, err := empty.Catalog(KindAlbum, nil, 300, 300)
	if err != nil || slices.Contains(themeIDs(cat), ThemeLegacy) {
		t.Errorf("пустая тема видна: %v %v", themeIDs(cat), err)
	}
}

// checkLegacyCatalog — три предмета набора в нужных темах, порядок и число «Прежних макетов».
func checkLegacyCatalog(t *testing.T, s *Store) {
	t.Helper()
	cat, err := s.Catalog(KindSticker, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range cat.Items {
		if it.Pack != "" {
			got[it.Name] = it.Theme
		}
	}
	want := map[string]string{"stickers/sa.png": ThemeLegacy, "stickers/sb.png": "basic", "stickers/sc.png": "school"}
	for n, th := range want {
		if got[n] != th {
			t.Errorf("%s: тема %q, ждали %q", n, got[n], th)
		}
	}
	album, err := s.Catalog(KindAlbum, nil, 300, 300)
	if err != nil || len(album.Items) != 1 || album.Items[0].Theme != ThemeLegacy {
		t.Errorf("макет набора: %+v %v", album.Items, err)
	}
	ids := themeIDs(cat)
	j := slices.Index(ids, ThemeLegacy)
	if j < 0 || j != slices.Index(ids, ThemeBrand)-1 || j < slices.Index(ids, "holiday") {
		t.Errorf("порядок тем %v", ids)
	}
	for _, th := range cat.Themes {
		if th.ID == ThemeLegacy && (th.Name != "Прежние макеты" || th.Count != 1) {
			t.Errorf("тема %+v", th)
		}
	}
}
