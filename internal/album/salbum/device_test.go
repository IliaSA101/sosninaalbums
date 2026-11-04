package salbum

// Имена устройств DOS с расширением (073-10): «CON.jpg» на Windows 11 — обычный файл, на 10 —
// устройство. Load не падает ни там, ни там: на «старой ОС» такое фото — недостающее.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// devFile — ОС считает «CON.jpg» файлом (Windows 11, не Windows): тест создаёт файлы на диске.
// Иначе (Windows 10) — подмена оракула «как Windows 11», файлов нет (создать — открыть консоль).
func devFile(t *testing.T) bool {
	t.Helper()
	if filepath.IsLocal("CON.jpg") {
		return true
	}
	t.Cleanup(paths.SetDeviceOracle(func(string) bool { return false }))
	return false
}

// devAlbum — альбом в папке dir: фото «CON.jpg», «Отбор/aux.2024.jpg» (устройство с расширением)
// и обычные «a.jpg», «Отбор/b.jpg» в одном развороте; папки «Отбор» и «Con.2024». onDisk — файлы на
// диске (длины 10, 20, 30, 40). Возвращает проект, файл альбома и пути фото по порядку.
func devAlbum(t *testing.T, onDisk bool) (*model.Project, string, []string) {
	t.Helper()
	dir := testkit.TempDir(t)
	files := []string{
		filepath.Join(dir, "CON.jpg"), filepath.Join(dir, "Отбор", "aux.2024.jpg"),
		filepath.Join(dir, "a.jpg"), filepath.Join(dir, "Отбор", "b.jpg"),
	}
	p := model.New()
	s := &model.Spread{ID: "s1"}
	for i, f := range files {
		if onDisk {
			putFile(t, f, 10*(i+1))
		}
		id := library.IDFor(f)
		p.Photos[id] = model.PhotoRef{Path: f, W: 300, H: 200, Size: int64(10 * (i + 1))}
		s.Photos = append(s.Photos, id)
	}
	p.Spreads = []*model.Spread{s}
	// Папки — соседи, а не вложенные: вложенную Load сворачивает в корень (093-4).
	p.Folders = []string{filepath.Join(dir, "Отбор"), filepath.Join(dir, "Con.2024")}
	if onDisk {
		if err := os.MkdirAll(p.Folders[1], 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "Ивановы.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	return p, path, files
}

// На ОС, где «CON.jpg» — файл, альбом с такими фото и папкой «Con.2024» открывается целиком:
// пути, длины и папки те же, Check (NewEditor, черновик) их принимает.
func TestLoadDeviceNamesNewOS(t *testing.T) {
	onDisk := devFile(t)
	want, path, files := devAlbum(t, onDisk)
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Check(got); err != nil {
		t.Fatalf("Check: %v", err)
	}
	for i, f := range files {
		r, ok := got.Photos[library.IDFor(f)]
		if !ok || !paths.SamePath(r.Path, f) || r.Alt != "" || r.W != 300 || r.H != 200 {
			t.Fatalf("фото %d: %+v, есть %v", i, r, ok)
		}
		if onDisk && r.Size != int64(10*(i+1)) {
			t.Fatalf("фото %d: длина %d", i, r.Size)
		}
	}
	if !slices.EqualFunc(got.Folders, want.Folders, paths.SamePath) {
		t.Fatalf("папки %v, ждали %v", got.Folders, want.Folders)
	}
}

// На «старой ОС» (подмена: Windows 10 считает «CON.jpg» устройством) альбом открывается: фото с
// устройством в пути — недостающие (путь, размеры и место в развороте те же, к файлу Load не
// обращается), папка «Con.2024» пропущена, остальное — как было. Save→Load на ней же путь не
// теряет, а вернувшись на «новую ОС», фото снова на месте.
func TestLoadDeviceNamesOldOS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	onDisk := devFile(t)
	_, path, files := devAlbum(t, onDisk)
	restore := paths.SetDeviceOracle(paths.Device)
	defer restore()
	old := reachable
	reachable = func(p string) bool {
		if paths.CheckLocalPath(p) != nil {
			t.Errorf("Load обратился к пути с устройством: %s", filepath.Base(p))
		}
		return old(p)
	}
	t.Cleanup(func() { reachable = old })

	check := func(when string, p *model.Project, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: Load: %v", when, err)
		}
		if err := Check(p); err != nil {
			t.Fatalf("%s: Check: %v", when, err)
		}
		if len(p.Spreads) != 1 || len(p.Spreads[0].Photos) != len(files) {
			t.Fatalf("%s: развороты %+v", when, p.Spreads)
		}
		for i, f := range files {
			r, ok := p.Photos[library.IDFor(f)]
			if !ok || !paths.SamePath(r.Path, f) || r.Alt != "" || r.W != 300 || r.H != 200 || r.Size != int64(10*(i+1)) {
				t.Fatalf("%s: фото %d: %+v, есть %v", when, i, r, ok)
			}
			if dev := errors.Is(paths.CheckLocalPath(r.Path), paths.ErrDevice); dev != (i < 2) {
				t.Fatalf("%s: фото %d: устройство %v", when, i, dev)
			}
		}
		if len(p.Folders) != 1 || strings.Contains(p.Folders[0], "Con.2024") {
			t.Fatalf("%s: папки %v, ждали без Con.2024", when, p.Folders)
		}
	}
	p, err := Load(path)
	check("Load", p, err)
	again := filepath.Join(filepath.Dir(path), "Второй.salbum")
	if err := Save(p, again); err != nil {
		t.Fatal(err)
	}
	p, err = Load(again)
	check("Save→Load", p, err)

	restore()
	reachable = old
	p, err = Load(again)
	if err != nil {
		t.Fatalf("снова Windows 11: %v", err)
	}
	for _, f := range files[:2] {
		if r := p.Photos[library.IDFor(f)]; !paths.SamePath(r.Path, f) {
			t.Fatalf("снова Windows 11: путь %q, ждали %q", r.Path, f)
		}
	}
}

