package album_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Копии фото (098 G2): запрос «<ID файла>-new» в прежних правках, «в альбоме» по файлу, «где стоит».

// copyInfos — сведения о фото a, b, c… с ID по пути (как у ленты).
func copyInfos(t *testing.T, n int) []model.PhotoInfo {
	t.Helper()
	dir := testkit.TempDir(t)
	out := make([]model.PhotoInfo, n)
	for i := range out {
		path := filepath.Join(dir, fmt.Sprintf("%c.jpg", 'a'+i))
		out[i] = model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 300 + 10*i, H: 200}
	}
	return out
}

// req — запрос копии фото ph.
func req(ph model.PhotoInfo) model.PhotoInfo { ph.ID = model.CopyReq(ph.ID); return ph }

// checkStep — правка прибавила ровно один шаг отмены, семьи целы, круг записи их не меняет.
func checkStep(t *testing.T, e *album.Editor, what string, undo int) {
	t.Helper()
	if got := edtest.UndoLen(e); got != undo+1 {
		t.Fatalf("%s: шагов отмены %d → %d, ждали +1", what, undo, got)
	}
	p := edtest.P(e)
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if err := edtest.Reopen(p); err != nil {
		t.Fatalf("%s: круг записи: %v", what, err)
	}
}

// Все четыре входа: запрос копии ставит новую запись, исходное фото остаётся на месте, один шаг
// отмены; отмена убирает копию целиком.
func TestCopyReqAllEntries(t *testing.T) {
	ph := copyInfos(t, 3)
	a, a2 := ph[0].ID, model.CopyID(ph[0].ID, 2)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[:2])
	auto := edtest.P(e).Spreads[0].ID
	edtest.NewSpread(e, -1, ph[2:3])
	grid := edtest.P(e).Spreads[1].ID
	e.SetGrid(grid, true)
	place := edtest.P(e).Spreads[1].Grid[0].ID
	entries := []struct {
		name string
		do   func() error
	}{
		{"AddPhotos", func() error { _, err := e.AddPhotos(auto, []model.PhotoInfo{req(ph[0])}, -1); return err }},
		{"NewSpread", func() error { _, err := e.NewSpread(-1, []model.PhotoInfo{req(ph[0])}); return err }},
		{"PlacePhotos", func() error {
			_, _, _, err := e.PlacePhotos(grid, place, []model.PhotoInfo{req(ph[0])})
			return err
		}},
		{"DropPhotos", func() error {
			_, _, err := e.DropPhotos(grid, "", []model.PhotoInfo{req(ph[0])}, -1, "add")
			return err
		}},
	}
	for _, c := range entries {
		undo := edtest.UndoLen(e)
		if err := c.do(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		checkStep(t, e, c.name, undo)
		p := edtest.P(e)
		if !slices.Contains(p.Spreads[0].Photos, a) || !p.Placed(a2) {
			t.Fatalf("%s: исходное %v, копия стоит %v", c.name, p.Spreads[0].Photos, p.Placed(a2))
		}
		if _, ok := p.Photos[model.CopyReq(a)]; ok {
			t.Fatalf("%s: запрос остался записью", c.name)
		}
		e.Undo()
		if _, ok := edtest.P(e).Photos[a2]; ok {
			t.Fatalf("%s: отмена не убрала копию", c.name)
		}
	}
}

// «Перенести сюда» — интерфейс шлёт запись единственного места (UsedAt.Move): переезжает именно
// она, исходное фото стоит, где стояло.
func TestCopyMoveRecord(t *testing.T) {
	ph := copyInfos(t, 2)
	a, a2 := ph[0].ID, model.CopyID(ph[0].ID, 2)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[:1])
	e.NewSpread(-1, []model.PhotoInfo{req(ph[0])})
	edtest.NewSpread(e, -1, ph[1:2])
	to := edtest.P(e).Spreads[2].ID
	w := e.WhereUsed([]string{model.CopyReq(a)})[0]
	if w.ID != a || w.Places != 2 || w.Move != "" {
		t.Fatalf("два места: %+v", w)
	}
	e.RemovePhotos([]string{a})
	w = e.WhereUsed([]string{a})[0]
	if w.Places != 1 || w.Move != a2 {
		t.Fatalf("одно место копией: %+v", w)
	}
	edtest.NewSpread(e, 0, ph[:1]) // исходное снова стоит — на новом первом листе
	undo := edtest.UndoLen(e)
	if _, _, err := e.DropPhotos(to, "", []model.PhotoInfo{{ID: a2, Path: ph[0].Path, W: ph[0].W, H: ph[0].H}}, -1, ""); err != nil {
		t.Fatal(err)
	}
	checkStep(t, e, "перенос", undo)
	p := edtest.P(e)
	if s := p.Spreads[len(p.Spreads)-1]; !slices.Contains(s.Photos, a2) || !slices.Contains(p.Spreads[0].Photos, a) {
		t.Fatalf("перенос записи: последний %v, первый %v", s.Photos, p.Spreads[0].Photos)
	}
}

