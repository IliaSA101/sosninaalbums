package album_test

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// Рамочки мест (079, design §12 «Ссылки»): запись картинки рамочки — не фото альбома.

const frKey = "my:0123456789abcdef"

// TestBorderRefNotAlbumPhoto — запись рамочки не в ленте альбома (BackdropUsed), её не берут
// «Заполнить пустые» и автосборка, «Убрать из альбома» её не снимает (холостая), счётчики пустых
// мест и фото разворота не меняются; так же после «Собрать в папку», где lib снят и остался путь.
func TestBorderRefNotAlbumPhoto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a"), edtest.Gp(130, 10, 80, 80, "")}, []string{"b"})
	fr := model.LibPhotoID(frKey)
	before := e.View()
	edtest.Change(e, func(p *model.Project) {
		p.Photos[fr] = model.PhotoRef{Lib: frKey, Path: edtest.Abs("frame.png"), W: 30, H: 40}
		for _, pl := range p.Spreads[0].Grid {
			pl.Border = &model.Border{Photo: fr, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}
		}
	})
	frInfo := model.PhotoInfo{ID: fr, Path: edtest.Abs("frame.png"), W: 30, H: 40, Lib: frKey}
	check := func(what string) {
		t.Helper()
		v := e.View()
		if !slices.Contains(v.BackdropUsed, fr) || v.EmptyPlaces != before.EmptyPlaces || slices.Contains(v.Spreads[0].Photos, fr) ||
			len(v.Spreads[0].Frames) != len(before.Spreads[0].Frames) {
			t.Fatalf("%s: в ленте %v, пустых %d (было %d), фото %v", what, v.BackdropUsed, v.EmptyPlaces, before.EmptyPlaces, v.Spreads[0].Photos)
		}
		for _, pv := range v.Spreads[0].Places {
			if pv.Border == nil || pv.Border.Photo != fr {
				t.Fatalf("%s: место %s без рамочки", what, pv.ID)
			}
		}
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		e.RemovePhotos([]string{fr})
		edtest.Same(t, e, rev, undo, what+": «Убрать из альбома»")
		_, left, _ := e.FillPlaces([]model.PhotoInfo{frInfo}, "s1")
		if edtest.P(e).Spreads[0].Grid[1].Photo != "" || left != 0 {
			t.Fatalf("%s: «Заполнить пустые» посадила рамочку (осталось %d)", what, left)
		}
		if _, err := e.AutoBuild([]model.PhotoInfo{frInfo, {ID: "x", Path: edtest.Abs("x.jpg"), W: 300, H: 200}}, -1, model.AutoOpts{Spreads: 1, MaxPer: 4}); err != nil {
			t.Fatal(err)
		}
		if edtest.P(e).Placed(fr) {
			t.Fatalf("%s: автосборка посадила рамочку", what)
		}
		if err := edtest.GridInvariants(edtest.P(e)); err != nil {
			t.Fatal(err)
		}
	}
	check("своя из библиотеки")
	edtest.Change(e, func(p *model.Project) { // «Собрать в папку»: lib снят, остался путь
		r := p.Photos[fr]
		r.Lib = ""
		p.Photos[fr] = r
	})
	frInfo.Lib = ""
	check("после сбора")
}

