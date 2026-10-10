package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// putFile создаёт файл из n байт (с папками).
func putFile(t testing.TB, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

// golden — testdata/v1.salbum.json (файл v1; .salbum в git не пускает хук) с подставленной папкой (пути v1 абсолютные).
func golden(t testing.TB, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "v1.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	q, _ := json.Marshal(filepath.ToSlash(dir))
	return bytes.ReplaceAll(data, []byte("{{DIR}}"), bytes.Trim(q, `"`))
}

// fileJSON — сохранённый файл как дерево JSON для проверок.
func fileJSON(t *testing.T, path string) FileProject {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// onePhoto — проект с одним фото по пути path в одном развороте.
func onePhoto(path string) (*model.Project, string) {
	p := model.New()
	id := library.IDFor(path)
	p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{id}}}
	return p, id
}

func TestLoadV1Migrates(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "old.salbum")
	if err := os.WriteFile(path, golden(t, dir), 0o644); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash(dir)
	putFile(t, filepath.Join(dir, "Отбор", "a.jpg"), 1234)
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || p.From() != 1 || !slices.Equal(p.Folders, []string{slash + "/Отбор"}) {
		t.Fatalf("версия %d, from %d, папки %v", p.Version, p.From(), p.Folders)
	}
	ida, idd := library.IDFor(slash+"/Отбор/a.jpg"), library.IDFor(slash+"/Архив/d.jpg")
	if len(p.Spreads) != 2 || len(p.Spreads[0].Photos) != 3 || p.Spreads[0].Photos[0] != ida || p.Spreads[1].Photos[0] != idd {
		t.Fatalf("ID фото не пересчитаны по путям: %+v", p.Spreads)
	}
	if r := p.Photos[ida]; r.Size != 1234 || r.W != 300 || r.Alt != "" {
		t.Fatalf("фото a: %+v", r)
	}
	if r := p.Photos[idd]; r.Size != 0 || r.Path != slash+"/Архив/d.jpg" {
		t.Fatalf("ненайденное фото: %+v", r)
	}
	want := layout.Layouts(p.Aspects(p.Spreads[0].Photos), p.Params(), model.VariantLimit)[3].Key
	if p.Spreads[0].Layout != want || p.Spreads[0].Variant != 0 || p.Spreads[1].Layout != "B[full:0]" {
		t.Fatalf("ключи %q %q, ждали %q", p.Spreads[0].Layout, p.Spreads[1].Layout, want)
	}
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	f := fileJSON(t, path)
	if f.Version != model.FormatVersion || f.Folders[0].Path != "Отбор" || f.Photos[ida].Path != "Отбор/a.jpg" || f.Photos[idd].Path != "Архив/d.jpg" {
		t.Fatalf("после сохранения: %+v", f)
	}
	if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(`"variant"`)) || bytes.Contains(data, []byte(`"alt"`)) {
		t.Fatalf("лишние поля:\n%s", data)
	}
}

func TestSaveRelativeInsideDir(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "Отбор", "sub", "a.jpg")
	p, id := onePhoto(ph)
	p.Folders = []string{dir, filepath.Join(dir, "Отбор")}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	f := fileJSON(t, path)
	if got := f.Photos[id]; got.Path != "Отбор/sub/a.jpg" || got.Abs != ph {
		t.Fatalf("фото: %+v", got)
	}
	if f.Folders[0] != (FileFolder{".", dir}) || f.Folders[1] != (FileFolder{"Отбор", filepath.Join(dir, "Отбор")}) {
		t.Fatalf("папки: %+v", f.Folders)
	}
}

func TestSaveAbsoluteOutside(t *testing.T) {
	root := testkit.TempDir(t)
	ph := filepath.Join(root, "x", "a.jpg")
	p, id := onePhoto(ph)
	path := filepath.Join(root, "a", "b", "c", "a.salbum") // до фото три уровня вверх
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, path).Photos[id]; got.Path != ph || got.Abs != "" {
		t.Fatalf("фото: %+v", got)
	}
	if runtime.GOOS == "windows" {
		if r, ok := RelPath(`C:\Альбом`, `D:\Фото\a.jpg`); ok {
			t.Fatalf("другой диск дал относительный путь %q", r)
		}
	}
}

