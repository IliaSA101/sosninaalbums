package collect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// rels — источник → Rel плана.
func rels(pl Plan) map[string]string {
	out := map[string]string{}
	for _, it := range pl.Items {
		out[it.Src] = it.Rel
	}
	return out
}

// album — проект с фото по путям (файлы создаются, если create) и папками folders.
func album(t *testing.T, folders []string, files ...string) *model.Project {
	t.Helper()
	p := model.New()
	p.Folders = folders
	for i, f := range files {
		put(t, f, strings.Repeat("x", 5+i))
		p.Photos[library.IDFor(f)] = model.PhotoRef{Path: f, W: 3, H: 2}
	}
	return p
}

// Сц. 5: повтор имён папок и файлов (и регистр), «Прочие» среди папок, устройство в имени,
// длинное имя с расширением, сайдкар — за новым именем фото.
func TestCollectNames(t *testing.T) {
	root := testkit.TempDir(t)
	a, b, c := filepath.Join(root, "1", "JPG"), filepath.Join(root, "2", "jpg"), filepath.Join(root, "3", "Прочие")
	long := strings.Repeat("я", 70) + ".jpeg"
	o1, o2, o3 := filepath.Join(root, "x", "IMG.jpg"), filepath.Join(root, "y", "img.JPG"), filepath.Join(root, "z", "IMG.jpg")
	files := []string{filepath.Join(a, "IMG.jpg"), filepath.Join(b, "IMG.jpg"), filepath.Join(c, "CON.jpg"), filepath.Join(c, long), o1, o2, o3}
	p := album(t, []string{a, b, c}, files...)
	put(t, o3+".xmp", "<x/>")
	pl, err := Make(p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		files[0]:    "Фото/JPG/IMG.jpg",
		files[1]:    "Фото/jpg (2)/IMG.jpg",
		files[2]:    "Фото/Прочие (2)/Фото.jpg",
		files[3]:    "Фото/Прочие (2)/" + strings.Repeat("я", 64) + ".jpeg",
		o1:          "Фото/Прочие/IMG.jpg",
		o2:          "Фото/Прочие/img (2).JPG",
		o3:          "Фото/Прочие/IMG (3).jpg",
		o3 + ".xmp": "Фото/Прочие/IMG (3).jpg.xmp",
	}
	if got := rels(pl); !mapsEqual(got, want) {
		t.Fatalf("имена:\n%v\nждали\n%v", got, want)
	}
	if in := pl.Info(); in.Photos != 7 || in.Extra != 0 || in.Own != 0 || in.Guides != 0 || in.Missing != 0 {
		t.Fatalf("сводка: %+v", in)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Корни: вложенная папка альбома — часть внешней (не отдельный корень), повтор папки в списке —
// один корень; имена — в порядке папок альбома; корень диска — «Папка».
func TestCollectRoots(t *testing.T) {
	r := makeRoots([]string{`/a/b/JPG`, `/a/JPG/sub`, `/a/JPG`, `/a/JPG/`, `/c/Прочие`, `/d/JPG`})
	got := make([]string, len(r))
	for i, x := range r {
		got[i] = filepath.ToSlash(x.path) + "=" + x.name
	}
	want := []string{"/a/b/JPG=JPG", "/a/JPG=JPG (2)", "/c/Прочие=Прочие (2)", "/d/JPG=JPG (3)"}
	if !slices.Equal(got, want) {
		t.Fatalf("корни %v, ждали %v", got, want)
	}
	if r := makeRoots([]string{string(filepath.Separator)}); len(r) != 1 || r[0].name != "Папка" {
		t.Fatalf("корень диска: %+v", r)
	}
	if makeRoots(nil) != nil {
		t.Fatal("без папок корней нет")
	}
}

// Сайдкар: сначала «имя.jpg.xmp», затем «имя.xmp» (как сканер); один сайдкар на два фото с общей
// основой — один раз; имя сайдкара занято чужим файлом — не копируется (SideBusy).
func TestCollectSidecar(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	j, pn, k := filepath.Join(a, "IMG.jpg"), filepath.Join(a, "IMG.png"), filepath.Join(a, "K.jpg")
	p := album(t, []string{a}, j, pn, k)
	put(t, filepath.Join(a, "IMG.xmp"), "<short/>")
	put(t, k+".xmp", "<long/>")
	put(t, filepath.Join(a, "K.xmp"), "<не тот/>")
	pl, err := Make(p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		j: "Фото/A/IMG.jpg", pn: "Фото/A/IMG.png", k: "Фото/A/K.jpg",
		filepath.Join(a, "IMG.xmp"): "Фото/A/IMG.xmp", k + ".xmp": "Фото/A/K.jpg.xmp",
	}
	if got := rels(pl); !mapsEqual(got, want) {
		t.Fatalf("сайдкары:\n%v\nждали\n%v", got, want)
	}
	// В папке назначения IMG.xmp чужой (другой размер) — сайдкар не копируется, фото — да.
	dir := filepath.Join(root, "Сбор")
	put(t, filepath.Join(dir, "Фото", "A", "IMG.xmp"), "<чужой, другой длины/>")
	if pl, err = Make(p, nil, dir, os.Stat); err != nil {
		t.Fatal(err)
	}
	if pl.SideBusy != 1 || rels(pl)[filepath.Join(a, "IMG.xmp")] != "" || len(pl.Items) != 4 {
		t.Fatalf("занятое имя сайдкара: %d, %v", pl.SideBusy, rels(pl))
	}
}

// Режим «Все»: из ленты берутся только фото внутри папок альбома, без повтора фото альбома;
// пропавшие (и своё без файла) — в Missing; разметки без файла — MissingGuides; байты — без Skip.
func TestCollectExtraAndMissing(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	in := filepath.Join(a, "in.jpg")
	p := album(t, []string{a}, in)
	unused, outside := filepath.Join(a, "sub", "u.jpg"), filepath.Join(root, "B", "o.jpg")
	put(t, unused, "12345678")
	put(t, outside, "1")
	gone := filepath.Join(a, "gone.jpg")
	p.Photos[library.IDFor(gone)] = model.PhotoRef{Path: gone}
	p.Photos[model.LibPhotoID(libKey)] = model.PhotoRef{Lib: libKey} // своё, которого нет в библиотеке
	p.SpreadGuide = filepath.Join(root, "нет.psd")
	pl, err := Make(p, []string{unused, outside, in, "", filepath.Join(a, "пропало.jpg")}, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(pl); len(got) != 2 || got[unused] != "Фото/A/sub/u.jpg" || got[in] != "Фото/A/in.jpg" {
		t.Fatalf("план: %v", got)
	}
	slices.Sort(pl.Missing)
	wantMiss := []string{library.IDFor(gone), model.LibPhotoID(libKey)}
	slices.Sort(wantMiss)
	if !slices.Equal(pl.Missing, wantMiss) || pl.MissingGuides != 1 {
		t.Fatalf("пропавшие %v, разметки %d", pl.Missing, pl.MissingGuides)
	}
	if info := pl.Info(); info.Photos != 1 || info.Extra != 1 || info.Missing != 2 || info.MissingGuides != 1 || info.Bytes != 5+8 {
		t.Fatalf("сводка: %+v", info)
	}
}

// Один файл у двух записей (фото альбома и своё по тому же пути) — один элемент с обоими ID,
// лежит с фото; источник не обычный файл (папка) — пропавший.
func TestCollectSameFileOnce(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	f := filepath.Join(a, "f.png")
	p := album(t, []string{a}, f)
	p.Photos[model.LibPhotoID(libKey)] = model.PhotoRef{Lib: libKey, Path: strings.ToUpper(f[:1]) + f[1:]}
	dirPhoto := filepath.Join(a, "папка.jpg")
	if err := os.MkdirAll(dirPhoto, 0o755); err != nil {
		t.Fatal(err)
	}
	p.Photos[library.IDFor(dirPhoto)] = model.PhotoRef{Path: dirPhoto}
	pl, err := Make(p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Items) != 1 || pl.Items[0].Kind != KindPhoto || len(pl.Items[0].Refs) != 2 || len(pl.Missing) != 1 {
		t.Fatalf("план: %+v, пропавшие %v", pl.Items, pl.Missing)
	}
}

// Один файл — фото альбома и своё: итоговый вид «фото» при любом порядке записей. Photos
// обходится по отсортированным ID, а в TestCollectSameFileOnce ID — хэши случайного пути, так что
// ветка «своё уже добавлено, пришло фото» (sources: смена KindOwn → KindPhoto) проходилась
// в половине прогонов. Здесь ID заданы: «a…» всегда раньше «b…».
func TestCollectOwnThenPhotoKind(t *testing.T) {
	for _, c := range []struct{ name, ownID, photoID string }{
		{"своё раньше фото", "a-own", "b-photo"},   // ветка смены вида
		{"фото раньше своего", "b-own", "a-photo"}, // вид «фото» уже есть
	} {
		t.Run(c.name, func(t *testing.T) {
			root := testkit.TempDir(t)
			a := filepath.Join(root, "A")
			f := filepath.Join(a, "f.png")
			p := album(t, []string{a}, f)
			clear(p.Photos)
			put(t, f, "xxxxx")
			p.Photos[c.ownID] = model.PhotoRef{Lib: libKey, Path: f}
			p.Photos[c.photoID] = model.PhotoRef{Path: f, W: 3, H: 2}
			// Второй файл, только своё: вид «своё» не должен смениться на «фото».
			only := filepath.Join(root, "own-only.png")
			put(t, only, "yy")
			p.Photos["c-only"] = model.PhotoRef{Lib: "my:9", Path: only}
			pl, err := Make(p, nil, "", os.Stat)
			if err != nil {
				t.Fatal(err)
			}
			kinds := map[string]string{}
			for _, it := range pl.Items {
				kinds[it.Src] = it.Kind
				if it.Src == f && len(it.Refs) != 2 {
					t.Errorf("ссылки файла: %v", it.Refs)
				}
			}
			if len(pl.Items) != 2 || kinds[f] != KindPhoto || kinds[only] != KindOwn {
				t.Fatalf("виды: %v, элементов %d", kinds, len(pl.Items))
			}
		})
	}
}

// Имя занято 999 раз — ошибка, а не бесконечный цикл и не перезапись.
func TestCollectTooManyDups(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	f := filepath.Join(a, "IMG.jpg")
	p := album(t, []string{a}, f)
	dir := filepath.Join(root, "Сбор")
	dest := filepath.Join(dir, "Фото", "A")
	put(t, filepath.Join(dest, "IMG.jpg"), "чужой")
	for n := 2; n <= maxDup; n++ {
		put(t, filepath.Join(dest, fmt.Sprintf("IMG (%d).jpg", n)), "чужой")
	}
	if _, err := Make(p, nil, dir, os.Stat); err == nil || !strings.Contains(err.Error(), "слишком много") {
		t.Fatalf("999 занятых имён: %v", err)
	}
}

// fakeInfo — Stat без диска (BenchmarkPlan).
type fakeInfo struct{ size int64 }

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

// План 5 000 записей (design §2: O(N), доли секунды) — без диска: Stat подменён, папки назначения нет.
func BenchmarkPlan(b *testing.B) {
	base := filepath.Join(os.TempDir(), "bench069")
	p := model.New()
	for i := range 10 {
		p.Folders = append(p.Folders, filepath.Join(base, fmt.Sprintf("Камера %d", i)))
	}
	for i := range 5000 {
		path := filepath.Join(p.Folders[i%10], fmt.Sprintf("п%d", i%7), fmt.Sprintf("IMG_%04d.jpg", i/10))
		p.Photos[library.IDFor(path)] = model.PhotoRef{Path: path}
	}
	stat := func(path string) (fs.FileInfo, error) {
		if strings.HasSuffix(path, ".xmp") {
			return nil, fs.ErrNotExist
		}
		return fakeInfo{1 << 20}, nil
	}
	b.ResetTimer()
	for b.Loop() {
		if pl, err := Make(p, nil, "", stat); err != nil || len(pl.Items) != 5000 {
			b.Fatalf("%d, %v", len(pl.Items), err)
		}
	}
}

// SizeText: МБ до гигабайта с запятой, ГБ — с одним знаком, совсем мало — «< 0,1 МБ».
func TestSizeText(t *testing.T) {
	for n, want := range map[int64]string{
		0: "< 0,1 МБ", 50 << 10: "< 0,1 МБ", 512 << 10: "0,5 МБ", 4608 << 10: "4,5 МБ",
		120 << 20: "120 МБ", 1 << 30: "1,0 ГБ", 6764573491: "6,3 ГБ",
	} {
		if got := SizeText(n); got != want {
			t.Errorf("SizeText(%d) = %q, ждали %q", n, got, want)
		}
	}
}

// Подменённый .salbum: фото и своё ведут на .lnk, .url, desktop.ini, документ, файл без
// расширения, SVG у фото, программу у своего — в Missing, не в план; лента с .lnk — не берётся;
// своё SVG и разметка PSD — в плане; разметка не того вида — MissingGuides.
func TestCollectRejectsNonImage(t *testing.T) {
	root := testkit.TempDir(t)
	a := filepath.Join(root, "A")
	good := filepath.Join(a, "IMG.JPEG")
	p := album(t, []string{a}, good)
	var bad []string
	for _, name := range []string{"ярлык.lnk", "сайт.url", "desktop.ini", "паспорт.pdf", "без расширения", "фото.svg"} {
		f := filepath.Join(a, name)
		put(t, f, "чужое")
		p.Photos[library.IDFor(f)] = model.PhotoRef{Path: f, W: 3, H: 2}
		bad = append(bad, library.IDFor(f))
	}
	svg, exe := filepath.Join(root, "Библиотека", "s.svg"), filepath.Join(root, "Библиотека", "s.exe")
	put(t, svg, "<svg/>")
	put(t, exe, "MZ")
	p.Photos["own-svg"] = model.PhotoRef{Lib: "my:1", Path: svg}
	p.Photos["own-exe"] = model.PhotoRef{Lib: "my:2", Path: exe}
	bad = append(bad, "own-exe")
	psd, docx := filepath.Join(root, "Т", "spreads.psd"), filepath.Join(root, "Т", "cover.docx")
	put(t, psd, "8BPS")
	put(t, docx, "PK")
	p.SpreadGuide = psd
	p.Cover = &model.Cover{Guide: docx}
	lnk := filepath.Join(a, "ещё.lnk")
	put(t, lnk, "чужое")
	pl, err := Make(p, []string{lnk}, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	var srcs []string
	for _, it := range pl.Items {
		srcs = append(srcs, it.Src)
	}
	slices.Sort(srcs)
	want := []string{good, svg, psd}
	slices.Sort(want)
	slices.Sort(bad)
	if !slices.Equal(srcs, want) || !slices.Equal(pl.Missing, bad) || pl.MissingGuides != 1 {
		t.Fatalf("план %v (ждали %v), пропавшие %v (ждали %v), разметок нет %d", srcs, want, pl.Missing, bad, pl.MissingGuides)
	}
	if in := pl.Info(); in.Photos != 1 || in.Extra != 0 || in.Own != 1 || in.Guides != 1 {
		t.Fatalf("сводка: %+v", in)
	}
}

// Имена с «й» в NFC и NFD (macOS пишет NFD, APFS их не различает) — одно имя в папке сбора:
// второе — « (2)», и у файлов, и у папок альбома; иначе они столкнулись бы только при
// копировании. ФС, на которой оба файла не создать, — пропуск.
func TestCollectNFDNames(t *testing.T) {
	const nfc, nfd = "й", "й"
	root := testkit.TempDir(t)
	a, b := filepath.Join(root, "1", nfc), filepath.Join(root, "2", nfd)
	fc, fd := filepath.Join(a, nfc+".jpg"), filepath.Join(a, nfd+".jpg")
	p := album(t, []string{a, b}, fc, fd, filepath.Join(b, "x.jpg"))
	if ents, _ := os.ReadDir(a); len(ents) != 2 {
		t.Skip("ФС не различает NFC и NFD: два таких файла не создать")
	}
	pl, err := Make(p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		fd:                        "Фото/" + nfc + "/" + nfd + ".jpg",
		fc:                        "Фото/" + nfc + "/" + nfc + " (2).jpg",
		filepath.Join(b, "x.jpg"): "Фото/" + nfd + " (2)/x.jpg",
	}
	if got := rels(pl); !mapsEqual(got, want) {
		t.Fatalf("имена:\n%q\nждали\n%q", got, want)
	}
}
