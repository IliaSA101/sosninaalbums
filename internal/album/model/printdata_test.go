package model

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// steps — таблица корешка из пар (до n разворотов, мм), как её пишет CSV шаблона.
func steps(pairs ...float64) []SpineStep { return stepsOf(pairs) }

// fabrikaCoverPresets — изделия Фабрики из общей таблицы (бывшая функция fabrika.go).
func fabrikaCoverPresets() []CoverPreset {
	return slices.DeleteFunc(CoverPresets(), func(c CoverPreset) bool { return c.Group != CoverGroupFabrika })
}

// TestGroupNamesMatchProfiles — константы групп совпадают с названиями встроенных профилей, а id
// форматов и порядок групп не изменились: имя группы лежит в .salbum и в настройках окна.
func TestGroupNamesMatchProfiles(t *testing.T) {
	want := map[string]string{"fsfera": CoverGroupFsfera, "netprint": CoverGroupNetprint, "fabrika": CoverGroupFabrika}
	var order []string
	for _, p := range printprof.Builtin().Profiles() {
		if want[p.ID] != p.Name {
			t.Errorf("профиль %s: название %q, константа %q", p.ID, p.Name, want[p.ID])
		}
		order = append(order, p.Name)
	}
	var groups []string
	for _, g := range PrintGroups() {
		groups = append(groups, g.Name)
	}
	if !slices.Equal(order, groups) || !slices.Equal(groups, []string{"Фотосфера", "Нетпринт", "Фабрика Фотокниги"}) {
		t.Errorf("порядок групп %v, профилей %v", groups, order)
	}
	if f := FabrikaFormats()[3]; f.ID != "fabrika-30x20" || FsferaFormats()[6].ID != "fsfera-21x30" {
		t.Errorf("id форматов: %s", f.ID)
	}
}

// TestTablesFollowSnapshot — таблицы берутся из действующего снимка: подмена профиля меняет
// форматы, изделия и пределы, возврат — возвращает; отдаваемые копии менять безопасно.
func TestTablesFollowSnapshot(t *testing.T) {
	base := printprof.Builtin().Profiles()
	p := base[2] // Фабрика
	p.Formats = slices.Clone(p.Formats)
	p.Formats[3].Page = printprof.Pair{305, 204}
	snap, err := printprof.NewSnapshot([]printprof.Profile{base[0], base[1], p})
	if err != nil {
		t.Fatal(err)
	}
	printprof.UseForTest(t, snap)
	if got := FabrikaFormats()[3]; got.PageW != 305 || got.PageH != 204 {
		t.Errorf("формат не из снимка: %+v", got)
	}
	if f, ok := FormatOf(CoverGroupFabrika, Settings{PageW: 305, PageH: 204}); !ok || f.Name != "30×20" {
		t.Errorf("FormatOf по подменённому: %v %v", f.Name, ok)
	}
	// Пустой снимок: ни типографий, ни изделий, кроме «Своих размеров».
	empty, _ := printprof.NewSnapshot(nil)
	printprof.UseForTest(t, empty)
	if len(PrintGroups()) != 0 || len(PrintLimits()) != 0 || len(CoverPresets()) != 1 || len(PagePresets()) != 6 || FabrikaFormats() != nil {
		t.Errorf("пустой снимок: групп %d, пределов %d, изделий %d, кнопок %d", len(PrintGroups()), len(PrintLimits()), len(CoverPresets()), len(PagePresets()))
	}
	if _, ok := CoverPresetOf("fabrika-photo"); ok {
		t.Error("изделие есть в пустом снимке")
	}
	// Id, которого нет в реестре, хранится (своя типография на чужом компьютере); мусор — «Свои размеры».
	if got := ClampCoverSpec(CoverSpec{Preset: "fabrika-photo", Wrap: 5}); got.Preset != "fabrika-photo" || CoverMissing(got.Preset) == "" {
		t.Errorf("id отсутствующего изделия потерян: %q", got.Preset)
	}
	if got := ClampCoverSpec(CoverSpec{Preset: "Fabrika Photo", Wrap: 5}); got.Preset != CoverCustom || CoverMissing(CoverCustom) != "" {
		t.Errorf("мусорный id не стал «Своими размерами»: %q", got.Preset)
	}
}

// TestTablesAreCopies — вызывающий вправе менять отданные таблицы: они копии, а не общие срезы.
func TestTablesAreCopies(t *testing.T) {
	before := builtinTablesLines(t)
	pr, _ := CoverPresetOf("fabrika-photo")
	pr.Spec.Steps[0].Mm = 99
	for _, c := range CoverPresets() {
		if len(c.Spec.Steps) > 0 {
			c.Spec.Steps[0].Mm = 99
		}
	}
	cf, _ := CoverPresetFor("fabrika-photo-2", Settings{PageW: 304, PageH: 203})
	cf.Spec.Steps[0].Mm = 99
	FabrikaFormats()[0].PageW = 1
	PagePresets()[8].W = 1
	PrintGroups()[0].Gen = "x"
	PrintLimits()[0].Max = 999
	tg, _ := templateGroupOf(CoverGroupFabrika)
	tg.Formats()[0].PageW = 1
	if after := builtinTablesLines(t); !slices.Equal(before, after) {
		t.Fatal("правка отданной копии изменила таблицы")
	}
}
