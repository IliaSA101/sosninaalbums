package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Типографии из наборов библиотеки (123 Б4, design §4): слой реестра, ставит только каталог.

// pfile — предмет тестового набора: имя записи, вид и байты.
type pfile struct {
	name, kind string
	data       []byte
}

// packZip — набор id версии version с minApp 0.26.0; манифест считает sha256 и размеры сам.
func packZip(t *testing.T, id string, version int, files ...pfile) string {
	t.Helper()
	items := make([]map[string]any, 0, len(files))
	for _, f := range files {
		sum := sha256.Sum256(f.data)
		items = append(items, map[string]any{"file": f.name, "kind": f.kind, "theme": "basic", "name": f.name,
			"sha256": hex.EncodeToString(sum[:]), "size": len(f.data)})
	}
	man, err := json.Marshal(map[string]any{"format": 1, "id": id, "version": version, "minApp": "0.26.0",
		"title": "Набор " + id, "license": "CC0-1.0", "items": items})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range append([]pfile{{name: "manifest.json", data: man}}, files...) {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), id+"-"+itoa(version)+".zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// libProfile — профиль набора на основе Фабрики: свой id, название и версия, изделия под id.
func libProfile(t *testing.T, id, name string, version int) []byte {
	t.Helper()
	p, ok := printprof.Builtin().ByID("fabrika")
	if !ok {
		t.Fatal("нет Фабрики")
	}
	m := map[string]string{}
	for i := range p.Products {
		old := p.Products[i].ID
		p.Products[i].ID = id + "-" + itoa(i+1)
		m[old] = p.Products[i].ID
	}
	p.Default = m[p.Default]
	p.ID, p.Name, p.Gen, p.Version = id, name, name, version
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// fabrikaV — Фабрика версии v с подписью в подсказке: по ней видно, чей профиль действует.
func fabrikaV(t *testing.T, v int, mark string) []byte {
	t.Helper()
	p, _ := printprof.Builtin().ByID("fabrika")
	p.Version = v
	p.Page.Hint += " [" + mark + "]"
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hintOf(t *testing.T, id string) string {
	t.Helper()
	p, ok := printprof.Current().ByID(id)
	if !ok {
		t.Fatalf("нет типографии %s", id)
	}
	return p.Page.Hint
}

func catchLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// libApp — App с сетевой библиотекой и реестром из встроенных.
func libApp(t *testing.T) (*App, *netLib, *capture) {
	t.Helper()
	a, l := netApp(t, true)
	printprof.UseForTest(t, nil)
	a.ownp.Reload()
	return a, l, captureEvents(a)
}

func publishPrints(t *testing.T, l *netLib, zips ...string) {
	t.Helper()
	l.publish(t, func(r map[string]any) { r["counts"] = map[string]int{"print": 1} }, zips...)
}

func updateOK(t *testing.T, a *App) LibraryUpdate {
	t.Helper()
	res, err := a.UpdateLibrary()
	if err != nil {
		t.Fatalf("обновление: %v", err)
	}
	return res
}

func printFiles(t *testing.T, a *App) []string {
	t.Helper()
	list, err := os.ReadDir(filepath.Join(a.mustStore(t).Root(), "Типографии"))
	if err != nil {
		return nil
	}
	var out []string
	for _, de := range list {
		out = append(out, de.Name())
	}
	return out
}

// Набор из подписанного каталога с «Фабрикой» версии 3 перекрывает встроенную, откат —
// удалить набор; круг через перезапуск (реестр читается с диска); событие print:changed.
func TestPackPrintOverrideAndRollback(t *testing.T) {
	a, l, ev := libApp(t)
	builtinHint := hintOf(t, "fabrika")
	publishPrints(t, l, filepath.Join("testdata", "printpack", "print-fabrika-1.zip"))
	res := updateOK(t, a)
	if res.Added != 1 || len(res.Skipped) != 0 {
		t.Fatalf("установка: %+v", res)
	}
	got, _ := printprof.Current().ByID("fabrika")
	if got.Version != 3 || !strings.Contains(got.Page.Hint, "поправка набора") || printprof.Current().Source("fabrika") != printprof.SourceLibrary {
		t.Fatalf("перекрытие не действует: версия %d, слой %s", got.Version, printprof.Current().Source("fabrika"))
	}
	if n := len(ev.of("print:changed")); n != 1 {
		t.Errorf("print:changed после установки: %d", n)
	}
	list, _ := a.LibraryPacks()
	if p := list.Packs[len(list.Packs)-1]; p.ID != "print-fabrika" || p.Print != 1 || p.Bg+p.Sticker+p.Spread+p.Frame != 0 || p.Origin != decor.OriginNet {
		t.Errorf("строка набора: %+v", p)
	}
	if infos := a.PrintProfiles(); len(infos) != 3 || infos[2].Version != 3 || infos[2].Source != printprof.SourceLibrary {
		t.Errorf("список типографий: %+v", infos)
	}
	// Перезапуск: новое хранилище читает реестр наборов и файл профиля.
	printprof.Install(nil)
	a.openDecor()
	a.ownp.Reload()
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 3 {
		t.Fatalf("после перезапуска версия %d", got.Version)
	}
	if len(printFiles(t, a)) != 1 {
		t.Fatalf("файлы типографий: %v", printFiles(t, a))
	}
	// Откат: удалить набор.
	if _, err := a.RemovePack("print-fabrika"); err != nil {
		t.Fatal(err)
	}
	if hintOf(t, "fabrika") != builtinHint || printprof.Current().Source("fabrika") != printprof.SourceBuiltin {
		t.Error("встроенная не вернулась после удаления набора")
	}
	if n := len(ev.of("print:changed")); n != 2 {
		t.Errorf("print:changed после удаления: %d", n)
	}
	if f := printFiles(t, a); len(f) != 0 {
		t.Errorf("файл профиля остался: %v", f)
	}
	// «Вернуть удалённые» и обновление поднимают перекрытие снова (каталог с seq+1).
	if _, err := a.ResetLibrarySkip(); err != nil {
		t.Fatal(err)
	}
	l.seq = 2
	publishPrints(t, l, filepath.Join("testdata", "printpack", "print-fabrika-1.zip"))
	updateOK(t, a)
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 3 {
		t.Errorf("повторная установка: версия %d", got.Version)
	}
}

// Встроенная новой программы старше набора не уступает: набор версии 3 при встроенной 4 и
// равная версия — встроенная действует, строка в log.txt; альбомы не двигаются.
func TestPackPrintBuiltinNewer(t *testing.T) {
	a, l, ev := libApp(t)
	logBuf := catchLog(t)
	b3, _ := printprof.Builtin().ByID("fabrika")
	b3.Version = 4
	b3.Page.Hint += " [встроенная 4]"
	base, _ := printprof.Builtin().ByID("fsfera")
	nb, _ := printprof.Builtin().ByID("netprint")
	snap, err := printprof.NewSnapshot([]printprof.Profile{base, nb, b3})
	if err != nil {
		t.Fatal(err)
	}
	printprof.UseBuiltinForTest(t, snap)
	printprof.Install(nil)
	publishPrints(t, l, filepath.Join("testdata", "printpack", "print-fabrika-1.zip"))
	updateOK(t, a)
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 4 || printprof.Current().Source("fabrika") != printprof.SourceBuiltin {
		t.Errorf("действует версия %d (%s)", got.Version, printprof.Current().Source("fabrika"))
	}
	if !strings.Contains(logBuf.String(), `типография "fabrika" пропущена: версия не новее`) {
		t.Errorf("нет строки в log.txt:\n%s", logBuf.String())
	}
	if n := len(ev.of("print:changed")); n != 0 {
		t.Errorf("реестр не изменился, а print:changed %d", n)
	}
	// Программу понизили: набор требует 0.26.0 — не действует, остальное работает.
	printprof.UseBuiltinForTest(t, nil)
	printprof.Install(nil)
	a.ownp.Reload()
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 3 {
		t.Fatalf("набор не действует при dev: %d", got.Version)
	}
	defer func(old []byte) { wailsJSON = old }(wailsJSON)
	wailsJSON = []byte(`{"info":{"productVersion":"0.25.1"}}`)
	a.ownp.Reload()
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 2 {
		t.Errorf("набор новее программы действует: %d", got.Version)
	}
	if !strings.Contains(logBuf.String(), "нужна версия Sosnina Albums 0.26.0") {
		t.Errorf("нет строки о minApp:\n%s", logBuf.String())
	}
}

// Из двух наборов с одним id действует старшая версия при любом порядке установки, при равной —
// установленный раньше.
func TestPackPrintTwoPacks(t *testing.T) {
	for name, c := range map[string]struct {
		va, vb int
		want   string
	}{
		"позже старше":  {3, 4, "B"},
		"раньше старше": {4, 3, "A"},
		"равные":        {3, 3, "A"},
	} {
		t.Run(name, func(t *testing.T) {
			a, l, _ := libApp(t)
			za := packZip(t, "print-a", 1, pfile{"fabrika.saprint", "print", fabrikaV(t, c.va, "A")})
			zb := packZip(t, "print-b", 1, pfile{"fabrika.saprint", "print", fabrikaV(t, c.vb, "B")})
			publishPrints(t, l, za, zb)
			updateOK(t, a)
			if h := hintOf(t, "fabrika"); !strings.HasSuffix(h, "["+c.want+"]") {
				t.Errorf("действует %q, ждали %s", h, c.want)
			}
			// Удалили победителя — действует другая (или встроенная при равных/младшей).
			win := "print-" + strings.ToLower(c.want)
			if _, err := a.RemovePack(win); err != nil {
				t.Fatal(err)
			}
			other := "A"
			if c.want == "A" {
				other = "B"
			}
			if h := hintOf(t, "fabrika"); !strings.HasSuffix(h, "["+other+"]") {
				t.Errorf("после удаления %s: %q", win, h)
			}
		})
	}
}

// Дубль названия: позже загруженный пропускается со строкой в log.txt (id, не имя файла); своя
// типография с таким же названием набором не вытесняется.
func TestPackPrintDuplicateName(t *testing.T) {
	a, l, _ := libApp(t)
	logBuf := catchLog(t)
	za := packZip(t, "print-a", 1, pfile{"novaya.saprint", "print", libProfile(t, "novaya-a", "Новая типография", 1)})
	zb := packZip(t, "print-b", 1, pfile{"novaya.saprint", "print", libProfile(t, "novaya-b", "Новая типография", 1)})
	publishPrints(t, l, za, zb)
	updateOK(t, a)
	if _, ok := printprof.Current().ByID("novaya-a"); !ok {
		t.Error("первая из двух не действует")
	}
	if _, ok := printprof.Current().ByID("novaya-b"); ok {
		t.Error("дубль названия принят")
	}
	// В log.txt — id, а не название (аудит 123 №7).
	if out := logBuf.String(); !strings.Contains(out, `типография "novaya-b" пропущена: название уже есть у типографии "novaya-a"`) ||
		strings.Contains(out, "Новая типография") || strings.Contains(out, "novaya.saprint") || strings.Contains(out, ".zip") {
		t.Errorf("log.txt:\n%s", out)
	}
	// Название занято своей: набор пропускается, своя остаётся.
	if _, err := a.RemovePack("print-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("print-b"); err != nil {
		t.Fatal(err)
	}
	saved(t, a, editOf(t, a, "fabrika", "Моя типография"))
	zc := packZip(t, "print-c", 1, pfile{"moya.saprint", "print", libProfile(t, "moya-c", "Моя типография", 1)})
	l.seq = 3
	publishPrints(t, l, zc)
	updateOK(t, a)
	if _, ok := printprof.Current().ByID("moya-c"); ok {
		t.Error("набор вытеснил название своей")
	}
	if ownCount(a) != 1 {
		t.Errorf("своих %d, ждали 1", ownCount(a))
	}
	if !strings.Contains(logBuf.String(), `типография "moya-c" набора print-c пропущена: название уже занято своей`) {
		t.Errorf("нет строки о своей:\n%s", logBuf.String())
	}
}

// Zip-файл пользователя: типография пропускается с причиной, остальное ставится; набор из одних
// типографий — «нечего ставить» с причиной; реестр не меняется, файла профиля на диске нет.
func TestPackPrintFromFileSkipped(t *testing.T) {
	a, _, ev := libApp(t)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	only := packZip(t, "print-only", 1, pfile{"fabrika.saprint", "print", fabrikaV(t, 3, "файл")})
	_, err := a.AddPackPath(only)
	if err == nil || !strings.Contains(err.Error(), "из файла она не принимается") || strings.Contains(err.Error(), only) {
		t.Fatalf("набор из одних типографий: %v", err)
	}
	mixed := packZip(t, "print-mixed", 1, pfile{"bg/one.png", "bg", pngBuf.Bytes()}, pfile{"fabrika.saprint", "print", fabrikaV(t, 3, "файл")})
	res, err := a.AddPackPath(mixed)
	if err != nil || res.Pack.Bg != 1 || res.Pack.Print != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "из файла") {
		t.Fatalf("смешанный набор: %v %+v", err, res)
	}
	if got, _ := printprof.Current().ByID("fabrika"); got.Version != 2 {
		t.Errorf("профиль из файла действует: версия %d", got.Version)
	}
	if f := printFiles(t, a); len(f) != 0 {
		t.Errorf("файл профиля записан: %v", f)
	}
	if n := len(ev.of("print:changed")); n != 0 {
		t.Errorf("print:changed %d без изменений реестра", n)
	}
}

// Битый профиль в наборе: профиль пропускается с причиной, остальные предметы и профили набора
// ставятся; набор из одного плохого — «нечего ставить», реестр прежний.
func TestPackPrintBrokenProfile(t *testing.T) {
	good := libProfile(t, "novaya-a", "Новая типография", 1)
	cases := map[string][]byte{
		"не json":     []byte("{не json"),
		"пустой":      {},
		"массив":      []byte("[]"),
		"усечённый":   good[:len(good)/2],
		"лишнее поле": bytes.Replace(good, []byte(`"format"`), []byte(`"extra": 1, "format"`), 1),
		"версия 0":    bytes.Replace(good, []byte(`"version": 1`), []byte(`"version": 0`), 1),
		"мусор после": append(slices.Clone(good), []byte(` {"x":1}`)...),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			a, l, ev := libApp(t)
			z := packZip(t, "print-bad", 1, pfile{"bad.saprint", "print", bad})
			publishPrints(t, l, z)
			res, err := a.UpdateLibrary()
			if err != nil || res.Added != 0 || len(res.Skipped) == 0 {
				t.Fatalf("плохой профиль: %v %+v", err, res)
			}
			if len(a.PrintProfiles()) != 3 || len(ev.of("print:changed")) != 0 || len(printFiles(t, a)) != 0 {
				t.Errorf("реестр изменился: %d типографий, файлов %v", len(a.PrintProfiles()), printFiles(t, a))
			}
		})
	}
	a, l, _ := libApp(t)
	z := packZip(t, "print-mix", 1, pfile{"bad.saprint", "print", []byte("{")}, pfile{"novaya.saprint", "print", good})
	publishPrints(t, l, z)
	res := updateOK(t, a)
	if res.Added != 1 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "не принята") {
		t.Fatalf("набор с одним плохим: %+v", res)
	}
	if _, ok := printprof.Current().ByID("novaya-a"); !ok {
		t.Error("хороший профиль набора не действует")
	}
}

