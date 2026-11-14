package library

import (
	"context"
	"image/color"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// oursScan — результат одного Scan: лента, файлы выгрузки из финального отчёта, все промежуточные списки.
type oursScan struct {
	names []string
	ours  []string
	lists [][]Photo
}

func scanOurs(t *testing.T, s *Scanner, src Sources) oursScan {
	t.Helper()
	var r oursScan
	photos, _, err := s.Scan(context.Background(), src, func(ph []Photo, pr ScanProgress, final bool) {
		if ph != nil {
			r.lists = append(r.lists, ph)
		}
		if final {
			r.ours = pr.Ours
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range photos {
		r.names = append(r.names, p.Name)
	}
	return r
}

// relNames — пути относительно root, через «/»: проверка не зависит от системы.
func relNames(root string, ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		rel, _ := filepath.Rel(root, p)
		out[i] = filepath.ToSlash(rel)
	}
	return out
}

// Файлы выгрузки (метка в заголовке) из обхода папки в ленту не идут, имя не важно (121-3): свой 01.jpg
// фотографа остаётся. Отдельный файл (097) с меткой остаётся в ленте. Повторное чтение из памяти даёт
// тот же итог без чтения файлов.
func TestScanSkipsExports(t *testing.T) {
	root := testkit.TempDir(t)
	album := filepath.Join(root, "ФА25")
	outside := filepath.Join(root, "Прошлый")
	testkit.JPEG(t, album, "IMG_1.jpg", 60, 40)
	testkit.JPEG(t, album, "01.jpg", 60, 40) // фото фотографа с именем выгрузки
	testkit.ExportJPEG(t, album, "00.jpg", 60, 40)
	testkit.ExportJPEG(t, album, "export/01.jpg", 60, 40)
	testkit.ExportJPEG(t, album, "export/1-02.jpg", 60, 40)
	prev := testkit.ExportJPEG(t, outside, "05.jpg", 60, 40)
	testkit.PNG(t, album, "метка.png", 20, 20, color.NRGBA{R: 255, G: 255, B: 255, A: 255}) // PNG метки не несёт

	cases := []struct {
		name      string
		src       Sources
		wantNames []string
		wantOurs  []string
	}{
		{"только папка", Sources{Folders: []string{album}},
			[]string{"01.jpg", "IMG_1.jpg", "метка.png"}, []string{"ФА25/00.jpg", "ФА25/export/01.jpg", "ФА25/export/1-02.jpg"}},
		{"подпапка выгрузок отдельным источником", Sources{Folders: []string{filepath.Join(album, "export")}},
			nil, []string{"ФА25/export/01.jpg", "ФА25/export/1-02.jpg"}},
		{"папка и отдельный файл с меткой", Sources{Folders: []string{album}, Files: []string{prev}},
			[]string{"05.jpg", "01.jpg", "IMG_1.jpg", "метка.png"}, []string{"ФА25/00.jpg", "ФА25/export/01.jpg", "ФА25/export/1-02.jpg"}},
		{"только отдельный файл с меткой", Sources{Files: []string{prev}}, []string{"05.jpg"}, nil},
		{"отдельный файл внутри папки: папка первее", Sources{Folders: []string{album}, Files: []string{filepath.Join(album, "00.jpg")}},
			[]string{"01.jpg", "IMG_1.jpg", "метка.png"}, []string{"ФА25/00.jpg", "ФА25/export/01.jpg", "ФА25/export/1-02.jpg"}},
		{"пусто", Sources{}, nil, nil},
	}
	for _, c := range cases {
		s := NewScanner()
		var reads atomic.Int32
		s.beforeRead = func(string) { reads.Add(1) }
		first := scanOurs(t, s, c.src)
		reads1 := reads.Load()
		second := scanOurs(t, s, c.src) // из памяти
		for i, r := range []oursScan{first, second} {
			if got := sortedCopy(relNames(root, r.ours)); !slices.Equal(got, sortedCopy(c.wantOurs)) {
				t.Errorf("%s, проход %d: выгрузка %v, ждали %v", c.name, i+1, got, c.wantOurs)
			}
			if !slices.Equal(sortedCopy(r.names), sortedCopy(c.wantNames)) {
				t.Errorf("%s, проход %d: лента %v, ждали %v", c.name, i+1, r.names, c.wantNames)
			}
			for _, l := range r.lists { // ни один промежуточный список не мигает файлами выгрузки
				for _, p := range l {
					if strings.Contains(filepath.ToSlash(p.Path), "/export/") || p.Name == "00.jpg" {
						t.Errorf("%s, проход %d: в промежуточном списке %s", c.name, i+1, p.Path)
					}
				}
			}
		}
		if reads.Load() != reads1 {
			t.Errorf("%s: второй проход читал файлы (%d → %d)", c.name, reads1, reads.Load())
		}
	}
}

func sortedCopy(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

// Файл с меткой, прочитанный при обходе папки, из памяти идёт в ленту, когда его выбрали явно (097),
// и наоборот: признак хранится в Photo, а не в состоянии прошлого прохода.
func TestScanOursMemoAcrossSources(t *testing.T) {
	root := testkit.TempDir(t)
	p := testkit.ExportJPEG(t, root, "ФА25/01.jpg", 60, 40)
	s := NewScanner()
	var reads atomic.Int32
	s.beforeRead = func(string) { reads.Add(1) }
	steps := []struct {
		src      Sources
		inFeed   bool
		wantOurs int
	}{
		{Sources{Folders: []string{filepath.Dir(p)}}, false, 1},
		{Sources{Files: []string{p}}, true, 0},
		{Sources{Folders: []string{filepath.Dir(p)}}, false, 1},
	}
	for i, st := range steps {
		r := scanOurs(t, s, st.src)
		if (len(r.names) == 1) != st.inFeed || len(r.ours) != st.wantOurs {
			t.Errorf("шаг %d: лента %v, выгрузка %v", i+1, r.names, r.ours)
		}
	}
	if reads.Load() != 1 {
		t.Errorf("файл читался %d раз, ждали один", reads.Load())
	}
}
