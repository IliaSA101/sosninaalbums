package album_test

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Переезд папки переносит отметки только своей папки: на Linux «Весна» и «ВЕСНА» — две папки
// (101, п. 4), на Windows и macOS — одна, и по папке переезжают обе (на один новый путь — одна).
func TestRelinkMarksFolderCase(t *testing.T) {
	dir := testkit.TempDir(t)
	low, up := filepath.Join(dir, "Весна"), filepath.Join(dir, "ВЕСНА")
	dst := filepath.Join(dir, "Новая")
	pl, pu := filepath.Join(low, "1.jpg"), filepath.Join(up, "1.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.SetMarks(t, e, []model.PhotoInfo{edtest.Lib(pl, model.XmpMarks{}), edtest.Lib(pu, model.XmpMarks{})}, model.MarkPatch{Flag: edtest.Ip(1)})
	delta := edtest.RelinkMarks(e, nil, []model.PathPair{{Old: low, New: dst}})
	added, gone := map[string]bool{}, map[string]bool{}
	for _, c := range delta {
		if c.After != nil {
			added[c.ID] = true
		} else {
			gone[c.ID] = true
		}
	}
	if !added[library.IDFor(filepath.Join(dst, "1.jpg"))] || len(delta) != 2 {
		t.Fatalf("отметка папки «Весна» не переехала: %+v", delta)
	}
	if paths.FoldEqual(low, up) {
		return // одна папка: на тот же новый путь переехала одна из двух отметок
	}
	if !gone[library.IDFor(pl)] || gone[library.IDFor(pu)] {
		t.Fatalf("отметка «ВЕСНА» уехала вместе с «Весна»: %+v", delta)
	}
}
