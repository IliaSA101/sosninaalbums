package libpacks

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Кэш листов образцов (спека 144 Г2, security 2, 6, 10): тип файла, размер, sha256, подмена,
// папка, уборка.

// put кладёт лист в папку образцов под его именем.
func (fx *prevFx) put(p *fetch.Preview, data []byte) string {
	fx.t.Helper()
	dir := filepath.Join(fx.dir, previewDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fx.t.Fatal(err)
	}
	path := filepath.Join(dir, p.SHA256+previewExt)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fx.t.Fatal(err)
	}
	return path
}

// Круг: лист в кэше читается байт в байт; пустой Size и больше предела — «листа нет» без чтения.
func TestPreviewCacheRoundTrip(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	fx.put(p, data)
	got, err := fx.s.readPreview(p)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("круг: %v", err)
	}
	for _, size := range []int64{0, -1, fetch.MaxPreview + 1} {
		q := *p
		q.Size = size
		if _, err := fx.s.readPreview(&q); err != errNoSheet {
			t.Errorf("size %d: %v", size, err)
		}
	}
	q := *p
	q.SHA256 = strings.ToUpper(p.SHA256) // не наше имя: только строчные hex
	if _, err := fx.s.readPreview(&q); err != errNoSheet {
		t.Errorf("верхний регистр: %v", err)
	}
	if _, err := fx.s.readPreview(nil); err != errNoSheet {
		t.Errorf("nil: %v", err)
	}
}

