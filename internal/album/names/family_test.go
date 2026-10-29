package names_test

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Копии фото (098 G2) и автоимена: «новое фото» — по файлу, а не по записи; копия имя наследует;
// повтор броска (нет запаса знаков, вопрос «остальные») разбирает запрос копии заново.

// fileInfo — фото с названием file и ID по пути, как у ленты (семьи копий считаются по пути).
func fileInfo(file string) model.PhotoInfo { return edtest.Lib(edtest.Abs(file), model.XmpMarks{}) }

// copyOf — запрос копии фото ph.
func copyOf(ph model.PhotoInfo) model.PhotoInfo { ph.ID = model.CopyReq(ph.ID); return ph }

// family — записи семьи id и проверка инварианта семьи (только её записи: у прочих фото capsEd
// ID не по пути).
func family(t *testing.T, e *album.Editor, id string) []string {
	t.Helper()
	p := edtest.P(e)
	fam := p.Family(id)
	only := &model.Project{Photos: map[string]model.PhotoRef{}}
	for _, x := range fam {
		only.Photos[x] = p.Photos[x]
	}
	if err := edtest.FamilyInvariant(only); err != nil {
		t.Fatal(err)
	}
	return fam
}

// TestNamesAutoFamily — копия имя наследует и в Named не попадает; файл, стоящий в альбоме только
// копией, — не новое фото: брошенная исходная запись имени из файла не получает.
func TestNamesAutoFamily(t *testing.T) {
	e := capsEd(t, 6, nameTpl)
	a := fileInfo("01_иванов_петя.jpg")
	a2 := model.CopyID(a.ID, 2)
	if _, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{a}, -1, restAdd); err != nil || !slices.Equal(out.Named, []string{a.ID}) {
		t.Fatalf("первый бросок: %v, named %v", err, out.Named)
	}
	undo := edtest.UndoLen(e)
	_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{copyOf(a)}, -1, restAdd)
	if err != nil || out.Named != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("копия: %v, named %v, шагов +%d", err, out.Named, edtest.UndoLen(e)-undo)
	}
	if got := who(e, a2); got != "Иванов Петя" {
		t.Fatalf("имя копии %q — копия наследует имя", got)
	}
	e.RemovePhotos([]string{a.ID}) // файл стоит только копией; запись исходного ушла с именем
	if fam := family(t, e, a.ID); !slices.Equal(fam, []string{a2}) {
		t.Fatalf("семья после «убрать»: %v", fam)
	}
	undo = edtest.UndoLen(e)
	_, out, err = e.DropPhotos("s1", "", []model.PhotoInfo{a}, -1, restAdd)
	if err != nil || out.Named != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("исходное при стоящей копии: %v, named %v, шагов +%d", err, out.Named, edtest.UndoLen(e)-undo)
	}
	if got := who(e, a.ID); got != "" {
		t.Fatalf("исходное получило имя %q, хотя файл уже в альбоме", got)
	}
	// Круг записи: имя копии и семья целы.
	q := edtest.SaveLoad(t, e)
	if r := q.Photos[a2]; r.Person == nil || r.Person.Last != "Иванов" || !q.PlacedFile(a.ID) || len(q.Family(a.ID)) != 2 {
		t.Fatalf("после Save → Load: копия %+v, семья %v", r.Person, q.Family(a.ID))
	}
}

// TestCopyReqDropRetry — запрос копии в броске, который повторяется: без запаса знаков (второй
// заход без имён) и после вопроса «остальные» (второй вызов с ответом). Каждый заход разбирает
// запрос над откаченным проектом: копия одна (номер 2), лишних записей нет, шаг отмены один.
func TestCopyReqDropRetry(t *testing.T) {
	t.Run("без запаса знаков", func(t *testing.T) {
		e := capsEd(t, 6, nameTpl)
		a, b := fileInfo("IMG_0001.jpg"), fileInfo("петрова_анна.jpg")
		if _, err := e.AddPhotos("s2", []model.PhotoInfo{a}, -1); err != nil {
			t.Fatal(err)
		}
		brim(t, e)
		undo, photos := edtest.UndoLen(e), len(edtest.P(e).Photos)
		_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{copyOf(a), b}, -1, "")
		if err != nil || out.Named != nil || out.Ask || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("бросок: %v, %+v, шагов +%d", err, out, edtest.UndoLen(e)-undo)
		}
		a2 := model.CopyID(a.ID, 2)
		p := edtest.P(e)
		if fam := family(t, e, a.ID); !slices.Equal(fam, []string{a.ID, a2}) || len(p.Photos) != photos+2 {
			t.Fatalf("семья %v, записей %d → %d", fam, photos, len(p.Photos))
		}
		if !slices.Contains(p.Spreads[0].Photos, a2) || !slices.Contains(p.Spreads[0].Photos, b.ID) ||
			!slices.Contains(p.Spreads[1].Photos, a.ID) || who(e, b.ID) != "" {
			t.Fatalf("листы %v | %v, имя b %q", p.Spreads[0].Photos, p.Spreads[1].Photos, who(e, b.ID))
		}
		e.Undo()
		if fam := family(t, e, a.ID); !slices.Equal(fam, []string{a.ID}) {
			t.Fatalf("отмена: семья %v", fam)
		}
	})
	t.Run("вопрос и ответ", func(t *testing.T) {
		e := capsEd(t, 1, nameTpl)
		a, b := fileInfo("IMG_0001.jpg"), fileInfo("IMG_0002.jpg")
		if _, err := e.AddPhotos("s2", []model.PhotoInfo{a}, -1); err != nil {
			t.Fatal(err)
		}
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		drop := []model.PhotoInfo{copyOf(a), b}
		_, out, err := e.DropPhotos("s1", "", drop, -1, "")
		if err != nil || !out.Ask || edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
			t.Fatalf("вопрос: %v, %+v, rev +%d", err, out, edtest.Rev(e)-rev)
		}
		if fam := family(t, e, a.ID); !slices.Equal(fam, []string{a.ID}) {
			t.Fatalf("после вопроса осталась запись: %v", fam)
		}
		if _, _, err := e.DropPhotos("s1", "", drop, -1, restAdd); err != nil || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("ответ: %v, шагов +%d", err, edtest.UndoLen(e)-undo)
		}
		a2 := model.CopyID(a.ID, 2)
		if fam := family(t, e, a.ID); !slices.Equal(fam, []string{a.ID, a2}) || !edtest.P(e).Placed(a2) {
			t.Fatalf("после ответа: семья %v", fam)
		}
	})
}
