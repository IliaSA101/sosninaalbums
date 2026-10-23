package paths

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// diskRoot — корень диска временной папки: абсолютный путь на любой ОС (без обращения к диску).
func diskRoot() string { return filepath.VolumeName(os.TempDir()) + string(filepath.Separator) }

func at(parts ...string) string { return filepath.Join(append([]string{diskRoot()}, parts...)...) }

// under — образец library.Under (paths от library не зависит): префикс с разделителем, регистр —
// по Fold (на Linux важен).
func under(path, folder string) bool {
	return strings.HasPrefix(Fold(filepath.Clean(path)), DirKey(folder))
}

// UnderAny совпадает с library.Under по любой папке: вложенные, корень, регистр, общий префикс имени.
func TestUnderAny(t *testing.T) {
	sets := [][]string{
		nil,
		{at("Съёмки")},
		{at("Съёмки"), at("съёмки", "Ивановы"), at("a b"), at("a")},
		{at("a", "b"), at("a b", "c"), at("ab")},
		{diskRoot()},
	}
	list := []string{
		at("Съёмки", "1.jpg"), at("СЪЁМКИ", "Ивановы", "2.jpg"), at("Съёмки2", "1.jpg"), at("Съёмки"),
		at("a", "1.jpg"), at("a b", "c", "1.jpg"), at("ab", "1.jpg"), at("a", "b", "c", "1.jpg"), at("a", "bc", "1.jpg"),
		at("x.jpg"), diskRoot(),
	}
	for _, folders := range sets {
		in := UnderAny(folders)
		for _, p := range list {
			want := false
			for _, f := range folders {
				want = want || under(p, f)
			}
			if got := in(p); got != want {
				t.Errorf("папки %q, путь %q: %v, ждали %v", folders, p, got, want)
			}
		}
	}
}

// TopKeys: повторы и вложенные уходят, соседи с общим началом имени («a b», «a» с разделителем
// на конце) — разные ключи; порядок — по возрастанию.
func TestTopKeys(t *testing.T) {
	sep := string(filepath.Separator)
	got := TopKeys([]string{at("a b"), at("a") + sep, at("A"), at("a", "x"), at("ab")})
	want := []string{DirKey(at("a b")), DirKey(at("a")), DirKey(at("ab"))} // пробел меньше разделителя
	// Linux: «A» — своя папка, её ключ меньше строчных.
	if !FoldEqual("A", "a") {
		want = append([]string{DirKey(at("A"))}, want...)
	}
	if !reflect.DeepEqual(got, want) || len(got) != len(want) {
		t.Fatalf("TopKeys: %q, ждали %q", got, want)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] || strings.HasPrefix(got[i], got[i-1]) {
			t.Fatalf("не по возрастанию или вложены: %q", got)
		}
	}
	if k := TopKeys(nil); len(k) != 0 {
		t.Fatalf("пусто: %q", k)
	}
}

// Reaches — правило сканера (093-4): точка и устройство в хвосте, ссылки в папках хвоста; у файла
// последний элемент — только устройство; root внутри себя нет; регистр — по Fold (на Linux важен).
func TestReaches(t *testing.T) {
	link := at("A", "Ссылка")
	linked := func(dir string) bool { return strings.EqualFold(dir, link) }
	cases := []struct {
		root, path string
		file, want bool
	}{
		{at("A"), at("A", "b"), false, true},
		{at("a"), at("A", "B", "c"), false, FoldEqual("a", "A")},
		{at("A"), at("A", "B", "c"), false, true},
		{at("A"), at("A"), false, false},
		{at("A"), at("AB"), false, false},
		{at("A"), at("A", ".edits"), false, false},
		{at("A"), at("A", "b", ".x", "c"), false, false},
		{at("A"), at("A", ".edits", "1.jpg"), true, false},
		{at("A"), at("A", ".1.jpg"), true, true},
		{at("A"), at("A", "b", "1.jpg"), true, true},
		{at("A"), link, false, false},
		{at("A"), filepath.Join(link, "z"), false, false},
		{at("A"), filepath.Join(link, "2.jpg"), true, false},
		// Корень-ссылка не накрывает ничего: сканер из неё не берёт ни файла (0.22-sec-1).
		{link, filepath.Join(link, "2.jpg"), true, false},
		{link, filepath.Join(link, "День2"), false, false},
		{at("A"), at("A", "Ссылка 2", "3.jpg"), true, true},
		{diskRoot(), at("A", "b"), false, true},
		{diskRoot(), diskRoot(), false, false},
		{at("A"), at("B", "1.jpg"), true, false},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct {
				root, path string
				file, want bool
			}{at("A"), at("A", "aux", "b"), false, false},
			struct {
				root, path string
				file, want bool
			}{at("A"), at("A", "con"), false, false},
			struct {
				root, path string
				file, want bool
			}{at("A"), at("A", "b", "NUL"), true, false})
	}
	for _, c := range cases {
		if got := Reaches(c.root, c.path, c.file, linked); got != c.want {
			t.Errorf("Reaches(%q, %q, file=%v) = %v, ждали %v", c.root, c.path, c.file, got, c.want)
		}
	}
}