// Файл, стоящий только копией, «в альбоме»: «Заполнить пустые» и автосборка его не берут.
func TestFillSkipsFileInAlbum(t *testing.T) {
	ph := copyInfos(t, 3)
	a := ph[0].ID
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[:1])
	e.AddPhotos(edtest.P(e).Spreads[0].ID, []model.PhotoInfo{req(ph[0])}, -1)
	e.RemovePhotos([]string{a})
	edtest.NewSpread(e, -1, ph[1:2])
	g := edtest.P(e).Spreads[1].ID
	e.SetGrid(g, true)
	e.RemovePhotos([]string{ph[1].ID}) // место сетки пустеет
	if pl, err := e.AutoPlan([]model.PhotoInfo{ph[0], ph[2]}, model.AutoOpts{Spreads: 1, MaxPer: 64}); err != nil || pl.Photos != 1 {
		t.Fatalf("автосборка: %+v, %v", pl, err)
	}
	_, left, _ := e.FillPlaces([]model.PhotoInfo{ph[0], ph[2]}, "")
	p := edtest.P(e)
	if left != 0 || p.Placed(a) || !p.Placed(ph[2].ID) {
		t.Fatalf("заполнение: left %d, a стоит %v, c стоит %v", left, p.Placed(a), p.Placed(ph[2].ID))
	}
}

// Убрали исходное — копия стоит; «где стоит» и карточка считают всю семью и фон альбома.
func TestUsesFamily(t *testing.T) {
	ph := copyInfos(t, 1)
	a, a2 := ph[0].ID, model.CopyID(ph[0].ID, 2)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph)
	e.NewSpread(-1, []model.PhotoInfo{req(ph[0])})
	uses, used := e.PhotoUses(a2)
	want := []model.PhotoUse{{Sheet: 0, Label: "Разворот 1 — место", Ref: a}, {Sheet: 1, Label: "Разворот 2 — место"}}
	if !slices.Equal(uses, want) || used != "Использовано 2 раза" {
		t.Fatalf("вхождения копии: %+v, %q", uses, used)
	}
	lib := model.LibPhotoID("my:00000000000000aa")
	edtest.Change(e, func(p *model.Project) {
		p.Photos[lib] = model.PhotoRef{Lib: "my:00000000000000aa", Path: filepath.Join(filepath.Dir(ph[0].Path), "лого.png"), W: 10, H: 10}
		p.AlbumBg = &model.AlbumBg{Photo: lib}
	})
	if uses, _ := e.PhotoUses(lib); len(uses) != 1 || uses[0].Sheet != -1 || uses[0].Label != "Фон альбома" {
		t.Fatalf("фон альбома: %+v", uses)
	}
	e.RemovePhotos([]string{a})
	if p := edtest.P(e); !p.Placed(a2) || p.Placed(a) {
		t.Fatal("убрали исходное — копия должна остаться")
	}
	if w := e.WhereUsed([]string{a, "нет такого"}); len(w) != 2 || w[0].Places != 1 || w[1].Uses == nil || w[1].Places != 0 {
		t.Fatalf("WhereUsed: %+v", w)
	}
}