// TestBorderPath — путь для /media/f (079-22, 079-35): только запись под рамочкой места, в том числе
// на обложке; фото места, фото фона, пустой и чужой ID, своя без файла — нет; снятая рамочка путь
// закрывает, отмена — открывает снова.
func TestBorderPath(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
	fr, gone, bg := model.LibPhotoID(frKey), model.LibPhotoID("my:00000000000000aa"), "bg"
	edtest.Change(e, func(p *model.Project) {
		p.Photos[fr] = model.PhotoRef{Lib: frKey, Path: edtest.Abs("frame.png"), W: 30, H: 40}
		p.Photos[gone] = model.PhotoRef{Lib: "my:00000000000000aa", W: 30, H: 40} // своя без файла
		p.Photos[bg] = model.PhotoRef{Path: edtest.Abs("bg.jpg"), W: 300, H: 200}
		p.Spreads[0].BgPhoto = &model.Backdrop{Photo: bg}
		p.Spreads[0].Grid[0].Border = &model.Border{Photo: gone}
		p.Cover.Sheet.Grid[0].Border = &model.Border{Photo: fr, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}
	})
	if path, ok := e.BorderPath(fr); !ok || path != edtest.Abs("frame.png") {
		t.Fatalf("рамочка обложки: %q %v", path, ok)
	}
	for _, id := range []string{"a", "cv", bg, gone, "", "нет"} {
		if path, ok := e.BorderPath(id); ok || path != "" {
			t.Errorf("BorderPath(%q) = %q, %v — ждали отказ", id, path, ok)
		}
	}
	if _, err := e.ApplyBorder(model.CoverID, []string{edtest.CoverPlace}, model.BorderSrc{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.BorderPath(fr); ok {
		t.Fatal("рамочка снята, а путь отдаётся")
	}
	e.Undo()
	if _, ok := e.BorderPath(fr); !ok {
		t.Fatal("после отмены путь не вернулся")
	}
}

// TestBorderFramePath — своя рамочка, загруженная и в «Фоны» (079-40): путь общей записи — фона, а
// /media/f (BorderPath) и вид печати (BorderView.File) берут копию из «Рамочек» — на двух местах
// разворота и на обложке. Копии нет, искатель снят или его путь не локальный — путь записи. Вид
// холста файла не несёт.
func TestBorderFramePath(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 60, "a"), edtest.Gp(130, 10, 80, 80, "")})
	fr := model.LibPhotoID(frKey)
	bgPath, frPath := edtest.Abs("bg.png"), edtest.Abs("frame.png")
	cut := [4]float64{0.1, 0.1, 0.1, 0.1}
	edtest.Change(e, func(p *model.Project) {
		p.Photos[fr] = model.PhotoRef{Lib: frKey, Path: bgPath, W: 4200, H: 300}
		p.Spreads[0].BgPhoto = &model.Backdrop{Photo: fr}
		for _, pl := range p.Spreads[0].Grid {
			pl.Border = &model.Border{Photo: fr, Cut: cut}
		}
		p.Cover.Sheet.Grid[0].Border = &model.Border{Photo: fr, Cut: cut}
	})
	if ids := e.BorderIDs(); !slices.Equal(ids, []string{fr}) { // три места — одна запись
		t.Fatalf("BorderIDs: %v", ids)
	}
	t.Cleanup(func() { salbum.SetFrameFinder(nil) })
	files := func(v model.View) []string {
		var out []string
		for _, s := range []model.SpreadView{v.Spreads[0], v.Cover.Sheet} {
			for _, pv := range s.Places {
				if pv.Border != nil {
					out = append(out, pv.Border.File)
				}
			}
		}
		return out
	}
	for _, c := range []struct {
		name string
		find salbum.LibFinder
		want string
	}{
		{"копия в «Рамочках»", func(key string) (string, bool) { return frPath, key == frKey }, frPath},
		{"копии нет", func(string) (string, bool) { return "", false }, bgPath},
		{"путь не локальный", func(string) (string, bool) { return `\srv\share\frame.png`, true }, bgPath},
		{"искатель снят", nil, bgPath},
	} {
		salbum.SetFrameFinder(c.find)
		if path, ok := e.BorderPath(fr); !ok || path != c.want {
			t.Errorf("%s: BorderPath %q %v, ждали %q", c.name, path, ok, c.want)
		}
		_, v := e.Snapshot()
		if got := files(v); !slices.Equal(got, []string{c.want, c.want, c.want}) {
			t.Errorf("%s: файлы печати %q", c.name, got)
		}
		if got := files(e.View()); !slices.Equal(got, []string{"", "", ""}) {
			t.Errorf("%s: вид холста несёт файл %q", c.name, got)
		}
		if ref, _ := e.Ref(fr); ref.Path != bgPath {
			t.Errorf("%s: запись фона переписана: %q", c.name, ref.Path)
		}
	}
	// Запись без пути — пропавшая до RelinkLib: копия из «Рамочек» её не подменяет.
	salbum.SetFrameFinder(func(string) (string, bool) { return frPath, true })
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[fr]
		r.Path = ""
		p.Photos[fr] = r
	})
	if path, ok := e.BorderPath(fr); ok || path != "" {
		t.Errorf("запись без пути: %q", path)
	}
	// Вид печати тоже без файла: копия из «Рамочек» пустую запись не подменяет.
	_, v := e.Snapshot()
	if got := files(v); !slices.Equal(got, []string{"", "", ""}) {
		t.Errorf("запись без пути: файлы печати %q", got)
	}
}
