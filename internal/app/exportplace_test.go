package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// inAlbum переносит фото проекта окружения e в папку «ФА25» (она — папка фото альбома, p.Folders)
// и ставит её папкой выгрузки, как выбор папки фото в окне. Возвращает папку фото.
func inAlbum(t *testing.T, e *exportEnv) string {
	t.Helper()
	album := filepath.Join(testkit.TempDir(t), "ФА25")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := e.a.editor().Snapshot()
	for id, ref := range p.Photos {
		ref.Path = testkit.JPEG(t, album, id+".jpg", ref.W, ref.H)
		p.Photos[id] = ref
	}
	p.Folders = []string{album}
	e.a.ed.Store(mustEditor(t, p, ""))
	e.dir = album
	return album
}

// listing — имена в папке dir (для проверки «ничего нового»).
func listing(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range ents {
		out = append(out, f.Name())
	}
	return out
}

// exportErr — Export без ожидания записи: ok, ошибка (для отказов и «Нет»).
func (e *exportEnv) exportErr(o ExportOptions) (bool, error) {
	o.DPI = 72
	return e.a.Export(o)
}

// albumQ — начало вопроса о выгрузке в саму папку фото «ФА25».
const albumQ = "Папка «ФА25» — папка фото этого альбома: развороты смешаются с фото. "

// TestExportIntoAlbumFolder — выгрузка в папку фото альбома (121): «Да» — в соседнюю «<имя> — выгрузка»,
// «Нет» — ничего не пишется; подпапка — тот же вопрос; соседняя уже с выгрузкой — вопрос о замене;
// Фабрика — папка книги внутри соседней; на месте соседней файл или невозможная соседняя — отказ без вопроса.
func TestExportIntoAlbumFolder(t *testing.T) {
	t.Run("нет: ни файла, ни папки, окно открыто", testExportAlbumNo)
	t.Run("подпапка папки фото: тот же вопрос, соседняя рядом с папкой фото", testExportAlbumSub)
	t.Run("Фабрика: папка книги внутри соседней", testExportAlbumFabrika)
	t.Run("на месте соседней файл: отказ без вопроса", testExportAlbumFileBlocks)
	t.Run("соседней быть не может: отказ без вопроса", testExportAlbumRefuse)
	t.Run("вне папок фото: вопроса нет", testExportAlbumOutside)
	t.Run("вторая выгрузка: соседняя уже есть, вопрос без «новую»", testExportAlbumTwice)
}

// нет: ни файла, ни папки, окно открыто.
func testExportAlbumNo(t *testing.T) {
	e := newExportEnv(t, 2)
	album := inAlbum(t, e)
	before := listing(t, album)
	e.answer = false
	if ok, err := e.exportErr(ExportOptions{}); ok || err != nil {
		t.Fatalf("Export = %v, %v; ждали false без ошибки", ok, err)
	}
	if got := listing(t, album); !slices.Equal(got, before) {
		t.Fatalf("в папке фото было %v, стало %v", before, got)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(album), "ФА25 — выгрузка")); !os.IsNotExist(err) {
		t.Fatalf("соседняя папка создана без «Да»: %v", err)
	}
	if got := prefs.LoadConfig(e.a.cfgDir).LastDirs[prefs.DirExport]; got != "" {
		t.Fatalf("запомнена папка %q после «Нет»", got)
	}
	// Флаг снят: можно выбрать другую папку.
	e.answer, e.dir = true, filepath.Join(testkit.TempDir(t), "out")
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
}

// подпапка папки фото: тот же вопрос, соседняя рядом с папкой фото.
func testExportAlbumSub(t *testing.T) {
	e := newExportEnv(t, 2)
	album := inAlbum(t, e)
	e.dir = filepath.Join(album, "export")
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	want := "Папка «export» лежит в папке фото этого альбома «ФА25»: развороты смешаются с фото. Выгрузить рядом, в новую папку «ФА25 — выгрузка»?"
	if !slices.Equal(e.asked, []string{want}) {
		t.Fatalf("вопросы %q", e.asked)
	}
	e.dir = filepath.Join(filepath.Dir(album), "ФА25 — выгрузка")
	if got := e.names(t); !slices.Equal(got, []string{"01.jpg", "02.jpg"}) {
		t.Fatalf("файлы выгрузки %v", got)
	}
	if got := listing(t, filepath.Join(album, "export")); len(got) != 0 {
		t.Fatalf("в подпапке фото файлы %v", got)
	}
}

