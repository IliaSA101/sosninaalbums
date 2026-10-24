package printshop

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestFabrikaCheck — правила 081 design §2 для Фабрики Фотокниги: готовый альбом (памятка, site,
// поля профиля), запреты по порядку, предупреждения, памятка без обложки и у лайфлата.
func TestFabrikaCheck(t *testing.T) {
	file := filepath.Join(testkit.TempDir(t), "Иванов.salbum")
	p := album(300, 305, 20, "fabrika-photo")
	before := model.Marshal(p)
	pl := Check(p, file, Fabrika)
	if !bytes.Equal(before, model.Marshal(p)) {
		t.Fatal("Check изменил проект")
	}
	want := Plan{Kind: "fabrika", Title: "Для Фабрики Фотокниги", Format: "30×30", Pages: "Фотобумага", Spreads: 20, Spine: 14, Min: 10, Max: 40,
		Folder: "Иванов", CoverName: "1-00.jpg", Quality: 100, Block: []string{}, FixAt: -1, Warn: []string{},
		Memo: []string{"Фотокнига на фотобумаге 30×30", "Разворот: Фотобумага", "Разворотов: 20 (корешок 14 мм)", "Папка: Иванов — 1-00.jpg и 1-01…1-20.jpg"},
		Done: "Готово для Фабрики Фотокниги", SiteButton: "Открыть Фабрику Фотокниги",
		Site: "На сайте: Фотокниги → «Фотокнига на фотобумаге» → эти параметры → «Создать заказ» → файлы папки «Иванов»."}
	if got, _ := json.Marshal(pl); !bytes.Equal(got, mustJSON(t, want)) {
		t.Fatalf("готовый альбом:\n%s\n%s", got, mustJSON(t, want))
	}

	// Лист 031 и страница не из таблицы — Format "" и одна причина.
	for _, c := range []struct {
		p    *model.Project
		want string
		off  string // Plan.Off: у листа 031 пуст
	}{
		{album(300, 300, 3, ""), "Страница 300 × 300 мм — не формат Фабрики Фотокниги. В «Настройках альбома» выберите типографию «Фабрика Фотокниги» и кнопку формата", "страница 300 × 300 мм — не формат Фабрики Фотокниги"},
		{album(201, 152, 3, ""), "Страница 201 × 152 мм — не формат Фабрики Фотокниги. В «Настройках альбома» выберите типографию «Фабрика Фотокниги» и кнопку формата", "страница 201 × 152 мм — не формат Фабрики Фотокниги"},
		{model.NewSheet(204, 304), "Лист виньетки в Фабрику Фотокниги не выгружается", ""},
	} {
		if pl := Check(c.p, "", Fabrika); pl.Format != "" || !slices.Equal(pl.Block, []string{c.want}) || pl.FixAt != -1 || pl.Off != c.off {
			t.Errorf("%q: %+v", c.want, pl)
		}
	}

	off := album(300, 305, 20, "fabrika-photo")
	off.Cover.Spec.Wrap = 15
	offFix := model.CoverFixText(off)
	if !strings.HasPrefix(offFix, "Не по шаблону Фабрики Фотокниги 30×30: ") {
		t.Fatalf("строка «не по шаблону»: %q", offFix)
	}
	flat25 := album(250, 254, 12, "fabrika-layflat-paper")
	flat25.Cover.Spec.Wrap = 15
	for _, c := range []struct {
		name string
		p    *model.Project
		want []string
		at   int
	}{
		{"0 разворотов", album(300, 305, 0, "fabrika-photo"), []string{"В альбоме нет разворотов"}, -1},
		{"Фотосфера", album(208, 296, 2, "fsfera-card"), []string{"Обложка размечена для «Фотосфера · Картон» — выберите «Фабрика Фотокниги» в карточке обложки"}, -1},
		{"свои", album(200, 203, 2, model.CoverCustom), []string{"Обложка со своими размерами — выберите «Фабрика Фотокниги» в карточке обложки"}, -1},
		{"не по шаблону", off, []string{offFix}, 0},
		{"лайфлат 25×25", album(250, 254, 12, "fabrika-layflat-paper"), []string{"«Лайфлат, бумага» не бывает 25×25 — выберите другую обложку"}, -1},
		{"прослойка 2 мм 30×30", album(300, 305, 50, "fabrika-photo-2"), []string{"«Фотобумага, прослойка 2 мм» не бывает 30×30 — выберите другую обложку"}, -1},
		{"не по шаблону и не бывает", flat25, []string{model.CoverFixText(flat25), "«Лайфлат, бумага» не бывает 25×25 — выберите другую обложку"}, 0},
		{"41 фотобумага", album(200, 203, 41, "fabrika-photo"), []string{"Разворотов 41 — у Фабрики Фотокниги («Фотобумага») не больше 40"}, -1},
		{"9 фотобумага 30×30", album(300, 305, 9, "fabrika-photo"), []string{"Разворотов 9 — у Фабрики Фотокниги («Фотобумага») не меньше 10"}, -1},
		{"5 лайфлат бумага", album(200, 203, 5, "fabrika-layflat-paper"), []string{"Разворотов 5 — у Фабрики Фотокниги («Лайфлат, бумага») не меньше 10"}, -1},
		{"21 лайфлат картон", album(208, 296, 21, "fabrika-layflat-card"), []string{"Разворотов 21 — у Фабрики Фотокниги («Лайфлат, картон») не больше 20"}, -1},
		{"9 без обложки 30×30", album(300, 305, 9, ""), []string{"Разворотов 9 — у Фабрики Фотокниги не меньше 10"}, -1},
		{"41 без обложки 30×20", album(304, 203, 41, ""), []string{"Разворотов 41 — у Фабрики Фотокниги не больше 40"}, -1},
		{"40 без обложки", album(304, 203, 40, ""), []string{}, -1},
		{"41 Нетпринт", album(300, 305, 41, "netprint-premium"), []string{"Обложка размечена для «Нетпринт · Премиум» — выберите «Фабрика Фотокниги» в карточке обложки",
			"Разворотов 41 — у Фабрики Фотокниги не больше 40"}, -1},
	} {
		if pl := Check(c.p, "", Fabrika); pl.Format == "" || !slices.Equal(pl.Block, c.want) || pl.CoverFix != (c.at >= 0) || pl.FixAt != c.at {
			t.Errorf("%s: Block %q, CoverFix %v, FixAt %d", c.name, pl.Block, pl.CoverFix, pl.FixAt)
		}
	}

	// Пределы плана — из таблицы 085; изделия нет у формата — 0.
	for _, c := range []struct {
		p        *model.Project
		min, max int
	}{
		{album(200, 203, 5, "fabrika-layflat-card-1"), 2, 12},
		{album(300, 305, 5, "fabrika-photo-1"), 10, 30},
		{album(200, 203, 5, "fabrika-photo-2"), 0, 9},
		{album(304, 203, 5, ""), 0, 40},
		{album(250, 254, 5, "fabrika-layflat-card"), 0, 0},
	} {
		if pl := Check(c.p, "", Fabrika); pl.Min != c.min || pl.Max != c.max {
			t.Errorf("%v×%v %s: предел %d–%d, ждали %d–%d", c.p.Settings.PageW, c.p.Settings.PageH, presetOf(c.p), pl.Min, pl.Max, c.min, c.max)
		}
	}

	// Без обложки и зона 8: два предупреждения, памятка «Фотокнига», на сайте — «изделие».
	bare := album(200, 203, 1, "")
	bare.Settings.Safe = 8
	pl = Check(bare, "", Fabrika)
	if !slices.Equal(pl.Warn, []string{"Обложки нет: выгрузятся только развороты (для обложки нужен 1-00.jpg)",
		"Безопасная зона 8 мм — у Фабрики Фотокниги 10 мм: проверьте края (Вид ▸ Безопасная зона)"}) || len(pl.Block) != 0 {
		t.Errorf("без обложки: Warn %q, Block %q", pl.Warn, pl.Block)
	}
	if !slices.Equal(pl.Memo, []string{"Фотокнига 20×20", "Разворотов: 1", "Папка: Альбом — 1-01.jpg"}) ||
		pl.Site != "На сайте: Фотокниги → изделие → эти параметры → «Создать заказ» → файлы папки «Альбом»." {
		t.Errorf("памятка без обложки: %q, %q", pl.Memo, pl.Site)
	}
	flat := Check(album(200, 203, 12, "fabrika-layflat-card"), "", Fabrika)
	if !slices.Equal(flat.Memo, []string{"Лайфлат 20×20", "Разворот: Картон", "Разворотов: 12 (корешок 12 мм)", "Папка: Альбом — 1-00.jpg и 1-01…1-12.jpg"}) ||
		!strings.Contains(flat.Site, "→ «Лайфлат» →") || len(flat.Block) != 0 {
		t.Errorf("лайфлат: %+v", flat)
	}
	// Чужая обложка: памятка без изделия, бумаги и корешка.
	if fs := Check(album(208, 296, 3, "fsfera-thin"), "", Fabrika); fs.Memo[0] != "Фотокнига 21×30" || fs.Pages != "" || fs.Spine != 0 {
		t.Errorf("обложка Фотосферы: %+v", fs)
	}
}

// presetOf — id пресета обложки для сообщения теста.
func presetOf(p *model.Project) string {
	if p.Cover == nil {
		return "без обложки"
	}
	return p.Cover.Spec.Preset
}

// TestProfilesProductsHaveLimits — у типографии с профилем каждое изделие группы есть хотя бы в
// одном её формате (иначе правило «не бывает» запретит его везде, 081 С2), изделия профиля — ровно
// пресеты группы, Gen — из таблицы 085.
func TestProfilesProductsHaveLimits(t *testing.T) {
	for _, prof := range Profiles() {
		var ids []string
		for _, pr := range model.CoverPresets() {
			if pr.Group != prof.Group {
				continue
			}
			ids = append(ids, pr.ID)
			if !slices.ContainsFunc(prof.Formats(), func(f model.PrintFormat) bool { _, ok := model.LimitFor(pr.ID, f.Name); return ok }) {
				t.Errorf("%s: у «%s» нет строки предела ни в одном формате", prof.Kind, pr.Name)
			}
			if _, ok := prof.products[pr.ID]; !ok {
				t.Errorf("%s: нет изделия для пресета %s", prof.Kind, pr.ID)
			}
		}
		if len(ids) == 0 || len(ids) != len(prof.products) || prof.Gen() == "" {
			t.Errorf("%s: пресетов %d, изделий %d, Gen %q", prof.Kind, len(ids), len(prof.products), prof.Gen())
		}
		if got, ok := ProfileOf(prof.Kind); !ok || got.Kind != prof.Kind {
			t.Errorf("ProfileOf(%q)", prof.Kind)
		}
	}
	if _, ok := ProfileOf(""); ok {
		t.Error("пустой вид — типография")
	}
	if _, ok := ProfileOf("netprint"); ok {
		t.Error("Нетпринт без профиля выгрузки — типография")
	}
}

// TestFabrikaURL — страница сайта по обложке: лайфлат, фотобумага, иначе каталог; Фотосфера
// адресом профиля не открывается (её кнопка — App.OpenFsfera).
func TestFabrikaURL(t *testing.T) {
	for preset, want := range map[string]string{
		"fabrika-layflat-card-1": "https://fabrika-fotoknigi.com/layflat",
		"fabrika-photo-2":        "https://fabrika-fotoknigi.com/photobook-photopaper",
		"fsfera-thin":            "https://fabrika-fotoknigi.com/photo-books",
		"":                       "https://fabrika-fotoknigi.com/photo-books",
	} {
		if got := Fabrika.URL(album(200, 203, 2, preset)); got != want {
			t.Errorf("%q: %s", preset, got)
		}
	}
	if Fabrika.URL(nil) != "https://fabrika-fotoknigi.com/photo-books" {
		t.Error("нет проекта")
	}
	if q := Fabrika.NoCoverQuestion(); q != "Обложки нет: в папке будут только развороты, а для обложки нужен 1-00.jpg. Выгрузить без обложки?" {
		t.Errorf("вопрос без обложки: %q", q)
	}
}

// TestExtraPrefix — папка книги Фабрики: свои 1-NN.jpg с меткой (и 1-00.jpg) не посторонние,
// 01.jpg обычного экспорта — посторонний даже с меткой; OldCover узнаёт свой 1-00.jpg.
func TestExtraPrefix(t *testing.T) {
	dir := testkit.TempDir(t)
	if err := export.Run(t.Context(), []render.Spread{{}, {}}, render.Options{PageW: 10, PageH: 10, DPI: 72, Quality: 80}, dir, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	ours, err := os.ReadFile(filepath.Join(dir, "01.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"1-00.jpg": ours, "1-01.jpg": ours, "1-05.jpg": ours, "1-07.jpg": []byte("чужой")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 01.jpg и 02.jpg с меткой — не имена Фабрики; 1-07.jpg без метки.
	extraSettledIn(t, "1-", "Фабрика", dir, []string{"1-01.jpg"}, 3, "01.jpg")
	// Та же папка для Фотосферы: свои — 01.jpg и 02.jpg, а 1-NN.jpg — посторонние.
	extraSettledIn(t, "", "Фотосфера", dir, nil, 4, "1-00.jpg")
	if got := OldCover(dir, "1-00.jpg"); !slices.Equal(got, []string{"1-00.jpg"}) {
		t.Errorf("свой 1-00.jpg: %q", got)
	}
	if got := OldCover(dir, "1-07.jpg"); got != nil {
		t.Errorf("чужой файл: %q", got)
	}
}