func TestSaveRelativeParentLimit(t *testing.T) {
	root := testkit.TempDir(t)
	ph := filepath.Join(root, "x", "a.jpg")
	p, id := onePhoto(ph)
	path := filepath.Join(root, "a", "b", "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, path).Photos[id]; got.Path != "../../x/a.jpg" || got.Abs != ph {
		t.Fatalf("два уровня вверх: %+v", got)
	}
}

func TestLoadRelativeMovedFolder(t *testing.T) {
	old, moved := filepath.Join(testkit.TempDir(t), "Ивановы"), filepath.Join(testkit.TempDir(t), "Флешка", "Ивановы")
	ph := filepath.Join(old, "Отбор", "a.jpg")
	putFile(t, ph, 7)
	p, oldID := onePhoto(ph)
	p.Folders = []string{filepath.Join(old, "Отбор")}
	if err := Save(p, filepath.Join(old, "Альбом", "a.salbum")); err != nil {
		t.Fatal(err)
	}
	// Копия на «флешку», исходник пропал.
	if err := os.CopyFS(moved, os.DirFS(old)); err != nil {
		t.Fatal(err)
	}
	testkit.RemoveAll(t, old)
	q, err := Load(filepath.Join(moved, "Альбом", "a.salbum"))
	if err != nil {
		t.Fatal(err)
	}
	nph := filepath.Join(moved, "Отбор", "a.jpg")
	nid := library.IDFor(nph)
	if nid == oldID || q.Photos[nid].Path != nph || q.Spreads[0].Photos[0] != nid || q.Photos[nid].Size != 7 {
		t.Fatalf("пути или ID не пересчитаны: %+v %+v", q.Photos, q.Spreads[0])
	}
	if !slices.Equal(q.Folders, []string{filepath.Join(moved, "Отбор")}) {
		t.Fatalf("папки: %v", q.Folders)
	}
}

// writeV2 пишет файл v2 с одним фото (path/abs как в файле) и возвращает путь.
func writeV2(t *testing.T, dir string, ph FilePhoto) string {
	t.Helper()
	f := FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Spreads: []*model.Spread{{ID: "s1", Photos: []string{"p"}}}, Photos: map[string]FilePhoto{"p": ph}}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func onlyRef(t *testing.T, p *model.Project) model.PhotoRef {
	t.Helper()
	if len(p.Photos) != 1 {
		t.Fatalf("фото: %+v", p.Photos)
	}
	for _, r := range p.Photos {
		return r
	}
	return model.PhotoRef{}
}

func TestLoadRelativeFallbackAbs(t *testing.T) {
	dir, other := testkit.TempDir(t), testkit.TempDir(t)
	abs := filepath.Join(other, "a.jpg")
	putFile(t, abs, 5)
	// По относительному пути нет — берётся запасной.
	path := writeV2(t, dir, FilePhoto{PhotoRef: model.PhotoRef{Path: "Отбор/a.jpg", W: 3, H: 2, Size: 5}, Abs: abs})
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r := onlyRef(t, q); r.Path != abs || r.Alt != "" || q.Spreads[0].Photos[0] != library.IDFor(abs) {
		t.Fatalf("ждали запасной путь: %+v", r)
	}
	// По относительному есть, но другой длины, а запасной совпадает — запасной.
	rel := filepath.Join(dir, "Отбор", "a.jpg")
	putFile(t, rel, 9)
	if q, _ = Load(path); onlyRef(t, q).Path != abs {
		t.Fatalf("ждали запасной путь при другой длине: %+v", onlyRef(t, q))
	}
	// Запасного нет — относительный другой длины, размеры читаются из файла.
	os.Remove(abs)
	img, _ := os.Create(rel)
	png.Encode(img, image.NewGray(image.Rect(0, 0, 30, 20)))
	img.Close()
	q, err = Load(path)
	if r := onlyRef(t, q); err != nil || r.Path != rel || r.W != 30 || r.H != 20 || r.Size == 5 {
		t.Fatalf("обновление по файлу: %v %+v", err, r)
	}
}

