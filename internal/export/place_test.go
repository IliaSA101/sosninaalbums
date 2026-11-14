package export

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Папки — строки, диска не касаемся: ссылки подставляет заглушка linked.
func placeBase(t *testing.T) string {
	t.Helper()
	return filepath.Join(testkit.TempDir(t), "альбомы")
}

// TestAlbumFolderOf — выбранная папка выгрузки против папок фото: сама папка (регистр — на Windows и
// macOS), подпапка, подпапка на «.», через ссылку — таблица; родитель, соседняя, чужая — вне.
func TestAlbumFolderOf(t *testing.T) {
	base := placeBase(t)
	fa, other := filepath.Join(base, "ФА25"), filepath.Join(base, "ФА26")
	nested := filepath.Join(fa, "вьетнам") // папка альбома внутри другой: решает ближайшая
	linked := func(d string) bool { return filepath.Base(d) == "ссылка" }
	ci := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	for _, c := range []struct {
		name    string
		dir     string
		folders []string
		root    string
		in      bool
	}{
		{"сама папка", fa, []string{fa}, fa, true},
		{"сама папка с разделителем на конце", fa + string(filepath.Separator), []string{fa}, fa, true},
		{"другой регистр", filepath.Join(base, "фа25"), []string{fa}, fa, ci},
		{"подпапка", filepath.Join(fa, "export"), []string{fa}, fa, true},
		{"подпапка глубже", filepath.Join(fa, "a", "b"), []string{fa}, fa, true},
		{"подпапка на «.»", filepath.Join(fa, ".export"), []string{fa}, "", false},
		{"подпапка под «.»", filepath.Join(fa, ".x", "y"), []string{fa}, "", false},
		{"через ссылку", filepath.Join(fa, "ссылка", "y"), []string{fa}, "", false},
		{"сама ссылка-подпапка", filepath.Join(fa, "ссылка"), []string{fa}, "", false},
		{"родитель", base, []string{fa}, "", false},
		{"соседняя", filepath.Join(base, "ФА25 — выгрузка"), []string{fa}, "", false},
		{"папка другого альбома вне", other, []string{fa}, "", false},
		{"вторая из двух папок", filepath.Join(other, "export"), []string{fa, other}, other, true},
		{"вложенная папка альбома", filepath.Join(nested, "export"), []string{fa, nested}, nested, true},
		{"нет папок (только отдельные фото, 097)", fa, nil, "", false},
		{"пустая выбранная", "", []string{fa}, "", false},
	} {
		root, in := AlbumFolderOf(c.dir, c.folders, linked)
		if in != c.in || in && !strings.EqualFold(filepath.Clean(root), filepath.Clean(c.root)) {
			t.Errorf("%s: AlbumFolderOf(%q) = %q, %v; ждали %q, %v", c.name, c.dir, root, in, c.root, c.in)
		}
	}
}

// TestNextToAlbum — соседняя «<имя> — выгрузка»: рядом с папкой фото; у корня диска и внутри другой
// папки альбома — нет; соседняя сама папка альбома — нет.
func TestNextToAlbum(t *testing.T) {
	base := placeBase(t)
	fa := filepath.Join(base, "ФА25")
	want := filepath.Join(base, "ФА25 — выгрузка")
	root := filepath.VolumeName(base) + string(filepath.Separator)
	for _, c := range []struct {
		name    string
		root    string
		folders []string
		dir     string
		ok      bool
	}{
		{"рядом", fa, []string{fa}, want, true},
		{"с разделителем на конце", fa + string(filepath.Separator), []string{fa}, want, true},
		{"корень диска", root, []string{root}, "", false},
		{"соседняя — папка альбома", fa, []string{fa, want}, "", false},
		{"соседняя внутри другой папки альбома", fa, []string{fa, base}, "", false},
		{"две папки альбома рядом", fa, []string{fa, filepath.Join(base, "ФА26")}, want, true},
	} {
		dir, ok := NextToAlbum(c.root, c.folders, nil)
		if ok != c.ok || dir != c.dir {
			t.Errorf("%s: NextToAlbum(%q) = %q, %v; ждали %q, %v", c.name, c.root, dir, ok, c.dir, c.ok)
		}
	}
}

// TestPlaceTexts — вопрос и отказ: только имена папок, без путей; «новую» — только если соседней нет.
func TestPlaceTexts(t *testing.T) {
	base := placeBase(t)
	fa, next := filepath.Join(base, "ФА25"), filepath.Join(base, "ФА25 — выгрузка")
	sub := filepath.Join(fa, "export")
	for _, c := range []struct {
		name, got, want string
	}{
		{"сама папка, новая", PlaceQuestion(fa, fa, next, false),
			"Папка «ФА25» — папка фото этого альбома: развороты смешаются с фото. Выгрузить рядом, в новую папку «ФА25 — выгрузка»?"},
		{"подпапка, новая", PlaceQuestion(sub, fa, next, false),
			"Папка «export» лежит в папке фото этого альбома «ФА25»: развороты смешаются с фото. Выгрузить рядом, в новую папку «ФА25 — выгрузка»?"},
		{"соседняя есть", PlaceQuestion(fa, fa, next, true),
			"Папка «ФА25» — папка фото этого альбома: развороты смешаются с фото. Выгрузить в папку «ФА25 — выгрузка» рядом?"},
		{"отказ", PlaceRefusal(fa, fa),
			"Папка «ФА25» — папка фото этого альбома: развороты смешаются с фото. Выберите другую папку."},
		{"отказ, подпапка", PlaceRefusal(sub, fa),
			"Папка «export» лежит в папке фото этого альбома «ФА25»: развороты смешаются с фото. Выберите другую папку."},
	} {
		if c.got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, c.got, c.want)
		}
		if strings.Contains(c.got, base) {
			t.Errorf("%s: в тексте полный путь %q", c.name, c.got)
		}
	}
	root := filepath.VolumeName(base) + string(filepath.Separator)
	if got := PlaceRefusal(root, root); !strings.Contains(got, "«"+root+"»") {
		t.Errorf("корень диска: %q — ждали сам путь вместо имени", got)
	}
}