// Cover.Root — ближайшая папка списка выше пути решает; отдельный корень внутри другого сам
// становится корнем для своих файлов; ссылки спрашиваются по разу и только под папками списка.
func TestCoverRoot(t *testing.T) {
	asked := map[string]int{}
	linked := func(dir string) bool { asked[strings.ToLower(dir)]++; return false }
	c := NewCover([]string{at("A"), at("A", ".edits"), at("a"), at("C")}, linked)
	for _, tc := range []struct {
		path string
		file bool
		root string
		ok   bool
	}{
		{at("A", ".edits", "1.jpg"), true, at("A", ".edits"), true},
		{at("A", ".edits"), false, at("A"), false},
		{at("A", "b", "2.jpg"), true, at("A"), true},
		{at("A", "b", "3.jpg"), true, at("A"), true},
		{at("A"), false, "", false},
		{at("B", "4.jpg"), true, "", false},
		{at("C", ".x", "5.jpg"), true, at("C"), false},
	} {
		root, ok := c.Root(tc.path, tc.file)
		if root != tc.root || ok != tc.ok {
			t.Errorf("Root(%q) = %q %v, ждали %q %v", tc.path, root, ok, tc.root, tc.ok)
		}
	}
	if !c.HasFile(at("A", "b", "2.jpg")) || c.HasFile(at("B", "4.jpg")) {
		t.Fatal("HasFile не совпал с Root")
	}
	if n := asked[strings.ToLower(at("A", "b"))]; n != 1 {
		t.Fatalf("ссылка A\\b спрошена %d раз, ждали 1 (%v)", n, asked)
	}
	if asked[strings.ToLower(at("B"))] != 0 {
		t.Fatalf("путь вне списка — ссылки не спрашиваются: %v", asked)
	}
	if _, ok := NewCover(nil, linked).Root(at("A", "1.jpg"), true); ok {
		t.Fatal("пустой список")
	}
}

// Linked на диске: обычная папка и пропавший путь — нет; ссылка или junction — да (Windows без
// прав на ссылку — junction, его создаёт любой пользователь; не вышло — Skip).
func TestLinked(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "real")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if Linked(plain) || Linked(filepath.Join(dir, "нет")) {
		t.Fatal("обычная или пропавшая папка — не ссылка")
	}
	link := testkit.DirLink(t, plain, filepath.Join(dir, "link"))
	if !Linked(link) {
		t.Fatal("ссылка не узнана")
	}
	if Reaches(dir, filepath.Join(link, "1.jpg"), true, Linked) || !Reaches(dir, filepath.Join(plain, "1.jpg"), true, Linked) {
		t.Fatal("Reaches сквозь ссылку на диске")
	}
	// Корень-ссылка (и с разделителем на конце) не накрывает ни файла, ни подпапки; без проверки
	// ссылок (nil) — прежнее правило по строкам.
	for _, root := range []string{link, link + string(filepath.Separator)} {
		if Reaches(root, filepath.Join(link, "1.jpg"), true, Linked) || Reaches(root, filepath.Join(link, "День2"), false, Linked) {
			t.Fatalf("корень-ссылка %q накрыла запись", root)
		}
	}
	if !Reaches(link, filepath.Join(link, "1.jpg"), true, nil) {
		t.Fatal("без проверки ссылок корень-ссылка — обычная папка")
	}
	c := NewCover([]string{link, filepath.Join(link, "День2"), dir}, Linked)
	if r, ok := c.Root(filepath.Join(link, "День2"), false); ok || r != link {
		t.Fatalf("подпапка корня-ссылки: %q %v, ждали отдельный корень под %q", r, ok, link)
	}
	if c.HasFile(filepath.Join(link, "1.jpg")) || !c.HasFile(filepath.Join(link, "День2", "1.jpg")) {
		t.Fatal("файл корня-ссылки накрыт или файл её подпапки-корня — нет")
	}
}

// Linked, когда узнать нельзя (0.22-sec-1): отказ доступа и «нет такого» на отключённом томе —
// как ссылка (запись не сворачивается); «нет такого» на живом томе — не ссылка.
func TestLinkedUnknown(t *testing.T) {
	dir := at("Свадьба", "Ссылка")
	vol := filepath.VolumeName(dir) + string(filepath.Separator)
	prev := lstat
	t.Cleanup(func() { lstat = prev })
	for _, c := range []struct {
		name         string
		dirErr, vErr error
		want         bool
	}{
		{"отказ доступа", fs.ErrPermission, nil, true},
		{"нет на живом томе", fs.ErrNotExist, nil, false},
		{"тома нет", fs.ErrNotExist, fs.ErrNotExist, true},
		{"том не отвечает", fs.ErrNotExist, fs.ErrPermission, true},
	} {
		lstat = func(p string) (fs.FileInfo, error) {
			if p != vol {
				return nil, c.dirErr
			}
			if c.vErr != nil {
				return nil, c.vErr
			}
			return prev(diskRoot())
		}
		if got := Linked(dir); got != c.want {
			t.Errorf("%s: Linked = %v, ждали %v", c.name, got, c.want)
		}
	}
	lstat = prev
	if Linked(at("нет-такой-папки-sec022")) {
		t.Error("нет такого на живом томе — не ссылка")
	}
	// Настоящий отключённый том (Windows): свободная буква диска; Reaches в корень на нём не сворачивает.
	gone := filepath.Join(testkit.MissingDrive(t), "Свадьба")
	if !Linked(filepath.Join(gone, "Ссылка")) || Reaches(gone, filepath.Join(gone, "Ссылка"), false, Linked) {
		t.Errorf("тома %s нет — запись свёрнута", gone)
	}
}