func TestLoadRejectsBadRelative(t *testing.T) {
	bad := []string{`C:x`, `\\srv\a`, `../../../x`, `a/../b`, `a:b`, "a\x00b", `a//b`, `./a`, `a/.`, `\x`}
	if runtime.GOOS == "windows" {
		bad = append(bad, `/x`, `a/NUL`, `COM1`)
	}
	for _, r := range bad {
		path := writeV2(t, testkit.TempDir(t), FilePhoto{PhotoRef: model.PhotoRef{Path: r, W: 1, H: 1}})
		if _, err := Load(path); err == nil {
			t.Errorf("путь %q должен отклоняться", r)
		}
		if CheckRel(r) == nil {
			t.Errorf("checkRel(%q) пропустил", r)
		}
	}
	for _, ok := range []string{".", "a.jpg", "Отбор/a.jpg", `Отбор\a.jpg`, "../a.jpg", "../../x/a.jpg"} {
		if err := CheckRel(ok); err != nil {
			t.Errorf("checkRel(%q): %v", ok, err)
		}
	}
	// Плохой запасной путь — тоже повреждённый файл.
	path := writeV2(t, testkit.TempDir(t), FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 1, H: 1}, Abs: `\\srv\share\a.jpg`})
	if _, err := Load(path); err == nil {
		t.Error("UNC в abs должен отклоняться")
	}
}

func TestLoadDedupe(t *testing.T) {
	dir := testkit.TempDir(t)
	ref := func(name string) (string, FilePhoto) {
		path := filepath.Join(dir, name+".jpg")
		return library.IDFor(path), FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: 3, H: 2}}
	}
	a, fa := ref("a")
	b, fb := ref("b")
	c, fc := ref("c")
	f := FileProject{Version: 2, Settings: model.DefaultSettings(), Spreads: []*model.Spread{
		{ID: "s1", Photos: []string{a, b, a}, Layout: "H(0,1,2)"},
		{ID: "s1", Photos: []string{b, c, "нет-записи"}},
		{ID: "s3", Photos: []string{"нет-записи"}},
		{ID: "", Photos: []string{}},
	}, Photos: map[string]FilePhoto{a: fa, b: fb, c: fc, "лишнее": fa}}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	os.WriteFile(path, data, 0o644)
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Spreads) != 3 || !slices.Equal(p.Spreads[0].Photos, []string{a, b}) || !slices.Equal(p.Spreads[1].Photos, []string{c}) {
		t.Fatalf("развороты: %+v %+v", p.Spreads[0], p.Spreads[1])
	}
	if p.Spreads[0].Layout != "" || p.Spreads[1].ID == "s1" || p.Spreads[2].ID == "" || len(p.Photos) != 3 {
		t.Fatalf("ключ %q, id %q %q, фото %d", p.Spreads[0].Layout, p.Spreads[1].ID, p.Spreads[2].ID, len(p.Photos))
	}
}

// Фото пересохранили повёрнутым (EXIF Orientation 6): размер файла другой,
// пропорции берутся из заголовка с учётом поворота, иначе раскладка исказит кадр.
func TestLoadChangedFileRotated(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEGOriented(t, dir, "a.jpg", 300, 200, 6)
	path := writeV2(t, dir, FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 300, H: 200, Size: 1}})
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r := onlyRef(t, p); r.W != 200 || r.H != 300 {
		t.Fatalf("размеры %dx%d, ждали 200x300 (поворот EXIF)", r.W, r.H)
	}
}

