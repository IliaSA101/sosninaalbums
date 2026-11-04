package album

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// BaseFaceNorm: память листа берётся как есть, негодная (nil, не число, размер ≤ 0) — начальная норма.
func TestBaseFaceNormWrapper(t *testing.T) {
	good := geom.FaceNorm{Size: 0.5, Eye: 0.4}
	if got := BaseFaceNorm(&good); got != good {
		t.Errorf("память листа: %+v", got)
	}
	for name, n := range map[string]*geom.FaceNorm{
		"nil": nil, "NaN": {Size: math.NaN(), Eye: 0.3}, "ноль": {Size: 0, Eye: 0.3}, "минус": {Size: -1, Eye: 0.3}, "Inf": {Size: math.Inf(1)},
	} {
		if got := BaseFaceNorm(n); got != geom.DefaultFaceNorm {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// LoadBase: нет файла и пустой файл — ошибка, а не паника; Save поверх папки — ошибка,
// путь Editor и признак «изменён» прежние, правка (rev) не потеряна.
func TestLoadBaseAndSaveFailures(t *testing.T) {
	dir := testkit.TempDir(t)
	if p, err := LoadBase(filepath.Join(dir, "нет.salbum"), dir); err == nil || p != nil {
		t.Errorf("нет файла: %v %v", p, err)
	}
	empty := filepath.Join(dir, "пусто.salbum")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if p, err := LoadBase(empty, dir); err == nil || p != nil {
		t.Errorf("пустой файл: %v %v", p, err)
	}

	e := newEditor(t, model.New(), "")
	newSpread(e, -1, photos("a"))
	rev, dirty, path := e.rev, e.dirty, e.path
	busy := filepath.Join(dir, "занято.salbum") // на месте файла лежит папка: замена отказывает
	if err := os.Mkdir(busy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := e.Save(busy); err == nil {
		t.Fatal("запись поверх папки не отказана")
	}
	if e.rev != rev || e.dirty != dirty || e.path != path {
		t.Fatalf("отказ записи тронул состояние: rev %d→%d dirty %v→%v path %q→%q", rev, e.rev, dirty, e.dirty, path, e.path)
	}
}

// Перечитывание разметки в фоне не затирает слот, если за это время выбрали другой путь или
// файл уже прочитан: ни сведений, ни пометки «идёт перечитывание» не появляется из чужого результата.
func TestReprobeGuideStaleSlot(t *testing.T) {
	e := newEditor(t, model.New(), "")
	e.guides[markupCover] = guideState{busy: true}
	e.reprobeGuide(markupCover, filepath.Join(testkit.TempDir(t), "другая.png")) // слот на пути ""
	if g := e.guides[markupCover]; !g.busy || g.OK || g.Path != "" {
		t.Errorf("слот с другим путём тронут: %+v", g)
	}
	e.guides[markupSpreads] = guideState{busy: true}
	e.guides[markupSpreads].Path, e.guides[markupSpreads].OK = "p.png", true
	e.reprobeGuide(markupSpreads, "p.png")
	if g := e.guides[markupSpreads]; !g.busy || !g.OK {
		t.Errorf("прочитанный слот тронут: %+v", g)
	}
}

// markVig: у вида без листа 031 (нет разворота / обычный разворот) имя шаблона и «Ручная» не проставляются.
func TestMarkVigNotVignette(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}}
	v := model.View{Spreads: []model.SpreadView{
		{ID: "s1", Vig: &model.VignetteView{}},
		{ID: "нет", Vig: &model.VignetteView{}},
		{ID: "s1"},
	}}
	markVigs(p, &v)
	for i, sv := range v.Spreads {
		if sv.Vig != nil && (sv.Vig.TplName != "" || sv.Vig.Manual) {
			t.Errorf("вид %d помечен: %+v", i, sv.Vig)
		}
	}
}