// «Убрать из альбома» снимает фон альбома с пропавшим фото одним шагом.
func TestPurgeAlbumBg(t *testing.T) {
	ph := copyInfos(t, 1)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph)
	lib := model.LibPhotoID("my:00000000000000aa")
	edtest.Change(e, func(p *model.Project) {
		p.Photos[lib] = model.PhotoRef{Lib: "my:00000000000000aa", Path: "", W: 10, H: 10}
		p.AlbumBg = &model.AlbumBg{Photo: lib}
	})
	undo := edtest.UndoLen(e)
	e.PurgePhotos([]string{lib})
	p := edtest.P(e)
	if p.AlbumBg != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("фон альбома %+v, шагов %d → %d", p.AlbumBg, undo, edtest.UndoLen(e))
	}
	if _, ok := p.Photos[lib]; ok {
		t.Fatal("запись фото фона осталась")
	}
}

// Все номера копий заняты — каждый вход отказывает без шага отмены, проект прежний.
func TestCopyReqLimit(t *testing.T) {
	ph := copyInfos(t, 2)
	a := ph[0].ID
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[:1])
	edtest.NewSpread(e, -1, ph[1:])
	g := edtest.P(e).Spreads[1].ID
	e.SetGrid(g, true)
	pl := edtest.P(e).Spreads[1].Grid[0].ID
	edtest.Change(e, func(p *model.Project) { // 98 копий стоят на первом листе
		for n := 2; n <= model.MaxCopy; n++ {
			p.Photos[model.CopyID(a, n)] = p.Photos[a]
			p.Spreads[0].Photos = append(p.Spreads[0].Photos, model.CopyID(a, n))
		}
	})
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	r := []model.PhotoInfo{req(ph[0])}
	_, err1 := e.AddPhotos(g, r, -1)
	_, err2 := e.NewSpread(-1, r)
	_, _, _, err3 := e.PlacePhotos(g, pl, r)
	_, _, err4 := e.DropPhotos(g, "", r, -1, "add")
	for i, err := range []error{err1, err2, err3, err4} {
		if !errors.Is(err, model.ErrCopies) {
			t.Errorf("вход %d: %v, ждали ErrCopies", i, err)
		}
	}
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("отказ оставил шаг: шагов %d → %d, rev %d → %d", undo, edtest.UndoLen(e), rev, edtest.Rev(e))
	}
}

// FileRef — сведения файла по записи, ID файла или запросу копии: запись с наименьшим номером.
func TestFileRef(t *testing.T) {
	ph := copyInfos(t, 1)
	a := ph[0].ID
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph)
	e.NewSpread(-1, []model.PhotoInfo{req(ph[0])})
	e.RemovePhotos([]string{a}) // осталась только копия
	for _, id := range []string{a, model.CopyReq(a), model.CopyID(a, 2)} {
		if r, ok := e.FileRef(id); !ok || r.Path != ph[0].Path || r.W != ph[0].W {
			t.Fatalf("FileRef(%q) = %+v, %v", id, r, ok)
		}
	}
	if _, ok := e.FileRef("нет такого"); ok {
		t.Fatal("FileRef неизвестного")
	}
}

// Бросок на плитку «+ Обложка» копию не делает: фото стоит на своём месте и становится фоном
// лицевой той же записью, одним шагом; запрос копии (интерфейс его сюда не шлёт) — то же самое.
func TestCreateCoverNoCopy(t *testing.T) {
	for _, asReq := range []bool{false, true} {
		ph := copyInfos(t, 2)
		a := ph[0].ID
		e := edtest.Open(t, model.New(), "")
		edtest.NewSpread(e, -1, ph)
		undo := edtest.UndoLen(e)
		in := ph[0]
		if asReq {
			in = req(ph[0])
		}
		if _, err := e.CreateCover("", &in); err != nil {
			t.Fatal(err)
		}
		checkStep(t, e, "обложка", undo)
		p := edtest.P(e)
		bg := p.Cover.Sheet.BgPhotoR
		if bg == nil || bg.Photo != a || !slices.Equal(p.Family(a), []string{a}) || !slices.Contains(p.Spreads[0].Photos, a) {
			t.Fatalf("запрос %v: фон %+v, семья %v, лист %v", asReq, bg, p.Family(a), p.Spreads[0].Photos)
		}
		if len(p.Photos) != 2 {
			t.Fatalf("запрос %v: записей %d, ждали 2", asReq, len(p.Photos))
		}
		e.Undo()
		if edtest.P(e).Cover != nil || edtest.UndoLen(e) != undo {
			t.Fatalf("запрос %v: отмена не сняла обложку", asReq)
		}
	}
}
