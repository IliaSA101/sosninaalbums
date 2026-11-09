package printshop

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// album — книга w×h мм из n пустых разворотов, безопасная зона 10, обложка пресета preset
// по шаблону страницы ("" — без обложки).
func album(w, h float64, n int, preset string) *model.Project {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = w, h, 10
	for range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	if preset != "" {
		pr, _ := model.CoverPresetFor(preset, p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec}
	}
	return p
}

// TestFsferaCheck — правила design §3.3: не формат и лист 031 — Format "" и одна причина;
// запреты по порядку; предупреждения; точная памятка; имя папки.
func TestFsferaCheck(t *testing.T) {
	file := filepath.Join(testkit.TempDir(t), "Иванов.salbum")
	p := album(281, 284, 20, "fsfera-thin")
	before := model.Marshal(p)
	pl := Check(p, file, Fsfera)
	if !bytes.Equal(before, model.Marshal(p)) {
		t.Fatal("Check изменил проект")
	}
	want := Plan{Format: "30×30", Px: "6638×3354 px", Pages: "Тонкие", Spreads: 20, Spine: 14, Max: 57, Folder: "Иванов", Block: []string{}, FixAt: -1, Warn: []string{},
		Memo: []string{"Фотокнига 30×30, обложка «Фото»", "Страницы: Тонкие", "Разворотов: 20 (корешок 14 мм)", "Файлы: 6638×3354 px", "Папка: Иванов — 00.jpg и 01…20.jpg"},
		// 081: поля профиля — прежние тексты окна «Готово для Фотосферы» (FsferaDone.svelte).
		Kind: "fsfera", Title: "Для Фотосферы Pro", CoverName: "00.jpg", Done: "Готово для Фотосферы", SiteButton: "Открыть Фотосферу",
		Site: "На сайте: калькулятор Pro → эти параметры → «Загрузить» → файлы папки «Иванов»."}
	if got, _ := json.Marshal(pl); !bytes.Equal(got, mustJSON(t, want)) {
		t.Fatalf("готовый альбом:\n%s\n%s", got, mustJSON(t, want))
	}

	sheet := model.NewSheet(204, 304)
	for _, c := range []struct {
		p    *model.Project
		want string
		off  string // Plan.Off: у листа 031 пуст
	}{
		{album(300, 300, 3, ""), "Страница 300 × 300 мм — не формат Фотосферы: разворот выйдет 7087 × 3543 px, у 30×30 — 6638 × 3354 (281 × 284 мм). " +
			"В «Настройках альбома» выберите типографию «Фотосфера» и кнопку формата или введите px файла", "страница 300 × 300 мм — не формат Фотосферы"},
		{sheet, "Лист виньетки в Фотосферу Pro не выгружается", ""},
	} {
		if pl := Check(c.p, "", Fsfera); pl.Format != "" || !slices.Equal(pl.Block, []string{c.want}) || pl.Folder != DefaultFolder || pl.FixAt != -1 || pl.Off != c.off {
			t.Errorf("%q: %+v", c.want, pl)
		}
	}

	legacy := album(281, 284, 20, "fsfera-thin")
	legacy.Cover.Spec.W, legacy.Cover.Spec.H, legacy.Cover.Spec.Wrap = 0, 0, 23.5
	custom := album(281, 284, 2, model.CoverCustom)
	for _, c := range []struct {
		name string
		p    *model.Project
		want []string
		fix  bool
	}{
		{"0 разворотов", album(281, 284, 0, "fsfera-thin"), []string{"В альбоме нет разворотов"}, false},
		{"Нетпринт", album(281, 284, 2, "netprint-premium"), []string{"Обложка размечена для «Нетпринт · Премиум» — выберите «Фотосфера» в карточке обложки"}, false},
		{"свои", custom, []string{"Обложка со своими размерами — выберите «Фотосфера» в карточке обложки"}, false},
		{"не по шаблону", legacy, []string{"Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326 (файл 7358 × 3909 px, нужен 7394 × 3850)"}, true},
		{"58 тонких", album(201, 152, 58, "fsfera-thin"), []string{"Разворотов 58 — у Фотосферы («Тонкие листы») не больше 57"}, false},
		{"31 картон", album(201, 152, 31, "fsfera-card"), []string{"Разворотов 31 — у Фотосферы («Картон») не больше 30"}, false},
		{"21 двойной", album(201, 152, 21, "fsfera-double"), []string{"Разворотов 21 — у Фотосферы («Двойной картон») не больше 20"}, false},
		{"58 без обложки", album(201, 152, 58, ""), []string{"Разворотов 58 — у Фотосферы не больше 57"}, false},
		{"57 без обложки", album(201, 152, 57, ""), []string{}, false},
		// Обложка Нетпринта на странице Фотосферы: предел Фотосферы без обложки, не Royal 30 (085 design §3).
		{"58 Нетпринт", album(201, 152, 58, "netprint-premium"), []string{"Обложка размечена для «Нетпринт · Премиум» — выберите «Фотосфера» в карточке обложки",
			"Разворотов 58 — у Фотосферы не больше 57"}, false},
		{"31 Royal", album(201, 152, 31, "netprint-royal"), []string{"Обложка размечена для «Нетпринт · Royal» — выберите «Фотосфера» в карточке обложки"}, false},
		{"58 свои", album(201, 152, 58, model.CoverCustom), []string{"Обложка со своими размерами — выберите «Фотосфера» в карточке обложки",
			"Разворотов 58 — у Фотосферы не больше 57"}, false},
	} {
		at := -1
		if c.fix {
			at = 0
		}
		if pl := Check(c.p, "", Fsfera); pl.Format == "" || !slices.Equal(pl.Block, c.want) || pl.CoverFix != c.fix || pl.FixAt != at {
			t.Errorf("%s: Block %q, CoverFix %v, FixAt %d", c.name, pl.Block, pl.CoverFix, pl.FixAt)
		}
	}
	// Max — прежний: таблица обложки Фотосферы, иначе 57 (и под обложкой Нетпринта, 085).
	for preset, want := range map[string]int{"fsfera-thin": 57, "fsfera-card": 30, "fsfera-double": 20, "": 57,
		"netprint-royal": 57, "netprint-soft": 57, model.CoverCustom: 57} {
		if got := Check(album(201, 152, 5, preset), "", Fsfera).Max; got != want {
			t.Errorf("Max у %q: %d, ждали %d", preset, got, want)
		}
	}
	// Строка «не по шаблону» не первая: FixAt — её номер, а не 0 (решение 141).
	legacy0 := album(281, 284, 0, "fsfera-thin")
	legacy0.Cover.Spec = legacy.Cover.Spec
	if pl := Check(legacy0, "", Fsfera); len(pl.Block) != 2 || pl.Block[0] != "В альбоме нет разворотов" || pl.FixAt != 1 ||
		!strings.HasPrefix(pl.Block[1], "Не по шаблону Фотосферы 30×30") {
		t.Errorf("0 разворотов и не по шаблону: Block %q, FixAt %d", pl.Block, pl.FixAt)
	}

	// Предупреждения и памятка без обложки; зона 8 — меньше 10 у 30×30, но не у 25×25.
	bare := album(281, 284, 1, "")
	bare.Settings.Safe = 8
	pl = Check(bare, "", Fsfera)
	if !slices.Equal(pl.Warn, []string{"Обложки нет: выгрузятся только развороты (для фотообложки нужен 00.jpg)",
		"Безопасная зона 8 мм — у Фотосферы 10 мм: проверьте края (Вид ▸ Безопасная зона)"}) || len(pl.Block) != 0 {
		t.Errorf("без обложки: Warn %q, Block %q", pl.Warn, pl.Block)
	}
	if !slices.Equal(pl.Memo, []string{"Фотокнига 30×30", "Разворотов: 1", "Файлы: 6638×3354 px", "Папка: Альбом — 01.jpg"}) || pl.Pages != "" || pl.Spine != 0 || pl.Max != 57 {
		t.Errorf("памятка без обложки: %+v", pl)
	}
	sq := album(251, 254, 2, "fsfera-card")
	sq.Settings.Safe = 8
	if pl := Check(sq, "", Fsfera); len(pl.Warn) != 0 || pl.Memo[4] != "Папка: Альбом — 00.jpg и 01…02.jpg" || pl.Max != 30 {
		t.Errorf("25×25, зона 8: %+v", pl)
	}
	// Пустые списки — [] в JSON, а не null: окно читает length.
	if b, _ := json.Marshal(Check(album(281, 284, 2, "fsfera-thin"), "", Fsfera)); !bytes.Contains(b, []byte(`"block":[]`)) || !bytes.Contains(b, []byte(`"warn":[]`)) {
		t.Errorf("пустые списки: %s", b)
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

// TestFolderNameOfAlbum — имя папки книги: имя файла без расширения через paths.FolderName,
// несохранённый альбом и негодное имя — «Альбом».
func TestFolderNameOfAlbum(t *testing.T) {
	dir := testkit.TempDir(t)
	for in, want := range map[string]string{
		"": "Альбом",
		filepath.Join(dir, "Иванов.salbum"):       "Иванов",
		filepath.Join(dir, "Свадьба 2026.salbum"): "Свадьба 2026",
		filepath.Join(dir, "con.salbum"):          "Альбом",
		filepath.Join(dir, "a.b.salbum"):          "a.b",
		filepath.Join(dir, ".salbum"):             "Альбом",
	} {
		if got := FolderName(in); got != want {
			t.Errorf("FolderName(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// TestCoverFixQuestion — вопрос обычного экспорта по строке карточки обложки.
func TestCoverFixQuestion(t *testing.T) {
	for in, want := range map[string]string{
		// px файла — для типографии (300 dpi); в вопросе обычного экспорта его нет
		"Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326 (файл 7358 × 3909 px, нужен 7394 × 3850)": "Обложка не по шаблону Фотосферы 30×30 (623 × 331 мм, нужно 626 × 326). Выгрузить как есть?",
		"Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326":                                          "Обложка не по шаблону Фотосферы 30×30 (623 × 331 мм, нужно 626 × 326). Выгрузить как есть?",
		"Не по шаблону Фотосферы: заворот 23,5 мм, нужно 20":                                                    "Обложка не по шаблону Фотосферы (заворот 23,5 мм, нужно 20). Выгрузить как есть?",
		"Не по шаблону Фотосферы":                                                                               "Обложка не по шаблону Фотосферы. Выгрузить как есть?",
	} {
		if got := CoverFixQuestion(in); got != want {
			t.Errorf("CoverFixQuestion(%q) = %q", in, got)
		}
	}
}