// Фабрика: папка книги внутри соседней.
func testExportAlbumFabrika(t *testing.T) {
	e, _ := printEnv(t, 2, 200, 203, "fabrika-photo")
	album := inAlbum(t, e)
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" {
		t.Fatal(msg)
	}
	if len(e.asked) != 1 || !strings.HasPrefix(e.asked[0], albumQ) {
		t.Fatalf("вопросы %q", e.asked)
	}
	book := filepath.Join(filepath.Dir(album), "ФА25 — выгрузка", "Альбом")
	e.dir = book
	if got := e.names(t); !slices.Equal(got, []string{"1-00.jpg", "1-01.jpg", "1-02.jpg"}) {
		t.Fatalf("файлы книги %v", got)
	}
}

// на месте соседней файл: отказ без вопроса.
func testExportAlbumFileBlocks(t *testing.T) {
	e := newExportEnv(t, 2)
	album := inAlbum(t, e)
	next := filepath.Join(filepath.Dir(album), "ФА25 — выгрузка")
	if err := os.WriteFile(next, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, want := paths.BookDir(filepath.Dir(album), "ФА25 — выгрузка", "выгрузки")
	ok, err := e.exportErr(ExportOptions{})
	if ok || err == nil || err.Error() != want.Error() || len(e.asked) != 0 {
		t.Fatalf("Export = %v, %v; вопросы %q; ждали отказ %v", ok, err, e.asked, want)
	}
}

// соседней быть не может: отказ без вопроса.
func testExportAlbumRefuse(t *testing.T) {
	e := newExportEnv(t, 2)
	album := inAlbum(t, e)
	p, _ := e.a.editor().Snapshot()
	p.Folders = []string{album, filepath.Dir(album)} // соседняя попала бы в папку альбома
	e.a.ed.Store(mustEditor(t, p, ""))
	ok, err := e.exportErr(ExportOptions{})
	if want := albumQ + "Выберите другую папку."; ok || err == nil || err.Error() != want || len(e.asked) != 0 {
		t.Fatalf("Export = %v, %v; вопросы %q; ждали отказ %q", ok, err, e.asked, want)
	}
	// Корень диска как папка фото: родителя нет.
	root := filepath.VolumeName(album) + string(filepath.Separator)
	p.Folders, e.dir = []string{root}, root
	e.a.ed.Store(mustEditor(t, p, ""))
	ok, err = e.exportErr(ExportOptions{})
	if ok || err == nil || !strings.Contains(err.Error(), "Выберите другую папку.") || len(e.asked) != 0 {
		t.Fatalf("корень диска: Export = %v, %v; вопросы %q", ok, err, e.asked)
	}
}

// вне папок фото: вопроса нет.
func testExportAlbumOutside(t *testing.T) {
	e := newExportEnv(t, 2)
	inAlbum(t, e)
	e.dir = filepath.Join(testkit.TempDir(t), "out")
	if msg := e.export(t, ExportOptions{}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка %q, вопросы %q", msg, e.asked)
	}
}

// вторая выгрузка в папку фото: соседняя уже есть — вопрос «рядом?» без слова «новую», затем вопрос о замене;
// выгрузка и запомненная папка — соседняя, в папку фото не пишется ничего.
func testExportAlbumTwice(t *testing.T) {
	e := newExportEnv(t, 2)
	album := inAlbum(t, e)
	before := listing(t, album)
	next := filepath.Join(filepath.Dir(album), "ФА25 — выгрузка")
	for i := range 2 {
		e.asked = nil
		if msg := e.export(t, ExportOptions{}); msg != "" {
			t.Fatalf("выгрузка %d: %s", i+1, msg)
		}
		if got := listing(t, album); !slices.Equal(got, before) {
			t.Fatalf("выгрузка %d: в папке фото было %v, стало %v", i+1, before, got)
		}
		e.a.mu.Lock()
		dir := e.a.exportDir
		e.a.mu.Unlock()
		if got := prefs.LoadConfig(e.a.cfgDir).LastDirs[prefs.DirExport]; dir != next || got != next {
			t.Fatalf("выгрузка %d: exportDir %q, запомнено %q; ждали %q", i+1, dir, got, next)
		}
	}
	// Второй проход: вопрос о соседней без «новую», потом о замене файлов.
	want := albumQ + "Выгрузить в папку «ФА25 — выгрузка» рядом?"
	if len(e.asked) != 2 || e.asked[0] != want || strings.Contains(e.asked[0], "новую") ||
		e.asked[1] != "В папке уже есть развороты прежнего экспорта: 2. Заменить?" {
		t.Fatalf("вопросы второй выгрузки %q; ждали %q и вопрос о замене", e.asked, want)
	}
}
