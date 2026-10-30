package model

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// psdPxFile — testdata/fabrika-psd.json: только числа из PSD-шаблонов Фабрики Фотокниги (скачаны
// 24.10, docs/research/fabrika-requirements.md, «PSD-шаблоны»): px разворота и листов обложки
// по корешку. Лайфлат побайтно равен фотобумаге 20×20 и 21×30, поэтому строки общие.
type psdPxFile struct {
	DPI     int `json:"dpi"`
	Formats []struct {
		Name   string `json:"name"`
		Spread struct{ W, H int }
		Covers []struct {
			Spine float64 `json:"spine"` // корешок, мм
			W, H  int
		} `json:"covers"`
	} `json:"formats"`
}

// TestFabrikaPsdPx (0.26-123-34): развороты пяти кнопок Фабрики и листы CoverGeo всех её
// изделий на каждом корешке равны px PSD-шаблонов. Правка числа Фабрики без нового PSD роняет тест.
func TestFabrikaPsdPx(t *testing.T) {
	data, err := os.ReadFile("testdata/fabrika-psd.json")
	if err != nil {
		t.Fatal(err)
	}
	var f psdPxFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.DPI != 300 || len(f.Formats) != len(FabrikaFormats()) {
		t.Fatalf("таблица: %d dpi, %d форматов; ждали 300 и %d", f.DPI, len(f.Formats), len(FabrikaFormats()))
	}
	rows := 0
	for _, pf := range f.Formats {
		i := slices.IndexFunc(FabrikaFormats(), func(x PrintFormat) bool { return x.Name == pf.Name })
		if i < 0 {
			t.Fatalf("формата %s нет у Фабрики", pf.Name)
		}
		page := FabrikaFormats()[i]
		// Разворот: две страницы, как в выгрузке (px от 2·PageW, а не 2·px).
		if w, h := Px(2*page.PageW, f.DPI), Px(page.PageH, f.DPI); w != pf.Spread.W || h != pf.Spread.H {
			t.Errorf("%s: разворот %d×%d px, PSD %d×%d", pf.Name, w, h, pf.Spread.W, pf.Spread.H)
		}
		rows += 1 + len(pf.Covers)
		for _, c := range pf.Covers {
			if !checkPsdCover(t, pf.Name, page, c.Spine, c.W, c.H, f.DPI) {
				t.Errorf("%s: корешка %v мм нет ни у одного изделия Фабрики", pf.Name, c.Spine)
			}
		}
	}
	// 5 разворотов + 69 листов (корешки 8…34, у 30×30 с 10): 74 строки, как в PSD.
	if rows != 74 {
		t.Errorf("строк таблицы %d, ждали 74", rows)
	}
}

// checkPsdCover сверяет лист обложки каждого изделия Фабрики с этим корешком (6 пресетов) с PSD
// w×h px; число разворотов — верхняя граница ступени, дающей корешок. false — корешка нет ни у одного.
func checkPsdCover(t *testing.T, name string, page PrintFormat, spine float64, w, h, dpi int) (found bool) {
	t.Helper()
	for _, base := range fabrikaCoverPresets() {
		pr, _ := CoverPresetFor(base.ID, Settings{PageW: page.PageW, PageH: page.PageH})
		for _, st := range pr.Spec.Steps {
			if st.Mm != spine {
				continue
			}
			found = true
			g := fsferaProject(page.PageW, page.PageH, st.Upto, pr.Spec).CoverGeo()
			if g.Spine != spine || g.Over {
				t.Errorf("%s %s, %d разворотов: корешок %v (вне таблицы %v), ждали %v", name, base.ID, st.Upto, g.Spine, g.Over, spine)
			}
			if gw, gh := Px(g.X[7], dpi), Px(g.Y[3], dpi); gw != w || gh != h {
				t.Errorf("%s %s, корешок %v: лист %v×%v мм = %d×%d px, PSD %d×%d", name, base.ID, spine, g.X[7], g.Y[3], gw, gh, w, h)
			}
		}
	}
	return found
}
