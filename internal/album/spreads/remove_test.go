package spreads_test

import (
	"fmt"
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

// «Убрать из альбома» в полосе «Не найдено» (075-13): фото уходят из мест, с фонов всех листов
// (с обложкой) — и стоящие в месте тоже, — и своими стикерами на нескольких листах; одним
// шагом отмены. Рисунок, чужие фото, фоны и стикеры стоят. Итог переживает Save→Load.
func TestPurgePhotos(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	edtest.Change(e, func(p *model.Project) {
		pr, _ := model.CoverPresetOf(model.CoverDefault)
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Photos: []string{}}}
	})
	edtest.SetBg(t, e, s1, model.ScopeSpread, "a")   // a и в месте, и фоном
	edtest.SetBg(t, e, s2, model.ScopeLeft, "x")     // только фон
	edtest.SetBg(t, e, s2, model.ScopeRight, "keep") // чужой фон
	edtest.SetBg(t, e, model.CoverID, model.ScopeSpread, "z")
	ownSticker(t, e, s1, "st")
	ownSticker(t, e, model.CoverID, "st")
	ownSticker(t, e, s2, "keepst")
	if _, id, err := e.AddSticker(s2, 0, model.StickerSrc{Art: &model.Art{Kind: "plate"}}); err != nil || id == "" {
		t.Fatalf("рисунок: %q, %v", id, err)
	}
	before := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)

	v := e.PurgePhotos([]string{"a", "x", "z", "st", "нет-такого", ""})
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d → %d, undo %d → %d: ждали один шаг", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	if len(v.Spreads) != 2 || !slices.Equal(v.Spreads[0].Photos, []string{"b"}) || !slices.Equal(v.Spreads[1].Photos, []string{"c"}) {
		t.Fatalf("места: %+v", v.Spreads)
	}
	purged(t, "правка", e, s1, s2)
	file := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(file, filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	purged(t, "Save→Load", edtest.Open(t, q, ""), s1, s2)

	e.Undo()
	if got := model.Marshal(edtest.P(e)); string(got) != string(before) {
		t.Fatalf("отмена вернула не тот проект:\n%s\n%s", got, before)
	}
	// RemovePhotos (бросок в ленту) фон фото, стоящего и в месте, оставляет — PurgePhotos нет.
	e.RemovePhotos([]string{"a"})
	if b := sheetBd(e, s1, model.ScopeSpread); b == nil || b.Photo != "a" {
		t.Fatalf("RemovePhotos снял фон a: %+v", b)
	}
}

// purged — итог TestPurgePhotos. После Load ID пересчитаны (Rekey): фото узнаём по пути.
func purged(t *testing.T, when string, e *album.Editor, s1, s2 string) {
	t.Helper()
	refs := e.Refs()
	path := func(b *model.Backdrop) string { return refs[b.Photo].Path }
	for _, c := range []struct{ id, scope string }{{s1, model.ScopeSpread}, {s2, model.ScopeLeft}, {model.CoverID, model.ScopeSpread}} {
		if b := sheetBd(e, c.id, c.scope); b != nil {
			t.Errorf("%s: фон %s/%s остался: %s", when, c.id, c.scope, path(b))
		}
	}
	if b := sheetBd(e, s2, model.ScopeRight); b == nil || path(b) != edtest.Abs("keep.jpg") {
		t.Errorf("%s: чужой фон снят: %+v", when, b)
	}
	if got := stickerPaths(e, s1, refs); len(got) != 0 {
		t.Errorf("%s: стикеры s1 %v", when, got)
	}
	if got := stickerPaths(e, model.CoverID, refs); len(got) != 0 {
		t.Errorf("%s: стикеры обложки %v", when, got)
	}
	if got := stickerPaths(e, s2, refs); !slices.Equal(got, []string{edtest.Abs("keepst.jpg"), "рисунок"}) {
		t.Errorf("%s: стикеры s2 %v", when, got)
	}
	gone := []string{edtest.Abs("a.jpg"), edtest.Abs("x.jpg"), edtest.Abs("z.jpg"), edtest.Abs("st.jpg")}
	for _, r := range refs {
		if slices.Contains(gone, r.Path) {
			t.Errorf("%s: в Refs осталось %s", when, r.Path)
		}
	}
}

