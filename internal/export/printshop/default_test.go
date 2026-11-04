package printshop

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// onButton — поля страницы p ставятся кнопкой формата label ряда group (размер, dpi, зона, обрез):
// так альбом получает кнопка «Настроек альбома».
func onButton(t *testing.T, p *model.Project, group, label string) *model.Project {
	t.Helper()
	for _, pr := range model.PagePresets() {
		if pr.Group == group && pr.Label == label {
			s := &p.Settings
			s.PageW, s.PageH, s.DPI, s.Layflat, s.Safe, s.Bleed = pr.W, pr.H, pr.DPI, pr.Layflat, pr.Safe, pr.Bleed
			return p
		}
	}
	t.Fatalf("кнопки %q нет в ряду %q", label, group)
	return nil
}

// TestDefaultKind (123 §1.5, таблица): что открывается в «Печати» при запомненном выборе. Альбом
// собран как у фотографа (кнопка формата, обложка), вид выбирает Plans.
func TestDefaultKind(t *testing.T) {
	fabrika21 := onButton(t, album(1, 1, 3, ""), model.CoverGroupFabrika, "21×30")
	fsfera21 := onButton(t, album(1, 1, 3, ""), model.CoverGroupFsfera, "21×30")
	marina := onButton(t, album(1, 1, 40, "fsfera-thin"), model.CoverGroupFabrika, "30×20") // 304×203 + обложка Фотосферы
	netCover := album(200, 203, 3, "netprint-premium")
	custom := album(200, 203, 3, model.CoverCustom)
	for _, c := range []struct {
		name string
		p    *model.Project
		last string
		want string
		deny bool // у выбранного пункта запрет
	}{
		{"готовая Фабрика, запомнена Фотосфера", album(300, 305, 20, "fabrika-photo"), "fsfera", "fabrika", false},
		{"готовая Фотосфера, запомнена Фабрика", album(281, 284, 20, "fsfera-thin"), "fabrika", "fsfera", false},
		{"готовая Фабрика, запомнена «Обычная»", album(300, 305, 20, "fabrika-photo"), "", "fabrika", false},
		{"21×30 без обложки, поля Фабрики, запомнена Фабрика", fabrika21, "fabrika", "fabrika", false},
		{"21×30 без обложки, поля Фабрики, запомнена Фотосфера (привычка сильнее догадки)", fabrika21, "fsfera", "fsfera", false},
		{"21×30 без обложки, поля Фотосферы, запомнена «Обычная»", fsfera21, "", "", false},
		{"21×30 без обложки, поля Фотосферы, запомнена Фотосфера", fsfera21, "fsfera", "fsfera", false},
		{"обложка Нетпринта на 200×203, запомнена «Обычная»", netCover, "", "", false},
		{"«Свои размеры» на 200×203, запомнена «Обычная»", custom, "", "", false},
		{"304×203 + обложка Фотосферы, запомнена Фабрика", marina, "fabrika", "fabrika", true},
		{"304×203 + обложка Фотосферы, запомнена Фотосфера", marina, "fsfera", "fsfera", true},
		{"общая 300×200 без обложки, запомнена Фабрика", album(300, 200, 3, ""), "fabrika", "fabrika", true},
		{"общая 300×200 без обложки, ничего не запомнено", album(300, 200, 3, ""), "", "", false},
		// Запомнена типография с запретом, а страница целиком подошла другой: открывается она.
		{"кнопка Фабрики 30×30, запомнена Фотосфера", onButton(t, album(1, 1, 12, ""), model.CoverGroupFabrika, "30×30"), "fsfera", "fabrika", false},
		{"запомнено неизвестное", album(300, 305, 20, "fabrika-photo"), "xyz", "fabrika", false},
		{"запомнено неизвестное, без обложки и кнопки", album(300, 200, 3, ""), "xyz", "", false},
	} {
		before := model.Marshal(c.p)
		plans := Plans(c.p, "", c.last)
		if string(before) != string(model.Marshal(c.p)) {
			t.Fatalf("%s: Plans изменил проект", c.name)
		}
		var got string
		for _, pl := range plans {
			if pl.Default {
				if got != "" {
					t.Errorf("%s: Default у двух планов", c.name)
				}
				got = pl.Kind
			}
		}
		if got != c.want {
			t.Errorf("%s: открывается %q, ждали %q", c.name, got, c.want)
			continue
		}
		for _, pl := range plans {
			if pl.Kind == got && got != "" && (len(pl.Block) > 0) != c.deny {
				t.Errorf("%s: запрет %q, ждали запрет=%v", c.name, pl.Block, c.deny)
			}
		}
	}
}

