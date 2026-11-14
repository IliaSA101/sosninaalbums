package collect

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Формат v17 (098 §2.3, §4.3): фон альбома и копии фото при сборе.

// itemOf — элемент плана с источником src; нет — t.Fatal.
func itemOf(t *testing.T, pl Plan, src string) Item {
	t.Helper()
	i := slices.IndexFunc(pl.Items, func(it Item) bool { return it.Src == src })
	if i < 0 {
		t.Fatalf("нет в плане: %s", src)
	}
	return pl.Items[i]
}

// TestCollectAlbumBg — фото фона альбома из библиотеки собирается в «Свои фоны и стикеры», запись
// смотрит на новый путь без lib; при повторном сборе (lib уже снят) — снова туда, а не в
// «Фото/Прочие» (ленту). Фото ленты, стоящее и в месте, и фоном альбома, остаётся с фото.
func TestCollectAlbumBg(t *testing.T) {
	sc := newScene(t)
	own := filepath.Join(sc.root, "Библиотека", "fedcba9876543210.png")
	put(t, own, "album-bg-png")
	edtest.LibFinder(t, map[string]string{libKey: filepath.Join(sc.root, "Библиотека", "0123456789abcdef.png"), frameKey: own})
	bg := model.LibPhotoID(frameKey)
	sc.p.Photos[bg] = model.PhotoRef{Lib: frameKey, Path: own, W: 300, H: 400, Size: 12}
	sc.p.AlbumBg = &model.AlbumBg{Photo: bg, Fade: 0.3}

	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, path := collectTo(t, sc, dir, false)
	want := filepath.Join(dir, DirOwn, "fedcba9876543210.png")
	if it := itemOf(t, pl, own); it.Kind != KindOwn || !slices.Equal(it.Refs, []string{bg}) {
		t.Fatalf("элемент фона альбома: %+v", it)
	}
	if in := pl.Info(); in.Own != 2 {
		t.Fatalf("своих в сводке %d, ждали 2 (стикер и фон альбома): %+v", in.Own, in)
	}
	salbum.SetLibFinder(nil) // другой компьютер: библиотеки нет
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if lp.AlbumBg == nil || lp.AlbumBg.Fade != 0.3 {
		t.Fatalf("фон альбома после сбора: %+v", lp.AlbumBg)
	}
	if id, ref := lp.AlbumBg.Photo, lp.Photos[lp.AlbumBg.Photo]; ref.Lib != "" || ref.Path != want || id != library.IDFor(want) {
		t.Fatalf("фото фона альбома %s → %+v", id, ref)
	}

	again := &scene{root: sc.root, p: lp, marks: edtest.Loaded(lp).Marks}
	dir2 := filepath.Join(sc.root, "Флешка2", "Иванов")
	pl2, path2 := collectTo(t, again, dir2, false)
	if _, err := os.Stat(filepath.Join(dir2, DirOwn, "fedcba9876543210.png")); err != nil {
		t.Errorf("фон альбома не в «%s»: %v", DirOwn, err)
	}
	if _, err := os.Stat(filepath.Join(dir2, DirPhotos, DirOther, "fedcba9876543210.png")); err == nil {
		t.Errorf("фон альбома попал в ленту: %s/%s", DirPhotos, DirOther)
	}
	if in := pl2.Info(); in.Own != 2 {
		t.Fatalf("второй сбор: своих %d, ждали 2: %+v", in.Own, in)
	}
	rp, err := salbum.Load(path2)
	if err != nil {
		t.Fatal(err)
	}
	if rp.AlbumBg == nil || rp.Photos[rp.AlbumBg.Photo].Path != filepath.Join(dir2, DirOwn, "fedcba9876543210.png") {
		t.Fatalf("фон альбома после второго сбора: %+v", rp.AlbumBg)
	}

	// Фото ленты в месте и фоном альбома — фото альбома, в своей папке.
	a1 := filepath.Join(sc.a, "IMG_0001.jpg")
	plain := newScene(t)
	plain.p.AlbumBg = &model.AlbumBg{Photo: library.IDFor(a1)}
	pp, err := Make(plain.p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if it := itemOf(t, pp, filepath.Join(plain.a, "IMG_0001.jpg")); it.Kind != KindPhoto || it.Rel != "Фото/JPG/IMG_0001.jpg" {
		t.Fatalf("фото ленты фоном альбома: %+v", it)
	}
}

// TestCollectCopiesOneFile — копии (098): файл с копиями — один элемент плана со всеми записями
// семьи; «фото альбома» и «пропавших» в сводке — по файлам, а Plan.Missing — по записям (его
// сверяет Save); в собранном альбоме обе записи смотрят на один новый файл и сохраняют номер.
func TestCollectCopiesOneFile(t *testing.T) {
	sc := newScene(t)
	a1 := filepath.Join(sc.a, "IMG_0001.jpg")
	base, gone := library.IDFor(a1), library.IDFor(sc.gone)
	cp, gcp := model.CopyID(base, 2), model.CopyID(gone, 3)
	sc.p.Photos[cp], sc.p.Photos[gcp] = sc.p.Photos[base], sc.p.Photos[gone]
	last := sc.p.Spreads[len(sc.p.Spreads)-1]
	last.Photos = append(last.Photos, cp, gcp)
	if err := edtest.FamilyInvariant(sc.p); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, path := collectTo(t, sc, dir, false)
	if it := itemOf(t, pl, a1); !slices.Equal(it.Refs, []string{base, cp}) {
		t.Fatalf("записи файла с копией: %v", it.Refs)
	}
	if !slices.Equal(pl.Missing, []string{gone, gcp}) {
		t.Fatalf("пропавшие записи: %v", pl.Missing)
	}
	if in := pl.Info(); in.Photos != 5 || in.Missing != 1 {
		t.Fatalf("сводка по файлам: фото %d (ждали 5), пропавших %d (ждали 1)", in.Photos, in.Missing)
	}
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := edtest.FamilyInvariant(lp); err != nil {
		t.Fatal(err)
	}
	na1 := filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg")
	nb := library.IDFor(na1)
	got := lp.Spreads[len(lp.Spreads)-1].Photos
	if want := []string{gone, model.CopyID(nb, 2), gcp}; !slices.Equal(got, want) {
		t.Fatalf("последний лист: %v, ждали %v", got, want)
	}
	if fam := lp.Family(nb); !slices.Equal(fam, []string{nb, model.CopyID(nb, 2)}) || lp.Photos[fam[1]].Path != na1 {
		t.Fatalf("семья в собранном: %v", fam)
	}
	if lost := lostOf(lp); len(lost) != 2 || lost[0] != sc.gone || lost[1] != sc.gone {
		t.Fatalf("пропавшие в собранном: %v", lost)
	}
}

// Сводка пропавших по файлам: пусто — 0; копии и исходная одного файла — один; разные файлы — по
// одному.
func TestInfoMissingByFile(t *testing.T) {
	for _, c := range []struct {
		ids  []string
		want int
	}{
		{nil, 0},
		{[]string{"0123456789abcdef"}, 1},
		{[]string{"0123456789abcdef", "0123456789abcdef-2", "0123456789abcdef-99"}, 1},
		{[]string{"0123456789abcdef-2", "fedcba9876543210", "fedcba9876543210-3"}, 2},
	} {
		if got := (Plan{Missing: c.ids}).Info().Missing; got != c.want {
			t.Errorf("%v: %d, ждали %d", c.ids, got, c.want)
		}
	}
}
