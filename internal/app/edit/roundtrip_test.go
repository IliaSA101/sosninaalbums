package edit

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Круг Save → Load после правок через методы разных областей сразу: обложка с фото и
// надписью, виньетка, фон-фото, имя, стиль подписей листа, свои отметки. Загруженный альбом
// показывает то же, что показывал сохранённый (без rev и служебного счёта).
func TestSaveLoadAfterEdits(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("L")
	three := 3

	if _, err := k.CreateCover("", "L"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.CoverSpineText(); err != nil {
		t.Fatal(err)
	}
	k.SetCoverSpineBg("#112233")
	if _, err := k.AddVignette(2); err != nil {
		t.Fatal(err)
	}
	if _, err := k.SetBackdrop("s2", "left", "a"); err != nil {
		t.Fatal(err)
	}
	k.SetBackdropFade("s2", "left", 0.4, "")
	k.SetPerson("b", model.Person{Last: "Иванов", First: "Пётр"}, "")
	k.SetSheetCaps("s1", true, model.SheetCaps{Pad: 2, Text: "{Фамилия}"})
	k.SetSpreadName("s1", "Церемония")
	if _, err := k.SetMarks([]string{"L"}, model.MarkPatch{Rating: &three}); err != nil {
		t.Fatal(err)
	}
	wantMarks := k.Marks()

	path := filepath.Join(t.TempDir(), "x.salbum")
	if err := k.ed.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	ed, err := album.NewEditor(q, path)
	if err != nil {
		t.Fatal(err)
	}
	got := ed.View()
	// ID фото при загрузке пересчитываются (от пути): сверяем состав по счёту, связи — по новым ID.
	if len(got.Spreads) != 3 || len(got.Spreads[0].Photos) != 2 || len(got.Spreads[1].Photos) != 1 || got.Cover == nil ||
		got.Cover.SpineBg != "#112233" || len(got.Cover.Sheet.Backdrops) != 1 || len(got.Cover.Sheet.Texts) != 1 {
		t.Fatalf("после загрузки состав другой: %v, cover %v", photosOf(got), got.Cover != nil)
	}
	if got.Spreads[0].Name != "Церемония" || got.Spreads[0].Caps == nil || got.Spreads[0].Caps.Pad != 2 {
		t.Errorf("лист s1: имя %q, стиль %+v", got.Spreads[0].Name, got.Spreads[0].Caps)
	}
	a, b := got.Spreads[0].Photos[0], got.Spreads[0].Photos[1]
	if bd := got.Spreads[1].Backdrops; len(bd) != 1 || bd[0].Photo != a || bd[0].Fade != 0.4 || bd[0].Scope != "left" {
		t.Errorf("фон s2: %+v (ждали фото %s)", bd, a)
	}
	if got.Spreads[2].Vig == nil {
		t.Error("виньетка потерялась")
	}
	if r, _ := ed.Ref(b); r.Person == nil || r.Person.Last != "Иванов" || r.Person.First != "Пётр" {
		t.Errorf("имя потерялось: %+v", r.Person)
	}
	m := ed.Marks()
	if l := got.Cover.Sheet.Backdrops[0].Photo; len(m) != len(wantMarks) || m[l].Rating == nil || *m[l].Rating != 3 {
		t.Errorf("отметки: %+v, было %+v", m, wantMarks)
	}
}