// TestDefaultKindPure — порядок 1–4 на планах без альбома: каждый шаг порядка отдельно, чтобы
// откат любого из них ловился.
func TestDefaultKindPure(t *testing.T) {
	free := func(k string) Plan { return Plan{Kind: k, Block: []string{}} }
	deny := func(k string) Plan { return Plan{Kind: k, Block: []string{"запрет"}} }
	for _, c := range []struct {
		name              string
		plans             []Plan
		last, cover, page string
		want              string
	}{
		{"1: обложка без запрета бьёт запомненное", []Plan{free("fsfera"), free("fabrika")}, "fsfera", "fabrika", "fsfera", "fabrika"},
		{"1: обложка бьёт «Обычную»", []Plan{free("fsfera"), free("fabrika")}, "", "fsfera", "", "fsfera"},
		{"2: обложка с запретом — запомненное", []Plan{free("fsfera"), deny("fabrika")}, "fsfera", "fabrika", "", "fsfera"},
		{"2: «Обычная» запрета не имеет и бьёт страницу", []Plan{free("fsfera"), deny("fabrika")}, "", "fabrika", "fsfera", ""},
		{"2: запомненное без запрета бьёт страницу", []Plan{free("fsfera"), free("fabrika")}, "fsfera", "", "fabrika", "fsfera"},
		{"3: запомненное с запретом — страница", []Plan{deny("fsfera"), free("fabrika")}, "fsfera", "", "fabrika", "fabrika"},
		{"4: все с запретом — запомненное", []Plan{deny("fsfera"), deny("fabrika")}, "fsfera", "fabrika", "fabrika", "fsfera"},
		{"4: запомненного нет — обложка", []Plan{deny("fsfera"), deny("fabrika")}, "xyz", "fabrika", "fsfera", "fabrika"},
		{"4: нет и обложки — страница", []Plan{deny("fsfera"), deny("fabrika")}, "xyz", "", "fsfera", "fsfera"},
		{"4: нет ничего — «Обычная»", []Plan{deny("fsfera"), deny("fabrika")}, "xyz", "", "", ""},
		{"планов нет (лист 031)", nil, "fabrika", "fabrika", "fabrika", ""},
		{"обложка не из планов", []Plan{free("fsfera")}, "", "fabrika", "", ""},
		{"пустой вид не ищется среди планов", []Plan{{Kind: "", Block: []string{}}}, "", "", "", ""},
	} {
		if got := DefaultKind(c.plans, c.last, c.cover, c.page); got != c.want {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}

// TestAlbumKinds — типография обложки и единственная типография страницы целиком.
func TestAlbumKinds(t *testing.T) {
	for _, c := range []struct {
		name        string
		p           *model.Project
		cover, page string
	}{
		{"без обложки, общая страница", album(300, 200, 3, ""), "", ""},
		{"21×30 Фабрики", onButton(t, album(1, 1, 3, ""), model.CoverGroupFabrika, "21×30"), "", "fabrika"},
		{"21×30 Фотосферы", onButton(t, album(1, 1, 3, ""), model.CoverGroupFsfera, "21×30"), "", "fsfera"},
		// 304×203 подошла бы Фабрике, но зона и обрез по умолчанию — не её; допуск 0,25 мм не размер губит.
		{"Фабрика 30×20 без кнопки", album(304, 203, 3, ""), "", ""},
		{"Фабрика 30×20 на 1 px уже", func() *model.Project {
			p := onButton(t, album(1, 1, 3, ""), model.CoverGroupFabrika, "30×20")
			p.Settings.PageW, p.Settings.PageH = 303.95, 202.95
			return p
		}(), "", "fabrika"},
		{"обложка Нетпринта", album(200, 203, 3, "netprint-premium"), "", ""},
		{"обложка Фотосферы на странице Фабрики", onButton(t, album(1, 1, 3, "fsfera-thin"), model.CoverGroupFabrika, "30×20"), "fsfera", "fabrika"},
		{"лист 031", model.NewSheet(204, 304), "", ""},
	} {
		if cover, page := AlbumKinds(c.p); cover != c.cover || page != c.page {
			t.Errorf("%s: обложка %q, страница %q; ждали %q, %q", c.name, cover, page, c.cover, c.page)
		}
	}
}

// TestPlansSheet — лист 031: планов нет, [] в JSON (не null).
func TestPlansSheet(t *testing.T) {
	b, _ := json.Marshal(Plans(model.NewSheet(204, 304), "", "fabrika"))
	if string(b) != "[]" {
		t.Fatalf("лист 031: %s", b)
	}
}

// TestPlanDefaultJSON — поле default есть у каждого плана, в том числе false: это контракт с
// интерфейсом (один exe), без omitempty — «Обычная» не выпадает из списка полей.
func TestPlanDefaultJSON(t *testing.T) {
	b, _ := json.Marshal(Plans(album(300, 200, 3, ""), "", ""))
	if strings.Count(string(b), `"default":false`) != 2 || strings.Contains(string(b), `"default":true`) {
		t.Fatalf("нет default у каждого плана: %s", b)
	}
	b, _ = json.Marshal(Plans(album(300, 305, 20, "fabrika-photo"), "", ""))
	if !strings.Contains(string(b), `"default":true`) || !strings.Contains(string(b), `"px":"7087×3602 px"`) {
		t.Fatalf("Фабрика готова: %s", b)
	}
}

// TestCheckPxWhy — «не формат» называет пиксели (123 §1.4): сколько выйдет и сколько у ближайшего
// формата; короткая подсказка Off прежняя; px формата — для памятки.
func TestCheckPxWhy(t *testing.T) {
	pl := Check(album(300, 200, 3, ""), "", Fabrika)
	want := "Страница 300 × 200 мм — не формат Фабрики Фотокниги: разворот выйдет 7087 × 2362 px, у 30×20 — 7181 × 2398 (304 × 203 мм). " +
		"В «Настройках альбома» выберите типографию «Фабрика Фотокниги» и кнопку формата или введите px файла"
	if len(pl.Block) != 1 || pl.Block[0] != want || pl.Off != "страница 300 × 200 мм — не формат Фабрики Фотокниги" || pl.Px != "" || len(pl.Memo) != 0 {
		t.Errorf("300×200: %+v", pl)
	}
	// Край допуска: 0,3 мм от формата — уже чужая страница, 0,25 — своя; px считает холст экспорта.
	for _, c := range []struct {
		w, h float64
		fmt  string // "" — не формат
		px   string
	}{
		{303.95, 202.95, "30×20", "7180×2397 px"},
		{304, 203, "30×20", "7181×2398 px"},
		{303.75, 202.75, "30×20", "7175×2395 px"},
		{303.7, 203, "", ""},
		{304, 203.3, "", ""},
	} {
		got := Check(album(c.w, c.h, 3, ""), "", Fabrika)
		if got.Format != c.fmt || got.Px != c.px {
			t.Errorf("%v×%v: формат %q, px %q; ждали %q, %q", c.w, c.h, got.Format, got.Px, c.fmt, c.px)
		}
		if c.fmt == "" && !strings.Contains(got.Block[0], "у 30×20 — 7181 × 2398 (304 × 203 мм)") {
			t.Errorf("%v×%v: запрет %q", c.w, c.h, got.Block)
		}
		if c.fmt != "" && !slices.Contains(got.Memo, "Файлы: "+c.px) {
			t.Errorf("%v×%v: памятка %q без «Файлы: %s»", c.w, c.h, got.Memo, c.px)
		}
	}
	// Лист 031: причина прежняя, без px.
	if got := Check(model.NewSheet(204, 304), "", Fabrika); got.Px != "" || strings.Contains(got.Block[0], "px") {
		t.Errorf("лист 031: %+v", got)
	}
}

// TestCoverFixPx — «Не по шаблону»: добавлены px файла обложки при 300 dpi; когда px сошлись,
// хвоста нет.
func TestCoverFixPx(t *testing.T) {
	p := album(300, 305, 20, "fabrika-photo")
	if model.CoverFixText(p) != "" {
		t.Fatal("обложка по шаблону")
	}
	p.Cover.Spec.W, p.Cover.Spec.H = 316, 310
	have, want, _, off := p.CoverOffTemplate()
	if !off {
		t.Fatal("обложка должна быть не по шаблону")
	}
	wantTail := fmt.Sprintf(" (файл %d × %d px, нужен %d × %d)", model.Px(have[0], 300), model.Px(have[1], 300), model.Px(want[0], 300), model.Px(want[1], 300))
	if fix := model.CoverFixText(p); !strings.HasSuffix(fix, wantTail) || strings.Count(fix, "px") != 1 {
		t.Errorf("%q, ждали хвост %q", fix, wantTail)
	}
	// Заворот другой, лист того же размера: слова про заворот, без px (они сошлись бы).
	w := album(300, 305, 20, "fabrika-photo")
	w.Cover.Spec.Wrap = 15
	if _, _, _, off := w.CoverOffTemplate(); !off {
		t.Fatal("заворот 15 — не по шаблону")
	}
	if fix := model.CoverFixText(w); strings.Contains(fix, "файл") && strings.Contains(fix, "заворот") {
		t.Errorf("заворот и px вместе: %q", fix)
	}
}

// TestSoleKind — страница целиком у двух типографий сразу: по ней не выбрать (ни одна). Сегодня
// таблицы такого не дают (TestAlbumKinds), поэтому ветка проверяется над списком.
func TestSoleKind(t *testing.T) {
	for _, c := range []struct {
		kinds []string
		want  string
	}{{nil, ""}, {[]string{}, ""}, {[]string{"fabrika"}, "fabrika"}, {[]string{"fsfera", "fabrika"}, ""}, {[]string{"a", "b", "c"}, ""}} {
		if got := soleKind(c.kinds); got != c.want {
			t.Errorf("%q: %q, ждали %q", c.kinds, got, c.want)
		}
	}
}

// TestMemoFolderLast — строка «Папка: …» последняя в памятке (интерфейс окна «Экспорт» берёт
// memo без неё: memo.slice(0, -1)); у «не формата» памятки нет вовсе.
func TestMemoFolderLast(t *testing.T) {
	for _, p := range []*model.Project{
		album(281, 284, 20, "fsfera-thin"), album(300, 305, 20, "fabrika-photo"), album(204, 304, 1, ""),
		album(300, 305, 0, ""), album(300, 200, 3, ""), album(200, 203, 12, "fabrika-layflat-card"), album(200, 203, 3, "netprint-premium"),
	} {
		for _, prof := range Profiles() {
			pl := Check(p, "", prof)
			if len(pl.Memo) == 0 {
				if pl.Format != "" {
					t.Errorf("%s %v×%v: формат %q, памятки нет", prof.Kind, p.Settings.PageW, p.Settings.PageH, pl.Format)
				}
				continue
			}
			if last := pl.Memo[len(pl.Memo)-1]; !strings.HasPrefix(last, "Папка: ") {
				t.Errorf("%s %v×%v: последняя строка памятки %q", prof.Kind, p.Settings.PageW, p.Settings.PageH, last)
			}
			for _, line := range pl.Memo[:len(pl.Memo)-1] {
				if strings.HasPrefix(line, "Папка: ") {
					t.Errorf("%s: вторая строка «Папка» в памятке %q", prof.Kind, pl.Memo)
				}
			}
		}
	}
}