// Устройство в пути без годного второго пути — недостающее, а с годным запасным abs —
// запасной; устройство в abs при годном основном — запасного нет. Размеры без проверки файла.
func TestLoadDeviceCandidates(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	t.Cleanup(paths.SetDeviceOracle(paths.Device))
	dir := testkit.TempDir(t)
	good := filepath.Join(dir, "x", "a.jpg")
	putFile(t, good, 7)
	for _, c := range []struct {
		name      string
		ph        FilePhoto
		path, alt string
	}{
		{"относительный — устройство, abs годный", FilePhoto{PhotoRef: model.PhotoRef{Path: "CON.jpg", W: 3, H: 2}, Abs: good}, good, ""},
		{"abs — устройство, основной годный", FilePhoto{PhotoRef: model.PhotoRef{Path: "x/a.jpg", W: 3, H: 2}, Abs: filepath.Join(dir, "nul.jpg")}, good, ""},
		{"оба — устройства", FilePhoto{PhotoRef: model.PhotoRef{Path: "Prn.jpg", W: 3, H: 2}, Abs: filepath.Join(dir, "aux.jpg")}, filepath.Join(dir, "Prn.jpg"), ""},
		{"без расширения, без размеров", FilePhoto{PhotoRef: model.PhotoRef{Path: "a/NUL"}}, filepath.Join(dir, "a", "NUL"), ""},
		{"абсолютный — устройство", FilePhoto{PhotoRef: model.PhotoRef{Path: filepath.Join(dir, "COM1.jpg"), W: 3, H: 2}}, filepath.Join(dir, "COM1.jpg"), ""},
	} {
		p, err := Load(writeV2(t, dir, c.ph))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		r := onlyRef(t, p)
		if !paths.SamePath(r.Path, c.path) || r.Alt != c.alt || r.W != c.ph.W || r.H != c.ph.H {
			t.Errorf("%s: %+v, ждали путь %q", c.name, r, c.path)
		}
		if err := Check(p); err != nil {
			t.Errorf("%s: Check: %v", c.name, err)
		}
	}
	// Альбом не с локального диска (base ""): относительный не годится, abs — устройство —
	// недостающее по abs; без abs — прежняя ошибка «не на локальном диске».
	devAbs := filepath.Join(dir, "aux.jpg")
	data, _ := os.ReadFile(writeV2(t, dir, FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 3, H: 2}, Abs: devAbs}))
	if p, err := Decode(data, ""); err != nil || !paths.SamePath(onlyRef(t, p).Path, devAbs) {
		t.Errorf("base пуст, abs — устройство: %v", err)
	}
	data, _ = os.ReadFile(writeV2(t, dir, FilePhoto{PhotoRef: model.PhotoRef{Path: "CON.jpg", W: 3, H: 2}}))
	if _, err := Decode(data, ""); err == nil || errors.Is(err, paths.ErrDevice) {
		t.Errorf("base пуст, без abs: %v, ждали «не на локальном диске»", err)
	}
}

// CheckRel: устройство в элементе — ErrDevice (не «повреждён»); без расширения — на любой
// версии Windows, с расширением — как судит ОС. Порча — не ErrDevice.
func TestCheckRelDevice(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	restore := paths.SetDeviceOracle(paths.Device)
	for _, r := range []string{"Отбор/CON.jpg", "aux.2024", "a/NUL", "COM1", "../LPT1./a.jpg"} {
		if err := CheckRel(r); !errors.Is(err, paths.ErrDevice) {
			t.Errorf("Windows 10: CheckRel(%q) = %v, ждали ErrDevice", r, err)
		}
	}
	restore()
	t.Cleanup(paths.SetDeviceOracle(func(string) bool { return false }))
	for _, r := range []string{"Отбор/CON.jpg", "aux.2024", "../Nul.salbum"} {
		if err := CheckRel(r); err != nil {
			t.Errorf("Windows 11: CheckRel(%q) = %v", r, err)
		}
	}
	for _, r := range []string{"a/NUL", "CON . ."} {
		if err := CheckRel(r); !errors.Is(err, paths.ErrDevice) {
			t.Errorf("Windows 11: CheckRel(%q) = %v, ждали ErrDevice", r, err)
		}
	}
	for _, r := range []string{"a/../b", "a:b", `\x`} {
		if err := CheckRel(r); err == nil || errors.Is(err, paths.ErrDevice) {
			t.Errorf("CheckRel(%q) = %v, ждали порчу без ErrDevice", r, err)
		}
	}
}
