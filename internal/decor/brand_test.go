package decor

import (
	"image/color"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var brandIDs = []string{"sa:st-brand-albums-black", "sa:st-brand-albums-white", "sa:st-brand-films-black", "sa:st-brand-films-white"}

// TestBrandCatalog — 4 фирменных стикера (спека 116): тема «Фирменные» только у стикеров и
// только встроенная (в каталоге фонов и рамочек её нет; свои и из набора в неё не попадают),
// ID, имя, пропорция из файла; щелчок отдаёт параметры генератора, а не путь к файлу.
func TestBrandCatalog(t *testing.T) {
	s, _ := openTemp(t)
	cat, err := s.Catalog(KindSticker, nil, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range cat.Items {
		if it.Theme == ThemeBrand {
			ids = append(ids, it.ID)
			if it.Own || it.Name == "" || it.Aspect < 2 || it.Aspect > 4 {
				t.Errorf("плитка %+v", it)
			}
		}
	}
	if !slices.Equal(ids, brandIDs) {
		t.Fatalf("фирменные %v", ids)
	}
	var brand *Theme
	for i := range cat.Themes {
		if cat.Themes[i].ID == ThemeBrand {
			brand = &cat.Themes[i]
		}
	}
	if brand == nil || brand.Name != "Фирменные" || brand.Count != 4 {
		t.Errorf("тема %+v", brand)
	}
	for _, kind := range []string{KindBg, KindFrame, KindSpread} {
		other, err := s.Catalog(kind, nil, 600, 300)
		if err != nil || slices.Contains(themeIDs(other), ThemeBrand) {
			t.Errorf("%s: тема «Фирменные» где не нужна: %v %v", kind, themeIDs(other), err)
		}
	}
	for _, id := range brandIDs {
		st, err := s.Sticker(id)
		if err != nil || st.Art == nil || !gen.IsArt(st.Art.Kind) || st.Aspect < 2 || st.Photo.Path != "" {
			t.Errorf("%s: %+v %v", id, st, err)
		}
	}
	// Свой стикер в тему «Фирменные» не кладётся — уходит в «Мои».
	res, err := s.Import(KindSticker, ThemeBrand, []string{testkit.PNG(t, testkit.TempDir(t), "my.png", 20, 10, color.NRGBA{200, 10, 10, 255})})
	if err != nil || len(res.Added) != 1 || res.Added[0].Theme != ThemeMine {
		t.Errorf("свой в тему «Фирменные»: %+v %v", res, err)
	}
}
