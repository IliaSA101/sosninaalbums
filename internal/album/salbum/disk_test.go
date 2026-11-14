package salbum

import (
	"encoding/json"
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

// noGlobalDisk ставит на все глобальные швы разбора t.Fatal: любое обращение к диску или искателю
// библиотеки из «сухого» разбора — провал (160, У100).
func noGlobalDisk(t *testing.T) {
	t.Helper()
	oldR, oldS, oldI, oldL := reachable, statLocalFn, readInfoFn, linkedFn
	t.Cleanup(func() { reachable, statLocalFn, readInfoFn, linkedFn = oldR, oldS, oldI, oldL; SetLibFinder(nil) })
	reachable = func(p string) bool { t.Fatalf("reachable(%q)", p); return false }
	statLocalFn = func(p string) (fs.FileInfo, error) { t.Fatalf("StatLocal(%q)", p); return nil, nil }
	readInfoFn = func(p string) (library.Photo, error) { t.Fatalf("ReadInfo(%q)", p); return library.Photo{}, nil }
	linkedFn = func(p string) bool { t.Fatalf("Linked(%q)", p); return false }
	SetLibFinder(func(string) (string, bool) { t.Fatal("искатель библиотеки"); return "", false })
}

// diskFile — файл альбома с папками (путь + запасной), отдельными фото и своим фоном.
func diskFile(t *testing.T, folders []FileFolder, loose []FileFolder, photos map[string]FilePhoto, more ...func(*FileProject)) []byte {
	t.Helper()
	list := []FileLoose{}
	if len(loose) > 0 {
		list = append(list, FileLoose{ID: "0123456789ab", Files: loose})
	}
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Spreads: []*model.Spread{},
		Folders: folders, Photos: photos, Loose: list}
	for _, m := range more {
		m(&f)
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDecodeDryNoDisk: сухой разбор не обращается ни к reachable, ни к Stat, ни к Lstat, ни к искателю
// библиотеки — даже с папками, запасными путями, отдельными фото и своим фоном (У100).
func TestDecodeDryNoDisk(t *testing.T) {
	root := testkit.TempDir(t)
	a, b := filepath.Join(root, "Весна"), filepath.Join(root, "Лето")
	data := diskFile(t,
		[]FileFolder{{Path: a, Abs: b}, {Path: filepath.Join(a, "Вложенная")}},
		[]FileFolder{{Path: filepath.Join(root, "Своё", "1.jpg"), Abs: filepath.Join(b, "1.jpg")}},
		map[string]FilePhoto{
			"0123456789ab": {PhotoRef: model.PhotoRef{Lib: "my:0123456789abcdef", W: 10, H: 10}},
			// Фото с основным и запасным путём и без размера в файле (Size=0): fileSize и fixDims тоже идут через Disk.
			"aaaaaaaaaaaa": {PhotoRef: model.PhotoRef{Path: filepath.Join(a, "1.jpg"), W: 0, H: 0}, Abs: filepath.Join(b, "1.jpg")},
		},
		func(f *FileProject) {
			f.Marks = markList{{Path: filepath.Join(a, "m.jpg"), Abs: filepath.Join(b, "m.jpg")}} // hasDir → isDir
			f.SpreadGuide, f.SpreadGuideAbs = filepath.Join(a, "g.psd"), filepath.Join(b, "g.psd")
			f.Cover = &FileCover{Cover: model.Cover{Guide: filepath.Join(a, "c.psd")}, GuideAbs: filepath.Join(b, "c.psd")}
		})
	noGlobalDisk(t)
	p, err := DecodeDry(data, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Folders) == 0 || p.FolderAlt[p.Folders[0]] != b {
		t.Errorf("запасная папка потеряна: %v %v", p.Folders, p.FolderAlt)
	}
	if got := p.LooseFiles(); len(got) != 1 {
		t.Errorf("отдельные фото: %v", got)
	}
	if p.SpreadGuide == "" || p.SpreadGuideAlt == "" || p.Cover == nil || p.Cover.GuideAlt == "" {
		t.Errorf("разметки: основной и запасной пути не остались: %q %q %+v", p.SpreadGuide, p.SpreadGuideAlt, p.Cover)
	}
	// И пустой, битый, усечённый вход — ошибка без паники и без диска.
	for _, bad := range [][]byte{nil, []byte("{"), data[:len(data)/2], []byte(`{"version":99999}`)} {
		if _, err := DecodeDry(bad, root); err == nil {
			t.Errorf("вход %q разобран", bad)
		}
	}
}

// TestDecodeDrySameModel: когда основные пути на месте, сухой и полный разбор дают одни и те же пути.
func TestDecodeDrySameModel(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Весна")
	img := testkit.JPEG(t, dir, "1.jpg", 40, 30)
	data := diskFile(t, []FileFolder{{Path: dir}}, []FileFolder{{Path: img}}, map[string]FilePhoto{})
	dry, err := DecodeDry(data, root)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Decode(data, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dry.Folders, full.Folders) || !slices.Equal(dry.LooseFiles(), full.LooseFiles()) {
		t.Errorf("пути разошлись: %v %v / %v %v", dry.Folders, dry.LooseFiles(), full.Folders, full.LooseFiles())
	}
}

// TestDecodeBoundedOutside: запасной путь вне проверенных не выбирается и диск по нему не трогается;
// внутри — выбирается, как в обычном Decode.
func TestDecodeBoundedOutside(t *testing.T) {
	root := testkit.TempDir(t)
	gone, alt := filepath.Join(root, "Нет"), filepath.Join(root, "Запас")
	img := testkit.JPEG(t, alt, "1.jpg", 40, 30)
	data := diskFile(t, []FileFolder{{Path: gone, Abs: alt}}, []FileFolder{{Path: filepath.Join(root, "Отдельно", "1.jpg"), Abs: img}}, map[string]FilePhoto{})

	var touched []string
	inside := func(path string) bool { return !strings.HasPrefix(path, alt) }
	d := Spy(Bounded(inside, nil), func(op, path string) {
		if !inside(path) {
			touched = append(touched, op+" "+path)
		}
	})
	p, err := DecodeDisk(data, root, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Folders) != 1 || p.Folders[0] != gone || p.FolderAlt[gone] != alt {
		t.Errorf("запасная папка выбрана вне проверенных: %v %v", p.Folders, p.FolderAlt)
	}
	if lf := p.LooseFiles(); len(lf) != 1 || strings.HasPrefix(lf[0], alt) {
		t.Errorf("отдельное фото вне проверенных: %v", lf)
	}
	// Спрашивали о запасном пути (Spy видит вопрос), но диск ответил «нет» сам (Bounded): это и есть граница.
	if len(touched) == 0 {
		t.Error("шпион не видел обращений вне проверенных — проверка бессмысленна")
	}
	full, err := DecodeDisk(data, root, Bounded(func(string) bool { return true }, nil))
	if err != nil {
		t.Fatal(err)
	}
	if full.Folders[0] != alt {
		t.Errorf("с разрешением запасная папка не выбрана: %v", full.Folders)
	}
}

// TestEachPathCoversCheck: EachPath отдаёт каждую пару путей модели (папки, FolderAlt, отдельные фото, фото,
// отметки, разметки), а Check падает на «what» первой негодной.
func TestEachPathCoversCheck(t *testing.T) {
	root := testkit.TempDir(t)
	p := model.New()
	p.Folders = []string{filepath.Join(root, "A")}
	p.FolderAlt = map[string]string{filepath.Join(root, "B"): filepath.Join(root, "B2")}
	p.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: filepath.Join(root, "l.jpg"), Alt: filepath.Join(root, "l2.jpg")}}}}
	p.Photos["a"] = model.PhotoRef{Path: filepath.Join(root, "p.jpg"), Alt: filepath.Join(root, "p2.jpg")}
	p.Photos["lib"] = model.PhotoRef{Lib: "my:0123456789abcdef"} // своё без пути — не строка
	p.Cover = &model.Cover{Guide: filepath.Join(root, "g.psd"), GuideAlt: filepath.Join(root, "g2.psd")}
	p.SpreadGuide = filepath.Join(root, "s.png")
	p.SetLoaded(model.Side{Marks: map[string]model.Mark{"m": {Path: filepath.Join(root, "m.jpg")}}})
	var got []string
	if err := EachPath(p, func(what, path, alt string) error {
		got = append(got, what+"|"+filepath.Base(path)+"|"+filepath.Base(alt))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{"папка|A|.", "папка|B|B2", "отдельное фото|l.jpg|l2.jpg", "фото|p.jpg|p2.jpg", "отметка|m.jpg|.",
		"разметка обложки|g.psd|g2.psd", "разметка разворотов|s.png|."}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("EachPath:\n got %q\nwant %q", got, want)
	}
	if err := Check(p); err != nil {
		t.Fatalf("Check годной модели: %v", err)
	}
	for _, c := range []struct {
		name, what string
		set        func(p *model.Project)
	}{
		{"папка", "папка", func(p *model.Project) { p.Folders[0] = "относительная" }},
		{"запас", "папка", func(p *model.Project) { p.FolderAlt[filepath.Join(root, "B")] = "x" }},
		{"отдельное", "отдельное фото", func(p *model.Project) { p.Loose[0].Files[0].Alt = "x" }},
		{"фото", "фото", func(p *model.Project) { r := p.Photos["a"]; r.Path = "x"; p.Photos["a"] = r }},
		{"обложка", "разметка обложки", func(p *model.Project) { p.Cover.Guide = "x" }},
		{"развороты", "разметка разворотов", func(p *model.Project) { p.SpreadGuide = "x" }},
	} {
		q := p.Clone()
		c.set(q)
		if err := Check(q); err == nil || !strings.Contains(err.Error(), c.what) {
			t.Errorf("%s: Check = %v, ждали «%s»", c.name, err, c.what)
		}
	}
}