// Старый файл: стиль хранится в подписи целиком, Load его не пересчитывает.
func TestLoadKeepsOldCaptionSize(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "a.jpg")
	putFile(t, ph, 10)
	p, id := onePhoto(ph)
	r := p.Photos[id]
	r.Caption = &model.Caption{Text: "Старая", Style: model.ClampStyle(model.TextStyle{Font: "ptserif", Size: 9, Color: "#555555", Align: "center"})}
	p.Photos[id] = r
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Photos[id].Caption.Style.Size; got != 9 {
		t.Fatalf("кегль старой подписи: %v", got)
	}
}

// Фото на отключённом сетевом томе: Load не ждёт таймаута на каждом фото — одна проба тома
// на ProbeTimeout, ссылки остаются как в файле (путь, запасной путь, размеры).
func TestLoadDeadVolumeFast(t *testing.T) {
	if filepath.VolumeName(`Z:\x`) == "" {
		t.Skip("буквы дисков есть только в Windows")
	}
	release := make(chan struct{})
	defer close(release)
	var probes atomic.Int32
	vols := osx.NewVolumes(func(root string) (fs.FileInfo, error) {
		if strings.EqualFold(root, `Z:\`) {
			probes.Add(1)
			<-release // сеть отключена: корень не отвечает
		}
		return os.Stat(root)
	}, osx.ProbeTimeout)
	old := reachable
	reachable = vols.Reachable
	t.Cleanup(func() { reachable = old })

	const n = 200
	photos := map[string]any{}
	var spreads []map[string]any
	for i := range n {
		id := fmt.Sprintf("p%03d", i)
		photos[id] = map[string]any{"path": `Z:\Новая\` + id + ".jpg", "abs": `Z:\Старая\` + id + ".jpg",
			"w": 600, "h": 400, "size": 1000 + i}
		if i%5 == 0 {
			spreads = append(spreads, map[string]any{"id": fmt.Sprintf("s%d", i/5), "photos": []string{}})
		}
		s := spreads[len(spreads)-1]
		s["photos"] = append(s["photos"].([]string), id)
	}
	data, err := json.Marshal(map[string]any{"version": model.FormatVersion, "product": model.ProductAlbum,
		"folders":  []any{map[string]string{"path": `Z:\Новая`, "abs": `Z:\Старая`}},
		"settings": model.DefaultSettings(), "spreads": spreads, "photos": photos})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(testkit.TempDir(t), "альбом"+".salbum")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	p, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("Load %v: ждал мёртвый том", d)
	}
	if got := probes.Load(); got != 1 {
		t.Errorf("проб тома %d, ждали одну", got)
	}
	if len(p.Photos) != n {
		t.Fatalf("фото %d, ждали %d", len(p.Photos), n)
	}
	for _, r := range p.Photos {
		base := filepath.Base(r.Path)
		if r.Path != `Z:\Новая\`+base || r.Alt != `Z:\Старая\`+base || r.W != 600 || r.H != 400 || r.Size == 0 {
			t.Fatalf("ссылка изменилась: %+v", r)
		}
	}
	if len(p.Folders) != 1 || p.Folders[0] != `Z:\Новая` || p.FolderAlt[`Z:\Новая`] != `Z:\Старая` {
		t.Errorf("папки: %v, запасные %v", p.Folders, p.FolderAlt)
	}
}

// Размеры фото из .salbum — чужие данные: нелепые перечитываются из заголовка файла
// (Size == 0 — тоже), а без файла нелепые обнуляются, годные остаются.
func TestLoadDimsSanitized(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "a.jpg", 300, 200)
	for _, c := range []struct {
		name  string
		ref   model.PhotoRef
		path  string
		wantW int
		wantH int
	}{
		{"полоса 4000×10 при Size=0", model.PhotoRef{W: 4000, H: 10}, "a.jpg", 300, 200},
		{"больше MaxPixels", model.PhotoRef{W: 100000, H: 100000, Size: 1}, "a.jpg", 300, 200},
		{"нет размера файла", model.PhotoRef{W: 50, H: 50}, "a.jpg", 300, 200},
		{"нелепая пропорция, файла нет", model.PhotoRef{W: 4000, H: 10, Size: 5}, "нет.jpg", 0, 0},
		{"годные при отсутствии файла", model.PhotoRef{W: 300, H: 200, Size: 5}, "нет.jpg", 300, 200},
	} {
		c.ref.Path = c.path
		p, err := Load(writeV2(t, dir, FilePhoto{PhotoRef: c.ref}))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if r := onlyRef(t, p); r.W != c.wantW || r.H != c.wantH {
			t.Errorf("%s: размеры %dx%d, ждали %dx%d", c.name, r.W, r.H, c.wantW, c.wantH)
		}
	}
}

// Файл без product (до 0.5.0) и с чужим значением открывается как album; запись возвращает album.
func TestLoadProductDefault(t *testing.T) {
	if p := model.New(); p.Product != model.ProductAlbum {
		t.Fatalf("New: %q", p.Product)
	}
	dir := testkit.TempDir(t)
	for name, src := range map[string]string{
		"без поля": `{"version":4,"folders":[],"spreads":[],"photos":{}}`,
		"пусто":    `{"version":4,"product":"","folders":[],"spreads":[],"photos":{}}`,
		"чужое":    `{"version":4,"product":"calendar","folders":[],"spreads":[],"photos":{}}`,
	} {
		path := filepath.Join(dir, "a.salbum")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := Load(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Product != model.ProductAlbum {
			t.Errorf("%s: product %q", name, p.Product)
		}
		if err := Save(p, path); err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(path); !strings.Contains(string(data), `"product": "album"`) {
			t.Errorf("%s: в записанном файле нет product: %s", name, data)
		}
	}
}

// Сквозные проверки через Load (вариант A замера №2): разворот без фото, но с
// названием, своим фоном или текстом переживает dedupe и деление на части.

func TestLoadDedupeKeepsUserData(t *testing.T) {
	dir := testkit.TempDir(t)
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Photos: map[string]FilePhoto{}, Spreads: []*model.Spread{
		{ID: "plain", Photos: []string{"нет-1"}},
		{ID: "named", Photos: []string{"нет-2"}, Name: "Сборы"},
		{ID: "bg", Photos: []string{"нет-3"}, BgR: "#445566"},
		{ID: "text", Photos: []string{"нет-4"}, Texts: []*model.TextBlock{{ID: "t1", Text: "Привет"}}},
	}}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range p.Spreads {
		ids = append(ids, s.ID)
	}
	if !slices.Equal(ids, []string{"named", "bg", "text"}) {
		t.Fatalf("развороты после Load: %v", ids)
	}
}

// Editor: убрали все фото — разворот с названием/фоном/текстом остаётся, пустой уходит.
func TestLoadSplitBigKeepsFields(t *testing.T) {
	p, dir := model.New(), testkit.TempDir(t)
	ids := make([]string, model.MaxPhotos+1)
	for i := range ids {
		path := filepath.Join(dir, "p"+strconv.Itoa(i)+".jpg")
		ids[i] = library.IDFor(path)
		p.Photos[ids[i]] = model.PhotoRef{Path: path, W: 3, H: 2}
	}
	p.Spreads = []*model.Spread{{
		ID: "big", Photos: ids, Name: "Вступление", BgL: "#112233", BgR: "#445566",
		Texts: []*model.TextBlock{{ID: "t1", Text: "Привет"}},
	}}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Spreads) != 2 {
		t.Fatalf("развороты: %d", len(q.Spreads))
	}
	a, b := q.Spreads[0], q.Spreads[1]
	if a.ID != "big" || a.Name != "Вступление" || a.BgL != "#112233" || a.BgR != "#445566" ||
		len(a.Texts) != 1 || len(a.Photos) != model.MaxPhotos {
		t.Errorf("первая часть: %+v", a)
	}
	if b.ID == "big" || b.ID == "" || b.Name != "" || b.BgL != "" || b.BgR != "" ||
		len(b.Texts) != 0 || len(b.Photos) != 1 {
		t.Errorf("вторая часть: %+v", b)
	}
}