// Холостая «Убрать из альбома»: nil, пустой список, неизвестные и пустые ID — без шага и rev.
func TestPurgePhotosNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, ids := range [][]string{nil, {}, {"нет"}, {""}} {
		e.PurgePhotos(ids)
		edtest.Same(t, e, rev, undo, fmt.Sprintf("PurgePhotos(%q)", ids))
	}
}

// ownSticker — свой стикер из фото id на лист sid.
func ownSticker(t *testing.T, e *album.Editor, sid, id string) {
	t.Helper()
	if _, sk, err := e.AddSticker(sid, 0, model.StickerSrc{Photo: edtest.Photo(id)}); err != nil || sk == "" {
		t.Fatalf("стикер %s на %s: %q, %v", id, sid, sk, err)
	}
}

// stickerPaths — стикеры листа по порядку: путь фото или «рисунок».
func stickerPaths(e *album.Editor, sid string, refs map[string]model.PhotoRef) []string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	var out []string
	for _, st := range edtest.P(e).Sheet(sid).Stickers {
		if st.Photo == "" {
			out = append(out, "рисунок")
		} else {
			out = append(out, refs[st.Photo].Path)
		}
	}
	return out
}

// «Убрать из альбома» снимает рамочку-картинку, чья запись в списке (079-38): у места с фото и у
// пустого, на листе и на обложке; места, их фото, встроенная рамочка и чужая картинка стоят. Один
// шаг, отмена возвращает всё; итог переживает Save→Load.
func TestPurgePhotosBorder(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 60, "a"), edtest.Gp(130, 10, 80, 80, ""), edtest.Gp(10, 100, 60, 60, "b")}, []string{"c"})
	cut := [4]float64{0.1, 0.1, 0.1, 0.1}
	edtest.Change(e, func(p *model.Project) {
		p.Photos["fr"] = model.PhotoRef{Path: edtest.Abs("fr.png"), W: 300, H: 200}
		p.Photos["keep"] = model.PhotoRef{Path: edtest.Abs("keep.png"), W: 300, H: 200}
		g := p.Spreads[0].Grid
		g[0].Border, g[1].Border = &model.Border{Photo: "fr", Cut: cut}, &model.Border{Photo: "fr", Cut: cut}
		g[2].Border = &model.Border{Kind: "line"}
		p.Cover.Sheet.Grid[0].Border = &model.Border{Photo: "fr", Cut: cut}
		p.Spreads[1].Grid = []*model.Place{{ID: "000000000099", X: 10, Y: 10, W: 50, H: 50, Photo: "c", Border: &model.Border{Photo: "keep", Cut: cut}}}
	})
	before := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.PurgePhotos([]string{"fr"})
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d → %d, undo %d → %d: ждали один шаг", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	check := func(when string, e *album.Editor) {
		t.Helper()
		p := edtest.P(e)
		g := p.Spreads[0].Grid
		if len(g) != 3 || g[0].Photo == "" || g[0].Border != nil || g[1].Border != nil || g[2].Border == nil || g[2].Border.Kind != "line" {
			t.Fatalf("%s: места листа %+v %+v %+v", when, g[0], g[1], g[2])
		}
		if cv := p.Cover.Sheet.Grid[0]; cv.Border != nil || cv.Photo == "" {
			t.Fatalf("%s: обложка %+v", when, cv)
		}
		if k := p.Spreads[1].Grid[0].Border; k == nil || p.Photos[k.Photo].Path != edtest.Abs("keep.png") {
			t.Fatalf("%s: чужая рамочка снята: %+v", when, k)
		}
		for _, r := range p.Photos {
			if r.Path == edtest.Abs("fr.png") {
				t.Fatalf("%s: запись рамочки осталась", when)
			}
		}
		if err := edtest.BorderInvariant(p); err != nil {
			t.Fatalf("%s: %v", when, err)
		}
	}
	check("правка", e)
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
	if got := model.Marshal(edtest.P(e)); string(got) != string(before) {
		t.Fatalf("отмена вернула не тот проект:\n%s\n%s", got, before)
	}
}