// TestBoundedSymlinkFile: ссылка-файл внутри разрешённой папки — «файла нет»: ни Stat по цели, ни чтение
// заголовка (аудит В1, В2); срок и счётчик обращений Budget обрывают разбор (В3).
func TestBoundedSymlinkFile(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Весна")
	outside := testkit.JPEG(t, filepath.Join(root, "Вне"), "r.jpg", 40, 30)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ln := filepath.Join(dir, "x.jpg")
	if os.Symlink(outside, ln) != nil {
		t.Skip("ссылку создать нельзя")
	}
	oldInfo := readInfoFn
	t.Cleanup(func() { readInfoFn = oldInfo })
	readInfoFn = func(p string) (library.Photo, error) {
		t.Fatalf("ReadInfo(%q) по ссылке", p)
		return library.Photo{}, nil
	}
	all := func(string) bool { return true }
	d := Bounded(all, nil)
	if _, ok := fileSizeOn(d, ln); ok {
		t.Error("ссылка-файл найдена")
	}
	if _, _, ok := probeDimsOn(d, ln); ok {
		t.Error("размеры прочитаны по ссылке")
	}
	// Бюджет: по числу обращений и по сроку.
	b := NewBudget(time.Time{}, 2)
	d = Bounded(all, b)
	for range 5 {
		d.reachable(dir)
	}
	if !b.Exceeded() {
		t.Error("число обращений не ограничено")
	}
	b = NewBudget(time.Now().Add(-time.Second), 0)
	if Bounded(all, b).reachable(dir) || !b.Exceeded() {
		t.Error("просроченный бюджет ответил «да»")
	}
}