// Байт изменён при том же размере: sha не сходится, файл удаляется, листа нет.
func TestPreviewTampered(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	bad := bytes.Clone(data)
	bad[len(bad)/2] ^= 1
	path := fx.put(p, bad)
	if _, err := fx.s.readPreview(p); err != errNoSheet {
		t.Fatalf("подмена: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Error("подменённый файл остался")
	}
}

// Размер файла не равен заявленному (короче и длиннее) — листа нет.
func TestPreviewWrongSize(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	for _, content := range [][]byte{data[:len(data)-1], append(bytes.Clone(data), 0), nil} {
		fx.put(p, content)
		if _, err := fx.s.readPreview(p); err != errNoSheet || fx.s.sheetPresent(p) {
			t.Errorf("размер %d: %v", len(content), err)
		}
	}
}

// Ссылка (хоть на годный файл) и папка с именем листа — листа нет; цель ссылки цела.
func TestPreviewNotRegular(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	outside := filepath.Join(testkit.TempDir(t), "real.jpg")
	if err := os.WriteFile(outside, data, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(fx.dir, previewDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, p.SHA256+previewExt)
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	if _, err := fx.s.readPreview(p); err != errNoSheet || fx.s.sheetPresent(p) {
		t.Errorf("ссылка: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Error("цель ссылки тронута")
	}
	_ = os.Remove(path)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.s.readPreview(p); err != errNoSheet || fx.s.sheetPresent(p) {
		t.Errorf("папка: %v", err)
	}
}

// Файл подменили между Lstat и открытием (тот же размер и тот же sha — новый файл): открытый не
// тот, что проверенный, листа нет.
func TestPreviewSwapAfterLstat(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	path := fx.put(p, data)
	fx.s.afterLstat = func() {
		other := path + ".new"
		if err := os.WriteFile(other, data, 0o600); err != nil {
			t.Error(err)
		}
		if err := os.Rename(other, path); err != nil {
			t.Error(err)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("Rename поверх файла на Windows — другая семантика")
	}
	if _, err := fx.s.readPreview(p); err != errNoSheet {
		t.Fatalf("подмена после Lstat: %v", err)
	}
}

// Папка листов — ссылка: образцы выключены на сеанс, запросов нет, цель ссылки пуста.
func TestPreviewDirSymlink(t *testing.T) {
	fx := newPrevFx(t, nil)
	rows, _ := rowsFx(fx)
	fx.list(rows...)
	target := testkit.TempDir(t)
	if err := os.Symlink(target, filepath.Join(fx.dir, previewDirName)); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	buf := logTo(t)
	if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 0 {
		t.Fatalf("задание: %d, запросов %d", got, fx.totalHits())
	}
	if left, _ := os.ReadDir(target); len(left) != 0 {
		t.Errorf("в цели ссылки появилось %d файлов", len(left))
	}
	if fx.s.previewDir() != "" || !strings.Contains(buf.String(), "образцы выключены") {
		t.Errorf("не выключено: %q", buf.String())
	}
	if _, err := fx.s.Cell(t.Context(), "pack-a", "bg", 0); err != ErrNoCell {
		t.Errorf("клетка: %v", err)
	}
}

// Нет папки настроек (пустая или относительная): ни запроса, ни файла в текущей папке.
func TestPreviewNoCfgDir(t *testing.T) {
	for _, cfg := range []string{"", "relative-cfg"} {
		cwd := testkit.TempDir(t)
		t.Chdir(cwd)
		fx := newPrevFx(t, func(h *Host) { h.CfgDir = func() string { return cfg } })
		rows, _ := rowsFx(fx)
		fx.list(rows...)
		if cfg != "" { // папка есть до записи согласия: иначе тест зелёный оттого, что согласие не записалось
			if err := os.MkdirAll(cfg, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		prefs.UpdateLibraryNet(cfg, func(n *prefs.LibraryNet) { n.Allowed = true })
		t.Cleanup(func() { prefs.UpdateLibraryNet(cfg, func(n *prefs.LibraryNet) { n.Allowed = false }) })
		if cfg != "" && !prefs.LibraryNetOf(cfg).Allowed {
			t.Fatalf("%q: согласие не записалось, проверка пуста", cfg)
		}
		if got := fx.s.FetchPreviews(); got != 0 || fx.totalHits() != 0 {
			t.Fatalf("%q: %d, запросов %d", cfg, got, fx.totalHits())
		}
		left, _ := os.ReadDir(cwd)
		for _, e := range left {
			if e.Name() != cfg { // только сама папка настроек (с config.json), без library-previews
				t.Errorf("%q: в текущей папке %s", cfg, e.Name())
			}
		}
		if cfg != "" {
			inner, _ := os.ReadDir(filepath.Join(cwd, cfg))
			for _, e := range inner {
				if e.Name() == previewDirName {
					t.Errorf("%q: появилась %s", cfg, previewDirName)
				}
			}
		}
		if fx.s.previewDir() != "" || fx.s.sheetPath(rows[0].Preview) != "" {
			t.Errorf("%q: папка не выключена", cfg)
		}
	}
}

// Уборка: остаются листы действующего списка; чужие имена, верхний регистр, ссылка и папка
// с подходящим именем не тронуты, цель ссылки цела.
func TestPreviewGC(t *testing.T) {
	fx := newPrevFx(t, nil)
	keep := previewOf("", sheetJPEG(t, 1))
	old := previewOf("", sheetJPEG(t, 2))
	fx.put(keep, sheetJPEG(t, 1))
	fx.put(old, sheetJPEG(t, 2))
	dir := filepath.Join(fx.dir, previewDirName)
	outside := filepath.Join(testkit.TempDir(t), "outside.jpg")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	name := func(c string) string { return strings.Repeat(c, 64) }
	files := []string{name("c") + ".JPG", strings.ToUpper(name("d")) + ".jpg", "notes.txt", name("e") + ".jpg.bak"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, name("f")+".jpg"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := name("1") + ".jpg"
	linked := os.Symlink(outside, filepath.Join(dir, link)) == nil
	fx.s.gcPreviews(fetch.Catalog{Packs: []fetch.Entry{{ID: "pack-a", Preview: keep}}})
	left := map[string]bool{}
	for _, n := range fx.folder() {
		left[n] = true
	}
	if !left[keep.SHA256+".jpg"] || left[old.SHA256+".jpg"] {
		t.Errorf("действующий/устаревший: %v", left)
	}
	for _, f := range append(files, name("f")+".jpg") {
		if !left[f] {
			t.Errorf("тронут чужой %s", f)
		}
	}
	if linked {
		if !left[link] {
			t.Error("ссылка удалена")
		}
		if _, err := os.Stat(outside); err != nil {
			t.Error("цель ссылки удалена")
		}
	}
}

// Задание держит prevMu: уборка пропущена, .dl-*.tmp идущей загрузки жив.
func TestPreviewGCSkipsRunningJob(t *testing.T) {
	fx := newPrevFx(t, nil)
	old := previewOf("", sheetJPEG(t, 2))
	path := fx.put(old, sheetJPEG(t, 2))
	tmp := filepath.Join(fx.dir, previewDirName, ".dl-123.tmp")
	if err := os.WriteFile(tmp, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.s.prevMu.Lock()
	fx.s.gcPreviews(fetch.Catalog{})
	fx.s.prevMu.Unlock()
	for _, p := range []string{path, tmp} {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("тронут %s", filepath.Base(p))
		}
	}
	fx.s.gcPreviews(fetch.Catalog{}) // свободно: устаревший убран, .dl-*.tmp уборка не трогает
	if _, err := os.Lstat(path); err == nil {
		t.Error("устаревший лист остался")
	}
	if _, err := os.Lstat(tmp); err != nil {
		t.Error("уборка тронула .dl-*.tmp")
	}
}

// remember с меньшим seq отвергается: уборка (Fresh) идёт по действующему, не по отвергнутому.
func TestPreviewGCOlderSeqKeeps(t *testing.T) {
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	path := fx.put(p, data)
	fx.s.remember(fetch.Catalog{Seq: 5, Packs: []fetch.Entry{{ID: "pack-a", Preview: p}}})
	eff := fx.s.remember(fetch.Catalog{Seq: 4, Packs: []fetch.Entry{{ID: "pack-a"}}}) // без превью, но старше
	fx.s.gcPreviews(eff)
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("лист действующего списка убран по отвергнутому")
	}
	fx.s.gcPreviews(fx.s.remember(fetch.Catalog{Seq: 6, Packs: []fetch.Entry{{ID: "pack-a"}}}))
	if _, err := os.Lstat(path); err == nil {
		t.Error("лист нового списка без превью остался")
	}
}
