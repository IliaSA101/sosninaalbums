package album_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// usedProject — альбом, где каждый вид «занятости» свой: место разворота (и его копия), фон
// разворота, лист обложки, фон альбома; шестое фото свободно.
func usedProject() (p *model.Project, ids []string) {
	p = model.New()
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		id := library.IDFor(edtest.Abs(n + ".jpg"))
		ids = append(ids, id)
		p.Photos[id] = model.PhotoRef{Path: edtest.Abs(n + ".jpg"), W: 300, H: 200}
	}
	copyID := model.CopyID(ids[0], 2)
	p.Photos[copyID] = p.Photos[ids[0]]
	p.Spreads = []*model.Spread{
		{ID: "0123456789ab", Photos: []string{ids[0], copyID}, BgPhoto: &model.Backdrop{Photo: ids[1], W: 300, H: 200}},
		{ID: "0123456789ac", Photos: []string{}},
	}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Photos: []string{ids[2]}}}
	p.AlbumBg = &model.AlbumBg{Photo: ids[3]}
	return p, ids
}

// derived — то, что agentFree берёт из вида: места листов и обложки (по файлам) и BackdropUsed.
func derived(v model.View) (placed []string, onBg map[string]bool) {
	set := map[string]bool{}
	for _, sv := range v.Spreads {
		for _, id := range sv.Photos {
			set[model.BaseID(id)] = true
		}
	}
	if v.Cover != nil && !v.Settings.Sheet {
		for _, id := range v.Cover.Sheet.Photos {
			set[model.BaseID(id)] = true
		}
	}
	onBg = map[string]bool{}
	for _, id := range v.BackdropUsed {
		onBg[id] = true
	}
	return slices.Sorted(maps.Keys(set)), onBg
}

// UsedFiles совпадает с выводом из View (места с обложкой и копией, фон разворота, фон альбома),
// читает без шага отмены и без смены rev, а круг Save→Load его не меняет.
func TestUsedFilesMatchesView(t *testing.T) {
	check := func(name string, p *model.Project) {
		t.Helper()
		e := edtest.Open(t, p, "")
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		placed, onBg := e.UsedFiles()
		wantP, wantB := derived(e.View())
		if !slices.Equal(placed, wantP) || !maps.Equal(onBg, wantB) {
			t.Errorf("%s: UsedFiles = %v %v, из вида %v %v", name, placed, onBg, wantP, wantB)
		}
		if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
			t.Errorf("%s: чтение изменило rev или отмену", name)
		}
	}
	p, ids := usedProject()
	check("полный", p)
	e := edtest.Open(t, p, "")
	placed, onBg := e.UsedFiles()
	if want := slices.Sorted(slices.Values([]string{ids[0], ids[2]})); !slices.Equal(placed, want) {
		t.Errorf("места: %v, ждали a (с копией — один файл) и c с обложки", placed)
	}
	if len(onBg) != 2 || !onBg[ids[1]] || !onBg[ids[3]] {
		t.Errorf("фоны: %v, ждали b (фон разворота) и d (фон альбома)", onBg)
	}
	// Круг записи: загрузка вправе почистить обложку без раскладки, поэтому сверка — с видом
	// загруженного проекта, а фоны и место с копией остаются.
	q := edtest.SaveLoad(t, e)
	check("после Save→Load", q)
	placed2, onBg2 := edtest.Open(t, q, "").UsedFiles()
	if !slices.Contains(placed2, ids[0]) || !maps.Equal(onBg, onBg2) {
		t.Errorf("после Save→Load: %v %v, было %v %v", placed2, onBg2, placed, onBg)
	}
	check("пустой", model.New())
	empty := model.New()
	empty.Spreads = []*model.Spread{{ID: "0123456789ab", Photos: []string{}}}
	check("развороты без фото", empty)
	if pl, bg := edtest.Open(t, model.New(), "").UsedFiles(); len(pl) != 0 || len(bg) != 0 {
		t.Errorf("пустой альбом: %v %v", pl, bg)
	}
}