// Файл профиля на диске испортили после установки (усечён, пуст, больше 64 КБ, не JSON): он
// пропускается со строкой в log.txt, остальные типографии работают, встроенная возвращается.
func TestPackPrintBrokenFileOnDisk(t *testing.T) {
	for name, content := range map[string][]byte{
		"усечён":   []byte(`{"format": 1, "id": "fab`),
		"пуст":     {},
		"огромный": bytes.Repeat([]byte("x"), 64<<10+1),
		"не json":  []byte("мусор"),
	} {
		t.Run(name, func(t *testing.T) {
			a, l, _ := libApp(t)
			logBuf := catchLog(t)
			publishPrints(t, l, filepath.Join("testdata", "printpack", "print-fabrika-1.zip"))
			updateOK(t, a)
			path := filepath.Join(a.mustStore(t).Root(), "Типографии", printFiles(t, a)[0])
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
			a.ownp.Reload()
			if got, _ := printprof.Current().ByID("fabrika"); got.Version != 2 || len(a.PrintProfiles()) != 3 {
				t.Errorf("версия %d, типографий %d", got.Version, len(a.PrintProfiles()))
			}
			if name == "не json" || name == "усечён" {
				if !strings.Contains(logBuf.String(), "типография набора print-fabrika пропущена") {
					t.Errorf("нет строки в log.txt:\n%s", logBuf.String())
				}
			}
		})
	}
}
