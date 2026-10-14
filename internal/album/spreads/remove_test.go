package spreads_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// «Убрать из альбома» снимает фото, стоящие только фоном, с фонов всех листов (с обложкой)
// одним шагом отмены; фон фото, стоящего и в месте, и фон с другим фото стоят. Итог
// переживает Save→Load.
func TestRemovePhotosClearsBackdrops(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	edtest.SetBg(t, e, s1, model.ScopeSpread, "x") // только фон
	edtest.SetBg(t, e, s2, model.ScopeLeft, "a")   // a и в месте s1, и фоном s2
	edtest.SetBg(t, e, s2, model.ScopeRight, "keep")
	edtest.Change(e, func(p *model.Project) {
		pr, _ := model.CoverPresetOf(model.CoverDefault)
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Photos: []string{}}}
	})
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpread, "z") // только фон, на обложке
	undo := edtest.UndoLen(e)

	v := e.RemovePhotos([]string{"x", "z", "a", "нет-такого"})
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	check := func(when string, e *album.Editor) {
		t.Helper()
		if b := sheetBd(e, s1, model.ScopeSpread); b != nil {
			t.Errorf("%s: фон x на %s остался", when, s1)
		}
		// После Load ID пересчитаны по путям (Rekey): фото узнаём по пути.
		refs := e.Refs()
		if b := sheetBd(e, s2, model.ScopeLeft); b == nil || refs[b.Photo].Path != edtest.Abs("a.jpg") {
			t.Errorf("%s: снят фон фото, стоявшего и в месте: %+v", when, b)
		}
		if b := sheetBd(e, s2, model.ScopeRight); b == nil || refs[b.Photo].Path != edtest.Abs("keep.jpg") {
			t.Errorf("%s: чужой фон снят: %+v", when, b)
		}
		if b := sheetBd(e, model.CoverID, model.ScopeSpread); b != nil {
			t.Errorf("%s: фон z на обложке остался", when)
		}
		for _, r := range refs {
			if r.Path == edtest.Abs("x.jpg") || r.Path == edtest.Abs("z.jpg") {
				t.Errorf("%s: в Refs осталось %s", when, r.Path)
			}
		}
	}
	check("правка", e)
	if slices.Contains(v.Spreads[0].Photos, "a") {
		t.Fatalf("a в местах: %v", v.Spreads[0].Photos)
	}
	file := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(file, filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	check("Save→Load", edtest.Open(t, q, ""))

	e.Undo()
	if b := sheetBd(e, s1, model.ScopeSpread); b == nil || b.Photo != "x" {
		t.Fatalf("отмена: фон x %+v", b)
	}
	if b := sheetBd(e, model.CoverID, model.ScopeSpread); b == nil || b.Photo != "z" {
		t.Fatalf("отмена: фон обложки %+v", b)
	}
	if !slices.Contains(e.View().Spreads[0].Photos, "a") {
		t.Fatal("отмена: a не вернулось в место")
	}
}

// sheetBd — фон области листа (и обложки, в отличие от edtest.BdOf); листа нет — nil.
func sheetBd(e *album.Editor, id, scope string) *model.Backdrop {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	if s := edtest.P(e).Sheet(id); s != nil {
		return *s.Slot(scope)
	}
	return nil
}
